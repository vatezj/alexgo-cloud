# Design: transfer-owner

## Context

后端 `POST /api/app/scorecard/rooms/:id/transfer-owner` 已落地（service/settle.go TransferOwner）：仅房主 30105、status=0 门禁、同人 30107、目标不在房 30113、黑名单 30108；成功后两行 is_creator 互换 + type=9 流水。前端缺 API 封装与 UI 入口。头像弹窗（AvatarPopup）是「我的」入口，房主判定已有先例：room 页 `snap.creator_id === myUserId && status === 0`。

约束：转让不可撤，须二次确认；接口返回空 data，成功后需重拉快照（或等轮询）刷新房主标记；成员选择层须排除自己（同人后端 30107）。

## Goals / Non-Goals

**Goals:**

- 房主在头像弹窗一键发起转让：选成员 → 二次确认 → 成功后房主标记即时切换。
- 复用既有后端接口与房间快照轮询，不新增后端字段。

**Non-Goals:**

- 不做多级房主/管理员、不做转让撤销、不做转让历史页（type=9 流水已在 ledger）。
- 不改后端错误码与接口形状。

## Decisions

1. **入口放 AvatarPopup「操作」区**：在「退出房间」旁仅当 `isOwner`（快照 creator_id===me 且 status===0）时展示「转让房主」ghost 按钮。备选：成员头像长按——不可发现，否决。
2. **选人用二级 wd-popup（成员横排 chips）**：候选 `members.filter(m => m.user_id !== myUserId)`；点选即进入二次确认（dialog.confirm 带昵称），不设「选中再确认按钮」两步——减少误触层级。备选：整屏选人页——超需求。
3. **成功后强制重拉快照**：`transferOwner` 返回 void，成功即 `snapshot(roomId)` → `roomStore.applySnapshot`；失败 toastApiError 不动快照。轮询兜底仍保留。备选：只靠轮询——房主标记切换延迟可感，否决。
4. **API 封装在 `api/scorecard/room.ts`**：`transferOwner(roomId, targetUserId)` → POST body `{ target_user_id }`，与 leave/settle 并列，注释标注错误码。
5. **防重**：`acting` ref 复用退出房间链路；选人层打开期间 acting=true，成功/失败后复位。

## Risks / Trade-offs

- [误转让不可撤] → 二次确认 + 昵称回显；产品已接受（后端同口径）。
- [快照拉取失败但转让已成功] → toast 成功后快照失败仅记日志级静默；下一轮轮询补齐（version 门保证不覆盖）。
- [黑名单目标在选人层仍可见] → 点选后后端 30108 拦截 toast；不在前端预过滤（黑名单数据不在房上下文）。

## Migration Plan

纯前端增量。发布即生效；旧版本小程序无此入口无回归。

## Verification

- `pnpm lint`、`pnpm type-check` 全绿。
- 手验：房主见入口→选成员→确认→房主标记切换、自己入口消失；非房主无入口；取消不调接口；已结算无入口。

## Open Questions

（无。）
