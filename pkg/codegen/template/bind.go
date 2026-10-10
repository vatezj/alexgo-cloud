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
	Name         string
	Comment      string
	GoName       string
	GoType       string
	JSONName     string
	GormTag      string
	HTMLType     string
	QueryOp      string
	IsPK         bool
	Nullable     bool
	ListEnable   bool
	FormEnable   bool
	QueryEnable  bool
	ListRequired bool
	FormRequired bool
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
		})
	}
	return b
}
