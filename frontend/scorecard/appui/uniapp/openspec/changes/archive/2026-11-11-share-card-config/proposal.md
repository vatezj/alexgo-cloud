# Proposal: share-card-config

## Why

分享 title 与分享图目前是小程序端硬编码常量（首页 `SHARE_TITLE` + 七牛 logo URL 写死在 `index.vue`），改一句文案就要改代码、重新发版；座位模拟器与工资记账两页甚至没有转发入口。后台已经能把分享卡配置存进 `system_configs`（`scorecard.share.*`，服务端 `GET /api/public/scorecard/share-config` 已落），小程序端需要消费这份配置，让运营在管理后台随时替换三页的分享标题与图片。

## What Changes

- 新增分享配置拉取：`GET /api/public/scorecard/share-config`（免鉴权、统一信封），返回 `{index, simulator, salary} → {title, imageUrl}`。
- 拉取结果缓存到 storage（键 `scorecard:shareCards`），带内存缓存；拉取失败静默回退内置默认（与现首页硬编码一致），不阻塞任何页面流程。
- 首页 `onShareAppMessage` / `onShareTimeline` 的 title、imageUrl 改读缓存配置；`inv=` 好友绑定逻辑保持不动。
- 座位模拟器（`pages/fun/compass/simulator`）与工资记账（`pages/fun/salary/index`）**新增** `onShareAppMessage` / `onShareTimeline`，path 落各自页面。
- 三页分享统一在 onShow 时后台刷新配置（不 await，不阻塞渲染）。
- `imageUrl` 为空串时不传 `imageUrl` 字段（微信回退当前页截图）。
- H5 与微信小程序差异：`onShareAppMessage` / `onShareTimeline` 仅微信小程序端生效；H5 无原生转发入口，配置拉取与缓存逻辑共享但不产生可分享行为（维持现状，不新增 H5 分享 UI）。

## Capabilities

### New Capabilities

- `share-card-config`: 三页（首页/座位模拟器/工资记账）分享卡配置的拉取、缓存、回退与分享钩子消费契约。

### Modified Capabilities

（无——`home` spec 只约束信息架构与请求面，不含分享钩子细节；分享行为此前由 `share-inviter-bind` 变更承载，其 `inv` 绑定语义在本变更中原样保留、不改需求。）

## Impact

- 代码：`src/api/scorecard/`（新增 sharecfg API）、`src/store/scorecard/`（新增分享配置 store）或 composable、`src/pages/index/index.vue`（分享钩子改读配置）、`src/pages/fun/compass/simulator.vue` 与 `src/pages/fun/salary/index.vue`（新增分享钩子）。
- 契约：依赖后端 `GET /api/public/scorecard/share-config`（同仓库 Go 侧已实现，免 Bearer）；storage 新增键 `scorecard:shareCards`。
- 风险：配置接口挂掉时分享回退默认文案——与硬编码时代行为等价，不劣化；错误静默（不上 toast，不打扰转发）。
