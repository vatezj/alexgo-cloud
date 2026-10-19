package newmodule

import (
	"fmt"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

// editConfigYAML 用 yaml.v3 定位 modules: 映射，并在块内末条目后插入 <name>: true。
// 走行定位 + 原字节单行插入：整文件排版与注释一字不动，绝不整文档 Marshal。
// modules 键缺失或重名 → 报错。
func editConfigYAML(src []byte, moduleName string) ([]byte, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(src, &root); err != nil {
		return nil, fmt.Errorf("解析 config.yaml: %w", err)
	}
	// root 是 DocumentNode，真实映射在 Content[0]
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return nil, fmt.Errorf("config.yaml 结构异常")
	}
	doc := root.Content[0]
	if doc.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("config.yaml 顶层不是映射")
	}

	// 定位 modules 键
	modulesVal := findYAMLValue(doc, "modules")
	if modulesVal == nil {
		return nil, fmt.Errorf("config.yaml 未找到 modules: 节点（锚点失效）")
	}
	if modulesVal.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("config.yaml modules: 不是映射")
	}
	// 重名检查
	for i := 0; i+1 < len(modulesVal.Content); i += 2 {
		if modulesVal.Content[i].Value == moduleName {
			return nil, fmt.Errorf("config.yaml modules 已含 %q", moduleName)
		}
	}

	// 行定位 + 文本单行插入：基于原始字节，除插入行外一字不动
	lines := strings.Split(string(src), "\n")
	// 缩进取块内首个 key 的列（Node.Column 为 1-based，前置空格数 = 列-1）
	indent := "  "
	if len(modulesVal.Content) > 0 {
		if col := modulesVal.Content[0].Column; col > 1 {
			indent = strings.Repeat(" ", col-1)
		}
	}
	// 锚行 = modules 块内最后一个条目（含嵌套后代）的最大行号
	anchor := modulesVal.Line
	for _, child := range modulesVal.Content {
		if l := maxYAMLLine(child); l > anchor {
			anchor = l
		}
	}
	if anchor < 1 || anchor > len(lines) {
		return nil, fmt.Errorf("config.yaml modules 行号越界")
	}
	// Node.Line 为 1-based；lines 为 0-based，插入点取锚行之后
	newLine := indent + moduleName + ": true"
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:anchor]...)
	out = append(out, newLine)
	out = append(out, lines[anchor:]...)
	return []byte(strings.Join(out, "\n")), nil
}

// findYAMLValue 在映射节点里找 key 对应的 value 节点。
func findYAMLValue(mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

// maxYAMLLine 递归取节点（含后代）的最大 1-based 行号。
func maxYAMLLine(n *yaml.Node) int {
	if n == nil {
		return 0
	}
	m := n.Line
	for _, c := range n.Content {
		if l := maxYAMLLine(c); l > m {
			m = l
		}
	}
	return m
}
