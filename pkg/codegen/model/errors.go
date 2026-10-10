package model

import "errors"

var (
	ErrMetadataUnreachable = errors.New("codegen: metadata unreachable")
	ErrTableNotFound       = errors.New("codegen: table not found")
	ErrTableInvalid        = errors.New("codegen: invalid table")
	ErrColumnInvalid       = errors.New("codegen: invalid column")
	ErrTypeMappingUnknown  = errors.New("codegen: unknown type mapping")
	ErrTemplateMissing     = errors.New("codegen: template missing")
	ErrRender              = errors.New("codegen: render failed")
	ErrFormat              = errors.New("codegen: format failed")
	ErrPathCollide         = errors.New("codegen: output path collision")
)
