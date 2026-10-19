package newmodule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditPermissionRoutes(t *testing.T) {
	root := repoRoot(t)
	src, err := os.ReadFile(filepath.Join(root, "modules/system/service/permission_routes.go"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := editPermissionRoutes(src, "demo", []string{"product", "category"})
	if err != nil {
		t.Fatalf("editPermissionRoutes() = %v", err)
	}
	got := string(out)
	// 新条目（含 /* 变体）
	for _, want := range []string{
		`"demo:product":`,
		`"/api/admin/demo/products"`,
		`"/api/admin/demo/products/*"`,
		`"demo:category":`,
		`"/api/admin/demo/categories"`,
		`"/api/admin/demo/categories/*"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q", want)
		}
	}
	// 插入位置：demo:* 在 infra:* 之前（d < i，落在既有首条之前）
	if strings.Index(got, `"demo:product"`) > strings.Index(got, `"infra:codegen"`) {
		t.Error("demo entries should be inserted before infra")
	}
	// 既有条目相对顺序保持不变（文件本就非全序：system:user 在 system:role 之前——
	// 编辑器只做位置插入，绝不整体重排，否则 diff 爆炸）
	if strings.Index(got, `"system:user"`) > strings.Index(got, `"system:role"`) {
		t.Error("existing key order must be preserved")
	}
	if strings.Index(got, `"oauth2:client"`) > strings.Index(got, `"system:user"`) {
		t.Error("existing key order must be preserved")
	}
	// 既有内容不丢
	for _, keep := range []string{
		`"order:order":`,
		"// permissionRoutes：permission 前缀 → 后端路由模板（keyMatch2）",
		"// 每条“集合路由”额外带一条 `/*` 变体",
	} {
		if !strings.Contains(got, keep) {
			t.Errorf("lost %q", keep)
		}
	}
	// 可解析
	if _, err := parseGo("permission_routes.go", out); err != nil {
		t.Fatalf("output not parseable: %v", err)
	}
	// 幂等拒绝
	if _, err := editPermissionRoutes(out, "demo", []string{"product"}); err == nil {
		t.Error("duplicate key should fail")
	}
}

func TestEditPermissionRoutes_AnchorMissing(t *testing.T) {
	if _, err := editPermissionRoutes([]byte("package service\n"), "demo", []string{"x"}); err == nil {
		t.Error("want error")
	}
}
