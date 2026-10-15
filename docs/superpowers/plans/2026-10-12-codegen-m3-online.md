# 代码生成在线链路（M3）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 打通 spec §8 M3 在线链路——配置行→引擎模型映射器 + `POST preview/download` API + 后台预览 Modal/ZIP 下载 + CLI 改读配置表（`--import` 现场导入）+ `make generate-*` 切换，使「导入→配置→预览→ZIP」浏览器验收与 CLI 用法等价全部通过。

**Architecture:** 核心新增是**配置行→`model.Table` 映射器**（`TableFromConfig`，预览/ZIP/CLI 三方共用）：`GoName`/`GormTag` 配置表不存须按列名/主键/自增重算，deprecated 列剔除，其余开关取存值。service 层新增 `Preview`（纯内存渲染）/`Download`（同一份渲染结果打 ZIP），controller 落两个 POST 端点取代 `GET generate` 占位；CLI 默认配置表模式（过滤→`svc.Preview`→写盘），`-import` 分支现场读元数据入配置。前端在现有 modal 结构上加预览 Modal（文件树+代码+下载 ZIP），启用工具栏「生成代码」与行内「预览/下载」。

**Tech Stack:** Go 1.25（gin、gorm + glebarez/sqlite 单测、archive/zip）、fx、Vue 3 + vben5（useVbenModal/useVbenVxeGrid、RequestClient.download）、text/template（已有引擎）。

**Spec:** `docs/superpowers/specs/2026-10-10-codegen-redesign-design.md`（本计划论据均出自 §2/§4/§5/§6/§7/§8/§9）

## Global Constraints

- spec M3 范围（§8）：预览/ZIP API + 后台页面能力 + CLI 改读配置表 + `make generate-*` 切换；**M4（前端模板族）/M5（树表）/M6（主子表）/M7（删旧工具、codegen-smoke 文档化）不在本计划**。
- 单向依赖：`modules/system/infra` → `pkg/codegen`；`pkg/codegen` 不 import `modules/*`。映射器放 `modules/system/infra/service`（CLI 亦可 import）。
- 响应信封对齐仓库惯例：预览 `{data:[{path,content}]}`、下载 `application/zip` 流 + `Content-Disposition`、错误 `{error}`；HTTP 只用 **400/500**，业务/配置/生成类错误一律 400（对齐 getTable/import 先例）。
- **预览/ZIP 纯内存**（spec §6）：引擎失败零副作用；CLI 全部渲染成功才写盘；写盘沿用「已存在不覆盖，`-force` 才覆盖」。
- **权限**：`permission_routes.go` 已有 `infra:codegen` + `/api/admin/infra/codegen/*` 通配，新端点免迁移；前端预览/下载按钮复用页面权限码 `infra:codegen`（无新按钮码，免 seed）。
- 系统级功能无租户隔离；`codegen_table`/`codegen_column` 硬删除语义不变。
- `UnitTestEnable`：service 与 CLI 统一读 `config.Codegen.UnitTestEnable`（loader 默认 true），CLI `-no-tests` 覆盖为 false。
- 提交直接落 main（项目约定，不建功能分支）；每任务一次提交，风格 `feat(codegen):` / `feat(web-antd):` / `feat(build):` / `docs:`。
- 每任务 TDD：先写失败测试 → 确认失败 → 最小实现 → 通过 → 提交。**例外（已裁定）**：Task 4 前端无单测设施（spec §7 golden 留 M4+），验证门 = `vue-tsc` typecheck；Task 6/8 为构建脚本与浏览器验收，验证门 = `make -n`/实跑输出与 UI 全链路。
- 预存环境问题（不修）：cgo/Security.tbd 链接失败影响 5 包（alexgo-server/cmd|server、modules/member/cmd、modules/system、modules/system/cmd）——`go build ./...`/`go test ./...` 挂这 5 包属已裁定预存，按包范围跑测试并如实记录。
- **已裁定的 spec 偏离**（写入文档，Task 7 落 usage.md）：
  ① spec §5「后台三页面」→ 沿用 M2 已交付的 modal 结构（导入/字段配置/编辑），M3 只**新增**预览 Modal；
  ② spec §5.3「代码高亮」→ 纯 `<pre>` 等宽展示（仓库无高亮依赖，YAGNI，M4 可加）；
  ③ spec §5 CLI 示例 `--dsn=... --tables=t1 --import` 未带 `--module` → 实现**要求 `-import` 必须配 `-module`**（导入要写目标模块），错误信息明确提示；
  ④ spec 伪代码 body `tableIds[]` → 实际 `{"table_ids":[...]}`（仓库 snake_case JSON 惯例）。

## Review Focus

以下 5 类输入/状态最可能咬到使用者，每条在所属任务中有钉死它的测试：

1. **映射器漏重算**：`GoName`/`GormTag` 配置表根本不存（CodegenColumn 无此字段），直接拼会产出无 gorm tag / 空字段名的代码 → Task 1 `TestTableFromConfig_RecomputesAndKeepsConfig`：断言 `GoName(id)=="ID"`、id 的 GormTag 含 `column:id`+`primaryKey`+`autoIncrement`（BuildGormTag 实际输出驼峰，已核实 typing.go:93）、name 的 GormTag 恰为 `column:name`、JSONName/开关取存值。
2. **deprecated 列仍进生成**：同步标废弃的列若被渲染，产物引用已从库消失的列 → `go build` 必挂 → Task 1 `TestTableFromConfig_SkipsDeprecated`：单列 deprecated 被剔除；全废弃 → `errors.Is(ErrColumnInvalid)`。
3. **空/缺 `table_ids` 被当成全表**：body `{}` 或 `[]` 若静默生成全部表，误渲染+慢请求 → Task 2 service 空入参报 `ErrTableInvalid` + Task 3 `TestPreview_EmptyIDs_400`（`{}`、`[]`、无 body 三例均 400）。
4. **ZIP 与预览不同源/坏流**：zip entry 缺文件、路径带反斜杠、或 Content-Type 不对 → 浏览器解压失败 → Task 2 `TestM3_Acceptance_PreviewDownload`：`PK` magic + entry 名集合与 preview paths 集合逐一对齐且计数相等；Task 3 断言 `Content-Type: application/zip` 与 `Content-Disposition`。
5. **CLI 过滤器静默丢表**：`-module`/`-tables` 过滤后空匹配却继续走 → 报「生成 0」假成功 → Task 5 `TestFilterConfigTables`：无匹配返回**非 nil error**（含修复建议文案）+ 两过滤器保留正确子集。

---

## File Structure

```
modules/system/infra/service/config_to_table.go                    # Task 1：TableFromConfig 映射器
modules/system/infra/service/config_to_table_test.go                # Task 1：表驱动/边界测试
modules/system/infra/service/codegen_service.go                     # Task 2：接口 + Preview/Download/generate/zipFiles；ctor 加 unitTestEnable
modules/system/infra/service/codegen_service_test.go                # Task 2：newSvc 改签名 + Preview/Download 错误测试
modules/system/infra/service/acceptance_test.go                     # Task 2：TestM3_Acceptance_PreviewDownload
modules/system/infra/module.go                                      # Task 2：newCodegenService 注入 *config.Config
modules/system/infra/controller/admin/codegen_controller.go         # Task 3：POST preview/download；删 generate 占位
modules/system/infra/controller/admin/codegen_controller_test.go    # Task 3：stub 加两方法；占位测试换新三测试
apps/web-antd/src/api/infra.ts                               # Task 4：CodegenFile/previewCodegen/downloadCodegen/saveBlob
apps/web-antd/src/views/infra/codegen/index.vue              # Task 4：预览 Modal + 行内预览/下载 + 启用工具栏按钮
tools/codegen/main.go                                        # Task 5：配置表默认模式 + -import 分支 + filterConfigTables
tools/codegen/main_test.go                                   # Task 5：TestFilterConfigTables
Makefile                                                     # Task 6：generate-crud / generate-db-crud 换实现
docs/usage.md                                                # Task 7：§2 界面/API/CLI 全面改写（去 M3 占位）
README.md                                                    # Task 7：命令表两行
```

---

### Task 1: 配置行 → 引擎模型映射器（TableFromConfig）

**Files:**
- Create: `modules/system/infra/service/config_to_table.go`
- Test: `modules/system/infra/service/config_to_table_test.go`

**Interfaces:**
- Consumes: `modules/system/infra/model.CodegenTable{Name,TableComment,Module,BusinessName,ClassName,TemplateType int,FrontType int,ParentTableID uint64,Remark}` / `CodegenColumn{Name,Type,Comment,GoType,JSONName,IsPK,AutoIncrement,Nullable,HTMLType,ListEnable,FormEnable,QueryEnable,QueryOperation,ListRequired,FormRequired,DictType,SortOrder,Deprecated}`；`pkg/codegen/naming.ToGoName(col string) string`；`pkg/codegen/builder.BuildGormTag(c metadata.ColumnMeta) string`（只用 Name/Key/Extra）；`pkg/codegen/model.Table/Column/TemplateTypeSingle/FrontTypeVben5Antd/ErrTableInvalid/ErrColumnInvalid`。
- Produces: `func TableFromConfig(t *model.CodegenTable, cols []*model.CodegenColumn) (*cgmodel.Table, error)` —— Task 2（service 生成链 `generate()`）直接调用；Task 5 的 CLI 经 `svc.Preview` 间接受益。注意 `CodegenTable.ParentTableID` 是 **uint64** 而 `cgmodel.Table.ParentTableID` 是 **int64**，须转换。

- [ ] **Step 1: 写失败测试**

新建 `modules/system/infra/service/config_to_table_test.go`：

```go
package service

import (
	"errors"
	"strings"
	"testing"

	"alexGo-cloud/modules/system/infra/model"
	cgmodel "alexGo-cloud/pkg/codegen/model"
)

// 映射器核心：GoName/GormTag 配置表不存须重算，JSONName/开关取存值（Review Focus #1）。
func TestTableFromConfig_RecomputesAndKeepsConfig(t *testing.T) {
	tbl := &model.CodegenTable{
		Name: "order_items", TableComment: "订单明细",
		Module: "order", BusinessName: "order_item", ClassName: "OrderItem",
		TemplateType: 1, FrontType: 1, Remark: "r", ParentTableID: 0,
	}
	cols := []*model.CodegenColumn{
		{Name: "id", Type: "bigint unsigned", GoType: "uint64", JSONName: "id",
			IsPK: true, AutoIncrement: true, QueryOperation: "eq", SortOrder: 0},
		{Name: "tenant_id", Type: "bigint unsigned", GoType: "uint64",
			JSONName: "-", SortOrder: 1},
		{Name: "name", Type: "varchar(64)", Comment: "名称", GoType: "string",
			JSONName: "name", HTMLType: "Input", ListEnable: true, FormEnable: true,
			QueryEnable: true, QueryOperation: "like", DictType: "status", SortOrder: 2},
	}

	got, err := TableFromConfig(tbl, cols)
	if err != nil {
		t.Fatalf("TableFromConfig: %v", err)
	}
	if got.TableName != "order_items" || got.Module != "order" ||
		got.ClassName != "OrderItem" || got.BusinessName != "order_item" ||
		got.TableComment != "订单明细" || got.Remark != "r" {
		t.Errorf("table fields: %+v", got)
	}
	if got.TemplateType != cgmodel.TemplateTypeSingle || got.FrontType != cgmodel.FrontTypeVben5Antd {
		t.Errorf("enum cast: template=%d front=%d", got.TemplateType, got.FrontType)
	}
	if !got.HasTenant {
		t.Error("含 tenant_id → HasTenant 应为 true")
	}
	if len(got.Columns) != 3 {
		t.Fatalf("columns=%d, want 3", len(got.Columns))
	}
	// SortOrder 决定输出顺序
	if got.Columns[0].Name != "id" || got.Columns[1].Name != "tenant_id" || got.Columns[2].Name != "name" {
		t.Errorf("order: %s,%s,%s",
			got.Columns[0].Name, got.Columns[1].Name, got.Columns[2].Name)
	}
	idCol := got.Columns[0]
	if idCol.GoName != "ID" {
		t.Errorf("GoName(id)=%q, want ID（naming.ToGoName 重算）", idCol.GoName)
	}
	if !strings.Contains(idCol.GormTag, "column:id") ||
		!strings.Contains(idCol.GormTag, "primaryKey") ||
		!strings.Contains(idCol.GormTag, "autoIncrement") {
		t.Errorf("id GormTag=%q", idCol.GormTag)
	}
	nameCol := got.Columns[2]
	if nameCol.GoName != "Name" || nameCol.GormTag != "column:name" {
		t.Errorf("name: GoName=%q GormTag=%q", nameCol.GoName, nameCol.GormTag)
	}
	if !nameCol.ListEnable || !nameCol.FormEnable || !nameCol.QueryEnable ||
		nameCol.QueryOp != "like" || nameCol.DictType != "status" || nameCol.HTMLType != cgmodel.HTMLInput {
		t.Errorf("config flags lost: %+v", nameCol)
	}
	if got.Columns[1].JSONName != "-" {
		t.Errorf("tenant_id JSONName=%q, want -", got.Columns[1].JSONName)
	}
}

// deprecated 列剔除；全废弃 → 列空 → ErrColumnInvalid（Review Focus #2）。
func TestTableFromConfig_SkipsDeprecated(t *testing.T) {
	tbl := &model.CodegenTable{
		Name: "t", Module: "order", BusinessName: "t", ClassName: "T",
		TemplateType: 1, FrontType: 1,
	}
	col := func(name string, dep bool, order int) *model.CodegenColumn {
		return &model.CodegenColumn{
			Name: name, GoType: "string", JSONName: name,
			HTMLType: "Input", SortOrder: order, Deprecated: dep,
		}
	}

	got, err := TableFromConfig(tbl, []*model.CodegenColumn{
		col("id", false, 0), col("gone", true, 1), col("name", false, 2),
	})
	if err != nil {
		t.Fatalf("TableFromConfig: %v", err)
	}
	if len(got.Columns) != 2 || got.Columns[0].Name != "id" || got.Columns[1].Name != "name" {
		t.Fatalf("columns=%+v, want [id name]", got.Columns)
	}
	for _, c := range got.Columns {
		if c.Deprecated {
			t.Errorf("输出列不应带 Deprecated: %+v", c)
		}
	}

	if _, err := TableFromConfig(tbl, []*model.CodegenColumn{col("gone", true, 0)}); // 全废弃
		!errors.Is(err, cgmodel.ErrColumnInvalid) {
		t.Errorf("全废弃 → ErrColumnInvalid, got %v", err)
	}
}

func TestTableFromConfig_RejectsEmpty(t *testing.T) {
	if _, err := TableFromConfig(nil, nil); err == nil {
		t.Error("nil table 应报错")
	}
	tbl := &model.CodegenTable{
		Name: "t", Module: "order", ClassName: "T", TemplateType: 1, FrontType: 1,
	}
	if _, err := TableFromConfig(tbl, nil); !errors.Is(err, cgmodel.ErrColumnInvalid) {
		t.Errorf("无列 → ErrColumnInvalid, got %v", err)
	}
}
```

> 常量名已核实：`pkg/codegen/model/enums.go` 为 `HTMLInput = "Input"`（非 HTMLTypeInput）、`TemplateTypeSingle = 1`、`FrontTypeVben5Antd = 1`。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./modules/system/infra/service/ -run TestTableFromConfig -v`
Expected: FAIL — `undefined: TableFromConfig`

- [ ] **Step 3: 最小实现**

新建 `modules/system/infra/service/config_to_table.go`：

```go
package service

import (
	"fmt"
	"sort"

	"alexGo-cloud/modules/system/infra/model"
	"alexGo-cloud/pkg/codegen/builder"
	"alexGo-cloud/pkg/codegen/metadata"
	cgmodel "alexGo-cloud/pkg/codegen/model"
	"alexGo-cloud/pkg/codegen/naming"
)

// TableFromConfig 配置行（codegen_table + codegen_column）→ 引擎输入模型。
// 预览 / ZIP / CLI 配置模式共用（spec §2：配置表行是一等引擎输入）。
// GoName/GormTag 配置表不存（导入只落快照+开关）——按列名/主键/自增重算；
// JSONName 与全部开关取存值；deprecated 列已从库消失，剔除不生成。
func TableFromConfig(t *model.CodegenTable, cols []*model.CodegenColumn) (*cgmodel.Table, error) {
	if t == nil {
		return nil, fmt.Errorf("%w: nil table", cgmodel.ErrTableInvalid)
	}
	sorted := make([]*model.CodegenColumn, 0, len(cols))
	for _, c := range cols {
		if c != nil {
			sorted = append(sorted, c)
		}
	}
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].SortOrder < sorted[j].SortOrder })

	out := make([]cgmodel.Column, 0, len(sorted))
	hasTenant := false
	for _, c := range sorted {
		if c.Deprecated {
			continue
		}
		if c.Name == "tenant_id" {
			hasTenant = true
		}
		key, extra := "", ""
		if c.IsPK {
			key = "PRI"
		}
		if c.AutoIncrement {
			extra = "auto_increment"
		}
		out = append(out, cgmodel.Column{
			Name:    c.Name,
			Comment: c.Comment,
			GoName:  naming.ToGoName(c.Name),
			GoType:  c.GoType,
			// 导入时已存（tenant_id 为 "-"、敏感字段为 "-"）
			JSONName: c.JSONName,
			GormTag: builder.BuildGormTag(metadata.ColumnMeta{
				Name: c.Name, Key: key, Extra: extra,
			}),
			HTMLType:       cgmodel.HTMLType(c.HTMLType),
			IsPK:           c.IsPK,
			AutoIncrement:  c.AutoIncrement,
			Nullable:       c.Nullable,
			ListEnable:     c.ListEnable,
			FormEnable:     c.FormEnable,
			QueryEnable:    c.QueryEnable,
			QueryOp:        c.QueryOperation,
			ListRequired:   c.ListRequired,
			FormRequired:   c.FormRequired,
			DictType:       c.DictType,
			SortOrder:      c.SortOrder,
			Deprecated:     false, // 上面已过滤
		})
	}
	tbl := &cgmodel.Table{
		TableName:     t.Name,
		TableComment:  t.TableComment,
		Module:        t.Module,
		BusinessName:  t.BusinessName,
		ClassName:     t.ClassName,
		TemplateType:  cgmodel.TemplateType(t.TemplateType),
		FrontType:     cgmodel.FrontType(t.FrontType),
		ParentTableID: int64(t.ParentTableID), // uint64 → int64
		Remark:        t.Remark,
		HasTenant:     hasTenant,
		Columns:       out,
	}
	// 提前校验：坏配置（类名空/模板值非法/列空/GoName 撞车）在渲染前以 400 暴露
	if err := tbl.Validate(); err != nil {
		return nil, err
	}
	return tbl, nil
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./modules/system/infra/service/ -run TestTableFromConfig -v`
Expected: PASS（3 个测试全绿）
Run: `go vet ./modules/system/infra/service/`
Expected: 无输出

- [ ] **Step 5: 提交**

```bash
git add modules/system/infra/service/config_to_table.go modules/system/infra/service/config_to_table_test.go
git commit -m "feat(codegen): 配置行→引擎模型映射器（重算 GoName/GormTag，剔除 deprecated）"
```

---

### Task 2: Service Preview / Download 编排

**Files:**
- Modify: `modules/system/infra/service/codegen_service.go`（接口 + struct + ctor + generate/zipFiles）
- Modify: `modules/system/infra/service/codegen_service_test.go`（newSvc 改签名 + 错误测试）
- Modify: `modules/system/infra/module.go`（newCodegenService 注入 cfg）
- Test: `modules/system/infra/service/acceptance_test.go`（M3 验收链）

**Interfaces:**
- Consumes: Task 1 的 `TableFromConfig`；已有 `s.GetTable(ctx, id) (*model.CodegenTable, []*model.CodegenColumn, error)`；`codegen.Generate(tables []*cgmodel.Table, opts codegen.Options{UnitTestEnable bool}) ([]cgmodel.GeneratedFile, error)`（`GeneratedFile{Path string; Content []byte}`，按 Path 排序、聚合错误）；`cgmodel.ErrTableInvalid`（service 文件里 pkg 模型的既有别名就是 `cgmodel`，第 13 行）。
- Produces:
  - `Preview(ctx context.Context, tableIDs []uint64) ([]cgmodel.GeneratedFile, error)`
  - `Download(ctx context.Context, tableIDs []uint64) ([]byte, error)`（ZIP 字节流）
  - `NewCodegenService(reader metadata.MetadataReader, repo repository.CodegenRepository, opts builder.Options, unitTestEnable bool) CodegenService`
  - Task 3（controller）、Task 5（CLI）按此签名消费。

- [ ] **Step 1: 写失败测试（验收链）**

在 `modules/system/infra/service/acceptance_test.go` 末尾追加（import 块补 `archive/zip`、`bytes`、`strings`）：

```go
// TestM3_Acceptance_PreviewDownload 是 spec §8 M3 验收链（服务层）：
// 导入 → 预览（引擎纯内存渲染）→ ZIP（同一份渲染结果打包）。
func TestM3_Acceptance_PreviewDownload(t *testing.T) {
	svc, _, _ := newSvc(t)
	ctx := context.Background()

	tbl, err := svc.ImportTable(ctx, "order_items")
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	files, err := svc.Preview(ctx, []uint64{tbl.ID})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if len(files) < 4 {
		t.Fatalf("files=%d, want >=4", len(files))
	}
	var paths []string
	for _, f := range files {
		if !strings.HasPrefix(f.Path, "modules/infra/") {
			t.Errorf("path=%q, want modules/infra/ 前缀", f.Path)
		}
		if len(f.Content) == 0 {
			t.Errorf("%s content 为空", f.Path)
		}
		paths = append(paths, f.Path)
	}
	// unitTestEnable=true（newSvc 传入）→ 单测骨架必须在（不硬编码文件总数，模板增减不误伤）
	hasTest := false
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			hasTest = true
		}
	}
	if !hasTest {
		t.Errorf("paths=%v, 缺 *_test.go 骨架", paths)
	}

	zipBytes, err := svc.Download(ctx, []uint64{tbl.ID})
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if len(zipBytes) < 4 || string(zipBytes[:2]) != "PK" {
		t.Fatalf("zip magic 错误（前 4 字节 %q）", zipBytes[:min(4, len(zipBytes))])
	}
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("zip open: %v", err)
	}
	zipPaths := map[string]bool{}
	for _, zf := range zr.File {
		zipPaths[zf.Name] = true
	}
	if len(zr.File) != len(paths) {
		t.Errorf("zip entries=%d, want %d", len(zr.File), len(paths))
	}
	for _, p := range paths {
		if !zipPaths[p] {
			t.Errorf("zip 缺 entry: %s", p)
		}
	}
}
```

再在 `modules/system/infra/service/codegen_service_test.go` 追加：

```go
// 空入参 / 未知 id：不渲染、不静默当全表（Review Focus #3）。
func TestPreview_Download_Errors(t *testing.T) {
	svc, _, _ := newSvc(t)
	ctx := context.Background()

	if _, err := svc.Preview(ctx, nil); !errors.Is(err, cgmodelErr.ErrTableInvalid) {
		t.Errorf("empty ids: %v", err)
	}
	if _, err := svc.Preview(ctx, []uint64{9999}); err == nil {
		t.Error("未知 id 应报错")
	}
	if _, err := svc.Download(ctx, []uint64{9999}); err == nil {
		t.Error("未知 id 下载应报错")
	}
}
```

> `errors` 与 `cgmodelErr` 别名该文件 import 块已有（第 5/16 行），无需改 import。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./modules/system/infra/service/ -run 'TestM3_Acceptance|TestPreview_Download' -v`
Expected: FAIL — 编译错（`svc.Preview undefined`）与 `NewCodegenService` 参数数不匹配（先只应出现前者；newSvc 改签名在 Step 3 一起做则此步先看到 undefined Preview）

- [ ] **Step 3: 实现 service**

`modules/system/infra/service/codegen_service.go` 三处修改：

(a) import 块追加（保持分组习惯）：

```go
	"alexGo-cloud/pkg/codegen"
```

并确认已有 `"errors"`、`"fmt"`；再补 `"archive/zip"`、`"bytes"`（标准库组）。

(b) 接口追加两方法（放在 `ListDBTables` 之后）：

```go
	// Preview 渲染选中配置表为生成文件（纯内存，spec §4/§6）。
	Preview(ctx context.Context, tableIDs []uint64) ([]cgmodel.GeneratedFile, error)
	// Download 与 Preview 同一份渲染结果打成 ZIP（spec §4）。
	Download(ctx context.Context, tableIDs []uint64) ([]byte, error)
```

(c) struct 加字段、ctor 加参数，并新增三个方法：

```go
type codegenService struct {
	reader         metadata.MetadataReader
	repo           repository.CodegenRepository
	opts           builder.Options
	unitTestEnable bool // config.Codegen.UnitTestEnable（CLI -no-tests 由调用方置 false）
}

func NewCodegenService(
	reader metadata.MetadataReader,
	repo repository.CodegenRepository,
	opts builder.Options,
	unitTestEnable bool,
) CodegenService {
	return &codegenService{reader: reader, repo: repo, opts: opts, unitTestEnable: unitTestEnable}
}
```

```go
// Preview 渲染选中配置表（spec §4：POST preview；纯内存，引擎失败无副作用）。
func (s *codegenService) Preview(ctx context.Context, tableIDs []uint64) ([]cgmodel.GeneratedFile, error) {
	return s.generate(ctx, tableIDs)
}

// Download 与 Preview 同一份渲染逻辑打包 ZIP（spec §4：POST download）。
func (s *codegenService) Download(ctx context.Context, tableIDs []uint64) ([]byte, error) {
	files, err := s.generate(ctx, tableIDs)
	if err != nil {
		return nil, err
	}
	return zipFiles(files)
}

// generate 配置行 → 映射 → 引擎。逐表聚合错误（spec §6：一轮改配置能解决的不逼跑多次）。
func (s *codegenService) generate(ctx context.Context, tableIDs []uint64) ([]cgmodel.GeneratedFile, error) {
	if len(tableIDs) == 0 {
		return nil, fmt.Errorf("%w: empty table_ids", cgmodel.ErrTableInvalid)
	}
	var errs []error
	tables := make([]*cgmodel.Table, 0, len(tableIDs))
	for _, id := range tableIDs {
		t, cols, err := s.GetTable(ctx, id)
		if err != nil {
			errs = append(errs, fmt.Errorf("table %d: %w", id, err))
			continue
		}
		tbl, err := TableFromConfig(t, cols)
		if err != nil {
			errs = append(errs, fmt.Errorf("table %s: %w", t.Name, err))
			continue
		}
		tables = append(tables, tbl)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return codegen.Generate(tables, codegen.Options{UnitTestEnable: s.unitTestEnable})
}

// zipFiles 纯内存打包；entry 名 = GeneratedFile.Path（引擎产出的仓库相对路径，恒正斜杠）。
func zipFiles(files []cgmodel.GeneratedFile) ([]byte, error) {
	buf := &bytes.Buffer{}
	w := zip.NewWriter(buf)
	for _, f := range files {
		fw, err := w.Create(f.Path)
		if err != nil {
			return nil, err
		}
		if _, err := fw.Write(f.Content); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
```

(d) `codegen_service_test.go` 的 `newSvc` 改一行：

```go
	svc := NewCodegenService(r, repo, builder.Options{Module: "infra"}, true)
```

(e) `modules/system/infra/module.go` 装配改签名（`config.Config` fx 图已有提供者，`newMetadataReader` 在用）：

```go
// newCodegenService 组装业务层，注入统一构建选项与单测骨架开关。
func newCodegenService(
	reader metadata.MetadataReader,
	repo repository.CodegenRepository,
	opts builder.Options,
	cfg *config.Config,
) service.CodegenService {
	unitTest := true
	if cfg != nil {
		unitTest = cfg.Codegen.UnitTestEnable
	}
	return service.NewCodegenService(reader, repo, opts, unitTest)
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./modules/system/infra/service/ -v`
Expected: 全绿（含新增 TestM3_Acceptance_PreviewDownload、TestPreview_Download_Errors 与全部旧测试）
Run: `go vet ./modules/system/infra/service/ ./modules/system/infra/`
Expected: 无输出

- [ ] **Step 5: fx 依赖图干跑 + 提交**

Run: `go test ./alexgo-server/cmd/`
Expected: PASS。**若**挂在这 5 包预存 cgo 链接失败（tapi/Security.tbd）上，如实记录为预存并继续（DI 变更只加了一个已被 `newMetadataReader` 证明可注入的 `*config.Config` 参数）。

```bash
git add modules/system/infra/service/codegen_service.go modules/system/infra/service/codegen_service_test.go \
  modules/system/infra/service/acceptance_test.go modules/system/infra/module.go
git commit -m "feat(codegen): service Preview/ZIP 编排（纯内存渲染，聚合错误）"
```

---

### Task 3: Controller `POST /preview` + `POST /download`，移除 generate 占位

**Files:**
- Modify: `modules/system/infra/controller/admin/codegen_controller.go`（路由、两个 handler、errOr 改写、删占位）
- Modify: `modules/system/infra/controller/admin/codegen_controller_test.go`（stub 扩两方法、删占位测试、加三个测试）

**Interfaces:**
- Consumes: Task 2 的 `Preview/Download` 签名；已有 `fail(c, status, err)`、`errOr(err, fallback)`、`doJSON(t, r, method, path, body)`、`newTestRouter(stub)`。
- Produces（Task 4/7 依赖的 HTTP 契约）:
  - `POST /api/admin/infra/codegen/preview` body `{"table_ids":[1,2]}` → `200 {"data":[{"path":"modules/...","content":"..."}]}`；空/缺 ids、业务错误 → `400 {"error":...}`。
  - `POST /api/admin/infra/codegen/download` 同 body → `200 application/zip`，头 `Content-Disposition: attachment; filename="codegen.zip"`。
  - `GET /tables/:id/generate` 路由**删除**（含 handler、`errM3Feature`、`featureNotReadyError`）。

- [ ] **Step 1: 写失败测试**

`codegen_controller_test.go`：

(a) 在既有 `stubService` struct 的字段列表**末尾追加**四个字段，并在文件内任意方法之后追加两个方法（接口已扩，先不改则编译失败=测试红）：

```go
	// —— M3 新增 ——
	previewFiles []cgmodel.GeneratedFile
	previewErr   error
	zipBytes     []byte
	downloadErr  error
```

```go
func (s *stubService) Preview(context.Context, []uint64) ([]cgmodel.GeneratedFile, error) {
	if s.previewErr != nil {
		return nil, s.previewErr
	}
	return s.previewFiles, nil
}

func (s *stubService) Download(context.Context, []uint64) ([]byte, error) {
	if s.downloadErr != nil {
		return nil, s.downloadErr
	}
	return s.zipBytes, nil
}
```

(b) 删除 `TestGenerate_ReservedForM3` 整个函数，替换为：

```go
// M3：preview 端点（取代 generate 占位）——信封 {data:[{path,content}]}。
func TestPreview_Envelope(t *testing.T) {
	stub := &stubService{previewFiles: []cgmodel.GeneratedFile{{
		Path:    "modules/infra/model/order_item.go",
		Content: []byte("package model\n"),
	}}}
	r := newTestRouter(stub)
	w := doJSON(t, r, "POST", "/api/admin/infra/codegen/preview", `{"table_ids":[1]}`)
	if w.Code != 200 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body)
	}
	body := w.Body.String()
	for _, want := range []string{`"data"`, "modules/infra/model/order_item.go", "package model"} {
		if !strings.Contains(body, want) {
			t.Errorf("body 缺 %q: %s", want, body)
		}
	}
}

// 空/缺 table_ids → 400，绝不静默当全表（Review Focus #3）。
func TestPreview_EmptyIDs_400(t *testing.T) {
	r := newTestRouter(&stubService{})
	for _, payload := range []string{"", "{}", `{"table_ids":[]}`} {
		w := doJSON(t, r, "POST", "/api/admin/infra/codegen/preview", payload)
		if w.Code != 400 {
			t.Errorf("payload=%q code=%d body=%s", payload, w.Code, w.Body)
		}
	}
	w := doJSON(t, r, "POST", "/api/admin/infra/codegen/preview", `{"table_ids":[1]}`)
	if w.Code != 200 { // stub 空 files → data:[]（合法信封，不算错）
		t.Errorf("合法空渲染 code=%d body=%s", w.Code, w.Body)
	}
}

// download：zip 流 + 两个头；业务错误 → 400（Review Focus #4）。
func TestDownload_ZipStream(t *testing.T) {
	r := newTestRouter(&stubService{zipBytes: []byte("PK\x03\x04fake-entry")})
	w := doJSON(t, r, "POST", "/api/admin/infra/codegen/download", `{"table_ids":[1]}`)
	if w.Code != 200 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/zip" {
		t.Errorf("Content-Type=%q", ct)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, `filename="codegen.zip"`) {
		t.Errorf("Content-Disposition=%q", cd)
	}
	if !strings.HasPrefix(w.Body.String(), "PK") {
		t.Errorf("body 前缀=%q", w.Body.String()[:min(4, w.Body.Len())])
	}

	errStub := newTestRouter(&stubService{downloadErr: errors.New("table 9: not found")})
	w = doJSON(t, errStub, "POST", "/api/admin/infra/codegen/download", `{"table_ids":[9]}`)
	if w.Code != 400 {
		t.Errorf("download err code=%d body=%s", w.Code, w.Body)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./modules/system/infra/controller/admin/ -run 'TestPreview|TestDownload' -v`
Expected: FAIL — `stubService does not implement service.CodegenService`（Preview/Download 未实现）或路由 404

- [ ] **Step 3: 实现 controller**

`codegen_controller.go`：

(a) import 块加 `"errors"`（`errOr` 改写用）。

(b) 路由表替换最后一行：

```go
	g.POST("/tables/:id/sync", c.sync)
	g.POST("/preview", c.preview)   // M3：纯内存渲染
	g.POST("/download", c.download) // M3：ZIP 流
```

（原 `g.GET("/tables/:id/generate", c.generate) // M3 占位` 整行删除。）

(c) 用两个 handler **整体替换** `generate` handler 与 `featureNotReadyError`/`errM3Feature` 类型定义，并改写 `errOr`：

```go
// preview 渲染选中配置表（spec §4）。body: {"table_ids":[...]} → {data:[{path,content}]}。
// 纯内存：引擎失败只回 400，不落盘、不留半成品（spec §6）。
func (c *CodegenController) preview(ctx *gin.Context) {
	var req struct {
		TableIDs []uint64 `json:"table_ids" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil || len(req.TableIDs) == 0 {
		fail(ctx, http.StatusBadRequest, errOr(err, "empty table_ids"))
		return
	}
	files, err := c.svc.Preview(ctx.Request.Context(), req.TableIDs)
	if err != nil {
		fail(ctx, http.StatusBadRequest, err) // 配置/生成类 = 400（对齐 getTable/import）
		return
	}
	items := make([]gin.H, 0, len(files))
	for _, f := range files {
		items = append(items, gin.H{"path": f.Path, "content": string(f.Content)})
	}
	ctx.JSON(http.StatusOK, gin.H{"data": items})
}

// download 同一份渲染结果打成 ZIP 流（spec §4）。
func (c *CodegenController) download(ctx *gin.Context) {
	var req struct {
		TableIDs []uint64 `json:"table_ids" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil || len(req.TableIDs) == 0 {
		fail(ctx, http.StatusBadRequest, errOr(err, "empty table_ids"))
		return
	}
	zipBytes, err := c.svc.Download(ctx.Request.Context(), req.TableIDs)
	if err != nil {
		fail(ctx, http.StatusBadRequest, err)
		return
	}
	ctx.Header("Content-Disposition", `attachment; filename="codegen.zip"`)
	ctx.Data(http.StatusOK, "application/zip", zipBytes)
}

func errOr(err error, fallback string) error {
	if err != nil {
		return err
	}
	return errors.New(fallback)
}
```

> 文件头注释里「9 个端点（preview/download 属 M3）」之类如存在，顺手改为 10 个端点无占位（可选，不单独提交）。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./modules/system/infra/... -v`（integration tag 的迁移测试不跑）
Expected: 全绿；确认 `TestGenerate_ReservedForM3` 已不存在：`grep -n "M3" modules/system/infra/controller/admin/codegen_controller_test.go` 无占位断言残留。
Run: `go vet ./modules/system/infra/...`
Expected: 无输出

- [ ] **Step 5: 提交**

```bash
git add modules/system/infra/controller/admin/codegen_controller.go \
  modules/system/infra/controller/admin/codegen_controller_test.go
git commit -m "feat(codegen): POST preview/download 端点，移除 generate M3 占位"
```

---

### Task 4: 前端预览 Modal + ZIP 下载 + 启用「生成代码」

**Files:**
- Modify: `apps/web-antd/src/api/infra.ts`（尾部追加）
- Modify: `apps/web-antd/src/views/infra/codegen/index.vue`（script handlers + 模板）

**Interfaces:**
- Consumes: Task 3 的两个端点与 `{"table_ids":[...]}` body；`requestClient.download`（`RequestClient` 公有方法，`responseType:'blob'`、`responseReturn:'body'`，支持 `{method:'POST', data}`）；`requestClient.post<T>` 解包 `{data}` → `T`；既有 `listCodegenTables()` / `useVbenModal` / `v-access:code`。
- Produces: `CodegenFile{path,content}`、`previewCodegen(tableIds): Promise<CodegenFile[]>`、`downloadCodegen(tableIds): Promise<Blob>`、`saveBlob(blob, filename)`（Task 7 文档描述的 UI 契约）。

**验证门（例外裁定见 Global Constraints）**：`vue-tsc` typecheck，无单测。

- [ ] **Step 1: API 封装**

`apps/web-antd/src/api/infra.ts` 文件尾部追加：

```ts
// ---- M3：预览 / ZIP 下载 ----

export interface CodegenFile {
  content: string;
  path: string;
}

// 预览选中表的生成结果（后端纯内存渲染，不落盘）。
export function previewCodegen(tableIds: number[]) {
  return requestClient.post<CodegenFile[]>('/admin/infra/codegen/preview', {
    table_ids: tableIds,
  });
}

// 下载选中表的生成结果 ZIP（application/zip 流）。
export function downloadCodegen(tableIds: number[]): Promise<Blob> {
  return requestClient.download<Blob>('/admin/infra/codegen/download', {
    data: { table_ids: tableIds },
    method: 'POST',
  });
}

// 触发浏览器保存（createObjectURL + a[download]），与 adapter 里已有用法一致。
export function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}
```

- [ ] **Step 2: 页面 handlers（script）**

`index.vue`：

(a) import 三处改动——
① 类型行（第 2 行）改为：

```ts
import type { CodegenColumn, CodegenFile, CodegenTable } from '#/api/infra';
```

② `ant-design-vue` 的 import 列表里**删掉** `Tooltip,`（占位按钮移除后无引用）；
③ `vue` 行改为 `import { computed, ref } from 'vue';`
④ `from '#/api/infra'` 的函数列表（现有 `deleteCodegenTable, importCodegenTable, listCodegenColumns, listCodegenTables, listDBTables, saveCodegenColumns, syncCodegenTable, updateCodegenTable`）追加三个，按字母序插入：

```ts
import {
  deleteCodegenTable,
  downloadCodegen,
  importCodegenTable,
  listCodegenColumns,
  listCodegenTables,
  listDBTables,
  previewCodegen,
  saveBlob,
  saveCodegenColumns,
  syncCodegenTable,
  updateCodegenTable,
} from '#/api/infra';
```

(b) 在 `// ---- 同步 / 删除 ----` 段之后、`</script>` 之前追加：

```ts
// ---- M3：代码预览（文件树 + 代码 + 下载 ZIP，spec §5.3）----
const previewFiles = ref<CodegenFile[]>([]);
const previewIds = ref<number[]>([]);
const previewPath = ref('');

// spec §5.3：左侧文件树按 后端(modules|pkg) / 前端(apps) 分组；空组不展示
const previewGroups = computed(() => {
  const groups: { files: CodegenFile[]; label: string }[] = [
    { files: [], label: '后端' },
    { files: [], label: '前端' },
    { files: [], label: '其他' },
  ];
  for (const f of previewFiles.value) {
    if (f.path.startsWith('modules/') || f.path.startsWith('pkg/')) {
      groups[0]!.files.push(f);
    } else if (f.path.startsWith('apps/')) {
      groups[1]!.files.push(f);
    } else {
      groups[2]!.files.push(f);
    }
  }
  return groups.filter((g) => g.files.length > 0);
});

const previewContent = computed(
  () =>
    previewFiles.value.find((f) => f.path === previewPath.value)?.content ?? '',
);

const [PreviewModal, previewModalApi] = useVbenModal({
  title: '代码预览',
  // 底部按钮在内容区自定义（关闭 + 下载 ZIP，spec §5.3）
  showDefaultActions: false,
  onOpenChange(isOpen) {
    if (!isOpen) {
      previewFiles.value = [];
      previewIds.value = [];
      previewPath.value = '';
    }
  },
});

async function openPreview(ids: number[], label: string) {
  if (ids.length === 0) {
    message.warning('暂无可生成的表配置');
    return;
  }
  let files: CodegenFile[];
  try {
    files = await previewCodegen(ids);
  } catch {
    return; // 拦截器已 toast
  }
  previewIds.value = ids;
  previewFiles.value = files;
  previewPath.value = files[0]?.path ?? '';
  previewModalApi.setState({ title: `代码预览 — ${label}` });
  previewModalApi.open();
}

function onPreview(row: CodegenTable) {
  void openPreview([row.id], row.table_name);
}

// 工具栏「生成代码」= 全部已导入表一起预览（spec §8 验收入口）
async function onPreviewAll() {
  let rows: CodegenTable[];
  try {
    rows = await listCodegenTables();
  } catch {
    return; // 拦截器已 toast
  }
  await openPreview(
    rows.map((r) => r.id),
    '全部表',
  );
}

// 行内「下载」：单表直接 ZIP（spec §5.1 行操作）
async function onDownload(row: CodegenTable) {
  try {
    const blob = await downloadCodegen([row.id]);
    saveBlob(blob, `codegen_${row.table_name}.zip`);
  } catch {
    // 拦截器已 toast
  }
}

// 预览弹窗底部「下载 ZIP」：与当前预览同一批 table_ids
async function onDownloadZip() {
  try {
    const blob = await downloadCodegen(previewIds.value);
    saveBlob(blob, 'codegen.zip');
  } catch {
    // 拦截器已 toast
  }
}
```

- [ ] **Step 3: 模板改动**

(a) 工具栏——用启用按钮**整块替换** Tooltip 占位：

```html
        <Button
          class="mr-2"
          v-access:code="['infra:codegen']"
          @click="onPreviewAll"
        >
          生成代码
        </Button>
```

（删除原 `<Tooltip title="生成代码是 M3 能力..."><Button disabled>...</Button></Tooltip>` 三行块。）

(b) 行操作——在「字段配置」后、「同步」前插入两个按钮，并把操作列宽 260 → 340（columns 定义处）：

```html
        <Button
          size="small"
          type="link"
          v-access:code="['infra:codegen']"
          @click="onPreview(row)"
        >
          预览
        </Button>
        <Button
          size="small"
          type="link"
          v-access:code="['infra:codegen']"
          @click="onDownload(row)"
        >
          下载
        </Button>
```

(c) 在 `<EditModal>...</EditModal>` 之后、`</Page>` 之前追加预览弹窗：

```html
    <!-- 代码预览：左文件树 + 右代码 + 底部下载 ZIP（spec §5.3；
         代码高亮属 M4 再接，此处等宽 pre 展示——见 usage.md 偏离说明） -->
    <PreviewModal>
      <div class="flex" style="height: 480px">
        <div class="w-60 shrink-0 overflow-auto border-r pr-2">
          <div v-for="g in previewGroups" :key="g.label">
            <div class="mt-1 mb-1 text-xs font-bold text-gray-500">
              {{ g.label }}
            </div>
            <div
              v-for="f in g.files"
              :key="f.path"
              :class="
                f.path === previewPath
                  ? 'bg-blue-100 text-blue-700'
                  : 'hover:bg-gray-100'
              "
              :title="f.path"
              class="cursor-pointer truncate px-1 py-0.5 font-mono text-xs"
              @click="previewPath = f.path"
            >
              {{ f.path.split('/').pop() }}
            </div>
          </div>
          <div
            v-if="previewFiles.length === 0"
            class="p-2 text-xs text-gray-400"
          >
            无生成文件
          </div>
        </div>
        <div class="flex-1 overflow-auto pl-3">
          <pre class="m-0 text-xs whitespace-pre">{{ previewContent }}</pre>
        </div>
      </div>
      <div class="mt-3 flex justify-end gap-2">
        <Button @click="previewModalApi.close()">关闭</Button>
        <Button type="primary" @click="onDownloadZip">下载 ZIP</Button>
      </div>
    </PreviewModal>
```

- [ ] **Step 4: typecheck 通过**

Run: `cd apps/web-antd && pnpm run typecheck`
Expected: 0 错误（有错逐个修：常见为漏 import `computed`/`CodegenFile`、或 `Tooltip` 残留引用）

- [ ] **Step 5: 提交**

```bash
git add apps/web-antd/src/api/infra.ts apps/web-antd/src/views/infra/codegen/index.vue
git commit -m "feat(web-antd): 代码预览弹窗（文件树+代码+ZIP）与行内预览/下载"
```

---

### Task 5: CLI 配置表默认模式 + `--import` 现场导入

**Files:**
- Modify: `tools/codegen/main.go`（flag、main 主流程、filterConfigTables）
- Test: `tools/codegen/main_test.go`（过滤器测试）

**Interfaces:**
- Consumes: Task 2 的 `service.NewCodegenService(reader, repo, opts, unitTestEnable bool)` 与 `svc.Preview(ctx, ids)`、`svc.ImportTable(ctx, name)`；`repository.NewCodegenRepository(gdb *gorm.DB)`、`repo.ListAllTables(ctx)`、`repo.ImportedTableNames(ctx)`；`inframodel.CodegenTable.Name/Module/ID`（import 别名 `inframodel "alexGo-cloud/modules/system/infra/model"`——main.go 的 `model` 已被 `pkg/codegen/model` 占用，勿直接用 `model.CodegenTable`）；既有 `writeFiles/parseTables/schemaFromDSN`（**不动**）。smoke 集成测试（`-tags integration`）直接调 pkg 层，不受本任务影响。
- Produces: CLI flag 契约（Task 6/7 依赖）：
  - 配置模式（默认）：`go run ./tools/codegen [-module=order] [-tables=a,b] [-out .] [-force] [-dry-run] [-no-tests] [-dsn ...]` —— 过滤已导入配置表（空 = 全部），经 `svc.Preview` 渲染后写盘；无匹配 → 报错退出（非 0）。
  - 导入模式：追加 `-import`（**必须配 `-module`**），先现场读元数据写配置表（已导入的跳过、幂等），再同上生成；`-schema` 仅此分支用。
  - 摘要输出格式不变：`生成 %d / 跳过 %d（已存在）/ 失败 0`。

- [ ] **Step 1: 写失败测试（过滤器）**

`tools/codegen/main_test.go`：import 块追加 `inframodel "alexGo-cloud/modules/system/infra/model"`（该文件现有 `model` 是 `pkg/codegen/model`，别冲突），末尾追加：

```go
// 配置模式选表过滤：module/tables 皆可空（空=不过滤）；无匹配必须报错（Review Focus #5）。
func TestFilterConfigTables(t *testing.T) {
	mk := func(name, module string) *inframodel.CodegenTable {
		return &inframodel.CodegenTable{Name: name, Module: module}
	}
	all := []*inframodel.CodegenTable{
		mk("order_items", "order"),
		mk("order_logs", "order"),
		mk("users", "system"),
	}

	got, err := filterConfigTables(all, "", "")
	if err != nil || len(got) != 3 {
		t.Fatalf("全量: n=%d err=%v", len(got), err)
	}

	got, err = filterConfigTables(all, "order", "")
	if err != nil || len(got) != 2 || got[0].Name != "order_items" {
		t.Fatalf("module 过滤: %+v err=%v", got, err)
	}

	got, err = filterConfigTables(all, "", "users,order_logs")
	if err != nil || len(got) != 2 {
		t.Fatalf("tables 过滤: n=%d err=%v", len(got), err)
	}

	// 双过滤器取交集；order ∩ users = ∅ → 必须报错
	if _, err := filterConfigTables(all, "order", "users"); err == nil {
		t.Error("交集为空应报错")
	}

	if _, err := filterConfigTables(all, "nope", ""); err == nil {
		t.Error("无匹配必须报错，不能静默空生成")
	}
	if _, err := filterConfigTables(nil, "", ""); err == nil {
		t.Error("空配置表集合必须报错")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./tools/codegen/ -run TestFilterConfigTables -v`
Expected: FAIL — `undefined: filterConfigTables`

- [ ] **Step 3: 重写 main.go 主流程**

`tools/codegen/main.go` 修改：

(a) import 块四处改动：
- **删除** `"alexGo-cloud/pkg/codegen"`（直调 `codegen.Generate` 的路径没了，生成统一走 service）
- **追加** `inframodel "alexGo-cloud/modules/system/infra/model"`、`"alexGo-cloud/modules/system/infra/repository"`、`"alexGo-cloud/modules/system/infra/service"`、`"gorm.io/driver/mysql"`（gorm 用，注意与已有 `"github.com/go-sql-driver/mysql"` 不同包，后者被 `schemaFromDSN` 的 `mysql.ParseDSN` 引用**保留**）、`"gorm.io/gorm"`
- `builder`/`metadata`/`database/sql`/`pkg/config` 均保留（import 分支用）
- `"alexGo-cloud/pkg/codegen/model"` 保持原样（别名仍为 `model`，供 `ErrMetadataUnreachable`、`writeFiles` 的 `model.GeneratedFile` 用）

(b) flag 区：保留全部既有 flag，新增一个，并**删除**开头的 `if *module == ""` 硬校验（改到 `-import` 分支）：

```go
	importFlag := flag.Bool("import", false, "先现场读元数据导入配置表（需 -module/-tables 或 -tables 留空=全库表），再从配置表生成")
	flag.Parse()
```

(c) `main()` 主体在 dsn 解析完成后（原 `ctx` 行起）整体替换为。原代码里位于 build 循环之后的 `unitTest` 计算块**原样上移**到开头（import 分支要用）：

```go
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 原样保留（从旧位置上移）：单测骨架开关
	unitTest := true
	if cfg != nil {
		unitTest = cfg.Codegen.UnitTestEnable
	}
	if *noTests {
		unitTest = false
	}

	// 配置表所在库（与元数据同 DSN）；gorm 只用于 config 读写
	gdb, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open config db: %v\n", err)
		os.Exit(1)
	}
	repo := repository.NewCodegenRepository(gdb)

	// -import：现场读元数据写入配置表（已导入跳过，幂等），随后统一走配置模式
	if *importFlag {
		if *module == "" {
			fmt.Fprintln(os.Stderr, "-import 需要 --module（导入行要写目标模块），如 --module=order --tables=t1 --import")
			os.Exit(2)
		}
		tableList, err := parseTables(*tables)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		sdb, err := sql.Open("mysql", dsn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		defer sdb.Close()
		if err := sdb.PingContext(ctx); err != nil {
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
		reader := metadata.NewMySQLReader(sdb, schema)
		if len(tableList) == 0 {
			// 与旧 dbgen 语义对齐：-tables 留空 = schema 全部表
			tableList, err = reader.ListTables(ctx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		}
		svc := service.NewCodegenService(reader, repo, builder.Options{Module: *module}, unitTest)
		done, err := repo.ImportedTableNames(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		imported := make(map[string]bool, len(done))
		for _, n := range done {
			imported[n] = true
		}
		var importErrs []error
		skipped := 0
		for _, name := range tableList {
			if imported[name] {
				skipped++
				continue
			}
			if _, err := svc.ImportTable(ctx, name); err != nil {
				importErrs = append(importErrs, fmt.Errorf("import %s: %w", name, err))
			}
		}
		if len(importErrs) > 0 {
			for _, e := range importErrs {
				fmt.Fprintf(os.Stderr, "%v\n", e)
			}
			fmt.Fprintf(os.Stderr, "导入失败 %d 个，未生成任何文件\n", len(importErrs))
			os.Exit(1)
		}
		fmt.Printf("导入 %d / 跳过 %d（已导入）\n", len(tableList)-skipped, skipped)
	}

	// 配置模式：过滤已导入表 → Preview（与后台同一渲染路径）
	all, err := repo.ListAllTables(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	selected, err := filterConfigTables(all, *module, *tables)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(2)
	}
	ids := make([]uint64, 0, len(selected))
	for _, t := range selected {
		ids = append(ids, t.ID)
	}
	// 配置模式不读元数据（reader=nil）；生成路径与后台 preview 完全同源
	svc := service.NewCodegenService(nil, repo, builder.Options{}, unitTest)
	files, err := svc.Preview(ctx, ids)
	if err != nil {
		fmt.Fprintln(os.Stderr, err) // errors.Join 多行 = 逐条打印（spec §6）
		fmt.Fprintln(os.Stderr, "未生成任何文件")
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
```

> 两个 `svc :=` 分处不同作用域（import 分支在 if 块内、配置模式在函数级），无遮蔽冲突；import 分支的 `svc` 用 reader，配置模式的 `svc` 传 nil（生成路径不读元数据）。

(d) 同文件追加过滤器（放 `parseTables` 旁边）：

```go
// filterConfigTables 配置模式选表：module/tables 都是过滤器（空=不过滤）。
// 无匹配返回 error（带修复建议），绝不静默生成 0 个文件当成功。
func filterConfigTables(tables []*inframodel.CodegenTable, module, names string) ([]*inframodel.CodegenTable, error) {
	want, err := parseTables(names)
	if err != nil {
		return nil, err
	}
	wantSet := make(map[string]bool, len(want))
	for _, w := range want {
		wantSet[w] = true
	}
	var out []*inframodel.CodegenTable
	for _, t := range tables {
		if t == nil {
			continue
		}
		if module != "" && t.Module != module {
			continue
		}
		if len(wantSet) > 0 && !wantSet[t.Name] {
			continue
		}
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf(
			"没有匹配的已导入表（--module=%q --tables=%q）：先在后台页面导入，或加 --import 现场导入",
			module, names,
		)
	}
	return out, nil
}
```

(e) 删除原「readTable → builder.Build 循环」、`if len(tableList) == 0 { reader.ListTables... }`（原 draft 元数据路径）与开头 `if *module == ""` 硬校验——配置模式不再直读元数据；原 `sql.Open`/ping/schema/reader 块整体挪进 `-import` 分支（已在 (c) 覆盖）。保留文件尾的 `writeFiles`/`parseTables`/`schemaFromDSN` 不动（`writeFiles` 参数类型 `[]model.GeneratedFile` 不变，Preview 返回的就是它）。

- [ ] **Step 4: 跑测试 + 构建**

Run: `go test ./tools/codegen/ -v`
Expected: PASS（TestWriteFiles_*、TestSchemaFromDSN、TestFilterConfigTables 全绿）
Run: `go vet ./tools/codegen/ && go build ./tools/codegen/`
Expected: 无输出、编译过

Run（无匹配报错路径）：`go run ./tools/codegen -module=__nope__`
Expected: 非 0 退出，stderr 含 `没有匹配的已导入表`（若本地配置库不可达则先 `docker-compose ... up -d mysql`，或记录环境不可用并在 Task 8 复验）

- [ ] **Step 5: 提交**

```bash
git add tools/codegen/main.go tools/codegen/main_test.go
git commit -m "feat(codegen): CLI 默认配置表模式 + --import 现场导入"
```

---

### Task 6: Makefile 切换 `generate-*`

**Files:**
- Modify: `Makefile`（`generate-crud` / `generate-db-crud` 两个 recipe；`generate-all` 行不动）

**Interfaces:**
- Consumes: Task 5 的 CLI flag 契约。
- Produces: `make generate-crud`（`MODULE ?= system` 过滤默认值生效）、`DB_DSN=... make generate-db-crud MODULE=... TABLES=...`（= `-import`，与旧 dbgen 用法等价）——Task 7 文档、Task 8 验收依赖。

- [ ] **Step 1: 改 recipe**

当前两段（Makefile 22-27 行附近）：

```make
generate-crud:
	go run ./scripts/crud_generator.go

generate-db-crud:
	go run ./tools/dbgen --module=$(MODULE) --tables=$(TABLES)
```

替换为：

```make
generate-crud: ## 从配置表生成已导入表（MODULE 过滤，默认 system；不覆盖已有文件）
	go run ./tools/codegen $(if $(MODULE),-module=$(MODULE))

generate-db-crud: ## 现场读元数据（未导入则先入配置表）再生成；TABLES 留空 = schema 全部表
	go run ./tools/codegen -module=$(MODULE) -tables=$(TABLES) -import
```

`generate-all: proto generate-crud` **保持原样**（spec §5：用法不变、实现随 generate-crud 替换）。

- [ ] **Step 2: 验证展开与冒烟**

Run: `make -n generate-crud`
Expected: 打印 `go run ./tools/codegen -module=system`
Run: `make -n generate-crud MODULE=order`
Expected: 打印 `go run ./tools/codegen -module=order`
Run: `make -n generate-db-crud MODULE=system TABLES=users`
Expected: 打印 `go run ./tools/codegen -module=system -tables=users -import`
Run: `make -n generate-all`
Expected: 先 `bash scripts/gen_proto.sh`，再 generate-crud 那行

- [ ] **Step 3: 实跑一次配置模式（dry-run，不落盘）**

前置：本地 MySQL 在跑（配置表所在库），且至少一张表已导入（可用后台页面导入，或 Task 8 验收时统一做）。
Run: `CGO_ENABLED=0 go run ./tools/codegen -dry-run`
Expected: 要么打印文件清单 + `dry-run：N 个文件`；要么以非 0 退出并提示先导入（两种都算过——证明接线通，数据态在 Task 8 补）

- [ ] **Step 4: 提交**

```bash
git add Makefile
git commit -m "feat(build): generate-crud/generate-db-crud 切到 tools/codegen 配置模式"
```

---

### Task 7: 文档更新（usage.md / README 去 M3 占位）

**Files:**
- Modify: `docs/usage.md`（§2.1/§2.2/§2.3/§2.7）
- Modify: `README.md`（命令表两行）

**Interfaces:**
- Consumes: Task 3/4/5/6 落定的端点、UI 行为、CLI flag、Makefile 展开（文档必须与实测一致——Task 8 若发现出入，回改此处）。

- [ ] **Step 1: usage.md §2 分阶段句**

84 行：

```diff
-代码生成分阶段交付：**M2** 完成配置持久化与同步的后端闭环 + 管理页面（当前）；**M3** 接生成执行（模板渲染 + 下载）。
+代码生成分阶段交付：**M2** 配置持久化与同步 + 管理页面；**M3** 在线链路已接通——预览 / ZIP 下载 / CLI 读配置表（当前）；M4–M7 依次续交付（见 spec §8）。
```

- [ ] **Step 2: usage.md §2.2 界面操作表**

把「生成代码」行替换，并在其后补两行：

```diff
-| **生成代码**（工具栏） | — | **M3 占位，恒禁用**（后端 `generate` 亦恒 500） |
+| **生成代码**（工具栏） | `infra:codegen` | 预览**全部**已导入表：弹窗文件树（后端/前端分组）+ 代码 + 底部「下载 ZIP」 |
+| **预览**（行） | `infra:codegen` | 同上弹窗，但只渲染该表 |
+| **下载**（行） | `infra:codegen` | 该表直接下载 ZIP（文件名 `codegen_<表名>.zip`） |
```

§2.1 权限码段末尾补一句：

```markdown
预览/下载不引入新按钮码，复用页面权限 `infra:codegen`。
```

- [ ] **Step 3: usage.md §2.3 接口表**

```diff
-| GET | `/api/admin/infra/codegen/tables/:id/generate` | **M3 占位**，恒返回 500 `generate is an M3 feature` |
+| POST | `/api/admin/infra/codegen/preview` | 预览渲染（body `{"table_ids":[1,2]}` → `{data:[{path,content}]}`；纯内存，失败无副作用） |
+| POST | `/api/admin/infra/codegen/download` | 同 body → `application/zip` 流（`Content-Disposition: attachment; filename="codegen.zip"`） |
```

- [ ] **Step 4: usage.md §2.7 CLI 整节重写**

```markdown
### 2.7 CLI（可选）

`tools/codegen` 提供同引擎的命令行入口，**默认读配置表**（与后台预览同一渲染路径 `service.Preview`）。
写盘规则：已存在不覆盖，`-force` 才覆盖；全部渲染成功才写盘。常用 flag：

```bash
# 日常主用法：从配置表生成（已导入的表；module/tables 均为过滤器，空=全部）
go run ./tools/codegen -module=order -tables=order_item
go run ./tools/codegen -module=order                # 模块下全部已导入表
go run ./tools/codegen                              # 全部已导入表

# 现场导入：读元数据写入配置表后再生成（已导入的表跳过，幂等）
# 注意：-import 必须配 -module（导入行要写目标模块）
go run ./tools/codegen -module=order -tables=order_item -import
go run ./tools/codegen -module=order -import        # -tables 留空 = schema 全部表

# 通用：-dsn（默认 config database.dsn / DB_DSN）、-schema（仅 -import，默从 dsn 解析）
#       -out（默认 .）、-dry-run、-force、-no-tests（覆盖 config codegen.unit_test_enable）
```

> 与旧 draft 模式（直读元数据、无需配置表）的差异：不带 `-import` 时**必须先在后台导入**，
> 否则报「没有匹配的已导入表」。`make generate-db-crud` 已自动带 `-import`，旧用法不受影响。
> spec §5 偏离说明：三页面沿用 M2 modal 结构、预览为等宽 pre 展示（高亮留 M4）、
> JSON body 用仓库惯例 snake_case（`table_ids`）。
```

- [ ] **Step 5: README 命令表两行**

```diff
-| `make generate-crud` | 模板 CRUD 生成（不覆盖已有文件） |
-| `DB_DSN=... make generate-db-crud MODULE=system TABLES=users,roles,menus` | 按数据表反向生成 CRUD |
+| `make generate-crud` | 从配置表生成已导入表（`MODULE` 过滤，默认 `system`；不覆盖已有文件） |
+| `DB_DSN=... make generate-db-crud MODULE=system TABLES=users,roles,menus` | 现场读元数据（未导入则先入配置表）生成 CRUD |
```

- [ ] **Step 6: 通读自查 + 提交**

自查点：文中不再有「M3 占位 / 恒禁用 / generate is an M3 feature」；端点方法/路径/body 与 Task 3 一致；CLI 示例与 Task 5 flag 一致。
Run: `grep -n "M3 占位\|恒禁用\|M3 feature" docs/usage.md README.md`
Expected: 无输出

```bash
git add docs/usage.md README.md
git commit -m "docs: M3 在线链路（预览/ZIP 端点、预览弹窗、CLI 配置模式）"
```

---

### Task 8: 端到端验收（浏览器全链路 + CLI/Make 实跑）

**Files:**
- 无新增（发现问题按对应层 `fix(...)` 修复并提交；本任务自身不产生提交）

**Interfaces:**
- Consumes: 全部前序任务产物。浏览器已有登录态（admin）；新按钮复用 `infra:codegen` 权限码，无需重取 access codes。

- [ ] **Step 1: 重启后端到最新代码**

```bash
# 结束旧的 go run 进程（本会话后台任务 bwocr2w7e，或 pgrep -f alexgo-server 后 kill）
# macOS 必须 CGO_ENABLED=0（README 已记录的链接器问题）
CGO_ENABLED=0 go run ./alexgo-server/cmd/main.go
```

前端 dev server（:5666）若在跑则 HMR 自动生效；不在则 `pnpm dev`。

- [ ] **Step 2: 浏览器「导入 → 配置 → 预览 → ZIP」（spec §8 M3 验收）**

用 Playwright MCP 在 `http://localhost:5666` 操作：

1. 进入 **系统管理 → 代码生成**：工具栏应出现启用的「生成代码」按钮（不再是灰底 Tooltip）。
2. 导入一张真实表（已有导入则复用）：「导入表」→ 下拉选表 → 导入成功。
3. 行内「字段配置」：随便开一个查询开关并保存（证明链路里配置生效入口还在）。
4. 行内「预览」→ 弹窗出现：左侧「后端」分组文件树（点击切换）、右侧等宽代码非空。
5. 弹窗底部「下载 ZIP」→ 浏览器下载 `codegen.zip`，`unzip -l` 或文件头确认 `PK` 且条目为 `modules/...`。
6. 行内「下载」→ 得到 `codegen_<表名>.zip`。
7. 工具栏「生成代码」→ 预览全部已导入表（文件树条目数 > 单表）。
8. 负例：控制台直接 `fetch` POST preview body `{}` → 400（可在 Playwright `browser_evaluate` 里带 token 调，或跳过并在 Go 测试已覆盖的前提下记录）。

- [ ] **Step 3: CLI 与 make 用法等价实跑**

```bash
# 配置模式 dry-run（Step 2 已导入过 → 应打印文件清单）
CGO_ENABLED=0 go run ./tools/codegen -dry-run
CGO_ENABLED=0 go run ./tools/codegen -module=<Step2 表的 module> -tables=<表名> -dry-run

# -import 幂等：重复执行 → 「跳过 N（已导入）」且随后正常 dry-run
CGO_ENABLED=0 go run ./tools/codegen -module=<module> -tables=<表名> -import -dry-run

# make 展开（Step 2 前置已具备后可去掉 -n 真跑一把 -dry-run 等价命令）
make -n generate-crud generate-db-crud MODULE=system TABLES=users
```

Expected: 全部非 0 退出仅出现在「故意无匹配」场景；正常场景打印 `dry-run：N 个文件`；`-import` 二次执行显示跳过计数。

- [ ] **Step 4: 回归面**

```bash
go test ./modules/system/infra/... ./tools/codegen/ ./pkg/codegen/...
cd apps/web-antd && pnpm run typecheck
```

Expected: 全绿（`-tags integration` 除外）。5 个预存 cgo 包的失败如实记录不修。

- [ ] **Step 5: 记录验收结论**

在会话中汇报：浏览器 8 步截图/要点、CLI 三条命令输出、测试与 typecheck 结果；若 Task 7 文档与实测有出入，回改 usage.md 并 `docs: ` 补提交。M3 完成后，P0 剩余：通知已读/我的通知；随后 M7 收尾（`make codegen-smoke`、删 3 个旧工具、文档收口）。
