// pkg/codegen/model/model_test.go
package model_test

import (
	"errors"
	"testing"

	"alexGo-cloud/pkg/codegen/model"
)

func validTable() *model.Table {
	return &model.Table{
		TableName:    "order_items",
		Module:       "order",
		BusinessName: "order_item",
		ClassName:    "OrderItem",
		TemplateType: model.TemplateTypeSingle,
		FrontType:    model.FrontTypeVben5Antd,
		Columns: []model.Column{
			{Name: "id", GoName: "ID", GoType: "uint64"},
		},
	}
}

func TestTableValidate_OK(t *testing.T) {
	if err := validTable().Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestTableValidate_NoColumns(t *testing.T) {
	tbl := validTable()
	tbl.Columns = nil
	if err := tbl.Validate(); !errors.Is(err, model.ErrColumnInvalid) {
		t.Fatalf("Validate() = %v, want ErrColumnInvalid", err)
	}
}

func TestTableValidate_DuplicateGoName(t *testing.T) {
	tbl := validTable()
	tbl.Columns = append(tbl.Columns, model.Column{Name: "name", GoName: "ID", GoType: "string"})
	if err := tbl.Validate(); !errors.Is(err, model.ErrColumnInvalid) {
		t.Fatalf("Validate() = %v, want ErrColumnInvalid", err)
	}
}

func TestTableValidate_MissingFields(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*model.Table)
	}{
		{"empty table name", func(t *model.Table) { t.TableName = "" }},
		{"empty module", func(t *model.Table) { t.Module = "" }},
		{"empty class name", func(t *model.Table) { t.ClassName = "" }},
		{"bad template type", func(t *model.Table) { t.TemplateType = 99 }},
		{"bad front type", func(t *model.Table) { t.FrontType = 0 }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tbl := validTable()
			c.mutate(tbl)
			if err := tbl.Validate(); err == nil {
				t.Fatal("Validate() = nil, want error")
			}
		})
	}
}

func TestEnums_Valid(t *testing.T) {
	for _, tt := range []model.TemplateType{1, 2, 3} {
		if !tt.Valid() {
			t.Errorf("TemplateType(%d).Valid() = false", tt)
		}
	}
	if model.TemplateType(0).Valid() || model.TemplateType(4).Valid() {
		t.Error("TemplateType out of range should be invalid")
	}
	if !model.FrontTypeVben5Antd.Valid() || model.FrontType(9).Valid() {
		t.Error("FrontType validation wrong")
	}
}
