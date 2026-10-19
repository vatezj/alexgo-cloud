package newmodule

import (
	"os"
	"strings"
	"testing"
	"time"
)

func validOpts() Options {
	return Options{
		Name:     "product",
		Entities: []string{"product", "category"},
		Now:      time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC),
	}
}

func TestOptionsValidate_OK(t *testing.T) {
	o := validOpts()
	o.Root = repoRoot(t)
	if err := o.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
}

func TestOptionsValidate_Defaults(t *testing.T) {
	o := validOpts()
	o.Root = ""
	o.MenuPrefix = ""
	_ = o.Validate()
	if o.MenuPrefix != "product" {
		t.Errorf("MenuPrefix default = %q, want product", o.MenuPrefix)
	}
	if o.Root != "." {
		t.Errorf("Root default = %q", o.Root)
	}
	if o.Now.IsZero() {
		t.Error("Now default should be set")
	}
}

func TestOptionsValidate_Rejects(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*Options)
		wantSub string
	}{
		{"empty name", func(o *Options) { o.Name = "" }, "模块名"},
		{"uppercase name", func(o *Options) { o.Name = "Product" }, "小写"},
		{"dash in name", func(o *Options) { o.Name = "my-mod" }, "小写"},
		{"no entities", func(o *Options) { o.Entities = nil }, "实体"},
		{"uppercase entity", func(o *Options) { o.Entities = []string{"Product"} }, "小写"},
		{"dup entity", func(o *Options) { o.Entities = []string{"a", "a"} }, "重复"},
		{"reserved module", func(o *Options) { o.Name = "system" }, "保留"},
		{"go keyword", func(o *Options) { o.Name = "type" }, "保留"},
		{"existing module dir", func(o *Options) { o.Name = "order" }, "已存在"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := validOpts()
			o.Root = repoRoot(t) // 「模块已存在」用例依赖仓库真实 modules/order
			tc.mutate(&o)
			err := o.Validate()
			if err == nil {
				t.Fatal("Validate() = nil, want error")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("err = %v, want substring %q", err, tc.wantSub)
			}
		})
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root, err := repoRootDir(wd)
	if err != nil {
		t.Fatal(err)
	}
	return root
}
