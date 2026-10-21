package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"alexGo-cloud/pkg/repo"
	"alexGo-cloud/pkg/tenant"
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
	// ListAdmin 本租户未过期管理员会话分页（显式 tenant_id 过滤，见实现注释）。
	ListAdmin(ctx context.Context, page, size int) ([]*OnlineSession, int64, error)
	// GetByID 按行 id 点查（含过期行——踢过期行等价清理）；跨租户显式过滤后查无。
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
	// 租户隔离显式收口（终审 C1，裁定「tid=0 按普通租户」）：gormplugin 对 tid=0
	// 不注入（plugin.go：平台/未解析保持历史行为），而生产主路径恰是种子管理员
	// tid=0——不显式过滤则跨租户裸奔。tid≠0 时插件再注入同款条件，重复无害，
	// 不为去重绕开插件（隔离下限与仓库显式过滤双层，照 order_repo.List 口径）。
	tid := tenant.TenantIDFromContext(ctx)
	q := r.db.WithContext(ctx).Model(&OnlineSession{}).
		Where("tenant_id = ? AND user_type = ? AND expires_time > ? AND deleted = ?", tid, 1, time.Now(), false).
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
	// 补查同款显式 tenant_id（匿名结构体无 TenantID 字段，插件不接管本查询）：
	// id IN 已由 token 行租户收敛，显式过滤是一致性防御（终审 C1）。
	if err := r.db.WithContext(ctx).Model(&struct {
		ID       uint64 `gorm:"column:id"`
		Username string `gorm:"column:username"`
	}{}).Table("system_users").
		Select("id, username").
		Where("tenant_id = ? AND id IN ? AND deleted = 0", tid, ids).
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
	// 租户显式过滤（终审 C1，同 ListAdmin 注释）：tid=0 插件不注入，
	// 跨租户按 id 直踢必须查无 → 30203。
	tid := tenant.TenantIDFromContext(ctx)
	var row OnlineSession
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tid, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil // 查无/跨租户：交 service 判 30203
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *onlineRepo) GetMemberForImpersonate(ctx context.Context, userID uint64) (*MemberBrief, error) {
	// 租户显式过滤（终审 C1，同 ListAdmin 注释）：tid=0 插件不注入，
	// 跨租户会员代登录必须查无 → 30201（否则直调 API 即可绕过 modal 的本租户筛选）。
	tid := tenant.TenantIDFromContext(ctx)
	var m MemberBrief
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tid, userID).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}
