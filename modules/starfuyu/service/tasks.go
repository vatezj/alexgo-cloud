package service

import (
	"context"
	"fmt"
)

// TaskRunner 定时任务编排（Task 9）：日期一律取 TodaySH()（上海日历日，
// 不跟随进程 TZ——生产容器是 UTC）。
type TaskRunner interface {
	// FetchToday 抓取补缺当日运势（忽略 skipped：cron 不关心）。
	FetchToday(ctx context.Context, force bool) error
	// GenerateImagesToday 生成当日 12 星座运势图。
	GenerateImagesToday(ctx context.Context) error
	// ExecuteAll 先 Fetch 后 Generate（调用序有测试钉死）。
	ExecuteAll(ctx context.Context, force bool) error
}

type taskRunner struct {
	fortunes FortuneService
	images   ImageService
}

func NewTaskRunner(fortunes FortuneService, images ImageService) TaskRunner {
	return &taskRunner{fortunes: fortunes, images: images}
}

func (r *taskRunner) FetchToday(ctx context.Context, force bool) error {
	_, err := r.fortunes.Fetch(ctx, TodaySH(), force)
	return err
}

func (r *taskRunner) GenerateImagesToday(ctx context.Context) error {
	_, err := r.images.GenerateAll(ctx, TodaySH())
	return err
}

func (r *taskRunner) ExecuteAll(ctx context.Context, force bool) error {
	if err := r.FetchToday(ctx, force); err != nil {
		return fmt.Errorf("抓取今日运势: %w", err)
	}
	if err := r.GenerateImagesToday(ctx); err != nil {
		return fmt.Errorf("生成今日运势图: %w", err)
	}
	return nil
}

// CronSpec 给 cron 表达式钉 CRON_TZ=Asia/Shanghai 前缀（spec 决定 10：
// 5 段标准表达式 + 时区前缀，不用 WithSeconds）。
func CronSpec(expr string) string {
	return "CRON_TZ=Asia/Shanghai " + expr
}
