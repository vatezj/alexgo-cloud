// Package social 微信第三方登录（渠道配置 system_social_app + 身份绑定 system_member_identity）。
//
// 租户口径（本包硬规则）：全部查询/写入显式 tenant_id 过滤——gormplugin 对
// tid=0 沉默不注入（P1-3 教训），不能依赖插件兜底；读方法未找到一律返回
// gorm.ErrRecordNotFound。
package social

import (
	"context"
	"time"

	"gorm.io/gorm"

	"alexGo-cloud/pkg/repo"
	"alexGo-cloud/pkg/tenant/gormplugin"
)

// SocialRepository 渠道配置 + 第三方身份绑定仓储（12 方法）。
// 显式 tenant_id 过滤——gormplugin 对 tid=0 沉默不注入。
type SocialRepository interface {
	GetApp(ctx context.Context, tenantID uint64, platform string) (*SocialApp, error)
	GetAppByID(ctx context.Context, tenantID, id uint64) (*SocialApp, error)
	ListApps(ctx context.Context, tenantID uint64, page, size int, platform string) ([]*SocialApp, int64, error) // platform=="" 不筛
	CreateApp(ctx context.Context, app *SocialApp) error
	UpdateApp(ctx context.Context, app *SocialApp) error // WHERE tenant_id=? AND id=? 走 Updates(map)
	DeleteApp(ctx context.Context, tenantID, id uint64) error
	GetIdentity(ctx context.Context, tenantID uint64, platform, openid string) (*MemberIdentity, error)
	GetIdentityByUnion(ctx context.Context, tenantID uint64, unionid string) (*MemberIdentity, error) // SQL 层再防 unionid=''
	ListIdentities(ctx context.Context, tenantID, memberUserID uint64) ([]*MemberIdentity, error)
	CreateIdentity(ctx context.Context, idn *MemberIdentity) error
	ReassignIdentities(ctx context.Context, tenantID, from, to uint64) (int64, error)
	DeleteIdentity(ctx context.Context, tenantID, memberUserID uint64, platform string) (int64, error)
}

type socialRepo struct{ db *gorm.DB }

// NewSocialRepository 构造 GORM 实现（fx 注入 *gorm.DB）。
func NewSocialRepository(db *gorm.DB) SocialRepository { return &socialRepo{db: db} }

// GetApp 按 (tenant, platform) 取渠道配置（每租户每平台唯一）。
func (r *socialRepo) GetApp(ctx context.Context, tid uint64, platform string) (*SocialApp, error) {
	var app SocialApp
	// 显式 tenant_id 过滤——gormplugin 对 tid=0 沉默不注入。
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND platform = ?", tid, platform).
		First(&app).Error
	if err != nil {
		return nil, err
	}
	return &app, nil
}

// GetAppByID 按 (tenant, id) 取渠道配置；跨租户 id 不可见。
func (r *socialRepo) GetAppByID(ctx context.Context, tid, id uint64) (*SocialApp, error) {
	var app SocialApp
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tid, id).
		First(&app).Error
	if err != nil {
		return nil, err
	}
	return &app, nil
}

// ListApps 分页列渠道配置：tenant 恒筛，platform=="" 不筛（全平台视图）。
func (r *socialRepo) ListApps(ctx context.Context, tid uint64, page, size int, platform string) ([]*SocialApp, int64, error) {
	q := r.db.Where("tenant_id = ?", tid)
	if platform != "" {
		q = q.Where("platform = ?", platform)
	}
	return repo.Paginate[*SocialApp](ctx, q.Order("id desc"), page, size)
}

// CreateApp 新建渠道配置。
// tenant_id 以 app.TenantID 显式值为准（含 tid=0 行）：IgnoreTenant 屏蔽 gormplugin
// 的零值回填——否则 ctx 带 tid≠0 时 tid=0 行会被改写成 ctx 租户并撞 (tenant,platform)
// 唯一键；create_time/update_time 零值补 now（模型字段非 CreatedAt/UpdatedAt，gorm 不自动跟踪）。
func (r *socialRepo) CreateApp(ctx context.Context, app *SocialApp) error {
	now := time.Now()
	if app.CreateTime.IsZero() {
		app.CreateTime = now
	}
	if app.UpdateTime.IsZero() {
		app.UpdateTime = now
	}
	return r.db.WithContext(gormplugin.IgnoreTenant(ctx)).Create(app).Error
}

// UpdateApp 按 (tenant, id) 走 Updates(map) 局部更新；0 行（不存在/跨租户）→ ErrRecordNotFound。
func (r *socialRepo) UpdateApp(ctx context.Context, app *SocialApp) error {
	// 显式 tenant_id 过滤——gormplugin 对 tid=0 沉默不注入。
	res := r.db.WithContext(ctx).
		Model(&SocialApp{}).
		Where("tenant_id = ? AND id = ?", app.TenantID, app.ID).
		Updates(map[string]interface{}{
			"platform":    app.Platform,
			"appid":       app.AppID,
			"secret":      app.Secret,
			"status":      app.Status,
			"update_time": time.Now(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteApp 按 (tenant, id) 硬删；0 行（不存在/跨租户）→ ErrRecordNotFound。
func (r *socialRepo) DeleteApp(ctx context.Context, tid, id uint64) error {
	res := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tid, id).
		Delete(&SocialApp{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// GetIdentity 按 (tenant, platform, openid) 取绑定行。
func (r *socialRepo) GetIdentity(ctx context.Context, tid uint64, platform, openid string) (*MemberIdentity, error) {
	var idn MemberIdentity
	// 显式 tenant_id 过滤——gormplugin 对 tid=0 沉默不注入。
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND platform = ? AND openid = ?", tid, platform, openid).
		First(&idn).Error
	if err != nil {
		return nil, err
	}
	return &idn, nil
}

// GetIdentityByUnion 按 (tenant, unionid) 取同人最早绑定行；
// SQL 层附加 unionid 非空条件——空串不参与同人匹配，传空串查询必落空。
func (r *socialRepo) GetIdentityByUnion(ctx context.Context, tid uint64, unionid string) (*MemberIdentity, error) {
	var idn MemberIdentity
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND unionid = ? AND unionid <> ''", tid, unionid).
		First(&idn).Error
	if err != nil {
		return nil, err
	}
	return &idn, nil
}

// ListIdentities 逐成员列绑定行（id asc 保序）。
func (r *socialRepo) ListIdentities(ctx context.Context, tid, memberUserID uint64) ([]*MemberIdentity, error) {
	var list []*MemberIdentity
	// 显式 tenant_id 过滤——gormplugin 对 tid=0 沉默不注入。
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND member_user_id = ?", tid, memberUserID).
		Order("id").
		Find(&list).Error
	return list, err
}

// CreateIdentity 新建绑定行。
// 同 CreateApp：tenant_id 以结构体显式值为准（屏蔽插件零值回填），create_time 零值补 now。
func (r *socialRepo) CreateIdentity(ctx context.Context, idn *MemberIdentity) error {
	if idn.CreateTime.IsZero() {
		idn.CreateTime = time.Now()
	}
	return r.db.WithContext(gormplugin.IgnoreTenant(ctx)).Create(idn).Error
}

// ReassignIdentities 把 (tenant, from) 的绑定行改指 to（换绑会员），返回影响行数。
// 显式 tenant 过滤——同 member 的跨租户行不受影响。
func (r *socialRepo) ReassignIdentities(ctx context.Context, tid, from, to uint64) (int64, error) {
	res := r.db.WithContext(ctx).
		Model(&MemberIdentity{}).
		Where("tenant_id = ? AND member_user_id = ?", tid, from).
		Update("member_user_id", to)
	return res.RowsAffected, res.Error
}

// DeleteIdentity 删 (tenant, member, platform) 绑定行，返回影响行数（0 行不算错误）。
func (r *socialRepo) DeleteIdentity(ctx context.Context, tid, memberUserID uint64, platform string) (int64, error) {
	res := r.db.WithContext(ctx).
		Where("tenant_id = ? AND member_user_id = ? AND platform = ?", tid, memberUserID, platform).
		Delete(&MemberIdentity{})
	return res.RowsAffected, res.Error
}
