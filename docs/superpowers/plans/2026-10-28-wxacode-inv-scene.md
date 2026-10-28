# 打牌记账：小程序码 scene 带邀请人（wxacode inv）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 房间小程序码 scene 从 `qr=<房号>` 扩展为 `qr=<房号>&inv=<请求者UserID>`，进程内缓存键从房号改为完整 scene，防止 A 用户的码命中到 B 用户。

**Architecture:** 只动 `wxacode.go` 控制器：出码时把 `claims.UserID` 拼进 scene；缓存 map 的键随之从 `roomCode` 换成 scene 全串（scene 已含房号+用户，天然按人隔离）。绑好友与解析是前端职责，由同日 OpenSpec 变更（uniapp）承接，本计划不涉前端。

**Tech Stack:** Go / gin / gorm（现有 `app_test.go` 测试基建，stubWxacoder 桩）

**Spec:** 2026-10-28 会话确认的设计——场景协议统一为 `qr=<房号>`（旧码兼容）/ `qr=<房号>&inv=<uid>` / `inv=<uid>`；scene 仍以 `qr=` 前缀开头；前端消费 `inv` 调现有 `POST /friends` 静默绑好友。微信 getwxacodeunlimit scene 上限 32 字符：`qr=`(3)+房号(≤8)+`&inv=`(5)+uint64(≤20) 理论最宽超限，但房号实际 ≤8 且随机 4 位、本系统 user_id 为自增整型（≤10 位量级 ≈ 26 字符），不引入截断逻辑。

## Global Constraints

- 直接在 main 上工作，提交落 main，不建功能分支。
- TDD：每个行为先写失败测试，确认失败后再实现。
- scene 前缀契约不变：仍以 `qr=` 开头（前端 `decoded.startsWith('qr=')` 解析依赖此契约）。
- page（`pages/index/index`）与 env_version 行为不得回归（现有 TestWxacode_EnvVersionConfigurable 钉死）。
- 错误路径门禁不放松：30101 房不存在、30102 非成员、50001 渠道未配置（现有测试覆盖，改后必须仍绿）。

## Review Focus

- **缓存按 scene 隔离**：两个成员取同一房间的码必须各自打微信（calls=2），scene 分别为 `qr=<code>&inv=100` 与 `qr=<code>&inv=300`——`TestWxacode_CacheScopedByUser` 钉死。
- **scene 格式**：新 scene 必须 `qr=<房号>&inv=<请求者UserID>` 精确拼接（无空格、inv 在尾部）——`TestWxacode_OK_PNG` 断言钉死。
- **旧前端兼容**：scene 仍以 `qr=` 开头；旧版小程序端 `slice(3)` 取到的串是 `<房号>&inv=<uid>`——这是前端 OpenSpec 变更负责解析的部分，后端只需保证前缀与分隔符 `&inv=` 稳定。
- **claims.UserID 为出码请求者**（不一定是房主）：场景语义是「谁分享谁被绑」；非成员本来就 30102 进不来，无额外分支。

---

### Task 1: scene 拼装带 inv + 缓存键改 scene

**Files:**
- Modify: `modules/scorecard/controller/app/wxacode.go:71-94`（Wxacode 方法取码与缓存调用）、`:97-113`（cachedWxacode/storeWxacode 签名）
- Test: `modules/scorecard/controller/app/app_test.go:214-242`（TestWxacode_OK_PNG）、同文件新增 TestWxacode_CacheScopedByUser

**Interfaces:**
- Consumes: `c.claims(ctx)` → `cl.UserID uint64`（已有）；`c.rooms.Snapshot` → `snap.RoomCode`（已有）；`c.wx.Wxacode(ctx, app, scene, page, envVersion)`（scene 参数契约不变，只变内容）
- Produces: scene 字符串 `qr=<RoomCode>&inv=<UserID>`（前端 OpenSpec 变更按此解析）；`cachedWxacode(scene string) ([]byte, bool)` / `storeWxacode(scene string, png []byte)`（键从 roomCode 改为 scene，签名同型换义）

- [ ] **Step 1: 改写/新增失败测试**

TestWxacode_OK_PNG（app_test.go:231）断言改为：

```go
if wx.scene != "qr=WXYZ&inv=100" || wx.page != "pages/index/index" || wx.env != "trial" {
    t.Fatalf("取码入参: scene=%q page=%q env=%q", wx.scene, wx.page, wx.env)
}
```

新增（tokA=UserID 100 开房；tokC=UserID 300 同租户 join 后也取码；两人 scene 不同 → 缓存不得互串）：

```go
// TestWxacode_CacheScopedByUser 缓存键含 inv：同房两人取码各自打微信，不互串缓存。
func TestWxacode_CacheScopedByUser(t *testing.T) {
	r, db, _, ctrl := newAppRouter(t)
	row := createRoomAs(t, r, db, "tokA", "CACH")
	if w := doReq(r, http.MethodPost, "/api/app/scorecard/rooms/join", "tokC", "",
		`{"room_code":"CACH"}`); w.Code != http.StatusOK || codeOf(t, w) != 0 {
		t.Fatalf("tokC 加入应成功: %d %s", w.Code, w.Body.String())
	}
	wx := &stubWxacoder{png: []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}}
	ctrl.wx = wx
	pngPath := "/api/app/scorecard/rooms/" + itoa(row.ID) + "/wxacode"
	if w := doReq(r, http.MethodGet, pngPath, "tokA", "", ""); w.Code != http.StatusOK {
		t.Fatalf("tokA 取码应 200: %d", w.Code)
	}
	if w := doReq(r, http.MethodGet, pngPath, "tokC", "", ""); w.Code != http.StatusOK {
		t.Fatalf("tokC 取码应 200: %d", w.Code)
	}
	if wx.calls != 2 {
		t.Fatalf("不同用户取码不得命中同一缓存: calls = %d", wx.calls)
	}
	if wx.scene != "qr=CACH&inv=300" {
		t.Fatalf("末次 scene = %q, want qr=CACH&inv=300", wx.scene)
	}
	// 同一用户二次取码仍命中缓存（calls 不再增长）。
	if w := doReq(r, http.MethodGet, pngPath, "tokC", "", ""); w.Code != http.StatusOK {
		t.Fatalf("tokC 二次取码应 200: %d", w.Code)
	}
	if wx.calls != 2 {
		t.Fatalf("同用户二次应命中缓存: calls = %d", wx.calls)
	}
}
```

（若 join 端点 body 形状与 `{"room_code":"..."}` 不符，以 app_test.go 里既有 join 用例为准对齐，断言语义不变。）

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./modules/scorecard/controller/app/ -run 'TestWxacode' -v`
Expected: TestWxacode_OK_PNG FAIL（scene=`qr=WXYZ`≠`qr=WXYZ&inv=100`）；TestWxacode_CacheScopedByUser FAIL（calls=1 或 scene 无 inv）；EnvVersion/错误路径用例保持原绿。

- [ ] **Step 3: 实现 scene 拼装与缓存键改造**

`wxacode.go` Wxacode 方法：

```go
scene := fmt.Sprintf("qr=%s&inv=%d", snap.RoomCode, cl.UserID)
if png, ok := c.cachedWxacode(scene); ok { ... }
png, err := c.wx.Wxacode(ctx.Request.Context(), app, scene, wxacodePage, c.wxEnvVersion)
...
c.storeWxacode(scene, png)
```

`cachedWxacode`/`storeWxacode` 形参 `roomCode string` 改名 `scene string`（map 语义随键变化，类型不变）；顶部 `import` 补 `fmt`。同步更新 godoc `@Description` 一句：`scene=qr+房号&inv+请求者UserID`。

- [ ] **Step 4: 跑测试确认全绿**

Run: `go test ./modules/scorecard/... -count=1`
Expected: 全部 PASS（含好友/进房/错误路径既有套件，确认无回归）。

- [ ] **Step 5: Commit**

```bash
git add modules/scorecard/controller/app/wxacode.go modules/scorecard/controller/app/app_test.go
git commit -m "feat(scorecard): 小程序码 scene 带邀请人 inv——缓存键改 scene 防跨用户串码"
```
