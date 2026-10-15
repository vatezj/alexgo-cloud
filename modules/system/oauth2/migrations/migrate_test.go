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

	"alexGo-cloud/modules/system/oauth2"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/migrate"
)

func chdirRepoRoot(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
		return
	}
	root := filepath.Clean(filepath.Join(wd, "..", "..", "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("chdirRepoRoot 上跳后不在仓库根（缺 go.mod）：%s", root)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
}

// runOAuth2 用 DB_DSN 构造 Runner 只跑 oauth2 迁移源。
func runOAuth2(t *testing.T) (*gorm.DB, *migrate.Runner) {
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
	runner := migrate.NewRunner(db, cfg, []migrate.Source{oauth2.NewMigrationSource()})
	return db, runner
}

// 双表建立 + 双跑稳定；oauth2_client 无 tenant_id/deleted（D4 硬删）。
func TestOAuth2Migrations_UpTwiceStable(t *testing.T) {
	db, runner := runOAuth2(t)
	if err := runner.Run(); err != nil {
		t.Fatalf("run #1: %v", err)
	}
	for _, tbl := range []string{"oauth2_client", "oauth2_authorize_code"} {
		if !db.Migrator().HasTable(tbl) {
			t.Fatalf("表 %s 未建立", tbl)
		}
	}
	if db.Migrator().HasColumn("oauth2_client", "tenant_id") || db.Migrator().HasColumn("oauth2_client", "deleted") {
		t.Fatal("oauth2_client 不得有 tenant_id/deleted（D4）")
	}
	var c1, a1 int64
	db.Table("oauth2_client").Count(&c1)
	db.Table("oauth2_authorize_code").Count(&a1)
	if err := runner.Run(); err != nil {
		t.Fatalf("run #2: %v", err)
	}
	var c2, a2 int64
	db.Table("oauth2_client").Count(&c2)
	db.Table("oauth2_authorize_code").Count(&a2)
	if c1 != c2 || a1 != a2 {
		t.Errorf("双跑行数变化: client %d→%d, code %d→%d", c1, c2, a1, a2)
	}
	if c1 != 2 {
		t.Errorf("种子 client = %d, want 2（alexgo-admin + alexgo-app）", c1)
	}
}

// 种子 SQL 直跑双次幂等（绕开 golang-migrate 只跑 pending 的真空坑）。
const seedClientsPath = "modules/system/oauth2/migrations/20261011000002_oauth2_seed_clients.up.sql"

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

func TestOAuth2SeedClients_SQLFileExecutedTwiceStable(t *testing.T) {
	raw := openMultiStmtDB(t)
	_, runner := runOAuth2(t)
	if err := runner.Run(); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	src, err := os.ReadFile(seedClientsPath)
	if err != nil {
		t.Fatal(err)
	}
	count := func() int64 {
		var n int64
		if err := raw.QueryRow(`SELECT COUNT(*) FROM oauth2_client WHERE client_id IN ('alexgo-admin','alexgo-app')`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	before := count()
	for i := 1; i <= 2; i++ {
		if _, err := raw.Exec(string(src)); err != nil {
			t.Fatalf("直跑 SQL #%d: %v", i, err)
		}
	}
	if after := count(); after != before || after != 2 {
		t.Errorf("种子行数 before=%d after=%d, want 恒 2", before, after)
	}
}

// 后台 SSO 登录的 redirect_uri 由 window.location.origin 派生（vben dev 端口 5666），
// 种子/回填必须覆盖 http://localhost:5666/callback，否则登录卡在白名单 400。
func TestOAuth2Migrations_AdminRedirectWhitelist_CoversDevOrigin(t *testing.T) {
	db, runner := runOAuth2(t)
	if err := runner.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	var uris string
	if err := db.Raw(`SELECT redirect_uris FROM oauth2_client WHERE client_id='alexgo-admin'`).
		Scan(&uris).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(uris, "http://localhost:5666/callback") {
		t.Errorf("redirect_uris = %q, want 含 http://localhost:5666/callback（vben dev origin）", uris)
	}
}
