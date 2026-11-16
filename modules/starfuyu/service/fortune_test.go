package service

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/starfuyu/model"
	"alexGo-cloud/modules/starfuyu/repository"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/tenant/gormplugin"
)

// stubFetcher FortuneFetcher 测试替身：记录调用次数、返回预置行/错误。
type stubFetcher struct {
	calls int
	rows  []*model.Fortune
	err   error
}

func (s *stubFetcher) FetchFortunes(_ context.Context, _ string) ([]*model.Fortune, *model.DailyHoroscope, error) {
	s.calls++
	return s.rows, nil, s.err
}

// newTestDB sqlite 内存库 + Fortune AutoMigrate + gormplugin（照 repository_test 惯例）。
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Fortune{}, &model.DailyHoroscope{}); err != nil {
		t.Fatal(err)
	}
	if err := gormplugin.Register(db, gormplugin.Options{ExemptTables: []string{"tenants", "casbin_rule"}}); err != nil {
		t.Fatal(err)
	}
	return db
}

// twelveRows 12 星座行（白羊→双鱼），summary 可区分；分享卡字段用 mark 拼值
// （match_constellation 取下一星座保证是合法枚举且≠自身），Task 2/5 测试复用。
func twelveRows(date, mark string) []*model.Fortune {
	consts := []string{"白羊座", "金牛座", "双子座", "巨蟹座", "狮子座", "处女座", "天秤座", "天蝎座", "射手座", "摩羯座", "水瓶座", "双鱼座"}
	rows := make([]*model.Fortune, 0, 12)
	for i, c := range consts {
		rows = append(rows, &model.Fortune{
			TenantID: 0, Date: date, Constellation: c,
			DailyOverview: mark + "概述", OverallScore: 60 + i, LuckyNumber: i + 1,
			LuckyColor:         model.LuckyColor{Chinese: "红色", English: "Red", Hex: "#ff0000"},
			Summary:            mark + "总评",
			MatchConstellation: consts[(i+1)%12],
			Advice:             mark + "建议",
			Avoid:              mark + "避免",
			ReunionTip:         mark + "复合",
			SingleTip:          mark + "脱单",
			JobTip:             mark + "求职",
			ReunionScore:       66,
			SingleScore:        77,
			JobScore:           88,
			DailyTodos:         model.DailyTodos{mark + "必做一", mark + "必做二", mark + "必做三"},
			Slogan:             mark + "口号",
		})
	}
	return rows
}

// TestFortuneFetch_Empty库 brief 用例 1：库空 → Fetch 调 stub 1 次、落库 12 条、skipped=false。
func TestFortuneFetch_Empty库(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewFortuneRepository(db)
	stub := &stubFetcher{rows: twelveRows("2026-11-12", "首版")}
	svc := NewFortuneService(repo, stub)
	ctx := context.Background()

	skipped, err := svc.Fetch(ctx, "2026-11-12", false)
	if err != nil {
		t.Fatalf("Fetch err = %v, want nil", err)
	}
	if skipped {
		t.Fatalf("空库 Fetch skipped = true, want false")
	}
	if stub.calls != 1 {
		t.Fatalf("stub 调用次数 = %d, want 1", stub.calls)
	}
	if n, err := repo.CountByDate(ctx, "2026-11-12"); err != nil || n != 12 {
		t.Fatalf("落库后 CountByDate = %d, %v; want 12, nil", n, err)
	}
}

// TestFortuneFetch_Full库Skip brief 用例 2：库已有 12 条 → 不调 stub、skipped=true。
func TestFortuneFetch_Full库Skip(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewFortuneRepository(db)
	stub := &stubFetcher{rows: twelveRows("2026-11-12", "不该出现")}
	svc := NewFortuneService(repo, stub)
	ctx := context.Background()

	if err := repo.UpsertBatch(ctx, twelveRows("2026-11-12", "已有")); err != nil {
		t.Fatalf("预置 12 行 err = %v", err)
	}

	skipped, err := svc.Fetch(ctx, "2026-11-12", false)
	if err != nil {
		t.Fatalf("Fetch err = %v, want nil", err)
	}
	if !skipped {
		t.Fatalf("满库 Fetch skipped = false, want true")
	}
	if stub.calls != 0 {
		t.Fatalf("满库不得调 stub, calls = %d, want 0", stub.calls)
	}
}

// TestFortuneFetch_Force覆盖 brief 用例 3：库已有 12 条 + force=true → 调 stub、Upsert 覆盖。
func TestFortuneFetch_Force覆盖(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewFortuneRepository(db)
	const date = "2026-11-12"
	stub := &stubFetcher{rows: twelveRows(date, "改版")}
	svc := NewFortuneService(repo, stub)
	ctx := context.Background()

	if err := repo.UpsertBatch(ctx, twelveRows(date, "旧版")); err != nil {
		t.Fatalf("预置 12 行 err = %v", err)
	}

	skipped, err := svc.Fetch(ctx, date, true)
	if err != nil {
		t.Fatalf("force Fetch err = %v, want nil", err)
	}
	if skipped {
		t.Fatalf("force Fetch skipped = true, want false")
	}
	if stub.calls != 1 {
		t.Fatalf("force 应调 stub, calls = %d, want 1", stub.calls)
	}
	if n, err := repo.CountByDate(ctx, date); err != nil || n != 12 {
		t.Fatalf("force 覆盖后 CountByDate = %d, %v; want 12（不得增行）", n, err)
	}
	row, err := repo.GetOne(ctx, date, "白羊座")
	if err != nil || row == nil {
		t.Fatalf("覆盖后 GetOne err=%v row=%v", err, row)
	}
	if row.Summary != "改版总评" {
		t.Fatalf("force 应覆盖业务列, Summary = %q, want 改版总评", row.Summary)
	}
}

// TestFortuneFetch_PartialBatchFails brief 用例 4：stub 返回 11 条 → 30506、库仍 0 条。
func TestFortuneFetch_PartialBatchFails(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewFortuneRepository(db)
	const date = "2026-11-12"
	rows := twelveRows(date, "缺一")[:11]
	stub := &stubFetcher{rows: rows}
	svc := NewFortuneService(repo, stub)
	ctx := context.Background()

	skipped, err := svc.Fetch(ctx, date, false)
	if err == nil {
		t.Fatalf("11 条应报错, skipped=%v err=nil", skipped)
	}
	if !errs.Is(err, errs.CodeStarfuyuFetchFailed) {
		t.Fatalf("错误码应为 30506, got %v", err)
	}
	if stub.calls != 1 {
		t.Fatalf("stub 调用次数 = %d, want 1", stub.calls)
	}
	if n, cerr := repo.CountByDate(ctx, date); cerr != nil || n != 0 {
		t.Fatalf("失败批不得落库, CountByDate = %d, %v; want 0, nil", n, cerr)
	}
}

// TestFortuneFetch_FetcherError brief 用例 4 变体：stub 报错 → 30506、库 0 条。
func TestFortuneFetch_FetcherError(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewFortuneRepository(db)
	stub := &stubFetcher{err: context.DeadlineExceeded}
	svc := NewFortuneService(repo, stub)

	_, err := svc.Fetch(context.Background(), "2026-11-12", false)
	if !errs.Is(err, errs.CodeStarfuyuFetchFailed) {
		t.Fatalf("抓取错误应包 30506, got %v", err)
	}
	if n, _ := repo.CountByDate(context.Background(), "2026-11-12"); n != 0 {
		t.Fatalf("抓取失败不得落库, CountByDate = %d, want 0", n)
	}
}

// TestFortuneFetch_NilRow 终审 Critical：JSON `[null,{…}×11]` 过 len==12 后
// row.TenantID 会 panic → 必须 30506、零落库、不崩。
func TestFortuneFetch_NilRow(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewFortuneRepository(db)
	const date = "2026-11-12"
	rows := twelveRows(date, "空行")
	rows[3] = nil // 12 行中夹一个 nil
	stub := &stubFetcher{rows: rows}
	svc := NewFortuneService(repo, stub)
	ctx := context.Background()

	skipped, err := svc.Fetch(ctx, date, false)
	if err == nil {
		t.Fatalf("含 nil 行应报错, skipped=%v err=nil", skipped)
	}
	if !errs.Is(err, errs.CodeStarfuyuFetchFailed) {
		t.Fatalf("nil 行错误码应为 30506, got %v", err)
	}
	if n, cerr := repo.CountByDate(ctx, date); cerr != nil || n != 0 {
		t.Fatalf("nil 行批不得落库, CountByDate = %d, %v; want 0, nil", n, cerr)
	}
}

// TestFortuneFetch_DuplicateConstellation 终审 Important：12 行含重复星座 →
// UpsertBatch 会静默折叠、CountByDate 永远 <12 → 必须 30506、零落库。
func TestFortuneFetch_DuplicateConstellation(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewFortuneRepository(db)
	const date = "2026-11-12"
	rows := twelveRows(date, "重复")
	rows[1].Constellation = rows[0].Constellation // 金牛→白羊，白羊出现两次
	stub := &stubFetcher{rows: rows}
	svc := NewFortuneService(repo, stub)
	ctx := context.Background()

	skipped, err := svc.Fetch(ctx, date, false)
	if err == nil {
		t.Fatalf("重复星座应报错, skipped=%v err=nil", skipped)
	}
	if !errs.Is(err, errs.CodeStarfuyuFetchFailed) {
		t.Fatalf("重复星座错误码应为 30506, got %v", err)
	}
	if n, cerr := repo.CountByDate(ctx, date); cerr != nil || n != 0 {
		t.Fatalf("重复星座批不得落库, CountByDate = %d, %v; want 0, nil", n, cerr)
	}
}

// TestFortuneFetch_UnknownConstellation 同校验：未知星座名 → 30506、零落库。
func TestFortuneFetch_UnknownConstellation(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewFortuneRepository(db)
	const date = "2026-11-12"
	rows := twelveRows(date, "未知")
	rows[0].Constellation = "不存在座"
	stub := &stubFetcher{rows: rows}
	svc := NewFortuneService(repo, stub)
	ctx := context.Background()

	_, err := svc.Fetch(ctx, date, false)
	if !errs.Is(err, errs.CodeStarfuyuFetchFailed) {
		t.Fatalf("未知星座错误码应为 30506, got %v", err)
	}
	if n, _ := repo.CountByDate(ctx, date); n != 0 {
		t.Fatalf("未知星座批不得落库, CountByDate = %d, want 0", n)
	}
}

// TestFortuneGetByDate_Empty库 brief 用例 5：空库 → 空切片非 error。
func TestFortuneGetByDate_Empty库(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewFortuneRepository(db)
	svc := NewFortuneService(repo, &stubFetcher{})

	got, err := svc.GetByDate(context.Background(), "2026-11-12")
	if err != nil {
		t.Fatalf("空库 GetByDate err = %v, want nil", err)
	}
	if got == nil {
		t.Fatalf("空库 GetByDate 返回 nil 切片, want 空列表")
	}
	if len(got) != 0 {
		t.Fatalf("空库 GetByDate len = %d, want 0", len(got))
	}
}

// TestFortuneGetOne_MissNotFound Step 3：miss → 30501；命中回读。
func TestFortuneGetOne_MissNotFound(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewFortuneRepository(db)
	const date = "2026-11-12"
	if err := repo.UpsertBatch(context.Background(), twelveRows(date, "存")); err != nil {
		t.Fatalf("预置 err = %v", err)
	}
	svc := NewFortuneService(repo, &stubFetcher{})

	hit, err := svc.GetOne(context.Background(), date, "白羊座")
	if err != nil || hit == nil || hit.Constellation != "白羊座" {
		t.Fatalf("GetOne 命中 err=%v row=%v", err, hit)
	}
	miss, err := svc.GetOne(context.Background(), date, "双子座不存在")
	if !errs.Is(err, errs.CodeStarfuyuFortuneNotFound) {
		t.Fatalf("GetOne miss 应为 30501, got %v", err)
	}
	if miss != nil {
		t.Fatalf("GetOne miss 应 nil, got %+v", miss)
	}
}
