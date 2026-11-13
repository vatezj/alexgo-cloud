package model

import (
	"database/sql/driver"
	"reflect"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// TestLuckyColorRoundtrip LuckyColor 的 driver.Valuer/Scan JSON 往返（goLucky models/fortune.go 同构）。
func TestLuckyColorRoundtrip(t *testing.T) {
	in := LuckyColor{Chinese: "红色", English: "Red", Hex: "#ff0000"}
	v, err := in.Value()
	if err != nil {
		t.Fatalf("Value() err = %v, want nil", err)
	}
	var out LuckyColor
	if err := out.Scan(v); err != nil {
		t.Fatalf("Scan(%T) err = %v, want nil", v, err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("roundtrip = %+v, want %+v", out, in)
	}
	// 空列/未赋值路径：Scan(nil) 不报错且不动已有值。
	if err := out.Scan(nil); err != nil {
		t.Fatalf("Scan(nil) err = %v, want nil", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("Scan(nil) 后被改动 = %+v, want %+v", out, in)
	}
}

// TestDailyTodosRoundtrip DailyTodos 的 Valuer/Scan JSON 往返 + nil 不动已有值。
func TestDailyTodosRoundtrip(t *testing.T) {
	in := []string{"喝八杯水", "给前任点赞前先想想", "19点后别做决定"}
	// in 的持有者：named type 才能挂 driver.Valuer / sql.Scanner。
	var _ driver.Valuer = DailyTodos(in)
	v, err := DailyTodos(in).Value()
	if err != nil {
		t.Fatalf("Value() err = %v, want nil", err)
	}
	b, ok := v.([]byte)
	if !ok {
		t.Fatalf("Value() type = %T, want []byte", v)
	}
	if !strings.Contains(string(b), "喝八杯水") {
		t.Fatalf("Value() = %s, want 含 %q", b, "喝八杯水")
	}

	var out DailyTodos
	if err := out.Scan(v); err != nil {
		t.Fatalf("Scan(%T) err = %v, want nil", v, err)
	}
	if !reflect.DeepEqual(DailyTodos(in), out) {
		t.Fatalf("roundtrip = %v, want %v", out, in)
	}
	// []byte 分支（MySQL json 列取回路径）。
	if err := out.Scan([]byte(`["a","b"]`)); err != nil {
		t.Fatalf("Scan([]byte) err = %v, want nil", err)
	}
	if !reflect.DeepEqual(DailyTodos{"a", "b"}, out) {
		t.Fatalf("Scan([]byte) = %v, want [a b]", out)
	}
	// string 分支（sqlite 部分路径）。
	if err := out.Scan(string(`["x"]`)); err != nil {
		t.Fatalf("Scan(string) err = %v, want nil", err)
	}
	if !reflect.DeepEqual(DailyTodos{"x"}, out) {
		t.Fatalf("Scan(string) = %v, want [x]", out)
	}
	// 空列/未赋值路径：Scan(nil) 不报错且不动已有值。
	if err := out.Scan(nil); err != nil {
		t.Fatalf("Scan(nil) err = %v, want nil", err)
	}
	if !reflect.DeepEqual(DailyTodos{"x"}, out) {
		t.Fatalf("Scan(nil) 后被改动 = %v, want [x]", out)
	}
}

// TestBaziInfoRoundtrip BaziInfo（四柱 + 五行统计 map）的 JSON 往返（goLucky models/bazi.go 同构）。
func TestBaziInfoRoundtrip(t *testing.T) {
	in := BaziInfo{
		YearGan: "甲", YearZhi: "子",
		MonthGan: "丙", MonthZhi: "寅",
		DayGan: "庚", DayZhi: "辰",
		HourGan: "壬", HourZhi: "午",
		Wuxing: WuxingInfo{
			YearWuxing:  "木",
			MonthWuxing: "火",
			DayWuxing:   "金",
			HourWuxing:  "水",
			TotalWuxing: map[string]int{"金": 2, "木": 1, "水": 1, "火": 2, "土": 2},
		},
		SolarTerm:     "立冬",
		LunarDate:     "十月初五",
		Zodiac:        "鼠",
		Constellation: "天蝎座",
	}
	v, err := in.Value()
	if err != nil {
		t.Fatalf("Value() err = %v, want nil", err)
	}
	var out BaziInfo
	if err := out.Scan(v); err != nil {
		t.Fatalf("Scan(%T) err = %v, want nil", v, err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("roundtrip = %+v, want %+v", out, in)
	}
	if err := out.Scan(nil); err != nil {
		t.Fatalf("Scan(nil) err = %v, want nil", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("Scan(nil) 后被改动 = %+v, want %+v", out, in)
	}
}

// TestTableName 六模型 TableName 必须与迁移 DDL 表名严格同构（spec §5；脚手架裸表名已清零）。
func TestTableName(t *testing.T) {
	cases := []struct {
		name  string
		model any
		want  string
	}{
		{"Profile", Profile{}, "starfuyu_profile"},
		{"Fortune", Fortune{}, "starfuyu_fortune"},
		{"Bazi", Bazi{}, "starfuyu_bazi"},
		{"Divination", Divination{}, "starfuyu_divination"},
		{"RequestLimit", RequestLimit{}, "starfuyu_request_limit"},
		{"SignCache", SignCache{}, "starfuyu_sign_cache"},
	}
	for _, c := range cases {
		var got string
		switch m := c.model.(type) {
		case Profile:
			got = m.TableName()
		case Fortune:
			got = m.TableName()
		case Bazi:
			got = m.TableName()
		case Divination:
			got = m.TableName()
		case RequestLimit:
			got = m.TableName()
		case SignCache:
			got = m.TableName()
		default:
			t.Fatalf("%s: unhandled model type %T", c.name, m)
		}
		if got != c.want {
			t.Errorf("%s.TableName() = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestAutoMigrateColumns sqlite AutoMigrate 六模型后抽查列名与唯一/索引名，防 gorm 标签笔误
// （DDL 走 MySQL 迁移，sqlite 只做「模型即真源」的结构自证，见 brief Step 4）。
func TestAutoMigrateColumns(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Profile{}, &Fortune{}, &Bazi{}, &Divination{}, &RequestLimit{}, &SignCache{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	colCases := []struct {
		model any
		cols  []string
	}{
		{Profile{}, []string{"id", "tenant_id", "member_user_id", "constellation", "created_at", "updated_at"}},
		{Fortune{}, []string{"id", "tenant_id", "date", "constellation", "daily_overview", "overall_score", "love_score", "career_score", "wealth_score", "health_score", "lucky_number", "lucky_color", "summary", "love_tip", "career_tip", "wealth_tip", "health_tip", "match_constellation", "advice", "avoid", "reunion_tip", "single_tip", "job_tip", "reunion_score", "single_score", "job_score", "daily_todos", "slogan", "created_at", "updated_at"}},
		{Bazi{}, []string{"id", "tenant_id", "member_user_id", "birth_date", "birth_time", "birth_place", "bazi_info", "created_at", "updated_at"}},
		{Divination{}, []string{"id", "tenant_id", "member_user_id", "bazi_id", "question", "sign_number", "sign_content", "interpretation", "category", "lucky_direction", "lucky_color", "advice", "created_at", "updated_at"}},
		{RequestLimit{}, []string{"id", "tenant_id", "member_user_id", "date", "type", "count", "created_at", "updated_at"}},
		{SignCache{}, []string{"id", "tenant_id", "sign_number", "sign_content", "created_at", "updated_at"}},
	}
	for _, c := range colCases {
		for _, col := range c.cols {
			if !db.Migrator().HasColumn(c.model, col) {
				t.Errorf("%T 缺列 %q", c.model, col)
			}
		}
	}

	// 唯一键 / 索引名与 brief Step 3 表格一一对应。
	idxCases := []struct {
		model any
		index string
	}{
		{Profile{}, "uk_profile_member"},
		{Fortune{}, "uk_fortune_date_const"},
		{Bazi{}, "uk_bazi_member"},
		{Divination{}, "idx_div_member_created"},
		{RequestLimit{}, "uk_limit_user_date_type"},
		{SignCache{}, "uk_sign_number"},
	}
	for _, c := range idxCases {
		if !db.Migrator().HasIndex(c.model, c.index) {
			t.Errorf("%T 缺索引 %q", c.model, c.index)
		}
	}
}
