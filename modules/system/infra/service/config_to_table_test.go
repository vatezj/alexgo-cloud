package service

import (
	"errors"
	"strings"
	"testing"

	"alexGo-cloud/modules/system/infra/model"
	cgmodel "alexGo-cloud/pkg/codegen/model"
)

// 映射器核心：GoName/GormTag 配置表不存须重算，JSONName/开关取存值（Review Focus #1）。
func TestTableFromConfig_RecomputesAndKeepsConfig(t *testing.T) {
	tbl := &model.CodegenTable{
		Name: "order_items", TableComment: "订单明细",
		Module: "order", BusinessName: "order_item", ClassName: "OrderItem",
		TemplateType: 1, FrontType: 1, Remark: "r", ParentTableID: 0,
	}
	cols := []*model.CodegenColumn{
		{Name: "id", Type: "bigint unsigned", GoType: "uint64", JSONName: "id",
			IsPK: true, AutoIncrement: true, QueryOperation: "eq", SortOrder: 0},
		{Name: "tenant_id", Type: "bigint unsigned", GoType: "uint64",
			JSONName: "-", SortOrder: 1},
		{Name: "name", Type: "varchar(64)", Comment: "名称", GoType: "string",
			JSONName: "name", HTMLType: "Input", ListEnable: true, FormEnable: true,
			QueryEnable: true, QueryOperation: "like", DictType: "status", SortOrder: 2},
	}

	got, err := TableFromConfig(tbl, cols)
	if err != nil {
		t.Fatalf("TableFromConfig: %v", err)
	}
	if got.TableName != "order_items" || got.Module != "order" ||
		got.ClassName != "OrderItem" || got.BusinessName != "order_item" ||
		got.TableComment != "订单明细" || got.Remark != "r" {
		t.Errorf("table fields: %+v", got)
	}
	if got.TemplateType != cgmodel.TemplateTypeSingle || got.FrontType != cgmodel.FrontTypeVben5Antd {
		t.Errorf("enum cast: template=%d front=%d", got.TemplateType, got.FrontType)
	}
	if !got.HasTenant {
		t.Error("含 tenant_id → HasTenant 应为 true")
	}
	if len(got.Columns) != 3 {
		t.Fatalf("columns=%d, want 3", len(got.Columns))
	}
	// SortOrder 决定输出顺序
	if got.Columns[0].Name != "id" || got.Columns[1].Name != "tenant_id" || got.Columns[2].Name != "name" {
		t.Errorf("order: %s,%s,%s",
			got.Columns[0].Name, got.Columns[1].Name, got.Columns[2].Name)
	}
	idCol := got.Columns[0]
	if idCol.GoName != "ID" {
		t.Errorf("GoName(id)=%q, want ID（naming.ToGoName 重算）", idCol.GoName)
	}
	if !strings.Contains(idCol.GormTag, "column:id") ||
		!strings.Contains(idCol.GormTag, "primaryKey") ||
		!strings.Contains(idCol.GormTag, "autoIncrement") {
		t.Errorf("id GormTag=%q", idCol.GormTag)
	}
	nameCol := got.Columns[2]
	if nameCol.GoName != "Name" || nameCol.GormTag != "column:name" {
		t.Errorf("name: GoName=%q GormTag=%q", nameCol.GoName, nameCol.GormTag)
	}
	if !nameCol.ListEnable || !nameCol.FormEnable || !nameCol.QueryEnable ||
		nameCol.QueryOp != "like" || nameCol.DictType != "status" || nameCol.HTMLType != cgmodel.HTMLInput {
		t.Errorf("config flags lost: %+v", nameCol)
	}
	if got.Columns[1].JSONName != "-" {
		t.Errorf("tenant_id JSONName=%q, want -", got.Columns[1].JSONName)
	}
}

// deprecated 列剔除；全废弃 → 列空 → ErrColumnInvalid（Review Focus #2）。
func TestTableFromConfig_SkipsDeprecated(t *testing.T) {
	tbl := &model.CodegenTable{
		Name: "t", Module: "order", BusinessName: "t", ClassName: "T",
		TemplateType: 1, FrontType: 1,
	}
	col := func(name string, dep bool, order int) *model.CodegenColumn {
		return &model.CodegenColumn{
			Name: name, GoType: "string", JSONName: name,
			HTMLType: "Input", SortOrder: order, Deprecated: dep,
		}
	}

	got, err := TableFromConfig(tbl, []*model.CodegenColumn{
		col("id", false, 0), col("gone", true, 1), col("name", false, 2),
	})
	if err != nil {
		t.Fatalf("TableFromConfig: %v", err)
	}
	if len(got.Columns) != 2 || got.Columns[0].Name != "id" || got.Columns[1].Name != "name" {
		t.Fatalf("columns=%+v, want [id name]", got.Columns)
	}
	for _, c := range got.Columns {
		if c.Deprecated {
			t.Errorf("输出列不应带 Deprecated: %+v", c)
		}
	}

	if _, err := TableFromConfig(tbl, []*model.CodegenColumn{col("gone", true, 0)}); // 全废弃
	!errors.Is(err, cgmodel.ErrColumnInvalid) {
		t.Errorf("全废弃 → ErrColumnInvalid, got %v", err)
	}
}

func TestTableFromConfig_RejectsEmpty(t *testing.T) {
	if _, err := TableFromConfig(nil, nil); err == nil {
		t.Error("nil table 应报错")
	}
	tbl := &model.CodegenTable{
		Name: "t", Module: "order", ClassName: "T", TemplateType: 1, FrontType: 1,
	}
	if _, err := TableFromConfig(tbl, nil); !errors.Is(err, cgmodel.ErrColumnInvalid) {
		t.Errorf("无列 → ErrColumnInvalid, got %v", err)
	}
}
