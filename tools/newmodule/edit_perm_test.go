package newmodule

import (
	"bytes"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
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
	// 字节级：输出 = 原文件恰好插入 2 个条目行
	//（本用例新 key 不宽于所在对齐组既有最大宽，差异应为空集：连冒号空格都一字不动）
	beforeLines := strings.Split(string(src), "\n")
	afterLines := strings.Split(got, "\n")
	if len(afterLines) != len(beforeLines)+2 {
		t.Errorf("行数应为原行数+2: got %d want %d", len(afterLines), len(beforeLines)+2)
	}
	assertDiffOnlyColonSpacing(t, src, out, []string{`"demo:product":`, `"demo:category":`})
	// 窄 key 不触发任何重对齐：删掉插入行后与原字节全等
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

// TestEditPermissionRoutes_WideKey 宽 key（product:category → key+冒号 19 列，与
// scorecard 组既有最大宽持平；T13 注释块已把 map 切成宽 16 / 宽 19 两组对齐）：
// 产物必须与 gofmt 的分组对齐输出逐字节一致——手工按单组目标宽重排会把窄组行
// 一并改宽，gofmt -l 标红。
// 字节级：输出相对原文的差异仅「条目行冒号后空格」+「2 条新增行」，无其它 diff。
func TestEditPermissionRoutes_WideKey(t *testing.T) {
	root := repoRoot(t)
	src, err := os.ReadFile(filepath.Join(root, "modules/system/service/permission_routes.go"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := editPermissionRoutes(src, "product", []string{"product", "category"})
	if err != nil {
		t.Fatalf("editPermissionRoutes() = %v", err)
	}
	got := string(out)

	// 新条目（宽 key）
	for _, want := range []string{
		`"product:category":`,
		`"product:product":`,
		`"/api/admin/product/categories"`,
		`"/api/admin/product/products/*"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("缺少 %q", want)
		}
	}
	// gofmt 干净：产物必须与 gofmt 输出逐字节一致（gofmt -l 为空）
	fmtSrc, err := format.Source(out)
	if err != nil {
		t.Fatalf("gofmt 失败: %v", err)
	}
	if !bytes.Equal(fmtSrc, out) {
		t.Error("产物不是 gofmt 干净的（gofmt -l 会标红，发生了整块重对齐）")
	}
	// 字节级：差异仅「条目行冒号后空格」+「2 条新增行」
	assertDiffOnlyColonSpacing(t, src, out, []string{`"product:product":`, `"product:category":`})
	// 可解析 + 幂等拒绝
	if _, err := parseGo("permission_routes.go", out); err != nil {
		t.Fatalf("输出不可解析: %v", err)
	}
	if _, err := editPermissionRoutes(out, "product", []string{"category"}); err == nil {
		t.Error("重复 key 应报错")
	}
}

// assertDiffOnlyColonSpacing 断言 out 相对 src 的差异仅为：
//  1. newLines 指定的整行新增；
//  2. 既有「条目行」冒号后的空格数（纯空白重对齐）。
//
// 其余任何一行（含注释、map 外代码、值文本）都必须逐字节相等。
func assertDiffOnlyColonSpacing(t *testing.T, src, out []byte, newLines []string) {
	t.Helper()
	entryRe := regexp.MustCompile(`^(\s*)("[^"]*"):( *)`)
	srcLines := strings.Split(string(src), "\n")
	outLines := strings.Split(string(out), "\n")
	kept := make([]string, 0, len(outLines))
	for _, line := range outLines {
		isNew := false
		for _, nl := range newLines {
			if strings.Contains(line, nl) {
				isNew = true
				break
			}
		}
		if isNew {
			continue
		}
		kept = append(kept, line)
	}
	if len(kept) != len(srcLines) {
		t.Errorf("去掉新增行后行数 = %d, 应等于原行数 %d", len(kept), len(srcLines))
		return
	}
	for i := range srcLines {
		if srcLines[i] == kept[i] {
			continue
		}
		m1, m2 := entryRe.FindStringSubmatch(srcLines[i]), entryRe.FindStringSubmatch(kept[i])
		if m1 == nil || m2 == nil || m1[2] != m2[2] {
			t.Errorf("第 %d 行出现条目行之外的 diff:\n  -%q\n  +%q", i+1, srcLines[i], kept[i])
			continue
		}
		// 归一化：抹掉冒号后空格后必须完全一致（值文本/其它字符一字未动）
		if entryRe.ReplaceAllString(srcLines[i], "$1$2:") != entryRe.ReplaceAllString(kept[i], "$1$2:") {
			t.Errorf("第 %d 行除冒号后空格外还有 diff:\n  -%q\n  +%q", i+1, srcLines[i], kept[i])
		}
	}
}

func TestEditPermissionRoutes_AnchorMissing(t *testing.T) {
	if _, err := editPermissionRoutes([]byte("package service\n"), "demo", []string{"x"}); err == nil {
		t.Error("锚点缺失应报错")
	}
}
