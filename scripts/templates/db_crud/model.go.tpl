package model

{{- if or .Imports.Time .Imports.JSON }}
import (
{{- if .Imports.JSON }}
	"encoding/json"
{{- end }}
{{- if .Imports.Time }}
	"time"
{{- end }}
)
{{- end }}

type {{.Entity}} struct {
{{- range .Columns }}
	{{.GoName}} {{.GoType}} `gorm:"{{.GormTag}}" json:"{{.JSONTag}}"`
{{- end }}
}

func ({{.Entity}}) TableName() string { return "{{.Name}}" }

