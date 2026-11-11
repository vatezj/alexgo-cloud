# Proposal: create-room-return-active

## Why

用户已有进行中牌局时点「我要开房」，后端 `POST /rooms` 返回 30114「你还在其他牌局中，请先退出」，前端 toast 后停留在首页——用户既没开成房也没回到原牌局。实际意图几乎总是「回到我在的那局」：应当直接返回该进行中牌局，前端顺势进入，而不是报错打断。

## What Changes

- 后端开房 `service.roomService.Create`：本人已有进行中牌局（`GetActiveRoomID != 0`）时，不再返回 30114，改为直接返回该局的房间快照（HTTP 200、信封 data 与新建成功完全同构）。
- 请求中的开房配置（tea_percent / max_lose_score / 自定义 room_code 等）在该分支被忽略——以已有局为准。
- 后端进房 `Join` 的 30114 拦截**保持不变**（进他房仍需先退原局，语义不同，不在本变更范围）。
- 前端 `doCreateRoom` 成功路径（`lastRoomId = snap.id` → push 房间页）无需改动即自动进入已有局；仅随注释同步。
- 30114 错误码语义收窄为「进房拦截」，错误码表注释与 swagger 描述同步。

## Capabilities

### New Capabilities

- `room-create`: 开房入口行为——已有进行中牌局时返回该局而非报错、无局时正常创建、请求配置在复用分支的取舍、与进房（Join）一人一局拦截的边界。

### Modified Capabilities

（无——`openspec/specs/` 无既有开房规范。）

## Impact

- 后端（alexGo-cloud 同仓，monorepo 内跨目录，不在 uniapp allowedEditRoots 但属本 git 仓）：
  - `modules/scorecard/service/room.go`（Create 的 30114 分支 → buildSnapshot 返回已有局）
  - `modules/scorecard/service/room_test.go`（既有「再开房应 30114」测试改期望 + 新增不重复建房断言）
  - `modules/scorecard/controller/app/app.go`（CreateRoom swagger @Description）
  - `pkg/errors/code.go`（CodeScorecardInOtherRoom 注释改为仅进房拦截）
- 前端 `src/pages/index/index.vue`：行为零改动（成功即进房），注释同步说明复用分支。
- 契约变化：`POST /api/app/scorecard/rooms` 在「已有进行中局」输入下从 400/30114 变为 200 + 既有局快照（**BREAKING** 于依赖 30114 做拦截的调用方——仓内无此调用方，前端未硬拦 30114）。
- 跨端：H5 与微信小程序行为一致（纯接口响应变化，无端差异逻辑）。
- 验收：`go test ./modules/scorecard/...` 全绿；前端 lint/type-check 通过；开发者工具实测开房返回已有局并进入。
