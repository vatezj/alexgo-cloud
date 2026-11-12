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
