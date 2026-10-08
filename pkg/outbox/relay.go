package outbox

import (
	"context"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"

	systemmodel "alexGo-cloud/modules/system/model"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/logger"
	"alexGo-cloud/pkg/mq"
)

// Relay 是 Transactional Outbox Pattern 的后台转发器（Relay/Dispatcher）。
//
// 目标：
// - 业务事务内写 outbox_events（pending）与业务表更新处于同一事务，保证原子性（不会“下单成功但事件丢失”）。
// - Relay 异步轮询 pending 事件并发布到消息系统（这里是 NATS JetStream），发布成功后标记 published。
//
// 并发策略：
// - 使用 SELECT ... FOR UPDATE SKIP LOCKED 抢占任务，支持多实例并行跑 Relay，避免重复处理同一行。
//
// 可靠性说明：
// - 本实现把状态改为 published/failed。
// - 生产可进一步增强：失败重试次数、指数退避、死信队列、按 tenant 分片处理等。
type Relay struct {
	db     *gorm.DB
	cfg    *config.Config
	broker mq.Broker
}

// NewRelay 构造 Relay。
//
// broker 为 nil 代表消息系统未启用（mq.nats.enabled=false），Relay 会直接 no-op（不处理 outbox）。
func NewRelay(db *gorm.DB, cfg *config.Config, broker mq.Broker) *Relay {
	return &Relay{db: db, cfg: cfg, broker: broker}
}

// RelayParams 注入 Lifecycle 与 Relay 本体，便于在启动阶段挂钩后台 goroutine。
type RelayParams struct {
	fx.In

	LC    fx.Lifecycle
	Relay *Relay
}

// StartRelay 把 Relay 作为后台任务启动（在 Fx 生命周期 OnStart 时起 goroutine）。
//
// 注意：
// - 是否启用由 outbox.enabled 控制。
// - 该 goroutine 会随着进程退出而结束（ctx.Done）。
func StartRelay(p RelayParams) {
	p.LC.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if p.Relay == nil || p.Relay.cfg == nil || !p.Relay.cfg.Outbox.Enabled {
				return nil
			}
			go p.Relay.run(ctx)
			return nil
		},
	})
}

// run 为轮询主循环：按 interval 触发批处理。
func (r *Relay) run(ctx context.Context) {
	interval := 5 * time.Second
	if r.cfg != nil && r.cfg.Outbox.IntervalSecond > 0 {
		interval = time.Duration(r.cfg.Outbox.IntervalSecond) * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	batch := 100
	if r.cfg != nil && r.cfg.Outbox.BatchSize > 0 {
		batch = r.cfg.Outbox.BatchSize
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.processPending(ctx, batch); err != nil && logger.Log != nil {
				logger.Log.Error("outbox processPending failed", zap.Error(err))
			}
		}
	}
}

// publishEvents 逐条发布事件并通过 mark 回调记录状态（published / failed）。
//
// 抽出为纯函数的目的：事务与 FOR UPDATE SKIP LOCKED 抓取依赖 MySQL，
// 无法在单测中执行；发布语义（成功标记、失败标记、错误传播）在这里独立验证。
// mark 返回错误时立即中止（processPending 中即为事务内 UPDATE 失败）。
func publishEvents(ctx context.Context, broker mq.Broker, events []systemmodel.OutboxEvent,
	mark func(id uint64, status string, publishedAt *time.Time) error) error {
	for _, e := range events {
		payload := []byte(e.Payload)
		if err := broker.Publish(ctx, e.EventType, payload); err != nil {
			if lerr := mark(e.ID, "failed", nil); lerr != nil {
				return lerr
			}
			if logger.Log != nil {
				logger.Log.Error("outbox publish failed", zap.Uint64("id", e.ID), zap.Error(err))
			}
			continue
		}
		now := time.Now()
		if merr := mark(e.ID, "published", &now); merr != nil {
			return merr
		}
	}
	return nil
}

// processPending 在单个事务内抓取并“占用”一批 pending 事件，然后逐条发布并更新状态。
//
// 这里选择“事务内抓取 + 发布 + 更新”是为了实现 SKIP LOCKED 的互斥效果；
// 如果发布耗时非常高，可以改造为：
// - 事务内仅将状态从 pending 改为 processing（并记录 worker_id），提交后再发布；
// - 发布成功再将 processing → published（失败记录 reason 并重试）。
func (r *Relay) processPending(ctx context.Context, limit int) error {
	if r == nil || r.db == nil || r.broker == nil {
		return nil
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var events []systemmodel.OutboxEvent
		if err := tx.Raw(
			"SELECT * FROM outbox_events WHERE status = ? ORDER BY id LIMIT ? FOR UPDATE SKIP LOCKED",
			"pending",
			limit,
		).Scan(&events).Error; err != nil {
			return err
		}

		return publishEvents(ctx, r.broker, events, func(id uint64, status string, publishedAt *time.Time) error {
			updates := map[string]any{"status": status}
			if publishedAt != nil {
				updates["published_at"] = publishedAt
			}
			return tx.Model(&systemmodel.OutboxEvent{}).Where("id = ?", id).Updates(updates).Error
		})
	})
}
