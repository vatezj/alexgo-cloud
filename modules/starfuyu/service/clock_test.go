package service

import (
	"regexp"
	"testing"
	"time"
)

// TestTodaySH_PinnedToShanghai brief Step 1 必写：进程 TZ=UTC 时 TodaySH 不得跟随，
// 必须与 Asia/Shanghai 的日历日一致，且格式为 YYYY-MM-DD。
func TestTodaySH_PinnedToShanghai(t *testing.T) {
	t.Setenv("TZ", "UTC") // 进程 UTC；NowSH 不得跟随

	sh, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("LoadLocation(Asia/Shanghai) err = %v", err)
	}

	// 固定断言：TodaySH() 与 time.Now().In(sh).Format 一致
	// （夹住 TodaySH 调用前后，避免跨日边界抖动误报）。
	before := time.Now().In(sh).Format("2006-01-02")
	got := TodaySH()
	after := time.Now().In(sh).Format("2006-01-02")
	if got != before && got != after {
		t.Fatalf("TodaySH() = %q, want 与 Shanghai 日历日一致（before=%q after=%q）", got, before, after)
	}

	// 格式匹配 ^\d{4}-\d{2}-\d{2}$
	if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`).MatchString(got) {
		t.Fatalf("TodaySH() = %q, 不匹配 ^\\d{4}-\\d{2}-\\d{2}$", got)
	}

	// NowSH 位置钉死 Asia/Shanghai（非 time.Local / UTC）。
	if loc := NowSH().Location().String(); loc != "Asia/Shanghai" {
		t.Fatalf("NowSH().Location() = %q, want Asia/Shanghai", loc)
	}
}
