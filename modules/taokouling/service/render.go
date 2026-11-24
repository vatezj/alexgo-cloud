package service

import (
	"strconv"
	"strings"
)

// RenderPrices 渲染所需价格输入。
type RenderPrices struct {
	Available bool    // false → 省价格三行（原价/标签/最高返利）
	Zk        float64 // 折扣价
	Reserve   float64 // 划线价
}

// RenderConfig 渲染配置。
type RenderConfig struct {
	RebateRate  float64 // <=0 视为 0.05
	TutorialURL string  // 空 → 省【返利教程】行
}

// Render 按 spec §8.1 分支与模板拼装营销文案（纯函数）。
//
// coupon=0 → 原价=Reserve、展示=Zk、标签=【促销价】、理由=「限时特惠」（忽略入参 reason）；
// coupon>0 → 原价=Zk、展示=Zk-coupon、标签=【券后价】、理由=reason（可空 → 省【必买理由】行）。
// 价格与返利格式化 strconv.FormatFloat(v,'f',2,64)；最高返利=展示价×RebateRate。
// 模板行（\n 连接，末尾无换行）：
// {title}\n【原价】{o}元\n{tag}{s}元\n【最高返利】{r}元\n[【返利教程】{url}\n]【复制此信息打开手机淘宝即可查看并下单】{newTkl}[\n【必买理由】{reason}]
func Render(title string, prices RenderPrices, coupon int, reason, newTkl string, cfg RenderConfig) string {
	rate := cfg.RebateRate
	if rate <= 0 {
		rate = 0.05
	}

	// 理由解析与价格可用性无关：coupon=0 恒「限时特惠」，coupon>0 取 reason（空则省行）。
	var resolvedReason string
	if coupon == 0 {
		resolvedReason = "限时特惠"
	} else {
		resolvedReason = reason
	}

	lines := make([]string, 0, 7)
	lines = append(lines, title)

	if prices.Available {
		var orig, show float64
		var tag string
		if coupon > 0 {
			orig = prices.Zk
			show = prices.Zk - float64(coupon)
			tag = "【券后价】"
		} else {
			orig = prices.Reserve
			show = prices.Zk
			tag = "【促销价】"
		}
		rebate := show * rate
		lines = append(lines,
			"【原价】"+formatMoney(orig)+"元",
			tag+formatMoney(show)+"元",
			"【最高返利】"+formatMoney(rebate)+"元",
		)
	}

	if cfg.TutorialURL != "" {
		lines = append(lines, "【返利教程】"+cfg.TutorialURL)
	}
	lines = append(lines, "【复制此信息打开手机淘宝即可查看并下单】"+newTkl)

	if resolvedReason != "" {
		lines = append(lines, "【必买理由】"+resolvedReason)
	}

	return strings.Join(lines, "\n")
}

// formatMoney 按 spec 固定两位小数格式化金额。
func formatMoney(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}
