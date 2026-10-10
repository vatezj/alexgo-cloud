// pkg/codegen/template/embed.go
package template

import "embed"

// tplFS 内嵌全部模板（引擎在任意 cwd 下可用，摆脱旧工具对 scripts/templates 的路径依赖）。
//
//go:embed templates
var tplFS embed.FS

func TemplateFS() embed.FS { return tplFS }
