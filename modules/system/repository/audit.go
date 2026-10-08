package repository

import (
	"context"

	"alexGo-cloud/modules/system/model"
	"gorm.io/gorm"
)

type AuditRepository interface {
	CreateLogin(ctx context.Context, l *model.LoginLog) error
	CreateOperate(ctx context.Context, l *model.OperateLog) error
	ListLogin(ctx context.Context, tenantID uint64, limit int) ([]*model.LoginLog, error)
	ListOperate(ctx context.Context, tenantID uint64, limit int) ([]*model.OperateLog, error)
}

type auditRepo struct {
	db *gorm.DB
}

func NewAuditRepository(db *gorm.DB) AuditRepository {
	return &auditRepo{db: db}
}

func (r *auditRepo) CreateLogin(ctx context.Context, l *model.LoginLog) error {
	return r.db.WithContext(ctx).Create(l).Error
}

func (r *auditRepo) CreateOperate(ctx context.Context, l *model.OperateLog) error {
	return r.db.WithContext(ctx).Create(l).Error
}

func (r *auditRepo) ListLogin(ctx context.Context, tenantID uint64, limit int) ([]*model.LoginLog, error) {
	if limit <= 0 {
		limit = 100
	}
	var list []*model.LoginLog
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("id desc").
		Limit(limit).
		Find(&list).Error
	return list, err
}

func (r *auditRepo) ListOperate(ctx context.Context, tenantID uint64, limit int) ([]*model.OperateLog, error) {
	if limit <= 0 {
		limit = 100
	}
	var list []*model.OperateLog
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("id desc").
		Limit(limit).
		Find(&list).Error
	return list, err
}

