# oauth2 / infra 并入 system 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 `modules/oauth2` 与 `modules/infra` 在目录与 fx 装配上并入 `modules/system`（方案 B），路由/权限/菜单/迁移记录表/DB schema/前端零变化。

**Architecture:** `git mv` 两模块进 `modules/system/`，全仓重写 import 前缀；`system.FxModule` 内嵌两个子模块的 `FxModule`，主入口只挂 `system.FxModule`；member 微服务进程保留 `oauth2.FxModule` 直引（不装 system）。文档（含历史 plan/spec）路径全量同步。

**Tech Stack:** Go 1.x + uber/fx + MySQL/Redis（DB 相关测试无 DSN 时跳过）、macOS `sed -i ''`。

**Spec:** `docs/superpowers/specs/2026-10-15-oauth2-infra-into-system-design.md`（执行者需先读 spec）

## Global Constraints

- **结构性不变量（spec §1）**：路由 `/api/oauth2/*`、`/api/admin/oauth2/*`、`/api/admin/infra/codegen/*`；权限码；菜单；`GetModuleName()` 返回值 `oauth2`/`infra`（记录表 `schema_migrations_oauth2`/`_infra` 不变）；DB schema；Go 包名（仍为 `oauth2`/`infra`）；`fx.Module` 名（仍为 `"oauth2"`/`"infra"`）；前端零改动。
- **不许碰的 codegen 产物路径字面量**（spec §5，行为未变）：`modules/system/infra` 迁移完成后，仓库里**仅允许**这 5 行仍含裸 `modules/infra` —— `modules/system/infra/controller/admin/codegen_controller_test.go:184`、`:193`，`modules/system/infra/service/acceptance_test.go:126`、`:127`、`:129`（生成物路径 `modules/{module}/...`，`Module:"infra"` 占位行为不变）。
- **spec §5.1 out-of-scope 原样保留**：`builder.Options{Module: "infra"}` 等占位模块名**一行不改**（含 `modules/system/infra/service` 源码与其在历史文档中的引用）。
- **`docs/superpowers/` 下 `2026-10-15-oauth2-infra-into-system` 两个文件（spec + 本计划）永不参与路径 sed** —— 它们记录的是迁移前路径本身。
- 直接在 `main` 分支提交（项目约定，见 memory `work-directly-on-main`）。
- 每个 Task 结束：`gofmt -l .` 必须为空（有输出则 `gofmt -w` 后重跑）。
- DB 相关测试（`migrations/*`、e2e）无 `DB_DSN` 时会跳过：**如实记录 skip，不得谎称跑过**。

## Review Focus

1. **chdir 固定深度失配**：迁移测试/e2e 的 `chdirRepoRoot` 用固定个数 `..` 上跳；搬家后深度变化 → 跳错层 → Runner **吞掉** init 错误、症状是「表没建」而非报错。归属 Task 1/2：每个 chdir 加 `go.mod` 断言（响亮失败），并用 `grep -o '"\.\."' | wc -l` 静态断言层数（3/4/4）。
2. **fx 双挂**：Task 3 若忘删 `alexgo-server/cmd/main.go` 里的 `oauth2.FxModule`/`infra.FxModule`，fx 重复 provider 直接 panic。归属 Task 3：`main_test.go` 三形态 `fx.ValidateApp` + grep 断言 main.go 中只剩 system/order/member。
3. **codegen 产物路径被 sed 误改**：对 `.go` 用**前缀锚定** sed（`alexGo-cloud/modules/infra`），裸路径逐处手改；误改会让 acceptance/controller 断言与未变的生成器行为失配。归属 Task 2：`git diff` 断言两个产物文件只有 import 行变化 + 期望残留恰为 5 行。
4. **e2e 测试包转换不彻底**：`e2e_test.go` 由 `package oauth2` 改 `package oauth2_test` 后若漏改未导出符号引用，带 integration tag 才编译失败。归属 Task 1：`go vet -tags integration ./modules/system/oauth2/` 必须通过。
5. **member 微服务被拖入 system**：`modules/member/cmd` 若误 import system root，会把 Casbin/全家桶带进 member 进程。归属 Task 3：grep 断言该文件无 `"alexGo-cloud/modules/system"`（精确整串）+ member 测试通过。

---

### Task 1: oauth2 模块迁入 `modules/system/oauth2`

**Files:**
- Move: `modules/oauth2/**` → `modules/system/oauth2/**`（git mv 整目录）
- Modify: 全仓含 `alexGo-cloud/modules/oauth2` 的 `.go` 文件（import 前缀；当前 23 个模块内文件 + `alexgo-server/cmd/main.go` + `modules/member/` 7 处 + `modules/system/` 9 处）
- Modify: `modules/system/oauth2/migration_source.go`、`modules/system/oauth2/migrations/migrate_test.go`、`modules/system/oauth2/e2e_test.go`、`modules/system/oauth2/module.go`（裸路径/注释/测试包，见步骤）

**Interfaces:**
- Consumes: 无（首个任务）
- Produces: import 路径 `alexGo-cloud/modules/system/oauth2/{service,controller,repository,model}`；`oauth2.FxModule` 位置不变（仍导出）；后续 Task 3 依赖 `alexGo-cloud/modules/system/oauth2` root 包

- [ ] **Step 1: 基线记录**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud
grep -rl 'alexGo-cloud/modules/oauth2' --include='*.go' . | wc -l   # 记下数字 N1（预期约 40：模块内 23 + system 9 + member 7 + alexgo-server 1）
go test ./... 2>&1 | tail -5   # 基线：全绿（DB 缺省时部分 skip）
```

- [ ] **Step 2: git mv**

```bash
mkdir -p modules/system
git mv modules/oauth2 modules/system/oauth2
ls modules/system/oauth2   # controller e2e_test.go migration_source.go migrations model module.go repository service
```

- [ ] **Step 3: 全仓重写 import 前缀**

```bash
grep -rl 'alexGo-cloud/modules/oauth2' --include='*.go' . | xargs sed -i '' 's|alexGo-cloud/modules/oauth2|alexGo-cloud/modules/system/oauth2|g'
grep -rn 'alexGo-cloud/modules/oauth2' --include='*.go' . | wc -l   # 必须 0（模式不含 system/ 前缀，不会误判）
```

预期：0。若不为 0，逐行人工检查后再继续。

- [ ] **Step 4: 手改裸路径与注释（4 个文件，sed 不覆盖，逐处精确替换）**

`modules/system/oauth2/migration_source.go`：

```go
// 改前
func (s *MigrationSource) GetMigrationPath() string { return "modules/oauth2/migrations" }
// 改后
func (s *MigrationSource) GetMigrationPath() string { return "modules/system/oauth2/migrations" }
```

`modules/system/oauth2/migrations/migrate_test.go`：

```go
// ① 路径常量
const seedClientsPath = "modules/system/oauth2/migrations/20261011000002_oauth2_seed_clients.up.sql"
// ② chdir 深度：modules/system/oauth2/migrations = 4 层 → 4 个 ".."
//    改前：root := filepath.Clean(filepath.Join(wd, "..", "..", ".."))
//    改后：
root := filepath.Clean(filepath.Join(wd, "..", "..", "..", ".."))
// ③ 在上跳后加响亮断言（紧跟 Chdir 之前插入；防跳错层时 Runner 静默吞错）：
if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
    t.Fatalf("chdirRepoRoot 上跳后不在仓库根（缺 go.mod）：%s", root)
}
```

`modules/system/oauth2/module.go` 注释（infra 此刻尚未搬家，但注释指向最终路径）：

```go
// 改前：// 照抄 modules/infra/module.go：迁移源注解进 group:"migration_sources"，
// 改后：// 照抄 modules/system/infra/module.go：迁移源注解进 group:"migration_sources"，
```

- [ ] **Step 5: e2e_test 转外部测试包（消 Task 3 将引入的测试期 import 环）**

文件 `modules/system/oauth2/e2e_test.go` 三处改：

```go
//go:build integration

package oauth2          // ← 改为：package oauth2_test

import (
    // ...既有 import 经 Step 3 已变为 alexGo-cloud/modules/system/oauth2/... ...
    "alexGo-cloud/modules/system/oauth2"   // ← 新增这一行（root 包，用于 NewMigrationSource）
)

// newE2E 内（原 361 行附近）：
//   NewMigrationSource(),        // oauth2_client / ...
// 改为：
    oauth2.NewMigrationSource(),        // oauth2_client / ...

// chdirRepoRoot（原 310-320 行）：
// 注释改：本包位于 modules/system/oauth2（距根三级）。
// 深度改：
root := filepath.Clean(filepath.Join(wd, "..", "..", ".."))
// 断言加（紧跟 Chdir 之前插入）：
if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
    t.Fatalf("chdirRepoRoot 上跳后不在仓库根（缺 go.mod）：%s", root)
}
```

- [ ] **Step 6: 格式化 + 编译**

```bash
gofmt -l . | tee /tmp/fmt1.txt; [ -s /tmp/fmt1.txt ] && xargs gofmt -w < /tmp/fmt1.txt
go build ./...
go vet ./...
go vet -tags integration ./modules/system/oauth2/...   # 关键：e2e 外部包可编译
```

预期：全部通过。若 Step 5 报「undefined: xxx」→ 按报错加 `oauth2.` 前缀（预期仅 `NewMigrationSource` 一处；如出现其它标识符，先 `grep -n` 确认它是 root 包符号再加前缀，若是子包符号检查 import 是否漏了）。

- [ ] **Step 7: 运行测试**

```bash
go test ./... 2>&1 | grep -v '^ok' | grep -v 'no test files'   # 预期无 FAIL；skip 属正常
go test ./modules/system/oauth2/... -count=1
```

若本机有 `DB_DSN`（MySQL）：`DB_DSN='...' go test -tags integration ./modules/system/oauth2/ -count=1 -v` —— 必须真跑；无 DSN 则如实记录「integration 未跑（无 DB_DSN）」，编译已由 Step 6 vet 保证。

- [ ] **Step 8: 验收断言**

```bash
# ① oauth2 裸路径归零（模块位置语义）
grep -rn 'modules/oauth2' --include='*.go' . | grep -v 'modules/system/oauth2'   # 预期 0 行
# ② 迁移模块名未变（记录表不变量）
grep -n 'GetModuleName' modules/system/oauth2/migration_source.go                # 预期 return "oauth2"
# ③ chdir 层数静态断言
grep -o '"\.\."' modules/system/oauth2/e2e_test.go | wc -l                       # 预期 3
grep -o '"\.\."' modules/system/oauth2/migrations/migrate_test.go | wc -l        # 预期 4
# ④ 没人误 import 不存在的旧路径
grep -rn '"alexGo-cloud/modules/oauth2"' --include='*.go' . | wc -l               # 预期 0
```

- [ ] **Step 9: Commit**

```bash
git add -A
git commit -m "refactor(system): oauth2 模块迁入 modules/system/oauth2（方案 B 目录迁移）"
```

---

### Task 2: infra 模块迁入 `modules/system/infra`

**Files:**
- Move: `modules/infra/**` → `modules/system/infra/**`
- Modify: 全仓含 `alexGo-cloud/modules/infra` 的 `.go`（模块内 14 文件 + `alexgo-server/cmd/main.go` + `tools/codegen/main.go` + `tools/codegen/main_test.go`）
- Modify: `modules/system/infra/migration_source.go`、`modules/system/infra/migrations/migrate_test.go`、`pkg/config/config.go:119`
- **禁改**：`modules/system/infra/controller/admin/codegen_controller_test.go`、`modules/system/infra/service/acceptance_test.go` 的产物路径行（仅允许它们的 import 行被 Step 2 改）

**Interfaces:**
- Consumes: Task 1 的目录结构（`modules/system/` 已存在）
- Produces: import 路径 `alexGo-cloud/modules/system/infra/{model,repository,service,controller}`；`infra.FxModule` 仍导出；`GetMigrationPath()` → `modules/system/infra/migrations`

- [ ] **Step 1: git mv**

```bash
git mv modules/infra modules/system/infra
ls modules/system/infra   # controller migration_source.go migrations model module.go repository service
```

- [ ] **Step 2: 全仓重写 import 前缀**

```bash
grep -rl 'alexGo-cloud/modules/infra' --include='*.go' . | xargs sed -i '' 's|alexGo-cloud/modules/infra|alexGo-cloud/modules/system/infra|g'
grep -rn 'alexGo-cloud/modules/infra' --include='*.go' . | wc -l   # 必须 0
```

- [ ] **Step 3: 手改裸路径/注释（3 个文件）**

`modules/system/infra/migration_source.go`：

```go
// 改前：return "modules/infra/migrations"
// 改后：
return "modules/system/infra/migrations"
```

`modules/system/infra/migrations/migrate_test.go`：

```go
// ① 注释（原 22 行）：
// 改前：// chdirRepoRoot 切到仓库根：Runner 的迁移源路径是 file://modules/infra/migrations，
// 改后：// chdirRepoRoot 切到仓库根：Runner 的迁移源路径是 file://modules/system/infra/migrations，
// ② 注释「第二次已在根则不再上跳三级」→「…上跳四级」
// ③ 深度：modules/system/infra/migrations = 4 层（原 3 层）
//    改前：root := filepath.Clean(filepath.Join(wd, "..", "..", ".."))
//    改后：
root := filepath.Clean(filepath.Join(wd, "..", "..", "..", ".."))
// ④ 上跳后加响亮断言（紧跟 Chdir 之前插入）：
if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
    t.Fatalf("chdirRepoRoot 上跳后不在仓库根（缺 go.mod）：%s", root)
}
```

`pkg/config/config.go:119` 注释：

```go
// 改前：// Codegen：代码生成器（modules/infra + tools/codegen）配置。
// 改后：// Codegen：代码生成器（modules/system/infra + tools/codegen）配置。
```

- [ ] **Step 4: 禁改清单核对（产物路径零漂移）**

```bash
git diff -U0 -- modules/system/infra/controller/admin/codegen_controller_test.go modules/system/infra/service/acceptance_test.go | grep '^[+-]' | grep -v '^[+-][+-]' | grep -v 'alexGo-cloud/modules/system/infra'
# 预期：0 行输出 —— 这两个文件的变化必须只有 import 行（alexGo-cloud/modules/system/infra/...）
```

若输出非 0：撤销该行改动（这些是 `Module:"infra"` 生成物路径字面量，行为未变，见 spec §5）。

- [ ] **Step 5: 格式化 + 编译 + 测试**

```bash
gofmt -l . | tee /tmp/fmt2.txt; [ -s /tmp/fmt2.txt ] && xargs gofmt -w < /tmp/fmt2.txt
go build ./...
go vet ./...
go test ./modules/system/infra/... ./tools/codegen/... -count=1   # 含 codegen 冒烟
go test ./... 2>&1 | grep -v '^ok' | grep -v 'no test files'       # 预期无 FAIL
```

有 `DB_DSN` 时：`DB_DSN='...' go test ./modules/system/infra/migrations/ -count=1 -v`；无则如实记 skip。

- [ ] **Step 6: 验收断言**

```bash
# ① 期望残留恰为 5 行产物路径（Global Constraints 禁改清单）
grep -rn 'modules/infra' --include='*.go' . | grep -v 'modules/system/infra' | tee /tmp/residual_infra.txt
wc -l < /tmp/residual_infra.txt    # 预期 5
grep -c . /tmp/residual_infra.txt  # 内容必须逐行命中：codegen_controller_test.go×2 + acceptance_test.go×3
# ② 迁移模块名不变
grep -n 'GetModuleName' modules/system/infra/migration_source.go   # 预期 return "infra"
# ③ chdir 层数
grep -o '"\.\."' modules/system/infra/migrations/migrate_test.go | wc -l   # 预期 4
# ④ 旧 import 归零
grep -rn '"alexGo-cloud/modules/infra"' --include='*.go' . | wc -l          # 预期 0
```

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "refactor(system): infra 模块迁入 modules/system/infra（方案 B 目录迁移）"
```

---

### Task 3: 装配并入 —— `system.FxModule` 内嵌 oauth2/infra

**Files:**
- Modify: `modules/system/module.go`（import + fx.Module 尾部）
- Modify: `alexgo-server/cmd/main.go`（删两行装配 + 两个 import + 头注释口径）
- Test（零逻辑改动，跑通即验收）: `alexgo-server/cmd/main_test.go`、`modules/system/cmd/grpc_main_test.go`、`modules/member/cmd/main_test.go`、`modules/system/registrar_test.go`

**Interfaces:**
- Consumes: Task 1/2 产出的 `alexGo-cloud/modules/system/oauth2`（root，导出 `FxModule`）与 `alexGo-cloud/modules/system/infra`（root，导出 `FxModule`）
- Produces: `system.FxModule` 单入口（内含两子模块）；`oauth2.FxModule`/`infra.FxModule` 保持导出供 `modules/member/cmd` 与测试

- [ ] **Step 1: `modules/system/module.go` 追加装配**

import 块新增两行（gofmt 会排序）：

```go
	"alexGo-cloud/modules/system/infra"
	"alexGo-cloud/modules/system/oauth2"
```

`fx.Module("system", ...)` 的**最后一个参数之后、闭合 `)` 之前**追加：

```go
	// B 方案（2026-10-15）：oauth2/infra 并入 system——子模块装配随 system 整体挂载。
	// 例外：member 微服务进程不装 system，仍直接引用 oauth2.FxModule（modules/member/cmd）。
	oauth2.FxModule,
	infra.FxModule,
)
```

（即原结尾 `	fx.Provide( fx.Annotate( NewMigrationSource, ... ), ),\n)` 变为 `	fx.Provide( ... ),\n	oauth2.FxModule,\n	infra.FxModule,\n)`。）

注意：`system/module.go` 既有 `oauth2svc "…/system/oauth2/service"` 别名 import 与新增 root import（包名 `oauth2`）不冲突。

- [ ] **Step 2: `alexgo-server/cmd/main.go` 删独立装配**

删除（连注释）：

```go
		// oauth2：OAuth2 授权服务骨架——当前仅迁移源（Task 3 起加路由）。
		oauth2.FxModule,
		// infra：代码生成配置持久化（M2）——迁移源 + 三层 + admin 路由。
		infra.FxModule,
```

删除 import `"alexGo-cloud/modules/system/infra"` 与 `"alexGo-cloud/modules/system/oauth2"`（Task 1/2 sed 后的形态；若仍是旧名说明前面任务没做完，先回 Task 1/2 补）。

在原位置留一行口径注释：

```go
		// 模块装配：每个模块将自身作为 server.Module 注册到 group:"modules"。
		// B 方案（2026-10-15）：oauth2/infra 已并入 system.FxModule，随 system 整体挂载。
		system.FxModule,
		order.FxModule,
```

头注释（约 38-44 行）mono/micro 两行口径同步：

```go
//   - mono 模式（默认，deployment.mode != "micro"）：单进程装配 system（内含 oauth2/infra）+order+member
//     全部模块（make run 一键全起），token 签发/校验都用本地 token.Service；
//   - micro 模式（DEPLOYMENT_MODE=micro，make run-system）：本进程只装 system（内含 oauth2/infra）+order，
//     member 路由由 modules/member/cmd 独立进程提供（make run-member），
```

- [ ] **Step 3: 编译 + 入口干跑测试**

```bash
gofmt -l . | tee /tmp/fmt3.txt; [ -s /tmp/fmt3.txt ] && xargs gofmt -w < /tmp/fmt3.txt
go build ./...
go vet ./...
go test ./alexgo-server/cmd/ -count=1 -v          # mono/micro/migrate-only 三形态 fx.ValidateApp
go test ./modules/system/... -count=1             # 含 registrar_test
go test ./modules/system/cmd/ -count=1 -v         # grpc_main 干跑（会带上新 provider，应仍可解析）
go test ./modules/member/... -count=1             # 含 member/cmd（oauth2.FxModule 直引路径已改）
go test ./... 2>&1 | grep -v '^ok' | grep -v 'no test files'   # 预期无 FAIL
```

若 `fx.ValidateApp` 报重复 provider → Step 1/2 忘做或没删干净，按报错定位补齐；若报缺依赖 → 检查嵌套 `oauth2.FxModule`/`infra.FxModule` 是否写进 `fx.Module("system", …)` 内。

- [ ] **Step 4: 验收断言（Review Focus #2/#5）**

```bash
# ① 主入口不再单挂两子模块
grep -n 'oauth2.FxModule\|infra.FxModule' alexgo-server/cmd/main.go          # 预期 0 行
grep -n 'system.FxModule\|order.FxModule\|member.FxModule' alexgo-server/cmd/main.go  # 预期 ≥3 行
# ② system.FxModule 已内嵌
grep -n 'oauth2.FxModule\|infra.FxModule' modules/system/module.go           # 预期各 1 行
# ③ member 进程只挂 oauth2 子入口、不 import system root
grep -n 'oauth2.FxModule' modules/member/cmd/main.go                         # 预期 1 行
grep -n '"alexGo-cloud/modules/system"' modules/member/cmd/main.go           # 预期 0 行（精确串，不含 oauth2 子路径）
# ④ 子 FxModule 仍导出（member/测试消费）
grep -n 'var FxModule' modules/system/oauth2/module.go modules/system/infra/module.go  # 预期各 1 行
```

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor(system): FxModule 并入——oauth2/infra 随 system 整体装配（方案 B）"
```

---

### Task 4: 文档全量同步（路径 sed + 枚举的口径改写）

**Files:**
- Modify: `README.md`（3 处路径）、`docs/modules.md`（5 处路径 + §3 口径行）、`docs/oauth2.md`（1 处路径）、`docs/architecture.md`（§21/§46 口径行，无裸路径）
- Modify: `docs/superpowers/plans/`：`2026-10-10-codegen-m1-engine.md`(1)、`2026-10-11-codegen-m2-infra.md`(63+import)、`2026-10-11-unified-login.md`(123+import)、`2026-10-12-codegen-m3-online.md`(44+import，**4 行产物引用受保护**)、`2026-10-14-codegen-m5-tree.md`(1)
- Modify: `docs/superpowers/specs/`：`2026-10-10-codegen-redesign-design.md`(8)、`2026-10-11-unified-login-design.md`(4)
- **禁改**: `docs/superpowers/plans/2026-10-15-oauth2-infra-into-system.md`（本计划）、`docs/superpowers/specs/2026-10-15-oauth2-infra-into-system-design.md`（spec）

**Interfaces:**
- Consumes: Task 1-3 完成后的最终路径
- Produces: 文档与代码零漂移；残留审计表（Step 6）

- [ ] **Step 1: 保护 m3 计划中 4 行 codegen 产物引用（行为未变，禁止改写）**

```bash
M3=docs/superpowers/plans/2026-10-12-codegen-m3-online.md
sed -i '' \
  -e 's|modules/infra/model/order_item.go|@@PROD_ORDER_ITEM@@|g' \
  -e 's|"modules/infra/"|"@@PROD_PREFIX@@"|g' \
  -e 's|want modules/infra/ 前缀|want @@PROD_PREFIX@@ 前缀|g' \
  "$M3"
grep -c '@@PROD' "$M3"   # 预期 4
```

- [ ] **Step 2: 全量路径 sed（排除本计划与 spec）**

```bash
grep -rl 'modules/oauth2\|modules/infra' README.md docs --include='*.md' \
  | grep -v '2026-10-15-oauth2-infra' \
  | xargs sed -i '' -e 's|modules/oauth2|modules/system/oauth2|g' -e 's|modules/infra|modules/system/infra|g'
```

- [ ] **Step 3: 还原 m3 保护的 4 行**

```bash
sed -i '' \
  -e 's|@@PROD_ORDER_ITEM@@|modules/infra/model/order_item.go|g' \
  -e 's|"@@PROD_PREFIX@@"|"modules/infra/"|g' \
  -e 's|want @@PROD_PREFIX@@ 前缀|want modules/infra/ 前缀|g' \
  "$M3"
grep -rn '@@PROD' README.md docs ; echo "预期 0（无占位符残留）"   # 必须无输出
```

- [ ] **Step 4: 手工口径改写（3 处，sed 覆盖不到的叙述行）**

① `docs/modules.md:3`：

```markdown
<!-- 改前 -->
> 覆盖全部 5 个后端业务模块（system / infra / member / oauth2 / order）的**功能、权限、后端端点、前端页面**，以及横切的认证、RBAC、租户体系。
<!-- 改后 -->
> 覆盖后端业务子系统（顶层 3 模块：**system 含 oauth2、infra 两个子模块**，另有 member、order；功能口径仍按 system / infra / member / oauth2 / order 划分）的**功能、权限、后端端点、前端页面**，以及横切的认证、RBAC、租户体系。
```

② `docs/architecture.md` 第 21 行 `modules/` 行：

```markdown
<!-- 改前（节选） -->
| `modules/` | 业务模块，当前有 `system`（用户/角色/菜单/字典/部门/公告/审计/租户等）、`infra`（代码生成在线链路）、`oauth2`（SSO 授权码+PKCE、客户端管理）、`order`（示例业务）与 `member`（会员注册/登录，micro 形态独立成 `cmd/` 入口）；内部按 `api / controller / model / repository / service` 分层 |
<!-- 改后 -->
| `modules/` | 业务模块，顶层为 `system`、`order`、`member`；其中 `system`（用户/角色/菜单/字典/部门/公告/审计/租户等）**内含 `oauth2`（SSO 授权码+PKCE、客户端管理，`modules/system/oauth2`）与 `infra`（代码生成在线链路，`modules/system/infra`）两个子模块**；`member`（会员注册/登录，micro 形态独立成 `cmd/` 入口）；内部按 `api / controller / model / repository / service` 分层 |
```

③ `docs/architecture.md` 第 46 行「模块聚合」：

```markdown
<!-- 改前 -->
4. **模块聚合**：`oauth2.FxModule`、`infra.FxModule`、`system.FxModule`、`order.FxModule` 通过
<!-- 改后 -->
4. **模块聚合**：`system.FxModule`（内嵌 `oauth2.FxModule`、`infra.FxModule`，2026-10-15 起并入）、`order.FxModule` 通过
```

**口径规则**：描述「进程装了哪些模块」的句子（如「单进程 oauth2+infra+system+order+member」）**保持原文**——子模块名仍是进程内容的准确描述；只改「FxModule 装配清单」「顶层模块数量」这类结构性表述。

- [ ] **Step 5: 格式化无关，直接文档审计**

```bash
grep -rn 'modules/oauth2\|modules/infra' README.md docs --include='*.md' \
  | grep -v 'modules/system/' | grep -v '2026-10-15-oauth2-infra' | tee /tmp/residual_md.txt
wc -l < /tmp/residual_md.txt    # 预期 4 —— 必须恰为 m3 的 360/361/627/636 四行

# 反向审计：产物路径被误转成新路径（保护失效/他处漏保护）——必须 0
grep -rn 'modules/system/infra/model/order_item.go\|want modules/system/infra/' README.md docs --include='*.md' | wc -l   # 0
```

若 residual 非 4 行：多出的行是漏保护的产物引用或漏改的位置引用，逐行判定（`order_item`/`HasPrefix`/`want …前缀` 属产物→补保护重跑；其余属位置→手工改）。若反向审计非 0：同理，找到该行恢复原路径。

- [ ] **Step 6: 全文回归**

```bash
go build ./... && go vet ./...          # 文档改动不影响编译，兜底
grep -rn '覆盖全部 5 个后端业务模块' docs ; echo 预期无输出
grep -n 'oauth2.FxModule`、`infra.FxModule`、`system.FxModule' docs/architecture.md ; echo 预期无输出
```

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "docs: oauth2/infra 并入 system 的路径与口径同步（含历史 plan/spec）"
```

---

### Task 5: 终验审计（预期无代码改动）

**Files:**
- 无新增修改（若审计发现问题，修复后再次全跑）

**Interfaces:**
- Consumes: Task 1-4 全部产出
- Produces: 交付判定（全绿 + 残留表符合预期）

- [ ] **Step 1: 全量构建与测试**

```bash
go build ./...
go vet ./...
go vet -tags integration ./...          # 含 integration 测试包整体可编译
go test ./... -count=1 2>&1 | tee /tmp/final_test.txt | grep -v '^ok' | grep -v 'no test files'
grep -c FAIL /tmp/final_test.txt        # 必须 0；skip 逐条目视环境如实记录
```

- [ ] **Step 2: 残留审计表（全部必须命中预期值）**

```bash
# ① 旧 import 前缀：0
grep -rn 'alexGo-cloud/modules/oauth2\|alexGo-cloud/modules/infra' --include='*.go' . | wc -l        # 0
# ② go 裸路径：恰 5 行（codeg 产物清单）
grep -rn 'modules/oauth2\|modules/infra' --include='*.go' . | grep -v 'modules/system/' | wc -l       # 5
# ③ md 裸路径：恰 4 行（m3 产物引用）且不含 2026-10-15 两个文件
grep -rn 'modules/oauth2\|modules/infra' README.md docs --include='*.md' | grep -v 'modules/system/' | grep -v '2026-10-15-oauth2-infra' | wc -l   # 4
# ④ spec/计划文件未被 sed 污染：仍含原始路径表述
grep -c 'git mv modules/oauth2' docs/superpowers/specs/2026-10-15-oauth2-infra-into-system-design.md   # ≥1
# ⑤ 迁移不变量
grep -rn 'GetModuleName' modules/system/oauth2/migration_source.go modules/system/infra/migration_source.go  # "oauth2" / "infra"
# ⑥ 格式
gofmt -l .    # 空
# ⑦ 记录表名出现在文档中的口径未被误改
grep -rn 'schema_migrations_oauth2\|schema_migrations_infra' docs --include='*.md' | head   # 仍为原名
```

- [ ] **Step 3: DB 可用时的迁移回归（不可用则如实记 skip）**

```bash
# 有 MySQL 时逐个真跑（记录表不变、不重跑已迁版本）：
DB_DSN='...' go test ./modules/system/oauth2/migrations/ ./modules/system/infra/migrations/ -count=1 -v
DB_DSN='...' go test -tags integration ./modules/system/oauth2/ -count=1 -v
```

- [ ] **Step 4: 收尾**

```bash
git status --porcelain    # 预期干净；若有审计修复则提交：
# git add -A && git commit -m "fix: 终审计修复（如有）"
```
