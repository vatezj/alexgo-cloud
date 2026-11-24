package service

import (
	"regexp"
	"strings"
)

// currencySymbols 成对货币符优先级（spec §3.2 钉死顺序，先命中者胜，与正文出现次序无关）。
var currencySymbols = []string{"￥", "₳", "₴", "€", "¢", "$", "₤"}

// currencyRes 按 currencySymbols 顺序预编译的成对匹配正则：
// <sym>([A-Za-z0-9]{8,24})<sym>（QuoteMeta 转义符号）。包级 plain var init。
var currencyRes = func() []*regexp.Regexp {
	res := make([]*regexp.Regexp, len(currencySymbols))
	for i, sym := range currencySymbols {
		res[i] = regexp.MustCompile(regexp.QuoteMeta(sym) + `([A-Za-z0-9]{8,24})` + regexp.QuoteMeta(sym))
	}
	return res
}()

// tkRe tk= 参数兜底（spec §3.4）：值在 & 或空白处截断。
var tkRe = regexp.MustCompile(`tk=([^&\s]+)`)

// titleRe 【...】 标题对；内层不含括号字符，允许空对（由调用方跳过）。
var titleRe = regexp.MustCompile(`【([^【】]*)】`)

// Extract 从用户粘贴文本提取口令体。优先级：货币符成对匹配（符号顺序见 currencySymbols）
// → tk= 参数兜底。全部未命中 → ("", false)。
func Extract(text string) (tkl string, ok bool) {
	for _, re := range currencyRes {
		if m := re.FindStringSubmatch(text); m != nil {
			return m[1], true
		}
	}
	if m := tkRe.FindStringSubmatch(text); m != nil {
		return m[1], true
	}
	return "", false
}

// TitleOf 抓首个非空 【...】 对的内文（TrimSpace 后），无则 ("", false)。spec §5.3。
func TitleOf(text string) (title string, ok bool) {
	for _, m := range titleRe.FindAllStringSubmatch(text, -1) {
		inner := strings.TrimSpace(m[1])
		if inner != "" {
			return inner, true
		}
	}
	return "", false
}
