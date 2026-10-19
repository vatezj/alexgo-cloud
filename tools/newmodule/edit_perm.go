package newmodule

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"sort"

	"alexGo-cloud/pkg/codegen/naming"
)

// editPermissionRoutes 在 permissionRoutes map 字面量里插入条目。
// key = "<module>:<snake(entity)>"；路由 = /api/admin/<module>/<plural> + /* 变体
// （plural 与 module_register 生成的挂载路径同源，都走 naming.Plural）。
//
// 插入策略：位置插入，绝不整体重排——既有 map 并非全字典序（system:user 在
// system:role 之前，按业务域分组），重排会造成巨大无关 diff（Review Focus #1）。
// 每条新 key 插到「第一条字典序更大的既有 key」之前；都小则追加末尾。
// 新条目之间按 key 排序后再依次插入。
func editPermissionRoutes(src []byte, module string, entities []string) ([]byte, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "permission_routes.go", src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("解析 permission_routes.go: %w", err)
	}

	lit := findPermissionRoutesLit(f)
	if lit == nil {
		return nil, fmt.Errorf("permission_routes.go 未找到 permissionRoutes 字面量（锚点失效）")
	}

	// 既有 key 集合
	existing := map[string]bool{}
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if bl, ok := kv.Key.(*ast.BasicLit); ok {
			existing[unquote(bl.Value)] = true
		}
	}

	// 新条目
	type newKV struct {
		key  string
		expr ast.Expr
	}
	var news []newKV
	for _, e := range entities {
		snake := naming.Snake(naming.EntityName(naming.Plural(e)))
		key := module + ":" + snake
		if existing[key] {
			return nil, fmt.Errorf("permissionRoutes 已含 %q", key)
		}
		plural := naming.Plural(naming.EntityName(naming.Plural(e)))
		inner := []ast.Expr{
			stringLit("/api/admin/" + module + "/" + plural),
			stringLit("/api/admin/" + module + "/" + plural + "/*"),
		}
		expr := &ast.CompositeLit{Type: &ast.ArrayType{Elt: ast.NewIdent("string")}, Elts: inner}
		news = append(news, newKV{key: key, expr: &ast.KeyValueExpr{Key: stringLit(key), Value: expr}})
	}
	sort.Slice(news, func(i, j int) bool { return news[i].key < news[j].key })

	// 位置插入：每条新 key 落到「第一条字典序更大的既有 key」之前，都小则末尾。
	// 既有 Elts 的相对顺序原样保留（map 里元素只会是 KeyValueExpr）。
	for _, n := range news {
		insertAt := len(lit.Elts)
		for i, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			bl, ok := kv.Key.(*ast.BasicLit)
			if !ok {
				continue
			}
			if unquote(bl.Value) > n.key {
				insertAt = i
				break
			}
		}
		lit.Elts = append(lit.Elts[:insertAt], append([]ast.Expr{n.expr}, lit.Elts[insertAt:]...)...)
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, f); err != nil {
		return nil, fmt.Errorf("输出 permission_routes.go: %w", err)
	}
	return buf.Bytes(), nil
}

// findPermissionRoutesLit 定位 var permissionRoutes = map[string][]string{...}。
func findPermissionRoutesLit(f *ast.File) *ast.CompositeLit {
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Names) != 1 || vs.Names[0].Name != "permissionRoutes" {
				continue
			}
			if len(vs.Values) == 0 {
				continue
			}
			if lit, ok := vs.Values[0].(*ast.CompositeLit); ok {
				return lit
			}
		}
	}
	return nil
}

func stringLit(s string) ast.Expr {
	return &ast.BasicLit{Kind: token.STRING, Value: `"` + s + `"`}
}

func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func parseGo(name string, src []byte) (*ast.File, error) {
	return parser.ParseFile(token.NewFileSet(), name, src, parser.AllErrors)
}
