package newmodule

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"
)

// editMainGo AST 级插入：
//  1. import 块按字母序插入 "alexGo-cloud/modules/<name>"（重复则报错）；
//  2. opts := []fx.Option{...} 里最后一个 <pkg>.FxModule 之后插入 <name>.FxModule,。
//
// 用 format.Node 整文件输出（main.go 本身 gofmt 干净，重排无损）。
func editMainGo(src []byte, moduleName string) ([]byte, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("解析 main.go: %w", err)
	}

	impPath := `"alexGo-cloud/modules/` + moduleName + `"`

	// ① import
	hasImport := false
	for _, imp := range f.Imports {
		if imp.Path.Value == impPath {
			hasImport = true
		}
	}
	if hasImport {
		return nil, fmt.Errorf("main.go 已 import %s", impPath)
	}
	impDecl := findImportDecl(f)
	if impDecl == nil {
		return nil, fmt.Errorf("main.go 未找到 import 块（锚点失效）")
	}
	// 在 alexGo-cloud 本地 import 段内按字典序找插入点：第一个 Value > impPath 的
	// alexGo-cloud/* spec 之前；都小则插到该段末尾。注意真实文件的 modules 段里
	// 混有 modules/system/grpcserver、modules/system/service——比较用完整路径字面量，
	// "modules/demo" < "modules/member" → 落在 member 前（字典序正确）。
	newImp := &ast.ImportSpec{Path: &ast.BasicLit{Kind: token.STRING, Value: impPath}}
	var localIdx []int // alexGo-cloud/* 在 impDecl.Specs 里的下标
	for i, spec := range impDecl.Specs {
		if imp, ok := spec.(*ast.ImportSpec); ok && strings.HasPrefix(imp.Path.Value, `"alexGo-cloud/`) {
			localIdx = append(localIdx, i)
		}
	}
	if len(localIdx) == 0 {
		return nil, fmt.Errorf("main.go 未找到 alexGo-cloud 本地 import 段（锚点失效）")
	}
	insertAt := -1
	for _, i := range localIdx {
		if imp, ok := impDecl.Specs[i].(*ast.ImportSpec); ok && imp.Path.Value > impPath {
			insertAt = i
			break
		}
	}
	if insertAt < 0 {
		insertAt = localIdx[len(localIdx)-1] + 1 // 段末尾之后
	}
	specs := impDecl.Specs
	specs = append(specs[:insertAt], append([]ast.Spec{newImp}, specs[insertAt:]...)...)
	impDecl.Specs = specs

	// ② opts 列表
	lit := findOptsCompositeLit(f)
	if lit == nil {
		return nil, fmt.Errorf("main.go 未找到 opts := []fx.Option{...}（锚点失效）")
	}
	// 已有 <name>.FxModule？（幂等拒绝）
	pkgIdent := moduleName
	for _, elt := range lit.Elts {
		if sel, ok := elt.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == pkgIdent && sel.Sel.Name == "FxModule" {
				return nil, fmt.Errorf("main.go opts 已含 %s.FxModule", pkgIdent)
			}
		}
	}
	// 定位最后一个 X.FxModule 元素，插到它后面
	last := -1
	for i, elt := range lit.Elts {
		if sel, ok := elt.(*ast.SelectorExpr); ok && sel.Sel.Name == "FxModule" {
			last = i
		}
	}
	if last < 0 {
		return nil, fmt.Errorf("main.go opts 里没有 xxx.FxModule 元素（锚点失效）")
	}
	newElt := &ast.SelectorExpr{
		X:   ast.NewIdent(pkgIdent),
		Sel: ast.NewIdent("FxModule"),
	}
	// SelectorExpr 没有 End 位置信息用于 Doc，构造带假位置的 ExprStmt 不行——
	// CompositeLit.Elts 是 []Expr，直接插元素即可；逗号由 format 输出。
	elts := lit.Elts
	lit.Elts = append(elts[:last+1], append([]ast.Expr{newElt}, elts[last+1:]...)...)

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, f); err != nil {
		return nil, fmt.Errorf("输出 main.go: %w", err)
	}
	return forceFxModuleOwnLine(buf.Bytes(), pkgIdent), nil
}

// forceFxModuleOwnLine 保证刚插入的 <pkg>.FxModule 独占一行。
// 根因：新元素是 NoPos，format.Node 会把它接在前一元素同行
// （如 "order.FxModule, demo.FxModule,"），且 gofmt 对已打包的行不会自动拆。
// 故打印后按行拆分：前缀（右 trim 后）以逗号结尾说明被同行打包，按该行缩进拆成两行；
// gofmt 会保留复合字面量元素的既有换行，拆分结果 gofmt 稳定。
func forceFxModuleOwnLine(out []byte, pkgIdent string) []byte {
	marker := pkgIdent + ".FxModule"
	lines := strings.Split(string(out), "\n")
	for i, line := range lines {
		idx := strings.Index(line, marker)
		if idx < 0 {
			continue
		}
		prefix := strings.TrimRight(line[:idx], " \t")
		if !strings.HasSuffix(prefix, ",") {
			continue // 已独占一行（前面只有缩进），或不是装配行
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		lines[i] = prefix + "\n" + indent + line[idx:]
		return []byte(strings.Join(lines, "\n"))
	}
	return out
}

// findImportDecl 找第一个 import GenDecl。
func findImportDecl(f *ast.File) *ast.GenDecl {
	for _, decl := range f.Decls {
		if gd, ok := decl.(*ast.GenDecl); ok && gd.Tok == token.IMPORT {
			return gd
		}
	}
	return nil
}

// findOptsCompositeLit 找 opts := []fx.Option{...} 的 CompositeLit。
func findOptsCompositeLit(f *ast.File) *ast.CompositeLit {
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		for _, stmt := range fn.Body.List {
			as, ok := stmt.(*ast.AssignStmt)
			if !ok || as.Tok != token.DEFINE || len(as.Lhs) != 1 {
				continue
			}
			id, ok := as.Lhs[0].(*ast.Ident)
			if !ok || id.Name != "opts" {
				continue
			}
			if lit, ok := as.Rhs[0].(*ast.CompositeLit); ok {
				return lit
			}
		}
	}
	return nil
}
