# Tasks: share-card-config

## 1. 配置拉取与缓存

- [x] 1.1 `src/api/scorecard/` 新增 sharecfg API：`getShareConfig()` 调 `GET /api/public/scorecard/share-config`（`scoreGet` 复用信封解包），类型定义 ShareCards（index/simulator/salary → title/imageUrl）
- [x] 1.2 新增 `src/store/scorecard/shareConfig.ts`：内存 cards + `get(page)` 三级回退（内存 → storage `scorecard:shareCards` → 内置 DEFAULTS，DEFAULTS 与服务端 sharecfg.go 同值）+ `ensureFresh()` 拉取写缓存、在途去重、失败静默不清缓存
- [x] 1.3 `App.vue` 启动后调 `ensureFresh()`（fire-and-forget，不 await 阻塞）

## 2. 首页分享改读配置

- [x] 2.1 `src/pages/index/index.vue`：删除硬编码 `SHARE_TITLE`/imageUrl 常量，`onShareAppMessage`/`onShareTimeline` 改读 store getter；`inv=` 拼接与未登录不带 inv 分支原样保留；onShow 调 `ensureFresh()` 轻刷新

## 3. 两新页分享钩子

- [x] 3.1 `src/pages/fun/compass/simulator.vue`：新增 `onShareAppMessage`（title=store.simulator.title，imageUrl 非空才传，path 落自身）+ `onShareTimeline` + onShow `ensureFresh()`
- [x] 3.2 `src/pages/fun/salary/index.vue`：同 3.1，读 salary 配置，path 落自身

## 4. 验证

- [x] 4.1 `pnpm lint` 与 `pnpm type-check` 通过（uniapp 目录，Node v22 PATH）
- [x] 4.2 spec 场景核对：默认回退（断网转发 title=硬编码默认）、后台改 title 后拉取生效、inv 语义不回归、imageUrl 空串不传字段——逐条对照 `specs/share-card-config/spec.md` 的 7 个 Scenario
- [x] 4.3 `openspec validate --change share-card-config` 通过
