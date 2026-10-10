// pkg/codegen/template/templates_test.go
package template_test

import (
	"io/fs"
	"testing"
	texttpl "text/template"

	"alexGo-cloud/pkg/codegen/template"
)

// TestTemplatesParse 解析 embedFS 内全部模板；语法错误 → 失败。
func TestTemplatesParse(t *testing.T) {
	fsys := template.TemplateFS()
	err := fs.WalkDir(fsys, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if _, perr := texttpl.New(path).Funcs(template.FuncMap()).ParseFS(fsys, path); perr != nil {
			t.Errorf("parse %s: %v", path, perr)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}
