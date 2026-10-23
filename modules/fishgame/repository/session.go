package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"alexGo-cloud/modules/fishgame/model"
)

// SessionRepository X-Session 会话数据访问。
type SessionRepository interface {
	Create(ctx context.Context, s *model.Session) error
	GetByToken(ctx context.Context, token string) (*model.Session, error) // 过期/不存在 → (nil, nil)
	TouchExpire(ctx context.Context, id uint64, newExpire time.Time) error
	DeleteByToken(ctx context.Context, token string) error
}

type sessionRepo struct{ db *gorm.DB }

func NewSessionRepository(db *gorm.DB) SessionRepository { return &sessionRepo{db: db} }

func (r *sessionRepo) Create(ctx context.Context, s *model.Session) error {
	if s.TenantID == 0 {
		s.TenantID = 0 // 游戏端恒 0（显式钉底，不走 ctx）
	}
	return r.db.WithContext(ctx).Create(s).Error
}

// GetByToken 有效会话软查：token 不存在或 expire_at 已过 → (nil, nil)。
func (r *sessionRepo) GetByToken(ctx context.Context, token string) (*model.Session, error) {
	var out model.Session
	err := r.db.WithContext(ctx).
		Where("token = ? AND expire_at > ?", token, time.Now()).
		First(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &out, err
}

// TouchExpire 滑动续期（剩余 <7d 时延到 now+30d）。
func (r *sessionRepo) TouchExpire(ctx context.Context, id uint64, newExpire time.Time) error {
	return r.db.WithContext(ctx).Model(&model.Session{}).
		Where("id = ?", id).UpdateColumn("expire_at", newExpire).Error
}

func (r *sessionRepo) DeleteByToken(ctx context.Context, token string) error {
	return r.db.WithContext(ctx).Where("token = ?", token).Delete(&model.Session{}).Error
}
