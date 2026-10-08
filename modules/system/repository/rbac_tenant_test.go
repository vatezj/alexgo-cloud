package repository

import (
	"context"
	"reflect"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/tenant/gormplugin"
)

// setupRoleMenuDB：sqlite :memory: + role_menus 表 + 租户隔离插件（与生产 cmd/main.go 同款装配，
// ExemptTables 不含 role_menus → 插件会按 ctx 租户注入 tenant_id 条件）。
func setupRoleMenuDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.RoleMenu{}); err != nil {
		t.Fatal(err)
	}
	if err := gormplugin.Register(db, gormplugin.Options{}); err != nil {
		t.Fatal(err)
	}
	// 落两行：role 100 → menu 1（租户1）、role 200 → menu 2（租户2）。
	// 无租户 ctx 插入，插件不回填；显式 tenant_id 不被覆盖。
	rows := []model.RoleMenu{
		{RoleID: 100, MenuID: 1, TenantID: 1},
		{RoleID: 200, MenuID: 2, TenantID: 2},
	}
	if err := db.WithContext(context.Background()).Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func idsEqual(a, b []uint64) bool { return reflect.DeepEqual(a, b) }

// Case A：ctx 租户与显式 tenantID 一致 → 插件条件与 repo 过滤同值，命中。
func TestListMenuIDsByRoleIDs_ConsistentTenant(t *testing.T) {
	db := setupRoleMenuDB(t)
	r := &roleMenuRepo{db: db}

	got, err := r.ListMenuIDsByRoleIDs(tenant.WithTenantID(context.Background(), 1), 1, []uint64{100})
	if err != nil {
		t.Fatal(err)
	}
	if !idsEqual(got, []uint64{1}) {
		t.Errorf("consistent tenant: got %v, want [1]", got)
	}
}

// Case B（the bug 判别）：role 属租户2、调用者 ctx 是租户1。
// 修前：插件按 ctx 注入 tenant_id=1，repo 又显式过滤 tenant_id=2 → 两条件矛盾 → 返回空，
// 该角色菜单读不到 → 全局 p 清空后其策略永不回填。
// 修后：必须返回 [2]（role_menus 是租户隔离的，但读取须以显式 tenantID 为准）。
func TestListMenuIDsByRoleIDs_CrossTenantCaller(t *testing.T) {
	db := setupRoleMenuDB(t)
	r := &roleMenuRepo{db: db}

	got, err := r.ListMenuIDsByRoleIDs(tenant.WithTenantID(context.Background(), 1), 2, []uint64{200})
	if err != nil {
		t.Fatal(err)
	}
	if !idsEqual(got, []uint64{2}) {
		t.Errorf("cross-tenant caller: got %v, want [2] (role tenant 2 readable despite caller ctx tenant 1)", got)
	}
}
