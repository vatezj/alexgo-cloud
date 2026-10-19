package newmodule

import (
	"strings"
	"testing"
	"time"
)

func TestExampleTableMeta(t *testing.T) {
	m := exampleTableMeta("demo", "category") // 不规则复数
	if m.Name != "categories" {
		t.Errorf("table name = %q, want categories", m.Name)
	}
	if len(m.Columns) != 6 {
		t.Fatalf("columns = %d, want 6", len(m.Columns))
	}
	if m.Columns[0].Name != "id" || m.Columns[0].Key != "PRI" {
		t.Errorf("first column should be id PRI, got %+v", m.Columns[0])
	}
	if m.Columns[1].Name != "tenant_id" {
		t.Errorf("second column should be tenant_id")
	}
}

func TestEntityFiles(t *testing.T) {
	// Name 固定 demo（不与未来真实模块撞车），断言路径也按 demo 钉死
	o := Options{
		Root:     repoRoot(t),
		Name:     "demo",
		Entities: []string{"product", "category"},
		Now:      time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC),
	}
	if err := o.Validate(); err != nil {
		t.Fatal(err)
	}
	files, err := entityFiles(o)
	if err != nil {
		t.Fatalf("entityFiles() = %v", err)
	}
	// 每实体 14 个文件（7 server + 7 front），2 实体 = 28。
	if len(files) != 28 {
		t.Errorf("files = %d, want 28", len(files))
	}
	byPath := map[string]bool{}
	for _, f := range files {
		byPath[f.Path] = true
		if strings.Contains(f.Path, "..") {
			t.Errorf("bad path %q", f.Path)
		}
	}
	for _, want := range []string{
		"modules/demo/model/category.go",
		"modules/demo/repository/category.go",
		"modules/demo/service/category.go",
		"modules/demo/service/category_test.go",
		"modules/demo/controller/admin/category.go",
		"modules/demo/controller/app/category.go",
		"modules/demo/module_register_category.go",
		"apps/web-antd/src/api/demo/category/types.ts",
		"apps/web-antd/src/views/demo/category/list.vue",
	} {
		if !byPath[want] {
			t.Errorf("missing %s", want)
		}
	}
}
