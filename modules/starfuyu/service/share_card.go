package service

import "strings"

// 分享卡渲染支撑：纯函数库（clamp/wrap/truncate/进度条）+ 布局常量表。

// slot 分享卡槽位 rect（左上角+宽高），画布 1728×2397。
type slot struct{ x, y, w, h float64 }

// 逐槽字号与颜色（初值：上卡值 44、运程条 40、小卡 34、必做 40；
// 文本色 #5B4A4A 深棕与底图印字同系）。视觉迭代时在此微调。
const (
	shareFontUpper = 44.0 // 上卡五槽值
	shareFontLane  = 40.0 // 四条运程条文字
	shareFontCard  = 34.0 // 三小卡文案
	shareFontTodo  = 40.0 // 今日必做三行

	shareTextColor  = "#3F2E2A" // 值文本深棕（视觉迭代：#5B4A4A 偏灰，压深与印字同系）
	shareTrackColor = "#F7EAD0" // 进度条铺底色（≈底图浅色段）
	shareBarReunion = "#F08C8C" // 复合运填充（粉）
	shareBarSingle  = "#F6A93B" // 脱单运填充（橙）
	shareBarJob     = "#F08C8C" // 求职运填充（粉）
)

// shareCardLayout 分享卡 17 槽布局常量表：上卡五槽（标签已印底图，值画冒号之后）、
// 四运程条文字区（胶囊内侧、标签药丸右侧）、三小卡文案区（卡题下方）、
// 三进度条槽（铺底+填充段）、今日必做三行（勾选框右侧）。
// 坐标对照 assets/bgimage/bg.png 实测量取（2026-11-13），微调只动本表。
var shareCardLayout = struct {
	Summary, Score, Match, Advice, Avoid slot             // 上卡五槽
	LaneWealth, LaneCareer, LaneLove, LaneHealth slot     // 四运程条文字区
	Reunion, Single, Job slot                             // 三小卡文案区
	BarReunion, BarSingle, BarJob struct{ x, y, w, h float64 } // 进度条槽（画填充段）
	Todos [3]slot                                         // 今日必做三行文字区
}{
	// 上卡：值 x=1100（标签冒号 1068 后留 32），行基线与印字标签对齐。
	// Summary 右侧无图标放宽到 1640；Avoid 行右有爱心图标收到 1500。
	Summary: slot{1100, 469, 540, 102}, // 今日简述（≤2 行）
	Score:   slot{1100, 584, 400, 46},  // 今日分数
	Match:   slot{1100, 685, 400, 46},  // 合拍星座
	Advice:  slot{1100, 788, 540, 46},  // 建议
	Avoid:   slot{1100, 884, 400, 46},  // 避免

	// 运程条：胶囊 y 1073..1235 / 1279..1436。文字区在卡题药丸（y 1070..1158 / 1276..1362）
	// 下方的内容行，左起于药丸下闪光项目符号（x~215/1015）右侧，槽内垂直居中。
	LaneWealth: slot{230, 1163, 590, 72},  // 财富
	LaneCareer: slot{1035, 1163, 565, 72}, // 事业
	LaneLove:   slot{230, 1372, 590, 68},  // 感情
	LaneHealth: slot{1035, 1372, 565, 68}, // 健康

	// 三小卡：卡题（y 1525..1580）下方文字区，底部进度条（y 1763）之上。
	Reunion: slot{176, 1596, 380, 124},  // 复合运
	Single:  slot{686, 1596, 380, 124},  // 脱单运
	Job:     slot{1202, 1596, 382, 124}, // 求职运

	// 进度条：实测槽 x 291..411 / 802..922 / 1319..1443，心形端点在槽外不覆盖。
	BarReunion: struct{ x, y, w, h float64 }{291, 1763, 120, 14},
	BarSingle:  struct{ x, y, w, h float64 }{802, 1763, 120, 14},
	BarJob:     struct{ x, y, w, h float64 }{1319, 1764, 124, 13},

	// 今日必做：勾选框（x 220..257）右侧，行心与框心对齐（1999/2076/2152）。
	Todos: [3]slot{
		{290, 1974, 1270, 52},
		{290, 2051, 1270, 52},
		{290, 2127, 1270, 52},
	},
}

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
