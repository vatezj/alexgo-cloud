package model

import "time"

type {{.ServiceName}} struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
{{- range .Fields }}
{{- if ne .GoName "ID" }}
	{{.GoName}} {{.GoType}} `json:"{{.JSONName}}"`
{{- end }}
{{- end }}
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
