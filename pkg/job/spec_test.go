package job

import "testing"

// TestTZSpec_DescriptorUnchanged @every/@hourly 原样返回（加 TZ 前缀反而可能不被 parser 接受）。
func TestTZSpec_DescriptorUnchanged(t *testing.T) {
	for _, expr := range []string{"@every 1m", "@hourly"} {
		if got := TZSpec(expr); got != expr {
			t.Fatalf("TZSpec(%q) = %q, want 原样", expr, got)
		}
	}
}

// TestTZSpec_StandardPrefixes 普通表达式钉 CRON_TZ 前缀。
func TestTZSpec_StandardPrefixes(t *testing.T) {
	if got := TZSpec("10 0 * * *"); got != "CRON_TZ=Asia/Shanghai 10 0 * * *" {
		t.Fatalf("TZSpec 标准表达式 = %q", got)
	}
}

// TestTZSpec_ParsesUnderParser TZSpec 输出必须能在包 parser 下解析
// （前缀格式或 parser 选项一变就红；Parse 内部走 time.LoadLocation，tzdata 已嵌入）。
func TestTZSpec_ParsesUnderParser(t *testing.T) {
	for _, expr := range []string{"10 0 * * *", "*/5 * * * *", "0 3 * * 1", "@every 1m"} {
		if _, err := parser.Parse(TZSpec(expr)); err != nil {
			t.Fatalf("parser.Parse(TZSpec(%q)) err = %v", expr, err)
		}
	}
}
