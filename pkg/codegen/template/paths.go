package template

import (
	"fmt"
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

func isTestTemplate(name string) bool {
	return strings.HasSuffix(name, "_test.go.tmpl")
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
		return serverSingleTemplates, nil
	default:
		return nil, fmt.Errorf("%w: template type %d (tree/main_sub 在 M5/M6 交付)", model.ErrTemplateMissing, t.TemplateType)
	}
}
