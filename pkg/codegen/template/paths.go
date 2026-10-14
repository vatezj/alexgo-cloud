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

// serverTreeTemplates：树表后端 overlay（spec §10.3）。value=输出模式与 single 完全相同——
// 按输出模式**覆盖**（非追加），族产物恒 14。
var serverTreeTemplates = map[string]string{
	"server/go/tree/repository.go.tmpl":   "modules/{module}/repository/{snake}.go",
	"server/go/tree/service.go.tmpl":      "modules/{module}/service/{snake}.go",
	"server/go/tree/service_test.go.tmpl": "modules/{module}/service/{snake}_test.go",
}

// ServerTreeTemplateNames 返回排序后的树表后端模板名列表（确定性）。
func ServerTreeTemplateNames() []string {
	names := make([]string, 0, len(serverTreeTemplates))
	for n := range serverTreeTemplates {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func ServerTreeTemplatePattern(name string) string {
	p, ok := serverTreeTemplates[name]
	if !ok {
		return ""
	}
	return p
}

// frontTreeTemplates：树表前端 overlay（spec §10.3）。
// 按任务渐进登记（与 single 同规则）：本任务先空占位，M5 T3 补 list.vue/form.vue——
// ParseFS 缺文件即整族报错，登记与文件必须同笔落地。
var frontTreeTemplates = map[string]string{}

// FrontTreeTemplateNames 返回排序后的树表前端模板名列表（确定性）。
func FrontTreeTemplateNames() []string {
	names := make([]string, 0, len(frontTreeTemplates))
	for n := range frontTreeTemplates {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func FrontTreeTemplatePattern(name string) string {
	return frontTreeTemplates[name]
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

// treeTemplates = single 全集，其中输出模式被 tree overlay 命中的条目被替换
// （spec §10.3 修订 §3「渲染该目录全部模板」：叠加而非整目录复制）。
// 同模式双条目会让 CheckCollide 报路径冲突，所以先删后加。
func treeTemplates() map[string]string {
	overlay := make(map[string]string, len(serverTreeTemplates)+len(frontTreeTemplates))
	maps.Copy(overlay, serverTreeTemplates)
	maps.Copy(overlay, frontTreeTemplates)
	// 按**输出模式**建集合：overlay 的 key 是模板名（server/go/tree/...），
	// 要删的 single 条目按 pattern 匹配，不能拿 pattern 当 overlay 的 key 查。
	replaced := make(map[string]bool, len(overlay))
	for _, pattern := range overlay {
		replaced[pattern] = true
	}
	m := make(map[string]string, len(singleTemplates()))
	for name, pattern := range singleTemplates() {
		if !replaced[pattern] {
			m[name] = pattern
		}
	}
	maps.Copy(m, overlay)
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

// templateMapFor 按模板类型选择模板族：single/tree（M5），main_sub 在 M6。
func templateMapFor(t *model.Table) (map[string]string, error) {
	switch t.TemplateType {
	case model.TemplateTypeSingle:
		return singleTemplates(), nil
	case model.TemplateTypeTree:
		return treeTemplates(), nil
	default:
		return nil, fmt.Errorf("%w: template type %d (main_sub 在 M6 交付)", model.ErrTemplateMissing, t.TemplateType)
	}
}
