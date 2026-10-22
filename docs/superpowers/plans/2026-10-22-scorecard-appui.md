# scorecard-appui 小程序前端 + 后端前置票 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `modules/scorecard/appui/uniapp`（Wot Starter v2）落地 breakout 12 屏全量小程序前端（微信登录/4 主题/小程序码邀请/语音/好友/手册），并先完成两个后端前置票（member me GET+PUT、小程序码生成）。

**Architecture:** 后端两票独立于前端，先行 TDD 落 main；前端在脚手架上裁剪演示件 → 建主题 token 体系（4 套 CSS 变量类 + pinia persist）→ 手写 alova API 层（不走 codegen）→ 按页面推进（首页/房间/支出/结算/战绩/好友/手册 + 三弹层），房间实时用 3s 快照轮询 + version 去重。设计依据 [`2026-10-22-scorecard-appui-design.md`](../specs/2026-10-22-scorecard-appui-design.md)（决策表 #1–15）。

**Tech Stack:** 后端 Gin+Fx+GORM（monorepo 惯例，`go test` TDD）；前端 Vue3 + uni-app + Wot UI v2 + Alova + UnoCSS + pinia persist + `@wot-ui/router`。

**Spec:** [`docs/superpowers/specs/2026-10-22-scorecard-appui-design.md`](../specs/2026-10-22-scorecard-appui-design.md)

## Global Constraints

- **产品红线**：界面文案与代码命名一律 `score`/`tea` 口径，不出现货币符号/资金词汇；`--vermillion` 合规红 + 红中印元素不进主题覆盖（全局恒定白名单）。
- **Git**：直接在 main 工作，不建功能分支；每任务一提交，conventional commits（`feat(scorecard):` / `fix(scorecard):` 前缀）。
- **设计稿唯一样式源**：`modules/scorecard/appui/desgin/preview/miniapp-breakout.html`（token/类名照翻译）；主题 token 源 `preview/themes.html`（t2/t3/t4 覆盖类）。
- **后端响应信封**：`{code,msg,data,total}`；错误码五位制（scorecard 段 30xxx）；app 端 Bearer Member token（UserType==Member）。
- **前端无单测基建**（脚手架未装 vitest，不引入）：每任务验证 = `pnpm type-check` + `pnpm lint` + 设计稿对照的验收要点；后端每任务验证 = `go test ./modules/... -run <Pattern> -v` 全绿。
- **不引入第二套脚手架/组件库**；Wot 组件仅 toast/popup/swipe/icon，牌桌筹码牌面自绘。
- **后端改动仅限**：Task 1 me 票 + Task 2 小程序码票；其余接口 gap 不动。
- **工作目录**：后端任务 cwd=`/Users/alex/Desktop/goWork/alexGo-cloud`（monorepo 根）；前端任务 cwd=`modules/scorecard/appui/uniapp`。
- **环境变量**：`.env.development` 的 `VITE_API_BASE_URL` 改为本地后端（`http://127.0.0.1:8080`，端口以本地后端实际监听为准，builder 启动后端确认）。

## Review Focus

以下五类输入/条件最可能咬人，各自钉在所属任务的验收步骤里：

1. **expense 幂等键重试**：网络超时后用户重按「提交」，不得双记 → Task 8 验收含「同 client_request_id 重放返回同 log_id」。
2. **轮询旧响应覆盖新状态**：3s 轮询与支出响应内嵌 snapshot 竞态，旧 version 不得覆盖新 version → Task 7 实现步骤钉 `version` 单调校验。
3. **scene 进房时序**：小程序码 scene 在 onShow 早于登录/页面就绪到达，须「登录完成后再 join、scene 只消费一次」→ Task 5 骨架 + Task 12 闭环验收。
4. **微信能力降级矩阵**：H5 下 wx.login/scanCode/小程序码/同声传译全部不可用，各入口必须有降级路径且不抛错 → Task 5/12/13 验收含 H5 降级项。
5. **主题硬编码色值回归**：组件写死 hex 则 4 主题切换失效 → Task 3 起每任务 `pnpm lint` + 验收含「切 t2/t3/t4 该页无残留旧色」；合规红/红中印为唯一白名单。

---

### Task 1: 后端 me 票 — GET/PUT /app/member/me

**Files:**
- Modify: `modules/member/service/member.go`（追加 `Me` / `UpdateMe` 方法到现有 `MemberService` 接口与实现）
- Create: `modules/member/controller/app/member_me.go`（Me/UpdateMe 控制器）
- Create: `modules/member/controller/app/member_me_test.go`
- Modify: `modules/member/module.go`（appGroup 挂两条路由）
- Modify: `modules/member/service/member_test.go`（service 层用例，若惯例是与实现同文件测试则并入现有测试文件）

**Interfaces:**
- Consumes: 现有 `MemberService`、`pkg/token` claims（userID 提取沿用 `member_auth.go` 的登录态中间件模式）、`pkg/resp` 信封。
- Produces（Task 4 前端消费）:
  - `GET /app/member/me` → `data: {user_id, nickname, avatar, created_at}`（user_id 即 MemberUser.ID）
  - `PUT /app/member/me` body `{nickname?, avatar?}` → `data: {user_id, nickname, avatar, created_at}`（只改提供的项；nickname 空串拒绝 10001；avatar 单独可清）

- [ ] **Step 1: 写失败测试**

service 层（`member_test.go` 惯例：sqlite :memory: + 直构 repo）：

```go
func TestMe_ReturnsProfile(t *testing.T)          // 建行后 Me(uid) → nickname/avatar/created_at 齐
func TestMe_NotFound_Deleted(t *testing.T)        // 软删行 → 40004
func TestUpdateMe_NicknameOnly(t *testing.T)      // 只传 nickname → avatar 不变
func TestUpdateMe_AvatarOnly(t *testing.T)        // 只传 avatar → nickname 不变
func TestUpdateMe_EmptyNickname_Rejected(t *testing.T) // nickname="" → 10001
```

controller 层（`member_me_test.go`，httptest + stub service，模式照 `member_auth_test.go`）：

```go
func TestMe_OK(t *testing.T)                 // Bearer 鉴权后 200，data 三键 json 键名逐字 user_id/nickname/avatar/created_at
func TestMe_Update_OK(t *testing.T)          // PUT 部分字段 200
func TestMe_Update_EmptyNickname_10001(t *testing.T)
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./modules/member/... -run 'TestMe|TestUpdateMe' -v`
Expected: 编译失败（方法未定义）或 FAIL。

- [ ] **Step 3: 实现 service + 控制器 + 路由**

- `Me(ctx, userID uint64) (*ProfileResult, error)`：`ProfileResult{UserID, Nickname, Avatar, CreatedAt}`；查行（软删→40004，沿用现有 `errs` 码 `CodeUserNotFound`）。
- `UpdateMe(ctx, userID uint64, nickname, avatar *string) (*ProfileResult, error)`：指针语义（nil=不改）；nickname 提供且为空串 → `CodeParamInvalid`；查行 → 更新 → 返回最新。
- 控制器：登录态中间件内取 userID（照 app.go 现有模式）；ShouldBindJSON 用指针字段。
- `module.go`：`appGroup.GET("/me", ...)`、`appGroup.PUT("/me", ...)`。

- [ ] **Step 4: 运行测试至全绿**

Run: `go test ./modules/member/... -v` 与 `go test ./modules/scorecard/... ./modules/system/... -count=1`（确认无涟漪）
Expected: PASS，0 FAIL。

- [ ] **Step 5: 提交**

```bash
git add modules/member/
git commit -m "feat(member): app 端 me 接口（GET/PUT 资料）"
```

---

### Task 2: 后端小程序码票 — social getwxacodeunlimit + scorecard 路由

**Files:**
- Modify: `modules/system/oauth2/social/wechat_mp.go`（追加 `StableToken` / `Wxacode` 方法）
- Create: `modules/system/oauth2/social/wxacode.go`（可选：若逻辑独立成文件更清晰）
- Modify: `modules/system/oauth2/social/wechat_mp_test.go`（httptest mock `api.weixin.qq.com`）
- Modify: `modules/scorecard/controller/app/app.go`（或新文件 `wxacode.go`）+ `modules/scorecard/module.go`（挂路由）
- Modify: `modules/scorecard/controller/app/app_test.go`

**Interfaces:**
- Consumes: `SocialApp`（渠道配置 appid/secret，`GetByPlatform("wechat_mp")` 模式照 social 登录取配置处）、`WechatMP.baseURL`（测试注入 httptest 地址）、scorecard 现有 room 存在性校验（30101）、`pkg/token` Member 鉴权。
- Produces（Task 12 前端消费）:
  - `GET /app/scorecard/rooms/:id/wxacode` → `200 image/png`（非 JSON 信封；字节直出，`Content-Type: image/png`），鉴权同 scorecard app 组。
  - social 内部：`func (m *WechatMP) StableToken(ctx, app *SocialApp) (string, error)`、`func (m *WechatMP) Wxacode(ctx, app *SocialApp, scene, page, envVersion string) ([]byte, error)`（`envVersion` 空=正式版，"trial"=体验版）。

- [ ] **Step 1: 写失败测试**

social 层（httptest 起假微信 server，注入 `m.baseURL`）：

```go
func TestStableToken_CachesAndReturns(t *testing.T)   // 200 → token；二次调用不再打 /cgi-bin/stable_token（计数断言）
func TestWxacode_ReturnsPNG(t *testing.T)             // 假 server 返回 PNG 字节 → 原样透传
func TestWxacode_WechatErr_MapsError(t *testing.T)    // errcode!=0 → error 上抛（不泄 secret）
func TestWxacode_SecretNotInError(t *testing.T)       // 网络错误含 URL 时沿用 wechat_mp.go 剥离先例，error 串不含 secret
```

scorecard 控制器层：

```go
func TestWxacode_OK_PNG(t *testing.T)         // 房存在+成员 → 200 image/png
func TestWxacode_RoomNotFound_30101(t *testing.T)
func TestWxacode_NotMember_30102(t *testing.T) // 非成员不可取码（防外人批量取码）
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./modules/system/oauth2/... ./modules/scorecard/... -run 'StableToken|Wxacode' -v`
Expected: FAIL/编译错误。

- [ ] **Step 3: 实现**

- `StableToken`：POST `https://api.weixin.qq.com/cgi-bin/stable_token`（body `{grant_type:"client_credential",appid,secret}`），**进程内缓存**（内存 map 按 appid 缓存至 expires_in-300s；单实例足够，不引入 redis）。
- `Wxacode`：POST `https://api.weixin.qq.com/wxa/getwxacodeunlimit`（body `{scene:"qr=<roomCode>",page:"pages/index/index",env_version,check_path:false}`），响应 errcode 非 0 → 上抛；否则 body 即图片字节。scene 前缀 `qr=` 由 scorecard 控制器拼（房号来自 room 行）。
- scorecard 控制器 `Wxacode`：登录态取 uid → 房存在校验（复用 Snapshot 的 30101/30102 门）→ 取 SocialApp（无配置 → 500 带明确 msg；social 对外暴露取配置的小接口，scorecard module 注入）→ 调 `Wxacode(ctx, app, "qr="+room.RoomCode, "pages/index/index", envVersion)`；**envVersion**：配置项或常量 `"trial"`（开发期，放 controller 顶部常量并注释，正式发版改空串）。**进程内按 roomCode 缓存 5 分钟**。
- 路由：`appGroup.GET("/rooms/:id/wxacode", ...)` 与现有 21 路由同组。

- [ ] **Step 4: 运行测试至全绿**

Run: `go test ./modules/system/oauth2/... ./modules/scorecard/... -v`
Expected: PASS，0 FAIL。

- [ ] **Step 5: 提交**

```bash
git add modules/system/oauth2/social/ modules/scorecard/
git commit -m "feat(scorecard): 房间小程序码接口（social 补 getwxacodeunlimit）"
```

---

### Task 3: 前端基座 — 脚手架裁剪 + 4 主题 token 体系

**Files:**
- Delete: `src/pages/about/`、`src/subPages/`（ci/create-uni/feedback/styles/pinia）、`src/subEcharts/`、`src/subAsyncEcharts/`（如存在）、`src/components/DemoBlock.vue`、`src/layouts/tabbar.vue`、`src/composables/useTabbar.ts`
- Modify: `pages.config.ts`（删 tabBar 整块；globalStyle 改 `navigationStyle: custom`、背景 `#14120F`、删 about 列表项）
- Modify: `vite.config.ts`（UniHelperPages subPackages 数组清空/移除已删目录；Components dirs 去掉 `src/business` 若不存在）
- Modify: `src/api/mock/mockAdapter.ts`（改为恒空/关闭 mock）+ `src/api/core/instance.ts`（去掉 `mockRequest` 传入）
- Modify: `.env.development`（`VITE_API_BASE_URL=http://127.0.0.1:<后端端口>`）
- Create: `src/theme/scorecard.scss`（四套主题变量 + 工具类）
- Create: `src/store/scorecard/theme.ts`（themeStore：`themeId: 1|2|3|4`，persist id `scorecard-theme`）
- Modify: `src/App.vue`（启动恢复主题 class；移除 tabbar 相关）
- Modify: `src/pages/index/index.vue`（占位重写：去 tabbar layout、去演示内容，改为 `definePage({style:{navigationStyle:'custom'}})` 的最小骨架——完整首页在 Task 6）
- Modify: `src/router/index.ts`（删 demo-protected 守卫演示，保留空守卫链）

**Interfaces:**
- Consumes: 脚手架 pinia persist 插件（`store/persist.ts`，按 `$id` 持久化）。
- Produces:
  - CSS 变量（`.theme-t1`..`.theme-t4` 挂根节点后全局可读）：`--ink-900 --ink-800 --felt --tile --brass --vermillion --indigo --jade --win-dark --lose-dark --on-ink --pad-safe`（t1 值照 breakout `:root`；t2/t3/t4 照 themes.html 覆盖；`--vermillion` 四套同值锁定）。
  - 工具类：`.num`（Oswald tabular）、`.win-n/.lose-n/.jade-n`、`.btn.brass/.bone/.ghost`、`.pcard`、`.chip`、`.comply`、`.sheet`。
  - `useScorecardTheme()`：`{ themeId, setTheme(id) }`，setTheme 写 store 并更新根节点 class（`theme-t1`..`t4` 互斥）。

- [ ] **Step 1: 裁剪与配置**

按 Files 清单删除/修改；`pnpm dev:h5` 起得来、首页为自定义 nav 空骨架。

- [ ] **Step 2: 主题 token 落地**

`scorecard.scss`：每主题一个类，变量组从设计稿逐值抄录（t1=breakout :root；t2=`--ink-900:#0D1420;--brass:#D98E5F;--win-dark:#F0A93B` 等整组；t3/t4 同理）；`--vermillion`/红中印相关值写死同一份放类外公共段。工具类只引用变量。

- [ ] **Step 3: themeStore + App 挂载**

store state `{ themeId: 1 }` persist；App.vue `onLaunch` 读 store → 根节点加 `theme-t{n}` class（H5 挂 `document.documentElement`，mp 挂 `page`/`App` 根——用条件编译，builder 按 uni-app 惯例）；`useScorecardTheme().setTheme` 即时换 class。

- [ ] **Step 4: 验证**

Run: `pnpm type-check && pnpm lint && pnpm build:h5`
Expected: 全过。手动：dev:h5 切 4 主题，首页骨架背景/文字随主题变。

- [ ] **Step 5: 提交**

```bash
git add -A modules/scorecard/appui/uniapp
git commit -m "feat(scorecard-appui): 脚手架裁剪 + 4 主题 token 基座"
```

---

### Task 4: API 层 + authStore

**Files:**
- Create: `src/api/scorecard/types.ts`（对齐后端 DTO：`RoomSnapshot/RoomMember/RoomLogItem/ExpenseReq/ExpenseResult/ConfigReq/StatResult/FriendItem/MeResult/LoginResult/PageData<T>`——字段名与 json 标签逐字对齐 spec §2 与后端 service 文件）
- Create: `src/api/scorecard/http.ts`（手写 method 工厂：`scoreGet<T>(url, params?)` / `scoreSend<T>(type, url, data?)`，内部 `alovaInstance.Get/Post/Put`，信封解包 `{code,msg,data}`，code!=0 throw）
- Create: `src/api/scorecard/auth.ts`（`loginByMobile` `wxLogin` `getMe` `updateMe`）
- Create: `src/api/scorecard/room.ts`（`createRoom` `joinRoom` `snapshot` `logs` `expense` `updateConfig` `settle` `leave`）
- Create: `src/api/scorecard/record.ts`（`myRecords` `recordDetail` `deleteRecord` `stat`）
- Create: `src/api/scorecard/friend.ts`（`listFriends` `addFriend` `removeFriend` `setBlack` `setRank`）
- Create: `src/api/scorecard/wxacode.ts`（`getRoomWxacode(roomId): Promise<ArrayBuffer>`，responseType 设置照 alova uniapp adapter 用法）
- Create: `src/store/scorecard/auth.ts`（authStore：`{token, refreshToken, user: MeResult|null, lastRoomId}` persist id `scorecard-auth`；actions: `setLogin(LoginResult)` `fetchMe()` `logout()`）
- Modify: `src/api/core/instance.ts`（beforeRequest 注入 `Authorization: Bearer <token>`——token 从 authStore 读；保留 Content-Type/_t 逻辑）
- Modify: `src/api/core/handlers.ts`（401 保持清态跳 login；响应信封解包若已在此层则与 http.ts 职责划分清楚——裁定：**信封解包在 http.ts**，handlers 只管 HTTP 层错误与 401）

**Interfaces:**
- Consumes: Task 1/2 后端契约；脚手架 `handleAlovaError`/`handleAlovaResponse`。
- Produces（后续所有页面任务消费）:

```ts
// types.ts 关键签名（字段以 json 标签为准）
interface RoomSnapshot { id: number; room_code: string; creator_id: number; status: number; version: number; expire_time: string; tea_target_amount: number; tea_percent: number; tea_pool_total: number; tea_auto_enabled: boolean; max_lose_score: number; round_no: number; pending_apply_round_no: number; members: RoomMember[]; recent_logs: RoomLogItem[] }
interface ExpenseReq { score: number; tea: number; to_user_id: number; tea_from_user_id: number; client_request_id: string }
// authStore: setLogin({token,refresh_token,expires_in}) → 持久化；fetchMe() → getMe 写 user
```

- [ ] **Step 1: types + http 工厂**

信封：`{code:0,msg,data}` 成功解包返回 `data`；非 0 throw `ApiError{code,msg}`。401 由 handlers 清 authStore（import store 需注意循环依赖——用动态 import 或事件，builder 定，但 API 必须保证 401 后 `uni.redirectTo` login）。

- [ ] **Step 2: 各域 method + authStore**

签名见 Produces；`expense` 的 `client_request_id` 由调用方生成（Task 8）传入，API 层不生成。

- [ ] **Step 3: 验证**

Run: `pnpm type-check && pnpm lint`
Expected: 过。手动：dev:h5 控制台跑一次 `Apis`/直接函数调 login（有本地后端时）打到真实接口。

- [ ] **Step 4: 提交**

```bash
git add modules/scorecard/appui/uniapp/src
git commit -m "feat(scorecard-appui): scorecard API 层 + authStore"
```

---

### Task 5: 登录页 + useSceneRoom 骨架

**Files:**
- Create: `src/pages/login/index.vue`
- Create: `src/composables/scorecard/useSceneRoom.ts`（scene 暂存/消费骨架）
- Modify: `src/api/core/handlers.ts`（确认 401 跳 `pages/login/index`）

**Interfaces:**
- Consumes: Task 4 `loginByMobile/wxLogin/getMe`、authStore。
- Produces:
  - `useSceneRoom()`: `{ pendingScene: Ref<string|null>, consumeScene(): string|null }`——`pendingScene` 模块级共享（非响应也可，用 ref + 模块单例）；App onShow 写入、进房流程消费并清空（**只消费一次**）。
  - 登录成功统一走 `afterLogin()`: 存 token → `fetchMe()` → 若 `pendingScene` 有房号则 `joinRoom` 进房间页，否则 `router.replaceAll('index')`。

- [ ] **Step 1: 页面实现**

- mp 端：大号「微信一键登录」按钮 → `uni.login()` 取 code → `wxLogin(code)` → `afterLogin()`；失败 toast 重试。
- 折叠「手机号登录」表单（mobile+password）→ `loginByMobile` → 同 `afterLogin()`；H5 默认展开密码表单（无 wx.login）。
- 视觉：骨白 pcard 底 + 黄铜 CTA（token 类），自定义 nav。
- `App.vue` onShow：`options.scene` decodeURIComponent 后形如 `qr=7AVW` → 提取房号写 `pendingScene`。

- [ ] **Step 2: 验证**

Run: `pnpm type-check && pnpm lint && pnpm build:h5 && pnpm build:mp-weixin`
Expected: 过。手动 H5：密码登录成功回首页；错误密码 toast。
验收（Review Focus #3/#4）：scene 暂存单元逻辑——手动在 console 写 pendingScene 后走 afterLogin 路径只消费一次。

- [ ] **Step 3: 提交**

```bash
git add modules/scorecard/appui/uniapp/src
git commit -m "feat(scorecard-appui): 登录页（微信一键+密码）+ scene 骨架"
```

---

### Task 6: 首页（01 扇面牌桌）

**Files:**
- Modify: `src/pages/index/index.vue`（全量实现）
- Create: `src/components/scorecard/PlayCard.vue`（pcard 底衬/角标/花色条纹，props: `suit? rank? label? tone?: 'tile'|'brass'|'win'`）

**Interfaces:**
- Consumes: `createRoom/joinRoom/stat/myRecords/listFriends`、authStore、`useSceneRoom`、`useScorecardTheme`。
- Produces: 首页路由 `pages/index/index` 为小程序码 scene 落地页（Task 2 后端 page 参数即此路径）。

- [ ] **Step 1: 实现页面**

- 身份牌：昵称/头像（authStore.user）+ `stat` 总分/胜率 + 注册天数（me.created_at）+ 好友数。
- 扇形 CTA：「我要开房」`createRoom({tea_percent:0,tea_target_amount:0,tea_auto_enabled:false,max_lose_score:0})` → 进房间页；「扫码进房」mp 端 `uni.scanCode` → `result.path`/`scene` 解析 `qr=` 房号 → `joinRoom` → 进房；H5 点击弹房号输入（`wd-popup`）→ `joinRoom`。
- pill：好友排名 → `pages/friends`；战绩流水 → `pages/records`；使用手册 → `pages/manual`（后两页 Task 10/15 前先 push 占位路由存在即可）。
- 最近战绩：`myRecords(page=1,size=5)` 条目（日期·±分·结算类型映射文案）。
- `afterLogin`/onShow：`pendingScene` 有值 → 登录态下直接 `joinRoom` 进房（Task 5 骨架消费点）。
- 广告位虚线占位卡；自绘 nav。

- [ ] **Step 2: 验证**

Run: `pnpm type-check && pnpm lint && pnpm build:h5`
Expected: 过。手动（本地后端）：开房成功进房间页；扫码按钮 H5 弹层可输房号进房。
验收（Review Focus #4）：未登录点开房 → 跳登录，登录后回跳原动作（或简化回首页，记录为已知简化）。

- [ ] **Step 3: 提交**

```bash
git add modules/scorecard/appui/uniapp/src
git commit -m "feat(scorecard-appui): 首页扇面牌桌"
```

---

### Task 7: 房间页（02 俯视牌桌）+ 轮询 + 合规横幅

**Files:**
- Create: `src/pages/room/index.vue`
- Create: `src/composables/scorecard/useRoomPolling.ts`
- Create: `src/store/scorecard/room.ts`（roomStore：当前 `snapshot: RoomSnapshot|null`、`roomId`，persist lastRoomId）
- Create: `src/components/scorecard/RoomRoster.vue`（横滚座位：头像/昵称/cur_score/房主徽标/me 描红 props: `members, myUserId, creatorId`）
- Create: `src/components/scorecard/RoomLedger.vue`（流水账板 props: `logs, nicknameMap`；type 文案映射 1记分/2提示/3进房/5结算/6上限/8超时关）
- Create: `src/components/scorecard/ComplianceBanner.vue`（可关，本会话 storage key；「了解详情」→ manual）
- Create: `src/components/scorecard/ScoreText.vue`（Oswald 数字 props: `value: number`，win/lose/jade 色）

**Interfaces:**
- Consumes: `snapshot/logs/joinRoom`、Task 4 types。
- Produces:
  - `useRoomPolling(roomId: number)`: `{ start() stop() refresh() }`——3s `setInterval` 拉 `snapshot`，**仅当 `next.version > current.version` 才写 store**（单调，防旧覆盖新——Review Focus #2）；`onShow`/下拉刷新立即 `refresh()`；页面 `onHide/onUnload` 停。
  - roomStore 被支出/结算任务复用。

- [ ] **Step 1: 实现轮询与 store**

version 单调：`if (snap.version >= old.version) store.snapshot = snap`（等值=去重跳过，旧值丢弃）。

- [ ] **Step 2: 实现房间页**

- 状态门：`status!=0` → `router.replaceAll('settle?roomId=')`。
- 俯视牌桌：Roster + 中央最近一手（最新 type=1 的 from/to/score，flyer 简单动画可省 builder 定）+ 茶饭池铭牌（`tea_pool_total` + `tea_percent` 摘要；点击 → emit 事件先 toast「Task 11 接入」占位，Task 11 换真弹层）。
- 流水：`recent_logs` 20 条 + 「查看更多」→ `wd-popup` 内 `logs` 分页上拉。
- 筹码轨：中央「支出」→ expense 页（预选空）；右侧「我的」→ 占位（Task 13 换 AvatarPopup）；左侧客服占位。
- 成员点击 → expense 页预填 `toUserId` query。
- ComplianceBanner 置顶。

- [ ] **Step 3: 验证**

Run: `pnpm type-check && pnpm lint && pnpm build:h5`
Expected: 过。手动双浏览器/双端：A 记分后 3s 内 B 房间页更新；B 的旧轮询响应不回滚 A 刚写入的 snapshot（快速连续记分场景）。

- [ ] **Step 4: 提交**

```bash
git add modules/scorecard/appui/uniapp/src
git commit -m "feat(scorecard-appui): 房间俯视牌桌 + 快照轮询"
```

---

### Task 8: 支出页（04 筹码加减档）

**Files:**
- Create: `src/pages/expense/index.vue`
- Create: `src/components/scorecard/ExpenseLane.vue`（分数档/茶饭档）
- Create: `src/components/scorecard/ChipButton.vue`（±筹码钮 props: `step? size? tone?`，长按连加）

**Interfaces:**
- Consumes: `expense`、roomStore（成员列表/tea_auto_enabled/tea_percent）。
- Produces: 无下游新接口；提交成功后写回 roomStore.snapshot（响应内嵌）+ 调 `useRoomPolling().refresh()` 重置。

- [ ] **Step 1: 实现页面**

- 分数档：当前值大数字 + `ChipButton` −/＋（步进 10，长按 500ms 后 200ms 连发）+ 预设 10/50/100/200 chip；`score>0` 才可提交。
- 收款方：query `toUserId` 预选，否则成员横选（头像+昵称 chip 组，排除自己）。
- 茶饭档：`tea_auto_enabled=false` 时显示手动 `tea` 输入 + `tea_from_user_id`（默认支出方）；true 时显示「按 X% 自动抽取」说明。
- 提交：页面进入时 `const reqId = genUuid()` 生成一次存 `ref`；点击 → `expense({score,tea,to_user_id,tea_from_user_id,client_request_id:reqId})`；**失败 toast 后重试复用同一 reqId**（Review Focus #1）；成功 → `uni.navigateBack` + toast「已记分」+ voice 占位注释（Task 13 接）。
- 底部「只记分数，不涉及金钱」。

- [ ] **Step 2: 验证**

Run: `pnpm type-check && pnpm lint`
Expected: 过。手动：记分后房间分变；**断网重试不双记**（后端幂等回放同 log_id，观察流水条数）。

- [ ] **Step 3: 提交**

```bash
git add modules/scorecard/appui/uniapp/src
git commit -m "feat(scorecard-appui): 支出页筹码加减档（幂等提交）"
```

---

### Task 9: 结算页（05 竖排开牌）

**Files:**
- Create: `src/pages/settle/index.vue`
- Create: `src/utils/scorecard/iou.ts`（贪心简化转账纯函数；前端无测试基建，验收用 Step 3 的控制台手测用例，不建测试文件）

**Interfaces:**
- Consumes: `snapshot/settle/leave`、`myRecords`（历史房间 pill）。
- Produces:
  - `calcIou(members: {user_id:number,nickname:string,cur_score:number}[]): {from_user_id,to_user_id,amount}[]`——只取非零分，贪心最大债权/债务配对，最小转账数；空/全零 → `[]`。

- [ ] **Step 1: 实现 IOU 纯函数**

签名如上；纯函数放 utils（无 uni API，可在 console 验收）。

- [ ] **Step 2: 实现页面**

- 排行大牌：`members` 按 `cur_score` 降序，第一名为 win-card（骨白大牌+黄铜描边）；竖排「战斗胜利」标题；全零/空态「本局无胜负」。
- IOU 行：`calcIou(members)` 结果列表 + 「策略参考，可不按」。
- 操作托盘：直接退房 `leave`（二次确认）/ 结算单（成员分数列表弹层）/ 总分排行榜（同列表弹层）/ 房间流水（logs 弹层）/ 历史房间 → records 页 / 使用手册 → manual。
- 房主（creator_id==myUserId 且 status==0）：「结束牌局」→ dialog 二次确认 → `settle` → 刷新状态。
- `status!=0`：隐藏可写操作，只读展示。

- [ ] **Step 3: 验收 IOU 手算**

Run: `pnpm dev:h5` 起后控制台执行：
```js
// 期望: A(100)→C(40) 100? 手算: 债务 A:-100? 用例: [{u:1,s:100},{u:2,s:-60},{u:3,s:-40}] → [{from:2,to:1,amount:60},{from:3,to:1,amount:40}]
```
另两组：全零 → `[]`；两人对敲 `[{1,50},{2,-50}] → [{2→1,50}]`。
Run: `pnpm type-check && pnpm lint` → 过。

- [ ] **Step 4: 提交**

```bash
git add modules/scorecard/appui/uniapp/src
git commit -m "feat(scorecard-appui): 结算竖排开牌 + IOU 贪心策略"
```

---

### Task 10: 战绩页（07 左滑）

**Files:**
- Create: `src/pages/records/index.vue`
- Create: `src/components/scorecard/RecordSwipable.vue`（`wd-swipe` 包装行 + 三操作按钮 props: `record` emit `detail/rank/delete`）

**Interfaces:**
- Consumes: `myRecords/recordDetail/deleteRecord/stat`。
- Produces: 无。

- [ ] **Step 1: 实现页面**

- stat-strip：`stat`（总局=win+lose+draw / 胜率百分 / 净成绩）。
- 筛选 chip：「全部」有效 + 玩法 chip 置灰（决策 #8）。
- 列表：分页（onReachBottom）；行=日期 + ±分（ScoreText）+ 结算类型文案（0 正常结算/1 房主退房自动/2 退房/3 零数据——以 record 模型 settle_type 实际枚举为准，builder 对照 `modules/scorecard/model`）。
- 左滑：流水 → popup 展示 `recordDetail`（user_id 序号+±分，无昵称）；排行 → detail 按分排序名次+分数；删除 → dialog 确认 → `deleteRecord` → 刷新列表。
- 「自定义统计」置灰「开发中」。

- [ ] **Step 2: 验证**

Run: `pnpm type-check && pnpm lint` → 过。手动：结算过的房间出现在列表；左滑三操作各走通；删除后列表少一行。

- [ ] **Step 3: 提交**

```bash
git add modules/scorecard/appui/uniapp/src
git commit -m "feat(scorecard-appui): 战绩页左滑流水/排行/删除"
```

---

### Task 11: 茶饭设置弹层（09）

**Files:**
- Create: `src/components/scorecard/TeaConfigSheet.vue`（props: `snapshot, myUserId, show` emit `update:show`）
- Modify: `src/pages/room/index.vue`（铭牌点击 → 显示弹层；非房主只读模式）

**Interfaces:**
- Consumes: `updateConfig`（ConfigReq 指针语义：只提交改动项）。
- Produces: 房间页铭牌 → `TeaConfigSheet` 正式接线（替换 Task 7 占位 toast）。

- [ ] **Step 1: 实现弹层**

- 底部 `wd-popup`：目标金额（number）、比例 percent（0–100 stepper）、自动抽取 switch、单人累计上限。
- 仅房主可编辑（`snapshot.creator_id === myUserId`）；非房主只读。
- 保存：diff 出改动字段组 `ConfigReq` → `updateConfig` → 用响应 snapshot 刷新 roomStore + toast「下一把生效」（数值项）/「已生效」（仅开关）；`pending_apply_round_no > round_no` 时显示「已暂存，第 N 把生效」。

- [ ] **Step 2: 验证**

Run: `pnpm type-check && pnpm lint` → 过。手动房主改比例 → 下一笔支出茶饭按新比例（或 pending 文案出现）；非房主打开为只读。

- [ ] **Step 3: 提交**

```bash
git add modules/scorecard/appui/uniapp/src
git commit -m "feat(scorecard-appui): 茶饭设置弹层（updateConfig）"
```

---

### Task 12: 邀请弹层（03 小程序码）+ scene 进房闭环

**Files:**
- Create: `src/components/scorecard/QrInvitePanel.vue`
- Modify: `src/pages/room/index.vue`（房号区/••• → 弹层）
- Modify: `src/pages/index/index.vue`（onShow 消费 pendingScene 的完整接线——Task 5 骨架在此闭环）

**Interfaces:**
- Consumes: `getRoomWxacode`（ArrayBuffer → base64 dataURL 展示）、`useSceneRoom.pendingScene`、`joinRoom`。
- Produces: 无。

- [ ] **Step 1: 实现弹层**

- 扇形牌背景 + 房号大字 + `<image :src="dataUrl">`（getRoomWxacode 成功转 base64；**失败 → 降级房号大字+复制按钮**，Review Focus #4）。
- 「转发给好友」`button open-type="share"`（onShareAppMessage 在房间页定义，path 带房号 query）；H5 → 复制房号。
- 文案「邀请好友扫描牌背小程序码加入房间」。

- [ ] **Step 2: scene 闭环**

首页 onShow：`pendingScene` 有值且已登录 → `joinRoom(code)` → 进房；未登录 → 留存 scene 走登录（Task 5 afterLogin 已接）；消费后立即清空（防 onShow 重复触发）。

- [ ] **Step 3: 验证**

Run: `pnpm type-check && pnpm lint && pnpm build:mp-weixin` → 过。
手动（mp 真机/开发者工具，本地后端 + trial 配置）：房间页出码（后端 200 时）；开发者工具「编译模式→扫小程序码」模拟 scene 进房一次成功且不重复进。
验收（Review Focus #3）：登录前触发 scene → 登录后自动进房且只进一次。

- [ ] **Step 4: 提交**

```bash
git add modules/scorecard/appui/uniapp/src
git commit -m "feat(scorecard-appui): 邀请小程序码弹层 + scene 进房闭环"
```

---

### Task 13: 头像弹窗（10）+ 语音 5 档

**Files:**
- Create: `src/components/scorecard/AvatarPopup.vue`
- Create: `src/components/scorecard/VoiceToggle.vue`
- Create: `src/composables/scorecard/useVoice.ts`
- Create: `src/store/scorecard/voice.ts`（voiceStore：`{entertain,ledger,roomDyn}` persist id `scorecard-voice`）
- Modify: `src/pages/room/index.vue`（「我的」/点自己头像 → AvatarPopup；替换 Task 7 占位）
- Modify: `src/pages/expense/index.vue`（成功后 `speak('…', 'ledger')` 接线）
- Modify: `manifest.config.ts`（mp-weixin plugins 声明 WechatSI 占位——appid 由运营侧后台添加，代码先声明）
- Modify: `src/App.vue`（onShow 场景下把 `options.scene` 写入 pendingScene 的逻辑确认在此文件——若 Task 5 已放此处则不动）

**Interfaces:**
- Consumes: `updateMe`、`leave`、authStore、`logout`。
- Produces:
  - `useVoice()`: `{ speak(text: string, category: 'entertain'|'ledger'|'roomDyn'): void, currentMode: Ref<VoiceMode> }`，`VoiceMode = 'all-off'|'all-on'|'entertain-off'|'ledger-off'|'ops-only'`；mode→三 bool 映射与 5 档循环 `nextMode(mode): VoiceMode`（顺序：all-off → all-on → entertain-off → ledger-off → ops-only → all-off）。speak：档位过滤该 category；mp 端 WechatSI 插件可用则 TTS，不可用 toast 一次「语音插件未启用」后本会话静默；H5 `speechSynthesis`，不可用静默。**任何失败不 throw**。
  - AvatarPopup：头像+昵称+「修改昵称」（输入 popup → `updateMe({nickname})` → 刷新 authStore.user）；VoiceToggle（5 档文案循环）；主题入口（4 色卡横排 → `useScorecardTheme().setTheme`——**我的裁定，spec §3.9.3**）；在房：「退出房间」`leave` 二次确认 → 回首页；不在房：「退出登录」清 authStore → login；「查看资料」只读（昵称/ID/注册时间）。

- [ ] **Step 1: voiceStore + useVoice**

5 档映射 + nextMode 纯逻辑先写（console 可验）；speak 降级链按 Produces。

- [ ] **Step 2: AvatarPopup + VoiceToggle + 页面接线**

房间页「我的」→ popup；expense 成功 → `speak(\`${昵称}支出${score}分\`, 'ledger')`（模板数字拼接，无货币词）；结算胜利 → entertain 类接线点留注释（settle 页可顺手接）。

- [ ] **Step 3: 验证**

Run: `pnpm type-check && pnpm lint && pnpm build:mp-weixin` → 过。
手动：5 档循环文案与顺序对稿（手册 §3.8）；改昵称后房间页刷新可见；H5 下 speak 不抛错；mp 未装插件时 toast 一次后静默。

- [ ] **Step 4: 提交**

```bash
git add modules/scorecard/appui/uniapp/src modules/scorecard/appui/uniapp/manifest.config.ts
git commit -m "feat(scorecard-appui): 头像弹窗 + 语音 5 档播报"
```

---

### Task 14: 好友排名页（08）

**Files:**
- Create: `src/pages/friends/index.vue`
- Create: `src/components/scorecard/FriendRow.vue`

**Interfaces:**
- Consumes: `listFriends/addFriend/removeFriend/setBlack/setRank`。
- Produces: 无。

- [ ] **Step 1: 实现页面**

- 列表：`listFriends`（已按 total_score 降序）行=头像/昵称/胜率/总分；自己若在榜标「· 我」（对比 authStore.user.user_id——注意好友列表可能不含自己，我方总分用 `stat` 补）。
- 顶部：好友数 + 「加入排名/关闭后不参与好友榜单」汇总开关（**我的裁定**：对全部我方 friend 行批量 `setRank` 联动——逐行调用，失败行 toast 继续）。
- 左滑（FriendRow）：删除 `removeFriend` 二次确认 / 拉黑 `setBlack(true)`；已拉黑行显示标记 + 解除（`setBlack(false)`）。
- 空态 + 头部「+」→ 输入对方 user_id → `addFriend`（错误码 30109/30108/40004 toast 透出 msg）。

- [ ] **Step 2: 验证**

Run: `pnpm type-check && pnpm lint` → 过。手动两账号：A 加 B → 双方列表互见；A 删 B → 互不见；A 拉黑 B → B 加 A 被 30108；开关关闭后榜单不可见语义（对稿文案）。

- [ ] **Step 3: 提交**

```bash
git add modules/scorecard/appui/uniapp/src
git commit -m "feat(scorecard-appui): 好友排名页（左滑删除/拉黑+参与开关）"
```

---

### Task 15: 手册页（06）+ 收尾构建回归

**Files:**
- Create: `src/pages/manual/index.vue`（静态长页）
- Modify: `src/pages/index/index.vue` / `settle/index.vue` / `ComplianceBanner.vue`（确认手册入口都可达——前序任务已埋点则只验）
- Modify: `README.md`（appui 子目录加一段运行说明：后端启动、env 端口、mp 开发者工具、微信插件/体验版注意事项）

**Interfaces:**
- Consumes: PlayCard（章节头）。
- Produces: 无。

- [ ] **Step 1: 手册页转写**

完整转写 breakout 06 屏章节文案（①重要提示 ②结算·退房·历史 ③茶饭设置 ④房间语音三类+5 档 ⑤好友排名 ⑥技巧/反馈）；版本号取 `package.json`；花色卡章节头（PlayCard suit 角标）；自定义 nav。

- [ ] **Step 2: 全量构建与回归清单**

Run: `pnpm type-check && pnpm lint && pnpm build:h5 && pnpm build:mp-weixin`
Expected: 全过。

真机/开发者工具回归清单（逐项勾）：
- [ ] 微信一键登录 → me 落 store
- [ ] 开房 → 记分 → 3s 内他端刷新 → 结算 → 战绩可见
- [ ] 邀请弹层出码；另一端 scene 进房一次成功
- [ ] 茶饭改比例 → 下笔生效/pending 提示
- [ ] 4 主题全页面切换无残留旧色；合规横幅/红中印恒红
- [ ] 语音 5 档循环；未装插件静默/一次提示
- [ ] 好友加/删/拉黑/开关
- [ ] H5：密码登录、输房号进房、复制房号、无 wx API 不抛错

- [ ] **Step 3: 提交**

```bash
git add modules/scorecard/appui/uniapp
git commit -m "feat(scorecard-appui): 使用手册页 + 构建回归收尾"
```
