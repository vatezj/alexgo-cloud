// pkg/codegen/template/templates_test.go
package template_test

import (
	"io/fs"
	"strings"
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
		tpl := texttpl.New(path).Funcs(template.FuncMap())
		if strings.HasPrefix(path, "templates/front/") {
			tpl.Delims("[[", "]]")
		}
		if _, perr := tpl.ParseFS(fsys, path); perr != nil {
			t.Errorf("parse %s: %v", path, perr)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}

func TestFuncMapPascal(t *testing.T) {
	fm := template.FuncMap()
	fn, ok := fm["pascal"]
	if !ok {
		t.Fatal("FuncMap missing pascal")
	}
	f, ok := fn.(func(string) string)
	if !ok {
		t.Fatalf("pascal type = %T", fn)
	}
	if got := f("codegen_demo_item"); got != "CodegenDemoItem" {
		t.Errorf("pascal = %q", got)
	}
}
