package service

import (
	"context"
	"fmt"
)

// TaskRunner 任务编排（Task 9 cron + Task 11 admin 手动触发）：
// Today* 方法钉 TodaySH()（上海日历日，不跟随进程 TZ——生产容器是 UTC），供 cron；
// *Date 方法收显式 date，供 admin 手动触发（spec §4.2 body date 缺省今日由 controller 缺省）。
type TaskRunner interface {
	// FetchToday 抓取补缺当日运势（忽略 skipped：cron 不关心）。
	FetchToday(ctx context.Context, force bool) error
	// GenerateImagesToday 生成当日 12 星座运势图。
	GenerateImagesToday(ctx context.Context) error
	// ExecuteAll 先 Fetch 后 Generate（调用序有测试钉死）。
	ExecuteAll(ctx context.Context, force bool) error
	// FetchDate 抓取指定日（admin 手动触发；force 语义同 FortuneService.Fetch：默认只补缺）。
	FetchDate(ctx context.Context, date string, force bool) error
	// GenerateImagesDate 生成指定日 12 星座运势图。
	GenerateImagesDate(ctx context.Context, date string) error
	// ExecuteAllDate 指定日一条龙：先 Fetch 后 Generate（调用序同 ExecuteAll）。
	ExecuteAllDate(ctx context.Context, date string, force bool) error
}

type taskRunner struct {
	fortunes FortuneService
	images   ImageService
}

func NewTaskRunner(fortunes FortuneService, images ImageService) TaskRunner {
	return &taskRunner{fortunes: fortunes, images: images}
}

func (r *taskRunner) FetchDate(ctx context.Context, date string, force bool) error {
	_, err := r.fortunes.Fetch(ctx, date, force)
	return err
}

func (r *taskRunner) FetchToday(ctx context.Context, force bool) error {
	return r.FetchDate(ctx, TodaySH(), force)
}

func (r *taskRunner) GenerateImagesDate(ctx context.Context, date string) error {
	_, err := r.images.GenerateAll(ctx, date)
	return err
}

func (r *taskRunner) GenerateImagesToday(ctx context.Context) error {
	return r.GenerateImagesDate(ctx, TodaySH())
}

func (r *taskRunner) ExecuteAllDate(ctx context.Context, date string, force bool) error {
	if err := r.FetchDate(ctx, date, force); err != nil {
		return fmt.Errorf("抓取运势: %w", err)
	}
	if err := r.GenerateImagesDate(ctx, date); err != nil {
		return fmt.Errorf("生成运势图: %w", err)
	}
	return nil
}

func (r *taskRunner) ExecuteAll(ctx context.Context, force bool) error {
	return r.ExecuteAllDate(ctx, TodaySH(), force)
}
