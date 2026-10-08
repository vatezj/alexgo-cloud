package service

import (
	"context"
	"encoding/json"
	"fmt"

	"alexGo-cloud/modules/order/model"
	"alexGo-cloud/modules/order/repository"
	systemmodel "alexGo-cloud/modules/system/model"
	systemservice "alexGo-cloud/modules/system/service"
	"alexGo-cloud/pkg/tenant"
	"gorm.io/gorm"
)

// OrderService 定义订单域的应用服务接口。
//
// 设计说明：
// - 对外暴露接口而不是结构体，便于未来替换实现（例如拆成 order 微服务后提供 gRPC client 实现同一接口）。
// - 在单体模式下，跨模块调用（order → system.UserService）是本地函数调用，零 RPC 开销。
type OrderService interface {
	Create(ctx context.Context, entity *model.Order) error
	List(ctx context.Context) ([]*model.Order, error)
}

// orderService 是 OrderService 的默认实现。
//
// db：
// - 用于事务管理（Transactional Outbox Pattern：业务写入 + outbox 写入同一事务）
//
// repo：
// - 负责订单表的 CRUD（GORM）
//
// userSvc：
// - 跨模块调用示例：创建订单前校验用户存在（接口依赖，未来可切换 gRPC client）
type orderService struct {
	db      *gorm.DB
	repo    repository.OrderRepository
	userSvc systemservice.UserService
}

// NewOrderService 通过 Fx 注入依赖构造 OrderService。
func NewOrderService(db *gorm.DB, repo repository.OrderRepository, userSvc systemservice.UserService) OrderService {
	return &orderService{db: db, repo: repo, userSvc: userSvc}
}

// Create 创建订单，并在同一事务内写入 outbox_events（pending）。
//
// Transactional Outbox Pattern：
// - tx.Create(order) 成功后，再 tx.Create(outbox_event)
// - 如果任意一步失败，事务回滚：不会出现“订单存在但事件缺失”或“事件存在但订单不存在”
//
// 事件约定：
// - EventType: "order.created"
// - AggregateID: "order:<id>"
// - Payload: JSON（示例包含 order_id/user_id）
//
// 多租户：
// - TenantID 从 context 读取（由 TenantMiddleware 注入 X-Tenant-ID）
func (s *orderService) Create(ctx context.Context, entity *model.Order) error {
	if entity == nil {
		return nil
	}
	if s.db == nil {
		return fmt.Errorf("db is nil")
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1) 跨模块校验：用户是否存在（单体本地调用；微服务模式可能被 Decorate 为 gRPC 调用）。
		_, err := s.userSvc.GetUserByID(ctx, entity.UserID)
		if err != nil {
			return err
		}
		// 2) 写入订单表。
		if err := tx.Create(entity).Error; err != nil {
			return err
		}

		// 3) 写入 outbox（pending）。
		// Relay 会异步把它发布到 MQ，并更新状态为 published/failed。
		payload, _ := json.Marshal(map[string]any{
			"order_id": entity.ID,
			"user_id":  entity.UserID,
		})
		out := systemmodel.OutboxEvent{
			AggregateID: fmt.Sprintf("order:%d", entity.ID),
			EventType:   "order.created",
			Payload:     payload,
			Status:      "pending",
			TenantID:    tenant.TenantIDFromContext(ctx),
		}
		return tx.Create(&out).Error
	})
}

// List 返回订单列表（简单示例：全量查询）。
//
// 生产建议：
// - 增加分页（limit/offset 或游标）
// - 增加租户过滤（WHERE tenant_id = ?）
// - 增加权限过滤（基于 claims 的角色/数据范围）
func (s *orderService) List(ctx context.Context) ([]*model.Order, error) {
	return s.repo.List(ctx)
}
