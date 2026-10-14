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

// TestFrontSinglePaths 钉住 vben5_antd 前端模板族输出路径（随任务推进 3→7）。
func TestFrontSinglePaths(t *testing.T) {
	b := template.NewBind(fixtureTable())
	want := map[string]string{
		"types.ts.tmpl": "apps/web-antd/src/api/order/order_item/types.ts",
		"api.ts.tmpl":   "apps/web-antd/src/api/order/order_item/api.ts",
		"index.ts.tmpl": "apps/web-antd/src/api/order/order_item/index.ts",
		"list.vue.tmpl": "apps/web-antd/src/views/order/order_item/list.vue",
		"form.vue.tmpl": "apps/web-antd/src/views/order/order_item/form.vue",
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
