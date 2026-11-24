package service

import "testing"

func TestExtract_RealSampleYuan(t *testing.T) {
	text := "【半亩花田旗舰店身体磨砂膏…】https://m.tb.cn/h.eXLDenB?sm=537a05 点击链接，再选择浏览器咑閞；或復·制这段描述￥6fj0Y1HoO3U￥后到淘♂寳"
	got, ok := Extract(text)
	if !ok || got != "6fj0Y1HoO3U" {
		t.Fatalf("Extract() = %q, %v; want %q, true", got, ok, "6fj0Y1HoO3U")
	}
}

func TestExtract_RealSampleCent(t *testing.T) {
	text := "【买2送1修正丁香叶茶…】，椱ァ製这段描述¢ewzsYK4rPaj¢后到◇綯℡寳"
	got, ok := Extract(text)
	if !ok || got != "ewzsYK4rPaj" {
		t.Fatalf("Extract() = %q, %v; want %q, true", got, ok, "ewzsYK4rPaj")
	}
}

func TestExtract_PriorityYuanOverDollar(t *testing.T) {
	// $ 对在文本中先出现，但 ￥ 符号优先级更高 → 返回 ￥ 对内口令。
	text := "复制 this $abc12345$ code 或 ￥6fj0Y1HoO3U￥ 后打开淘宝"
	got, ok := Extract(text)
	if !ok || got != "6fj0Y1HoO3U" {
		t.Fatalf("Extract() = %q, %v; want %q, true", got, ok, "6fj0Y1HoO3U")
	}
}

func TestExtract_DollarPairOnly(t *testing.T) {
	text := "复制 this $deal2024x$ code"
	got, ok := Extract(text)
	if !ok || got != "deal2024x" {
		t.Fatalf("Extract() = %q, %v; want %q, true", got, ok, "deal2024x")
	}
}

func TestExtract_SingleDollarNoMatch(t *testing.T) {
	text := "价格$5 和 €9优惠"
	if got, ok := Extract(text); ok || got != "" {
		t.Fatalf("Extract() = %q, %v; want \"\", false", got, ok)
	}
}

func TestExtract_LengthBounds(t *testing.T) {
	// 7 位与 25 位配对 → 超出 {8,24} → false。
	for _, body := range []string{"abc1234", "abc1234567890123456789012"} { // 7, 25
		text := "￥" + body + "￥"
		if got, ok := Extract(text); ok || got != "" {
			t.Fatalf("Extract(%q) = %q, %v; want \"\", false", text, got, ok)
		}
	}
	// 8 位与 24 位配对 → 边界内 → true。
	eight := "abc12345"                      // 8
	twentyFour := "abc123456789012345678901" // 24
	for _, body := range []string{eight, twentyFour} {
		text := "￥" + body + "￥"
		got, ok := Extract(text)
		if !ok || got != body {
			t.Fatalf("Extract(%q) = %q, %v; want %q, true", text, got, ok, body)
		}
	}
}

func TestExtract_TkParam(t *testing.T) {
	text := "https://x.y/h?tk=Abc12345&spm=a 复制打开淘宝"
	got, ok := Extract(text)
	if !ok || got != "Abc12345" {
		t.Fatalf("Extract() = %q, %v; want %q, true", got, ok, "Abc12345")
	}

	// tk= 值后跟空格 → 在空格处截断。
	text2 := "https://x.y/h?tk=Abc12345 复制打开淘宝"
	got2, ok2 := Extract(text2)
	if !ok2 || got2 != "Abc12345" {
		t.Fatalf("Extract() = %q, %v; want %q, true", got2, ok2, "Abc12345")
	}
}

func TestExtract_CurrencyWinsOverTk(t *testing.T) {
	text := "￥pairpair1￥ 和 https://x.y/h?tk=Abc12345"
	got, ok := Extract(text)
	if !ok || got != "pairpair1" {
		t.Fatalf("Extract() = %q, %v; want %q, true", got, ok, "pairpair1")
	}
}

func TestExtract_NoPassword(t *testing.T) {
	if got, ok := Extract("首页 优惠券"); ok || got != "" {
		t.Fatalf("Extract() = %q, %v; want \"\", false", got, ok)
	}
}

func TestTitleOf_Pair(t *testing.T) {
	text := "【半亩花田…磨砂膏】描述"
	got, ok := TitleOf(text)
	if !ok || got != "半亩花田…磨砂膏" {
		t.Fatalf("TitleOf() = %q, %v; want %q, true", got, ok, "半亩花田…磨砂膏")
	}
}

func TestTitleOf_NoPair(t *testing.T) {
	if got, ok := TitleOf("没有标题符号"); ok || got != "" {
		t.Fatalf("TitleOf() = %q, %v; want \"\", false", got, ok)
	}
}

func TestTitleOf_MultipleTakesFirst(t *testing.T) {
	text := "【第一个标题】中间【第二个标题】"
	got, ok := TitleOf(text)
	if !ok || got != "第一个标题" {
		t.Fatalf("TitleOf() = %q, %v; want %q, true", got, ok, "第一个标题")
	}
}
