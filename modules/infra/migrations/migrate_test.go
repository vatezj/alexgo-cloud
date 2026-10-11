//go:build integration

package migrations

import (
	"os"
	"path/filepath"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"alexGo-cloud/modules/infra"
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
