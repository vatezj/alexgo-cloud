package template

import (
	"strings"
	texttpl "text/template"

	"alexGo-cloud/pkg/codegen/naming"
)

// FuncMap 模板可用函数。
func FuncMap() texttpl.FuncMap {
	return texttpl.FuncMap{
		"lower":  strings.ToLower,
		"snake":  naming.Snake,
		"plural": naming.Plural,
	}
}
