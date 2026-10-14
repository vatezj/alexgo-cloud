package template

import (
	"fmt"
	"maps"
	"sort"
	"strings"

	"alexGo-cloud/pkg/codegen/model"
)

// serverSingleTemplates：单表后端模板族。key=embed 相对路径（templates/ 之后），value=输出路径模式。
var serverSingleTemplates = map[string]string{
	"server/go/single/model.go.tmpl":            "modules/{module}/model/{snake}.go",
	"server/go/single/repository.go.tmpl":       "modules/{module}/repository/{snake}.go",
	"server/go/single/service.go.tmpl":          "modules/{module}/service/{snake}.go",
	"server/go/single/service_test.go.tmpl":     "modules/{module}/service/{snake}_test.go",
	"server/go/single/controller_admin.go.tmpl": "modules/{module}/controller/admin/{snake}.go",
	"server/go/single/controller_app.go.tmpl":   "modules/{module}/controller/app/{snake}.go",
	"server/go/single/module_register.go.tmpl":  "modules/{module}/module_register_{snake}.go",
}

// ServerSingleTemplateNames 返回排序后的模板名列表（确定性）。
func ServerSingleTemplateNames() []string {
	names := make([]string, 0, len(serverSingleTemplates))
	for n := range serverSingleTemplates {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func ServerSingleTemplatePattern(name string) string {
	p, ok := serverSingleTemplates[name]
	if !ok {
		return ""
	}
	return p
}

// frontSingleTemplates：vben5_antd 单表前端模板族（spec §3）。
// 按任务渐进登记：只列已存在的模板文件（ParseFS 缺文件即整族报错）。
var frontSingleTemplates = map[string]string{
	"front/vben5_antd/single/types.ts.tmpl":    "apps/web-antd/src/api/{module}/{business}/types.ts",
	"front/vben5_antd/single/api.ts.tmpl":      "apps/web-antd/src/api/{module}/{business}/api.ts",
	"front/vben5_antd/single/index.ts.tmpl":    "apps/web-antd/src/api/{module}/{business}/index.ts",
	"front/vben5_antd/single/list.vue.tmpl":    "apps/web-antd/src/views/{module}/{business}/list.vue",
	"front/vben5_antd/single/form.vue.tmpl":    "apps/web-antd/src/views/{module}/{business}/form.vue",
	"front/vben5_antd/single/drawer.vue.tmpl":  "apps/web-antd/src/views/{module}/{business}/drawer.vue",
	"front/vben5_antd/single/api.test.ts.tmpl": "apps/web-antd/src/api/{module}/{business}/__tests__/{business}.test.ts",
}

// FrontSingleTemplateNames 返回排序后的前端模板名列表（确定性）。
func FrontSingleTemplateNames() []string {
	names := make([]string, 0, len(frontSingleTemplates))
	for n := range frontSingleTemplates {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func FrontSingleTemplatePattern(name string) string {
	return frontSingleTemplates[name]
}

// singleTemplates 合并 server+front 两族（base 名唯一性由 TestFrontSinglePaths 钉住）。
func singleTemplates() map[string]string {
	m := make(map[string]string, len(serverSingleTemplates)+len(frontSingleTemplates))
	maps.Copy(m, serverSingleTemplates)
	maps.Copy(m, frontSingleTemplates)
	return m
}

func isTestTemplate(name string) bool {
	return strings.HasSuffix(name, "_test.go.tmpl") || strings.HasSuffix(name, ".test.ts.tmpl")
}

// RenderPath 将模式中的占位符替换为绑定值。
func RenderPath(pattern string, b Bind) string {
	return strings.NewReplacer(
		"{module}", b.Module,
		"{snake}", b.Snake,
		"{entity}", b.Entity,
		"{business}", b.Business,
	).Replace(pattern)
}

// templateMapFor 按模板类型选择模板族；M1 仅 single。
func templateMapFor(t *model.Table) (map[string]string, error) {
	switch t.TemplateType {
	case model.TemplateTypeSingle:
		return singleTemplates(), nil
	default:
		return nil, fmt.Errorf("%w: template type %d (tree/main_sub 在 M5/M6 交付)", model.ErrTemplateMissing, t.TemplateType)
	}
}
