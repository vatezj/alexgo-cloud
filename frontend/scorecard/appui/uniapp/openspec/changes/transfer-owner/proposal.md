# Proposal: transfer-owner

## Why

后端 `POST /rooms/:id/transfer-owner` 与流水 type=9 已落地，但小程序端没有任何入口：房主想把房主交给牌友只能退房重开。牌局进行中换房主是真实诉求（房主临时离开、轮流记账）。

## What Changes

- API 层新增 `transferOwner(roomId, targetUserId)` 封装（POST body `target_user_id`）。
- 房间页头像弹窗（AvatarPopup）在「我=房主且牌局进行中」时展示「转让房主」入口；点开选人层（房内成员横选，排除自己）；选中后二次确认 → 调接口 → 重拉快照刷新房主标记。
- 仅房主且 `status===0` 可见入口；非房主/已结算弹窗不展示该项（后端另有 30105/30104/30103 门禁兜底）。
- H5 与微信小程序行为一致（纯 API + 弹层，无端差异能力）。

## Capabilities

### New Capabilities

- `transfer-owner`: 房主在头像弹窗内选择房内成员转让房主，二次确认后生效并刷新快照。

### Modified Capabilities

（无——既有 room-ledger / AvatarPopup 需求不变，本变更为纯增量入口。）

## Impact

- 代码：`src/api/scorecard/room.ts`（+transferOwner）、`src/components/scorecard/AvatarPopup.vue`（入口+选人+确认）；房间页轮询/快照自动反映新房主（type=9 流水已在既有 ledger 展示）。
- 数据/契约：复用既有后端接口，不改 Go 代码、不新增字段。
- 行为风险：误点转让不可撤（房主身份即时转移）——二次确认兜底；黑名单/目标不在房由后端 30108/30113 拦截并 toast。
