package service

import (
	"slices"
	"strings"
	"testing"
)

// TestClampScore 截到 [0,100]：-5→0、0→0、66→66、100→100、300→100。
func TestClampScore(t *testing.T) {
	cases := []struct {
		in, want int
	}{
		{-5, 0},
		{0, 0},
		{66, 66},
		{100, 100},
		{300, 100},
	}
	for _, c := range cases {
		if got := clampScore(c.in); got != c.want {
			t.Errorf("clampScore(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

// TestWrapByWidth 按测宽贪心断行；\n 强制断行；单字超宽独占一行不丢字。
func TestWrapByWidth(t *testing.T) {
	// measure 假函数：按 rune 数 ×10（"一二三"→30 ≤35 <40→四字超宽）
	measure := func(s string) float64 {
		return float64(len([]rune(s))) * 10
	}
	cases := []struct {
		name string
		text string
		maxW float64
		want []string
	}{
		{"贪心断行", "一二三四五", 35, []string{"一二三", "四五"}}, // 3×10=30 ≤35 <4×10
		{"换行强制断行", "ab\ncd", 35, []string{"ab", "cd"}},
		{"单字超宽不丢字", "中", 5, []string{"中"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := wrapByWidth(c.text, c.maxW, measure)
			if !slices.Equal(got, c.want) {
				t.Errorf("wrapByWidth(%q, %v) = %q, want %q", c.text, c.maxW, got, c.want)
			}
		})
	}
}

// TestTruncateLines 超 maxLines 截最后一条并追加 …；maxLines<=0 或未超原样。
func TestTruncateLines(t *testing.T) {
	// 4 行 maxLines 3 → 3 行，末行以 … 结尾
	four := []string{"甲", "乙", "丙", "丁"}
	got := truncateLines(four, 3)
	if len(got) != 3 {
		t.Fatalf("truncateLines(4行, 3) 行数 = %d, want 3", len(got))
	}
	if !slices.Equal(got[:2], []string{"甲", "乙"}) {
		t.Errorf("前两行 = %q, want [甲 乙]", got[:2])
	}
	if !strings.HasSuffix(got[2], "…") {
		t.Errorf("末行 = %q, want 以 … 结尾", got[2])
	}
	if !strings.HasPrefix(got[2], "丙") {
		t.Errorf("末行 = %q, want 基于被截行「丙」", got[2])
	}

	// 2 行 maxLines 3 → 原样
	two := []string{"甲", "乙"}
	if got := truncateLines(two, 3); !slices.Equal(got, two) {
		t.Errorf("truncateLines(2行, 3) = %q, want 原样 %q", got, two)
	}

	// maxLines<=0 → 原样（接口合同）
	for _, maxLines := range []int{0, -1} {
		if got := truncateLines(four, maxLines); !slices.Equal(got, four) {
			t.Errorf("truncateLines(4行, %d) = %q, want 原样 %q", maxLines, got, four)
		}
	}
}

// TestProgressFillWidth barW × clampScore/100：(300,0)→0、(300,50)→150、
// (300,300)→300（越界截 100）、(300,-5)→0（负分截 0）。
func TestProgressFillWidth(t *testing.T) {
	cases := []struct {
		barW  float64
		score int
		want  float64
	}{
		{300, 0, 0},
		{300, 50, 150},
		{300, 300, 300},
		{300, -5, 0},
	}
	for _, c := range cases {
		got := progressFillWidth(c.barW, c.score)
		if got != c.want {
			t.Errorf("progressFillWidth(%v, %d) = %v, want %v", c.barW, c.score, got, c.want)
		}
		if got < 0 {
			t.Errorf("progressFillWidth(%v, %d) = %v, want 非负", c.barW, c.score, got)
		}
	}
}
