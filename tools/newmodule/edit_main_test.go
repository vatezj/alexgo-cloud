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

// TestEditMainGo_LongName 长名（≥5 字符）字节级保真：输出必须 = 原文恰好插入
// 2 行（import + opts 装配行），删掉这两行后与原字节逐字节相等。
// demo 短名看不出的注释错位（go/printer 整文件重打印把 opts 上方注释冲进
// <name>.FxModule 中间）在这里必须被抓红。
func TestEditMainGo_LongName(t *testing.T) {
	root := repoRoot(t)
	src, err := os.ReadFile(filepath.Join(root, "alexgo-server/cmd/main.go"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := editMainGo(src, "product")
	if err != nil {
		t.Fatalf("editMainGo(product) = %v", err)
	}
	got := string(out)

	const impLine = `"alexGo-cloud/modules/product"`
	const fxLine = "product.FxModule,"
	if strings.Count(got, impLine) != 1 {
		t.Fatalf("import 行出现 %d 次，应为 1 次", strings.Count(got, impLine))
	}
	if strings.Count(got, fxLine) != 1 {
		t.Fatalf("装配行出现 %d 次，应为 1 次", strings.Count(got, fxLine))
	}
	// import 字典序：product 落在 system 之前、order 之后
	if strings.Index(got, impLine) > strings.Index(got, `"alexGo-cloud/modules/system"`) {
		t.Error("product import 应排在 system 之前")
	}
	if strings.Index(got, impLine) < strings.Index(got, `"alexGo-cloud/modules/order"`) {
		t.Error("product import 应排在 order 之后")
	}
	// 字节级：删掉恰好这 2 行后必须与原文全等（任何注释错位/重排/拆行都会红）
	outLines := strings.Split(got, "\n")
	srcLines := strings.Split(string(src), "\n")
	if len(outLines) != len(srcLines)+2 {
		t.Errorf("行数应为原行数+2: got %d want %d", len(outLines), len(srcLines)+2)
	}
	kept := make([]string, 0, len(outLines))
	for _, line := range outLines {
		if strings.TrimSpace(line) == impLine || strings.TrimSpace(line) == fxLine {
			continue
		}
		kept = append(kept, line)
	}
	if strings.Join(kept, "\n") != string(src) {
		t.Error("输出不是原文件恰好插入 2 行（发生了重排/注释错位）")
	}
	// 缩进锚定：跟随各自既有锚行（既有本地 import 行 / order.FxModule, 行）的缩进
	wantIndent := map[string]string{}
	for _, l := range srcLines {
		switch strings.TrimSpace(l) {
		case `"alexGo-cloud/modules/order"`:
			wantIndent[impLine] = leadingWS(l)
		case "order.FxModule,":
			wantIndent[fxLine] = leadingWS(l)
		}
	}
	if len(wantIndent) != 2 {
		t.Fatalf("原文未找到缩进锚行（本地 import 行 / order.FxModule, 行）")
	}
	for _, line := range outLines {
		trim := strings.TrimSpace(line)
		want, ok := wantIndent[trim]
		if !ok {
			continue
		}
		if got := leadingWS(line); got != want {
			t.Errorf("%s 行缩进 = %q, 应为 %q", trim, got, want)
		}
	}
	// 合法 Go
	if _, err := parser.ParseFile(token.NewFileSet(), "main.go", out, parser.AllErrors); err != nil {
		t.Fatalf("输出不是合法 Go：%v", err)
	}
	// 幂等拒绝
	if _, err := editMainGo(out, "product"); err == nil {
		t.Error("重复插入应报错")
	}
}

func TestEditMainGo_AnchorMissing(t *testing.T) {
	if _, err := editMainGo([]byte("package main\n"), "demo"); err == nil {
		t.Error("锚点缺失应报错")
	}
}
