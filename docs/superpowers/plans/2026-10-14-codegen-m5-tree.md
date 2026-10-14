# codegen M5 树表 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付 `template_type=2` 树表模板族——引擎入口 `parent_id`+`name` 固定约定校验、后端 overlay 3 模板（HasChildren/validateParent/树语义单测）、前端 overlay 2 模板（vxe treeConfig 列表 + TreeSelect 表单）、codegen 编辑弹窗模板类型入口，验收 = 树表族 golden 14 产物 + 冒烟 42 产物。

**Architecture:** 树表族采用 **overlay**（spec §10.3 对 §3 的字面修订）：`treeTemplates()` = single 全集按**输出模式**被 5 个 tree 模板替换，其余 9 个复用——族产物恒为 14，不是追加。约定校验放 `generate.go` 入口（`t.Validate()` 之后、`Render` 之前），CLI 与在线预览同路径生效。后端只加 `HasChildren` + `validateParent` + Delete 有子拒绝，List 保持平铺；构树全部在前端（vxe `treeConfig.transform` + 表单 `TreeSelect` 本地构树）。

**Tech Stack:** Go text/template（front 前缀切 `[[ ]]` delims）、gofmt golden 测试、vben5_antd + ant-design-vue 4（vxe-grid treeConfig、antd TreeSelect）、vitest、`make codegen-smoke`（`-tags integration`，本地 MySQL）。

**Spec:** `docs/superpowers/specs/2026-10-10-codegen-redesign-design.md` §10（增补已提交 `a037706`）；roadmap 记账 `2ebb4c7`。本计划逐条论证自 §10.1–10.5。

## Global Constraints

- 直接落 main、不建功能分支、**不 push**（push 由用户决定）；每任务 TDD、独立 conventional commit（中文正文允许）。
- Go 侧门禁：`CGO_ENABLED=0 go test ./...` 全绿（每任务跑）。
- front 模板 delims 是 `[[ ]]`；产物不得残留未翻译 `[[` 或 Go 动作形态 `{{`（残留正则在 `TestGenerate_FrontNoDelimResidue`）。
- `JSONName == "-"` 的列（password/tenant_id）不得出现在任何 front 产物（`TestGenerate_FrontOmitsHiddenColumns`）。
- **M5 计数链**（每个断言只在其归属任务改动一次）：
  - `TestGenerate_Golden` = 14（**不变**）；`TestGenerate_TwoTablesSameModule` = 28（**不变**）；`TestGenerate_NoUnitTest` = 12（**不变**）。
  - `TestGenerate_AggregatesErrors` 的 ErrTemplateMissing 载体从 `TemplateTypeTree` 改为 `TemplateTypeMainSub`（Task 1，一次性）。
  - 新增 `TestGenerate_TreeGolden` = 14（Task 2）；树 NoUnitTest = 12（Task 2 内断言）。
  - smoke `28 → 42`（3 表×14）**只在 Task 5 改一次**。
  - `acceptance_test.go files >= 4`、`tenant_writepath_test.go 打戳 >= 2` 不动。
- 前端门禁：`pnpm --filter @vben/web-antd run typecheck`（vue-tsc）+ `pnpm --filter @vben/web-antd run test`（vitest）；**生成产物不跑 lint**。
- `make codegen-smoke` 只在 Task 5 跑；本机 MySQL 不可达时测试会 `Skip`——记录实情，不伪造通过。
- M4 跟进票 P2-1-F（7 条 deferred minors，roadmap 已记）**不阻塞、M5 不顺手修**。

## Review Focus

（每条挂到拥有该代码的任务与其测试。）

1. **约定校验必须先于 Render**——树族未实现时缺列会报 `ErrTemplateMissing`，掩盖真因「缺 parent_id」。→ Task 1 `TestGenerate_TreeConvention`：断言 `errors.Is(err, ErrColumnInvalid)` **且** `!errors.Is(err, ErrTemplateMissing)`，错误文本点名缺的列。
2. **overlay 必须按输出模式替换 single 条目**——若追加而非替换，同路径双模板 → `CheckCollide` 报错或漏覆盖，族产物 ≠14。→ Task 2 `TestServerTreePaths`（tree 与 single 逐 base 名比对模式相等）+ `TestGenerate_TreeGolden` files==14。
3. **front 树产物的 delims 残留与敏感列泄漏**——tree 的 list/form 是新写的 front 模板，最易残留 `[[` 或把 `parent_id` 之外的隐藏列带进去。→ Task 3 把 `TestGenerate_FrontNoDelimResidue`、`TestGenerate_FrontOmitsHiddenColumns` 扩到单表+树表双 fixture。
4. **TreeSelect 的 `componentProps` 必须是函数形式**——对象形式在 modal 打开后不随 `parentTree.value` 更新，父级下拉永远空树。→ Task 3 golden 钉 `componentProps: () =>` 与 `treeConfig:`，并以 fixture 落盘 + vue-tsc typecheck 作为类型防波堤。
5. **生成的树语义单测必须真实存在并被冒烟执行**（父不存在拒绝/自引用拒绝/有子禁删）——模板少写一个分支，线上才炸。→ Task 2 golden 钉 `TestCodegenDemoMenuService_Tree`；Task 5 smoke 用 `-run 'Test(... )Service_'` 真实执行三实体全部生成测试。

## File Structure

| 动作 | 路径 | 职责 |
| --- | --- | --- |
| 修改 | `pkg/codegen/generate.go` | 入口加 `validateTreeConvention`（spec §10.1） |
| 修改 | `pkg/codegen/generate_test.go` | AggregatesErrors 改 MainSub；加 treeFixtureMeta/buildTreeFixture/TreeGolden/TreeConvention；front 两测试扩双 fixture |
| 修改 | `pkg/codegen/template/paths.go` | `serverTreeTemplates`/`frontTreeTemplates`、导出 Names/Pattern、`treeTemplates()` 按输出模式覆盖、`templateMapFor` 接 `TemplateTypeTree` |
| 修改 | `pkg/codegen/template/paths_test.go` | `TestServerTreePaths` / `TestFrontTreePaths` |
| 新建 | `pkg/codegen/template/templates/server/go/tree/repository.go.tmpl` | single + `HasChildren` |
| 新建 | `pkg/codegen/template/templates/server/go/tree/service.go.tmpl` | single + `validateParent` + Delete 有子拒绝 |
| 新建 | `pkg/codegen/template/templates/server/go/tree/service_test.go.tmpl` | fake + HasChildren + 树语义测试 |
| 新建 | `pkg/codegen/template/templates/front/vben5_antd/tree/list.vue.tmpl` | treeConfig + 关分页 + 无搜索 |
| 新建 | `pkg/codegen/template/templates/front/vben5_antd/tree/form.vue.tmpl` | parent_id 硬认 TreeSelect + 打开构树 |
| 修改 | `apps/web-antd/src/views/infra/codegen/index.vue` | EditForm 加模板类型 Select（主子置灰）+ onEdit 回填 |
| 修改 | `tools/codegen/smoke_integration_test.go` | 第 3 张树表、计数 42、pattern 三实体 |
| 修改 | `docs/usage.md` | 追加 §2.9 树表模板族 |
| 修改 | `docs/roadmap.md` | P2-1 记 M5 ✅ 与提交范围 |

（golden 文件 `pkg/codegen/testdata/golden/*` 由 `-update` 生成，Task 2 建、Task 3 更新。）

---

### Task 1: 引擎树表约定校验（spec §10.1）

**Files:**
- Modify: `pkg/codegen/generate.go`（`t.Validate()` 之后插入校验 + 文件尾加函数）
- Test: `pkg/codegen/generate_test.go`

**Interfaces:**
- Consumes: `model.Table.Validate()`、`model.ErrColumnInvalid`、`builder.Build(meta, builder.Options{Module, TemplateType})`、`metadata.TableMeta`。
- Produces: `validateTreeConvention(t *model.Table) error`（包内未导出；`TemplateTypeTree` 缺列 → wrap `ErrColumnInvalid`）。Task 2 起 `treeTemplates()` 才存在——本任务正例（约定通过）会落进 `ErrTemplateMissing`，所以本任务**只测负例与顺序**，正例由 Task 2 的 `TestGenerate_TreeGolden` 覆盖。

- [ ] **Step 1: 写树 fixture helper 与失败测试**

在 `pkg/codegen/generate_test.go` 追加（`buildSecondFixture` 之后）：

```go
// treeFixtureMeta：树表 fixture（spec §10.1 固定约定 parent_id + name）。
// 表名 codegen_demo_menus → 实体 CodegenDemoMenu、snake codegen_demo_menu，
// 与单表 fixture（codegen_demo_item）不同名，可同包共存。
func treeFixtureMeta() *metadata.TableMeta {
	return &metadata.TableMeta{
		Schema:  "alexgo",
		Name:    "codegen_demo_menus",
		Comment: "codegen demo tree",
		Columns: []metadata.ColumnMeta{
			{Name: "id", DataType: "bigint", ColumnType: "bigint unsigned", Key: "PRI", Extra: "auto_increment", Comment: "主键", Ordinal: 1},
			{Name: "parent_id", DataType: "bigint", ColumnType: "bigint unsigned", Comment: "父级ID", Ordinal: 2},
			{Name: "name", DataType: "varchar", ColumnType: "varchar(64)", Comment: "名称", Ordinal: 3},
			{Name: "remark", DataType: "varchar", ColumnType: "varchar(255)", Nullable: true, Comment: "备注", Ordinal: 4},
			{Name: "created_at", DataType: "datetime", ColumnType: "datetime", Comment: "创建时间", Ordinal: 5},
			{Name: "tenant_id", DataType: "bigint", ColumnType: "bigint unsigned", Comment: "租户", Ordinal: 6},
		},
	}
}

// buildTreeFixture 树表 fixture（TemplateType=2）。
func buildTreeFixture(t *testing.T) *model.Table {
	t.Helper()
	tbl, err := builder.Build(treeFixtureMeta(), builder.Options{Module: "order", TemplateType: model.TemplateTypeTree})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return tbl
}

// dropColumns 返回去掉指定列的 meta 拷贝（不改原 meta）。
func dropColumns(meta *metadata.TableMeta, names ...string) *metadata.TableMeta {
	drop := map[string]bool{}
	for _, n := range names {
		drop[n] = true
	}
	out := *meta
	out.Columns = out.Columns[:0:0]
	for _, c := range meta.Columns {
		if !drop[c.Name] {
			out.Columns = append(out.Columns, c)
		}
	}
	return &out
}

// TestGenerate_TreeConvention 钉 spec §10.1：树表缺 parent_id/name 必须报
// ErrColumnInvalid，且校验先于 Render——否则树族未实现的 ErrTemplateMissing
// 会掩盖「缺列」这个真因（Review Focus ①）。
func TestGenerate_TreeConvention(t *testing.T) {
	cases := []struct {
		name    string
		meta    *metadata.TableMeta
		wantMsg string
	}{
		{"缺 parent_id", dropColumns(treeFixtureMeta(), "parent_id"), "parent_id"},
		{"缺 name", dropColumns(treeFixtureMeta(), "name"), "name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tbl, err := builder.Build(tc.meta, builder.Options{Module: "order", TemplateType: model.TemplateTypeTree})
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			_, err = codegen.Generate([]*model.Table{tbl}, codegen.Options{UnitTestEnable: true})
			if !errors.Is(err, model.ErrColumnInvalid) {
				t.Fatalf("err = %v, want ErrColumnInvalid", err)
			}
			if errors.Is(err, model.ErrTemplateMissing) {
				t.Fatalf("校验必须先于 Render，ErrTemplateMissing 掩盖了缺列: %v", err)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("err = %q, 应点名缺列 %q", err, tc.wantMsg)
			}
		})
	}
}
```

同时把 `TestGenerate_AggregatesErrors` 里的一行改成 MainSub（树族 Task 2 起可用，ErrTemplateMissing 的载体改由主子表承担）：

```go
	badType := buildFixture(t)
	badType.TemplateType = model.TemplateTypeMainSub // M6 未实现 → ErrTemplateMissing（M5 起 tree 已交付，不能再当缺模板载体）
```

- [ ] **Step 2: 跑红**

Run: `CGO_ENABLED=0 go test ./pkg/codegen -run 'TreeConvention|AggregatesErrors' -v`
Expected: `TestGenerate_TreeConvention` FAIL——`err = ... template type 2 ... ErrTemplateMissing`（尚未实现校验，落进了 Render 的 default 分支）；`AggregatesErrors` 若也红属预期（同样未改完），以 TreeConvention 红为准。

- [ ] **Step 3: 实现校验**

`pkg/codegen/generate.go` 主循环改为：

```go
		if err := t.Validate(); err != nil {
			errs = append(errs, err)
			continue
		}
		// 树表固定约定（spec §10.1）：必须在 Render 之前——缺列报 ErrColumnInvalid，
		// 不被 tree 族的 ErrTemplateMissing 掩盖；CLI 与在线预览共用本入口，同一路径生效。
		if err := validateTreeConvention(t); err != nil {
			errs = append(errs, err)
			continue
		}
		fs, err := template.Render(t, template.Options{UnitTestEnable: opts.UnitTestEnable})
```

文件尾（`Generate` 之后）加：

```go
// validateTreeConvention 树表固定约定（spec §10.1）：parent_id（整型自引用，根=0）
// 与 name（显示名）两列必须存在。不做可配置列名（YAGNI，与房规 SystemMenusPage 同款）。
func validateTreeConvention(t *model.Table) error {
	if t.TemplateType != model.TemplateTypeTree {
		return nil
	}
	has := make(map[string]bool, len(t.Columns))
	for _, c := range t.Columns {
		has[c.Name] = true
	}
	if !has["parent_id"] {
		return fmt.Errorf("%w: %s: tree table requires parent_id column (spec §10.1)", model.ErrColumnInvalid, t.TableName)
	}
	if !has["name"] {
		return fmt.Errorf("%w: %s: tree table requires name column (spec §10.1)", model.ErrColumnInvalid, t.TableName)
	}
	return nil
}
```

- [ ] **Step 4: 跑绿**

Run: `CGO_ENABLED=0 go test ./pkg/codegen -run 'TreeConvention|AggregatesErrors' -v`
Expected: PASS（TreeConvention 两子测：缺列 → ErrColumnInvalid 且非 ErrTemplateMissing；AggregatesErrors 两错误仍聚齐）。

Run: `CGO_ENABLED=0 go test ./...`
Expected: 全绿（单表 golden 14/28/12 等计数不受影响——校验对非 tree 类型直接放行）。

- [ ] **Step 5: Commit**

```bash
git add pkg/codegen/generate.go pkg/codegen/generate_test.go
git commit -m "feat(codegen): 树表约定校验 parent_id+name 先于渲染（spec §10.1）

缺列报 ErrColumnInvalid 而非 ErrTemplateMissing，CLI 与在线预览同路径生效。
AggregatesErrors 的缺模板载体改用 TemplateTypeMainSub（tree 自本笔起已交付）。"
```

---

### Task 2: 树表后端 overlay（paths 接线 + 3 模板 + TreeGolden）

**Files:**
- Modify: `pkg/codegen/template/paths.go`
- Create: `pkg/codegen/template/templates/server/go/tree/repository.go.tmpl`
- Create: `pkg/codegen/template/templates/server/go/tree/service.go.tmpl`
- Create: `pkg/codegen/template/templates/server/go/tree/service_test.go.tmpl`
- Test: `pkg/codegen/template/paths_test.go`、`pkg/codegen/generate_test.go`
- 生成: `pkg/codegen/testdata/golden/*`（`-update`）

**Interfaces:**
- Consumes: Task 1 的 `buildTreeFixture`；single 模板全文（本任务 tree 模板 = single 全文 + 增量）；`template.Render` 按 `path.Base(name)` 关联接收者（接收者与模板同名，不同目录可复用 base 名）。
- Produces: `treeTemplates() map[string]string`；导出 `ServerTreeTemplateNames() []string` / `ServerTreeTemplatePattern(name) string`；`frontTreeTemplates`（本任务为空 map 占位，Task 3 填充）；`TestGenerate_TreeGolden` = 14。`templateMapFor` 对 `TemplateTypeTree` 返回 `treeTemplates()`。

- [ ] **Step 1: 写失败测试（paths + golden）**

`pkg/codegen/template/paths_test.go` 追加：

```go
// TestServerTreePaths 钉树表后端 overlay 3 模板（spec §10.3）：
// 与 single 同输出模式（覆盖语义，族产物恒 14），路径一一对应。
func TestServerTreePaths(t *testing.T) {
	b := template.NewBind(fixtureTable())
	want := map[string]string{
		"repository.go.tmpl":   "modules/order/repository/order_item.go",
		"service.go.tmpl":      "modules/order/service/order_item.go",
		"service_test.go.tmpl": "modules/order/service/order_item_test.go",
	}
	singleByBase := map[string]string{}
	for _, n := range template.ServerSingleTemplateNames() {
		singleByBase[path.Base(n)] = template.ServerSingleTemplatePattern(n)
	}
	names := template.ServerTreeTemplateNames()
	if len(names) != len(want) {
		t.Fatalf("template count = %d, want %d: %v", len(names), len(want), names)
	}
	for _, name := range names {
		short := path.Base(name)
		pattern := template.ServerTreeTemplatePattern(name)
		if got := template.RenderPath(pattern, b); got != want[short] {
			t.Errorf("%s path = %q, want %q", name, got, want[short])
		}
		// 覆盖语义（Review Focus ②）：模式必须与 single 完全相同，否则不是覆盖是分叉。
		if singleByBase[short] == "" || singleByBase[short] != pattern {
			t.Errorf("%s pattern = %q, want same as single %q", name, pattern, singleByBase[short])
		}
	}
}
```

`pkg/codegen/generate_test.go` 追加（`TestGenerate_AggregatesErrors` 之后即可）：

```go
// TestGenerate_TreeGolden 树表族 golden（spec §10.5 验收）：
// overlay 产物恒 14（5 覆盖 + 9 复用，Review Focus ②），
// 内容钉住后端树语义（HasChildren/validateParent/树测试）与模型 ParentID。
func TestGenerate_TreeGolden(t *testing.T) {
	files, err := codegen.Generate([]*model.Table{buildTreeFixture(t)}, codegen.Options{UnitTestEnable: true})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(files) != 14 {
		t.Fatalf("files = %d, want 14: %+v", len(files), pathsOf(files))
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
			t.Fatalf("read golden %s: %v (run: go test ./pkg/codegen -run TreeGolden -update)", golden, err)
		}
		if !bytes.Equal(f.Content, want) {
			t.Errorf("mismatch %s\n--- got ---\n%s\n--- want ---\n%s", f.Path, f.Content, want)
		}
	}
	byPath := map[string][]byte{}
	for _, f := range files {
		byPath[f.Path] = f.Content
	}
	// 内容钉（Review Focus ⑤）：三个后端 overlay 必须带树语义，模型必须有 ParentID。
	checks := []struct{ p, want string }{
		{"modules/order/repository/codegen_demo_menu.go", "HasChildren("},
		{"modules/order/service/codegen_demo_menu.go", "validateParent("},
		{"modules/order/service/codegen_demo_menu_test.go", "TestCodegenDemoMenuService_Tree"},
		{"modules/order/model/codegen_demo_menu.go", "ParentID"},
	}
	for _, c := range checks {
		if !bytes.Contains(byPath[c.p], []byte(c.want)) {
			t.Errorf("%s missing %q", c.p, c.want)
		}
	}
	// 单测开关对树族同样生效：14 → 12（去 service_test + api.test）。
	off, err := codegen.Generate([]*model.Table{buildTreeFixture(t)}, codegen.Options{UnitTestEnable: false})
	if err != nil {
		t.Fatalf("Generate(no test): %v", err)
	}
	if len(off) != 12 {
		t.Fatalf("files = %d, want 12 (no unit test)", len(off))
	}
}
```

- [ ] **Step 2: 跑红**

Run: `CGO_ENABLED=0 go test ./pkg/codegen/template -run TestServerTreePaths -v`
Expected: FAIL——`ServerTreeTemplateNames` undefined（paths.go 还没接）。

Run: `CGO_ENABLED=0 go test ./pkg/codegen -run TestGenerate_TreeGolden -v`
Expected: FAIL——`Generate: ... template type 2 ... ErrTemplateMissing`。

- [ ] **Step 3: paths.go 接线**

在 `serverSingleTemplates` 一段之后加：

```go
// serverTreeTemplates：树表后端 overlay（spec §10.3）。value=输出模式与 single 完全相同——
// 按输出模式**覆盖**（非追加），族产物恒 14。
var serverTreeTemplates = map[string]string{
	"server/go/tree/repository.go.tmpl":   "modules/{module}/repository/{snake}.go",
	"server/go/tree/service.go.tmpl":      "modules/{module}/service/{snake}.go",
	"server/go/tree/service_test.go.tmpl": "modules/{module}/service/{snake}_test.go",
}

// ServerTreeTemplateNames 返回排序后的树表后端模板名列表（确定性）。
func ServerTreeTemplateNames() []string {
	names := make([]string, 0, len(serverTreeTemplates))
	for n := range serverTreeTemplates {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func ServerTreeTemplatePattern(name string) string {
	p, ok := serverTreeTemplates[name]
	if !ok {
		return ""
	}
	return p
}

// frontTreeTemplates：树表前端 overlay（spec §10.3）。
// 按任务渐进登记（与 single 同规则）：本任务先空占位，M5 T3 补 list.vue/form.vue——
// ParseFS 缺文件即整族报错，登记与文件必须同笔落地。
var frontTreeTemplates = map[string]string{}

// FrontTreeTemplateNames 返回排序后的树表前端模板名列表（确定性）。
func FrontTreeTemplateNames() []string {
	names := make([]string, 0, len(frontTreeTemplates))
	for n := range frontTreeTemplates {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func FrontTreeTemplatePattern(name string) string {
	return frontTreeTemplates[name]
}
```

在 `singleTemplates()` 之后加：

```go
// treeTemplates = single 全集，其中输出模式被 tree overlay 命中的条目被替换
// （spec §10.3 修订 §3「渲染该目录全部模板」：叠加而非整目录复制）。
// 同模式双条目会让 CheckCollide 报路径冲突，所以先删后加。
func treeTemplates() map[string]string {
	overlay := make(map[string]string, len(serverTreeTemplates)+len(frontTreeTemplates))
	maps.Copy(overlay, serverTreeTemplates)
	maps.Copy(overlay, frontTreeTemplates)
	m := make(map[string]string, len(singleTemplates()))
	for name, pattern := range singleTemplates() {
		if _, replaced := overlay[pattern]; !replaced {
			m[name] = pattern
		}
	}
	maps.Copy(m, overlay)
	return m
}
```

`templateMapFor` 改为：

```go
func templateMapFor(t *model.Table) (map[string]string, error) {
	switch t.TemplateType {
	case model.TemplateTypeSingle:
		return singleTemplates(), nil
	case model.TemplateTypeTree:
		return treeTemplates(), nil
	default:
		return nil, fmt.Errorf("%w: template type %d (main_sub 在 M6 交付)", model.ErrTemplateMissing, t.TemplateType)
	}
}
```

（顶部注释 `// templateMapFor 按模板类型选择模板族；M1 仅 single。` 改为 `// templateMapFor 按模板类型选择模板族：single/tree（M5），main_sub 在 M6。`）

- [ ] **Step 4: 跑 paths 测试（此时 tree 模板文件还不存在，golden 仍红）**

Run: `CGO_ENABLED=0 go test ./pkg/codegen/template -run TestServerTreePaths -v`
Expected: PASS（只测映射表，不 ParseFS）。

- [ ] **Step 5: 写三个 server tree 模板（全文，= single 全文 + 树增量）**

`pkg/codegen/template/templates/server/go/tree/repository.go.tmpl`：

```go
package repository

import (
	"context"

	"alexGo-cloud/modules/{{.Module}}/model"
{{- if .HasTenant }}
	"alexGo-cloud/pkg/tenant"
{{- end }}
	"gorm.io/gorm"
)

type {{.Entity}}Repository interface {
	Create(ctx context.Context, entity *model.{{.Entity}}) error
	List(ctx context.Context{{ if .HasTenant }}, tenantID uint64{{ end }}) ([]*model.{{.Entity}}, error)
	GetByID(ctx context.Context{{ if .HasTenant }}, tenantID uint64{{ end }}, id uint64) (*model.{{.Entity}}, error)
	Update(ctx context.Context, entity *model.{{.Entity}}) error
	Delete(ctx context.Context{{ if .HasTenant }}, tenantID uint64{{ end }}, id uint64) error
	// HasChildren 树表专用（spec §10.2）：id 是否有子节点（根 parent_id=0 的子也计入）。
	HasChildren(ctx context.Context{{ if .HasTenant }}, tenantID uint64{{ end }}, id uint64) (bool, error)
}

type {{ lower .Entity }}Repo struct {
	db *gorm.DB
}

func New{{.Entity}}Repository(db *gorm.DB) {{.Entity}}Repository {
	return &{{ lower .Entity }}Repo{db: db}
}

func (r *{{ lower .Entity }}Repo) Create(ctx context.Context, entity *model.{{.Entity}}) error {
	{{- if .HasTenant }}
	// 打戳（spec §9.2）：json:"-" 使客户端灌不进来，零值才回填、显式值保留。
	if entity.TenantID == 0 {
		entity.TenantID = tenant.TenantIDFromContext(ctx)
	}
	{{- end }}
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
	{{- if .HasTenant }}
	// 禁用 Save（spec §9.2）：Save 丢弃链式 WHERE，跨租户照改不误。
	// Select("*") 保证零值也写入（全量覆盖语义），双条件把行锁在本租户内。
	return r.db.WithContext(ctx).Model(&model.{{.Entity}}{}).
		Where("id = ? AND tenant_id = ?", entity.ID, tenant.TenantIDFromContext(ctx)).
		Select("*").Updates(entity).Error
	{{- else }}
	return r.db.WithContext(ctx).Save(entity).Error
	{{- end }}
}

func (r *{{ lower .Entity }}Repo) Delete(ctx context.Context{{ if .HasTenant }}, tenantID uint64{{ end }}, id uint64) error {
	q := r.db.WithContext(ctx).Model(&model.{{.Entity}}{})
	{{- if .HasTenant }}
	q = q.Where("tenant_id = ?", tenantID)
	{{- end }}
	return q.Where("id = ?", id).Delete(&model.{{.Entity}}{}).Error
}

func (r *{{ lower .Entity }}Repo) HasChildren(ctx context.Context{{ if .HasTenant }}, tenantID uint64{{ end }}, id uint64) (bool, error) {
	var count int64
	q := r.db.WithContext(ctx).Model(&model.{{.Entity}}{}).Where("parent_id = ?", id)
	{{- if .HasTenant }}
	q = q.Where("tenant_id = ?", tenantID)
	{{- end }}
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
```

`pkg/codegen/template/templates/server/go/tree/service.go.tmpl`：

```go
package service

import (
	"context"
	"fmt"

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

// validateParent 树表父级校验（spec §10.2）：parent_id=0 是根直接放行；
// 否则父必须存在且非自身。不加 Children 字段、不加 GetTree，List 保持平铺。
func (s *{{ lower .Entity }}Service) validateParent(ctx context.Context, entity *model.{{.Entity}}) error {
	if entity.ParentID == 0 {
		return nil
	}
	if entity.ParentID == entity.ID {
		return fmt.Errorf("cannot set self (id=%d) as parent", entity.ID)
	}
	{{- if .HasTenant }}
	_, err := s.repo.GetByID(ctx, tenant.TenantIDFromContext(ctx), entity.ParentID)
	{{- else }}
	_, err := s.repo.GetByID(ctx, entity.ParentID)
	{{- end }}
	if err != nil {
		return fmt.Errorf("parent id=%d: %w", entity.ParentID, err)
	}
	return nil
}

func (s *{{ lower .Entity }}Service) Create(ctx context.Context, entity *model.{{.Entity}}) error {
	if err := s.validateParent(ctx, entity); err != nil {
		return err
	}
	{{- if .HasTenant }}
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	{{- end }}
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
	if err := s.validateParent(ctx, entity); err != nil {
		return err
	}
	{{- if .HasTenant }}
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	{{- end }}
	return s.repo.Update(ctx, entity)
}

func (s *{{ lower .Entity }}Service) Delete(ctx context.Context, id uint64) error {
	{{- if .HasTenant }}
	has, err := s.repo.HasChildren(ctx, tenant.TenantIDFromContext(ctx), id)
	{{- else }}
	has, err := s.repo.HasChildren(ctx, id)
	{{- end }}
	if err != nil {
		return err
	}
	if has {
		return fmt.Errorf("cannot delete id=%d: has children", id)
	}
	{{- if .HasTenant }}
	return s.repo.Delete(ctx, tenant.TenantIDFromContext(ctx), id)
	{{- else }}
	return s.repo.Delete(ctx, id)
	{{- end }}
}
```

`pkg/codegen/template/templates/server/go/tree/service_test.go.tmpl`：

```go
package service

import (
	"context"
	"errors"
	"testing"

	"alexGo-cloud/modules/{{.Module}}/model"
{{- if .HasTenant }}
	"alexGo-cloud/pkg/tenant"
{{- end }}
)

type fake{{.Entity}}Repo struct {
	data map[uint64]*model.{{.Entity}}
	seq  uint64
}

var err{{.Entity}}RecordNotFound = errors.New("record not found")

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
	return nil, err{{.Entity}}RecordNotFound
}

func (f *fake{{.Entity}}Repo) Update(_ context.Context, e *model.{{.Entity}}) error {
	f.data[e.ID] = e
	return nil
}

func (f *fake{{.Entity}}Repo) Delete(_ context.Context{{ if .HasTenant }}, _ uint64{{ end }}, id uint64) error {
	delete(f.data, id)
	return nil
}

// HasChildren 语义镜像：data 里存在 ParentID==id 的行即为有子。
func (f *fake{{.Entity}}Repo) HasChildren(_ context.Context{{ if .HasTenant }}, _ uint64{{ end }}, id uint64) (bool, error) {
	for _, e := range f.data {
		if e.ParentID == id {
			return true, nil
		}
	}
	return false, nil
}

func Test{{.Entity}}Service_CRUD(t *testing.T) {
	svc := New{{.Entity}}Service(&fake{{.Entity}}Repo{data: map[uint64]*model.{{.Entity}}{}})
	ctx := context.Background()
	{{- if .HasTenant }}
	ctx = tenant.WithTenantID(ctx, 42)
	{{- end }}

	e := &model.{{.Entity}}{}
	if err := svc.Create(ctx, e); err != nil {
		t.Fatalf("Create: %v", err)
	}
	{{- if .HasTenant }}
	if e.TenantID != 42 {
		t.Fatalf("Create 应从 ctx 打戳租户 42, got %d", e.TenantID)
	}
	{{- end }}
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

// Test{{.Entity}}Service_Tree 钉树语义（spec §10.2，Review Focus ⑤）：
// 父不存在拒绝、自引用拒绝、有子禁删、无子可删。
func Test{{.Entity}}Service_Tree(t *testing.T) {
	svc := New{{.Entity}}Service(&fake{{.Entity}}Repo{data: map[uint64]*model.{{.Entity}}{}})
	ctx := context.Background()
	{{- if .HasTenant }}
	ctx = tenant.WithTenantID(ctx, 42)
	{{- end }}

	if err := svc.Create(ctx, &model.{{.Entity}}{ParentID: 999}); err == nil {
		t.Fatal("父不存在应拒绝")
	}
	root := &model.{{.Entity}}{}
	if err := svc.Create(ctx, root); err != nil {
		t.Fatalf("Create root: %v", err)
	}
	child := &model.{{.Entity}}{ParentID: root.ID}
	if err := svc.Create(ctx, child); err != nil {
		t.Fatalf("Create child: %v", err)
	}
	child.ParentID = child.ID
	if err := svc.Update(ctx, child); err == nil {
		t.Fatal("自引用应拒绝")
	}
	child.ParentID = root.ID
	if err := svc.Update(ctx, child); err != nil {
		t.Fatalf("恢复父级: %v", err)
	}
	if err := svc.Delete(ctx, root.ID); err == nil {
		t.Fatal("有子禁删")
	}
	if err := svc.Delete(ctx, child.ID); err != nil {
		t.Fatalf("Delete child: %v", err)
	}
	if err := svc.Delete(ctx, root.ID); err != nil {
		t.Fatalf("Delete root: %v", err)
	}
}
```

注意：B1 回归钉——包级 var 必须保持 `err{{.Entity}}RecordNotFound` 按实体限定，**不得**新增不带实体前缀的包级变量（`TestGenerate_TwoTablesSameModule_NoDuplicateTopLevelVars` 用 AST 扫描钉死）。

- [ ] **Step 6: 生成 golden 并跑绿**

Run: `CGO_ENABLED=0 go test ./pkg/codegen -run TreeGolden -update -count=1`
Expected: 写入 14 个 golden（文件名 = Path 中 `/`→`_`，如 `modules_order_model_codegen_demo_menu.go`）。

Run: `CGO_ENABLED=0 go test ./pkg/codegen -run TreeGolden -v`
Expected: PASS（14 产物 + 4 条内容钉 + 树 12 产物断言）。

Run: `CGO_ENABLED=0 go test ./pkg/codegen/...`
Expected: 全绿——`TestGenerate_Golden`（单表 14）、`TwoTablesSameModule`（28）、`NoUnitTest`（12）不被波及；`TestServerSinglePaths`/`TestFrontSinglePaths` 不变。

- [ ] **Step 7: 冒烟式编译验证（生成代码可编译）**

tree 族的 golden 内容此时含 front 复用单表模板（Task 3 才 overlay），后端三文件已换新。抽验生成 Go 产物能编译：golden 只是文本，真正编译在 Task 5 的 smoke。本步仅跑全量测试：

Run: `CGO_ENABLED=0 go test ./...`
Expected: 全绿。

- [ ] **Step 8: Commit**

```bash
git add pkg/codegen/template/paths.go pkg/codegen/template/paths_test.go \
  pkg/codegen/template/templates/server/go/tree/ \
  pkg/codegen/generate_test.go pkg/codegen/testdata/golden/
git commit -m "feat(codegen): 树表后端 overlay 三模板 + treeTemplates 按输出模式覆盖（spec §10.2/10.3）

- paths: serverTreeTemplates/frontTreeTemplates(占位) + treeTemplates() 先删后加，
  族产物恒 14（5 覆盖 + 9 复用）
- repository +HasChildren；service +validateParent/Create/Update 前置 + Delete 有子拒绝；
  service_test +树语义测试（父不存在/自引用/有子禁删）
- TreeGolden 14 产物 + 内容钉 + 树 NoUnitTest 12"
```

---

### Task 3: 树表前端 overlay（list/form + 残留/隐藏列扩树 + typecheck）

**Files:**
- Create: `pkg/codegen/template/templates/front/vben5_antd/tree/list.vue.tmpl`
- Create: `pkg/codegen/template/templates/front/vben5_antd/tree/form.vue.tmpl`
- Modify: `pkg/codegen/template/paths.go`（`frontTreeTemplates` 填 2 条）
- Test: `pkg/codegen/template/paths_test.go`、`pkg/codegen/generate_test.go`（TreeGolden 更新 + 两 front 测试扩双 fixture + FrontFixturesToDisk 双 fixture）
- 生成: `pkg/codegen/testdata/golden/*`（`-update` 重写树族 front 5 个 golden）

**Interfaces:**
- Consumes: Task 2 的 `treeTemplates()`（此时按输出模式把 single 的 `list.vue/form.vue` 条目换成 front tree）；房规样板 `SystemMenusPage.vue` 的 treeConfig 原样；`useVbenForm` schema `componentProps` 函数形式（`packages/@core/ui-kit/form-ui/src/types.ts` `ComponentProps` 联合类型含函数分支，`form-field.vue` computed 内求值 → ref 响应式）；`TreeSelect` 已在 `apps/web-antd/src/adapter/component/index.ts` 注册；antd `fillFieldNames` label 默认 `['title','label']`——treeData 节点用 `{title, value, children}`。
- Produces: `FrontTreeTemplateNames()/FrontTreeTemplatePattern()` 的实际 2 条登记；`TestFrontTreePaths`；TreeGolden 中 front 产物含 `treeConfig`/`componentProps: () =>`；双 fixture 的残留/隐藏列/落盘 typecheck 门禁。

- [ ] **Step 1: 写失败测试**

`pkg/codegen/template/paths_test.go` 追加：

```go
// TestFrontTreePaths 钉树表前端 overlay 2 模板（spec §10.3/10.4）：
// 与 single 同输出模式（覆盖语义），路径一一对应。
func TestFrontTreePaths(t *testing.T) {
	b := template.NewBind(fixtureTable())
	want := map[string]string{
		"list.vue.tmpl":  "apps/web-antd/src/views/order/order_item/list.vue",
		"form.vue.tmpl":  "apps/web-antd/src/views/order/order_item/form.vue",
	}
	singleByBase := map[string]string{}
	for _, n := range template.FrontSingleTemplateNames() {
		singleByBase[path.Base(n)] = template.FrontSingleTemplatePattern(n)
	}
	names := template.FrontTreeTemplateNames()
	if len(names) != len(want) {
		t.Fatalf("front tree count = %d, want %d: %v", len(names), len(want), names)
	}
	for _, name := range names {
		short := path.Base(name)
		pattern := template.FrontTreeTemplatePattern(name)
		if got := template.RenderPath(pattern, b); got != want[short] {
			t.Errorf("%s path = %q, want %q", name, got, want[short])
		}
		if singleByBase[short] == "" || singleByBase[short] != pattern {
			t.Errorf("%s pattern = %q, want same as single %q", name, pattern, singleByBase[short])
		}
	}
}
```

`pkg/codegen/generate_test.go` 改动四处：

① `TestGenerate_TreeGolden` 的内容钉追加两条（在 `checks` 切片里加）：

```go
		{"apps/web-antd/src/views/order/codegen_demo_menu/list.vue", "treeConfig"},
		{"apps/web-antd/src/views/order/codegen_demo_menu/form.vue", "componentProps: () =>"},
```

② `TestGenerate_FrontOmitsHiddenColumns` 改为双 fixture 循环（原单表覆盖不变）：

```go
// 回归钉（M4 Review Focus ④ / M5 扩树）：JSONName="-" 的列不得出现在任何 front 产物中。
func TestGenerate_FrontOmitsHiddenColumns(t *testing.T) {
	for _, tc := range []struct {
		name  string
		table *model.Table
	}{
		{"single", buildFixture(t)},
		{"tree", buildTreeFixture(t)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files, err := codegen.Generate([]*model.Table{tc.table}, codegen.Options{UnitTestEnable: true})
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			found := 0
			for _, f := range files {
				if !strings.Contains(f.Path, "apps/web-antd/") {
					continue
				}
				found++
				for _, hidden := range []string{"password", "tenant_id"} {
					if strings.Contains(string(f.Content), hidden) {
						t.Errorf("%s leaks %q", f.Path, hidden)
					}
				}
			}
			if found < 3 {
				t.Fatalf("front files = %d, want >= 3", found)
			}
		})
	}
}
```

③ `TestGenerate_FrontNoDelimResidue` 同样改双 fixture 循环（循环体与断言原样搬进 `t.Run`，正则不变）：

```go
// 回归钉（M4 Review Focus ③ / M5 扩树）：front 产物不得残留未翻译的 Go 动作。
func TestGenerate_FrontNoDelimResidue(t *testing.T) {
	re := regexp.MustCompile(`\[\[|\{\{\s*([.\-]|(range|if|else|end|with|define|template)\b)`)
	for _, tc := range []struct {
		name  string
		table *model.Table
	}{
		{"single", buildFixture(t)},
		{"tree", buildTreeFixture(t)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files, err := codegen.Generate([]*model.Table{tc.table}, codegen.Options{UnitTestEnable: true})
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			for _, f := range files {
				if !strings.Contains(f.Path, "apps/web-antd/") {
					continue
				}
				if loc := re.FindIndex(f.Content); loc != nil {
					off := loc[0]
					lo, hi := off-40, off+60
					if lo < 0 {
						lo = 0
					}
					if hi > len(f.Content) {
						hi = len(f.Content)
					}
					t.Errorf("%s residue at %d: …%s…", f.Path, off, f.Content[lo:hi])
				}
			}
		})
	}
}
```

④ `TestGenerate_FrontFixturesToDisk` 改双 fixture（供 typecheck/vitest 门禁）：

```go
func TestGenerate_FrontFixturesToDisk(t *testing.T) {
	if os.Getenv("CODEGEN_FRONT_FIXTURE") != "1" {
		t.Skip("set CODEGEN_FRONT_FIXTURE=1 to write web-antd fixtures")
	}
	written := 0
	for _, tbl := range []*model.Table{buildFixture(t), buildTreeFixture(t)} {
		files, err := codegen.Generate([]*model.Table{tbl}, codegen.Options{UnitTestEnable: true})
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		for _, f := range files {
			if !strings.HasPrefix(f.Path, "apps/web-antd/") {
				continue
			}
			dst := filepath.Join("..", "..", filepath.FromSlash(f.Path))
			if mkErr := os.MkdirAll(filepath.Dir(dst), 0o755); mkErr != nil {
				t.Fatalf("mkdir %s: %v", dst, mkErr)
			}
			if wErr := os.WriteFile(dst, f.Content, 0o644); wErr != nil {
				t.Fatalf("write %s: %v", dst, wErr)
			}
			written++
		}
	}
	if written < 12 {
		t.Fatalf("written front files = %d, want >= 12 (single 7 + tree 7)", written)
	}
	t.Logf("wrote %d fixture files under apps/web-antd/src (clean up after typecheck)", written)
}
```

- [ ] **Step 2: 跑红**

Run: `CGO_ENABLED=0 go test ./pkg/codegen/template -run TestFrontTreePaths -v`
Expected: FAIL——`FrontTreeTemplateNames` 返回 0 条（空 map），count != 2。

Run: `CGO_ENABLED=0 go test ./pkg/codegen -run 'TestGenerate_TreeGolden|FrontNoDelimResidue' -v`
Expected: TreeGolden FAIL——树 front golden 还是 Task 2 的单表内容，缺 `treeConfig` 内容钉。

- [ ] **Step 3: 写两个 front tree 模板（全文）**

`pkg/codegen/template/templates/front/vben5_antd/tree/list.vue.tmpl`：

```vue
<script lang="ts" setup>
import type { VxeGridProps } from '#/adapter/vxe-table';
import type { [[.Entity]] } from '#/api/[[.Module]]/[[.Business]]/types';

import { Button, message, Modal as AntdModal } from 'ant-design-vue';
import { Page, useVbenDrawer, useVbenModal } from '@vben/common-ui';

[[- $hasDate := false ]][[- range .Columns ]][[ if and .ListEnable (ne .JSONName "-") (eq .HTMLType "DatePicker") ]][[ $hasDate = true ]][[ end ]][[ end ]][[ if $hasDate ]]
import { formatDateTime } from '#/utils/format';
[[- end ]]
import {
  delete[[.Entity]],
  list[[.Entity]],
} from '#/api/[[.Module]]/[[.Business]]/api';
import { useVbenVxeGrid } from '#/adapter/vxe-table';

import DetailDrawerView from './drawer.vue';
import FormModalView from './form.vue';

defineOptions({ name: '[[pascal .Module]][[.Entity]]Page' });

// 树表显示名固定 name 列（spec §10.1 约定列，删除确认文案用）。
[[- $label := "name" ]]
const gridOptions: VxeGridProps<[[.Entity]]> = {
  columns: [
[[- range .Columns ]][[ if and .ListEnable (ne .JSONName "-") ]]
    {
      field: '[[ .JSONName ]]',
      title: '[[ if .Comment ]][[ .Comment ]][[ else ]][[ .Name ]][[ end ]]',
[[- if .IsPK ]]
      width: 80,
[[- else ]]
      minWidth: 120,
[[- end ]][[ if eq .HTMLType "DatePicker" ]]
      formatter: ({ cellValue }) => formatDateTime(cellValue),
[[- end ]][[ if eq .HTMLType "Switch" ]]
      formatter: ({ cellValue }) =>
        Number(cellValue) === 1 ? '启用' : '停用',
[[- end ]]
    },
[[- end ]][[ end ]]
    {
      fixed: 'right',
      slots: { default: 'actions' },
      title: '操作',
      width: 180,
    },
  ],
  height: 'auto',
  keepSource: true,
  // 树形交互（spec §10.4，房规 SystemMenusPage 原样）：平铺 parent_id 由 vxe 转树、
  // 默认全展开；关分页与搜索——过滤/分页都会破坏树形完整性。
  treeConfig: { expandAll: true, parentField: 'parent_id', rowField: 'id', transform: true },
  pagerConfig: { enabled: false },
  proxyConfig: {
    ajax: {
      query: async () => {
        const rows = await list[[.Entity]]();
        return { items: rows, total: rows.length };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: true, zoom: true },
};

// 无 formOptions：树表不提供搜索表单（spec §10.4）。
const [Grid, gridApi] = useVbenVxeGrid({ gridOptions });

const [FormModal, formModalApi] = useVbenModal({
  connectedComponent: FormModalView,
});

const [DetailDrawer, detailDrawerApi] = useVbenDrawer({
  connectedComponent: DetailDrawerView,
});

function onCreate() {
  formModalApi.setState({ title: '新建[[.Entity]]' });
  formModalApi.setData({}); // 清残留：防子组件把上次的旧数据当编辑
  formModalApi.open();
}

function onEdit(row: [[.Entity]]) {
  formModalApi.setState({ title: '编辑[[.Entity]]' });
  formModalApi.setData(row);
  formModalApi.open();
}

function onDetail(row: [[.Entity]]) {
  detailDrawerApi.setData(row);
  detailDrawerApi.open();
}

function onDelete(row: [[.Entity]]) {
  AntdModal.confirm({
    title: `确认删除「${String(row.[[$label]] ?? '')}」？`,
    content: '删除后不可恢复。',
    onOk: async () => {
      try {
        await delete[[.Entity]](row.id);
      } catch {
        return; // 拦截器已 toast，保持确认框
      }
      message.success('删除成功');
      gridApi.reload();
    },
  });
}

function onSaved() {
  gridApi.reload();
}
</script>

<template>
  <Page auto-content-height>
    <Grid>
      <template #toolbar-tools>
        <Button
          class="mr-2"
          type="primary"
          v-access:code="['[[.Module]]:[[.Business]]:*']"
          @click="onCreate"
        >
          新建
        </Button>
      </template>
      <template #actions="{ row }">
        <Button
          size="small"
          type="link"
          v-access:code="['[[.Module]]:[[.Business]]:*']"
          @click="onDetail(row)"
        >
          详情
        </Button>
        <Button
          size="small"
          type="link"
          v-access:code="['[[.Module]]:[[.Business]]:*']"
          @click="onEdit(row)"
        >
          编辑
        </Button>
        <Button
          danger
          size="small"
          type="link"
          v-access:code="['[[.Module]]:[[.Business]]:*']"
          @click="onDelete(row)"
        >
          删除
        </Button>
      </template>
    </Grid>
    <FormModal @saved="onSaved" />
    <DetailDrawer />
  </Page>
</template>
```

`pkg/codegen/template/templates/front/vben5_antd/tree/form.vue.tmpl`：

```vue
<script lang="ts" setup>
[[- $hasDateForm := false ]][[- range .Columns ]][[ if and .FormEnable (ne .JSONName "-") (eq .HTMLType "DatePicker") ]][[ $hasDateForm = true ]][[ end ]][[ end ]][[ if $hasDateForm ]]
import dayjs from 'dayjs';
[[- end ]]
import { message } from 'ant-design-vue';
import { ref } from 'vue';

import { useVbenModal } from '@vben/common-ui';

import {
  create[[.Entity]],
  list[[.Entity]],
  update[[.Entity]],
} from '#/api/[[.Module]]/[[.Business]]/api';
import { useVbenForm } from '#/adapter/form';

const emit = defineEmits<{ saved: [] }>();

/** null = 新建；非空 = 正在编辑的记录 id */
const editingId = ref<number | null>(null);

/** 父级树：打开弹窗时拉平铺 list 本地构树（spec §10.4，后端 List 不构树）。
 * 走函数形式 componentProps——form-field.vue 的 computed 内求值，
 * 依赖 parentTree.value 响应式，晚到的异步数据也能刷新下拉。 */
interface TreeNode {
  children?: TreeNode[];
  title: string;
  value: number;
}
const parentTree = ref<TreeNode[]>([{ children: [], title: '顶级', value: 0 }]);

async function loadParentTree() {
  try {
    const rows = await list[[.Entity]]();
    const nodes = new Map<number, TreeNode>();
    for (const r of rows) {
      nodes.set(Number(r.id), {
        children: [],
        title: String(r.name ?? r.id),
        value: Number(r.id),
      });
    }
    const roots: TreeNode[] = [];
    for (const r of rows) {
      const node = nodes.get(Number(r.id));
      if (!node) continue;
      const pid = Number(r.parent_id ?? 0);
      // 根（parent_id=0）与孤儿（父已被删）都挂顶层。
      if (pid === 0 || !nodes.has(pid)) {
        roots.push(node);
      } else {
        nodes.get(pid)?.children?.push(node);
      }
    }
    // 根=0 可选（spec §10.4）：虚拟顶级节点 value=0，真实节点全挂其下。
    parentTree.value = [{ children: roots, title: '顶级', value: 0 }];
  } catch {
    // 拦截器已 toast；树数据留空仅影响下拉可选项
  }
}

const [Modal, modalApi] = useVbenModal({
  onConfirm: async () => {
    try {
      await formApi.validateAndSubmitForm();
    } catch {
      // 校验失败或接口报错：拦截器已 toast，弹窗保持打开
    }
  },
  onOpenChange(isOpen: boolean) {
    if (!isOpen) {
      editingId.value = null;
      return;
    }
    void loadParentTree(); // 异步拉取，与下面回填互不阻塞
    // open() 之后先重置再回填（与 SystemTenantsPage 同款时序）
    void (async () => {
      await formApi.resetForm();
      const row = modalApi.getData();
      if (row.id != null) {
        editingId.value = Number(row.id);
        await formApi.setValues({
[[- range .Columns ]][[ if and .FormEnable (ne .JSONName "-") ]]
[[- if eq .HTMLType "DatePicker" ]]
          [[ .JSONName ]]: row.[[ .JSONName ]] ? dayjs(row.[[ .JSONName ]]) : undefined,
[[- else if eq .HTMLType "Switch" ]]
          [[ .JSONName ]]: Boolean(row.[[ .JSONName ]]),
[[- else ]]
          [[ .JSONName ]]: row.[[ .JSONName ]],
[[- end ]][[ end ]][[ end ]]
        });
      }
    })();
  },
});

const [Form, formApi] = useVbenForm({
  handleSubmit: async (values) => {
    const payload: Record<string, any> = { ...values };
    // 树表根=0：TreeSelect 未选/选顶级都归一为 0（spec §10.1/10.4）。
    payload.parent_id = Number(payload.parent_id ?? 0);
[[- range .Columns ]][[ if and .FormEnable (ne .JSONName "-") (eq .HTMLType "Switch") ]]
    payload.[[ .JSONName ]] = Number(payload.[[ .JSONName ]] ?? 0);
[[- end ]][[ end ]]
    if (editingId.value != null) {
      await update[[.Entity]](editingId.value, payload);
      message.success('保存成功');
    } else {
      await create[[.Entity]](payload);
      message.success('创建成功');
    }
    modalApi.close();
    emit('saved');
  },
  schema: [
[[- range .Columns ]][[ if and .FormEnable (ne .JSONName "-") ]]
[[- if eq .Name "parent_id" ]]
    {
      // 列名硬认（spec §10.4）：parent_id 一律 TreeSelect，html_type 不参与。
      // 不设 rules:'required'——根节点 value=0 是合法默认，避免 required 卡 0 值。
      component: 'TreeSelect',
      componentProps: () => ({
        allowClear: true,
        treeData: parentTree.value,
        treeDefaultExpandAll: true,
      }),
      fieldName: 'parent_id',
      label: '[[ if .Comment ]][[ .Comment ]][[ else ]][[ .Name ]][[ end ]]',
    },
[[- else ]]
    {
      component: '[[ .VbenComponent ]]',
[[- if eq .VbenComponent "DatePicker" ]]
      componentProps: { showTime: true },
[[- end ]][[ if .FormRequired ]]
      rules: 'required',
[[- end ]]
      fieldName: '[[ .JSONName ]]',
      label: '[[ if .Comment ]][[ .Comment ]][[ else ]][[ .Name ]][[ end ]]',
    },
[[- end ]][[ end ]]
  ],
  showDefaultActions: false,
});
</script>

<template>
  <Modal>
    <Form />
  </Modal>
</template>
```

- [ ] **Step 4: paths.go 登记 front overlay**

把 `frontTreeTemplates` 的空 map 替换为：

```go
var frontTreeTemplates = map[string]string{
	"front/vben5_antd/tree/list.vue.tmpl": "apps/web-antd/src/views/{module}/{business}/list.vue",
	"front/vben5_antd/tree/form.vue.tmpl": "apps/web-antd/src/views/{module}/{business}/form.vue",
}
```

（`treeTemplates()` 无需改——它已经合并 `frontTreeTemplates` 并按输出模式替换 single 条目。）

- [ ] **Step 5: 跑 paths 测试 + 更新 golden**

Run: `CGO_ENABLED=0 go test ./pkg/codegen/template -run 'TreePaths|SinglePaths' -v`
Expected: PASS——`TestFrontTreePaths` 2 条、`TestFrontSinglePaths` 7 条不回退（single 族未动）。

Run: `CGO_ENABLED=0 go test ./pkg/codegen -update -run 'Golden' -count=1`
Expected: 重写 golden——`TestGenerate_Golden`（单表）内容应零 diff，`TestGenerate_TreeGolden` 的 front 5 产物换成 tree 内容。

Run: `CGO_ENABLED=0 go test ./pkg/codegen -v`
Expected: 全绿——TreeGolden 14（含 `treeConfig`/`componentProps: () =>` 内容钉）、双 fixture 残留/隐藏列测试、其余计数 14/12/28 不变。

Run: `git diff --stat pkg/codegen/testdata/golden/`
Expected: 只有 `..._codegen_demo_menu_*` 的 5 个 front 文件变化（list/form 两文件 + 无单表 golden 变动）。若单表 golden 有变动 → single 模板被误改，回查。

- [ ] **Step 6: fixture 落盘 + 前端类型门禁（Review Focus ④）**

Run:
```bash
CODEGEN_FRONT_FIXTURE=1 CGO_ENABLED=0 go test ./pkg/codegen -run FrontFixtures -count=1
pnpm --filter @vben/web-antd run typecheck
pnpm --filter @vben/web-antd run test
rm -rf apps/web-antd/src/api/order/codegen_demo_item apps/web-antd/src/api/order/codegen_demo_menu \
  apps/web-antd/src/views/order/codegen_demo_item apps/web-antd/src/views/order/codegen_demo_menu
git status --porcelain
```
Expected: 落盘 `written >= 12`；typecheck 通过（重点：`treeConfig` 的 `parentField: 'parent_id'` 对 `VxeGridProps<T>` 可编译、`componentProps` 函数形式过 `ComponentProps` 联合类型、`TreeSelect` 已注册）；vitest 全绿（`--passWithNoTests` 底线 + 生成的 api.test）；`git status --porcelain` 无残留（fixture 目录已清）。

若 typecheck 在 `treeConfig` 或 `componentProps` 处报类型错：以房规 `SystemMenusPage.vue`（treeConfig 同款写法可编译）为基准对齐写法；`componentProps` 保持箭头函数。**不得**为了过编译把函数形式改回对象形式（Review Focus ④：对象形式不随 ref 更新）。

- [ ] **Step 7: 全量回归 + Commit**

Run: `CGO_ENABLED=0 go test ./...`
Expected: 全绿。

```bash
git add pkg/codegen/template/paths.go pkg/codegen/template/paths_test.go \
  pkg/codegen/template/templates/front/vben5_antd/tree/ \
  pkg/codegen/generate_test.go pkg/codegen/testdata/golden/
git commit -m "feat(codegen): 树表前端 list/form overlay（spec §10.4）

- list.vue: vxe treeConfig(transform+全展开) + 关分页 + 无搜索表单，房规 SystemMenusPage 原样
- form.vue: parent_id 列名硬认 TreeSelect（componentProps 函数形式），
  打开时拉平铺 list 本地构树，虚拟顶级 value=0 可选
- frontTreeTemplates 登记 2 条按输出模式覆盖 single；TreeGolden 更新
- 残留/隐藏列/FrontFixtures 测试扩到单表+树表双 fixture（>=12 落盘过 vue-tsc）"
```

---

### Task 4: codegen 编辑弹窗「模板类型」Select（spec §10.5）

**Files:**
- Modify: `apps/web-antd/src/views/infra/codegen/index.vue`（EditForm schema + `onEdit` setValues）

**Interfaces:**
- Consumes: `updateCodegenTable(id, payload: Partial<CodegenTable>)`（`apps/web-antd/src/api/infra.ts` 已有 `template_type: number // 1 单表 2 树 3 主子`）；后端 `UpdateTable` 白名单 `if t.TemplateType != 0 { existing.TemplateType = t.TemplateType }`（`modules/infra/service/codegen_service.go`，已支持、仅缺 UI 入口）；`model.TemplateTypeSingle/Tree/MainSub = 1/2/3`；grid 已有 `field: 'template_type'` 列（index.vue ~71 行）。
- Produces: 编辑弹窗可切 1/2（3 置灰），PUT 携带 `template_type` 落库。无 Go 侧改动。

- [ ] **Step 1: 改 UI（先写后测，本任务无独立单测——门禁在 Step 2）**

EditForm `schema` 的 `class_name` 条目之后、`remark` 条目之前插入：

```ts
    {
      component: 'Select',
      componentProps: {
        options: [
          { label: '单表', value: 1 },
          { label: '树表', value: 2 },
          { label: '主子表（M6 开放）', value: 3, disabled: true },
        ],
      },
      fieldName: 'template_type',
      label: '模板类型',
      rules: 'required',
    },
```

`onEdit` 的 `setValues` 对象补一行（字段无序，放任意位置，保持字母序风格可放 `remark` 前）：

```ts
  void editFormApi.setValues({
    business_name: row.business_name,
    class_name: row.class_name,
    module: row.module,
    remark: row.remark,
    template_type: row.template_type,
  });
```

- [ ] **Step 2: 前端门禁**

Run: `pnpm --filter @vben/web-antd run typecheck && pnpm --filter @vben/web-antd run test`
Expected: 两者全绿（Select 是 adapter 已注册组件；`template_type` 在 `CodegenTable` 类型里）。

手工核对（读代码即可，不需起浏览器）：`handleSubmit` 把 `values` 整体发给 `updateCodegenTable` → 后端 `UpdateTable` 白名单只在 `!= 0` 时更新——单选 1/2 都非 0，生效；3 选项 `disabled` 不可选中。

- [ ] **Step 3: Commit**

```bash
git add apps/web-antd/src/views/infra/codegen/index.vue
git commit -m "feat(web-antd): codegen 编辑弹窗加模板类型 Select（spec §10.5）

单表/树表可切换，主子表（M6）置灰占位；PUT 走既有 UpdateTable 白名单。"
```

---

### Task 5: 冒烟 28→42 + 文档收口（spec §10.5）

**Files:**
- Modify: `tools/codegen/smoke_integration_test.go`
- Modify: `docs/usage.md`（§2.8 之后追加 §2.9）
- Modify: `docs/roadmap.md`（P2-1 状态行）

**Interfaces:**
- Consumes: Task 2/3 完整的树表族（14 产物）；`builder.Options{Module, TemplateType: model.TemplateTypeTree}`；约定校验（第三张表必须有 `parent_id`+`name` 列，否则 Generate 报错）；`make codegen-smoke` = `go test -tags integration ./tools/codegen/ -run TestSmoke -v -count=1`。
- Produces: 冒烟计数 42、pattern 覆盖三实体生成测试（含 `Test<X>Service_Tree` 真实执行，Review Focus ⑤ 闭环）、usage §2.9、roadmap M5 ✅。

- [ ] **Step 1: 改 smoke（计数 28→42 只在此处发生）**

`tools/codegen/smoke_integration_test.go` 四处改动：

① 建表段（原「1. 建两张冒烟表」）改为——两张单表 DDL 循环不动，其后追加树表：

```go
	// 1. 建三张冒烟表（同模块，随机后缀防残留污染）
	// 第二张表回归双表同包重声明（B1）；第三张树表回归 M5 树表族（spec §10.5：冒烟 28→42）。
	smokeTable := "codegen_smoke_item_" + randSuffix()
	smokeTable2 := "codegen_smoke_note_" + randSuffix()
	smokeTree := "codegen_smoke_tree_" + randSuffix()
	for _, name := range []string{smokeTable, smokeTable2} {
		mustExec(t, db, "DROP TABLE IF EXISTS `"+name+"`")
		mustExec(t, db, "CREATE TABLE `"+name+"`"+` (
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
	}
	// 树表：parent_id+name 固定约定（spec §10.1）。刻意不含 tenant_id——
	// 单表冒烟覆盖 HasTenant 分支，树表覆盖 else 分支，两条模板分支都有真实编译验证。
	mustExec(t, db, "DROP TABLE IF EXISTS `"+smokeTree+"`")
	mustExec(t, db, "CREATE TABLE `"+smokeTree+"`"+` (
  id bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  parent_id bigint unsigned NOT NULL DEFAULT 0 COMMENT '父级',
  name varchar(64) NOT NULL COMMENT '名称',
  remark varchar(255) DEFAULT NULL COMMENT '备注',
  created_at datetime NOT NULL COMMENT '创建时间',
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='codegen smoke tree'`)
	t.Cleanup(func() {
		mustExec(t, db, "DROP TABLE IF EXISTS "+smokeTable)
		mustExec(t, db, "DROP TABLE IF EXISTS "+smokeTable2)
		mustExec(t, db, "DROP TABLE IF EXISTS "+smokeTree)
	})
```

② 构建段改为（原 `for _, name := range []string{smokeTable, smokeTable2}` 单一 Options 循环）：

```go
	// 2. 读元数据 + 构建 + 生成（三张表 → 同一模块，前两张 single、树表 tree）
	schema := schemaFromDSN(cfg.Database.DSN)
	if schema == "" {
		t.Fatal("cannot parse schema from dsn")
	}
	var tables []*model.Table
	for _, spec := range []struct {
		name string
		opts builder.Options
	}{
		{smokeTable, builder.Options{Module: "order"}},
		{smokeTable2, builder.Options{Module: "order"}},
		{smokeTree, builder.Options{Module: "order", TemplateType: model.TemplateTypeTree}},
	} {
		meta, err := metadata.NewMySQLReader(db, schema).ReadTable(ctx, spec.name)
		if err != nil {
			t.Fatalf("ReadTable %s: %v", spec.name, err)
		}
		tbl, err := builder.Build(meta, spec.opts)
		if err != nil {
			t.Fatalf("Build %s: %v", spec.name, err)
		}
		tables = append(tables, tbl)
	}
```

③ 计数：

```go
	if len(files) != 42 {
		t.Fatalf("files = %d, want 42 (3 tables x 14)", len(files))
	}
```

④ 测试 pattern（原只拼两张表的 `Service_CRUD`）——改为三实体、前缀放开放到 `Service_`，让 `Test<X>Service_Tree` 也真实执行：

```go
	// 5. 跑三个实体生成的全部单测（CRUD + 树语义）+ vet（防重声明回归）
	classNames := make([]string, 0, len(tables))
	for _, tbl := range tables {
		classNames = append(classNames, tbl.ClassName)
	}
	pattern := "Test(" + strings.Join(classNames, "|") + ")Service_"
	out, err = exec.Command("go", "test", "./modules/order/service/",
		"-run", pattern, "-count=1").CombinedOutput()
```

（`strings` 已在文件 import 里；step 3/4 的 writeFiles/newPaths 断言是泛化的，不用动。）

- [ ] **Step 2: 跑冒烟（MySQL 不可达则如实记 Skip）**

Run: `make codegen-smoke`
Expected（MySQL 可达时）: `files = 42` 断言过、生成测试三实体全过（含树语义四断言）、`go build`/`go vet` 绿、cleanup 删三表。MySQL 不可达时 `Skip`——在提交正文/执行记录如实注明「本机未通、Skip」，**不得**据此宣称冒烟通过。

- [ ] **Step 3: 全量回归**

Run: `CGO_ENABLED=0 go test ./... && pnpm --filter @vben/web-antd run typecheck && pnpm --filter @vben/web-antd run test`
Expected: 全绿。

- [ ] **Step 4: 冒烟提交**

```bash
git add tools/codegen/smoke_integration_test.go
git commit -m "feat(codegen): 冒烟加第三张树表，28→42 产物（spec §10.5）

树表 DDL parent_id+name 走 TemplateTypeTree；pattern 放开到 Service_ 前缀，
生成的树语义测试（父不存在/自引用/有子禁删）随冒烟真实执行。"
```

- [ ] **Step 5: 文档与 roadmap 收口**

`docs/usage.md` 在 §2.8 段落之后追加（保持该文件条目风格）：

```markdown
### 2.9 树表模板族（template_type=2，M5）

- **固定约定**（spec §10.1）：表必须有 `parent_id`（整型自引用，根=0）与 `name`（varchar 显示名），
  缺列在生成入口报 `ErrColumnInvalid`（CLI 与在线预览同一路径）；导入不加配置列、无迁移。
- **后端语义**（spec §10.2）：repository 增 `HasChildren`；service `Create/Update` 前
  `validateParent`（parent_id≠0 时父存在且非自身），`Delete` 前有子拒绝；
  List 平铺返回、不加 Children 字段与 GetTree，构树在前端。
- **模板组织**（spec §10.3）：树表族 = 单表全集 14 产物，其中 5 个被 tree overlay 按输出模式覆盖——
  server `repository/service/service_test`、front `list.vue/form.vue`；其余 9 个复用单表模板。
- **前端**（spec §10.4）：`list.vue` 用 vxe `treeConfig`（transform + parentField/rowField + 全展开）、
  关分页、无搜索表单；`form.vue` 的 `parent_id` 按列名硬认 `TreeSelect`
  （打开时拉平铺 list 本地构树，虚拟顶级 value=0 可选）。
- **界面**：编辑弹窗「模板类型」下拉可切单表/树表（主子表 M6 置灰）。
- **验收**（spec §10.5）：`CGO_ENABLED=0 go test ./pkg/codegen -run TreeGolden`（14 产物）
  + `make codegen-smoke`（3 表 × 14 = 42 产物）。
```

`docs/roadmap.md` P2-1 状态列改为（提交范围用 Task 5 完成后的实际哈希填写，`git log --oneline` 取 Task 1 首笔到 Task 5 docs 笔）：

```markdown
| P2-1 | codegen M4→M5→M6 | 前端列表模板 → 树表 → 主子表（spec：`docs/superpowers/specs/2026-10-10-codegen-redesign-design.md`） | M4 ✅ `a932b56..20330df`（7 模板 + E2E + 终审），M5 ✅ `<task1-hash>..<docs-hash>`（树表族 overlay 5 模板 + 约定校验 + 冒烟 42），当前 M6 |
```

（P2-1-F 的 7 条 minors 行**不动**，仍标「不阻塞」——M5 不修。）

- [ ] **Step 6: 收口提交**

```bash
git add docs/usage.md docs/roadmap.md
git commit -m "docs(codegen): M5 树表 usage §2.9 + roadmap 记账

树表固定约定/后端语义/overlay 组织/前端树交互/验收计数四件套收口；
P2-1 记 M5 提交范围，P2-1-F 仍不阻塞。"
```

- [ ] **Step 7: 终验**

Run: `CGO_ENABLED=0 go test ./... && git log --oneline -6 && git status`
Expected: 测试全绿；5 笔任务提交（Task1–5，Task5 含两笔共 6 笔）在 main 顶上；工作区干净；**不 push**。

---

## 计划自审记录

1. **Spec §10 覆盖**：§10.1 → Task 1（校验 + 测试）；§10.2 → Task 2（三后端模板 + TreeGolden 内容钉）；§10.3 → Task 2/3（treeTemplates 按输出模式覆盖 + 产物恒 14）；§10.4 → Task 3（list treeConfig/关分页/无搜索、form 硬认 TreeSelect + 打开构树 + 根=0）；§10.5 → Task 4（模板类型 Select 主子置灰）+ Task 5（TreeGolden 验收 + 冒烟 42 + 不含浏览器 E2E）。无缺口。
2. **占位符扫描**：无 TBD/TODO/「类似 Task N」；五个模板与全部测试代码均全文内联；roadmap 提交哈希是执行期回填的指令性占位（哈希在计划期不可知），非内容缺口。
3. **类型一致性**：`validateTreeConvention(*model.Table) error`（T1 定义，T1 使用）；`treeTemplates()`/`ServerTreeTemplateNames`/`ServerTreeTemplatePattern`（T2 定义，T2/T3 测试使用）；`FrontTreeTemplateNames/Pattern`（T2 定义签名、T3 填数据，T3 测试使用）；`buildTreeFixture`/`treeFixtureMeta`/`dropColumns`（T1 定义，T2/T3 测试使用）；`TreeNode {title,value,children}`（T3 form 模板内自洽，antd fillFieldNames 默认匹配）；smoke `classNames` 与 `strings.Join`（T5 自洽，`strings` 已 import）。
4. **Review Focus**：5 条各挂一个任务的具名测试（① T1 TreeConvention、② T2 TreePaths+TreeGolden 14、③ T3 双 fixture 残留/隐藏列、④ T3 内容钉+落盘 typecheck、⑤ T2 golden 内容钉 + T5 smoke Service_ pattern 真实执行）。无未挂载条目。
