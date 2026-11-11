# Proposal: share-inviter-bind

## Why

好友目前只能手输 user_id 添加，拉新靠口口相传；首页也没有任何转发入口。同时「扫码进房、登录后直达房间」的 scene 暂存只存在内存——微信回收进程后房号丢失，用户登录完被丢回首页。需要把「转发/扫码」两条入口都带上邀请人身份，打开即自动进好友排名，并让场景参数在进程被杀后仍可续用。

## What Changes

- 首页新增 `onShareAppMessage` / `onShareTimeline`：胶囊菜单与转发卡片 path 统一带 `inv=<我的 user_id>`，落在首页。
- scene 协议扩展并统一解析：`qr=<房号>`（旧码兼容）、`qr=<房号>&inv=<邀请人ID>`（扫码/房间转发进房+绑好友）、`inv=<邀请人ID>`（首页转发只绑好友）。
- `pendingScene` 从「纯内存房号字符串」升级为 `{ roomCode, inviterId }` 结构并**落盘 storage**（进程被杀后冷启仍可续上）。
- 登录成功后 / 已登录态消费场景时，自动调用现有 `POST /friends` 静默绑定邀请人为好友（已是好友 30109、被拉黑 30108、自加 10001 一律静默吞掉不打扰）；有房号则照旧 join 进房，无房号停首页。
- 房间页既有转发（`onShareAppMessage` 与牌背小程序码 path）同步补 `inv=<我的 user_id>`。
- H5 与微信小程序差异：转发/小程序码/朋友圈仅微信小程序端生效；H5 端无 `onShareAppMessage` 场景，scene 解析逻辑共享但无原生转发入口（维持现状降级，不新增 H5 分享 UI）。
- 后端出码 scene 拼装 `inv` 由同日后端计划（`docs/superpowers/plans/2026-10-28-wxacode-inv-scene.md`）承接，协议与本变更一致；本变更不改后端 Go 代码。

## Capabilities

### New Capabilities

- `share-inviter-bind`: 首页转发、scene 邀请解析（含旧码兼容）、pendingScene 持久化、打开即静默绑好友进好友排名。

### Modified Capabilities

（无——既有 `expense-tea-only` / `room-ledger-layout` 的需求不受影响；scene 进房此前无独立 spec，本变更将其与邀请绑定一并立项。）

## Impact

- 代码：`src/App.vue`（onShow 解析）、`src/composables/scorecard/useSceneRoom.ts`（结构+落盘）、`src/pages/index/index.vue`（转发钩子+消费绑定）、`src/pages/room/index.vue`（转发 path 补 inv）、`src/pages/login/index.vue`（afterLogin 绑好友）；复用 `src/api/scorecard/friend.ts` 的 `addFriend`，不新增接口。
- 数据/契约：scene 字符串协议扩展（`&inv=` 分段）；storage 新增/扩展 pendingScene 键。
- 行为风险：静默绑好友可能被用于刷好友——接受（好友可单向删除/拉黑，榜单互相可见是产品意图）；`addFriend` 失败不得阻塞进房或登录收口。
