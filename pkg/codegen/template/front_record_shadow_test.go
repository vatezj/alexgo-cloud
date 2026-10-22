package template_test

import (
	"strings"
	"testing"

	"alexGo-cloud/pkg/codegen/builder"
	"alexGo-cloud/pkg/codegen/metadata"
	"alexGo-cloud/pkg/codegen/template"
)

// 票8：实体名 Record（表 records）会遮蔽 TS 内建 Record<>——同时 import 实体类型
// 又使用内建 Record<> 的产物（api.ts / list.vue）编译报错。两文件不得再含 Record<。
func TestFrontRecordEntityNoBuiltinShadow(t *testing.T) {
	meta := &metadata.TableMeta{
		Schema: "alexgo", Name: "records", Comment: "demo",
		Columns: []metadata.ColumnMeta{
			{Name: "id", DataType: "bigint", ColumnType: "bigint unsigned", Key: "PRI", Extra: "auto_increment", Ordinal: 1},
			{Name: "name", DataType: "varchar", ColumnType: "varchar(64)", Comment: "名称", Ordinal: 2},
			{Name: "created_at", DataType: "datetime", ColumnType: "datetime", Comment: "创建时间", Ordinal: 3},
		},
	}
	tbl, err := builder.Build(meta, builder.Options{Module: "scorecard"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if tbl.ClassName != "Record" {
		t.Fatalf("fixture 应产出实体名 Record, got %q", tbl.ClassName)
	}
	files, err := template.Render(tbl, template.Options{UnitTestEnable: true})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	checked := 0
	for _, f := range files {
		if strings.HasSuffix(f.Path, "api.ts") || strings.HasSuffix(f.Path, "list.vue") {
			checked++
			if strings.Contains(string(f.Content), "Record<") {
				t.Errorf("%s 含内建 Record<>：实体名 Record 遮蔽类型别名，改用索引签名", f.Path)
			}
		}
	}
	if checked < 2 {
		t.Fatalf("应渲染出 api.ts 与 list.vue, checked=%d", checked)
	}
}
