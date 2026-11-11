# Tasks: create-room-return-active

## 1. 测试先行（红）

- [x] 1.1 `modules/scorecard/service/room_test.go`：既有「进行中牌局未结束再开房应 30114」（room_test.go:337 附近）改期望——Create 返回 200 语义的既有局快照（err 为 nil、`snap.ID`/`room_code` 等于既有局），并断言未创建新房间（房表计数不变）；跑测试确认红

## 2. 后端实现（绿）

- [x] 2.1 `modules/scorecard/service/room.go` `Create`：`activeRoomID != 0` 分支由 `errs.New(CodeScorecardInOtherRoom)` 改为 `return s.buildSnapshot(ctx, s.db, activeRoomID, userID, false)`，注释说明复用语义与「微信/前端零改动进入既有局」，入参校验仍在该检查之前
- [x] 2.2 注释与文档同步：`pkg/errors/code.go` 的 `CodeScorecardInOtherRoom` 注释收窄为「进房拦截」（CreateRoom godoc/`@Description` 补「已有进行中牌局返回该局快照（200）」——swagger 注释改动即可，`make swagger` 生成物是否重跑视仓库惯例）

## 3. 前端同步（零行为改动）

- [x] 3.1 `src/pages/index/index.vue`：头部 CTA 注释与 `doCreateRoom` 注释补一句「已有进行中牌局时接口直接返回该局快照，进房逻辑复用（lastRoomId+push 不变）」——确认成功路径确实无分支需要改

## 4. 验证

- [x] 4.1 `go build ./modules/scorecard/... && go test ./modules/scorecard/... -count=1` 全绿（Join 的 30114 既有测试必须保持通过）
- [x] 4.2 uniapp：`pnpm lint` 与 `pnpm type-check` 通过
- [ ] 4.3 手动：重启本地服务后用同款 curl body 调 `POST /api/app/scorecard/rooms` → 200 返回进行中局快照（房号为既有局）；前端小程序点「我要开房」直接进入该局；无局用户开房仍新建
- [x] 4.4 验证期修正：`GetActiveRoomID` 排序 asc→desc（多活跃局脏数据下复用取最新局；`room_test.go` TestCreate_ReusedPrefersLatestActiveRoom TDD 钉死；design.md 决策 6）
