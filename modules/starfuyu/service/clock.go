package service

import (
	"sync"
	"time"
)

// shanghaiOnce 进程内缓存 Asia/Shanghai 位置（LoadLocation 每次都读 tzdata 文件，钉死一次）。
var (
	shanghaiOnce sync.Once
	shanghaiLoc  *time.Location
)

// shLocation 返回钉死的 Asia/Shanghai；LoadLocation 失败（容器无 tzdata）时
// 回退固定 +08:00（上海自 1991 年起无夏令时，偏移恒定）。
func shLocation() *time.Location {
	shanghaiOnce.Do(func() {
		loc, err := time.LoadLocation("Asia/Shanghai")
		if err != nil || loc == nil {
			loc = time.FixedZone("CST", 8*3600)
		}
		shanghaiLoc = loc
	})
	return shanghaiLoc
}

// NowSH 当前时间钉死 Asia/Shanghai（生产容器是 UTC，不得跟随 time.Local / TZ）。
func NowSH() time.Time {
	return time.Now().In(shLocation())
}

// TodaySH 当前上海日历日，格式 YYYY-MM-DD。
func TodaySH() string {
	return NowSH().Format("2006-01-02")
}
