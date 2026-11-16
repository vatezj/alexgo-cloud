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
