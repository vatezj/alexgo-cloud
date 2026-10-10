package template_test

import (
	"testing"

	"alexGo-cloud/pkg/codegen/model"
	"alexGo-cloud/pkg/codegen/template"
)

func fixtureTable() *model.Table {
	return &model.Table{
		TableName:    "order_items",
		Module:       "order",
		BusinessName: "order_item",
		ClassName:    "OrderItem",
		TemplateType: model.TemplateTypeSingle,
		FrontType:    model.FrontTypeVben5Antd,
		HasTenant:    true,
		Columns: []model.Column{
			{Name: "id", GoName: "ID", GoType: "uint64", JSONName: "id", GormTag: "column:id;primaryKey;autoIncrement", SortOrder: 0},
			{Name: "created_at", GoName: "CreatedAt", GoType: "time.Time", JSONName: "created_at", SortOrder: 1},
			{Name: "payload", GoName: "Payload", GoType: "json.RawMessage", JSONName: "payload", SortOrder: 2},
		},
	}
}

func TestNewBind(t *testing.T) {
	b := template.NewBind(fixtureTable())
	if b.Entity != "OrderItem" || b.Snake != "order_item" || b.Plural != "order_items" {
		t.Errorf("bind naming = %q/%q/%q", b.Entity, b.Snake, b.Plural)
	}
	if b.Module != "order" || b.TableName != "order_items" || !b.HasTenant {
		t.Errorf("bind = %+v", b)
	}
	if !b.Imports.Time || !b.Imports.JSON {
		t.Errorf("Imports = %+v, want Time+JSON true", b.Imports)
	}
	if len(b.Columns) != 3 || b.Columns[0].GoName != "ID" {
		t.Errorf("Columns = %+v", b.Columns)
	}
}

func TestNewBind_NoTimeNoJSON(t *testing.T) {
	tbl := fixtureTable()
	tbl.Columns = tbl.Columns[:1] // 仅 uint64
	b := template.NewBind(tbl)
	if b.Imports.Time || b.Imports.JSON {
		t.Errorf("Imports = %+v, want both false", b.Imports)
	}
}
