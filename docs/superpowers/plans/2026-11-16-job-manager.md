# Job Manager（pkg/job 统一模块任务设施）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让每个模块一行 `m.Register(name, 默认表达式, fn)` 即可挂后台 job，起停/时区/panic 恢复/模块开关/配置改期/手动触发全部收敛到 `pkg/job`。

**Architecture:** `pkg/job` 提供 fx 单例 Manager（注册表 + robfig/cron 生命周期 + 状态跟踪），模块在 fx.Invoke 阶段注册任务；表达式覆盖表进 `config.yaml` 的 `jobs:` 段；admin 两条路由（列表/触发）挂 system 模块并以 `system:job` 权限键走既有 casbin 前缀鉴权；现有三处任务面（starfuyu cron、system/scorecard ticker）全部迁入。

**Tech Stack:** Go 1.x、uber/fx（fxtest）、robfig/cron/v3 v3.0.1、gin、casbin（keyMatch2）、viper。

**Spec:** `docs/superpowers/specs/2026-11-16-job-manager-design.md`

## Global Constraints

- 提交直接落 `main` 分支（本仓库不建功能分支）；每个 Task 一次提交。
- 注释、提交信息、文档用中文（项目惯例）。
- cron：5 段标准 parser（`Minute|Hour|Dom|Month|Dow|Descriptor`），**不 WithSeconds**；非 `@` 开头的表达式统一加 `CRON_TZ=Asia/Shanghai` 前缀，`@` 开头（`@every`/`@hourly`）原样不加。
- 进程级嵌入 `time/tzdata`（`pkg/job` 内 import `_ "time/tzdata"`，瘦身容器无 zoneinfo）。
- 任务体错误只 `logger.Log.Warn` 不致命；panic 由 safeJob recover 成 warn，不崩进程。
- 模块开关守卫（`cfg.Modules[模块段] == false`）必须在**表达式解析之前** return——关模块 + 非法表达式必须能启动。
- admin 路由统一信封 `resp.OK/resp.Fail`，casbin 全局中间件鉴权，handler 不取 claims。
- 新错误码占 `306xx` 段（303=social、304=fishgame、305=starfuyu 已占；306 空闲）。
- 任务体签名恒为 `func(ctx context.Context) error`；cron 路径传 `context.Background()`，手动 trigger 传 request ctx。
- 不做：执行历史持久化、重试/死信、前端页面、多实例去重、分布式锁。

## Review Focus

1. **模块开关先于表达式解析**——关模块 + 非法表达式必须启动成功（否则运维改配置后进程起不来）；钉在 Task 2 `TestStart_ModuleOffSkipsParse`。
2. **`cfg.Jobs` 空串 = 停用、未知 key 只 warn**——运维空串不能触发解析报错，写错 key 名不能炸启动；钉在 Task 2 `TestStart_ConfigEmptyDisables` + `TestStart_UnknownConfigKeyWarnsOnly`。
3. **Trigger 拒绝停用任务**——`modules.starfuyu: false` 时手动触发不能绕过开关打 DeepSeek；钉在 Task 2 `TestTrigger_Disabled30602`。
4. **任务 panic 被 recover 且状态可查**——fn panic 不外溢、`lastErr` 有记录、后续 trigger 仍可用；钉在 Task 2 `TestTrigger_PanicRecovered`。
5. **seed 按钮幂等且与 permissionRoutes 同键**——`system:job` 按钮缺了则新端点全员 403，判据不同尺度则每次启动重复插菜单；钉在 Task 4 `TestSeedJobButtons_MatchRouteKeys` + `TestSeedJobButtons_Idempotent`。

---

### Task 1: `pkg/job` 表达式解析（spec.go）

**Files:**
- Create: `pkg/job/spec.go`
- Test: `pkg/job/spec_test.go`

**Interfaces:**
- Consumes: `github.com/robfig/cron/v3`（go.mod 已有 v3.0.1）
- Produces:
  - `var parser cron.Parser`（5 段 + Descriptor，包内共用）
  - `func TZSpec(expr string) string` —— 非 `@` 表达式加 `CRON_TZ=Asia/Shanghai ` 前缀，`@` 开头原样返回

- [ ] **Step 1: 写失败测试**

```go
// pkg/job/spec_test.go
package job

import "testing"

// TestTZSpec_DescriptorUnchanged @every/@hourly 原样返回（加 TZ 前缀反而可能不被 parser 接受）。
func TestTZSpec_DescriptorUnchanged(t *testing.T) {
	for _, expr := range []string{"@every 1m", "@hourly"} {
		if got := TZSpec(expr); got != expr {
			t.Fatalf("TZSpec(%q) = %q, want 原样", expr, got)
		}
	}
}

// TestTZSpec_StandardPrefixes 普通表达式钉 CRON_TZ 前缀。
func TestTZSpec_StandardPrefixes(t *testing.T) {
	if got := TZSpec("10 0 * * *"); got != "CRON_TZ=Asia/Shanghai 10 0 * * *" {
		t.Fatalf("TZSpec 标准表达式 = %q", got)
	}
}

// TestTZSpec_ParsesUnderParser TZSpec 输出必须能在包 parser 下解析
// （前缀格式或 parser 选项一变就红；Parse 内部走 time.LoadLocation，tzdata 已嵌入）。
func TestTZSpec_ParsesUnderParser(t *testing.T) {
	for _, expr := range []string{"10 0 * * *", "*/5 * * * *", "0 3 * * 1", "@every 1m"} {
		if _, err := parser.Parse(TZSpec(expr)); err != nil {
			t.Fatalf("parser.Parse(TZSpec(%q)) err = %v", expr, err)
		}
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./pkg/job/ -run TestTZSpec -v`
Expected: FAIL（`undefined: TZSpec` / `undefined: parser`）

- [ ] **Step 3: 实现 `pkg/job/spec.go`**

```go
package job

import (
	_ "time/tzdata" // 瘦身容器无 zoneinfo：CRON_TZ 的 LoadLocation 必须恒成功

	"github.com/robfig/cron/v3"
)

// parser 5 段标准 spec（分 时 日 月 周 + @descriptor）——不 WithSeconds（项目惯例，见 starfuyu cron.go）。
var parser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

// TZSpec 给标准表达式钉 CRON_TZ=Asia/Shanghai 前缀；@ 开头的 descriptor 原样返回。
func TZSpec(expr string) string {
	if len(expr) > 0 && expr[0] == '@' {
		return expr
	}
	return "CRON_TZ=Asia/Shanghai " + expr
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./pkg/job/ -v`
Expected: PASS（3 个 TestTZSpec 全绿）

- [ ] **Step 5: 提交**

```bash
git add pkg/job/spec.go pkg/job/spec_test.go
git commit -m "feat(job): pkg/job 表达式解析——5段 parser + CRON_TZ 前缀 + tzdata 嵌入"
```

---

### Task 2: Manager（注册表 + 生命周期 + 状态 + Trigger）

**Files:**
- Create: `pkg/job/manager.go`
- Create: `pkg/job/manager_test.go`
- Modify: `pkg/errors/code.go`（30601/30602 常量 + registry 条目）

**Interfaces:**
- Consumes: Task 1 的 `parser`、`TZSpec`
- Produces（后续任务全部依赖，签名不得改动）:
  - `type Fn func(ctx context.Context) error`
  - `func NewManager(cfg *config.Config) *Manager`
  - `func (m *Manager) Register(name, defaultSpec string, fn Fn)` —— name 必须含 `.`（`模块.任务名`）；重复 name 或启动后调用 panic
  - `func (m *Manager) List() []Status`
  - `func (m *Manager) Trigger(ctx context.Context, name string) error` —— 未注册 `errs.CodeJobNotFound`、停用 `errs.CodeJobDisabled`
  - `func BindLifecycle(lc fx.Lifecycle, m *Manager)` —— `fx.Invoke` 目标
  - `var FxModule fx.Module` —— `fx.Module("job", fx.Provide(NewManager), fx.Invoke(BindLifecycle))`
  - `type Status struct { Name, Module, Spec, DisabledReason string; Enabled, Running bool; LastStart, LastEnd, NextRun time.Time; LastErr string; RunCount uint64 }`（json tag 同名小写驼峰）
  - 错误码：`errs.CodeJobNotFound Code = 30601`（"任务不存在"）、`errs.CodeJobDisabled Code = 30602`（"任务已停用"）

- [ ] **Step 1: 在 `pkg/errors/code.go` 加两个错误码**

在 `// ---- 305xx starfuyu ----` 段之后加 `// ---- 306xx job ----`：常量 `CodeJobNotFound = 30601`、`CodeJobDisabled = 30602`，并在 `registry` map 加 `"任务不存在"` / `"任务已停用"` 两条（放 `CodeStarfuyuMpUploadFailed` 条目之后）。

- [ ] **Step 2: 写失败测试（manager_test.go）**

```go
package job

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.uber.org/fx/fxtest"

	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/config"
)

// newTestManager 造一个开关开的最小 cfg + Manager。
func newTestManager(t *testing.T) *Manager {
	t.Helper()
	cfg := &config.Config{Modules: map[string]bool{"demo": true}}
	return NewManager(cfg)
}

// TestRegister_DuplicatePanics 重复 name 必须开发期就炸。
func TestRegister_DuplicatePanics(t *testing.T) {
	m := newTestManager(t)
	m.Register("demo.a", "@every 1m", func(context.Context) error { return nil })
	defer func() {
		if recover() == nil {
			t.Fatal("重复 Register 必须 panic")
		}
	}()
	m.Register("demo.a", "@every 1m", func(context.Context) error { return nil })
}

// TestRegister_NoDotPanics name 必须是 模块.任务名。
func TestRegister_NoDotPanics(t *testing.T) {
	m := newTestManager(t)
	defer func() {
		if recover() == nil {
			t.Fatal("无点 name 必须 panic")
		}
	}()
	m.Register("nodot", "@every 1m", func(context.Context) error { return nil })
}

// TestStart_ModuleOffSkipsParse（Review Focus 1）关模块 + 非法表达式必须启动成功，
// 守卫先于解析——故意用非法表达式钉住顺序（照 starfuyu cron_test 同款手法）。
func TestStart_ModuleOffSkipsParse(t *testing.T) {
	cfg := &config.Config{Modules: map[string]bool{"demo": false}}
	m := NewManager(cfg)
	m.Register("demo.a", "not-a-cron", func(context.Context) error { return nil })
	lc := fxtest.NewLifecycle(t)
	BindLifecycle(lc, m)
	lc.RequireStart() // 守卫先 return → 不碰表达式 → 不报错
	lc.RequireStop()
	st := m.List()[0]
	if st.Enabled || st.DisabledReason != "module_off" {
		t.Fatalf("List = %+v, want disabled module_off", st)
	}
}

// TestStart_InvalidExprFails 对照：开关开时放行到解析，非法表达式必须报错拒启。
func TestStart_InvalidExprFails(t *testing.T) {
	m := newTestManager(t)
	m.Register("demo.a", "not-a-cron", func(context.Context) error { return nil })
	lc := fxtest.NewLifecycle(t)
	BindLifecycle(lc, m)
	if err := lc.Start(context.Background()); err == nil {
		t.Fatal("开关开 + 非法表达式应报错")
	}
}

// TestStart_ConfigOverride cfg.Jobs 有值覆盖 defaultSpec。
func TestStart_ConfigOverride(t *testing.T) {
	cfg := &config.Config{Modules: map[string]bool{"demo": true}}
	cfg.Jobs = map[string]string{"demo.a": "5 4 * * *"}
	m := NewManager(cfg)
	m.Register("demo.a", "10 0 * * *", func(context.Context) error { return nil })
	lc := fxtest.NewLifecycle(t)
	BindLifecycle(lc, m)
	lc.RequireStart()
	lc.RequireStop()
	if st := m.List()[0]; st.Spec != "5 4 * * *" {
		t.Fatalf("生效表达式 = %q, want 配置覆盖值", st.Spec)
	}
}

// TestStart_ConfigEmptyDisables（Review Focus 2）空串 = 停用，不得进解析。
func TestStart_ConfigEmptyDisables(t *testing.T) {
	cfg := &config.Config{Modules: map[string]bool{"demo": true}}
	cfg.Jobs = map[string]string{"demo.a": ""}
	m := NewManager(cfg)
	m.Register("demo.a", "not-a-cron", func(context.Context) error { return nil }) // 若误解析会红
	lc := fxtest.NewLifecycle(t)
	BindLifecycle(lc, m)
	lc.RequireStart()
	st := m.List()[0]
	if st.Enabled || st.DisabledReason != "spec_empty" {
		t.Fatalf("List = %+v, want disabled spec_empty", st)
	}
}

// TestStart_UnknownConfigKeyWarnsOnly 配置写错名只 warn 不炸启动。
func TestStart_UnknownConfigKeyWarnsOnly(t *testing.T) {
	cfg := &config.Config{Modules: map[string]bool{"demo": true}}
	cfg.Jobs = map[string]string{"demo.typo": "10 0 * * *"}
	m := NewManager(cfg)
	m.Register("demo.a", "@every 1m", func(context.Context) error { return nil })
	lc := fxtest.NewLifecycle(t)
	BindLifecycle(lc, m)
	lc.RequireStart() // 不报错即通过
	lc.RequireStop()
}

// TestTrigger_OK 同步执行、状态落 LastStart/LastEnd/RunCount。
func TestTrigger_OK(t *testing.T) {
	m := newTestManager(t)
	called := 0
	m.Register("demo.a", "@every 1m", func(context.Context) error { called++; return nil })
	lc := fxtest.NewLifecycle(t)
	BindLifecycle(lc, m)
	lc.RequireStart()
	defer lc.RequireStop()
	if err := m.Trigger(context.Background(), "demo.a"); err != nil {
		t.Fatalf("Trigger err = %v", err)
	}
	if called != 1 {
		t.Fatalf("called = %d, want 1", called)
	}
	st := m.List()[0]
	if st.RunCount != 1 || st.LastStart.IsZero() || st.LastEnd.IsZero() {
		t.Fatalf("状态未记录: %+v", st)
	}
}

// TestTrigger_NotFound30601 未注册 name。
func TestTrigger_NotFound30601(t *testing.T) {
	m := newTestManager(t)
	lc := fxtest.NewLifecycle(t)
	BindLifecycle(lc, m)
	lc.RequireStart()
	defer lc.RequireStop()
	err := m.Trigger(context.Background(), "demo.nope")
	if !errs.Is(err, errs.CodeJobNotFound) {
		t.Fatalf("err = %v, want 30601", err)
	}
}

// TestTrigger_Disabled30602（Review Focus 3）停用任务拒绝手动触发——
// 模块关时 trigger 不能绕过开关打外部 API。
func TestTrigger_Disabled30602(t *testing.T) {
	cfg := &config.Config{Modules: map[string]bool{"demo": false}}
	m := NewManager(cfg)
	m.Register("demo.a", "@every 1m", func(context.Context) error { return nil })
	lc := fxtest.NewLifecycle(t)
	BindLifecycle(lc, m)
	lc.RequireStart()
	defer lc.RequireStop()
	if err := m.Trigger(context.Background(), "demo.a"); !errs.Is(err, errs.CodeJobDisabled) {
		t.Fatalf("err = %v, want 30602", err)
	}
}

// TestTrigger_PanicRecovered（Review Focus 4）fn panic 被 recover：
// Trigger 返回 error 不外溢，lastErr 有记录，进程与后续 trigger 不受影响。
func TestTrigger_PanicRecovered(t *testing.T) {
	m := newTestManager(t)
	m.Register("demo.a", "@every 1m", func(context.Context) error { panic("boom") })
	lc := fxtest.NewLifecycle(t)
	BindLifecycle(lc, m)
	lc.RequireStart()
	defer lc.RequireStop()
	if err := m.Trigger(context.Background(), "demo.a"); err == nil {
		t.Fatal("panic 必须转成 error 返回")
	}
	if st := m.List()[0]; !strings.Contains(st.LastErr, "boom") {
		t.Fatalf("lastErr = %q, want 含 boom", st.LastErr)
	}
}

// TestStart_CronScheduled NextRun 非零（cron 已排程）。
func TestStart_CronScheduled(t *testing.T) {
	m := newTestManager(t)
	m.Register("demo.a", "@every 1h", func(context.Context) error { return nil })
	lc := fxtest.NewLifecycle(t)
	BindLifecycle(lc, m)
	lc.RequireStart()
	defer lc.RequireStop()
	if st := m.List()[0]; st.NextRun.IsZero() || st.NextRun.Before(time.Now()) {
		t.Fatalf("NextRun = %v, want 未来时刻", st.NextRun)
	}
}
```

- [ ] **Step 3: 跑测试确认失败**

Run: `go test ./pkg/job/ -run 'TestRegister|TestStart|TestTrigger' -v`
Expected: FAIL（`undefined: Manager` 等编译错误；错误码两常量若 Step 1 已做则 code_test 过）

- [ ] **Step 4: 实现 `pkg/job/manager.go`**

要点（签名已钉，实现按此组织）：

```go
package job

type Fn func(ctx context.Context) error

type Status struct { /* 见 Interfaces，json tag 同名 */ }

type entry struct {
	name, module, defaultSpec string
	fn          Fn
	enabled     bool
	disabledReason string // module_off | spec_empty | ""
	spec        string    // 生效表达式
	entryID     cron.EntryID
	running     int32     // atomic，Trigger/cron 并发自增
	mu          sync.Mutex
	lastStart   time.Time
	lastEnd     time.Time
	lastErr     string
	runCount    uint64
}

type Manager struct {
	cfg    *config.Config
	mu     sync.Mutex
	byName map[string]*entry
	order  []string // 注册序（List 输出确定性）
	cron   *cron.Cron
	started bool
}

func NewManager(cfg *config.Config) *Manager
func (m *Manager) Register(name, defaultSpec string, fn Fn)  // 无 "." panic；started 后 panic；重复 panic
func (m *Manager) List() []Status                            // 按 order 序；NextRun 取 cron.Entry(e.entryID).Next
func (m *Manager) Trigger(ctx context.Context, name string) error
func BindLifecycle(lc fx.Lifecycle, m *Manager)
var FxModule = fx.Module("job", fx.Provide(NewManager), fx.Invoke(BindLifecycle))
```

实现细则（测试不显式钉但必须遵守的决策）：
- `module` = `name` 第一个 `.` 之前的部分；`BindLifecycle.OnStart` 对每个 entry：`cfg.Modules[module] == false` → `enabled=false, disabledReason="module_off"` 并 **continue（不解析）**；否则查 `cfg.Jobs[name]`：键存在且空串 → `spec_empty`；键存在非空 → 覆盖；键不存在 → `defaultSpec`。
- 解析任一 enabled entry 失败 → `OnStart` 返回 `fmt.Errorf("注册 job %s 表达式 %q: %w", ...)` 拒启。
- `cfg.Jobs` 中未注册的 key → `logger.Log.Warn("job config key not registered", ...)`（logger nil 时跳过日志行，不跳过启动）。
- 执行外壳 `func (e *entry) run(ctx) error`：`atomic` 置 running/清、记 lastStart/lastEnd/lastErr、`defer recover()` 把 panic 转 `fmt.Errorf("panic: %v", r)` 记入 lastErr 并返回；cron 路径由 `BindLifecycle` 注册的闭包以 `context.Background()` 调用。
- `OnStop`：`m.cron == nil` 直接 nil；否则 `select { case <-m.cron.Stop().Done(): return nil; case <-ctx.Done(): return ctx.Err() }`。
- `Trigger`：未注册 → `errs.New(errs.CodeJobNotFound, ...)`；`!enabled` → `errs.New(errs.CodeJobDisabled, ...)`；否则同步 `e.run(ctx)` 原样返回错误（非 BizError 由 `resp.Fail` 兜底 50000）。
- 时区前缀在解析时经 `TZSpec` 包一层。

- [ ] **Step 5: 跑测试确认通过**

Run: `go test ./pkg/job/ ./pkg/errors/ -v`
Expected: PASS（Task 1 + Task 2 全绿）

- [ ] **Step 6: 提交**

```bash
git add pkg/job/ pkg/errors/code.go
git commit -m "feat(job): Manager 注册表+fx 生命周期+状态跟踪+Trigger（30601/30602）"
```

---

### Task 3: 配置 `jobs:` 段

**Files:**
- Modify: `pkg/config/config.go`（`Modules` 字段之后加 `Jobs`）
- Modify: `alexgo-server/configs/config.yaml`（顶层加 `jobs:` 段，值 = 各任务代码缺省）
- Modify: `pkg/config/starfuyu_test.go`（删 `Starfuyu.Cron` 两断言——Task 5 删字段前先改这里会红？**顺序注意：本 Task 只加不删**，starfuyu_test 不动；删字段归 Task 5）
- Test: `pkg/config/jobs_test.go`

**Interfaces:**
- Consumes: 无
- Produces: `cfg.Jobs map[string]string`（`mapstructure:"jobs"`）——Task 2 的 `NewManager` 已读此字段

- [ ] **Step 1: 写失败测试**

```go
// pkg/config/jobs_test.go  —— 同包测试，package config（照 starfuyu_test.go 现状）
package config

import "testing"

func TestLoadGlobalConfig_JobsSection(t *testing.T) {
	cfg, err := LoadGlobalConfig()
	if err != nil {
		t.Fatalf("LoadGlobalConfig() error = %v", err)
	}
	// config.yaml 显式写死的四个 key（值 = 代码缺省，运维直接改这里）
	want := map[string]string{
		"system.sweep_tokens":     "@every 1h",
		"scorecard.sweep_rooms":   "@every 1m",
		"starfuyu.fetch_fortune":  "10 0 * * *",
		"starfuyu.generate_images": "20 0 * * *",
	}
	for k, v := range want {
		if got := cfg.Jobs[k]; got != v {
			t.Errorf("jobs[%s] = %q, want %q", k, got, v)
		}
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./pkg/config/ -run TestLoadGlobalConfig_JobsSection -v`
Expected: FAIL（`cfg.Jobs` undefined）

- [ ] **Step 3: 实现**

`pkg/config/config.go`：`Modules` 字段后加

```go
// Jobs：job 表达式覆盖表，key = "模块.任务名"。
// 缺键 → 用代码 Register 的 defaultSpec；显式空串 → 停用该任务；未知 key 启动只 warn。
Jobs map[string]string `mapstructure:"jobs"`
```

`alexgo-server/configs/config.yaml`：顶层（`modules:` 段之后）加

```yaml
jobs:                         # job 覆盖表：key="模块.任务名"；缺键=代码缺省，空串=停用
  system.sweep_tokens: "@every 1h"
  scorecard.sweep_rooms: "@every 1m"
  starfuyu.fetch_fortune: "10 0 * * *"
  starfuyu.generate_images: "20 0 * * *"
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./pkg/config/ -v`
Expected: PASS（新增 + 既有 `TestLoadGlobalConfig_*` 全绿）

- [ ] **Step 5: 提交**

```bash
git add pkg/config/config.go pkg/config/jobs_test.go alexgo-server/configs/config.yaml
git commit -m "feat(job): config jobs 覆盖段——缺省/空串停用/未知键语义"
```

---

### Task 4: Admin API（controller + 路由 + 权限 + seed + main 装配）

**Files:**
- Create: `modules/system/controller/admin/job.go`
- Test: `modules/system/controller/admin/job_test.go`
- Modify: `modules/system/module.go`（systemModule 加 `jobCtrl` 字段、`NewSystemModule` 加参、`RegisterRoutes` 加两行、`FxModule` 加 `fx.Provide(NewJobController)`）
- Modify: `modules/system/service/permission_routes.go`（`permissionRoutes` 加 `system:job`）
- Modify: `modules/system/service/seed.go`（加 `jobButtons` + 追加块）
- Test: `modules/system/service/seed_test.go`
- Modify: `alexgo-server/cmd/main.go`（options 加 `job.FxModule`）

**Interfaces:**
- Consumes: Task 2 的 `job.Manager`/`FxModule`/`Status`/`Trigger`
- Produces:
  - `func NewJobController(mgr *job.Manager) *JobController`
  - `(*JobController).List` / `(*JobController).Trigger`（gin handler）
  - 路由：`GET /api/admin/jobs`、`POST /api/admin/jobs/:name/trigger`
  - `permissionRoutes["system:job"] = {"/api/admin/jobs", "/api/admin/jobs/*"}`
  - seed 按钮：`system:job:read`（任务查看，sort 17）、`system:job:trigger`（任务触发，sort 18）

- [ ] **Step 1: 写失败的 controller 测试**

`modules/system/controller/admin/job_test.go`（照 `online_controller_test.go` 的"与 module.go RegisterRoutes 同形"风格）：

```go
// newJobRouter 装 /admin/jobs 两路由（与 module.go RegisterRoutes 同形）。
// 不挂 auth 中间件：用例只验 handler 信封与参数校验；鉴权由 casbin 中间件全局负责（不在本文件）。
func newJobRouter(t *testing.T, m *job.Manager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/api")
	c := NewJobController(m)
	g.GET("/admin/jobs", c.List)
	g.POST("/admin/jobs/:name/trigger", c.Trigger)
	return r
}

// TestJobList_ReturnsRegistered 列表返回已注册任务信封 {code:0,...}。
func TestJobList_ReturnsRegistered(t *testing.T) {
	m := job.NewManager(&config.Config{Modules: map[string]bool{"demo": true}})
	m.Register("demo.a", "@every 1m", func(context.Context) error { return nil })
	r := newJobRouter(t, m)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/admin/jobs", nil))
	// 断言 200 + code=0 + data 数组含 name=="demo.a"
}

// TestJobTrigger_NotFound30601 未注册 name → 信封 code=30601。
func TestJobTrigger_NotFound30601(t *testing.T) {
	m := job.NewManager(&config.Config{Modules: map[string]bool{"demo": true}})
	// 未注册分支不依赖 started，无需 BindLifecycle/Start，直接发请求。
	r := newJobRouter(t, m)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/admin/jobs/nope/trigger", nil))
	// 断言 body code=30601
}
```

（断言细节实现时按项目现有 envelope 断言 helper 写——starfuyu `module_test.go` 的 `envelope` 结构可参照复制。）

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./modules/system/controller/admin/ -run TestJob -v`
Expected: FAIL（`undefined: NewJobController`）

- [ ] **Step 3: 实现 `modules/system/controller/admin/job.go`**

```go
type JobController struct{ mgr *job.Manager }
func NewJobController(mgr *job.Manager) *JobController

// List godoc：@Router /admin/jobs [get]，@Description 权限 system:job:read；
// resp.OK(ctx, c.mgr.List())
func (c *JobController) List(ctx *gin.Context)

// Trigger godoc：@Router /admin/jobs/:name/trigger [post]，@Description 权限 system:job:trigger；同步执行返回。
// err := c.mgr.Trigger(ctx.Request.Context(), ctx.Param("name"))
// err != nil → resp.Fail(ctx, err)；nil → resp.OK(ctx, gin.H{"name": name})
func (c *JobController) Trigger(ctx *gin.Context)
```

- [ ] **Step 4: 装配 system 模块**

- `systemModule` 结构体加 `jobCtrl *JobController`；`NewSystemModule` 加参 `jobCtrl *JobController` 并透传。
- `RegisterRoutes` 里（**直接挂 `r`，不进 `/admin/system` 的 `adminGroup`**，因路径是 `/api/admin/jobs`）：

```go
r.GET("/admin/jobs", m.jobCtrl.List)
r.POST("/admin/jobs/:name/trigger", m.jobCtrl.Trigger)
```

- `system.FxModule` 的 `fx.Provide(...)` 块加 `admin.NewJobController`。
- `alexgo-server/cmd/main.go` 的 options 列表（模块装配注释处）加 `job.FxModule`（import `alexGo-cloud/pkg/job`）。

- [ ] **Step 5: 权限键 + seed 按钮**

`permission_routes.go` 的 `permissionRoutes` 加（`system:tenant` 条目之后）：

```go
"system:job": {"/api/admin/jobs", "/api/admin/jobs/*"},
```

`seed.go`：仿 `fileStorageButtons` 加

```go
// jobButtons 任务中心两按钮（pkg/job admin API）：同属 system:job 前缀，
// 整组绑一次 hasPermPrefix 幂等判（照 fileStorageButtons，逐条判会只插首条 → 403）。
var jobButtons = []struct {
	perm string
	name string
	icon string
	sort int
}{
	{perm: "system:job:read", name: "任务查看", icon: "list", sort: 17},
	{perm: "system:job:trigger", name: "任务触发", icon: "play", sort: 18},
}
```

追加块放在 `fileStorageButtons` 追加块之后，判据 `if rootID != 0 && !hasPermPrefix(allMenus, "system:job")`，循环体逐字复用 fileStorageButtons 的按钮构造（ParentID=rootID, Type="button", Status=1, append 进 allMenus）。

- [ ] **Step 6: 写 seed 测试**

`seed_test.go` 加（照 `TestSeedButtonPerms_MatchRouteKeys` 同尺度断言）：

```go
// TestSeedJobButtons_MatchRouteKeys（Review Focus 5a）按钮权限前缀必须与
// permissionRoutes 的 system:job 键同尺度命中（permPrefix 取前两段比对）。
func TestSeedJobButtons_MatchRouteKeys(t *testing.T) { /* 遍历 jobButtons：permPrefix(b.perm) == "system:job" 且 permissionRoutes["system:job"] 非空 */ }

// TestSeedJobButtons_Idempotent（Review Focus 5b）幂等判据：已有任一 system:job 前缀菜单则整组跳过。
func TestSeedJobButtons_Idempotent(t *testing.T) { /* 用 hasPermPrefix 构造已含 system:job:read 的菜单集 → 断言判据为 true（不会重复插） */ }
```

- [ ] **Step 7: 跑测试**

Run: `go test ./modules/system/... ./pkg/job/ -v`
Expected: PASS（controller 新用例 + seed 新用例 + 既有全绿；`TestMono_GraphValidates` 会验证 main 图——`job.FxModule` 缺失或 `NewSystemModule` 参数不齐会在此红）

- [ ] **Step 8: 提交**

```bash
git add modules/system/controller/admin/job.go modules/system/controller/admin/job_test.go \
  modules/system/module.go modules/system/service/permission_routes.go \
  modules/system/service/seed.go modules/system/service/seed_test.go \
  alexgo-server/cmd/main.go
git commit -m "feat(job): admin 任务中心 API——列表/触发 + system:job 权限键 + seed 按钮"
```

---

### Task 5: 迁移 starfuyu（删 cron.go）

**Files:**
- Delete: `modules/starfuyu/cron.go`
- Modify: `modules/starfuyu/module.go`（删 `fx.Invoke(cronLifecycle)`，改为注册两个 job）
- Modify: `modules/starfuyu/cron_test.go` → 重写为 `modules/starfuyu/jobs_test.go`（删 cron.go 后原测试编译不过）
- Modify: `pkg/config/config.go`（删 `Starfuyu.Cron` 结构段）
- Modify: `pkg/config/loader.go`（删 `starfuyu.cron.fetch_fortune`/`generate_images` 两条 `SetDefault`）
- Modify: `pkg/config/starfuyu_test.go`（删两条 Cron 断言）
- Modify: `alexgo-server/configs/config.yaml`（删 `starfuyu.cron:` 段——值已在顶层 `jobs:`）
- Modify: `modules/starfuyu/service/tasks.go`（删 `CronSpec` 函数——迁 `pkg/job.TZSpec`；先 `grep -rn "CronSpec" modules/` 确认无其他引用）

**Interfaces:**
- Consumes: Task 2 `Manager.Register`、`service.TaskRunner`（既有）、`skipFetch`（原 cron.go，随迁到 module.go）
- Produces: 注册名钉死 `starfuyu.fetch_fortune`（缺省 `"10 0 * * *"`）、`starfuyu.generate_images`（缺省 `"20 0 * * *"`）——与 Task 3 config.yaml key 一致

- [ ] **Step 1: 重写测试（jobs_test.go）**

删 `cron_test.go`，建 `jobs_test.go` 保留/改写三类：

```go
// TestSkipFetch_EmptyKeyUnconditional 原样保留（skipFetch 迁到 module.go 后同包可见）。

// TestRegisterJobs_TwoJobs 注册面冒烟：装配 invoke 闭包（或直接调 module 内注册函数），
// 断言 List 恰有 starfuyu.fetch_fortune / starfuyu.generate_images 两个 name、
// 缺省 spec 分别为 "10 0 * * *" / "20 0 * * *"。
func TestRegisterJobs_TwoJobs(t *testing.T) { ... }

// TestRegisterJobs_SkipFetchGuard 闭包语义：APIKey 空时 fetch 任务体返回 nil 且不调 runner
// （runner 用 stub：FetchToday 被调则 t.Fatal）。
```

（原 `TestCronSpec_*`、两个 `TestCronLifecycle_*` 已被 Task 1/2 同款测试覆盖，**不迁**。）

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./modules/starfuyu/ -v`
Expected: FAIL（`undefined: cronLifecycle`——module.go 还在引用；或新测试 `undefined`）

- [ ] **Step 3: 实现迁移**

`module.go`：
- 删 `fx.Invoke(cronLifecycle)`，加

```go
// 定时任务迁 pkg/job（原 cron.go）：表达式缺省在此，运维覆盖走 config jobs: 段。
fx.Invoke(func(m *job.Manager, runner service.TaskRunner, cfg *config.Config) {
	m.Register("starfuyu.fetch_fortune", "10 0 * * *", func(ctx context.Context) error {
		if skipFetch(cfg.Starfuyu.Deepseek.APIKey) {
			return nil
		}
		return runner.FetchToday(ctx, false)
	})
	m.Register("starfuyu.generate_images", "20 0 * * *", func(ctx context.Context) error {
		return runner.GenerateImagesToday(ctx)
	})
})
```

- `skipFetch` 函数体从 cron.go 原样搬到 module.go（含注释），import 清理（cron.go 删除后无 robfig 引用；module.go 需要 `context`、`job`）。
- 删 `cron.go` 文件。

`tasks.go`：删 `CronSpec`（先确认无引用）。

config 三处：删 `Starfuyu.Cron` struct、loader 两条 `SetDefault`、starfuyu_test 两断言、config.yaml `starfuyu.cron:` 段。

- [ ] **Step 4: 跑测试**

Run: `go test ./modules/starfuyu/... ./pkg/config/ ./pkg/job/ -v`
Expected: PASS（含 starfuyu 既有 `module_test` 路由回归）

- [ ] **Step 5: 提交**

```bash
git add -A modules/starfuyu pkg/config alexgo-server/configs/config.yaml
git commit -m "refactor(starfuyu): cron 迁 pkg/job——删 cron.go，配置收敛 jobs 段"
```

---

### Task 6: 迁移 system / scorecard ticker

**Files:**
- Modify: `modules/system/module.go`（token 清扫 fx.Invoke 块 → job 注册）
- Modify: `modules/scorecard/module.go`（超时关房 fx.Invoke 块 → job 注册）
- Test: 各模块补一个注册冒烟（可并入既有测试文件）

**Interfaces:**
- Consumes: Task 2 `Register`；既有 `(*token.Service).SweepExpired(ctx) error`、`(RoomService).SweepExpired(ctx) (??, error)`
- Produces: 注册名 `system.sweep_tokens`（缺省 `@every 1h`）、`scorecard.sweep_rooms`（缺省 `@every 1m`）——与 Task 3 config.yaml 一致

**行为变化（设计 §4.6 已裁定，提交信息注明）**：两个旧 ticker 不查模块开关，迁后受 `cfg.Modules` 守卫（`modules.scorecard: false` 会连带停清扫）。

- [ ] **Step 1: 写失败测试**

两模块测试文件各加：

```go
// TestRegisterSweepJob 注册冒烟：调模块内注册函数 → List 含
// "system.sweep_tokens"（或 "scorecard.sweep_rooms"），缺省 spec 正确。
func TestRegisterSweepJob(t *testing.T) { ... }
```

（scorecard `SweepExpired` 返回 `(_, error)`，注册闭包内 `_, err := svc.SweepExpired(ctx); return err`。）

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./modules/system/ ./modules/scorecard/ -run TestRegisterSweepJob -v`
Expected: FAIL（undefined）

- [ ] **Step 3: 实现**

`modules/system/module.go`：整个手写 ticker `fx.Invoke` 块（runCtx/NewTicker/select 循环）替换为

```go
// token 清扫迁 pkg/job（原手写 ticker，见 git 历史）：@every 1h，开关守卫由 Manager 统一负责。
fx.Invoke(func(m *job.Manager, svc *token.Service) {
	m.Register("system.sweep_tokens", "@every 1h", func(ctx context.Context) error {
		return svc.SweepExpired(ctx)
	})
})
```

`modules/scorecard/module.go`：同款替换（注意保留该文件顶部关于"分享卡参数补建"等**其他** fx.Invoke 块不动；只动超时关房那一块）。

import 清理：两文件若 `time`/`logger`/`zap` 因删 ticker 不再使用则移除；加 `alexGo-cloud/pkg/job`。

- [ ] **Step 4: 跑测试**

Run: `go test ./modules/system/... ./modules/scorecard/... -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add modules/system/module.go modules/scorecard/module.go
git commit -m "refactor: system/scorecard 手写 ticker 迁 pkg/job（行为变化：受模块开关守卫）"
```

---

### Task 7: 全量回归 + 文档

**Files:**
- Modify: `docs/modules.md`（§13.7 starfuyu 定时任务段改指 pkg/job；system §后台任务、scorecard §11.6 同步；新增一段 pkg/job 简述）

**Interfaces:**
- Consumes: Task 1–6 全部

- [ ] **Step 1: 全量测试**

Run: `go build ./... && go test ./...`
Expected: 全绿（0 FAIL）

- [ ] **Step 2: lint**

Run: `make lint`（或 `golangci-lint run ./...`，按仓库现有配置）
Expected: 无新增告警

- [ ] **Step 3: 更新 docs/modules.md**

三处同步：system"后台任务"行、scorecard §11.6、starfuyu §13.7——措辞改为"由 `pkg/job` Manager 调度（表达式覆盖见 config `jobs:` 段）"，并补一行 admin `GET/POST /api/admin/jobs` 与权限键 `system:job`。原 §13.7 的"补跑走 admin POST /tasks/*"句保留（starfuyu 带参端点未变）。

- [ ] **Step 4: 提交**

```bash
git add docs/modules.md
git commit -m "docs(job): modules.md 同步 pkg/job 调度与任务中心端点"
```

---

## Self-Review 记录

- **Spec 覆盖**：§4.1 包结构→Task 1/2；§4.2 API→Task 2；§4.3 配置→Task 3；§4.4 admin+权限+停用语义+同步 trigger→Task 2/4；§4.6 迁移三任务面→Task 5/6；§5 测试计划→各 Task 步骤；§6 切分→Task 1–7（原 3.5 并入 Task 4）。
- **类型一致性**：`Register(name, defaultSpec string, fn Fn)`、`Trigger(ctx, name) error`、`Status` 字段、注册名与 config key 四处交叉处已对齐（starfuyu 两名、system/scorecard 两名在 Task 3/5/6 重复钉死）。
- **Review Focus 5 条**各自有归属测试（见各条标注）。
- **比例**：代码块仅签名、关键闭包与测试断言；Manager 实现体以细则列出不写全文（除结构骨架）。
