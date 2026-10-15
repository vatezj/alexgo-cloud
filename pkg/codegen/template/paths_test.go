package template_test

import (
	"path"
	"testing"

	"alexGo-cloud/pkg/codegen/template"
)

func TestRenderPath(t *testing.T) {
	b := template.NewBind(fixtureTable())
	got := template.RenderPath("modules/{module}/model/{snake}.go", b)
	if got != "modules/order/model/order_item.go" {
		t.Errorf("RenderPath = %q", got)
	}
}

// TestServerSinglePaths 通过导出的 ServerSingleTemplateNames + RenderPath
// 验证 7 个单表模板的输出路径全集（名字与路径一一对应）。
func TestServerSinglePaths(t *testing.T) {
	b := template.NewBind(fixtureTable())
	want := map[string]string{
		"model.go.tmpl":            "modules/order/model/order_item.go",
		"repository.go.tmpl":       "modules/order/repository/order_item.go",
		"service.go.tmpl":          "modules/order/service/order_item.go",
		"service_test.go.tmpl":     "modules/order/service/order_item_test.go",
		"controller_admin.go.tmpl": "modules/order/controller/admin/order_item.go",
		"controller_app.go.tmpl":   "modules/order/controller/app/order_item.go",
		"module_register.go.tmpl":  "modules/order/module_register_order_item.go",
	}
	names := template.ServerSingleTemplateNames()
	if len(names) != len(want) {
		t.Fatalf("template count = %d, want %d: %v", len(names), len(want), names)
	}
	for _, name := range names {
		pattern := template.ServerSingleTemplatePattern(name)
		got := template.RenderPath(pattern, b)
		short := path.Base(name)
		if got != want[short] {
			t.Errorf("%s path = %q, want %q", name, got, want[short])
		}
	}
}

// TestServerTreePaths 钉树表后端 overlay 3 模板（spec §10.3）：
// 与 single 同输出模式（覆盖语义，族产物恒 14），路径一一对应。
func TestServerTreePaths(t *testing.T) {
	b := template.NewBind(fixtureTable())
	want := map[string]string{
		"repository.go.tmpl":   "modules/order/repository/order_item.go",
		"service.go.tmpl":      "modules/order/service/order_item.go",
		"service_test.go.tmpl": "modules/order/service/order_item_test.go",
	}
	singleByBase := map[string]string{}
	for _, n := range template.ServerSingleTemplateNames() {
		singleByBase[path.Base(n)] = template.ServerSingleTemplatePattern(n)
	}
	names := template.ServerTreeTemplateNames()
	if len(names) != len(want) {
		t.Fatalf("template count = %d, want %d: %v", len(names), len(want), names)
	}
	for _, name := range names {
		short := path.Base(name)
		pattern := template.ServerTreeTemplatePattern(name)
		if got := template.RenderPath(pattern, b); got != want[short] {
			t.Errorf("%s path = %q, want %q", name, got, want[short])
		}
		// 覆盖语义（Review Focus ②）：模式必须与 single 完全相同，否则不是覆盖是分叉。
		if singleByBase[short] == "" || singleByBase[short] != pattern {
			t.Errorf("%s pattern = %q, want same as single %q", name, pattern, singleByBase[short])
		}
	}
}

// TestFrontTreePaths 钉树表前端 overlay 2 模板（spec §10.3/10.4）：
// 与 single 同输出模式（覆盖语义），路径一一对应。
func TestFrontTreePaths(t *testing.T) {
	b := template.NewBind(fixtureTable())
	want := map[string]string{
		"list.vue.tmpl": "apps/web-antd/src/views/order/order_item/list.vue",
		"form.vue.tmpl": "apps/web-antd/src/views/order/order_item/form.vue",
	}
	singleByBase := map[string]string{}
	for _, n := range template.FrontSingleTemplateNames() {
		singleByBase[path.Base(n)] = template.FrontSingleTemplatePattern(n)
	}
	names := template.FrontTreeTemplateNames()
	if len(names) != len(want) {
		t.Fatalf("front tree count = %d, want %d: %v", len(names), len(want), names)
	}
	for _, name := range names {
		short := path.Base(name)
		pattern := template.FrontTreeTemplatePattern(name)
		if got := template.RenderPath(pattern, b); got != want[short] {
			t.Errorf("%s path = %q, want %q", name, got, want[short])
		}
		if singleByBase[short] == "" || singleByBase[short] != pattern {
			t.Errorf("%s pattern = %q, want same as single %q", name, pattern, singleByBase[short])
		}
	}
}

// TestFrontSinglePaths 钉住 vben5_antd 前端模板族输出路径（随任务推进 3→7）。
func TestFrontSinglePaths(t *testing.T) {
	b := template.NewBind(fixtureTable())
	want := map[string]string{
		"types.ts.tmpl":    "apps/web-antd/src/api/order/order_item/types.ts",
		"api.ts.tmpl":      "apps/web-antd/src/api/order/order_item/api.ts",
		"index.ts.tmpl":    "apps/web-antd/src/api/order/order_item/index.ts",
		"list.vue.tmpl":    "apps/web-antd/src/views/order/order_item/list.vue",
		"form.vue.tmpl":    "apps/web-antd/src/views/order/order_item/form.vue",
		"drawer.vue.tmpl":  "apps/web-antd/src/views/order/order_item/drawer.vue",
		"api.test.ts.tmpl": "apps/web-antd/src/api/order/order_item/__tests__/order_item.test.ts",
	}
	names := template.FrontSingleTemplateNames()
	if len(names) != len(want) {
		t.Fatalf("front template count = %d, want %d: %v", len(names), len(want), names)
	}
	serverBase := map[string]bool{}
	for _, n := range template.ServerSingleTemplateNames() {
		serverBase[path.Base(n)] = true
	}
	for _, name := range names {
		if serverBase[path.Base(name)] {
			t.Errorf("front base name collides with server family: %s", name)
		}
		pattern := template.FrontSingleTemplatePattern(name)
		got := template.RenderPath(pattern, b)
		if short := path.Base(name); got != want[short] {
			t.Errorf("%s path = %q, want %q", name, got, want[short])
		}
	}
}
