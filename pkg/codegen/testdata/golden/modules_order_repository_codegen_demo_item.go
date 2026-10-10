package repository

import (
	"context"

	"alexGo-cloud/modules/order/model"
	"gorm.io/gorm"
)

type CodegenDemoItemRepository interface {
	Create(ctx context.Context, entity *model.CodegenDemoItem) error
	List(ctx context.Context, tenantID uint64) ([]*model.CodegenDemoItem, error)
	GetByID(ctx context.Context, tenantID uint64, id uint64) (*model.CodegenDemoItem, error)
	Update(ctx context.Context, entity *model.CodegenDemoItem) error
	Delete(ctx context.Context, tenantID uint64, id uint64) error
}

type codegendemoitemRepo struct {
	db *gorm.DB
}

func NewCodegenDemoItemRepository(db *gorm.DB) CodegenDemoItemRepository {
	return &codegendemoitemRepo{db: db}
}

func (r *codegendemoitemRepo) Create(ctx context.Context, entity *model.CodegenDemoItem) error {
	return r.db.WithContext(ctx).Create(entity).Error
}

func (r *codegendemoitemRepo) List(ctx context.Context, tenantID uint64) ([]*model.CodegenDemoItem, error) {
	var list []*model.CodegenDemoItem
	q := r.db.WithContext(ctx).Model(&model.CodegenDemoItem{})
	q = q.Where("tenant_id = ?", tenantID)
	err := q.Find(&list).Error
	return list, err
}

func (r *codegendemoitemRepo) GetByID(ctx context.Context, tenantID uint64, id uint64) (*model.CodegenDemoItem, error) {
	var entity model.CodegenDemoItem
	q := r.db.WithContext(ctx).Model(&model.CodegenDemoItem{})
	q = q.Where("tenant_id = ?", tenantID)
	err := q.Where("id = ?", id).First(&entity).Error
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

func (r *codegendemoitemRepo) Update(ctx context.Context, entity *model.CodegenDemoItem) error {
	return r.db.WithContext(ctx).Save(entity).Error
}

func (r *codegendemoitemRepo) Delete(ctx context.Context, tenantID uint64, id uint64) error {
	q := r.db.WithContext(ctx).Model(&model.CodegenDemoItem{})
	q = q.Where("tenant_id = ?", tenantID)
	return q.Where("id = ?", id).Delete(&model.CodegenDemoItem{}).Error
}
