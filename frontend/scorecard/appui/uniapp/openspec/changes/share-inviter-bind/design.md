# Design: share-inviter-bind

## Context

现状（详见 proposal.md Why）：scene 进房只认 `qr=<房号>`，由 `App.vue` onShow 解析后写入 `useSceneRoom` 的模块级内存 ref；`login/index.vue` afterLogin 与 `index/index.vue` consumePendingScene 两处消费；好友只能 `POST /friends` 手输 user_id。后端小程序码 scene 拼装由同日后端计划扩展为 `qr=<房号>&inv=<请求者>`（协议契约一致，本变更不改 Go 代码）。

约束：微信 `onShow` 可能连续触发（消费必须只读一次）；微信会回收后台进程（内存态会丢）；`addFriend` 现有错误码 30109 已是好友 / 30108 拉黑 / 10001 参数非法（自加）；主端为 MP-WEIXIN，H5 无原生转发。

## Goals / Non-Goals

**Goals:**

- 三条入口（首页转发 / 房间转发 / 小程序码）统一走同一 scene 协议与同一暂存结构。
- 暂存落盘，进程被杀后冷启可续。
- 绑好友全静默、与进房/登录收口互不阻塞。

**Non-Goals:**

- 不做自定义转发海报、邀请统计、防刷风控（好友可删/拉黑，产品已接受）。
- 不做通用 returnUrl 登录回跳（scene 仍是唯一特例）。
- 不改后端接口、不新增 API；不为 H5 做分享 UI。

## Decisions

1. **解析器收进 `useSceneRoom.ts`**：新增纯函数 `parseScene(raw: string): { roomCode: string | null, inviterId: number | null } | null`，按 `&` 切段解析 `qr=` / `inv=`，兼容无 `inv` 段旧码；`App.vue` 只负责取 raw（`query.scene` 兜底 `options.scene`）与 decode，然后调 `setPendingScene(parseScene(...))`。备选：解析散在 App.vue——否决，登录/首页消费方需要同一解析语义测试面。
2. **暂存结构 `{ roomCode, inviterId }` + storage 键 `scorecard:pendingScene`**：`setPendingScene` 同时写内存 ref 与 `uni.setStorageSync`；`consumeScene()` 读后清内存并 `uni.removeStorageSync`。备选：只靠 storage——否决，onShow 高频路径读 storage 无谓开销；只靠内存——即现状缺陷。
3. **绑好友抽共享助手 `bindInviterSilently(inviterId, myId)`**（同文件导出）：`inviterId === myId` 直接 return；否则 `try { await addFriend(inviterId) } catch {}` 吞掉全部错误（含网络）。两个消费点（afterLogin / consumePendingScene）都调它。备选：各消费点内联 try/catch——易漂移。
4. **消费点不合并页面流**：`login` 与 `index` 各自保留进房编排（资料门、lastRoomId、toast 差异已存在），只在消费处改调「取暂存 → 静默绑定 → 有房号才 join」。备选：抽统一 `consumeAndEnter()`——会牵动两页资料门/回首页语义，超出本变更。
5. **首页转发用页面钩子而非按钮**：`index/index.vue` 增加 `onShareAppMessage` / `onShareTimeline`，path 由 `authStore.user?.id` 拼 `inv`；未登录（无 user.id）不带 `inv`。页面内分享按钮属 UI 增项，不做（胶囊菜单已可达）。房间页仅改既有 `onShareAppMessage` path 与 QrInvitePanel 出码 path 的拼接，补 `&inv=`。

## Risks / Trade-offs

- [storage 残留旧暂存] → 每次新场景覆盖写；消费即删；无 TTL（未登录滞留可被下一次扫码覆盖，语义可接受）。
- [静默绑好友被刷] → 接受（proposal Impact）；好友侧已有删除/拉黑兜底。
- [inv 解析到非法值] → `parseScene` 对非数字/≤0 的 inv 一律置 null（只进房不绑）。
- [与后端计划上线时序] → 前端解析兼容「无 inv 段」旧码与旧后端出码；后端先/后上均不破。

## Migration Plan

纯前端增量，无数据迁移。发布即生效：旧小程序码（无 inv）行为不变；新码需后端 `wxacode` inv 计划同时或先行上线才有 `&inv=` 内容。

## Verification

- `pnpm lint`、`pnpm type-check` 全绿。
- `parseScene` 为纯函数，行为由任务清单里的用例表驱动核对（三协议+非法值）。
- MP-WEIXIN 真机/开发者工具手验：转发首页→新号打开→登录后互见好友；扫码进房→杀进程→重进仍直达房间；已是好友/自扫无弹错。

## Open Questions

（无——转发文案沿用首页既有调性「登录后开房记账 · 好友互通」/ 房间页现状文案，不另立文案需求。）
