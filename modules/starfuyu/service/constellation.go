package service

// ConstellationList 12 星座枚举（从 goLucky utils/constellation.go 原样迁入本包——
// ProfileService 校验要用，Task 7 图片服务只复用；brief 钉死避免任务倒序依赖）。
var ConstellationList = []string{
	"白羊座",
	"金牛座",
	"双子座",
	"巨蟹座",
	"狮子座",
	"处女座",
	"天秤座",
	"天蝎座",
	"射手座",
	"摩羯座",
	"水瓶座",
	"双鱼座",
}

// IsValidConstellation 是否 12 星座枚举值之一（非法 → controller 映射 10001）。
func IsValidConstellation(constellation string) bool {
	for _, c := range ConstellationList {
		if c == constellation {
			return true
		}
	}
	return false
}

// constellationSlugs 中星座→cute 素材英文 slug（与 scripts/starfuyu_cutout.py
// 产物命名 cute_{slug}_cut.png 逐字一致，12 条硬表，两边同步改）。
var constellationSlugs = map[string]string{
	"白羊座": "aries",
	"金牛座": "taurus",
	"双子座": "gemini",
	"巨蟹座": "cancer",
	"狮子座": "leo",
	"处女座": "virgo",
	"天秤座": "libra",
	"天蝎座": "scorpio",
	"射手座": "sagittarius",
	"摩羯座": "capricorn",
	"水瓶座": "aquarius",
	"双鱼座": "pisces",
}

// constellationSlug 中星座→slug；未命中返回 ok=false（调用方映射 30502/10001）。
func constellationSlug(c string) (string, bool) {
	s, ok := constellationSlugs[c]
	return s, ok
}
