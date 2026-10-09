# vben-admin 后台管理接入实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用 vben-admin（vue-vben-admin v5.7.0 web-antd）完全替换现有 admin-web，后端补 5 个 vben 兼容端点 + 中间件自读档 + 菜单数据回填，交付带全部 11 个管理页面的可用后台。

**Architecture:** 上游 monorepo 精简 vendoring 到仓库根（保留 `apps/web-antd` + `packages/**` + `internal/**` + `scripts/**`）；后端在 `modules/system/controller/vben` 加薄壳控制器，把 vben 契约（`/api/auth/login` 等 5 端点）映射到既有 service；中间件把这 5 端点中的 3 个自读端点列为"token 即可、跳过 data_scope 与 Casbin"档；菜单数据用一条幂等迁移（5 行 + 11 处 icon 更新）+ seed 门控改造回填；前端只改拦截器信封解包、偏好与代理，11 个页面按 vben `useVbenVxeGrid` + `useVbenModal` 范式重建。

**Tech Stack:** Go 1.25 + Gin + FX + GORM + Casbin + golang-migrate（MySQL）；pnpm@10.33.4 workspace + turbo + Vue 3 + Vite + TypeScript + Tailwind v4 + ant-design-vue 4 + vxe-table。

**Spec:** `docs/superpowers/specs/2026-10-09-vben-admin-design.md`（已获批；本计划吸收其全部 10 节，并在下方"Spec Errata"记录 8 处侦察修订——与 spec 冲突处以 Errata 为准）。

## Global Constraints

- **上游唯一来源**：`https://github.com/vbenjs/vue-vben-admin.git`，tag `v5.7.0`，SHA 必须为 `63a38dce49ba109f61607994e21ba921d8e970e9`（注意：仓库名是 `vue-vben-admin`，**不是** `vben-admin`，后者 404）。
- **5 端点契约**（baseURL = `/api`，业务接口路径不带 `/api` 前缀）：
  | 方法 | 路径 | 成功响应 |
  | --- | --- | --- |
  | POST | `/api/auth/login` | `{"code":0,"data":{"accessToken","refreshToken","expiresIn"},"error":null,"message":"ok"}` |
  | GET | `/api/auth/codes` | `{"code":0,"data":["perm:string",...],"error":null,"message":"ok"}` |
  | GET | `/api/user/info` | `{"code":0,"data":{"userId"(字符串),"username","realName","avatar","roles":[],"desc":"","homePath":"","token"},"error":null,"message":"ok"}` |
  | GET | `/api/menu/all` | `{"code":0,"data":RouteRecordStringComponent[],...}`；目录 `component` 恒为 `""`（后端存 `LAYOUT`，壳层输出前置空） |
  | POST | `/api/auth/logout` | 恒 200 信封 `{"code":0,"data":{},...}`（无 token/无效 token 均 no-op） |
- **信封格式**：成功 `{"code":0,"data":...,"error":null,"message":"ok"}`；失败 `{"code":<HTTP状态码>,"data":null,"error":"<文案>","message":"<文案>"}`；HTTP 状态码必须真实（登录失败 401、参数错误 400、鉴权失败 401/403）。
- **中间件三档**：公开（`/health`、`/api/auth/login`、`/api/auth/logout` 等全部非 `/api/admin/**` 且非自读清单）/ 完整门控（`/api/admin/**`：token→claims→租户覆盖→member 403→data_scope→Casbin，现行为不变）/ vben 自读档（**精确匹配** `/api/auth/codes`、`/api/user/info`、`/api/menu/all`：token→claims→租户覆盖→member 403 后直接放行）。失败体保持 `gin.H{"error":"unauthorized"}` / `gin.H{"error":"forbidden"}`。
- **菜单终态 19 行**（`tenant_id=0`）：已有库 = 原 14 行 + 迁移插 5 行；fresh 库 = 迁移 5 行 + seed 建 11 行 + seed 按钮 3 行。验收必须数出 19。
- **icon UPDATE 过滤条件**必须是 `tenant_id=0 AND type IN ('dir','menu') AND icon='<旧值>'`——11 处；`type IN` 过滤防 roles 菜单 `icon='team'` 与会员管理按钮 `icon='team'` 相撞，`dir` 必须在内（`/system` 目录 type='dir'）。
- **`meta.order` 字段**：vben `generate-menus` 读 `meta.order ?? 999` 排序；后端 `VbenRouteMeta` 必须输出 `order`（取自 `Menu.Sort`），既有 `orderNo` 保留。
- **默认首页** `/dashboard/analytics`；`accessMode: 'backend'`；dev 端口 **5666**；`VITE_GLOB_API_URL=/api`；`VITE_NITRO_MOCK=false`。
- **order 模块开关**必须为 true：`alexgo-server/configs/config.yaml:46` 与 `deployments/helm/alexgo-cloud/values.yaml:81`（k8s configmap、values-dev、values-prod 已是 true，勿动）。
- **前端验证命令**：`pnpm --filter @vben/web-antd build`、`pnpm --filter @vben/web-antd typecheck`；后端 `make test`（= `go test ./...`，**必须零外部依赖、不起 DB**）；FX 装配 `go test ./alexgo-server/cmd`。macOS 链接报错时加 `CGO_ENABLED=0` 前缀。
- **`make migrate` 会挂起**：`--migrate-only` 只跳过 HTTP，fx `Run()` 阻塞等退出信号——迁移+种子跑完后进程不退出是**预期行为**，必须后台运行并按日志（`migrate success`、`seed default admin done`）判定完成后杀进程，不得误判为卡死。
- **一期不做编辑**：全部 11 页基线 = 列表 + 创建 + 删除（users 另有重置密码；login/operate 日志只读；orders 只有创建无删除）。全仓库零 PUT/edit 使用，不引入。
- **动作按钮权限**用页面自身菜单 perm **原串**（如 `system:role:*`、`order:order:*`、`system:log:login`）——`/auth/codes` 原样返回，vben `v-access` 逐字包含匹配，不做通配展开。
- **操作列用 vxe slot**（`slots:{default:'actions'}` + `<template #actions="{row}">`）——web-antd 的 CellOperation 适配器不存在。
- **表格数据一律经 `localPage` 本地切页**：后端列表接口回全量数组（无分页参数）。
- **弹窗约定**：`onConfirm` 写 `formApi.validateAndSubmitForm().then(() => modalApi.close()).catch(() => {})`——resolve 不自动关（已从 `modal-api.ts`/`modal.vue` 源码确认：确认按钮只调 `onConfirm`，无自动 close），reject 保持打开；页面 catch **不再 `message.error`**（拦截器已 toast，防双弹）。
- **提交全部直接落 main**（本仓库约定，不建功能分支）。
- **不改的东西**：`alexgo-server/configs/config.yaml` 中真实 DB 密码（已进 git，属既有问题，本计划不碰）；`.github/`（用本仓库自己的）；根 `README.md`（只改行，不覆盖）。

## Spec Errata（侦察修订，本计划已按此执行）

1. **上游仓库名**：spec 未点名 URL——正确仓库是 `vbenjs/vue-vben-admin`（tag v5.7.0 = `63a38dce49ba109f61607994e21ba921d8e970e9`）。
2. **`VbenRouteMeta` 补 `order` 字段**：spec §4 说"已对齐 orderNo"，但 vben 实读 `meta.order ?? 999`——`orderNo` 保留、另加 `order`（json `order,omitempty`），值 = `Menu.Sort`。
3. **spec §9 "父目录无 redirect、手输 /dashboard 空白"过时**：`accessible.ts` 的 `mapTree` 会给"无 redirect 且首子 path 以 `/` 开头"的父路由自动补 `redirect`；子菜单一律用绝对路径（`/dashboard/analytics` 等）即可。
4. **fresh-DB seed 毒化（Option A 定案）**：迁移（fx.Invoke，先于 seed 的 OnStart goroutine）插 5 行会让旧门控 `len(menus)==0` 失败 → fresh 库永远缺 11 个系统菜单。改法：门控提取 `hasSystemDir`（存在 `Path=="/system" && type dir` 即跳过创建）；`rootID` 查找同步改为按 `/system` path（旧的 `ParentID==0 && dir` 取第一个会命中 dashboard）。
5. **`modules.order` 翻 true 有两处**：本地 `alexgo-server/configs/config.yaml:46` **和** `deployments/helm/alexgo-cloud/values.yaml:81`（spec 漏了 helm values——gray 环境无覆盖值，不改则 order 菜单指向死路由）。
6. **spec §7 定制点漏了 `request.ts` 信封拦截器函数式改造**：不改则后端所有无 `code` 字段的业务响应（`{"data":[...]}`、`{"status":"ok"}`）被 `successCode: 0` 判失败、登录响应被 `dataField: 'data'` 解包成 `undefined`——全站不可用。
7. **CellOperation 在 web-antd 不存在**：操作列改用 vxe 原生 slot 方案。
8. **`make migrate` 阻塞语义**：spec 未提——计划所有迁移步骤都按"后台跑 + 日志判定 + 杀进程"写。

## Review Focus

以下 5 类失败最可能咬到使用本软件的人；每条已挂到拥有该代码的任务及其测试：

1. **旧业务响应无 `code` 字段被当失败抛出**（登录后所有列表报错、页面白数据）→ Task 11 实现拦截器/代理；形态断言分布：Task 12 登录冒烟证信封形态，Task 14 用户列表证 `{data:[...]}`、Task 14 创建/删除证 `{status:"ok"}`（均要求成功且控制台无 throw），Task 26 全链路复验三形态。
2. **fresh 库 seed 门控被迁移毒化**（新环境永远缺 11 个系统菜单）→ Task 8：`TestHasSystemDir` 单测（nil / 只有 dashboard / 有 `/system` dir / type 大小写 / type=menu 五断言）。
3. **自读端点被 Casbin/data_scope 误拦，或匹配过宽误伤公开端点** → Task 4：`auth_test.go` 新用例——空策略+合法 token 自读 200（跳过 Casbin）、自读+fakeScope 时 `loader.calls==0`（跳过 data_scope）、`/api/auth/other` 无 token 200（精确匹配非前缀）、自读无 token 401、member token 自读 403。
4. **icon UPDATE 无 `type IN ('dir','menu')` 过滤**（会员管理按钮图标被改、`/system` 目录漏改）→ Task 7：迁移 SQL 逐条核对步骤 + Task 26 验收中 `GET /api/menu/all` 断言 11 个 `lucide:*` 图标在位、`GET /api/auth/codes` 断言 `member:user:manage` 仍在（按钮行未被误伤的间接证据）。
5. **`userId` 返回数字类型导致 vben 类型/显示错**（`BasicUserInfo.userId: string`）→ Task 2：UserInfo 测试断言响应体含 `"userId":"9"`（字符串），且 `desc`/`homePath`/`token` 三必填字段齐备。

---

# 阶段 A：后端兼容层（Go，TDD）

### Task 1: 信封助手 + 登录端点 `POST /api/auth/login`

**Files:**
- Create: `modules/system/controller/vben/controller.go`
- Test: `modules/system/controller/vben/controller_test.go`
- Test: `modules/system/controller/vben/fakes_test.go`

**Interfaces:**
- Consumes: `service.AuthService.Login(ctx, username, password) (*service.LoginResult, *model.User, error)`；`service.AuditService.RecordLogin(ctx, username string, userID uint64, ip, ua string, success bool, msg string) error`；`service.LoginResult{AccessToken, RefreshToken string; ExpiresIn int64}`。
- Produces: `func NewController(authSvc service.AuthService, auditSvc service.AuditService, permSvc service.PermissionService, userSvc service.UserService) *Controller`；`func (ctrl *Controller) Register(r *gin.RouterGroup)`（Task 3 扩全 5 条路由，签名不变）；`func ok(c *gin.Context, data any)` / `func fail(c *gin.Context, status int, msg string)`。

- [ ] **Step 1: 写共享测试替身（fakes_test.go）**

创建 `modules/system/controller/vben/fakes_test.go`：

```go
package vben

import (
	"context"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/service"
)

// fakeAuth 实现 service.AuthService：Login 按注入值返回；Logout 记账。
type fakeAuth struct {
	loginRes    *service.LoginResult
	loginErr    error
	logoutCount int
}

func (f *fakeAuth) Login(_ context.Context, _, _ string) (*service.LoginResult, *model.User, error) {
	if f.loginErr != nil {
		return nil, nil, f.loginErr
	}
	return f.loginRes, &model.User{ID: 7, Username: "admin"}, nil
}

func (f *fakeAuth) Refresh(_ context.Context, _ string) (*service.LoginResult, error) {
	return nil, nil
}

func (f *fakeAuth) Logout(_ context.Context, _ string) error {
	f.logoutCount++
	return nil
}

// fakeAudit 实现 service.AuditService：记录 RecordLogin 的 success 序列。
type fakeAudit struct {
	successes []bool
}

func (f *fakeAudit) RecordLogin(_ context.Context, _ string, _ uint64, _, _ string, success bool, _ string) error {
	f.successes = append(f.successes, success)
	return nil
}

func (f *fakeAudit) RecordOperate(context.Context, uint64, string, string, string, int, int64, string) error {
	return nil
}

func (f *fakeAudit) ListLogin(context.Context, int) ([]*model.LoginLog, error) { return nil, nil }

func (f *fakeAudit) ListOperate(context.Context, int) ([]*model.OperateLog, error) { return nil, nil }

// fakePerm 实现 service.PermissionService（本阶段 Login 不触达，返回空）。
type fakePerm struct {
	codes []string
	routes []*service.VbenRoute
	roles  []*model.Role
}

func (f *fakePerm) UserRoles(context.Context, uint64) ([]*model.Role, error) { return f.roles, nil }

func (f *fakePerm) UserMenus(context.Context, uint64) ([]*model.Menu, error) { return nil, nil }

func (f *fakePerm) UserPermCodes(context.Context, uint64) ([]string, error) { return f.codes, nil }

func (f *fakePerm) UserRoutes(context.Context, uint64) ([]*service.VbenRoute, error) {
	return f.routes, nil
}

func (f *fakePerm) EnsureUserRolePolicy(context.Context, string, []*model.Role) error { return nil }

func (f *fakePerm) RebuildPolicies(context.Context) error { return nil }

func (f *fakePerm) RebuildRolePolicies(context.Context, uint64) error { return nil }

// fakeUser 实现 service.UserService：GetUserByID 按注入值返回。
type fakeUser struct {
	user *model.User
	err  error
}

func (f *fakeUser) ListUsers(context.Context, map[string]any) ([]*model.User, int64, error) {
	return nil, 0, nil
}

func (f *fakeUser) GetUserByID(context.Context, uint64) (*model.User, error) {
	return f.user, f.err
}

func (f *fakeUser) CreateUser(context.Context, string, string, string) (*model.User, error) {
	return nil, nil
}

func (f *fakeUser) UpdateUser(context.Context, uint64, string, int) error { return nil }

func (f *fakeUser) ResetPassword(context.Context, uint64, string) error { return nil }

func (f *fakeUser) SetRoles(context.Context, uint64, []uint64) error { return nil }
```

> 若 `service.UserService` 的方法集与上述签名不完全一致（以 `modules/system/service/service.go` 接口定义为准），以接口为准修正 fake 的方法签名——fake 必须精确实现接口，编译不过就改到过。

- [ ] **Step 2: 写登录端点的失败测试（controller_test.go）**

创建 `modules/system/controller/vben/controller_test.go`：

```go
package vben

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/service"
)

func newTestRouter(auth *fakeAuth, audit *fakeAudit, perm *fakePerm, user *fakeUser) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	NewController(auth, audit, perm, user).Register(r)
	return r
}

func postJSON(r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestLogin_Success_Envelope(t *testing.T) {
	auth := &fakeAuth{loginRes: &service.LoginResult{
		AccessToken: "at-1", RefreshToken: "rt-1", ExpiresIn: 7200,
	}}
	audit := &fakeAudit{}
	w := postJSON(newTestRouter(auth, audit, &fakePerm{}, &fakeUser{}),
		"/api/auth/login", `{"username":"admin","password":"admin123"}`)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Code  int     `json:"code"`
		Error *string `json:"error"`
		Data  struct {
			AccessToken  string `json:"accessToken"`
			RefreshToken string `json:"refreshToken"`
			ExpiresIn    int64  `json:"expiresIn"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Code != 0 || resp.Error != nil {
		t.Errorf("envelope = %+v, want code=0 error=null", resp)
	}
	if resp.Data.AccessToken != "at-1" || resp.Data.RefreshToken != "rt-1" || resp.Data.ExpiresIn != 7200 {
		t.Errorf("data = %+v, want vben accessToken/refreshToken/expiresIn", resp.Data)
	}
	if len(audit.successes) != 1 || !audit.successes[0] {
		t.Errorf("audit successes = %v, want [true]", audit.successes)
	}
}

func TestLogin_WrongPassword_401Envelope(t *testing.T) {
	auth := &fakeAuth{loginErr: errInvalidCredentials}
	audit := &fakeAudit{}
	w := postJSON(newTestRouter(auth, audit, &fakePerm{}, &fakeUser{}),
		"/api/auth/login", `{"username":"admin","password":"nope"}`)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Code  int    `json:"code"`
		Data  any    `json:"data"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Code != 401 || resp.Error == "" || resp.Data != nil {
		t.Errorf("envelope = %+v, want {code:401, data:null, error:非空}", resp)
	}
	if len(audit.successes) != 1 || audit.successes[0] {
		t.Errorf("audit successes = %v, want [false]", audit.successes)
	}
}

func TestLogin_BadBody_400(t *testing.T) {
	w := postJSON(newTestRouter(&fakeAuth{}, &fakeAudit{}, &fakePerm{}, &fakeUser{}),
		"/api/auth/login", `not-json`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
}
```

测试文件顶部需要一个错误变量（Step 2 引用了它，Step 3 一起建）：`var errInvalidCredentials = errors.New("invalid username or password")`——放在 `controller.go` 或测试文件均可，Step 3 实现时一并落。

- [ ] **Step 3: 跑测试确认失败**

Run: `go test ./modules/system/controller/vben/ -run TestLogin -v`
Expected: FAIL（包 `alexGo-cloud/modules/system/controller/vben` 不存在 / `NewController` 未定义）

- [ ] **Step 4: 实现信封助手 + Login**

创建 `modules/system/controller/vben/controller.go`：

```go
// Package vben 提供 vben-admin 前端契约的薄壳端点（5 条），
// 把请求翻译到既有 service，不承载业务逻辑。
package vben

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/service"
)

var errInvalidCredentials = errors.New("invalid username or password")

type Controller struct {
	authSvc  service.AuthService
	auditSvc service.AuditService
	permSvc  service.PermissionService
	userSvc  service.UserService
}

func NewController(
	authSvc service.AuthService,
	auditSvc service.AuditService,
	permSvc service.PermissionService,
	userSvc service.UserService,
) *Controller {
	return &Controller{
		authSvc:  authSvc,
		auditSvc: auditSvc,
		permSvc:  permSvc,
		userSvc:  userSvc,
	}
}

// Register 把 5 个 vben 端点挂到 server 的 /api 根组（全路径 /api/auth/login 等）。
func (ctrl *Controller) Register(r *gin.RouterGroup) {
	r.POST("/auth/login", ctrl.Login)
}

// ok vben 成功信封。
func ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": data, "error": nil, "message": "ok"})
}

// fail vben 失败信封：HTTP 状态码与 code 同值，error/message 同文案。
func fail(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"code": status, "data": nil, "error": msg, "message": msg})
}

// Login POST /api/auth/login —— vben 登录页契约。
// 失败：401 + 信封（errorMessage 拦截器会把 error 文案弹给用户）。
func (ctrl *Controller) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid request body")
		return
	}
	ip, ua := c.ClientIP(), c.GetHeader("User-Agent")
	res, u, err := ctrl.authSvc.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		_ = ctrl.auditSvc.RecordLogin(c.Request.Context(), req.Username, 0, ip, ua, false, err.Error())
		fail(c, http.StatusUnauthorized, errInvalidCredentials.Error())
		return
	}
	var userID uint64
	if u != nil {
		userID = u.ID
	}
	_ = ctrl.auditSvc.RecordLogin(c.Request.Context(), req.Username, userID, ip, ua, true, "ok")
	ok(c, gin.H{
		"accessToken":  res.AccessToken,
		"refreshToken": res.RefreshToken,
		"expiresIn":    res.ExpiresIn,
	})
}
```

注意：`Register` 此时只挂 login（Task 2/3 增量补齐），签名从一开始就是最终形态。

- [ ] **Step 5: 跑测试确认通过**

Run: `go test ./modules/system/controller/vben/ -v`
Expected: PASS（TestLogin_Success_Envelope / TestLogin_WrongPassword_401Envelope / TestLogin_BadBody_400）

- [ ] **Step 6: 全量回归**

Run: `make test`
Expected: PASS（全部包；若 macOS 链接报错用 `CGO_ENABLED=0 make test`）

- [ ] **Step 7: Commit**

```bash
git add modules/system/controller/vben/
git commit -m "feat(system): vben shell login endpoint with envelope"
```

---

### Task 2: Logout + Codes + UserInfo

**Files:**
- Modify: `modules/system/controller/vben/controller.go`（Register 补 3 条路由；新增 3 个 handler）
- Test: `modules/system/controller/vben/controller_test.go`

**Interfaces:**
- Consumes: `service.AuthService.Logout(ctx, accessToken) error`；`service.PermissionService.UserPermCodes(ctx, userID) ([]string, error)`、`UserRoles(ctx, userID) ([]*model.Role, error)`；`service.UserService.GetUserByID(ctx, id uint64) (*model.User, error)`；`auth.Claims{UserID uint64, Username string, TenantID int, UserType int, DeptID int}`（从 `c.Get("claims")` 取，生产由中间件注入——Task 4）。
- Produces: `GET /api/auth/codes`、`GET /api/user/info`、`POST /api/auth/logout` 的行为契约（见 Global Constraints 5 端点表）；`func claimsOf(c *gin.Context) (*auth.Claims, bool)`。

- [ ] **Step 1: 写三个端点的失败测试**

在 `controller_test.go` 追加（import 增加 `"context"`、`"errors"`、`"alexGo-cloud/pkg/auth"`、`"alexGo-cloud/modules/system/model"`）：

```go
// withClaims 复刻中间件注入（Task 4 之前测试自注入）。
func withClaims(r *gin.Engine, path string, cl *auth.Claims, h gin.HandlerFunc) {
	r.GET(path, func(c *gin.Context) {
		if cl != nil {
			c.Set("claims", cl)
		}
		h(c)
	})
}

func TestLogout_NoToken_Still200(t *testing.T) {
	authSvc := &fakeAuth{}
	r := newTestRouter(authSvc, &fakeAudit{}, &fakePerm{}, &fakeUser{})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200（no-op 也要成功）; body=%s", w.Code, w.Body.String())
	}
	if authSvc.logoutCount != 0 {
		t.Errorf("logout called %d times without token, want 0", authSvc.logoutCount)
	}
	if !strings.Contains(w.Body.String(), `"code":0`) {
		t.Errorf("body = %s, want envelope code 0", w.Body.String())
	}
}

func TestLogout_WithBearer_CallsService(t *testing.T) {
	authSvc := &fakeAuth{}
	r := newTestRouter(authSvc, &fakeAudit{}, &fakePerm{}, &fakeUser{})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer the-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || authSvc.logoutCount != 1 {
		t.Errorf("status = %d logoutCount = %d, want 200/1", w.Code, authSvc.logoutCount)
	}
}

func TestCodes_ReturnsRawPermStrings(t *testing.T) {
	r := newTestRouter(&fakeAuth{}, &fakeAudit{}, &fakePerm{
		codes: []string{"order:order:*", "system:role:*"},
	}, &fakeUser{})
	withClaims(r, "/api/auth/codes", &auth.Claims{UserID: 9, Username: "alice"}, func(c *gin.Context) {
		NewController(&fakeAuth{}, &fakeAudit{}, &fakePerm{codes: []string{"order:order:*", "system:role:*"}}, &fakeUser{}).Codes(c)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/auth/codes", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Code int      `json:"code"`
		Data []string `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Code != 0 || len(resp.Data) != 2 || resp.Data[0] != "order:order:*" {
		t.Errorf("resp = %+v, want raw perm strings in data", resp)
	}
}

func TestCodes_NilCodes_ReturnEmptyArray(t *testing.T) {
	ctrl := NewController(&fakeAuth{}, &fakeAudit{}, &fakePerm{codes: nil}, &fakeUser{})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/auth/codes", func(c *gin.Context) {
		c.Set("claims", &auth.Claims{UserID: 1})
		ctrl.Codes(c)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/auth/codes", nil))
	// data 必须是 [] 而非 null：vben accessStore 期望数组。
	if !strings.Contains(w.Body.String(), `"data":[]`) {
		t.Errorf("body = %s, want data:[] (not null)", w.Body.String())
	}
}

func TestUserInfo_StringUserIDAndRequiredFields(t *testing.T) {
	ctrl := NewController(&fakeAuth{}, &fakeAudit{}, &fakePerm{
		roles: []*model.Role{{Code: "admin"}, {Code: "ops"}},
	}, &fakeUser{user: &model.User{Nickname: "管理员", Avatar: "http://a/x.png"}})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/user/info", func(c *gin.Context) {
		c.Set("claims", &auth.Claims{UserID: 9, Username: "alice"})
		ctrl.UserInfo(c)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/user/info", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			UserID   string   `json:"userId"`
			Username string   `json:"username"`
			RealName string   `json:"realName"`
			Avatar   string   `json:"avatar"`
			Roles    []string `json:"roles"`
			Desc     *string  `json:"desc"`
			HomePath *string  `json:"homePath"`
			Token    *string  `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	d := resp.Data
	if d.UserID != "9" {
		t.Errorf("userId = %q (%T), want string \"9\"（vben BasicUserInfo.userId: string）", d.UserID, d.UserID)
	}
	if d.Username != "alice" || d.RealName != "管理员" || d.Avatar != "http://a/x.png" {
		t.Errorf("data = %+v, want username/nickname/avatar 映射", d)
	}
	if len(d.Roles) != 2 || d.Roles[0] != "admin" {
		t.Errorf("roles = %v, want [admin ops]", d.Roles)
	}
	if d.Desc == nil || d.HomePath == nil || d.Token == nil {
		t.Errorf("desc/homePath/token 必须存在（UserInfo 三必填），got %+v", d)
	}
}

func TestUserInfo_NoClaims_401(t *testing.T) {
	ctrl := NewController(&fakeAuth{}, &fakeAudit{}, &fakePerm{}, &fakeUser{})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/user/info", ctrl.UserInfo) // 无 claims 注入
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/user/info", nil))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401（claims 缺失防御）", w.Code)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./modules/system/controller/vben/ -run 'TestLogout|TestCodes|TestUserInfo' -v`
Expected: FAIL（`ctrl.Logout/Codes/UserInfo` 未定义）

- [ ] **Step 3: 实现三个 handler 并补 Register**

`controller.go` 修改：

1. import 增加 `"context"`、`"net/http"`、`"strconv"`、`"alexGo-cloud/pkg/auth"`（`strings` 已有）。
2. `Register` 扩为：

```go
func (ctrl *Controller) Register(r *gin.RouterGroup) {
	r.POST("/auth/login", ctrl.Login)
	r.POST("/auth/logout", ctrl.Logout)
	r.GET("/auth/codes", ctrl.Codes)
	r.GET("/user/info", ctrl.UserInfo)
	// /menu/all 由 Task 3 补齐
}
```

3. 追加：

```go
// claimsOf 取中间件注入的 claims（生产路径由 auth 中间件保证；handler 内再判一次作纵深防御）。
func claimsOf(c *gin.Context) (*auth.Claims, bool) {
	v, exists := c.Get("claims")
	if !exists {
		return nil, false
	}
	cl, ok := v.(*auth.Claims)
	return cl, ok && cl != nil
}

// Logout POST /api/auth/logout —— 恒 200：无 token/无效 token 一律 no-op，
// vben 退出是前端状态清理，不能因后端失败把用户卡在登出流程里。
func (ctrl *Controller) Logout(c *gin.Context) {
	if raw := c.GetHeader("Authorization"); raw != "" {
		tok := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "Bearer "))
		if tok != "" {
			_ = ctrl.authSvc.Logout(c.Request.Context(), tok)
		}
	}
	ok(c, gin.H{})
}

// Codes GET /api/auth/codes —— 返回原始 perm 字符串数组（v-access 逐字匹配，含 `*` 形态）。
func (ctrl *Controller) Codes(c *gin.Context) {
	cl, found := claimsOf(c)
	if !found {
		fail(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	codes, err := ctrl.permSvc.UserPermCodes(c.Request.Context(), cl.UserID)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if codes == nil {
		codes = []string{}
	}
	ok(c, codes)
}

// UserInfo GET /api/user/info —— 对齐 vben BasicUserInfo + UserInfo：
// userId 必须字符串；desc/homePath/token 三个必填字段恒在。
func (ctrl *Controller) UserInfo(c *gin.Context) {
	cl, found := claimsOf(c)
	if !found {
		fail(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	realName := cl.Username
	avatar := ""
	if u, err := ctrl.userSvc.GetUserByID(c.Request.Context(), cl.UserID); err == nil && u != nil {
		if u.Nickname != "" {
			realName = u.Nickname
		}
		avatar = u.Avatar
	}
	roles := []string{}
	if rs, err := ctrl.permSvc.UserRoles(c.Request.Context(), cl.UserID); err == nil {
		for _, r := range rs {
			if r != nil && r.Code != "" {
				roles = append(roles, r.Code)
			}
		}
	}
	ok(c, gin.H{
		"userId":   strconv.FormatUint(cl.UserID, 10),
		"username": cl.Username,
		"realName": realName,
		"avatar":   avatar,
		"roles":    roles,
		"desc":     "",
		"homePath": "",
		"token":    strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")),
	})
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./modules/system/controller/vben/ -v`
Expected: PASS（Task 1 + Task 2 全部用例）

- [ ] **Step 5: 全量回归**

Run: `make test`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add modules/system/controller/vben/
git commit -m "feat(system): vben shell logout/codes/user-info endpoints"
```

---

### Task 3: MenuAll + module.go 装配 + 全路径挂载验证

**Files:**
- Modify: `modules/system/controller/vben/controller.go`（MenuAll + blankLayout；Register 最后一条）
- Modify: `modules/system/module.go`（import、fx.Provide、struct 字段、构造参数、RegisterRoutes）
- Test: `modules/system/controller/vben/controller_test.go`

**Interfaces:**
- Consumes: `service.PermissionService.UserRoutes(ctx, userID) ([]*service.VbenRoute, error)`；`service.VbenRoute{Path, Name, Component string, Meta VbenRouteMeta, Children []*VbenRoute}`（json：`path/name/component/meta/children`）；`service.VbenRouteMeta{Title string json:"title", Icon string json:"icon,omitempty", OrderNo int json:"orderNo,omitempty", Permissions []string json:"permissions,omitempty"}`（`order` 字段在 Task 6 加）。
- Produces: `GET /api/menu/all`；`module.go` 中 `m.vbenCtrl.Register(r)` 生产挂载（与测试同一条代码路径）；FX 装配可过 `go test ./alexgo-server/cmd`。

- [ ] **Step 1: 写 MenuAll 失败测试**

`controller_test.go` 追加：

```go
func TestMenuAll_LAYOUTBlankedAndTreeKept(t *testing.T) {
	ctrl := NewController(&fakeAuth{}, &fakeAudit{}, &fakePerm{
		routes: []*service.VbenRoute{
			{
				Path: "/system", Name: "system", Component: "LAYOUT",
				Meta: service.VbenRouteMeta{Title: "系统管理", OrderNo: 10},
				Children: []*service.VbenRoute{
					{
						Path: "/system/users", Name: "system_users",
						Component: "views/system/SystemUsersPage",
						Meta:      service.VbenRouteMeta{Title: "用户管理", OrderNo: 1},
					},
				},
			},
		},
	}, &fakeUser{})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/menu/all", func(c *gin.Context) {
		c.Set("claims", &auth.Claims{UserID: 9})
		ctrl.MenuAll(c)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/menu/all", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, "LAYOUT") {
		t.Errorf("body 含 LAYOUT（vben convertRoutes 会报 component invalid）: %s", body)
	}
	if !strings.Contains(body, `"component":""`) {
		t.Errorf("目录 component 必须置空: %s", body)
	}
	if !strings.Contains(body, `"component":"views/system/SystemUsersPage"`) {
		t.Errorf("叶子 component 必须原样保留: %s", body)
	}
	if !strings.Contains(body, `"path":"/system/users"`) || !strings.Contains(body, `"title":"用户管理"`) {
		t.Errorf("树结构/meta 丢失: %s", body)
	}
}

func TestMenuAll_NilRoutes_EmptyArray(t *testing.T) {
	ctrl := NewController(&fakeAuth{}, &fakeAudit{}, &fakePerm{routes: nil}, &fakeUser{})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/menu/all", func(c *gin.Context) {
		c.Set("claims", &auth.Claims{UserID: 9})
		ctrl.MenuAll(c)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/menu/all", nil))
	if !strings.Contains(w.Body.String(), `"data":[]`) {
		t.Errorf("body = %s, want data:[]（nil 转空数组，vben fetchMenuListAsync 期望数组）", w.Body.String())
	}
}

// Register 必须挂齐 5 条路由：任一缺失在 vben 运行期表现为 404 → 登录流程卡死。
func TestRegister_MountsAllFiveRoutes(t *testing.T) {
	r := newTestRouter(&fakeAuth{}, &fakeAudit{}, &fakePerm{}, &fakeUser{})
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/api/auth/login"},
		{http.MethodPost, "/api/auth/logout"},
		{http.MethodGet, "/api/auth/codes"},
		{http.MethodGet, "/api/user/info"},
		{http.MethodGet, "/api/menu/all"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader("{}"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code == http.StatusNotFound {
			t.Errorf("%s %s 未挂载（gin 404）", tc.method, tc.path)
		}
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./modules/system/controller/vben/ -run 'TestMenuAll|TestRegister' -v`
Expected: FAIL（`ctrl.MenuAll` 未定义；Register 缺路由导致 404 断言失败）

- [ ] **Step 3: 实现 MenuAll + blankLayout，补全 Register**

`controller.go`：Register 最后加 `r.GET("/menu/all", ctrl.MenuAll)`；追加：

```go
// MenuAll GET /api/menu/all —— vben backend 模式的完整路由树。
// 目录占位 LAYOUT 在此置空：vben convertRoutes 对空 component 直接跳过，
// generateAccessible 随后 delete 有 children 顶层路由的 component；
// 若输出 "LAYOUT" 会走 pageMap 查找失败分支（console.error + not-found 组件）。
func (ctrl *Controller) MenuAll(c *gin.Context) {
	cl, found := claimsOf(c)
	if !found {
		fail(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	routes, err := ctrl.permSvc.UserRoutes(c.Request.Context(), cl.UserID)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if routes == nil {
		routes = []*service.VbenRoute{}
	}
	blankLayout(routes)
	ok(c, routes)
}

// blankLayout 递归把 component=="LAYOUT" 置空（UserRoutes 每次请求新建树，原地改安全）。
func blankLayout(routes []*service.VbenRoute) {
	for _, r := range routes {
		if r == nil {
			continue
		}
		if r.Component == "LAYOUT" {
			r.Component = ""
		}
		blankLayout(r.Children)
	}
}
```

- [ ] **Step 4: 跑 vben 包测试确认通过**

Run: `go test ./modules/system/controller/vben/ -v`
Expected: PASS（全部用例）

- [ ] **Step 5: 装配 module.go（生产挂载）**

`modules/system/module.go` 四处修改：

1. import 块加：`"alexGo-cloud/modules/system/controller/vben"`
2. `fx.Provide(...)` 列表里 `admin.NewAuthController,` 之后加一行：`vben.NewController,`
3. `systemModule` struct 加字段 `vbenCtrl *vben.Controller`；`NewSystemModule` 参数列表末尾加 `vbenCtrl *vben.Controller,`（第 10 参），函数体 `return &systemModule{...}` 加 `vbenCtrl: vbenCtrl,`
4. `RegisterRoutes` 末尾（`appGroup.POST("/auth/logout", ...)` 之后）加：

```go
	// vben-admin 自读端点（/api/auth/login|logout|codes、/api/user/info、/api/menu/all）：
	// 挂在 /api 根组（r 已带 /api 前缀），与 admin/app 组无路径冲突。
	m.vbenCtrl.Register(r)
```

- [ ] **Step 6: FX 装配干跑**

Run: `go test ./alexgo-server/cmd -v`
Expected: PASS（正/反向依赖图干跑；`vben.NewController` 四参均已有 Provide——`service.NewAuthService`、`service.NewAuditService`、`service.NewPermissionService`、经 `service.NewService` 聚合的 UserService）

若报缺 Provide：对照报错的接口名，在 `module.go` 的 `fx.Provide` 列表补齐对应 `service.New*`（这些构造器已存在，只可能是漏挂不是缺实现）。

- [ ] **Step 7: 全量回归 + Commit**

Run: `make test`
Expected: PASS

```bash
git add modules/system/controller/vben/ modules/system/module.go
git commit -m "feat(system): vben shell menu-all endpoint and fx wiring"
```

---

### Task 4: 中间件三档——vben 自读端点放行（跳过 data_scope 与 Casbin）

**Files:**
- Modify: `pkg/middleware/auth.go`（两处锚点 + 新 helper）
- Test: `pkg/middleware/auth_test.go`

**Interfaces:**
- Consumes: 现有 `NewAuthMiddleware` 流程（公开门 → parseClaims → claims 注入 → 租户覆盖 → member 403 → ScopeLoader → Enforcer）；测试 helpers `fakeValidator`、`claimsFor`、`testCfg()`、`newEnforcer(t)`、`do(r,method,path,token)`、`fakeScope`。
- Produces: `func isVbenSelfRead(path string) bool`（精确匹配 3 路径，供两处锚点共用）；`/api/auth/codes`、`/api/user/info`、`/api/menu/all` 三端点的新鉴权语义（见 Global Constraints 三档定义）。

- [ ] **Step 1: 写三档语义的失败测试**

在 `auth_test.go` 末尾追加（import 无需新增——`token`、`auth`、`tenant`、`http`、`httptest`、`gin` 均已在列）：

```go
// ---- Task: vben 自读端点三档（/api/auth/codes、/api/user/info、/api/menu/all） ----

// newSelfReadRouter：装 3 条自读端点 + 判别路由 /api/auth/other + admin 对照路由。
func newSelfReadRouter(cfg *config.Config, enf *casbin.Enforcer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.NewAuthMiddleware(middleware.AuthDeps{
		Cfg: cfg, Enforcer: enf, Validator: fakeValidator{claims: claimsFor(cfg)},
	}))
	echo := func(c *gin.Context) {
		a, _ := c.Get("claims")
		cl, _ := a.(*auth.Claims)
		name := ""
		if cl != nil {
			name = cl.Username
		}
		c.JSON(http.StatusOK, gin.H{"username": name})
	}
	r.GET("/api/auth/codes", echo)
	r.GET("/api/user/info", echo)
	r.GET("/api/menu/all", echo)
	r.GET("/api/auth/other", echo) // 不在精确清单 → 公开档判别
	r.GET("/api/admin/system/users", echo)
	r.POST("/api/auth/login", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	return r
}

// 自读无 token → 401（token→claims 是必经档，不能退化成公开）。
func TestVbenSelfRead_NoToken_401(t *testing.T) {
	r := newSelfReadRouter(testCfg(), nil)
	for _, p := range []string{"/api/auth/codes", "/api/user/info", "/api/menu/all"} {
		if w := do(r, "GET", p, ""); w.Code != http.StatusUnauthorized {
			t.Errorf("GET %s 无 token status = %d, want 401; body=%s", p, w.Code, w.Body.String())
		}
	}
}

// 自读合法 token → 200 且 claims 已注入 handler。
func TestVbenSelfRead_ValidToken_200InjectsClaims(t *testing.T) {
	cfg := testCfg()
	tok, err := auth.GenerateToken(9, "alice", 1, cfg)
	if err != nil {
		t.Fatal(err)
	}
	r := newSelfReadRouter(cfg, nil)
	w := do(r, "GET", "/api/auth/codes", "Bearer "+tok)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if w.Body.String() != `{"username":"alice"}` {
		t.Errorf("body = %s, want claims injected", w.Body.String())
	}
}

// 空策略 enforcer + 合法 token 打自读 → 200：判别 Casbin 被跳过
//（若走到 Enforce，空策略必 403）。
func TestVbenSelfRead_SkipsCasbin(t *testing.T) {
	cfg := testCfg()
	tok, err := auth.GenerateToken(9, "alice", 1, cfg)
	if err != nil {
		t.Fatal(err)
	}
	r := newSelfReadRouter(cfg, newEnforcer(t)) // 零策略
	w := do(r, "GET", "/api/user/info", "Bearer "+tok)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200（空策略下仍放行 → 跳过 Casbin）; body=%s", w.Code, w.Body.String())
	}
}

// 自读 + ScopeLoader → loader 零调用（跳过 data_scope；跳过前不得 Load）。
func TestVbenSelfRead_SkipsScopeLoader(t *testing.T) {
	cfg := testCfg()
	cfg.Auth.Mode = "token"
	loader := &fakeScope{ds: tenant.DataScope{Mode: 3, UserID: 9, DeptID: 77}}
	v := fakeValidator{claims: &token.Claims{
		UserID: 9, Username: "alice", UserType: token.UserTypeAdmin, TenantID: 1, DeptID: 77,
	}}
	r := gin.New()
	r.Use(middleware.NewAuthMiddleware(middleware.AuthDeps{Cfg: cfg, Validator: v, ScopeLoader: loader}))
	r.GET("/api/auth/codes", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	w := do(r, "GET", "/api/auth/codes", "Bearer t")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if loader.calls != 0 {
		t.Errorf("loader called %d times on self-read, want 0（先跳过 scope 再放行）", loader.calls)
	}
}

// member token（token 模式 ut=2）打自读 → 403：member 门槛对自读同样生效。
func TestVbenSelfRead_Member_403(t *testing.T) {
	cfg := testCfg()
	cfg.Auth.Mode = "token"
	v := fakeValidator{claims: &token.Claims{
		UserID: 5, Username: "bob", UserType: token.UserTypeMember, TenantID: 1,
	}}
	r := gin.New()
	r.Use(middleware.NewAuthMiddleware(middleware.AuthDeps{Cfg: cfg, Validator: v}))
	r.GET("/api/menu/all", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	w := do(r, "GET", "/api/menu/all", "Bearer t")
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403（member 门槛先于自读放行）; body=%s", w.Code, w.Body.String())
	}
	if w.Body.String() != `{"error":"forbidden"}` {
		t.Errorf("body = %s, want {\"error\":\"forbidden\"}", w.Body.String())
	}
}

// 判别"精确匹配非前缀"：/api/auth/other 不在清单 → 公开档无 token 直接 200；
// /api/auth/login、/api/auth/logout 回归公开档。
func TestVbenSelfRead_ExactMatchNotPrefix(t *testing.T) {
	r := newSelfReadRouter(testCfg(), nil)
	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/auth/other"},
		{"POST", "/api/auth/login"},
		{"POST", "/api/auth/logout"},
	} {
		if w := do(r, tc.method, tc.path, ""); w.Code != http.StatusOK {
			t.Errorf("%s %s 无 token status = %d, want 200（公开档，未被前缀吞掉）; body=%s",
				tc.method, tc.path, w.Code, w.Body.String())
		}
	}
}

// admin 全门控回归：自读改动不得松动 /api/admin/**（空策略 + 合法 token → 403）。
func TestVbenSelfRead_AdminPathStillGated(t *testing.T) {
	cfg := testCfg()
	tok, err := auth.GenerateToken(9, "alice", 1, cfg)
	if err != nil {
		t.Fatal(err)
	}
	r := newSelfReadRouter(cfg, newEnforcer(t)) // 零策略
	if w := do(r, "GET", "/api/admin/system/users", "Bearer "+tok); w.Code != http.StatusForbidden {
		t.Errorf("admin 路径 status = %d, want 403（空策略照常 Enforce）; body=%s", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./pkg/middleware/ -run TestVbenSelfRead -v`
Expected: FAIL——`TestVbenSelfRead_NoToken_401`：自读路径现在落在公开档返回 200（要 401）；`ValidToken/SkipsCasbin` 恰好可能先"意外通过"（公开档也 200，但 body 无 claims → `username` 断言失败）；`ExactMatchNotPrefix` 中 `/api/auth/other` 现状本就 200（会先过，保留作回归）。

- [ ] **Step 3: 改 auth.go 锚点 1（入口档）**

`pkg/middleware/auth.go` —— 替换公开路径块（原 45-52 行，唯一出现处）：

```go
	// 公开路径：/api/admin/** 需登录态；vben 自读三端点（isVbenSelfRead）走
	// "token→claims→租户→member 门槛"后放行（跳过 data_scope 与 Casbin，见锚点 2）；
	// 其余（含 /api/app/** 全部、/api/auth/login|logout、health/metrics/pprof）一律放行。
	// I1 取舍：member 的 refresh/logout 是 possession-based——凭 body/头自行校验，
	// 放进本中间件只会在 access 过期时把刷新链路也 401 拦死，故不列入鉴权清单。
	if !strings.HasPrefix(path, "/api/admin/") && !isVbenSelfRead(path) {
		c.Next()
		return
	}
```

- [ ] **Step 4: 改 auth.go 锚点 2（自读短路）+ helper**

在 member 403 门槛块（`if claims.UserType != int(token.UserTypeAdmin) { ... }`）与 ScopeLoader 块（`// 数据范围注入（T10）...`）**之间**插入：

```go
		// vben 自读档（锚点 2）：已过 token→claims→租户覆盖→member 门槛，
		// 到此必为管理员自读请求——跳过 data_scope 注入与 Casbin 直接放行。
		// /api/admin/** 不满足本条件，继续走下方完整门控，现行为不变。
		if !strings.HasPrefix(path, "/api/admin/") {
			c.Next()
			return
		}
```

在 `parseClaims` 函数之后（文件末尾）追加：

```go
// isVbenSelfRead：vben 前端启动期自读端点的精确清单（非前缀匹配——
// /api/auth/other、/api/auth/login 等必须留在公开档）。
// 这三个端点带 token 即代表登录态，授权语义由 handler 内按 claims 自查，
// 不需要 data_scope（不查业务行）也不走 Casbin（无对应路由策略）。
func isVbenSelfRead(path string) bool {
	switch path {
	case "/api/auth/codes", "/api/user/info", "/api/menu/all":
		return true
	}
	return false
}
```

- [ ] **Step 5: 跑测试确认通过**

Run: `go test ./pkg/middleware/ -v`
Expected: PASS（新增 TestVbenSelfRead\* 七个用例 + 全部既有用例——既有公开档/门槛/Scope/Casbin 用例是回归网）

- [ ] **Step 6: 全量回归**

Run: `make test`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add pkg/middleware/auth.go pkg/middleware/auth_test.go
git commit -m "feat(middleware): vben self-read tier skips casbin and data scope"
```

---

### Task 5: order 路由授权 + modules.order 开关翻 true

**Files:**
- Modify: `modules/system/service/permission_routes.go:22-35`（permissionRoutes 表加一行）
- Modify: `alexgo-server/configs/config.yaml:46`
- Modify: `deployments/helm/alexgo-cloud/values.yaml:81`
- Test: `modules/system/service/policy_sync_test.go`（追加用例）

**Interfaces:**
- Consumes: `rebuildRolePolicies(ctx, e, role, menus)`（既有，内部 RemoveFilteredPolicy→按 `permPrefix` 查 `permissionRoutes`→AddPolicy）、`roleSub(tenantID, roleCode)`、`newMemEnforcer(t)` 既有测试 helper。
- Produces: `permissionRoutes["order:order"] = ["/api/admin/order/orders", "/api/admin/order/orders/*"]`——order 模块的两条路由（GET/POST 集合）从此可被角色策略命中；`modules.order=true` 使 `modules/order` 模块 RegisterRoutes 真正执行（门控在 `modules/order/module.go:47`）。

- [ ] **Step 1: 写 order 路由授权的失败测试**

`policy_sync_test.go` 末尾追加（import 若缺 `context` 补上；若既有内存 enforcer helper 名不是 `newMemEnforcer`，改用本文件现有构造 helper，断言不动）：

```go
// order 模块启用后：角色按 "order:order:*" 权限必须拿到 /api/admin/order/orders
//（集合 + item 两形态），且未授权的 system 路由仍拒绝——判别表项真加进去了、
// rebuild 不再对 order 按钮权限静默丢弃。
func TestRebuild_OrderRoutes(t *testing.T) {
	e := newMemEnforcer(t)
	role := &model.Role{ID: 1, Code: "admin", TenantID: 1}
	menus := []*model.Menu{{ID: 1, Permission: "order:order:*", Type: "menu"}}
	if err := rebuildRolePolicies(context.Background(), e, role, menus); err != nil {
		t.Fatalf("rebuild error = %v", err)
	}
	if _, err := e.AddRoleForUser("1:1:admin", "1:role:admin"); err != nil {
		t.Fatal(err)
	}
	for _, obj := range []string{"/api/admin/order/orders", "/api/admin/order/orders/123"} {
		if ok, _ := e.Enforce("1:1:admin", obj, "GET"); !ok {
			t.Errorf("order route %s must be allowed", obj)
		}
	}
	if ok, _ := e.Enforce("1:1:admin", "/api/admin/system/users", "GET"); ok {
		t.Error("unassigned route must be denied")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./modules/system/service/ -run TestRebuild_OrderRoutes -v`
Expected: FAIL——`permissionRoutes["order:order"]` 键不存在，rebuild 后无策略 → `order route ... must be allowed`

- [ ] **Step 3: permissionRoutes 加 order 行**

`permission_routes.go` 的 map（`"member:user"` 行之后）加：

```go
	"order:order":   {"/api/admin/order/orders", "/api/admin/order/orders/*"},
```

（gofmt 对齐既有列。`/*` 变体语义见 map 顶部注释：keyMatch2 覆盖 `:id` item 路径。）

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./modules/system/service/ -run 'TestRebuild' -v`
Expected: PASS（TestRebuild_OrderRoutes + 既有 TestRebuild\* 全过）

- [ ] **Step 5: 翻两处 modules.order 开关**

1. `alexgo-server/configs/config.yaml`（文件末段，44-46 行）：

```yaml
modules:
  system: true
  order: true
```

2. `deployments/helm/alexgo-cloud/values.yaml`（79-81 行）：

```yaml
  modules:
    system: true
    order: true
```

> `values-dev:20` / `values-prod:25` / k8s configmap:27 已是 true，**不要动**；只翻这两处 false。

Run: `grep -rn "order: false" alexgo-server/configs/ deployments/helm/alexgo-cloud/values.yaml`
Expected: 无输出（两处均已翻转）

- [ ] **Step 6: 全量回归**

Run: `make test`
Expected: PASS（`go test ./...` 零外部依赖，不起 DB、不读 config——yaml 翻转由 Step 5 的 grep 与 Task 26 端到端验收覆盖）

- [ ] **Step 7: Commit**

```bash
git add modules/system/service/permission_routes.go modules/system/service/policy_sync_test.go alexgo-server/configs/config.yaml deployments/helm/alexgo-cloud/values.yaml
git commit -m "feat(order): authorize order routes and enable order module"
```

---

### Task 6: `VbenRouteMeta.Order` 输出（vben `meta.order` 排序）

**Files:**
- Modify: `modules/system/service/rbac.go:504-509`（struct 加字段）
- Modify: `modules/system/service/rbac.go:356-360`（UserRoutes 填充）
- Test: `modules/system/service/rbac_meta_test.go`（新建）

**Interfaces:**
- Consumes: `model.Menu.Sort`（uint/int，既有）。
- Produces: `/api/menu/all` 响应的每个非按钮节点 `meta.order`（json `order,omitempty`）；`orderNo` 字段保留不动（既有消费方不断）。

- [ ] **Step 1: 写序列化失败测试**

创建 `modules/system/service/rbac_meta_test.go`：

```go
package service

import (
	"encoding/json"
	"strings"
	"testing"
)

// vben generate-menus 读 meta.order ?? 999 排序——必须输出 order，
// 且 Sort=0 时省略（omitempty）以免用 0 覆盖 vben 默认 999 把 0 号排到最后。
func TestVbenRouteMeta_OrderJSON(t *testing.T) {
	b, err := json.Marshal(VbenRoute{
		Path: "/system/users",
		Meta: VbenRouteMeta{Title: "用户管理", OrderNo: 5, Order: 10},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"order":10`) {
		t.Errorf("json = %s, want \"order\":10", s)
	}
	if !strings.Contains(s, `"orderNo":5`) {
		t.Errorf("json = %s, want orderNo 保留", s)
	}

	zero, err := json.Marshal(VbenRouteMeta{Title: "x"})
	if err != nil {
		t.Fatal(err)
	}
	// "order":（带冒号）不会误匹配 "orderNo":。
	if strings.Contains(string(zero), `"order":`) {
		t.Errorf("json = %s, want order omitted when 0", zero)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./modules/system/service/ -run TestVbenRouteMeta_OrderJSON -v`
Expected: 编译失败——`unknown field Order in type VbenRouteMeta`

- [ ] **Step 3: struct 加字段 + UserRoutes 填充**

1. `rbac.go:504-509` 的 struct 改为：

```go
type VbenRouteMeta struct {
	Title       string   `json:"title"`
	Icon        string   `json:"icon,omitempty"`
	OrderNo     int      `json:"orderNo,omitempty"`
	Order       int      `json:"order,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}
```

2. `rbac.go:356-360` 的 Meta 字面量改为：

```go
				Meta: VbenRouteMeta{
					Title:   m.Name,
					Icon:    m.Icon,
					OrderNo: m.Sort,
					Order:   m.Sort,
				},
```

> `routeComponent`（dir→`LAYOUT`）**不动**——`LAYOUT` 占位由壳层 `blankLayout`（Task 3）负责输出前置空，此层保持与既有 `GET /menus` 消费方一致。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./modules/system/service/ -run TestVbenRouteMeta -v`
Expected: PASS

- [ ] **Step 5: 全量回归**

Run: `make test`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add modules/system/service/rbac.go modules/system/service/rbac_meta_test.go
git commit -m "feat(system): emit meta.order for vben menu sorting"
```

**阶段 A 到此完成**：5 端点 + 中间件三档 + order 授权 + meta.order 全部落地，`make test` 与 `go test ./alexgo-server/cmd` 双绿。

---

# 阶段 B：菜单数据（迁移 + seed 门控）

> **执行顺序硬约束**：本阶段两个任务必须**先完成 Task 8 的代码**再对任何 fresh 库跑 `make migrate`。现有 dev 库（已有 `/system` 目录）两个任务间任意顺序都安全；但 fresh 库上若"迁移已插 5 行、旧 seed 门控未改"跑一次 migrate，11 个系统菜单将永久缺失。Task 7 的迁移验收只对现有 dev 库执行；fresh 库全链路在 Task 26 验收时做（届时两任务代码均在）。

### Task 7: 菜单回填迁移（5 行 + 11 处 icon UPDATE，幂等）

**Files:**
- Create: `modules/system/migrations/20261009000001_menus_vben_backfill.up.sql`
- Create: `modules/system/migrations/20261009000001_menus_vben_backfill.down.sql`

**Interfaces:**
- Consumes: 既有表结构（`menus`：`id,parent_id,type,name,path,component,icon,permission,sort,status,deleted,creator,updater,tenant_id,created_at,updated_at`，`created_at/updated_at` 为 `datetime NOT NULL` 无默认值 → raw INSERT 必须给 `NOW()`；`role_menus(tenant_id,role_id,menu_id)` 唯一键 `uk_role_menus_tenant_role_menu`；`roles` 有 `deleted`）；迁移机制（`pkg/migrate/runner.go`：golang-migrate、`schema_migrations_system`、只跑 `m.Up()`、`ErrNoChange` 容忍、日志 `migrate success`；`migration_source.go` 自动收集目录，加文件即可，module.go 不动）。
- Produces: 迁移版本 `20261009000001`；existing 库终态 19 行（14+5）；11 处 icon 换新（见映射表）；admin 角色对 5 新行的 `role_menus` 绑定（fresh 库此处 0 行，由 Task 8 seed 全量兜底）。

- [ ] **Step 1: 写 up SQL（全文）**

创建 `modules/system/migrations/20261009000001_menus_vben_backfill.up.sql`：

```sql
-- vben-admin 菜单回填：仪表盘/订单 2 目录 + 3 子页 + admin 角色绑定 + 11 处 icon 换新。
-- 幂等：全部 INSERT 走 NOT EXISTS，UPDATE 按旧 icon 精确匹配（重跑命中 0 行）。
-- 过滤必须带 type IN ('dir','menu')：member 按钮 icon='team' 与 roles 菜单 icon='team'
-- 同值，不带 type 会把按钮图标改掉；/system（dir）也在这 11 行内。

-- ① 根目录 2 行
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, 0, 'dir', '仪表盘', '/dashboard', '', 'lucide:layout-dashboard', '', 1, 1, 0, NOW(), NOW() FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND `path`='/dashboard' AND `type`='dir' AND `deleted`=0);

INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, 0, 'dir', '订单', '/order', '', 'lucide:shopping-cart', 'order', 2, 1, 0, NOW(), NOW() FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND `path`='/order' AND `type`='dir' AND `deleted`=0);

-- ② 子页 3 行（parent 经父目录 JOIN 定位；component 与页面文件路径严格一致）
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', '分析看板', '/dashboard/analytics', 'views/dashboard/analytics/index', 'lucide:area-chart', '', 1, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/dashboard' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND `path`='/dashboard/analytics' AND `deleted`=0);

INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', '工作台', '/dashboard/workspace', 'views/dashboard/workspace/index', 'carbon:workspace', '', 2, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/dashboard' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND `path`='/dashboard/workspace' AND `deleted`=0);

INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', '订单列表', '/order/orders', 'views/order/OrderOrdersPage', 'lucide:list', 'order:order:*', 1, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/order' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND `path`='/order/orders' AND `deleted`=0);

-- ③ admin 角色绑定 5 新行（JOIN roles 而非标量子查询：fresh 库尚无 admin 角色时产出 0 行，
--    由 seed 尾部 SetRoleMenus 全量兜底；已有库立即绑定 → Casbin 重建可见 order 权限）
INSERT INTO `role_menus` (`role_id`,`menu_id`,`tenant_id`)
SELECT r.`id`, m.`id`, 0
FROM `menus` m
JOIN `roles` r ON r.`tenant_id`=0 AND r.`code`='admin' AND r.`deleted`=0
WHERE m.`tenant_id`=0 AND m.`deleted`=0
  AND m.`path` IN ('/dashboard','/dashboard/analytics','/dashboard/workspace','/order','/order/orders')
  AND NOT EXISTS (SELECT 1 FROM `role_menus` rm WHERE rm.`tenant_id`=0 AND rm.`role_id`=r.`id` AND rm.`menu_id`=m.`id`);

-- ④ 11 处 icon UPDATE（旧 ant 图标 → iconify；type 过滤见文件头注释）
UPDATE `menus` SET `icon`='lucide:settings'    WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='settings';
UPDATE `menus` SET `icon`='lucide:user'        WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='user';
UPDATE `menus` SET `icon`='lucide:users'       WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='team';
UPDATE `menus` SET `icon`='lucide:list-tree'   WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='menu';
UPDATE `menus` SET `icon`='lucide:building-2'  WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='apartment';
UPDATE `menus` SET `icon`='lucide:id-card'     WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='idcard';
UPDATE `menus` SET `icon`='lucide:book-open'   WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='book';
UPDATE `menus` SET `icon`='lucide:settings-2'  WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='setting';
UPDATE `menus` SET `icon`='lucide:bell'        WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='bell';
UPDATE `menus` SET `icon`='lucide:history'     WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='history';
UPDATE `menus` SET `icon`='lucide:file-text'   WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='profile';
```

> 11 条 UPDATE 的旧值（`settings/user/team/menu/apartment/idcard/book/setting/bell/history/profile`）逐一取自 `seed.go` 现值（line 104-264），与既有 dev 库一致。

- [ ] **Step 2: 写 down SQL（全文）**

创建 `modules/system/migrations/20261009000001_menus_vben_backfill.down.sql`：

```sql
-- 逆序还原。注意：fresh 库上 seed（新代码）用新 icon 建的 11 行也会被 ③ 还原成旧 ant icon——
-- 属接受语义（down 本就是回到迁移前代码形态；再跑 up 会重新 UPDATE 回新值）。

-- ① 先解绑 5 行的 role_menus（经 path 定位菜单）
DELETE `rm` FROM `role_menus` `rm`
JOIN `menus` `m` ON `m`.`id` = `rm`.`menu_id`
WHERE `rm`.`tenant_id`=0 AND `m`.`tenant_id`=0
  AND `m`.`path` IN ('/dashboard','/dashboard/analytics','/dashboard/workspace','/order','/order/orders');

-- ② 删 5 行菜单
DELETE FROM `menus`
WHERE `tenant_id`=0
  AND `path` IN ('/dashboard','/dashboard/analytics','/dashboard/workspace','/order','/order/orders')
  AND `type` IN ('dir','menu');

-- ③ 11 处 icon 还原（新 → 旧，过滤条件与 up 对称）
UPDATE `menus` SET `icon`='settings'   WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='lucide:settings';
UPDATE `menus` SET `icon`='user'       WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='lucide:user';
UPDATE `menus` SET `icon`='team'       WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='lucide:users';
UPDATE `menus` SET `icon`='menu'       WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='lucide:list-tree';
UPDATE `menus` SET `icon`='apartment'  WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='lucide:building-2';
UPDATE `menus` SET `icon`='idcard'     WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='lucide:id-card';
UPDATE `menus` SET `icon`='book'       WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='lucide:book-open';
UPDATE `menus` SET `icon`='setting'    WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='lucide:settings-2';
UPDATE `menus` SET `icon`='bell'       WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='lucide:bell';
UPDATE `menus` SET `icon`='history'    WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='lucide:history';
UPDATE `menus` SET `icon`='profile'    WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='lucide:file-text';
```

- [ ] **Step 3: 第一遍迁移（现有 dev 库，后台执行——`make migrate` 完成后进程会挂起属预期）**

```bash
make migrate > /tmp/migrate-run1.log 2>&1 &
for i in $(seq 1 60); do
  grep -q "seed default admin done" /tmp/migrate-run1.log 2>/dev/null && break
  sleep 2
done
grep -E "migrate success|seed default admin done" /tmp/migrate-run1.log
pkill -f -- "--migrate-only" || true
```

Expected: 日志同时出现 `migrate success`（module=system）与 `seed default admin done`；无 ERROR/panic。
`pkill` 后用 `pgrep -f -- "--migrate-only"` 确认无残留（go run 父子进程都会匹配到）。

> 只允许对 config.yaml 指向的现有 dev 库（81.71.152.135:3399，已有 14 行）执行——**fresh 库禁跑**（见阶段 B 开头硬约束）。

- [ ] **Step 4: 第二遍迁移（幂等验证）**

```bash
make migrate > /tmp/migrate-run2.log 2>&1 &
for i in $(seq 1 60); do
  grep -q "seed default admin done" /tmp/migrate-run2.log 2>/dev/null && break
  sleep 2
done
grep -E "migrate success|seed default admin done" /tmp/migrate-run2.log
grep -iE "error|dirty" /tmp/migrate-run2.log || echo "no errors"
pkill -f -- "--migrate-only" || true
```

Expected: 仍出现 `migrate success`（版本已应用 → `ErrNoChange` 容忍路径，无 SQL 重放）+ `seed default admin done`（seed 每次启动都跑，幂等）；**无** `error`/`dirty`。

- [ ] **Step 5: 数据核对（有 mysql 客户端则做，没有则推迟到 Task 26 的 API 断言）**

```bash
command -v mysql >/dev/null && mysql -h81.71.152.135 -P3399 -ualexgo-cloud -p'caMYe2mB3jpEerbY' alexgo-cloud -e "
SELECT COUNT(*) AS total FROM menus WHERE tenant_id=0 AND deleted=0;
SELECT path, icon FROM menus WHERE tenant_id=0 AND type='dir' AND path IN ('/system','/dashboard','/order') ORDER BY sort;
SELECT COUNT(*) AS buttons FROM menus WHERE tenant_id=0 AND type='button';
" || echo "no mysql client — 断言推迟到 Task 26（menu/all=16 节点 + codes 含按钮 perm）"
```

Expected: `total=19`；三目录 icon 分别为 `lucide:settings`/`lucide:layout-dashboard`/`lucide:shopping-cart`；`buttons=3`（member 按钮 `icon='team'` 未被 ④ 误伤的直接证据）。

- [ ] **Step 6: 回归 + Commit**

Run: `make test`
Expected: PASS（迁移不在单测路径——单测零 DB）

```bash
git add modules/system/migrations/20261009000001_menus_vben_backfill.up.sql modules/system/migrations/20261009000001_menus_vben_backfill.down.sql
git commit -m "feat(system): migrate vben menu backfill with icon upgrades"
```

---

### Task 8: seed 门控 Option A（`hasSystemDir`）+ 新 icon + rootID 锚定

**Files:**
- Modify: `modules/system/service/seed.go`（门控 line 96-97、rootID line 283-288、11 处 Icon、新增 helper）
- Test: `modules/system/service/seed_test.go`（追加）

**Interfaces:**
- Consumes: `model.Menu{Path, Type string...}`；`strings.EqualFold`（文件已 import）；`p.Menus.List(ctx, tid)`。
- Produces: `func hasSystemDir(menus []*model.Menu) bool`（文件级 helper，与既有 `hasPermPrefix` 同风格）；seed 创建门控语义 = "/system 目录存在即已播种"；rootID 恒为 /system 目录 ID；11 行新 icon（与 Task 7 UPDATE 目标值一致）。

- [ ] **Step 1: 写门控失败测试**

`seed_test.go` 末尾追加（`strings` import 若缺则补）：

```go
// 迁移（fx.Invoke）先于 seed（OnStart）执行：fresh 库在 seed 观察时已非空——
// 旧门控 len==0 会误判"已播种"，11 个系统菜单永久缺失。以 /system 目录存在为准。
func TestHasSystemDir(t *testing.T) {
	if hasSystemDir(nil) {
		t.Error("empty must be false")
	}
	if hasSystemDir([]*model.Menu{{Path: "/dashboard", Type: "dir"}}) {
		t.Error("dashboard dir must not count (migration inserts it first)")
	}
	if !hasSystemDir([]*model.Menu{{Path: "/system", Type: "dir"}}) {
		t.Error("/system dir must count")
	}
	if hasSystemDir([]*model.Menu{{Path: "/system", Type: "menu"}}) {
		t.Error("type must be dir")
	}
	if !hasSystemDir([]*model.Menu{{Path: "/system", Type: "DIR"}}) {
		t.Error("type compare must be case-insensitive")
	}
	if hasSystemDir([]*model.Menu{nil, {Path: "/system/users", Type: "menu"}}) {
		t.Error("nil element must be skipped, not panic")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./modules/system/service/ -run TestHasSystemDir -v`
Expected: 编译失败——`undefined: hasSystemDir`

- [ ] **Step 3: 实现 hasSystemDir**

`seed.go` 文件末尾（`hasPermPrefix` 附近）追加：

```go
// hasSystemDir 判断 /system 目录是否已存在——seed 菜单创建门控（Option A）。
// 迁移可在 seed 之前向 menus 插行（仪表盘/订单），"表非空"不再等于"已播种"。
func hasSystemDir(menus []*model.Menu) bool {
	for _, m := range menus {
		if m != nil && m.Path == "/system" && strings.EqualFold(m.Type, "dir") {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: 换门控与 rootID**

1. 门控（原 96-97 行）：

```go
	menus, _ := p.Menus.List(ctx, tid)
	// 迁移 20261009000001 先于本函数插入仪表盘/订单 5 行，fresh 库 len(menus)!=0
	// 会被误判"已播种"→ 11 个系统菜单永远缺失（Option A，spec errata 4）。
	if !hasSystemDir(menus) {
```

2. rootID（原 283-288 行，按钮挂载锚点）：

```go
	var rootID uint64
	for _, m := range allMenus {
		// 迁移先插了 /dashboard、/order 两个 ParentID==0 目录——按"第一个 dir"取
		// 会把 3 个按钮挂到仪表盘下；锚定 /system 目录本身。
		if m.Path == "/system" && strings.EqualFold(m.Type, "dir") {
			rootID = m.ID
			break
		}
	}
```

- [ ] **Step 5: 11 处 Icon 换新（与 Task 7 UPDATE 目标一致）**

`seed.go` 中按 path 逐一替换（每个旧值字面量在文件内唯一，Edit 不会误中）：

| Path（line） | 旧 Icon | 新 Icon |
| --- | --- | --- |
| `/system` (104) | `settings` | `lucide:settings` |
| `/system/users` (120) | `user` | `lucide:user` |
| `/system/roles` (136) | `team` | `lucide:users` |
| `/system/menus` (152) | `menu` | `lucide:list-tree` |
| `/system/depts` (168) | `apartment` | `lucide:building-2` |
| `/system/posts` (184) | `idcard` | `lucide:id-card` |
| `/system/dict` (200) | `book` | `lucide:book-open` |
| `/system/configs` (216) | `setting` | `lucide:settings-2` |
| `/system/notices` (232) | `bell` | `lucide:bell` |
| `/system/logins` (248) | `history` | `lucide:history` |
| `/system/operates` (264) | `profile` | `lucide:file-text` |

按钮块的 `icon: "key"/"cluster"/"team"`（line 295-297）**不动**。

Run: `grep -n 'Icon: "' modules/system/service/seed.go`
Expected: 11 行均为 `lucide:`/`carbon:` 前缀，按钮 3 行仍是 `key`/`cluster`/`team`。

- [ ] **Step 6: 跑测试确认通过**

Run: `go test ./modules/system/service/ -run TestHasSystemDir -v`
Expected: PASS

- [ ] **Step 7: 幂等回归（现有 dev 库再跑一遍 migrate——门控/rootID 改动的真实执行路径）**

```bash
make migrate > /tmp/migrate-run3.log 2>&1 &
for i in $(seq 1 60); do
  grep -q "seed default admin done" /tmp/migrate-run3.log 2>/dev/null && break
  sleep 2
done
grep -E "migrate success|seed default admin done" /tmp/migrate-run3.log
grep -iE "error|dirty" /tmp/migrate-run3.log || echo "no errors"
pkill -f -- "--migrate-only" || true
```

Expected: 门控 `hasSystemDir`=true → 跳过创建（不重复插 11 行）；rootID 锚到 /system → `hasPermPrefix` 全命中 → 不重复插按钮；日志无 error。

- [ ] **Step 8: 全量回归 + Commit**

Run: `make test`
Expected: PASS

```bash
git add modules/system/service/seed.go modules/system/service/seed_test.go
git commit -m "fix(seed): hasSystemDir gate and /system root anchor for vben migration"
```

**阶段 B 到此完成**：迁移两遍幂等、seed 门控免疫迁移毒化、icon 全量换新。

---

# 阶段 C：前端 vendoring 与对接通（T9-T12）

### Task 9: vendoring 上游 v5.7.0 基线落地

**Files:**
- Create: 仓库根新增 `apps/`（仅 `web-antd` 先保留全量后剪）、`packages/`、`internal/`、`playground/`、`.changeset/`、根 `package.json`、`pnpm-workspace.yaml`、`pnpm-lock.yaml`、`turbo.json`、`.npmrc`、`.nvmrc` 等（tar 全量拷入，排除项见 Step 4）
- Modify: `.gitignore`（合并上游内容，**不得覆盖**本仓库 Go 忽略规则）
- Merge: `scripts/`（上游脚本并入，**不得删除**既有 `crud_generator.go` 等）
- Modify: `pnpm-workspace.yaml`（删 `- docs` 行——docs 目录不拷）
- Create: `apps/web-antd/VENDOR.md`（溯源：仓库 URL + tag + SHA + 日期；随 vendored 代码走，admin-web 即将删除故不放那边）
- Create（条件）: `LICENSE` 或 `LICENSE.vben`（上游 MIT 许可证必须随源码保留）

**Interfaces:**
- Consumes: 上游 `https://github.com/vbenjs/vue-vben-admin.git` tag `v5.7.0`（SHA `63a38dce49ba109f61607994e21ba921d8e970e9`）；本机 corepack/pnpm。
- Produces: 可 `pnpm install` + 可 build 的 vendored monorepo 基线；`apps/web-antd` 原样（定制从 T11 开始）；node_modules 就绪。

- [ ] **Step 1: 碰撞预检（有任何 EXISTS 输出就停下手工合并，不得盲拷）**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud
for f in package.json pnpm-workspace.yaml pnpm-lock.yaml turbo.json .npmrc .nvmrc LICENSE Makefile .gitignore; do
  test -e "$f" && echo "EXISTS: $f"
done
```

Expected: 只有 `Makefile`、`.gitignore`（本仓库必有）；`LICENSE` 有无均按 Step 4 处理。若出现 `package.json`/`pnpm-workspace.yaml` 等 → **停止**，先与现有内容 diff 合并再继续。

- [ ] **Step 2: 浅克隆并核对 SHA（SHA 不符立即中止）**

```bash
git clone --depth 1 --branch v5.7.0 https://github.com/vbenjs/vue-vben-admin.git /tmp/vue-vben-admin-570
cd /tmp/vue-vben-admin-570 && git rev-parse HEAD
```

Expected: `63a38dce49ba109f61607994e21ba921d8e970e9`（不匹配 = tag 被移动，中止并报告）。

- [ ] **Step 3: 上游根目录清单确认**

```bash
ls -A /tmp/vue-vben-admin-570
```

Expected: 看到 `apps packages internal docs playground scripts .changeset .github .gitignore package.json pnpm-workspace.yaml turbo.json ...`——与 Step 4 排除清单核对后继续。

- [ ] **Step 4: 拷入仓库（排除清单——每项都有理由）**

```bash
cd /tmp/vue-vben-admin-570
tar --exclude='.git' --exclude='.github' --exclude='README.md' --exclude='LICENSE' \
    --exclude='.gitignore' --exclude='scripts' --exclude='docs' --exclude='Makefile' \
    -cf - . | tar -xf - -C /Users/alex/Desktop/goWork/alexGo-cloud
```

排除理由：`.git`/`.github`/`README.md`/`Makefile`/`.gitignore` = 本仓库自有，覆盖即事故；`scripts` = 需合并不可覆盖；`docs` = 本仓库 `docs/` 存放 spec/architecture，**绝不允许上游 docs 混入或被后续删除**；`LICENSE` = 本仓库可能已有同名文件，Step 5 条件处理。

- [ ] **Step 5: 三件合并 + 溯源**

1. `.gitignore` 追加（追加而非覆盖，重复行无害）：

```bash
cat /tmp/vue-vben-admin-570/.gitignore >> /Users/alex/Desktop/goWork/alexGo-cloud/.gitignore
```

2. `scripts/` 合并（上游 node/sh 脚本进本仓库既有目录，不删 `.go` 文件）：

```bash
cp -R /tmp/vue-vben-admin-570/scripts/. /Users/alex/Desktop/goWork/alexGo-cloud/scripts/
```

3. LICENSE 条件处理：

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud
if test -e LICENSE; then
  cp /tmp/vue-vben-admin-570/LICENSE LICENSE.vben
else
  cp /tmp/vue-vben-admin-570/LICENSE LICENSE
fi
```

4. 删 pnpm-workspace 的 `- docs` 行（docs 未拷，行留着 pnpm install 可能报错；`playground` 行 T10 再删）：

```bash
sed -i '' '/^- docs$/d' pnpm-workspace.yaml
grep -n "docs\|playground" pnpm-workspace.yaml
```

Expected: grep 只剩 `- playground`。

5. 溯源文件 `apps/web-antd/VENDOR.md`：

```markdown
# Vendored from vue-vben-admin

- Repo: https://github.com/vbenjs/vue-vben-admin
- Tag: v5.7.0
- Commit: 63a38dce49ba109f61607994e21ba921d8e970e9
- Vendored on: 2026-10-09
- 本仓库只保留 apps/web-antd，其余应用/文档/playground 已剪除；packages 为运行所需，改动上游代码时请对照该 SHA。
```

- [ ] **Step 6: 工具链就绪 + 安装**

```bash
corepack enable
corepack prepare pnpm@10.33.4 --activate
pnpm --version        # → 10.33.4
pnpm install          # 数分钟；node v22.14 < engines ^22.18 仅告警（.npmrc 无 engine-strict）
```

Expected: `pnpm install` 成功（postinstall `pnpm -r run stub --if-present` 跑通）；无 ERR_PNPM。

- [ ] **Step 7: 基线 build（未做任何定制——必须绿，红了说明拷贝不完整）**

```bash
pnpm --filter @vben/web-antd build
```

Expected: build 成功。若报找不到 `@vben/vite-config` 等 workspace 产物：`pnpm turbo run build --filter=@vben/web-antd`（turbo 按 `^build` 先建依赖）后再重试本命令。

- [ ] **Step 8: 拷贝完整性自检 + Commit**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud
git status --porcelain | grep '^ M'     # 已跟踪文件的修改
git status --porcelain | grep '^??' | head -20
```

Expected ` M` 行只有：`.gitignore`（合并）、`scripts/` 若原本被跟踪（合并产生的修改）。若出现其他已跟踪文件被修改 → 逐个检查是否 tar 覆盖（不该发生，排除清单已挡）。

```bash
git add -A
git commit -m "chore(frontend): vendor vue-vben-admin v5.7.0 baseline"
```

> 提交体积大属预期；确认 `git status` 里没有 `node_modules`、`dist/`（.gitignore 合并生效的验证点——若出现则先补 .gitignore 再提交）。

---

### Task 10: monorepo 剪除 + 删 admin-web

**Files:**
- Delete: `apps/backend-mock`、`apps/web-ele`、`apps/web-naive`、`apps/web-tdesign`、`apps/web-antdv-next`、`playground`、`.changeset`、`admin-web`、`.commitlintrc.js`、`lefthook.yml`、`vitest.config.ts`、`README.*.md`（上游多语言 README；根 `README.md` 保留）、`CHANGELOG.md`（若上游带来）
- Modify: `pnpm-workspace.yaml`（删 `- playground` 行）
- Modify: `turbo.json`（删 `@vben/backend-mock#build` 块、`test:e2e` 块）
- Modify: 根 `package.json`（脚本与 devDependencies 修剪清单见 Step 4）
- Verify: `.gitignore` 覆盖 `dist/`、`.turbo/`

**Interfaces:**
- Consumes: Task 9 的 vendored 树。
- Produces: 只含 `apps/web-antd` + `packages` + `internal` + `scripts` 的精简 monorepo；`admin-web` 删除（旧前端完成历史使命）；lockfile 与修剪后依赖一致。

- [ ] **Step 1: 删除目录与散件**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud
rm -rf apps/backend-mock apps/web-ele apps/web-naive apps/web-tdesign apps/web-antdv-next \
       playground .changeset admin-web
rm -f .commitlintrc.js lefthook.yml vitest.config.ts README.*.md CHANGELOG.md
ls apps/
```

Expected: `apps/` 只剩 `web-antd`；根不再有 `admin-web/`、`playground/`。

> ⚠️ `docs/` 一个字都不能删——Task 9 已把上游 docs 排除在外，本仓库 `docs/`（spec/计划/architecture）是自有资产。

- [ ] **Step 2: pnpm-workspace 修剪**

```bash
sed -i '' '/^- playground$/d' pnpm-workspace.yaml
cat pnpm-workspace.yaml
```

Expected: packages 段只剩 `apps/*`、`packages/*`、`internal/*`（无 docs/playground 行）。

- [ ] **Step 3: turbo.json 修剪**

打开 `turbo.json`，删除两处：
1. pipeline 中的整个 `"/@vben/backend-mock#build"` ——键名形如 `"@vben/backend-mock#build": { ... }` 的键值对（含逗号按相邻项调整）；
2. 整个 `"test:e2e": {}` 键值对。

```bash
node -e "JSON.parse(require('fs').readFileSync('turbo.json','utf8')); console.log('turbo.json OK')"
grep -n "backend-mock\|test:e2e" turbo.json || echo "clean"
```

Expected: `turbo.json OK` + `clean`。

- [ ] **Step 4: 根 package.json 修剪（逐项核对）**

删 **scripts** 这 20 项：`build:docker`、`build:docs`、`build:ele`、`build:naive`、`build:tdesign`、`build:play`、`changeset`、`commit`、`dev:antdv-next`、`dev:docs`、`dev:ele`、`dev:naive`、`dev:tdesign`、`dev:play`、`version`、`test:e2e`、`update:deps`、`catalog`、`prepare`、`test:unit`。

改 **scripts.`check:cspell`**：从其 glob 数组里删 `".changeset/*.md"` 一项。

删 **devDependencies** 这 6 项：`@changesets/changelog-github`、`@changesets/cli`、`happy-dom`、`lefthook`、`playwright`、`vitest`。

**必须保留**（删了就坏）：`preinstall`（`npx only-allow pnpm`）、`postinstall`（`pnpm -r run stub --if-present`）、`devDependencies` 里的 `@vben/turbo-run` 与 `@vben/vsh`（均 `workspace:*`）、`engines`（`node ^22.18.0 || ^24.0.0`）、`packageManager`（`pnpm@10.33.4`）。

```bash
node -e "JSON.parse(require('fs').readFileSync('package.json','utf8')); console.log('package.json OK')"
grep -nE "changeset|lefthook|vitest|playwright|happy-dom|test:e2e|build:docs|dev:ele" package.json || echo "clean"
```

Expected: `package.json OK` + `clean`。

- [ ] **Step 5: .gitignore 覆盖检查**

```bash
grep -nE "^/?dist/?$|turbo" .gitignore | head; grep -n "\.turbo" .gitignore | head -3
```

Expected: 能命中 `dist` 与 `.turbo`（上游 .gitignore 已带；缺则在追加区补两行 `dist/`、`.turbo/`）。

- [ ] **Step 6: 重装 + 双验证**

```bash
pnpm install
pnpm --filter @vben/web-antd build
pnpm --filter @vben/web-antd typecheck
make test
```

Expected: 全绿。`pnpm install` 会因 devDeps 变化更新 `pnpm-lock.yaml`（一并提交）；`grep -rn "backend-mock\|web-ele\|antdv-next" package.json turbo.json pnpm-workspace.yaml` 无输出。

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "chore(frontend): trim vendored monorepo and retire admin-web"
```

---

### Task 11: 请求链路对接通——拦截器信封 + 代理 + logout 带 token + 访问偏好

**Files:**
- Modify: `apps/web-antd/src/api/request.ts`（defaultResponseInterceptor 参数）
- Modify: `apps/web-antd/src/api/core/auth.ts`（logoutApi 改 requestClient）
- Rewrite: `apps/web-antd/vite.config.ts`（mock 代理 → Go 后端）
- Modify: `apps/web-antd/src/preferences.ts`（accessMode/homePath/refresh 开关/copyright）

**Interfaces:**
- Consumes: 后端信封三种形态（Task 1-3）；旧 admin 响应两形态 `{"data":[...]}`/`{"status":"ok"}`；`requestClient` 自动带 `Authorization: Bearer`（既有请求拦截器）；`baseRequestClient`（裸 axios，不带 token——logout 现用它，服务端撤销拿不到 token）。
- Produces: `requestClient` 对三形态均正确解包；`POST /api/auth/logout` 带 Authorization；dev 代理把 `/api/**`、`/swagger` 转 `http://localhost:8080`（member 双前缀可覆盖）；偏好 = 后端菜单模式 + 首页 `/dashboard/analytics` + 关闭 refresh 链。

- [ ] **Step 1: request.ts 拦截器函数式改造**

`apps/web-antd/src/api/request.ts` 中创建 client 的调用（原参数 `codeField: 'code', dataField: 'data', successCode: 0`）替换为：

```ts
  createRequestClient(apiURL, {
    responseReturn: 'data',
    // alexGo：兼容三形态——
    //   ① 新信封 {code:0,data,...}      → successCode 0，解 data
    //   ② 旧业务 {data:[...]}           → 无 code → undefined 视为成功，解 data
    //   ③ 旧动作 {"status":"ok"}        → 无 code、无 data 键 → 返回原 body
    // 非 2xx 走 rejected 链（successCode 不参与）→ authenticate → errorMessage
    //（优先弹后端 error 文案）。
    interceptors: {
      response: [
        defaultResponseInterceptor({
          codeField: 'code',
          dataField: (response: any) =>
            response &&
            typeof response === 'object' &&
            'data' in response &&
            response.data !== undefined
              ? response.data
              : response,
          successCode: (code: any) => code === undefined || code === 0,
        }),
        // ……其后两个拦截器（authenticate、errorMessage）保持原样，链序不动
      ],
    },
  })
```

> 落笔方式：只替换 `defaultResponseInterceptor({...})` 这一个对象实参；外层 `createRequestClient` 的结构、后续拦截器、`formatToken`、`doReAuthenticate` 全部不动。以文件现值为准——若现值的组织形式与上述有出入，保持 `defaultResponseInterceptor` 三参数为函数式的这一处修改即可。

- [ ] **Step 2: logoutApi 换 requestClient**

`apps/web-antd/src/api/core/auth.ts` 替换 logout 函数：

```ts
/**
 * 退出登录
 */
export async function logoutApi() {
  // alexGo：走 requestClient（带 Authorization）——后端按 token 撤销；
  // 后端恒 200，撤销失败也不阻断前端登出流程。refreshTokenApi 保留原样（一期关闭 refresh 链）。
  return requestClient.post('/auth/logout');
}
```

（原实现 `baseRequestClient.post('/auth/logout', { withCredentials: true })` 删除；`baseRequestClient` 的 import 保留——refreshTokenApi 仍用。）

- [ ] **Step 3: vite 代理指向 Go 后端**

整文件重写 `apps/web-antd/vite.config.ts`：

```ts
import { defineConfig } from '@vben/vite-config';

export default defineConfig(async () => {
  return {
    application: {},
    vite: {
      server: {
        proxy: {
          // 会员端独立代理（MEMBER_PROXY 可覆盖）——长前缀必须排在 /api 之前
          '/api/app/member': {
            changeOrigin: true,
            target: process.env.MEMBER_PROXY || 'http://localhost:8080',
          },
          '/api/admin/member': {
            changeOrigin: true,
            target: process.env.MEMBER_PROXY || 'http://localhost:8080',
          },
          // 业务 API → Go 后端。不 rewrite：vben baseURL=/api，请求已是 /api/**，gin 挂的正是 /api/**
          '/api': {
            changeOrigin: true,
            target: 'http://localhost:8080',
          },
          '/swagger': {
            changeOrigin: true,
            target: 'http://localhost:8080',
          },
        },
      },
    },
  };
});
```

（上游 5320 mock 代理与 `rewrite` 整体消失——这是本文件唯一用途的改变。）

- [ ] **Step 4: preferences.ts 定制**

`apps/web-antd/src/preferences.ts` 的 `overridesPreferences` 替换为（文件其余部分 `preferencesExtension` 不动）：

```ts
export const overridesPreferences = defineOverridesPreferences({
  app: {
    accessMode: 'backend',
    defaultHomePath: '/dashboard/analytics',
    enableRefreshToken: false,
    name: import.meta.env.VITE_APP_TITLE,
  },
  copyright: {
    companyName: 'alexGo-cloud',
    companySiteLink: 'https://github.com/vatezj/alexgo-cloud',
    date: '2026',
  },
});
```

> `enableRefreshToken: false` 必须显式写：401 时走 `doReAuthenticate`（重登弹窗），不打不存在的 `/api/auth/refresh`。若 typecheck 报 `copyright` 键类型不匹配，对照 `packages/@core/preferences` 的类型定义把键放对层级（语义不变：页脚公司名/链接/年份）。

- [ ] **Step 5: 静态验证**

```bash
pnpm --filter @vben/web-antd typecheck
pnpm --filter @vben/web-antd build
```

Expected: 双绿（一期不建前端测试基建——spec 定稿；运行期证据在 Task 12 冒烟）。

- [ ] **Step 6: Commit**

```bash
git add apps/web-antd/src/api/request.ts apps/web-antd/src/api/core/auth.ts apps/web-antd/vite.config.ts apps/web-antd/src/preferences.ts
git commit -m "feat(web-antd): envelope interceptor, go proxy, backend access prefs"
```

---

### Task 12: env 定制 + 路由剪除 + 全链路登录冒烟

**Files:**
- Modify: `apps/web-antd/.env`
- Modify: `apps/web-antd/.env.development`
- Delete: `apps/web-antd/src/router/routes/modules/dashboard.ts`、`demos.ts`、`vben.ts`
- Test: 运行期冒烟（Playwright 浏览器，起双端）

**Interfaces:**
- Consumes: Task 11 的拦截器/代理/偏好；Task 1-8 的后端全链路（登录、三自读端点、迁移菜单数据）。
- Produces: 标题/命名空间/store 密钥为 alexGo 归属；mock 关闭；backend 模式下菜单来自 `/api/menu/all` 的端到端证据；`routes/modules` 清空（backend 模式不消费前端模块路由）。

- [ ] **Step 1: .env 三键改值**

`apps/web-antd/.env`（上游原值 → 新值）：

```env
# 应用标题
VITE_APP_TITLE=alexGo-cloud 管理后台

# 应用命名空间，用于缓存、store等功能的前缀，确保隔离
VITE_APP_NAMESPACE=alexgo-cloud-admin

# 对store进行加密的密钥，在将store持久化到localStorage时会使用该密钥进行加密
VITE_APP_STORE_SECURE_KEY=alexgo-cloud-store-sec-2026
```

> STORE_SECURE_KEY 换值会让已持久化的旧 store 解密失败 → 重新登录，属预期；此值从此固定不再改。

- [ ] **Step 2: .env.development 只改一个开关**

`apps/web-antd/.env.development`：`VITE_NITRO_MOCK=true` → `VITE_NITRO_MOCK=false`。
其余（`VITE_PORT=5666`、`VITE_BASE=/`、`VITE_GLOB_API_URL=/api`、`VITE_DEVTOOLS=false`、`VITE_INJECT_APP_LOADING=true`）保持上游原值。

```bash
grep -E "VITE_NITRO_MOCK|VITE_PORT|VITE_GLOB_API_URL" apps/web-antd/.env.development
```

Expected: `false` / `5666` / `/api`。

- [ ] **Step 3: 删除前端模块路由（3 个文件）**

```bash
rm -f apps/web-antd/src/router/routes/modules/dashboard.ts \
      apps/web-antd/src/router/routes/modules/demos.ts \
      apps/web-antd/src/router/routes/modules/vben.ts
ls apps/web-antd/src/router/routes/modules/
```

Expected: 目录空（或仅剩 `.`/`..`）。`routes/core.ts` 不在 modules 下，**保留**（登录页/403 等核心路由）；`import.meta.glob` 空结果安全。backend 模式菜单/路由全部来自后端，三文件的 demo 页面不再可达。

- [ ] **Step 4: 起双端（两个后台进程）**

```bash
# 后端（migrate+seed 会在启动日志里再跑一遍，幂等）
make run > /tmp/alexgo-dev-backend.log 2>&1 &
for i in $(seq 1 30); do curl -sf http://localhost:8080/health >/dev/null && break; sleep 1; done
curl -sf http://localhost:8080/health && echo " backend UP"

# 前端
pnpm --filter @vben/web-antd dev > /tmp/alexgo-dev-web.log 2>&1 &
for i in $(seq 1 30); do curl -sf http://localhost:5666/ >/dev/null && break; sleep 1; done
curl -sf http://localhost:5666/ >/dev/null && echo " frontend UP"
```

Expected: `backend UP` + `frontend UP`。

- [ ] **Step 5: 浏览器冒烟（Playwright）**

1. `browser_navigate` → `http://localhost:5666/` → 应重定向到登录页（URL 含 `/login`）。
2. 填 `admin` / `admin123` → 提交。
3. 断言：跳转到 `/dashboard/analytics`；左侧菜单出现 **仪表盘**（子项：分析看板、工作台）、**系统管理**、**订单**；版权页脚显示 `alexGo-cloud`。
4. `browser_network_requests` 核对（按 filter 抓关键路径）：
   - `POST /api/auth/login` → 200，响应体 `{"code":0,"data":{"accessToken":...}}`（`browser_network_request part=response-body`）；
   - `GET /api/auth/codes` → 200，`data` 为字符串数组（含 `system:role:*`、`order:order:*`）；
   - `GET /api/user/info` → 200，`"userId":"<数字串>"`（**是字符串**）、`realName` 非空；
   - `GET /api/menu/all` → 200，数组长度 **≥16**（19 行菜单 −3 个 button 不进路由），含 `component:""` 的目录节点。
5. 点击菜单 **仪表盘 → 分析看板**、**工作台** → 页面正常渲染（stock 页面存在）。
6. `browser_console_messages level=error`：**不得出现** `/api/auth/`、`/api/user/`、`/api/menu/` 相关报错、不得有 uncaught 异常。vben 自带仪表盘演示组件若请求 `/api/dashboard/*`（上游 mock 路径）→ 后端 404 的资源日志属**预期**（一期不接仪表盘 mock），不计失败。
7. 点击用户头像 → 退出登录 → 回到 `/login`；network 断言 `POST /api/auth/logout` 200 且请求头带 `Authorization`。
8. **不要点**系统管理/订单的子菜单——那些页面在阶段 D（T14-T24）才创建，此刻点了 not-found 属预期。

- [ ] **Step 6: 收尾 + Commit**

```bash
pkill -f "alexgo-server/cmd" || true
pkill -f "web-antd.*vite" || true
# 兜底：按端口杀
lsof -ti:8080 | xargs kill 2>/dev/null || true
lsof -ti:5666 | xargs kill 2>/dev/null || true
git add apps/web-antd/.env apps/web/antd/.env.development apps/web-antd/src/router/routes/modules/ 2>/dev/null || git add -A apps/web-antd
git commit -m "feat(web-antd): env branding, drop frontend module routes, login smoke"
```

（`git add` 路径以实际删除/修改为准：`.env`、`.env.development`、三个被删模块文件用 `git add -A apps/web-antd` 覆盖即可。）

**阶段 C 到此完成**：vendored monorepo 精简可用，登录→菜单→仪表盘全链路通，三端点契约与信封解包有浏览器证据。

---

# 阶段 D：API 层 + 11 个业务页面（T13-T24）

**全阶段统一范式**（每个任务的代码完整体现本节，不引用其他任务）：

- **结构**：`Page`（`@vben/common-ui`，`auto-content-height`）包裹 `Grid`；有创建弹窗的页面再挂 `<Modal><Form /></Modal>`（vben 弹窗组件，与 antd 的 `Modal` 静态方法区分见下）。
- **数据**：旧接口一次性返回全量数组 → `query` 里调 API 后用 `localPage(rows, page.currentPage, page.pageSize)` 切片，返回 `{ items, total }`（`#/adapter/vxe-table` 的 `proxyConfig.response` 已映射 `result: 'items'`、`total: 'total'`）。
- **列**：`VxeGridProps<T>`；时间列 `formatter: ({ cellValue }) => formatDateTime(cellValue)`；状态列 `formatter: ({ cellValue }) => (Number(cellValue) === 1 ? '启用' : '停用')`；操作列 `{ field: 'operation', fixed: 'right', slots: { default: 'actions' }, title: '操作', width: 140 }` + 模板 `<template #actions="{ row }">`（vben 官方 demo 即自定义具名 slot 形态，typecheck 通过）。
- **按钮**：新建放 `<template #toolbar-tools>`，挂 `v-access:code="['<页面 perm 原串>']"`（与 `/api/auth/codes` 返回值**逐字**匹配——`UserPermCodes` 收集用户全部菜单的 `permission` 原串，`hasAccessByCodes` 是 Set 精确命中，不做通配展开）。
- **表单**：`useVbenForm`（`#/adapter/form`）+ `showDefaultActions: false`；**数字字段一律 `InputNumber` 发数字**（后端 DTO 是 `uint64`/`int`，旧 admin-web 的字符串表单会被 `ShouldBindJSON` 拒成 400——新页面修正该缺陷）；状态用 `RadioGroup` + `defaultValue: 1`；旧表单的默认值放 schema 字段的 `defaultValue`。
- **弹窗**（`useVbenModal`，`@vben/common-ui`，`fullscreenButton: false`）：
  - `onOpenChange(isOpen)` 打开时 `formApi.resetForm()`（回到 schema `defaultValue`）；
  - `handleSubmit`（`useVbenForm` 顶层选项）：`await <create API>` → `message.success('创建成功')` → `modalApi.close()` → `gridApi.reload()`；API 失败时 rejection 上抛；
  - `onConfirm: async () => { try { await formApi.validateAndSubmitForm(); } catch { /* 拦截器已 toast，弹窗保持打开 */ } }`。依据 `form-api.ts` 源码：`validateAndSubmitForm` 校验不过时 **resolve `undefined` 且不触发 `handleSubmit`**（弹窗自然不关）；后端拒绝时 reject 被 catch（弹窗不关）；成功时 `handleSubmit` 已自行关窗。
- **删除确认**：`AntdModal.confirm`——`import { Button, message, Modal as AntdModal } from 'ant-design-vue'`（别名避开 vben `Modal` 变量名）；`onOk` 内 `try/catch` 吞错（拦截器已 toast），成功 `message.success('删除成功')` + `gridApi.reload()`。
- **每页** `defineOptions({ name: '<文件名>' })`；`gridOptions` 统一带 `height: 'auto'`、`keepSource: true`、`pagerConfig: {}`、`toolbarConfig: { custom: true, refresh: true, zoom: true }`。
- **每任务固定验证**：`pnpm --filter @vben/web-antd typecheck` → 绿 → commit。T14/T24/T26 另跑 `build` 与浏览器冒烟（各任务内单独列出）。

---

### Task 13: API 层 + 本地工具（11 页共用底座）

**Files:**
- Create: `apps/web-antd/src/utils/format.ts`
- Create: `apps/web-antd/src/utils/local-page.ts`
- Create: `apps/web-antd/src/api/admin.ts`

**Interfaces:**
- Consumes: `requestClient`（`apps/web-antd/src/api/request.ts` 导出；`baseURL = /api`，故路径**不带** `/api` 前缀；拦截器已把三形态响应解包成裸数据）；后端实体 JSON 字段（`modules/system/model/{model,rbac,basic,audit}.go`、`modules/order/model/order.go`）；旧 admin-web 的调用面（`admin-web/src/api/admin.ts`，全仓唯一 API 定义文件）。
- Produces（T14-T24 逐字依赖这些签名）：
  - `formatDateTime(value?: null | Date | number | string): string`
  - `localPage<T>(rows: T[], currentPage: number, pageSize: number): T[]`
  - 类型：`User/Role/Menu/Dept/Post/DictType/DictData/SystemConfig/Notice/LoginLog/OperateLog/Order`（字段见 Step 3）
  - 查询（均返回 `Promise<T[]>`）：`listUsers/listRoles/listMenus/listDepts/listPosts/listDictTypes/listConfigs/listNotices/listOrders`、`listDictDatas(typeId: number)`、`listLoginLogs(limit = 100)`、`listOperateLogs(limit = 100)`
  - 创建（均返回 Promise，页面不用其载荷）：`createUser(payload: { nickname: string; password: string; username: string })`、`createRole(payload: { code: string; name: string })`、`createMenu(payload: { component: string; icon: string; name: string; parent_id: number; path: string; permission: string; sort: number; status: number; type: string })`、`createDept(payload: { name: string; parent_id: number; sort: number; status: number })`、`createPost(payload: { code: string; name: string; sort: number; status: number })`、`createDictType(payload: { name: string; status: number; type: string })`、`createDictData(payload: { label: string; sort: number; status: number; type_id: number; value: string })`、`createConfig(payload: { key: string; name: string; remark: string; value: string })`、`createNotice(payload: { content: string; status: number; title: string })`、`createOrder(payload: { amount: number; order_no: string; product_id: number; status: number; user_id: number })`
  - 删除/动作：`deleteRole(id)/deleteMenu(id)/deleteDept(id)/deletePost(id)/deleteDictType(id)/deleteDictData(id)/deleteConfig(id)/deleteNotice(id)`（均 `(id: number) => Promise<{ status: string }>`）、`resetUserPassword(userId: number, password: string) => Promise<{ status: string }>`

- [ ] **Step 1: 写 `src/utils/format.ts` 与 `src/utils/local-page.ts`**

```ts
// apps/web-antd/src/utils/format.ts
import dayjs from 'dayjs';

/** MySQL datetime / ISO 时间串统一展示为 YYYY-MM-DD HH:mm:ss。 */
export function formatDateTime(value?: null | Date | number | string): string {
  if (value === null || value === undefined || value === '') return '';
  const d = dayjs(value);
  return d.isValid() ? d.format('YYYY-MM-DD HH:mm:ss') : String(value);
}
```

```ts
// apps/web-antd/src/utils/local-page.ts
/** 旧接口一次性返回全量数组，由前端切页。 */
export function localPage<T>(
  rows: T[],
  currentPage: number,
  pageSize: number,
): T[] {
  const start = (currentPage - 1) * pageSize;
  return rows.slice(start, start + pageSize);
}
```

> `dayjs` 是 `@vben/web-antd` 的直接依赖（package.json dependencies），`src/utils/` 是新建目录（上游 web-antd 没有 utils 目录）；`#/utils/*` 别名指向 `src/utils/*`。

- [ ] **Step 2: 写 `src/api/admin.ts`（承接旧 admin-web 全部调用）**

```ts
// apps/web-antd/src/api/admin.ts
/**
 * alexGo-cloud 管理端接口：承接旧 admin-web 的全部调用（路径去掉 /api 前缀——
 * baseURL = VITE_GLOB_API_URL = /api）。响应三形态由 src/api/request.ts 的
 * 拦截器解包：{"code":0,...}/{"data":...}/{"status":"ok"} 到这里都是裸数据。
 */
import { requestClient } from '#/api/request';

export interface User {
  id: number;
  username: string;
  nickname: string;
  status: number;
  created_at: string;
  updated_at: string;
}

export interface Role {
  id: number;
  code: string;
  name: string;
  status: number;
}

export interface Menu {
  id: number;
  parent_id: number;
  type: string; // dir | menu | button
  name: string;
  path: string;
  component: string;
  icon: string;
  permission: string;
  sort: number;
  status: number;
}

export interface Dept {
  id: number;
  parent_id: number;
  name: string;
  sort: number;
  status: number;
}

export interface Post {
  id: number;
  code: string;
  name: string;
  sort: number;
  status: number;
}

export interface DictType {
  id: number;
  type: string;
  name: string;
  status: number;
}

export interface DictData {
  id: number;
  type_id: number;
  label: string;
  value: string;
  sort: number;
  status: number;
}

export interface SystemConfig {
  id: number;
  key: string;
  value: string;
  name: string;
  remark?: string;
}

export interface Notice {
  id: number;
  title: string;
  content: string;
  status: number;
  created_at: string;
}

export interface LoginLog {
  id: number;
  tenant_id: number;
  username: string;
  user_id: number;
  ip: string;
  success: number;
  message: string;
  created_at: string;
}

export interface OperateLog {
  id: number;
  tenant_id: number;
  username: string;
  method: string;
  path: string;
  status: number;
  latency_ms: number;
  error: string;
  created_at: string;
}

export interface Order {
  id: number;
  user_id: number;
  product_id: number;
  amount: number;
  status: number;
  order_no: string;
  created_at: string;
}

// ---- 用户 ----
export function listUsers() {
  return requestClient.get<User[]>('/admin/system/users');
}

export function createUser(payload: {
  nickname: string;
  password: string;
  username: string;
}) {
  return requestClient.post<User>('/admin/system/users', payload);
}

export function resetUserPassword(userId: number, password: string) {
  return requestClient.post<{ status: string }>(
    `/admin/system/users/${userId}/reset-password`,
    { password },
  );
}

// ---- 角色 ----
export function listRoles() {
  return requestClient.get<Role[]>('/admin/system/roles');
}

export function createRole(payload: { code: string; name: string }) {
  return requestClient.post<Role>('/admin/system/roles', payload);
}

export function deleteRole(id: number) {
  return requestClient.delete<{ status: string }>(`/admin/system/roles/${id}`);
}

// ---- 菜单 ----
export function listMenus() {
  return requestClient.get<Menu[]>('/admin/system/menus');
}

export function createMenu(payload: {
  component: string;
  icon: string;
  name: string;
  parent_id: number;
  path: string;
  permission: string;
  sort: number;
  status: number;
  type: string;
}) {
  return requestClient.post<Menu>('/admin/system/menus', payload);
}

export function deleteMenu(id: number) {
  return requestClient.delete<{ status: string }>(`/admin/system/menus/${id}`);
}

// ---- 部门 ----
export function listDepts() {
  return requestClient.get<Dept[]>('/admin/system/depts');
}

export function createDept(payload: {
  name: string;
  parent_id: number;
  sort: number;
  status: number;
}) {
  return requestClient.post<Dept>('/admin/system/depts', payload);
}

export function deleteDept(id: number) {
  return requestClient.delete<{ status: string }>(`/admin/system/depts/${id}`);
}

// ---- 岗位 ----
export function listPosts() {
  return requestClient.get<Post[]>('/admin/system/posts');
}

export function createPost(payload: {
  code: string;
  name: string;
  sort: number;
  status: number;
}) {
  return requestClient.post<Post>('/admin/system/posts', payload);
}

export function deletePost(id: number) {
  return requestClient.delete<{ status: string }>(`/admin/system/posts/${id}`);
}

// ---- 字典 ----
export function listDictTypes() {
  return requestClient.get<DictType[]>('/admin/system/dict/types');
}

export function createDictType(payload: {
  name: string;
  status: number;
  type: string;
}) {
  return requestClient.post<DictType>('/admin/system/dict/types', payload);
}

export function deleteDictType(id: number) {
  return requestClient.delete<{ status: string }>(
    `/admin/system/dict/types/${id}`,
  );
}

export function listDictDatas(typeId: number) {
  return requestClient.get<DictData[]>(
    `/admin/system/dict/datas?type_id=${typeId}`,
  );
}

export function createDictData(payload: {
  label: string;
  sort: number;
  status: number;
  type_id: number;
  value: string;
}) {
  return requestClient.post<DictData>('/admin/system/dict/datas', payload);
}

export function deleteDictData(id: number) {
  return requestClient.delete<{ status: string }>(
    `/admin/system/dict/datas/${id}`,
  );
}

// ---- 系统参数 ----
export function listConfigs() {
  return requestClient.get<SystemConfig[]>('/admin/system/configs');
}

export function createConfig(payload: {
  key: string;
  name: string;
  remark: string;
  value: string;
}) {
  return requestClient.post<SystemConfig>('/admin/system/configs', payload);
}

export function deleteConfig(id: number) {
  return requestClient.delete<{ status: string }>(
    `/admin/system/configs/${id}`,
  );
}

// ---- 通知公告 ----
export function listNotices() {
  return requestClient.get<Notice[]>('/admin/system/notices');
}

export function createNotice(payload: {
  content: string;
  status: number;
  title: string;
}) {
  return requestClient.post<Notice>('/admin/system/notices', payload);
}

export function deleteNotice(id: number) {
  return requestClient.delete<{ status: string }>(
    `/admin/system/notices/${id}`,
  );
}

// ---- 日志 ----
export function listLoginLogs(limit = 100) {
  return requestClient.get<LoginLog[]>(
    `/admin/system/logs/login?limit=${limit}`,
  );
}

export function listOperateLogs(limit = 100) {
  return requestClient.get<OperateLog[]>(
    `/admin/system/logs/operate?limit=${limit}`,
  );
}

// ---- 订单 ----
export function listOrders() {
  return requestClient.get<Order[]>('/admin/order/orders');
}

export function createOrder(payload: {
  amount: number;
  order_no: string;
  product_id: number;
  status: number;
  user_id: number;
}) {
  return requestClient.post<Order>('/admin/order/orders', payload);
}
```

> 旧 `setUserRoles` 不移植：全仓无调用点（死代码）。`profile/login` 也不移植——登录由 vben 自带 `api/core/auth.ts` 走 `/api/auth/login`。

- [ ] **Step 3: 静态验证**

```bash
pnpm --filter @vben/web-antd typecheck
pnpm --filter @vben/web-antd build
```

Expected: 双绿。

- [ ] **Step 4: Commit**

```bash
git add apps/web-antd/src/api/admin.ts apps/web-antd/src/utils
git commit -m "feat(web-antd): legacy admin api layer with local paging utils"
```

---

### Task 14: SystemUsersPage（用户管理）——承担三形态信封的运行期断言

**Files:**
- Create: `apps/web-antd/src/views/system/SystemUsersPage.vue`
- Test: 浏览器冒烟（Step 3，Playwright MCP）

**Interfaces:**
- Consumes: T13 的 `listUsers/createUser/resetUserPassword/User`；后端菜单行（seed：`path=/system/users`、`component=views/system/SystemUsersPage`、`permission=system:user:list`）→ 路由 `GET /system/users` 渲染本文件（component 字段不带 `.vue`，vben `pageMap = import.meta.glob('../views/**/*.vue')` 自动拼上）。
- Produces: `/system/users` 页面（列表+创建+重置密码）；`SystemUsersPage` 组件名；Review Focus 1 的三条运行期证据（Step 3 断言①②③）。

- [ ] **Step 1: 写页面文件**

```vue
<!-- apps/web-antd/src/views/system/SystemUsersPage.vue -->
<script lang="ts" setup>
import type { VxeGridProps } from '#/adapter/vxe-table';

import { Page, useVbenModal } from '@vben/common-ui';

import { Button, message } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { createUser, listUsers, resetUserPassword, type User } from '#/api/admin';
import { formatDateTime } from '#/utils/format';
import { localPage } from '#/utils/local-page';

defineOptions({ name: 'SystemUsersPage' });

const gridOptions: VxeGridProps<User> = {
  columns: [
    { field: 'id', title: 'ID', width: 80 },
    { field: 'username', title: '用户名', minWidth: 140 },
    { field: 'nickname', title: '昵称', minWidth: 140 },
    {
      field: 'status',
      formatter: ({ cellValue }) => (Number(cellValue) === 1 ? '启用' : '停用'),
      title: '状态',
      width: 80,
    },
    {
      field: 'created_at',
      formatter: ({ cellValue }) => formatDateTime(cellValue),
      title: '创建时间',
      width: 180,
    },
    {
      field: 'updated_at',
      formatter: ({ cellValue }) => formatDateTime(cellValue),
      title: '更新时间',
      width: 180,
    },
    {
      field: 'operation',
      fixed: 'right',
      slots: { default: 'actions' },
      title: '操作',
      width: 130,
    },
  ],
  height: 'auto',
  keepSource: true,
  pagerConfig: {},
  proxyConfig: {
    ajax: {
      query: async ({ page }) => {
        const rows = await listUsers();
        return {
          items: localPage(rows, page.currentPage, page.pageSize),
          total: rows.length,
        };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: true, zoom: true },
};

const [Grid, gridApi] = useVbenVxeGrid({ gridOptions });

const [Modal, modalApi] = useVbenModal({
  fullscreenButton: false,
  title: '新建用户',
  onOpenChange(isOpen: boolean) {
    if (isOpen) formApi.resetForm();
  },
  onConfirm: async () => {
    try {
      await formApi.validateAndSubmitForm();
    } catch {
      // 后端错误已由拦截器 toast；弹窗保持打开
    }
  },
});

const [Form, formApi] = useVbenForm({
  handleSubmit: async (values) => {
    await createUser(
      values as { nickname: string; password: string; username: string },
    );
    message.success('创建成功');
    modalApi.close();
    gridApi.reload();
  },
  schema: [
    {
      component: 'Input',
      componentProps: { placeholder: '请输入用户名' },
      fieldName: 'username',
      label: '用户名',
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '请输入昵称' },
      fieldName: 'nickname',
      label: '昵称',
      rules: 'required',
    },
    {
      component: 'InputPassword',
      componentProps: { placeholder: '请输入初始密码' },
      fieldName: 'password',
      label: '密码',
      rules: 'required',
    },
  ],
  showDefaultActions: false,
});

function onResetPassword(row: User) {
  const pwd = window.prompt(`为 ${row.username} 设置新密码`);
  if (!pwd) return;
  resetUserPassword(row.id, pwd)
    .then(() => message.success('密码已重置'))
    .catch(() => {
      // 拦截器已 toast
    });
}
</script>

<template>
  <Page auto-content-height>
    <Grid>
      <template #toolbar-tools>
        <Button
          class="mr-2"
          type="primary"
          v-access:code="['system:user:list']"
          @click="() => modalApi.open()"
        >
          新建用户
        </Button>
      </template>
      <template #actions="{ row }">
        <Button
          size="small"
          type="link"
          v-access:code="['system:user:list']"
          @click="onResetPassword(row)"
        >
          重置密码
        </Button>
      </template>
    </Grid>
    <Modal>
      <Form />
    </Modal>
  </Page>
</template>
```

- [ ] **Step 2: 静态验证**

```bash
pnpm --filter @vben/web-antd typecheck
pnpm --filter @vben/web-antd build
```

Expected: 双绿。

- [ ] **Step 3: 浏览器冒烟（Review Focus 1 的形态断言在这里落地）**

启动双端：

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud
make run > /tmp/alexgo-dev-backend.log 2>&1 &
for i in $(seq 1 30); do curl -sf http://localhost:8080/health >/dev/null && break; sleep 1; done
curl -sf http://localhost:8080/health && echo " backend UP"
pnpm --filter @vben/web-antd dev > /tmp/alexgo-dev-web.log 2>&1 &
for i in $(seq 1 30); do curl -sf http://localhost:5666/ >/dev/null && break; sleep 1; done
curl -sf http://localhost:5666/ >/dev/null && echo " frontend UP"
```

Playwright 操作与断言：

1. `browser_navigate` → `http://localhost:5666/` → 登录页 → `admin`/`admin123` 登录 → 落 `/dashboard/analytics`。
2. 侧栏 **系统管理 → 用户管理** → URL 变 `/system/users`，表格渲染出「用户名」列头与 seed 的 admin 行（≥1 行）。
3. **断言①`{data:[...]}` 形态**：`browser_network_requests` 找 `GET /api/admin/system/users` → `browser_network_request part=response-body` → 200 且 body 以 `{"data":[` 开头（**无 `code` 字段**——旧信封；若拦截器当失败抛出，列表会空并弹红 toast，即 Review Focus 1 的故障态）。
4. 点 **新建用户** → 弹窗 → 填 `vben_smoke` / `冒烟用户` / `123456` → 点确认键 → 断言：toast「创建成功」、表格 reload 后出现 `vben_smoke` 行、`POST /api/admin/system/users` 200 且 body 以 `{"data":{` 开头。
5. **断言③`{"status":"ok"}` 形态**：行上点 **重置密码** → 出现浏览器 prompt → `browser_handle_dialog`（accept，文本 `Sm0ke!pass`）→ 断言 toast「密码已重置」、`POST /api/admin/system/users/<id>/reset-password` 200 且 body **恰为** `{"status":"ok"}`（无 `code`、无 `data` 键——拦截器必须返回原 body 而非当成失败）。
6. `browser_console_messages level=error`：**不得有 uncaught JS 异常**；`/api/dashboard/*` 的 404 资源错误属预期（容忍规则同 T12：stock 仪表盘组件请求 mock 路径）。
7. 停双端：

```bash
pkill -f "alexgo-server/cmd" || true
pkill -f "web-antd.*vite" || true
lsof -ti:8080 | xargs kill 2>/dev/null || true
lsof -ti:5666 | xargs kill 2>/dev/null || true
```

- [ ] **Step 4: Commit**

```bash
git add apps/web-antd/src/views/system/SystemUsersPage.vue
git commit -m "feat(web-antd): system users page (list/create/reset-password)"
```

---

### Task 15: SystemRolesPage（角色管理）

**Files:**
- Create: `apps/web-antd/src/views/system/SystemRolesPage.vue`

**Interfaces:**
- Consumes: T13 `listRoles/createRole/deleteRole/Role`；seed 菜单 `path=/system/roles`、`component=views/system/SystemRolesPage`、`permission=system:role:*`；范式约定（阶段 D 头部）。
- Produces: `/system/roles` 页面（列表+创建+删除）；`SystemRolesPage` 组件名。

- [ ] **Step 1: 写页面文件**

```vue
<!-- apps/web-antd/src/views/system/SystemRolesPage.vue -->
<script lang="ts" setup>
import type { VxeGridProps } from '#/adapter/vxe-table';

import { Page, useVbenModal } from '@vben/common-ui';

import { Button, message, Modal as AntdModal } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { createRole, deleteRole, listRoles, type Role } from '#/api/admin';
import { localPage } from '#/utils/local-page';

defineOptions({ name: 'SystemRolesPage' });

const gridOptions: VxeGridProps<Role> = {
  columns: [
    { field: 'id', title: 'ID', width: 80 },
    { field: 'code', title: '编码', minWidth: 140 },
    { field: 'name', title: '名称', minWidth: 160 },
    {
      field: 'status',
      formatter: ({ cellValue }) => (Number(cellValue) === 1 ? '启用' : '停用'),
      title: '状态',
      width: 80,
    },
    {
      field: 'operation',
      fixed: 'right',
      slots: { default: 'actions' },
      title: '操作',
      width: 130,
    },
  ],
  height: 'auto',
  keepSource: true,
  pagerConfig: {},
  proxyConfig: {
    ajax: {
      query: async ({ page }) => {
        const rows = await listRoles();
        return {
          items: localPage(rows, page.currentPage, page.pageSize),
          total: rows.length,
        };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: true, zoom: true },
};

const [Grid, gridApi] = useVbenVxeGrid({ gridOptions });

const [Modal, modalApi] = useVbenModal({
  fullscreenButton: false,
  title: '新建角色',
  onOpenChange(isOpen: boolean) {
    if (isOpen) formApi.resetForm();
  },
  onConfirm: async () => {
    try {
      await formApi.validateAndSubmitForm();
    } catch {
      // 拦截器已 toast，弹窗保持打开
    }
  },
});

const [Form, formApi] = useVbenForm({
  handleSubmit: async (values) => {
    await createRole(values as { code: string; name: string });
    message.success('创建成功');
    modalApi.close();
    gridApi.reload();
  },
  schema: [
    {
      component: 'Input',
      componentProps: { placeholder: '如 ops_admin' },
      fieldName: 'code',
      label: '角色编码',
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '请输入角色名称' },
      fieldName: 'name',
      label: '角色名称',
      rules: 'required',
    },
  ],
  showDefaultActions: false,
});

function onDelete(row: Role) {
  AntdModal.confirm({
    title: `确认删除角色「${row.name}」？`,
    onOk: async () => {
      try {
        await deleteRole(row.id);
      } catch {
        return; // 拦截器已 toast，数据保持不动
      }
      message.success('删除成功');
      gridApi.reload();
    },
  });
}
</script>

<template>
  <Page auto-content-height>
    <Grid>
      <template #toolbar-tools>
        <Button
          class="mr-2"
          type="primary"
          v-access:code="['system:role:*']"
          @click="() => modalApi.open()"
        >
          新建角色
        </Button>
      </template>
      <template #actions="{ row }">
        <Button
          danger
          size="small"
          type="link"
          v-access:code="['system:role:*']"
          @click="onDelete(row)"
        >
          删除
        </Button>
      </template>
    </Grid>
    <Modal>
      <Form />
    </Modal>
  </Page>
</template>
```

- [ ] **Step 2: 静态验证**

```bash
pnpm --filter @vben/web-antd typecheck
```

Expected: 绿。

- [ ] **Step 3: Commit**

```bash
git add apps/web-antd/src/views/system/SystemRolesPage.vue
git commit -m "feat(web-antd): system roles page (list/create/delete)"
```

---

### Task 16: SystemMenusPage（菜单管理）

**Files:**
- Create: `apps/web-antd/src/views/system/SystemMenusPage.vue`

**Interfaces:**
- Consumes: T13 `listMenus/createMenu/deleteMenu/Menu`；seed 菜单 `path=/system/menus`、`component=views/system/SystemMenusPage`、`permission=system:menu:*`；`createMenu` payload 九字段（T13 Interfaces 块）。
- Produces: `/system/menus` 页面（列表+创建+删除）；`SystemMenusPage` 组件名。

- [ ] **Step 1: 写页面文件**

```vue
<!-- apps/web-antd/src/views/system/SystemMenusPage.vue -->
<script lang="ts" setup>
import type { VxeGridProps } from '#/adapter/vxe-table';

import { Page, useVbenModal } from '@vben/common-ui';

import { Button, message, Modal as AntdModal } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { createMenu, deleteMenu, listMenus, type Menu } from '#/api/admin';
import { localPage } from '#/utils/local-page';

defineOptions({ name: 'SystemMenusPage' });

const gridOptions: VxeGridProps<Menu> = {
  columns: [
    { field: 'id', title: 'ID', width: 70 },
    { field: 'parent_id', title: '父ID', width: 80 },
    {
      field: 'type',
      formatter: ({ cellValue }) =>
        cellValue === 'dir' ? '目录' : cellValue === 'button' ? '按钮' : '菜单',
      title: '类型',
      width: 80,
    },
    { field: 'name', title: '名称', minWidth: 140 },
    { field: 'path', title: '路径', minWidth: 160 },
    { field: 'permission', title: '权限', minWidth: 150 },
    { field: 'sort', title: '排序', width: 70 },
    {
      field: 'status',
      formatter: ({ cellValue }) => (Number(cellValue) === 1 ? '启用' : '停用'),
      title: '状态',
      width: 80,
    },
    {
      field: 'operation',
      fixed: 'right',
      slots: { default: 'actions' },
      title: '操作',
      width: 130,
    },
  ],
  height: 'auto',
  keepSource: true,
  pagerConfig: {},
  proxyConfig: {
    ajax: {
      query: async ({ page }) => {
        const rows = await listMenus();
        return {
          items: localPage(rows, page.currentPage, page.pageSize),
          total: rows.length,
        };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: true, zoom: true },
};

const [Grid, gridApi] = useVbenVxeGrid({ gridOptions });

const [Modal, modalApi] = useVbenModal({
  fullscreenButton: false,
  title: '新建菜单',
  onOpenChange(isOpen: boolean) {
    if (isOpen) formApi.resetForm();
  },
  onConfirm: async () => {
    try {
      await formApi.validateAndSubmitForm();
    } catch {
      // 拦截器已 toast，弹窗保持打开
    }
  },
});

const [Form, formApi] = useVbenForm({
  handleSubmit: async (values) => {
    await createMenu(
      values as {
        component: string;
        icon: string;
        name: string;
        parent_id: number;
        path: string;
        permission: string;
        sort: number;
        status: number;
        type: string;
      },
    );
    message.success('创建成功');
    modalApi.close();
    gridApi.reload();
  },
  schema: [
    {
      component: 'InputNumber',
      componentProps: { min: 0 },
      defaultValue: 0,
      fieldName: 'parent_id',
      label: '父ID（0=根）',
    },
    {
      component: 'Select',
      componentProps: {
        options: [
          { label: '目录', value: 'dir' },
          { label: '菜单', value: 'menu' },
          { label: '按钮', value: 'button' },
        ],
      },
      defaultValue: 'menu',
      fieldName: 'type',
      label: '类型',
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '如导出日志' },
      fieldName: 'name',
      label: '名称',
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '如 /system/export' },
      fieldName: 'path',
      label: '路径',
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '如 views/system/SystemExportPage（按钮留空）' },
      fieldName: 'component',
      label: '组件路径',
      defaultValue: '',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '如 lucide:file-down' },
      fieldName: 'icon',
      label: '图标',
      defaultValue: '',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '如 system:log:export' },
      fieldName: 'permission',
      label: '权限串',
      defaultValue: '',
    },
    {
      component: 'InputNumber',
      componentProps: { min: 0 },
      defaultValue: 1,
      fieldName: 'sort',
      label: '排序',
    },
    {
      component: 'RadioGroup',
      componentProps: {
        options: [
          { label: '启用', value: 1 },
          { label: '停用', value: 0 },
        ],
      },
      defaultValue: 1,
      fieldName: 'status',
      label: '状态',
      rules: 'required',
    },
  ],
  showDefaultActions: false,
});

function onDelete(row: Menu) {
  AntdModal.confirm({
    title: `确认删除菜单「${row.name}」？`,
    onOk: async () => {
      try {
        await deleteMenu(row.id);
      } catch {
        return; // 拦截器已 toast
      }
      message.success('删除成功');
      gridApi.reload();
    },
  });
}
</script>

<template>
  <Page auto-content-height>
    <Grid>
      <template #toolbar-tools>
        <Button
          class="mr-2"
          type="primary"
          v-access:code="['system:menu:*']"
          @click="() => modalApi.open()"
        >
          新建菜单
        </Button>
      </template>
      <template #actions="{ row }">
        <Button
          danger
          size="small"
          type="link"
          v-access:code="['system:menu:*']"
          @click="onDelete(row)"
        >
          删除
        </Button>
      </template>
    </Grid>
    <Modal>
      <Form />
    </Modal>
  </Page>
</template>
```

- [ ] **Step 2: 静态验证**

```bash
pnpm --filter @vben/web-antd typecheck
```

Expected: 绿。

- [ ] **Step 3: Commit**

```bash
git add apps/web-antd/src/views/system/SystemMenusPage.vue
git commit -m "feat(web-antd): system menus page (list/create/delete)"
```

---

### Task 17: SystemDeptsPage（部门管理）

**Files:**
- Create: `apps/web-antd/src/views/system/SystemDeptsPage.vue`

**Interfaces:**
- Consumes: T13 `listDepts/createDept/deleteDept/Dept`；seed 菜单 `path=/system/depts`、`component=views/system/SystemDeptsPage`、`permission=system:dept:*`；后端 `CreateDept` 绑定 `model.Dept`（`ParentID uint64`、`Sort int` → payload 必须是数字，T13 Interfaces 块已约定）。
- Produces: `/system/depts` 页面（列表+创建+删除）；`SystemDeptsPage` 组件名。

- [ ] **Step 1: 写页面文件**

```vue
<!-- apps/web-antd/src/views/system/SystemDeptsPage.vue -->
<script lang="ts" setup>
import type { VxeGridProps } from '#/adapter/vxe-table';

import { Page, useVbenModal } from '@vben/common-ui';

import { Button, message, Modal as AntdModal } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { createDept, deleteDept, listDepts, type Dept } from '#/api/admin';
import { localPage } from '#/utils/local-page';

defineOptions({ name: 'SystemDeptsPage' });

const gridOptions: VxeGridProps<Dept> = {
  columns: [
    { field: 'id', title: 'ID', width: 80 },
    { field: 'parent_id', title: '父ID', width: 80 },
    { field: 'name', title: '部门名称', minWidth: 180 },
    { field: 'sort', title: '排序', width: 80 },
    {
      field: 'status',
      formatter: ({ cellValue }) => (Number(cellValue) === 1 ? '启用' : '停用'),
      title: '状态',
      width: 80,
    },
    {
      field: 'operation',
      fixed: 'right',
      slots: { default: 'actions' },
      title: '操作',
      width: 130,
    },
  ],
  height: 'auto',
  keepSource: true,
  pagerConfig: {},
  proxyConfig: {
    ajax: {
      query: async ({ page }) => {
        const rows = await listDepts();
        return {
          items: localPage(rows, page.currentPage, page.pageSize),
          total: rows.length,
        };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: true, zoom: true },
};

const [Grid, gridApi] = useVbenVxeGrid({ gridOptions });

const [Modal, modalApi] = useVbenModal({
  fullscreenButton: false,
  title: '新建部门',
  onOpenChange(isOpen: boolean) {
    if (isOpen) formApi.resetForm();
  },
  onConfirm: async () => {
    try {
      await formApi.validateAndSubmitForm();
    } catch {
      // 拦截器已 toast，弹窗保持打开
    }
  },
});

const [Form, formApi] = useVbenForm({
  handleSubmit: async (values) => {
    await createDept(
      values as { name: string; parent_id: number; sort: number; status: number },
    );
    message.success('创建成功');
    modalApi.close();
    gridApi.reload();
  },
  schema: [
    {
      component: 'Input',
      componentProps: { placeholder: '请输入部门名称' },
      fieldName: 'name',
      label: '部门名称',
      rules: 'required',
    },
    {
      component: 'InputNumber',
      componentProps: { min: 0 },
      defaultValue: 0,
      fieldName: 'parent_id',
      label: '父ID（0=根）',
    },
    {
      component: 'InputNumber',
      componentProps: { min: 0 },
      defaultValue: 1,
      fieldName: 'sort',
      label: '排序',
    },
    {
      component: 'RadioGroup',
      componentProps: {
        options: [
          { label: '启用', value: 1 },
          { label: '停用', value: 0 },
        ],
      },
      defaultValue: 1,
      fieldName: 'status',
      label: '状态',
      rules: 'required',
    },
  ],
  showDefaultActions: false,
});

function onDelete(row: Dept) {
  AntdModal.confirm({
    title: `确认删除部门「${row.name}」？`,
    onOk: async () => {
      try {
        await deleteDept(row.id);
      } catch {
        return; // 拦截器已 toast
      }
      message.success('删除成功');
      gridApi.reload();
    },
  });
}
</script>

<template>
  <Page auto-content-height>
    <Grid>
      <template #toolbar-tools>
        <Button
          class="mr-2"
          type="primary"
          v-access:code="['system:dept:*']"
          @click="() => modalApi.open()"
        >
          新建部门
        </Button>
      </template>
      <template #actions="{ row }">
        <Button
          danger
          size="small"
          type="link"
          v-access:code="['system:dept:*']"
          @click="onDelete(row)"
        >
          删除
        </Button>
      </template>
    </Grid>
    <Modal>
      <Form />
    </Modal>
  </Page>
</template>
```

- [ ] **Step 2: 静态验证**

```bash
pnpm --filter @vben/web-antd typecheck
```

Expected: 绿。

- [ ] **Step 3: Commit**

```bash
git add apps/web-antd/src/views/system/SystemDeptsPage.vue
git commit -m "feat(web-antd): system depts page (list/create/delete)"
```

---

### Task 18: SystemPostsPage（岗位管理）

**Files:**
- Create: `apps/web-antd/src/views/system/SystemPostsPage.vue`

**Interfaces:**
- Consumes: T13 `listPosts/createPost/deletePost/Post`；seed 菜单 `path=/system/posts`、`component=views/system/SystemPostsPage`、`permission=system:post:*`；后端 `model.Post` 的 `Sort int`/`Status int`（数字 payload）。
- Produces: `/system/posts` 页面（列表+创建+删除）；`SystemPostsPage` 组件名。

- [ ] **Step 1: 写页面文件**

```vue
<!-- apps/web-antd/src/views/system/SystemPostsPage.vue -->
<script lang="ts" setup>
import type { VxeGridProps } from '#/adapter/vxe-table';

import { Page, useVbenModal } from '@vben/common-ui';

import { Button, message, Modal as AntdModal } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { createPost, deletePost, listPosts, type Post } from '#/api/admin';
import { localPage } from '#/utils/local-page';

defineOptions({ name: 'SystemPostsPage' });

const gridOptions: VxeGridProps<Post> = {
  columns: [
    { field: 'id', title: 'ID', width: 80 },
    { field: 'code', title: '岗位编码', minWidth: 140 },
    { field: 'name', title: '岗位名称', minWidth: 160 },
    { field: 'sort', title: '排序', width: 80 },
    {
      field: 'status',
      formatter: ({ cellValue }) => (Number(cellValue) === 1 ? '启用' : '停用'),
      title: '状态',
      width: 80,
    },
    {
      field: 'operation',
      fixed: 'right',
      slots: { default: 'actions' },
      title: '操作',
      width: 130,
    },
  ],
  height: 'auto',
  keepSource: true,
  pagerConfig: {},
  proxyConfig: {
    ajax: {
      query: async ({ page }) => {
        const rows = await listPosts();
        return {
          items: localPage(rows, page.currentPage, page.pageSize),
          total: rows.length,
        };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: true, zoom: true },
};

const [Grid, gridApi] = useVbenVxeGrid({ gridOptions });

const [Modal, modalApi] = useVbenModal({
  fullscreenButton: false,
  title: '新建岗位',
  onOpenChange(isOpen: boolean) {
    if (isOpen) formApi.resetForm();
  },
  onConfirm: async () => {
    try {
      await formApi.validateAndSubmitForm();
    } catch {
      // 拦截器已 toast，弹窗保持打开
    }
  },
});

const [Form, formApi] = useVbenForm({
  handleSubmit: async (values) => {
    await createPost(
      values as { code: string; name: string; sort: number; status: number },
    );
    message.success('创建成功');
    modalApi.close();
    gridApi.reload();
  },
  schema: [
    {
      component: 'Input',
      componentProps: { placeholder: '如 dev' },
      fieldName: 'code',
      label: '岗位编码',
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '如 开发工程师' },
      fieldName: 'name',
      label: '岗位名称',
      rules: 'required',
    },
    {
      component: 'InputNumber',
      componentProps: { min: 0 },
      defaultValue: 1,
      fieldName: 'sort',
      label: '排序',
    },
    {
      component: 'RadioGroup',
      componentProps: {
        options: [
          { label: '启用', value: 1 },
          { label: '停用', value: 0 },
        ],
      },
      defaultValue: 1,
      fieldName: 'status',
      label: '状态',
      rules: 'required',
    },
  ],
  showDefaultActions: false,
});

function onDelete(row: Post) {
  AntdModal.confirm({
    title: `确认删除岗位「${row.name}」？`,
    onOk: async () => {
      try {
        await deletePost(row.id);
      } catch {
        return; // 拦截器已 toast
      }
      message.success('删除成功');
      gridApi.reload();
    },
  });
}
</script>

<template>
  <Page auto-content-height>
    <Grid>
      <template #toolbar-tools>
        <Button
          class="mr-2"
          type="primary"
          v-access:code="['system:post:*']"
          @click="() => modalApi.open()"
        >
          新建岗位
        </Button>
      </template>
      <template #actions="{ row }">
        <Button
          danger
          size="small"
          type="link"
          v-access:code="['system:post:*']"
          @click="onDelete(row)"
        >
          删除
        </Button>
      </template>
    </Grid>
    <Modal>
      <Form />
    </Modal>
  </Page>
</template>
```

- [ ] **Step 2: 静态验证**

```bash
pnpm --filter @vben/web-antd typecheck
```

Expected: 绿。

- [ ] **Step 3: Commit**

```bash
git add apps/web-antd/src/views/system/SystemPostsPage.vue
git commit -m "feat(web-antd): system posts page (list/create/delete)"
```

---

### Task 19: SystemDictPage（字典管理，双表）

**Files:**
- Create: `apps/web-antd/src/views/system/SystemDictPage.vue`

**Interfaces:**
- Consumes: T13 `listDictTypes/createDictType/deleteDictType/listDictDatas/createDictData/deleteDictData/DictType/DictData`（`listDictDatas(typeId: number)` 带 `?type_id=`）；seed 菜单 `path=/system/dict`、`component=views/system/SystemDictPage`、`permission=system:dict:*`；`useVbenVxeGrid` 可多实例调用（官方文档多 grid 同页）；`table-title` 属性可动态绑定（vxe-grid 支持）。
- Produces: `/system/dict` 页面（上=字典类型表、下=字典数据表，操作列「字典数据」选中类型并刷新数据表）；`SystemDictPage` 组件名。

- [ ] **Step 1: 写页面文件**

```vue
<!-- apps/web-antd/src/views/system/SystemDictPage.vue -->
<script lang="ts" setup>
import type { VxeGridProps } from '#/adapter/vxe-table';

import { Page, useVbenModal } from '@vben/common-ui';

import { Button, message, Modal as AntdModal } from 'ant-design-vue';

import { ref } from 'vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import {
  createDictData,
  createDictType,
  deleteDictData,
  deleteDictType,
  listDictDatas,
  listDictTypes,
  type DictData,
  type DictType,
} from '#/api/admin';
import { localPage } from '#/utils/local-page';

defineOptions({ name: 'SystemDictPage' });

/** 上表：字典类型。行操作列的「字典数据」按钮选中该行，驱动下表刷新。 */
const typeGridOptions: VxeGridProps<DictType> = {
  columns: [
    { field: 'id', title: 'ID', width: 70 },
    { field: 'type', title: '字典类型', minWidth: 160 },
    { field: 'name', title: '名称', minWidth: 140 },
    {
      field: 'status',
      formatter: ({ cellValue }) => (Number(cellValue) === 1 ? '启用' : '停用'),
      title: '状态',
      width: 80,
    },
    {
      field: 'operation',
      fixed: 'right',
      slots: { default: 'actions' },
      title: '操作',
      width: 190,
    },
  ],
  height: 'auto',
  keepSource: true,
  pagerConfig: {},
  proxyConfig: {
    ajax: {
      query: async ({ page }) => {
        const rows = await listDictTypes();
        return {
          items: localPage(rows, page.currentPage, page.pageSize),
          total: rows.length,
        };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: true, zoom: true },
};

const [TypeGrid, typeGridApi] = useVbenVxeGrid({ gridOptions: typeGridOptions });

/** 当前选中的字典类型；下表的 data query、新建数据的 type_id、动态表题都读它。 */
const selectedType = ref<DictType | null>(null);

/** 下表：字典数据。未选中类型时返回空集，表格空转。 */
const dataGridOptions: VxeGridProps<DictData> = {
  columns: [
    { field: 'id', title: 'ID', width: 70 },
    { field: 'label', title: '标签', minWidth: 140 },
    { field: 'value', title: '值', minWidth: 120 },
    { field: 'sort', title: '排序', width: 70 },
    {
      field: 'status',
      formatter: ({ cellValue }) => (Number(cellValue) === 1 ? '启用' : '停用'),
      title: '状态',
      width: 80,
    },
    {
      field: 'operation',
      fixed: 'right',
      slots: { default: 'actions' },
      title: '操作',
      width: 130,
    },
  ],
  height: 'auto',
  keepSource: true,
  pagerConfig: {},
  proxyConfig: {
    ajax: {
      query: async ({ page }) => {
        if (!selectedType.value) return { items: [], total: 0 };
        const rows = await listDictDatas(selectedType.value.id);
        return {
          items: localPage(rows, page.currentPage, page.pageSize),
          total: rows.length,
        };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: true, zoom: true },
};

const [DataGrid, dataGridApi] = useVbenVxeGrid({ gridOptions: dataGridOptions });

function onSelectDictType(row: DictType) {
  selectedType.value = row;
  dataGridApi.reload();
}

const [TypeModal, typeModalApi] = useVbenModal({
  fullscreenButton: false,
  title: '新建字典类型',
  onOpenChange(isOpen: boolean) {
    if (isOpen) typeFormApi.resetForm();
  },
  onConfirm: async () => {
    try {
      await typeFormApi.validateAndSubmitForm();
    } catch {
      // 拦截器已 toast，弹窗保持打开
    }
  },
});

const [TypeForm, typeFormApi] = useVbenForm({
  handleSubmit: async (values) => {
    await createDictType(
      values as { name: string; status: number; type: string },
    );
    message.success('创建成功');
    typeModalApi.close();
    typeGridApi.reload();
  },
  schema: [
    {
      component: 'Input',
      componentProps: { placeholder: '如 sys_user_sex' },
      fieldName: 'type',
      label: '字典类型',
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '如 用户性别' },
      fieldName: 'name',
      label: '名称',
      rules: 'required',
    },
    {
      component: 'RadioGroup',
      componentProps: {
        options: [
          { label: '启用', value: 1 },
          { label: '停用', value: 0 },
        ],
      },
      defaultValue: 1,
      fieldName: 'status',
      label: '状态',
      rules: 'required',
    },
  ],
  showDefaultActions: false,
});

const [DataModal, dataModalApi] = useVbenModal({
  fullscreenButton: false,
  title: '新建字典数据',
  onOpenChange(isOpen: boolean) {
    if (isOpen) dataFormApi.resetForm();
  },
  onConfirm: async () => {
    try {
      await dataFormApi.validateAndSubmitForm();
    } catch {
      // 拦截器已 toast，弹窗保持打开
    }
  },
});

const [DataForm, dataFormApi] = useVbenForm({
  handleSubmit: async (values) => {
    if (!selectedType.value) {
      message.warning('请先在字典类型表中点击「字典数据」');
      return;
    }
    await createDictData({
      ...(values as { label: string; sort: number; status: number; value: string }),
      type_id: selectedType.value.id,
    });
    message.success('创建成功');
    dataModalApi.close();
    dataGridApi.reload();
  },
  schema: [
    {
      component: 'Input',
      componentProps: { placeholder: '如 男' },
      fieldName: 'label',
      label: '标签',
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '如 1' },
      fieldName: 'value',
      label: '值',
      rules: 'required',
    },
    {
      component: 'InputNumber',
      componentProps: { min: 0 },
      defaultValue: 1,
      fieldName: 'sort',
      label: '排序',
    },
    {
      component: 'RadioGroup',
      componentProps: {
        options: [
          { label: '启用', value: 1 },
          { label: '停用', value: 0 },
        ],
      },
      defaultValue: 1,
      fieldName: 'status',
      label: '状态',
      rules: 'required',
    },
  ],
  showDefaultActions: false,
});

function onDeleteType(row: DictType) {
  AntdModal.confirm({
    title: `确认删除字典类型「${row.name}」？其下数据请自行清理。`,
    onOk: async () => {
      try {
        await deleteDictType(row.id);
      } catch {
        return; // 拦截器已 toast
      }
      if (selectedType.value?.id === row.id) selectedType.value = null;
      message.success('删除成功');
      typeGridApi.reload();
      dataGridApi.reload();
    },
  });
}

function onDeleteData(row: DictData) {
  AntdModal.confirm({
    title: `确认删除字典数据「${row.label}」？`,
    onOk: async () => {
      try {
        await deleteDictData(row.id);
      } catch {
        return; // 拦截器已 toast
      }
      message.success('删除成功');
      dataGridApi.reload();
    },
  });
}
</script>

<template>
  <Page auto-content-height>
    <div class="flex h-full flex-col gap-4">
      <div class="h-[45%]">
        <TypeGrid>
          <template #toolbar-tools>
            <Button
              class="mr-2"
              type="primary"
              v-access:code="['system:dict:*']"
              @click="() => typeModalApi.open()"
            >
              新建类型
            </Button>
          </template>
          <template #actions="{ row }">
            <Button
              size="small"
              type="link"
              v-access:code="['system:dict:*']"
              @click="onSelectDictType(row)"
            >
              字典数据
            </Button>
            <Button
              danger
              size="small"
              type="link"
              v-access:code="['system:dict:*']"
              @click="onDeleteType(row)"
            >
              删除
            </Button>
          </template>
        </TypeGrid>
      </div>
      <div class="flex-1">
        <DataGrid
          :table-title="
            selectedType
              ? `字典数据：${selectedType.name}（${selectedType.type}）`
              : '字典数据：请先点击上方字典类型的「字典数据」'
          "
        >
          <template #toolbar-tools>
            <Button
              class="mr-2"
              type="primary"
              v-access:code="['system:dict:*']"
              @click="() => dataModalApi.open()"
            >
              新建数据
            </Button>
          </template>
          <template #actions="{ row }">
            <Button
              danger
              size="small"
              type="link"
              v-access:code="['system:dict:*']"
              @click="onDeleteData(row)"
            >
              删除
            </Button>
          </template>
        </DataGrid>
      </div>
    </div>
    <TypeModal>
      <TypeForm />
    </TypeModal>
    <DataModal>
      <DataForm />
    </DataModal>
  </Page>
</template>
```

- [ ] **Step 2: 静态验证**

```bash
pnpm --filter @vben/web-antd typecheck
```

Expected: 绿（`useVbenVxeGrid` 泛型 T 默认 `Record<string, any>`，`operation` slot 列不在 `DictData` 里也编译通过——官方 custom-cell demo 同型）。

- [ ] **Step 3: Commit**

```bash
git add apps/web-antd/src/views/system/SystemDictPage.vue
git commit -m "feat(web-antd): system dict page (dual grid: types + data)"
```

---

### Task 20: SystemConfigsPage（参数配置）

**Files:**
- Create: `apps/web-antd/src/views/system/SystemConfigsPage.vue`

**Interfaces:**
- Consumes: T13 `listConfigs/createConfig/deleteConfig/SystemConfig`；seed 菜单 `path=/system/configs`、`component=views/system/SystemConfigsPage`、`permission=system:config:*`。
- Produces: `/system/configs` 页面（列表+创建+删除）；`SystemConfigsPage` 组件名。

- [ ] **Step 1: 写页面文件**

```vue
<!-- apps/web-antd/src/views/system/SystemConfigsPage.vue -->
<script lang="ts" setup>
import type { VxeGridProps } from '#/adapter/vxe-table';

import { Page, useVbenModal } from '@vben/common-ui';

import { Button, message, Modal as AntdModal } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import {
  createConfig,
  deleteConfig,
  listConfigs,
  type SystemConfig,
} from '#/api/admin';
import { localPage } from '#/utils/local-page';

defineOptions({ name: 'SystemConfigsPage' });

const gridOptions: VxeGridProps<SystemConfig> = {
  columns: [
    { field: 'id', title: 'ID', width: 80 },
    { field: 'key', title: '参数键', minWidth: 180 },
    { field: 'value', title: '参数值', minWidth: 200 },
    { field: 'name', title: '名称', minWidth: 140 },
    {
      field: 'operation',
      fixed: 'right',
      slots: { default: 'actions' },
      title: '操作',
      width: 130,
    },
  ],
  height: 'auto',
  keepSource: true,
  pagerConfig: {},
  proxyConfig: {
    ajax: {
      query: async ({ page }) => {
        const rows = await listConfigs();
        return {
          items: localPage(rows, page.currentPage, page.pageSize),
          total: rows.length,
        };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: true, zoom: true },
};

const [Grid, gridApi] = useVbenVxeGrid({ gridOptions });

const [Modal, modalApi] = useVbenModal({
  fullscreenButton: false,
  title: '新建参数',
  onOpenChange(isOpen: boolean) {
    if (isOpen) formApi.resetForm();
  },
  onConfirm: async () => {
    try {
      await formApi.validateAndSubmitForm();
    } catch {
      // 拦截器已 toast，弹窗保持打开
    }
  },
});

const [Form, formApi] = useVbenForm({
  handleSubmit: async (values) => {
    await createConfig(
      values as { key: string; name: string; remark: string; value: string },
    );
    message.success('创建成功');
    modalApi.close();
    gridApi.reload();
  },
  schema: [
    {
      component: 'Input',
      componentProps: { placeholder: '如 site.name' },
      fieldName: 'key',
      label: '参数键',
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '如 alexGo-cloud' },
      fieldName: 'value',
      label: '参数值',
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '如 站点名称' },
      fieldName: 'name',
      label: '名称',
      rules: 'required',
    },
    {
      component: 'Textarea',
      componentProps: { placeholder: '备注（可空）' },
      defaultValue: '',
      fieldName: 'remark',
      label: '备注',
    },
  ],
  showDefaultActions: false,
});

function onDelete(row: SystemConfig) {
  AntdModal.confirm({
    title: `确认删除参数「${row.key}」？`,
    onOk: async () => {
      try {
        await deleteConfig(row.id);
      } catch {
        return; // 拦截器已 toast
      }
      message.success('删除成功');
      gridApi.reload();
    },
  });
}
</script>

<template>
  <Page auto-content-height>
    <Grid>
      <template #toolbar-tools>
        <Button
          class="mr-2"
          type="primary"
          v-access:code="['system:config:*']"
          @click="() => modalApi.open()"
        >
          新建参数
        </Button>
      </template>
      <template #actions="{ row }">
        <Button
          danger
          size="small"
          type="link"
          v-access:code="['system:config:*']"
          @click="onDelete(row)"
        >
          删除
        </Button>
      </template>
    </Grid>
    <Modal>
      <Form />
    </Modal>
  </Page>
</template>
```

- [ ] **Step 2: 静态验证**

```bash
pnpm --filter @vben/web-antd typecheck
```

Expected: 绿。

- [ ] **Step 3: Commit**

```bash
git add apps/web-antd/src/views/system/SystemConfigsPage.vue
git commit -m "feat(web-antd): system configs page (list/create/delete)"
```

---

### Task 21: SystemNoticesPage（通知公告）

**Files:**
- Create: `apps/web-antd/src/views/system/SystemNoticesPage.vue`

**Interfaces:**
- Consumes: T13 `listNotices/createNotice/deleteNotice/Notice`；seed 菜单 `path=/system/notices`、`component=views/system/SystemNoticesPage`、`permission=system:notice:*`。
- Produces: `/system/notices` 页面（列表+创建+删除）；`SystemNoticesPage` 组件名。

- [ ] **Step 1: 写页面文件**

```vue
<!-- apps/web-antd/src/views/system/SystemNoticesPage.vue -->
<script lang="ts" setup>
import type { VxeGridProps } from '#/adapter/vxe-table';

import { Page, useVbenModal } from '@vben/common-ui';

import { Button, message, Modal as AntdModal } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import {
  createNotice,
  deleteNotice,
  listNotices,
  type Notice,
} from '#/api/admin';
import { formatDateTime } from '#/utils/format';
import { localPage } from '#/utils/local-page';

defineOptions({ name: 'SystemNoticesPage' });

const gridOptions: VxeGridProps<Notice> = {
  columns: [
    { field: 'id', title: 'ID', width: 80 },
    { field: 'title', title: '标题', minWidth: 220 },
    {
      field: 'status',
      formatter: ({ cellValue }) => (Number(cellValue) === 1 ? '启用' : '停用'),
      title: '状态',
      width: 80,
    },
    {
      field: 'created_at',
      formatter: ({ cellValue }) => formatDateTime(cellValue),
      title: '创建时间',
      width: 180,
    },
    {
      field: 'operation',
      fixed: 'right',
      slots: { default: 'actions' },
      title: '操作',
      width: 130,
    },
  ],
  height: 'auto',
  keepSource: true,
  pagerConfig: {},
  proxyConfig: {
    ajax: {
      query: async ({ page }) => {
        const rows = await listNotices();
        return {
          items: localPage(rows, page.currentPage, page.pageSize),
          total: rows.length,
        };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: true, zoom: true },
};

const [Grid, gridApi] = useVbenVxeGrid({ gridOptions });

const [Modal, modalApi] = useVbenModal({
  fullscreenButton: false,
  title: '新建公告',
  onOpenChange(isOpen: boolean) {
    if (isOpen) formApi.resetForm();
  },
  onConfirm: async () => {
    try {
      await formApi.validateAndSubmitForm();
    } catch {
      // 拦截器已 toast，弹窗保持打开
    }
  },
});

const [Form, formApi] = useVbenForm({
  handleSubmit: async (values) => {
    await createNotice(
      values as { content: string; status: number; title: string },
    );
    message.success('创建成功');
    modalApi.close();
    gridApi.reload();
  },
  schema: [
    {
      component: 'Input',
      componentProps: { placeholder: '请输入公告标题' },
      fieldName: 'title',
      label: '标题',
      rules: 'required',
    },
    {
      component: 'Textarea',
      componentProps: { placeholder: '请输入公告内容', rows: 4 },
      fieldName: 'content',
      label: '内容',
    },
    {
      component: 'RadioGroup',
      componentProps: {
        options: [
          { label: '启用', value: 1 },
          { label: '停用', value: 0 },
        ],
      },
      defaultValue: 1,
      fieldName: 'status',
      label: '状态',
      rules: 'required',
    },
  ],
  showDefaultActions: false,
});

function onDelete(row: Notice) {
  AntdModal.confirm({
    title: `确认删除公告「${row.title}」？`,
    onOk: async () => {
      try {
        await deleteNotice(row.id);
      } catch {
        return; // 拦截器已 toast
      }
      message.success('删除成功');
      gridApi.reload();
    },
  });
}
</script>

<template>
  <Page auto-content-height>
    <Grid>
      <template #toolbar-tools>
        <Button
          class="mr-2"
          type="primary"
          v-access:code="['system:notice:*']"
          @click="() => modalApi.open()"
        >
          新建公告
        </Button>
      </template>
      <template #actions="{ row }">
        <Button
          danger
          size="small"
          type="link"
          v-access:code="['system:notice:*']"
          @click="onDelete(row)"
        >
          删除
        </Button>
      </template>
    </Grid>
    <Modal>
      <Form />
    </Modal>
  </Page>
</template>
```

- [ ] **Step 2: 静态验证**

```bash
pnpm --filter @vben/web-antd typecheck
```

Expected: 绿。

- [ ] **Step 3: Commit**

```bash
git add apps/web-antd/src/views/system/SystemNoticesPage.vue
git commit -m "feat(web-antd): system notices page (list/create/delete)"
```

---

### Task 22: SystemLoginLogsPage（登录日志，只读）

**Files:**
- Create: `apps/web-antd/src/views/system/SystemLoginLogsPage.vue`

**Interfaces:**
- Consumes: T13 `listLoginLogs(limit?: number)`（默认 100，query `?limit=`）与 `LoginLog`；seed 菜单 `path=/system/logins`、`component=views/system/SystemLoginLogsPage`、`permission=system:log:login`（该权限码无 `:*` 后缀——v-access 必须用原串精确匹配）。
- Produces: `/system/logins` 页面（只读列表，无操作列、无弹窗）；`SystemLoginLogsPage` 组件名。

- [ ] **Step 1: 写页面文件**

```vue
<!-- apps/web-antd/src/views/system/SystemLoginLogsPage.vue -->
<script lang="ts" setup>
import type { VxeGridProps } from '#/adapter/vxe-table';

import { Page } from '@vben/common-ui';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { listLoginLogs, type LoginLog } from '#/api/admin';
import { formatDateTime } from '#/utils/format';
import { localPage } from '#/utils/local-page';

defineOptions({ name: 'SystemLoginLogsPage' });

const gridOptions: VxeGridProps<LoginLog> = {
  columns: [
    { field: 'id', title: 'ID', width: 80 },
    { field: 'tenant_id', title: '租户', width: 80 },
    { field: 'username', title: '用户名', minWidth: 120 },
    { field: 'user_id', title: '用户ID', width: 90 },
    { field: 'ip', title: 'IP', minWidth: 130 },
    {
      field: 'success',
      formatter: ({ cellValue }) => (Number(cellValue) === 1 ? '成功' : '失败'),
      title: '结果',
      width: 80,
    },
    { field: 'message', title: '信息', minWidth: 160 },
    {
      field: 'created_at',
      formatter: ({ cellValue }) => formatDateTime(cellValue),
      title: '时间',
      width: 180,
    },
  ],
  height: 'auto',
  keepSource: true,
  pagerConfig: {},
  proxyConfig: {
    ajax: {
      query: async ({ page }) => {
        const rows = await listLoginLogs();
        return {
          items: localPage(rows, page.currentPage, page.pageSize),
          total: rows.length,
        };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: true, zoom: true },
};

const [Grid] = useVbenVxeGrid({ gridOptions });
</script>

<template>
  <Page auto-content-height>
    <Grid />
  </Page>
</template>
```

- [ ] **Step 2: 静态验证**

```bash
pnpm --filter @vben/web-antd typecheck
```

Expected: 绿。

- [ ] **Step 3: Commit**

```bash
git add apps/web-antd/src/views/system/SystemLoginLogsPage.vue
git commit -m "feat(web-antd): system login logs page (read-only)"
```

---

### Task 23: SystemOperateLogsPage（操作日志，只读）

**Files:**
- Create: `apps/web-antd/src/views/system/SystemOperateLogsPage.vue`

**Interfaces:**
- Consumes: T13 `listOperateLogs(limit?: number)` 与 `OperateLog`；seed 菜单 `path=/system/operates`、`component=views/system/SystemOperateLogsPage`、`permission=system:log:operate`（精确原串，无 `:*`）。
- Produces: `/system/operates` 页面（只读列表）；`SystemOperateLogsPage` 组件名。

- [ ] **Step 1: 写页面文件**

```vue
<!-- apps/web-antd/src/views/system/SystemOperateLogsPage.vue -->
<script lang="ts" setup>
import type { VxeGridProps } from '#/adapter/vxe-table';

import { Page } from '@vben/common-ui';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { listOperateLogs, type OperateLog } from '#/api/admin';
import { formatDateTime } from '#/utils/format';
import { localPage } from '#/utils/local-page';

defineOptions({ name: 'SystemOperateLogsPage' });

const gridOptions: VxeGridProps<OperateLog> = {
  columns: [
    { field: 'id', title: 'ID', width: 80 },
    { field: 'tenant_id', title: '租户', width: 80 },
    { field: 'username', title: '用户名', minWidth: 110 },
    { field: 'method', title: '方法', width: 80 },
    { field: 'path', title: '路径', minWidth: 200 },
    { field: 'status', title: 'HTTP状态', width: 100 },
    { field: 'latency_ms', title: '耗时(ms)', width: 100 },
    { field: 'error', title: '错误', minWidth: 150 },
    {
      field: 'created_at',
      formatter: ({ cellValue }) => formatDateTime(cellValue),
      title: '时间',
      width: 180,
    },
  ],
  height: 'auto',
  keepSource: true,
  pagerConfig: {},
  proxyConfig: {
    ajax: {
      query: async ({ page }) => {
        const rows = await listOperateLogs();
        return {
          items: localPage(rows, page.currentPage, page.pageSize),
          total: rows.length,
        };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: true, zoom: true },
};

const [Grid] = useVbenVxeGrid({ gridOptions });
</script>

<template>
  <Page auto-content-height>
    <Grid />
  </Page>
</template>
```

- [ ] **Step 2: 静态验证**

```bash
pnpm --filter @vben/web-antd typecheck
```

Expected: 绿。

- [ ] **Step 3: Commit**

```bash
git add apps/web-antd/src/views/system/SystemOperateLogsPage.vue
git commit -m "feat(web-antd): system operate logs page (read-only)"
```

---

### Task 24: OrderOrdersPage（订单管理）——阶段 D 收口：全页面巡检冒烟

**Files:**
- Create: `apps/web-antd/src/views/order/OrderOrdersPage.vue`
- Test: 浏览器冒烟（Step 4，Playwright MCP）

**Interfaces:**
- Consumes: T13 `listOrders/createOrder/Order`；seed 菜单 `path=/order/orders`、`component=views/order/OrderOrdersPage`、`permission=order:order:*`（T7 迁移新增行）；后端订单路由需 `order.enabled` 开关（T8：本地 `config.yaml` 的 `order.enabled: true`——**只改本地配置，不 commit config.yaml**）。
- Produces: `/order/orders` 页面（列表+创建，无删除）；`OrderOrdersPage` 组件名；阶段 D 收口证据（Step 4：11 页逐页巡检）。

- [ ] **Step 1: 写页面文件**

```vue
<!-- apps/web-antd/src/views/order/OrderOrdersPage.vue -->
<script lang="ts" setup>
import type { VxeGridProps } from '#/adapter/vxe-table';

import { Page, useVbenModal } from '@vben/common-ui';

import { Button, message } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { createOrder, listOrders, type Order } from '#/api/admin';
import { formatDateTime } from '#/utils/format';
import { localPage } from '#/utils/local-page';

defineOptions({ name: 'OrderOrdersPage' });

const gridOptions: VxeGridProps<Order> = {
  columns: [
    { field: 'id', title: 'ID', width: 80 },
    { field: 'user_id', title: '用户ID', width: 90 },
    { field: 'product_id', title: '商品ID', width: 90 },
    { field: 'amount', title: '金额', width: 110 },
    {
      field: 'status',
      formatter: ({ cellValue }) => String(cellValue ?? ''),
      title: '状态',
      width: 100,
    },
    { field: 'order_no', title: '订单号', minWidth: 180 },
    {
      field: 'created_at',
      formatter: ({ cellValue }) => formatDateTime(cellValue),
      title: '创建时间',
      width: 180,
    },
    {
      field: 'updated_at',
      formatter: ({ cellValue }) => formatDateTime(cellValue),
      title: '更新时间',
      width: 180,
    },
  ],
  height: 'auto',
  keepSource: true,
  pagerConfig: {},
  proxyConfig: {
    ajax: {
      query: async ({ page }) => {
        const rows = await listOrders();
        return {
          items: localPage(rows, page.currentPage, page.pageSize),
          total: rows.length,
        };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: true, zoom: true },
};

const [Grid, gridApi] = useVbenVxeGrid({ gridOptions });

const [Modal, modalApi] = useVbenModal({
  fullscreenButton: false,
  title: '新建订单',
  onOpenChange(isOpen: boolean) {
    if (isOpen) formApi.resetForm();
  },
  onConfirm: async () => {
    try {
      await formApi.validateAndSubmitForm();
    } catch {
      // 拦截器已 toast，弹窗保持打开
    }
  },
});

const [Form, formApi] = useVbenForm({
  handleSubmit: async (values) => {
    await createOrder(
      values as {
        amount: number;
        order_no: string;
        product_id: number;
        status: number;
        user_id: number;
      },
    );
    message.success('创建成功');
    modalApi.close();
    gridApi.reload();
  },
  schema: [
    {
      component: 'InputNumber',
      componentProps: { min: 1 },
      defaultValue: 1,
      fieldName: 'user_id',
      label: '用户ID',
      rules: 'required',
    },
    {
      component: 'InputNumber',
      componentProps: { min: 1 },
      defaultValue: 1,
      fieldName: 'product_id',
      label: '商品ID',
      rules: 'required',
    },
    {
      component: 'InputNumber',
      componentProps: { min: 0, precision: 2 },
      defaultValue: 100,
      fieldName: 'amount',
      label: '金额',
      rules: 'required',
    },
    {
      component: 'InputNumber',
      componentProps: { min: 0 },
      defaultValue: 0,
      fieldName: 'status',
      label: '状态',
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '如 SO20261009001' },
      fieldName: 'order_no',
      label: '订单号',
      rules: 'required',
    },
  ],
  showDefaultActions: false,
});
</script>

<template>
  <Page auto-content-height>
    <Grid>
      <template #toolbar-tools>
        <Button
          class="mr-2"
          type="primary"
          v-access:code="['order:order:*']"
          @click="() => modalApi.open()"
        >
          新建订单
        </Button>
      </template>
    </Grid>
    <Modal>
      <Form />
    </Modal>
  </Page>
</template>
```

> 本页**无操作列、无删除**——旧 admin-web 的订单页也只有列表+创建（后端订单模块未提供 delete）。

- [ ] **Step 2: 静态验证**

```bash
pnpm --filter @vben/web-antd typecheck
pnpm --filter @vben/web-antd build
```

Expected: 双绿。

- [ ] **Step 3: 确认订单开关**

```bash
grep -n "enabled" /Users/alex/Desktop/goWork/alexGo-cloud/alexgo-server/configs/config.yaml
```

Expected: `order.enabled: true`。若为 `false`，本地改为 `true` **且不加入本次 commit**（`git status` 里 config.yaml 必须干净——该文件含真实凭据，任何 diff 都不得提交）。

- [ ] **Step 4: 全页面巡检冒烟（阶段 D 收口）**

启动双端：

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud
make run > /tmp/alexgo-dev-backend.log 2>&1 &
for i in $(seq 1 30); do curl -sf http://localhost:8080/health >/dev/null && break; sleep 1; done
curl -sf http://localhost:8080/health && echo " backend UP"
pnpm --filter @vben/web-antd dev > /tmp/alexgo-dev-web.log 2>&1 &
for i in $(seq 1 30); do curl -sf http://localhost:5666/ >/dev/null && break; sleep 1; done
curl -sf http://localhost:5666/ >/dev/null && echo " frontend UP"
```

Playwright 操作与断言：

1. `browser_navigate` → `http://localhost:5666/` → `admin`/`admin123` 登录 → 落 `/dashboard/analytics`。
2. 逐个点开侧栏菜单，对 **11 个管理页**各断言三件事（顺序：用户→角色→菜单→部门→岗位→字典→参数→公告→登录日志→操作日志→订单）：
   - URL 命中且**关键列头可见**：用户=「用户名」、角色=「编码」、菜单=「路径」、部门=「部门名称」、岗位=「岗位编码」、字典=「字典类型」+「标签」双表、参数=「参数键」、公告=「标题」、登录日志=「IP」、操作日志=「路径」、订单=「订单号」；
   - `browser_network_requests` 中对应 `GET /api/admin/...`（用户 `/admin/system/users`、菜单 `/admin/system/menus`、订单 `/admin/order/orders` 等）**200** 且非空列表（订单页若库中无单，允许 `{data":[]}` 空——见第 3 条）；
   - 页面**无新增** error 级 console 消息。
3. 订单页新建一单（user_id=1、product_id=1、amount=99、status=0、order_no=`SO_SMOKE_001`）→ 断言 toast「创建成功」、`POST /api/admin/order/orders` 200、表格出现 `SO_SMOKE_001` 行。
4. 汇总断言：`browser_console_messages level=error` 全程**无 uncaught JS 异常**；`/api/dashboard/*` 404 资源错误属预期（容忍规则同 T12/T14）。
5. 任一条失败：修页面或权限路由后重跑该页，直至 11 页全绿。
6. 停双端：

```bash
pkill -f "alexgo-server/cmd" || true
pkill -f "web-antd.*vite" || true
lsof -ti:8080 | xargs kill 2>/dev/null || true
lsof -ti:5666 | xargs kill 2>/dev/null || true
```

- [ ] **Step 5: Commit**

```bash
git add apps/web-antd/src/views/order/OrderOrdersPage.vue
git commit -m "feat(web-antd): order orders page (list/create)"
```

**阶段 D 到此完成**：11 个管理页全部落地，每页 typecheck 绿、T14/T24 两轮浏览器冒烟分别钉死信封三形态与全页面可访问性。

---

# 阶段 E：文档同步与总验收（T25-T26）

### Task 25: README / architecture.md 引用改写

**Files:**
- Modify: `README.md:20,64-70,85-86,95`
- Modify: `docs/architecture.md:23,222-226`

**Interfaces:**
- Consumes: T10（`admin-web/` 已删除、vendored monorepo 落在仓库根：`apps/`、`packages/`、`pnpm-workspace.yaml`）；T11/T12（dev 端口 5666、登录接口 `/api/auth/login`、`pnpm --filter @vben/web-antd dev`）。
- Produces: 全仓文档零 `admin-web`/`npm run dev` 残留（T26 Step 7 的 sweep 断言对象）。

- [ ] **Step 1: README.md 四处替换（每处 old 文本唯一，逐个 Edit）**

1. 第 20 行表格行：

```diff
-| 后台前端（`admin-web/`） | Vue 3 + Naive UI + Vite 管理后台，覆盖上述系统管理页面 |
+| 后台前端（`apps/web-antd/`） | vue-vben-admin v5.7.0（Vite + Ant Design Vue + vxe-table）管理后台，覆盖上述系统管理页面 |
```

2. 启动命令块与访问说明（第 64-70 行）：

```diff
-```bash
-cd admin-web
-npm i
-npm run dev
-```
-
-- 访问 <http://localhost:5174>，登录接口为 `/api/app/system/auth/login`
+```bash
+pnpm install
+pnpm --filter @vben/web-antd dev
+```
+
+- 访问 <http://localhost:5666>，登录接口为 `/api/auth/login`
```

3. 第 85-86 行（自动刷新说明 + member 代理联调命令）：

```diff
-admin-web 的自动刷新为后续项——access 过期后需重新登录（app 端可用 `/api/app/member/auth/refresh` 换新）。
-micro 本地联调前端分流：`MEMBER_PROXY=http://localhost:8081 npm run dev`。
+管理后台的自动刷新为后续项——access 过期后需重新登录（vben `enableRefreshToken=false`，app 端可用 `/api/app/member/auth/refresh` 换新）。
+micro 本地联调前端分流：`MEMBER_PROXY=http://localhost:8081 pnpm --filter @vben/web-antd dev`。
```

4. 第 95 行目录树：

```diff
-├── admin-web/         # 管理后台前端（Vue 3 + Naive UI + Vite）
+├── apps/              # 管理后台前端（vendored vue-vben-admin v5.7.0，主用 web-antd）
+├── packages/          # vendored vben 共享包（@vben/*）
```

- [ ] **Step 2: docs/architecture.md 三处替换**

1. 第 23 行目录表：

```diff
-| `admin-web/` | 管理后台前端（Vue 3 + Vite），经 `/api/**` 调用后端 |
+| `apps/web-antd/` | 管理后台前端（vendored vue-vben-admin v5.7.0，Ant Design Vue），经 `/api/**` 调用后端 |
```

2. 第 222-223 行（member 代理说明）：

```diff
-    之前（nginx-ingress 最长前缀匹配）；本地前端用 `MEMBER_PROXY=http://localhost:8081 npm run dev`
-    把 Vite 的 member 代理指到 :8081（`admin-web/vite.config.ts`），mono 时默认全部指 :8080。
+    之前（nginx-ingress 最长前缀匹配）；本地前端用 `MEMBER_PROXY=http://localhost:8081 pnpm --filter @vben/web-antd dev`
+    把 Vite 的 member 代理指到 :8081（`apps/web-antd/vite.config.ts`），mono 时默认全部指 :8080。
```

3. 第 225-226 行（本地运行说明）：

```diff
-  含 MySQL，`DB_DSN` 经 `environment:` 注入，内置 system/member 双容器）；前端 `admin-web/` 独立
-  `npm run dev`。micro 本地双进程用 `make run-system` + `make run-member`。
+  含 MySQL，`DB_DSN` 经 `environment:` 注入，内置 system/member 双容器）；前端 `apps/web-antd/` 独立
+  `pnpm --filter @vben/web-antd dev`。micro 本地双进程用 `make run-system` + `make run-member`。
```

- [ ] **Step 3: 全仓 sweep（不预设清单——spec 风险表"逐处核对"的落地）**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud
grep -rn "admin-web\|npm run dev\|npm i\b" README.md docs/ Makefile .github/ deployments/ scripts/ 2>/dev/null | grep -v "docs/superpowers" || echo "CLEAN"
```

Expected: `CLEAN`（或仅剩与本变更无关的行——逐条人工判断，确属前端引用就一并改掉）。
附带确认 `.github/workflows/ci.yml`、`cd.yml`：**当前无任何前端构建步骤**（grep 无 `npm`/`pnpm`/`web-antd` 命中则记录"CI 未构建前端，无需改"，有则按 `pnpm --filter @vben/web-antd build` 改写）。

- [ ] **Step 4: Commit**

```bash
git add README.md docs/architecture.md
git commit -m "docs: sync README/architecture for vben-admin frontend"
```

---

### Task 26: 总验收（spec §10 清单全量 + fresh 库全链路）

**Files:**
- Test: 无新增文件——本任务是纯验收；每步失败则就地修复并单独 commit（`fix(...)`），修完重跑该步。

**Interfaces:**
- Consumes: 阶段 A-E 全部产物；dev 库 81.71.152.135:3399（库/用户 `alexgo-cloud`，密码见 `alexgo-server/configs/config.yaml` 的 `database.dsn`——该凭据已在仓库内，命令直接引用）；本机 `mysql` 客户端（`/Users/alex/Library/PhpWebStudy/env/mysql/bin/mysql`，PATH 里通常已有 `mysql`）。
- Produces: spec §10 七项全过的验收结论；fresh 库 seed 门控（T8）+ 迁移（T7）的终态证据（19 行菜单）。

- [ ] **Step 1: 后端测试双绿**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud
make test
go test ./alexgo-server/cmd
```

Expected: 双 PASS。macOS 若 `go test` 链接报错，加 `CGO_ENABLED=0` 重跑（README 既有提示）。

- [ ] **Step 2: dev 库迁移——幂等 up + down 还原 + 再 up（spec §10 第 2 项）**

第一遍 up（后台跑 + 日志判定 + 杀进程——`make migrate` 完成后挂起属预期）：

```bash
make migrate > /tmp/t26-up1.log 2>&1 &
for i in $(seq 1 60); do
  grep -q "seed default admin done" /tmp/t26-up1.log 2>/dev/null && break
  sleep 2
done
grep -E "migrate success|seed default admin done" /tmp/t26-up1.log
grep -iE "error|dirty" /tmp/t26-up1.log || echo "no errors"
pkill -f -- "--migrate-only" || true
```

Expected: `migrate success` + `seed default admin done`，无 error/dirty。

数据断言（dev 库终态 19 行）：

```bash
mysql -h 81.71.152.135 -P 3399 -u alexgo-cloud -pcaMYe2mB3jpEerbY alexgo-cloud \
  -e "SELECT COUNT(*) AS total FROM menus WHERE tenant_id=0 AND deleted=0;"
```

Expected: `total` = 19。

down 还原（runner 只跑 `m.Up()`，down 直接用 mysql 客户端执行 down SQL）：

```bash
mysql -h 81.71.152.135 -P 3399 -u alexgo-cloud -pcaMYe2mB3jpEerbY alexgo-cloud \
  < modules/system/migrations/20261009000001_menus_vben_backfill.down.sql
mysql -h 81.71.152.135 -P 3399 -u alexgo-cloud -pcaMYe2mB3jpEerbY alexgo-cloud \
  -e "SELECT COUNT(*) AS total FROM menus WHERE tenant_id=0 AND deleted=0;"
```

Expected: `total` = 14（down 可还原）。

再 up（还原迁移）：

```bash
make migrate > /tmp/t26-up2.log 2>&1 &
for i in $(seq 1 60); do
  grep -q "seed default admin done" /tmp/t26-up2.log 2>/dev/null && break
  sleep 2
done
grep -E "migrate success|seed default admin done" /tmp/t26-up2.log
pkill -f -- "--migrate-only" || true
mysql -h 81.71.152.135 -P 3399 -u alexgo-cloud -pcaMYe2mB3jpEerbY alexgo-cloud \
  -e "SELECT COUNT(*) AS total FROM menus WHERE tenant_id=0 AND deleted=0;"
```

Expected: `total` = 19（up→down→up 闭环）。

- [ ] **Step 3: curl 五端点 + 门控 + 旧接口回归（spec §4/§5 + §10 第 3、6 项）**

起后端：

```bash
make run > /tmp/t26-backend.log 2>&1 &
for i in $(seq 1 30); do curl -sf http://localhost:8080/health >/dev/null && break; sleep 1; done
curl -sf http://localhost:8080/health && echo " backend UP"
```

拿到 admin token 并逐项断言：

```bash
# ① 登录（信封形态① {code:0,data:{...}}；vben 契约字段 accessToken）
curl -s -o /tmp/t26-login.json -w "%{http_code}\n" -X POST http://localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' -d '{"username":"admin","password":"wrong-pass"}'
# Expected: 401
curl -s -X POST http://localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}' > /tmp/t26-login.json
grep -o '"code":0' /tmp/t26-login.json && grep -o '"accessToken":"[^"]*"' /tmp/t26-login.json
ATOKEN=$(grep -o '"accessToken":"[^"]*"' /tmp/t26-login.json | head -1 | cut -d'"' -f4)
# Expected: 两者都命中；ATOKEN 非空

# ② codes（admin token → string[]；RF4：member:user:manage 必须在——member 按钮行未被误伤的间接证据）
curl -s http://localhost:8080/api/auth/codes -H "Authorization: Bearer $ATOKEN" > /tmp/t26-codes.json
grep -o '"system:user:list"' /tmp/t26-codes.json && grep -o '"member:user:manage"' /tmp/t26-codes.json && grep -o '"order:order:\*"' /tmp/t26-codes.json
# Expected: 三者都命中；无 token → 401：
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/api/auth/codes
# Expected: 401

# ③ user/info（RF5：userId 必须是字符串）
curl -s http://localhost:8080/api/user/info -H "Authorization: Bearer $ATOKEN" | grep -o '"userId":"[^"]*"'
# Expected: "userId":"<数字>"（带引号 = 字符串；若 grep 不中说明返回了数字型，测试失败）

# ④ menu/all（≥16 节点、目录 component 为空串、11 处 lucide 图标在位）
curl -s http://localhost:8080/api/menu/all -H "Authorization: Bearer $ATOKEN" > /tmp/t26-menu.json
grep -o '"component":""' /tmp/t26-menu.json | head -1
for icon in lucide:settings lucide:user lucide:users lucide:list-tree lucide:building-2 lucide:id-card lucide:book-open lucide:settings-2 lucide:bell lucide:history lucide:file-text lucide:layout-dashboard lucide:area-chart lucide:shopping-cart lucide:list; do grep -q "$icon" /tmp/t26-menu.json || echo "MISSING $icon"; done
node -e "const a=JSON.parse(require('fs').readFileSync('/tmp/t26-menu.json','utf8')); const arr=Array.isArray(a.data)?a.data:[]; if(arr.length<16){console.error('FAIL len='+arr.length);process.exit(1)}; console.log('menu/all len='+arr.length)"
# Expected: 无 MISSING；menu/all len ≥16；"component":"" 命中（目录节点）

# ⑤ logout 恒 200（vben 契约：即使未带 token 也 200）
curl -s -o /dev/null -w "%{http_code}\n" -X POST http://localhost:8080/api/auth/logout
# Expected: 200

# ⑥ 信封三形态复验（RF1 的 Task 26 复验位——① {code:0} 已由 ① 登录证明，这里补 ② ③）
# 形态② {"data":[...]}：
curl -s http://localhost:8080/api/admin/system/users -H "Authorization: Bearer $ATOKEN" | head -c 9
# Expected: {"data":[（无 code 键）
# 形态③ {"status":"ok"}：建一次性角色再删（create 返 {"data":{...}}，delete 返 {"status":"ok"}）
curl -s -X POST http://localhost:8080/api/admin/system/roles -H "Authorization: Bearer $ATOKEN" \
  -H 'Content-Type: application/json' -d '{"code":"t26_smoke","name":"T26验收角色"}' > /tmp/t26-role.json
grep -o '"data":{' /tmp/t26-role.json
RID=$(grep -o '"id":[0-9]*' /tmp/t26-role.json | head -1 | cut -d: -f2)
curl -s -X DELETE "http://localhost:8080/api/admin/system/roles/$RID" -H "Authorization: Bearer $ATOKEN"
# Expected: create 命中 "data":{；delete 输出恰为 {"status":"ok"}（无 code、无 data 键）
# 验收残数据 t26_smoke 已随 delete 清掉
```

门控（自读档）与 member 403：

```bash
# 自读端点无 token → 401（失败体 {"error":"unauthorized"}）
curl -s -w "\n%{http_code}\n" http://localhost:8080/api/user/info
# Expected: {"error":"unauthorized"} + 401

# member token 打自读 → 403（spec §8 手工清单第 6 项）
curl -s -X POST http://localhost:8080/api/app/member/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"mobile":"13900000001","password":"member123","nickname":"验收会员"}' > /dev/null
curl -s -X POST http://localhost:8080/api/app/member/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"mobile":"13900000001","password":"member123"}' > /tmp/t26-member.json
MTOKEN=$(grep -o '"token":"[^"]*"' /tmp/t26-member.json | head -1 | cut -d'"' -f4)
curl -s -w "\n%{http_code}\n" http://localhost:8080/api/user/info -H "Authorization: Bearer $MTOKEN"
# Expected: {"error":"forbidden"} + 403
```

> 若 register 返回 400（该手机号已存在），直接用该手机号 login 即可；member 账号是验收残留数据，留在 dev 库无害。

旧接口回归（spec §10 第 6 项——行为不变）：

```bash
# 旧登录（形状 {token,refresh_token,expires_in}，无 code 包装）
curl -s -X POST http://localhost:8080/api/app/system/auth/login \
  -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}' > /tmp/t26-oldlogin.json
grep -o '"token":"[^"]*"' /tmp/t26-oldlogin.json
# 旧 profile
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/api/admin/system/auth/profile -H "Authorization: Bearer $ATOKEN"
# Expected: 旧登录命中 "token":"..."；profile 200
```

- [ ] **Step 4: 前端构建（spec §10 第 4 项，路径已是 pnpm workspace 根）**

```bash
pnpm install
pnpm --filter @vben/web-antd typecheck
pnpm --filter @vben/web-antd build
```

Expected: 全绿；产物在 `apps/web-antd/dist`。

- [ ] **Step 5: 手工验收清单 7 项（spec §8——浏览器用 Playwright MCP）**

保持 Step 3 的后端运行，另起前端：

```bash
pnpm --filter @vben/web-antd dev > /tmp/t26-web.log 2>&1 &
for i in $(seq 1 30); do curl -sf http://localhost:5666/ >/dev/null && break; sleep 1; done
curl -sf http://localhost:5666/ >/dev/null && echo " frontend UP"
```

1. **登录**：输错密码 → 页面弹红 toast（拦截器 `message.error`，文案取响应 `error` 字段）；输对 → 落 `/dashboard/analytics`，侧栏渲染。
2. **侧栏**：三组菜单 **工作台 / 订单 / 系统管理**，顺序正确（工作台→订单→系统管理，meta.order 生效——T6）；每项图标**渲染非空**（不是空白占位；11 处 `lucide:*` + dashboard/order 图标）。
3. **11 页 CRUD 冒烟**：T24 已全量巡检过；此处快速抽查 3 页复验——用户页新建一单行数据、字典页选类型→新建数据、订单页新建一单，均列表即时刷新。
4. **v-access 按钮权限**：admin 角色拥有全部权限码 → 11 页工具栏「新建」按钮全部可见。交叉断言：Step 3 的 codes 响应含 `system:user:list`，用户页新建按钮（`v-access:code="['system:user:list']"`）同时可见——directive 与后端码源一致。
5. **登出后旧 token 立即失效**：先记录 `$ATOKEN`（Step 3 已导出，若已换 shell 重新登录取一个），浏览器右上角**登出**（触发 `POST /api/auth/logout`，断言 network 200 且带 `Authorization` 头——T11 改造点），然后：

```bash
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/api/user/info -H "Authorization: Bearer $ATOKEN"
# Expected: 401（真撤销，不是前端清了 localStorage 而已）
```

6. **member token 打自读 403**：Step 3 已用 curl 覆盖，记录结论即可（不重复执行）。
7. **micro 形态 `MEMBER_PROXY` 联调（可选）**：`make run-system` + `make run-member` 起双进程，另起 `MEMBER_PROXY=http://localhost:8081 pnpm --filter @vben/web-antd dev`，会员管理页列表可加载则过；本机不做双进程联调时在验收结论标注"第 7 项可选、未执行"。

控制台总断言：全程**无 uncaught JS 异常**；`/api/dashboard/*` 404 资源错误属预期（容忍规则同 T12/T14/T24）。

- [ ] **Step 6: fresh 库全链路（阶段 B 硬约束承诺的收口——seed 门控不再被迁移毒化）**

```bash
mysql -h 81.71.152.135 -P 3399 -u alexgo-cloud -pcaMYe2mB3jpEerbY \
  -e "CREATE DATABASE \`alexgo_fresh_t26\` DEFAULT CHARSET utf8mb4;"

FRESH_DSN='alexgo-cloud:caMYe2mB3jpEerbY@tcp(81.71.152.135:3399)/alexgo_fresh_t26?charset=utf8mb4&parseTime=True&loc=Local'
DB_DSN="$FRESH_DSN" make run > /tmp/t26-fresh1.log 2>&1 &
for i in $(seq 1 60); do
  grep -q "seed default admin done" /tmp/t26-fresh1.log 2>/dev/null && break
  sleep 2
done
grep -E "migrate success|seed default admin done" /tmp/t26-fresh1.log
grep -iE "error|dirty" /tmp/t26-fresh1.log || echo "no errors"
for i in $(seq 1 30); do curl -sf http://localhost:8080/health >/dev/null && break; sleep 1; done

# fresh 库全链路：登录 → 菜单 → 权限码
curl -s -X POST http://localhost:8080/api/auth/login -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' > /tmp/t26-fresh-login.json
FTOKEN=$(grep -o '"accessToken":"[^"]*"' /tmp/t26-fresh-login.json | head -1 | cut -d'"' -f4)
curl -s http://localhost:8080/api/menu/all -H "Authorization: Bearer $FTOKEN" > /tmp/t26-fresh-menu.json
node -e "const a=JSON.parse(require('fs').readFileSync('/tmp/t26-fresh-menu.json','utf8')); const arr=Array.isArray(a.data)?a.data:[]; if(arr.length<16){console.error('FAIL len='+arr.length);process.exit(1)}; console.log('fresh menu/all len='+arr.length)"
curl -s http://localhost:8080/api/auth/codes -H "Authorization: Bearer $FTOKEN" | grep -o '"order:order:\*"' || echo "MISSING order code"
# order API 可用（T8 开关 + T5 授权在 fresh 库的端到端证据）
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/api/admin/order/orders -H "Authorization: Bearer $FTOKEN"
# Expected: 200

pkill -f "alexgo-server/cmd" || true
lsof -ti:8080 | xargs kill 2>/dev/null || true

# 第二启：迁移 ErrNoChange + 门控跳过（不重复插行）
DB_DSN="$FRESH_DSN" make run > /tmp/t26-fresh2.log 2>&1 &
for i in $(seq 1 60); do
  grep -q "seed default admin done" /tmp/t26-fresh2.log 2>/dev/null && break
  sleep 2
done
grep -iE "error|dirty" /tmp/t26-fresh2.log || echo "no errors"
pkill -f "alexgo-server/cmd" || true
lsof -ti:8080 | xargs kill 2>/dev/null || true

mysql -h 81.71.152.135 -P 3399 -u alexgo-cloud -pcaMYe2mB3jpEerbY alexgo_fresh_t26 \
  -e "SELECT COUNT(*) AS total FROM menus WHERE tenant_id=0 AND deleted=0;"
mysql -h 81.71.152.135 -P 3399 -u alexgo-cloud -pcaMYe2mB3jpEerbY \
  -e "DROP DATABASE alexgo_fresh_t26;"
```

Expected: 第一启 `migrate success` + `seed default admin done` 无 error；fresh `menu/all` **len ≥ 16**；codes 含 `order:order:*`；order API **200**；第二启无 error；**total = 19**（迁移 5 + seed 建 11 + seed 按钮 3——Global Constraints"菜单终态 19 行"的 fresh 路径达成）；`DROP DATABASE` 成功（验收库不留残）。

- [ ] **Step 7: 文档引用收口（spec §10 第 7 项）**

```bash
grep -rn "admin-web\|npm run dev" README.md docs/ Makefile .github/ deployments/ scripts/ 2>/dev/null | grep -v "docs/superpowers" || echo "CLEAN"
```

Expected: `CLEAN`。有残留 → 回 Task 25 就地补改并 commit。

- [ ] **Step 8: 停双端 + 汇总验收结论**

```bash
pkill -f "alexgo-server/cmd" || true
pkill -f "web-antd.*vite" || true
lsof -ti:8080 | xargs kill 2>/dev/null || true
lsof -ti:5666 | xargs kill 2>/dev/null || true
git status --short   # 必须干净：config.yaml（含真实 DSN）任何情况下不得入本任务 diff
```

把 Step 1-7 的结论按 spec §10 七项逐条记为 PASS/FAIL（FAIL 则修复后重跑对应 Step），最终回答：**七项全过 + 手工清单 7 项全过（第 7 项可选可标注未执行）**。
有修复改动时逐项 commit（`git add <file> && git commit -m "fix: ..."`）；无改动则本任务无 commit。

**阶段 E 到此完成，计划全部 26 任务就绪。**
