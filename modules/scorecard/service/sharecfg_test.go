package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	sysmodel "alexGo-cloud/modules/system/model"
	sysrepo "alexGo-cloud/modules/system/repository"
	syssvc "alexGo-cloud/modules/system/service"
	"alexGo-cloud/pkg/tenant"
)

// newShareCfgForTest sqlite 内存库 + system_configs 表 + 真 ConfigService（同生产装配路径）。
func newShareCfgForTest(t *testing.T) (*ShareConfigService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&sysmodel.SystemConfig{}); err != nil {
		t.Fatal(err)
	}
	cfgs := syssvc.NewConfigService(sysrepo.NewConfigRepository(db))
	return NewShareConfigService(cfgs), db
}

// TestShareCfg_Get_DefaultsWhenMissing 空库三页全回退内置默认（后台未 seed 也不挂）。
func TestShareCfg_Get_DefaultsWhenMissing(t *testing.T) {
	svc, _ := newShareCfgForTest(t)
	ctx := tenant.WithTenantID(context.Background(), 0)

	got := svc.Get(ctx)
	if len(got) != 3 {
		t.Fatalf("应返回 3 页，实际 %d", len(got))
	}
	for page, def := range shareCardDefaults {
		if got[page] != def {
			t.Errorf("page=%s 应等于默认 %+v，实际 %+v", page, def, got[page])
		}
	}
}

// TestShareCfg_SeedIdempotent_SkipsExisting Seed 补缺不覆盖：先手插自定义值，Seed 后保持原样；缺的行被补上。
func TestShareCfg_SeedIdempotent_SkipsExisting(t *testing.T) {
	svc, db := newShareCfgForTest(t)
	ctx := tenant.WithTenantID(context.Background(), 0)

	// 预插一条管理员改过的首页参数
	custom := `{"title":"管理员改过的标题","imageUrl":"https://example.com/x.png"}`
	if err := db.Create(&sysmodel.SystemConfig{
		Key: ShareConfigKeyIndex, Value: custom, Name: "分享卡 · 首页", TenantID: 0,
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := svc.Seed(context.Background()); err != nil {
		t.Fatalf("Seed 失败: %v", err)
	}
	if err := svc.Seed(context.Background()); err != nil {
		t.Fatalf("二次 Seed（幂等）失败: %v", err)
	}

	var rows []sysmodel.SystemConfig
	if err := db.Where("tenant_id = 0").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("应恰好 3 行（index 预插 + simulator/salary 补建），实际 %d", len(rows))
	}
	for _, r := range rows {
		if r.Key == ShareConfigKeyIndex && r.Value != custom {
			t.Errorf("Seed 覆盖了管理员改过的值: %s", r.Value)
		}
	}

	// 读取应看到自定义首页 + 两页默认
	got := svc.Get(ctx)
	if got["index"].Title != "管理员改过的标题" {
		t.Errorf("index 未读到自定义值: %+v", got["index"])
	}
	if got["simulator"] != shareCardDefaults["simulator"] {
		t.Errorf("simulator 应为默认: %+v", got["simulator"])
	}
}

// TestShareCfg_Get_TenantFallbackToZero 后台写在租户 0，公开端点 ctx 被域名解析成非 0 租户也要读到。
func TestShareCfg_Get_TenantFallbackToZero(t *testing.T) {
	svc, db := newShareCfgForTest(t)

	raw, _ := json.Marshal(ShareCard{Title: "平台租户的标题", ImageURL: "https://example.com/p.png"})
	if err := db.Create(&sysmodel.SystemConfig{
		Key: ShareConfigKeySalary, Value: string(raw), Name: "分享卡 · 工资记账", TenantID: 0,
	}).Error; err != nil {
		t.Fatal(err)
	}

	// 租户 5 的 ctx（其名下无此 key）→ 应回落读到租户 0 的行
	got := svc.Get(tenant.WithTenantID(context.Background(), 5))
	if got["salary"].Title != "平台租户的标题" {
		t.Errorf("跨租户回落失败: %+v", got["salary"])
	}
}

// TestShareCfg_Get_BadJSONFallsBack 坏 value 单页回默认，不炸整个接口、不影响其他页。
func TestShareCfg_Get_BadJSONFallsBack(t *testing.T) {
	svc, db := newShareCfgForTest(t)
	ctx := tenant.WithTenantID(context.Background(), 0)

	if err := db.Create(&sysmodel.SystemConfig{
		Key: ShareConfigKeySimulator, Value: "not-json{{", Name: "分享卡 · 座位模拟器", TenantID: 0,
	}).Error; err != nil {
		t.Fatal(err)
	}

	got := svc.Get(ctx)
	if got["simulator"] != shareCardDefaults["simulator"] {
		t.Errorf("坏 JSON 应回退默认: %+v", got["simulator"])
	}
	if got["index"] != shareCardDefaults["index"] {
		t.Errorf("坏 JSON 不应影响其他页: %+v", got["index"])
	}
}

// TestShareCfg_Get_EmptyTitleFillsDefault JSON 合法但 title 空 → 补默认 title（后台误清空不产空卡片）。
func TestShareCfg_Get_EmptyTitleFillsDefault(t *testing.T) {
	svc, db := newShareCfgForTest(t)
	ctx := tenant.WithTenantID(context.Background(), 0)

	if err := db.Create(&sysmodel.SystemConfig{
		Key: ShareConfigKeyIndex, Value: `{"title":"","imageUrl":"https://example.com/y.png"}`,
		Name: "分享卡 · 首页", TenantID: 0,
	}).Error; err != nil {
		t.Fatal(err)
	}

	got := svc.Get(ctx)
	if got["index"].Title != shareCardDefaults["index"].Title {
		t.Errorf("空 title 应回补默认 title: %+v", got["index"])
	}
	if got["index"].ImageURL != "https://example.com/y.png" {
		t.Errorf("imageUrl 应保留自定义值: %+v", got["index"])
	}
}
