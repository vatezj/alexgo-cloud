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
