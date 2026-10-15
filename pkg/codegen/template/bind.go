package template

import (
	"strings"

	"alexGo-cloud/pkg/codegen/model"
	"alexGo-cloud/pkg/codegen/naming"
)

type Options struct {
	UnitTestEnable bool
}

// Bind 是模板渲染的绑定数据（模板唯一可见的世界）。
type Bind struct {
	Module    string
	Entity    string
	Snake     string
	Plural    string
	Business  string
	TableName string
	HasTenant bool
	Imports   struct{ Time, JSON bool }
	Columns   []BindColumn
}

type BindColumn struct {
	Name          string
	Comment       string
	GoName        string
	GoType        string
	JSONName      string
	GormTag       string
	HTMLType      string
	QueryOp       string
	IsPK          bool
	Nullable      bool
	ListEnable    bool
	FormEnable    bool
	QueryEnable   bool
	ListRequired  bool
	FormRequired  bool
	TSType        string
	VbenComponent string
}

func NewBind(t *model.Table) Bind {
	b := Bind{
		Module:    t.Module,
		Entity:    t.ClassName,
		Snake:     naming.Snake(t.ClassName),
		Plural:    naming.Plural(t.ClassName),
		Business:  t.BusinessName,
		TableName: t.TableName,
		HasTenant: t.HasTenant,
	}
	for _, c := range t.Columns {
		if strings.Contains(c.GoType, "time.Time") {
			b.Imports.Time = true
		}
		if strings.Contains(c.GoType, "json.RawMessage") {
			b.Imports.JSON = true
		}
		b.Columns = append(b.Columns, BindColumn{
			Name: c.Name, Comment: c.Comment,
			GoName: c.GoName, GoType: c.GoType, JSONName: c.JSONName,
			GormTag: c.GormTag, HTMLType: string(c.HTMLType), QueryOp: c.QueryOp,
			IsPK: c.IsPK, Nullable: c.Nullable,
			ListEnable: c.ListEnable, FormEnable: c.FormEnable, QueryEnable: c.QueryEnable,
			ListRequired: c.ListRequired, FormRequired: c.FormRequired,
			TSType:        tsType(c.GoType),
			VbenComponent: vbenComponent(string(c.HTMLType), c.GoType),
		})
	}
	return b
}

// tsType Go 类型 → TS 类型（front 模板专用；先剥指针再匹配）。
func tsType(goType string) string {
	gt := strings.TrimPrefix(goType, "*")
	switch {
	case gt == "time.Time":
		return "string"
	case gt == "json.RawMessage":
		return "any"
	case gt == "bool":
		return "boolean"
	case strings.HasPrefix(gt, "string"):
		return "string"
	case strings.Contains(gt, "int"):
		return "number"
	case strings.Contains(gt, "float"):
		return "number"
	default:
		return "any"
	}
}

// vbenComponent HTMLType → 实际 vben 表单组件：
// decimal 的 GoType 是 string，InputNumber 会送 number 触发 ShouldBindJSON 拒收 → 降为 Input。
func vbenComponent(htmlType, goType string) string {
	if htmlType == "" {
		return "Input"
	}
	if htmlType == "InputNumber" && strings.Contains(goType, "string") {
		return "Input"
	}
	return htmlType
}
