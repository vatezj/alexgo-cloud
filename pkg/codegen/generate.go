// pkg/codegen/generate.go
package codegen

import (
	"errors"
	"fmt"
	"sort"

	"alexGo-cloud/pkg/codegen/model"
	"alexGo-cloud/pkg/codegen/postprocess"
	"alexGo-cloud/pkg/codegen/template"
)

type Options struct {
	UnitTestEnable bool
}

// Generate 是引擎唯一入口：校验 → 渲染 → 格式化 → 碰撞检查。
// 全部成功才返回产物（按 Path 排序）；任一错误 → (nil, errors.Join(errs...))。
func Generate(tables []*model.Table, opts Options) ([]model.GeneratedFile, error) {
	if len(tables) == 0 {
		return nil, fmt.Errorf("%w: no tables to generate", model.ErrTableInvalid)
	}

	var errs []error
	var files []model.GeneratedFile
	for _, t := range tables {
		if t == nil {
			errs = append(errs, fmt.Errorf("%w: nil table", model.ErrTableInvalid))
			continue
		}
		if err := t.Validate(); err != nil {
			errs = append(errs, err)
			continue
		}
		// 树表固定约定（spec §10.1）：必须在 Render 之前——缺列报 ErrColumnInvalid，
		// 不被 tree 族的 ErrTemplateMissing 掩盖；CLI 与在线预览共用本入口，同一路径生效。
		if err := validateTreeConvention(t); err != nil {
			errs = append(errs, err)
			continue
		}
		fs, err := template.Render(t, template.Options{UnitTestEnable: opts.UnitTestEnable})
		if err != nil {
			errs = append(errs, err)
			continue
		}
		files = append(files, fs...)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	var ferrs []error
	for i, f := range files {
		out, err := postprocess.FormatGo(f.Path, f.Content)
		if err != nil {
			ferrs = append(ferrs, err)
			continue
		}
		files[i].Content = out
	}
	if len(ferrs) > 0 {
		return nil, errors.Join(ferrs...)
	}

	if err := postprocess.CheckCollide(files); err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

// validateTreeConvention 树表固定约定（spec §10.1）：parent_id（整型自引用，根=0）
// 与 name（显示名）两列必须存在。不做可配置列名（YAGNI，与房规 SystemMenusPage 同款）。
func validateTreeConvention(t *model.Table) error {
	if t.TemplateType != model.TemplateTypeTree {
		return nil
	}
	has := make(map[string]bool, len(t.Columns))
	for _, c := range t.Columns {
		has[c.Name] = true
	}
	if !has["parent_id"] {
		return fmt.Errorf("%w: %s: tree table requires parent_id column (spec §10.1)", model.ErrColumnInvalid, t.TableName)
	}
	if !has["name"] {
		return fmt.Errorf("%w: %s: tree table requires name column (spec §10.1)", model.ErrColumnInvalid, t.TableName)
	}
	return nil
}
