package repository

import (
	"context"

	"alexGo-cloud/modules/order/model"
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

func (r *orderRepo) List(ctx context.Context) ([]*model.Order, error) {
	var list []*model.Order
	err := r.db.WithContext(ctx).Find(&list).Error
	return list, err
}

func (r *orderRepo) GetByID(ctx context.Context, id uint64) (*model.Order, error) {
	var order model.Order
	err := r.db.WithContext(ctx).First(&order, id).Error
	return &order, err
}
