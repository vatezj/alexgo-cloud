# 前端目录收敛设计（frontend/）

日期：2026-11-11  
状态：待批准

## Intent / Why

仓库根目录目前是「Go 后端 monorepo + vue-vben-admin monorepo」咬合生长：根上一半是 `go.mod`/`Makefile`/`alexgo-server`，另一半是 `package.json`/`apps`/`packages`/`turbo`/一堆 lint 配置；小程序埋在 `modules/scorecard/appui/`，Remotion 视频在根上 `promo-video/`。打开仓库第一眼分不清前后端。

目标：**根目录一眼是后端**；所有前端收进 `frontend/`，内部按模块分目录。三个前端子目录**完全独立**（不共享 workspace、不共享根 `package.json`）。

## Goals

1. 根目录只保留后端与工程文档/部署物；**第一层有白名单**（见 Root hygiene），清单外一律不出现。
2. `frontend/` 下三个平级模块目录（`admin` / `scorecard/appui` / `promo-video`），各自独立安装与构建；`frontend/` 根不堆散文件。
3. 后端编译、测试、CI、Docker 路径行为不变（除 codegen 输出路径前缀，见下）。
4. 用 `git mv` 保留历史；一笔结构提交 + 必要的引用修正提交。
5. **迁移收尾目录干净整洁**：无空壳残留、无迁移中间产物、无本地垃圾入库。

## Non-goals / Anti-goals

- 不合并三个前端为一个 pnpm workspace。
- 不改任何业务代码、路由、OpenSpec 语义、页面文案。
- 不重写 `docs/superpowers/plans|specs` 历史文档里的旧路径（只改「活文档」：README + `docs/*.md` 非 superpowers）。
- 不动 Go 模块布局（`modules/` `pkg/` `alexgo-server/`）。
- 不顺手升级依赖、不清理 promo-video 的未跟踪大文件内容（只搬目录）。

## Constraints

- 用户裁定：后端源码不 import 前端；前端三目录独立。
- **例外（已核实）**：代码生成器硬编码前端输出路径——`pkg/codegen/template/paths.go`、`tools/newmodule`、相关测试写/断言 `apps/web-antd/...`。这不是运行时 import，但是路径契约，必须随迁移改前缀。
- `findMonorepoRoot()`（`internal/node-utils`）向上找 `pnpm-lock.yaml`——lock 与 monorepo 同迁后，从 `frontend/admin/**` 启动仍能解析根。
- CI（`.github/workflows/*`）只跑 Go + helm + docker，不引用前端路径；Docker `COPY . .` context 是仓库根，`.dockerignore` 已排除 `node_modules`/`*.md`。
- `scripts/` 所有权分裂：一半是后端 shell/py，一半是 vben 的 workspace 包与 node 工具。

## Target layout

```
alexGo-cloud/                    # 后端根
├── alexgo-server/  modules/  pkg/  tools/
├── go.mod  go.sum  Makefile
├── scripts/                     # 仅后端脚本（见下表拆分）
├── deployments/  docs/  data/  bin/
├── .github/  README.md  LICENSE
│
└── frontend/
    ├── admin/                   # 整个 vben monorepo（独立 workspace）
    │   ├── apps/  packages/  internal/
    │   ├── scripts/             # check-node-version / clean.mjs / turbo-run / vsh
    │   ├── package.json  pnpm-workspace.yaml  pnpm-lock.yaml  turbo.json
    │   ├── eslint|oxfmt|oxlint|stylelint|cspell 配置
    │   ├── .npmrc  .node-version  .browserslistrc  .stylelintignore
    │   └── vben-admin.code-workspace  .gitpod.yml
    ├── scorecard/
    │   └── appui/             # 原 modules/scorecard/appui（uniapp + desgin）
    │       ├── uniapp/          # 小程序（自带 pnpm/openspec）
    │       └── desgin/
    └── promo-video/             # Remotion（原根上未跟踪目录）
```

`frontend/` 根**不**放 `package.json`（用户裁定：不用独立开）。

### scripts/ 拆分

| 去向 | 内容 |
|------|------|
| 留根 `scripts/` | `gen_proto.sh` `gen-full-sql.sh` `swagger_inject_taggroups.py` `load_test.js` `deploy/` `.keep` |
| 迁 `frontend/admin/scripts/` | `check-node-version.mjs` `check-node-version.test.mjs` `clean.mjs` `turbo-run/` `vsh/` |

根 `package.json` 若已迁走，根上不再有 `pnpm lint`；后端 `make lint` 仍是 golangci，不受影响。

### 根上迁入 `frontend/admin/` 的文件

`package.json` `pnpm-workspace.yaml` `pnpm-lock.yaml` `turbo.json`  
`eslint.config.mjs` `oxfmt.config.ts` `oxlint.config.ts` `stylelint.config.mjs` `cspell.json`  
`.browserslistrc` `.stylelintignore` `.npmrc` `.node-version` `.gitpod.yml`  
`vben-admin.code-workspace`  
以及目录：`apps/` `packages/` `internal/`（internal 无 .go，全是 vben 配置包）

根 `node_modules/`（1.8G，未跟踪）`mv` 到 `frontend/admin/node_modules`，避免重装。

### 保留根上的（易误判）

- `.gitignore` `.gitattributes` `.editorconfig` `.gitconfig` `.github/` `.vscode/` —— 仓库级，根 `.gitignore` 已含通用 `node_modules`/`dist` 规则，三处 node_modules 皆被覆盖。
- `tea.yaml` `LICENSE` `README.md` `Makefile` `go.*` `deployments/` `docs/` `tools/` `alexgo-server/` `modules/` `pkg/` `scripts/`（后端半边）。
- `.claude/` `.ccto/` `.playwright-mcp/` `.turbo/`（根上 `.turbo` 缓存可删可留，建议迁 admin 后删根缓存）。

## Codegen / newmodule 路径契约

前缀 `apps/` → `frontend/admin/apps/`：

| 位置 | 改动 |
|------|------|
| `pkg/codegen/template/paths.go` | 9 处 front 模板输出模式 |
| `pkg/codegen/generate_test.go` | 断言/fixture 路径 |
| `tools/newmodule/*_test.go` `report.go` | 脚手架产物路径与回滚文案 |
| `modules/system/infra/service/acceptance_test.go` | preview 前缀断言 |

不引入配置项——路径硬编码收敛为常量 `frontend/admin/apps/web-antd`（或保持字面量但全局替换），与「仓库根 = go.mod 根」的 `repoRootDir` 假设一致。

生成器**写文件**的目标目录必须在迁移后真实存在（先 `git mv` 再改路径）。

## 活文档与命令

更新（非 superpowers 历史）：

- `README.md`：目录树、`pnpm install`/`pnpm --filter @vben/web-antd dev` 前加 `cd frontend/admin`
- `docs/architecture.md` `docs/usage.md` `docs/modules.md` `docs/oauth2.md` `docs/oauth2-compat-retirement.md`：`apps/web-antd` → `frontend/admin/apps/web-antd`；开发命令工作目录说明
- `docs/roadmap.md` 若有 `pnpm run typecheck` 描述则补 `cd frontend/admin`
- uniapp 项目内 CLAUDE/README：**不动**（相对路径，自包含）

命令约定（写进 README）：

```bash
# 后端
make run | make test | make build

# 管理后台
cd frontend/admin && pnpm install && pnpm dev:antd

# 小程序
cd frontend/scorecard/appui/uniapp && pnpm install && pnpm dev:h5   # 或 build:mp-weixin

# 推广视频
cd frontend/promo-video && npm i && npm run dev
```

## Migration steps（执行序）

1. **搬 vben monorepo**：`mkdir -p frontend/admin`；`git mv` 上表文件与 `apps packages internal`；`git mv scripts/{check-node-version*,clean.mjs,turbo-run,vsh} frontend/admin/scripts/`；`mv node_modules frontend/admin/`（未跟踪）。
2. **搬 appui**：`git mv modules/scorecard/appui frontend/scorecard/appui`（含 uniapp node_modules 随目录 mv；330 个跟踪文件走 git mv）。
3. **搬 promo-video**：`mv promo-video frontend/promo-video`（未跟踪，不进 git mv）。
4. **改 codegen/newmodule/测试** 路径前缀；跑 `go test ./pkg/codegen/... ./tools/... ./modules/system/infra/...`。
5. **改活文档**（上表）。
6. **Root hygiene**：删根 `.turbo`/`.stylelintcache`/`dump.rdb`（ignore 补 `dump.rdb`）；`find` 清空壳；对照白名单。
7. **验证**：
   - 根：`go test ./...`、`make build`、`golangci-lint`（与 CI 同款）
   - `cd frontend/admin && pnpm install && pnpm --filter @vben/web-antd typecheck`（或 `pnpm build:antd` 视耗时）
   - `cd frontend/scorecard/appui/uniapp && pnpm type-check && pnpm build:h5`（已有门禁同款）
   - `openspec list` 在 uniapp 目录仍正常（planningHome 跟 cwd）
8. **提交**：建议两笔——`refactor(repo): 收敛前端至 frontend/`（纯移动 + hygiene）+ `refactor(codegen): 前端输出路径前缀 frontend/admin`（含文档）。也可一笔，消息里分段说明。

## Root hygiene（干净整洁 · 硬验收）

### 迁移后根目录第一层白名单

**允许出现（仅此）：**

```
隐藏：.git .github .gitignore .gitattributes .gitconfig .editorconfig
      .dockerignore .vscode .claude .ccto .superpowers .gitpod.yml   # 工具/IDE 本地态
可见：alexgo-server/  modules/  pkg/  tools/  scripts/  deployments/
      docs/  data/  bin/
      go.mod  go.sum  Makefile  README.md  LICENSE  tea.yaml
      frontend/
```

**禁止出现在根（迁移后 `ls -1A` 对照，0 命中）：**

```
apps/  packages/  internal/  node_modules/
package.json  pnpm-lock.yaml  pnpm-workspace.yaml  turbo.json
eslint.config.mjs  oxfmt.config.ts  oxlint.config.ts  stylelint.config.mjs  cspell.json
.browserslistrc  .stylelintignore  .npmrc  .node-version
vben-admin.code-workspace  promo-video/  dump.rdb
.turbo/  .stylelintcache/  .DS_Store  *.zip（根级）
```

### 清理动作（随迁移同一笔或紧随的 hygiene 提交）

| 项 | 动作 |
|----|------|
| 根 `.turbo/` `.stylelintcache/` | 删除（vben 缓存，已随 monorepo 失去宿主） |
| `dump.rdb` | 删除 + `.gitignore` 增加 `dump.rdb`（Redis 本地落盘，从未该入库） |
| 根 `node_modules/` | `mv` 进 `frontend/admin/`，根不留 |
| 迁移后空目录 | `find . -type d -empty` 排查；`modules/scorecard` 去掉 appui 后不得留空 `appui` 壳 |
| `frontend/` 根 | 只允许三个子目录，不放 README/杂项（各模块说明写在各自目录内） |
| 各模块内部 | 不本次大扫除依赖树；只要求**无跨模块散落文件**（文件都在自己模块下） |
| 未跟踪本地物 | `data/uploads/`、`zhiwei/归档.zip` 等不迁不提交；若在根则确认仍被 ignore 或留原位（`data/` 在白名单） |

### 验收命令

```bash
# 1) 根第一层 == 白名单（人工 ls 对照）
ls -1A

# 2) 禁止项 0 命中
for x in apps packages internal node_modules package.json turbo.json promo-video dump.rdb; do
  [ -e "$x" ] && echo "FAIL: $x still at root"
done

# 3) 无空壳（排除 .git 与 node_modules）
find . -type d -empty -not -path './.git/*' -not -path '*/node_modules/*'

# 4) frontend 根只有三个子目录
ls -1 frontend   # → admin  scorecard  promo-video
```

## Risks

| 风险 | 缓解 |
|------|------|
| 漏改 codegen 路径 → 生成文件落错地方 | 迁移后全量 `grep -r "apps/web-antd"`（排除 superpowers 历史与 node_modules）必须 0 命中 |
| IDE/脚本仍从根跑 `pnpm` | README 根部加「前端已迁 frontend/」指引；根上无 package.json 后 pnpm 自然失败，不会静默错 |
| promo-video 未跟踪、体积大 | 不 git mv；确认本地 mv 成功即可；`.gitignore` 若将来要跟踪再单独处理 |
| 大目录 `git mv` 触发权限/耗时 | appui 仅 330 跟踪文件，node_modules 走文件系统 mv |
| CI docker context 变大 | context 本就 `COPY . .`；可选后续把 `frontend/` 加入 `.dockerignore`（本次可做可不做，默认做，减小 context） |
| 历史 plan 文档路径过时 | 明确 non-goal，不改 |

## Decisions（已拍板）

1. 三子目录独立，`frontend/` 根不建 workspace。
2. 后端不 import 前端；codegen 输出路径前缀属必须改的路径契约。
3. 小程序从 `modules/scorecard/appui` 迁到 `frontend/scorecard/appui`（Go scorecard 不引用它，已核实）。
4. `scripts/` 按所有权拆分，不整目录搬。

## Open for builder（实现时可自行定）

- codegen 前缀用字面量全替换，还是抽 `const WebAntdRoot = "frontend/admin/apps/web-antd"`——默认**抽常量**（paths.go 一处，测试可 import 或重复字面量）。
- `.dockerignore` 是否加 `frontend/`——默认**加**（Go 镜像不需要前端源码；若将来要多阶段构建前端再撤）。
- 根 `.turbo` 缓存目录——默认**删除**（迁 admin 后无用）。

## Success criteria

- **根目录白名单验收全过**（Root hygiene 三条命令 0 FAIL、空目录 0、`frontend/` 仅三子目录）。
- 仓库根 `ls` 第一层：后端目录 + 单一 `frontend/`，无 `apps` `packages` `package.json` `turbo.json`。
- `frontend/{admin,scorecard/appui,promo-video}` 各自可独立 install/build。
- `grep -r "apps/web-antd" --exclude-dir=node_modules --exclude-dir=superpowers` 仅允许出现在 `frontend/admin/...` 与有意保留的历史 superpowers 文档。
- CI 同款 Go 门禁绿；admin typecheck 绿；scorecard/appui type-check + build:h5 绿。
- 根上无迁移残留（禁止单列：旧 vben 配置、`promo-video`、`dump.rdb`、空 `appui` 壳）。
