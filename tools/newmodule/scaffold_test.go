package newmodule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// copyRealTouchpoints 把 3 个既有文件拷进临时 Root，构造可落盘的最小仓库骨架。
func copyRealTouchpoints(t *testing.T, root string) {
	t.Helper()
	wd, _ := os.Getwd()
	real, err := repoRootDir(wd)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"alexgo-server/cmd/main.go",
		"alexgo-server/configs/config.yaml",
		"modules/system/service/permission_routes.go",
	} {
		src, err := os.ReadFile(filepath.Join(real, rel))
		if err != nil {
			t.Fatal(err)
		}
		dst := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// system 迁移目录（菜单版本号扫描用）
	if err := os.MkdirAll(filepath.Join(root, "modules/system/migrations"), 0o755); err != nil {
		t.Fatal(err)
	}
	// 现 system 最大版本文件（逼 nextMigrationVersion 走对分支）
	if err := os.WriteFile(filepath.Join(root, "modules/system/migrations/20261016000002_x.up.sql"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func tempOpts(t *testing.T) Options {
	t.Helper()
	root := t.TempDir()
	copyRealTouchpoints(t, root)
	o := Options{
		Root: root, Name: "demo", Entities: []string{"product", "category"},
		Now:         time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC),
		VerifyBuild: false, // 临时骨架不是完整仓库，跳过 go build
	}
	if err := o.Validate(); err != nil {
		t.Fatal(err)
	}
	return o
}

func TestPlanDryRun(t *testing.T) {
	o := tempOpts(t)
	cs, err := Plan(o)
	if err != nil {
		t.Fatalf("Plan() = %v", err)
	}
	// 3 patch
	if len(cs.Patches) != 3 {
		t.Fatalf("patches = %d, want 3", len(cs.Patches))
	}
	paths := map[string]bool{}
	for _, c := range cs.Creates {
		paths[c.Path] = true
	}
	for _, want := range []string{
		"modules/demo/module.go",
		"modules/demo/migration_source.go",
		"modules/demo/configs/config.yaml",
		"modules/demo/migrations/20261019000001_products.up.sql",
		"modules/demo/migrations/20261019000002_categories.down.sql",
	} {
		if !paths[want] {
			t.Errorf("creates missing %s", want)
		}
	}
	// 菜单种子在 system
	menuOK := false
	for _, c := range cs.Creates {
		if strings.HasPrefix(c.Path, "modules/system/migrations/") && strings.HasSuffix(c.Path, "_menus.up.sql") {
			menuOK = true
		}
	}
	if !menuOK {
		t.Error("missing system menu seed up.sql")
	}
	// Dry-run 不落盘：demo 目录不应出现
	if _, err := os.Stat(filepath.Join(o.Root, "modules/demo")); !os.IsNotExist(err) {
		t.Error("dry-run must not write files")
	}
	// patch After 与 Before 必须不同
	for _, p := range cs.Patches {
		if string(p.Before) == string(p.After) {
			t.Errorf("patch %s Before == After", p.Path)
		}
	}
}

func TestApplyAndUndo(t *testing.T) {
	o := tempOpts(t)
	cs, err := Plan(o)
	if err != nil {
		t.Fatal(err)
	}
	undo, err := Apply(cs)
	if err != nil {
		t.Fatalf("Apply() = %v", err)
	}
	// 落盘生效
	if _, err := os.Stat(filepath.Join(o.Root, "modules/demo/module.go")); err != nil {
		t.Fatal("module.go not written")
	}
	mainAfter, _ := os.ReadFile(filepath.Join(o.Root, "alexgo-server/cmd/main.go"))
	if !strings.Contains(string(mainAfter), "demo.FxModule,") {
		t.Error("main.go patch not applied")
	}
	// undo 恢复
	undo()
	if _, err := os.Stat(filepath.Join(o.Root, "modules/demo")); !os.IsNotExist(err) {
		t.Error("undo should remove created module dir")
	}
	mainRestored, _ := os.ReadFile(filepath.Join(o.Root, "alexgo-server/cmd/main.go"))
	if strings.Contains(string(mainRestored), "demo.FxModule") {
		t.Error("undo should restore main.go")
	}
}

func TestApply_RollbackOnFailure(t *testing.T) {
	o := tempOpts(t)
	cs, err := Plan(o)
	if err != nil {
		t.Fatal(err)
	}
	// 注入失败：在 modules/demo/controller 放一个文件占住目录名，
	// 使 controller/admin/*.go 的 MkdirAll 报 ENOTDIR。
	// 该 create 位于 shell 批（module.go 等 6 文件）与 apps 批（14 文件）之后——
	// 失败时已写盘数十个文件，回滚必须全部清掉（非真空断言）。
	if err := os.MkdirAll(filepath.Join(o.Root, "modules/demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(o.Root, "modules/demo/controller"), []byte("block"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(cs); err == nil {
		t.Fatal("Apply should fail")
	}
	// 已写的 create 被回滚（shell 批的 module.go 必然已写过才轮到 controller）
	if _, err := os.Stat(filepath.Join(o.Root, "modules/demo/module.go")); !os.IsNotExist(err) {
		t.Error("rollback should delete earlier creates")
	}
	if _, err := os.Stat(filepath.Join(o.Root, "apps/web-antd/src/api/demo/product/types.ts")); !os.IsNotExist(err) {
		t.Error("rollback should delete apps creates")
	}
	// 空目录也被清掉（pruneEmptyDirs），modules/demo 整树不留残渣
	if _, err := os.Stat(filepath.Join(o.Root, "modules/demo")); !os.IsNotExist(err) {
		t.Error("rollback should prune empty dirs")
	}
	// patch 未被触碰（Apply 里 patch 在 create 之后）
	mainNow, _ := os.ReadFile(filepath.Join(o.Root, "alexgo-server/cmd/main.go"))
	if strings.Contains(string(mainNow), "demo.FxModule") {
		t.Error("main.go must be untouched when create fails first")
	}
}

func TestRun_SkipFrontend(t *testing.T) {
	o := tempOpts(t)
	o.SkipFrontend = true
	cs, err := Plan(o)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cs.Creates {
		if strings.HasPrefix(c.Path, "apps/") {
			t.Errorf("SkipFrontend should not create %s", c.Path)
		}
		if strings.Contains(c.Path, "_menus") {
			t.Errorf("SkipFrontend should skip menu seed, got %s", c.Path)
		}
	}
}
