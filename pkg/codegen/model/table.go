package model

import "fmt"

type Table struct {
	TableName     string
	TableComment  string
	Module        string // 目标模块，如 order
	BusinessName  string // 业务名，如 order_item
	ClassName     string // 类名，如 OrderItem
	TemplateType  TemplateType
	FrontType     FrontType
	ParentTableID int64 // 主子表：子表指向主表配置行；0=无
	Remark        string
	HasTenant     bool // 含 tenant_id 列
	Columns       []Column
}

func (t *Table) Validate() error {
	if t == nil {
		return fmt.Errorf("%w: nil table", ErrTableInvalid)
	}
	if t.TableName == "" {
		return fmt.Errorf("%w: empty table name", ErrTableInvalid)
	}
	if t.Module == "" {
		return fmt.Errorf("%w: %s: empty module", ErrTableInvalid, t.TableName)
	}
	if t.ClassName == "" {
		return fmt.Errorf("%w: %s: empty class name", ErrTableInvalid, t.TableName)
	}
	if !t.TemplateType.Valid() {
		return fmt.Errorf("%w: %s: template type %d", ErrTableInvalid, t.TableName, t.TemplateType)
	}
	if !t.FrontType.Valid() {
		return fmt.Errorf("%w: %s: front type %d", ErrTableInvalid, t.TableName, t.FrontType)
	}
	if len(t.Columns) == 0 {
		return fmt.Errorf("%w: %s: no columns", ErrColumnInvalid, t.TableName)
	}
	seen := make(map[string]bool, len(t.Columns))
	for _, c := range t.Columns {
		if c.Name == "" || c.GoName == "" || c.GoType == "" {
			return fmt.Errorf("%w: %s: column missing name/gotype: %+v", ErrColumnInvalid, t.TableName, c)
		}
		if seen[c.GoName] {
			return fmt.Errorf("%w: %s: duplicate go name %s", ErrColumnInvalid, t.TableName, c.GoName)
		}
		seen[c.GoName] = true
	}
	return nil
}
