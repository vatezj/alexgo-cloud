package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"alexGo-cloud/modules/scorecard/model"
	"alexGo-cloud/pkg/repo"
	"alexGo-cloud/pkg/tenant"
)

// FriendRepository 好友关系数据访问（单向行，双向各一行由 service 保证）。
// 租户隔离靠 gormplugin。
type FriendRepository interface {
	Create(ctx context.Context, f *model.Friend) error
	Get(ctx context.Context, userID, friendID uint64) (*model.Friend, error) // 软查：不存在 → (nil, nil)
	ListByUser(ctx context.Context, userID uint64) ([]*model.Friend, error)
	SetRank(ctx context.Context, userID, friendID uint64, enabled int) error
	Delete(ctx context.Context, userID, friendID uint64) error
	ListAdmin(ctx context.Context, page, size int) ([]*model.Friend, int64, error)
}

type friendRepo struct {
	db *gorm.DB
}

func NewFriendRepository(db *gorm.DB) FriendRepository {
	return &friendRepo{db: db}
}

func (r *friendRepo) Create(ctx context.Context, entity *model.Friend) error {
	// 打戳（spec §9.2）：json:"-" 使客户端灌不进来，零值才回填、显式值保留。
	if entity.TenantID == 0 {
		entity.TenantID = tenant.TenantIDFromContext(ctx)
	}
	return r.db.WithContext(ctx).Create(entity).Error
}

// Get 单向关系行软查（好友存在性/排名开关读取）。
func (r *friendRepo) Get(ctx context.Context, userID, friendID uint64) (*model.Friend, error) {
	var out model.Friend
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND friend_user_id = ?", userID, friendID).First(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &out, err
}

// ListByUser 该用户的好友列表（排名页数据源）。
func (r *friendRepo) ListByUser(ctx context.Context, userID uint64) ([]*model.Friend, error) {
	var list []*model.Friend
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).Order("id asc").Find(&list).Error
	return list, err
}

// SetRank 排名参与开关（0/1）。
func (r *friendRepo) SetRank(ctx context.Context, userID, friendID uint64, enabled int) error {
	return r.db.WithContext(ctx).Model(&model.Friend{}).
		Where("user_id = ? AND friend_user_id = ?", userID, friendID).
		UpdateColumn("rank_enabled", enabled).Error
}

func (r *friendRepo) Delete(ctx context.Context, userID, friendID uint64) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND friend_user_id = ?", userID, friendID).
		Delete(&model.Friend{}).Error
}

// ListAdmin 监管端好友关系分页。
func (r *friendRepo) ListAdmin(ctx context.Context, page, size int) ([]*model.Friend, int64, error) {
	q := r.db.Model(&model.Friend{}).Order("id desc")
	return repo.Paginate[*model.Friend](ctx, q, page, size)
}
