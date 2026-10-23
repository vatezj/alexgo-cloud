package service

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"time"
)

// 确定性自算（spec §4.7 / plan Task 6 Ruling）：
// 不复刻 Godot PCG32，直接 sha256 前 4 字节 LE → uint32 种子。
// 同一天全局相同即可；客户端 fetch 先本地后请求，服务端值权威覆盖。

var luckPeriods = [4]string{"day", "dusk", "night", "any"}

// YMDToDays civil 算法（照 fish_db/online_service.gd _ymd_to_days，正年份）。
func YMDToDays(y, m, d int) int {
	yy := y
	if m <= 2 {
		yy--
	}
	era := floorDiv(yy, 400)
	yoe := yy - era*400
	md := m + 9
	if m > 2 {
		md = m - 3
	}
	doy := (153*md+2)/5 + d - 1
	doe := yoe*365 + floorDiv(yoe, 4) - floorDiv(yoe, 100) + doy
	return era*146097 + doe - 719468
}

// daysToYMD civil 反向（_days_to_ymd）。
func daysToYMD(zIn int) (y, m, d int) {
	z := zIn + 719468
	era := floorDiv(z, 146097)
	doe := z - era*146097
	yoe := (doe - floorDiv(doe, 1460) + floorDiv(doe, 36524) - floorDiv(doe, 146096)) / 365
	y = yoe + era*400
	doy := doe - (365*yoe + floorDiv(yoe, 4) - floorDiv(yoe, 100))
	mp := (5*doy + 2) / 153
	d = doy - (153*mp+2)/5 + 1
	m = mp + 3
	if mp >= 10 {
		m = mp - 9
	}
	if m <= 2 {
		y++
	}
	return y, m, d
}

func floorDiv(a, b int) int {
	q := a / b
	if (a^b) < 0 && q*b != a {
		q--
	}
	return q
}

// SeedFromSHA256 str(seed).sha256() 前 4 字节 little-endian → uint32。
func SeedFromSHA256(s string) uint32 {
	sum := sha256.Sum256([]byte(s))
	return binary.LittleEndian.Uint32(sum[:4])
}

// ComputeLuck 当日 epoch 天数 → {period, depth_bias}。
// period = [day,dusk,night,any][seed%4]；depth_bias = ((seed>>8)%201)/1000 ∈ [0, 0.2]。
func ComputeLuck(epochDays int) (period string, depthBias float64) {
	seed := SeedFromSHA256(fmt.Sprintf("%d", epochDays))
	period = luckPeriods[seed%4]
	depthBias = float64((seed>>8)%201) / 1000.0
	return period, depthBias
}

// ComputeFishKing ISO 周文本 → pool 内确定性一条。pool 空 → ""。
func ComputeFishKing(week string, pool []string) string {
	if len(pool) == 0 {
		return ""
	}
	seed := SeedFromSHA256(week)
	return pool[seed%uint32(len(pool))]
}

// ISOMonthWeek "YYYY-Www"：本周四所在年为周序年（周四锚定，civil-day 空间，与本地时区无关）。
func ISOMonthWeek(t time.Time) string {
	loc := t.Location()
	y, m, d := t.Date()
	days := YMDToDays(y, int(m), d)
	weekday := mod7(days + 3) // 0=周一（1970-01-01 为周四）
	thuDays := days + (3 - weekday)
	thy, _, _ := daysToYMD(thuDays)
	jan4 := YMDToDays(thy, 1, 4)
	week1Thu := jan4 + (3 - mod7(jan4+3))
	week := (thuDays-week1Thu)/7 + 1
	_ = loc
	return fmt.Sprintf("%04d-W%02d", thy, week)
}

func mod7(x int) int {
	r := x % 7
	if r < 0 {
		r += 7
	}
	return r
}
