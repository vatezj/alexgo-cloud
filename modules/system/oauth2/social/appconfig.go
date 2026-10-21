package social

import (
	"context"
	"errors"

	"go.uber.org/fx"
	"gorm.io/gorm"

	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/tenant"
)

// AppConfigDeps Fx 装配结构（Task 9 admin controller 消费 AppConfigService）。
type AppConfigDeps struct {
	fx.In
	Repo     SocialRepository
	Adapters *AdapterRegistry
}

// AppConfigService admin 渠道配置（system_social_app）CRUD 业务规则：
//   - 平台必须已注册适配器（写路径先于其余校验，未知平台一律 10001）；
//   - 每租户每平台唯一（(tid, platform) 查重）；
//   - secret 明文存储（调平台 API 必须明文；json:"-" 保证 admin 响应不回显）；
//   - 编辑时 secret 留空 = 保留库中旧值（表单留空不改机密）。
type AppConfigService struct {
	repo     SocialRepository
	adapters *AdapterRegistry
}

// NewAppConfigService 构造（fx 按 AppConfigDeps 注入）。
func NewAppConfigService(deps AppConfigDeps) *AppConfigService {
	return &AppConfigService{repo: deps.Repo, adapters: deps.Adapters}
}

// List 分页列渠道配置（platform=="" 全平台视图，透传 repo）；租户取自 ctx。
func (s *AppConfigService) List(ctx context.Context, page, size int, platform string) ([]*SocialApp, int64, error) {
	tid := tenant.TenantIDFromContext(ctx)
	return s.repo.ListApps(ctx, tid, page, size, platform)
}

// Create 新建渠道配置，规则顺序（brief 逐条）：
// 平台已注册 → appid/secret 必填（10002）→ status ∈ {0,1} → 同 (tid,platform)
// 查重 → ID/TenantID 归位 → 落库。
func (s *AppConfigService) Create(ctx context.Context, app *SocialApp) error {
	if !s.adapters.Exists(app.Platform) {
		return errs.New(errs.CodeParamInvalid, nil) // 未知平台是管理端输入
	}
	if app.AppID == "" || app.Secret == "" {
		return errs.New(errs.CodeParamMissing, nil)
	}
	if app.Status != 0 && app.Status != 1 {
		return errs.New(errs.CodeParamInvalid, nil)
	}
	tid := tenant.TenantIDFromContext(ctx)
	// 同 (tid, platform) 已存在 = 重复：brief 规则「非 ErrRecordNotFound → 10001」
	if _, err := s.repo.GetApp(ctx, tid, app.Platform); !errors.Is(err, gorm.ErrRecordNotFound) {
		return errs.New(errs.CodeParamInvalid, err)
	}
	// CreateApp 走 IgnoreTenant（T3 契约）：tenant_id 只认这里的显式赋值，
	// ID 同步归 0（JSON body 可带 id 字段，防止指定主键落库）
	app.ID = 0
	app.TenantID = tid
	return s.repo.CreateApp(ctx, app)
}

// Update 编辑渠道配置，规则顺序（brief 逐条）：
// ID!=0 → 平台已注册 → status ∈ {0,1} → appid 必填（10002）→ 取旧行
// （GetAppByID(tid,id) 未找到含跨租户不可见 → 10001）→ secret 留空保留旧值
// → platform 变更且被本租户其他行占用 → 10001 → 落库。
func (s *AppConfigService) Update(ctx context.Context, app *SocialApp) error {
	if app.ID == 0 {
		return errs.New(errs.CodeParamInvalid, nil)
	}
	if !s.adapters.Exists(app.Platform) {
		return errs.New(errs.CodeParamInvalid, nil)
	}
	if app.Status != 0 && app.Status != 1 {
		return errs.New(errs.CodeParamInvalid, nil)
	}
	if app.AppID == "" {
		return errs.New(errs.CodeParamMissing, nil)
	}
	tid := tenant.TenantIDFromContext(ctx)
	old, err := s.repo.GetAppByID(ctx, tid, app.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errs.New(errs.CodeParamInvalid, err) // 不存在 / 跨租户不可见
	}
	if err != nil {
		return err
	}
	if app.Secret == "" {
		app.Secret = old.Secret // 编辑表单留空 = 不改机密
	}
	if app.Platform != old.Platform {
		hit, herr := s.repo.GetApp(ctx, tid, app.Platform)
		if herr == nil && hit.ID != app.ID {
			return errs.New(errs.CodeParamInvalid, nil) // 平台已被本租户其他行占用
		}
		if herr != nil && !errors.Is(herr, gorm.ErrRecordNotFound) {
			return herr
		}
	}
	// UpdateApp 的 WHERE 走 app.TenantID：必须显式注入 ctx 租户，
	// 否则 body 里 tenant_id 缺省为 0 → 0 行 → 误报 10001
	app.TenantID = tid
	return s.repo.UpdateApp(ctx, app)
}

// Delete 删渠道配置：repo 按 (tid, id) 硬删，0 行（不存在/跨租户）回
// ErrRecordNotFound → 映射 10001。
func (s *AppConfigService) Delete(ctx context.Context, id uint64) error {
	tid := tenant.TenantIDFromContext(ctx)
	err := s.repo.DeleteApp(ctx, tid, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errs.New(errs.CodeParamInvalid, err)
	}
	return err
}
