package newmodule

import (
	"fmt"

	"alexGo-cloud/pkg/codegen"
	"alexGo-cloud/pkg/codegen/builder"
	"alexGo-cloud/pkg/codegen/metadata"
	"alexGo-cloud/pkg/codegen/model"
	"alexGo-cloud/pkg/codegen/naming"
)

// exampleTableMeta 示例实体表元数据：6 列固定骨架，与迁移 DDL 模板严格同构
// （id/tenant_id/name/status/created_at/updated_at）。表名 = 实体名复数化。
func exampleTableMeta(module, entity string) *metadata.TableMeta {
	_ = module // 表名只依赖实体；module 保留签名对称，便于扩展
	table := naming.Plural(entity)
	return &metadata.TableMeta{
		Name:    table,
		Comment: entity + " 示例表",
		Columns: []metadata.ColumnMeta{
			{Name: "id", DataType: "bigint", ColumnType: "bigint unsigned", Key: "PRI", Extra: "auto_increment", Ordinal: 1},
			{Name: "tenant_id", DataType: "bigint", ColumnType: "bigint unsigned", Comment: "租户ID", Ordinal: 2},
			{Name: "name", DataType: "varchar", ColumnType: "varchar(64)", Comment: "名称", Ordinal: 3},
			{Name: "status", DataType: "tinyint", ColumnType: "tinyint", Comment: "状态 1=启用 0=停用", Ordinal: 4},
			{Name: "created_at", DataType: "datetime", ColumnType: "datetime", Comment: "创建时间", Ordinal: 5},
			{Name: "updated_at", DataType: "datetime", ColumnType: "datetime", Comment: "更新时间", Ordinal: 6},
		},
	}
}

// entityFiles 经 codegen 引擎生成全部实体的后端+前端文件（含单测模板）。
func entityFiles(o Options) ([]model.GeneratedFile, error) {
	tables := make([]*model.Table, 0, len(o.Entities))
	for _, e := range o.Entities {
		t, err := builder.Build(exampleTableMeta(o.Name, e), builder.Options{Module: o.Name})
		if err != nil {
			return nil, fmt.Errorf("构建实体 %s 失败: %w", e, err)
		}
		tables = append(tables, t)
	}
	files, err := codegen.Generate(tables, codegen.Options{UnitTestEnable: true})
	if err != nil {
		return nil, fmt.Errorf("codegen 生成失败: %w", err)
	}
	if o.SkipFrontend {
		filtered := files[:0]
		for _, f := range files {
			if len(f.Path) >= 8 && f.Path[:8] == "modules/" {
				filtered = append(filtered, f)
			}
		}
		files = filtered
	}
	return files, nil
}
