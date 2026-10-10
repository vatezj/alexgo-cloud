// pkg/codegen/builder/builder.go
package builder

import (
	"fmt"
	"strings"

	"alexGo-cloud/pkg/codegen/metadata"
	"alexGo-cloud/pkg/codegen/model"
	"alexGo-cloud/pkg/codegen/naming"
)

type Options struct {
	Module       string
	TemplateType model.TemplateType // 0 → Single
	FrontType    model.FrontType    // 0 → Vben5Antd
}

func Build(meta *metadata.TableMeta, opts Options) (*model.Table, error) {
	if meta == nil {
		return nil, fmt.Errorf("%w: nil meta", model.ErrTableInvalid)
	}
	if len(meta.Columns) == 0 {
		return nil, fmt.Errorf("%w: %s: no columns", model.ErrColumnInvalid, meta.Name)
	}

	tt := opts.TemplateType
	if tt == 0 {
		tt = model.TemplateTypeSingle
	}
	ft := opts.FrontType
	if ft == 0 {
		ft = model.FrontTypeVben5Antd
	}

	entity := naming.EntityName(meta.Name)
	tbl := &model.Table{
		TableName:    meta.Name,
		TableComment: meta.Comment,
		Module:       opts.Module,
		BusinessName: naming.Snake(entity),
		ClassName:    entity,
		TemplateType: tt,
		FrontType:    ft,
	}

	for i, c := range meta.Columns {
		goType, err := MySQLTypeToGo(c)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", meta.Name, c.Name, err)
		}
		isPK := c.Key == "PRI"
		auto := containsAutoIncrement(c.Extra)
		if c.Name == "tenant_id" {
			tbl.HasTenant = true
		}
		required := !c.Nullable && !auto
		tbl.Columns = append(tbl.Columns, model.Column{
			Name:          c.Name,
			Comment:       flattenComment(c.Comment),
			GoName:        naming.ToGoName(c.Name),
			GoType:        goType,
			JSONName:      naming.JSONName(c.Name),
			GormTag:       BuildGormTag(c),
			HTMLType:      MySQLTypeToHTML(c, goType),
			IsPK:          isPK,
			AutoIncrement: auto,
			Nullable:      c.Nullable,
			ListEnable:    true,
			FormEnable:    !(isPK && auto),
			QueryEnable:   true,
			QueryOp:       queryOp(goType),
			ListRequired:  required,
			FormRequired:  required && !isPK,
			SortOrder:     i,
		})
	}

	if err := tbl.Validate(); err != nil {
		return nil, err
	}
	return tbl, nil
}

// flattenComment 多行注释压平为单行（与 metadata.MySQLReader 的处理一致）。
func flattenComment(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func queryOp(goType string) string {
	switch {
	case strings.Contains(goType, "time.Time"):
		return "between"
	case strings.Contains(goType, "string"):
		return "like"
	default:
		return "eq"
	}
}

func containsAutoIncrement(extra string) bool {
	return strings.Contains(strings.ToLower(extra), "auto_increment")
}
