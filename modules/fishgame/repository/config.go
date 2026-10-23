package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"alexGo-cloud/modules/fishgame/model"
	"alexGo-cloud/pkg/repo"
)

// ConfigRepository 运营覆盖配置（luck / fish_king）。
type ConfigRepository interface {
	// ListActiveByType 当前生效的一条：status=1 且 start_at<=now（或空）且 end_at>=now（或空）；无 → (nil, nil)。
	ListActiveByType(ctx context.Context, typ string, now time.Time) (*model.Config, error)
	ListAdmin(ctx context.Context, typ string, page, size int) ([]*model.Config, int64, error)
	GetByID(ctx context.Context, id uint64) (*model.Config, error) // 软查
	Create(ctx context.Context, c *model.Config) error
	Update(ctx context.Context, c *model.Config) error
	Delete(ctx context.Context, id uint64) error
}

type configRepo struct{ db *gorm.DB }

func NewConfigRepository(db *gorm.DB) ConfigRepository { return &configRepo{db: db} }

func (r *configRepo) ListActiveByType(ctx context.Context, typ string, now time.Time) (*model.Config, error) {
	var out model.Config
	q := r.db.WithContext(ctx).
		Where("type = ? AND status = 1", typ).
		Where("(start_at IS NULL OR start_at <= ?)", now).
		Where("(end_at IS NULL OR end_at >= ?)", now).
		Order("id desc").Limit(1)
	err := q.First(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &out, err
}

func (r *configRepo) ListAdmin(ctx context.Context, typ string, page, size int) ([]*model.Config, int64, error) {
	q := r.db.Model(&model.Config{}).Order("id desc")
	if typ != "" {
		q = q.Where("type = ?", typ)
	}
	return repo.Paginate[*model.Config](ctx, q, page, size)
}

func (r *configRepo) GetByID(ctx context.Context, id uint64) (*model.Config, error) {
	var out model.Config
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &out, err
}

func (r *configRepo) Create(ctx context.Context, c *model.Config) error {
	if c.TenantID == 0 {
		c.TenantID = 0
	}
	return r.db.WithContext(ctx).Create(c).Error
}

func (r *configRepo) Update(ctx context.Context, c *model.Config) error {
	return r.db.WithContext(ctx).Save(c).Error
}

func (r *configRepo) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Config{}).Error
}
