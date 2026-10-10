// pkg/codegen/builder/builder_test.go
package builder_test

import (
	"errors"
	"testing"

	"alexGo-cloud/pkg/codegen/builder"
	"alexGo-cloud/pkg/codegen/metadata"
	"alexGo-cloud/pkg/codegen/model"
)

func fixtureMeta() *metadata.TableMeta {
	return &metadata.TableMeta{
		Schema:  "alexgo",
		Name:    "order_items",
		Comment: "订单项",
		Columns: []metadata.ColumnMeta{
			{Name: "id", DataType: "bigint", ColumnType: "bigint unsigned", Key: "PRI", Extra: "auto_increment", Ordinal: 1},
			{Name: "name", DataType: "varchar", ColumnType: "varchar(64)", Comment: "名称", Ordinal: 2},
			{Name: "price", DataType: "decimal", ColumnType: "decimal(10,2)", Comment: "价格", Ordinal: 3},
			{Name: "status", DataType: "tinyint", ColumnType: "tinyint(1)", Comment: "状态", Ordinal: 4},
			{Name: "remark", DataType: "varchar", ColumnType: "varchar(255)", Nullable: true, Comment: "备注", Ordinal: 5},
			{Name: "password", DataType: "varchar", ColumnType: "varchar(128)", Comment: "密码", Ordinal: 6},
			{Name: "created_at", DataType: "datetime", ColumnType: "datetime", Comment: "创建时间", Ordinal: 7},
			{Name: "tenant_id", DataType: "bigint", ColumnType: "bigint unsigned", Comment: "租户", Ordinal: 8},
		},
	}
}

func TestBuild_Defaults(t *testing.T) {
	tbl, err := builder.Build(fixtureMeta(), builder.Options{Module: "order"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if tbl.ClassName != "OrderItem" {
		t.Errorf("ClassName = %q, want OrderItem", tbl.ClassName)
	}
	if tbl.BusinessName != "order_item" {
		t.Errorf("BusinessName = %q, want order_item", tbl.BusinessName)
	}
	if tbl.TemplateType != model.TemplateTypeSingle || tbl.FrontType != model.FrontTypeVben5Antd {
		t.Errorf("defaults = %v/%v", tbl.TemplateType, tbl.FrontType)
	}
	if !tbl.HasTenant {
		t.Error("HasTenant = false, want true (tenant_id 列)")
	}
	if err := tbl.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}

	byName := map[string]model.Column{}
	for _, c := range tbl.Columns {
		byName[c.Name] = c
	}
	id := byName["id"]
	if !id.IsPK || !id.AutoIncrement || id.FormEnable || id.ListRequired {
		t.Errorf("id column = %+v", id)
	}
	if id.GoType != "uint64" {
		t.Errorf("id.GoType = %q, want uint64", id.GoType)
	}
	remark := byName["remark"]
	if remark.GoType != "string" || remark.FormRequired || remark.ListRequired {
		t.Errorf("remark = %+v", remark)
	}
	if remark.QueryOp != "like" || !remark.ListEnable || !remark.FormEnable || !remark.QueryEnable {
		t.Errorf("remark switches = %+v", remark)
	}
	created := byName["created_at"]
	if created.GoType != "time.Time" || created.QueryOp != "between" {
		t.Errorf("created_at = %+v", created)
	}
	if byName["password"].JSONName != "-" {
		t.Errorf("password JSONName = %q, want -", byName["password"].JSONName)
	}
	if byName["status"].HTMLType != model.HTMLSwitch {
		t.Errorf("status HTMLType = %s, want Switch", byName["status"].HTMLType)
	}
	if tbl.Columns[0].SortOrder != 0 || tbl.Columns[7].SortOrder != 7 {
		t.Errorf("SortOrder wrong: %d..%d", tbl.Columns[0].SortOrder, tbl.Columns[7].SortOrder)
	}
}

func TestBuild_CommentFlattened(t *testing.T) {
	meta := fixtureMeta()
	meta.Columns[1].Comment = "第一行\n第二行"
	tbl, err := builder.Build(meta, builder.Options{Module: "order"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := tbl.Columns[1].Comment; got != "第一行 第二行" {
		t.Errorf("Comment = %q, want flattened single line", got)
	}
}

func TestBuild_UnknownType(t *testing.T) {
	meta := fixtureMeta()
	meta.Columns[1].DataType = "geometry"
	meta.Columns[1].ColumnType = "geometry"
	_, err := builder.Build(meta, builder.Options{Module: "order"})
	if !errors.Is(err, model.ErrTypeMappingUnknown) {
		t.Fatalf("err = %v, want ErrTypeMappingUnknown", err)
	}
}

func TestBuild_EmptyMeta(t *testing.T) {
	if _, err := builder.Build(&metadata.TableMeta{Name: "x"}, builder.Options{Module: "order"}); !errors.Is(err, model.ErrColumnInvalid) {
		t.Fatalf("err = %v, want ErrColumnInvalid", err)
	}
	if _, err := builder.Build(nil, builder.Options{Module: "order"}); err == nil {
		t.Fatal("nil meta should error")
	}
}
