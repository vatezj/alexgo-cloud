package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"gorm.io/gorm"

	"alexGo-cloud/modules/starfuyu/model"
	"alexGo-cloud/modules/starfuyu/repository"
	"alexGo-cloud/pkg/config"
	errs "alexGo-cloud/pkg/errors"
)

// stubInterpreter SignInterpreter 测试替身：记录调用、返回预置签文/解读或错误。
type stubInterpreter struct {
	contentCalls   int
	interpretCalls int
	content        string
	contentErr     error
	result         *DivinationResult
	interpretErr   error
}

func (s *stubInterpreter) SignContent(_ context.Context, _ int) (string, error) {
	s.contentCalls++
	return s.content, s.contentErr
}

func (s *stubInterpreter) Interpret(_ context.Context, _ *model.BaziInfo, _, _, _ string) (*DivinationResult, error) {
	s.interpretCalls++
	if s.interpretErr != nil {
		return nil, s.interpretErr
	}
	return s.result, nil
}

// stubLimitRepo RequestLimitRepository 测试替身：CheckAndIncrement 固定返回预置 count
// （模拟"当日已计数"，配合真实 LimitService 打满 30503）。
type stubLimitRepo struct{ count int }

func (s stubLimitRepo) CheckAndIncrement(_ context.Context, _ uint64, _, _ string) (int, error) {
	return s.count, nil
}

func (s stubLimitRepo) AdminTodaySummary(_ context.Context, _ string) ([]repository.TodayLimitRow, error) {
	return nil, nil
}

// newDivSvc 组装 DivinationService（真仓储 + sqlite 内存库），返回 concrete 便于注入 rng。
// limits 传 nil 时用缺省 LimitService（cfg 零值 → divination_draw 日限 10）。
func newDivSvc(t *testing.T, interp SignInterpreter, limits LimitService) (*divinationService, *gorm.DB) {
	t.Helper()
	db := newTask5DB(t)
	if limits == nil {
		limits = NewLimitService(repository.NewRequestLimitRepository(db), &config.Config{})
	}
	svc := NewDivinationService(
		repository.NewSignCacheRepository(db),
		repository.NewDivinationRepository(db),
		repository.NewBaziRepository(db),
		limits,
		interp,
	).(*divinationService)
	return svc, db
}

// TestGetSign_SecondCallHitsCache brief 用例：stub contentCalls==1；GetSign(42) 两次
// → 第二次 interp 不再调、两次内容相同（决定 15：首访落缓存后直读，零 LLM）。
func TestGetSign_SecondCallHitsCache(t *testing.T) {
	stub := &stubInterpreter{content: "云开见月明，前路自光明。"}
	svc, _ := newDivSvc(t, stub, nil)
	ctx := context.Background()

	first, err := svc.GetSign(ctx, 42)
	if err != nil {
		t.Fatalf("首次 GetSign(42) err = %v, want nil", err)
	}
	second, err := svc.GetSign(ctx, 42)
	if err != nil {
		t.Fatalf("二次 GetSign(42) err = %v, want nil", err)
	}
	if stub.contentCalls != 1 {
		t.Fatalf("SignContent 调用次数 = %d, want 1（二次命中缓存）", stub.contentCalls)
	}
	if first != stub.content || second != first {
		t.Fatalf("两次内容应一致且等于生成值: first=%q second=%q stub=%q", first, second, stub.content)
	}
}

// TestGetSign_InvalidNumber brief 用例：0/101 → 10001，且不打 interpreter。
func TestGetSign_InvalidNumber(t *testing.T) {
	stub := &stubInterpreter{content: "不该被调"}
	svc, _ := newDivSvc(t, stub, nil)
	ctx := context.Background()

	for _, n := range []int{0, 101} {
		got, err := svc.GetSign(ctx, n)
		if !errs.Is(err, errs.CodeParamInvalid) {
			t.Fatalf("GetSign(%d) 应 10001, got %v", n, err)
		}
		if got != "" {
			t.Fatalf("GetSign(%d) 应空串, got %q", n, got)
		}
	}
	if stub.contentCalls != 0 {
		t.Fatalf("越界签号不得调 SignContent, calls = %d", stub.contentCalls)
	}
}

// TestGetSign_InterpreterFallbackStillCaches interpreter 失败 → 按 number 模板兜底，
// 且兜底也写缓存：二次调用直读、contentCalls 不再涨（同号稳定）。
func TestGetSign_InterpreterFallbackStillCaches(t *testing.T) {
	stub := &stubInterpreter{contentErr: errors.New("deepseek down")}
	svc, _ := newDivSvc(t, stub, nil)
	ctx := context.Background()

	first, err := svc.GetSign(ctx, 9)
	if err != nil {
		t.Fatalf("interp 失败应兜底不报错, got err = %v", err)
	}
	if first == "" {
		t.Fatal("模板兜底不得空串")
	}
	if !strings.Contains(first, "第9签") {
		t.Fatalf("兜底应为按 number 变体, got %q", first)
	}
	second, err := svc.GetSign(ctx, 9)
	if err != nil {
		t.Fatalf("二次 GetSign err = %v, want nil", err)
	}
	if second != first {
		t.Fatalf("兜底也须二次稳定: second=%q != first=%q", second, first)
	}
	if stub.contentCalls != 1 {
		t.Fatalf("兜底写缓存后 SignContent calls = %d, want 1", stub.contentCalls)
	}
}

// TestDraw_UsesCacheAndLimit brief 用例：先 GetSign(7) 灌缓存；注入 rng 恒抽 7 号
// → Draw 不得二次调 SignContent；限流打满 → 第一次 Draw 30503。
func TestDraw_UsesCacheAndLimit(t *testing.T) {
	stub := &stubInterpreter{content: "第七签：积善之家必有余庆。"}
	svc, _ := newDivSvc(t, stub, nil)
	ctx := context.Background()

	if _, err := svc.GetSign(ctx, 7); err != nil {
		t.Fatalf("灌缓存 GetSign(7) err = %v, want nil", err)
	}
	if stub.contentCalls != 1 {
		t.Fatalf("灌缓存后 SignContent calls = %d, want 1", stub.contentCalls)
	}

	svc.rng = func(int) int { return 6 } // rng(100)+1 → 恒 7 号
	rec, err := svc.Draw(ctx, 3, "问事业", "")
	if err != nil {
		t.Fatalf("Draw err = %v, want nil", err)
	}
	if rec.SignNumber != 7 {
		t.Fatalf("注入 rng 后签号 = %d, want 7", rec.SignNumber)
	}
	if stub.contentCalls != 1 {
		t.Fatalf("Draw 命中缓存不得再调 SignContent, calls = %d", stub.contentCalls)
	}

	// 限流：cfg 零值（DivinationDrawDaily=0 → Task 5 缺省 10），桩仓储返回当日已计 11
	// （>10）→ 第一次 Draw 即 30503，且不落记录。
	blocked, _ := newDivSvc(t, &stubInterpreter{content: "x"},
		NewLimitService(stubLimitRepo{count: 11}, &config.Config{}))
	blocked.rng = func(int) int { return 6 }
	if _, err := blocked.Draw(ctx, 3, "问事业", ""); !errs.Is(err, errs.CodeStarfuyuLimitExceeded) {
		t.Fatalf("限流打满首次 Draw 应 30503, got %v", err)
	}
	list, total, hErr := blocked.History(ctx, 3, 1, 10)
	if hErr != nil || total != 0 || len(list) != 0 {
		t.Fatalf("30503 不得落记录: total=%d len=%d err=%v", total, len(list), hErr)
	}
}

// TestDraw_PersistsRecord brief 用例：Draw 落一条记录，History 能查到 1 条，
// sign_number ∈ [1,100]。无八字走降级路径：解读字段非空模板、不调 Interpret。
func TestDraw_PersistsRecord(t *testing.T) {
	stub := &stubInterpreter{content: "宝剑锋从磨砺出，梅花香自苦寒来。"}
	svc, _ := newDivSvc(t, stub, nil)
	ctx := context.Background()
	svc.rng = func(int) int { return 41 } // → 42 号

	rec, err := svc.Draw(ctx, 5, "问感情", "爱情")
	if err != nil {
		t.Fatalf("Draw err = %v, want nil", err)
	}
	if rec.SignNumber < signMin || rec.SignNumber > signMax {
		t.Fatalf("sign_number = %d, want ∈ [%d,%d]", rec.SignNumber, signMin, signMax)
	}
	if rec.SignContent != stub.content {
		t.Fatalf("落库签文应取缓存值, got %q", rec.SignContent)
	}
	if rec.Interpretation == "" || rec.Advice == "" || rec.LuckyDirection == "" || rec.LuckyColor == "" {
		t.Fatalf("无八字降级解读字段须非空模板: %+v", rec)
	}
	if rec.Category != "爱情" {
		t.Fatalf("降级类别应取入参 category, got %q", rec.Category)
	}
	if stub.interpretCalls != 0 {
		t.Fatalf("无八字不得调 Interpret, calls = %d", stub.interpretCalls)
	}

	list, total, err := svc.History(ctx, 5, 1, 10)
	if err != nil {
		t.Fatalf("History err = %v, want nil", err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("History total=%d len=%d, want 1/1", total, len(list))
	}
	if list[0].SignNumber < signMin || list[0].SignNumber > signMax {
		t.Fatalf("History sign_number = %d, want ∈ [1,100]", list[0].SignNumber)
	}
}

// TestDraw_InterpretsWhenBaziExists 有八字 → 调 Interpret，解读字段取返回值并挂 bazi_id。
func TestDraw_InterpretsWhenBaziExists(t *testing.T) {
	stub := &stubInterpreter{
		content: "天时地利人和，万事皆可成。",
		result: &DivinationResult{
			Interpretation: "此签大吉，事业可成。",
			LuckyDirection: "东方",
			LuckyColor:     "红色",
			Advice:         "把握时机，顺势而为。",
			Category:       "事业",
		},
	}
	svc, db := newDivSvc(t, stub, nil)
	ctx := context.Background()
	if _, err := repository.NewBaziRepository(db).Upsert(ctx, 6, &model.Bazi{
		BirthDate: "1990-05-05", BirthTime: "12:30", BaziInfo: *fixedBaziInfo(),
	}); err != nil {
		t.Fatalf("预置八字 err = %v", err)
	}
	svc.rng = func(int) int { return 41 } // → 42 号

	rec, err := svc.Draw(ctx, 6, "问事业", "")
	if err != nil {
		t.Fatalf("Draw err = %v, want nil", err)
	}
	if stub.interpretCalls != 1 {
		t.Fatalf("有八字应调 Interpret 一次, calls = %d", stub.interpretCalls)
	}
	if rec.Interpretation != "此签大吉，事业可成。" || rec.Category != "事业" ||
		rec.LuckyDirection != "东方" || rec.LuckyColor != "红色" {
		t.Fatalf("解读字段未取 interp 返回值: %+v", rec)
	}
	if rec.BaziID == nil || *rec.BaziID == 0 {
		t.Fatalf("有八字应挂 bazi_id, got %v", rec.BaziID)
	}
}

// TestDraw_InterpretFailureDegrades 解读失败不 fail 整个 draw：签文照落库，
// 解读字段填短模板（非空）——controller ruling。
func TestDraw_InterpretFailureDegrades(t *testing.T) {
	stub := &stubInterpreter{
		content:      "山重水复疑无路，柳暗花明又一村。",
		interpretErr: errors.New("llm 500"),
	}
	svc, db := newDivSvc(t, stub, nil)
	ctx := context.Background()
	if _, err := repository.NewBaziRepository(db).Upsert(ctx, 7, &model.Bazi{
		BirthDate: "1990-05-05", BaziInfo: *fixedBaziInfo(),
	}); err != nil {
		t.Fatalf("预置八字 err = %v", err)
	}
	svc.rng = func(int) int { return 41 } // → 42 号

	rec, err := svc.Draw(ctx, 7, "问健康", "健康")
	if err != nil {
		t.Fatalf("解读失败不得 fail draw, got err = %v", err)
	}
	if rec.SignContent != stub.content {
		t.Fatalf("签文照常落库, got %q", rec.SignContent)
	}
	if rec.Interpretation == "" || rec.Advice == "" || rec.LuckyDirection == "" || rec.LuckyColor == "" {
		t.Fatalf("解读失败须填模板字段: %+v", rec)
	}
	if rec.Category != "健康" {
		t.Fatalf("降级类别应取入参 category, got %q", rec.Category)
	}
	list, total, err := svc.History(ctx, 7, 1, 10)
	if err != nil || total != 1 || len(list) != 1 {
		t.Fatalf("降级也须落记录: total=%d len=%d err=%v", total, len(list), err)
	}
}
