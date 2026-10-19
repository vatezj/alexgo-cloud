package newmodule

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"sort"
	"strings"
)

// editMainGo 解析定位 + 原字节行插入（T6/T7 同款契约）：
//  1. import 块按字典序插入 "alexGo-cloud/modules/<name>"（已存在则报错）；
//  2. opts := []fx.Option{...} 里最后一个 <pkg>.FxModule, 行之后插入 <name>.FxModule,（已有则报错）。
//
// parser 只做校验与锚点定位；落盘一律「原始字节 + 单行文本插入」，
// 除这 2 个插入行外原文件字节一字不动。
// 绝不 format.Node 整文件重打印：go/printer 对 ≥5 字符模块名会把 opts 上方
// 注释块冲进 <name>.FxModule 中间（注释错位 + 装配行被拆成 "product." / "FxModule,"
// 两截，forceFxModuleOwnLine 也找不到连续 marker），行插入才是字节级保真（终审 C1）。
func editMainGo(src []byte, moduleName string) ([]byte, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("解析 main.go: %w", err)
	}

	impPath := `"alexGo-cloud/modules/` + moduleName + `"`

	// ① import：已存在 → 幂等拒绝
	for _, imp := range f.Imports {
		if imp.Path.Value == impPath {
			return nil, fmt.Errorf("main.go 已 import %s", impPath)
		}
	}
	impDecl := findImportDecl(f)
	if impDecl == nil {
		return nil, fmt.Errorf("main.go 未找到 import 块（锚点失效）")
	}
	// alexGo-cloud 本地段：真实文件的 modules 段里混有 modules/system/grpcserver、
	// modules/system/service——比较用完整路径字典序（"modules/order" < "modules/product"
	// < "modules/system"）。锚行 = 段内第一条路径字典序更大的行；都小则插在段末之后。
	var locals []localImp
	for _, spec := range impDecl.Specs {
		imp, ok := spec.(*ast.ImportSpec)
		if !ok || !strings.HasPrefix(imp.Path.Value, `"alexGo-cloud/`) {
			continue
		}
		locals = append(locals, localImp{path: imp.Path.Value, line: fset.Position(imp.Pos()).Line})
	}
	if len(locals) == 0 {
		return nil, fmt.Errorf("main.go 未找到 alexGo-cloud 本地 import 段（锚点失效）")
	}
	impLine, impBefore := -1, true
	for _, l := range locals {
		if l.path > impPath {
			impLine, impBefore = l.line, true
			break
		}
	}
	if impLine < 0 { // 段末之后
		impLine, impBefore = locals[len(locals)-1].line, false
	}
	srcLines := splitLines(src)
	if impLine < 1 || impLine > len(srcLines) {
		return nil, fmt.Errorf("main.go import 锚行 %d 越界（文件 %d 行）", impLine, len(srcLines))
	}
	// 锚行必须独占一条 import（一行多 import 时无法按行定位插入点 → 报错而非猜位置）
	anchorLine := srcLines[impLine-1]
	if n := len(quotedPathRe.FindAllString(anchorLine, -1)); n != 1 {
		return nil, fmt.Errorf("main.go import 锚行 %d 含 %d 条 import，无法行定位（锚点失效）", impLine, n)
	}
	impIndent := leadingWS(anchorLine)

	// ② opts 字面量：仅校验 + 定位
	lit := findOptsCompositeLit(f)
	if lit == nil {
		return nil, fmt.Errorf("main.go 未找到 opts := []fx.Option{...}（锚点失效）")
	}
	for _, elt := range lit.Elts {
		sel, ok := elt.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == moduleName && sel.Sel.Name == "FxModule" {
			return nil, fmt.Errorf("main.go opts 已含 %s.FxModule", moduleName)
		}
	}
	// opts 锚行 = 文件序中最后一条 `^\s*\w+\.FxModule,\s*$`
	// （真实文件是 174 行 order.FxModule,；member 是条件 append，不在字面量里）。
	optsLine := -1
	for i, line := range srcLines {
		if fxModuleLineRe.MatchString(line) {
			optsLine = i + 1
		}
	}
	if optsLine < 1 {
		return nil, fmt.Errorf("main.go opts 里没有 xxx.FxModule, 装配行（锚点失效）")
	}
	optsIndent := leadingWS(srcLines[optsLine-1])

	// 原字节单行插入：恰好 2 行，其余行原样保留（按行号降序插，避免先行插入挪动锚行）
	ins := []lineIns{
		{line: impLine, before: impBefore, text: impIndent + impPath},
		{line: optsLine, before: false, text: optsIndent + moduleName + ".FxModule,"},
	}
	out, err := insertLines(srcLines, ins)
	if err != nil {
		return nil, err
	}
	res := []byte(strings.Join(out, "\n"))

	// 自检：结果必须仍是合法 Go，且新 import 恰好出现一次
	f2, err := parser.ParseFile(token.NewFileSet(), "main.go", res, parser.AllErrors)
	if err != nil {
		return nil, fmt.Errorf("插入后 main.go 不可解析（锚点错位）: %w", err)
	}
	n := 0
	for _, imp := range f2.Imports {
		if imp.Path.Value == impPath {
			n++
		}
	}
	if n != 1 {
		return nil, fmt.Errorf("插入后 %s 出现 %d 次，应为 1 次", impPath, n)
	}
	return res, nil
}

// localImp 一条 alexGo-cloud 本地 import 的路径与所在行（1-based）。
type localImp struct {
	path string
	line int
}

// fxModuleLineRe 装配行：独占一行的 `\w+.FxModule,`。
var fxModuleLineRe = regexp.MustCompile(`^\s*\w+\.FxModule,\s*$`)

// quotedPathRe import 行上的路径字面量（用于判定锚行是否独占一条 import）。
var quotedPathRe = regexp.MustCompile(`"[^"]*"`)

// lineIns 一次单行插入：插在第 line 行之前（before）或之后。
type lineIns struct {
	line   int
	before bool
	text   string
}

// insertLines 在行切片上做原字节级插入；除插入行外既有行一字不动。
func insertLines(lines []string, ins []lineIns) ([]string, error) {
	sorted := append([]lineIns(nil), ins...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].line != sorted[j].line {
			return sorted[i].line > sorted[j].line // 行号降序：先插后面的行
		}
		return sorted[i].before // 同行号：before 先插（落在 after 之前）
	})
	out := append([]string(nil), lines...)
	for _, in := range sorted {
		if in.line < 1 || in.line > len(out) {
			return nil, fmt.Errorf("main.go 插入行号 %d 越界（当前 %d 行）", in.line, len(out))
		}
		pos := in.line // 1-based 行号 → after 该行 = 下标 in.line
		if in.before {
			pos = in.line - 1
		}
		out = append(out[:pos], append([]string{in.text}, out[pos:]...)...)
	}
	return out, nil
}

func splitLines(src []byte) []string {
	return strings.Split(string(src), "\n")
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
