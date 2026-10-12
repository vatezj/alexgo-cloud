package service

import (
	"fmt"
	"sort"

	"alexGo-cloud/modules/infra/model"
	"alexGo-cloud/pkg/codegen/builder"
	"alexGo-cloud/pkg/codegen/metadata"
	cgmodel "alexGo-cloud/pkg/codegen/model"
	"alexGo-cloud/pkg/codegen/naming"
)

// TableFromConfig 配置行（codegen_table + codegen_column）→ 引擎输入模型。
// 预览 / ZIP / CLI 配置模式共用（spec §2：配置表行是一等引擎输入）。
// GoName/GormTag 配置表不存（导入只落快照+开关）——按列名/主键/自增重算；
// JSONName 与全部开关取存值；deprecated 列已从库消失，剔除不生成。
func TableFromConfig(t *model.CodegenTable, cols []*model.CodegenColumn) (*cgmodel.Table, error) {
	if t == nil {
		return nil, fmt.Errorf("%w: nil table", cgmodel.ErrTableInvalid)
	}
	sorted := make([]*model.CodegenColumn, 0, len(cols))
	for _, c := range cols {
		if c != nil {
			sorted = append(sorted, c)
		}
	}
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].SortOrder < sorted[j].SortOrder })

	out := make([]cgmodel.Column, 0, len(sorted))
	hasTenant := false
	for _, c := range sorted {
		if c.Deprecated {
			continue
		}
		if c.Name == "tenant_id" {
			hasTenant = true
		}
		key, extra := "", ""
		if c.IsPK {
			key = "PRI"
		}
		if c.AutoIncrement {
			extra = "auto_increment"
		}
		out = append(out, cgmodel.Column{
			Name:    c.Name,
			Comment: c.Comment,
			GoName:  naming.ToGoName(c.Name),
			GoType:  c.GoType,
			// 导入时已存（tenant_id 为 "-"、敏感字段为 "-"）
			JSONName: c.JSONName,
			GormTag: builder.BuildGormTag(metadata.ColumnMeta{
				Name: c.Name, Key: key, Extra: extra,
			}),
			HTMLType:      cgmodel.HTMLType(c.HTMLType),
			IsPK:          c.IsPK,
			AutoIncrement: c.AutoIncrement,
			Nullable:      c.Nullable,
			ListEnable:    c.ListEnable,
			FormEnable:    c.FormEnable,
			QueryEnable:   c.QueryEnable,
			QueryOp:       c.QueryOperation,
			ListRequired:  c.ListRequired,
			FormRequired:  c.FormRequired,
			DictType:      c.DictType,
			SortOrder:     c.SortOrder,
			Deprecated:    false, // 上面已过滤
		})
	}
	tbl := &cgmodel.Table{
		TableName:     t.Name,
		TableComment:  t.TableComment,
		Module:        t.Module,
		BusinessName:  t.BusinessName,
		ClassName:     t.ClassName,
		TemplateType:  cgmodel.TemplateType(t.TemplateType),
		FrontType:     cgmodel.FrontType(t.FrontType),
		ParentTableID: int64(t.ParentTableID), // uint64 → int64
		Remark:        t.Remark,
		HasTenant:     hasTenant,
		Columns:       out,
	}
	// 提前校验：坏配置（类名空/模板值非法/列空/GoName 撞车）在渲染前以 400 暴露
	if err := tbl.Validate(); err != nil {
		return nil, err
	}
	return tbl, nil
}
