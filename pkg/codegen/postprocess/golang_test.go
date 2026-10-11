// pkg/codegen/postprocess/golang_test.go
package postprocess

import "testing"

// 非 .go 文件（.vue/.ts/.sql，M4 前端输出）不得送进 gofmt——
// format.Source 对非 Go 源会报错，导致整批生成失败。
func TestFormatGo_NonGoPassthrough(t *testing.T) {
	for _, path := range []string{"a.vue", "b.ts", "c.sql", "README.md", "noext"} {
		src := []byte("const a = 1\n  <div>not go</div>\n")
		got, err := FormatGo(path, src)
		if err != nil {
			t.Errorf("FormatGo(%q) err = %v, want nil passthrough", path, err)
		}
		if string(got) != string(src) {
			t.Errorf("FormatGo(%q) 改变了内容", path)
		}
	}
}

// .go 仍正常 gofmt。
func TestFormatGo_GoStillFormatted(t *testing.T) {
	got, err := FormatGo("x.go", []byte("package  p\n"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "package p\n" {
		t.Errorf("got %q", got)
	}
}
