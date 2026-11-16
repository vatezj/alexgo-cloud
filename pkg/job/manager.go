package job

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"alexGo-cloud/pkg/config"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/logger"
)

// Fn 任务体。error 只记 warn，不影响其他任务（由 Manager 外壳统一处理）。
type Fn func(ctx context.Context) error

// Status 单个任务的对外状态（admin GET /api/admin/jobs 的 data 元素）。
type Status struct {
	Name           string    `json:"name"`
	Module         string    `json:"module"`
	Spec           string    `json:"spec"`             // 生效表达式；停用时为注册缺省（诊断用）
	Enabled        bool      `json:"enabled"`
	DisabledReason string    `json:"disabledReason,omitempty"` // module_off | spec_empty
	Running        bool      `json:"running"`
	LastStart      time.Time `json:"lastStart"`
	LastEnd        time.Time `json:"lastEnd"`
	NextRun        time.Time `json:"nextRun"`
	LastErr        string    `json:"lastErr,omitempty"`
	RunCount       uint64    `json:"runCount"`
}

// entry 注册表条目：调度面（enabled/spec/entryID）与执行面（run 状态）合一。
type entry struct {
	name, module, defaultSpec string
	fn                        Fn
	enabled                   bool
	disabledReason            string // module_off | spec_empty | ""
	spec                      string // 生效表达式
	entryID                   cron.EntryID

	running int32 // atomic 计数：并发自增/自减，Running = counter > 0（spec §4.4 "至少一个在跑"）
	mu      sync.Mutex
	// 执行状态由 mu 保护（run 频率低，不需要 lock-free）。
	lastStart time.Time
	lastEnd   time.Time
	lastErr   string
	runCount  uint64
}

// run 执行外壳：记状态 + panic recover 转 error（Review Focus 4）。
func (e *entry) run(ctx context.Context) (err error) {
	atomic.AddInt32(&e.running, 1)
	e.mu.Lock()
	e.lastStart = time.Now()
	e.mu.Unlock()

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
		e.mu.Lock()
		e.lastEnd = time.Now()
		e.runCount++
		if err != nil {
			e.lastErr = err.Error()
		} else {
			e.lastErr = ""
		}
		e.mu.Unlock()
		atomic.AddInt32(&e.running, -1)
	}()

	return e.fn(ctx)
}

// Manager job 注册表 + robfig/cron 生命周期（fx 单例，见 FxModule）。
type Manager struct {
	cfg *config.Config

	mu      sync.Mutex
	byName  map[string]*entry
	order   []string // 注册序（List 输出确定性）
	started bool

	cron *cron.Cron
}

// NewManager 构造空注册表。cfg 为 nil 时退化为空配置（所有模块视为开、无覆盖）。
func NewManager(cfg *config.Config) *Manager {
	if cfg == nil {
		cfg = &config.Config{}
	}
	return &Manager{cfg: cfg, byName: map[string]*entry{}}
}

// Register 注册任务。name 约定 "模块.任务名"；必须在 fx.Invoke 阶段调用
// （OnStart 之前）——启动后注册 panic，重复 name panic，无点 name panic。
func (m *Manager) Register(name, defaultSpec string, fn Fn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.started {
		panic(fmt.Sprintf("job: Register(%q) 在启动后调用", name))
	}
	if !strings.Contains(name, ".") {
		panic(fmt.Sprintf("job: name %q 必须是 模块.任务名 格式", name))
	}
	if _, dup := m.byName[name]; dup {
		panic(fmt.Sprintf("job: 重复注册 %q", name))
	}
	mod := name[:strings.Index(name, ".")]
	m.byName[name] = &entry{name: name, module: mod, defaultSpec: defaultSpec, fn: fn}
	m.order = append(m.order, name)
}

// List 按注册序返回全部任务状态（含停用任务）。
func (m *Manager) List() []Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Status, 0, len(m.order))
	for _, name := range m.order {
		e := m.byName[name]
		e.mu.Lock()
		st := Status{
			Name:           e.name,
			Module:         e.module,
			Spec:           e.spec,
			Enabled:        e.enabled,
			DisabledReason: e.disabledReason,
			Running:        atomic.LoadInt32(&e.running) > 0,
			LastStart:      e.lastStart,
			LastEnd:        e.lastEnd,
			LastErr:        e.lastErr,
			RunCount:       e.runCount,
		}
		e.mu.Unlock()
		if st.Spec == "" {
			st.Spec = e.defaultSpec // 未启动/停用时展示缺省，便于运维对照
		}
		if e.enabled && m.cron != nil {
			st.NextRun = m.cron.Entry(e.entryID).Next
		}
		out = append(out, st)
	}
	return out
}

// Trigger 同步执行一次任务。未注册 → 30601；停用（模块关/配置空串）→ 30602；
// 否则原样返回任务体错误（非 BizError 由 resp.Fail 兜底 50000）。
func (m *Manager) Trigger(ctx context.Context, name string) error {
	m.mu.Lock()
	e, ok := m.byName[name]
	started := m.started
	m.mu.Unlock()
	if !ok {
		return errs.New(errs.CodeJobNotFound, fmt.Errorf("job %q 未注册", name))
	}
	if !started {
		return errs.New(errs.CodeJobDisabled, fmt.Errorf("job %q 尚未启动", name))
	}
	if !e.enabled {
		return errs.New(errs.CodeJobDisabled, fmt.Errorf("job %q 已停用: %s", name, e.disabledReason))
	}
	return e.run(ctx)
}

// FxModule pkg/job 的 fx 装配：提供 Manager 单例 + 挂生命周期。
// 模块侧在各自 fx.Invoke 里 m.Register(...)（必须先于 OnStart，fx Invoke 顺序保证）。
var FxModule = fx.Module("job",
	fx.Provide(NewManager),
	fx.Invoke(BindLifecycle),
)

// BindLifecycle fx 生命周期：OnStart 解析表达式并排程（开关守卫先于解析），
// OnStop 等在跑任务收尾（同时受 ctx 截断防拖死关停）。
func BindLifecycle(lc fx.Lifecycle, m *Manager) {
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error { return m.start() },
		OnStop: func(ctx context.Context) error {
			m.mu.Lock()
			c := m.cron
			m.mu.Unlock()
			if c == nil {
				return nil
			}
			select {
			case <-c.Stop().Done():
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})
}

// start 解析全部已注册任务并起 cron。
// 顺序钉死（Review Focus 1）：模块开关守卫 → 配置覆盖/空串停用 → 表达式解析。
// 守卫在解析之前：关模块 + 非法表达式必须启动成功（运维关模块不应被坏配置卡死进程）。
func (m *Manager) start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.started {
		return nil
	}

	c := cron.New(cron.WithParser(parser))
	registered := map[string]bool{}
	jobs := m.cfg.FlatJobs() // 嵌套配置摊平成 "模块.任务名"（viper 按点拆 key，见 config.Jobs）

	for _, name := range m.order {
		e := m.byName[name]
		registered[name] = true

		// 1) 模块开关守卫（先于一切解析）。
		if m.cfg.Modules != nil {
			if on, ok := m.cfg.Modules[e.module]; ok && !on {
				e.enabled, e.disabledReason = false, "module_off"
				continue
			}
		}

		// 2) 配置覆盖：键存在空串 = 停用；键存在非空 = 覆盖；缺键 = defaultSpec。
		spec := e.defaultSpec
		if v, ok := jobs[name]; ok {
			if v == "" {
				e.enabled, e.disabledReason = false, "spec_empty"
				continue
			}
			spec = v
		}

		// 3) 解析（TZSpec 包时区前缀），失败拒启。
		parsed, err := parser.Parse(TZSpec(spec))
		if err != nil {
			return fmt.Errorf("注册 job %s 表达式 %q: %w", name, spec, err)
		}
		e.spec = spec
		e.enabled, e.disabledReason = true, ""
		e.entryID = c.Schedule(parsed, cron.FuncJob(func() {
			// cron 路径用 Background：不受生命周期/请求 ctx 影响（项目惯例）。
			if err := e.run(context.Background()); err != nil && logger.Log != nil {
				logger.Log.Warn("job failed", zap.String("job", name), zap.Error(err))
			}
		}))
	}

	// 4) 配置里未注册的 key：只 warn（写错名可见，不炸启动）。
	for k := range jobs {
		if !registered[k] && logger.Log != nil {
			logger.Log.Warn("job config key not registered", zap.String("key", k))
		}
	}

	c.Start()
	m.cron = c
	m.started = true
	return nil
}
