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
		t.Error("缺少 import")
	}
	// import 字母序：demo 落在 member 之前
	if strings.Index(got, `"alexGo-cloud/modules/demo"`) > strings.Index(got, `"alexGo-cloud/modules/member"`) {
		t.Error("demo import 应排在 member 之前")
	}
	if !strings.Contains(got, "demo.FxModule,") {
		t.Error("缺少 opts 装配行")
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
			t.Errorf("丢了原有内容 %q", keep)
		}
	}
	// 装配行必须在 member 条件分支之前
	if strings.Index(got, "demo.FxModule,") > strings.Index(got, `if cfg.Deployment.Mode != "micro"`) {
		t.Error("demo.FxModule 应在 member 条件分支之前")
	}
	// 装配行恰出现一次
	if n := strings.Count(got, "demo.FxModule"); n != 1 {
		t.Errorf("demo.FxModule 应恰出现一次，实际 %d 次", n)
	}
	// 装配行必须独占一行（不与 order.FxModule 等同行打包）
	for _, line := range strings.Split(got, "\n") {
		if !strings.Contains(line, "demo.FxModule") {
			continue
		}
		if strings.Count(line, "FxModule") != 1 {
			t.Errorf("demo.FxModule 应独占一行，实际整行：%q", line)
		}
	}
	// 结果必须是合法 Go
	if _, err := parser.ParseFile(token.NewFileSet(), "main.go", out, parser.AllErrors); err != nil {
		t.Fatalf("输出不是合法 Go：%v", err)
	}
	// 幂等拒绝：再插一次同名模块应报错
	if _, err := editMainGo(out, "demo"); err == nil {
		t.Error("重复插入应报错")
	}
}

func TestEditMainGo_AnchorMissing(t *testing.T) {
	if _, err := editMainGo([]byte("package main\n"), "demo"); err == nil {
		t.Error("锚点缺失应报错")
	}
}
