# Frontend Directory Convergence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 根目录收敛为纯后端仓库；全部前端收进 `frontend/{admin,scorecard/appui,promo-video}`，路径契约与活文档同步，根目录白名单验收通过。

**Architecture:** 用 `git mv` 整体下沉 vben monorepo 到 `frontend/admin/`；`modules/scorecard/appui` → `frontend/scorecard/appui`；根上未跟踪的 `promo-video` 文件系统 `mv`。codegen/newmodule 的 `apps/web-antd/` 输出前缀改为 `frontend/admin/apps/web-antd/`（抽常量）。`scripts/` 按所有权拆分。收尾做根目录 hygiene 并按白名单验收。

**Tech Stack:** Go 1.25（codegen/newmodule 测试）、pnpm monorepo（vben admin）、uni-app（appui）、git mv。

**Spec:** `docs/superpowers/specs/2026-11-11-frontend-directory-design.md`

## Global Constraints

- 用户裁定：`frontend/` 根不放 `package.json`；三子目录完全独立 workspace。
- 小程序路径：`frontend/scorecard/appui`（保留模块名，与后端 `modules/scorecard` 对称）。
- 活文档只改：`README.md` + `docs/*.md` 中非 `superpowers` 路径；**不改** `docs/superpowers/plans|specs` 历史。
- 不改业务代码、路由、OpenSpec 语义、页面文案；不动 `modules/` `pkg/` `alexgo-server/` Go 布局。
- codegen 输出前缀常量：`WebAntdRoot = "frontend/admin/apps/web-antd"`（repo 根 = go.mod 根）。
- `.dockerignore` 追加一行 `frontend/`。
- 提交直接落 main（仓库约定，不建功能分支）；建议两笔：纯移动+hygiene，再 codegen+文档。
- 禁止把 `dump.rdb`、`data/uploads`、`zhiwei/归档.zip` 纳入提交。

## Review Focus

1. **codegen 写错目录**：迁移后生成器仍写 `apps/web-antd/`（根上已无此目录）→ 任务 4 后必须 `go test ./pkg/codegen/... ./tools/newmodule/...` 且全仓 `grep -rn "apps/web-antd" --include="*.go"` 仅允许 `frontend/admin/apps` 字面量前缀出现。
2. **git mv 半途失败**：大目录/跨设备导致部分文件移动 → 每步移动后立刻 `git status` 确认无残留旧路径；失败则 `git checkout -- .` + `git clean` 针对性回滚再重试。
3. **pnpm workspace 断裂**：`pnpm-lock.yaml` 与 `package.json` 不同迁或 `scripts/*` 包丢失 → 任务 1 结束后 `cd frontend/admin && pnpm install --frozen-lockfile` 必须成功。
4. **文档命令仍指向根**：README/docs 里的 `pnpm --filter @vben/web-antd` 未加 `cd frontend/admin` → 任务 5 后 grep 活文档 0 处裸跑根 pnpm。
5. **根白名单漏项**：`.gitpod.yml`/`.npmrc` 等已迁但根仍留副本，或 hygiene 漏删 `dump.rdb` → 任务 6 验收命令全过才算完成。

---

### Task 1: 迁移 vben monorepo → frontend/admin

**Files:**
- Move: 根 `package.json` `pnpm-workspace.yaml` `pnpm-lock.yaml` `turbo.json` `eslint.config.mjs` `oxfmt.config.ts` `oxlint.config.ts` `stylelint.config.mjs` `cspell.json` `.browserslistrc` `.stylelintignore` `.npmrc` `.node-version` `.gitpod.yml` `vben-admin.code-workspace` `apps/` `packages/` `internal/`
- Move: `scripts/check-node-version.mjs` `scripts/check-node-version.test.mjs` `scripts/clean.mjs` `scripts/turbo-run/` `scripts/vsh/` → `frontend/admin/scripts/`
- Move (untracked): 根 `node_modules/` → `frontend/admin/node_modules/`

**Interfaces:**
- Produces: `frontend/admin/` 为完整 vben workspace 根（后续 Task 4 路径、Task 5 文档都依赖此布局）；根 `scripts/` 只剩后端脚本。

- [ ] **Step 1: 创建目标目录并 git mv 跟踪文件**

```bash
mkdir -p frontend/admin/scripts
git mv package.json pnpm-workspace.yaml pnpm-lock.yaml turbo.json \
  eslint.config.mjs oxfmt.config.ts oxlint.config.ts stylelint.config.mjs \
  cspell.json .browserslistrc .stylelintignore .npmrc .node-version \
  .gitpod.yml vben-admin.code-workspace \
  apps packages internal \
  frontend/admin/
git mv scripts/check-node-version.mjs scripts/check-node-version.test.mjs \
  scripts/clean.mjs scripts/turbo-run scripts/vsh \
  frontend/admin/scripts/
```

- [ ] **Step 2: 校验 git 状态无根上 vben 残留**

Run: `git status --short | grep -E '^[MADR].* (package.json|apps/|packages/|internal/|turbo.json)' || true`  
Expected: 只有 rename 指向 `frontend/admin/...`，无停留在根的 `apps/` 等 path

- [ ] **Step 3: 文件系统 mv 未跟踪 node_modules 与 turbo-run/vsh 内嵌 node_modules**

```bash
# 根 node_modules（若 Step1 的 git mv 已带走跟踪部分，剩余未跟踪在根）
[ -d node_modules ] && mv node_modules frontend/admin/node_modules
# scripts 子包内 node_modules 随 git mv 目录应已走；若根还有残留：
[ -d scripts/node_modules ] && mv scripts/node_modules frontend/admin/scripts/node_modules
```

Expected: 根无 `node_modules/`；`frontend/admin/node_modules/.pnpm` 存在

- [ ] **Step 4: 根 scripts 只剩后端**

Run: `ls scripts/`  
Expected: 仅 `deploy/ gen-full-sql.sh gen_proto.sh load_test.js swagger_inject_taggroups.py .keep`（可有 `.keep`）

- [ ] **Step 5: workspace 安装自检**

Run: `cd frontend/admin && pnpm install --frozen-lockfile`  
Expected: exit 0（lock 与 workspace 一致）

- [ ] **Step 6: Commit（纯移动第一笔可与 Task 2–3 合并提交，见 Task 7）**

本任务可不单独提交；若需中断保护可先 `git add -A && git commit -m "wip: move vben monorepo under frontend/admin"`，最终由 Task 7 整理提交信息。

---

### Task 2: 迁移 appui → frontend/scorecard/appui

**Files:**
- Move: `modules/scorecard/appui/`（330 个跟踪文件 + 其下 uniapp/desgin 全部，含未跟踪 node_modules/dist）

**Interfaces:**
- Produces: `frontend/scorecard/appui/{uniapp,desgin}`；`modules/scorecard/` 不再含前端。

- [ ] **Step 1: mkdir + git mv 整目录**

```bash
mkdir -p frontend/scorecard
git mv modules/scorecard/appui frontend/scorecard/appui
```

- [ ] **Step 2: 确认无空壳与跟踪残留**

Run: `test ! -e modules/scorecard/appui && git status --short | grep appui | head`  
Expected: `test` 成功；status 中 appui 路径均为 `frontend/scorecard/appui/...` rename

- [ ] **Step 3: 打开 openspec 仍可用（路径跟 cwd）**

Run: `cd frontend/scorecard/appui/uniapp && openspec list`  
Expected: 正常列出 changes（fun-compass 等），无 root 解析错误

---

### Task 3: 迁移 promo-video + 根 hygiene

**Files:**
- Move (untracked): `promo-video/` → `frontend/promo-video/`
- Delete: 根 `.turbo/` `.stylelintcache/` `dump.rdb` `.DS_Store`（根级）
- Modify: `.gitignore`（追加 `dump.rdb`）
- Modify: `.dockerignore`（追加 `frontend/`）

**Interfaces:**
- Produces: `frontend/` 仅 `admin` `scorecard` `promo-video` 三子目录；根白名单可验收。

- [ ] **Step 1: mv promo-video**

```bash
mv promo-video frontend/promo-video
```

- [ ] **Step 2: 删本地垃圾 + ignore**

```bash
rm -rf .turbo .stylelintcache dump.rdb .DS_Store
# .gitignore 追加一行 dump.rdb（在文件合适分组）
# .dockerignore 追加一行 frontend/
```

- [ ] **Step 3: 清空壳**

Run: `find . -type d -empty -not -path './.git/*' -not -path '*/node_modules/*' -not -path './frontend/*'`  
Expected: 无输出（`frontend/` 内允许短暂空目录则一并 `rmdir`；`modules/scorecard` 无 `appui` 壳）

- [ ] **Step 4: frontend 根三子目录**

Run: `ls -1 frontend`  
Expected: 恰为 `admin` `promo-video` `scorecard`（顺序任意）

- [ ] **Step 5: 禁止项循环**

```bash
for x in apps packages internal node_modules package.json pnpm-lock.yaml \
  turbo.json promo-video dump.rdb eslint.config.mjs cspell.json \
  vben-admin.code-workspace .npmrc .node-version; do
  [ -e "$x" ] && echo "FAIL: $x still at root"
done
```

Expected: 无 `FAIL` 行

---

### Task 4: codegen / newmodule 路径前缀

**Files:**
- Modify: `pkg/codegen/template/paths.go`（9 处 front 模板 + 新增导出常量）
- Modify: `pkg/codegen/generate_test.go`（约 6 处断言/日志）
- Modify: `tools/newmodule/report.go:39`（回滚文案）
- Modify: `tools/newmodule/scaffold_test.go` `entity_test.go` `report_test.go`（路径断言；`scaffold_test.go:182` 的 `strings.HasPrefix(c.Path, "apps/")` → `frontend/admin/apps/`）
- Modify: `modules/system/infra/service/acceptance_test.go:126-129`（preview 前缀）

**Interfaces:**
- Produces: `template.WebAntdRoot = "frontend/admin/apps/web-antd"`；所有 front 输出模式以该常量为前缀（或测试内相同字面量）。仓库根仍由 `repoRootDir`（向上找 `go.mod`）解析。

- [ ] **Step 1: 在 paths.go 增加常量**

在 `pkg/codegen/template/paths.go` 包级声明：

```go
// WebAntdRoot 相对仓库根（go.mod 所在）的管理后台前端根路径。
const WebAntdRoot = "frontend/admin/apps/web-antd"
```

- [ ] **Step 2: 替换 9 处模板输出模式**

将 `"apps/web-antd/src/..."` 改为 `WebAntdRoot + "/src/..."`（fmt.Sprintf 或常量拼接，与文件内现有风格一致）。  
若 map literal 不便引用 const，允许直接写 `"frontend/admin/apps/web-antd/src/..."` 字面量——**全文件只许一种写法，禁止混用**。

- [ ] **Step 3: 全量替换测试与 newmodule/acceptance 中的 `apps/web-antd` → `frontend/admin/apps/web-antd`**

含：`generate_test.go`、`tools/newmodule/*_test.go`、`report.go`、`acceptance_test.go`。  
`scaffold_test.go` 前缀判断 `"apps/"` → `"frontend/admin/apps/"`。

- [ ] **Step 4: 跑受影响 Go 测试**

Run:
```bash
go test ./pkg/codegen/... ./tools/newmodule/... ./modules/system/infra/...
```
Expected: 全部 PASS

- [ ] **Step 5: 全仓 Go 面 grep 收口**

Run: `grep -rn "apps/web-antd" --include="*.go" . | grep -v "frontend/admin/apps/web-antd" | grep -v node_modules`  
Expected: 空输出

- [ ] **Step 6: Commit（第二笔主体）**

```bash
git add pkg/codegen tools/newmodule modules/system/infra
git commit -m "refactor(codegen): 前端输出路径前缀 frontend/admin/apps/web-antd"
```
（若 Task 1–3 尚未提交，先提交移动笔再提交本笔。）

---

### Task 5: 活文档同步

**Files:**
- Modify: `README.md`（目录树、前后端启动命令、Node 说明处补 `cd frontend/admin`）
- Modify: `docs/architecture.md` `docs/usage.md` `docs/modules.md` `docs/oauth2.md` `docs/oauth2-compat-retirement.md`（`apps/web-antd` → `frontend/admin/apps/web-antd`；`pnpm --filter` 前补 `cd frontend/admin`）
- 不改: `docs/superpowers/**`

**Interfaces:**
- Consumes: Task 1/2 的最终路径。
- Produces: 活文档 0 处旧根路径。

- [ ] **Step 1: 批量替换活文档路径字面量**

```bash
for f in README.md docs/architecture.md docs/usage.md docs/modules.md \
  docs/oauth2.md docs/oauth2-compat-retirement.md; do
  sed -i '' 's|apps/web-antd|frontend/admin/apps/web-antd|g' "$f"
done
```

注意：`docs/modules.md` 等若含锚点 `#8-前端总览appsweb-antd` 同类 slug，替换后检查标题锚点是否仍自洽（slug 由标题生成，标题内路径变了 slug 会变——同步改目录里指向该锚的链接）。

- [ ] **Step 2: README 启动命令补工作目录**

「后台前端」一节命令改为：

```bash
cd frontend/admin
pnpm install
pnpm --filter @vben/web-antd dev
```

项目结构树更新为设计稿 Target layout（根只留后端 + `frontend/`）。  
micro 联调行的 `MEMBER_PROXY=... pnpm --filter ...` 同样加 `cd frontend/admin &&`。

- [ ] **Step 3: 活文档 grep 验收**

```bash
grep -rn "apps/web-antd" README.md docs --include="*.md" \
  | grep -v superpowers | grep -v "frontend/admin/apps/web-antd"
```
Expected: 空（或仅剩有意保留的对比说明，须逐条人工确认）

- [ ] **Step 4: README 增加三端命令约定块**

按 Spec「命令约定」四段（后端 make / admin / appui / promo-video）写入 README 开发章节。

---

### Task 6: 全量门禁 + 白名单验收

**Files:**
- 无代码改动；只跑验证与修补（若 FAIL 回到对应 Task）。

- [ ] **Step 1: Go 门禁（CI 同款核心）**

```bash
go test ./... 
make build
```
Expected: test 全过；build 产出 `bin/alexgo-server`

- [ ] **Step 2: admin typecheck**

Run: `cd frontend/admin && pnpm --filter @vben/web-antd run typecheck`  
Expected: exit 0（若脚本名是 `typecheck` 以外，以 `apps/web-antd/package.json` scripts 为准）

- [ ] **Step 3: appui 门禁**

Run: `cd frontend/scorecard/appui/uniapp && pnpm type-check && pnpm build:h5`  
Expected: 双绿

- [ ] **Step 4: 根白名单四条验收（Spec Root hygiene）**

```bash
ls -1A
for x in apps packages internal node_modules package.json turbo.json promo-video dump.rdb; do
  [ -e "$x" ] && echo "FAIL: $x still at root"
done
find . -type d -empty -not -path './.git/*' -not -path '*/node_modules/*'
ls -1 frontend
```
Expected: 对照白名单人工确认；无 FAIL；空目录无输出；frontend 仅三子目录

- [ ] **Step 5: apps/web-antd 残留总 grep**

```bash
grep -rn "apps/web-antd" . \
  --exclude-dir=node_modules --exclude-dir=.git --exclude-dir=superpowers \
  2>/dev/null | grep -v "frontend/admin/apps/web-antd" | head
```
Expected: 空（promo-video/dist 等未跟踪产物若命中则 exclude 对应 dist）

---

### Task 7: 提交策略

**Files:**
- 无新文件；组织最终 commit。

- [ ] **Step 1: 若 Task 1–3 仍为未提交移动**

```bash
git add -A -- . ':!docs/superpowers' ':!alexgo-server/configs/config.yaml' ':!data/' ':!modules/scorecard/plugin/zhiwei/归档.zip'
git status --short  # 确认无 config.yaml/归档.zip/data
git commit -m "refactor(repo): 前端收敛至 frontend/{admin,scorecard,promo-video} + 根 hygiene

- vben monorepo 整体下沉 frontend/admin（含 scripts 拆分与 node_modules）
- modules/scorecard/appui → frontend/scorecard/appui
- promo-video → frontend/promo-video；删根 .turbo/.stylelintcache/dump.rdb
- .gitignore 补 dump.rdb；.dockerignore 排除 frontend/"
```

- [ ] **Step 2: 若 codegen+文档尚未提交（Task 4/5）**

```bash
git add pkg/codegen tools/newmodule modules/system/infra README.md docs
# 排除 superpowers：git add 已跟踪修改的 docs/*.md 即可；新 design/plan 文档单独：
git add docs/superpowers/specs/2026-11-11-frontend-directory-design.md \
        docs/superpowers/plans/2026-11-11-frontend-directory.md
git commit -m "refactor(codegen)+docs: 前端输出路径前缀与活文档同步 frontend/"
```

- [ ] **Step 3: 终态 status**

Run: `git status --short`  
Expected: 仅剩有意不提交项（`alexgo-server/configs/config.yaml` 本地密钥、`data/uploads`、`归档.zip` 等）

---

## Self-review notes

- Spec 的 Migration steps 1–8 与 Task 1–7 一一对应；Root hygiene 落在 Task 3/6；codegen 常量在 Task 4。
- 活文档列表与 Spec 相同；`docs/roadmap.md` 0 处 `apps/web-antd`，Step 中不强制改。
- 不引入配置项、不合并 workspace、不改 superpowers 历史——与 Non-goals 一致。
