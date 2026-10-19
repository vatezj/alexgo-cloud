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
			t.Errorf("缺少 %q", want)
		}
	}
	// 插入位置：demo:* 在 infra:* 之前（d < i，落在既有首条之前）
	if strings.Index(got, `"demo:product"`) > strings.Index(got, `"infra:codegen"`) {
		t.Error("demo 条目应插在 infra 之前")
	}
	// 既有条目相对顺序保持不变（文件本就非全序：system:user 在 system:role 之前——
	// 编辑器只做位置插入，绝不整体重排，否则 diff 爆炸）
	if strings.Index(got, `"system:user"`) > strings.Index(got, `"system:role"`) {
		t.Error("既有条目顺序必须保持不变")
	}
	if strings.Index(got, `"oauth2:client"`) > strings.Index(got, `"system:user"`) {
		t.Error("既有条目顺序必须保持不变")
	}
	// 既有内容不丢
	for _, keep := range []string{
		`"order:order":`,
		"// permissionRoutes：permission 前缀 → 后端路由模板（keyMatch2）",
		"// 每条“集合路由”额外带一条 `/*` 变体",
	} {
		if !strings.Contains(got, keep) {
			t.Errorf("丢了既有内容 %q", keep)
		}
	}
	// 字节级：输出 = 原文件恰好插入 2 个条目行（删掉插入行后与原字节全等）
	beforeLines := strings.Split(string(src), "\n")
	afterLines := strings.Split(got, "\n")
	if len(afterLines) != len(beforeLines)+2 {
		t.Errorf("行数应为原行数+2: got %d want %d", len(afterLines), len(beforeLines)+2)
	}
	kept := make([]string, 0, len(afterLines))
	for _, line := range afterLines {
		if strings.Contains(line, `"demo:product":`) || strings.Contains(line, `"demo:category":`) {
			continue
		}
		kept = append(kept, line)
	}
	if strings.Join(kept, "\n") != string(src) {
		t.Error("输出不是原文件恰好插入两行（发生了重排）")
	}
	// 可解析
	if _, err := parseGo("permission_routes.go", out); err != nil {
		t.Fatalf("输出不可解析: %v", err)
	}
	// 幂等拒绝
	if _, err := editPermissionRoutes(out, "demo", []string{"product"}); err == nil {
		t.Error("重复 key 应报错")
	}
}

func TestEditPermissionRoutes_AnchorMissing(t *testing.T) {
	if _, err := editPermissionRoutes([]byte("package service\n"), "demo", []string{"x"}); err == nil {
		t.Error("锚点缺失应报错")
	}
}
