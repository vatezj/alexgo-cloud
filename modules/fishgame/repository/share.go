package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"alexGo-cloud/modules/fishgame/model"
)

// ShareRepository 分享图元数据（图本体在 pkg/storage）。
type ShareRepository interface {
	Create(ctx context.Context, s *model.Share) error
	GetByKey(ctx context.Context, key string) (*model.Share, error) // 软查
}

type shareRepo struct{ db *gorm.DB }

func NewShareRepository(db *gorm.DB) ShareRepository { return &shareRepo{db: db} }

func (r *shareRepo) Create(ctx context.Context, s *model.Share) error {
	if s.TenantID == 0 {
		s.TenantID = 0
	}
	return r.db.WithContext(ctx).Create(s).Error
}

func (r *shareRepo) GetByKey(ctx context.Context, key string) (*model.Share, error) {
	var out model.Share
	err := r.db.WithContext(ctx).Where("`key` = ?", key).First(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &out, err
}
