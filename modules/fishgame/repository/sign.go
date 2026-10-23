package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"alexGo-cloud/modules/fishgame/model"
)

// SignRepository 签到对账数据访问。
type SignRepository interface {
	GetByPlayerDate(ctx context.Context, playerID uint64, date string) (*model.Sign, error) // 软查
	// GetLastByPlayer 最近一条签到（date desc）；无 → (nil, nil)。服务端权威 streak 用。
	GetLastByPlayer(ctx context.Context, playerID uint64) (*model.Sign, error)
	Create(ctx context.Context, s *model.Sign) error
}

type signRepo struct{ db *gorm.DB }

func NewSignRepository(db *gorm.DB) SignRepository { return &signRepo{db: db} }

func (r *signRepo) GetByPlayerDate(ctx context.Context, playerID uint64, date string) (*model.Sign, error) {
	var out model.Sign
	err := r.db.WithContext(ctx).
		Where("player_id = ? AND date = ?", playerID, date).First(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &out, err
}

func (r *signRepo) GetLastByPlayer(ctx context.Context, playerID uint64) (*model.Sign, error) {
	var out model.Sign
	err := r.db.WithContext(ctx).
		Where("player_id = ?", playerID).Order("date desc").First(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &out, err
}

func (r *signRepo) Create(ctx context.Context, s *model.Sign) error {
	if s.TenantID == 0 {
		s.TenantID = 0
	}
	return r.db.WithContext(ctx).Create(s).Error
}
