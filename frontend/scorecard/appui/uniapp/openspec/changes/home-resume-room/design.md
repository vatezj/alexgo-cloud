# Design: home-resume-room

## Context

首页回房现状（见 proposal.md Why）：仅有冷启动自动回房 `resumeLastRoom`（once 语义、静默），无手动入口。`authStore.lastRoomId` 已持久化且在 createRoom / joinRoom / scene 进房时写入。校验房况用的 `snapshot(rid)` API 已存在。首页布局：身份牌 → fan-cta（扫码进房/我要开房）→ fan-quick pills。

## Goals / Non-Goals

- Goals：首页在 `isLoggedIn && lastRoomId>0` 时展示「回到牌局」条；点击校验房况后进房或清理+提示；覆盖同会话 ⌂ 滞留与冷启动静默失败两场景。
- Non-Goals：不改 `resumeTried` once 语义（防拽回循环是有意设计）；不改后端契约；不新增独立组件（改动收敛在 index.vue 单文件）；不动 scene 优先级。

## Decisions

1. **入口位置**：fan-cta 上方插一条全宽「回到牌局」条（`v-if="isLoggedIn && authStore.lastRoomId > 0"`），复用主题 token（`--brass` 边框 + `--ink-700` 底）与全局 `.num`，风格对齐 fan-quick pill。备选「并入 fan-cta 第三瓣」被否：扇面三瓣布局固定，插入第三动作破坏视觉对稿；独立条更显眼且易做显隐。

2. **独立处理函数 `onResumeRoom`，不复用 `resumeLastRoom` 的静默 catch**：自动回房保持静默（安静路径不打扰），手动点击必须有反馈——30101/30102 清 id + toast；已结束清 id + toast「牌局已结束」；网络错误保留 id + toast 可重试。两者共享 `snapshot` 校验与 `status===0` 判断即可，不强行抽公共函数（逻辑各 ~15 行，抽反而绕）。

3. **防重复**：`resuming` ref 门禁点击（与 `creating`/`joining` 同模式）；进入房间前 `resumeTried = true` 不必要——push 后离开首页，但置位可防回跳残留。

4. **资料门**：点击路径同样走 `gate.ensureProfileThen`（与开房/进局四路径一致的 reminder 软提醒；资料完整立即执行）。备选「跳过门」被否：进房动作语义一致，且 reminder 已是软门不硬拦。

5. **未登录**：入口条随 `isLoggedIn` 隐藏（无 token 调 snapshot 会被 401 清态）。

## Risks / Trade-offs

- [lastRoomId 指向的房在别的设备被退/结束] → 点击时 snapshot 校验，30101/30102/已结束均清 id + toast，入口条消失。
- [点击进房后房间页 onLoad 再次拉快照] → 可接受的双请求；不额外传 store 快照（snapshot 响应本就进不了房间页 store，房间页自己拉）。
- [与自动回房竞态（冷启动自动回房在途时用户手点）] → `resuming` 与 `resumeTried` 互不干涉；即便双 push 同一房间，房间页有 roomId query 幂等，风险低。

## Migration Plan

纯前端增量，无数据迁移。回滚 = revert 单文件改动。

## Verification

- `pnpm lint` / `pnpm type-check` 通过。
- H5 手动：登录进房 → ⌂ 回首页 → 入口条出现 → 点击回房；改 store 中 lastRoomId 为已结束房 → 点击 toast + 清条；登出 → 条消失。

## Open Questions

（无）
