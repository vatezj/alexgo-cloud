package service

import (
	"context"
	"testing"

	"alexGo-cloud/modules/starfuyu/model"
)

// stubFortune 记录 Fetch 入参（date/force），其余读方法不参与任务编排测试。
type stubFortune struct {
	log    *[]string // 调用序共享记录
	dates  []string
	forces []bool
	err    error
}

func (s *stubFortune) Fetch(_ context.Context, date string, force bool) (bool, error) {
	if s.log != nil {
		*s.log = append(*s.log, "fetch:"+date)
	}
	s.dates = append(s.dates, date)
	s.forces = append(s.forces, force)
	return false, s.err
}

func (s *stubFortune) GetByDate(context.Context, string) ([]*model.Fortune, error) {
	return nil, nil
}

func (s *stubFortune) GetOne(context.Context, string, string) (*model.Fortune, error) {
	return nil, nil
}

func (s *stubFortune) ListAdmin(context.Context, string, string, int, int) ([]*model.Fortune, int64, error) {
	return nil, 0, nil
}

// stubImages 记录 GenerateAll 入参（date）。
type stubImages struct {
	log   *[]string
	dates []string
	ok    int
	err   error
}

func (s *stubImages) Ensure(context.Context, string, string) (string, error) {
	return "", nil
}

func (s *stubImages) GenerateAll(_ context.Context, date string) (int, error) {
	if s.log != nil {
		*s.log = append(*s.log, "images:"+date)
	}
	s.dates = append(s.dates, date)
	return s.ok, s.err
}

// TestExecuteAll_CallsFetchThenImages 调用序钉死：先 Fetch 后 Generate，日期同为 TodaySH()。
func TestExecuteAll_CallsFetchThenImages(t *testing.T) {
	var order []string
	f := &stubFortune{log: &order}
	im := &stubImages{log: &order, ok: 12}

	runner := NewTaskRunner(f, im)
	if err := runner.ExecuteAll(context.Background(), false); err != nil {
		t.Fatalf("ExecuteAll: %v", err)
	}

	if len(order) != 2 || order[0] != "fetch:"+TodaySH() || order[1] != "images:"+TodaySH() {
		t.Fatalf("调用序 = %v, want [fetch:%s images:%s]", order, TodaySH(), TodaySH())
	}
}

// TestCronSpec_PrefixedWithShanghai cronSpec 表达式钉 CRON_TZ=Asia/Shanghai 前缀（spec 决定）。
func TestCronSpec_PrefixedWithShanghai(t *testing.T) {
	got := CronSpec("10 0 * * *")
	want := "CRON_TZ=Asia/Shanghai 10 0 * * *"
	if got != want {
		t.Fatalf("CronSpec = %q, want %q", got, want)
	}
}

// TestFetchToday_UsesTodaySH FetchToday 传给 Fetch 的 date 必须是 TodaySH()（钉上海日历日，
// 不跟随进程 TZ——容器生产是 UTC）。
func TestFetchToday_UsesTodaySH(t *testing.T) {
	t.Setenv("TZ", "UTC")

	f := &stubFortune{}
	runner := NewTaskRunner(f, &stubImages{})

	if err := runner.FetchToday(context.Background(), true); err != nil {
		t.Fatalf("FetchToday: %v", err)
	}
	if len(f.dates) != 1 {
		t.Fatalf("Fetch 调用次数 = %d, want 1", len(f.dates))
	}
	if f.dates[0] != TodaySH() {
		t.Fatalf("Fetch date = %q, want TodaySH()=%q（TZ=UTC 下仍须上海日历日）", f.dates[0], TodaySH())
	}
	if !f.forces[0] {
		t.Fatal("force 未透传：want true")
	}
}

// TestFetchDate_Passthrough Task 11：FetchDate 透传 date+force 到 FortuneService.Fetch
// （admin 手动触发的日期/强制语义；FetchToday 委托本方法，既有用例继续钉 TodaySH）。
func TestFetchDate_Passthrough(t *testing.T) {
	f := &stubFortune{}
	runner := NewTaskRunner(f, &stubImages{})

	if err := runner.FetchDate(context.Background(), "2026-11-01", true); err != nil {
		t.Fatalf("FetchDate: %v", err)
	}
	if len(f.dates) != 1 || f.dates[0] != "2026-11-01" {
		t.Fatalf("Fetch date = %v want [2026-11-01]", f.dates)
	}
	if len(f.forces) != 1 || !f.forces[0] {
		t.Fatalf("Fetch force = %v want [true]（force 未透传）", f.forces)
	}
}

// TestGenerateImagesDate_Passthrough GenerateImagesDate 透传 date 到 ImageService.GenerateAll。
func TestGenerateImagesDate_Passthrough(t *testing.T) {
	im := &stubImages{}
	runner := NewTaskRunner(&stubFortune{}, im)

	if err := runner.GenerateImagesDate(context.Background(), "2026-11-01"); err != nil {
		t.Fatalf("GenerateImagesDate: %v", err)
	}
	if len(im.dates) != 1 || im.dates[0] != "2026-11-01" {
		t.Fatalf("GenerateAll date = %v want [2026-11-01]", im.dates)
	}
}

// TestExecuteAllDate_Order 调用序钉死（同 ExecuteAll）：先 Fetch 后 Generate，date/force 透传。
func TestExecuteAllDate_Order(t *testing.T) {
	var order []string
	f := &stubFortune{log: &order}
	im := &stubImages{log: &order, ok: 12}
	runner := NewTaskRunner(f, im)

	if err := runner.ExecuteAllDate(context.Background(), "2026-11-01", true); err != nil {
		t.Fatalf("ExecuteAllDate: %v", err)
	}
	if len(order) != 2 || order[0] != "fetch:2026-11-01" || order[1] != "images:2026-11-01" {
		t.Fatalf("调用序 = %v want [fetch:2026-11-01 images:2026-11-01]", order)
	}
	if !f.forces[0] {
		t.Fatal("force 未透传：want true")
	}
}
