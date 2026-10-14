package template

import (
	"bytes"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	texttpl "text/template"

	"alexGo-cloud/pkg/codegen/model"
)

// Render 单表渲染：模板族 → 文件列表（按 Path 排序）。错误聚合返回。
func Render(t *model.Table, opts Options) ([]model.GeneratedFile, error) {
	tplMap, err := templateMapFor(t)
	if err != nil {
		return nil, err
	}
	b := NewBind(t)

	var files []model.GeneratedFile
	var errs []error
	for name, pattern := range tplMap {
		if !opts.UnitTestEnable && isTestTemplate(name) {
			continue
		}
		// ParseFS 按文件 base 名关联内容；接收者必须同名，否则 Execute 拿到空模板。
		t := texttpl.New(path.Base(name)).Funcs(FuncMap())
		// front 模板与 Vue 插值撞默认 delims → front 族切 [[ ]]（spec §3）。
		if strings.HasPrefix(name, "front/") {
			t.Delims("[[", "]]")
		}
		tpl, perr := t.ParseFS(tplFS, "templates/"+name)
		if perr != nil {
			errs = append(errs, fmt.Errorf("%w: %s: %v", model.ErrRender, name, perr))
			continue
		}
		var buf bytes.Buffer
		if eerr := tpl.Execute(&buf, b); eerr != nil {
			errs = append(errs, fmt.Errorf("%w: %s: %v", model.ErrRender, name, eerr))
			continue
		}
		files = append(files, model.GeneratedFile{
			Path:    RenderPath(pattern, b),
			Content: buf.Bytes(),
		})
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}
