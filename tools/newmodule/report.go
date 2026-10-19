package newmodule

import (
	"fmt"
	"sort"
	"strings"
)

// BuildReport 中文收尾报告：新建/修改清单 + 手工项 + 验证结果。
func BuildReport(cs *ChangeSet, o Options, verified bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "✔ 新建模块 %s（实体: %s）\n", o.Name, strings.Join(o.Entities, ", "))

	creates := make([]string, 0, len(cs.Creates))
	for _, c := range cs.Creates {
		creates = append(creates, c.Path)
	}
	sort.Strings(creates)
	b.WriteString("\n新建:\n")
	for _, p := range creates {
		fmt.Fprintf(&b, "  %s\n", p)
	}

	patches := make([]string, 0, len(cs.Patches))
	for _, p := range cs.Patches {
		patches = append(patches, p.Path)
	}
	sort.Strings(patches)
	b.WriteString("\n修改:\n")
	for _, p := range patches {
		fmt.Fprintf(&b, "  %s\n", p)
	}

	b.WriteString("\n手工项:\n")
	fmt.Fprintf(&b, "  - docs/modules.md: 追加 %s 模块条目（端点前缀 /api/admin/%s、/api/app/%s；权限码 %s:<entity>:*）\n",
		o.Name, o.Name, o.Name, o.Name)
	fmt.Fprintf(&b, "  - 菜单名为英文占位（%s 等），可在管理后台「菜单管理」改中文\n", strings.ToUpper(o.Name[:1])+o.Name[1:])
	fmt.Fprintf(&b, "  - 如需独立部署: 参照 modules/member/cmd/main.go 加 micro 入口\n")
	fmt.Fprintf(&b, "  - 如需回滚本模块: 删 modules/%s + 删 apps/web-antd/src/api/%s + 删 apps/web-antd/src/views/%s + git checkout 三个修改文件 + 删 modules/system/migrations/*_%s_menus.*\n", o.Name, o.Name, o.Name, o.Name)

	if o.DryRun {
		b.WriteString("\n（-dry-run：以上仅为计划，未落盘）\n")
	} else if verified {
		b.WriteString("\n验证: go build ./... 通过\n")
	} else {
		b.WriteString("\n验证: 跳过\n")
	}
	return b.String()
}
