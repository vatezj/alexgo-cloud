package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"alexGo-cloud/pkg/repo"
	"alexGo-cloud/pkg/token"
)

// OnlineSession 投影：只取列表/踢人需要的列，不取 refresh_token（最小暴露）。
// Username 非表列，由 ListAdmin 分批补查填充。
type OnlineSession struct {
	ID          uint64         `gorm:"column:id" json:"id"`
	UserID      uint64         `gorm:"column:user_id" json:"user_id"`
	UserType    token.UserType `gorm:"column:user_type" json:"user_type"` // Task 2 Kick 校验 user_type=1 依赖此列投影
	ClientID    string         `gorm:"column:client_id" json:"client_id"`
	AccessToken string         `gorm:"column:access_token" json:"-"`
	IssuedAt    time.Time      `gorm:"column:create_time" json:"issued_at"`
	ExpiresAt   time.Time      `gorm:"column:expires_time" json:"expires_at"`
	TenantID    uint64         `gorm:"column:tenant_id" json:"tenant_id"`
	Username    string         `gorm:"-" json:"username"` // 分批补查，非表列
}

func (OnlineSession) TableName() string { return "system_oauth2_access_token" }

// MemberBrief 代登录目标的最小画像（member_user 表，system 侧只读点查，
// 照 tenant.go CountAccounts 的 raw Table 先例——不 import modules/member）。
type MemberBrief struct {
	ID       uint64 `gorm:"column:id"`
	Nickname string `gorm:"column:nickname"`
	Status   int    `gorm:"column:status"` // 1启用 0停用
	Deleted  bool   `gorm:"column:deleted"`
	TenantID uint64 `gorm:"column:tenant_id"`
}

func (MemberBrief) TableName() string { return "member_user" }

type OnlineRepository interface {
	// ListAdmin 本租户未过期管理员会话分页（gormplugin 自动追加 tenant_id）。
	ListAdmin(ctx context.Context, page, size int) ([]*OnlineSession, int64, error)
	// GetByID 按行 id 点查（含过期行——踢过期行等价清理）；跨租户经插件过滤后查无。
	GetByID(ctx context.Context, id uint64) (*OnlineSession, error)
	// GetMemberForImpersonate 代登录目标点查（含已删/停用行，交 service 判定）。
	GetMemberForImpersonate(ctx context.Context, userID uint64) (*MemberBrief, error)
}

type onlineRepo struct{ db *gorm.DB }

func NewOnlineRepository(db *gorm.DB) OnlineRepository {
	return &onlineRepo{db: db}
}

func (r *onlineRepo) ListAdmin(ctx context.Context, page, size int) ([]*OnlineSession, int64, error) {
	// 禁止 JOIN：gormplugin 追加的非限定 tenant_id 在 JOIN system_users 时列名歧义。
	// 先分页 token 行（Count 只数本表，准确），再按页内 user_id 批量补 username。
	q := r.db.WithContext(ctx).Model(&OnlineSession{}).
		Where("user_type = ? AND expires_time > ? AND deleted = ?", 1, time.Now(), false).
		Order("id DESC")
	rows, total, err := repo.Paginate[*OnlineSession](ctx, q, page, size)
	if err != nil || len(rows) == 0 {
		return rows, total, err
	}
	ids := make([]uint64, 0, len(rows))
	seen := map[uint64]bool{}
	for _, row := range rows {
		if !seen[row.UserID] {
			seen[row.UserID] = true
			ids = append(ids, row.UserID)
		}
	}
	var users []struct {
		ID       uint64 `gorm:"column:id"`
		Username string `gorm:"column:username"`
	}
	if err := r.db.WithContext(ctx).Model(&struct {
		ID       uint64 `gorm:"column:id"`
		Username string `gorm:"column:username"`
	}{}).Table("system_users").
		Select("id, username").
		Where("id IN ? AND deleted = 0", ids).
		Find(&users).Error; err != nil {
		return nil, 0, err
	}
	nameOf := map[uint64]string{}
	for _, u := range users {
		nameOf[u.ID] = u.Username
	}
	for _, row := range rows {
		row.Username = nameOf[row.UserID] // 已删用户留空串，前端渲染 "-"
	}
	return rows, total, nil
}

func (r *onlineRepo) GetByID(ctx context.Context, id uint64) (*OnlineSession, error) {
	var row OnlineSession
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil // 查无/跨租户：交 service 判 30203
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *onlineRepo) GetMemberForImpersonate(ctx context.Context, userID uint64) (*MemberBrief, error) {
	var m MemberBrief
	err := r.db.WithContext(ctx).Where("id = ?", userID).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}
