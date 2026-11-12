package starfuyu

import (
	"testing"

	"alexGo-cloud/modules/starfuyu/service"
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
