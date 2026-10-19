package newmodule

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditMainGo(t *testing.T) {
	root := repoRoot(t)
	src, err := os.ReadFile(filepath.Join(root, "alexgo-server/cmd/main.go"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := editMainGo(src, "demo")
	if err != nil {
		t.Fatalf("editMainGo() = %v", err)
	}
	got := string(out)

	// 新 import 与装配行
	if !strings.Contains(got, `"alexGo-cloud/modules/demo"`) {
		t.Error("missing import")
	}
	// import 字母序：demo 落在 member 之前
	if strings.Index(got, `"alexGo-cloud/modules/demo"`) > strings.Index(got, `"alexGo-cloud/modules/member"`) {
		t.Error("demo import should sort before member")
	}
	if !strings.Contains(got, "demo.FxModule,") {
		t.Error("missing opts entry")
	}
	// 既有内容不丢（抽查关键行）
	for _, keep := range []string{
		`"alexGo-cloud/modules/order"`,
		"order.FxModule,",
		"system.FxModule,",
		"member.FxModule",
		`if cfg.Deployment.Mode != "micro"`,
	} {
		if !strings.Contains(got, keep) {
			t.Errorf("lost original content %q", keep)
		}
	}
	// 装配行必须在 member 条件分支之前
	if strings.Index(got, "demo.FxModule,") > strings.Index(got, `if cfg.Deployment.Mode != "micro"`) {
		t.Error("demo.FxModule should be before member conditional branch")
	}
	// 结果必须是合法 Go
	if _, err := parser.ParseFile(token.NewFileSet(), "main.go", out, parser.AllErrors); err != nil {
		t.Fatalf("output not parseable: %v", err)
	}
	// 幂等拒绝：再插一次同名模块应报错
	if _, err := editMainGo(out, "demo"); err == nil {
		t.Error("second insert should fail (duplicate)")
	}
}

func TestEditMainGo_AnchorMissing(t *testing.T) {
	if _, err := editMainGo([]byte("package main\n"), "demo"); err == nil {
		t.Error("want error on missing anchors")
	}
}
