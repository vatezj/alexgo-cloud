package service

import "strings"

// 分享卡渲染支撑：纯函数库（clamp/wrap/truncate/进度条）。
// 布局常量表 shareCardLayout 在 Task 5 加入本文件。

// clampScore 把分数截到 [0,100]。
func clampScore(s int) int {
	switch {
	case s < 0:
		return 0
	case s > 100:
		return 100
	}
	return s
}

// wrapByWidth 按测宽贪心断行：measure 收整串返回像素宽。
// 遇 \n 强制断行（空段保留空行）；单字超宽也独占一行，不丢字。
func wrapByWidth(text string, maxW float64, measure func(string) float64) []string {
	if text == "" {
		return nil
	}
	var lines []string
	for _, seg := range strings.Split(text, "\n") {
		lines = append(lines, wrapSegment(seg, maxW, measure)...)
	}
	return lines
}

// wrapSegment 单段（无 \n）贪心断行：逐 rune 累加，超 maxW 先断行再起新行。
func wrapSegment(seg string, maxW float64, measure func(string) float64) []string {
	if seg == "" {
		return []string{""} // 显式空行原样保留
	}
	var lines []string
	var current []rune
	for _, r := range seg {
		candidate := string(current) + string(r)
		if len(current) > 0 && measure(candidate) > maxW {
			lines = append(lines, string(current))
			current = []rune{r} // 单字超宽：r 独占新行，不丢字
			continue
		}
		current = append(current, r)
	}
	if len(current) > 0 {
		lines = append(lines, string(current))
	}
	return lines
}

// truncateLines 超 maxLines 截断：保前 maxLines 条，最后一条追加 …。
// maxLines<=0 或行数未超 → 原样返回（不改入参）。
func truncateLines(lines []string, maxLines int) []string {
	if maxLines <= 0 || len(lines) <= maxLines {
		return lines
	}
	out := make([]string, maxLines)
	copy(out, lines[:maxLines])
	out[maxLines-1] += "…"
	return out
}

// progressFillWidth 进度条填充宽 = barW × clampScore(score)/100，结果非负。
func progressFillWidth(barW float64, score int) float64 {
	w := barW * float64(clampScore(score)) / 100
	if w < 0 {
		return 0
	}
	return w
}
