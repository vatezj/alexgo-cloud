# 代码生成配置持久化（M2）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 建成 `modules/infra` 配置持久化——迁移建表 + 导入/同步 + admin API + 菜单权限 seed，使「导入 → 改配置 → 同步 diff」全链路 HTTP 测试通过（spec §8 M2 验收）。

**Architecture:** `modules/infra` 按 member/system 惯例做 fx 模块（module.go + migration_source.go + 三层 + admin 控制器），配置存 `codegen_table`/`codegen_column`（MySQL 快照 + 可编辑配置）；service 编排 `pkg/codegen`（metadata 读 → builder 初置/刷新 → diff），controller 提供 9 个端点（preview/download 属 M3）。同时按用户裁决修复两处引擎缺口：租户写路径（模板）与非 id 主键（构建期拒绝）。本计划仅覆盖 spec 里程碑 **M2**；M3–M7 依次续写独立计划。

**Tech Stack:** Go 1.25、gin、fx、gorm + glebarez/sqlite（单测）/ MySQL（迁移）、golang-migrate、go-sql-driver/mysql（ParseDSN）、text/template（已有引擎）。

**Spec:** `docs/superpowers/specs/2026-10-10-codegen-redesign-design.md`

## Global Constraints

- 用户已裁决（写入本计划与 spec，Task 1 落 spec 文本）：
  ① **租户写路径缺口 M2 内修复**——`tenant_id` json tag 固定 `-"；repository Create 打戳、Update 加租户过滤；service Create/Update 从 ctx 取租户；golden 刷新。
  ② **非 id 主键约定写进 spec + 构建期拒绝**——仅支持单列 `id` 主键；`builder.Build`（TemplateTypeSingle）对无主键/复合主键/非 `id` 主键返回 `ErrColumnInvalid`。
- spec M2 范围（§8）：迁移建表 + 导入/同步 + admin API + 菜单权限 seed；**预览/下载（POST preview/download）与后台页面留 M3，本计划不实现**。
- `modules/infra` → `pkg/codegen` 单向依赖；`pkg/codegen` 不 import `modules/*`、不依赖 fx。
- 系统级功能不做租户隔离（spec §1 非目标）：`codegen_table`/`codegen_column` **无 `tenant_id` 列**，infra 的 gorm 模型**不带 `TenantID` 字段**（租户插件对无该字段的模型自动跳过，无需进 ExemptTables）。
- 配置删除 = **硬删除**（DELETE 级联删字段行，事务内），不设 `deleted` 列（spec errata，Task 1 落文本）。
- 同步语义（spec §4，Task 1 落可执行定义）：新增列追加配置行且**开关默认关闭**；消失列标 `deprecated=1` 不物理删；快照字段刷新、**开关与配置字段一律不覆盖**；曾 deprecated 的列重新出现 → 恢复并计入 updated。
- admin 路由前缀 `/admin/infra/codegen`（挂 server 的 `/api` group 下，全路径 `/api/admin/infra/codegen/...`）；响应包裹对齐现有惯例：列表 `gin.H{"data":..., "total":...}`、详情 `gin.H{"data":...}`、变更 `gin.H{"status":"ok"}`、错误 `gin.H{"error": msg}`；HTTP 分类只用 **400（校验失败/未找到/业务拒绝）与 500**（仓库现状无 404，不引入）。
- **权限链**：菜单 seed 的 permission 前缀 `infra:codegen` 必须同步注册进 `modules/system/service/permission_routes.go` 的 `permissionRoutes`，否则 casbin 无策略、admin 也 403（Task 9 断言钉死）。
- `alexgo-server/cmd/main_test.go` 的 3 个 `fx.ValidateApp` 干跑测试（mono/micro/migrate-only）必须始终绿——infra.FxModule 装配变更不得破坏依赖图。
- 提交直接落 main（项目约定，不建功能分支）；每任务一次提交，风格 `feat(infra):` / `fix(codegen):` / `test(infra):` / `docs:`。
- 每任务 TDD：先写失败测试 → 确认失败 → 最小实现 → 通过 → 提交（例外见 Task 10 的 characterize-then-refactor 步骤，已标注）。
- 预存环境问题（不修）：cgo/Security.tbd 链接失败影响 5 包（alexgo-server/cmd|server、modules/member/cmd、modules/system、modules/system/cmd）——`go build ./...`/`go test ./...` 挂这 5 包属已裁定预存，验证时按包范围跑测试并如实记录。

## Review Focus

以下 5 类输入/状态最可能咬到使用者，每条在所属任务中有钉死它的测试：

1. **gorm `Save` 丢弃链式 WHERE**（已实证：跨租户 `Save` 照改不误）→ 模板 Update 若用 Save，租户过滤形同虚设 → Task 2 渲染断言（必须含 `Where("id = ? AND tenant_id = ?"` + `Select("*").Updates`、不得含 `Save(entity)`）+ gorm 语义镜像测试。
2. **同步冲掉用户配置或物理删列**：diff 刷新覆盖了用户改过的开关、或消失列被 DELETE → 用户配置丢失、违反 spec §4 → Task 6「开关保留 + deprecated 行仍在 + 重新出现恢复」三断言。
3. **导入校验缺口**：重复导入/非法 module 名/类名冲突/非 id 主键表 → 撞唯一键 500、生成物写进非法路径、或坏配置入库 → Task 5 四类拒绝测试（均断言 `errors.Is(*, ErrTableInvalid/ErrColumnInvalid)` 且库内 0 新行）。
4. **菜单 seed 不幂等或权限映射缺失**：迁移重跑产生重复菜单行、或 `permissionRoutes` 漏注册 `infra:codegen` → 前端菜单重复 / admin 调 codegen API 403 → Task 9 迁移文件双跑计数不变 + Go 侧路由映射断言。
5. **tenant_id 客户端可控**：请求体灌 `tenant_id` 或 Update 零值把行迁出租户 → 跨租户数据污染 → Task 2：builder 把 `tenant_id` 的 JSONName 置 `"-"` 的断言 + 模板打戳断言 + service_test 模板用 `WithTenantID(ctx,42)` 打戳断言。

## File Structure

```
docs/superpowers/specs/2026-10-10-codegen-redesign-design.md   # Task 1：errata（主键约定/租户写路径/硬删除/deprecated）
pkg/codegen/builder/builder.go                                  # Task 1：validateSinglePK；Task 2：tenant_id JSONName="-"
pkg/codegen/builder/builder_test.go                             # Task 1：三类拒绝测试
pkg/codegen/template/templates/server/go/single/
├── repository.go.tmpl                                          # Task 2：Create/Update 租户打戳 + 过滤更新
├── service.go.tmpl                                             # Task 2：Create/Update 传租户
└── service_test.go.tmpl                                        # Task 2：fake 签名 + 打戳断言
pkg/codegen/template/tenant_writepath_test.go                   # Task 2：渲染断言 + gorm 语义镜像测试
pkg/codegen/testdata/golden/*                                   # Task 2：-update 刷新（model/repository/service/service_test 4 个）
modules/infra/
├── module.go                                                   # Task 3：骨架（迁移源）；Task 7：完整形态（repo/svc/ctrl/reader/route）
├── migration_source.go                                         # Task 3
├── migrations/
│   ├── 20261011000001_codegen_config.up.sql / .down.sql        # Task 3：建表
│   ├── 20261011000002_codegen_menus.up.sql / .down.sql         # Task 9：菜单/按钮/角色绑定 seed
│   └── migrate_test.go                                         # Task 3：迁移集成测试（-tags integration，Task 9 扩展 seed 断言）
├── model/codegen.go                                            # Task 4：CodegenTable / CodegenColumn gorm 模型
├── repository/
│   ├── codegen_repo.go                                         # Task 4：接口 + 实现（Task 6 追加 ApplySync）
│   └── codegen_repo_test.go                                    # Task 4：sqlite 测试
├── service/
│   ├── codegen_service.go                                      # Task 5：导入 + 表/字段编排（Task 6 追加 Sync）
│   ├── codegen_service_test.go                                 # Task 5/6：fake reader + sqlite repo 测试
│   └── acceptance_test.go                                      # Task 8：M2 验收 HTTP 测试（全链路）
└── controller/admin/
    ├── codegen.go                                              # Task 7：9 端点
    └── codegen_test.go                                         # Task 7：HTTP 测试（stub service）
alexgo-server/cmd/main.go                                       # Task 3：追加 infra.FxModule
modules/system/service/permission_routes.go                     # Task 9：permissionRoutes 加 "infra:codegen"
modules/system/service/permission_routes_test.go                # Task 9：路由映射断言（新建）
tools/codegen/main.go                                           # Task 10：schemaFromDSN → mysql.ParseDSN
tools/codegen/main_test.go                                      # Task 10：schemaFromDSN 测试
pkg/codegen/postprocess/golang.go                               # Task 10：FormatGo 非 .go 直通
pkg/codegen/postprocess/postprocess_test.go                     # Task 10：直通测试
pkg/codegen/naming/naming_test.go                               # Task 10：Snake 边界例
pkg/config/loader_test.go                                       # Task 10：DEPLOYMENT_MODE/SYSTEM_GRPC_ADDR env 覆盖
tools/codegen/smoke_integration_test.go                         # Task 10：表名随机后缀
```

---
---

### Task 1: spec 勘误回填 + 构建期单列 `id` 主键拒绝

**Files:**
- Modify: `docs/superpowers/specs/2026-10-10-codegen-redesign-design.md`（文末追加 §9 勘误）
- Modify: `pkg/codegen/builder/builder.go`
- Test: `pkg/codegen/builder/builder_test.go`

**Interfaces:**
- Consumes: `model.ErrColumnInvalid`、`metadata.TableMeta{Columns []ColumnMeta{Name, Key, ...}}`（M1 已有）。
- Produces（后续任务依赖，签名精确）:
  - `builder.Build` 对**任意**模板类型拒绝三类主键：无主键 / 复合主键 / 非 `id` 主键 → `error` 满足 `errors.Is(err, model.ErrColumnInvalid)`
  - spec §9 勘误四条（§9.1 主键约定、§9.2 租户写路径、§9.3 硬删除、§9.4 同步语义）——Task 2/5/6/7/8 的验收都论证到 §9。

- [ ] **Step 1: spec 追加勘误文本**

在 `docs/superpowers/specs/2026-10-10-codegen-redesign-design.md` 文件末尾追加：

```markdown
## 9. 勘误（2026-10-11，M2 实施前回填）

以下四条为 M1 实施后裁决的补充约定，随 M2 落地（优先级等同正文）。

### 9.1 主键约定：仅支持单列 `id` 主键

- 生成器只支持主键恰为单列 `id`（通常自增）的表；无主键、复合主键、主键非 `id`
  （如 `item_id`）的表**在构建期拒绝**：`builder.Build` 返回 `ErrColumnInvalid`，
  坏配置不入库、生成物不落非法路径。
- 覆盖全部模板类型（single/tree/main_sub 共用同一约定，M5/M6 不另行放宽）。

### 9.2 租户写路径（M1 缺口，M2 内修复）

- 生成模型的 `tenant_id` 字段 JSON tag 固定为 `"-"`：客户端请求体永远灌不进租户值。
- repository `Create` 打戳：`entity.TenantID` 为零时从 `tenant.TenantIDFromContext(ctx)` 回填。
- repository `Update` 禁用 `Save`：必须 `Where("id = ? AND tenant_id = ?", ...)` +
  `Select("*").Updates(...)`——gorm 的 `Save` 丢弃链式 WHERE，跨租户照改不误
  （语义镜像测试钉死：`pkg/codegen/template/tenant_writepath_test.go`）。
- service `Create`/`Update` 从 ctx 取租户写入 entity。

### 9.3 配置删除 = 硬删除

- `codegen_table` / `codegen_column` 不设 `deleted` 列：DELETE 在事务内物理删行
  （级联删字段配置）。配置是系统级管理数据，无审计恢复诉求；软删还会让
  `table_name` 唯一键在「删了再导」时冲突。

### 9.4 同步语义（§4「导入/同步语义」的可执行定义）

对「既有配置行 vs 当前元数据列」按列名做 diff：

| 情形 | 动作 | 计数 |
|---|---|---|
| 元数据有、配置无 | 追加新配置行，**全部开关默认关闭**（list/form/query=false、required=false） | added |
| 配置有、元数据无、且 `deprecated=0` | 置 `deprecated=1`，**不物理删** | deprecated |
| 两边都有 | 只刷新快照字段 `type/comment/go_type/json_name/is_pk/auto_increment/nullable`；**开关与配置字段（`html_type/query_operation/dict_type/sort_order`）一律不覆盖** | 快照有变化才计 updated |
| 曾 `deprecated=1` 的列重新出现 | 清 `deprecated=0` + 刷新快照 | 计 updated |
| 两边都有且快照无变化 | 不写库 | unchanged |

- 表级同步同时刷新 `table_comment` 快照；`module/class_name/template_type/front_type/remark`
  等表级配置不覆盖。
```

- [ ] **Step 2: 写失败测试**

在 `pkg/codegen/builder/builder_test.go` 追加（复用同包 `fixtureMeta()`）：

```go
// 三类主键拒绝（spec §9.1）：坏表在构建期就报 ErrColumnInvalid，不进入配置/生成链路。
func TestBuild_PrimaryKeyRejections(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*metadata.TableMeta)
	}{
		{"no primary key", func(m *metadata.TableMeta) {
			for i := range m.Columns {
				m.Columns[i].Key = ""
			}
		}},
		{"composite primary key", func(m *metadata.TableMeta) {
			m.Columns[1].Key = "PRI" // id 已是 PRI → 两列复合
		}},
		{"non-id primary key", func(m *metadata.TableMeta) {
			m.Columns[0].Name = "item_id" // PRI 列不叫 id
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			meta := fixtureMeta()
			c.mutate(meta)
			_, err := builder.Build(meta, builder.Options{Module: "order"})
			if !errors.Is(err, model.ErrColumnInvalid) {
				t.Fatalf("Build() err = %v, want ErrColumnInvalid", err)
			}
		})
	}
}
```

- [ ] **Step 3: 运行确认失败**

Run: `go test ./pkg/codegen/builder/ -run TestBuild_PrimaryKeyRejections -v`
Expected: FAIL，三个子测试均 `err = <nil>, want ErrColumnInvalid`

- [ ] **Step 4: 最小实现**

`pkg/codegen/builder/builder.go`：在 `Build` 的 `len(meta.Columns) == 0` 校验之后、列循环之前加调用，并新增函数：

```go
// validateSinglePK：仅支持单列 `id` 主键（spec §9.1）。无主键 / 复合主键 / 非 `id`
// 主键在构建期拒绝，防止坏配置入库与生成物写进非法路径。
func validateSinglePK(meta *metadata.TableMeta) error {
	pks := 0
	for _, c := range meta.Columns {
		if c.Key != "PRI" {
			continue
		}
		pks++
		if c.Name != "id" {
			return fmt.Errorf("%w: %s: primary key must be `id`, got %q",
				model.ErrColumnInvalid, meta.Name, c.Name)
		}
	}
	if pks == 0 {
		return fmt.Errorf("%w: %s: no primary key", model.ErrColumnInvalid, meta.Name)
	}
	if pks > 1 {
		return fmt.Errorf("%w: %s: composite primary key not supported", model.ErrColumnInvalid, meta.Name)
	}
	return nil
}
```

`Build` 内（在 `if len(meta.Columns) == 0 {...}` 之后）：

```go
	if err := validateSinglePK(meta); err != nil {
		return nil, err
	}
```

- [ ] **Step 5: 运行全量 builder + 引擎测试确认通过、且未打破既有用例**

Run: `go test ./pkg/codegen/... -count=1`
Expected: PASS（既有 `TestBuild_*`、Golden 均不受影响——fixture 主键就是单列 `id`）

- [ ] **Step 6: Commit**

```bash
git add docs/superpowers/specs/2026-10-10-codegen-redesign-design.md pkg/codegen/builder/
git commit -m "feat(codegen): 单列 id 主键构建期拒绝 + spec §9 勘误回填"
```

---

### Task 2: 租户写路径——`tenant_id` JSON 隐藏 + Create 打戳 + Update 过滤更新

**Files:**
- Modify: `pkg/codegen/builder/builder.go`（`tenant_id` 的 `JSONName` 置 `"-"`）
- Modify: `pkg/codegen/template/templates/server/go/single/repository.go.tmpl`（import + Create 打戳 + Update 双条件过滤）
- Modify: `pkg/codegen/template/templates/server/go/single/service.go.tmpl`（Create/Update 从 ctx 取租户）
- Modify: `pkg/codegen/template/templates/server/go/single/service_test.go.tmpl`（fake + 打戳断言）
- Test: `pkg/codegen/template/tenant_writepath_test.go`（新建：渲染断言 + gorm 语义镜像）
- Refresh: `pkg/codegen/testdata/golden/*`（model/repository/service/service_test 4 个）

**Interfaces:**
- Consumes: Task 1 的 builder；M1 的 `template.Render(tbl, template.Options{UnitTestEnable})`、`builder.Build(meta, builder.Options{Module})`、golden `-update` 开关（`go test ./pkg/codegen -run Golden -update`）。
- Produces（M3 预览/下载与后续任务依赖）:
  - 生成的 `tenant_id` 字段 `json:"-"`；请求体无法写入租户
  - 生成 repository 的 `Update`：`Where("id = ? AND tenant_id = ?", entity.ID, tenant.TenantIDFromContext(ctx))` + `Select("*").Updates(entity)`；`Create` 零值打戳
  - 生成 service 的 `Create`/`Update`：`entity.TenantID = tenant.TenantIDFromContext(ctx)`
  - 非租户表（无 `tenant_id` 列）渲染分支保持原样（`Save` 无租户可泄露，不受此约束）

- [ ] **Step 1: 写失败测试——渲染断言 + gorm 语义镜像**

新建 `pkg/codegen/template/tenant_writepath_test.go`：

```go
package template_test

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/pkg/codegen/builder"
	"alexGo-cloud/pkg/codegen/metadata"
	"alexGo-cloud/pkg/codegen/template"
)

// renderSingle 用 fixture 元数据渲染单表后端模板族，返回指定模板的产物源码。
func renderSingle(t *testing.T, columns []metadata.ColumnMeta, wantPathSuffix string) string {
	t.Helper()
	meta := &metadata.TableMeta{Schema: "alexgo", Name: "codegen_demo_items", Comment: "demo", Columns: columns}
	tbl, err := builder.Build(meta, builder.Options{Module: "order"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	files, err := template.Render(tbl, template.Options{UnitTestEnable: true})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, f := range files {
		if strings.HasSuffix(f.Path, wantPathSuffix) {
			return string(f.Content)
		}
	}
	t.Fatalf("no generated file ends with %q", wantPathSuffix)
	return ""
}

func tenantColumns() []metadata.ColumnMeta {
	return []metadata.ColumnMeta{
		{Name: "id", DataType: "bigint", ColumnType: "bigint unsigned", Key: "PRI", Extra: "auto_increment", Ordinal: 1},
		{Name: "name", DataType: "varchar", ColumnType: "varchar(64)", Comment: "名称", Ordinal: 2},
		{Name: "tenant_id", DataType: "bigint", ColumnType: "bigint unsigned", Comment: "租户", Ordinal: 3},
		{Name: "created_at", DataType: "datetime", ColumnType: "datetime", Comment: "创建时间", Ordinal: 4},
	}
}

// 渲染断言（spec §9.2）：租户表的产物必须隐藏 tenant_id、打戳、双条件过滤更新，且不用 Save。
func TestTenantWritePath_Render(t *testing.T) {
	repo := renderSingle(t, tenantColumns(), "repository/codegen_demo_item.go")
	for _, want := range []string{
		`"alexGo-cloud/pkg/tenant"`,
		`Where("id = ? AND tenant_id = ?", entity.ID, tenant.TenantIDFromContext(ctx))`,
		`Select("*").Updates(entity)`,
		`entity.TenantID = 0`,
	} {
		if !strings.Contains(repo, want) {
			t.Errorf("repository 产物缺少 %q", want)
		}
	}
	if strings.Contains(repo, `Save(entity)`) {
		t.Error("repository 产物含 Save(entity)：Save 丢弃链式 WHERE，跨租户照改（spec §9.2）")
	}

	svc := renderSingle(t, tenantColumns(), "service/codegen_demo_item.go")
	if got := strings.Count(svc, `entity.TenantID = tenant.TenantIDFromContext(ctx)`); got < 2 {
		t.Errorf("service 产物打戳语句 %d 处, want >= 2（Create + Update）", got)
	}

	modelSrc := renderSingle(t, tenantColumns(), "model/codegen_demo_item.go")
	for _, line := range strings.Split(modelSrc, "\n") {
		if strings.Contains(line, "TenantID") {
			if !strings.Contains(line, `json:"-"`) {
				t.Errorf("TenantID 行未隐藏 json: %s", line)
			}
			return
		}
	}
	t.Error("model 产物无 TenantID 字段")
}

// gorm 语义镜像：证明模板禁用 Save 的理由——
// ① 双条件 + Select("*").Updates 把行锁在租户内（跨租户 0 行）；
// ② 链式 WHERE 后接 Save，gorm 丢弃 WHERE 照改不误（这是要防的坑本身）。
func TestTenantWritePath_GormSemantics(t *testing.T) {
	type mirrorRow struct {
		ID       uint64 `gorm:"primaryKey"`
		TenantID uint64
		Data     string
	}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&mirrorRow{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&mirrorRow{ID: 1, TenantID: 1, Data: "original"}).Error; err != nil {
		t.Fatal(err)
	}

	// ① 模板现行写法：租户不匹配 → 0 行受影响，数据不动。
	err = db.Model(&mirrorRow{}).
		Where("id = ? AND tenant_id = ?", 1, 2).
		Select("*").Updates(&mirrorRow{ID: 1, TenantID: 2, Data: "hacked"}).Error
	if err != nil {
		t.Fatalf("Updates: %v", err)
	}
	var row mirrorRow
	if err := db.First(&row, 1).Error; err != nil {
		t.Fatal(err)
	}
	if row.Data != "original" {
		t.Errorf("跨租户 Updates 改动了数据: %+v, want 保持 original", row)
	}

	// ② 反例（characterization of gorm footgun）：链式 WHERE 被 Save 丢弃。
	_ = db.Where("id = ? AND tenant_id = ?", 1, 2).Save(&mirrorRow{ID: 1, TenantID: 1, Data: "hacked-save"})
	if err := db.First(&row, 1).Error; err != nil {
		t.Fatal(err)
	}
	if row.Data != "hacked-save" {
		t.Errorf("Save 行为变化: data=%q；若此处失败说明 gorm 语义已变，重新审视模板 Update 写法", row.Data)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./pkg/codegen/template/ -run TestTenantWritePath -v`
Expected: FAIL——渲染断言报「repository 产物缺少 Where(...)」「含 Save(entity)」「TenantID 行未隐藏 json」；镜像测试 ② 可能先过（characterization，不阻塞）

- [ ] **Step 3: 最小实现**

① `pkg/codegen/builder/builder.go` 列循环内，把现有

```go
		if c.Name == "tenant_id" {
			tbl.HasTenant = true
		}
```

改为

```go
		jsonName := naming.JSONName(c.Name)
		if c.Name == "tenant_id" {
			tbl.HasTenant = true
			// 租户字段客户端不可控（spec §9.2）：json:"-" 后请求体灌不进值，打戳只源自 ctx。
			jsonName = "-"
		}
```

并在结构体字面量里 `JSONName: jsonName,`（替换原来的 `JSONName: naming.JSONName(c.Name),`）。

② `repository.go.tmpl`——import 块加租户包：

```gotemplate
import (
	"context"

	"alexGo-cloud/modules/{{.Module}}/model"
{{- if .HasTenant }}
	"alexGo-cloud/pkg/tenant"
{{- end }}
	"gorm.io/gorm"
)
```

`Create` 替换为：

```gotemplate
func (r *{{ lower .Entity }}Repo) Create(ctx context.Context, entity *model.{{.Entity}}) error {
	{{- if .HasTenant }}
	// 打戳（spec §9.2）：json:"-" 使客户端灌不进来，零值才回填、显式值保留。
	if entity.TenantID == 0 {
		entity.TenantID = tenant.TenantIDFromContext(ctx)
	}
	{{- end }}
	return r.db.WithContext(ctx).Create(entity).Error
}
```

`Update` 替换为：

```gotemplate
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
```

③ `service.go.tmpl`——`Create`/`Update` 替换为：

```gotemplate
func (s *{{ lower .Entity }}Service) Create(ctx context.Context, entity *model.{{.Entity}}) error {
	{{- if .HasTenant }}
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	{{- end }}
	return s.repo.Create(ctx, entity)
}
```

```gotemplate
func (s *{{ lower .Entity }}Service) Update(ctx context.Context, entity *model.{{.Entity}}) error {
	{{- if .HasTenant }}
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	{{- end }}
	return s.repo.Update(ctx, entity)
}
```

④ `service_test.go.tmpl`——import 加：

```gotemplate
	"alexGo-cloud/modules/{{.Module}}/model"
{{- if .HasTenant }}
	"alexGo-cloud/pkg/tenant"
{{- end }}
```

`Test{{.Entity}}Service_CRUD` 内 `ctx := context.Background()` 后加：

```gotemplate
	{{- if .HasTenant }}
	ctx = tenant.WithTenantID(ctx, 42)
	{{- end }}
```

`Create` 成功后加断言：

```gotemplate
	{{- if .HasTenant }}
	if e.TenantID != 42 {
		t.Fatalf("Create 应从 ctx 打戳租户 42, got %d", e.TenantID)
	}
	{{- end }}
```

- [ ] **Step 4: 运行渲染断言确认通过**

Run: `go test ./pkg/codegen/template/ -run TestTenantWritePath -v`
Expected: PASS（含 gorm 镜像两条）

- [ ] **Step 5: Golden 变红 → 刷新 → 全绿**

Run: `go test ./pkg/codegen -run Golden`
Expected: FAIL（model/repository/service/service_test 4 个 golden 与新渲染不一致，提示 `(run: go test ./pkg/codegen -run Golden -update)`）

Run: `go test ./pkg/codegen -run Golden -update`
Run: `go test ./pkg/codegen/... -count=1`
Expected: PASS

刷新后人工核对（Step 6 前）：`git diff pkg/codegen/testdata/golden/` 应只见 4 个文件变化，且
- `modules_order_model_codegen_demo_item.go`：`TenantID ... json:"-"`（与 password 同款隐藏）
- `modules_order_repository_codegen_demo_item.go`：出现 `Where("id = ? AND tenant_id = ?"`、`Select("*").Updates`，无 `Save(entity)`
- `modules_order_service_codegen_demo_item.go`：Create/Update 各一处 `entity.TenantID = tenant.TenantIDFromContext(ctx)`
- `modules_order_service_codegen_demo_item_test.go`：`tenant.WithTenantID(ctx, 42)` + 打戳断言
- 其余 3 个 golden（controller ×2、module_register）**零变化**

- [ ] **Step 6: Commit**

```bash
git add pkg/codegen/builder/ pkg/codegen/template/ pkg/codegen/testdata/golden/
git commit -m "fix(codegen): 租户写路径——tenant_id json隐藏 + Create打戳 + Update双条件过滤"
```

---

### Task 3: `modules/infra` 骨架——fx 模块 + 迁移源 + 建表迁移

**Files:**
- Create: `modules/infra/module.go`
- Create: `modules/infra/migration_source.go`
- Create: `modules/infra/migrations/20261011000001_codegen_config.up.sql`
- Create: `modules/infra/migrations/20261011000001_codegen_config.down.sql`
- Test: `modules/infra/migrations/migrate_test.go`（`-tags integration`）
- Modify: `alexgo-server/cmd/main.go`（追加 `infra.FxModule`）

**Interfaces:**
- Consumes: `pkg/migrate.Source`（`GetMigrationPath()/GetModuleName()`）、`migrate.Runner`（按 `group:"migration_sources"` 聚合）、member 模块的 fx 装配惯例。
- Produces（Task 4–9 依赖）:
  - `infra.FxModule fx.Option`——Task 7 在其中追加三层 Provider 与路由
  - `infra.NewMigrationSource() migrate.Source`，path=`modules/infra/migrations`，module=`infra`（迁移记录表 `schema_migrations_infra`）
  - 表 `codegen_table`（`uk_codegen_table_name` 唯一键、**无 `tenant_id`、无 `deleted`**——spec §9.3）与 `codegen_column`（`uk_codegen_column_table_name(table_id,name)`、`deprecated` 列）
  - `migrate_test.go` 的双重跑幂等断言骨架（Task 9 在同文件追加 seed 断言）

- [ ] **Step 1: 写失败测试——迁移集成测试**

新建 `modules/infra/migrations/migrate_test.go`：

```go
//go:build integration

package migrations

import (
	"os"
	"testing"

	"gorm.io/gorm"
	"gorm.io/driver/mysql"

	"alexGo-cloud/modules/infra"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/migrate"
)

// runInfra 用 DB_DSN 构造 Runner 只跑 infra 迁移源，返回 gorm 句柄供断言。
func runInfra(t *testing.T) (*gorm.DB, *migrate.Runner) {
	t.Helper()
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		t.Skip("DB_DSN not set; skip integration test")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("mysql unreachable: %v", err)
	}
	cfg := &config.Config{}
	cfg.Migrate.Auto = true
	cfg.Database.DSN = dsn
	runner := migrate.NewRunner(db, cfg, []migrate.Source{infra.NewMigrationSource()})
	return db, runner
}

func countCodegenRows(t *testing.T, db *gorm.DB) (tables, columns int64) {
	t.Helper()
	if !db.Migrator().HasTable("codegen_table") || !db.Migrator().HasTable("codegen_column") {
		t.Fatal("codegen_table / codegen_column 未建表")
	}
	if err := db.Table("codegen_table").Count(&tables).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("codegen_column").Count(&columns).Error; err != nil {
		t.Fatal(err)
	}
	return tables, columns
}

// 双跑幂等：第一次跑建表，第二次跑行数与迁移记录均不增长；
// 系统级配置表不得有 tenant_id / deleted 列（spec §1 非目标 + §9.3）。
func TestInfraMigrations_UpTwiceStable(t *testing.T) {
	db, runner := runInfra(t)

	if err := runner.Run(); err != nil {
		t.Fatalf("run #1: %v", err)
	}
	t1, c1 := countCodegenRows(t, db)

	var n1 int64
	if err := db.Table("schema_migrations_infra").Count(&n1).Error; err != nil {
		t.Fatal(err)
	}
	if n1 < 1 {
		t.Fatal("schema_migrations_infra 无记录")
	}

	if err := runner.Run(); err != nil {
		t.Fatalf("run #2: %v", err)
	}
	t2, c2 := countCodegenRows(t, db)
	if t2 != t1 || c2 != c1 {
		t.Errorf("双跑行数变化: tables %d→%d, columns %d→%d", t1, t2, c1, c2)
	}
	var n2 int64
	if err := db.Table("schema_migrations_infra").Count(&n2).Error; err != nil {
		t.Fatal(err)
	}
	if n2 != n1 {
		t.Errorf("迁移记录 %d→%d, want 不变", n1, n2)
	}

	for _, col := range []string{"tenant_id", "deleted"} {
		if db.Migrator().HasColumn("codegen_table", col) {
			t.Errorf("codegen_table 含禁用列 %q（spec §1/§9.3：系统级、硬删除）", col)
		}
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test -tags integration ./modules/infra/migrations/ -count=1 -v`
Expected: FAIL（`no required module provides package alexGo-cloud/modules/infra`）

- [ ] **Step 3: 最小实现**

`modules/infra/migration_source.go`：

```go
package infra

import "alexGo-cloud/pkg/migrate"

type migrationSource struct{}

func NewMigrationSource() migrate.Source {
	return &migrationSource{}
}

// GetMigrationPath 迁移 SQL 所在目录（相对仓库根，Runner 以 file:// 打开）。
func (s *migrationSource) GetMigrationPath() string {
	return "modules/infra/migrations"
}

// GetModuleName 决定 schema_migrations_<name> 记录表。
func (s *migrationSource) GetModuleName() string {
	return "infra"
}
```

`modules/infra/module.go`（Task 7 会在 `fx.Provide` 里追加三层构造器）：

```go
package infra

import "go.uber.org/fx"

// FxModule infra 模块的 fx 装配。系统级功能：不做租户隔离（spec §1）。
// M2 先挂迁移源；三层与 admin 路由在 Task 7 追加。
var FxModule = fx.Module("infra",
	fx.Provide(
		fx.Annotate(
			NewMigrationSource,
			fx.As(new(migrate.Source)),
			fx.ResultTags(`group:"migration_sources"`),
		),
	),
)
```

（注意补 import：`"alexGo-cloud/pkg/migrate"`。）

`modules/infra/migrations/20261011000001_codegen_config.up.sql`：

```sql
-- codegen 配置表（spec §4）：元数据快照 + 可编辑配置。
-- 系统级功能：无 tenant_id（§1 非目标）、硬删除无 deleted 列（§9.3）。
-- deleted 语义见 §9.4：codegen_column.deprecated 标记消失列，不物理删。

CREATE TABLE IF NOT EXISTS `codegen_table` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `table_name` varchar(128) NOT NULL,
  `table_comment` varchar(255) NOT NULL DEFAULT '',
  `module` varchar(64) NOT NULL DEFAULT '',
  `business_name` varchar(128) NOT NULL DEFAULT '',
  `class_name` varchar(128) NOT NULL DEFAULT '',
  `template_type` tinyint NOT NULL DEFAULT 1,
  `front_type` tinyint NOT NULL DEFAULT 1,
  `parent_table_id` bigint unsigned NOT NULL DEFAULT 0,
  `remark` varchar(255) NOT NULL DEFAULT '',
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_codegen_table_name` (`table_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `codegen_column` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `table_id` bigint unsigned NOT NULL,
  `name` varchar(128) NOT NULL,
  `type` varchar(64) NOT NULL DEFAULT '',
  `comment` varchar(255) NOT NULL DEFAULT '',
  `go_type` varchar(64) NOT NULL DEFAULT '',
  `json_name` varchar(128) NOT NULL DEFAULT '',
  `is_pk` tinyint(1) NOT NULL DEFAULT 0,
  `auto_increment` tinyint(1) NOT NULL DEFAULT 0,
  `nullable` tinyint(1) NOT NULL DEFAULT 0,
  `html_type` varchar(32) NOT NULL DEFAULT '',
  `list_enable` tinyint(1) NOT NULL DEFAULT 1,
  `form_enable` tinyint(1) NOT NULL DEFAULT 1,
  `query_enable` tinyint(1) NOT NULL DEFAULT 1,
  `query_operation` varchar(16) NOT NULL DEFAULT 'eq',
  `list_required` tinyint(1) NOT NULL DEFAULT 0,
  `form_required` tinyint(1) NOT NULL DEFAULT 0,
  `dict_type` varchar(64) NOT NULL DEFAULT '',
  `sort_order` int NOT NULL DEFAULT 0,
  `deprecated` tinyint(1) NOT NULL DEFAULT 0,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_codegen_column_table_name` (`table_id`,`name`),
  KEY `idx_codegen_column_table` (`table_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

`modules/infra/migrations/20261011000001_codegen_config.down.sql`：

```sql
-- 逆序删除配置表（硬删除语义：配置行即全部状态，无恢复诉求）。
DROP TABLE IF EXISTS `codegen_column`;
DROP TABLE IF EXISTS `codegen_table`;
```

`alexgo-server/cmd/main.go`：import 块加 `"alexGo-cloud/modules/infra"`，并在 `system.FxModule,`（约 172 行）之前加：

```go
		// infra：代码生成配置持久化（M2）——迁移源 + 三层 + admin 路由。
		infra.FxModule,
```

- [ ] **Step 4: 运行迁移集成测试（需本地 MySQL）**

Run: `DB_DSN='user:pass@tcp(127.0.0.1:3306)/alexgo?charset=utf8mb4&parseTime=True&loc=Local' go test -tags integration ./modules/infra/migrations/ -count=1 -v`
Expected: PASS（无 DB_DSN 时为 `skip`——如实记录跳过）

- [ ] **Step 5: fx 干跑测试确认依赖图完好**

Run: `CGO_ENABLED=0 go test ./alexgo-server/cmd/ -count=1`
Expected: PASS（`TestMono_GraphValidates` / `TestMicro_GraphValidates` / `TestMigrateOnly_GraphValidates` 全绿；`CGO_ENABLED=0` 是本机 cgo 链接问题的既定绕法，见 architecture.md §8）

- [ ] **Step 6: Commit**

```bash
git add modules/infra/ alexgo-server/cmd/main.go
git commit -m "feat(infra): 模块骨架、迁移源与 codegen 配置表迁移"
```

---

### Task 4: gorm 模型 + 配置仓库（sqlite 测试）

**Files:**
- Create: `modules/infra/model/codegen.go`
- Create: `modules/infra/repository/codegen_repo.go`
- Test: `modules/infra/repository/codegen_repo_test.go`

**Interfaces:**
- Consumes: Task 3 的表结构；`github.com/glebarez/sqlite`（纯 Go 内存库，仓库既有惯例）；`model.SyncDiff/SyncResult`（本任务一并定义，Task 6 消费）。
- Produces（Task 5–8 依赖，签名精确）:
  - `model.CodegenTable{ID, TableName, TableComment, Module, BusinessName, ClassName string; TemplateType, FrontType int; ParentTableID uint64; Remark string; CreatedAt, UpdatedAt time.Time}`，`TableName() = "codegen_table"`
  - `model.CodegenColumn{ID, TableID uint64; Name, Type, Comment, GoType, JSONName string; IsPK, AutoIncrement, Nullable bool; HTMLType string; ListEnable, FormEnable, QueryEnable bool; QueryOperation string; ListRequired, FormRequired bool; DictType string; SortOrder int; Deprecated bool; CreatedAt, UpdatedAt time.Time}`，`TableName() = "codegen_column"`
  - `model.SyncDiff{Added []*CodegenColumn; Refresh []*CodegenColumn; Deprecated []uint64}`
  - `model.SyncResult{Added, Updated, Deprecated, Unchanged int}`（json: `added/updated/deprecated/unchanged`）
  - `repository.CodegenRepository` 接口（下方代码块全文）

- [ ] **Step 1: 写失败测试**

新建 `modules/infra/repository/codegen_repo_test.go`：

```go
package repository

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/infra/model"
)

func setupRepo(t *testing.T) (*gorm.DB, CodegenRepository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.CodegenTable{}, &model.CodegenColumn{}); err != nil {
		t.Fatal(err)
	}
	return db, NewCodegenRepository(db)
}

func seedTable(t *testing.T, repo CodegenRepository) *model.CodegenTable {
	t.Helper()
	tbl := &model.CodegenTable{TableName: "order_items", Module: "order", ClassName: "OrderItem", TemplateType: 1, FrontType: 1}
	cols := []*model.CodegenColumn{
		{Name: "id", Type: "bigint", GoType: "uint64", JSONName: "id", IsPK: true, AutoIncrement: true, SortOrder: 0},
		{Name: "name", Type: "varchar(64)", GoType: "string", JSONName: "name", SortOrder: 1},
	}
	if err := repo.CreateTable(context.Background(), tbl, cols); err != nil {
		t.Fatal(err)
	}
	return tbl
}

// CreateTable 落表+字段；同名表唯一键冲突报错。
func TestRepo_CreateTableAndCascade(t *testing.T) {
	db, repo := setupRepo(t)
	tbl := seedTable(t, repo)

	var n int64
	db.Model(&model.CodegenColumn{}).Where("table_id = ?", tbl.ID).Count(&n)
	if n != 2 {
		t.Errorf("columns = %d, want 2", n)
	}

	dup := &model.CodegenTable{TableName: "order_items", Module: "order"}
	if err := repo.CreateTable(context.Background(), dup, nil); err == nil {
		t.Error("重复 table_name 应报错（uk_codegen_table_name）")
	}

	// 分页 + total
	got, total, err := repo.ListTables(context.Background(), 1, 10)
	if err != nil || total != 1 || len(got) != 1 || got[0].ID != tbl.ID {
		t.Errorf("ListTables = %d rows total=%d err=%v", len(got), total, err)
	}
}

// DeleteTable 硬删除并级联删字段（spec §9.3：无 deleted 列）。
func TestRepo_DeleteTable_CascadesColumns(t *testing.T) {
	db, repo := setupRepo(t)
	tbl := seedTable(t, repo)

	if err := repo.DeleteTable(context.Background(), tbl.ID); err != nil {
		t.Fatal(err)
	}
	var tc, cc int64
	db.Model(&model.CodegenTable{}).Count(&tc)
	db.Model(&model.CodegenColumn{}).Count(&cc)
	if tc != 0 || cc != 0 {
		t.Errorf("after delete: tables=%d columns=%d, want 0/0", tc, cc)
	}
	if _, err := repo.GetTable(context.Background(), tbl.ID); err == nil {
		t.Error("已删行 GetTable 应报错")
	}
}

// SaveColumns 只覆盖配置字段：快照字段（type/comment/go_type/...）不被动。
func TestRepo_SaveColumns_ConfigOnly(t *testing.T) {
	db, repo := setupRepo(t)
	tbl := seedTable(t, repo)

	cols, err := repo.ListColumns(context.Background(), tbl.ID)
	if err != nil || len(cols) != 2 {
		t.Fatalf("ListColumns = %d, err=%v", len(cols), err)
	}
	target := cols[1] // name 列
	origComment, origType := target.Comment, target.Type
	target.ListEnable = false
	target.HTMLType = "Select"
	target.QueryOperation = "between"
	target.Comment = "想覆盖快照的攻击值"
	target.Type = "hacked"
	if err := repo.SaveColumns(context.Background(), tbl.ID, []*model.CodegenColumn{target}); err != nil {
		t.Fatal(err)
	}

	var after model.CodegenColumn
	db.First(&after, target.ID)
	if after.ListEnable || after.HTMLType != "Select" || after.QueryOperation != "between" {
		t.Errorf("配置字段未保存: %+v", after)
	}
	if after.Comment != origComment || after.Type != origType {
		t.Errorf("快照字段被覆盖: comment %q→%q type %q→%q", origComment, after.Comment, origType, after.Type)
	}

	// 跨表 id（table_id 不匹配）→ 拒绝且 0 行改动
	other := *target
	other.ID = 999999
	other.ListEnable = false
	if err := repo.SaveColumns(context.Background(), tbl.ID, []*model.CodegenColumn{&other}); err == nil {
		t.Error("table_id 不匹配应报错")
	}
}

// ImportedTableNames / ListAllTables：导入去重与类名冲突检测的数据源。
func TestRepo_ImportHelpers(t *testing.T) {
	_, repo := setupRepo(t)
	tbl := seedTable(t, repo)

	names, err := repo.ImportedTableNames(context.Background())
	if err != nil || len(names) != 1 || names[0] != "order_items" {
		t.Errorf("ImportedTableNames = %v, err=%v", names, err)
	}
	all, err := repo.ListAllTables(context.Background())
	if err != nil || len(all) != 1 || all[0].ClassName != tbl.ClassName {
		t.Errorf("ListAllTables = %d rows err=%v", len(all), err)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./modules/infra/repository/ -count=1`
Expected: FAIL（`undefined: CodegenRepository`、包不存在）

- [ ] **Step 3: 最小实现**

`modules/infra/model/codegen.go`：

```go
package model

import "time"

// CodegenTable 代码生成表配置（快照+配置）。系统级：无 tenant_id、无 deleted（spec §1/§9.3）。
type CodegenTable struct {
	ID            uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TableName     string    `gorm:"column:table_name" json:"table_name"`
	TableComment  string    `gorm:"column:table_comment" json:"table_comment"`
	Module        string    `gorm:"column:module" json:"module"`
	BusinessName  string    `gorm:"column:business_name" json:"business_name"`
	ClassName     string    `gorm:"column:class_name" json:"class_name"`
	TemplateType  int       `gorm:"column:template_type" json:"template_type"`
	FrontType     int       `gorm:"column:front_type" json:"front_type"`
	ParentTableID uint64    `gorm:"column:parent_table_id" json:"parent_table_id"`
	Remark        string    `gorm:"column:remark" json:"remark"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (CodegenTable) TableName() string { return "codegen_table" }

// CodegenColumn 字段配置：type/comment/go_type/json_name/主键空性 = 快照；
// html_type/开关/query_operation/dict_type/sort_order = 用户配置（同步不覆盖，§9.4）。
type CodegenColumn struct {
	ID             uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TableID        uint64    `gorm:"column:table_id" json:"table_id"`
	Name           string    `gorm:"column:name" json:"name"`
	Type           string    `gorm:"column:type" json:"type"`
	Comment        string    `gorm:"column:comment" json:"comment"`
	GoType         string    `gorm:"column:go_type" json:"go_type"`
	JSONName       string    `gorm:"column:json_name" json:"json_name"`
	IsPK           bool      `gorm:"column:is_pk" json:"is_pk"`
	AutoIncrement  bool      `gorm:"column:auto_increment" json:"auto_increment"`
	Nullable       bool      `gorm:"column:nullable" json:"nullable"`
	HTMLType       string    `gorm:"column:html_type" json:"html_type"`
	ListEnable     bool      `gorm:"column:list_enable" json:"list_enable"`
	FormEnable     bool      `gorm:"column:form_enable" json:"form_enable"`
	QueryEnable    bool      `gorm:"column:query_enable" json:"query_enable"`
	QueryOperation string    `gorm:"column:query_operation" json:"query_operation"`
	ListRequired   bool      `gorm:"column:list_required" json:"list_required"`
	FormRequired   bool      `gorm:"column:form_required" json:"form_required"`
	DictType       string    `gorm:"column:dict_type" json:"dict_type"`
	SortOrder      int       `gorm:"column:sort_order" json:"sort_order"`
	Deprecated     bool      `gorm:"column:deprecated" json:"deprecated"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (CodegenColumn) TableName() string { return "codegen_column" }

// SyncDiff 是 service 对比元数据算出的差异（纯数据，由仓库事务落库）。
type SyncDiff struct {
	Added      []*CodegenColumn // 新列：开关全关（§9.4）
	Refresh    []*CodegenColumn // 快照刷新或 deprecated 恢复的既有行（含 ID）
	Deprecated []uint64         // 消失列的行 id（置 deprecated=1）
}

// SyncResult 是同步结果计数（service 算，controller 直接序列化）。
type SyncResult struct {
	Added      int `json:"added"`
	Updated    int `json:"updated"`
	Deprecated int `json:"deprecated"`
	Unchanged  int `json:"unchanged"`
}
```

`modules/infra/repository/codegen_repo.go`：

```go
package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"alexGo-cloud/modules/infra/model"
)

// CodegenRepository 配置表读写。删除一律硬删（spec §9.3）；
// 批量字段保存只写配置字段，快照由同步链路维护（§9.4）。
type CodegenRepository interface {
	ListTables(ctx context.Context, page, size int) ([]*model.CodegenTable, int64, error)
	ListAllTables(ctx context.Context) ([]*model.CodegenTable, error)
	GetTable(ctx context.Context, id uint64) (*model.CodegenTable, error)
	CreateTable(ctx context.Context, t *model.CodegenTable, cols []*model.CodegenColumn) error
	UpdateTable(ctx context.Context, t *model.CodegenTable) error
	DeleteTable(ctx context.Context, id uint64) error
	ListColumns(ctx context.Context, tableID uint64) ([]*model.CodegenColumn, error)
	SaveColumns(ctx context.Context, tableID uint64, cols []*model.CodegenColumn) error
	ImportedTableNames(ctx context.Context) ([]string, error)
	// ApplySync 单事务落同步差异：插新行、标 deprecated、刷快照、刷表注释。
	ApplySync(ctx context.Context, tableID uint64, tableComment string, diff model.SyncDiff) error
}

type codegenRepo struct{ db *gorm.DB }

func NewCodegenRepository(db *gorm.DB) CodegenRepository { return &codegenRepo{db: db} }

func (r *codegenRepo) ListTables(ctx context.Context, page, size int) ([]*model.CodegenTable, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&model.CodegenTable{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []*model.CodegenTable
	err := r.db.WithContext(ctx).
		Order("id desc").
		Offset((page - 1) * size).Limit(size).
		Find(&items).Error
	return items, total, err
}

func (r *codegenRepo) ListAllTables(ctx context.Context) ([]*model.CodegenTable, error) {
	var items []*model.CodegenTable
	err := r.db.WithContext(ctx).Order("id asc").Find(&items).Error
	return items, err
}

func (r *codegenRepo) GetTable(ctx context.Context, id uint64) (*model.CodegenTable, error) {
	var item model.CodegenTable
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// CreateTable 事务内建表+字段；table_name 唯一键冲突原样上抛（导入去重的最后防线）。
func (r *codegenRepo) CreateTable(ctx context.Context, t *model.CodegenTable, cols []*model.CodegenColumn) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(t).Error; err != nil {
			return err
		}
		for _, c := range cols {
			c.TableID = t.ID
			if err := tx.Create(c).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *codegenRepo) UpdateTable(ctx context.Context, t *model.CodegenTable) error {
	return r.db.WithContext(ctx).Save(t).Error
}

// DeleteTable 硬删 + 级联删字段（§9.3）。
func (r *codegenRepo) DeleteTable(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("table_id = ?", id).Delete(&model.CodegenColumn{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(&model.CodegenTable{}).Error
	})
}

func (r *codegenRepo) ListColumns(ctx context.Context, tableID uint64) ([]*model.CodegenColumn, error) {
	var items []*model.CodegenColumn
	err := r.db.WithContext(ctx).
		Where("table_id = ?", tableID).
		Order("sort_order asc, id asc").
		Find(&items).Error
	return items, err
}

// configFields 是用户可编辑、同步不得覆盖的字段（§9.4 的「配置字段」）。
var configFields = []string{
	"html_type", "list_enable", "form_enable", "query_enable",
	"query_operation", "list_required", "form_required", "dict_type", "sort_order",
}

// SaveColumns 按 id 批量覆盖配置字段；table_id 不匹配的行报错（防跨表写）。
func (r *codegenRepo) SaveColumns(ctx context.Context, tableID uint64, cols []*model.CodegenColumn) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, c := range cols {
			res := tx.Model(&model.CodegenColumn{}).
				Where("id = ? AND table_id = ?", c.ID, tableID).
				Select(configFields).
				Updates(c)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return fmt.Errorf("column %d not in table %d", c.ID, tableID)
			}
		}
		return nil
	})
}

func (r *codegenRepo) ImportedTableNames(ctx context.Context) ([]string, error) {
	var names []string
	err := r.db.WithContext(ctx).
		Model(&model.CodegenTable{}).
		Pluck("table_name", &names).Error
	return names, err
}

// snapshotFields 是同步允许刷新的快照字段（§9.4）。
var snapshotFields = []string{
	"type", "comment", "go_type", "json_name",
	"is_pk", "auto_increment", "nullable", "deprecated", "updated_at",
}

// ApplySync 单事务落库（§9.4）：插新行 → 标 deprecated → 刷快照 → 刷表注释。
func (r *codegenRepo) ApplySync(ctx context.Context, tableID uint64, tableComment string, diff model.SyncDiff) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, c := range diff.Added {
			c.ID = 0
			c.TableID = tableID
			if err := tx.Create(c).Error; err != nil {
				return err
			}
		}
		if len(diff.Deprecated) > 0 {
			if err := tx.Model(&model.CodegenColumn{}).
				Where("id IN ? AND table_id = ?", diff.Deprecated, tableID).
				Update("deprecated", true).Error; err != nil {
				return err
			}
		}
		for _, c := range diff.Refresh {
			if err := tx.Model(&model.CodegenColumn{}).
				Where("id = ? AND table_id = ?", c.ID, tableID).
				Select(snapshotFields).
				Updates(c).Error; err != nil {
				return err
			}
		}
		return tx.Model(&model.CodegenTable{}).
			Where("id = ?", tableID).
			Updates(map[string]any{"table_comment": tableComment, "updated_at": time.Now()}).Error
	})
}
```

（补 import `"time"`。）

- [ ] **Step 4: 运行确认通过**

Run: `go test ./modules/infra/repository/ -count=1 -v`
Expected: PASS（4 个测试全绿）

- [ ] **Step 5: Commit**

```bash
git add modules/infra/model/ modules/infra/repository/
git commit -m "feat(infra): codegen 配置模型与仓库（事务级联/配置字段白名单/同步落库)"
```

---

### Task 5: 导入服务——元数据读取 + 四类拒绝 + 配置落库

**Files:**
- Create: `modules/infra/service/codegen_service.go`
- Test: `modules/infra/service/codegen_service_test.go`

**Interfaces:**
- Consumes: `pkg/codegen/metadata`（`MetadataReader`/`TableMeta`/`ColumnMeta`）、`pkg/codegen/builder`（`Build(meta, opts)`）、`pkg/codegen/model` 哨兵（`ErrTableInvalid`/`ErrColumnInvalid`）、Task 4 的 `CodegenRepository`。
- Produces（Task 6–8 依赖，签名精确）:

```go
package service

// CodegenService 是 admin 业务层入口。
type CodegenService interface {
	ListTables(ctx context.Context, page, size int) ([]*model.CodegenTable, int64, error)
	GetTable(ctx context.Context, id uint64) (*model.CodegenTable, []*model.CodegenColumn, error)
	ImportTable(ctx context.Context, tableName string) (*model.CodegenTable, error)
	UpdateTable(ctx context.Context, t *model.CodegenTable) error
	DeleteTable(ctx context.Context, id uint64) error
	SaveColumns(ctx context.Context, tableID uint64, cols []*model.CodegenColumn) error
	ListColumns(ctx context.Context, tableID uint64) ([]*model.CodegenColumn, error)
	// Sync 从元数据源重读表并计算+落库差异（spec §9.4）。
	Sync(ctx context.Context, tableID uint64) (*model.SyncResult, error)
}

func NewCodegenService(reader metadata.MetadataReader, repo repository.CodegenRepository, opts builder.Options) CodegenService
```

- 其中 `model` = `alexGo-cloud/modules/infra/model`（与仓库同包别名，见实现步骤的 import 约定）；`metadata.MetadataReader` 接口：`ListTables(ctx) ([]string, error)`、`ReadTable(ctx, table string) (*metadata.TableMeta, error)`。
- `opts builder.Options{Module: "infra"}` 只作为模板构建的默认值（M2 导入仅落配置、不生成文件，文件生成在 M3/M4；`opts` 在 Import 时传给 `builder.Build` 以复用主键/租户校验）。

- [ ] **Step 1: 写失败测试**

新建 `modules/infra/service/codegen_service_test.go`。测试用 `fakeReader`（内存表，可 mutate 模拟库表变更）+ Task 4 的 sqlite 仓库：

```go
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	cgmodel "alexGo-cloud/modules/infra/model"
	"alexGo-cloud/modules/infra/repository"
	"alexGo-cloud/pkg/codegen/builder"
	cgmodelErr "alexGo-cloud/pkg/codegen/model"
	"alexGo-cloud/pkg/codegen/metadata"
)

// fakeReader 是内存 metadata.MetadataReader：支持外部 mutate 后 Sync。
type fakeReader struct {
	tables map[string]*metadata.TableMeta
	order  []string
}

func (f *fakeReader) ListTables(_ context.Context) ([]string, error) {
	return append([]string(nil), f.order...), nil
}

func (f *fakeReader) ReadTable(_ context.Context, name string) (*metadata.TableMeta, error) {
	t, ok := f.tables[name]
	if !ok {
		return nil, cgmodelErr.ErrTableNotFound
	}
	// 返回副本，防调用方改内部态
	cp := *t
	cp.Columns = append([]*metadata.ColumnMeta(nil), t.Columns...)
	return &cp, nil
}

func col(name, dtype, key, extra string) *metadata.ColumnMeta {
	return &metadata.ColumnMeta{
		Name: name, DataType: dtype, ColumnType: dtype, Key: key, Extra: extra,
		Ordinal: 0, Nullable: "NO",
	}
}

func newFixtureReader() *fakeReader {
	return &fakeReader{
		tables: map[string]*metadata.TableMeta{
			"order_items": {
				Schema: "alexgo", Name: "order_items", Comment: "订单明细",
				Columns: []*metadata.ColumnMeta{
					col("id", "bigint", "PRI", "auto_increment"),
					col("name", "varchar(64)", "", ""),
					col("qty", "int", "", ""),
				},
			},
		},
		order: []string{"order_items"},
	}
}

func newSvc(t *testing.T) (CodegenService, repository.CodegenRepository, *fakeReader) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&cgmodel.CodegenTable{}, &cgmodel.CodegenColumn{}); err != nil {
		t.Fatal(err)
	}
	repo := repository.NewCodegenRepository(db)
	r := newFixtureReader()
	svc := NewCodegenService(r, repo, builder.Options{Module: "infra"})
	return svc, repo, r
}

// 导入 happy path：快照 3 列、开关默认（list/form 开、query 关——§9.4 新列全关的导入初值见实现）。
func TestImportTable_HappyPath(t *testing.T) {
	svc, _, _ := newSvc(t)
	got, err := svc.ImportTable(context.Background(), "order_items")
	if err != nil {
		t.Fatalf("ImportTable: %v", err)
	}
	if got.TableName != "order_items" || got.ClassName == "" {
		t.Errorf("got %+v", got)
	}
	_, cols, err := svc.GetTable(context.Background(), got.ID)
	if err != nil || len(cols) != 3 {
		t.Fatalf("cols=%d err=%v", len(cols), err)
	}
	if !cols[0].IsPK {
		t.Error("id 应标记 is_pk")
	}
	// 重复导入拒绝
	if _, err := svc.ImportTable(context.Background(), "order_items"); err == nil {
		t.Error("重复导入应报错")
	}
}

// 四类拒绝（Review Focus #3）：每类 errors.Is 精确断言 + 0 新行。
func TestImportTable_Rejections(t *testing.T) {
	cases := []struct {
		name    string
		table   string // reader 中的表名
		mutate  func(*fakeReader)
		wantErr error
	}{
		{
			name:    "库中不存在",
			table:   "ghost_table",
			mutate:  func(r *fakeReader) {},
			wantErr: cgmodelErr.ErrTableNotFound,
		},
		{
			name:  "无主键",
			table: "no_pk",
			mutate: func(r *fakeReader) {
				r.tables["no_pk"] = &metadata.TableMeta{Name: "no_pk", Columns: []*metadata.ColumnMeta{col("a", "int", "", "")}}
				r.order = append(r.order, "no_pk")
			},
			wantErr: cgmodelErr.ErrColumnInvalid,
		},
		{
			name:  "复合主键",
			table: "composite_pk",
			mutate: func(r *fakeReader) {
				r.tables["composite_pk"] = &metadata.TableMeta{Name: "composite_pk", Columns: []*metadata.ColumnMeta{
					col("a", "int", "PRI", ""), col("b", "int", "PRI", ""),
				}}
				r.order = append(r.order, "composite_pk")
			},
			wantErr: cgmodelErr.ErrColumnInvalid,
		},
		{
			name:  "主键非 id",
			table: "uuid_pk",
			mutate: func(r *fakeReader) {
				r.tables["uuid_pk"] = &metadata.TableMeta{Name: "uuid_pk", Columns: []*metadata.ColumnMeta{
					col("order_no", "varchar(32)", "PRI", ""),
				}}
				r.order = append(r.order, "uuid_pk")
			},
			wantErr: cgmodelErr.ErrColumnInvalid,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, repo, reader := newSvc(t)
			tc.mutate(reader)
			_, err := svc.ImportTable(context.Background(), tc.table)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("err = %v, want errors.Is(%v)", err, tc.wantErr)
			}
			total, _, listErr := repo.ListTables(context.Background(), 1, 100)
			if listErr != nil || total != 0 {
				t.Errorf("拒绝后仍有落库: total=%d err=%v", total, listErr)
			}
		})
	}
}

// Sync 前置：未导入的表 → ErrTableNotFound（不静默建行）。
func TestSync_NotImported_Rejected(t *testing.T) {
	svc, _, _ := newSvc(t)
	if _, err := svc.Sync(context.Background(), 4242); !errors.Is(err, cgmodelErr.ErrTableNotFound) {
		t.Errorf("err = %v, want ErrTableNotFound", err)
	}
}
```

注意：`cgmodel` 别名 import 的是 `modules/infra/model`，而哨兵在 `pkg/codegen/model`（别名 `cgmodelErr`）。**两个 model 包同名，测试与实现里必须区分别名**——这是本任务最常见的编译错误，写实现时先想清楚。

- [ ] **Step 2: 运行确认失败**

Run: `go test ./modules/infra/service/ -count=1`
Expected: FAIL（`undefined: NewCodegenService`、`undefined: CodegenService`）

- [ ] **Step 3: 实现**

新建 `modules/infra/service/codegen_service.go`：

```go
package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"alexGo-cloud/modules/infra/model"
	"alexGo-cloud/modules/infra/repository"
	"alexGo-cloud/pkg/codegen/builder"
	cgmodel "alexGo-cloud/pkg/codegen/model"
	"alexGo-cloud/pkg/codegen/metadata"
)

// CodegenService admin 业务层（spec §4）。
type CodegenService interface {
	ListTables(ctx context.Context, page, size int) ([]*model.CodegenTable, int64, error)
	GetTable(ctx context.Context, id uint64) (*model.CodegenTable, []*model.CodegenColumn, error)
	ImportTable(ctx context.Context, tableName string) (*model.CodegenTable, error)
	UpdateTable(ctx context.Context, t *model.CodegenTable) error
	DeleteTable(ctx context.Context, id uint64) error
	SaveColumns(ctx context.Context, tableID uint64, cols []*model.CodegenColumn) error
	ListColumns(ctx context.Context, tableID uint64) ([]*model.CodegenColumn, error)
	Sync(ctx context.Context, tableID uint64) (*model.SyncResult, error)
}

type codegenService struct {
	reader metadata.MetadataReader
	repo   repository.CodegenRepository
	opts   builder.Options
}

func NewCodegenService(reader metadata.MetadataReader, repo repository.CodegenRepository, opts builder.Options) CodegenService {
	return &codegenService{reader: reader, repo: repo, opts: opts}
}

func (s *codegenService) ListTables(ctx context.Context, page, size int) ([]*model.CodegenTable, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 10
	}
	return s.repo.ListTables(ctx, page, size)
}

func (s *codegenService) GetTable(ctx context.Context, id uint64) (*model.CodegenTable, []*model.CodegenColumn, error) {
	t, err := s.repo.GetTable(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	cols, err := s.repo.ListColumns(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return t, cols, nil
}

// ImportTable 元数据 → builder 构建期校验（含 §9 单列 id 主键）→ 落配置。
// 校验失败时 repo 一行不写：Build 在前，Create 在后。
func (s *codegenService) ImportTable(ctx context.Context, tableName string) (*model.CodegenTable, error) {
	if err := validateIdent(tableName); err != nil {
		return nil, err
	}
	// 1) 去重：已导入直接拒绝
	names, err := s.repo.ImportedTableNames(ctx)
	if err != nil {
		return nil, err
	}
	for _, n := range names {
		if n == tableName {
			return nil, fmt.Errorf("table %q already imported", tableName)
		}
	}
	// 2) 读元数据（不存在 → ErrTableNotFound，原样上抛给 400）
	meta, err := s.reader.ReadTable(ctx, tableName)
	if err != nil {
		return nil, err
	}
	// 3) 构建期校验：无列/主键规则 → ErrColumnInvalid 系
	tbl, err := builder.Build(meta, s.opts)
	if err != nil {
		return nil, err
	}
	// 4) 快照落库；导入初值 = 字段位置即排序、开关全开（list/form）、query 默认关
	t := &model.CodegenTable{
		TableName:    meta.Name,
		TableComment: meta.Comment,
		Module:       s.opts.Module,
		BusinessName: lowerFirst(tbl.Entity),
		ClassName:    tbl.Entity,
		TemplateType: int(s.opts.TemplateType),
		FrontType:    int(s.opts.FrontType),
	}
	cols := make([]*model.CodegenColumn, 0, len(tbl.Columns))
	for i, c := range tbl.Columns {
		cols = append(cols, &model.CodegenColumn{
			TableID:       t.ID,
			Name:          c.Name,
			Type:          c.DBType,
			Comment:       c.Comment,
			GoType:        c.GoType,
			JSONName:      c.JSONName,
			IsPK:          c.IsPK,
			AutoIncrement: c.AutoIncrement,
			Nullable:      c.Nullable,
			HTMLType:      defaultHTMLType(c),
			ListEnable:    !c.IsPK && !c.AutoIncrement,
			FormEnable:    !c.IsPK && !c.AutoIncrement,
			QueryEnable:   false, // 新列/导入默认 query 关（§9.4 风格：需显式打开）
			QueryOperation: "eq",
			SortOrder:     i,
		})
	}
	if err := s.repo.CreateTable(ctx, t, cols); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *codegenService) UpdateTable(ctx context.Context, t *model.CodegenTable) error {
	_, err := s.repo.GetTable(ctx, t.ID)
	if err != nil {
		return err
	}
	return s.repo.UpdateTable(ctx, t)
}

func (s *codegenService) DeleteTable(ctx context.Context, id uint64) error {
	return s.repo.DeleteTable(ctx, id)
}

func (s *codegenService) SaveColumns(ctx context.Context, tableID uint64, cols []*model.CodegenColumn) error {
	if len(cols) == 0 {
		return errors.New("no columns to save")
	}
	return s.repo.SaveColumns(ctx, tableID, cols)
}

func (s *codegenService) ListColumns(ctx context.Context, tableID uint64) ([]*model.CodegenColumn, error) {
	return s.repo.ListColumns(ctx, tableID)
}

var identRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// validateIdent 表名只允许小写 snake——它是拼进 SQL/文件名的输入面。
func validateIdent(name string) error {
	if !identRe.MatchString(name) || len(name) > 128 {
		return fmt.Errorf("invalid table name %q", name)
	}
	return nil
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]|0x20) + s[1:]
}

func defaultHTMLType(c *model.CodegenColumn) string {
	// 主键/自增不进表单；类型粗映射，前端可再改（配置字段，同步不覆盖）
	switch {
	case c.IsPK:
		return "Input"
	case c.GoType == "bool":
		return "Switch"
	case c.GoType == "time.Time":
		return "DatePicker"
	case c.GoType == "int" || c.GoType == "int64" || c.GoType == "uint64" || c.GoType == "float64":
		return "InputNumber"
	default:
		return "Input"
	}
}
```

**注意两处以实现为准的签名核对**（执行者先打开源文件核对，不匹配则以源文件为准修本任务代码，测试断言不变）：

1. `builder.Build` 返回的 `*model.Table`（`pkg/codegen/model`）字段名——`Entity`/`Columns[].Name/DBType/GoType/JSONName/IsPK/AutoIncrement/Nullable/Comment` 按 `pkg/codegen/model/table.go` 实际字段核对；
2. `metadata.ColumnMeta` 无 `DBType`/`GoType` 概念——`builder.Build` 的输出 `model.Column` 才有。若 `Build` 输出的列结构与上述不符，先 `go doc alexGo-cloud/pkg/codegen/model.Table` 确认后改 import 落库映射。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./modules/infra/service/ -count=1 -v`
Expected: PASS（happy path + 4 拒绝子测试 + Sync 前置）

- [ ] **Step 5: Commit**

```bash
git add modules/infra/service/
git commit -m "feat(infra): 导入服务——元数据构建期校验与四类拒绝"
```

---

### Task 6: 同步服务——diff 分类 + 事务落库（§9.4 三钉子）

**Files:**
- Modify: `modules/infra/service/codegen_service.go`（实现 `Sync`）
- Test: `modules/infra/service/sync_test.go`

**Interfaces:**
- Consumes: Task 4 `repository.ApplySync(ctx, tableID, tableComment, diff)` 与 `model.SyncDiff/SyncResult`；Task 5 `fakeReader`（同包，直接复用）；`pkg/codegen/builder`（元数据→`model.Table` 规范化，复用 Task 1/2 后的规则）。
- Produces（Task 8 消费）:
  - `svc.Sync(ctx, tableID) (*model.SyncResult, error)`——语义（spec §9.4）：
    - 元数据新列 → `diff.Added`（`ListEnable=false, FormEnable=false, QueryEnable=false`，`SortOrder` = 现有最大 +1 起排）
    - 快照字段（`type/comment/go_type/json_name/is_pk/auto_increment/nullable`）变化 → `diff.Refresh`（配置字段不动）
    - 库中消失列 → `diff.Deprecated = [行id]`（**只标 `deprecated=1`，不物理删**，Review Focus #2）
    - `deprecated=1` 且重新出现 → `Refresh` 且 `Deprecated=false`（恢复），计入 `Updated`
    - 未变化 → 计入 `Unchanged`
    - 表注释 → `ApplySync` 一并刷新
  - 返回 `*model.SyncResult{Added, Updated, Deprecated, Unchanged}`

- [ ] **Step 1: 写失败测试**

新建 `modules/infra/service/sync_test.go`（复用 Task 5 的 `newSvc`/`newFixtureReader`/`col`）：

```go
package service

import (
	"context"
	"testing"

	"alexGo-cloud/modules/infra/model"
)

// 钉子①（Review Focus #2）：同步绝不覆盖配置字段——list/query 开关保持原值。
func TestSync_ConfigSwitchesSurvive(t *testing.T) {
	svc, _, reader := newSvc(t)

	// 导入后把 name 列的配置改掉（模拟用户编辑）
	tbl, err := svc.ImportTable(context.Background(), "order_items")
	if err != nil {
		t.Fatal(err)
	}
	_, cols, _ := svc.GetTable(context.Background(), tbl.ID)
	nameCol := cols[1]
	nameCol.ListEnable = false
	nameCol.QueryEnable = true
	nameCol.QueryOperation = "like"
	nameCol.HTMLType = "Select"
	if err := svc.SaveColumns(context.Background(), tbl.ID, []*model.CodegenColumn{nameCol}); err != nil {
		t.Fatal(err)
	}

	// 库表没变：只改注释逼出 Refresh
	meta := reader.tables["order_items"]
	meta.Columns[1].Comment = "改了注释"
	meta.Comment = "订单明细v2"

	res, err := svc.Sync(context.Background(), tbl.ID)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Updated != 1 || res.Added != 0 || res.Deprecated != 0 || res.Unchanged != 2 {
		t.Errorf("res = %+v", res)
	}

	_, cols2, _ := svc.GetTable(context.Background(), tbl.ID)
	var after model.CodegenColumn
	for _, c := range cols2 {
		if c.Name == "name" {
			after = *c
		}
	}
	if after.ListEnable || !after.QueryEnable || after.QueryOperation != "like" || after.HTMLType != "Select" {
		t.Errorf("配置字段被同步覆盖: %+v", after)
	}
	if after.Comment != "改了注释" {
		t.Errorf("快照未刷新: comment=%q", after.Comment)
	}
}

// 钉子②：消失列只标 deprecated，行仍在（不物理删）。
func TestSync_MissingColumn_MarkedDeprecatedNotDeleted(t *testing.T) {
	svc, repo, reader := newSvc(t)
	tbl, err := svc.ImportTable(context.Background(), "order_items")
	if err != nil {
		t.Fatal(err)
	}

	// 库表删掉 qty 列
	m := reader.tables["order_items"]
	m.Columns = m.Columns[:2] // id, name
	reader.tables["order_items"] = m

	res, err := svc.Sync(context.Background(), tbl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res.Deprecated != 1 || res.Updated != 0 || res.Added != 0 {
		t.Errorf("res = %+v", res)
	}
	_, cols, _ := svc.GetTable(context.Background(), tbl.ID)
	if len(cols) != 3 {
		t.Fatalf("行被物理删: cols=%d, want 3", len(cols))
	}
	var found bool
	for _, c := range cols {
		if c.Name == "qty" {
			found = true
			if !c.Deprecated {
				t.Error("qty 未标 deprecated")
			}
		}
	}
	if !found {
		t.Fatal("qty 行消失")
	}
}

// 钉子③：deprecated 列重新出现 → 恢复 + 计入 Updated，配置字段保留。
func TestSync_Reappear_RestoresDeprecated(t *testing.T) {
	svc, repo, reader := newSvc(t)
	tbl, _ := svc.ImportTable(context.Background(), "order_items")
	_, cols, _ := svc.GetTable(context.Background(), tbl.ID)
	qty := cols[2]
	qty.QueryEnable = true
	qty.QueryOperation = "gt"
	if err := svc.SaveColumns(context.Background(), tbl.ID, []*model.CodegenColumn{qty}); err != nil {
		t.Fatal(err)
	}

	// 删列 → 同步 → 恢复列 → 同步
	m := reader.tables["order_items"]
	m.Columns = m.Columns[:2]
	if _, err := svc.Sync(context.Background(), tbl.ID); err != nil {
		t.Fatal(err)
	}
	m.Columns = append(m.Columns, col("qty", "int", "", ""))
	reader.tables["order_items"] = m

	res, err := svc.Sync(context.Background(), tbl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res.Updated != 1 || res.Deprecated != 0 {
		t.Errorf("res = %+v", res)
	}
	_, cols2, _ := svc.GetTable(context.Background(), tbl.ID)
	for _, c := range cols2 {
		if c.Name == "qty" {
			if c.Deprecated {
				t.Error("恢复后仍 deprecated")
			}
			if !c.QueryEnable || c.QueryOperation != "gt" {
				t.Errorf("恢复时配置丢失: %+v", c)
			}
		}
	}
}

// 新列：开关全关 + sort_order 接在末尾（§9.4）。
func TestSync_NewColumn_AllSwitchesOff(t *testing.T) {
	svc, _, reader := newSvc(t)
	tbl, _ := svc.ImportTable(context.Background(), "order_items")
	m := reader.tables["order_items"]
	m.Columns = append(m.Columns, col("remark", "varchar(255)", "", ""))
	reader.tables["order_items"] = m

	res, err := svc.Sync(context.Background(), tbl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res.Added != 1 {
		t.Fatalf("res = %+v", res)
	}
	_, cols, _ := svc.GetTable(context.Background(), tbl.ID)
	var remark *model.CodegenColumn
	for _, c := range cols {
		if c.Name == "remark" {
			remark = c
		}
	}
	if remark == nil {
		t.Fatal("remark 未落库")
	}
	if remark.ListEnable || remark.FormEnable || remark.QueryEnable {
		t.Errorf("新列开关应全关: %+v", remark)
	}
	if remark.SortOrder != 3 {
		t.Errorf("sort_order = %d, want 3（既有最大 2 + 1）", remark.SortOrder)
	}
}
```

四个测试全部复用 Task 5 的 `newSvc(t)`（同包测试辅助函数），返回的 `reader` 可直接 mutate 模拟库表变更。

- [ ] **Step 2: 运行确认失败**

Run: `go test ./modules/infra/service/ -run Sync -count=1`
Expected: FAIL（`svc.Sync undefined`——`Sync` 方法尚未实现 / stub 返回 nil）

- [ ] **Step 3: 实现 Sync**

在 `codegen_service.go` 追加（`Sync` 已在接口里声明）：

```go
// Sync 重读元数据并按 spec §9.4 分类：新列全关插入、消失列标 deprecated、
// 快照刷新不碰配置、重现恢复。整个落库走 repo.ApplySync 单事务。
func (s *codegenService) Sync(ctx context.Context, tableID uint64) (*model.SyncResult, error) {
	t, err := s.repo.GetTable(ctx, tableID)
	if err != nil {
		return nil, err
	}
	meta, err := s.reader.ReadTable(ctx, t.TableName)
	if err != nil {
		return nil, err
	}
	tbl, err := builder.Build(meta, s.opts)
	if err != nil {
		return nil, err
	}
	existing, err := s.repo.ListColumns(ctx, tableID)
	if err != nil {
		return nil, err
	}
	byName := make(map[string]*model.CodegenColumn, len(existing))
	for _, c := range existing {
		byName[c.Name] = c
	}

	diff := model.SyncDiff{}
	res := &model.SyncResult{}
	maxSort := -1
	for _, c := range existing {
		if c.SortOrder > maxSort {
			maxSort = c.SortOrder
		}
	}

	seen := map[string]bool{}
	for _, gc := range tbl.Columns {
		seen[gc.Name] = true
		cur, ok := byName[gc.Name]
		if !ok {
			// 新列：开关全关，sort 接末尾
			maxSort++
			diff.Added = append(diff.Added, &model.CodegenColumn{
				Name: gc.Name, Type: gc.DBType, Comment: gc.Comment,
				GoType: gc.GoType, JSONName: gc.JSONName,
				IsPK: gc.IsPK, AutoIncrement: gc.AutoIncrement, Nullable: gc.Nullable,
				HTMLType: "", ListEnable: false, FormEnable: false, QueryEnable: false,
				QueryOperation: "eq", SortOrder: maxSort, Deprecated: false,
			})
			res.Added++
			continue
		}
		// 快照对比：命中差异或从 deprecated 复活 → Refresh
		changed := cur.Type != gc.DBType || cur.Comment != gc.Comment ||
			cur.GoType != gc.GoType || cur.JSONName != gc.JSONName ||
			cur.IsPK != gc.IsPK || cur.AutoIncrement != gc.AutoIncrement ||
			cur.Nullable != gc.Nullable
		if changed || cur.Deprecated {
			refresh := *cur
			refresh.Type = gc.DBType
			refresh.Comment = gc.Comment
			refresh.GoType = gc.GoType
			refresh.JSONName = gc.JSONName
			refresh.IsPK = gc.IsPK
			refresh.AutoIncrement = gc.AutoIncrement
			refresh.Nullable = gc.Nullable
			refresh.Deprecated = false
			diff.Refresh = append(diff.Refresh, &refresh)
			res.Updated++
		} else {
			res.Unchanged++
		}
	}
	for _, c := range existing {
		if !seen[c.Name] {
			diff.Deprecated = append(diff.Deprecated, c.ID)
			res.Deprecated++
		}
	}

	if len(diff.Added) > 0 || len(diff.Refresh) > 0 || len(diff.Deprecated) > 0 ||
		t.TableComment != meta.Comment {
		if err := s.repo.ApplySync(ctx, tableID, meta.Comment, diff); err != nil {
			return nil, err
		}
	} else if t.TableComment != meta.Comment {
		if err := s.repo.ApplySync(ctx, tableID, meta.Comment, diff); err != nil {
			return nil, err
		}
	}
	return res, nil
}
```

**执行者核对**：`model.Table.Columns[].DBType` 字段名以 `pkg/codegen/model/table.go` 为准（若为 `Type`/`DBTypeName` 则同步改本文件与 Task 5 的映射，测试断言不变）。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./modules/infra/service/ -count=1 -v`
Expected: PASS（Task 5 的 6 例 + Task 6 的 4 例全绿）

- [ ] **Step 5: Commit**

```bash
git add modules/infra/service/
git commit -m "feat(infra): 同步 diff 分类——配置保留/硬删豁免/重现恢复（spec §9.4）"
```

---

### Task 7: Admin Controller——9 个端点 + fx 全量装配

**Files:**
- Create: `modules/infra/controller/admin/codegen_controller.go`
- Test: `modules/infra/controller/admin/codegen_controller_test.go`
- Modify: `modules/infra/module.go`（追加三层 Provider + RegisterRoutes）

**Interfaces:**
- Consumes: Task 5/6 的 `service.CodegenService`；member 模块的 controller 测试模式（`gin.SetMode(gin.TestMode)` + `gin.New()` 手动挂路由 + `httptest`）；server `Module` 接口（`RegisterRoutes(rg *gin.RouterGroup)` 挂在 `/api` 组下）。
- Produces（Task 8 与前端消费）:
  - 路由前缀 `/admin/infra/codegen`（相对 `/api`，完整 `/api/admin/infra/codegen`）
  - 9 个端点（spec §4，preview/download 属 M3 不在本任务）:

| Method | Path（相对前缀） | 行为 | 成功信封 | 失败 |
|---|---|---|---|---|
| GET | `/tables?page=&size=` | 分页列表 | `{"data":[...],"total":N}` | — |
| GET | `/tables/:id` | 表+字段详情 | `{"data":{table..., "columns":[...]}}` | 400 |
| POST | `/import` body `{"table_name":"..."}` | 导入 | `{"data":{table}}` | 400/500 |
| PUT | `/tables/:id` body table | 改表配置 | `{"status":"ok"}` | 400 |
| DELETE | `/tables/:id` | 硬删+级联 | `{"status":"ok"}` | 400 |
| GET | `/tables/:id/columns` | 字段列表 | `{"data":[...]}` | 400 |
| PUT | `/tables/:id/columns` body `[...]` | 批量存配置 | `{"status":"ok"}` | 400 |
| POST | `/tables/:id/sync` | 同步 | `{"data":{added,updated,deprecated,unchanged}}` | 400/500 |
| GET | `/tables/:id/generate` | **占位→500 `"M3 feature"`** | — | 500 |

  - 错误信封统一 `gin.H{"error": msg}`；**状态码只用 400（校验/不存在/业务拒绝）与 500（读库失败）——仓库无 404 惯例**
  - `newMetadataReader(lc fx.Lifecycle, cfg *config.Config) (metadata.MetadataReader, error)`——fx Provider，M2 用 MySQL DSN 构造 `metadata.MetadataReader`（打开连接、`lc.Append` 关闭）
  - `infra.FxModule` 完整形态：`fx.Provide(NewCodegenRepository, newMetadataReader, NewCodegenService, NewCodegenController)` + `fx.Annotate(NewModule, fx.As(new(server.Module)), fx.ResultTags(`group:"modules"`))`

- [ ] **Step 1: 写失败测试**

新建 `modules/infra/controller/admin/codegen_controller_test.go`（stub 直接实现接口，参考 `modules/member/controller/admin/member_user_test.go`）：

```go
package admin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/infra/model"
	"alexGo-cloud/pkg/codegen/builder"
	cgmodel "alexGo-cloud/pkg/codegen/model"
)

// stubService 记录调用，按 case 返回。
type stubService struct {
	importErr error
	syncRes   *model.SyncResult
	syncErr   error
	gotImport string
	saved     []*model.CodegenColumn
}

func (s *stubService) ListTables(context.Context, int, int) ([]*model.CodegenTable, int64, error) {
	return []*model.CodegenTable{{ID: 1, TableName: "order_items", ClassName: "OrderItem"}}, 1, nil
}
func (s *stubService) GetTable(_ context.Context, id uint64) (*model.CodegenTable, []*model.CodegenColumn, error) {
	if id != 1 {
		return nil, nil, errors.New("record not found")
	}
	return &model.CodegenTable{ID: 1, TableName: "order_items"},
		[]*model.CodegenColumn{{ID: 10, TableID: 1, Name: "id", IsPK: true}}, nil
}
func (s *stubService) ImportTable(_ context.Context, name string) (*model.CodegenTable, error) {
	s.gotImport = name
	if s.importErr != nil {
		return nil, s.importErr
	}
	return &model.CodegenTable{ID: 1, TableName: name}, nil
}
func (s *stubService) UpdateTable(context.Context, *model.CodegenTable) error { return nil }
func (s *stubService) DeleteTable(context.Context, uint64) error             { return nil }
func (s *stubService) SaveColumns(_ context.Context, _ uint64, cols []*model.CodegenColumn) error {
	s.saved = cols
	return nil
}
func (s *stubService) ListColumns(_ context.Context, _ uint64) ([]*model.CodegenColumn, error) {
	return []*model.CodegenColumn{{ID: 10, TableID: 1, Name: "id"}}, nil
}
func (s *stubService) Sync(context.Context, uint64) (*model.SyncResult, error) {
	if s.syncErr != nil {
		return nil, s.syncErr
	}
	return s.syncRes, nil
}

func newTestRouter(stub *stubService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	c := NewCodegenController(stub, builder.Options{Module: "infra"})
	g := r.Group("/api/admin/infra/codegen")
	c.RegisterRoutes(g)
	return r
}

func doJSON(t *testing.T, r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestListTables_Envelope(t *testing.T) {
	r := newTestRouter(&stubService{})
	w := doJSON(t, r, "GET", "/api/admin/infra/codegen/tables?page=1&size=10", "")
	if w.Code != 200 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"total":1`) || !strings.Contains(body, "order_items") {
		t.Errorf("body=%s", body)
	}
}

// 信封铁律：列表 data+total；详情 data；变更 status ok；错误 error 字段。
func TestEnvelopes_AllShapes(t *testing.T) {
	r := newTestRouter(&stubService{})

	w := doJSON(t, r, "GET", "/api/admin/infra/codegen/tables/1", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"data"`) {
		t.Errorf("detail: %d %s", w.Code, w.Body)
	}

	w = doJSON(t, r, "POST", "/api/admin/infra/codegen/import", `{"table_name":"order_items"}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"data"`) {
		t.Errorf("import: %d %s", w.Code, w.Body)
	}

	w = doJSON(t, r, "DELETE", "/api/admin/infra/codegen/tables/1", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"ok"`) {
		t.Errorf("delete: %d %s", w.Code, w.Body)
	}

	w = doJSON(t, r, "POST", "/api/admin/infra/codegen/tables/1/sync", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"added"`) {
		t.Errorf("sync: %d %s", w.Code, w.Body)
	}
}

// 业务拒绝/不存在 → 400 + error；绝不 404（仓库无此惯例）。
func TestErrors_400Never404(t *testing.T) {
	r := newTestRouter(&stubService{importErr: cgmodel.ErrTableNotFound})
	w := doJSON(t, r, "POST", "/api/admin/infra/codegen/import", `{"table_name":"ghost"}`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("import notfound: code=%d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"error"`) {
		t.Errorf("body=%s", w.Body)
	}

	r2 := newTestRouter(&stubService{})
	w2 := doJSON(t2, r2, "GET", "/api/admin/infra/codegen/tables/999", "")
	if w2.Code != http.StatusBadRequest {
		t.Errorf("detail missing: code=%d, want 400", w2.Code)
	}
}

// 空 table_name → 400，且不触达 service。
func TestImport_ValidationBeforeService(t *testing.T) {
	stub := &stubService{}
	r := newTestRouter(stub)
	w := doJSON(t, r, "POST", "/api/admin/infra/codegen/import", `{"table_name":""}`)
	if w.Code != 400 {
		t.Errorf("code=%d", w.Code)
	}
	if stub.gotImport != "" {
		t.Error("空表名不应调用 service")
	}
}

// generate 端点是 M3 占位：返回 500 + 明确文案，不 panic。
func TestGenerate_ReservedForM3(t *testing.T) {
	r := newTestRouter(&stubService{})
	w := doJSON(t, r, "GET", "/api/admin/infra/codegen/tables/1/generate", "")
	if w.Code != 500 || !strings.Contains(w.Body.String(), "M3") {
		t.Errorf("code=%d body=%s", w.Code, w.Body)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./modules/infra/controller/admin/ -count=1`
Expected: FAIL（`undefined: NewCodegenController`）

- [ ] **Step 3: 实现 Controller**

新建 `modules/infra/controller/admin/codegen_controller.go`：

```go
package admin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/infra/model"
	"alexGo-cloud/modules/infra/service"
	"alexGo-cloud/pkg/codegen/builder"
)

// CodegenController codegen 配置 admin 接口（spec §4）。
// 信封：列表 {data,total} / 详情 {data} / 变更 {status:"ok"} / 错误 {error}。
// 状态码只用 400 与 500——仓库无 404 惯例。
type CodegenController struct {
	svc  service.CodegenService
	opts builder.Options
}

func NewCodegenController(svc service.CodegenService, opts builder.Options) *CodegenController {
	return &CodegenController{svc: svc, opts: opts}
}

// RegisterRoutes 挂在 server /api 组下，前缀 /admin/infra/codegen。
func (c *CodegenController) RegisterRoutes(g *gin.RouterGroup) {
	g.GET("/tables", c.listTables)
	g.POST("/import", c.importTable)
	g.GET("/tables/:id", c.getTable)
	g.PUT("/tables/:id", c.updateTable)
	g.DELETE("/tables/:id", c.deleteTable)
	g.GET("/tables/:id/columns", c.listColumns)
	g.PUT("/tables/:id/columns", c.saveColumns)
	g.POST("/tables/:id/sync", c.sync)
	g.GET("/tables/:id/generate", c.generate) // M3 占位
}

func idParam(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return 0, false
	}
	return id, true
}

// fail 统一错误出口：读库类 → 500；校验/不存在/业务拒绝 → 400。
func fail(c *gin.Context, status int, err error) {
	c.JSON(status, gin.H{"error": err.Error()})
}

func (c *CodegenController) listTables(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(ctx.DefaultQuery("size", "10"))
	items, total, err := c.svc.ListTables(ctx.Request.Context(), page, size)
	if err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": items, "total": total})
}

func (c *CodegenController) getTable(ctx *gin.Context) {
	id, ok := idParam(ctx)
	if !ok {
		return
	}
	t, cols, err := c.svc.GetTable(ctx.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusBadRequest, err) // 不存在 = 400
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": t, "columns": cols})
}

func (c *CodegenController) importTable(ctx *gin.Context) {
	var req struct {
		TableName string `json:"table_name" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	t, err := c.svc.ImportTable(ctx.Request.Context(), req.TableName)
	if err != nil {
		// 哨兵（不存在/主键规则）与重复导入 → 400；其余读库失败 → 500
		status := http.StatusBadRequest
		fail(c, status, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": t})
}

func (c *CodegenController) updateTable(ctx *gin.Context) {
	id, ok := idParam(ctx)
	if !ok {
		return
	}
	var t model.CodegenTable
	if err := ctx.ShouldBindJSON(&t); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	t.ID = id // 路径 id 压过 body
	if err := c.svc.UpdateTable(ctx.Request.Context(), &t); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (c *CodegenController) deleteTable(ctx *gin.Context) {
	id, ok := idParam(ctx)
	if !ok {
		return
	}
	if err := c.svc.DeleteTable(ctx.Request.Context(), id); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (c *CodegenController) listColumns(ctx *gin.Context) {
	id, ok := idParam(ctx)
	if !ok {
		return
	}
	cols, err := c.svc.ListColumns(ctx.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": cols})
}

func (c *CodegenController) saveColumns(ctx *gin.Context) {
	id, ok := idParam(ctx)
	if !ok {
		return
	}
	var cols []*model.CodegenColumn
	if err := ctx.ShouldBindJSON(&cols); err != nil || len(cols) == 0 {
		fail(c, http.StatusBadRequest, errOr(err, "empty columns"))
		return
	}
	if err := c.svc.SaveColumns(ctx.Request.Context(), id, cols); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (c *CodegenController) sync(ctx *gin.Context) {
	id, ok := idParam(ctx)
	if !ok {
		return
	}
	res, err := c.svc.Sync(ctx.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": res})
}

// generate 是 M3 能力（模板渲染+下载），M2 显式占位防误调。
func (c *CodegenController) generate(ctx *gin.Context) {
	fail(c, http.StatusInternalServerError, errM3Feature)
}

type featureNotReadyError struct{ msg string }

func (e *featureNotReadyError) Error() string {
	if e.msg != "" {
		return e.msg
	}
	return "generate is an M3 feature"
}

var errM3Feature = &featureNotReadyError{}

func errOr(err error, fallback string) error {
	if err != nil {
		return err
	}
	return &featureNotReadyError{msg: fallback}
}
```

- [ ] **Step 4: 运行 controller 测试**

Run: `go test ./modules/infra/controller/admin/ -count=1 -v`
Expected: PASS（5 个测试全绿）

- [ ] **Step 5: fx 全量装配**

`modules/infra/module.go` 替换为完整形态：

```go
package infra

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"alexGo-cloud/modules/infra/controller/admin"
	"alexGo-cloud/modules/infra/repository"
	"alexGo-cloud/modules/infra/service"
	"alexGo-cloud/pkg/codegen/builder"
	"alexGo-cloud/pkg/codegen/metadata"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/migrate"
	"alexGo-cloud/pkg/server"
)

// FxModule infra 模块装配：迁移源 + 三层 + admin 路由。系统级：不做租户隔离。
var FxModule = fx.Module("infra",
	fx.Provide(
		fx.Annotate(
			NewMigrationSource,
			fx.As(new(migrate.Source)),
			fx.ResultTags(`group:"migration_sources"`),
		),
		repository.NewCodegenRepository,
		newMetadataReader,
		newCodegenService,
		admin.NewCodegenController,
		fx.Annotate(
			NewModule,
			fx.As(new(server.Module)),
			fx.ResultTags(`group:"modules"`),
		),
	),
)

// newCodegenService 注入默认构建选项（Module=infra）。
func newCodegenService(reader metadata.MetadataReader, repo repository.CodegenRepository) service.CodegenService {
	return service.NewCodegenService(reader, repo, builder.Options{
		Module: "infra", TemplateType: builder.TemplateSingle, FrontType: builder.FrontVben5Antd,
	})
}
```

（`builder.TemplateSingle`/`builder.FrontVben5Antd` 常量名以 `pkg/codegen/builder` 实际定义为准，若为枚举值 `1`/`1` 直接写数字并加注释。）

`newMetadataReader`（同文件）——**先执行 Step 5a 核对元数据读取构造方式**：

- [ ] **Step 5a: 核对 `metadata.MetadataReader` 的 MySQL 实现**

Run: `grep -rn "MetadataReader\|func New.*Reader" pkg/codegen/metadata/ --include=*.go | grep -v _test`
Expected: 找到构造函数（如 `metadata.NewMySQLReader(dsn)` 或已有实现）。

按结果二选一：
- **已有 MySQL 构造器** → 直接接线：

```go
func newMetadataReader(lc fx.Lifecycle, cfg *config.Config) (metadata.MetadataReader, error) {
	r, err := metadata.NewMySQLReader(cfg.Database.DSN) // 换成实际签名
	if err != nil {
		return nil, err
	}
	lc.Append(fx.Hook{OnStop: func(context.Context) error { return r.Close() }})
	return r, nil
}
```

- **只有接口、无实现** → 新建 `pkg/codegen/metadata/mysql_reader.go`（实现 `ListTables`/`ReadTable`，用 `information_schema` 查询，与 Task 3 测试同 DSN），并在本任务带上最小实现 + `ListTables`/`ReadTable` 各一个 sqlite 不可用时 skip 的单元测试。**把实际选择与签名写进 commit message。**

- [ ] **Step 6: fx 干跑 + 全量测试**

Run: `CGO_ENABLED=0 go test ./alexgo-server/cmd/ -count=1 && go test ./modules/infra/... -count=1`
Expected: PASS（fx 图有效；infra 全包绿）

- [ ] **Step 7: Commit**

```bash
git add modules/infra/ pkg/codegen/metadata/
git commit -m "feat(infra): admin 9 端点 + fx 全量装配 + 元数据读取器接线"
```

---

### Task 8: 端到端验收——导入→改配置→同步 diff（spec §8 M2 验收）

**Files:**
- Test: `modules/infra/service/acceptance_test.go`

**Interfaces:**
- Consumes: Task 5–7 的 `NewCodegenService` + `fakeReader` + sqlite 仓库；controller 层信封在 Task 7 已测，本任务专注**业务链路**：导入 → PUT 字段配置 → mutate 元数据 → Sync → diff 计数正确。
- Produces: spec §8 M2 验收证据（"HTTP 测试：导入→改配置→同步 diff 通过"的 service 等价链；HTTP 层信封由 Task 7 覆盖，两层合起来即验收）。

- [ ] **Step 1: 写验收测试**

新建 `modules/infra/service/acceptance_test.go`：

```go
package service

import (
	"context"
	"testing"
)

// TestM2_Acceptance_ImportEditSync 是 spec §8 M2 验收链：
// 导入 → 改配置（query 开关+操作符）→ 库表加列删列改注释 → 同步 → diff 计数。
func TestM2_Acceptance_ImportEditSync(t *testing.T) {
	svc, _, reader := newSvc(t)
	ctx := context.Background()

	// 1) 导入
	tbl, err := svc.ImportTable(ctx, "order_items")
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	_, cols, _ := svc.GetTable(ctx, tbl.ID)
	if len(cols) != 3 {
		t.Fatalf("import cols=%d, want 3", len(cols))
	}

	// 2) 改配置：name 列 query 开、between
	var nameID uint64
	for _, c := range cols {
		if c.Name == "name" {
			nameID = c.ID
		}
	}
	if err := svc.SaveColumns(ctx, tbl.ID, []*model.CodegenColumn{{
		ID: nameID, TableID: tbl.ID, Name: "name",
		QueryEnable: true, QueryOperation: "between", HTMLType: "Select",
	}}); err != nil {
		t.Fatalf("save config: %v", err)
	}

	// 3) 库表变更：+remark 列、删 qty 列、改表注释
	m := reader.tables["order_items"]
	m.Columns = append(m.Columns[:2], col("remark", "varchar(255)", "", "")) // [id,name,remark]
	m.Comment = "订单明细v2"
	reader.tables["order_items"] = m

	// 4) 同步 → diff：Added=1(remark), Deprecated=1(qty), Updated>=1(name 快照可能不变→0),
	//    Unchanged=1(id)
	res, err := svc.Sync(ctx, tbl.ID)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if res.Added != 1 {
		t.Errorf("added=%d, want 1", res.Added)
	}
	if res.Deprecated != 1 {
		t.Errorf("deprecated=%d, want 1", res.Deprecated)
	}
	if res.Unchanged != 1 {
		t.Errorf("unchanged=%d, want 1 (id)", res.Unchanged)
	}

	// 5) 配置字段跨同步存活 + qty 行仍在（deprecated）+ 表注释刷新
	t2, cols2, err := svc.GetTable(ctx, tbl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if t2.TableComment != "订单明细v2" {
		t.Errorf("table_comment=%q", t2.TableComment)
	}
	if len(cols2) != 4 {
		t.Fatalf("cols=%d, want 4 (id,name,qty,remark)", len(cols2))
	}
	for _, c := range cols2 {
		switch c.Name {
		case "name":
			if !c.QueryEnable || c.QueryOperation != "between" || c.HTMLType != "Select" {
				t.Errorf("配置丢失: %+v", c)
			}
		case "qty":
			if !c.Deprecated {
				t.Error("qty 应 deprecated")
			}
		case "remark":
			if c.ListEnable || c.FormEnable || c.QueryEnable {
				t.Errorf("新列开关应全关: %+v", c)
			}
		}
	}

	// 6) 再同步一次：全 unchanged、计数归零（幂等）
	res2, err := svc.Sync(ctx, tbl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res2.Added != 0 || res2.Updated != 0 || res2.Deprecated != 0 {
		t.Errorf("二次同步非幂等: %+v", res2)
	}
}
```

（`model` 包 import 用 `cgmodel "alexGo-cloud/modules/infra/model"` 别名或沿用本文件 `model`——与同包其他测试文件保持一致即可。）

- [ ] **Step 2: 运行确认**

Run: `go test ./modules/infra/service/ -run M2_Acceptance -count=1 -v`
Expected: PASS（若 step 1 的计数与实际分类逻辑有出入，**先修测试预期对齐 §9.4 语义，再修实现**——不允许为过测试改语义）

- [ ] **Step 3: 全量回归**

Run: `go test ./pkg/codegen/... ./modules/infra/... -count=1 && CGO_ENABLED=0 go test ./alexgo-server/cmd/ -count=1`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add modules/infra/service/acceptance_test.go
git commit -m "test(infra): M2 验收链——导入→改配置→同步 diff（spec §8）"
```

---

### Task 9: 菜单种子迁移 + permissionRoutes 注册（403 修复闭环）

**Files:**
- Create: `modules/system/migrations/20261011000002_codegen_menus.up.sql`
- Create: `modules/system/migrations/20261011000002_codegen_menus.down.sql`
- Create: `modules/system/service/permission_routes_test.go`
- Modify: `modules/system/service/permission_routes.go`（`permissionRoutes` 追加 `infra:codegen` 条目）
- Modify: `modules/infra/migrations/migrate_test.go`（追加 seed 双跑断言）

**Interfaces:**
- Consumes: 菜单种子幂等惯例（`modules/system/migrations/20261009000001_menus_vben_backfill.up.sql` 的 `INSERT ... SELECT ... FROM DUAL WHERE NOT EXISTS` 模式）；`permissionRoutes` map + `permPrefix()`（取前两段 `infra:codegen`）；seed 测试模式 `modules/system/service/seed_test.go::TestSeedButtonPerms_MatchRouteKeys`。
- Produces:
  - 权限前缀 `infra:codegen` → 路由 `/api/admin/infra/codegen` + `/api/admin/infra/codegen/*`（**两个都要**——keyMatch2 对集合与子路径分别匹配，Review Focus #4）
  - 菜单：目录 `代码生成`（parent = 系统管理目录，用 `WHERE parent_id = (SELECT id FROM ... WHERE name='系统管理')` 关联，**不写死 id**）+ 页面菜单 `代码生成配置` + 按钮 `导入`/`同步`/`删除`（权限码 `infra:codegen:import` 等，全部 `NOT EXISTS` 幂等）
  - `permissionRoutes` 单测：种子 SQL 里出现的每个权限码前缀都必须在 `permissionRoutes` 有键（防种子与路由注册漂移——鸡生蛋问题的守护测试）

- [ ] **Step 1: 写失败测试**

新建 `modules/system/service/permission_routes_test.go`：

```go
package service

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// 权限码 → 路由映射的双向守护：
// ① permissionRoutes 里每个键的前缀都能被 permPrefix 正确归类；
// ② 种子迁移里出现的每个 infra 权限码，其前缀必须已注册路由——
//    否则 casbin 无 policy，admin 403（Review Focus #4）。
func TestPermissionRoutes_CoverInfraSeed(t *testing.T) {
	// permPrefix 是前两段：infra:codegen:import → infra:codegen
	if got := permPrefix("infra:codegen:import"); got != "infra:codegen" {
		t.Errorf("permPrefix = %q", got)
	}
	if _, ok := permissionRoutes["infra:codegen"]; !ok {
		t.Fatal("permissionRoutes 缺 infra:codegen（种子菜单会 403）")
	}
	// 映射必须含根集合与通配子路径
	patterns := permissionRoutes["infra:codegen"]
	want := []string{"/api/admin/infra/codegen", "/api/admin/infra/codegen/*"}
	for _, w := range want {
		found := false
		for _, p := range patterns {
			if p == w {
				found = true
			}
		}
		if !found {
			t.Errorf("infra:codegen 缺路由模式 %q，got %v", w, patterns)
		}
	}
}

// 扫种子 SQL：出现的 infra:codegen:* 权限码前缀必须都在 permissionRoutes。
func TestInfraSeed_PermsRegistered(t *testing.T) {
	// 本测试在 modules/system/service 下运行；迁移在 ../../modules/system/migrations
	migPath := filepath.Join("..", "migrations")
	entries, err := os.ReadDir(migPath)
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	permRe := regexp.MustCompile(`'((infra|system):[a-z0-9_:]+)'`)
	checked := 0
	for _, e := range entries {
		if !strings.Contains(e.Name(), "codegen_menus") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(migPath, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range permRe.FindAllStringSubmatch(string(b), -1) {
			checked++
			if _, ok := permissionRoutes[permPrefix(m[1])]; !ok {
				t.Errorf("%s 中权限码 %q 前缀 %q 未注册路由", e.Name(), m[1], permPrefix(m[1]))
			}
		}
	}
	if checked == 0 {
		t.Error("种子迁移未扫到任何权限码——正则或 SQL 结构变了，测试失效")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./modules/system/service/ -run "InfraSeed|CoverInfra" -count=1`
Expected: FAIL（`permissionRoutes 缺 infra:codegen`；且此时种子迁移还不存在，`checked==0` 报"测试失效"）

- [ ] **Step 3: 注册 permissionRoutes**

`modules/system/service/permission_routes.go` 的 `permissionRoutes` map 追加：

```go
		"infra:codegen": {
			"/api/admin/infra/codegen",
			"/api/admin/infra/codegen/*",
		},
```

（放在 map 字面量现有条目之后、闭括号之前，键按字母序或与相邻条目一致——跟随文件现有风格。）

- [ ] **Step 4: 写种子迁移**

`modules/system/migrations/20261011000002_codegen_menus.up.sql`（幂等 `NOT EXISTS`；**所有 id 一律子查询，不写死**；业务键：目录用 `type=0 AND name`，页面/按钮用 `permission`）：

```sql
-- codegen 菜单 + 按钮（spec §4 admin 入口）。
-- 幂等：全部 NOT EXISTS 守卫，可重复跑；权限码前缀 infra:codegen 必须
-- 已在 permission_routes.go 注册，否则本迁移产生的按钮是死权限（403）。
-- 依赖 20261009000001 的「系统管理」目录与超管角色。

-- 1) 目录「代码生成」（挂在系统管理下）
INSERT INTO `system_menu` (`name`, `permission`, `type`, `sort_order`, `parent_id`, `path`, `component`, `icon`, `status`, `created_at`, `updated_at`)
SELECT '代码生成', '', 0, 90, p.`id`, 'codegen', 'layout', 'ant-design:code-outlined', 1, NOW(), NOW()
FROM `system_menu` p
WHERE p.`name` = '系统管理' AND p.`type` = 0
  AND NOT EXISTS (
    SELECT 1 FROM `system_menu` d WHERE d.`type` = 0 AND d.`name` = '代码生成'
  )
LIMIT 1;

-- 2) 页面菜单「代码生成配置」（挂在目录下）
INSERT INTO `system_menu` (`name`, `permission`, `type`, `sort_order`, `parent_id`, `path`, `component`, `icon`, `status`, `created_at`, `updated_at`)
SELECT '代码生成配置', 'infra:codegen', 2, 1, d.`id`, 'config', 'infra/codegen/index', 'ant-design:file-outlined', 1, NOW(), NOW()
FROM `system_menu` d
WHERE d.`type` = 0 AND d.`name` = '代码生成'
  AND NOT EXISTS (
    SELECT 1 FROM `system_menu` m WHERE m.`permission` = 'infra:codegen'
  )
LIMIT 1;

-- 3) 按钮：导入 / 同步 / 删除（挂在页面菜单下）
INSERT INTO `system_menu` (`name`, `permission`, `type`, `sort_order`, `parent_id`, `path`, `component`, `icon`, `status`, `created_at`, `updated_at`)
SELECT t.name, t.perm, 3, t.sort, m.`id`, '', '', '', 1, NOW(), NOW()
FROM `system_menu` m
JOIN (
  SELECT '导入' AS name, 'infra:codegen:import' AS perm, 1 AS sort
  UNION ALL SELECT '同步', 'infra:codegen:sync', 2
  UNION ALL SELECT '删除', 'infra:codegen:delete', 3
) t
WHERE m.`permission` = 'infra:codegen' AND m.`type` = 2
  AND NOT EXISTS (
    SELECT 1 FROM `system_menu` b WHERE b.`permission` = t.perm
  );

-- 4) 超管角色挂菜单（tenant 超管 role_id=1 惯例见 20261009000001；
--    按该迁移的实际角色定位方式对齐——优先复用其子查询模式）
INSERT INTO `system_role_menu` (`tenant_id`, `role_id`, `menu_id`, `created_at`, `updated_at`)
SELECT rm.`tenant_id`, rm.`role_id`, m.`id`, NOW(), NOW()
FROM `system_menu` m
JOIN `system_role_menu` rm ON rm.`menu_id` = (
    SELECT d2.id FROM system_menu d2 WHERE d2.`type` = 0 AND d2.`name` = '系统管理' LIMIT 1
  )
WHERE (m.`permission` = 'infra:codegen' OR m.`permission` LIKE 'infra:codegen:%'
       OR m.`id` = (SELECT d3.id FROM system_menu d3 WHERE d3.`type` = 0 AND d3.`name` = '代码生成' LIMIT 1))
  AND NOT EXISTS (
    SELECT 1 FROM `system_role_menu` e
    WHERE e.`tenant_id` = rm.`tenant_id` AND e.`role_id` = rm.`role_id` AND e.`menu_id` = m.`id`
  )
GROUP BY rm.`tenant_id`, rm.`role_id`, m.`id`;
```

**执行者核对清单**（写文件前打开 `modules/system/migrations/20261009000001_menus_vben_backfill.up.sql` 对齐）：
1. 表名是 `system_menu` 还是 `sys_menu`？列名 `permission`/`sort_order`/`parent_id` 与既有迁移是否一致；
2. 目录 type、页面 type、按钮 type 的取值（0/2/3 还是 1/2/3）以既有数据为准；
3. 超管角色挂载模式（tenant_id/role_id 定位）**直接抄 20261009 的第 4 段**，只替换 menu 定位子查询；
4. 若实际结构与上面不符，**以既有迁移为准改写，保留三个语义**：业务键定位不写死 id、每条 `NOT EXISTS` 幂等、down 完整逆序。

`modules/system/migrations/20261011000002_codegen_menus.down.sql`：

```sql
-- 逆序删除：先角色关联，再按钮/页面/目录。
DELETE rm FROM `system_role_menu` rm
JOIN `system_menu` m ON m.`id` = rm.`menu_id`
WHERE m.`permission` LIKE 'infra:codegen%'
   OR m.`name` = '代码生成';

DELETE FROM `system_menu`
WHERE `permission` LIKE 'infra:codegen%'
   OR `name` IN ('代码生成', '代码生成配置');
```

- [ ] **Step 5: 跑权限测试确认通过**

Run: `go test ./modules/system/service/ -run "InfraSeed|CoverInfra" -count=1 -v`
Expected: PASS（`permissionRoutes` 已注册 + 种子 SQL 存在且权限码被扫到）

- [ ] **Step 6: 种子双跑幂等断言（改 `modules/infra/migrations/migrate_test.go`）**

**执行者核对**：菜单种子在 **system** 模块迁移里。若 `runInfra` 只跑 infra 源，本断言需改为跑 system 源——在同文件加第二个 helper `runSystem(t)`（构造 `[]migrate.Source{system.NewMigrationSource()}`，import `"alexGo-cloud/modules/system"`）。追加测试：

```go
// 菜单种子双跑幂等：两次 run 后 infra 菜单数恒为 1 目录+1 页面+3 按钮。
func TestInfraSeedMenus_UpTwiceIdempotent(t *testing.T) {
	db, runner := runSystem(t) // system 迁移源（含 20261011000002）

	if err := runner.Run(); err != nil {
		t.Fatalf("run #1: %v", err)
	}
	count := func() int64 {
		var n int64
		db.Table("system_menu").
			Where("permission LIKE 'infra:codegen%' OR name = '代码生成'").
			Count(&n)
		return n
	}
	first := count()
	if first != 5 {
		t.Errorf("菜单数 = %d, want 5（目录+页面+3按钮，按实际表名调整）", first)
	}
	if err := runner.Run(); err != nil {
		t.Fatalf("run #2: %v", err)
	}
	if second := count(); second != first {
		t.Errorf("双跑幂等破坏: %d → %d", first, second)
	}
}
```

- [ ] **Step 7: 运行**

Run: `DB_DSN='...' go test -tags integration ./modules/infra/migrations/ -count=1 -v && go test ./modules/system/service/ -count=1`
Expected: PASS（无 DB_DSN 时集成部分 skip，service 部分必绿）

- [ ] **Step 8: Commit**

```bash
git add modules/system/migrations/20261011000002_codegen_menus.* \
        modules/system/service/permission_routes.go \
        modules/system/service/permission_routes_test.go \
        modules/infra/migrations/migrate_test.go
git commit -m "feat(system): codegen 菜单种子 + infra:codegen 路由注册（403 闭环）"
```

---

### Task 10: 特征化测试 + 三个小修复（唯一非 TDD 任务——先钉行为再改）

**Files:**
- Test: `tools/codegen/main_test.go`（追加 `schemaFromDSN` 表驱动测试）
- Modify: `tools/codegen/main.go`（`schemaFromDSN` → `mysql.ParseDSN`）
- Test: `pkg/codegen/postprocess/golang_test.go`（新建/追加）
- Modify: `pkg/codegen/postprocess/golang.go`（非 `.go` 路径直通）
- Test: `pkg/codegen/naming/naming_char_test.go`（新建特征化）
- Test: `pkg/config/loader_test.go`（追加 2 个 env 用例）
- Test: `tools/codegen/smoke_test.go`（新建：随机表名端到端 smoke）

**Interfaces:**
- Consumes: `go-sql-driver/mysql.ParseDSN`（已是间接依赖，加直接 import）；`format.Source`；`naming.Snake/EntityName` 既有行为；`config.applyEnvOverrides` 既有分支。
- Produces: 无新接口——本任务是**加固与清理**，为 M3 前端输出（非 `.go` 文件必须免 gofmt）与 CLI 可靠性兜底。**本任务是计划里唯一允许「先写测试看红/绿、再改代码」顺序与 TDD 不同的：特征化测试先钉现状，明确标红的两个 case 才是待修缺陷。**

- [ ] **Step 1: schemaFromDSN 特征化（红→绿）**

`tools/codegen/main_test.go` 追加：

```go
func TestSchemaFromDSN(t *testing.T) {
	cases := []struct {
		dsn  string
		want string
	}{
		{"user:pass@tcp(127.0.0.1:3306)/alexgo?charset=utf8mb4", "alexgo"},
		{"user:pass@tcp(127.0.0.1:3306)/alexgo", "alexgo"},
		// 缺陷①：unix socket 无库名——手写解析返回 "mysqld.sock)"，ParseDSN 返回 ""
		{"user:pass@unix(/var/run/mysqld/mysqld.sock)", ""},
		{"user:pass@unix(/var/run/mysqld/mysqld.sock)/alexgo", "alexgo"},
		// 缺陷②：密码含 @/斜杠时 LastIndex 截断错位
		{"user:p@ss@word@tcp(h:3306)/db1", "db1"},
	}
	for _, tc := range cases {
		if got := schemaFromDSN(tc.dsn); got != tc.want {
			t.Errorf("schemaFromDSN(%q) = %q, want %q", tc.dsn, got, tc.want)
		}
	}
}
```

Run: `go test ./tools/codegen/ -run SchemaFromDSN -count=1`
Expected: **FAIL，恰好 2 个 case**（unix socket 无库名、密码含 `@`）——这就是要修的缺陷边界；若其他 case 也红，先核对期望值与 `mysql.ParseDSN` 实测输出再继续。

实现：`tools/codegen/main.go` 的 `schemaFromDSN` 体替换为：

```go
// schemaFromDSN 返回 DSN 中的库名；无库名时返回 ""。
// 用驱动的 ParseDSN 而非手写切串：unix socket、密码含特殊字符均正确。
func schemaFromDSN(dsn string) string {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return ""
	}
	return cfg.DBName
}
```

（import `"github.com/go-sql-driver/mysql"`；`go mod tidy` 确认直接依赖。）

Run: `go test ./tools/codegen/ -count=1`
Expected: PASS（含既有 2 个写文件测试）

- [ ] **Step 2: FormatGo 非 `.go` 直通（红→绿）**

`pkg/codegen/postprocess/golang_test.go`：

```go
package postprocess

import "testing"

// 非 .go 文件（.vue/.ts/.sql，M4 前端输出）不得送进 gofmt——
// format.Source 对非 Go 源会报错，导致整批生成失败。
func TestFormatGo_NonGoPassthrough(t *testing.T) {
	for _, path := range []string{"a.vue", "b.ts", "c.sql", "README.md", "noext"} {
		src := []byte("const a = 1\n  <div>not go</div>\n")
		got, err := FormatGo(path, src)
		if err != nil {
			t.Errorf("FormatGo(%q) err = %v, want nil passthrough", path, err)
		}
		if string(got) != string(src) {
			t.Errorf("FormatGo(%q) 改变了内容", path)
		}
	}
}

// .go 仍正常 gofmt。
func TestFormatGo_GoStillFormatted(t *testing.T) {
	got, err := FormatGo("x.go", []byte("package  p\n"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "package p\n" {
		t.Errorf("got %q", got)
	}
}
```

Run: `go test ./pkg/codegen/postprocess/ -count=1`
Expected: 第一例 FAIL（当前 `FormatGo` 无条件 `format.Source`），第二例 PASS

实现：`golang.go` 的 `FormatGo` 开头加：

```go
	if !strings.HasSuffix(path, ".go") {
		return src, nil // 非 Go 源直通（前端/SQL 输出）
	}
```

（补 import `"strings"`。若 `FormatGo` 当前签名不含 `path` 参数，按调用方 `pkg/codegen` 生成链的实际签名适配——**保持既有调用点编译通过**，必要时改为在调用点判断。）

Run: `go test ./pkg/codegen/postprocess/ ./pkg/codegen/ -count=1`
Expected: PASS（golden 不受影响——golden 全是 `.go`）

- [ ] **Step 3: Snake/EntityName 特征化（全绿钉现状）**

`pkg/codegen/naming/naming_char_test.go`：

```go
package naming

import "testing"

// 特征化测试：钉住当前分词行为，重构 Snake 时防静默漂移。
// 注意 V2/IPv4 这类大小写边界当前会插下划线（order_item_v_2）——
// 这是**现状**而非期望值；改行为必须先改本测试并全量评估生成物 diff。
func TestSnake_Characterized(t *testing.T) {
	cases := map[string]string{
		"APIKey":     "api_key",
		"OrderItemV2": "order_item_v_2",
		"V2Ray":      "v_2_ray",
		"UserID2":    "user_id_2",
		"HTTPServer": "http_server",
		"IPv4Addr":   "i_pv_4_addr",
		"tenantID":   "tenant_id",
	}
	for in, want := range cases {
		if got := Snake(in); got != want {
			t.Errorf("Snake(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEntityName_Characterized(t *testing.T) {
	if got := EntityName("APIKey"); got != "Apikey" {
		t.Errorf("EntityName(APIKey) = %q, want %q", got, "Apikey")
	}
}
```

Run: `go test ./pkg/codegen/naming/ -count=1 -v`
Expected: PASS（这些值是实测输出；若红，以 `Snake` 当前实测值为准更新期望并**在 commit message 里注明哪条与计划不符**）

- [ ] **Step 4: loader env 覆盖补测（全绿钉现状）**

`pkg/config/loader_test.go` 追加：

```go
// DEPLOYMENT_MODE / SYSTEM_GRPC_ADDR 也必须被 env 压过（K8s 注入前提）。
// 此前只测了 DB_DSN/JWT_SECRET/REDIS_PASSWORD——覆盖矩阵缺口。
func TestLoadGlobalConfig_EnvOverridesDeploymentAndGRPC(t *testing.T) {
	chdirRepoRoot(t)
	t.Setenv("DEPLOYMENT_MODE", "micro")
	t.Setenv("SYSTEM_GRPC_ADDR", ":60051")

	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		t.Fatalf("LoadGlobalConfig() error = %v", err)
	}
	if cfg.Deployment.Mode != "micro" { // 字段名以 config.Config 实际结构为准
		t.Errorf("Deployment.Mode = %q, want micro", cfg.Deployment.Mode)
	}
	if cfg.System.GRPCAddr != ":60051" {
		t.Errorf("System.GRPCAddr = %q", cfg.System.GRPCAddr)
	}
}
```

Run: `go test ./pkg/config/ -count=1 -v`
Expected: PASS（`applyEnvOverrides` 已实现这两个分支——特征化；若字段名不符，以 `pkg/config` 实际结构为准改测试）

- [ ] **Step 5: CLI 端到端 smoke（随机表后缀防污染）**

M1 已有端到端 smoke（commit `137cbc5`：真实表 → 生成 → go build → 清理）。本步**先定位既有文件再决定合并还是新增**：

Run: `git show 137cbc5 --stat | head -30` + `grep -rln "smoke\|RoundTrip" --include="*_test.go" tools/ pkg/codegen/`
Expected: 找到既有 smoke 测试文件与测试函数名。

**情况 A（既有 smoke 已存在）**：只改表名生成与清理——打开该文件，将固定表名替换为：

```go
	// 随机后缀防残留污染：上次运行崩溃留下的表不干扰本次
	suffix := make([]byte, 6)
	for i := range suffix {
		suffix[i] = "abcdefghijklmnopqrstuvwxyz0123456789"[rand.Intn(36)]
	}
	tbl := "cg_smoke_" + string(suffix)
	t.Cleanup(func() {
		_, _ = db.Exec("DROP TABLE IF EXISTS `" + tbl + "`")
	})
```

（import `"math/rand"`。若既有测试已含随机后缀与 `t.Cleanup` drop，则该测试已达标，**跳过本步**并在 commit message 注明"smoke 已由 137cbc5 覆盖"。）

**情况 B（找不到既有 smoke）**：新建 `tools/codegen/smoke_test.go`：

```go
//go:build smoke

package main

import (
	"context"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"alexGo-cloud/pkg/codegen"
	"alexGo-cloud/pkg/codegen/builder"
	"alexGo-cloud/pkg/codegen/metadata"
)

// 真实 MySQL：建临时表 → 读元数据 → builder.Build → codegen.Generate →
// 落临时目录 → go build 生成的包 → 清理。DB_DSN 未设时 skip。
func TestSmoke_RoundTrip(t *testing.T) {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		t.Skip("DB_DSN not set")
	}
	reader, err := metadata.NewMySQLReader(dsn) // 签名以 Task 7 Step 5a 的核对结果为准
	if err != nil {
		t.Fatalf("reader: %v", err)
	}
	ctx := context.Background()

	suffix := make([]byte, 6)
	for i := range suffix {
		suffix[i] = "abcdefghijklmnopqrstuvwxyz0123456789"[rand.Intn(36)]
	}
	tbl := "cg_smoke_" + string(suffix)

	// 建临时表：单列 id 主键 + 2 列（满足 §9.1 单列 id 约定）
	if _, err := dbExec(t, dsn,
		"CREATE TABLE `"+tbl+"` ("+
			"`id` bigint unsigned NOT NULL AUTO_INCREMENT,"+
			"`name` varchar(64) NOT NULL,"+
			"`qty` int NOT NULL DEFAULT 0,"+
			"PRIMARY KEY (`id`)) ENGINE=InnoDB"); err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { _, _ = dbExec(t, dsn, "DROP TABLE IF EXISTS `"+tbl+"`") })

	meta, err := reader.ReadTable(ctx, tbl)
	if err != nil {
		t.Fatalf("ReadTable: %v", err)
	}
	built, err := builder.Build(meta, builder.Options{Module: "smoke", TemplateType: builder.TemplateSingle, FrontType: builder.FrontVben5Antd})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	files, err := codegen.Generate([]*model.Table{built}, codegen.Options{UnitTestEnable: true})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no files generated")
	}

	// 落盘并 go build（生成物必须可编译）
	dir := t.TempDir()
	pkgDir := filepath.Join(dir, "smoke")
	for _, f := range files {
		p := filepath.Join(pkgDir, f.Path)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, f.Content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = pkgDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build generated pkg: %v\n%s", err, out)
	}
}
```

（`dbExec` 是打开 MySQL 执行裸 SQL 的小 helper——若 M1 smoke 里已有等价函数/import 的 `database.NewDB` 可复用则复用；`model.Table` 的 import 路径是 `alexGo-cloud/pkg/codegen/model`；`codegen.Generate`/`builder.Options` 字段名以源码为准，与 M1 golden 测试调用处一致。）

Run: `DB_DSN='...' go test -tags smoke ./tools/codegen/ ./pkg/codegen/ -count=1 -v -run Smoke`
Expected: PASS（无 DSN 时 skip）

- [ ] **Step 6: 全量回归**

Run: `go test ./pkg/... ./modules/infra/... ./modules/system/service/... ./tools/codegen/... -count=1 && CGO_ENABLED=0 go test ./alexgo-server/cmd/ -count=1`
Expected: PASS（已知 cgo 链接问题的 5 个包用 `CGO_ENABLED=0`，见 architecture.md §8）

- [ ] **Step 7: Commit**

```bash
git add tools/codegen/ pkg/codegen/postprocess/ pkg/codegen/naming/ pkg/config/loader_test.go
git commit -m "fix(codegen): DSN 解析改 ParseDSN + 非 go 直通；补特征化与 env 覆盖测试"
```

（smoke 若单独成文件则一并 `git add`。）

---

## M2 完成验收

- [ ] **引擎修复**（Task 1–2）：`go test ./pkg/codegen/... -count=1` 全绿；golden 恰好 4 个文件变更（model/repository/service/service_test），3 个不变；`TestTenantWritePath_GormSemantics` 钉住 gorm `Save` 丢 WHERE 的坑
- [ ] **配置持久化**（Task 3–4）：`-tags integration` 双跑迁移幂等；`codegen_table` 无 `tenant_id`/`deleted`；仓库 4 测试绿
- [ ] **导入与同步**（Task 5–6）：四类拒绝 `errors.Is` 精确断言 + 0 落库；§9.4 三钉子（配置保留/deprecated 不物理删/重现恢复）+ 新列全关全绿
- [ ] **admin API**（Task 7）：9 端点信封测试绿；只用 400/500；`generate` 占位返回 500 带 "M3"；fx 干跑 3 测试绿
- [ ] **验收链**（Task 8）：导入→改配置→同步 diff→二次同步幂等，一条测试跑通（spec §8 M2）
- [ ] **403 闭环**（Task 9）：`permissionRoutes` 注册 + 种子幂等 + 守护测试绿；`DB_DSN` 双跑菜单数恒 5
- [ ] **加固**（Task 10）：`schemaFromDSN` 两缺陷修复；`.vue/.ts` 直通；Snake/loader 特征化钉现状；smoke 过
- [ ] 全量：`go test ./... `（除已知 cgo 包 `CGO_ENABLED=0`）绿；每个 task 一个独立 commit，风格 `feat(infra):`/`fix(codegen):`/`test(infra):`
- [ ] 本计划之外的既有未提交改动（menu 编辑功能）不混入任何上述 commit

