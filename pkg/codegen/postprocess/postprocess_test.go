// pkg/codegen/postprocess/postprocess_test.go
package postprocess_test

import (
	"errors"
	"testing"

	"alexGo-cloud/pkg/codegen/model"
	"alexGo-cloud/pkg/codegen/postprocess"
)

func TestFormatGo_Reformats(t *testing.T) {
	src := []byte("package model\ntype A struct{X int}\n")
	out, err := postprocess.FormatGo("model/a.go", src)
	if err != nil {
		t.Fatalf("FormatGo: %v", err)
	}
	want := "package model\n\ntype A struct{ X int }\n"
	if string(out) != want {
		t.Errorf("got %q, want %q", out, want)
	}
}

func TestFormatGo_InvalidSource_Fails(t *testing.T) {
	_, err := postprocess.FormatGo("model/bad.go", []byte("package {{{"))
	if !errors.Is(err, model.ErrFormat) {
		t.Fatalf("err = %v, want ErrFormat", err)
	}
}

func TestCheckCollide(t *testing.T) {
	ok := []model.GeneratedFile{
		{Path: "modules/order/model/a.go"},
		{Path: "modules/order/model/b.go"},
	}
	if err := postprocess.CheckCollide(ok); err != nil {
		t.Fatalf("CheckCollide(ok) = %v, want nil", err)
	}
	bad := []model.GeneratedFile{
		{Path: "modules/order/model/a.go"},
		{Path: "modules/order/model/a.go"},
	}
	if err := postprocess.CheckCollide(bad); !errors.Is(err, model.ErrPathCollide) {
		t.Fatalf("err = %v, want ErrPathCollide", err)
	}
}
