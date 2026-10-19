// Package newmodule 新建模块脚手架：一条命令生成模块骨架 + 示例实体 CRUD，
// 并自动接线 main.go / config.yaml / permission_routes.go / 菜单种子。
package newmodule

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// identRe 小写下蛇标识符：product / order_item。
var identRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// reservedNames 不允许占用的模块名 / Go 关键字。
var reservedNames = map[string]bool{
	"system": true, "member": true, "infra": true, "oauth2": true,
	"package": true, "import": true, "func": true, "type": true, "var": true,
	"const": true, "return": true, "if": true, "else": true, "for": true,
	"go": true, "map": true, "chan": true, "defer": true, "range": true,
}

type Options struct {
	Root         string
	Name         string
	Entities     []string
	MenuPrefix   string
	DryRun       bool
	SkipFrontend bool
	VerifyBuild  bool
	Now          time.Time
}

// Validate 校验并补默认值（可就地修改 o）。
func (o *Options) Validate() error {
	if o.Root == "" {
		o.Root = "."
	}
	if o.Now.IsZero() {
		o.Now = time.Now().UTC()
	}
	if o.Name == "" {
		return fmt.Errorf("模块名不能为空（-name）")
	}
	if !identRe.MatchString(o.Name) {
		return fmt.Errorf("模块名 %q 必须是小写下蛇标识符（^[a-z][a-z0-9_]*$）", o.Name)
	}
	if reservedNames[o.Name] {
		return fmt.Errorf("模块名 %q 是保留名或 Go 关键字", o.Name)
	}
	if _, err := os.Stat(filepath.Join(o.Root, "modules", o.Name)); err == nil {
		return fmt.Errorf("模块目录 modules/%s 已存在", o.Name)
	}
	if len(o.Entities) == 0 {
		return fmt.Errorf("至少指定一个实体（-entities=product,category）")
	}
	seen := map[string]bool{}
	for _, e := range o.Entities {
		if !identRe.MatchString(e) {
			return fmt.Errorf("实体名 %q 必须是小写下蛇标识符", e)
		}
		if reservedNames[e] {
			return fmt.Errorf("实体名 %q 是保留名或 Go 关键字", e)
		}
		if seen[e] {
			return fmt.Errorf("实体名 %q 重复", e)
		}
		seen[e] = true
	}
	if o.MenuPrefix == "" {
		o.MenuPrefix = o.Name
	}
	// -menu-prefix 校验后全链路零消费（menu/路由/权限码模板一律写死 /{{.Module}}）：
	// 自定义前缀会被静默忽略，属「设置与产物不一致」——最小诚实修 = 显式拒绝，
	// 仅允许留空（默认 = name）或显式等于 -name（终审 I3）。
	if o.MenuPrefix != o.Name {
		return fmt.Errorf("暂不支持自定义菜单前缀（-menu-prefix 必须等于 -name 或留空）")
	}
	// MenuPrefix 与 Name 相等，而 Name 已过 identRe/保留名校验——此处无需重复校验。
	return nil
}

// repoRootDir 从 dir 向上找 go.mod，返回仓库根（测试 helper 的底层实现）。
func repoRootDir(dir string) (string, error) {
	for d := dir; ; {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d, nil
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", fmt.Errorf("从 %s 向上未找到 go.mod", dir)
		}
		d = parent
	}
}
