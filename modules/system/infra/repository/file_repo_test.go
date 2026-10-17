package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/system/infra/model"
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/tenant/gormplugin"
)

func setupFileRepo(t *testing.T) (*gorm.DB, FileRepository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.SystemFile{}); err != nil {
		t.Fatal(err)
	}
	return db, NewFileRepository(db)
}

func sampleFile(tenantID uint64, key string) *model.SystemFile {
	return &model.SystemFile{
		Name: "照片.jpg", Ext: "jpg", Mime: "image/jpeg", Size: 12,
		RelPath: "2026/10/deadbeef.jpg", PublicKey: key,
		Public: true, BizType: "notice", TenantID: tenantID,
	}
}

// CRUD + 租户边界：admin 读按 tenant 隔离；GetByKey 不受租户过滤（public 读唯一例外）。
func TestFileRepo_CRUDAndTenantBoundary(t *testing.T) {
	db, repo := setupFileRepo(t)
	ctx1 := tenant.WithTenantID(context.Background(), 1)

	f := sampleFile(1, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err := repo.Create(ctx1, f); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// 本租户命中
	got, err := repo.GetByID(ctx1, 1, f.ID)
	if err != nil || got.Name != "照片.jpg" {
		t.Fatalf("GetByID own tenant: got=%+v err=%v", got, err)
	}
	// 跨租户查无
	if _, err := repo.GetByID(ctx1, 99, f.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("GetByID cross tenant err = %v, want ErrRecordNotFound", err)
	}
	// GetByKey 不带租户条件：另一个租户的 ctx 同样命中（Review Focus ① 的仓库侧半边）
	ctx99 := tenant.WithTenantID(context.Background(), 99)
	gk, err := repo.GetByKey(ctx99, f.PublicKey)
	if err != nil || gk.ID != f.ID {
		t.Errorf("GetByKey cross tenant ctx: got=%+v err=%v, want hit", gk, err)
	}
	// key 不存在
	if _, err := repo.GetByKey(ctx1, "ffffffffffffffffffffffffffffffff"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("GetByKey missing err = %v, want ErrRecordNotFound", err)
	}
	// 删除按租户隔离：错误租户删不掉
	if err := repo.Delete(ctx1, 99, f.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("Delete wrong tenant err = %v, want ErrRecordNotFound", err)
	}
	var n int64
	db.Model(&model.SystemFile{}).Count(&n)
	if n != 1 {
		t.Fatalf("row count = %d after wrong-tenant delete, want 1", n)
	}
	if err := repo.Delete(ctx1, 1, f.ID); err != nil {
		t.Fatalf("Delete own tenant: %v", err)
	}
	if _, err := repo.GetByID(ctx1, 1, f.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("GetByID after delete err = %v, want ErrRecordNotFound", err)
	}
}

// 终审 Critical 2 的 pin：注册 gormplugin（生产装配，main.go 同款 Options）后，
// ctx 带租户的 Query 会自动追加 tenant_id 条件——GetByKey 必须 IgnoreTenant
// 放行，否则 /api/public 下跨租户读恒 gorm.ErrRecordNotFound → 30010。
// 对照组：GetByID/Create/Delete 在插件下仍保持租户过滤。
func TestFileRepo_GetByKey_IgnoresTenantUnderPlugin(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.SystemFile{}); err != nil {
		t.Fatal(err)
	}
	// 与 alexgo-server/cmd/main.go 生产注册一致。
	if err := gormplugin.Register(db, gormplugin.Options{
		ExemptTables: []string{"tenants", "casbin_rule"},
	}); err != nil {
		t.Fatal(err)
	}
	repo := NewFileRepository(db)

	ctx1 := tenant.WithTenantID(context.Background(), 1)
	f := sampleFile(1, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if err := repo.Create(ctx1, f); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// 插件在位 + ctx 带租户 99：GetByKey 仍命中租户 1 的行。
	ctx99 := tenant.WithTenantID(context.Background(), 99)
	gk, err := repo.GetByKey(ctx99, f.PublicKey)
	if err != nil || gk.ID != f.ID || gk.TenantID != 1 {
		t.Fatalf("GetByKey under plugin with foreign tenant ctx: got=%+v err=%v, want hit tenant 1 row", gk, err)
	}

	// 对照：其余方法仍被插件 + 仓库双层租户过滤。
	if _, err := repo.GetByID(ctx99, 99, f.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("GetByID cross tenant under plugin err = %v, want ErrRecordNotFound", err)
	}
	if err := repo.Delete(ctx99, 1, f.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("Delete cross tenant under plugin err = %v, want ErrRecordNotFound", err)
	}
	// 本租户入口不受影响（对照组仍可用）。
	if got, err := repo.GetByID(ctx1, 1, f.ID); err != nil || got.ID != f.ID {
		t.Errorf("GetByID own tenant under plugin: got=%+v err=%v, want hit", got, err)
	}
}
