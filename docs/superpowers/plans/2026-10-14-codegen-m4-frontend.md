# codegen M4 前端单表模板族（vben5_antd）实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付 spec §3 的 7 个 vben5_antd 单表前端模板（types/api/index/list/form/drawer/api.test），接入渲染引擎、在线预览与 CLI，golden 全绿，生成页在 web-antd 本地可打开（spec §8 M4 验收）。

**Architecture:** 渲染引擎在三处扩展——`paths.go` 增 front 模板族映射（按任务渐进登记，防 ParseFS 缺文件）、`render.go` 对 `front/` 前缀切 `[[ ]]` 定界（Vue 保留 `{{ }}`）、`bind.go` 为每列计算 `TSType`/`VbenComponent`。页面模板以房规 `SystemTenantsPage.vue` 为样板：`useVbenVxeGrid` + `formOptions` 搜索表单 + `matchQuery` 客户端过滤（controller 一次性返回全量）+ connected 弹窗/抽屉（`useVbenModal`/`useVbenDrawer` + `@saved` 回刷）。E2E 用临时表 `codegen_demo_widget` 走「DDL → CLI 导入 → 改配置 → 生成 14 文件 → 临时接线 module.go/casbin → Playwright 全链路 → 全量回滚」。

**Tech Stack:** Go 1.25 `text/template`（定界 `[[ ]]`）、Vue 3 SFC + vben5（ant-design-vue、vxe-table、`@vben/common-ui`）、Vitest 4（pnpm catalog）、`vue-tsc`（typecheck）、Playwright（E2E）。

**Spec:** `docs/superpowers/specs/2026-10-10-codegen-redesign-design.md`（§3 模板树为模板全集权威；§8 M4 验收行为验收权威）

## Global Constraints

- 直接在 main 上工作、不建功能分支、**不 push**（push 由用户决定）；提交用 conventional commits（`feat(codegen): …` 等，中文正文允许）。
- 所有 go 命令显式 `cd /Users/alex/Desktop/goWork/alexGo-cloud`（cwd 会漂），一律 `CGO_ENABLED=0`；每任务提交前 `CGO_ENABLED=0 go test ./...` 全绿。
- 模板权威 = spec §3：front 7 文件，**无 index.vue**（list.vue 即页面入口，菜单 component=`views/{module}/{business}/list`；index.ts 只是 api 桶文件）。
- front 模板 Go 动作用 `[[ ]]`，Vue 插值保留 `{{ }}`；`JSONName == "-"` 的列（password/secret/token、tenant_id）在 front **所有出现处**排除（types/list/form/query/setValues/descriptions 一个不漏）。
- **计数链（每任务推进且保持全绿）**：
  - `TestGenerate_Golden`：`7 → T2:10 → T3:11 → T4:12 → T5:13 → T6:14`
  - `TestGenerate_NoUnitTest`：`6 → T2:9 → T3:10 → T4:11 → T5:12 → T6:12`（T6 不加，`.test.ts` 被 UnitTestEnable 排除；T2 起循环加 `.test.ts` 后缀检查）
  - `TestGenerate_TwoTablesSameModule_NoDuplicateTopLevelVars`：`14 → T2:20 → T3:22 → T4:24 → T5:26 → T6:28`
  - smoke `files != 14 → 28` **只在 T6 改一次**：`//go:build integration` 不进默认套件；T2–T5 间 smoke 断言暂时性过期，**不要跑 `make codegen-smoke`**。
  - `tenant_writepath_test` 的 `want >= 2` 与 `AggregatesErrors`（ErrTemplateMissing）**不用改**。
- 前端门禁：`pnpm --filter @vben/web-antd run typecheck`；vitest `pnpm --filter @vben/web-antd test`。
- E2E 临时接线（module.go、permission_routes.go、14 个生成文件、`codegen_demo_widget` 表、codegen 配置行、菜单/角色绑定、fixture 写盘）**必须全部回滚**，终态 `git status` 无残留（casbin 残留策略无害，记档即可）。
- 模板产出的代码**不跑 lint**（`pnpm lint` 不是门禁）；门禁只有 typecheck + vitest + go test。

## Review Focus

| # | 风险面（spec 静默处） | 期望行为 | 挂靠任务与测试 |
| --- | --- | --- | --- |
| 1 | `formOptions` 全仓库首次使用——查询表单值是否真能进 `query` 第二参 | 搜索框输入关键字点「查询」，列表行数按客户端过滤收缩 | **T7 Step 6**：Playwright 输入 `Alpha` 断言仅 1 行（无自动化单测可替代，属运行链路） |
| 2 | Switch/DatePicker 的提交回显与类型转换（Go 侧是 int/`*time.Time`，TS 侧是 boolean/dayjs） | 新建填开关与日期 → 保存 → 再编辑，开关状态与日期正确回显；保存后列表 formatter 显示 启用/停用 与格式化时间 | **T7 Step 6**：新建→编辑回显→改值→保存→详情 drawer 复核 |
| 3 | front 模板漏译 Go 动作或残留 `[[`，产出带花括号的死代码 | 任何 front 产物不得含未翻译 `[[`，也不得含 `{{ .X }}` 式 Go 动作 | **T2** `TestGenerate_FrontNoDelimResidue`（正则扫全部 front 输出，T3–T6 每次 Generate 持续生效）+ T5/T6 产物过 `vue-tsc` |
| 4 | `JSONName == "-"` 的列泄漏进前端产物 | front 产物全文不得出现 `password`、`tenant_id` | **T2** `TestGenerate_FrontOmitsHiddenColumns` |
| 5 | casbin fail-closed：新路由没进 permissionRoutes 或角色没绑菜单 → 页面全 403 | 页面 CRUD + 过滤全链路请求 200、按钮不消失 | **T7 Step 6**：任一请求 403 / 操作无响应即判失败 |

---

## File Structure

**创建（模板族，7 个）：**

- `pkg/codegen/template/templates/front/vben5_antd/single/types.ts.tmpl`（T2）
- `pkg/codegen/template/templates/front/vben5_antd/single/api.ts.tmpl`（T2）
- `pkg/codegen/template/templates/front/vben5_antd/single/index.ts.tmpl`（T2）
- `pkg/codegen/template/templates/front/vben5_antd/single/list.vue.tmpl`（T3）
- `pkg/codegen/template/templates/front/vben5_antd/single/form.vue.tmpl`（T4）
- `pkg/codegen/template/templates/front/vben5_antd/single/drawer.vue.tmpl`（T5）
- `pkg/codegen/template/templates/front/vben5_antd/single/api.test.ts.tmpl`（T6）

**修改（引擎与测试）：**

- `pkg/codegen/naming/naming.go`、`naming_test.go`（T1：`Pascal`）
- `pkg/codegen/template/funcs.go`、`templates_test.go`（T1：`pascal`；T2：front delims 解析）
- `pkg/codegen/template/paths.go`、`paths_test.go`（T2–T6：front 族登记 + `FrontSingleTemplateNames/Pattern` + `isTestTemplate` + 合并）
- `pkg/codegen/template/render.go`（T2：front 切 `[[ ]]`）
- `pkg/codegen/template/bind.go`、`bind_test.go`（T2：`TSType`/`VbenComponent`）
- `pkg/codegen/generate_test.go`（T2–T6：计数链 + 3 个回归钉 + T5 fixture 写盘测试）
- `tools/codegen/smoke_integration_test.go`（仅 T6：`14 → 28`）
- `apps/web-antd/package.json`、`apps/web-antd/vitest.config.ts`（T6：vitest 接线）
- `docs/usage.md`（T7：front 产物与客户端过滤说明）

**E2E 临时（T7 全量回滚）：** `modules/order/module.go`、`modules/system/service/permission_routes.go`、14 个生成文件（`modules/order/...` 7 + `apps/web-antd/src/{api,views}/order/codegen_demo_widget/...` 7）、表 `codegen_demo_widget`、`codegen_table`/`codegen_column` 配置行、menus/role_menus 菜单与角色绑定。

---

### Task 1: naming.Pascal 与模板 pascal 函数

**Files:**
- Modify: `pkg/codegen/naming/naming.go`（末尾 `capitalize` 前加 `Pascal`）
- Test: `pkg/codegen/naming/naming_test.go`
- Modify: `pkg/codegen/template/funcs.go`
- Test: `pkg/codegen/template/templates_test.go`

**Interfaces:**
- Produces: `naming.Pascal(s string) string` —— 任意标识 → PascalCase（全小写切词逐词首字母大写，**不查缩写表**）；FuncMap 键 `"pascal"`，签名 `func(string) string`。T2 起模板用 `[[pascal .Module]]` 生成组件名前缀（如 `Order`）。

- [ ] **Step 1: 写失败测试**

`pkg/codegen/naming/naming_test.go` 末尾追加（包名 `naming_test`，限定调用）：

```go
func TestPascal(t *testing.T) {
	cases := map[string]string{
		"":                  "",
		"order":             "Order",
		"codegen_demo_item": "CodegenDemoItem",
		"order_items":       "OrderItems",
		"my-page":           "MyPage",
		"Order":             "Order",
	}
	for in, want := range cases {
		if got := naming.Pascal(in); got != want {
			t.Errorf("Pascal(%q) = %q, want %q", in, got, want)
		}
	}
}
```

`pkg/codegen/template/templates_test.go` 末尾追加：

```go
func TestFuncMapPascal(t *testing.T) {
	fm := template.FuncMap()
	fn, ok := fm["pascal"]
	if !ok {
		t.Fatal("FuncMap missing pascal")
	}
	f, ok := fn.(func(string) string)
	if !ok {
		t.Fatalf("pascal type = %T", fn)
	}
	if got := f("codegen_demo_item"); got != "CodegenDemoItem" {
		t.Errorf("pascal = %q", got)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./pkg/codegen/naming/ ./pkg/codegen/template/ -run 'Pascal' -v
```

预期：编译失败 `undefined: naming.Pascal`（或 FuncMap missing pascal）。

- [ ] **Step 3: 最小实现**

`pkg/codegen/naming/naming.go` 在 `func capitalize` 之前加：

```go
// Pascal 任意标识 → PascalCase：全小写切词后逐词首字母大写，不查缩写表。
// 模块名/表名 → 页面组件名前缀（order → Order）。
func Pascal(s string) string {
	parts := nonWord.Split(strings.ToLower(s), -1)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		out = append(out, capitalize(p))
	}
	return strings.Join(out, "")
}
```

`pkg/codegen/template/funcs.go` 的 FuncMap 加一行：

```go
		"pascal": naming.Pascal,
```

- [ ] **Step 4: 跑测试确认通过 + 全量绿**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./pkg/codegen/naming/ ./pkg/codegen/template/ -run 'Pascal' -v
cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./...
```

预期：Pascal 测试 PASS；全量 PASS（计数无变化）。

- [ ] **Step 5: 提交**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud && git add pkg/codegen/naming/naming.go pkg/codegen/naming/naming_test.go pkg/codegen/template/funcs.go pkg/codegen/template/templates_test.go && git commit -m "feat(codegen): naming.Pascal 与模板 pascal 函数"
```

---

### Task 2: front 模板族引擎接线 + types/api/index 模板

**Files:**
- Modify: `pkg/codegen/template/paths.go`（front 族 3 条 + 导出 + 合并 + isTestTemplate）
- Test: `pkg/codegen/template/paths_test.go`（`TestFrontSinglePaths`）
- Modify: `pkg/codegen/template/render.go`（front 切 delims）
- Modify: `pkg/codegen/template/bind.go`（`TSType`/`VbenComponent`）
- Test: `pkg/codegen/template/bind_test.go`（`TestNewBind_FrontTypes`）
- Modify: `pkg/codegen/template/templates_test.go`（`TestTemplatesParse` front 用 `[[ ]]` 解析）
- Modify: `pkg/codegen/generate_test.go`（计数 10/9/20 + 两个回归钉 + `.test.ts` 检查）
- Create: `pkg/codegen/template/templates/front/vben5_antd/single/{types.ts,api.ts,index.ts}.tmpl`

**Interfaces:**
- Consumes: T1 的 `pascal` FuncMap；现有 `BindColumn`（`Name/Comment/GoName/GoType/JSONName/HTMLType/QueryOp/IsPK/Nullable/ListEnable/FormEnable/QueryEnable/ListRequired/FormRequired`）。
- Produces:
  - `template.FrontSingleTemplateNames() []string`、`template.FrontSingleTemplatePattern(name string) string`
  - `BindColumn.TSType string`（Go→TS：剥 `*` 后 `time.Time→string`、`json.RawMessage→any`、`bool→boolean`、`string*→string`、含 int/float→`number`、其余 `any`）
  - `BindColumn.VbenComponent string`（htmlType 空→`Input`；`InputNumber` 且 GoType 含 `string`→`Input`，防 Go ShouldBindJSON 拒 number；其余原样）
  - 三个产物路径（Business=Snake(entity)，fixture `codegen_demo_item`）：
    `apps/web-antd/src/api/{module}/{business}/types.ts`、`.../api.ts`、`.../index.ts`
  - `queryOps: Record<string,string>`（键=JSONName，值=QueryOp，仅 QueryEnable 且 JSONName≠"-" 列）与 `matchQuery<T extends object>(rows: T[], formValues: Record<string, any>): T[]`（空值跳过；eq 精确、其余子串不区分大小写）——T3 list.vue 与 T6 测试依赖

- [ ] **Step 1: 改计数 + 加测试（红）**

`pkg/codegen/generate_test.go`：

1. `TestGenerate_Golden` 计数：`if len(files) != 7` → `!= 10`，Fatalf 文案 `want 7` → `want 10`。
2. `TestGenerate_NoUnitTest`：`!= 6` → `!= 9`（文案 `want 9 (no service_test)`），循环检查改为：

```go
	for _, f := range files {
		if strings.HasSuffix(f.Path, "_test.go") {
			t.Errorf("unexpected test file %s", f.Path)
		}
		if strings.HasSuffix(f.Path, ".test.ts") {
			t.Errorf("unexpected front test file %s", f.Path)
		}
	}
```

3. `TestGenerate_TwoTablesSameModule_NoDuplicateTopLevelVars`：`!= 14` → `!= 20`（文案 `want 20`）。
4. 文件顶部 import 加 `"regexp"`。
5. 文件末尾 `pathsOf` 之前追加两个回归钉：

```go
// 回归钉（M4 Review Focus ④）：JSONName="-" 的列不得出现在任何 front 产物中。
func TestGenerate_FrontOmitsHiddenColumns(t *testing.T) {
	files, err := codegen.Generate([]*model.Table{buildFixture(t)}, codegen.Options{UnitTestEnable: true})
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
}

// 回归钉（M4 Review Focus ③）：front 产物不得残留未翻译的 Go 动作。
// [[ = front delims 残留；{{ 后接 . - 或 Go 关键字 = 用错 delims 的 Go 动作。
// Vue 插值 {{ f.value }}（空格+标识符）与 TS 对象字面量 { ... } 不在命中集。
func TestGenerate_FrontNoDelimResidue(t *testing.T) {
	files, err := codegen.Generate([]*model.Table{buildFixture(t)}, codegen.Options{UnitTestEnable: true})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	re := regexp.MustCompile(`\[\[|\{\{\s*([.\-]|(range|if|else|end|with|define|template)\b)`)
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
}
```

`pkg/codegen/template/paths_test.go` 末尾追加：

```go
// TestFrontSinglePaths 钉住 vben5_antd 前端模板族输出路径（随任务推进 3→7）。
func TestFrontSinglePaths(t *testing.T) {
	b := template.NewBind(fixtureTable())
	want := map[string]string{
		"types.ts.tmpl": "apps/web-antd/src/api/order/order_item/types.ts",
		"api.ts.tmpl":   "apps/web-antd/src/api/order/order_item/api.ts",
		"index.ts.tmpl": "apps/web-antd/src/api/order/order_item/index.ts",
	}
	names := template.FrontSingleTemplateNames()
	if len(names) != len(want) {
		t.Fatalf("front template count = %d, want %d: %v", len(names), len(want), names)
	}
	serverBase := map[string]bool{}
	for _, n := range template.ServerSingleTemplateNames() {
		serverBase[path.Base(n)] = true
	}
	for _, name := range names {
		if serverBase[path.Base(name)] {
			t.Errorf("front base name collides with server family: %s", name)
		}
		pattern := template.FrontSingleTemplatePattern(name)
		got := template.RenderPath(pattern, b)
		if short := path.Base(name); got != want[short] {
			t.Errorf("%s path = %q, want %q", name, got, want[short])
		}
	}
}
```

`pkg/codegen/template/bind_test.go` 末尾追加：

```go
// TestNewBind_FrontTypes 钉 Go→TS 类型映射与 vben 组件选型（含 decimal 降级）。
func TestNewBind_FrontTypes(t *testing.T) {
	tbl := fixtureTable()
	tbl.Columns = []model.Column{
		{Name: "id", GoName: "ID", GoType: "uint64", JSONName: "id", IsPK: true},
		{Name: "price", GoName: "Price", GoType: "string", JSONName: "price", HTMLType: model.HTMLInputNumber},
		{Name: "status", GoName: "Status", GoType: "int", JSONName: "status", HTMLType: model.HTMLSwitch},
		{Name: "created_at", GoName: "CreatedAt", GoType: "*time.Time", JSONName: "created_at", HTMLType: model.HTMLDatePicker},
		{Name: "payload", GoName: "Payload", GoType: "json.RawMessage", JSONName: "payload", HTMLType: model.HTMLTextarea},
		{Name: "secret", GoName: "Secret", GoType: "string", JSONName: "-"},
	}
	b := template.NewBind(tbl)
	cases := []struct {
		idx  int
		name string
		ts   string
		comp string
	}{
		{0, "id", "number", "Input"},           // HTMLType 空 → Input
		{1, "price", "string", "Input"},        // decimal：InputNumber→Input 防 ShouldBindJSON 拒 number
		{2, "status", "number", "Switch"},
		{3, "created_at", "string", "DatePicker"}, // *time.Time 剥指针 → string
		{4, "payload", "any", "Textarea"},
	}
	for _, c := range cases {
		col := b.Columns[c.idx]
		if col.TSType != c.ts {
			t.Errorf("%s TSType = %q, want %q", c.name, col.TSType, c.ts)
		}
		if col.VbenComponent != c.comp {
			t.Errorf("%s VbenComponent = %q, want %q", c.name, col.VbenComponent, c.comp)
		}
	}
}
```

`pkg/codegen/template/templates_test.go` 的 `TestTemplatesParse` 改为 front 用 `[[ ]]` 定界解析（否则 front 的 Go 动作语法错误永远测不到），import 加 `"strings"`：

```go
		tpl := texttpl.New(path).Funcs(template.FuncMap())
		if strings.HasPrefix(path, "templates/front/") {
			tpl.Delims("[[", "]]")
		}
		if _, perr := tpl.ParseFS(fsys, path); perr != nil {
			t.Errorf("parse %s: %v", path, perr)
		}
```

（替换原单行 `if _, perr := texttpl.New(path).Funcs(...).ParseFS(...); perr != nil {`。）

- [ ] **Step 2: 跑测试确认失败**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./pkg/codegen/... -v -run 'Golden|NoUnitTest|TwoTables|FrontOmits|FrontNoDelim' 2>&1 | tail -30
cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./pkg/codegen/template/ -v -run 'FrontSingle|FrontTypes|TemplatesParse' 2>&1 | tail -30
```

预期：编译失败（`undefined: template.FrontSingleTemplateNames`、`b.Columns[0].TSType undefined`）——红。

- [ ] **Step 3: 引擎实现**

`pkg/codegen/template/paths.go`：

1. import 加 `"maps"`。
2. `serverSingleTemplates` 声明之后加：

```go
// frontSingleTemplates：vben5_antd 单表前端模板族（spec §3）。
// 按任务渐进登记：只列已存在的模板文件（ParseFS 缺文件即整族报错）。
var frontSingleTemplates = map[string]string{
	"front/vben5_antd/single/types.ts.tmpl": "apps/web-antd/src/api/{module}/{business}/types.ts",
	"front/vben5_antd/single/api.ts.tmpl":   "apps/web-antd/src/api/{module}/{business}/api.ts",
	"front/vben5_antd/single/index.ts.tmpl": "apps/web-antd/src/api/{module}/{business}/index.ts",
}

// FrontSingleTemplateNames 返回排序后的前端模板名列表（确定性）。
func FrontSingleTemplateNames() []string {
	names := make([]string, 0, len(frontSingleTemplates))
	for n := range frontSingleTemplates {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func FrontSingleTemplatePattern(name string) string {
	return frontSingleTemplates[name]
}

// singleTemplates 合并 server+front 两族（base 名唯一性由 TestFrontSinglePaths 钉住）。
func singleTemplates() map[string]string {
	m := make(map[string]string, len(serverSingleTemplates)+len(frontSingleTemplates))
	maps.Copy(m, serverSingleTemplates)
	maps.Copy(m, frontSingleTemplates)
	return m
}
```

3. `isTestTemplate` 改：

```go
func isTestTemplate(name string) bool {
	return strings.HasSuffix(name, "_test.go.tmpl") || strings.HasSuffix(name, ".test.ts.tmpl")
}
```

4. `templateMapFor` 的 single 分支改 `return singleTemplates(), nil`。

`pkg/codegen/template/render.go`：import 加 `"strings"`；把

```go
		tpl, perr := texttpl.New(path.Base(name)).Funcs(FuncMap()).ParseFS(tplFS, "templates/"+name)
```

改为

```go
		t := texttpl.New(path.Base(name)).Funcs(FuncMap())
		// front 模板与 Vue 插值撞默认 delims → front 族切 [[ ]]（spec §3）。
		if strings.HasPrefix(name, "front/") {
			t.Delims("[[", "]]")
		}
		tpl, perr := t.ParseFS(tplFS, "templates/"+name)
```

`pkg/codegen/template/bind.go`：

1. `BindColumn` 末尾加字段：

```go
	TSType        string
	VbenComponent string
```

2. `NewBind` 的 `append` 中加两行计算：

```go
			TSType:        tsType(c.GoType),
			VbenComponent: vbenComponent(string(c.HTMLType), c.GoType),
```

3. 文件末尾加两个纯函数：

```go
// tsType Go 类型 → TS 类型（front 模板专用；先剥指针再匹配）。
func tsType(goType string) string {
	gt := strings.TrimPrefix(goType, "*")
	switch {
	case gt == "time.Time":
		return "string"
	case gt == "json.RawMessage":
		return "any"
	case gt == "bool":
		return "boolean"
	case strings.HasPrefix(gt, "string"):
		return "string"
	case strings.Contains(gt, "int"):
		return "number"
	case strings.Contains(gt, "float"):
		return "number"
	default:
		return "any"
	}
}

// vbenComponent HTMLType → 实际 vben 表单组件：
// decimal 的 GoType 是 string，InputNumber 会送 number 触发 ShouldBindJSON 拒收 → 降为 Input。
func vbenComponent(htmlType, goType string) string {
	if htmlType == "" {
		return "Input"
	}
	if htmlType == "InputNumber" && strings.Contains(goType, "string") {
		return "Input"
	}
	return htmlType
}
```

- [ ] **Step 4: 写三个模板（完整内容）**

`pkg/codegen/template/templates/front/vben5_antd/single/types.ts.tmpl`：

```
// 生成代码，勿手改（codegen M4 · [[.TableName]]）。
export interface [[.Entity]] {
[[- range .Columns ]][[ if ne .JSONName "-" ]][[ if .IsPK ]]
  [[ .JSONName ]]: [[ .TSType ]];
[[- else ]]
  [[ .JSONName ]]?: [[ .TSType ]];
[[- end ]][[ end ]][[ end ]]
}
```

（渲染效果：主键列必选、其余全可选；JSONName=="-" 的列整列跳过。）

`pkg/codegen/template/templates/front/vben5_antd/single/api.ts.tmpl`：

```
// 生成代码，勿手改（codegen M4 · [[.TableName]]）。
import { requestClient } from '#/api/request';

import type { [[.Entity]] } from './types';

/** 查询操作表：键 = JSONName，值 = query_operation 配置（eq/like/between）。 */
export const queryOps: Record<string, string> = {
[[- range .Columns ]][[ if and .QueryEnable (ne .JSONName "-") ]]
  [[ .JSONName ]]: '[[ .QueryOp ]]',
[[- end ]][[ end ]]
};

/** 客户端过滤：controller 一次性返回全量，查询表单在此过滤。
 * 空值（undefined/null/''）跳过；eq 精确匹配（String 双向），
 * like/between 走子串匹配且不区分大小写（between≈like，见 docs/usage.md）。 */
export function matchQuery<T extends object>(
  rows: T[],
  formValues: Record<string, any>,
): T[] {
  const entries = Object.entries(formValues ?? {}).filter(
    ([, v]) => v !== undefined && v !== null && v !== '',
  );
  if (entries.length === 0) {
    return rows;
  }
  return rows.filter((row) =>
    entries.every(([key, value]) => {
      const cell = String((row as any)[key] ?? '');
      const target = String(value);
      if ((queryOps[key] ?? 'eq') === 'eq') {
        return cell === target;
      }
      return cell.toLowerCase().includes(target.toLowerCase());
    }),
  );
}

/** 列表：一次性返回全量，页面用 localPage 本地切页。 */
export function list[[.Entity]](): Promise<[[.Entity]][]> {
  return requestClient.get<[[.Entity]][]>('/admin/[[.Module]]/[[.Plural]]');
}

export function create[[.Entity]](
  data: Partial<[[.Entity]]>,
): Promise<[[.Entity]]> {
  return requestClient.post<[[.Entity]]>(
    '/admin/[[.Module]]/[[.Plural]]',
    data,
  );
}

export function update[[.Entity]](
  id: number | string,
  data: Partial<[[.Entity]]>,
): Promise<[[.Entity]]> {
  return requestClient.put<[[.Entity]]>(
    `/admin/[[.Module]]/[[.Plural]]/${id}`,
    data,
  );
}

export function delete[[.Entity]](
  id: number | string,
): Promise<{ status: string }> {
  return requestClient.delete<{ status: string }>(
    `/admin/[[.Module]]/[[.Plural]]/${id}`,
  );
}
```

（注意：JS 模板字符串 `` `${id}` `` 是 TS 反引号，与 `[[ ]]` 不冲突；`queryOps`/`matchQuery` 是导出的纯函数，T6 单测直接测。）

`pkg/codegen/template/templates/front/vben5_antd/single/index.ts.tmpl`：

```
// 生成代码，勿手改（codegen M4 · [[.TableName]]）。
export * from './api';
export * from './types';
```

- [ ] **Step 5: golden 重生成 + 跑绿**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./pkg/codegen -run TestGenerate_Golden -update
cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./pkg/codegen/... -v -run 'Golden|NoUnitTest|TwoTables|FrontOmits|FrontNoDelim|FrontSingle|FrontTypes|TemplatesParse|FuncMap|Pascal' 2>&1 | tail -40
```

预期：全 PASS。golden 目录新增 3 个 `apps_web-antd_*` 文件。

- [ ] **Step 6: 全量绿 + 提交**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./...
cd /Users/alex/Desktop/goWork/alexGo-cloud && git add pkg/codegen/ && git commit -m "feat(codegen): front/vben5_antd 模板族接线（delims/paths/bind）+ types/api/index 模板"
```

（**不跑** `make codegen-smoke`——smoke 仍断言 14，暂时性过期，T6 才改。）

---

### Task 3: list.vue 模板（页面入口 + 搜索表单 + 过滤）

**Files:**
- Modify: `pkg/codegen/generate_test.go`（计数 11/10/22）
- Modify: `pkg/codegen/template/paths_test.go`（want 加 `list.vue.tmpl`）
- Modify: `pkg/codegen/template/paths.go`（front 族 +1）
- Create: `pkg/codegen/template/templates/front/vben5_antd/single/list.vue.tmpl`

**Interfaces:**
- Consumes: T2 的 `matchQuery`/`list[[.Entity]]`/`delete[[.Entity]]`、`pascal`、`TSType/VbenComponent`；`useVbenVxeGrid({ formOptions, gridOptions })`（`formOptions` 是顶层选项，vxe 原生表单已全局禁用）；`localPage(rows, currentPage, pageSize)`；`formatDateTime(value?: null|Date|number|string): string`（空值返回 `''`）。
- Produces（T4/T5 依赖）:
  - `list.vue` 导入 `./form.vue` 为 `FormModalView`、`./drawer.vue` 为 `DetailDrawerView`；connected 元组 `const [FormModal, formModalApi] = useVbenModal({ connectedComponent })`、`const [DetailDrawer, detailDrawerApi] = useVbenDrawer({ connectedComponent })`
  - 父侧握手：`onCreate = setState({title}) + setData({}) + open`（**必须清残留**）；`onEdit = setData(row) + setState + open`；`onDetail = setData(row) + open`；子组件 `@saved="onSaved"` → `gridApi.reload()`
  - grid `query: async ({ page }, formValues: Record<string, any>)`——第二参是查询表单值（vxe `extends.ts` 把最后一次提交的表单值并进来），内部 `list → matchQuery → localPage`
  - 菜单/按钮 `v-access:code="['[[.Module]]:[[.Business]]:*']"`（与 E2E 菜单 permission 逐字对应）
  - defineOptions name：`[[pascal .Module]][[.Entity]]Page`（如 `OrderCodegenDemoItemPage`）

- [ ] **Step 1: 改计数（红）**

- `generate_test.go`：Golden `!= 10` → `!= 11`（文案 want 11）；NoUnitTest `!= 9` → `!= 10`（文案 want 10）；TwoTables `!= 20` → `!= 22`。
- `paths_test.go` 的 `TestFrontSinglePaths` want 加一行：

```go
		"list.vue.tmpl": "apps/web-antd/src/views/order/order_item/list.vue",
```

- [ ] **Step 2: 跑测试确认失败**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./pkg/codegen/... -run 'Golden|NoUnitTest|TwoTables|FrontSingle' 2>&1 | tail -20
```

预期：Golden want 11 got 10、FrontSingle count 失败。

- [ ] **Step 3: 写 list.vue 模板**

`paths.go` 的 `frontSingleTemplates` 加一行：

```go
	"front/vben5_antd/single/list.vue.tmpl":   "apps/web-antd/src/views/{module}/{business}/list.vue",
```

`pkg/codegen/template/templates/front/vben5_antd/single/list.vue.tmpl` 完整内容：

```
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
  matchQuery,
} from '#/api/[[.Module]]/[[.Business]]/api';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { localPage } from '#/utils/local-page';

import DetailDrawerView from './drawer.vue';
import FormModalView from './form.vue';

defineOptions({ name: '[[pascal .Module]][[.Entity]]Page' });

[[- $label := "id" ]][[- $labelSet := false ]][[- range .Columns ]][[ if and (not $labelSet) .ListEnable (not .IsPK) (ne .JSONName "-") ]][[ $label = .JSONName ]][[ $labelSet = true ]][[ end ]][[ end ]]
const [Grid, gridApi] = useVbenVxeGrid({
  formOptions: {
    schema: [
[[- range .Columns ]][[ if and .QueryEnable (ne .JSONName "-") ]]
      {
        component: 'Input',
        fieldName: '[[ .JSONName ]]',
        label: '[[ if .Comment ]][[ .Comment ]][[ else ]][[ .Name ]][[ end ]]',
      },
[[- end ]][[ end ]]
    ],
  },
  gridOptions: {
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
    pagerConfig: {},
    proxyConfig: {
      ajax: {
        // 第二参 = 查询表单值（vxe extends.ts 把最后一次提交的表单值并进参数）。
        query: async ({ page }, formValues: Record<string, any>) => {
          const rows = await list[[.Entity]]();
          const filtered = matchQuery(rows, formValues ?? {});
          return {
            items: localPage(filtered, page.currentPage, page.pageSize),
            total: filtered.length,
          };
        },
      },
    },
    toolbarConfig: { custom: true, refresh: true, search: true, zoom: true },
  },
});

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
    title: `确认删除「${String(row.[[.Label]] ?? '')}」？`,
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

要点（实现者照抄，勿自创）：

- `$hasDate` 预扫必须在条件 import **之前**（防 noUnusedLocals：无日期列时 `formatDateTime` 不 import）。
- `$labelSet` 用首标志而非 `break`（Go 模板 `break` 在 if 嵌套内语义不必依赖），回退 `"id"`。
- 删除确认文案、`v-access` 逐字、`setData({})` 顺序均为 Review Focus 相关硬语义。
- 该模板在仓库中**不存在同名消费文件**——它只是 embed 源，门禁靠 golden + 残留/泄漏钉 + T5/T6 的 vue-tsc。

- [ ] **Step 4: golden 重生成 + 跑绿**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./pkg/codegen -run TestGenerate_Golden -update
cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./pkg/codegen/... -run 'Golden|NoUnitTest|TwoTables|FrontSingle|FrontOmits|FrontNoDelim|TemplatesParse' -v 2>&1 | tail -30
```

预期：全 PASS（FrontNoDelimResidue 现在开始扫 list.vue 输出——Vue `{{ f.value }}` 不命中，若命中说明 delims 写错）。

- [ ] **Step 5: 全量绿 + 提交**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./...
cd /Users/alex/Desktop/goWork/alexGo-cloud && git add pkg/codegen/ && git commit -m "feat(codegen): 生成 vben5 单表 list.vue 模板"
```

---

### Task 4: form.vue 模板（新建/编辑弹窗）

**Files:**
- Modify: `pkg/codegen/generate_test.go`（计数 12/11/24）
- Modify: `pkg/codegen/template/paths_test.go`（want + `form.vue.tmpl`）
- Modify: `pkg/codegen/template/paths.go`（front 族 +1）
- Create: `pkg/codegen/template/templates/front/vben5_antd/single/form.vue.tmpl`

**Interfaces:**
- Consumes: T3 的 `FormModalView`/`@saved`/`setData` 握手；`useVbenModal` child 模式（`onConfirm`/`onOpenChange` 子钩子先于父执行，`modalApi.getData()` 默认返回 `Record<string, any>`）；`useVbenForm` **必须从 `#/adapter/form` 导入**（房规同款，common-ui 的裸 hook 没有 antd 组件映射）；`VbenComponent` 生成 `component` 字面量；`VbenFormProps.handleSubmit` 的 `values` 上下文类型。
- Produces（T7 依赖）:
  - `editingId: ref<number | null>`——`null`=新建；`onOpenChange(true)` → `await resetForm()` → `getData().id != null` 则回填+置 id，否则保持新建
  - 提交：Switch 列 `Number(values.x ?? 0)`；`editingId != null ? update(id, payload) : create(payload)`；成功 → `message` + `modalApi.close()` + `emit('saved')`；失败上抛给 `onConfirm` 的 catch（拦截器已 toast，弹窗保持开）
  - DatePicker 回填 `row.x ? dayjs(row.x) : undefined`、组件 `componentProps: { showTime: true }`（**不用 valueFormat**——提交走 `dayjs.toJSON() → RFC3339 → Go time.Time`）
  - `defineEmits<{ saved: [] }>()`

- [ ] **Step 1: 改计数（红）**

- `generate_test.go`：Golden `!= 11` → `!= 12`；NoUnitTest `!= 10` → `!= 11`；TwoTables `!= 22` → `!= 24`。
- `paths_test.go` want 加：

```go
		"form.vue.tmpl": "apps/web-antd/src/views/order/order_item/form.vue",
```

- [ ] **Step 2: 跑测试确认失败**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./pkg/codegen/... -run 'Golden|NoUnitTest|TwoTables|FrontSingle' 2>&1 | tail -20
```

预期：Golden want 12 got 11、FrontSingle count 失败。

- [ ] **Step 3: 写 form.vue 模板**

`paths.go` `frontSingleTemplates` 加：

```go
	"front/vben5_antd/single/form.vue.tmpl":   "apps/web-antd/src/views/{module}/{business}/form.vue",
```

`pkg/codegen/template/templates/front/vben5_antd/single/form.vue.tmpl` 完整内容：

```
<script lang="ts" setup>
import type { [[.Entity]] } from '#/api/[[.Module]]/[[.Business]]/types';

[[- $hasDateForm := false ]][[- range .Columns ]][[ if and .FormEnable (ne .JSONName "-") (eq .HTMLType "DatePicker") ]][[ $hasDateForm = true ]][[ end ]][[ end ]][[ if $hasDateForm ]]
import dayjs from 'dayjs';
[[- end ]]
import { message } from 'ant-design-vue';
import { ref } from 'vue';

import { useVbenModal } from '@vben/common-ui';

import {
  create[[.Entity]],
  update[[.Entity]],
} from '#/api/[[.Module]]/[[.Business]]/api';
import { useVbenForm } from '#/adapter/form';

const emit = defineEmits<{ saved: [] }>();

/** null = 新建；非空 = 正在编辑的记录 id */
const editingId = ref<number | null>(null);

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

要点：

- `useVbenModal` 在 `useVbenForm` **之前**声明但只经闭包引用 `formApi`——运行时晚绑定，与房规一致；勿改成先 form 后 modal（会与 onOpenChange 的 resetForm 时序无关，但没必要动房规形状）。
- `modalApi.getData()` 用默认泛型 `Record<string, any>`：新建流父组件 `setData({})` → `row.id == null` → 保持 editingId=null。
- `[[ else if ... ]]` 是 Go 模板合法语法（else-if 链）。
- Switch 提交恒 `Number(...)`、回显恒 `Boolean(...)`——Go 侧 `tinyint→int`（无 bool 分支）。

- [ ] **Step 4: golden 重生成 + 跑绿**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./pkg/codegen -run TestGenerate_Golden -update
cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./pkg/codegen/... -run 'Golden|NoUnitTest|TwoTables|FrontSingle|FrontOmits|FrontNoDelim|TemplatesParse' -v 2>&1 | tail -30
```

预期：全 PASS。

- [ ] **Step 5: 全量绿 + 提交**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./...
cd /Users/alex/Desktop/goWork/alexGo-cloud && git add pkg/codegen/ && git commit -m "feat(codegen): 生成 vben5 单表 form.vue 模板"
```

---

### Task 5: drawer.vue 模板 + 前端产物 vue-tsc 防波堤

**Files:**
- Modify: `pkg/codegen/generate_test.go`（计数 13/12/26 + fixture 写盘测试）
- Modify: `pkg/codegen/template/paths_test.go`（want + `drawer.vue.tmpl`）
- Modify: `pkg/codegen/template/paths.go`（front 族 +1）
- Create: `pkg/codegen/template/templates/front/vben5_antd/single/drawer.vue.tmpl`

**Interfaces:**
- Consumes: T3 的 `DetailDrawerView`/`detailDrawerApi.setData+open` 握手；`useVbenDrawer` child 模式（`onOpenChange` 子钩子，无 onConfirm）；`formatDateTime`。
- Produces:
  - drawer：`onOpenChange(true)` → `getData()` 逐列生成 `items: {label, value}[]`（DatePicker→`formatDateTime`、Switch→`启用/停用`、其余空值 `'-'`）；`<Drawer :footer="false">` + `<Descriptions>`
  - `TestGenerate_FrontFixturesToDisk`（env `CODEGEN_FRONT_FIXTURE=1` 才跑）：把 fixture 的 front 产物写进 `apps/web-antd/src/api/order/codegen_demo_item/` 与 `src/views/order/codegen_demo_item/` 真实路径，供 typecheck 检查（**只写 `apps/web-antd/` 前缀产物**，server 产物不写）；调用方负责 typecheck 后删除两目录——T5/T6 类型防波堤

- [ ] **Step 1: 改计数（红）**

- `generate_test.go`：Golden `!= 12` → `!= 13`；NoUnitTest `!= 11` → `!= 12`；TwoTables `!= 24` → `!= 26`。
- `paths_test.go` want 加：

```go
		"drawer.vue.tmpl": "apps/web-antd/src/views/order/order_item/drawer.vue",
```

- [ ] **Step 2: 跑测试确认失败**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./pkg/codegen/... -run 'Golden|NoUnitTest|TwoTables|FrontSingle' 2>&1 | tail -20
```

预期：Golden want 13 got 12、FrontSingle count 失败。

- [ ] **Step 3: 写 drawer.vue 模板**

`paths.go` `frontSingleTemplates` 加：

```go
	"front/vben5_antd/single/drawer.vue.tmpl": "apps/web-antd/src/views/{module}/{business}/drawer.vue",
```

`pkg/codegen/template/templates/front/vben5_antd/single/drawer.vue.tmpl` 完整内容：

```
<script lang="ts" setup>
import type { [[.Entity]] } from '#/api/[[.Module]]/[[.Business]]/types';

[[- $hasDate := false ]][[- range .Columns ]][[ if and .ListEnable (ne .JSONName "-") (eq .HTMLType "DatePicker") ]][[ $hasDate = true ]][[ end ]][[ end ]][[ if $hasDate ]]
import { formatDateTime } from '#/utils/format';
[[- end ]]
import { Descriptions } from 'ant-design-vue';
import { ref } from 'vue';

import { useVbenDrawer } from '@vben/common-ui';

interface DetailItem {
  label: string;
  value: string;
}

const items = ref<DetailItem[]>([]);

const [Drawer, drawerApi] = useVbenDrawer({
  onOpenChange(isOpen: boolean) {
    if (!isOpen) {
      return;
    }
    const row = drawerApi.getData();
    items.value = [
[[- range .Columns ]][[ if and .ListEnable (ne .JSONName "-") ]]
      {
        label: '[[ if .Comment ]][[ .Comment ]][[ else ]][[ .Name ]][[ end ]]',
        value:
[{{- if eq .HTMLType "DatePicker" }}][[ if eq .HTMLType "DatePicker" ]]
          row.[[ .JSONName ]] ? formatDateTime(row.[[ .JSONName ]]) : '-',
[[- else if eq .HTMLType "Switch" ]]
          row.[[ .JSONName ]] == null
            ? '-'
            : Number(row.[[ .JSONName ]]) === 1
              ? '启用'
              : '停用',
[[- else ]]
          row.[[ .JSONName ]] == null || row.[[ .JSONName ]] === ''
            ? '-'
            : String(row.[[ .JSONName ]]),
[[- end ]]
      },
[[- end ]][[ end ]]
    ];
  },
});
</script>

<template>
  <Drawer :footer="false" :title="'[[.Entity]] 详情'">
    <Descriptions :column="1" bordered size="small">
      <Descriptions.Item v-for="f in items" :key="f.label" :label="f.label">
        {{ f.value }}
      </Descriptions.Item>
    </Descriptions>
  </Drawer>
</template>
```

**⚠ 上面 `value:` 分支第一行有一个笔误风险点——实现时该行只写：**

```
[[- if eq .HTMLType "DatePicker" ]]
```

（即删除任何 `{{-` 形态的行；此处唯一合法定界是 `[[ ]]`。若误写 `{{ if ... }}` 会被 FrontNoDelimResidue 钉住，但请直接写对。）

要点：

- `row.x == null` 对 `undefined/null` 都真（`getData()` 空对象时全 `'-'`）；`row.id === ''` 之类比较发生在 `any` 上（默认泛型），**无 TS2367 风险**——这就是不用 `Partial<[[.Entity]]>` 的原因。
- Switch 嵌套三元右结合，缩进如上；`Number(cellValue)` 与 list formatter 同款。
- 子 drawer 不定义 onConfirm；`:footer="false"` 隐藏默认底部按钮（lock-screen 同款）。

- [ ] **Step 4: golden 重生成 + 跑绿**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./pkg/codegen -run TestGenerate_Golden -update
cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./pkg/codegen/... -run 'Golden|NoUnitTest|TwoTables|FrontSingle|FrontOmits|FrontNoDelim|TemplatesParse' -v 2>&1 | tail -30
```

预期：全 PASS。

- [ ] **Step 5: 加 fixture 写盘测试 + 产物过 vue-tsc（类型防波堤）**

`pkg/codegen/generate_test.go` import 加 `"path/filepath"`（已有则跳过），`pathsOf` 之前追加：

```go
// TestGenerate_FrontFixturesToDisk 把 fixture 的 front 产物写进 web-antd 真实路径，
// 供 vue-tsc/vitest 做类型门禁（T5/T6 防波堤）。默认 skip：
//
//	CODEGEN_FRONT_FIXTURE=1 go test ./pkg/codegen -run FrontFixtures
//
// 写盘方负责随后删除两个目录（见计划 T5 Step 5/T6 Step 5）。
func TestGenerate_FrontFixturesToDisk(t *testing.T) {
	if os.Getenv("CODEGEN_FRONT_FIXTURE") != "1" {
		t.Skip("set CODEGEN_FRONT_FIXTURE=1 to write web-antd fixtures")
	}
	files, err := codegen.Generate([]*model.Table{buildFixture(t)}, codegen.Options{UnitTestEnable: true})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	written := 0
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
	if written < 6 {
		t.Fatalf("written front files = %d, want >= 6", written)
	}
	t.Logf("wrote %d fixture files under apps/web-antd/src (clean up after typecheck)", written)
}
```

执行（写盘 → typecheck → 清理，三步连做）：

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud && CODEGEN_FRONT_FIXTURE=1 CGO_ENABLED=0 go test ./pkg/codegen -run TestGenerate_FrontFixturesToDisk -v
cd /Users/alex/Desktop/goWork/alexGo-cloud && pnpm --filter @vben/web-antd run typecheck
cd /Users/alex/Desktop/goWork/alexGo-cloud && rm -rf apps/web-antd/src/api/order/codegen_demo_item apps/web-antd/src/views/order/codegen_demo_item
```

预期：typecheck **0 错误**（这一步是 list/form/drawer/types/api/index 六个模板的首次真实类型检查；报错就修模板并回到 Step 4 重生成 golden）。

- [ ] **Step 6: 全量绿 + 提交**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud && git status --porcelain  # 必须无 fixture 残留
cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./...
cd /Users/alex/Desktop/goWork/alexGo-cloud && git add pkg/codegen/ && git commit -m "feat(codegen): 生成 vben5 单表 drawer.vue 模板 + front 产物类型防波堤"
```

---

### Task 6: api.test.ts 模板 + vitest 接线 + smoke 计数

**Files:**
- Modify: `pkg/codegen/generate_test.go`（计数 14/12/28；NoUnit 保持 12）
- Modify: `pkg/codegen/template/paths_test.go`（want + `api.test.ts.tmpl`，共 7）
- Modify: `pkg/codegen/template/paths.go`（front 族 +1 = 7，齐）
- Create: `pkg/codegen/template/templates/front/vben5_antd/single/api.test.ts.tmpl`
- Modify: `tools/codegen/smoke_integration_test.go`（`!= 14` → `!= 28`）
- Modify: `apps/web-antd/package.json`（`test` 脚本 + vitest devDep）
- Create: `apps/web-antd/vitest.config.ts`

**Interfaces:**
- Consumes: T2 的 `matchQuery`/`queryOps`；`isTestTemplate` 已在 T2 识别 `.test.ts.tmpl`（T6 无需再改）。
- Produces:
  - 产物路径 `apps/web-antd/src/api/{module}/{business}/__tests__/{business}.test.ts`
  - web-antd 脚本 `"test": "vitest run --passWithNoTests"`；`vitest.config.ts` alias `#`→`./src`、`environment: 'node'`、`include: ['src/**/*.test.ts']`
  - 测试骨架必须 `vi.mock('#/api/request')`（防 request.ts 在 node 环境重链）
  - smoke 断言 `files != 28`（双表 × 14）

- [ ] **Step 1: 改计数（红）**

- `generate_test.go`：Golden `!= 13` → `!= 14`；NoUnitTest **保持 `!= 12`**（`.test.ts` 被 UnitTestEnable 排除，后缀检查已在 T2 加好）；TwoTables `!= 26` → `!= 28`。
- `paths_test.go` want 加（此后 7 条齐）：

```go
		"api.test.ts.tmpl": "apps/web-antd/src/api/order/order_item/__tests__/order_item.test.ts",
```

- [ ] **Step 2: 跑测试确认失败**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./pkg/codegen/... -run 'Golden|NoUnitTest|TwoTables|FrontSingle' 2>&1 | tail -20
```

预期：Golden want 14 got 13、FrontSingle count 6≠7。

- [ ] **Step 3: 写 api.test.ts 模板**

`paths.go` `frontSingleTemplates` 加（最后一条）：

```go
	"front/vben5_antd/single/api.test.ts.tmpl": "apps/web-antd/src/api/{module}/{business}/__tests__/{business}.test.ts",
```

`pkg/codegen/template/templates/front/vben5_antd/single/api.test.ts.tmpl` 完整内容：

```
// 生成代码，勿手改（codegen M4 · [[.TableName]]）。
import { describe, expect, it, vi } from 'vitest';

import { matchQuery, queryOps } from '../api';

// 隔离 request.ts（axios 链在 node 环境不可用）：matchQuery 是纯函数，但同模块会 import 客户端。
vi.mock('#/api/request', () => ({
  requestClient: {
    delete: vi.fn(),
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
  },
}));

interface Row {
  id?: number;
  name?: string;
  status?: number;
}

const rows: Row[] = [
  { id: 1, name: 'Alpha', status: 1 },
  { id: 2, name: 'beta', status: 0 },
  { id: 3, name: '', status: 1 },
];

describe('matchQuery', () => {
  it('空表单值返回全部行', () => {
    expect(matchQuery(rows, {})).toHaveLength(3);
    expect(matchQuery(rows, { name: '' })).toHaveLength(3);
    expect(matchQuery(rows, { name: null })).toHaveLength(3);
    expect(matchQuery(rows, { name: undefined })).toHaveLength(3);
  });

  it('eq 精确匹配且 String 双向转换', () => {
    expect(matchQuery(rows, { id: 1 })).toHaveLength(1);
    expect(matchQuery(rows, { id: '1' })).toHaveLength(1);
    expect(matchQuery(rows, { id: 99 })).toHaveLength(0);
  });

  it('like 子串匹配且不区分大小写', () => {
    expect(matchQuery(rows, { name: 'ALP' })).toHaveLength(1);
    expect(matchQuery(rows, { name: 'et' })).toHaveLength(1);
  });

  it('多条件 AND 组合', () => {
    expect(matchQuery(rows, { name: 'a', status: 1 })).toHaveLength(1);
  });
});

describe('queryOps', () => {
  it('按列类型给出操作符（string→like、time→between、其余 eq）', () => {
    expect(queryOps.id).toBe('eq');
    expect(queryOps.name).toBe('like');
    expect(queryOps.created_at).toBe('between');
  });
});
```

（`queryOps` 断言钉 builder 默认：fixture 的 `created_at` 列 QueryOp=`between`、`name` varchar→`like`、`id`→`eq`；JSONName=="-" 的 password/tenant_id 不在表内。）

- [ ] **Step 4: golden 重生成 + 跑绿**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./pkg/codegen -run TestGenerate_Golden -update
cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./pkg/codegen/... -v -run 'Golden|NoUnitTest|TwoTables|FrontSingle|FrontOmits|FrontNoDelim|TemplatesParse' 2>&1 | tail -30
```

预期：全 PASS（NoUnit 仍 12：`.test.ts` 被排除；Golden=14、TwoTables=28）。

- [ ] **Step 5: vitest 接线 + 产物实测（写盘 → typecheck → vitest → 清理）**

1. `apps/web-antd/package.json`：

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud && pnpm --filter @vben/web-antd add -D vitest@catalog:
```

scripts 中加：

```json
    "test": "vitest run --passWithNoTests",
```

2. 创建 `apps/web-antd/vitest.config.ts`：

```ts
import { fileURLToPath } from 'node:url';

import { defineConfig } from 'vitest/config';

export default defineConfig({
  resolve: {
    alias: {
      '#': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts'],
  },
});
```

3. 三连（fixture 含 `.test.ts` 共 7 个 front 文件）：

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud && CODEGEN_FRONT_FIXTURE=1 CGO_ENABLED=0 go test ./pkg/codegen -run TestGenerate_FrontFixturesToDisk -v
cd /Users/alex/Desktop/goWork/alexGo-cloud && pnpm --filter @vben/web-antd run typecheck
cd /Users/alex/Desktop/goWork/alexGo-cloud && pnpm --filter @vben/web-antd test
cd /Users/alex/Desktop/goWork/alexGo-cloud && rm -rf apps/web-antd/src/api/order/codegen_demo_item apps/web-antd/src/views/order/codegen_demo_item
```

预期：typecheck 0 错误；vitest **6 个断言用例全过**（4 matchQuery + 1 queryOps + …）。之后再跑一次 `pnpm --filter @vben/web-antd test` 应 `No test files found, exiting with 0`（--passWithNoTests，保 CI 绿）。

- [ ] **Step 6: smoke 计数 14 → 28**

`tools/codegen/smoke_integration_test.go`：

```go
	if len(files) != 14 {
		t.Fatalf("files = %d, want 14", len(files))
	}
```

改为：

```go
	if len(files) != 28 {
		t.Fatalf("files = %d, want 28", len(files))
	}
```

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud && make codegen-smoke
```

预期：PASS（双表 × 14 文件，含 `go build ./modules/order/...` 与生成单测/vet）。

- [ ] **Step 7: 全量绿 + 提交**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud && git status --porcelain   # 无 fixture 残留；预期新增 vitest.config.ts、package.json、pnpm-lock.yaml 变更
cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./...
cd /Users/alex/Desktop/goWork/alexGo-cloud && git add -A && git commit -m "feat(codegen): 生成 api.test.ts 模板 + web-antd vitest 接线 + smoke 计数 28"
```

---

### Task 7: E2E 全链路验收（生成页可打开）+ 文档 + 回滚

**Files:**
- Modify（临时，回滚）: `modules/order/module.go`、`modules/system/service/permission_routes.go`
- 生成（临时，回滚）: 14 个产物文件 + 表 `codegen_demo_widget` + `codegen_table`/`codegen_column` 行 + menus/role_menus 行
- Modify（永久）: `docs/usage.md`

**Interfaces:**
- Consumes: 全部 T1–T6 产物；server 模板构造器 `New{Entity}Repository(db *gorm.DB)` / `New{Entity}Service(repo)` / `NewAdmin{Entity}Controller(svc)` 与 `Register{Entity}DBRoutes(r *gin.RouterGroup, c *adminctrl.Admin{Entity}Controller)`（内部 `r.Group("/admin/{module}")`）；`permissionRoutes` map 结构；`BusinessName=Snake(entity)`。
- Produces: M4 验收证据（页面可开、CRUD/过滤/回显全通）、`docs/usage.md` 的 front 产物与客户端过滤说明；**代码树终态 = T1–T6 提交 + docs 提交，无任何 E2E 残留**。

**前置检查（Step 0）：**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud && mysql -uroot -proot -e "SELECT 1" && CGO_ENABLED=0 go test ./... && pnpm --filter @vben/web-antd run typecheck
```

（DSN 以 `alexgo-server/configs/config.yaml` 的 `database.dsn` 为准；任一不过先修再进 E2E。）

- [ ] **Step 1: 建演示表 + 种子数据**

```bash
mysql -uroot -proot alexgo <<'SQL'
CREATE TABLE codegen_demo_widget (
  id bigint NOT NULL AUTO_INCREMENT,
  name varchar(64) NOT NULL,
  price decimal(10,2) NOT NULL,
  status tinyint(1) NOT NULL,
  remark varchar(255) NULL,
  created_at datetime NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
INSERT INTO codegen_demo_widget (name, price, status, remark, created_at) VALUES
  ('Alpha widget', 12.50, 1, 'first', NOW()),
  ('beta widget', 99.99, 0, NULL, NOW());
SQL
```

**关键裁决（勿改 DDL）**：`created_at` 必须 `datetime NULL`——BuildGormTag 无 default，GORM 插入零值 `time.Time` 会落 `'0001-01-01'`，超出 DATETIME 范围（min 1000-01-01）直接报错；NULL 列 → GoType `*time.Time` → 空表单提交 NULL。无 tenant_id（租户插件按字段存在性跳过，`pkg/tenant/gormplugin` 已测）。

- [ ] **Step 2: CLI 导入 → 改配置 → 生成 14 文件**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go run ./tools/codegen --module=order --tables=codegen_demo_widget --import --dry-run
```

预期：打印 14 个计划路径（7 server + 7 front），不写盘（`--import` 已落配置表）。

```bash
mysql -uroot -proot alexgo <<'SQL'
UPDATE codegen_column
   SET query_enable = 1, query_operation = 'like'
 WHERE table_id = (SELECT id FROM (SELECT id FROM codegen_table WHERE table_name = 'codegen_demo_widget') t)
   AND name = 'name';
UPDATE codegen_column
   SET html_type = 'Switch'
 WHERE table_id = (SELECT id FROM (SELECT id FROM codegen_table WHERE table_name = 'codegen_demo_widget') t)
   AND name = 'status';
SQL
```

（导入默认 `html_type` 看 GoType：`tinyint→int→InputNumber`，故 status 必须改成 `Switch`；`name` 的查询开关按需打开。）

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go run ./tools/codegen --module=order --tables=codegen_demo_widget
```

预期：`生成 14 / 跳过 0 / 失败 0`。

- [ ] **Step 3: 临时后端接线（fx + casbin 路由表）**

`modules/order/module.go`：

1. import 加生成物：

```go
	admin "alexGo-cloud/modules/order/controller/admin"
	"alexGo-cloud/modules/order/repository"
	"alexGo-cloud/modules/order/service"
```

（若已有同名 import 按现有别名并入；`module_register_codegen_demo_widget.go` 落包名核对后照抄。）

2. `OrderModuleParams` 加字段：

```go
	DemoCtrl *admin.AdminCodegenDemoWidgetController
```

3. `RegisterRoutes` 中 `adminGroup` 之后加：

```go
	repo := repository.NewCodegenDemoWidgetRepository(m.db) // 形参名以现有 NewOrderModule 签名为准
	...
```

**实现者注意**：构造器注入方式以 fx 装配现状为准——最小侵入做法是 `fx.Provide` 三行 + params 注入 controller，然后在 `RegisterRoutes` 调 `RegisterCodegenDemoWidgetDBRoutes(adminGroup所属的 /api 根组, m.DemoCtrl)`。**函数内部自己 `Group("/admin/order")`，所以必须传 `/api` 根组**（不是已 Group 过的 `adminGroup`），路由最终为 `GET/POST /api/admin/order/codegen_demo_widgets` 与 `.../:id`。以 `module_register.go.tmpl` 渲染产物为准，写完 `CGO_ENABLED=0 go build ./modules/order/...` 验证。

`modules/system/service/permission_routes.go` 的 `permissionRoutes` map 加一行（照抄 `order:order` 格式）：

```go
	"order:codegen_demo_widget": {"/api/admin/order/codegen_demo_widgets", "/api/admin/order/codegen_demo_widgets/*"},
```

（casbin fail-closed：permPrefix 取前两段，菜单 permission `order:codegen_demo_widget:*` → 命中本行。）

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go build ./modules/order/... && CGO_ENABLED=0 go test ./modules/system/service/ ./modules/order/...
```

预期：build + 测试全过。**随后重启后端**（`make run`，另开后台）让新路由生效。

- [ ] **Step 4: 生成物门禁（真实产物过三关）**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud && pnpm --filter @vben/web-antd run typecheck
cd /Users/alex/Desktop/goWork/alexGo-cloud && pnpm --filter @vben/web-antd test
cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./...
```

预期：typecheck 0 错误；vitest **跑真实生成的 `__tests__/codegen_demo_widget.test.ts` 全过**；go 全绿。

- [ ] **Step 5: 起前端 + Playwright 全链路**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud && pnpm --filter @vben/web-antd dev   # :5666，后台
```

用 Playwright MCP 浏览器执行（登录 **admin / admin123**）：

1. **菜单**：菜单管理 → 新建目录「订单部件」（或挂到已有订单目录）→ 新建菜单：名称「部件管理」、path `/order/codegen_demo_widget`、component `views/order/codegen_demo_widget/list`、permission `order:codegen_demo_widget:*`、type=菜单。
2. **角色**：角色管理 → 管理员角色 → 分配菜单（勾选「部件管理」）→ 保存；必要时**重新登录**刷新权限码。
3. **打开页面**：侧栏点「部件管理」→ 列表渲染 2 行（Alpha/beta），列含 价格/状态/创建时间 formatter（启用/停用、`YYYY-MM-DD HH:mm:ss`）。
4. **过滤（Review Focus ①）**：搜索表单输入 名称=`Alpha` → 点「查询」→ 仅 1 行；清空 → 「查询」→ 2 行。
5. **新建（Review Focus ②）**：新建 → 名称 `Gamma`、价格 `1.5`、状态开关开、备注 `created by e2e`、创建时间选一个 → 创建成功 toast → 列表 3 行。
6. **编辑回显（Review Focus ②）**：编辑 Gamma → 表单回显（名称/价格/开关=开/日期已填）→ 改备注 `edited` → 保存成功。
7. **详情**：详情抽屉 → descriptions 含正确值（状态「启用」、时间已格式化、备注 edited）。
8. **删除**：删除 Gamma → 确认框带名称 → 确认 → 2 行。
9. **权限（Review Focus ⑤）**：全程任一 XHR 返回 403、或按钮因权限消失 → **判失败**，先查 permission_routes/角色绑定。

（若实现者无浏览器工具，此 Step 交回控制器执行——报告 NEEDS_CONTEXT 并附上已完成的 Step 1–4 状态。）

- [ ] **Step 6: smoke + 文档**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud && make codegen-smoke
```

预期：PASS（28）。

`docs/usage.md` 在 `### 2.7 CLI（可选）` 之后新增：

```markdown
### 2.8 前端产物（vben5_antd，M4 起）

单表生成在后端 7 个 Go 文件之外，额外产出 7 个前端文件（spec §3）：

| 文件 | 说明 |
| --- | --- |
| `apps/web-antd/src/api/{module}/{business}/types.ts` | 实体 TS 类型（主键必选、其余可选；`json:"-"` 列不出现） |
| `api.ts` | `list/create/update/delete` + `queryOps` + `matchQuery` |
| `index.ts` | 桶文件（`export *`） |
| `apps/web-antd/src/views/{module}/{business}/list.vue` | 页面入口（菜单 component 填 `views/{module}/{business}/list`） |
| `form.vue` | 新建/编辑弹窗（connected 模式，`@saved` 回列表） |
| `drawer.vue` | 详情抽屉 |
| `api/{module}/{business}/__tests__/{business}.test.ts` | Vitest 骨架（`--no-tests` 或 `codegen.unit_test_enable=false` 时跳过） |

**查询语义**：controller 一次性返回全量，列表在客户端过滤（`matchQuery`）：

- `eq`：`String` 双向精确匹配；
- `like` / `between`：子串匹配、不区分大小写（`between` 按子串近似处理，未实现区间）；
- 空值（`undefined/null/''`）跳过该条件。

**列类型映射**：`tinyint(1)→Switch`（提交 Number、回显 Boolean、列表 启用/停用）、
`datetime→DatePicker(showTime)`、`decimal→Input`（防 Go 侧 string 收到 number）、
`json→Textarea`。开关/日期的回填与提交由 form 模板统一处理，无需手改。
```

- [ ] **Step 7: 全量回滚（E2E 痕迹一个不留）**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud && git checkout -- modules/order/module.go modules/system/service/permission_routes.go
cd /Users/alex/Desktop/goWork/alexGo-cloud && rm -f modules/order/model/codegen_demo_widget.go \
  modules/order/repository/codegen_demo_widget.go \
  modules/order/service/codegen_demo_widget.go \
  modules/order/service/codegen_demo_widget_test.go \
  modules/order/controller/admin/codegen_demo_widget.go \
  modules/order/controller/app/codegen_demo_widget.go \
  modules/order/module_register_codegen_demo_widget.go
cd /Users/alex/Desktop/goWork/alexGo-cloud && rm -rf apps/web-antd/src/api/order/codegen_demo_widget apps/web-antd/src/views/order/codegen_demo_widget
mysql -uroot -proot alexgo <<'SQL'
DELETE FROM codegen_column WHERE table_id IN (SELECT id FROM (SELECT id FROM codegen_table WHERE table_name='codegen_demo_widget') x);
DELETE FROM codegen_table WHERE table_name = 'codegen_demo_widget';
DROP TABLE IF EXISTS codegen_demo_widget;
DELETE FROM role_menus WHERE menu_id IN (SELECT id FROM (SELECT id FROM menus WHERE path LIKE '%codegen_demo_widget%') y);
DELETE FROM menus WHERE path LIKE '%codegen_demo_widget%';
SQL
```

（若 Step 5 是在 UI 建的菜单，以上 SQL 删除为准——目录若为本次新建可一并删；casbin 中 `order:codegen_demo_widget` 的残留策略无害，记档不清理。前端 dev 服务与后端进程可停。）

- [ ] **Step 8: 终态核验 + 提交 docs**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud && git status --porcelain   # 只允许 docs/usage.md 与 T6 的既有改动
cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./...
cd /Users/alex/Desktop/goWork/alexGo-cloud && pnpm --filter @vben/web-antd run typecheck
cd /Users/alex/Desktop/goWork/alexGo-cloud && git add docs/usage.md && git commit -m "docs(usage): codegen 前端产物清单与客户端过滤语义"
```

**M4 验收对照（spec §8）**：vben5_antd 单表模板族 7 文件 ✅（T2–T6）；接入预览与 CLI ✅（paths/render 全链 + T7 Step 2 dry-run 14）；golden 通过 ✅；生成页在 web-antd 本地可打开 ✅（T7 Step 5，验收后回滚为计划既定终态）。
