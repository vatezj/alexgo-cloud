package newmodule

import (
	"strings"
	"testing"
	"time"
)

func TestMenuSeedFiles(t *testing.T) {
	o := Options{Root: repoRoot(t), Name: "demo", Entities: []string{"product", "category"},
		Now: time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)}
	if err := o.Validate(); err != nil {
		t.Fatal(err)
	}
	files, err := menuSeedFiles(&o)
	if err != nil {
		t.Fatalf("menuSeedFiles() = %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("files = %d, want 2 (up+down)", len(files))
	}
	var up, down string
	for _, f := range files {
		if strings.HasSuffix(f.Path, ".up.sql") {
			up = string(f.Content)
			if !strings.HasPrefix(f.Path, "modules/system/migrations/") {
				t.Errorf("up path %q should be under system migrations", f.Path)
			}
			// 版本必须 > 现有 system 最大版本（当前 20261016...）
			ver := f.Path[len("modules/system/migrations/"):][:14]
			if ver <= "20261016000002" {
				t.Errorf("version %q not greater than existing max", ver)
			}
		}
		if strings.HasSuffix(f.Path, ".down.sql") {
			down = string(f.Content)
		}
	}
	for _, want := range []string{
		"WHERE NOT EXISTS",          // 幂等 guard
		"'/demo'",                   // dir path
		"'dir'",                     // dir type
		"'/demo/products'",          // menu path
		"'views/demo/product/list'", // component 与 codegen 视图同源
		"'/demo/categories'",
		"'views/demo/category/list'",
		"'demo:product:*'", // 权限码
		"'demo:category:*'",
		"FROM `menus` d", // parent 经父目录 JOIN
		"tenant_id`=0",   // 全局菜单
	} {
		if !strings.Contains(up, want) {
			t.Errorf("up.sql missing %q\n%s", want, up)
		}
	}
	if !strings.Contains(down, "DELETE FROM `menus`") || !strings.Contains(down, "'/demo'") {
		t.Errorf("down.sql wrong:\n%s", down)
	}
	// down 不应解绑 role_menus（本设计不绑角色；但保险起见 down 只删菜单行）
}
