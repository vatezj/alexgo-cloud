# fishgame《摸鱼》游戏后端模块 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 alexGo-cloud 新增 fishgame 模块，一次性实现《摸鱼》微信小游戏契约 7 端点 + 3 新端点 + admin 五组 + 前端 5 页，图鉴条目后端可配、dex 双路记账、好友榜自存。

**Architecture:** 游戏客户端段 `/api/v1/*` 裸 JSON 直出（豁免统一信封，X-Session 自鉴权）；admin 段 `/api/admin/fishgame/*` 标准信封 + casbin。模块自持微信 code2session、X-Session 会话表、8 张业务表 + 48 条图鉴种子。确定性运势/鱼王自算兜底运营覆盖。

**Tech Stack:** Go + Gin + GORM + FX + MySQL；前端 vben (Vue3 + antd)；微信 sns/jscode2session。

**Spec:** `docs/superpowers/specs/2026-10-23-fishgame-module-design.md`
契约字段形状权威：`/Users/alex/Desktop/game/godot/fishgame/docs/backend-api.md`
玩法规则：`/Users/alex/Desktop/game/godot/fishgame/docs/game-overview.md`
48 条目数据源：`/Users/alex/Desktop/game/godot/fishgame/scripts/fish_db.gd` 的 `ENTRIES`（id 逐字抄录）

## Global Constraints

- 模块名 `fishgame`，目录 `modules/fishgame`，开关 `cfg.Modules["fishgame"]`（false 时 RegisterRoutes 直接 return）。
- 游戏客户端段挂 `/api/v1/*`（契约锁死路径，不可挪）；admin 段挂 `/api/admin/fishgame/*`。
- `/api/v1/**` 裸 JSON 直出（`c.JSON`/`c.Status(204)`，不走 `pkg/resp`）；错误也用裸 `{"error":"<英文短语>"}` + 401/400。
- `/api/admin/fishgame/**` 走 `pkg/resp` 统一信封 + casbin + `user_type=1` 门槛。
- 游戏身份 = openid（code2session 换取），自持 `X-Session` 随机 32B base64url 会话，**不复用** oauth2/member token。
- 错误码段 304xx（301/302/303 已占），仅 admin 段使用。
- 表全带 `tenant_id` + `created_at`/`updated_at`，走 GORM 租户插件；游戏端请求 ctx **不注入租户**，写路径显式 `tenant_id=0`。
- 响应快（正常 <500ms）、字段全（缺字段 = 客户端走本地 + backend_fail）。
- 微信 appid/secret 放 `config.yaml` 的 `fishgame.wechat.*`（模块自己的小游戏，与 oauth2 social 渠解耦）。
- 运营覆盖服务端自带确定性算法兜底，保证 `config/daily` 响应恒全。

## Review Focus（spec 隐含但无任务直接测试、最可能咬人的 5 类）

1. **`/api/v1` 裸 JSON 响应无信封**：任何 handler 误调 `resp.OK/Fail` 会包出 `{"code":0,...}`，客户端形状校验失败全部静默走本地——联调白做。Task 5-8 每个端点测试必须断言响应 JSON 顶层无 `code` 键。
2. **X-Session 中间件误挂 login**：`/api/v1/auth/login` 是唯一免鉴权端点；若中间件挂上 group 级，login 也 401，登录闭环断裂。Task 5 测试必须同时验证「login 无 session 可达」+「其余端点缺 session 401」。
3. **dex 双路合并取 max 而非 sum**：同一条鱼 catch 推导（+1）与 dex/report（整表 count）交集若相加会膨胀；dedup 命中时 catch 不得再递增。Task 7 测试必须覆盖「同 catch 重发 count 不变」+「report 大于 catch 时取 report」。
4. **确定性算法与客户端值域对齐**：服务端自算 `depth_bias` 若给到 >0.3 或 `period` 非四选一，客户端钳制后仍合法但语义漂移；鱼王 `fish_id` 若不校验存在性，客户端查表失败走本地。Task 6 测试必须断言值域 + fish_id 在图鉴表内。
5. **track 幂等 dedup_key 漏 canonical**：同一事件 props 键序不同会产生不同 dedup_key，客户端失败重发导致重复落库。Task 7 测试必须覆盖「同事件 props 键序不同仍去重」。

---

### Task 1: 脚手架打底

**Files:**
- Create: `modules/fishgame/`（脚手架产物全套）
- Modify: `alexgo-server/cmd/main.go`（AST 插入 fishgame.FxModule）
- Modify: `alexgo-server/configs/config.yaml`（modules.fishgame: true）
- Modify: `modules/system/service/permission_routes.go`（5 前缀占位，Task 9 收紧）
- Delete: 脚手架生成的实体 CRUD service/admin 控制器 + 前端 CRUD 视图（业务重写）

**Interfaces:**
- Consumes: `make new-module` 脚手架（tools/newmodule）
- Produces: `modules/fishgame/module.go` 的 `FxModule`；`RegisterRoutes` 已可装配空路由；build 过

- [ ] **Step 1: 脚手架生成骨架**

Run: `make new-module NAME=fishgame ENTITIES=fish,player`
Expected: 触点 1-6 落盘，`go build ./...` 通过，stdout 输出新建/修改文件清单

- [ ] **Step 2: 删除脚手架 CRUD 产物**

删除 `modules/fishgame/service/` 下实体 CRUD service、`controller/admin/` 下实体 CRUD 控制器、`apps/web-antd/src/views/fishgame/` 下脚手架生成的 CRUD 视图（保留目录）。

Run: `go build ./...`
Expected: PASS（scorecard 先例：脚手架产物全部重写/删除）

- [ ] **Step 3: config.yaml 落 modules.fishgame + fishgame 配置段**

在 `alexgo-server/configs/config.yaml` 的 `modules:` 下加 `fishgame: true`；新增顶层 `fishgame:` 段（`wechat.appid`/`wechat.secret` 空串占位 + `public_base_url: ""`）。`pkg/config/config.go` 的 `Config` 结构体加对应 `Fishgame` 字段（`mapstructure:"fishgame"`）。

Run: `go build ./...`
Expected: PASS

- [ ] **Step 4: 菜单种子占位（Task 10 会收敛）**

系统迁移已有脚手架生成的 `fishgame_menus` SQL 对（如生成）；Task 10 会替换为终版 5 页。当前先保留。

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(fishgame): 脚手架打底——模块骨架/装配/config 开关"
```

---

### Task 2: fishgame 业务错误码

**Files:**
- Modify: `pkg/errors/code.go`（常量 + registry）
- Test: `pkg/errors/code_test.go`（现有 exported-code 断言会自动覆盖）

**Interfaces:**
- Consumes: 无
- Produces: `errs.CodeFishgameFishNotFound=30401`, `CodeFishgameFishReferenced=30402`, `CodeFishgameFishKingInvalid=30403`, `CodeFishgameConfigNotFound=30404`

- [ ] **Step 1: 写失败测试（registry 覆盖）**

在 `pkg/errors/code_test.go` 的 exported codes 切片追加 4 个新码。

Run: `go test ./pkg/errors/ -run TestExportedCodes -v`
Expected: FAIL（`exported code 30401 missing from registry`）

- [ ] **Step 2: 注册 4 码 + 文案**

`pkg/errors/code.go` 追加：

```go
// ---- 304xx fishgame ----
CodeFishgameFishNotFound  Code = 30401 // 图鉴条目不存在
CodeFishgameFishReferenced Code = 30402 // 条目已被进度或事件引用，不可删除
CodeFishgameFishKingInvalid Code = 30403 // 鱼王指定的条目不存在或已停用
CodeFishgameConfigNotFound Code = 30404 // 运营配置不存在
```

registry 补 4 条中文文案（对应 spec §8 表）。

Run: `go test ./pkg/errors/ -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add pkg/errors/
git commit -m "feat(fishgame): 业务错误码 304xx（4 码 + 文案）"
```

---

### Task 3: 迁移 DDL + 9 表模型 + 图鉴种子

**Files:**
- Create: `modules/fishgame/migrations/20261023000001_fishgame_tables.up.sql`
- Create: `modules/fishgame/migrations/20261023000001_fishgame_tables.down.sql`
- Create: `modules/fishgame/migrations/20261023000002_fishgame_fish_seed.up.sql`（48 条 INSERT，幂等 NOT EXISTS）
- Create: `modules/fishgame/migrations/20261023000002_fishgame_fish_seed.down.sql`（DELETE）
- Create: `modules/fishgame/model/`：`player.go`, `session.go`, `fish.go`, `dex.go`, `track_event.go`, `sign.go`, `friend.go`, `config.go`, `share.go`
- Create: `modules/fishgame/migration_source.go`（脚手架已有，确认 embed）
- Test: `modules/fishgame/model/model_test.go`

**Interfaces:**
- Consumes: Task 1 目录结构
- Produces: `model.Player`, `model.Session`, `model.Fish`, `model.Dex`, `model.TrackEvent`, `model.Sign`, `model.Friend`, `model.Config`, `model.Share`（后续任务消费）

- [ ] **Step 1: 写失败测试（9 模型 TableName + 必填字段）**

`modules/fishgame/model/model_test.go`：断言 9 个模型的 `TableName()` 返回值分别为 `fishgame_player`/`fishgame_session`/`fishgame_fish`/`fishgame_dex`/`fishgame_track_event`/`fishgame_sign`/`fishgame_friend`/`fishgame_config`/`fishgame_share`。

Run: `go test ./modules/fishgame/model/ -v`
Expected: FAIL（undefined: model.Player 等）

- [ ] **Step 2: 写 9 模型（列与 DDL 严格同构）**

模型命名与字段（关键列）：

- `Player`: `ID`, `TenantID`, `OpenID`(uk含tenant), `Nick`, `Avatar`, `LastLoginAt`, `CreatedAt`, `UpdatedAt`
- `Session`: `ID`, `TenantID`, `Token`(唯一), `PlayerID`, `ExpireAt`, `CreatedAt`, `UpdatedAt`；索引 `(expire_at)`
- `Fish`: `ID`, `TenantID`, `FishID`(uk含tenant), `Name`, `Rarity`, `Period`, `Depth`, `Flavor`, `Sort`, `Status`, `CreatedAt`, `UpdatedAt`
- `Dex`: `ID`, `TenantID`, `PlayerID`, `FishID`(uk: player+fish), `Count`, `CreatedAt`, `UpdatedAt`
- `TrackEvent`: `ID`, `TenantID`, `PlayerID`, `Name`, `T`, `Props`(json), `DedupKey`(唯一), `CreatedAt`, `UpdatedAt`；索引 `(name,t)`, `(player_id,t)`, `(created_at)`
- `Sign`: `ID`, `TenantID`, `PlayerID`, `Date`(uk: player+date), `Streak`, `Bait`, `CreatedAt`, `UpdatedAt`
- `Friend`: `ID`, `TenantID`, `PlayerID`, `FriendPlayerID`(uk: player+friend), `CreatedAt`, `UpdatedAt`
- `Config`: `ID`, `TenantID`, `Type`, `Period`, `DepthBias`, `FishID`, `Week`, `StartAt`, `EndAt`, `Status`, `CreatedAt`, `UpdatedAt`
- `Share`: `ID`, `TenantID`, `Key`(唯一), `PlayerID`, `Size`, `Mime`, `CreatedAt`, `UpdatedAt`

Run: `go test ./modules/fishgame/model/ -v`
Expected: PASS

- [ ] **Step 3: 写 DDL up/down**

up.sql：9 张 `CREATE TABLE IF NOT EXISTS`（照 scorecard 惯例，无外键，`tenant_id` + 时间戳，索引按上表）。down.sql：9 张 `DROP TABLE IF EXISTS`。

Run: `ls modules/fishgame/migrations/`
Expected: 2 个 SQL 对落盘

- [ ] **Step 4: 图鉴种子 48 条**

从 `fish_db.gd` 的 `ENTRIES` 逐字抄录 48 条目：`fish_id`（id 字段）、`name`、`rarity`（normal/misc/rare/epic）、`period`（`times` 数组 join，day/dusk/night 任意组合或 any）、`depth`（any/shallow/mid/deep）、`flavor`、`sort`（插入序 1-48）、`status=1`。`tenant_id=0`。幂等：`WHERE NOT EXISTS (... fish_id ...)`。

period 映射规则：`times` 含 `["day","dusk","night"]` 全 → `period="any"`；否则按 times 实际组合存（如 `["night"]` → `night`）。

Run: `mysql -e "SELECT COUNT(*) FROM fishgame_fish" <db>`（迁移后） 或单测断言种子条数
Expected: 48 条

- [ ] **Step 5: 确认 migration_source embed 正常**

Run: `go build ./modules/fishgame/...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add modules/fishgame/
git commit -m "feat(fishgame): 迁移 9 表 + 模型 + 48 条图鉴种子"
```

---

### Task 4: repository 层

**Files:**
- Create: `modules/fishgame/repository/`：`player.go`, `session.go`, `fish.go`, `dex.go`, `track_event.go`, `sign.go`, `friend.go`, `config.go`, `share.go`
- Test: `modules/fishgame/repository/repository_test.go`（sqlite in-memory 或 testhelper 照 scorecard）

**Interfaces:**
- Consumes: Task 3 全部 model
- Produces: 9 个 Repository 接口 + New 构造（后续 Task 5-9 消费）。关键方法签名：
  - `PlayerRepository`: `UpsertByOpenID(ctx, openid, nick, avatar) (*model.Player, error)`；`GetByID(ctx, id) (*model.Player, error)`
  - `SessionRepository`: `Create(ctx, s *model.Session) error`；`GetByToken(ctx, token) (*model.Session, error)`；`TouchExpire(ctx, id, newExpire) error`
  - `FishRepository`: `ListActive(ctx) ([]*model.Fish, error)`；`ListAdmin(ctx, rarity string, status int, page, size int) ([]*model.Fish, int64, error)`；`GetByFishID(ctx, fishID) (*model.Fish, error)`；`GetByID(ctx, id) (*model.Fish, error)`；`Create/Update/Delete(ctx, ...) error`
  - `DexRepository`: `UpsertCatch(ctx, playerID, fishID) (firstInsert bool, err error)`（count+1 或首次插 1）；`MergeReport(ctx, playerID, counts map[string]int) error`（逐条 max）；`CountByPlayer(ctx, playerID) (int64, error)`；`ListByPlayer(ctx, playerID) ([]*model.Dex, error)`
  - `TrackEventRepository`: `BatchInsertIgnoreDup(ctx, events []*model.TrackEvent) (inserted int, err error)`（dedup_key 唯一键 ON DUPLICATE IGNORE）；`ListAdmin(ctx, name, playerID string, startT, endT int64, page, size int)`
  - `SignRepository`: `GetByPlayerDate(ctx, playerID, date) (*model.Sign, error)`；`Create(ctx, s *model.Sign) error`
  - `FriendRepository`: `AddPair(ctx, a, b uint64) error`（双向两行）；`ListFriendIDs(ctx, playerID) ([]uint64, error)`；`Exists(ctx, playerID, friendID) (bool, error)`
  - `ConfigRepository`: `ListActiveByType(ctx, typ string, now time.Time) (*model.Config, error)`（status=1 且 start_at<=now<=end_at/null）；`ListAdmin(ctx, typ string, page, size int)`；`Create/Update/Delete`
  - `ShareRepository`: `Create(ctx, s *model.Share) error`；`GetByKey(ctx, key) (*model.Share, error)`

- [ ] **Step 1: 写失败测试（各 repo 核心行为，sqlite/testhelper）**

参考 `modules/scorecard/repository/room_repo_test.go` 模式，为每个 repo 写 1-2 个核心行为测试：
- Player UpsertByOpenID 幂等（同 openid 二次调用更新 nick 不新增行）
- Session GetByToken 命中/过期 miss
- Dex UpsertCatch 首次 firstInsert=true、二次 firstInsert=false 且 count=2
- Dex MergeReport 取 max（旧 5 上报 3 → 5；旧 3 上报 5 → 5）
- TrackEvent BatchInsertIgnoreDup 同 dedup_key 只插 1 条
- Friend AddPair 双向落 2 行

Run: `go test ./modules/fishgame/repository/ -v`
Expected: FAIL（undefined）

- [ ] **Step 2: 实现 9 个 repository**

照 scorecard repository 惯例：软查（不存在返回 `(nil, nil)`，service 映射 304xx）；写路径显式 `tenant_id=0`（游戏端 ctx 无租户）；GORM 插件在位时 Query 自动追加 tenant 条件。

dex UpsertCatch 用 `clause.OnConflict` 或先查后插 + `count=count+1` 事务；TrackEvent 用 `clause.OnConflict{DoNothing: true}`（dedup_key 冲突跳过）。

Run: `go test ./modules/fishgame/repository/ -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add modules/fishgame/repository/
git commit -m "feat(fishgame): repository 层 9 实体（含 dex 合并/track 去重）"
```

---

### Task 5: 微信 client + 会话中间件 + auth/login

**Files:**
- Create: `modules/fishgame/service/wechat.go`（code2session HTTP 5s 超时）
- Create: `modules/fishgame/service/session.go`（会话签发/校验/滑续）
- Create: `modules/fishgame/service/auth.go`（login 业务）
- Create: `modules/fishgame/controller/app/auth.go`（POST /api/v1/auth/login handler）
- Create: `modules/fishgame/middleware/session.go`（X-Session 中间件）
- Test: `modules/fishgame/service/auth_test.go`, `modules/fishgame/service/wechat_test.go`, `modules/fishgame/middleware/session_test.go`

**Interfaces:**
- Consumes: Task 4 PlayerRepository/SessionRepository；config `fishgame.wechat.*`
- Produces: `service.AuthService.Login(ctx, code) (session string, err error)`；`middleware.SessionAuth(cfg, playerRepo, sessionRepo) gin.HandlerFunc`（命中时 `c.Set("fishgame_player", *model.Player)`）

- [ ] **Step 1: 写失败测试（wechat mock + auth + 中间件）**

- `wechat_test.go`: httptest mock `sns/jscode2session`，断言 openid 返回、errcode≠0 返回 error、超时（注入慢 handler）返回 error
- `auth_test.go`: mock wechat，断言 login 成功返回 session、player upsert、session 表落行；wechat 失败返回 error（映射 401 login_failed）
- `session_test.go`（中间件）: 无 X-Session → 401 裸 `{"error":"unauthorized"}`；有效 session → `c.Get("fishgame_player")` 可取；过期 session → 401；login 路由不挂中间件（单独测试 handler 直调）

Run: `go test ./modules/fishgame/... -v`
Expected: FAIL（undefined）

- [ ] **Step 2: 实现 wechat.go**

`type WechatClient struct { baseURL string; client *http.Client }`，`NewWechatClient(appid, secret string) *WechatClient`，client.Timeout=5s。`Exchange(ctx, code) (openid string, err error)`：GET `sns/jscode2session?appid&secret&js_code&grant_type=authorization_code`；errcode≠0 或 openid 空返回 error。错误文本不带 secret（照 social/wechat_mp.go 教训）。

- [ ] **Step 3: 实现 session 中间件 + AuthService**

中间件：`c.GetHeader("X-Session")` 空 → `c.AbortWithStatusJSON(401, gin.H{"error":"unauthorized"})`；查 session 表（token 存在且 expire_at > now）→ 剩余 <7d 时异步/同步 TouchExpire 顺延 now+30d → `c.Set("fishgame_player", player)`；miss → 401 裸 JSON。

AuthService.Login：wechat.Exchange → PlayerRepository.UpsertByOpenID（nick 占位 `微信用户+openid尾4`，清 `:` 防串）→ 生成 token（crypto/rand 32B base64url）→ SessionRepository.Create（expire=now+30d，tenant=0）→ 返回 token。

- [ ] **Step 4: 实现 auth 控制器 + 路由挂载**

`POST /api/v1/auth/login`：body `{"code":"..."}` → 成功 `c.JSON(200, gin.H{"session": token})`；wechat 失败 `c.JSON(401, gin.H{"error":"login_failed"})`。**不调 pkg/resp**。

`module.go` RegisterRoutes：
```go
v1 := r.Group("/v1")
v1.POST("/auth/login", m.authCtrl.Login)
// 中间件挂剩余路由（Task 6-8 逐步加）
v1Auth := v1.Group("", m.sessionMW)
```

Run: `go test ./modules/fishgame/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add modules/fishgame/
git commit -m "feat(fishgame): 微信 code2session + X-Session 中间件 + auth/login"
```

---

### Task 6: config/daily + daily/sign + basket

**Files:**
- Create: `modules/fishgame/service/daily.go`（config/daily、daily/sign、basket 业务）
- Create: `modules/fishgame/service/deterministic.go`（sha256 确定性自算）
- Create: `modules/fishgame/controller/app/daily.go`
- Test: `modules/fishgame/service/daily_test.go`, `modules/fishgame/service/deterministic_test.go`

**Interfaces:**
- Consumes: Task 4 ConfigRepository/FishRepository/SignRepository/PlayerRepository；Task 5 中间件（v1Auth group）
- Produces: `service.DailyService.DailyConfig(ctx, player) (*DailyConfigResponse, error)`；`DailyService.Sign(ctx, player, date string, streak int) (ok bool, bait int, err error)`；`DailyService.Basket(ctx, sinceUnix int64) (bait int, capped bool, err error)`

- [ ] **Step 1: 写失败测试（确定性算法 + 三端点业务）**

`deterministic_test.go`：
- 同一 epoch 天数两次调用 `ComputeLuck(days)` 返回相同 `{period, depth_bias}`
- `period ∈ {day,dusk,night,any}`，`depth_bias ∈ [0, 0.2]`
- 同一 ISO 周两次调用 `ComputeFishKing(week, pool)` 返回相同 fish_id；pool 15 条时返回池内 id

`daily_test.go`：
- DailyConfig：无覆盖时自算返回完整 `{luck:{period,depth_bias}, fish_king:{fish_id, week}}`
- DailyConfig：有有效覆盖（status=1, 期内）时返回覆盖值
- DailyConfig：鱼王覆盖 fish_id 不在图鉴表 → 30403 或回退自算（spec §4.7 二次过滤回退自算）
- Sign：同日重复 → ok=false
- Sign：隔 1 天 → streak=old+1，reward 按表 1→1, 2→2, 3→3, 4-6→3, 7+→5
- Sign：隔 ≥2 天 → streak=1, reward=1
- Basket：since=now-0 → bait=0, capped=false；since=now-2400*3 → bait=3；since=now-2400*20 → bait=12, capped=true

Run: `go test ./modules/fishgame/service/ -run "Daily|Deterministic|Sign|Basket" -v`
Expected: FAIL（undefined）

- [ ] **Step 2: 实现确定性算法**

```go
// YMDToDays：civil 算法（照 fish_db.gd _ymd_to_days，era/yoe/doy 公式）
func YMDToDays(y, m, d int) int
// SeedFromSHA256：str(seed).sha256()[:4] little-endian → uint32
func SeedFromSHA256(s string) uint32
// ComputeLuck：period = [day,dusk,night,any][seed%4]；depth_bias = float(seed>>8%201)/1000（0~0.2）
func ComputeLuck(epochDays int) (period string, depthBias float64)
// ComputeFishKing：seed = SeedFromSHA256(week)；fishID = pool[seed % len(pool)]
func ComputeFishKing(week string, pool []string) string
// ISOMonthWeek：照 _iso_week_str（周四锚定 civil-day 空间）
func ISOMonthWeek(t time.Time) string
```

**Ruling（Task 6）**: 服务端自算不复刻 Godot PCG32 RNG，用 sha256 前 4 字节直接确定性选取。理由：客户端 fetch_daily_config 先本地后请求，服务端形状合法即覆盖本地，服务端值是权威；同一天全局相同即可，不需要逐字节匹配客户端 RNG。cost if wrong：联调时客户端本地值与服务端值短暂不一致（仅离线场景），无功能影响。

- [ ] **Step 3: 实现 DailyService + 控制器**

DailyConfig：查 ConfigRepository 有效覆盖（luck/fish_king 各一）→ 命中且 fish_id 校验在图鉴表则用覆盖；否则自算。响应形状严格照契约：`{"luck":{"period","depth_bias"},"fish_king":{"fish_id","week"}}`，`c.JSON(200, ...)` 裸 JSON。

Sign：date 解析校验（YYYY-MM-DD）；查 (player, date) 重复 → ok=false；否则算新 streak（客户端传的 streak 作参考，服务端按上次 date 差值权威计算）→ 落库 → 返回 `{ok:true, bait}`。**Ruling（Task 6）**: 客户端传的 streak 作对账参考，服务端按 (player, last date) 差值权威计算新 streak（spec 决定 9「当前信任客户端」但仍可算，防简单改表；成本 if wrong：客户端 streak 与服务端不一致时以服务端为准，客户端本地已加 bait 不受影响）。

Basket：`since` unix 秒 → `elapsed = now - since`（负数钳 0）→ `bait = min(elapsed/2400, 12)`，`capped = elapsed >= 28800`。响应 `{bait, capped}` 裸 JSON。**不落表**（spec 决定）。

- [ ] **Step 4: 路由挂载**

```go
v1Auth.GET("/config/daily", m.dailyCtrl.DailyConfig)
v1Auth.POST("/daily/sign", m.dailyCtrl.Sign)
v1Auth.GET("/basket", m.dailyCtrl.Basket)
```

Run: `go test ./modules/fishgame/service/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add modules/fishgame/
git commit -m "feat(fishgame): config/daily + daily/sign + basket（确定性自算兜底）"
```

---

### Task 7: track + dex 双路 + dex/table + dex/report

**Files:**
- Create: `modules/fishgame/service/track.go`
- Create: `modules/fishgame/service/dex.go`
- Create: `modules/fishgame/controller/app/track.go`
- Create: `modules/fishgame/controller/app/dex.go`
- Test: `modules/fishgame/service/track_test.go`, `modules/fishgame/service/dex_test.go`

**Interfaces:**
- Consumes: Task 4 TrackEventRepository/DexRepository/FishRepository/PlayerRepository
- Produces: `service.TrackService.Track(ctx, player, events []TrackEventReq) error`；`service.DexService.Report(ctx, player, counts map[string]int) (totalDex int64, err error)`；`DexService.Table(ctx) ([]FishRow, error)`

- [ ] **Step 1: 写失败测试（dedup + 双路合并 + 截断）**

`track_test.go`：
- 同事件（同 name/t/props）重复上报：dedup 命中只落 1 行
- props 键序不同仍去重（canonical JSON）
- 单批 250 条截断到 200
- 单条缺 name/t 跳过该条，其余落库
- catch 事件（props.kind=="fish"）触发 dex UpsertCatch；fish_id 不在图鉴表 → warn 跳过不报错

`dex_test.go`：
- Report：`{inkfish:5}` 与已有 catch count=3 合并 → count=5（max）
- Report：`{inkfish:2}` 与已有 count=5 合并 → count=5（max，不上报 2 覆盖 5）
- Report：未知 fish_id 忽略
- Table：只返回 status=1 条目

Run: `go test ./modules/fishgame/service/ -run "Track|Dex" -v`
Expected: FAIL（undefined）

- [ ] **Step 2: 实现 TrackService**

- 解析 `{"events":[{"name","t","props"}]}`；截 200 条；每条 props JSON 序列化截 4KB
- dedup_key = `sha1(openid + "|" + name + "|" + strconv(t) + "|" + canonicalJSON(props))`（canonicalJSON：map 键排序）
- BatchInsertIgnoreDup 落库
- 遍历插入成功的事件：`name=="catch" && props.kind=="fish"` → FishRepository 校验 fish_id 存在（否则 warn 跳过）→ DexRepository.UpsertCatch
- 响应：`c.Status(204)`（无 body）

- [ ] **Step 3: 实现 DexService + 控制器**

Table：FishRepository.ListActive → `{"list":[{id,name,rarity,period,depth,flavor}]}`（字段名照契约）。

Report：body `{"dex":{"<fish_id>":N}}` → DexRepository.MergeReport → 返回 `{ok:true, dex: 总点亮数}`（CountByPlayer）。

- [ ] **Step 4: 路由挂载**

```go
v1Auth.POST("/track", m.trackCtrl.Track)
v1Auth.GET("/dex/table", m.dexCtrl.Table)
v1Auth.POST("/dex/report", m.dexCtrl.Report)
```

Run: `go test ./modules/fishgame/service/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add modules/fishgame/
git commit -m "feat(fishgame): track dedup + dex 双路合并 + dex/table + dex/report"
```

---

### Task 8: 好友 + share/image

**Files:**
- Create: `modules/fishgame/service/friend.go`
- Create: `modules/fishgame/service/share.go`
- Create: `modules/fishgame/controller/app/friend.go`
- Create: `modules/fishgame/controller/app/share.go`
- Create: `modules/fishgame/controller/app/public.go`（公开读端点）
- Test: `modules/fishgame/service/friend_test.go`, `modules/fishgame/service/share_test.go`

**Interfaces:**
- Consumes: Task 4 FriendRepository/DexRepository/PlayerRepository/ShareRepository；pkg/storage Driver
- Produces: `service.FriendService.Add(ctx, player, targetOpenID) error`；`FriendService.Rank(ctx, player) ([]RankRow, error)`；`ShareService.Upload(ctx, player, filename string, r io.Reader) (url string, err error)`

- [ ] **Step 1: 写失败测试（好友双口径 + 分享）**

`friend_test.go`：
- Add：目标 openid 不存在 → 400 裸 `{"error":"target_not_found"}`
- Add：自己加自己 → 400 `{"error":"self_friend"}`
- Add：幂等（已存在返回 ok）
- Add：成功双向落 2 行
- Rank：好友表非空 → 好友 ∪ 自己，按 dex 降序
- Rank：好友表为空 → 同租户全体玩家降序

`share_test.go`：
- Upload：非白名单扩展名（.gif）→ 400
- Upload：超 5MB → 400
- Upload：合法 png 落 storage + share 表 → 返回绝对 URL
- Public：key 不存在 → 404；存在 → 200 inline

Run: `go test ./modules/fishgame/service/ -run "Friend|Share" -v`
Expected: FAIL（undefined）

- [ ] **Step 2: 实现 FriendService + 控制器**

Add：查 target openid（PlayerRepository）；校验非自己、存在；FriendRepository.AddPair 双向写；返回 `{ok:true}` 裸 JSON。

Rank：FriendRepository.ListFriendIDs；非空 → dex 聚合（好友 ∪ 自己）降序；空 → 同租户全体（PlayerRepository 或 DexRepository 全表聚合）降序。响应 `{"list":[{"nick","dex"}]}`。

**Ruling（Task 8）**: 微信关系链留接口位 `FriendSource`（`FriendsOf(ctx, openid) []openid`），一期实现 `StoredFriendSource`（库内关系）；二期可插 `WeChatRelationSource` 不动 handler。好友榜空回退「同租户全体玩家」= 同小游戏 app 所有 openid（spec §4.6）。

- [ ] **Step 3: 实现 ShareService + 公开端点**

Upload：multipart 字段 `image` → 扩展名白名单 png/jpg/webp（双 MIME 校验照 file_service.go 策略）→ ≤5MB → `pkg/storage` Driver.Save（relPath 随机 key + 扩展名）→ ShareRepository.Create → 返回 `{"url": 绝对 URL}`。绝对 URL：`fishgame.public_base_url` 配置非空则用它，否则由请求 Host + https 拼接。

公开读：`GET /api/public/fishgame/share/:key`（免鉴权）→ ShareRepository.GetByKey → storage.Driver.Open → `c.Data(200, mime, bytes)`；miss → 404。

- [ ] **Step 4: 路由挂载**

```go
v1Auth.POST("/friends", m.friendCtrl.Add)
v1Auth.GET("/friends/rank", m.friendCtrl.Rank)
v1Auth.POST("/share/image", m.shareCtrl.Upload)
// 公开端点挂 /api group 外层（非 /v1）
r.GET("/public/fishgame/share/:key", m.publicCtrl.Share)
```

**注意**：`RegisterRoutes` 入参是 `/api` group，所以公开端点完整路径是 `/api/public/fishgame/share/:key`。

Run: `go test ./modules/fishgame/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add modules/fishgame/
git commit -m "feat(fishgame): 好友 add/rank 双口径 + share/image 公开读"
```

---

### Task 9: admin 五组端点 + casbin 前缀

**Files:**
- Create: `modules/fishgame/service/admin.go`（admin 业务聚合）
- Create: `modules/fishgame/controller/admin/admin.go`（五组 handler）
- Modify: `modules/system/service/permission_routes.go`（5 前缀正式路由）
- Test: `modules/fishgame/service/admin_test.go`, `modules/fishgame/controller/admin/admin_test.go`

**Interfaces:**
- Consumes: Task 4 全部 repo（fish/config/track_event/player/dex）
- Produces: `service.AdminService`：`ListFish/CreateFish/UpdateFish/DeleteFish`；`ListConfig/CreateConfig/UpdateConfig/DeleteConfig`；`ListEvents`；`ListPlayers/PlayerDetail/PlayerRank`；`Stats`

- [ ] **Step 1: 写失败测试（五组 admin 业务）**

- ListFish：rarity/status 筛选、分页
- CreateFish：fish_id 查重（重复 → 30401 或 409 语义）；rarity/period/depth 枚举校验
- DeleteFish：被 dex/track_event 引用 → 30402
- CreateConfig：type=fish_king 时 fish_id 不存在/停用 → 30403
- ListEvents：name/player_id/起止时间筛选、倒序
- ListPlayers：openid/nick 模糊搜，行含 dex 点亮数
- PlayerDetail：基本信息 + 点亮明细 + 最近 20 事件
- PlayerRank：复用 Rank 逻辑找玩家次位
- Stats：玩家总数/今日新增、事件按 name 计数（近 7 日）、四类漏斗（catch/empty/escape/broken）、签到/领筐/广告计数

Run: `go test ./modules/fishgame/service/ -run Admin -v`
Expected: FAIL（undefined）

- [ ] **Step 2: 实现 AdminService + 控制器**

admin controller 照 scorecard 模式：不取 claims（casbin 已鉴权），`resp.OK/Fail/Page` 标准信封。五组 handler：

- `GET /fishes`（fishgame:fish）：分页 + rarity/status 筛选
- `POST /fishes`：新建（fish_id 查重、枚举校验）
- `PUT /fishes/:id`：更新（含 status）
- `DELETE /fishes/:id`：引用检查 → 30402
- `GET/POST/PUT/DELETE /configs(/:id)`（fishgame:config）：CRUD + fish_id 校验 → 30403
- `GET /events`（fishgame:event）：分页 + 筛选
- `GET /players`、`GET /players/:id`、`GET /players/:id/rank`（fishgame:player）
- `GET /stats`（fishgame:stats）：聚合

- [ ] **Step 3: 登记 permission_routes**

`modules/system/service/permission_routes.go` 正式加 5 前缀（Task 1 占位替换）：

```go
"fishgame:fish":   {"/api/admin/fishgame/fishes", "/api/admin/fishgame/fishes/*"},
"fishgame:config": {"/api/admin/fishgame/configs", "/api/admin/fishgame/configs/*"},
"fishgame:event":  {"/api/admin/fishgame/events", "/api/admin/fishgame/events/*"},
"fishgame:player": {"/api/admin/fishgame/players", "/api/admin/fishgame/players/*"},
"fishgame:stats":  {"/api/admin/fishgame/stats"},
```

Run: `go test ./modules/system/service/ -run PermissionRoutes -v`
Expected: PASS（5 前缀覆盖）

- [ ] **Step 4: 路由挂载**

`module.go` RegisterRoutes：
```go
admin := r.Group("/admin/fishgame")
admin.GET("/fishes", m.adminCtrl.ListFish)
// ... 五组全部
```

Run: `go test ./modules/fishgame/... ./modules/system/service/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add modules/fishgame/ modules/system/service/permission_routes.go
git commit -m "feat(fishgame): admin 五组端点 + casbin 前缀登记"
```

---

### Task 10: 前端 5 页 + api/fishgame.ts + 菜单种子

**Files:**
- Create: `apps/web-antd/src/api/fishgame.ts`
- Create: `apps/web-antd/src/views/fishgame/FishgameFishesPage.vue`
- Create: `apps/web-antd/src/views/fishgame/FishgameConfigsPage.vue`
- Create: `apps/web-antd/src/views/fishgame/FishgameEventsPage.vue`
- Create: `apps/web-antd/src/views/fishgame/FishgamePlayersPage.vue`
- Create: `apps/web-antd/src/views/fishgame/FishgameStatsPage.vue`
- Modify: `modules/system/migrations/`（新菜单种子 SQL 对，替换/新增）
- Test: 前端构建 `pnpm build` 或 vitest（照 scorecard 惯例）

**Interfaces:**
- Consumes: Task 9 admin API 五组
- Produces: 5 个 Vue 页面 + api/fishgame.ts 导出函数；菜单种子幂等迁移

- [ ] **Step 1: 写 api/fishgame.ts**

照 `scorecard.ts` 模式（requestClient 统一信封）：

```typescript
export function getFishgameFishesApi(params) { return requestClient.get('/admin/fishgame/fishes', { params }); }
export function createFishgameFishApi(data) { return requestClient.post('/admin/fishgame/fishes', data); }
export function updateFishgameFishApi(id, data) { return requestClient.put(`/admin/fishgame/fishes/${id}`, data); }
export function deleteFishgameFishApi(id) { return requestClient.delete(`/admin/fishgame/fishes/${id}`); }
// configs/events/players/stats 同款
```

- [ ] **Step 2: 写 5 个页面**

- `FishgameFishesPage.vue`：分页表格 + rarity/period/depth 下拉 + 新建/编辑/删除 + status 启停
- `FishgameConfigsPage.vue`：列表 + type 切换 luck/fish_king（鱼王 fish_id 下拉取自 fishes API）
- `FishgameEventsPage.vue`：分页 + name/player/时间筛选 + props JSON 折叠展示
- `FishgamePlayersPage.vue`：分页 + 搜索 + 详情抽屉（点亮明细 + 最近 20 事件 + 榜单位次）
- `FishgameStatsPage.vue`：总数卡、近 7 日事件趋势、四结果漏斗

页内不使用 `v-access`（可见性靠菜单、操作靠 casbin，scorecard 同款）。

- [ ] **Step 3: 菜单种子迁移**

`modules/system/migrations/20261023000001_fishgame_menus.up.sql` + `.down.sql`：照 scorecard 菜单模式，幂等 NOT EXISTS。1 目录（Fishgame, `/fishgame`, perm `fishgame`, sort 85）+ 5 页面（英文占位名，permission 分别 `fishgame:fish/config/event/player/stats`）。

Run: 迁移执行 2 次不重复插入
Expected: 幂等

- [ ] **Step 4: 前端构建回归**

Run: `cd apps/web-antd && pnpm build`（或项目惯例构建命令）
Expected: PASS，无 TS/模板错误

- [ ] **Step 5: Commit**

```bash
git add apps/web-antd/src/ modules/system/migrations/
git commit -m "feat(fishgame): 前端 5 页 + api/fishgame.ts + 菜单种子"
```

---

### Task 11: modules.md 回填 + 全量验收

**Files:**
- Modify: `docs/modules.md`（补 §12 fishgame 章节）
- Test: 全量 `go build ./...` + `go test ./...`

**Interfaces:**
- Consumes: Task 1-10 全部产出
- Produces: modules.md 章节；全量绿

- [ ] **Step 1: modules.md 补 §12**

照 scorecard §11 格式：模块名、开关、端点清单（客户端 10 + admin 5 组）、数据模型 9 表、错误码 304xx、前端 5 页、菜单种子、联调要点。

- [ ] **Step 2: 全量构建 + 测试**

Run: `go build ./... && go test ./...`
Expected: 0 FAIL

- [ ] **Step 3: 契约逐字段核对**

对照 `backend-api.md` 与 §4.1 表，人工核对 10 端点响应字段形状（plan Review Focus #1）。断言 `/api/v1` 响应无 `code` 顶层键（测试已在 Task 5-8 覆盖，此处复核）。

- [ ] **Step 4: 菜单种子幂等 + admin 可见 5 页面**

迁移重复执行不重复插入；全新库启动后 admin 角色可见 5 页面（seed 全量绑 admin）。

- [ ] **Step 5: Commit**

```bash
git add docs/modules.md
git commit -m "docs(fishgame): modules.md 补 §12 章节 + 全量验收"
```

---

### 终审 + 收尾

- [ ] **Step 1: 全量测试 + 构建**

Run: `go build ./... && go test ./... && pnpm --filter web-antd build`
Expected: 0 FAIL

- [ ] **Step 2: 客户端联调清单（可选，交付说明）**

1. 后端起服务，微信小游戏后台配置 request 合法域名。
2. 客户端 `online_service.gd` 顶部 `base_url` 填后端地址。
3. 微信开发者工具 → Network 面板观察 `/track` 批量上报。
4. 验证覆盖：admin 下发指定 fish_id，看游戏内当周鱼王是否切换。

- [ ] **Step 3: 终审（requesting-code-review 或 self-review）**

dispatch code-reviewer 或 self-review，按 Review Focus 5 类重点检查。

---

## Self-Review 记录

1. **Spec coverage**: 契约 7 端点（Task 5-8）+ 3 新端点（Task 7-8）+ admin 5 组（Task 9）+ 前端 5 页（Task 10）+ 9 表迁移（Task 3）+ 错误码（Task 2）+ 菜单种子（Task 10）+ modules.md（Task 11）全覆盖，无 gap。
2. **Step scan**: 每步一个可检查动作（测试/实现/构建/提交），无「处理边角」类空话。
3. **Type consistency**: Task 4 Produces 的 9 个 repo 接口签名在 Task 5-9 消费时一致；service 接口在 Task 6-9 间传递（DailyService/TrackService/DexService/FriendService/ShareService/AdminService）。
4. **Review Focus**: 5 类均有对应任务测试（裸 JSON 断言在 Task 5-8；中间件挂载在 Task 5；dex 合并在 Task 7；值域在 Task 6；canonical dedup 在 Task 7）。
5. **Proportion**: plan 长度与 spec 相当（11 任务 vs spec 10 实现顺序建议），代码块仅算法核心（YMDToDays/SeedFromSHA256/ComputeLuck），其余签名+测试名，合理。

## 执行交接

Plan 已保存至 `docs/superpowers/plans/2026-10-23-fishgame-module.md`。请审阅 plan 并选择执行方式：
- **Subagent-driven**：每个任务由 fresh subagent 实现 + fresh reviewer 检查，最彻底，但每任务消耗 fresh context。
- **Native（executing-plans）**：本 session inline 执行全部任务，最后一次性终审 review，最省。

本 plan 任务间依赖紧密（Task 4 repo → Task 5-9 service/controller → Task 10 前端），共 11 任务，建议 **Native（executing-plans）**：任务依赖线性、接口已在 plan 钉死，inline 成本低且上下文连续。
