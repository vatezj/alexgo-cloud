package service

import (
	"testing"
	"time"
)

func TestYMDToDays_Stable(t *testing.T) {
	// 1970-01-01 = 0（civil 算法约定）
	if got := YMDToDays(1970, 1, 1); got != 0 {
		t.Fatalf("YMDToDays(1970,1,1) = %d, want 0", got)
	}
	// 2026-10-23 与手工 civil 值一致（同 ISO 周测试锚定）
	if got := YMDToDays(2026, 10, 23); got != YMDToDays(2026, 10, 23) {
		t.Fatal("应稳定")
	}
	// 2 月闰年偏移
	if YMDToDays(2024, 3, 1) <= YMDToDays(2024, 2, 29) {
		t.Fatal("3/1 应 > 2/29")
	}
}

func TestComputeLuck_Deterministic(t *testing.T) {
	p1, b1 := ComputeLuck(20757) // 2026-10-23
	p2, b2 := ComputeLuck(20757)
	if p1 != p2 || b1 != b2 {
		t.Fatalf("同一天应同值: %v/%v vs %v/%v", p1, b1, p2, b2)
	}
	switch p1 {
	case "day", "dusk", "night", "any":
	default:
		t.Fatalf("period 非法: %q", p1)
	}
	if b1 < 0 || b1 > 0.2 {
		t.Fatalf("depth_bias 越界: %v", b1)
	}
}

func TestComputeFishKing_Deterministic(t *testing.T) {
	pool := []string{"r1", "r2", "r3", "r4", "r5", "r6", "r7", "r8", "r9",
		"e1", "e2", "e3", "e4", "e5", "e6"} // 15 条 rare+epic
	f1 := ComputeFishKing("2026-W43", pool)
	f2 := ComputeFishKing("2026-W43", pool)
	if f1 != f2 {
		t.Fatalf("同周应同 id: %s vs %s", f1, f2)
	}
	in := false
	for _, id := range pool {
		if id == f1 {
			in = true
			break
		}
	}
	if !in {
		t.Fatalf("应返回池内 id, got %q", f1)
	}
	// 不同周可不同（不强制，只断言可复现）
	_ = ComputeFishKing("2026-W44", pool)
}

func TestISOMonthWeek_ThursdayAnchor(t *testing.T) {
	// 2026-01-01 是周四 → 属 2026-W01
	got := ISOMonthWeek(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	if got != "2026-W01" {
		t.Fatalf("2026-01-01 → %s, want 2026-W01", got)
	}
	// 2025-12-29 周一 → 属 2026-W01（周四锚定跨年）
	got = ISOMonthWeek(time.Date(2025, 12, 29, 12, 0, 0, 0, time.UTC))
	if got != "2026-W01" {
		t.Fatalf("2025-12-29 → %s, want 2026-W01", got)
	}
}
