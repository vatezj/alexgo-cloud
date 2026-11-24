package service

import (
	"strings"
	"testing"
)

func TestRender_CouponBranch(t *testing.T) {
	prices := RenderPrices{Available: true, Zk: 100, Reserve: 129}
	cfg := RenderConfig{RebateRate: 0.05, TutorialURL: "http://suo.im/4CYwC4"}
	got := Render("测试商品标题", prices, 30, "desc", "abc123", cfg)

	lines := strings.Split(got, "\n")
	// 期望行：title / 原价100.00 / 券后价70.00 / 返利3.50 / 教程 / tkl / 理由desc
	if len(lines) != 7 {
		t.Fatalf("line count = %d, want 7; got:\n%s", len(lines), got)
	}
	want := []string{
		"测试商品标题",
		"【原价】100.00元",
		"【券后价】70.00元",
		"【最高返利】3.50元",
		"【返利教程】http://suo.im/4CYwC4",
		"【复制此信息打开手机淘宝即可查看并下单】abc123",
		"【必买理由】desc",
	}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("line[%d] = %q, want %q", i, lines[i], w)
		}
	}
}

func TestRender_NoCouponBranch(t *testing.T) {
	prices := RenderPrices{Available: true, Zk: 100, Reserve: 129}
	cfg := RenderConfig{RebateRate: 0.05, TutorialURL: "http://suo.im/4CYwC4"}
	// reason 非空但应被忽略
	got := Render("测试商品标题", prices, 0, "ignored-reason", "abc123", cfg)

	lines := strings.Split(got, "\n")
	if len(lines) != 7 {
		t.Fatalf("line count = %d, want 7; got:\n%s", len(lines), got)
	}
	want := []string{
		"测试商品标题",
		"【原价】129.00元", // Reserve
		"【促销价】100.00元", // Zk
		"【最高返利】5.00元", // 100×0.05
		"【返利教程】http://suo.im/4CYwC4",
		"【复制此信息打开手机淘宝即可查看并下单】abc123",
		"【必买理由】限时特惠",
	}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("line[%d] = %q, want %q", i, lines[i], w)
		}
	}
}

func TestRender_OmitsLines(t *testing.T) {
	// 1) TutorialURL="" → 无【返利教程】行
	cfg := RenderConfig{RebateRate: 0.05, TutorialURL: ""}
	got := Render("T", RenderPrices{Available: true, Zk: 100, Reserve: 129}, 30, "desc", "tk", cfg)
	if strings.Contains(got, "【返利教程】") {
		t.Errorf("expected no tutorial line, got:\n%s", got)
	}

	// 2) reason="" with coupon>0 → 无【必买理由】行
	got = Render("T", RenderPrices{Available: true, Zk: 100, Reserve: 129}, 30, "", "tk", cfg)
	if strings.Contains(got, "【必买理由】") {
		t.Errorf("expected no reason line, got:\n%s", got)
	}

	// 3) prices.Available=false → 无价格三行，其余行仍在
	got = Render("T", RenderPrices{Available: false, Zk: 100, Reserve: 129}, 30, "desc", "tk", cfg)
	if strings.Contains(got, "【原价】") || strings.Contains(got, "【券后价】") ||
		strings.Contains(got, "【促销价】") || strings.Contains(got, "【最高返利】") {
		t.Errorf("expected no price lines, got:\n%s", got)
	}
	if !strings.Contains(got, "T") || !strings.Contains(got, "【复制此信息打开手机淘宝即可查看并下单】tk") ||
		!strings.Contains(got, "【必买理由】desc") {
		t.Errorf("other lines should remain, got:\n%s", got)
	}
	if strings.HasPrefix(got, "\n") || strings.HasSuffix(got, "\n") {
		t.Errorf("unexpected leading/trailing newline: %q", got)
	}
}

func TestRender_ExactTemplate(t *testing.T) {
	prices := RenderPrices{Available: true, Zk: 100, Reserve: 129}
	cfg := RenderConfig{RebateRate: 0.05, TutorialURL: "http://suo.im/4CYwC4"}
	got := Render("测试商品标题", prices, 30, "desc", "abc123", cfg)

	want := strings.Join([]string{
		"测试商品标题",
		"【原价】100.00元",
		"【券后价】70.00元",
		"【最高返利】3.50元",
		"【返利教程】http://suo.im/4CYwC4",
		"【复制此信息打开手机淘宝即可查看并下单】abc123",
		"【必买理由】desc",
	}, "\n")
	if got != want {
		t.Errorf("Render() =\n%q\nwant\n%q", got, want)
	}
}
