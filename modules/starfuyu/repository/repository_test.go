package repository

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/starfuyu/model"
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/tenant/gormplugin"
)

// newTestDB sqlite 内存库 + 六模型 AutoMigrate + gormplugin（照 fishgame module_test 前半）。
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&model.Profile{}, &model.Fortune{}, &model.Bazi{},
		&model.Divination{}, &model.RequestLimit{}, &model.SignCache{},
	); err != nil {
		t.Fatal(err)
	}
	if err := gormplugin.Register(db, gormplugin.Options{ExemptTables: []string{"tenants", "casbin_rule"}}); err != nil {
		t.Fatal(err)
	}
	return db
}

// baseCtx 无租户注入（tid=0）：插件不注入，用户表落 0、内容表恒 0 的缺省测试路径。
func baseCtx() context.Context { return context.Background() }

// TestProfileUpsertIdempotent brief Step 1 必写：软查 miss + 同 member 二次 Upsert 只有一行且覆盖星座。
func TestProfileUpsertIdempotent(t *testing.T) {
	db := newTestDB(t)
	repo := NewProfileRepository(db)
	ctx := baseCtx()

	miss, err := repo.GetByMember(ctx, 1001)
	if err != nil {
		t.Fatalf("GetByMember miss err = %v, want nil", err)
	}
	if miss != nil {
		t.Fatalf("GetByMember miss = %+v, want nil", miss)
	}

	p1, err := repo.Upsert(ctx, 1001, "白羊座")
	if err != nil {
		t.Fatalf("首次 Upsert err = %v", err)
	}
	if p1.ID == 0 || p1.Constellation != "白羊座" || p1.MemberUserID != 1001 {
		t.Fatalf("首次 Upsert 行形状不对: %+v", p1)
	}

	p2, err := repo.Upsert(ctx, 1001, "金牛座")
	if err != nil {
		t.Fatalf("二次 Upsert err = %v", err)
	}
	if p2.ID != p1.ID {
		t.Fatalf("同 member 应更新不新增: p1.ID=%d p2.ID=%d", p1.ID, p2.ID)
	}
	if p2.Constellation != "金牛座" {
		t.Fatalf("二次 Upsert 应覆盖星座, got %q", p2.Constellation)
	}

	var n int64
	if err := db.Model(&model.Profile{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("应只有 1 行, got %d", n)
	}
}

// TestFortuneUpsertBatchOverwrites brief Step 1 必写：(date,constellation) 冲突更新整批业务列、不增行。
func TestFortuneUpsertBatchOverwrites(t *testing.T) {
	db := newTestDB(t)
	repo := NewFortuneRepository(db)
	ctx := baseCtx()
	const date = "2026-11-12"

	rows := []*model.Fortune{
		{
			TenantID: 0, Date: date, Constellation: "白羊座",
			DailyOverview: "初版概述", OverallScore: 80, LuckyNumber: 3,
			LuckyColor: model.LuckyColor{Chinese: "红色", English: "Red", Hex: "#ff0000"},
			Summary:    "初版总评",
		},
		{
			TenantID: 0, Date: date, Constellation: "金牛座",
			DailyOverview: "初版概述二", OverallScore: 70, LuckyNumber: 7,
			LuckyColor: model.LuckyColor{Chinese: "绿色", English: "Green", Hex: "#00ff00"},
			Summary:    "初版总评二",
		},
	}
	if err := repo.UpsertBatch(ctx, rows); err != nil {
		t.Fatalf("UpsertBatch 首批 err = %v", err)
	}
	if n, err := repo.CountByDate(ctx, date); err != nil || n != 2 {
		t.Fatalf("CountByDate = %d, %v; want 2, nil", n, err)
	}
	got, err := repo.GetByDate(ctx, date)
	if err != nil {
		t.Fatalf("GetByDate err = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("GetByDate len = %d, want 2", len(got))
	}
	for _, f := range got {
		if f.TenantID != 0 {
			t.Fatalf("内容表 tenant_id 应恒 0, got %d (const=%s)", f.TenantID, f.Constellation)
		}
	}
	firstA, err := repo.GetOne(ctx, date, "白羊座")
	if err != nil || firstA == nil {
		t.Fatalf("GetOne 首次命中 err=%v row=%v", err, firstA)
	}
	idA := firstA.ID

	// 整批覆盖：分数/摘要/幸运色全换，行数不变、主键不变。
	overwrite := []*model.Fortune{
		{
			TenantID: 0, Date: date, Constellation: "白羊座",
			DailyOverview: "改版概述", OverallScore: 90, LoveScore: 88, LuckyNumber: 9,
			LuckyColor: model.LuckyColor{Chinese: "金色", English: "Gold", Hex: "#ffd700"},
			Summary:    "改版总评",
		},
		{
			TenantID: 0, Date: date, Constellation: "金牛座",
			DailyOverview: "改版概述二", OverallScore: 95, CareerScore: 99, LuckyNumber: 1,
			LuckyColor: model.LuckyColor{Chinese: "蓝色", English: "Blue", Hex: "#0000ff"},
			Summary:    "改版总评二",
		},
	}
	if err := repo.UpsertBatch(ctx, overwrite); err != nil {
		t.Fatalf("UpsertBatch 覆盖批 err = %v", err)
	}
	if n, err := repo.CountByDate(ctx, date); err != nil || n != 2 {
		t.Fatalf("覆盖后 CountByDate = %d, %v; want 2, nil（不得增行）", n, err)
	}
	after, err := repo.GetOne(ctx, date, "白羊座")
	if err != nil || after == nil {
		t.Fatalf("覆盖后 GetOne err=%v row=%v", err, after)
	}
	if after.ID != idA {
		t.Fatalf("覆盖应更新原行: id %d → %d", idA, after.ID)
	}
	if after.OverallScore != 90 || after.LoveScore != 88 || after.Summary != "改版总评" {
		t.Fatalf("业务列未覆盖: %+v", after)
	}
	if after.LuckyColor != (model.LuckyColor{Chinese: "金色", English: "Gold", Hex: "#ffd700"}) {
		t.Fatalf("lucky_color JSON 未覆盖: %+v", after.LuckyColor)
	}
	miss, err := repo.GetOne(ctx, date, "双子座")
	if err != nil {
		t.Fatalf("GetOne miss err = %v, want nil", err)
	}
	if miss != nil {
		t.Fatalf("GetOne miss = %+v, want nil", miss)
	}
	if n, err := repo.CountByDate(ctx, "2026-11-13"); err != nil || n != 0 {
		t.Fatalf("他日 CountByDate = %d, %v; want 0, nil", n, err)
	}
}

// TestSignCacheCreateRaceReRead brief Step 1 必写：同号 Create 两次，第二次返回已存在行内容（首签稳定）。
func TestSignCacheCreateRaceReRead(t *testing.T) {
	db := newTestDB(t)
	repo := NewSignCacheRepository(db)
	ctx := baseCtx()

	first, err := repo.Create(ctx, 42, "第一签原文")
	if err != nil {
		t.Fatalf("首次 Create err = %v", err)
	}
	if first.ID == 0 || first.TenantID != 0 || first.SignContent != "第一签原文" {
		t.Fatalf("首次 Create 行形状不对: %+v", first)
	}

	second, err := repo.Create(ctx, 42, "被抢占的签文")
	if err != nil {
		t.Fatalf("二次 Create err = %v", err)
	}
	if second.SignContent != "第一签原文" {
		t.Fatalf("二次 Create 应返回已存在行内容（首签稳定）, got %q", second.SignContent)
	}
	if second.ID != first.ID {
		t.Fatalf("同号应同一行: first.ID=%d second.ID=%d", first.ID, second.ID)
	}

	got, err := repo.Get(ctx, 42)
	if err != nil {
		t.Fatalf("Get err = %v", err)
	}
	if got == nil || got.SignContent != "第一签原文" {
		t.Fatalf("Get 应命中首签, got %+v", got)
	}
	miss, err := repo.Get(ctx, 99)
	if err != nil {
		t.Fatalf("Get miss err = %v, want nil", err)
	}
	if miss != nil {
		t.Fatalf("Get miss = %+v, want nil", miss)
	}
}

// TestBaziUpsertAndGetByMember 软查 miss + (tenant,member) 覆盖保存（uk 单行）。
func TestBaziUpsertAndGetByMember(t *testing.T) {
	db := newTestDB(t)
	repo := NewBaziRepository(db)
	ctx := baseCtx()

	miss, err := repo.GetByMember(ctx, 2002)
	if err != nil {
		t.Fatalf("GetByMember miss err = %v, want nil", err)
	}
	if miss != nil {
		t.Fatalf("GetByMember miss = %+v, want nil", miss)
	}

	in := &model.Bazi{
		MemberUserID: 2002, BirthDate: "1990-01-01", BirthTime: "08:30:00", BirthPlace: "上海",
		BaziInfo: model.BaziInfo{
			YearGan: "己", YearZhi: "午", MonthGan: "丙", MonthZhi: "子",
			DayGan: "甲", DayZhi: "戌", HourGan: "戊", HourZhi: "辰",
			Constellation: "摩羯座",
		},
	}
	b1, err := repo.Upsert(ctx, 2002, in)
	if err != nil {
		t.Fatalf("首次 Upsert err = %v", err)
	}
	if b1.ID == 0 || b1.MemberUserID != 2002 || b1.BaziInfo.YearGan != "己" {
		t.Fatalf("首次 Upsert 行形状不对: %+v", b1)
	}

	in2 := &model.Bazi{
		MemberUserID: 2002, BirthDate: "1990-01-01", BirthTime: "09:00:00", BirthPlace: "北京",
		BaziInfo: model.BaziInfo{YearGan: "己", YearZhi: "午", Constellation: "摩羯座"},
	}
	b2, err := repo.Upsert(ctx, 2002, in2)
	if err != nil {
		t.Fatalf("二次 Upsert err = %v", err)
	}
	if b2.ID != b1.ID {
		t.Fatalf("同 member 应更新不新增: b1.ID=%d b2.ID=%d", b1.ID, b2.ID)
	}
	if b2.BirthTime != "09:00:00" || b2.BirthPlace != "北京" {
		t.Fatalf("出生信息未覆盖: %+v", b2)
	}

	var n int64
	if err := db.Model(&model.Bazi{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("应只有 1 行, got %d", n)
	}
}

// TestDivinationCreateAndListByMember 创建回读 + member 分页（倒序、total 独立）。
func TestDivinationCreateAndListByMember(t *testing.T) {
	db := newTestDB(t)
	repo := NewDivinationRepository(db)
	ctx := baseCtx()

	var lastID uint64
	for i, q := range []string{"问事业", "问财运", "问健康"} {
		row, err := repo.Create(ctx, &model.Divination{
			MemberUserID: 7, Question: q, SignNumber: i + 1,
			SignContent: "签文", Interpretation: "解读", Category: "事业",
		})
		if err != nil {
			t.Fatalf("Create[%d] err = %v", i, err)
		}
		if row.ID == 0 || row.MemberUserID != 7 || row.Question != q {
			t.Fatalf("Create[%d] 行形状不对: %+v", i, row)
		}
		lastID = row.ID
	}
	// 他人记录不串页。
	if _, err := repo.Create(ctx, &model.Divination{MemberUserID: 8, Question: "别人的", SignNumber: 9}); err != nil {
		t.Fatal(err)
	}

	page1, total, err := repo.ListByMember(ctx, 7, 1, 2)
	if err != nil {
		t.Fatalf("ListByMember err = %v", err)
	}
	if total != 3 || len(page1) != 2 {
		t.Fatalf("第一页 total=%d len=%d, want 3/2", total, len(page1))
	}
	if page1[0].ID != lastID {
		t.Fatalf("应按最新在前排序: 首行 ID=%d want %d", page1[0].ID, lastID)
	}
	page2, total2, err := repo.ListByMember(ctx, 7, 2, 2)
	if err != nil {
		t.Fatalf("ListByMember 第二页 err = %v", err)
	}
	if total2 != 3 || len(page2) != 1 {
		t.Fatalf("第二页 total=%d len=%d, want 3/1", total2, len(page2))
	}
	if page2[0].Question == "别人的" {
		t.Fatalf("分页不得串入他人记录: %+v", page2[0])
	}
}

// TestRequestLimitCheckAndIncrement 查或建 + 逐次 +1；按 (member,date,type) 隔离。
func TestRequestLimitCheckAndIncrement(t *testing.T) {
	db := newTestDB(t)
	repo := NewRequestLimitRepository(db)
	ctx := baseCtx()
	const date = "2026-11-12"

	for want := 1; want <= 3; want++ {
		got, err := repo.CheckAndIncrement(ctx, 3003, date, model.RequestTypeBaziCalculate)
		if err != nil {
			t.Fatalf("CheckAndIncrement 第 %d 次 err = %v", want, err)
		}
		if got != want {
			t.Fatalf("CheckAndIncrement 第 %d 次 = %d, want %d", want, got, want)
		}
	}
	// 换 member / 换 type / 换 date 各自从 1 起。
	if got, err := repo.CheckAndIncrement(ctx, 4004, date, model.RequestTypeBaziCalculate); err != nil || got != 1 {
		t.Fatalf("他人计数 = %d, %v; want 1, nil", got, err)
	}
	if got, err := repo.CheckAndIncrement(ctx, 3003, date, model.RequestTypeDivinationDraw); err != nil || got != 1 {
		t.Fatalf("他类型计数 = %d, %v; want 1, nil", got, err)
	}
	if got, err := repo.CheckAndIncrement(ctx, 3003, "2026-11-13", model.RequestTypeBaziCalculate); err != nil || got != 1 {
		t.Fatalf("他日计数 = %d, %v; want 1, nil", got, err)
	}
}

// TestRequestLimitAdminTodaySummary 按 type 聚合 SUM(count)，且只统计指定 date。
func TestRequestLimitAdminTodaySummary(t *testing.T) {
	db := newTestDB(t)
	repo := NewRequestLimitRepository(db)
	ctx := baseCtx()
	const date = "2026-11-12"

	// A: bazi 3 次、divination 1 次；B: bazi 1 次；他日 9 次不入账。
	for i := 0; i < 3; i++ {
		if _, err := repo.CheckAndIncrement(ctx, 1, date, model.RequestTypeBaziCalculate); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repo.CheckAndIncrement(ctx, 1, date, model.RequestTypeDivinationDraw); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CheckAndIncrement(ctx, 2, date, model.RequestTypeBaziCalculate); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 9; i++ {
		if _, err := repo.CheckAndIncrement(ctx, 3, "2026-11-13", model.RequestTypeBaziCalculate); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := repo.AdminTodaySummary(ctx, date)
	if err != nil {
		t.Fatalf("AdminTodaySummary err = %v", err)
	}
	want := map[string]int64{
		model.RequestTypeBaziCalculate:  4,
		model.RequestTypeDivinationDraw: 1,
	}
	if len(rows) != len(want) {
		t.Fatalf("summary 行数 = %d (%+v), want %d", len(rows), rows, len(want))
	}
	for _, row := range rows {
		if want[row.Type] != row.Count {
			t.Fatalf("summary[%s] = %d, want %d (全部: %+v)", row.Type, row.Count, want[row.Type], rows)
		}
	}

	empty, err := repo.AdminTodaySummary(ctx, "2000-01-01")
	if err != nil {
		t.Fatalf("空日 summary err = %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("空日 summary 应为空, got %+v", empty)
	}
}

// TestUserTableTenantViaPlugin 用户表不硬编码租户：写走 ctx 回填、读按 ctx 过滤（gormplugin 惯例）。
func TestUserTableTenantViaPlugin(t *testing.T) {
	db := newTestDB(t)
	repo := NewProfileRepository(db)
	ctx5 := tenant.WithTenantID(baseCtx(), 5)
	ctx6 := tenant.WithTenantID(baseCtx(), 6)

	p, err := repo.Upsert(ctx5, 9, "天秤座")
	if err != nil {
		t.Fatalf("tid=5 Upsert err = %v", err)
	}
	if p.TenantID != 5 {
		t.Fatalf("gormplugin 应回填 tenant_id=5, got %d", p.TenantID)
	}
	hit, err := repo.GetByMember(ctx5, 9)
	if err != nil || hit == nil || hit.Constellation != "天秤座" {
		t.Fatalf("同租户读命中失败: hit=%+v err=%v", hit, err)
	}
	miss, err := repo.GetByMember(ctx6, 9)
	if err != nil {
		t.Fatalf("跨租户读 err = %v, want nil", err)
	}
	if miss != nil {
		t.Fatalf("跨租户读应 miss, got %+v", miss)
	}
}
