package newmodule

import (
	"fmt"

	yaml "go.yaml.in/yaml/v3"
)

// editConfigYAML 用 yaml.Node 定位 modules: 映射并追加 <name>: true。
// goccy Node 往返保留 Head/Line 注释；modules 键缺失或重名 → 报错。
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
	// 追加 key: true（key 不设 Style → 纯量裸写，贴原文排版）
	modulesVal.Content = append(modulesVal.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: moduleName, Tag: "!!str"},
		&yaml.Node{Kind: yaml.ScalarNode, Value: "true", Tag: "!!bool"},
	)

	out, err := yaml.Marshal(&root)
	if err != nil {
		return nil, fmt.Errorf("序列化 config.yaml: %w", err)
	}
	return out, nil
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
