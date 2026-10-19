package newmodule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNextMigrationVersion(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)

	// 空目录 → 日期起版
	v, err := nextMigrationVersion(dir, now)
	if err != nil || v != "20261019000001" {
		t.Fatalf("empty dir: v=%q err=%v", v, err)
	}

	// 同日已有 000001 → 递增
	for _, f := range []string{"20261019000001_x.up.sql", "20261019000001_x.down.sql"} {
		if err := os.WriteFile(filepath.Join(dir, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	v, err = nextMigrationVersion(dir, now)
	if err != nil || v != "20261019000002" {
		t.Fatalf("same day: v=%q err=%v", v, err)
	}

	// 已有未来版本（如 20261231000001）→ 超过它
	if err := os.WriteFile(filepath.Join(dir, "20261231000001_future.up.sql"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	v, err = nextMigrationVersion(dir, now)
	if err != nil || v != "20261231000002" {
		t.Fatalf("future version: v=%q err=%v", v, err)
	}
}

func TestShellFiles(t *testing.T) {
	o := Options{Root: repoRoot(t), Name: "demo", Entities: []string{"product", "category"},
		Now: time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)}
	if err := o.Validate(); err != nil {
		t.Fatal(err)
	}
	files, err := shellFiles(o)
	if err != nil {
		t.Fatalf("shellFiles() = %v", err)
	}
	byPath := map[string]string{}
	for _, f := range files {
		byPath[f.Path] = string(f.Content)
	}
	// module.go
	mg, ok := byPath["modules/demo/module.go"]
	if !ok {
		t.Fatal("missing modules/demo/module.go")
	}
	for _, want := range []string{
		`fx.Module("demo"`,
		"repository.NewProductRepository",
		"repository.NewCategoryRepository",
		"service.NewCategoryService",
		"admin.NewAdminCategoryController",
		"app.NewAppCategoryController",
		`NewMigrationSource`,
		"RegisterProductDBRoutes(r, m.ProductAdminCtrl)",
		"RegisterCategoryDBRoutes(r, m.CategoryAdminCtrl)",
		`r.Group("/app/demo")`,
		`m.cfg.Modules["demo"]`,
	} {
		if !strings.Contains(mg, want) {
			t.Errorf("module.go missing %q", want)
		}
	}
	// migration_source.go
	ms := byPath["modules/demo/migration_source.go"]
	if !strings.Contains(ms, `"modules/demo/migrations"`) || !strings.Contains(ms, `"demo"`) {
		t.Error("migration_source.go content wrong")
	}
	// 实体迁移 up：表名与 6 列
	up := byPath["modules/demo/migrations/20261019000001_products.up.sql"]
	if !strings.Contains(up, "CREATE TABLE `products`") ||
		!strings.Contains(up, "`tenant_id`") || !strings.Contains(up, "`status`") {
		t.Errorf("products up.sql wrong:\n%s", up)
	}
	up2 := byPath["modules/demo/migrations/20261019000002_categories.up.sql"]
	if !strings.Contains(up2, "CREATE TABLE `categories`") {
		t.Errorf("categories up.sql wrong:\n%s", up2)
	}
	// down
	if !strings.Contains(byPath["modules/demo/migrations/20261019000001_products.down.sql"], "DROP TABLE IF EXISTS `products`") {
		t.Error("products down.sql wrong")
	}
	// module 私有配置
	if !strings.Contains(byPath["modules/demo/configs/config.yaml"], "demo") {
		t.Error("module config.yaml wrong")
	}
	// module.go 必须是合法 Go（FormatGo 已跑，能 parse 即证）
}
