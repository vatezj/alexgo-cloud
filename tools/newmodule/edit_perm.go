package newmodule

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"sort"
	"strings"

	"alexGo-cloud/pkg/codegen/naming"
)

// editPermissionRoutes 在 permissionRoutes map 里按位置插入条目行。
// key = "<module>:<snake(entity)>"；路由 = /api/admin/<module>/<plural> + /* 变体
// （plural 与 module_register 生成的挂载路径同源，都走 naming.Plural）。
//
// 插入策略：位置插入，绝不整体重排——既有 map 并非全字典序（system:user 在
// system:role 之前，按业务域分组），重排会造成巨大无关 diff。
// 每条新 key 插到「文件顺序中第一条字典序更大的既有条目行」之前；都小则插在
// map 闭合 `}` 行之前。新条目之间按 key 排序后依次成行。
//
// 实现为原字节行插入（parser 只做校验与定位，插入行之外的原文件字节一字不动）：
// 不用 format.Node 整文件重打印——go/printer 对插入的无位置节点不产生换行，
// 会把新条目挤进 var 首行，产出超长行排版破坏（R1 裁定方案 B）。
// 唯一的例外是 I1 的局部重对齐：新宽 key 计入目标值列宽后，map 字面量内每条
// 条目行的冒号后空格按目标宽重排（纯空白，值文本一字不动），否则产物 gofmt 不干净。
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

	// 既有条目：按文件出现顺序（Elts 序）记 key 与所在行；条目必须单行，跨行无法行定位。
	type entry struct {
		key  string
		line int
	}
	existing := map[string]bool{}
	var entries []entry
	keyColonWidth := 0 // 既有条目 key+冒号 的最大列宽（不含缩进），供插入行值列对齐
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		bl, ok := kv.Key.(*ast.BasicLit)
		if !ok {
			continue
		}
		k := unquote(bl.Value)
		if fset.Position(kv.Pos()).Line != fset.Position(kv.End()).Line {
			return nil, fmt.Errorf("permissionRoutes 条目 %q 跨多行，无法行定位", k)
		}
		existing[k] = true
		entries = append(entries, entry{key: k, line: fset.Position(kv.Pos()).Line})
		if w := len(bl.Value) + 1; w > keyColonWidth {
			keyColonWidth = w
		}
	}

	// 新条目（key 升序），风格与既有行一致用简写 {...}。
	type newKV struct {
		key   string
		value string // 值文本：{"/api/admin/...", "/api/admin/.../*"}
	}
	var news []newKV
	for _, e := range entities {
		snake := naming.Snake(naming.EntityName(naming.Plural(e)))
		key := module + ":" + snake
		if existing[key] {
			return nil, fmt.Errorf("permissionRoutes 已含 %q", key)
		}
		plural := naming.Plural(naming.EntityName(naming.Plural(e)))
		value := fmt.Sprintf("{%q, %q}",
			"/api/admin/"+module+"/"+plural,
			"/api/admin/"+module+"/"+plural+"/*")
		news = append(news, newKV{key: key, value: value})
	}
	sort.Slice(news, func(i, j int) bool { return news[i].key < news[j].key })

	// 目标值列宽 = max(既有条目, 新条目) 的 key+冒号列宽——新宽 key（如
	// product:category 18 列 > 既有最大 16 列）必须一并计入：只把新行单独加宽，
	// 产物 gofmt -l 会按新最大宽把整个 map 块重对齐，带出十几行无关 diff（终审 I1）。
	targetWidth := keyColonWidth
	for _, n := range news {
		if w := len(`"`+n.key+`"`) + 1; w > targetWidth {
			targetWidth = w
		}
	}

	// 位置插入：每条新 key 的锚 = 文件顺序中第一条字典序更大的既有条目行；
	// 都小则锚 = map 闭合 `}` 行。news 已按 key 升序，锚行号单调不减，
	// 同锚组内天然保持 key 升序。
	rbraceLine := fset.Position(lit.Rbrace).Line
	type insRow struct {
		keyQ   string // 带引号 key
		spaces int    // 冒号后空格数（对齐到既有值列）
		value  string
	}
	anchors := map[int][]insRow{} // 锚行（1-based）→ 待插行
	for _, n := range news {
		anchor := rbraceLine
		for _, en := range entries {
			if en.key > n.key {
				anchor = en.line
				break
			}
		}
		keyQ := `"` + n.key + `"`
		anchors[anchor] = append(anchors[anchor], insRow{keyQ: keyQ, spaces: spacesAfterColon(targetWidth, len(keyQ)), value: n.value})
	}

	// 原字节行拼接：仅在锚行前插入生成行；map 块内既有条目行按 targetWidth
	// 做纯空白重对齐（只动冒号后空格数，值文本/其它行一字不动），范围严格限于
	// map 字面量内的条目行——这是局部对齐，不是整文档重打印。
	entryLines := map[int]bool{}
	for _, en := range entries {
		entryLines[en.line] = true
	}
	lines := strings.Split(string(src), "\n")
	for line := range anchors {
		if line < 1 || line > len(lines) {
			return nil, fmt.Errorf("插入行号 %d 越界（文件 %d 行）", line, len(lines))
		}
	}
	for l := range entryLines {
		if l < 1 || l > len(lines) {
			return nil, fmt.Errorf("条目行号 %d 越界（文件 %d 行）", l, len(lines))
		}
	}
	out := make([]string, 0, len(lines)+len(news))
	for i, line := range lines {
		if rows := anchors[i+1]; len(rows) > 0 {
			indent := leadingWS(line)
			for _, r := range rows {
				out = append(out, indent+r.keyQ+":"+strings.Repeat(" ", r.spaces)+r.value+",")
			}
		}
		if entryLines[i+1] {
			line = realignEntryLine(line, targetWidth)
		}
		out = append(out, line)
	}
	return []byte(strings.Join(out, "\n")), nil
}

// spacesAfterColon 值列对齐后的冒号后空格数：indent + key + ":" + spaces + value，
// value 起始列 = indent + width + 1。至少留 1 格。
func spacesAfterColon(width, keyQLen int) int {
	s := width + 1 - (keyQLen + 1)
	if s < 1 {
		s = 1
	}
	return s
}

// entryLineRe 识别条目行：缩进 + 带引号 key + 冒号（值文本里的 ':' 不会被误判，
// 因为 key 段从行首缩进后立刻起引号）。
var entryLineRe = regexp.MustCompile(`^(\s*)("[^"]*"):( *)`)

// realignEntryLine 把条目行的冒号后空格重排到目标值列；除这串空白外一字不动。
func realignEntryLine(line string, targetWidth int) string {
	m := entryLineRe.FindStringSubmatchIndex(line)
	if m == nil {
		return line
	}
	indent, keyQ := line[m[2]:m[3]], line[m[4]:m[5]]
	return indent + keyQ + ":" + strings.Repeat(" ", spacesAfterColon(targetWidth, len(keyQ))) + line[m[7]:]
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

func leadingWS(s string) string {
	for i, r := range s {
		if r != ' ' && r != '\t' {
			return s[:i]
		}
	}
	return s
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
