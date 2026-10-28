# Tasks: transfer-owner

## 1. API 封装

- [ ] 1.1 `src/api/scorecard/room.ts`：新增 `transferOwner(roomId: number, targetUserId: number): Promise<void>`——`scoreSend('POST', /rooms/:id/transfer-owner, { target_user_id })`；注释标注 30105/30104/30103/30107/30108/30113
- [ ] 1.2 验证：`pnpm type-check` 通过（调用点随任务 2 出现）

## 2. 头像弹窗入口 + 选人 + 确认

- [ ] 2.1 `AvatarPopup.vue`：computed `isOwner`（roomStore.snapshot 存在、`creator_id === me.user_id`、`status === 0`）；「操作」区在 `isOwner` 时渲染「转让房主」按钮（ghost 样式，disabled=acting）
- [ ] 2.2 同文件：二级 `wd-popup` 成员选择层——候选 `members.filter(m => m.user_id !== me.user_id)` 横排 chips（昵称/首字兜底）；点选触发二次确认 `dialog.confirm`（标题「转让房主」，msg 含目标昵称 +「转让后你将不再是房主」）
- [ ] 2.3 同文件：确认后 `acting=true` → `transferOwner(roomId, target)` → `snapshot(roomId)` → `applySnapshot` → 关闭两层弹窗 → toast「已转让给 X」；取消/失败走既有 toastApiError 与 acting 复位
- [ ] 2.4 验证：`pnpm lint`、`pnpm type-check` 全绿

## 3. 闭环验收

- [ ] 3.1 手验：房主见入口→选人→确认→筹码轨房主角标切换、原房主入口消失、type=9 流水出现
- [ ] 3.2 手验：非房主/已结算无入口；取消不调接口；重复点击防重
- [ ] 3.3 变更说明附到提交（OpenSpec 归档另走 verify/archive）
