# Design: create-room-return-active

## Context

`service.roomService.Create`（modules/scorecard/service/room.go:110）在建房前置检查中：`member.GetActiveRoomID(userID) != 0` → 返回 30114。`GetActiveRoomID` 口径 = 本人成员行 ∩ `room.status=0`（不看 expire，与一人一局既有约束一致）。前端 `doCreateRoom` 成功路径 = `lastRoomId = snap.id` → push 房间页，对响应内容无分支。前端未硬拦 30114（无任何 30114 引用）。动机见 proposal.md Why。

## Goals / Non-Goals

**Goals:**
- Create 的活跃局分支改为返回既有局快照（200、信封与新建同构），前端零改动进入该局。
- 复用分支绝不创建新房间/新流水（只读返回）。
- 明确 Join 的 30114 不受影响，错误码语义收窄注释同步。

**Non-Goals:**
- 不改 Join / 同房幂等重进的任何行为。
- 不改 `GetActiveRoomID` 的口径（expire 不在本变更讨论）。
- 不为复用分支引入响应新字段或新接口。
- 不改前端开房交互（资料门、配置默认全 0）。

## Decisions

1. **service 层就地返回快照，而非 controller 层前置分支**：Create 内已有 `activeRoomID` 与 `s.buildSnapshot(ctx, s.db, activeRoomID, userID, false)` 所需全部上下文，一行替换错误分支；备选 controller 先查活跃局被否——泄漏一人一局规则到 HTTP 层，且 controller 拿不到 member repo 接口。

2. **不加 `reused: true` 之类区分字段**：前端成功路径对两种情形行为完全一致（进房），加字段扩大契约且无人消费；备选「返回 30114 + body 携带房 id」被否——半错半对的信封违背统一 resp 语义，前端还得为「成功错误」写分支。

3. **入参校验保留在活跃局检查之前**：非法参数（tea 越界等）仍先 40002——不因「反正要复用」放行脏入参，保持入参契约稳定。自定义房号冲突检查（30106）在活跃局之后：有局直接复用，房号 B 占没占与本次请求无关（spec「自定义房号入参被忽略」）。

4. **30114 仅收窄至 Join**：错误码与文案保留（进他房仍需用）；`pkg/errors/code.go` 注释由「开房/进房拦截」改「进房拦截」，Create 的 swagger @Description 补一句复用语义。备选「彻底删除 30114」被否——Join 仍依赖。

5. **并发窗口维持现状**：先查后插的双开竞态注释已声明 best-effort 接受，本变更不引入锁/唯一约束（复用分支只读，不扩大竞态面）。

6. **多活跃局排序 asc→desc（验证期修正）**：手动验证发现——双后端共享远程库期间旧后端会绕过一人一局建出多间进行中局；`GetActiveRoomID` 原 `ORDER BY id asc` 取最早，复用会把用户带回最老残留局（而非刚开的局）。改为 `id desc` 取最新。成员行 ∩ `status=0` 的口径不变（Non-Goal「不改口径」仅指 expire/成员判定）；干净数据（单活跃局）下无行为差异。

## Risks / Trade-offs

- [依赖 30114 做开房拦截的外部调用方行为破坏] → 仓内外 grep 确认仅前端消费且未硬拦；接口语义变化已在 proposal 标注 BREAKING。
- [用户点「我要开房」却进入旧局，感知「没响应」] → 返回局与新建同构直接进房页，房号即既有局，视觉上等价于「回到了我的局」，正是需求意图；若未来要提示可在前端按入参比对房号区分。
- [有局 + 自定义房号用户期待开第二局] → 一人一局是既定产品约束（Join 同样拦截），复用是该约束下的唯一正向出路。

## Migration Plan

纯后端行为变更，无数据迁移。回滚 = revert service/room.go 分支与测试期望（前端本就无改动）。

## Verification

- TDD：`room_test.go` 既有「再开房应 30114」改期望为「返回既有局快照、房数不变」→ 红 → 实现 → 绿；`go test ./modules/scorecard/...` 全量。
- Join 既有 30114 测试（room_test.go:355/379）必须保持通过——防误伤边界。
- `go build ./modules/scorecard/...`、前端 `pnpm lint` / `pnpm type-check`。
- 手动：重启服务后带同款 curl body 调 POST /rooms → 200 返回进行中局快照；前端点「我要开房」直接进入该局。

## Open Questions

（无）
