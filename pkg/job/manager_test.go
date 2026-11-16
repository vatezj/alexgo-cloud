package job

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.uber.org/fx/fxtest"

	"alexGo-cloud/pkg/config"
	errs "alexGo-cloud/pkg/errors"
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
	cfg.Jobs = map[string]map[string]string{"demo": {"a": "5 4 * * *"}}
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
	cfg.Jobs = map[string]map[string]string{"demo": {"a": ""}}
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
	cfg.Jobs = map[string]map[string]string{"demo": {"typo": "10 0 * * *"}}
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

// TestTrigger_ConcurrentOverlap_RunningStaysTrue（终审 Important 1）spec §4.4：
// running 布尔反映"至少一个在跑"——两 run 重叠时先结束者不得把标志清零。
func TestTrigger_ConcurrentOverlap_RunningStaysTrue(t *testing.T) {
	m := newTestManager(t)
	started := make(chan struct{})
	release := make(chan struct{})
	m.Register("demo.slow", "@every 1h", func(context.Context) error {
		select {
		case <-started: // 第二次进入：直接放行（首个已在跑）
		default:
			close(started)
			<-release // 阻塞到测试主动释放
		}
		return nil
	})
	lc := fxtest.NewLifecycle(t)
	BindLifecycle(lc, m)
	lc.RequireStart()
	defer lc.RequireStop()

	firstDone := make(chan error, 1)
	go func() { firstDone <- m.Trigger(context.Background(), "demo.slow") }()
	<-started // 首个 run 已在跑

	// 第二个 run 完整跑完（并发允许）。
	if err := m.Trigger(context.Background(), "demo.slow"); err != nil {
		t.Fatalf("第二实例 err = %v", err)
	}
	// 首个仍在阻塞 → running 必须仍为 true（旧实现：第二实例结束时清零 → false）。
	if st := m.List()[0]; !st.Running {
		t.Fatalf("首实例未结束时 Running = false，want true（计数语义）")
	}
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("首实例 err = %v", err)
	}
	if st := m.List()[0]; st.Running {
		t.Fatalf("全部结束后 Running = true, want false")
	}
}
