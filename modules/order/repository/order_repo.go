package repository

import (
	"context"

	"alexGo-cloud/modules/order/model"
	"alexGo-cloud/pkg/tenant"
	"gorm.io/gorm"
)

type OrderRepository interface {
	Create(ctx context.Context, entity *model.Order) error
	List(ctx context.Context) ([]*model.Order, error)
	GetByID(ctx context.Context, id uint64) (*model.Order, error)
}

type orderRepo struct {
	db *gorm.DB
}

func NewOrderRepository(db *gorm.DB) OrderRepository {
	return &orderRepo{db: db}
}

func (r *orderRepo) Create(ctx context.Context, entity *model.Order) error {
	return r.db.WithContext(ctx).Create(entity).Error
}

// List 按当前租户过滤取全量（与导出共用）。
//
// Order 模型无 TenantID 字段（orders 表无 tenant_id 列）→ gormplugin 明文跳过
// 不接管（tid=0 也不注入），隔离必须显式做：订单归属经 user_id → system_users。
// 与 system/member repo 显式 `tenant_id = ?` 同口径；tid 取法同 service 先例
// （tenant.TenantIDFromContext）。tid=0（ctx 无租户）时子查询只命中
// tenant_id=0 的用户（生产无此数据）→ 空列表，与 member/system 传 0 同语义。
func (r *orderRepo) List(ctx context.Context) ([]*model.Order, error) {
	var list []*model.Order
	tid := tenant.TenantIDFromContext(ctx)
	err := r.db.WithContext(ctx).
		Where("user_id IN (SELECT id FROM system_users WHERE tenant_id = ?)", tid).
		Find(&list).Error
	return list, err
}

func (r *orderRepo) GetByID(ctx context.Context, id uint64) (*model.Order, error) {
	var order model.Order
	err := r.db.WithContext(ctx).First(&order, id).Error
	return &order, err
}
