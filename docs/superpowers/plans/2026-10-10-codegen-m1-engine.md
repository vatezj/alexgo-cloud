# 代码生成引擎地基（M1）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 建成 `pkg/codegen` 统一引擎（元数据 → 中间模型 → 模板渲染 → 后处理）+ 单表后端模板族 + CLI 草稿模式，使 order 库真实表可生成并通过 `go build`。

**Architecture:** 引擎为无依赖纯库（叶子包 `model` 承载全部类型与哨兵错误，避免 import 环）；模板经 `//go:embed` 内嵌，输出 `[]model.GeneratedFile` 内存产物；`tools/codegen` CLI 以"先全部渲染成功、再逐文件写盘、默认不覆盖"落盘。本计划仅覆盖 spec 里程碑 **M1**；M2–M7 依次续写独立计划。

**Tech Stack:** Go 1.25、text/template、database/sql + go-sql-driver/mysql、jinzhu/inflection、标准库 go/format。

**Spec:** `docs/superpowers/specs/2026-10-10-codegen-redesign-design.md`

## Global Constraints

- 仅 MySQL；元数据读取必须经 `metadata.MetadataReader` 接口（保留扩展位，不引入其他数据库）。
- `pkg/codegen` 树内禁止 import `modules/*`、禁止依赖 fx；除标准库外仅允许 `github.com/go-sql-driver/mysql` 与 `github.com/jinzhu/inflection`。
- CLI 默认不覆盖已存在文件；只有 `--force` 才覆盖。任何生成错误 → 不写盘、非零退出、错误逐条列出。
- `go/format` 失败必须报错（`model.ErrFormat`），绝不输出未格式化文件。
- `unit-test-enable` 默认开启：`configs/config.yaml` 键 `codegen.unit_test_enable`，loader `SetDefault(true)`；CLI `--no-tests` 可关。
- M1 只实现 `server/go/single` 模板族；树表/主子表/前端 → 返回 `model.ErrTemplateMissing`。
- 生成文件命名与位置：`modules/<module>/{model,repository,service,controller/admin,controller/app}/<snake>.go`、`modules/<module>/service/<snake>_test.go`、`modules/<module>/module_register_<snake>.go`（`<snake>` = SnakeCase(ClassName)，如 `order_item`）。
- 表主键列名约定为 `id`：`service_test`/`controller_admin` 模板硬编码 `entity.ID` 字段；主键列非 `id` 的表不在 M1 保证范围。
- 提交直接落 main；每个任务一次提交，遵循现有 commit message 风格（`feat(codegen):` / `test(codegen):` / `docs:`）。
- 每个任务 TDD：先写失败测试 → 确认失败 → 最小实现 → 通过 → 提交。

## Review Focus

以下 5 类输入/状态最可能咬到使用者，每条在所属任务中有钉死它的测试：

1. **不规则复数表名**（`addresses`、`categories`、`user_infos`）→ 实体名算错（旧 dbgen 的 `TrimSuffix "s"` 会把 `address` 砍成 `Addres`）→ Task 2 `EntityName` 表驱动测试。
2. **敏感字段**（`password`/`secret`/`token`）→ JSON 输出泄露 → Task 2 `JSONName` 测试 + Task 10 golden 断言产物含 `json:"-"`。
3. **可空/unsigned 类型**（`int NULL`→`*int`、`datetime NULL`→`*time.Time`、`bigint unsigned`→`uint64`）→ 映射错则生成代码编译失败 → Task 4 `MySQLTypeToGo` 表驱动测试。
4. **失败被静默**（format 失败放行、多表只报第一个错）→ 产出垃圾或返工多轮 → Task 6 `FormatGo` 失败测试 + Task 10 聚合错误测试（`errors.Is` 同时命中两个哨兵）。
5. **CLI 覆盖手写代码** → 毁掉人工修改 → Task 11 `writeFiles` 无 force 跳过/内容不变测试 + force 覆盖测试。

## File Structure

```
pkg/codegen/
├── generate.go              # 根入口 Generate（编排 render→format→collide，聚合错误）
├── generate_test.go         # golden 测试 + 聚合错误测试
├── model/                   # 叶子包：领域类型 + 哨兵错误（所有子包可安全依赖）
│   ├── errors.go            # Err* 哨兵
│   ├── enums.go             # TemplateType / FrontType / HTMLType
│   ├── table.go             # Table + Validate()
│   ├── column.go            # Column
│   ├── file.go              # GeneratedFile
│   └── model_test.go
├── naming/                  # 纯字符串工具（builder 与 template 共用，无依赖）
│   ├── naming.go            # EntityName / ToGoName / Snake / Plural / JSONName / IsSensitive
│   └── naming_test.go
├── metadata/                # 元数据读取（叶子包，仅依赖 model 哨兵）
│   ├── reader.go            # ColumnMeta / TableMeta / MetadataReader
│   ├── mysql.go             # information_schema 实现
│   ├── mysql_integration_test.go  # -tags integration
├── builder/
│   ├── builder.go           # Build(meta, Options) → *model.Table（默认值/开关初置）
│   ├── typing.go            # MySQLTypeToGo / MySQLTypeToHTML / BuildGormTag
│   └── builder_test.go, typing_test.go
├── postprocess/
│   ├── golang.go            # FormatGo（失败→ErrFormat）
│   ├── collide.go           # CheckCollide（重复路径→ErrPathCollide）
│   └── postprocess_test.go
└── template/
    ├── bind.go              # Bind / BindColumn / newBind
    ├── paths.go             # 模板文件 → 输出路径模式表 + renderPath
    ├── funcs.go             # FuncMap: lower/snake/plural
    ├── embed.go             # //go:embed templates
    ├── render.go            # Render(table, Options) → []GeneratedFile
    ├── bind_test.go, paths_test.go, templates_test.go
    └── templates/server/go/single/   # 7 个 .tmpl（见 Task 9）
tools/codegen/
├── main.go                  # CLI：flags → 元数据 → Build → Generate → writeFiles
├── main_test.go             # writeFiles 覆盖行为测试
└── smoke_integration_test.go  # -tags integration：真实表→写盘→go build→清理
pkg/config/config.go         # + Codegen.UnitTestEnable
pkg/config/loader.go         # + SetDefault("codegen.unit_test_enable", true)
pkg/config/loader_test.go    # 默认值测试
Makefile                     # + codegen-smoke 目标
```

依赖方向（单向，禁止反向）：
`tools/codegen → {metadata, builder, codegen(root), config}`；
`codegen(root) → {model, template, postprocess}`；
`template → {model, naming}`；`builder → {model, metadata, naming}`；`postprocess → model`；`metadata → model`（仅哨兵错误）。

---

### Task 1: model 领域类型与哨兵错误

**Files:**
- Create: `pkg/codegen/model/errors.go`
- Create: `pkg/codegen/model/enums.go`
- Create: `pkg/codegen/model/table.go`
- Create: `pkg/codegen/model/column.go`
- Create: `pkg/codegen/model/file.go`
- Test: `pkg/codegen/model/model_test.go`

**Interfaces:**
- Consumes: 无（叶子包）。
- Produces（后续所有任务依赖，签名精确）:
  - `model.Table{ TableName, TableComment, Module, BusinessName, ClassName string; TemplateType TemplateType; FrontType FrontType; ParentTableID int64; Remark string; HasTenant bool; Columns []Column }`
  - `(*model.Table).Validate() error`
  - `model.Column{ Name, Comment, GoName, GoType, JSONName, GormTag string; HTMLType HTMLType; IsPK, AutoIncrement, Nullable bool; ListEnable, FormEnable, QueryEnable bool; QueryOp string; ListRequired, FormRequired bool; DictType string; SortOrder int; Deprecated bool }`
  - `model.GeneratedFile{ Path string; Content []byte }`
  - 枚举: `TemplateTypeSingle/Tree/MainSub = 1/2/3`、`FrontTypeVben5Antd = 1`、`HTMLInput|HTMLTextarea|HTMLInputNumber|HTMLSelect|HTMLSwitch|HTMLDatePicker`
  - 哨兵: `ErrMetadataUnreachable, ErrTableNotFound, ErrTableInvalid, ErrColumnInvalid, ErrTypeMappingUnknown, ErrTemplateMissing, ErrRender, ErrFormat, ErrPathCollide`

- [ ] **Step 1: 写失败测试**

```go
// pkg/codegen/model/model_test.go
package model_test

import (
	"errors"
	"testing"

	"alexGo-cloud/pkg/codegen/model"
)

func validTable() *model.Table {
	return &model.Table{
		TableName:    "order_items",
		Module:       "order",
		BusinessName: "order_item",
		ClassName:    "OrderItem",
		TemplateType: model.TemplateTypeSingle,
		FrontType:    model.FrontTypeVben5Antd,
		Columns: []model.Column{
			{Name: "id", GoName: "ID", GoType: "uint64"},
		},
	}
}

func TestTableValidate_OK(t *testing.T) {
	if err := validTable().Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestTableValidate_NoColumns(t *testing.T) {
	tbl := validTable()
	tbl.Columns = nil
	if err := tbl.Validate(); !errors.Is(err, model.ErrColumnInvalid) {
		t.Fatalf("Validate() = %v, want ErrColumnInvalid", err)
	}
}

func TestTableValidate_DuplicateGoName(t *testing.T) {
	tbl := validTable()
	tbl.Columns = append(tbl.Columns, model.Column{Name: "name", GoName: "ID", GoType: "string"})
	if err := tbl.Validate(); !errors.Is(err, model.ErrColumnInvalid) {
		t.Fatalf("Validate() = %v, want ErrColumnInvalid", err)
	}
}

func TestTableValidate_MissingFields(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*model.Table)
	}{
		{"empty table name", func(t *model.Table) { t.TableName = "" }},
		{"empty module", func(t *model.Table) { t.Module = "" }},
		{"empty class name", func(t *model.Table) { t.ClassName = "" }},
		{"bad template type", func(t *model.Table) { t.TemplateType = 99 }},
		{"bad front type", func(t *model.Table) { t.FrontType = 0 }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tbl := validTable()
			c.mutate(tbl)
			if err := tbl.Validate(); err == nil {
				t.Fatal("Validate() = nil, want error")
			}
		})
	}
}

func TestEnums_Valid(t *testing.T) {
	for _, tt := range []model.TemplateType{1, 2, 3} {
		if !tt.Valid() {
			t.Errorf("TemplateType(%d).Valid() = false", tt)
		}
	}
	if model.TemplateType(0).Valid() || model.TemplateType(4).Valid() {
		t.Error("TemplateType out of range should be invalid")
	}
	if !model.FrontTypeVben5Antd.Valid() || model.FrontType(9).Valid() {
		t.Error("FrontType validation wrong")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./pkg/codegen/model/ -v`
Expected: FAIL（包不存在 / `undefined: model.Table`）

- [ ] **Step 3: 最小实现**

```go
// pkg/codegen/model/errors.go
package model

import "errors"

var (
	ErrMetadataUnreachable = errors.New("codegen: metadata unreachable")
	ErrTableNotFound       = errors.New("codegen: table not found")
	ErrTableInvalid        = errors.New("codegen: invalid table")
	ErrColumnInvalid       = errors.New("codegen: invalid column")
	ErrTypeMappingUnknown  = errors.New("codegen: unknown type mapping")
	ErrTemplateMissing     = errors.New("codegen: template missing")
	ErrRender              = errors.New("codegen: render failed")
	ErrFormat              = errors.New("codegen: format failed")
	ErrPathCollide         = errors.New("codegen: output path collision")
)
```

```go
// pkg/codegen/model/enums.go
package model

type TemplateType int

const (
	TemplateTypeSingle  TemplateType = 1
	TemplateTypeTree    TemplateType = 2
	TemplateTypeMainSub TemplateType = 3
)

func (t TemplateType) Valid() bool { return t >= TemplateTypeSingle && t <= TemplateTypeMainSub }

type FrontType int

const FrontTypeVben5Antd FrontType = 1

func (f FrontType) Valid() bool { return f == FrontTypeVben5Antd }

type HTMLType string

const (
	HTMLInput       HTMLType = "Input"
	HTMLTextarea    HTMLType = "Textarea"
	HTMLInputNumber HTMLType = "InputNumber"
	HTMLSelect      HTMLType = "Select"
	HTMLSwitch      HTMLType = "Switch"
	HTMLDatePicker  HTMLType = "DatePicker"
)
```

```go
// pkg/codegen/model/column.go
package model

type Column struct {
	Name    string // 列名，如 user_id
	Comment string // 列注释（已压平为单行）

	GoName   string // Go 字段名，如 UserID
	GoType   string // Go 类型，如 uint64、*time.Time
	JSONName string // json tag；敏感字段为 "-"
	GormTag  string // gorm tag 内容，如 column:user_id
	HTMLType HTMLType

	IsPK           bool
	AutoIncrement  bool
	Nullable       bool
	ListEnable     bool
	FormEnable     bool
	QueryEnable    bool
	QueryOp        string // eq / like / between
	ListRequired   bool
	FormRequired   bool
	DictType       string // 预留字典类型
	SortOrder      int
	Deprecated     bool // 同步时列已消失
}
```

```go
// pkg/codegen/model/table.go
package model

import "fmt"

type Table struct {
	TableName    string
	TableComment string
	Module       string // 目标模块，如 order
	BusinessName string // 业务名，如 order_item
	ClassName    string // 类名，如 OrderItem
	TemplateType TemplateType
	FrontType    FrontType
	ParentTableID int64 // 主子表：子表指向主表配置行；0=无
	Remark       string
	HasTenant    bool // 含 tenant_id 列
	Columns      []Column
}

func (t *Table) Validate() error {
	if t == nil {
		return fmt.Errorf("%w: nil table", ErrTableInvalid)
	}
	if t.TableName == "" {
		return fmt.Errorf("%w: empty table name", ErrTableInvalid)
	}
	if t.Module == "" {
		return fmt.Errorf("%w: %s: empty module", ErrTableInvalid, t.TableName)
	}
	if t.ClassName == "" {
		return fmt.Errorf("%w: %s: empty class name", ErrTableInvalid, t.TableName)
	}
	if !t.TemplateType.Valid() {
		return fmt.Errorf("%w: %s: template type %d", ErrTableInvalid, t.TableName, t.TemplateType)
	}
	if !t.FrontType.Valid() {
		return fmt.Errorf("%w: %s: front type %d", ErrTableInvalid, t.TableName, t.FrontType)
	}
	if len(t.Columns) == 0 {
		return fmt.Errorf("%w: %s: no columns", ErrColumnInvalid, t.TableName)
	}
	seen := make(map[string]bool, len(t.Columns))
	for _, c := range t.Columns {
		if c.Name == "" || c.GoName == "" || c.GoType == "" {
			return fmt.Errorf("%w: %s: column missing name/gotype: %+v", ErrColumnInvalid, t.TableName, c)
		}
		if seen[c.GoName] {
			return fmt.Errorf("%w: %s: duplicate go name %s", ErrColumnInvalid, t.TableName, c.GoName)
		}
		seen[c.GoName] = true
	}
	return nil
}
```

```go
// pkg/codegen/model/file.go
package model

// GeneratedFile 是引擎输出的单个产物。Path 为仓库根相对路径（POSIX 分隔）。
type GeneratedFile struct {
	Path    string
	Content []byte
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./pkg/codegen/model/ -v`
Expected: PASS（全部 4 个测试）

- [ ] **Step 5: 提交**

```bash
git add pkg/codegen/model/
git commit -m "feat(codegen): add model domain types, enums and sentinel errors"
```

---

### Task 2: naming 命名工具

**Files:**
- Create: `pkg/codegen/naming/naming.go`
- Test: `pkg/codegen/naming/naming_test.go`

**Interfaces:**
- Consumes: 无（纯函数，仅标准库 + jinzhu/inflection）。
- Produces（builder/template/CLI 依赖）:
  - `naming.EntityName(table string) string` — `"order_items"→"OrderItem"`, `"addresses"→"Address"`
  - `naming.ToGoName(col string) string` — `"user_id"→"UserID"`, `"identity"→"Identity"`
  - `naming.Snake(s string) string` — `"OrderItem"→"order_item"`, `"UserID"→"user_id"`
  - `naming.Plural(entity string) string` — `"OrderItem"→"order_items"`, `"Address"→"addresses"`
  - `naming.IsSensitive(col string) bool`
  - `naming.JSONName(col string) string` — 敏感字段返回 `"-"`

- [ ] **Step 1: 写失败测试**

```go
// pkg/codegen/naming/naming_test.go
package naming_test

import (
	"testing"

	"alexGo-cloud/pkg/codegen/naming"
)

func TestEntityName(t *testing.T) {
	cases := map[string]string{
		"order_items":  "OrderItem",
		"orders":       "Order",
		"order":        "Order",
		"addresses":    "Address",
		"categories":   "Category",
		"user_infos":   "UserInfo",
		"":             "",
	}
	for in, want := range cases {
		if got := naming.EntityName(in); got != want {
			t.Errorf("EntityName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestToGoName(t *testing.T) {
	cases := map[string]string{
		"user_id":     "UserID",
		"identity":    "Identity", // 回归：不得出现 "IDentity"
		"order_no":    "OrderNo",
		"created_at":  "CreatedAt",
		"url":         "URL",
		"json_data":   "JSONData",
		"api_key":     "APIKey",
		"":            "",
	}
	for in, want := range cases {
		if got := naming.ToGoName(in); got != want {
			t.Errorf("ToGoName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSnake(t *testing.T) {
	cases := map[string]string{
		"OrderItem": "order_item",
		"UserID":    "user_id",
		"Order":     "order",
		"HTTPRequest": "http_request",
		"":          "",
	}
	for in, want := range cases {
		if got := naming.Snake(in); got != want {
			t.Errorf("Snake(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPlural(t *testing.T) {
	cases := map[string]string{
		"Order":     "orders",
		"OrderItem": "order_items",
		"Address":   "addresses",
	}
	for in, want := range cases {
		if got := naming.Plural(in); got != want {
			t.Errorf("Plural(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestJSONName_Sensitive(t *testing.T) {
	if got := naming.JSONName("password"); got != "-" {
		t.Errorf("JSONName(password) = %q, want -", got)
	}
	if got := naming.JSONName("api_token"); got != "-" {
		t.Errorf("JSONName(api_token) = %q, want -", got)
	}
	if got := naming.JSONName("client_secret"); got != "-" {
		t.Errorf("JSONName(client_secret) = %q, want -", got)
	}
	if got := naming.JSONName("name"); got != "name" {
		t.Errorf("JSONName(name) = %q, want name", got)
	}
	if naming.IsSensitive("user_name") || !naming.IsSensitive("access_token") {
		t.Error("IsSensitive wrong")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./pkg/codegen/naming/ -v`
Expected: FAIL（包不存在）

- [ ] **Step 3: 最小实现**

```go
// pkg/codegen/naming/naming.go
package naming

import (
	"regexp"
	"strings"

	"github.com/jinzhu/inflection"
)

var (
	nonWord = regexp.MustCompile(`[^a-zA-Z0-9]+`)
	// camelPat 切驼峰：先匹配整段缩写（后不跟小写，如 HTTP），再匹配"大写+小写"单词（如 Request）。
	// 不能用 `[A-Z]+(?:[a-z]+|[0-9]+)?`——贪心会把 HTTPRequest 合成一坨。
	camelPat = regexp.MustCompile(`[A-Z]+(?![a-z])|[A-Z][a-z]*|[a-z]+|[0-9]+`)
)

// acronym 表内缩写保持全大写；表外单词走首字母大写。
var acronym = map[string]string{
	"id": "ID", "url": "URL", "json": "JSON", "api": "API",
	"http": "HTTP", "sql": "SQL", "xml": "XML", "uid": "UID",
}

// EntityName 表名 → 类名：按 _ 切词，末词单数化，再驼峰。
// order_items → OrderItem；addresses → Address（旧 dbgen 的 TrimSuffix "s" 会错成 Addres）。
func EntityName(table string) string {
	if table == "" {
		return ""
	}
	parts := nonWord.Split(strings.ToLower(table), -1)
	out := make([]string, 0, len(parts))
	for i, p := range parts {
		if p == "" {
			continue
		}
		if i == len(parts)-1 {
			p = inflection.Singular(p)
		}
		out = append(out, capitalize(p))
	}
	return strings.Join(out, "")
}

// ToGoName 列名 → Go 字段名：user_id → UserID；identity → Identity。
func ToGoName(col string) string {
	if col == "" {
		return ""
	}
	parts := nonWord.Split(col, -1)
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		low := strings.ToLower(p)
		if a, ok := acronym[low]; ok {
			b.WriteString(a)
			continue
		}
		b.WriteString(capitalize(low))
	}
	return b.String()
}

// Snake 驼峰 → 下蛇：OrderItem → order_item；UserID → user_id。
func Snake(s string) string {
	if s == "" {
		return ""
	}
	parts := camelPat.FindAllString(s, -1)
	for i, p := range parts {
		parts[i] = strings.ToLower(p)
	}
	return strings.Join(parts, "_")
}

// Plural 实体名 → 路径用复数下蛇：OrderItem → order_items。
func Plural(entity string) string {
	if entity == "" {
		return ""
	}
	return inflection.Plural(Snake(entity))
}

func IsSensitive(col string) bool {
	low := strings.ToLower(col)
	return strings.Contains(low, "password") || strings.Contains(low, "secret") || strings.Contains(low, "token")
}

// JSONName 列名 → json tag；敏感字段返回 "-"（不输出到 JSON）。
func JSONName(col string) string {
	if IsSensitive(col) {
		return "-"
	}
	return col
}

func capitalize(s string) string {
	return strings.ToUpper(s[:1]) + s[1:]
}
```

- [ ] **Step 4: 引入依赖并运行确认通过**

Run:
```bash
go get github.com/jinzhu/inflection@v1.0.0
go mod tidy
go test ./pkg/codegen/naming/ -v
```
Expected: PASS。若 `Plural("Address")` 实测不是 `addresses`，以 inflection 实际行为为准修正测试期望值并在提交信息注明（jinzhu 规则集对 `address` 类词尾需实测确认）。

- [ ] **Step 5: 提交**

```bash
git add pkg/codegen/naming/ go.mod go.sum
git commit -m "feat(codegen): add naming utils (entity/go-name/snake/plural, sensitive json)"
```

---

### Task 3: metadata 元数据读取

**Files:**
- Create: `pkg/codegen/metadata/reader.go`
- Create: `pkg/codegen/metadata/mysql.go`
- Test: `pkg/codegen/metadata/mysql_integration_test.go`（build tag `integration`）

**Interfaces:**
- Consumes: `model.ErrMetadataUnreachable`、`model.ErrTableNotFound`。
- Produces（builder/CLI 依赖，签名精确）:
  - `metadata.ColumnMeta{Name, DataType, ColumnType, Comment, Key, Extra string; Nullable bool; Ordinal int}`
  - `metadata.TableMeta{Schema, Name, Comment string; Columns []ColumnMeta}`
  - `metadata.MetadataReader interface { ListTables(ctx context.Context) ([]string, error); ReadTable(ctx context.Context, table string) (*TableMeta, error) }`
  - `metadata.NewMySQLReader(db *sql.DB, schema string) *MySQLReader`

- [ ] **Step 1: 写失败的集成测试**

```go
// pkg/codegen/metadata/mysql_integration_test.go
//go:build integration

package metadata_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"

	"github.com/go-sql-driver/mysql"

	"alexGo-cloud/pkg/codegen/metadata"
	"alexGo-cloud/pkg/codegen/model"
)

// TestMySQLReader_RealDB 需要真实 MySQL：DB_DSN 未设置或连不上时跳过。
func TestMySQLReader_RealDB(t *testing.T) {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		t.Skip("DB_DSN not set; skip integration test")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("mysql unreachable: %v", err)
	}

	cfg, err := mysql.ParseDSN(dsn)
	if err != nil || cfg.DBName == "" {
		t.Fatalf("cannot parse db name from dsn: %v", err)
	}

	reader := metadata.NewMySQLReader(db, cfg.DBName)

	tables, err := reader.ListTables(ctx)
	if err != nil {
		t.Fatalf("ListTables: %v", err)
	}
	if len(tables) == 0 {
		t.Fatal("ListTables returned 0 tables")
	}

	meta, err := reader.ReadTable(ctx, tables[0])
	if err != nil {
		t.Fatalf("ReadTable(%s): %v", tables[0], err)
	}
	if meta.Name != tables[0] || len(meta.Columns) == 0 {
		t.Fatalf("meta = %+v, want table %s with columns", meta, tables[0])
	}

	if _, err := reader.ReadTable(ctx, "definitely_not_a_table_xyz"); !errors.Is(err, model.ErrTableNotFound) {
		t.Fatalf("err = %v, want ErrTableNotFound", err)
	}
}
```

- [ ] **Step 2: 运行确认失败（需本地 MySQL）**

Run: `DB_DSN='root:root@tcp(127.0.0.1:3306)/alexgo' go test -tags integration ./pkg/codegen/metadata/ -v`
Expected: FAIL（`metadata.NewMySQLReader` 未定义）。无可用 MySQL 时该测试 SKIP——在有 DB 的环境执行本步。

- [ ] **Step 3: 最小实现**

```go
// pkg/codegen/metadata/reader.go
package metadata

import "context"

// ColumnMeta 是 information_schema.columns 的原始快照（不含任何推导）。
type ColumnMeta struct {
	Name       string
	DataType   string // varchar / bigint / datetime ...
	ColumnType string // varchar(64) unsigned / tinyint(1) ...
	Comment    string
	Key        string // PRI / UNI / MUL / ""
	Extra      string // auto_increment / ...
	Nullable   bool
	Ordinal    int
}

// TableMeta 是表级原始快照。
type TableMeta struct {
	Schema  string
	Name    string
	Comment string
	Columns []ColumnMeta
}

// MetadataReader 抽象库表元数据读取；M1 仅 MySQL 实现，接口保留扩展位。
type MetadataReader interface {
	ListTables(ctx context.Context) ([]string, error)
	ReadTable(ctx context.Context, table string) (*TableMeta, error)
}
```

```go
// pkg/codegen/metadata/mysql.go
package metadata

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"alexGo-cloud/pkg/codegen/model"
)

type MySQLReader struct {
	db     *sql.DB
	schema string
}

func NewMySQLReader(db *sql.DB, schema string) *MySQLReader {
	return &MySQLReader{db: db, schema: schema}
}

func (r *MySQLReader) ListTables(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT table_name
FROM information_schema.tables
WHERE table_schema = ? AND table_type = 'BASE TABLE'
ORDER BY table_name`, r.schema)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrMetadataUnreachable, err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("%w: %v", model.ErrMetadataUnreachable, err)
		}
		out = append(out, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrMetadataUnreachable, err)
	}
	return out, nil
}

func (r *MySQLReader) ReadTable(ctx context.Context, table string) (*TableMeta, error) {
	var comment string
	err := r.db.QueryRowContext(ctx, `
SELECT table_comment
FROM information_schema.tables
WHERE table_schema = ? AND table_name = ?`, r.schema, table).Scan(&comment)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%w: %s.%s", model.ErrTableNotFound, r.schema, table)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrMetadataUnreachable, err)
	}

	rows, err := r.db.QueryContext(ctx, `
SELECT column_name, data_type, column_type, column_comment, column_key, extra, is_nullable, ordinal_position
FROM information_schema.columns
WHERE table_schema = ? AND table_name = ?
ORDER BY ordinal_position`, r.schema, table)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrMetadataUnreachable, err)
	}
	defer rows.Close()

	meta := &TableMeta{Schema: r.schema, Name: table, Comment: comment}
	for rows.Next() {
		var c ColumnMeta
		var isNullable string
		if err := rows.Scan(&c.Name, &c.DataType, &c.ColumnType, &c.Comment, &c.Key, &c.Extra, &isNullable, &c.Ordinal); err != nil {
			return nil, fmt.Errorf("%w: %v", model.ErrMetadataUnreachable, err)
		}
		c.Nullable = strings.EqualFold(isNullable, "YES")
		c.Comment = strings.Join(strings.Fields(c.Comment), " ") // 注释压平单行
		meta.Columns = append(meta.Columns, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrMetadataUnreachable, err)
	}
	if len(meta.Columns) == 0 {
		return nil, fmt.Errorf("%w: %s has no columns", model.ErrTableNotFound, table)
	}
	return meta, nil
}
```

- [ ] **Step 4: 集成测试通过 + 全量编译**

Run:
```bash
DB_DSN='<本地 DSN>' go test -tags integration ./pkg/codegen/metadata/ -v
go build ./...
```
Expected: PASS（非 skip）

- [ ] **Step 5: 提交**

```bash
git add pkg/codegen/metadata/
git commit -m "feat(codegen): metadata reader interface + mysql information_schema impl"
```

---

### Task 4: typing 类型与控件映射

**Files:**
- Create: `pkg/codegen/builder/typing.go`
- Test: `pkg/codegen/builder/typing_test.go`

**Interfaces:**
- Consumes: `metadata.ColumnMeta`（Task 3）、`model.HTMLType`/`model.ErrTypeMappingUnknown`（Task 1）。
- Produces:
  - `builder.MySQLTypeToGo(c metadata.ColumnMeta) (string, error)` — 返回 Go 类型；未知类型 → 包装 `model.ErrTypeMappingUnknown`
  - `builder.MySQLTypeToHTML(c metadata.ColumnMeta, goType string) model.HTMLType`
  - `builder.BuildGormTag(c metadata.ColumnMeta) string`

- [ ] **Step 1: 写失败测试**

```go
// pkg/codegen/builder/typing_test.go
package builder_test

import (
	"errors"
	"testing"

	"alexGo-cloud/pkg/codegen/builder"
	"alexGo-cloud/pkg/codegen/metadata"
	"alexGo-cloud/pkg/codegen/model"
)

func col(dataType, columnType string, nullable bool) metadata.ColumnMeta {
	return metadata.ColumnMeta{Name: "c", DataType: dataType, ColumnType: columnType, Nullable: nullable}
}

func TestMySQLTypeToGo(t *testing.T) {
	cases := []struct {
		name     string
		col      metadata.ColumnMeta
		want     string
		wantErr  bool
	}{
		{"bigint unsigned", col("bigint", "bigint unsigned", false), "uint64", false},
		{"bigint", col("bigint", "bigint", false), "int64", false},
		{"int", col("int", "int", false), "int", false},
		{"int nullable", col("int", "int", true), "*int", false},
		{"varchar", col("varchar", "varchar(64)", false), "string", false},
		{"varchar nullable stays string", col("varchar", "varchar(64)", true), "string", false},
		{"decimal", col("decimal", "decimal(10,2)", false), "string", false},
		{"datetime", col("datetime", "datetime", false), "time.Time", false},
		{"datetime nullable", col("datetime", "datetime", true), "*time.Time", false},
		{"timestamp nullable", col("timestamp", "timestamp", true), "*time.Time", false},
		{"json", col("json", "json", false), "json.RawMessage", false},
		{"blob nullable", col("blob", "blob", true), "[]byte", false},
		{"enum", col("enum", "enum('a','b')", false), "string", false},
		{"set", col("set", "set('a')", false), "string", false},
		{"geometry", col("geometry", "geometry", false), "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := builder.MySQLTypeToGo(c.col)
			if c.wantErr {
				if !errors.Is(err, model.ErrTypeMappingUnknown) {
					t.Fatalf("err = %v, want ErrTypeMappingUnknown", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestMySQLTypeToHTML(t *testing.T) {
	cases := []struct {
		name  string
		col   metadata.ColumnMeta
		goTyp string
		want  model.HTMLType
	}{
		{"varchar → Input", col("varchar", "varchar(64)", false), "string", model.HTMLInput},
		{"text → Textarea", col("text", "text", false), "string", model.HTMLTextarea},
		{"tinyint(1) → Switch", col("tinyint", "tinyint(1)", false), "int", model.HTMLSwitch},
		{"tinyint(4) → InputNumber", col("tinyint", "tinyint(4)", false), "int", model.HTMLInputNumber},
		{"int → InputNumber", col("int", "int", false), "int", model.HTMLInputNumber},
		{"nullable int → InputNumber", col("int", "int", true), "*int", model.HTMLInputNumber},
		{"decimal → InputNumber", col("decimal", "decimal(10,2)", false), "string", model.HTMLInputNumber},
		{"datetime → DatePicker", col("datetime", "datetime", false), "time.Time", model.HTMLDatePicker},
		{"json → Textarea", col("json", "json", false), "json.RawMessage", model.HTMLTextarea},
	}
	for _, c := range cases {
		if got := builder.MySQLTypeToHTML(c.col, c.goTyp); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}

func TestBuildGormTag(t *testing.T) {
	pk := col("bigint", "bigint unsigned", false)
	pk.Name = "id"
	pk.Key = "PRI"
	pk.Extra = "auto_increment"
	if got, want := builder.BuildGormTag(pk), "column:id;primaryKey;autoIncrement"; got != want {
		t.Errorf("BuildGormTag(pk) = %q, want %q", got, want)
	}
plain := col("varchar", "varchar(64)", false)
	plain.Name = "name"
	if got, want := builder.BuildGormTag(plain), "column:name"; got != want {
		t.Errorf("BuildGormTag(name) = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./pkg/codegen/builder/ -run 'Type|Gorm' -v`
Expected: FAIL（`builder.MySQLTypeToGo` 未定义；`metadata` 包由 Task 3 已交付，按任务顺序执行即可）

- [ ] **Step 3: 最小实现**

```go
// pkg/codegen/builder/typing.go
package builder

import (
	"fmt"
	"strings"

	"alexGo-cloud/pkg/codegen/metadata"
	"alexGo-cloud/pkg/codegen/model"
)

// MySQLTypeToGo 数据库类型 → Go 类型（可空值类型加 *，string/[]byte 除外）。
func MySQLTypeToGo(c metadata.ColumnMeta) (string, error) {
	ct := strings.ToLower(c.ColumnType)
	dt := strings.ToLower(c.DataType)
	unsigned := strings.Contains(ct, "unsigned")

	var base string
	switch dt {
	case "bigint":
		if unsigned {
			base = "uint64"
		} else {
			base = "int64"
		}
	case "int", "integer", "mediumint":
		if unsigned {
			base = "uint32"
		} else {
			base = "int"
		}
	case "smallint", "tinyint":
		if unsigned {
			base = "uint16"
		} else {
			base = "int"
		}
	case "float", "double":
		base = "float64"
	case "decimal", "numeric":
		base = "string"
	case "char", "varchar", "text", "mediumtext", "longtext", "tinytext",
		"enum", "set", "year":
		base = "string"
	case "json":
		base = "json.RawMessage"
	case "datetime", "timestamp", "date", "time":
		base = "time.Time"
	case "blob", "tinyblob", "mediumblob", "longblob", "binary", "varbinary":
		base = "[]byte"
	default:
		return "", fmt.Errorf("%w: %s (data_type=%s)", model.ErrTypeMappingUnknown, c.ColumnType, c.DataType)
	}

	if c.Nullable && needsPointer(base) {
		return "*" + base, nil
	}
	return base, nil
}

func needsPointer(base string) bool {
	switch base {
	case "string", "[]byte", "json.RawMessage":
		return false
	}
	return true
}

// MySQLTypeToHTML 数据库类型 + Go 类型 → 前端控件类型。
func MySQLTypeToHTML(c metadata.ColumnMeta, goType string) model.HTMLType {
	dt := strings.ToLower(c.DataType)
	ct := strings.ToLower(c.ColumnType)
	switch {
	case strings.Contains(goType, "time.Time"):
		return model.HTMLDatePicker
	case dt == "json":
		return model.HTMLTextarea
	case dt == "text" || dt == "mediumtext" || dt == "longtext" || dt == "tinytext":
		return model.HTMLTextarea
	case dt == "tinyint" && strings.HasPrefix(ct, "tinyint(1)"):
		return model.HTMLSwitch
	case dt == "decimal" || dt == "numeric" || dt == "float" || dt == "double":
		return model.HTMLInputNumber
	case dt == "int" || dt == "integer" || dt == "bigint" ||
		dt == "smallint" || dt == "tinyint" || dt == "mediumint":
		return model.HTMLInputNumber
	default:
		return model.HTMLInput
	}
}

// BuildGormTag 生成 gorm tag 内容（不含引号）。
func BuildGormTag(c metadata.ColumnMeta) string {
	parts := []string{"column:" + c.Name}
	if c.Key == "PRI" {
		parts = append(parts, "primaryKey")
	}
	if strings.Contains(strings.ToLower(c.Extra), "auto_increment") {
		parts = append(parts, "autoIncrement")
	}
	return strings.Join(parts, ";")
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./pkg/codegen/builder/ -run 'Type|Gorm' -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add pkg/codegen/builder/typing.go pkg/codegen/builder/typing_test.go
git commit -m "feat(codegen): mysql type→go/html mapping and gorm tag builder"
```

---

### Task 5: builder.Build 元数据 → 领域模型

**Files:**
- Create: `pkg/codegen/builder/builder.go`
- Test: `pkg/codegen/builder/builder_test.go`

**Interfaces:**
- Consumes: `metadata.TableMeta`、`naming.*`、`MySQLTypeToGo/ToHTML/BuildGormTag`、`model.*`。
- Produces（CLI/后续导入逻辑依赖）:
  - `builder.Options{Module string; TemplateType model.TemplateType; FrontType model.FrontType}`（零值分别默认 Single / Vben5Antd）
  - `builder.Build(meta *metadata.TableMeta, opts Options) (*model.Table, error)`

- [ ] **Step 1: 写失败测试**

```go
// pkg/codegen/builder/builder_test.go
package builder_test

import (
	"errors"
	"testing"

	"alexGo-cloud/pkg/codegen/builder"
	"alexGo-cloud/pkg/codegen/metadata"
	"alexGo-cloud/pkg/codegen/model"
)

func fixtureMeta() *metadata.TableMeta {
	return &metadata.TableMeta{
		Schema:  "alexgo",
		Name:    "order_items",
		Comment: "订单项",
		Columns: []metadata.ColumnMeta{
			{Name: "id", DataType: "bigint", ColumnType: "bigint unsigned", Key: "PRI", Extra: "auto_increment", Ordinal: 1},
			{Name: "name", DataType: "varchar", ColumnType: "varchar(64)", Comment: "名称", Ordinal: 2},
			{Name: "price", DataType: "decimal", ColumnType: "decimal(10,2)", Comment: "价格", Ordinal: 3},
			{Name: "status", DataType: "tinyint", ColumnType: "tinyint(1)", Comment: "状态", Ordinal: 4},
			{Name: "remark", DataType: "varchar", ColumnType: "varchar(255)", Nullable: true, Comment: "备注", Ordinal: 5},
			{Name: "password", DataType: "varchar", ColumnType: "varchar(128)", Comment: "密码", Ordinal: 6},
			{Name: "created_at", DataType: "datetime", ColumnType: "datetime", Comment: "创建时间", Ordinal: 7},
			{Name: "tenant_id", DataType: "bigint", ColumnType: "bigint unsigned", Comment: "租户", Ordinal: 8},
		},
	}
}

func TestBuild_Defaults(t *testing.T) {
	tbl, err := builder.Build(fixtureMeta(), builder.Options{Module: "order"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if tbl.ClassName != "OrderItem" {
		t.Errorf("ClassName = %q, want OrderItem", tbl.ClassName)
	}
	if tbl.BusinessName != "order_item" {
		t.Errorf("BusinessName = %q, want order_item", tbl.BusinessName)
	}
	if tbl.TemplateType != model.TemplateTypeSingle || tbl.FrontType != model.FrontTypeVben5Antd {
		t.Errorf("defaults = %v/%v", tbl.TemplateType, tbl.FrontType)
	}
	if !tbl.HasTenant {
		t.Error("HasTenant = false, want true (tenant_id 列)")
	}
	if err := tbl.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}

	byName := map[string]model.Column{}
	for _, c := range tbl.Columns {
		byName[c.Name] = c
	}
	id := byName["id"]
	if !id.IsPK || !id.AutoIncrement || id.FormEnable || id.ListRequired {
		t.Errorf("id column = %+v", id)
	}
	if id.GoType != "uint64" {
		t.Errorf("id.GoType = %q, want uint64", id.GoType)
	}
	remark := byName["remark"]
	if remark.GoType != "string" || remark.FormRequired || remark.ListRequired {
		t.Errorf("remark = %+v", remark)
	}
	if remark.QueryOp != "like" || !remark.ListEnable || !remark.FormEnable || !remark.QueryEnable {
		t.Errorf("remark switches = %+v", remark)
	}
	created := byName["created_at"]
	if created.GoType != "time.Time" || created.QueryOp != "between" {
		t.Errorf("created_at = %+v", created)
	}
	if byName["password"].JSONName != "-" {
		t.Errorf("password JSONName = %q, want -", byName["password"].JSONName)
	}
	if byName["status"].HTMLType != model.HTMLSwitch {
		t.Errorf("status HTMLType = %s, want Switch", byName["status"].HTMLType)
	}
	if tbl.Columns[0].SortOrder != 0 || tbl.Columns[7].SortOrder != 7 {
		t.Errorf("SortOrder wrong: %d..%d", tbl.Columns[0].SortOrder, tbl.Columns[7].SortOrder)
	}
}

func TestBuild_CommentFlattened(t *testing.T) {
	meta := fixtureMeta()
	meta.Columns[1].Comment = "第一行\n第二行"
	tbl, err := builder.Build(meta, builder.Options{Module: "order"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := tbl.Columns[1].Comment; got != "第一行 第二行" {
		t.Errorf("Comment = %q, want flattened single line", got)
	}
}

func TestBuild_UnknownType(t *testing.T) {
	meta := fixtureMeta()
	meta.Columns[1].DataType = "geometry"
	meta.Columns[1].ColumnType = "geometry"
	_, err := builder.Build(meta, builder.Options{Module: "order"})
	if !errors.Is(err, model.ErrTypeMappingUnknown) {
		t.Fatalf("err = %v, want ErrTypeMappingUnknown", err)
	}
}

func TestBuild_EmptyMeta(t *testing.T) {
	if _, err := builder.Build(&metadata.TableMeta{Name: "x"}, builder.Options{Module: "order"}); !errors.Is(err, model.ErrColumnInvalid) {
		t.Fatalf("err = %v, want ErrColumnInvalid", err)
	}
	if _, err := builder.Build(nil, builder.Options{Module: "order"}); err == nil {
		t.Fatal("nil meta should error")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./pkg/codegen/builder/ -run Build -v`
Expected: FAIL（`builder.Build` 未定义）

- [ ] **Step 3: 最小实现**

```go
// pkg/codegen/builder/builder.go
package builder

import (
	"fmt"

	"alexGo-cloud/pkg/codegen/metadata"
	"alexGo-cloud/pkg/codegen/model"
	"alexGo-cloud/pkg/codegen/naming"
)

type Options struct {
	Module       string
	TemplateType model.TemplateType // 0 → Single
	FrontType    model.FrontType    // 0 → Vben5Antd
}

func Build(meta *metadata.TableMeta, opts Options) (*model.Table, error) {
	if meta == nil {
		return nil, fmt.Errorf("%w: nil meta", model.ErrTableInvalid)
	}
	if len(meta.Columns) == 0 {
		return nil, fmt.Errorf("%w: %s: no columns", model.ErrColumnInvalid, meta.Name)
	}

	tt := opts.TemplateType
	if tt == 0 {
		tt = model.TemplateTypeSingle
	}
	ft := opts.FrontType
	if ft == 0 {
		ft = model.FrontTypeVben5Antd
	}

	entity := naming.EntityName(meta.Name)
	tbl := &model.Table{
		TableName:    meta.Name,
		TableComment: meta.Comment,
		Module:       opts.Module,
		BusinessName: naming.Snake(entity),
		ClassName:    entity,
		TemplateType: tt,
		FrontType:    ft,
	}

	for i, c := range meta.Columns {
		goType, err := MySQLTypeToGo(c)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", meta.Name, c.Name, err)
		}
		isPK := c.Key == "PRI"
		auto := containsAutoIncrement(c.Extra)
		if c.Name == "tenant_id" {
			tbl.HasTenant = true
		}
		required := !c.Nullable && !auto
		tbl.Columns = append(tbl.Columns, model.Column{
			Name:          c.Name,
			Comment:       c.Comment,
			GoName:        naming.ToGoName(c.Name),
			GoType:        goType,
			JSONName:      naming.JSONName(c.Name),
			GormTag:       BuildGormTag(c),
			HTMLType:      MySQLTypeToHTML(c, goType),
			IsPK:          isPK,
			AutoIncrement: auto,
			Nullable:      c.Nullable,
			ListEnable:    true,
			FormEnable:    !(isPK && auto),
			QueryEnable:   true,
			QueryOp:       queryOp(goType),
			ListRequired:  required,
			FormRequired:  required && !isPK,
			SortOrder:     i,
		})
	}

	if err := tbl.Validate(); err != nil {
		return nil, err
	}
	return tbl, nil
}

func queryOp(goType string) string {
	switch {
	case strings.Contains(goType, "time.Time"):
		return "between"
	case strings.Contains(goType, "string"):
		return "like"
	default:
		return "eq"
	}
}

func containsAutoIncrement(extra string) bool {
	return strings.Contains(strings.ToLower(extra), "auto_increment")
}
```

文件顶部 import 块：

```go
import (
	"fmt"
	"strings"

	"alexGo-cloud/pkg/codegen/metadata"
	"alexGo-cloud/pkg/codegen/model"
	"alexGo-cloud/pkg/codegen/naming"
)
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./pkg/codegen/builder/ -v`
Expected: PASS（Build + typing 全部）

- [ ] **Step 5: 提交**

```bash
git add pkg/codegen/builder/builder.go pkg/codegen/builder/builder_test.go
git commit -m "feat(codegen): builder converts table metadata to domain model with defaults"
```

---

### Task 6: postprocess 后处理

**Files:**
- Create: `pkg/codegen/postprocess/golang.go`
- Create: `pkg/codegen/postprocess/collide.go`
- Test: `pkg/codegen/postprocess/postprocess_test.go`

**Interfaces:**
- Consumes: `model.GeneratedFile`、`model.ErrFormat`、`model.ErrPathCollide`。
- Produces（root `generate.go` 依赖）:
  - `postprocess.FormatGo(path string, src []byte) ([]byte, error)`
  - `postprocess.CheckCollide(files []model.GeneratedFile) error`

- [ ] **Step 1: 写失败测试**

```go
// pkg/codegen/postprocess/postprocess_test.go
package postprocess_test

import (
	"errors"
	"testing"

	"alexGo-cloud/pkg/codegen/model"
	"alexGo-cloud/pkg/codegen/postprocess"
)

func TestFormatGo_Reformats(t *testing.T) {
	src := []byte("package model\ntype A struct{X int}\n")
	out, err := postprocess.FormatGo("model/a.go", src)
	if err != nil {
		t.Fatalf("FormatGo: %v", err)
	}
	want := "package model\n\ntype A struct{ X int }\n"
	if string(out) != want {
		t.Errorf("got %q, want %q", out, want)
	}
}

func TestFormatGo_InvalidSource_Fails(t *testing.T) {
	_, err := postprocess.FormatGo("model/bad.go", []byte("package {{{"))
	if !errors.Is(err, model.ErrFormat) {
		t.Fatalf("err = %v, want ErrFormat", err)
	}
}

func TestCheckCollide(t *testing.T) {
	ok := []model.GeneratedFile{
		{Path: "modules/order/model/a.go"},
		{Path: "modules/order/model/b.go"},
	}
	if err := postprocess.CheckCollide(ok); err != nil {
		t.Fatalf("CheckCollide(ok) = %v, want nil", err)
	}
	bad := []model.GeneratedFile{
		{Path: "modules/order/model/a.go"},
		{Path: "modules/order/model/a.go"},
	}
	if err := postprocess.CheckCollide(bad); !errors.Is(err, model.ErrPathCollide) {
		t.Fatalf("err = %v, want ErrPathCollide", err)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./pkg/codegen/postprocess/ -v`
Expected: FAIL（包不存在）

- [ ] **Step 3: 最小实现**

```go
// pkg/codegen/postprocess/golang.go
package postprocess

import (
	"fmt"
	"go/format"

	"alexGo-cloud/pkg/codegen/model"
)

// FormatGo 格式化 Go 产物；失败返回 ErrFormat（绝不静默放行）。
func FormatGo(path string, src []byte) ([]byte, error) {
	out, err := format.Source(src)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", model.ErrFormat, path, err)
	}
	return out, nil
}
```

```go
// pkg/codegen/postprocess/collide.go
package postprocess

import (
	"fmt"

	"alexGo-cloud/pkg/codegen/model"
)

// CheckCollide 校验输出路径无重复（同一次生成内两模板映射到同一文件）。
func CheckCollide(files []model.GeneratedFile) error {
	seen := make(map[string]bool, len(files))
	for _, f := range files {
		if seen[f.Path] {
			return fmt.Errorf("%w: duplicate path %s", model.ErrPathCollide, f.Path)
		}
		seen[f.Path] = true
	}
	return nil
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./pkg/codegen/postprocess/ -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add pkg/codegen/postprocess/
git commit -m "feat(codegen): go format (fail-loud) and output path collision check"
```

---

### Task 7: unit-test-enable 配置开关

**Files:**
- Modify: `pkg/config/config.go`（Config 结构体追加 Codegen 段）
- Modify: `pkg/config/loader.go`（SetDefault）
- Test: `pkg/config/loader_test.go`（追加测试函数）

**Interfaces:**
- Consumes: 现有 viper loader（`LoadGlobalConfig` 内 `v.SetDefault` 模式）。
- Produces: `config.Config.Codegen.UnitTestEnable bool`，yaml 键 `codegen.unit_test_enable`，默认 `true`。

- [ ] **Step 1: 写失败测试**

```go
// pkg/config/loader_test.go —— 追加到现有文件
func TestCodegenUnitTestEnable_DefaultTrue(t *testing.T) {
	cfg, err := LoadGlobalConfig()
	if err != nil {
		t.Fatalf("LoadGlobalConfig() error = %v", err)
	}
	if !cfg.Codegen.UnitTestEnable {
		t.Error("codegen.unit_test_enable default = false, want true")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./pkg/config/ -run Codegen -v`
Expected: FAIL（`cfg.Codegen` 未定义，编译错误）

- [ ] **Step 3: 最小实现**

在 `pkg/config/config.go` 的 `Config` struct 中（与其他段平级）追加：

```go
	// Codegen：代码生成器（modules/infra + tools/codegen）配置。
	Codegen struct {
		// UnitTestEnable：生成代码时是否附带单元测试骨架（默认开启）。
		UnitTestEnable bool `mapstructure:"unit_test_enable"`
	} `mapstructure:"codegen"`
```

在 `pkg/config/loader.go` 的 `SetDefault` 区块追加：

```go
	v.SetDefault("codegen.unit_test_enable", true)
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./pkg/config/ -v`
Expected: PASS（新增 + 现有 loader 测试全绿）

- [ ] **Step 5: 提交**

```bash
git add pkg/config/config.go pkg/config/loader.go pkg/config/loader_test.go
git commit -m "feat(config): add codegen.unit_test_enable switch (default true)"
```

---

### Task 8: template 绑定数据、路径映射与 FuncMap

**Files:**
- Create: `pkg/codegen/template/bind.go`
- Create: `pkg/codegen/template/paths.go`
- Create: `pkg/codegen/template/funcs.go`
- Test: `pkg/codegen/template/bind_test.go`
- Test: `pkg/codegen/template/paths_test.go`

**Interfaces:**
- Consumes: `model.Table`、`naming.*`。
- Produces（Render 与模板文件依赖）:
  - `template.Bind{Module, Entity, Snake, Plural, Business, TableName string; HasTenant bool; Imports struct{Time, JSON bool}; Columns []BindColumn}`
  - `template.BindColumn{Name, GoName, GoType, JSONName, GormTag, HTMLType, QueryOp, Comment string; IsPK, Nullable, ListEnable, FormEnable, QueryEnable, ListRequired, FormRequired bool}`
  - `template.Options{UnitTestEnable bool}`
  - `template.NewBind(t *model.Table) Bind`（导出，便于测试）
  - `template.RenderPath(pattern string, b Bind) string`
  - `template.FuncMap() texttpl.FuncMap`（funcs：`lower`/`snake`/`plural`）
  - 路径表 `serverSingleTemplates map[string]string`（包内，测试经渲染结果间接验证）

- [ ] **Step 1: 写失败测试**

```go
// pkg/codegen/template/bind_test.go
package template_test

import (
	"testing"

	"alexGo-cloud/pkg/codegen/model"
	"alexGo-cloud/pkg/codegen/template"
)

func fixtureTable() *model.Table {
	return &model.Table{
		TableName:    "order_items",
		Module:       "order",
		BusinessName: "order_item",
		ClassName:    "OrderItem",
		TemplateType: model.TemplateTypeSingle,
		FrontType:    model.FrontTypeVben5Antd,
		HasTenant:    true,
		Columns: []model.Column{
			{Name: "id", GoName: "ID", GoType: "uint64", JSONName: "id", GormTag: "column:id;primaryKey;autoIncrement", SortOrder: 0},
			{Name: "created_at", GoName: "CreatedAt", GoType: "time.Time", JSONName: "created_at", SortOrder: 1},
			{Name: "payload", GoName: "Payload", GoType: "json.RawMessage", JSONName: "payload", SortOrder: 2},
		},
	}
}

func TestNewBind(t *testing.T) {
	b := template.NewBind(fixtureTable())
	if b.Entity != "OrderItem" || b.Snake != "order_item" || b.Plural != "order_items" {
		t.Errorf("bind naming = %q/%q/%q", b.Entity, b.Snake, b.Plural)
	}
	if b.Module != "order" || b.TableName != "order_items" || !b.HasTenant {
		t.Errorf("bind = %+v", b)
	}
	if !b.Imports.Time || !b.Imports.JSON {
		t.Errorf("Imports = %+v, want Time+JSON true", b.Imports)
	}
	if len(b.Columns) != 3 || b.Columns[0].GoName != "ID" {
		t.Errorf("Columns = %+v", b.Columns)
	}
}

func TestNewBind_NoTimeNoJSON(t *testing.T) {
	tbl := fixtureTable()
	tbl.Columns = tbl.Columns[:1] // 仅 uint64
	b := template.NewBind(tbl)
	if b.Imports.Time || b.Imports.JSON {
		t.Errorf("Imports = %+v, want both false", b.Imports)
	}
}
```

```go
// pkg/codegen/template/paths_test.go
package template_test

import (
	"testing"

	"alexGo-cloud/pkg/codegen/template"
)

func TestRenderPath(t *testing.T) {
	b := template.NewBind(fixtureTable())
	got := template.RenderPath("modules/{module}/model/{snake}.go", b)
	if got != "modules/order/model/order_item.go" {
		t.Errorf("RenderPath = %q", got)
	}
}

// TestServerSinglePaths 通过导出的 ServerSingleTemplateNames + RenderPath
// 验证 7 个单表模板的输出路径全集（名字与路径一一对应）。
func TestServerSinglePaths(t *testing.T) {
	b := template.NewBind(fixtureTable())
	want := map[string]string{
		"model.go.tmpl":            "modules/order/model/order_item.go",
		"repository.go.tmpl":       "modules/order/repository/order_item.go",
		"service.go.tmpl":          "modules/order/service/order_item.go",
		"service_test.go.tmpl":     "modules/order/service/order_item_test.go",
		"controller_admin.go.tmpl": "modules/order/controller/admin/order_item.go",
		"controller_app.go.tmpl":   "modules/order/controller/app/order_item.go",
		"module_register.go.tmpl":  "modules/order/module_register_order_item.go",
	}
	names := template.ServerSingleTemplateNames()
	if len(names) != len(want) {
		t.Fatalf("template count = %d, want %d: %v", len(names), len(want), names)
	}
	for _, name := range names {
		pattern := template.ServerSingleTemplatePattern(name)
		got := template.RenderPath(pattern, b)
		short := name
		if got != want[short] {
			t.Errorf("%s path = %q, want %q", name, got, want[short])
		}
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./pkg/codegen/template/ -v`
Expected: FAIL（包不存在）

- [ ] **Step 3: 最小实现**

```go
// pkg/codegen/template/funcs.go
package template

import (
	"strings"
	texttpl "text/template"

	"alexGo-cloud/pkg/codegen/naming"
)

// FuncMap 模板可用函数。
func FuncMap() texttpl.FuncMap {
	return texttpl.FuncMap{
		"lower":  strings.ToLower,
		"snake":  naming.Snake,
		"plural": naming.Plural,
	}
}
```

```go
// pkg/codegen/template/bind.go
package template

import (
	"strings"

	"alexGo-cloud/pkg/codegen/model"
	"alexGo-cloud/pkg/codegen/naming"
)

type Options struct {
	UnitTestEnable bool
}

// Bind 是模板渲染的绑定数据（模板唯一可见的世界）。
type Bind struct {
	Module    string
	Entity    string
	Snake     string
	Plural    string
	Business  string
	TableName string
	HasTenant bool
	Imports   struct{ Time, JSON bool }
	Columns   []BindColumn
}

type BindColumn struct {
	Name         string
	Comment      string
	GoName       string
	GoType       string
	JSONName     string
	GormTag      string
	HTMLType     string
	QueryOp      string
	IsPK         bool
	Nullable     bool
	ListEnable   bool
	FormEnable   bool
	QueryEnable  bool
	ListRequired bool
	FormRequired bool
}

func NewBind(t *model.Table) Bind {
	b := Bind{
		Module:    t.Module,
		Entity:    t.ClassName,
		Snake:     naming.Snake(t.ClassName),
		Plural:    naming.Plural(t.ClassName),
		Business:  t.BusinessName,
		TableName: t.TableName,
		HasTenant: t.HasTenant,
	}
	for _, c := range t.Columns {
		if strings.Contains(c.GoType, "time.Time") {
			b.Imports.Time = true
		}
		if strings.Contains(c.GoType, "json.RawMessage") {
			b.Imports.JSON = true
		}
		b.Columns = append(b.Columns, BindColumn{
			Name: c.Name, Comment: c.Comment,
			GoName: c.GoName, GoType: c.GoType, JSONName: c.JSONName,
			GormTag: c.GormTag, HTMLType: string(c.HTMLType), QueryOp: c.QueryOp,
			IsPK: c.IsPK, Nullable: c.Nullable,
			ListEnable: c.ListEnable, FormEnable: c.FormEnable, QueryEnable: c.QueryEnable,
			ListRequired: c.ListRequired, FormRequired: c.FormRequired,
		})
	}
	return b
}
```

```go
// pkg/codegen/template/paths.go
package template

import (
	"fmt"
	"sort"
	"strings"

	"alexGo-cloud/pkg/codegen/model"
)

// serverSingleTemplates：单表后端模板族。key=embed 相对路径（templates/ 之后），value=输出路径模式。
var serverSingleTemplates = map[string]string{
	"server/go/single/model.go.tmpl":            "modules/{module}/model/{snake}.go",
	"server/go/single/repository.go.tmpl":       "modules/{module}/repository/{snake}.go",
	"server/go/single/service.go.tmpl":          "modules/{module}/service/{snake}.go",
	"server/go/single/service_test.go.tmpl":     "modules/{module}/service/{snake}_test.go",
	"server/go/single/controller_admin.go.tmpl": "modules/{module}/controller/admin/{snake}.go",
	"server/go/single/controller_app.go.tmpl":   "modules/{module}/controller/app/{snake}.go",
	"server/go/single/module_register.go.tmpl":  "modules/{module}/module_register_{snake}.go",
}

// ServerSingleTemplateNames 返回排序后的模板名列表（确定性）。
func ServerSingleTemplateNames() []string {
	names := make([]string, 0, len(serverSingleTemplates))
	for n := range serverSingleTemplates {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func ServerSingleTemplatePattern(name string) string {
	p, ok := serverSingleTemplates[name]
	if !ok {
		return ""
	}
	return p
}

func isTestTemplate(name string) bool {
	return strings.HasSuffix(name, "_test.go.tmpl")
}

// RenderPath 将模式中的占位符替换为绑定值。
func RenderPath(pattern string, b Bind) string {
	return strings.NewReplacer(
		"{module}", b.Module,
		"{snake}", b.Snake,
		"{entity}", b.Entity,
		"{business}", b.Business,
	).Replace(pattern)
}

// templateMapFor 按模板类型选择模板族；M1 仅 single。
func templateMapFor(t *model.Table) (map[string]string, error) {
	switch t.TemplateType {
	case model.TemplateTypeSingle:
		return serverSingleTemplates, nil
	default:
		return nil, fmt.Errorf("%w: template type %d (tree/main_sub 在 M5/M6 交付)", model.ErrTemplateMissing, t.TemplateType)
	}
}
```

- [ ] **Step 4: 运行确认通过（除已注明挪到 Task 10 的用例）**

Run: `go test ./pkg/codegen/template/ -v`
Expected: PASS（`TestNewBind*`、`TestRenderPath`、`TestServerSinglePaths`；树表 `ErrTemplateMissing` 行为在 Task 10 经 `Generate` 覆盖）

- [ ] **Step 5: 提交**

```bash
git add pkg/codegen/template/bind.go pkg/codegen/template/paths.go pkg/codegen/template/funcs.go pkg/codegen/template/bind_test.go pkg/codegen/template/paths_test.go
git commit -m "feat(codegen): template bind data, path mapping and func map"
```

---

### Task 9: 单表后端模板族（7 个模板）+ 解析测试

**Files:**
- Create: `pkg/codegen/template/embed.go`
- Create: `pkg/codegen/template/templates/server/go/single/model.go.tmpl`
- Create: `pkg/codegen/template/templates/server/go/single/repository.go.tmpl`
- Create: `pkg/codegen/template/templates/server/go/single/service.go.tmpl`
- Create: `pkg/codegen/template/templates/server/go/single/service_test.go.tmpl`
- Create: `pkg/codegen/template/templates/server/go/single/controller_admin.go.tmpl`
- Create: `pkg/codegen/template/templates/server/go/single/controller_app.go.tmpl`
- Create: `pkg/codegen/template/templates/server/go/single/module_register.go.tmpl`
- Test: `pkg/codegen/template/templates_test.go`

**Interfaces:**
- Consumes: `Bind`/`FuncMap`（Task 8）、旧模板风格（`scripts/templates/db_crud/*.tpl`，仅作参考，不复用文件）。
- Produces: embed FS `tplFS`（包内）+ 7 个可解析模板；产出代码风格对齐仓库手写代码（`gin.H{"data":...}`/`gin.H{"status":"ok"}`/`gin.H{"error":...}`）。

- [ ] **Step 1: 写失败的解析测试**

```go
// pkg/codegen/template/templates_test.go
package template_test

import (
	"io/fs"
	"testing"
	texttpl "text/template"

	"alexGo-cloud/pkg/codegen/template"
)

// TestTemplatesParse 解析 embedFS 内全部模板；语法错误 → 失败。
func TestTemplatesParse(t *testing.T) {
	fsys := template.TemplateFS()
	err := fs.WalkDir(fsys, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if _, perr := texttpl.New(path).Funcs(template.FuncMap()).ParseFS(fsys, path); perr != nil {
			t.Errorf("parse %s: %v", path, perr)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./pkg/codegen/template/ -run TemplatesParse -v`
Expected: FAIL（`template.TemplateFS` 未定义 / embed 文件不存在）

- [ ] **Step 3: 创建 embed 与 7 个模板**

```go
// pkg/codegen/template/embed.go
package template

import "embed"

// tplFS 内嵌全部模板（引擎在任意 cwd 下可用，摆脱旧工具对 scripts/templates 的路径依赖）。
//
//go:embed templates
var tplFS embed.FS

func TemplateFS() embed.FS { return tplFS }
```

`model.go.tmpl`（结构体 + gorm/json tag + 注释；敏感字段经 `JSONName="-"` 自动屏蔽）：

```
package model

{{- if or .Imports.Time .Imports.JSON }}
import (
{{- if .Imports.JSON }}
	"encoding/json"
{{- end }}
{{- if .Imports.Time }}
	"time"
{{- end }}
)
{{- end }}

type {{.Entity}} struct {
{{- range .Columns }}
	{{.GoName}} {{.GoType}} `gorm:"{{.GormTag}}" json:"{{.JSONName}}"`{{ if .Comment }} // {{.Comment}}{{ end }}
{{- end }}
}

func ({{.Entity}}) TableName() string { return "{{.TableName}}" }
```

`repository.go.tmpl`（对齐旧 db_crud 风格，含租户分支）：

```
package repository

import (
	"context"

	"alexGo-cloud/modules/{{.Module}}/model"
	"gorm.io/gorm"
)

type {{.Entity}}Repository interface {
	Create(ctx context.Context, entity *model.{{.Entity}}) error
	List(ctx context.Context{{ if .HasTenant }}, tenantID uint64{{ end }}) ([]*model.{{.Entity}}, error)
	GetByID(ctx context.Context{{ if .HasTenant }}, tenantID uint64{{ end }}, id uint64) (*model.{{.Entity}}, error)
	Update(ctx context.Context, entity *model.{{.Entity}}) error
	Delete(ctx context.Context{{ if .HasTenant }}, tenantID uint64{{ end }}, id uint64) error
}

type {{ lower .Entity }}Repo struct {
	db *gorm.DB
}

func New{{.Entity}}Repository(db *gorm.DB) {{.Entity}}Repository {
	return &{{ lower .Entity }}Repo{db: db}
}

func (r *{{ lower .Entity }}Repo) Create(ctx context.Context, entity *model.{{.Entity}}) error {
	return r.db.WithContext(ctx).Create(entity).Error
}

func (r *{{ lower .Entity }}Repo) List(ctx context.Context{{ if .HasTenant }}, tenantID uint64{{ end }}) ([]*model.{{.Entity}}, error) {
	var list []*model.{{.Entity}}
	q := r.db.WithContext(ctx).Model(&model.{{.Entity}}{})
	{{- if .HasTenant }}
	q = q.Where("tenant_id = ?", tenantID)
	{{- end }}
	err := q.Find(&list).Error
	return list, err
}

func (r *{{ lower .Entity }}Repo) GetByID(ctx context.Context{{ if .HasTenant }}, tenantID uint64{{ end }}, id uint64) (*model.{{.Entity}}, error) {
	var entity model.{{.Entity}}
	q := r.db.WithContext(ctx).Model(&model.{{.Entity}}{})
	{{- if .HasTenant }}
	q = q.Where("tenant_id = ?", tenantID)
	{{- end }}
	err := q.Where("id = ?", id).First(&entity).Error
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

func (r *{{ lower .Entity }}Repo) Update(ctx context.Context, entity *model.{{.Entity}}) error {
	return r.db.WithContext(ctx).Save(entity).Error
}

func (r *{{ lower .Entity }}Repo) Delete(ctx context.Context{{ if .HasTenant }}, tenantID uint64{{ end }}, id uint64) error {
	q := r.db.WithContext(ctx).Model(&model.{{.Entity}}{})
	{{- if .HasTenant }}
	q = q.Where("tenant_id = ?", tenantID)
	{{- end }}
	return q.Where("id = ?", id).Delete(&model.{{.Entity}}{}).Error
}
```

`service.go.tmpl`（对齐旧 db_crud）：

```
package service

import (
	"context"

	"alexGo-cloud/modules/{{.Module}}/model"
	"alexGo-cloud/modules/{{.Module}}/repository"
{{- if .HasTenant }}
	"alexGo-cloud/pkg/tenant"
{{- end }}
)

type {{.Entity}}Service interface {
	Create(ctx context.Context, entity *model.{{.Entity}}) error
	List(ctx context.Context) ([]*model.{{.Entity}}, error)
	GetByID(ctx context.Context, id uint64) (*model.{{.Entity}}, error)
	Update(ctx context.Context, entity *model.{{.Entity}}) error
	Delete(ctx context.Context, id uint64) error
}

type {{ lower .Entity }}Service struct {
	repo repository.{{.Entity}}Repository
}

func New{{.Entity}}Service(repo repository.{{.Entity}}Repository) {{.Entity}}Service {
	return &{{ lower .Entity }}Service{repo: repo}
}

func (s *{{ lower .Entity }}Service) Create(ctx context.Context, entity *model.{{.Entity}}) error {
	return s.repo.Create(ctx, entity)
}

func (s *{{ lower .Entity }}Service) List(ctx context.Context) ([]*model.{{.Entity}}, error) {
	{{- if .HasTenant }}
	return s.repo.List(ctx, tenant.TenantIDFromContext(ctx))
	{{- else }}
	return s.repo.List(ctx)
	{{- end }}
}

func (s *{{ lower .Entity }}Service) GetByID(ctx context.Context, id uint64) (*model.{{.Entity}}, error) {
	{{- if .HasTenant }}
	return s.repo.GetByID(ctx, tenant.TenantIDFromContext(ctx), id)
	{{- else }}
	return s.repo.GetByID(ctx, id)
	{{- end }}
}

func (s *{{ lower .Entity }}Service) Update(ctx context.Context, entity *model.{{.Entity}}) error {
	return s.repo.Update(ctx, entity)
}

func (s *{{ lower .Entity }}Service) Delete(ctx context.Context, id uint64) error {
	{{- if .HasTenant }}
	return s.repo.Delete(ctx, tenant.TenantIDFromContext(ctx), id)
	{{- else }}
	return s.repo.Delete(ctx, id)
	{{- end }}
}
```

`service_test.go.tmpl`（手写 fake repo，全 CRUD 走查；**假定实体含 `id` 主键字段**，仓库表约定如此）：

```
package service

import (
	"context"
	"errors"
	"testing"

	"alexGo-cloud/modules/{{.Module}}/model"
)

type fake{{.Entity}}Repo struct {
	data map[uint64]*model.{{.Entity}}
	seq  uint64
}

var errRecordNotFound = errors.New("record not found")

func (f *fake{{.Entity}}Repo) Create(_ context.Context, e *model.{{.Entity}}) error {
	f.seq++
	e.ID = f.seq
	f.data[e.ID] = e
	return nil
}

func (f *fake{{.Entity}}Repo) List(_ context.Context{{ if .HasTenant }}, _ uint64{{ end }}) ([]*model.{{.Entity}}, error) {
	out := make([]*model.{{.Entity}}, 0, len(f.data))
	for _, e := range f.data {
		out = append(out, e)
	}
	return out, nil
}

func (f *fake{{.Entity}}Repo) GetByID(_ context.Context{{ if .HasTenant }}, _ uint64{{ end }}, id uint64) (*model.{{.Entity}}, error) {
	if e, ok := f.data[id]; ok {
		return e, nil
	}
	return nil, errRecordNotFound
}

func (f *fake{{.Entity}}Repo) Update(_ context.Context, e *model.{{.Entity}}) error {
	f.data[e.ID] = e
	return nil
}

func (f *fake{{.Entity}}Repo) Delete(_ context.Context{{ if .HasTenant }}, _ uint64{{ end }}, id uint64) error {
	delete(f.data, id)
	return nil
}

func Test{{.Entity}}Service_CRUD(t *testing.T) {
	svc := New{{.Entity}}Service(&fake{{.Entity}}Repo{data: map[uint64]*model.{{.Entity}}{}})
	ctx := context.Background()

	e := &model.{{.Entity}}{}
	if err := svc.Create(ctx, e); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if e.ID == 0 {
		t.Fatal("Create should assign ID")
	}
	if _, err := svc.GetByID(ctx, e.ID); err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if err := svc.Update(ctx, e); err != nil {
		t.Fatalf("Update: %v", err)
	}
	list, err := svc.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("List: err=%v len=%d", err, len(list))
	}
	if err := svc.Delete(ctx, e.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if list, _ = svc.List(ctx); len(list) != 0 {
		t.Fatalf("after Delete len=%d", len(list))
	}
}
```

`controller_admin.go.tmpl`（完整 CRUD，对齐仓库 `gin.H` 响应风格——旧 db_crud 缺 Update/Get，此处补齐）：

```
package admin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/{{.Module}}/model"
	"alexGo-cloud/modules/{{.Module}}/service"
)

type Admin{{.Entity}}Controller struct {
	svc service.{{.Entity}}Service
}

func NewAdmin{{.Entity}}Controller(svc service.{{.Entity}}Service) *Admin{{.Entity}}Controller {
	return &Admin{{.Entity}}Controller{svc: svc}
}

func (c *Admin{{.Entity}}Controller) List(ctx *gin.Context) {
	list, err := c.svc.List(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": list})
}

func (c *Admin{{.Entity}}Controller) Get(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil || id == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	entity, err := c.svc.GetByID(ctx.Request.Context(), id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": entity})
}

func (c *Admin{{.Entity}}Controller) Create(ctx *gin.Context) {
	var req model.{{.Entity}}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := c.svc.Create(ctx.Request.Context(), &req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (c *Admin{{.Entity}}Controller) Update(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil || id == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req model.{{.Entity}}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.ID = id
	if err := c.svc.Update(ctx.Request.Context(), &req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (c *Admin{{.Entity}}Controller) Delete(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil || id == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := c.svc.Delete(ctx.Request.Context(), id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}
```

`controller_app.go.tmpl`（C 端占位，对齐旧 crud 模板）：

```
package app

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type App{{.Entity}}Controller struct{}

func NewApp{{.Entity}}Controller() *App{{.Entity}}Controller {
	return &App{{.Entity}}Controller{}
}

func (c *App{{.Entity}}Controller) Ping(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}
```

`module_register.go.tmpl`（每实体独立注册文件，不覆盖手写 module.go；聚合标记块合并属 M3）：

```
package {{.Module}}

import (
	"github.com/gin-gonic/gin"

	adminctrl "alexGo-cloud/modules/{{.Module}}/controller/admin"
)

// Register{{.Entity}}DBRoutes 注册 {{.Entity}} 的 admin 路由（生成文件，勿手改）。
// 由 module.go 手动调用接线：adminctrl := admin.New{{.Entity}}Controller(...)
func Register{{.Entity}}DBRoutes(r *gin.RouterGroup, c *adminctrl.Admin{{.Entity}}Controller) {
	g := r.Group("/admin/{{.Module}}")
	g.GET("/{{.Plural}}", c.List)
	g.GET("/{{.Plural}}/:id", c.Get)
	g.POST("/{{.Plural}}", c.Create)
	g.PUT("/{{.Plural}}/:id", c.Update)
	g.DELETE("/{{.Plural}}/:id", c.Delete)
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./pkg/codegen/template/ -v`
Expected: PASS（TemplatesParse 通过，且 Task 8 用例仍绿）

- [ ] **Step 5: 提交**

```bash
git add pkg/codegen/template/
git commit -m "feat(codegen): single-table server template family (model/repo/service/test/controllers/register)"
```

---

### Task 10: 引擎入口 Generate + golden 测试

**Files:**
- Create: `pkg/codegen/generate.go`
- Create: `pkg/codegen/template/render.go`
- Test: `pkg/codegen/generate_test.go`
- Create: `pkg/codegen/testdata/golden/*`（由 `-update` 生成后评审入库）

**Interfaces:**
- Consumes: `template.NewBind/RenderPath/templateMapFor/FuncMap/tplFS`、`postprocess.FormatGo/CheckCollide`、`model.*`。
- Produces（CLI 与 M2+ 服务依赖，签名精确）:
  - `codegen.Options{UnitTestEnable bool}`
  - `codegen.Generate(tables []*model.Table, opts Options) ([]model.GeneratedFile, error)` — 全部成功才有产物；任一错误 → `(nil, errors.Join(...))`；输出按 Path 排序。

- [ ] **Step 1: 写失败测试**

```go
// pkg/codegen/generate_test.go
package codegen_test

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"alexGo-cloud/pkg/codegen"
	"alexGo-cloud/pkg/codegen/builder"
	"alexGo-cloud/pkg/codegen/metadata"
	"alexGo-cloud/pkg/codegen/model"
)

var update = flag.Bool("update", false, "rewrite golden files")

func fixtureMeta() *metadata.TableMeta {
	return &metadata.TableMeta{
		Schema:  "alexgo",
		Name:    "codegen_demo_items",
		Comment: "codegen demo",
		Columns: []metadata.ColumnMeta{
			{Name: "id", DataType: "bigint", ColumnType: "bigint unsigned", Key: "PRI", Extra: "auto_increment", Comment: "主键", Ordinal: 1},
			{Name: "name", DataType: "varchar", ColumnType: "varchar(64)", Comment: "名称", Ordinal: 2},
			{Name: "price", DataType: "decimal", ColumnType: "decimal(10,2)", Comment: "价格", Ordinal: 3},
			{Name: "status", DataType: "tinyint", ColumnType: "tinyint(1)", Comment: "状态", Ordinal: 4},
			{Name: "remark", DataType: "varchar", ColumnType: "varchar(255)", Nullable: true, Comment: "备注", Ordinal: 5},
			{Name: "password", DataType: "varchar", ColumnType: "varchar(128)", Comment: "密码", Ordinal: 6},
			{Name: "created_at", DataType: "datetime", ColumnType: "datetime", Comment: "创建时间", Ordinal: 7},
			{Name: "tenant_id", DataType: "bigint", ColumnType: "bigint unsigned", Comment: "租户", Ordinal: 8},
		},
	}
}

func buildFixture(t *testing.T) *model.Table {
	t.Helper()
	tbl, err := builder.Build(fixtureMeta(), builder.Options{Module: "order"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return tbl
}

func TestGenerate_Golden(t *testing.T) {
	files, err := codegen.Generate([]*model.Table{buildFixture(t)}, codegen.Options{UnitTestEnable: true})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(files) != 7 {
		t.Fatalf("files = %d, want 7: %+v", len(files), pathsOf(files))
	}
	for _, f := range files {
		golden := filepath.Join("testdata", "golden", strings.ReplaceAll(f.Path, "/", "_"))
		if *update {
			if err := os.MkdirAll("testdata/golden", 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(golden, f.Content, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatalf("read golden %s: %v (run: go test ./pkg/codegen -run Golden -update)", golden, err)
		}
		if !bytes.Equal(f.Content, want) {
			t.Errorf("mismatch %s\n--- got ---\n%s\n--- want ---\n%s", f.Path, f.Content, want)
		}
	}
	// 回归钉：敏感字段必须是 json:"-"
	var modelFile []byte
	for _, f := range files {
		if strings.HasSuffix(f.Path, "/model/codegen_demo_item.go") {
			modelFile = f.Content
		}
	}
	if !bytes.Contains(modelFile, []byte(`json:"-"`)) {
		t.Error("model output missing sensitive json:\"-\"")
	}
}

func TestGenerate_NoUnitTest(t *testing.T) {
	files, err := codegen.Generate([]*model.Table{buildFixture(t)}, codegen.Options{UnitTestEnable: false})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(files) != 6 {
		t.Fatalf("files = %d, want 6 (no service_test)", len(files))
	}
	for _, f := range files {
		if strings.HasSuffix(f.Path, "_test.go") {
			t.Errorf("unexpected test file %s", f.Path)
		}
	}
}

func TestGenerate_AggregatesErrors(t *testing.T) {
	badNoCols := &model.Table{TableName: "t1", Module: "m", ClassName: "T1",
		TemplateType: model.TemplateTypeSingle, FrontType: model.FrontTypeVben5Antd}
	badType := buildFixture(t)
	badType.TemplateType = model.TemplateTypeTree // M1 未实现 → ErrTemplateMissing
	_, err := codegen.Generate([]*model.Table{badNoCols, badType}, codegen.Options{UnitTestEnable: true})
	if err == nil {
		t.Fatal("Generate should fail")
	}
	if !errors.Is(err, model.ErrColumnInvalid) {
		t.Errorf("err missing ErrColumnInvalid: %v", err)
	}
	if !errors.Is(err, model.ErrTemplateMissing) {
		t.Errorf("err missing ErrTemplateMissing: %v", err)
	}
}

func TestGenerate_Collide(t *testing.T) {
	a := buildFixture(t)
	b := buildFixture(t) // 同表两次 → 同路径
	_, err := codegen.Generate([]*model.Table{a, b}, codegen.Options{UnitTestEnable: true})
	if !errors.Is(err, model.ErrPathCollide) {
		t.Fatalf("err = %v, want ErrPathCollide", err)
	}
}

func TestGenerate_SortedOutput(t *testing.T) {
	files, err := codegen.Generate([]*model.Table{buildFixture(t)}, codegen.Options{UnitTestEnable: true})
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(files); i++ {
		if files[i-1].Path >= files[i].Path {
			t.Fatalf("unsorted: %s >= %s", files[i-1].Path, files[i].Path)
		}
	}
}

func pathsOf(files []model.GeneratedFile) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.Path
	}
	return out
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./pkg/codegen/ -v`
Expected: FAIL（`codegen.Generate` 未定义）

- [ ] **Step 3: 实现 Render 与 Generate**

```go
// pkg/codegen/template/render.go
package template

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	texttpl "text/template"

	"alexGo-cloud/pkg/codegen/model"
)

// Render 单表渲染：模板族 → 文件列表（按 Path 排序）。错误聚合返回。
func Render(t *model.Table, opts Options) ([]model.GeneratedFile, error) {
	tplMap, err := templateMapFor(t)
	if err != nil {
		return nil, err
	}
	b := NewBind(t)

	var files []model.GeneratedFile
	var errs []error
	for name, pattern := range tplMap {
		if !opts.UnitTestEnable && isTestTemplate(name) {
			continue
		}
		tpl, perr := texttpl.New(name).Funcs(FuncMap()).ParseFS(tplFS, "templates/"+name)
		if perr != nil {
			errs = append(errs, fmt.Errorf("%w: %s: %v", model.ErrRender, name, perr))
			continue
		}
		var buf bytes.Buffer
		if eerr := tpl.Execute(&buf, b); eerr != nil {
			errs = append(errs, fmt.Errorf("%w: %s: %v", model.ErrRender, name, eerr))
			continue
		}
		files = append(files, model.GeneratedFile{
			Path:    RenderPath(pattern, b),
			Content: buf.Bytes(),
		})
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}
```

```go
// pkg/codegen/generate.go
package codegen

import (
	"errors"
	"fmt"
	"sort"

	"alexGo-cloud/pkg/codegen/model"
	"alexGo-cloud/pkg/codegen/postprocess"
	"alexGo-cloud/pkg/codegen/template"
)

type Options struct {
	UnitTestEnable bool
}

// Generate 是引擎唯一入口：校验 → 渲染 → 格式化 → 碰撞检查。
// 全部成功才返回产物（按 Path 排序）；任一错误 → (nil, errors.Join(errs...))。
func Generate(tables []*model.Table, opts Options) ([]model.GeneratedFile, error) {
	if len(tables) == 0 {
		return nil, fmt.Errorf("%w: no tables to generate", model.ErrTableInvalid)
	}

	var errs []error
	var files []model.GeneratedFile
	for _, t := range tables {
		if t == nil {
			errs = append(errs, fmt.Errorf("%w: nil table", model.ErrTableInvalid))
			continue
		}
		if err := t.Validate(); err != nil {
			errs = append(errs, err)
			continue
		}
		fs, err := template.Render(t, template.Options{UnitTestEnable: opts.UnitTestEnable})
		if err != nil {
			errs = append(errs, err)
			continue
		}
		files = append(files, fs...)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	var ferrs []error
	for i, f := range files {
		out, err := postprocess.FormatGo(f.Path, f.Content)
		if err != nil {
			ferrs = append(ferrs, err)
			continue
		}
		files[i].Content = out
	}
	if len(ferrs) > 0 {
		return nil, errors.Join(ferrs...)
	}

	if err := postprocess.CheckCollide(files); err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}
```

- [ ] **Step 4: 生成 golden 并评审**

Run:
```bash
go test ./pkg/codegen/ -run Golden -update -v
go test ./pkg/codegen/ -v
```
Expected: 第一条生成 `testdata/golden/`；第二条 PASS。

**人工评审 golden 清单**（提交前逐条确认）：
- [ ] 共 7 个文件，路径为 `testdata/golden/modules_order_*`
- [ ] `model` 文件：`package model`、`type CodegenDemoItem struct`、含 `time` import、`password` 行为 `json:"-"`、`created_at` 为 `time.Time`、`remark` 为 `string`
- [ ] `repository/service` 文件：编译级语法完整、租户分支带 `tenant_id = ?` 与 `pkg/tenant` import
- [ ] `service_test` 文件：fake repo 五方法 + `TestCodegenDemoItemService_CRUD`
- [ ] `controller_admin` 文件：List/Get/Create/Update/Delete 五方法
- [ ] `module_register` 文件：5 条路由、路径 `/admin/order/codegen_demo_items`
- [ ] 所有文件均已 `go/format` 规范化

- [ ] **Step 5: 提交**

```bash
git add pkg/codegen/ pkg/codegen/template/
git commit -m "feat(codegen): Generate entrypoint with golden tests and error aggregation"
```

---

### Task 11: CLI `tools/codegen`（草稿模式）

**Files:**
- Create: `tools/codegen/main.go`
- Test: `tools/codegen/main_test.go`

**Interfaces:**
- Consumes: `config.LoadGlobalConfig`、`metadata.NewMySQLReader`、`builder.Build`、`codegen.Generate`、`model.GeneratedFile`。
- Produces:
  - CLI flags：`--module`(必填) `--tables`(逗号分隔，空=全部) `--dsn` `--schema` `--out`(默认 `.`) `--force` `--dry-run` `--no-tests`
  - `writeFiles(root string, files []model.GeneratedFile, force bool) (created, skipped int, err error)`（包内，测试直接调用）

- [ ] **Step 1: 写失败测试**

```go
// tools/codegen/main_test.go
package main

import (
	"os"
	"path/filepath"
	"testing"

	"alexGo-cloud/pkg/codegen/model"
)

func TestWriteFiles_NoOverwrite(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "modules/order/model/order.go")
	if err := os.MkdirAll(filepath.Dir(existing), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existing, []byte("ORIGINAL"), 0o644); err != nil {
		t.Fatal(err)
	}
	files := []model.GeneratedFile{
		{Path: "modules/order/model/order.go", Content: []byte("NEW")},
		{Path: "modules/order/model/fresh.go", Content: []byte("FRESH")},
	}

	created, skipped, err := writeFiles(root, files, false)
	if err != nil {
		t.Fatalf("writeFiles: %v", err)
	}
	if created != 1 || skipped != 1 {
		t.Fatalf("created=%d skipped=%d, want 1/1", created, skipped)
	}
	got, _ := os.ReadFile(existing)
	if string(got) != "ORIGINAL" {
		t.Errorf("existing file overwritten: %q", got)
	}
	fresh, err := os.ReadFile(filepath.Join(root, "modules/order/model/fresh.go"))
	if err != nil || string(fresh) != "FRESH" {
		t.Errorf("fresh file = %q, err=%v", fresh, err)
	}
}

func TestWriteFiles_ForceOverwrites(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "a.go")
	if err := os.WriteFile(existing, []byte("OLD"), 0o644); err != nil {
		t.Fatal(err)
	}
	created, skipped, err := writeFiles(root, []model.GeneratedFile{{Path: "a.go", Content: []byte("NEW")}}, true)
	if err != nil {
		t.Fatalf("writeFiles: %v", err)
	}
	if created != 1 || skipped != 0 {
		t.Fatalf("created=%d skipped=%d, want 1/0", created, skipped)
	}
	got, _ := os.ReadFile(existing)
	if string(got) != "NEW" {
		t.Errorf("content = %q, want NEW", got)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./tools/codegen/ -v`
Expected: FAIL（`writeFiles` 未定义）

- [ ] **Step 3: 最小实现**

```go
// tools/codegen/main.go
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"alexGo-cloud/pkg/codegen"
	"alexGo-cloud/pkg/codegen/builder"
	"alexGo-cloud/pkg/codegen/metadata"
	"alexGo-cloud/pkg/codegen/model"
	"alexGo-cloud/pkg/config"
)

func main() {
	module := flag.String("module", "", "target module name (e.g. order)")
	tables := flag.String("tables", "", "comma-separated table names (empty = all in schema)")
	dsnFlag := flag.String("dsn", "", "mysql dsn (default: config database.dsn / DB_DSN)")
	schemaFlag := flag.String("schema", "", "mysql schema (default: from dsn)")
	out := flag.String("out", ".", "output root dir (repo root)")
	force := flag.Bool("force", false, "overwrite existing files")
	dryRun := flag.Bool("dry-run", false, "print planned files, do not write")
	noTests := flag.Bool("no-tests", false, "skip generating unit test skeletons")
	flag.Parse()

	if *module == "" {
		fmt.Fprintln(os.Stderr, "missing --module")
		flag.Usage()
		os.Exit(2)
	}

	cfg, _ := config.LoadGlobalConfig()
	dsn := *dsnFlag
	if dsn == "" && cfg != nil {
		dsn = cfg.Database.DSN
	}
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "missing dsn: pass --dsn or set DB_DSN or config database.dsn")
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", model.ErrMetadataUnreachable, err)
		os.Exit(1)
	}

	schema := *schemaFlag
	if schema == "" {
		schema = schemaFromDSN(dsn)
	}
	if schema == "" {
		fmt.Fprintln(os.Stderr, "cannot determine schema; pass --schema or include db name in dsn")
		os.Exit(2)
	}

	reader := metadata.NewMySQLReader(db, schema)

	tableList, err := parseTables(*tables)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if len(tableList) == 0 {
		tableList, err = reader.ListTables(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	}

	// 读取 + 构建：聚合所有错误，任一失败即不生成。
	var errs []error
	var tablesToGen []*model.Table
	for _, name := range tableList {
		meta, merr := reader.ReadTable(ctx, name)
		if merr != nil {
			errs = append(errs, merr)
			continue
		}
		tbl, berr := builder.Build(meta, builder.Options{Module: *module})
		if berr != nil {
			errs = append(errs, berr)
			continue
		}
		tablesToGen = append(tablesToGen, tbl)
	}
	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "%v\n", e)
		}
		fmt.Fprintf(os.Stderr, "共 %d 个错误，未生成任何文件\n", len(errs))
		os.Exit(1)
	}

	unitTest := true
	if cfg != nil {
		unitTest = cfg.Codegen.UnitTestEnable
	}
	if *noTests {
		unitTest = false
	}

	files, err := codegen.Generate(tablesToGen, codegen.Options{UnitTestEnable: unitTest})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	if *dryRun {
		for _, f := range files {
			fmt.Println(f.Path)
		}
		fmt.Printf("dry-run：%d 个文件\n", len(files))
		return
	}

	created, skipped, err := writeFiles(*out, files, *force)
	if err != nil {
		fmt.Fprintf(os.Stderr, "写盘失败: %v（已写 %d，跳过 %d）\n", err, created, skipped)
		os.Exit(1)
	}
	fmt.Printf("生成 %d / 跳过 %d（已存在）/ 失败 0\n", created, skipped)
}

func writeFiles(root string, files []model.GeneratedFile, force bool) (created, skipped int, err error) {
	for _, f := range files {
		dst := filepath.Join(root, filepath.FromSlash(f.Path))
		if !force {
			if _, statErr := os.Stat(dst); statErr == nil {
				skipped++
				continue
			}
		}
		if mkErr := os.MkdirAll(filepath.Dir(dst), 0o755); mkErr != nil {
			return created, skipped, mkErr
		}
		if wErr := os.WriteFile(dst, f.Content, 0o644); wErr != nil {
			return created, skipped, wErr
		}
		created++
	}
	return created, skipped, nil
}

func parseTables(s string) ([]string, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	var out []string
	for _, t := range strings.Split(s, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			out = append(out, t)
		}
	}
	return out, nil
}

func schemaFromDSN(dsn string) string {
	// user:pass@tcp(host:port)/dbname?params
	slash := strings.LastIndex(dsn, "/")
	if slash < 0 {
		return ""
	}
	rest := dsn[slash+1:]
	if q := strings.IndexAny(rest, "?"); q >= 0 {
		rest = rest[:q]
	}
	return rest
}
```

- [ ] **Step 4: 运行确认通过**

Run:
```bash
go test ./tools/codegen/ -v
go build ./tools/codegen/
```
Expected: PASS + 编译通过

- [ ] **Step 5: 手工冒烟（dry-run，需本地 MySQL）**

Run: `go run ./tools/codegen --module=order --dry-run`
Expected: 打印该 schema 全部表的计划文件清单（每表 7 行）+ `dry-run：N 个文件`；无 DB 时明确报 `metadata unreachable` 且退出码 1。

- [ ] **Step 6: 提交**

```bash
git add tools/codegen/
git commit -m "feat(codegen): cli draft mode (metadata→build→generate→write with no-overwrite)"
```

---

### Task 12: 端到端冒烟 `make codegen-smoke`（M1 验收）

**Files:**
- Create: `tools/codegen/smoke_integration_test.go`（build tag `integration`）
- Modify: `Makefile`（追加 `codegen-smoke` 目标并加入 `.PHONY`）

**Interfaces:**
- Consumes: 全部上游产物；`config.LoadGlobalConfig`（repo root 下取 DSN）。
- Produces: `make codegen-smoke` —— 建临时表 → 生成 → 写入 `modules/order` → `go build` → 跑生成的测试 → 全量清理。

- [ ] **Step 1: 写失败测试**

```go
// tools/codegen/smoke_integration_test.go
//go:build integration

package main

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"

	"alexGo-cloud/pkg/codegen"
	"alexGo-cloud/pkg/codegen/builder"
	"alexGo-cloud/pkg/codegen/metadata"
	"alexGo-cloud/pkg/codegen/model"
	"alexGo-cloud/pkg/config"
)

const smokeTable = "codegen_smoke_item"

// TestSmoke_RealTableGoBuild 是 M1 验收：真实表 → 生成 → 编译 → 跑生成的单测 → 清理。
// 需要本地 MySQL（docker compose 起）。DB 不可达时 Skip。
func TestSmoke_RealTableGoBuild(t *testing.T) {
	repoRoot := findRepoRoot(t)
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadGlobalConfig()
	if err != nil || cfg == nil || cfg.Database.DSN == "" {
		t.Skipf("no dsn in config: %v", err)
	}

	ctx := context.Background()
	db, err := sql.Open("mysql", cfg.Database.DSN)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("mysql unreachable: %v (先 docker compose up mysql)", err)
	}

	// 1. 建冒烟表（幂等）
	mustExec(t, db, "DROP TABLE IF EXISTS "+smokeTable)
	mustExec(t, db, `CREATE TABLE `+smokeTable+` (
  id bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  name varchar(64) NOT NULL COMMENT '名称',
  price decimal(10,2) NOT NULL DEFAULT '0.00' COMMENT '价格',
  status tinyint NOT NULL DEFAULT '0' COMMENT '状态',
  remark varchar(255) DEFAULT NULL COMMENT '备注',
  password varchar(128) DEFAULT NULL COMMENT '密码',
  tenant_id bigint unsigned NOT NULL DEFAULT '0' COMMENT '租户',
  created_at datetime NOT NULL COMMENT '创建时间',
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='codegen smoke'`)
	t.Cleanup(func() {
		mustExec(t, db, "DROP TABLE IF EXISTS "+smokeTable)
	})

	// 2. 读元数据 + 构建 + 生成
	schema := schemaFromDSN(cfg.Database.DSN)
	if schema == "" {
		t.Fatal("cannot parse schema from dsn")
	}
	meta, err := metadata.NewMySQLReader(db, schema).ReadTable(ctx, smokeTable)
	if err != nil {
		t.Fatalf("ReadTable: %v", err)
	}
	tbl, err := builder.Build(meta, builder.Options{Module: "order"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	files, err := codegen.Generate([]*model.Table{tbl}, codegen.Options{UnitTestEnable: true})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(files) != 7 {
		t.Fatalf("files = %d, want 7", len(files))
	}

	// 3. 只记录本次新建的路径，cleanup 只删自己的文件
	var newPaths []string
	for _, f := range files {
		if _, statErr := os.Stat(f.Path); statErr != nil {
			newPaths = append(newPaths, f.Path)
		}
	}
	t.Cleanup(func() {
		for _, p := range newPaths {
			_ = os.Remove(p)
		}
	})
	created, skipped, err := writeFiles(".", files, false)
	if err != nil {
		t.Fatalf("writeFiles: %v", err)
	}
	if created != len(newPaths) || skipped != len(files)-len(newPaths) {
		t.Fatalf("created=%d skipped=%d, newPaths=%d", created, skipped, len(newPaths))
	}

	// 4. 编译整个 order 模块（含生成文件）
	out, err := exec.Command("go", "build", "./modules/order/...").CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	// 5. 跑生成的单测 + vet（vet 覆盖 _test.go 编译）
	entity := tbl.ClassName // CodegenSmokeItem
	out, err = exec.Command("go", "test", "./modules/order/service/",
		"-run", "Test"+entity+"Service_CRUD", "-count=1").CombinedOutput()
	if err != nil {
		t.Fatalf("generated test failed: %v\n%s", err, out)
	}
	out, err = exec.Command("go", "vet", "./modules/order/...").CombinedOutput()
	if err != nil {
		t.Fatalf("go vet: %v\n%s", err, out)
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("go.mod not found from cwd")
	return ""
}

func mustExec(t *testing.T, db *sql.DB, query string) {
	t.Helper()
	if _, err := db.Exec(query); err != nil {
		t.Fatalf("exec %q: %v", strings.TrimSpace(query[:min(40, len(query))]), err)
	}
}

func schemaFromDSN(dsn string) string {
	slash := strings.LastIndex(dsn, "/")
	if slash < 0 {
		return ""
	}
	rest := dsn[slash+1:]
	if q := strings.IndexAny(rest, "?"); q >= 0 {
		rest = rest[:q]
	}
	return rest
}
```

- [ ] **Step 2: 运行测试**

Run: `go test -tags integration ./tools/codegen/ -run TestSmoke -v`
Expected: PASS（DB 可达时完整跑通建表→生成→build→单测→清理）；MySQL 不可达则 SKIP——**Skip 不算验收**，需先 `make docker-up` 再跑。

- [ ] **Step 3: 接 Makefile**

Makefile 修改：

```makefile
.PHONY: ... codegen-smoke   # 在原 .PHONY 行追加

codegen-smoke: ## M1 验收：真实表生成→编译→跑生成测试→清理（需本地 MySQL）
	go test -tags integration ./tools/codegen/ -run TestSmoke -v
```

- [ ] **Step 4: 运行验收**

Run:
```bash
make docker-up   # 若本地 MySQL 未起
make codegen-smoke
go build ./... && go test ./...
```
Expected:
- `TestSmoke_RealTableGoBuild ... PASS`（**不是 Skip**——Skip 说明 DB 没通，验收无效）
- `go build ./...` 全绿、`go test ./...` 全绿
- 跑完后 `git status` 无残留：冒烟表已 DROP、生成文件已删除（`modules/order` 下无 `codegen_smoke_item*.go`）

- [ ] **Step 5: 提交**

```bash
git add tools/codegen/smoke_integration_test.go Makefile
git commit -m "test(codegen): end-to-end smoke (real table → generate → go build → cleanup)"
```

---

## M1 完成验收（超出单任务，执行完 Task 12 后核对）

- [ ] `go build ./...`、`go test ./...`、`golangci-lint run` 全绿
- [ ] `make codegen-smoke` PASS（非 Skip）
- [ ] `pkg/codegen` 树内无 `modules/*`、fx 依赖（`go list -deps ./pkg/codegen/... | grep alexGo` 仅出现 `pkg/config`、`pkg/codegen/*`、`pkg/*`叶子）
- [ ] 旧工具未动（`scripts/`、`tools/dbgen` 保持原样——删除属 M7）
- [ ] 向用户汇报 M1 完成，请求撰写 **Plan #2（M2 配置持久化）**
