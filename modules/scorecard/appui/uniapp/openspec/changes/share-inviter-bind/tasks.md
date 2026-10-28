# Tasks: share-inviter-bind

## 1. 场景协议与暂存（useSceneRoom + App.vue）

- [x] 1.1 `useSceneRoom.ts`：新增 `PendingScene = { roomCode: string | null, inviterId: number | null }` 与纯函数 `parseScene(raw: string): PendingScene | null`——按 `&` 分段解析 `qr=<房号>` / `qr=<房号>&inv=<uid>` / `inv=<uid>`；`inv` 非正整数时置 null；两段皆空返回 null
- [x] 1.2 同文件：`setPendingScene(p)` 写内存 ref + `uni.setStorageSync('scorecard:pendingScene', p)`；`consumeScene(): PendingScene | null` 读后清内存 + `removeStorageSync`；`restorePendingScene()` 在模块加载或 App onLaunch 时从 storage 恢复到内存 ref
- [x] 1.3 同文件：新增 `bindInviterSilently(inviterId: number | null, myId: number | null)`——相等或空值直接 return；否则调 `addFriend(inviterId)`，catch 全部错误静默（不 toast、不上抛）
- [x] 1.4 `App.vue` onShow：保留 raw 提取与 decode，改调 `parseScene` + `setPendingScene`（仅非 null 时写入；空/非法不覆盖已有暂存）；另兜底朋友圈 `query.inv`（timeline 直给 page query 不经 scene 键）
- [x] 1.5 验证：`pnpm lint`、`pnpm type-check` 通过；用例表手核 `parseScene`：`qr=7AVW` → 房号 only；`qr=7AVW&inv=100` → 两者；`inv=100` → 邀请人 only；`inv=abc` → null；空串 → null

## 2. 消费链路（login + index）

- [x] 2.1 `pages/index/index.vue` consumePendingScene：已登录路径消费 `PendingScene`，先 `bindInviterSilently(inviterId, authStore.user?.user_id ?? null)`，再按现有资料门 + join 流程处理 `roomCode`（roomCode 为空则不进房、返回 true 占用本启程意图）；未登录路径保持「留存 + 跳登录」
- [x] 2.2 `pages/login/index.vue` afterLogin：消费同一暂存结构，登录收口后先静默绑定、有房号再走现有资料门 + join；无房号 `replaceAll` 回首页（现状不变）
- [x] 2.3 验证：`pnpm lint`、`pnpm type-check`；确认两处消费均只读一次、storage 消费后清除（代码走查）

## 3. 转发入口（首页 + 房间页）

- [x] 3.1 `pages/index/index.vue`：新增 `onShareAppMessage`——已登录（`authStore.user?.user_id`）时 path `/pages/index/index?scene=${encodeURIComponent('inv=' + uid)}`，未登录不带 scene；title「登录后开房记账 · 好友互通」
- [x] 3.2 同文件：新增 `onShareTimeline`，`title` 同上，query 携带 `inv=<uid>`（未登录空串）
- [x] 3.3 `pages/room/index.vue` onShareAppMessage path：scene 由 `qr=<code>` 改为 `qr=<code>&inv=<myUid>`（无快照房号时维持现状回首页）
- [x] 3.4 `QrInvitePanel.vue`：转发走房间页钩子（`open-type="share"`），组件内无 path 拼接——N/A
- [x] 3.5 验证：`pnpm lint`、`pnpm type-check` 通过；MP-WEIXIN 开发者工具手验右上角胶囊转发 path 含 `inv`（真机/工具待手验）

## 4. 闭环验收（规格场景抽查）

- [ ] 4.1 手验 A：首页转发 → 另一未登录新号打开 → 登录 → 双方好友排名互见，落首页
- [ ] 4.2 手验 B：房间码扫码进房 → 登录后直达房间；中途杀进程重进，仍能恢复暂存（登录后进房 + 绑定邀请人）
- [ ] 4.3 手验 C：已是好友再打开转发、扫自己的码——均无弹错；进房失败时绑定结果不受影响
- [ ] 4.4 全量 `pnpm lint && pnpm type-check`（已过）；变更说明附到提交（OpenSpec 归档另走 verify/archive 流程）
