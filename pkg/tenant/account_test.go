package tenant_test

import (
	"context"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/tenant/gormplugin"
)

// setupAccountDB sqlite 内存库 + tenants/system_users 表；
// withMemberTable=true 时再建 member_user（false 模拟纯 system 场景缺 member 表）。
func setupAccountDB(t *testing.T, withMemberTable bool) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	stmts := []string{
		`CREATE TABLE tenants (
			id INTEGER PRIMARY KEY,
			name TEXT,
			account_limit INTEGER NOT NULL DEFAULT -1,
			deleted INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE system_users (
			id INTEGER PRIMARY KEY,
			tenant_id INTEGER NOT NULL,
			deleted INTEGER NOT NULL DEFAULT 0
		)`,
	}
	if withMemberTable {
		stmts = append(stmts, `CREATE TABLE member_user (
			id INTEGER PRIMARY KEY,
			tenant_id INTEGER NOT NULL,
			deleted INTEGER NOT NULL DEFAULT 0
		)`)
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

// seedTenant 落一行租户（deleted 由调用方给 0/1）。
func seedTenant(t *testing.T, db *gorm.DB, id uint64, limit int, deleted int) {
	t.Helper()
	if err := db.Exec(
		`INSERT INTO tenants (id, name, account_limit, deleted) VALUES (?, ?, ?, ?)`,
		id, "t", limit, deleted,
	).Error; err != nil {
		t.Fatal(err)
	}
}

// seedAccount 向 system_users/member_user 落一行账号（deleted 由调用方给 0/1）。
func seedAccount(t *testing.T, db *gorm.DB, table string, id, tenantID uint64, deleted int) {
	t.Helper()
	if err := db.Exec(
		`INSERT INTO `+table+` (id, tenant_id, deleted) VALUES (?, ?, ?)`,
		id, tenantID, deleted,
	).Error; err != nil {
		t.Fatal(err)
	}
}

// 超限拒：limit=1 且已有 1 个账号 → 触额返回错误（文案与 system 侧一致）。
func TestCheckAccountLimit_OverLimitRejected(t *testing.T) {
	db := setupAccountDB(t, true)
	seedTenant(t, db, 1, 1, 0)
	seedAccount(t, db, "system_users", 10, 1, 0)

	err := tenant.NewAccountLimitChecker(db).CheckAccountLimit(context.Background(), 1)
	if err == nil {
		t.Fatal("超限必须返回 error")
	}
	if !strings.Contains(err.Error(), "account limit reached (1)") {
		t.Fatalf("error = %v, want 含 account limit reached (1)", err)
	}
}

// 未超放：limit=2 且仅 1 个账号 → 放行。
func TestCheckAccountLimit_UnderLimitAllowed(t *testing.T) {
	db := setupAccountDB(t, true)
	seedTenant(t, db, 1, 2, 0)
	seedAccount(t, db, "system_users", 10, 1, 0)

	if err := tenant.NewAccountLimitChecker(db).CheckAccountLimit(context.Background(), 1); err != nil {
		t.Fatalf("未超限必须放行, got %v", err)
	}
}

// limit=-1 不限：即使账号很多也放行。
func TestCheckAccountLimit_NegativeLimitUnlimited(t *testing.T) {
	db := setupAccountDB(t, true)
	seedTenant(t, db, 1, -1, 0)
	for i := uint64(1); i <= 5; i++ {
		seedAccount(t, db, "system_users", i, 1, 0)
	}

	if err := tenant.NewAccountLimitChecker(db).CheckAccountLimit(context.Background(), 1); err != nil {
		t.Fatalf("limit=-1 必须放行, got %v", err)
	}
}

// tid=0（平台租户）放行：空库无表也不查库、不报错。
func TestCheckAccountLimit_PlatformTenantTidZero(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := tenant.NewAccountLimitChecker(db).CheckAccountLimit(context.Background(), 0); err != nil {
		t.Fatalf("tid=0 必须放行, got %v", err)
	}
}

// member 缺表按 0 容错：无 member_user 表、system_users 未满 → 放行（不因缺表报错）。
func TestCheckAccountLimit_MemberTableMissingCountsZero(t *testing.T) {
	db := setupAccountDB(t, false)
	seedTenant(t, db, 1, 1, 0)

	if err := tenant.NewAccountLimitChecker(db).CheckAccountLimit(context.Background(), 1); err != nil {
		t.Fatalf("member 缺表必须按 0 计而非报错, got %v", err)
	}
}

// 双表合计：system_users(1) + member_user(1) >= limit=2 → 拒。
func TestCheckAccountLimit_CombinedTablesRejected(t *testing.T) {
	db := setupAccountDB(t, true)
	seedTenant(t, db, 1, 2, 0)
	seedAccount(t, db, "system_users", 10, 1, 0)
	seedAccount(t, db, "member_user", 20, 1, 0)

	err := tenant.NewAccountLimitChecker(db).CheckAccountLimit(context.Background(), 1)
	if err == nil {
		t.Fatal("system_users+member_user 合计触额必须拒绝")
	}
	if !strings.Contains(err.Error(), "account limit reached (2)") {
		t.Fatalf("error = %v, want 含 account limit reached (2)", err)
	}
}

// 软删账号不计数：limit=2、1 活 + 1 删 → 合计 1 < 2 → 放行。
func TestCheckAccountLimit_SoftDeletedAccountsNotCounted(t *testing.T) {
	db := setupAccountDB(t, true)
	seedTenant(t, db, 1, 2, 0)
	seedAccount(t, db, "system_users", 10, 1, 0)
	seedAccount(t, db, "system_users", 11, 1, 1)

	if err := tenant.NewAccountLimitChecker(db).CheckAccountLimit(context.Background(), 1); err != nil {
		t.Fatalf("deleted=1 账号不应计入, got %v", err)
	}
}

// 租户不存在 / 已软删 → 错误（与 system 侧 GetByID not found 语义一致）。
func TestCheckAccountLimit_TenantNotFound(t *testing.T) {
	db := setupAccountDB(t, true)
	seedTenant(t, db, 7, -1, 1) // 已软删

	for _, tid := range []uint64{99, 7} {
		err := tenant.NewAccountLimitChecker(db).CheckAccountLimit(context.Background(), tid)
		if err == nil {
			t.Fatalf("tid=%d 租户不存在/已删必须返回 error", tid)
		}
		if !strings.Contains(err.Error(), "tenant not found") {
			t.Fatalf("tid=%d error = %v, want 含 tenant not found", tid, err)
		}
	}
}

// 与生产同款装配（gormplugin 注册 + ctx 带租户编号）：
// 插件不得干扰按显式 tenantID 的额度计数——超限拒、未超放照常。
func TestCheckAccountLimit_WithTenantPlugin(t *testing.T) {
	db := setupAccountDB(t, true)
	if err := gormplugin.Register(db, gormplugin.Options{
		ExemptTables: []string{"tenants", "casbin_rule"},
	}); err != nil {
		t.Fatal(err)
	}
	seedTenant(t, db, 1, 1, 0)
	seedAccount(t, db, "system_users", 10, 1, 0)
	ctx := tenant.WithTenantID(context.Background(), 1)

	if err := tenant.NewAccountLimitChecker(db).CheckAccountLimit(ctx, 1); err == nil {
		t.Fatal("插件注册下超限仍须拒绝")
	}
	seedTenant(t, db, 2, 5, 0)
	if err := tenant.NewAccountLimitChecker(db).CheckAccountLimit(ctx, 2); err != nil {
		t.Fatalf("插件注册下未超限须放行, got %v", err)
	}
}
