package starfuyu

import (
	"context"
	"fmt"

	"github.com/robfig/cron/v3"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"alexGo-cloud/modules/starfuyu/service"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/logger"
)

// cronParser 5 段标准 spec（分 时 日 月 周 + @descriptor）——不 WithSeconds
// （goLucky 的 WithSeconds+5 段自相矛盾，plan 钉标准 parser）。
// 时区由 CronSpec 的 CRON_TZ=Asia/Shanghai 前缀钉死。
var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

// safeJob 任务外壳：panic recover 成 warn 不崩进程；错误只记 warn 不致命
// （DeepSeek 空配/资产缺失走 err 返回，同路径落日志）。
func safeJob(name string, fn func(ctx context.Context)) func() {
	return func() {
		defer func() {
			if r := recover(); r != nil && logger.Log != nil {
				logger.Log.Warn("starfuyu cron job panic", zap.String("job", name), zap.Any("recover", r))
			}
		}()
		// cron 不带 ctx；独立 Background（同系统 ticker 惯例，不受生命周期 ctx 影响）。
		fn(context.Background())
	}
}

// cronLifecycle OnStart 起 cron（两任务读 cfg 表达式），OnStop c.Stop() 等在跑任务收尾。
// 模式照 modules/system/module.go 的 token SweepExpired fx.Invoke 生命周期。
func cronLifecycle(lc fx.Lifecycle, cfg *config.Config, runner service.TaskRunner) {
	var c *cron.Cron
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			c = cron.New(cron.WithParser(cronParser))

			fetchSpec := service.CronSpec(cfg.Starfuyu.Cron.FetchFortune)
			if _, err := c.AddFunc(fetchSpec, safeJob("fetch_fortune", func(ctx context.Context) {
				if cfg.Starfuyu.Deepseek.APIKey == "" && logger.Log != nil {
					logger.Log.Warn("starfuyu fetch_fortune skipped: deepseek api_key empty")
					return
				}
				if err := runner.FetchToday(ctx, false); err != nil && logger.Log != nil {
					logger.Log.Warn("starfuyu fetch_fortune failed", zap.Error(err))
				}
			})); err != nil {
				return fmt.Errorf("注册 fetch_fortune cron %q: %w", fetchSpec, err)
			}

			imgSpec := service.CronSpec(cfg.Starfuyu.Cron.GenerateImages)
			if _, err := c.AddFunc(imgSpec, safeJob("generate_images", func(ctx context.Context) {
				if err := runner.GenerateImagesToday(ctx); err != nil && logger.Log != nil {
					logger.Log.Warn("starfuyu generate_images failed", zap.Error(err))
				}
			})); err != nil {
				return fmt.Errorf("注册 generate_images cron %q: %w", imgSpec, err)
			}

			c.Start()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if c == nil {
				return nil
			}
			// Stop 等在跑任务结束；同时受 OnStop ctx 截断，避免拖死关停。
			select {
			case <-c.Stop().Done():
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})
}
