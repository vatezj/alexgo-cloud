// pkg/codegen/builder/typing.go
package builder

import (
	"fmt"
	"strings"

	"alexGo-cloud/pkg/codegen/metadata"
	"alexGo-cloud/pkg/codegen/model"
)

// MySQLTypeToGo 数据库类型 → Go 类型（可空值类型加 *，string/[]byte 除外）。
func MySQLTypeToGo(c metadata.ColumnMeta) (string, error) {
	ct := strings.ToLower(c.ColumnType)
	dt := strings.ToLower(c.DataType)
	unsigned := strings.Contains(ct, "unsigned")

	var base string
	switch dt {
	case "bigint":
		if unsigned {
			base = "uint64"
		} else {
			base = "int64"
		}
	case "int", "integer", "mediumint":
		if unsigned {
			base = "uint32"
		} else {
			base = "int"
		}
	case "smallint", "tinyint":
		if unsigned {
			base = "uint16"
		} else {
			base = "int"
		}
	case "float", "double":
		base = "float64"
	case "decimal", "numeric":
		base = "string"
	case "char", "varchar", "text", "mediumtext", "longtext", "tinytext",
		"enum", "set", "year":
		base = "string"
	case "json":
		base = "json.RawMessage"
	case "datetime", "timestamp", "date", "time":
		base = "time.Time"
	case "blob", "tinyblob", "mediumblob", "longblob", "binary", "varbinary":
		base = "[]byte"
	default:
		return "", fmt.Errorf("%w: %s (data_type=%s)", model.ErrTypeMappingUnknown, c.ColumnType, c.DataType)
	}

	if c.Nullable && needsPointer(base) {
		return "*" + base, nil
	}
	return base, nil
}

func needsPointer(base string) bool {
	switch base {
	case "string", "[]byte", "json.RawMessage":
		return false
	}
	return true
}

// MySQLTypeToHTML 数据库类型 + Go 类型 → 前端控件类型。
func MySQLTypeToHTML(c metadata.ColumnMeta, goType string) model.HTMLType {
	dt := strings.ToLower(c.DataType)
	ct := strings.ToLower(c.ColumnType)
	switch {
	case strings.Contains(goType, "time.Time"):
		return model.HTMLDatePicker
	case dt == "json":
		return model.HTMLTextarea
	case dt == "text" || dt == "mediumtext" || dt == "longtext" || dt == "tinytext":
		return model.HTMLTextarea
	case dt == "tinyint" && strings.HasPrefix(ct, "tinyint(1)"):
		return model.HTMLSwitch
	case dt == "decimal" || dt == "numeric" || dt == "float" || dt == "double":
		return model.HTMLInputNumber
	case dt == "int" || dt == "integer" || dt == "bigint" ||
		dt == "smallint" || dt == "tinyint" || dt == "mediumint":
		return model.HTMLInputNumber
	default:
		return model.HTMLInput
	}
}

// BuildGormTag 生成 gorm tag 内容（不含引号）。
func BuildGormTag(c metadata.ColumnMeta) string {
	parts := []string{"column:" + c.Name}
	if c.Key == "PRI" {
		parts = append(parts, "primaryKey")
	}
	if strings.Contains(strings.ToLower(c.Extra), "auto_increment") {
		parts = append(parts, "autoIncrement")
	}
	return strings.Join(parts, ";")
}
