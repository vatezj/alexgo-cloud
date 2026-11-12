package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"gorm.io/gorm"

	"alexGo-cloud/modules/starfuyu/model"
	"alexGo-cloud/modules/starfuyu/repository"
	"alexGo-cloud/pkg/config"
	errs "alexGo-cloud/pkg/errors"
)

// stubBaziCalculator BaziCalculator 测试替身：记录调用、返回预置 BaziInfo/错误。
type stubBaziCalculator struct {
	calls int
	info  *model.BaziInfo
	err   error
}

func (s *stubBaziCalculator) Calculate(_ context.Context, _, _, _ string) (*model.BaziInfo, error) {
	s.calls++
	return s.info, s.err
}

// fixedBaziInfo 预置八字（四柱可断言，经 JSON valuer 落列后回读）。
func fixedBaziInfo() *model.BaziInfo {
	return &model.BaziInfo{
		YearGan: "甲", YearZhi: "午",
		MonthGan: "己", MonthZhi: "巳",
		DayGan: "丙", DayZhi: "寅",
		HourGan: "戊", HourZhi: "子",
		Wuxing: model.WuxingInfo{
			YearWuxing: "木", MonthWuxing: "土", DayWuxing: "火", HourWuxing: "土",
			TotalWuxing: map[string]int{"金": 2, "木": 1, "水": 1, "火": 2, "土": 2},
		},
		SolarTerm: "立夏", LunarDate: "四月初五", Zodiac: "马", Constellation: "金牛座",
	}
}

// countBaziRows 按 member 数行（幂等覆盖断言用）。
func countBaziRows(t *testing.T, db *gorm.DB, memberUserID uint64) int {
	t.Helper()
	var n int64
	if err := db.Model(&model.Bazi{}).Where("member_user_id = ?", memberUserID).Count(&n).Error; err != nil {
		t.Fatalf("count bazi rows: %v", err)
	}
	return int(n)
}

// TestBazi_CalculateUpsertsAndGetMyMiss30504 brief 用例：空库 GetMy → 30504；
// Calculate（限流 + stub 计算 + Upsert）后 GetMy 命中且 BaziInfo 完整回读。
func TestBazi_CalculateUpsertsAndGetMyMiss30504(t *testing.T) {
	db := newTask5DB(t)
	bazis := repository.NewBaziRepository(db)
	limits := NewLimitService(repository.NewRequestLimitRepository(db), &config.Config{})
	stub := &stubBaziCalculator{info: fixedBaziInfo()}
	svc := NewBaziService(bazis, limits, stub)
	ctx := context.Background()

	miss, err := svc.GetMy(ctx, 7)
	if !errs.Is(err, errs.CodeStarfuyuBaziNotFound) {
		t.Fatalf("空库 GetMy 应 30504, got %v", err)
	}
	if miss != nil {
		t.Fatalf("空库 GetMy 应 nil, got %+v", miss)
	}

	got, err := svc.Calculate(ctx, 7, BaziReq{
		BirthDate: "1990-05-05", BirthTime: "12:30", BirthPlace: "上海",
	})
	if err != nil {
		t.Fatalf("Calculate err = %v, want nil", err)
	}
	if stub.calls != 1 {
		t.Fatalf("stub 调用次数 = %d, want 1", stub.calls)
	}
	if got.BirthDate != "1990-05-05" || got.BirthTime != "12:30" || got.BirthPlace != "上海" {
		t.Fatalf("落库生辰列不对: %+v", got)
	}
	if got.BaziInfo.YearGan != "甲" || got.BaziInfo.Wuxing.TotalWuxing["火"] != 2 {
		t.Fatalf("BaziInfo JSON 未完整回读: %+v", got.BaziInfo)
	}

	hit, err := svc.GetMy(ctx, 7)
	if err != nil || hit == nil {
		t.Fatalf("Calculate 后 GetMy err = %v, row = %v", err, hit)
	}
	if hit.BaziInfo.Zodiac != "马" || hit.BaziInfo.Constellation != "金牛座" {
		t.Fatalf("GetMy 回读 BaziInfo 不一致: %+v", hit.BaziInfo)
	}
}

// TestBazi_CalculateRejectsBadBirthDate birth_date 必填且须 2006-01-02（10002/10001）；
// birth_time 可空但非空须 15:04(:05)；校验在限流之前——坏请求不消耗当日额度。
func TestBazi_CalculateRejectsBadBirthDate(t *testing.T) {
	db := newTask5DB(t)
	bazis := repository.NewBaziRepository(db)
	limits := NewLimitService(repository.NewRequestLimitRepository(db), &config.Config{})
	stub := &stubBaziCalculator{info: fixedBaziInfo()}
	svc := NewBaziService(bazis, limits, stub)
	ctx := context.Background()

	if _, err := svc.Calculate(ctx, 8, BaziReq{}); !errs.Is(err, errs.CodeParamMissing) {
		t.Fatalf("空 birth_date 应 10002, got %v", err)
	}
	if _, err := svc.Calculate(ctx, 8, BaziReq{BirthDate: "1990-5-5"}); !errs.Is(err, errs.CodeParamInvalid) {
		t.Fatalf("非 2006-01-02 格式应 10001, got %v", err)
	}
	if _, err := svc.Calculate(ctx, 8, BaziReq{BirthDate: "1990-05-05", BirthTime: "25:99"}); !errs.Is(err, errs.CodeParamInvalid) {
		t.Fatalf("非法 birth_time 应 10001, got %v", err)
	}
	if stub.calls != 0 {
		t.Fatalf("校验失败不得调计算器, calls = %d", stub.calls)
	}
	// 坏请求不占额度：修正后仍可用满缺省 5 次。
	if _, err := svc.Calculate(ctx, 8, BaziReq{BirthDate: "1990-05-05"}); err != nil {
		t.Fatalf("修正后 Calculate err = %v, want nil", err)
	}
}

// TestBazi_UpdateMySameFlow UpdateMy 同 Calculate：限流 + 计算 + 覆盖保存（幂等单行）。
func TestBazi_UpdateMySameFlow(t *testing.T) {
	db := newTask5DB(t)
	bazis := repository.NewBaziRepository(db)
	limits := NewLimitService(repository.NewRequestLimitRepository(db), &config.Config{})
	stub := &stubBaziCalculator{info: fixedBaziInfo()}
	svc := NewBaziService(bazis, limits, stub)
	ctx := context.Background()

	if _, err := svc.UpdateMy(ctx, 7, BaziReq{BirthDate: "1990-05-05", BirthTime: "12:30"}); err != nil {
		t.Fatalf("UpdateMy err = %v, want nil", err)
	}
	stub.info = fixedBaziInfo()
	stub.info.DayGan = "庚" // 改版八字
	updated, err := svc.UpdateMy(ctx, 7, BaziReq{BirthDate: "1991-08-08", BirthTime: "08:00", BirthPlace: "北京"})
	if err != nil {
		t.Fatalf("二次 UpdateMy err = %v, want nil", err)
	}
	if updated.BirthDate != "1991-08-08" || updated.BaziInfo.DayGan != "庚" {
		t.Fatalf("UpdateMy 应覆盖旧值: %+v", updated)
	}
	if n := countBaziRows(t, db, 7); n != 1 {
		t.Fatalf("幂等覆盖后行数 = %d, want 1", n)
	}
	if stub.calls != 2 {
		t.Fatalf("stub 调用次数 = %d, want 2", stub.calls)
	}
}

// TestBaziCalculator_DeepSeekEmptyKeyInternal controller ruling：空 api_key →
// errs.CodeInternal（50000），不占 305xx，且不得 panic（含 nil cfg）。
func TestBaziCalculator_DeepSeekEmptyKeyInternal(t *testing.T) {
	calc := NewDeepSeekBaziCalculator(&config.Config{})
	info, err := calc.Calculate(context.Background(), "1990-05-05", "12:30", "上海")
	if !errs.Is(err, errs.CodeInternal) {
		t.Fatalf("空 api_key 应 50000, got %v", err)
	}
	if info != nil {
		t.Fatalf("空 api_key 应 nil 结果, got %+v", info)
	}
	// nil cfg 同样走内部错误路径，不 panic。
	calc = NewDeepSeekBaziCalculator(nil)
	if _, err = calc.Calculate(context.Background(), "1990-05-05", "", ""); !errs.Is(err, errs.CodeInternal) {
		t.Fatalf("nil cfg 应 50000, got %v", err)
	}
}

// TestBaziCalculator_APIFailureInternal API 失败（非 200）→ 50000 而非 305xx。
func TestBaziCalculator_APIFailureInternal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad","type":"invalid_request_error"}}`))
	}))
	defer srv.Close()

	cfg := &config.Config{}
	cfg.Starfuyu.Deepseek.APIKey = "k"
	cfg.Starfuyu.Deepseek.BaseURL = srv.URL
	calc := NewDeepSeekBaziCalculator(cfg)
	if _, err := calc.Calculate(context.Background(), "1990-05-05", "12:30", "上海"); !errs.Is(err, errs.CodeInternal) {
		t.Fatalf("API 失败应 50000, got %v", err)
	}
}
