# 整体规划（Roadmap）

> 对标 [yudao-cloud](https://github.com/YunaiV/yudao-cloud) 的能力面，按「P0 收尾欠账 → P1 常用管理能力 → P2 深水区」推进。
> 现状基线：见 [模块功能与权限手册](modules.md)（2026-10-13 全量盘点）。
> 约定：**直接落 main、不建功能分支、不 push（等用户决定）**；每完成一项把状态改为 ✅ 并注明提交号。

---

## 状态总览

| 层 | 主题 | 完成度 |
| --- | --- | --- |
| P0 | 基座开箱短板（收尾欠账） | 4/4 ✅ 全清（codegen M3/M7、通知已读、M4 跟进票） |
| P1 | 常用管理能力 | 4/5 |
| P2 | 深水区（可缓） | 0/5 |
| 工程收尾 | 零散隐患 | 2/3 ✅ |

---

## P0 — 基座开箱短板（先做，工作量小、缺口扎眼）

| # | 项 | 内容 | 状态 |
| --- | --- | --- | --- |
| P0-1 | **通知已读 / 我的通知** | 现只有公告 CRUD（`system:notice`），无 read 状态、无"我的通知"页。补：已读回执表 + 我的未读/已读接口 + 我的通知页面（菜单种子） | ✅ 1956a4b |
| P0-2 | **codegen M7 收尾** | `make codegen-smoke` 绿；删 3 套旧工具 + `scripts/templates/`；README/architecture 指向 `tools/codegen` | ✅ `138b92c` |
| P0-3 | **M4 跟进票**（M3 终审挂出） | ① size=200 截断 → 分页拉全；② 预览 loading；③ blob 下载错误 toast 退化 → Blob 特判；④ CLI nil reader → nopReader | ✅ `bdb24a4` |
| P0-4 | ~~codegen M3 在线链路~~ | 预览/ZIP API + 后台页面 + CLI 改读配置表 | ✅ `4ed021a..0d0327b`（9 笔，终审通过） |
| — | ~~租户管理页面~~ | 种子菜单 + 前端 API + 页面 | ✅ 已在 baseline |

## P1 — 常用管理能力（yudao 有、我们没有）

| # | 项 | 内容 | 依赖 |
| --- | --- | --- | --- |
| P1-1 | **文件存储** | `pkg/storage`：本地盘驱动 + 统一上传 API（`/api/admin/infra/file`）。解锁：公告附件、头像、codegen 导入产物、Excel 导出落地 | ✅ |
| P1-2 | **Excel 导入导出** | 用户/会员/订单列表导出（export 落地：`pkg/excel` + 三域 `/export` 端点 + 前端两按钮；导入留小票） | ✅ |
| P1-3 | **在线用户 + 代登录** | 在线会话列表（查 `system_oauth2_access_token`）、踢人（Revoke）、impersonate 端点；对照 yudao 在线用户页 | ✅ `fbaebd4` |
| P1-4 | **API 文档** | swagger 注解 + 聚合页（或从 gin routes 生成）；减少 `docs/usage.md` 手维护漂移 | 无 |
| P1-5 | **HTTP 基座规范化** | 统一信封 {code,msg,data} + 五位错误码 + Paginate[T] + DateTime + AccessLog/Recovery（spec：`docs/superpowers/specs/2026-10-15-http-foundation-design.md`） | ✅ |

> **P1-5 遗留项**：✅ 已清（`9a5377e`）——codegen 模板/golden 迁统一信封（`resp.OK/Fail` + id/bind 走 `errs.CodeParamInvalid`，动作端点 `resp.OK(ctx,nil)` + 前端 `Promise<void>`），生成产物过 go build/vet + service 测试 + vue-tsc + vitest 门禁。**M6 前置门禁解除**。

## P2 — 深水区（可缓，按需启动）

| # | 项 | 内容 | 状态 |
| --- | --- | --- | --- |
| P2-1 | codegen M4→M5→M6 | 前端列表模板 → 树表 → 主子表（spec：`docs/superpowers/specs/2026-10-10-codegen-redesign-design.md`） | M4 ✅ `a932b56..20330df`（7 模板 + E2E + 终审），M5 ✅ `c016a1f..3465434`（树表族 overlay 5 模板 + 约定校验 + 冒烟 42），当前 M6 |
| P2-1-F | **codegen M4 跟进票**（M4 终审挂出，7 minor；⑤ 已随信封迁移修） | ① form.vue `editingId` 在 async resetForm 后赋值，极快点确定可能走 create（竞态）；② list/form/drawer 模板 `.Comment` 未转义撇号（注释含 `'` 则 JS 语法错）；③ generate_test 残留正则漏 `{{fn}}`/`{{ $x }}` 形态；④ smoke cleanup 不删空目录（遗留 `codegen_smoke_*` 空目录）；⑤ ~~create/update 返回类型声明 `Promise<Entity>` 实返 `{status:ok}`（类型说谎）~~ ✅ `9a5377e` 改 `Promise<void>`；⑥ `int/uint64→number` >2^53 雪花 ID 精度丢失（与有符号主键局限同票，`docs/usage.md` §2.8「局限票」指此）；⑦ preview 验收不断言 front 产物存在（golden 别处兜底） | ⬜ 不阻塞 |
| P2-2 | member 深化 | 实名认证、会员等级、积分、签到 |
| P2-3 | 社交登录 | 微信/QQ OAuth 联邦到统一登录（account_type 扩展） |
| P2-4 | 定时任务可视化 | 任务表 + 触发器 + 执行日志页（对照 yudao Job） |
| P2-5 | 消息中心三件套 | 站内信（per-user message）、短信、邮件——公告之外的通知渠道 |
| **跳过** | ~~网关 / 支付 / ES~~ | 与单体基座定位无关（既定决策，不排期） |

## 工程收尾（零散）

| # | 项 | 状态 |
| --- | --- | --- |
| E-1 | 工作区改动未提交 | ✅ 已清 |
| E-2 | `.playwright-mcp/` 进 `.gitignore` | ✅ 已加 |
| E-3 | **`make lint` node 版本卡点**：package.json 要 `^22.18.0 \|\| ^24.0.0`，本机 v22.14 | ⬜ 升 node 或放宽 engines |

---

## 执行约定

- **顺序**：P0 按 1→2→3 严格推进；P0 清完再进 P1（P1 内五项无依赖，可任选）；P2 按需。
- **流程**：每项先写实现计划（`docs/superpowers/plans/`）→ 执行 → 测试绿（`CGO_ENABLED=0 go test ./...` + `pnpm run typecheck`）→ 单独提交落 main。
- **本文档**：每完成一项更新状态列与提交号；新发现的缺口追加到对应层，不在会话中散失。
