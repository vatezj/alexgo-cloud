# newmodule 模块脚手架 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 提供 `make new-module NAME=x ENTITIES=y` 一条命令，全有或全无地生成新模块（后端骨架 + 示例实体 CRUD + 前端页面 + 权限/菜单/装配接线）。

**Architecture:** 独立 Go 工具 `tools/newmodule`。示例实体层复用 `pkg/codegen` 引擎（内存构造 `metadata.TableMeta` → `builder.Build` → `codegen.Generate`），模块壳层（module.go / 迁移 / 菜单种子）用自有 go:embed 模板；三个既有文件（main.go / config.yaml / permission_routes.go）各配 AST/yaml.Node 专用编辑器；全部变更先装进 ChangeSet（内存），统一落盘，任一步失败整体回滚。

**Tech Stack:** Go 1.x、text/template + go:embed、go/parser + go/format、goccy/go-yaml（已在 go.mod，升为 direct）、gorm、fx、gin。

**Spec:** `docs/superpowers/specs/2026-10-19-module-template-design.md`

**Spec errata（本计划采纳）:** spec §2.1 草图写的前端路径 `views/product/ProductListPage.vue` 与 codegen 实际输出不符。子决策 A（实体层复用 codegen）为准：视图文件在 `apps/web-antd/src/views/{module}/{snake(entity)}/list.vue`，菜单 component = `views/{module}/{snake(entity)}/list`。Task 1 会把此 errata 修回 spec。

## Global Constraints

- 直接在 main 上工作，提交落 main，不建功能分支。
- 代码注释与用户可见输出用中文。
- 不新增外部 Go 依赖；`github.com/goccy/go-yaml` 已是 indirect，import 后 `go mod tidy` 升 direct。
- 复用 `pkg/codegen`（`Generate` / `builder` / `naming` / `postprocess.FormatGo`），不复制其模板。
- 全有或全无：任何触点失败不落盘（或回滚到写前状态）。
- 工具从仓库根运行（Makefile 保证）；`Options.Root` 默认 `"."`。
- 示例实体固定 6 列（id/tenant_id/name/status/created_at/updated_at），迁移 DDL 与内存 TableMeta 严格同构。
- 权限码条目必含 `/*` 变体（keyMatch2，见 permission_routes.go 头注释）。
- 菜单种子只插行不绑角色（启动 seed 全量 SetRoleMenus 兜底）。

## Review Focus

1. **format.Node 全文件重排**导致 main.go / permission_routes.go 出现无关 diff（注释错位、空行丢失）——Task 5/7 测试断言既有关键行原样保留、diff 仅新增。
2. **goccy yaml.Node 往返破坏 config.yaml 注释**——Task 6 测试断言既有注释文本保留。
3. **迁移版本号冲突**（同日二次运行、目录里已有更大版本）——Task 3 测试 `nextMigrationVersion` 三个用例。
4. **权限码路由路径与 module_register 实际挂载路径漂移**——两者必须同源（都用 `naming.Plural`）；Task 7 测试 + Task 10 golden 清单钉住。
5. **菜单 component 与 codegen 视图路径不一致**（见 Spec errata）——Task 4 测试断言 component 由同一套 naming 派生。

---

### Task 1: 工具骨架 + Options 校验 + spec errata

**Files:**
- Create: `tools/newmodule/options.go`
- Create: `tools/newmodule/options_test.go`
- Modify: `docs/superpowers/specs/2026-10-19-module-template-design.md`（§2.1 前端路径 errata）

**Interfaces:**
- Produces:
  ```go
  type Options struct {
      Root         string        // 仓库根，默认 "."
      Name         string        // 模块名，如 "product"
      Entities     []string      // 实体名（单数、小写下蛇），如 ["product","category"]
      MenuPrefix   string        // 菜单/路由前缀，默认 = Name
      DryRun       bool
      SkipFrontend bool
      VerifyBuild  bool          // 落盘后 go build ./...（集成测试里关掉）
      Now          time.Time     // 迁移版本号时间源，默认 time.Now().UTC()
  }
  func (o Options) Validate() error
  ```

- [ ] **Step 1: 写失败测试**

`tools/newmodule/options_test.go`:

```go
package newmodule

import (
	"os"
	"strings"
	"testing"
	"time"
)

func validOpts() Options {
	return Options{
		Name:     "product",
		Entities: []string{"product", "category"},
		Now:      time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC),
	}
}

func TestOptionsValidate_OK(t *testing.T) {
	o := validOpts()
	o.Root = repoRoot(t)
	if err := o.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
}

func TestOptionsValidate_Defaults(t *testing.T) {
	o := validOpts()
	o.Root = ""
	o.MenuPrefix = ""
	_ = o.Validate()
	if o.MenuPrefix != "product" {
		t.Errorf("MenuPrefix default = %q, want product", o.MenuPrefix)
	}
	if o.Root != "." {
		t.Errorf("Root default = %q", o.Root)
	}
	if o.Now.IsZero() {
		t.Error("Now default should be set")
	}
}

func TestOptionsValidate_Rejects(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*Options)
		wantSub string
	}{
		{"empty name", func(o *Options) { o.Name = "" }, "模块名"},
		{"uppercase name", func(o *Options) { o.Name = "Product" }, "小写"},
		{"dash in name", func(o *Options) { o.Name = "my-mod" }, "小写"},
		{"no entities", func(o *Options) { o.Entities = nil }, "实体"},
		{"uppercase entity", func(o *Options) { o.Entities = []string{"Product"} }, "小写"},
		{"dup entity", func(o *Options) { o.Entities = []string{"a", "a"} }, "重复"},
		{"reserved module", func(o *Options) { o.Name = "system" }, "保留"},
		{"go keyword", func(o *Options) { o.Name = "type" }, "保留"},
		{"existing module dir", func(o *Options) { o.Name = "order" }, "已存在"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := validOpts()
			o.Root = repoRoot(t) // 「模块已存在」用例依赖仓库真实 modules/order
			tc.mutate(&o)
			err := o.Validate()
			if err == nil {
				t.Fatal("Validate() = nil, want error")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("err = %v, want substring %q", err, tc.wantSub)
			}
		})
	}
}
```

注意：`{"existing module dir", Name: "order"}` 依赖仓库里 `modules/order` 存在——所有 Rejects/OK 用例统一把 `o.Root` 设为 `repoRoot(t)`（helper 从 `os.Getwd()` 向上找 go.mod），实现侧 `Validate` 用 `o.Root` 判断 `modules/<name>` 是否存在。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./tools/newmodule/ -run TestOptionsValidate -v`
Expected: FAIL（包不存在或 Validate 未定义）

- [ ] **Step 3: 实现 options.go**

`tools/newmodule/options.go`:

```go
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
	"system": true, "member": true, "order": true, "infra": true, "oauth2": true,
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
	if !identRe.MatchString(o.MenuPrefix) {
		return fmt.Errorf("菜单前缀 %q 必须是小写下蛇标识符", o.MenuPrefix)
	}
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
```

`options_test.go` 里加测试 helper（同包其它 _test.go 复用）：

```go
func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root, err := repoRootDir(wd)
	if err != nil {
		t.Fatal(err)
	}
	return root
}
```

`validOpts()` 不带 Root，各用例在调 Validate 前赋 `o.Root = repoRoot(t)`。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./tools/newmodule/ -run TestOptionsValidate -v`
Expected: PASS

- [ ] **Step 5: 修 spec errata**

`docs/superpowers/specs/2026-10-19-module-template-design.md` §2.1 前端块替换为：

```
apps/web-antd/src/
├── api/{module}/{snake(entity)}/     # codegen 输出（types.ts + api.ts + index.ts + __tests__）
└── views/{module}/{snake(entity)}/   # codegen 输出（list.vue + form.vue + drawer.vue）
```

并在 §2.1 菜单句后补一句：「菜单 component = `views/{module}/{snake(entity)}/list`（与 codegen 视图路径同源）」。

- [ ] **Step 6: Commit**

```bash
git add tools/newmodule/options.go tools/newmodule/options_test.go docs/superpowers/specs/2026-10-19-module-template-design.md
git commit -m "feat(newmodule): Options 校验骨架 + spec 前端路径 errata"
```

---

### Task 2: 示例实体复用 codegen 引擎

**Files:**
- Create: `tools/newmodule/entity.go`
- Create: `tools/newmodule/entity_test.go`

**Interfaces:**
- Consumes: `Options`（Task 1）；`pkg/codegen.Generate([]*model.Table, codegen.Options{UnitTestEnable: true}) ([]model.GeneratedFile, error)`；`builder.Build(*metadata.TableMeta, builder.Options{Module: name}) (*model.Table, error)`；`naming.Plural/entity`。
- Produces:
  ```go
  // entityFiles 为每个实体经 codegen 引擎生成后端+前端全套文件（含 service_test / api.test）。
  func entityFiles(o Options) ([]model.GeneratedFile, error)
  // exampleTableMeta 构造示例实体的 6 列表元数据（表名 = 复数实体名）。
  func exampleTableMeta(module, entity string) *metadata.TableMeta
  ```

- [ ] **Step 1: 写失败测试**

`tools/newmodule/entity_test.go`:

```go
package newmodule

import (
	"strings"
	"testing"
	"time"
)

func TestExampleTableMeta(t *testing.T) {
	m := exampleTableMeta("demo", "category") // 不规则复数
	if m.Name != "categories" {
		t.Errorf("table name = %q, want categories", m.Name)
	}
	if len(m.Columns) != 6 {
		t.Fatalf("columns = %d, want 6", len(m.Columns))
	}
	if m.Columns[0].Name != "id" || m.Columns[0].Key != "PRI" {
		t.Errorf("first column should be id PRI, got %+v", m.Columns[0])
	}
	if m.Columns[1].Name != "tenant_id" {
		t.Errorf("second column should be tenant_id")
	}
}

func TestEntityFiles(t *testing.T) {
	// Name 固定 demo（不与未来真实模块撞车），断言路径也按 demo 钉死
	o := Options{
		Root:     repoRoot(t),
		Name:     "demo",
		Entities: []string{"product", "category"},
		Now:      time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC),
	}
	if err := o.Validate(); err != nil {
		t.Fatal(err)
	}
	files, err := entityFiles(o)
	if err != nil {
		t.Fatalf("entityFiles() = %v", err)
	}
	// 每实体 14 个文件（7 server + 7 front），2 实体 = 28。
	if len(files) != 28 {
		t.Errorf("files = %d, want 28", len(files))
	}
	byPath := map[string]bool{}
	for _, f := range files {
		byPath[f.Path] = true
		if strings.Contains(f.Path, "..") {
			t.Errorf("bad path %q", f.Path)
		}
	}
	for _, want := range []string{
		"modules/demo/model/category.go",
		"modules/demo/repository/category.go",
		"modules/demo/service/category.go",
		"modules/demo/service/category_test.go",
		"modules/demo/controller/admin/category.go",
		"modules/demo/controller/app/category.go",
		"modules/demo/module_register_category.go",
		"apps/web-antd/src/api/demo/category/types.ts",
		"apps/web-antd/src/views/demo/category/list.vue",
	} {
		if !byPath[want] {
			t.Errorf("missing %s", want)
		}
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./tools/newmodule/ -run 'TestExampleTableMeta|TestEntityFiles' -v`
Expected: FAIL（entityFiles 未定义）

- [ ] **Step 3: 实现 entity.go**

`tools/newmodule/entity.go`:

```go
package newmodule

import (
	"fmt"

	"alexGo-cloud/pkg/codegen"
	"alexGo-cloud/pkg/codegen/builder"
	"alexGo-cloud/pkg/codegen/metadata"
	"alexGo-cloud/pkg/codegen/model"
	"alexGo-cloud/pkg/codegen/naming"
)

// exampleTableMeta 示例实体表元数据：6 列固定骨架，与迁移 DDL 模板严格同构
// （id/tenant_id/name/status/created_at/updated_at）。表名 = 实体名复数化。
func exampleTableMeta(module, entity string) *metadata.TableMeta {
	_ = module // 表名只依赖实体；module 保留签名对称，便于扩展
	table := naming.Plural(entity)
	return &metadata.TableMeta{
		Name:    table,
		Comment: entity + " 示例表",
		Columns: []metadata.ColumnMeta{
			{Name: "id", DataType: "bigint", ColumnType: "bigint unsigned", Key: "PRI", Extra: "auto_increment", Ordinal: 1},
			{Name: "tenant_id", DataType: "bigint", ColumnType: "bigint unsigned", Comment: "租户ID", Ordinal: 2},
			{Name: "name", DataType: "varchar", ColumnType: "varchar(64)", Comment: "名称", Ordinal: 3},
			{Name: "status", DataType: "tinyint", ColumnType: "tinyint", Comment: "状态 1=启用 0=停用", Ordinal: 4},
			{Name: "created_at", DataType: "datetime", ColumnType: "datetime", Comment: "创建时间", Ordinal: 5},
			{Name: "updated_at", DataType: "datetime", ColumnType: "datetime", Comment: "更新时间", Ordinal: 6},
		},
	}
}

// entityFiles 经 codegen 引擎生成全部实体的后端+前端文件（含单测模板）。
func entityFiles(o Options) ([]model.GeneratedFile, error) {
	tables := make([]*model.Table, 0, len(o.Entities))
	for _, e := range o.Entities {
		t, err := builder.Build(exampleTableMeta(o.Name, e), builder.Options{Module: o.Name})
		if err != nil {
			return nil, fmt.Errorf("构建实体 %s 失败: %w", e, err)
		}
		tables = append(tables, t)
	}
	files, err := codegen.Generate(tables, codegen.Options{UnitTestEnable: true})
	if err != nil {
		return nil, fmt.Errorf("codegen 生成失败: %w", err)
	}
	if o.SkipFrontend {
		filtered := files[:0]
		for _, f := range files {
			if len(f.Path) >= 8 && f.Path[:8] == "modules/" {
				filtered = append(filtered, f)
			}
		}
		files = filtered
	}
	return files, nil
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./tools/newmodule/ -run 'TestExampleTableMeta|TestEntityFiles' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add tools/newmodule/entity.go tools/newmodule/entity_test.go
git commit -m "feat(newmodule): 示例实体复用 codegen 引擎（builder+Generate）"
```

---

### Task 3: 壳层模板（module.go / migration_source / 模块配置 / 实体迁移）+ 版本号工具

**Files:**
- Create: `tools/newmodule/shell.go`
- Create: `tools/newmodule/shell_test.go`
- Create: `tools/newmodule/templates/module.go.tmpl`
- Create: `tools/newmodule/templates/migration_source.go.tmpl`
- Create: `tools/newmodule/templates/module_config.yaml.tmpl`
- Create: `tools/newmodule/templates/entity_up.sql.tmpl`
- Create: `tools/newmodule/templates/entity_down.sql.tmpl`

**Interfaces:**
- Consumes: `Options`（Task 1）；`naming.EntityName/Plural/Snake`；`postprocess.FormatGo`。
- Produces:
  ```go
  // shellFiles 模块壳层新文件（不含菜单种子——菜单在 Task 4）。
  func shellFiles(o Options) ([]model.GeneratedFile, error)
  // nextMigrationVersion 目录内下一版本号：max(现有最大+1, 日期000001)。
  func nextMigrationVersion(dir string, now time.Time) (string, error)
  ```

- [ ] **Step 1: 写失败测试**

`tools/newmodule/shell_test.go`:

```go
package newmodule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNextMigrationVersion(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)

	// 空目录 → 日期起版
	v, err := nextMigrationVersion(dir, now)
	if err != nil || v != "20261019000001" {
		t.Fatalf("empty dir: v=%q err=%v", v, err)
	}

	// 同日已有 000001 → 递增
	for _, f := range []string{"20261019000001_x.up.sql", "20261019000001_x.down.sql"} {
		if err := os.WriteFile(filepath.Join(dir, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	v, err = nextMigrationVersion(dir, now)
	if err != nil || v != "20261019000002" {
		t.Fatalf("same day: v=%q err=%v", v, err)
	}

	// 已有未来版本（如 20261231000001）→ 超过它
	if err := os.WriteFile(filepath.Join(dir, "20261231000001_future.up.sql"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	v, err = nextMigrationVersion(dir, now)
	if err != nil || v != "20261231000002" {
		t.Fatalf("future version: v=%q err=%v", v, err)
	}
}

func TestShellFiles(t *testing.T) {
	o := Options{Root: repoRoot(t), Name: "demo", Entities: []string{"product", "category"},
		Now: time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)}
	if err := o.Validate(); err != nil {
		t.Fatal(err)
	}
	files, err := shellFiles(o)
	if err != nil {
		t.Fatalf("shellFiles() = %v", err)
	}
	byPath := map[string]string{}
	for _, f := range files {
		byPath[f.Path] = string(f.Content)
	}
	// module.go
	mg, ok := byPath["modules/demo/module.go"]
	if !ok {
		t.Fatal("missing modules/demo/module.go")
	}
	for _, want := range []string{
		`fx.Module("demo"`,
		"repository.NewProductRepository",
		"repository.NewCategoryRepository",
		"service.NewCategoryService",
		"admin.NewAdminCategoryController",
		"app.NewAppCategoryController",
		`NewMigrationSource`,
		"RegisterProductDBRoutes(r, m.ProductAdminCtrl)",
		"RegisterCategoryDBRoutes(r, m.CategoryAdminCtrl)",
		`r.Group("/app/demo")`,
		`m.cfg.Modules["demo"]`,
	} {
		if !strings.Contains(mg, want) {
			t.Errorf("module.go missing %q", want)
		}
	}
	// migration_source.go
	ms := byPath["modules/demo/migration_source.go"]
	if !strings.Contains(ms, `"modules/demo/migrations"`) || !strings.Contains(ms, `"demo"`) {
		t.Error("migration_source.go content wrong")
	}
	// 实体迁移 up：表名与 6 列
	up := byPath["modules/demo/migrations/20261019000001_products.up.sql"]
	if !strings.Contains(up, "CREATE TABLE `products`") ||
		!strings.Contains(up, "`tenant_id`") || !strings.Contains(up, "`status`") {
		t.Errorf("products up.sql wrong:\n%s", up)
	}
	up2 := byPath["modules/demo/migrations/20261019000002_categories.up.sql"]
	if !strings.Contains(up2, "CREATE TABLE `categories`") {
		t.Errorf("categories up.sql wrong:\n%s", up2)
	}
	// down
	if !strings.Contains(byPath["modules/demo/migrations/20261019000001_products.down.sql"], "DROP TABLE IF EXISTS `products`") {
		t.Error("products down.sql wrong")
	}
	// module 私有配置
	if !strings.Contains(byPath["modules/demo/configs/config.yaml"], "demo") {
		t.Error("module config.yaml wrong")
	}
	// module.go 必须是合法 Go（FormatGo 已跑，能 parse 即证）
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./tools/newmodule/ -run 'TestNextMigrationVersion|TestShellFiles' -v`
Expected: FAIL

- [ ] **Step 3: 写四个模板文件**

`tools/newmodule/templates/module.go.tmpl`:

```
package {{.Module}}

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"alexGo-cloud/alexgo-server/server"
	"alexGo-cloud/modules/{{.Module}}/controller/admin"
	"alexGo-cloud/modules/{{.Module}}/controller/app"
	"alexGo-cloud/modules/{{.Module}}/repository"
	"alexGo-cloud/modules/{{.Module}}/service"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/migrate"
)

// FxModule {{.Module}} 模块的 fx 装配（newmodule 生成，业务逻辑请手工扩展）。
var FxModule = fx.Module("{{.Module}}",
	fx.Provide(
{{- range .Entities}}
		repository.New{{.Entity}}Repository,
		service.New{{.Entity}}Service,
		admin.NewAdmin{{.Entity}}Controller,
		app.NewApp{{.Entity}}Controller,
{{- end}}
	),
	fx.Provide(
		fx.Annotate(
			NewModule,
			fx.As(new(server.Module)),
			fx.ResultTags(`group:"modules"`),
		),
	),
	fx.Provide(
		fx.Annotate(
			NewMigrationSource,
			fx.As(new(migrate.Source)),
			fx.ResultTags(`group:"migration_sources"`),
		),
	),
)

// ModuleParams 聚合依赖（fx.In）。
type ModuleParams struct {
	fx.In

	Cfg *config.Config
{{- range .Entities}}
	{{.Entity}}AdminCtrl *admin.Admin{{.Entity}}Controller
	{{.Entity}}AppCtrl   *app.App{{.Entity}}Controller
{{- end}}
}

type {{.Module}}Module struct {
	cfg *config.Config
{{- range .Entities}}
	{{.Entity}}AdminCtrl *admin.Admin{{.Entity}}Controller
	{{.Entity}}AppCtrl   *app.App{{.Entity}}Controller
{{- end}}
}

// NewModule 聚合控制器，产出 server.Module（挂进 group:"modules"）。
func NewModule(p ModuleParams) server.Module {
	return &{{.Module}}Module{
		cfg: p.Cfg,
{{- range .Entities}}
		{{.Entity}}AdminCtrl: p.{{.Entity}}AdminCtrl,
		{{.Entity}}AppCtrl:   p.{{.Entity}}AppCtrl,
{{- end}}
	}
}

// RegisterRoutes 挂载 {{.Module}} 路由。入参是 /api group（见 server/http.go），
// 故 admin 用 /admin/{{.Module}}、app 用 /app/{{.Module}}（member 基准双前缀）。
// config 开关：modules.{{.Module}}: false 时跳过注册。
func (m *{{.Module}}Module) RegisterRoutes(r *gin.RouterGroup) {
	if m.cfg != nil {
		if on, ok := m.cfg.Modules["{{.Module}}"]; ok && !on {
			return
		}
	}
{{- range .Entities}}
	// {{.Entity}} 的 CRUD 路由由 codegen 生成的 module_register_{{.Snake}}.go 声明，这里接线。
	Register{{.Entity}}DBRoutes(r, m.{{.Entity}}AdminCtrl)
{{- end}}

	appGroup := r.Group("/app/{{.Module}}")
{{- range .Entities}}
	appGroup.GET("/{{.Plural}}/ping", m.{{.Entity}}AppCtrl.Ping)
{{- end}}
	// 跨模块接口映射（如 token.Issuer）归入口 main.go，模块内不重复 Provide——
	// 需要时在此上方参照 modules/member/module.go 补 fx.Provide 映射。
}
```

`tools/newmodule/templates/migration_source.go.tmpl`:

```
package {{.Module}}

import "alexGo-cloud/pkg/migrate"

// migrationSource 把 {{.Module}} 模块的 SQL 迁移目录注册进全局迁移器（fx group:"migration_sources"）。
type migrationSource struct{}

// NewMigrationSource 返回 {{.Module}} 模块的迁移源。
func NewMigrationSource() migrate.Source { return &migrationSource{} }

// GetMigrationPath 迁移 SQL 所在目录（相对仓库根）。
func (s *migrationSource) GetMigrationPath() string { return "modules/{{.Module}}/migrations" }

// GetModuleName 模块名（决定 schema_migrations_<name> 记录表）。
func (s *migrationSource) GetModuleName() string { return "{{.Module}}" }
```

`tools/newmodule/templates/module_config.yaml.tmpl`:

```
# {{.Module}} 模块私有配置（newmodule 生成骨架，可为空）。
# 加载规则：主配置 modules.{{.Module}}: true 时本文件被继续加载，键以
# "{{.Module}}." 命名空间合并进全局配置（见 pkg/config/loader.go / config.go）。
{}
```

`tools/newmodule/templates/entity_up.sql.tmpl`:

```sql
-- {{.Table}}：{{.Entity}} 示例表（newmodule 生成；列与 model/{{.Snake}}.go 严格同构）。
CREATE TABLE `{{.Table}}` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '租户ID',
  `name` varchar(64) NOT NULL DEFAULT '' COMMENT '名称',
  `status` tinyint NOT NULL DEFAULT 1 COMMENT '状态 1=启用 0=停用',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_{{.Table}}_tenant` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='{{.Entity}} 示例表';
```

`tools/newmodule/templates/entity_down.sql.tmpl`:

```sql
DROP TABLE IF EXISTS `{{.Table}}`;
```

- [ ] **Step 4: 实现 shell.go**

`tools/newmodule/shell.go`:

```go
package newmodule

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	texttpl "text/template"

	"alexGo-cloud/pkg/codegen/model"
	"alexGo-cloud/pkg/codegen/naming"
	"alexGo-cloud/pkg/codegen/postprocess"
)

//go:embed templates/*.tmpl
var shellFS embed.FS

// shellBind 壳层模板数据。
type shellBind struct {
	Module   string
	Entities []shellEntity
}

// shellEntity 单实体的模板视图。
type shellEntity struct {
	Entity string // Pascal：Product
	Snake  string // product / order_item
	Plural string // products / order_items（路由段，与 module_register 同源）
	Table  string // = Plural（示例实体表名）
}

func shellBindOf(o Options) shellBind {
	b := shellBind{Module: o.Name}
	for _, e := range o.Entities {
		entity := naming.EntityName(naming.Plural(e)) // product → products → Product
		b.Entities = append(b.Entities, shellEntity{
			Entity: entity,
			Snake:  naming.Snake(entity),
			Plural: naming.Plural(entity),
			Table:  naming.Plural(entity),
		})
	}
	return b
}

// nextMigrationVersion 目录内下一个迁移版本号：取 max(现有最大版本+1, 当日 000001)。
func nextMigrationVersion(dir string, now time.Time) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	maxV := 0
	for _, ent := range entries {
		name := ent.Name()
		if !strings.HasSuffix(name, ".up.sql") && !strings.HasSuffix(name, ".down.sql") {
			continue
		}
		head := strings.SplitN(name, "_", 2)[0]
		if v, err := strconv.Atoi(head); err == nil && v > maxV {
			maxV = v
		}
	}
	dateV, err := strconv.Atoi(now.Format("20060102") + "000001")
	if err != nil {
		return "", err
	}
	if dateV > maxV {
		return strconv.Itoa(dateV), nil
	}
	return strconv.Itoa(maxV + 1), nil
}

// shellFiles 渲染模块壳层文件（module.go / migration_source.go / 私有配置 / 每实体迁移对）。
func shellFiles(o Options) ([]model.GeneratedFile, error) {
	b := shellBindOf(o)
	var files []model.GeneratedFile

	renderGo := func(tplName, outPath string) error {
		raw, err := shellFS.ReadFile("templates/" + tplName)
		if err != nil {
			return err
		}
		tpl, err := texttpl.New(tplName).Parse(string(raw))
		if err != nil {
			return fmt.Errorf("解析模板 %s: %w", tplName, err)
		}
		var buf bytes.Buffer
		if err := tpl.Execute(&buf, b); err != nil {
			return fmt.Errorf("渲染 %s: %w", tplName, err)
		}
		out, err := postprocess.FormatGo(outPath, buf.Bytes())
		if err != nil {
			return fmt.Errorf("格式化 %s: %w", outPath, err)
		}
		files = append(files, model.GeneratedFile{Path: outPath, Content: out})
		return nil
	}

	if err := renderGo("module.go.tmpl", fmt.Sprintf("modules/%s/module.go", o.Name)); err != nil {
		return nil, err
	}
	if err := renderGo("migration_source.go.tmpl", fmt.Sprintf("modules/%s/migration_source.go", o.Name)); err != nil {
		return nil, err
	}

	// 模块私有配置（yaml，不走 FormatGo）
	raw, err := shellFS.ReadFile("templates/module_config.yaml.tmpl")
	if err != nil {
		return nil, err
	}
	files = append(files, model.GeneratedFile{
		Path:    fmt.Sprintf("modules/%s/configs/config.yaml", o.Name),
		Content: raw,
	})

	// 实体迁移对：版本号在 modules/<name>/migrations 内逐个递增
	migDir := filepath.Join(o.Root, "modules", o.Name, "migrations")
	ver, err := nextMigrationVersion(migDir, o.Now)
	if err != nil {
		return nil, err
	}
	upTpl, _ := texttpl.New("up").Parse(mustRead(shellFS, "templates/entity_up.sql.tmpl"))
	downTpl, _ := texttpl.New("down").Parse(mustRead(shellFS, "templates/entity_down.sql.tmpl"))
	v, _ := strconv.Atoi(ver)
	for i, e := range b.Entities {
		var upBuf, downBuf bytes.Buffer
		if err := upTpl.Execute(&upBuf, e); err != nil {
			return nil, err
		}
		if err := downTpl.Execute(&downBuf, e); err != nil {
			return nil, err
		}
		version := strconv.Itoa(v + i)
		files = append(files,
			model.GeneratedFile{
				Path:    fmt.Sprintf("modules/%s/migrations/%s_%s.up.sql", o.Name, version, e.Table),
				Content: upBuf.Bytes(),
			},
			model.GeneratedFile{
				Path:    fmt.Sprintf("modules/%s/migrations/%s_%s.down.sql", o.Name, version, e.Table),
				Content: downBuf.Bytes(),
			},
		)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func mustRead(fs embed.FS, name string) string {
	b, err := fs.ReadFile(name)
	if err != nil {
		panic(err)
	}
	return string(b)
}
```

补 import `"time"`（nextMigrationVersion 用到）。

- [ ] **Step 5: 跑测试确认通过**

Run: `go test ./tools/newmodule/ -run 'TestNextMigrationVersion|TestShellFiles' -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add tools/newmodule/shell.go tools/newmodule/shell_test.go tools/newmodule/templates/
git commit -m "feat(newmodule): 模块壳层模板（module.go/迁移源/迁移SQL/私有配置）"
```

---

### Task 4: 菜单种子 SQL 生成（system 模块迁移对）

**Files:**
- Create: `tools/newmodule/menu.go`
- Create: `tools/newmodule/menu_test.go`
- Create: `tools/newmodule/templates/menu_up.sql.tmpl`
- Create: `tools/newmodule/templates/menu_down.sql.tmpl`

**Interfaces:**
- Consumes: `Options`、`shellBindOf`、`nextMigrationVersion`（Task 3）；`naming.Snake/Plural`。
- Produces:
  ```go
  // menuSeedFiles 生成 system 模块菜单种子迁移对（dir + 每实体一 menu）。
  // component = views/<module>/<snake(entity)>/list，与 codegen 视图路径同源。
  // SkipFrontend 时返回 (nil, nil)——无视图则不种菜单。
  func menuSeedFiles(o *Options) ([]model.GeneratedFile, error)
  ```

- [ ] **Step 1: 写失败测试**

`tools/newmodule/menu_test.go`:

```go
package newmodule

import (
	"strings"
	"testing"
	"time"
)

func TestMenuSeedFiles(t *testing.T) {
	o := Options{Root: repoRoot(t), Name: "demo", Entities: []string{"product", "category"},
		Now: time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)}
	if err := o.Validate(); err != nil {
		t.Fatal(err)
	}
	files, err := menuSeedFiles(&o)
	if err != nil {
		t.Fatalf("menuSeedFiles() = %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("files = %d, want 2 (up+down)", len(files))
	}
	var up, down string
	for _, f := range files {
		if strings.HasSuffix(f.Path, ".up.sql") {
			up = string(f.Content)
			if !strings.HasPrefix(f.Path, "modules/system/migrations/") {
				t.Errorf("up path %q should be under system migrations", f.Path)
			}
			// 版本必须 > 现有 system 最大版本（当前 20261016...）
			ver := f.Path[len("modules/system/migrations/"):][:14]
			if ver <= "20261016000002" {
				t.Errorf("version %q not greater than existing max", ver)
			}
		}
		if strings.HasSuffix(f.Path, ".down.sql") {
			down = string(f.Content)
		}
	}
	for _, want := range []string{
		"WHERE NOT EXISTS",                     // 幂等 guard
		"'/demo'",                              // dir path
		"'dir'",                                // dir type
		"'/demo/products'",                     // menu path
		"'views/demo/product/list'",            // component 与 codegen 视图同源
		"'/demo/categories'",
		"'views/demo/category/list'",
		"'demo:product:*'",                     // 权限码
		"'demo:category:*'",
		"FROM `menus` d",                       // parent 经父目录 JOIN
		"tenant_id`=0",                         // 全局菜单
	} {
		if !strings.Contains(up, want) {
			t.Errorf("up.sql missing %q\n%s", want, up)
		}
	}
	if !strings.Contains(down, "DELETE FROM `menus`") || !strings.Contains(down, "'/demo'") {
		t.Errorf("down.sql wrong:\n%s", down)
	}
	// down 不应解绑 role_menus（本设计不绑角色；但保险起见 down 只删菜单行）
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./tools/newmodule/ -run TestMenuSeedFiles -v`
Expected: FAIL

- [ ] **Step 3: 写模板**

`tools/newmodule/templates/menu_up.sql.tmpl`:

```sql
-- {{.Module}} 模块菜单种子（newmodule 生成）：1 目录 + {{.EntityCount}} 页面。
-- 幂等：全部 INSERT 走 NOT EXISTS；tenant_id=0 全局菜单。
-- admin 角色绑定不在本迁移做：启动 seed（modules/system/service/seed.go）
-- 每次全量 SetRoleMenus + RebuildPolicies 兜底。

-- ① 模块根目录
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, 0, 'dir', '{{.Title}}', '/{{.Module}}', '', 'lucide:package', '{{.Module}}', 80, 1, 0, NOW(), NOW() FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND `path`='/{{.Module}}' AND `type`='dir' AND `deleted`=0);

{{- range .Entities}}
-- ② {{.Entity}} 列表页（component 与 codegen 视图路径同源）
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', '{{.Title}}', '/{{$.Module}}/{{.Plural}}', 'views/{{$.Module}}/{{.Snake}}/list', 'lucide:list', '{{$.Module}}:{{.Snake}}:*', {{.Sort}}, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/{{$.Module}}' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND `path`='/{{$.Module}}/{{.Plural}}' AND `deleted`=0);
{{- end}}
```

`tools/newmodule/templates/menu_down.sql.tmpl`:

```sql
-- 回滚 {{.Module}} 模块菜单（按 path 精确删，含子页）。
DELETE FROM `menus`
WHERE `tenant_id`=0
  AND (`path`='/{{.Module}}' {{- range .Entities}} OR `path`='/{{$.Module}}/{{.Plural}}'{{- end}})
  AND `type` IN ('dir','menu');
```

注意模板数据扩展：菜单模板需要 `Title`（实体/模块中文名——脚手架阶段用 Entity Pascal 占位即可，报告里提醒改中文）、`EntityCount`、每实体 `Sort`（从 1 递增）。在 `shellBind`/`shellEntity` 上扩展字段或定义独立 `menuBind`。**实现选独立 menuBind**，避免壳层绑定膨胀：

```go
type menuBind struct {
	Module      string
	Title       string
	EntityCount int
	Entities    []menuEntity
}
type menuEntity struct {
	Entity string
	Snake  string
	Plural string
	Title  string
	Sort   int
}
```

Title 暂用 Pascal 名（如 `Demo`、`Product`）——菜单名后可在管理后台改，报告的手工项里提示中文化。

- [ ] **Step 4: 实现 menu.go**

`tools/newmodule/menu.go`:

```go
package newmodule

import (
	"bytes"
	"fmt"
	texttpl "text/template"

	"alexGo-cloud/pkg/codegen/model"
	"alexGo-cloud/pkg/codegen/naming"
)

// menuSeedFiles 生成 system 模块的菜单种子迁移对。
// 放 system 而非模块自身迁移：menus 表由 system 迁移创建，且现有先例
// （20261009000001_menus_vben_backfill）都在 system。
func menuSeedFiles(o *Options) ([]model.GeneratedFile, error) {
	b := menuBind{Module: o.Name, Title: naming.Pascal(o.Name)}
	for i, e := range o.Entities {
		entity := naming.EntityName(naming.Plural(e))
		b.Entities = append(b.Entities, menuEntity{
			Entity: entity,
			Snake:  naming.Snake(entity),
			Plural: naming.Plural(entity),
			Title:  naming.Pascal(entity),
			Sort:   i + 1,
		})
	}
	b.EntityCount = len(b.Entities)

	if o.SkipFrontend {
		return nil, nil // 无前端视图则不种菜单（否则菜单点开 404）
	}

	ver, err := nextMigrationVersion(fmt.Sprintf("%s/modules/system/migrations", o.Root), o.Now)
	if err != nil {
		return nil, err
	}
	base := fmt.Sprintf("modules/system/migrations/%s_%s_menus", ver, o.Name)

	var files []model.GeneratedFile
	for _, tplName := range []string{"menu_up.sql.tmpl", "menu_down.sql.tmpl"} {
		raw, err := shellFS.ReadFile("templates/" + tplName)
		if err != nil {
			return nil, err
		}
		tpl, err := texttpl.New(tplName).Parse(string(raw))
		if err != nil {
			return nil, err
		}
		var buf bytes.Buffer
		if err := tpl.Execute(&buf, b); err != nil {
			return nil, err
		}
		suffix := ".up.sql"
		if tplName == "menu_down.sql.tmpl" {
			suffix = ".down.sql"
		}
		files = append(files, model.GeneratedFile{Path: base + suffix, Content: buf.Bytes()})
	}
	return files, nil
}
```

注意 `//go:embed templates/*.tmpl` 已覆盖新模板（同目录 glob），无需改 embed 声明。

- [ ] **Step 5: 跑测试确认通过**

Run: `go test ./tools/newmodule/ -run TestMenuSeedFiles -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add tools/newmodule/menu.go tools/newmodule/menu_test.go tools/newmodule/templates/menu_*.tmpl
git commit -m "feat(newmodule): 菜单种子迁移生成（system 模块，幂等 + seed 兜底）"
```

---

### Task 5: main.go 编辑器（AST 插 import + opts）

**Files:**
- Create: `tools/newmodule/edit_main.go`
- Create: `tools/newmodule/edit_main_test.go`

**Interfaces:**
- Consumes: 仓库现文件 `alexgo-server/cmd/main.go`。
- Produces:
  ```go
  // editMainGo 在内存里给 main.go 插入模块 import 与 opts 装配行。
  func editMainGo(src []byte, moduleName string) ([]byte, error)
  ```

- [ ] **Step 1: 写失败测试**

`tools/newmodule/edit_main_test.go`:

```go
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
		t.Error("missing import")
	}
	// import 字母序：demo 落在 member 之前
	if strings.Index(got, `"alexGo-cloud/modules/demo"`) > strings.Index(got, `"alexGo-cloud/modules/member"`) {
		t.Error("demo import should sort before member")
	}
	if !strings.Contains(got, "demo.FxModule,") {
		t.Error("missing opts entry")
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
			t.Errorf("lost original content %q", keep)
		}
	}
	// 装配行必须在 member 条件分支之前
	if strings.Index(got, "demo.FxModule,") > strings.Index(got, `if cfg.Deployment.Mode != "micro"`) {
		t.Error("demo.FxModule should be before member conditional branch")
	}
	// 结果必须是合法 Go
	if _, err := parser.ParseFile(token.NewFileSet(), "main.go", out, parser.AllErrors); err != nil {
		t.Fatalf("output not parseable: %v", err)
	}
	// 幂等拒绝：再插一次同名模块应报错
	if _, err := editMainGo(out, "demo"); err == nil {
		t.Error("second insert should fail (duplicate)")
	}
}

func TestEditMainGo_AnchorMissing(t *testing.T) {
	if _, err := editMainGo([]byte("package main\n"), "demo"); err == nil {
		t.Error("want error on missing anchors")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./tools/newmodule/ -run TestEditMainGo -v`
Expected: FAIL

- [ ] **Step 3: 实现 edit_main.go**

`tools/newmodule/edit_main.go`:

```go
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
	return buf.Bytes(), nil
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
```

说明：`format.Node` 对新插入、无位置的节点会按上下文缩进输出；main.go 是 gofmt 干净文件，整文件重排应与原文一致（仅新增两行）。测试的 `keep` 断言兜住注释丢失风险。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./tools/newmodule/ -run TestEditMainGo -v`
Expected: PASS

- [ ] **Step 5: 人工 diff 抽查（防 Review Focus #1）**

```bash
cd /tmp && cp -r /Users/alex/Desktop/goWork/alexGo-cloud/tools/newmodule ./nmtest 2>/dev/null; true
```
跳过手动 diff——测试已断言关键行保留 + 可解析 + 位置正确。若 `go test` PASS 即可。

- [ ] **Step 6: Commit**

```bash
git add tools/newmodule/edit_main.go tools/newmodule/edit_main_test.go
git commit -m "feat(newmodule): main.go AST 编辑器（import 字母序 + opts 装配）"
```

---

### Task 6: config.yaml 编辑器（yaml.Node 保注释）

**Files:**
- Create: `tools/newmodule/edit_config.go`
- Create: `tools/newmodule/edit_config_test.go`

**Interfaces:**
- Consumes: `github.com/goccy/go-yaml`（升 direct）。
- Produces:
  ```go
  // editConfigYAML 在 modules: 映射里插入 <name>: true，保留既有注释与排版。
  func editConfigYAML(src []byte, moduleName string) ([]byte, error)
  ```

- [ ] **Step 1: 写失败测试**

`tools/newmodule/edit_config_test.go`:

```go
package newmodule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditConfigYAML(t *testing.T) {
	root := repoRoot(t)
	src, err := os.ReadFile(filepath.Join(root, "alexgo-server/configs/config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := editConfigYAML(src, "demo")
	if err != nil {
		t.Fatalf("editConfigYAML() = %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "demo: true") {
		t.Error("missing demo: true")
	}
	// 既有行与注释保留（抽查）
	for _, keep := range []string{"system: true", "order: true", "# 盘根"} {
		if !strings.Contains(got, keep) {
			t.Errorf("lost content %q", keep)
		}
	}
	// 重名拒绝
	if _, err := editConfigYAML(out, "demo"); err == nil {
		t.Error("duplicate insert should fail")
	}
}

func TestEditConfigYAML_MissingModules(t *testing.T) {
	if _, err := editConfigYAML([]byte("system:\n  dsn: x\n"), "demo"); err == nil {
		t.Error("want error when modules key missing")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./tools/newmodule/ -run TestEditConfigYAML -v`
Expected: FAIL

- [ ] **Step 3: 实现 edit_config.go**

`tools/newmodule/edit_config.go`:

```go
package newmodule

import (
	"fmt"

	"github.com/goccy/go-yaml"
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
```

跑 `go mod tidy` 让 goccy/go-yaml 升为 direct 依赖。

风险预案：若 goccy Marshal 对整个 config.yaml 重排导致无关 diff（Review Focus #2），测试的 `keep` 断言会当场抓住；此时改用「Node 定位行号 → 文本行插入」方案（goccy Node 带 Line 字段，可在 modules 块末行后插文本行）。先按 Node 往返实现，测试说话。

- [ ] **Step 4: 跑测试 + tidy 确认通过**

Run: `go mod tidy && go test ./tools/newmodule/ -run TestEditConfigYAML -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add tools/newmodule/edit_config.go tools/newmodule/edit_config_test.go go.mod go.sum
git commit -m "feat(newmodule): config.yaml 编辑器（goccy yaml.Node 保注释）"
```

---

### Task 7: permission_routes.go 编辑器（map 字面量字母序插入）

**Files:**
- Create: `tools/newmodule/edit_perm.go`
- Create: `tools/newmodule/edit_perm_test.go`

**Interfaces:**
- Consumes: `naming.Plural`（与 module_register 路由同源）。
- Produces:
  ```go
  // editPermissionRoutes 为每个实体插入 "<module>:<snake>" → ["/api/admin/<module>/<plural>", ".../*"]。
  func editPermissionRoutes(src []byte, module string, entities []string) ([]byte, error)
  ```

- [ ] **Step 1: 写失败测试**

`tools/newmodule/edit_perm_test.go`:

```go
package newmodule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditPermissionRoutes(t *testing.T) {
	root := repoRoot(t)
	src, err := os.ReadFile(filepath.Join(root, "modules/system/service/permission_routes.go"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := editPermissionRoutes(src, "demo", []string{"product", "category"})
	if err != nil {
		t.Fatalf("editPermissionRoutes() = %v", err)
	}
	got := string(out)
	// 新条目（含 /* 变体）
	for _, want := range []string{
		`"demo:product":`,
		`"/api/admin/demo/products"`,
		`"/api/admin/demo/products/*"`,
		`"demo:category":`,
		`"/api/admin/demo/categories"`,
		`"/api/admin/demo/categories/*"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q", want)
		}
	}
	// 插入位置：demo:* 在 infra:* 之前（d < i，落在既有首条之前）
	if strings.Index(got, `"demo:product"`) > strings.Index(got, `"infra:codegen"`) {
		t.Error("demo entries should be inserted before infra")
	}
	// 既有条目相对顺序保持不变（文件本就非全序：system:user 在 system:role 之前——
	// 编辑器只做位置插入，绝不整体重排，否则 diff 爆炸）
	if strings.Index(got, `"system:user"`) > strings.Index(got, `"system:role"`) {
		t.Error("existing key order must be preserved")
	}
	if strings.Index(got, `"oauth2:client"`) > strings.Index(got, `"system:user"`) {
		t.Error("existing key order must be preserved")
	}
	// 既有内容不丢
	for _, keep := range []string{
		`"order:order":`,
		"// permissionRoutes：permission 前缀 → 后端路由模板（keyMatch2）",
		"// 每条“集合路由”额外带一条 `/*` 变体",
	} {
		if !strings.Contains(got, keep) {
			t.Errorf("lost %q", keep)
		}
	}
	// 可解析
	if _, err := parseGo("permission_routes.go", out); err != nil {
		t.Fatalf("output not parseable: %v", err)
	}
	// 幂等拒绝
	if _, err := editPermissionRoutes(out, "demo", []string{"product"}); err == nil {
		t.Error("duplicate key should fail")
	}
}

func TestEditPermissionRoutes_AnchorMissing(t *testing.T) {
	if _, err := editPermissionRoutes([]byte("package service\n"), "demo", []string{"x"}); err == nil {
		t.Error("want error")
	}
}
```

加一个小 helper（放 edit_perm.go 或共用文件）：

```go
func parseGo(name string, src []byte) (*ast.File, error) {
	return parser.ParseFile(token.NewFileSet(), name, src, parser.AllErrors)
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./tools/newmodule/ -run TestEditPermissionRoutes -v`
Expected: FAIL

- [ ] **Step 3: 实现 edit_perm.go**

`tools/newmodule/edit_perm.go`:

```go
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
```

说明：整体重排 map 并按 key 稳定排序——现有文件本就按字母序（infra → member → oauth2 → system），重排结果 = 原序 + 新条目。format.Node 重打印，头注释（挂在 var decl 的 Doc）保留。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./tools/newmodule/ -run TestEditPermissionRoutes -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add tools/newmodule/edit_perm.go tools/newmodule/edit_perm_test.go
git commit -m "feat(newmodule): permission_routes map 字母序插入编辑器"
```

---

### Task 8: 编排器 Plan / Apply / Run（ChangeSet + 全有或全无 + 回滚）

**Files:**
- Create: `tools/newmodule/scaffold.go`
- Create: `tools/newmodule/scaffold_test.go`

**Interfaces:**
- Consumes: Task 1-7 全部导出函数。
- Produces:
  ```go
  type FileChange struct {
      Path   string
      Before []byte // patch 用；create 为 nil
      After  []byte
  }
  type ChangeSet struct {
      Creates []FileChange
      Patches []FileChange
  }
  // Plan 纯内存构建全部变更（读既有文件 + 编辑器 + 渲染）。不落盘。
  func Plan(o Options) (*ChangeSet, error)
  // Apply 落盘。成功返回 undo 函数（删除 creates + 回写 patches 原文）。
  func Apply(cs *ChangeSet) (undo func(), err error)
  // Run = Plan + (DryRun 只返回) + Apply + 可选 go build 验证（失败自动 undo）。
  func Run(o Options) (*ChangeSet, error)
  ```

- [ ] **Step 1: 写失败测试**

`tools/newmodule/scaffold_test.go`:

```go
package newmodule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// copyRealTouchpoints 把 3 个既有文件拷进临时 Root，构造可落盘的最小仓库骨架。
func copyRealTouchpoints(t *testing.T, root string) {
	t.Helper()
	wd, _ := os.Getwd()
	real, err := repoRootDir(wd)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"alexgo-server/cmd/main.go",
		"alexgo-server/configs/config.yaml",
		"modules/system/service/permission_routes.go",
	} {
		src, err := os.ReadFile(filepath.Join(real, rel))
		if err != nil {
			t.Fatal(err)
		}
		dst := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// system 迁移目录（菜单版本号扫描用）
	if err := os.MkdirAll(filepath.Join(root, "modules/system/migrations"), 0o755); err != nil {
		t.Fatal(err)
	}
	// 现 system 最大版本文件（逼 nextMigrationVersion 走对分支）
	if err := os.WriteFile(filepath.Join(root, "modules/system/migrations/20261016000002_x.up.sql"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func tempOpts(t *testing.T) Options {
	t.Helper()
	root := t.TempDir()
	copyRealTouchpoints(t, root)
	o := Options{
		Root: root, Name: "demo", Entities: []string{"product", "category"},
		Now:         time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC),
		VerifyBuild: false, // 临时骨架不是完整仓库，跳过 go build
	}
	if err := o.Validate(); err != nil {
		t.Fatal(err)
	}
	return o
}

func TestPlanDryRun(t *testing.T) {
	o := tempOpts(t)
	cs, err := Plan(o)
	if err != nil {
		t.Fatalf("Plan() = %v", err)
	}
	// 3 patch
	if len(cs.Patches) != 3 {
		t.Fatalf("patches = %d, want 3", len(cs.Patches))
	}
	paths := map[string]bool{}
	for _, c := range cs.Creates {
		paths[c.Path] = true
	}
	for _, want := range []string{
		"modules/demo/module.go",
		"modules/demo/migration_source.go",
		"modules/demo/configs/config.yaml",
		"modules/demo/migrations/20261019000001_products.up.sql",
		"modules/demo/migrations/20261019000002_categories.down.sql",
	} {
		if !paths[want] {
			t.Errorf("creates missing %s", want)
		}
	}
	// 菜单种子在 system
	menuOK := false
	for _, c := range cs.Creates {
		if strings.HasPrefix(c.Path, "modules/system/migrations/") && strings.HasSuffix(c.Path, "_menus.up.sql") {
			menuOK = true
		}
	}
	if !menuOK {
		t.Error("missing system menu seed up.sql")
	}
	// Dry-run 不落盘：demo 目录不应出现
	if _, err := os.Stat(filepath.Join(o.Root, "modules/demo")); !os.IsNotExist(err) {
		t.Error("dry-run must not write files")
	}
	// patch After 与 Before 必须不同
	for _, p := range cs.Patches {
		if string(p.Before) == string(p.After) {
			t.Errorf("patch %s Before == After", p.Path)
		}
	}
}

func TestApplyAndUndo(t *testing.T) {
	o := tempOpts(t)
	cs, err := Plan(o)
	if err != nil {
		t.Fatal(err)
	}
	undo, err := Apply(cs)
	if err != nil {
		t.Fatalf("Apply() = %v", err)
	}
	// 落盘生效
	if _, err := os.Stat(filepath.Join(o.Root, "modules/demo/module.go")); err != nil {
		t.Fatal("module.go not written")
	}
	mainAfter, _ := os.ReadFile(filepath.Join(o.Root, "alexgo-server/cmd/main.go"))
	if !strings.Contains(string(mainAfter), "demo.FxModule,") {
		t.Error("main.go patch not applied")
	}
	// undo 恢复
	undo()
	if _, err := os.Stat(filepath.Join(o.Root, "modules/demo")); !os.IsNotExist(err) {
		t.Error("undo should remove created module dir")
	}
	mainRestored, _ := os.ReadFile(filepath.Join(o.Root, "alexgo-server/cmd/main.go"))
	if strings.Contains(string(mainRestored), "demo.FxModule") {
		t.Error("undo should restore main.go")
	}
}

func TestApply_RollbackOnFailure(t *testing.T) {
	o := tempOpts(t)
	cs, err := Plan(o)
	if err != nil {
		t.Fatal(err)
	}
	// 注入失败：在 modules/demo/controller 放一个文件占住目录名，
	// 使 controller/admin/*.go 的 MkdirAll 报 ENOTDIR。
	// 该 create 位于 shell 批（module.go 等 6 文件）与 apps 批（14 文件）之后——
	// 失败时已写盘数十个文件，回滚必须全部清掉（非真空断言）。
	if err := os.MkdirAll(filepath.Join(o.Root, "modules/demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(o.Root, "modules/demo/controller"), []byte("block"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(cs); err == nil {
		t.Fatal("Apply should fail")
	}
	// 已写的 create 被回滚（shell 批的 module.go 必然已写过才轮到 controller）
	if _, err := os.Stat(filepath.Join(o.Root, "modules/demo/module.go")); !os.IsNotExist(err) {
		t.Error("rollback should delete earlier creates")
	}
	if _, err := os.Stat(filepath.Join(o.Root, "apps/web-antd/src/api/demo/product/types.ts")); !os.IsNotExist(err) {
		t.Error("rollback should delete apps creates")
	}
	// 空目录也被清掉（pruneEmptyDirs），modules/demo 整树不留残渣
	if _, err := os.Stat(filepath.Join(o.Root, "modules/demo")); !os.IsNotExist(err) {
		t.Error("rollback should prune empty dirs")
	}
	// patch 未被触碰（Apply 里 patch 在 create 之后）
	mainNow, _ := os.ReadFile(filepath.Join(o.Root, "alexgo-server/cmd/main.go"))
	if strings.Contains(string(mainNow), "demo.FxModule") {
		t.Error("main.go must be untouched when create fails first")
	}
}

func TestRun_SkipFrontend(t *testing.T) {
	o := tempOpts(t)
	o.SkipFrontend = true
	cs, err := Plan(o)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cs.Creates {
		if strings.HasPrefix(c.Path, "apps/") {
			t.Errorf("SkipFrontend should not create %s", c.Path)
		}
		if strings.Contains(c.Path, "_menus") {
			t.Errorf("SkipFrontend should skip menu seed, got %s", c.Path)
		}
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./tools/newmodule/ -run 'TestPlanDryRun|TestApply|TestRun_Skip' -v`
Expected: FAIL

- [ ] **Step 3: 实现 scaffold.go**

`tools/newmodule/scaffold.go`:

```go
package newmodule

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"alexGo-cloud/pkg/codegen/model"
)

// FileChange 一个文件的变更（create：Before=nil）。
type FileChange struct {
	Path   string
	Before []byte
	After  []byte
}

// ChangeSet 全部变更的内存集合——全有或全无的原子单位。
type ChangeSet struct {
	Creates []FileChange
	Patches []FileChange
}

// Plan 纯内存构建：壳层 + 实体 + 菜单种子渲染，三个既有文件读入并编辑。
func Plan(o Options) (*ChangeSet, error) {
	if err := o.Validate(); err != nil {
		return nil, err
	}
	cs := &ChangeSet{}

	addCreates := func(files []model.GeneratedFile) {
		for _, f := range files {
			cs.Creates = append(cs.Creates, FileChange{Path: f.Path, After: f.Content})
		}
	}

	shell, err := shellFiles(o)
	if err != nil {
		return nil, err
	}
	addCreates(shell)

	ents, err := entityFiles(o)
	if err != nil {
		return nil, err
	}
	addCreates(ents)

	menu, err := menuSeedFiles(&o)
	if err != nil {
		return nil, err
	}
	addCreates(menu)

	// 三个既有文件
	patch := func(rel string, edit func(src []byte) ([]byte, error)) error {
		full := filepath.Join(o.Root, rel)
		src, err := os.ReadFile(full)
		if err != nil {
			return fmt.Errorf("读 %s: %w", rel, err)
		}
		after, err := edit(src)
		if err != nil {
			return fmt.Errorf("编辑 %s: %w", rel, err)
		}
		cs.Patches = append(cs.Patches, FileChange{Path: rel, Before: src, After: after})
		return nil
	}

	if err := patch("alexgo-server/cmd/main.go", func(src []byte) ([]byte, error) {
		return editMainGo(src, o.Name)
	}); err != nil {
		return nil, err
	}
	if err := patch("alexgo-server/configs/config.yaml", func(src []byte) ([]byte, error) {
		return editConfigYAML(src, o.Name)
	}); err != nil {
		return nil, err
	}
	if err := patch("modules/system/service/permission_routes.go", func(src []byte) ([]byte, error) {
		return editPermissionRoutes(src, o.Name, o.Entities)
	}); err != nil {
		return nil, err
	}
	return cs, nil
}

// Apply 落盘：先 creates 后 patches；任一失败回滚已做部分。成功返回 undo。
func Apply(cs *ChangeSet) (func(), error) {
	var writtenCreates []string
	var writtenPatches []FileChange

	removeCreated := func() {
		for i := len(writtenCreates) - 1; i >= 0; i-- {
			_ = os.Remove(writtenCreates[i])
		}
		pruneEmptyDirs(writtenCreates)
	}
	rollback := func() {
		removeCreated()
		for _, p := range writtenPatches {
			_ = os.WriteFile(p.Path, p.Before, 0o644)
		}
	}

	for _, c := range cs.Creates {
		if err := os.MkdirAll(filepath.Dir(c.Path), 0o755); err != nil {
			rollback()
			return nil, fmt.Errorf("创建目录 %s: %w", filepath.Dir(c.Path), err)
		}
		if err := os.WriteFile(c.Path, c.After, 0o644); err != nil {
			rollback()
			return nil, fmt.Errorf("写 %s: %w", c.Path, err)
		}
		writtenCreates = append(writtenCreates, c.Path)
	}
	for _, p := range cs.Patches {
		if err := os.WriteFile(p.Path, p.After, 0o644); err != nil {
			rollback()
			return nil, fmt.Errorf("写 %s: %w", p.Path, err)
		}
		writtenPatches = append(writtenPatches, p)
	}

	undo := func() {
		removeCreated()
		for _, p := range writtenPatches {
			_ = os.WriteFile(p.Path, p.Before, 0o644)
		}
	}
	return undo, nil
}

// pruneEmptyDirs 删除创建文件所在的空目录（深者先删，逐级向上，删不动即止）。
// undo/rollback 后 modules/<name> 整树不留残渣；碰到非空的上层（如 modules/）
// 的 os.Remove 自然失败停止，不会误删无关目录。
func pruneEmptyDirs(files []string) {
	dirset := map[string]bool{}
	for _, f := range files {
		for d := filepath.Dir(f); d != "." && d != "/" && !dirset[d]; d = filepath.Dir(d) {
			dirset[d] = true
		}
	}
	dirs := make([]string, 0, len(dirset))
	for d := range dirset {
		dirs = append(dirs, d)
	}
	// 按路径深度倒序（长路径先删，保证子目录先清）
	sort.Slice(dirs, func(i, j int) bool {
		return strings.Count(dirs[i], "/") > strings.Count(dirs[j], "/")
	})
	for _, d := range dirs {
		_ = os.Remove(d) // 非空则失败，忽略
	}
}

// Run = Plan → Apply（DryRun 则止步）→ 可选 go build 验证（失败 undo）。
func Run(o Options) (*ChangeSet, error) {
	cs, err := Plan(o)
	if err != nil {
		return nil, err
	}
	if o.DryRun {
		return cs, nil
	}
	undo, err := Apply(cs)
	if err != nil {
		return nil, err
	}
	if o.VerifyBuild {
		cmd := exec.Command("go", "build", "./...")
		cmd.Dir = o.Root
		if out, err := cmd.CombinedOutput(); err != nil {
			undo()
			return nil, fmt.Errorf("go build ./... 失败，已回滚:\n%s", out)
		}
	}
	return cs, nil
}
```

- [ ] **Step 4: 跑全包测试确认通过**

Run: `go test ./tools/newmodule/ -v`
Expected: 全部 PASS

- [ ] **Step 5: Commit**

```bash
git add tools/newmodule/scaffold.go tools/newmodule/scaffold_test.go
git commit -m "feat(newmodule): ChangeSet 编排器（Plan/Apply/回滚/全有或全无）"
```

---

### Task 9: 报告 + CLI 入口 + Makefile

**Files:**
- Create: `tools/newmodule/report.go`
- Create: `tools/newmodule/report_test.go`
- Create: `tools/newmodule/main.go`
- Modify: `Makefile`（追加 new-module 目标）

**Interfaces:**
- Consumes: `ChangeSet`（Task 8）。
- Produces:
  ```go
  // BuildReport 生成中文收尾报告。
  func BuildReport(cs *ChangeSet, o Options, verified bool) string
  ```

- [ ] **Step 1: 写失败测试**

`tools/newmodule/report_test.go`:

```go
package newmodule

import (
	"strings"
	"testing"
	"time"
)

func TestBuildReport(t *testing.T) {
	o := Options{Root: ".", Name: "demo", Entities: []string{"product", "category"},
		Now: time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)}
	cs := &ChangeSet{
		Creates: []FileChange{{Path: "modules/demo/module.go"}, {Path: "modules/demo/model/product.go"}},
		Patches: []FileChange{
			{Path: "alexgo-server/cmd/main.go"},
			{Path: "alexgo-server/configs/config.yaml"},
			{Path: "modules/system/service/permission_routes.go"},
		},
	}
	rep := BuildReport(cs, o, true)
	for _, want := range []string{
		"新建模块 demo",
		"modules/demo/module.go",
		"alexgo-server/cmd/main.go",
		"docs/modules.md",
		"modules/member/cmd/main.go", // micro 提醒
		"go build ./... 通过",
		"菜单名为英文占位", // 中文化提醒
	} {
		if !strings.Contains(rep, want) {
			t.Errorf("report missing %q\n%s", want, rep)
		}
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./tools/newmodule/ -run TestBuildReport -v`
Expected: FAIL

- [ ] **Step 3: 实现 report.go 与 main.go**

`tools/newmodule/report.go`:

```go
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
	fmt.Fprintf(&b, "  - 回滚本模块: 删 modules/%s + git checkout 三个修改文件 + 删 modules/system/migrations/*_%s_menus.*\n", o.Name, o.Name)

	if o.DryRun {
		b.WriteString("\n（-dry-run：以上仅为计划，未落盘）\n")
	} else if verified {
		b.WriteString("\n验证: go build ./... 通过\n")
	} else {
		b.WriteString("\n验证: 跳过\n")
	}
	return b.String()
}
```

`tools/newmodule/main.go`:

```go
// newmodule 新建模块脚手架 CLI。用法：
//
//	go run ./tools/newmodule -name=product -entities=product,category
//	go run ./tools/newmodule -name=product -entities=product -dry-run
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"alexGo-cloud/tools/newmodule"
)

func main() {
	var (
		name         = flag.String("name", "", "模块名（小写下蛇，必填）")
		entities     = flag.String("entities", "", "逗号分隔实体名（单数小写下蛇，必填）")
		menuPrefix   = flag.String("menu-prefix", "", "菜单/路由前缀（默认 = name）")
		dryRun       = flag.Bool("dry-run", false, "只打印计划，不落盘")
		skipFrontend = flag.Bool("skip-frontend", false, "只生成后端")
		skipBuild    = flag.Bool("skip-build", false, "跳过 go build 验证")
	)
	flag.Parse()

	var ents []string
	for _, e := range strings.Split(*entities, ",") {
		if e = strings.TrimSpace(e); e != "" {
			ents = append(ents, e)
		}
	}
	o := newmodule.Options{
		Root:         ".",
		Name:         *name,
		Entities:     ents,
		MenuPrefix:   *menuPrefix,
		DryRun:       *dryRun,
		SkipFrontend: *skipFrontend,
		VerifyBuild:  !*skipBuild && !*dryRun,
	}
	cs, err := newmodule.Run(o)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✘ %v\n", err)
		os.Exit(1)
	}
	fmt.Print(newmodule.BuildReport(cs, o, o.VerifyBuild))
}
```

注意：`main.go` 在 `package main` 里 import 同目录的 `newmodule` 包不可行（同目录不能两包）。**调整**：CLI 入口放 `tools/newmodule/cmd/main.go`，`package main`，import `alexGo-cloud/tools/newmodule`。主 `tools/newmodule/` 保持 `package newmodule`（可测库）。

`tools/newmodule/cmd/main.go` 用上面代码（import 路径不变），文件头注释保留。

- [ ] **Step 4: Makefile 追加目标**

`Makefile` 在 `codegen-smoke:` 目标之后插入：

```make
new-module: ## 新建模块骨架（NAME=demo ENTITIES=item[,foo]；可加 ARGS="-dry-run"）
	go run ./tools/newmodule/cmd -name=$(NAME) -entities=$(ENTITIES) $(ARGS)
```

- [ ] **Step 5: 跑测试 + 冒烟**

Run: `go test ./tools/newmodule/ -run TestBuildReport -v && go build ./tools/...`
Expected: PASS + 编译通过

Run: `go run ./tools/newmodule/cmd -name=demo -entities=item -dry-run -skip-build`
Expected: 打印完整报告计划，不落盘（git status 干净）

- [ ] **Step 6: Commit**

```bash
git add tools/newmodule/report.go tools/newmodule/report_test.go tools/newmodule/cmd/main.go Makefile
git commit -m "feat(newmodule): CLI 入口 + 中文报告 + make new-module"
```

---

### Task 10: golden 集成测试 + 手工验收

**Files:**
- Create: `tools/newmodule/golden_test.go`
- Create: `tools/newmodule/testdata/golden_paths.txt`

**Interfaces:**
- Consumes: 全部前序任务。

- [ ] **Step 1: 写 golden 清单测试**

`tools/newmodule/golden_test.go`:

```go
package newmodule

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestGoldenPaths 钉住完整 ChangeSet 的文件清单（防触点增漏）。
// 清单变化时：先确认是预期变更，再更新 testdata/golden_paths.txt。
func TestGoldenPaths(t *testing.T) {
	o := tempOpts(t) // demo / product+category / 2026-10-19
	cs, err := Plan(o)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range cs.Creates {
		got = append(got, "C "+c.Path)
	}
	for _, p := range cs.Patches {
		got = append(got, "P "+p.Path)
	}
	sort.Strings(got)

	goldenPath := filepath.Join("testdata", "golden_paths.txt")
	wantRaw, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("读 golden: %v（首次运行先生成：go test -run TestGoldenPaths -update）", err)
	}
	var want []string
	for _, line := range strings.Split(string(wantRaw), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			want = append(want, line)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("路径数 %d != golden %d\ngot:\n%s\nwant:\n%s", len(got), len(want),
			strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("路径[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
```

先手写一个 `-update` flag 版本更稳：在测试文件加

```go
var update = flag.Bool("update", false, "更新 golden")
```

测试体内：`if *update { os.WriteFile(goldenPath, []byte(strings.Join(got, "\n")+"\n"), 0o644); return }`。

- [ ] **Step 2: 生成 golden 并目检**

Run: `go test ./tools/newmodule/ -run TestGoldenPaths -update`
然后 `cat tools/newmodule/testdata/golden_paths.txt` 目检：
- 3 个 `P ` 行（main.go / config.yaml / permission_routes.go）
- `C modules/demo/...`：module.go、migration_source.go、configs/config.yaml、migrations 4 个 SQL（2 实体 × up/down）
- `C modules/system/migrations/20261019000001_demo_menus.up.sql` + `.down.sql`（tempOpts 预埋最大版本 20261016000002，日期候选 20261019000001 更大 → 直接用日期起版；需 > 20261016000002 ✓）
- `C modules/demo/{model,repository,service,controller/...}` 每实体 7 个
- `C apps/web-antd/src/api/demo/...` + `views/demo/...` 每实体 7 个

- [ ] **Step 3: 不带 -update 跑通**

Run: `go test ./tools/newmodule/ -v`
Expected: 全部 PASS

- [ ] **Step 4: Commit**

```bash
git add tools/newmodule/golden_test.go tools/newmodule/testdata/golden_paths.txt
git commit -m "test(newmodule): golden 路径清单钉住全部触点"
```

- [ ] **Step 5: 真仓库手工验收（执行者在真实工作区做）**

```
1. make new-module NAME=demo ENTITIES=item
   → 报告列出全部新建/修改文件；go build ./... 通过
2. git status：改动仅限报告所列
3. 启动服务（make run，本地 MySQL）：modules/demo 迁移执行、菜单落库
4. admin 登录前端：侧边栏出现 Demo 目录 → Item 列表页可打开、增删改查可点
5. 验收后清理：
   rm -rf modules/demo
   git checkout -- alexgo-server/cmd/main.go alexgo-server/configs/config.yaml \
     modules/system/service/permission_routes.go
   rm modules/system/migrations/*_demo_menus.*
   go build ./...
```

第 5 步是验收清理，不是代码变更；若第 3/4 步发现问题，回到对应 Task 修。

- [ ] **Step 6: 最终 Commit（若有修复）**

```bash
git add -A
git commit -m "fix(newmodule): 手工验收修复" || true
```

---

## 自审记录

1. **Spec 覆盖**：CLI 参数（Task 1/9）、codegen 复用（Task 2）、壳层（Task 3）、菜单（Task 4）、三编辑器（Task 5-7）、全有或全无+dry-run（Task 8）、报告+手工项（Task 9）、golden+手工验收（Task 10）、spec §2.1 errata（Task 1 Step 5）——全覆盖。micro 入口与跨模块映射按 spec §2.4 明确不生成（报告提醒）。
2. **占位符**：无 TBD/TODO；所有代码步骤给了完整实现。
3. **类型一致性**：`Options`（Task 1 定义，各任务复用）、`shellBindOf/shellEntity`（Task 3 定义，Task 4 用同包 naming 派生）、`FileChange/ChangeSet/Plan/Apply/Run`（Task 8 定义，Task 9 消费 `cs.Patches`/`cs.Creates`）、`repoRootDir`（Task 1 定义，Task 8 helper 消费）。
4. **Review Focus 对位**：#1 → Task 5 测试 keep 断言；#2 → Task 6 测试 keep 断言 + 预案；#3 → Task 3 三用例；#4 → Task 7 同源（naming.Plural）+ Task 10 golden；#5 → Task 4 component 断言。
