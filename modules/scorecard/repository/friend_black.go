package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"alexGo-cloud/modules/scorecard/model"
	"alexGo-cloud/pkg/repo"
	"alexGo-cloud/pkg/tenant"
)

// FriendBlackRepository 拉黑关系数据访问（单向行，判定时查双向）。
// 租户隔离靠 gormplugin。
type FriendBlackRepository interface {
	Create(ctx context.Context, b *model.FriendBlack) error
	Exists(ctx context.Context, a, b uint64) (bool, error) // 双向判定 (a黑b) 或 (b黑a)
	Delete(ctx context.Context, userID, blockedID uint64) error
	Get(ctx context.Context, userID, blockedID uint64) (*model.FriendBlack, error) // 软查：不存在 → (nil, nil)
	ListAdmin(ctx context.Context, page, size int) ([]*model.FriendBlack, int64, error)
}

type friendBlackRepo struct {
	db *gorm.DB
}

func NewFriendBlackRepository(db *gorm.DB) FriendBlackRepository {
	return &friendBlackRepo{db: db}
}

func (r *friendBlackRepo) Create(ctx context.Context, entity *model.FriendBlack) error {
	// 打戳（spec §9.2）：json:"-" 使客户端灌不进来，零值才回填、显式值保留。
	if entity.TenantID == 0 {
		entity.TenantID = tenant.TenantIDFromContext(ctx)
	}
	return r.db.WithContext(ctx).Create(entity).Error
}

// Exists 双向拉黑判定（进房/加好友/转让拦截共用）：(a黑b) 或 (b黑a) 任一命中即 true。
func (r *friendBlackRepo) Exists(ctx context.Context, a, b uint64) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.FriendBlack{}).
		Where("(user_id = ? AND blocked_user_id = ?) OR (user_id = ? AND blocked_user_id = ?)", a, b, b, a).
		Count(&n).Error
	return n > 0, err
}

func (r *friendBlackRepo) Delete(ctx context.Context, userID, blockedID uint64) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND blocked_user_id = ?", userID, blockedID).
		Delete(&model.FriendBlack{}).Error
}

// Get 单向拉黑行软查。
func (r *friendBlackRepo) Get(ctx context.Context, userID, blockedID uint64) (*model.FriendBlack, error) {
	var out model.FriendBlack
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND blocked_user_id = ?", userID, blockedID).First(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &out, err
}

// ListAdmin 监管端拉黑关系分页。
func (r *friendBlackRepo) ListAdmin(ctx context.Context, page, size int) ([]*model.FriendBlack, int64, error) {
	q := r.db.Model(&model.FriendBlack{}).Order("id desc")
	return repo.Paginate[*model.FriendBlack](ctx, q, page, size)
}
