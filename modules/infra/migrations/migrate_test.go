//go:build integration

package migrations

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"alexGo-cloud/modules/infra"
	"alexGo-cloud/modules/system"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/migrate"
)

// chdirRepoRoot 切到仓库根：Runner 的迁移源路径是 file://modules/infra/migrations，
// 相对进程 cwd 解析；测试 cwd 在包目录下，不切则 migrate init 失败。
// （Runner 吞掉 init/up 错误返回 nil，症状表现为「表没建」而非报错。）
func chdirRepoRoot(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// 幂等：一个测试里 openMultiStmtDB + runSystem 会各进一次，
	// 第二次已在根则不再上跳三级（否则逃出仓库，相对路径全断）。
	if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
		return
	}
	root := filepath.Clean(filepath.Join(wd, "..", "..", ".."))
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
}

// runInfra 用 DB_DSN 构造 Runner 只跑 infra 迁移源，返回 gorm 句柄供断言。
func runInfra(t *testing.T) (*gorm.DB, *migrate.Runner) {
	t.Helper()
	chdirRepoRoot(t)
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		t.Skip("DB_DSN not set; skip integration test")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("mysql unreachable: %v", err)
	}
	cfg := &config.Config{}
	cfg.Migrate.Auto = true
	cfg.Database.DSN = dsn
	runner := migrate.NewRunner(db, cfg, []migrate.Source{infra.NewMigrationSource()})
	return db, runner
}

// runSystem 与 runInfra 同构，但只跑 system 迁移源（含 20261011000002 codegen 菜单种子）。
func runSystem(t *testing.T) (*gorm.DB, *migrate.Runner) {
	t.Helper()
	chdirRepoRoot(t)
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		t.Skip("DB_DSN not set; skip integration test")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("mysql unreachable: %v", err)
	}
	cfg := &config.Config{}
	cfg.Migrate.Auto = true
	cfg.Database.DSN = dsn
	runner := migrate.NewRunner(db, cfg, []migrate.Source{system.NewMigrationSource()})
	return db, runner
}

func countCodegenRows(t *testing.T, db *gorm.DB) (tables, columns int64) {
	t.Helper()
	if !db.Migrator().HasTable("codegen_table") || !db.Migrator().HasTable("codegen_column") {
		t.Fatal("codegen_table / codegen_column 未建表")
	}
	if err := db.Table("codegen_table").Count(&tables).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("codegen_column").Count(&columns).Error; err != nil {
		t.Fatal(err)
	}
	return tables, columns
}

// 双跑幂等：第一次跑建表，第二次跑行数与迁移记录均不增长；
// 系统级配置表不得有 tenant_id / deleted 列（spec §1 非目标 + §9.3）。
func TestInfraMigrations_UpTwiceStable(t *testing.T) {
	db, runner := runInfra(t)

	if err := runner.Run(); err != nil {
		t.Fatalf("run #1: %v", err)
	}
	t1, c1 := countCodegenRows(t, db)

	var n1 int64
	if err := db.Table("schema_migrations_infra").Count(&n1).Error; err != nil {
		t.Fatal(err)
	}
	if n1 < 1 {
		t.Fatal("schema_migrations_infra 无记录")
	}

	if err := runner.Run(); err != nil {
		t.Fatalf("run #2: %v", err)
	}
	t2, c2 := countCodegenRows(t, db)
	if t2 != t1 || c2 != c1 {
		t.Errorf("双跑行数变化: tables %d→%d, columns %d→%d", t1, t2, c1, c2)
	}
	var n2 int64
	if err := db.Table("schema_migrations_infra").Count(&n2).Error; err != nil {
		t.Fatal(err)
	}
	if n2 != n1 {
		t.Errorf("迁移记录 %d→%d, want 不变", n1, n2)
	}

	for _, col := range []string{"tenant_id", "deleted"} {
		if db.Migrator().HasColumn("codegen_table", col) {
			t.Errorf("codegen_table 含禁用列 %q（spec §1/§9.3：系统级、硬删除）", col)
		}
	}
}

// 菜单种子双跑幂等：两次 run 后 infra 菜单恒为 5 行
// （1 目录『代码生成』按 name 匹配 + 1 页面 + 3 按钮按 permission 匹配）。
func TestInfraSeedMenus_UpTwiceIdempotent(t *testing.T) {
	db, runner := runSystem(t)

	if err := runner.Run(); err != nil {
		t.Fatalf("run #1: %v", err)
	}
	count := func() int64 {
		var n int64
		if err := db.Table("menus").
			Where("permission LIKE 'infra:codegen%' OR (name = '代码生成' AND type = 'dir' AND deleted = 0)").
			Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		return n
	}
	first := count()
	if first != 5 {
		t.Errorf("菜单数 = %d, want 5（目录+页面+3按钮）", first)
	}
	if err := runner.Run(); err != nil {
		t.Fatalf("run #2: %v", err)
	}
	if second := count(); second != first {
		t.Errorf("双跑幂等破坏: %d → %d", first, second)
	}
}

// openMultiStmtDB 打开带 multiStatements 的裸连接：直跑迁移文件原文需要
// 一条连接执行多语句（评审 Important#3——golang-migrate Up() 只跑 pending
// 版本，runner 双跑永远测不出守卫缺失）。
func openMultiStmtDB(t *testing.T) *sql.DB {
	t.Helper()
	chdirRepoRoot(t)
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		t.Skip("DB_DSN not set; skip integration test")
	}
	if strings.Contains(dsn, "?") {
		dsn += "&multiStatements=true"
	} else {
		dsn += "?multiStatements=true"
	}
	raw, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := raw.Ping(); err != nil {
		t.Skipf("mysql unreachable: %v", err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	return raw
}

const seedSQLPath = "modules/system/migrations/20261011000002_codegen_menus.up.sql"

// seedMenuCount 与 runner 双跑测试同口径的种子行计数。
func seedMenuCount(t *testing.T, raw *sql.DB) int64 {
	t.Helper()
	var n int64
	if err := raw.QueryRow(`SELECT COUNT(*) FROM menus
		WHERE permission LIKE 'infra:codegen%' OR (name = '代码生成' AND type = 'dir' AND deleted = 0)`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// 直跑 up.sql 原文两遍：这是对 NOT EXISTS 守卫的真断言——
// 守卫被删时本测试必须失败（runner 双跑测不出，见评审 Important#3）。
func TestInfraSeedMenus_SQLFileExecutedTwiceStable(t *testing.T) {
	raw := openMultiStmtDB(t)
	_, runner := runSystem(t)
	if err := runner.Run(); err != nil {
		t.Fatalf("baseline run: %v", err)
	}
	src, err := os.ReadFile(seedSQLPath)
	if err != nil {
		t.Fatal(err)
	}
	before := seedMenuCount(t, raw)
	for i := 1; i <= 2; i++ {
		if _, err := raw.Exec(string(src)); err != nil {
			t.Fatalf("直跑 SQL #%d: %v", i, err)
		}
	}
	after := seedMenuCount(t, raw)
	if after != before {
		t.Errorf("直跑两遍行数变化: %d → %d", before, after)
	}
	if after != 5 {
		t.Errorf("菜单数 = %d, want 5（目录+页面+3按钮）", after)
	}
}

// 按钮 NOT EXISTS 必须限定 tenant_id=0：其他租户同 permission 的行
// 不得挡住 tenant 0 按钮落库（up.sql ③ 缺租户过滤的缺陷，评审 Important#3）。
func TestInfraSeedMenus_ButtonNotBlockedByOtherTenant(t *testing.T) {
	raw := openMultiStmtDB(t)
	_, runner := runSystem(t)
	if err := runner.Run(); err != nil {
		t.Fatalf("baseline run: %v", err)
	}

	// 清出靶子：删 tenant 0 按钮（连带角色绑定），清理函数负责还原
	// （无论缺陷是否已修，都要把 DB 还原成 5 行，避免污染同包后续测试）。
	if _, err := raw.Exec(`DELETE rm FROM role_menus rm JOIN menus m ON m.id = rm.menu_id
		WHERE m.tenant_id = 0 AND m.permission LIKE 'infra:codegen:%'`); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`DELETE FROM menus
		WHERE tenant_id = 0 AND type = 'button' AND permission LIKE 'infra:codegen:%'`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { restoreSeedMenus(t, raw) })

	// 跨租户诱饵：tenant 999 的同名 permission 行，deleted=0
	if _, err := raw.Exec(`INSERT INTO menus
		(tenant_id,parent_id,type,name,path,component,icon,permission,sort,status,deleted,created_at,updated_at)
		VALUES (999, 0, 'button', '诱饵', '', '', '', 'infra:codegen:import', 1, 1, 0, NOW(), NOW())`); err != nil {
		t.Fatal(err)
	}

	src, err := os.ReadFile(seedSQLPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(string(src)); err != nil {
		t.Fatalf("直跑 SQL: %v", err)
	}
	var n int64
	if err := raw.QueryRow(`SELECT COUNT(*) FROM menus
		WHERE tenant_id = 0 AND permission = 'infra:codegen:import' AND deleted = 0`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("tenant 0 导入按钮 = %d, want 1（tenant 999 诱饵不该挡住）", n)
	}
}

// restoreSeedMenus 还原种子到 5 行 + admin 绑定，容忍任意执行顺序/缺陷状态。
func restoreSeedMenus(t *testing.T, raw *sql.DB) {
	t.Helper()
	_, _ = raw.Exec(`DELETE FROM menus WHERE tenant_id <> 0 AND permission LIKE 'infra:codegen%'`)
	for _, row := range []struct {
		name string
		perm string
		sort int
	}{{"导入", "infra:codegen:import", 1}, {"同步", "infra:codegen:sync", 2}, {"删除", "infra:codegen:delete", 3}} {
		if _, err := raw.Exec(`INSERT INTO menus
			(tenant_id,parent_id,type,name,path,component,icon,permission,sort,status,deleted,created_at,updated_at)
			SELECT 0, m.id, 'button', ?, '', '', '', ?, ?, 1, 0, NOW(), NOW()
			FROM menus m
			WHERE m.tenant_id = 0 AND m.permission = 'infra:codegen' AND m.type = 'menu' AND m.deleted = 0
			AND NOT EXISTS (SELECT 1 FROM menus b WHERE b.tenant_id = 0 AND b.permission = ? AND b.deleted = 0)`,
			row.name, row.perm, row.sort, row.perm); err != nil {
			t.Errorf("还原按钮 %s: %v", row.name, err)
		}
	}
	if _, err := raw.Exec(`INSERT INTO role_menus (role_id, menu_id, tenant_id)
		SELECT r.id, m.id, 0 FROM menus m
		JOIN roles r ON r.tenant_id = 0 AND r.code = 'admin' AND r.deleted = 0
		WHERE m.tenant_id = 0 AND m.deleted = 0
		  AND (m.path IN ('/codegen','/codegen/config') OR m.permission LIKE 'infra:codegen%')
		  AND NOT EXISTS (SELECT 1 FROM role_menus rm WHERE rm.tenant_id = 0 AND rm.role_id = r.id AND rm.menu_id = m.id)`); err != nil {
		t.Errorf("还原角色绑定: %v", err)
	}
}
