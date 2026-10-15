# oauth2 / infra 并入 system 模块设计

- 日期：2026-10-15
- 状态：已评审（用户确认方案 B）
- 范围：后端代码组织与 fx 装配；不动路由、权限、菜单、数据库 schema、前端

## 1. 背景与目标

`modules/oauth2`（OAuth2 协议端点 + 统一登录门面 + 客户端管理）与 `modules/infra`（代码生成在线链路）目前是与 `modules/system` 平级的顶层模块。二者在领域上是系统层面的能力，需求是**融入 system：它们是 system 的一部分**。

目标（方案 B，用户选定）：

1. 目录迁入：`modules/oauth2` → `modules/system/oauth2`，`modules/infra` → `modules/system/infra`。
2. 装配并入：`oauth2.FxModule`、`infra.FxModule` 嵌入 `system.FxModule`，`alexgo-server` 主入口只挂 `system.FxModule`。
3. 唯一保留的独立入口：member 微服务进程（`modules/member/cmd`）继续直接挂 `oauth2.FxModule` —— 该进程按现有 micro 架构**不装 system**（无 Casbin/菜单），不能拖入 system 全家桶。

### 结构性不变量（绝不改变）

| 类别 | 不变量 |
| --- | --- |
| 路由 | `/api/oauth2/*`、`/api/admin/oauth2/*`、`/api/admin/infra/codegen/*` 完全不变 |
| 权限 | 权限码（`oauth2:client*`、`infra:codegen*`）、casbin 前缀映射、菜单种子不变 |
| 迁移 | `GetModuleName()` 保持 `oauth2` / `infra` → `schema_migrations_oauth2` / `schema_migrations_infra` 记录表不变，已跑过的库不重跑 |
| 数据库 | schema 零变化 |
| 包名 | Go 包名仍为 `oauth2` / `infra`；`fx.Module` 名仍为 `"oauth2"` / `"infra"` |
| 前端 | `apps/web-antd` 不动 |
| codegen 产物 | 模板路径 `modules/{module}/...` 中的 `{module}` 是**目标业务模块名**，与本次搬家无关 |

## 2. 依赖核查（可行性依据）

- `modules/oauth2` 非测试代码只依赖 `pkg/*`（config/migrate/token/middleware/auth/server）与自身子包，**从不 import system/member**。
- `modules/infra` 非测试代码只依赖 `pkg/*`（codegen/migrate/config/server）与自身子包，**从不 import system**。
- 反向边：`system`（root + service + controller/app + controller/vben）→ `oauth2/service` 共 9 处；`member` → `oauth2` 共 7 处；`alexgo-server/cmd`、`member/cmd` → `oauth2` root；`alexgo-server/cmd`、`tools/codegen` → `infra`。

结论：嵌套后依赖图是 `system root → {system/oauth2, system/infra} → pkg/*` 的单向图，**无生产代码环**。

唯一环点在测试期：`modules/oauth2/e2e_test.go`（`package oauth2`，`//go:build integration`）import `modules/system` root，而 B 方案落地后 system root 会 import oauth2 root → 测试成环。**处置：转外部测试包 `package oauth2_test`**（其对包内符号的使用全部为导出符号，`NewMigrationSource()` 加 `oauth2.` 限定即可），环消除。

## 3. 改动清单

### 3.1 目录迁移（保留 git 历史）

```
git mv modules/oauth2  modules/system/oauth2
git mv modules/infra   modules/system/infra
```

### 3.2 import 前缀全局重写

- `alexGo-cloud/modules/oauth2` → `alexGo-cloud/modules/system/oauth2`
  - 模块内部自引用（23 个 go 文件中的 import）
  - `modules/system`：module.go、controller/app、controller/vben、service/user_authenticator.go 及配套测试（9 个文件）
  - `modules/member`：module.go、service、controller/app、cmd/main.go 及配套测试（7 个文件）
  - `alexgo-server/cmd/main.go`
- `alexGo-cloud/modules/infra` → `alexGo-cloud/modules/system/infra`
  - 模块内部自引用（14 个 go 文件）
  - `alexgo-server/cmd/main.go`、`tools/codegen/main.go`、`tools/codegen/main_test.go`

### 3.3 路径字符串（3 处）

- `modules/system/oauth2/migration_source.go`：`GetMigrationPath()` → `"modules/system/oauth2/migrations"`
- `modules/system/infra/migration_source.go`：`GetMigrationPath()` → `"modules/system/infra/migrations"`
- `modules/system/oauth2/migrations/migrate_test.go`：`seedClientsPath` → 新路径
- （`modules/system/infra/migrations/migrate_test.go` 引用的是 `modules/system/migrations/*`，迁后天然正确，无改动）

### 3.4 fx 装配（方案 B 核心）

`modules/system/module.go`：

```go
import (
    "alexGo-cloud/modules/system/oauth2"
    "alexGo-cloud/modules/system/infra"
    // ...既有 import 同步改前缀
)

var FxModule = fx.Module("system",
    // ...现有 Provide/Invoke 不变...
    oauth2.FxModule,
    infra.FxModule,
)
```

- `alexgo-server/cmd/main.go`：删除 `oauth2.FxModule`、`infra.FxModule` 两行及两个顶层 import（保留会造成 fx 重复 provider 直接报错）。
- `modules/member/cmd/main.go`：保留 `oauth2.FxModule`，仅改 import 路径为 `alexGo-cloud/modules/system/oauth2`；`member.FxModule` 不变。
- `oauth2.FxModule`、`infra.FxModule` 保持导出（member 进程与测试消费）。

### 3.5 测试适配

- `e2e_test.go`：`package oauth2` → `package oauth2_test`，import 增加 `oauth2 "alexGo-cloud/modules/system/oauth2"`，`NewMigrationSource()` → `oauth2.NewMigrationSource()`；同时更新其全部 import 前缀。
- 其余测试文件：仅随 import 前缀机械更新；`registrar_test.go`（不整树干跑）与 `alexgo-server/cmd/main_test.go`（`fx.ValidateApp` 覆盖 mono/micro/migrate-only 三形态，恰好验收组合后的装配图）逻辑零改动。

## 4. 文档同步（用户要求：活文档与历史文档全改）

- **活文档**：`README.md`（3 处）、`docs/modules.md`（5 处）、`docs/oauth2.md`（1 处）做路径替换；`docs/architecture.md` 等叙述「modules/ 下有哪些模块」的段落做口径同步 —— 除路径机械替换外，口径同步：「5 个后端业务模块」→ 3 个顶层模块（system 含 oauth2/infra 子模块、member、order）；模块装配清单（mono/micro 进程装哪些模块）表述同步。
- **历史文档**：`docs/superpowers/plans/*`（unified-login 176 处、codegen-m2-infra 81 处、codegen-m3-online 49 处、m1/m5 各 1 处）、`docs/superpowers/specs/*`（codegen-redesign 8 处、unified-login-design 4 处）—— **只机械替换路径字符串**，历史叙述不改写。
- 代码内注释中的旧路径引用（如「照抄 modules/infra/module.go」）一并更新。

## 5. 明确不改（Out of scope）

1. `infra/module.go` 的 `builder.Options{Module: "infra"}`：这是在线 codegen **导入快照时的占位模块名**（拼进 zip 内路径，操作者本就需改成目标模块如 `order`），与目录归属无关，改它扩大行为面。
2. 路由、权限码、菜单、casbin 映射、数据库 schema、前端。
3. Go 包名与 `fx.Module` 名。

## 6. 验证策略

1. `go build ./...`、`go vet ./...` —— 全仓编译。
2. `go test ./...` —— 单测全量（含 `alexgo-server/cmd` 的三形态 `fx.ValidateApp`、oauth2/system/member 单测）。
3. `go test -tags integration ./modules/system/oauth2/...` —— e2e（需 MySQL/Redis；环境不齐时如实记录跳过）。
4. 残留 grep 必须为 0：`alexGo-cloud/modules/oauth2`、`alexGo-cloud/modules/infra`、`modules/oauth2/`、`modules/infra/`（路径串，含文档）。
5. 迁移回归：migrate 双跑测试（oauth2/infra/system 各自 `migrations/migrate_test.go`）确认记录表名不变、不重跑。
6. `tools/codegen` 测试（含冒烟）通过 —— 确认 import 改写无遗漏。

## 7. 风险与对策

| 风险 | 对策 |
| --- | --- |
| 漏改 import/路径导致编译失败 | 全仓 sed + grep 残留为 0 作为验收门槛 |
| fx 重复 provider（双挂 FxModule） | main.go 删除两行；`main_test.go` 的 `ValidateApp` 三形态兜底 |
| e2e 测试期 import 环 | 转 `package oauth2_test`（§2） |
| 历史迁移在新路径下重跑 | `GetModuleName()` 不变，记录表照旧；migrate 测试断言 |
| member 微服务被拖入 system | `member/cmd` 仍只挂 `oauth2.FxModule`，不 import system root |
