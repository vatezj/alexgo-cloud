package starfuyu

import (
	"context"
	"testing"

	"go.uber.org/fx/fxtest"

	"alexGo-cloud/modules/starfuyu/service"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/logger"
)

// TestCronSpec_ParsesUnderModuleParser 钉 CronSpec 输出能在模块 5 段 parser 下
// 解析（CRON_TZ 前缀 × parser 交互：前缀格式或 parser 选项一变就红）。
// Parse 内部走 time.LoadLocation——本包已 import time/tzdata，无 tzdata 容器也须过。
func TestCronSpec_ParsesUnderModuleParser(t *testing.T) {
	for _, expr := range []string{"10 0 * * *", "*/5 * * * *", "0 3 * * 1"} {
		spec := service.CronSpec(expr)
		if _, err := cronParser.Parse(spec); err != nil {
			t.Fatalf("cronParser.Parse(%q) err = %v", spec, err)
		}
	}
}

// TestSkipFetch_EmptyKeyUnconditional 空 APIKey 跳过必须无条件生效：
// 旧写法 `key=="" && logger.Log != nil` 在 logger 为 nil 时不跳过，注定失败的
// 空 key 请求照跑。测试显式把 logger.Log 置 nil 钉住该场景（仅日志行允许门控）。
func TestSkipFetch_EmptyKeyUnconditional(t *testing.T) {
	saved := logger.Log
	logger.Log = nil
	t.Cleanup(func() { logger.Log = saved })

	if !skipFetch("") {
		t.Fatal("空 APIKey + nil logger 必须跳过（return 无条件）")
	}
	if skipFetch("sk-test") {
		t.Fatal("非空 APIKey 不得跳过")
	}
}

// nopRunner TaskRunner 空实现：开关关闭时任务不得被调度，方法永不应被调。
type nopRunner struct{}

func (nopRunner) FetchToday(context.Context, bool) error             { return nil }
func (nopRunner) GenerateImagesToday(context.Context) error          { return nil }
func (nopRunner) ExecuteAll(context.Context, bool) error             { return nil }
func (nopRunner) FetchDate(context.Context, string, bool) error      { return nil }
func (nopRunner) GenerateImagesDate(context.Context, string) error   { return nil }
func (nopRunner) ExecuteAllDate(context.Context, string, bool) error { return nil }

// TestCronLifecycle_ModuleSwitchOff 终审 Important：modules.starfuyu: false 时
// OnStart 必须在 AddFunc 之前 return——故意用非法 cron 表达式钉住该顺序：
// 若守卫缺失，AddFunc 解析失败 → RequireStart 红；守卫在位 → 提前返回、
// 不解析任何表达式、不启动 cron，RequireStart/RequireStop 全绿。
func TestCronLifecycle_ModuleSwitchOff(t *testing.T) {
	cfg := &config.Config{Modules: map[string]bool{"starfuyu": false}}
	cfg.Starfuyu.Cron.FetchFortune = "not-a-cron"        // 非法：守卫必须先于解析
	cfg.Starfuyu.Cron.GenerateImages = "also-not-a-cron" // 非法：同上

	lc := fxtest.NewLifecycle(t)
	cronLifecycle(lc, cfg, nopRunner{})
	lc.RequireStart() // 开关关 → 提前 return nil，不碰表达式
	lc.RequireStop()  // c 仍为 nil → OnStop 直接 nil
}

// TestCronLifecycle_SwitchOnParsesCron 对照：开关开时放行到 AddFunc，
// 非法表达式必须报错——证明上例的绿不是“永远不解析”。
func TestCronLifecycle_SwitchOnParsesCron(t *testing.T) {
	cfg := &config.Config{Modules: map[string]bool{"starfuyu": true}}
	cfg.Starfuyu.Cron.FetchFortune = "not-a-cron"
	cfg.Starfuyu.Cron.GenerateImages = "10 0 * * *"

	lc := fxtest.NewLifecycle(t)
	cronLifecycle(lc, cfg, nopRunner{})
	// Start 返回 error 而非 RequireStart：守卫放行后 AddFunc 解析必失败。
	if err := lc.Start(context.Background()); err == nil {
		t.Fatal("开关开 + 非法表达式应报错，nil 说明未走到 AddFunc")
	}
}
