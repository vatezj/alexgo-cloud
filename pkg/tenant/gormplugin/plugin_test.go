package gormplugin

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/pkg/tenant"
)

type row struct {
	ID       uint64 `gorm:"primaryKey"`
	Name     string
	TenantID uint64 `gorm:"column:tenant_id"`
}

func (row) TableName() string { return "rows" }

type noTenantRow struct {
	ID   uint64 `gorm:"primaryKey"`
	Name string
}

func (noTenantRow) TableName() string { return "no_tenant_rows" }

func setup(t *testing.T, opts Options) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&row{}, &noTenantRow{}); err != nil {
		t.Fatal(err)
	}
	if err := Register(db, opts); err != nil {
		t.Fatal(err)
	}
	return db
}

// INSERT 自动填 tenant_id；SELECT 只见本租户行（跨租户行互不可见）。
func TestIsolation_CrossTenant(t *testing.T) {
	db := setup(t, Options{})

	ctx1 := tenant.WithTenantID(context.Background(), 1)
	ctx2 := tenant.WithTenantID(context.Background(), 2)

	if err := db.WithContext(ctx1).Create(&row{Name: "a"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx2).Create(&row{Name: "b"}).Error; err != nil {
		t.Fatal(err)
	}

	var got []row
	if err := db.WithContext(ctx1).Find(&got).Error; err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "a" || got[0].TenantID != 1 {
		t.Fatalf("tenant1 sees %+v, want only own row with injected tenant_id", got)
	}

	var got2 []row
	if err := db.WithContext(ctx2).Find(&got2).Error; err != nil {
		t.Fatal(err)
	}
	if len(got2) != 1 || got2[0].Name != "b" || got2[0].TenantID != 2 {
		t.Fatalf("tenant2 sees %+v", got2)
	}
}

// tid=0（未解析/平台）：不过滤不填充——保持历史行为：
// 平台侧显式写入他租户行（seed/回填）与 0 行都可见。
func TestTenantZero_NoInjection(t *testing.T) {
	db := setup(t, Options{})
	if err := db.WithContext(context.Background()).Create(&row{Name: "x"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(context.Background()).Create(&row{Name: "seed", TenantID: 5}).Error; err != nil {
		t.Fatal(err)
	}
	var got []row
	if err := db.WithContext(context.Background()).Find(&got).Error; err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("tid=0 must not filter: %+v", got)
	}
	for _, g := range got {
		if g.Name == "x" && g.TenantID != 0 {
			t.Fatalf("tid=0 must not inject: %+v", g)
		}
		if g.Name == "seed" && g.TenantID != 5 {
			t.Fatalf("explicit tenant_id must be preserved: %+v", g)
		}
	}
}

// 无 TenantID 字段的模型完全不受影响。
func TestNoTenantField_Skip(t *testing.T) {
	db := setup(t, Options{})
	if err := db.WithContext(tenant.WithTenantID(context.Background(), 9)).Create(&noTenantRow{Name: "n"}).Error; err != nil {
		t.Fatal(err)
	}
	var got []noTenantRow
	if err := db.WithContext(tenant.WithTenantID(context.Background(), 9)).Find(&got).Error; err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %+v", got)
	}
}

// 白名单表跳过过滤（也不回填）。
func TestExemptTable(t *testing.T) {
	db := setup(t, Options{ExemptTables: []string{"rows"}})
	if err := db.WithContext(tenant.WithTenantID(context.Background(), 1)).Create(&row{Name: "e", TenantID: 77}).Error; err != nil {
		t.Fatal(err)
	}
	// 零值行也不得被回填（白名单完全不参与注入）。
	if err := db.WithContext(tenant.WithTenantID(context.Background(), 1)).Create(&row{Name: "zero"}).Error; err != nil {
		t.Fatal(err)
	}
	var got []row
	if err := db.WithContext(tenant.WithTenantID(context.Background(), 2)).Find(&got).Error; err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("exempt table must not be filtered: %+v", got)
	}
	for _, g := range got {
		switch g.Name {
		case "e":
			if g.TenantID != 77 {
				t.Fatalf("explicit tenant_id must be preserved: %+v", g)
			}
		case "zero":
			if g.TenantID != 0 {
				t.Fatalf("exempt table must not be filled: %+v", g)
			}
		}
	}
}

// IgnoreTenant 显式放行（平台侧全量查询）：ctx 同时带 tenant_id 与 ignore 标记，
// 必须绕过过滤看到所有租户的行。
func TestIgnoreTenant(t *testing.T) {
	db := setup(t, Options{})
	if err := db.WithContext(tenant.WithTenantID(context.Background(), 1)).Create(&row{Name: "a"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(tenant.WithTenantID(context.Background(), 2)).Create(&row{Name: "b"}).Error; err != nil {
		t.Fatal(err)
	}

	// 不带 ignore：只见到本租户 1 行（对照组）。
	var scoped []row
	if err := db.WithContext(tenant.WithTenantID(context.Background(), 1)).Find(&scoped).Error; err != nil {
		t.Fatal(err)
	}
	if len(scoped) != 1 || scoped[0].Name != "a" {
		t.Fatalf("scoped query sees %+v, want only tenant1 row", scoped)
	}

	// 带 ignore：全量 2 行。
	var got []row
	if err := db.WithContext(IgnoreTenant(tenant.WithTenantID(context.Background(), 1))).Find(&got).Error; err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("IgnoreTenant sees %+v, want all", got)
	}
}

// Update/Delete 同样追加租户条件（本任务统一处理，T10 才区分 scope）。
func TestUpdateDelete_CrossTenantFiltered(t *testing.T) {
	db := setup(t, Options{})
	if err := db.WithContext(tenant.WithTenantID(context.Background(), 1)).Create(&row{Name: "a"}).Error; err != nil {
		t.Fatal(err)
	}
	var seed []row
	if err := db.WithContext(tenant.WithTenantID(context.Background(), 1)).Find(&seed).Error; err != nil || len(seed) != 1 {
		t.Fatalf("seed: %+v err=%v", seed, err)
	}
	id := seed[0].ID

	// 跨租户 UPDATE 不得命中。
	res := db.WithContext(tenant.WithTenantID(context.Background(), 2)).
		Model(&row{}).Where("id = ?", id).Update("name", "hacked")
	if res.Error != nil {
		t.Fatal(res.Error)
	}
	if res.RowsAffected != 0 {
		t.Fatalf("cross-tenant update affected %d rows, want 0", res.RowsAffected)
	}

	// 跨租户 DELETE 不得命中。
	res = db.WithContext(tenant.WithTenantID(context.Background(), 2)).Where("id = ?", id).Delete(&row{})
	if res.Error != nil {
		t.Fatal(res.Error)
	}
	if res.RowsAffected != 0 {
		t.Fatalf("cross-tenant delete affected %d rows, want 0", res.RowsAffected)
	}

	// 本租户 UPDATE/DELETE 正常。
	res = db.WithContext(tenant.WithTenantID(context.Background(), 1)).
		Model(&row{}).Where("id = ?", id).Update("name", "a2")
	if res.Error != nil || res.RowsAffected != 1 {
		t.Fatalf("owner update affected=%d err=%v", res.RowsAffected, res.Error)
	}
	res = db.WithContext(tenant.WithTenantID(context.Background(), 1)).Where("id = ?", id).Delete(&row{})
	if res.Error != nil || res.RowsAffected != 1 {
		t.Fatalf("owner delete affected=%d err=%v", res.RowsAffected, res.Error)
	}
}

// INSERT 回填仅零值：非白名单表上显式赋值的 tenant_id 不得被覆盖。
func TestInsert_ExplicitTenantIDPreserved(t *testing.T) {
	db := setup(t, Options{})
	if err := db.WithContext(tenant.WithTenantID(context.Background(), 1)).Create(&row{Name: "explicit", TenantID: 42}).Error; err != nil {
		t.Fatal(err)
	}
	var got []row
	// 过滤 tenant_id=1 查不到 42；必须用全量通道读回。
	if err := db.WithContext(IgnoreTenant(context.Background())).Find(&got).Error; err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].TenantID != 42 {
		t.Fatalf("explicit tenant_id must be preserved: %+v", got)
	}
}
