# HTTP 基座规范化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 收敛统一响应信封（`{code,msg,data}`）、五位业务错误码、统一分页、`DateTime` 时间参数约定、结构化访问日志与 zap Recovery，存量 ~190 处 `ctx.JSON` 全迁。

**Architecture:** 显式 helper 方案——`pkg/resp` 三个函数承载所有出站响应；`pkg/errors` 扩展五位码注册表；`pkg/repo.Paginate[T]` 收敛分页 SQL；`pkg/middleware` 新增 AccessLog/Recovery 替换 gin 默认。依赖单向：types → errors → resp → middleware。前端拦截器双信封窗口期内不打断，全迁后收紧 `code === 0`。

**Tech Stack:** Go 1.x / gin v1.12 / gorm v1.31 / zap v1.27；前端 vben-admin（vue3 + axios interceptor，`apps/web-antd`）。

**Spec:** `docs/superpowers/specs/2026-10-15-http-foundation-design.md`

## Global Constraints

- 直接落 main，不建功能分支，不 push；每个 Task 单独提交
- 每个 Task 完成即跑：`CGO_ENABLED=0 go test ./...` 全绿；涉及前端的 Task 加 `pnpm run typecheck`
- 信封格式（不可变）：成功 `{code:0,msg:"ok",data:...}`；分页 `data:{list,total,page,size}`；失败 `{code:<五位>,msg:<注册表文案>,data:null}`
- HTTP status 映射：1xxxx→400；2xxxx→401（仅 20003→403）；3xxxx→400；5xxxx→500
- **豁免清单（严禁套信封）**：`modules/system/oauth2/controller/protocol.go` 全部端点、`/health*`、`/metrics`、`/debug/pprof/**`
- `PageReq` 调用约定：controller 绑定后必须先调 `req.PageReq.Defaults()` 再传 service/repo/`resp.Page`
- `Paginate[T]` 不注入默认 Order；Where/Order 由调用方链好
- 前端 `successCode` 双兼容（`code === undefined || code === 0`）**只允许在 Task 11 收紧**，之前任何 Task 禁止改动

## Review Focus

（最可能坑人的五个输入/条件，每条由所属 Task 的测试钉住）

1. **迁移窗口内提前收紧前端 `successCode`** → 会让所有未迁完的旧信封接口全挂。钉在 Task 11 步骤 1：收紧前先全仓 grep 确认无 `gin.H{"data"` / `gin.H{"status"` 残留。
2. **`PageReq` 零值（缺省 page/size）产生 `Offset(负数)`/`Limit(0)`** → 钉在 Task 3 测试：零值绑定后 Defaults 必须得 page=1,size=10。
3. **AccessLog 放链首导致 user_id 恒空**（claims 在 Auth 中间件才写入）→ 钉在 Task 5 测试：链序含 fake auth（c.Set("claims")）时日志字段 user_id 非空。
4. **oauth2 protocol 端点被误迁**（RFC 6749 响应体，SSO 依赖）→ 钉在 Task 8/10：`protocol_test.go`、`e2e_test.go` 必须原样绿，且步骤含 `git diff` 断言该文件未动。
5. **service 层裸 `fmt.Errorf` 透传被 `resp.Fail` 兜成 50000 但 msg 外泄原始错误** → 钉在 Task 3 测试：非 `*BizError` 的 error，响应 msg 必须是注册表 50000 文案，不含 cause 字符串。

---

### Task 1: pkg/types — DateTime 宽松解析

**Files:**
- Create: `pkg/types/time.go`
- Test: `pkg/types/time_test.go`

**Interfaces:**
- Consumes: 无
- Produces: `type DateTime struct{ time.Time }`；`UnmarshalJSON` / `UnmarshalParam` / `MarshalJSON`；解析顺序 `2006-01-02 15:04:05` → `2006-01-02` → RFC3339；输出固定 `2006-01-02 15:04:05`（本地时区）

- [ ] **Step 1: 写失败测试**

```go
package types

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDateTimeUnmarshalJSON(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
		want    string // "2006-01-02 15:04:05"
	}{
		{`"2026-01-02 15:04:05"`, false, "2026-01-02 15:04:05"},
		{`"2026-01-02"`, false, "2026-01-02 00:00:00"},
		{`"2026-01-02T15:04:05+08:00"`, false, "2026-01-02 15:04:05"},
		{`""`, true, ""},
		{`"not-a-time"`, true, ""},
		{`12345`, true, ""},
	}
	for _, c := range cases {
		var d DateTime
		err := json.Unmarshal([]byte(c.in), &d)
		if c.wantErr {
			if err == nil {
				t.Fatalf("input %s: want err, got %v", c.in, d)
			}
			continue
		}
		if err != nil {
			t.Fatalf("input %s: %v", c.in, err)
		}
		if got := d.Format("2006-01-02 15:04:05"); got != c.want {
			t.Errorf("input %s: got %s, want %s", c.in, got, c.want)
		}
	}
}

func TestDateTimeMarshalJSON(t *testing.T) {
	d := DateTime{Time: time.Date(2026, 1, 2, 15, 4, 5, 0, time.Local)}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `"2026-01-02 15:04:05"` {
		t.Fatalf("got %s", b)
	}
}

func TestDateTimeUnmarshalParam(t *testing.T) {
	var d DateTime
	if err := d.UnmarshalParam("2026-01-02 15:04:05"); err != nil {
		t.Fatal(err)
	}
	if err := d.UnmarshalParam(""); err == nil {
		t.Fatal("empty string should error")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./pkg/types/ -run TestDateTime -v`
Expected: FAIL（包不存在）

- [ ] **Step 3: 最小实现**

```go
// Package types 提供跨模块共享的 API 值类型。
package types

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// DateTime 是 API 层时间参数/响应值：JSON 与 query 一律 "2006-01-02 15:04:05"
// （本地时区，与 MySQL parseTime=True&loc=Local 一致）。
// Unmarshal 宽松：兼容 "2006-01-02" 与 RFC3339。
type DateTime struct {
	time.Time
}

var timeLayouts = []string{
	"2006-01-02 15:04:05",
	"2006-01-02",
	time.RFC3339,
}

func parseLoose(s string) (time.Time, error) {
	for _, layout := range timeLayouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid datetime %q", s)
}

func (d *DateTime) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("empty datetime")
	}
	t, err := parseLoose(s)
	if err != nil {
		return err
	}
	d.Time = t
	return nil
}

func (d *DateTime) UnmarshalParam(s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("empty datetime")
	}
	t, err := parseLoose(s)
	if err != nil {
		return err
	}
	d.Time = t
	return nil
}

func (d DateTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Format("2006-01-02 15:04:05"))
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./pkg/types/ -run TestDateTime -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add pkg/types/time.go pkg/types/time_test.go
git commit -m "feat(types): DateTime 宽松解析——API 时间参数统一 2006-01-02 15:04:05"
```

---

### Task 2: pkg/errors — 五位错误码注册表

**Files:**
- Modify: `pkg/errors/errors.go`（保留现有 `ErrUnauthorized` sentinel，新增 code 体系）
- Create: `pkg/errors/code.go`
- Test: `pkg/errors/code_test.go`

**Interfaces:**
- Consumes: 无（标准库 only）
- Produces:
  - `type Code int`
  - 常量：`CodeParamInvalid=10001`、`CodeParamMissing=10002`、`CodeUnauthorized=20001`、`CodeTokenExpired=20002`、`CodeForbidden=20003`、`CodeUsernameTaken=30001`、`CodeUserNotFound=30002`、`CodeRoleTaken=30003`、`CodeTenantDisabled=30004`、`CodeCaptchaInvalid=30005`、`CodeInternal=50000`
  - `type BizError struct { Code Code; cause error }`
  - `func New(code Code, cause error) *BizError`
  - `func (e *BizError) Error() string` / `Msg() string` / `Unwrap() error`
  - `func As(err error) (*BizError, bool)`、`func Is(err error, code Code) bool`
  - 语义常量后续迁移中按需补（码号不许与已注册重复；单测强制「导出 Code 全有文案」）

- [ ] **Step 1: 写失败测试**

```go
package errors

import (
	"errors"
	"testing"
)

func TestRegistryNoDuplicateAndAllHaveMsg(t *testing.T) {
	seen := map[Code]string{}
	for code, msg := range registry {
		if prev, ok := seen[code]; ok {
			t.Fatalf("duplicate code %d: %q vs %q", code, prev, msg)
		}
		if msg == "" {
			t.Fatalf("code %d has empty msg", code)
		}
		seen[code] = msg
	}
	// 导出常量必须都在注册表里
	for _, c := range []Code{
		CodeParamInvalid, CodeParamMissing,
		CodeUnauthorized, CodeTokenExpired, CodeForbidden,
		CodeUsernameTaken, CodeUserNotFound, CodeRoleTaken, CodeTenantDisabled, CodeCaptchaInvalid,
		CodeInternal,
	} {
		if _, ok := registry[c]; !ok {
			t.Errorf("exported code %d missing from registry", c)
		}
	}
}

func TestNewAndMsgAndUnwrap(t *testing.T) {
	cause := errors.New("db down")
	e := New(CodeInternal, cause)
	if !errors.Is(e, cause) {
		t.Fatal("Unwrap must expose cause")
	}
	if e.Msg() != registry[CodeInternal] {
		t.Fatalf("Msg = %q", e.Msg())
	}
	if e.Error() == "" {
		t.Fatal("Error() empty")
	}
}

func TestIs(t *testing.T) {
	err := New(CodeUsernameTaken, nil)
	if !Is(err, CodeUsernameTaken) {
		t.Fatal("Is should match code")
	}
	if Is(err, CodeUserNotFound) {
		t.Fatal("Is should not match other code")
	}
	if Is(errors.New("x"), CodeUsernameTaken) {
		t.Fatal("Is on non-BizError must be false")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./pkg/errors/ -v`
Expected: FAIL（Code 未定义）

- [ ] **Step 3: 实现 `code.go`，不动 `errors.go`**

```go
package errors

import "fmt"

// Code 五位业务错误码：1xxxx 参数 / 2xxxx 认证 / 3xxxx 业务 / 5xxxx 系统。
type Code int

const (
	// 1xxxx 参数
	CodeParamInvalid Code = 10001
	CodeParamMissing Code = 10002

	// 2xxxx 认证授权
	CodeUnauthorized Code = 20001
	CodeTokenExpired Code = 20002
	CodeForbidden    Code = 20003

	// 3xxxx 业务规则
	CodeUsernameTaken  Code = 30001
	CodeUserNotFound   Code = 30002
	CodeRoleTaken      Code = 30003
	CodeTenantDisabled Code = 30004
	CodeCaptchaInvalid Code = 30005

	// 5xxxx 系统
	CodeInternal Code = 50000
)

// registry 是错误码文案表（中文、写死代码，不做 i18n）。新码追加到这里 + 顶部常量。
var registry = map[Code]string{
	CodeParamInvalid:   "参数校验失败",
	CodeParamMissing:   "缺少必填参数",
	CodeUnauthorized:   "未登录或登录已失效",
	CodeTokenExpired:   "登录已过期",
	CodeForbidden:      "无权限",
	CodeUsernameTaken:  "用户名已存在",
	CodeUserNotFound:   "用户不存在",
	CodeRoleTaken:      "角色标识已存在",
	CodeTenantDisabled: "租户已停用",
	CodeCaptchaInvalid: "验证码错误",
	CodeInternal:       "系统繁忙，请稍后重试",
}

// BizError 携带业务错误码的错误。cause 可为 nil；cause 只进日志，禁止外泄给客户端。
type BizError struct {
	Code  Code
	cause error
}

func New(code Code, cause error) *BizError {
	return &BizError{Code: code, cause: cause}
}

func (e *BizError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Msg(), e.cause)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Msg())
}

func (e *BizError) Msg() string {
	if m, ok := registry[e.Code]; ok {
		return m
	}
	return registry[CodeInternal]
}

func (e *BizError) Unwrap() error { return e.cause }

// As 从错误链提取 *BizError。
func As(err error) (*BizError, bool) {
	var be *BizError
	if errors.As(err, &be) {
		return be, true
	}
	return nil, false
}

func Is(err error, code Code) bool {
	be, ok := As(err)
	return ok && be.Code == code
}
```

文件头 import：`import "errors"` 与 `import "fmt"` 合并为 `import ("errors"; "fmt")`。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./pkg/errors/ -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add pkg/errors/code.go pkg/errors/code_test.go
git commit -m "feat(errors): 五位业务错误码注册表 + BizError"
```

---

### Task 3: pkg/resp — 统一响应 helper + PageReq

**Files:**
- Create: `pkg/resp/resp.go`
- Create: `pkg/resp/page.go`
- Test: `pkg/resp/resp_test.go`

**Interfaces:**
- Consumes: `pkg/errors`（`Code`/`BizError`/`As`/`New`/`CodeInternal`）
- Produces:
  - `func OK(c *gin.Context, data any)`
  - `func Fail(c *gin.Context, err error)`
  - `func Page[T any](c *gin.Context, list []T, total int64, req PageReq)`
  - `type PageReq struct { Page int `form:"page" binding:"omitempty,min=1"`; Size int `form:"size" binding:"omitempty,min=1,max=100"` }`
  - `func (p *PageReq) Defaults()`（page=1, size=10）
  - `func statusFor(code errs.Code) int`

- [ ] **Step 1: 写失败测试**

```go
package resp

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	errs "alexGo-cloud/pkg/errors"
)

func newCtx() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return c, w
}

type envelope struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

func TestOK(t *testing.T) {
	c, w := newCtx()
	OK(c, gin.H{"id": 1})
	if w.Code != http.StatusOK {
		t.Fatalf("http = %d", w.Code)
	}
	var e envelope
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatal(err)
	}
	if e.Code != 0 || e.Msg != "ok" {
		t.Fatalf("envelope = %+v", e)
	}
}

func TestFailCodeToStatusMapping(t *testing.T) {
	cases := []struct {
		code       errs.Code
		wantStatus int
	}{
		{errs.CodeParamInvalid, 400},
		{errs.CodeUnauthorized, 401},
		{errs.CodeTokenExpired, 401},
		{errs.CodeForbidden, 403},
		{errs.CodeUsernameTaken, 400},
		{errs.CodeInternal, 500},
	}
	for _, tc := range cases {
		if got := statusFor(tc.code); got != tc.wantStatus {
			t.Errorf("statusFor(%d) = %d, want %d", tc.code, got, tc.wantStatus)
		}
	}
}

func TestFailBizError(t *testing.T) {
	c, w := newCtx()
	Fail(c, errs.New(errs.CodeUsernameTaken, nil))
	if w.Code != 400 {
		t.Fatalf("http = %d", w.Code)
	}
	var e envelope
	_ = json.Unmarshal(w.Body.Bytes(), &e)
	if e.Code != int(errs.CodeUsernameTaken) || e.Msg != "用户名已存在" || e.Data != nil {
		t.Fatalf("envelope = %+v", e)
	}
}

func TestFailPlainErrorNoLeak(t *testing.T) {
	c, w := newCtx()
	Fail(c, errors.New("internal secret detail: db password wrong"))
	if w.Code != 500 {
		t.Fatalf("http = %d", w.Code)
	}
	var e envelope
	_ = json.Unmarshal(w.Body.Bytes(), &e)
	if e.Code != int(errs.CodeInternal) {
		t.Fatalf("code = %d", e.Code)
	}
	if e.Msg != "系统繁忙，请稍后重试" {
		t.Fatalf("msg = %q, want registry text", e.Msg)
	}
	body := w.Body.String()
	if contains(body, "secret") {
		t.Fatalf("cause leaked to client: %s", body)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestPageEnvelope(t *testing.T) {
	c, w := newCtx()
	Page(c, []string{"a", "b"}, 42, PageReq{Page: 2, Size: 2})
	var e struct {
		Code int `json:"code"`
		Data struct {
			List  []string `json:"list"`
			Total int64    `json:"total"`
			Page  int      `json:"page"`
			Size  int      `json:"size"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatal(err)
	}
	if e.Code != 0 || e.Data.Total != 42 || e.Data.Page != 2 || e.Data.Size != 2 || len(e.Data.List) != 2 {
		t.Fatalf("page envelope = %+v", e)
	}
}

func TestPageReqDefaults(t *testing.T) {
	var p PageReq
	p.Defaults()
	if p.Page != 1 || p.Size != 10 {
		t.Fatalf("defaults = %+v", p)
	}
	p = PageReq{Page: 3, Size: 50}
	p.Defaults()
	if p.Page != 3 || p.Size != 50 {
		t.Fatalf("defaults overwrote explicit values: %+v", p)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./pkg/resp/ -v`
Expected: FAIL（包不存在）

- [ ] **Step 3: 实现**

`pkg/resp/resp.go`：

```go
// Package resp 是唯一出站 HTTP 响应出口：所有 API 响应必须经 OK/Fail/Page。
// 豁免清单（勿改）：oauth2 protocol 端点、/health*、/metrics、/debug/pprof/**。
package resp

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/logger"
)

type body struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

// statusFor 按错误码段映射 HTTP status：1→400，2→401（20003→403），3→400，5→500。
func statusFor(code errs.Code) int {
	switch {
	case code == errs.CodeForbidden:
		return http.StatusForbidden
	case code >= 10000 && code < 20000:
		return http.StatusBadRequest
	case code >= 20000 && code < 30000:
		return http.StatusUnauthorized
	case code >= 30000 && code < 40000:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func write(c *gin.Context, status int, code int, msg string, data any) {
	c.JSON(status, body{Code: code, Msg: msg, Data: data})
}

// OK 写成功信封；无返回值动作传 nil。
func OK(c *gin.Context, data any) {
	write(c, http.StatusOK, 0, "ok", data)
}

// Fail 写失败信封：*BizError 取码与文案；其他错误兜底 50000，原始错误只进日志不外泄。
func Fail(c *gin.Context, err error) {
	if err == nil {
		write(c, http.StatusInternalServerError, int(errs.CodeInternal), "系统繁忙，请稍后重试", nil)
		return
	}
	if be, ok := errs.As(err); ok {
		write(c, statusFor(be.Code), int(be.Code), be.Msg(), nil)
		return
	}
	if logger.Log != nil {
		logger.Log.Error("unhandled error", zap.Error(err), zap.String("path", c.Request.URL.Path))
	}
	write(c, http.StatusInternalServerError, int(errs.CodeInternal), "系统繁忙，请稍后重试", nil)
}
```

`pkg/resp/page.go`：

```go
package resp

import "github.com/gin-gonic/gin"

// PageReq 是统一分页入参（query: page/size）。绑定后必须调 Defaults()。
type PageReq struct {
	Page int `form:"page" binding:"omitempty,min=1"`
	Size int `form:"size" binding:"omitempty,min=1,max=100"`
}

// Defaults 补缺省值：page=1，size=10；已有非零值不覆盖。
func (p *PageReq) Defaults() {
	if p.Page <= 0 {
		p.Page = 1
	}
	if p.Size <= 0 {
		p.Size = 10
	}
}

// Page 写分页信封。req 须是 Defaults() 之后的值。
func Page[T any](c *gin.Context, list []T, total int64, req PageReq) {
	write(c, 200, 0, "ok", gin.H{
		"list":  list,
		"total": total,
		"page":  req.Page,
		"size":  req.Size,
	})
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./pkg/resp/ -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add pkg/resp/resp.go pkg/resp/page.go pkg/resp/resp_test.go
git commit -m "feat(resp): 统一响应信封 OK/Fail/Page + PageReq"
```

---

### Task 4: pkg/repo — Paginate[T] 泛型分页

**Files:**
- Create: `pkg/repo/paginate.go`
- Test: `pkg/repo/paginate_test.go`

**Interfaces:**
- Consumes: gorm.io/gorm；sqlite 测试驱动 `github.com/glebarez/sqlite`（go.mod 已有先例，若缺依赖按 `go get github.com/glebarez/sqlite` 添加）
- Produces: `func Paginate[T any](ctx context.Context, q *gorm.DB, page, size int) (items []T, total int64, err error)`；Where/Order 由调用方链好；不注入默认 Order；page/size 须已 Defaults（≤0 时按 1/10 兜底，防御性）

- [ ] **Step 1: 写失败测试**

```go
package repo

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type pageRow struct {
	ID   uint   `gorm:"primaryKey"`
	Name string
}

func (pageRow) TableName() string { return "page_rows" }

func setupDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&pageRow{}); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 25; i++ {
		if err := db.Create(&pageRow{Name: "n"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestPaginate(t *testing.T) {
	db := setupDB(t)
	items, total, err := Paginate[pageRow](context.Background(),
		db.Where("id > 0").Order("id asc"), 2, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 25 {
		t.Fatalf("total = %d, want 25", total)
	}
	if len(items) != 10 {
		t.Fatalf("len = %d, want 10", len(items))
	}
	if items[0].ID != 11 || items[9].ID != 20 {
		t.Fatalf("wrong page: first=%d last=%d", items[0].ID, items[9].ID)
	}
}

func TestPaginateLastPage(t *testing.T) {
	db := setupDB(t)
	items, total, err := Paginate[pageRow](context.Background(),
		db.Order("id asc"), 3, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 25 || len(items) != 5 {
		t.Fatalf("total=%d len=%d", total, len(items))
	}
}

func TestPaginateZeroArgsDefensive(t *testing.T) {
	db := setupDB(t)
	items, total, err := Paginate[pageRow](context.Background(), db, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 25 || len(items) != 10 { // 兜底 1/10
		t.Fatalf("total=%d len=%d", total, len(items))
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./pkg/repo/ -v`
Expected: FAIL（包不存在）

- [ ] **Step 3: 实现**

```go
// Package repo 提供 gorm 数据访问公共 helper。
package repo

import (
	"context"

	"gorm.io/gorm"
)

// Paginate 执行分页查询：Count + Offset + Limit + Find。
// Where/Order 由调用方链好；本函数不注入默认 Order。page/size ≤0 时按 1/10 兜底
// （正常路径 controller 侧 Defaults() 已保证，此处仅防御）。
func Paginate[T any](ctx context.Context, q *gorm.DB, page, size int) ([]T, int64, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 10
	}
	var total int64
	if err := q.WithContext(ctx).Model(new(T)).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []T
	if err := q.WithContext(ctx).
		Offset((page - 1) * size).
		Limit(size).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
```

注意：`Model(new(T))` 之后再 `Find(&items)` 在 gorm 里 Count 会污染 session——实现须用 `q.Session(&gorm.Session{})` 派生两次：

```go
func Paginate[T any](ctx context.Context, q *gorm.DB, page, size int) ([]T, int64, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 10
	}
	var total int64
	if err := q.WithContext(ctx).Session(&gorm.Session{}).Model(new(T)).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []T
	if err := q.WithContext(ctx).Session(&gorm.Session{}).
		Offset((page - 1) * size).
		Limit(size).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
```

（用第二个版本——两次独立 session，Count 的 Model 链不污染 Find。）

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./pkg/repo/ -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add pkg/repo/paginate.go pkg/repo/paginate_test.go go.mod go.sum
git commit -m "feat(repo): Paginate[T] 泛型分页——Count/Offset/Limit 收敛"
```

---

### Task 5: AccessLog 中间件（替换 Logger）+ 链序调整

**Files:**
- Create: `pkg/middleware/access_log.go`
- Test: `pkg/middleware/access_log_test.go`
- Modify: `alexgo-server/server/http.go:70-90`（中间件链：删 `middleware.Logger()`，AccessLog 挂到 OTEL 之后）

**Interfaces:**
- Consumes: zap、`pkg/auth.Claims`（claims context key `"claims"`）、otel `trace.SpanFromContext`
- Produces: `func AccessLog() gin.HandlerFunc`；字段 `method/path/query/status/latency_ms/client_ip/user_id/tenant_id/trace_id`；级别 5xx→Error、4xx→Warn、其他→Info；排除 `/health`、`/health/ready`、`/metrics`、`/debug/pprof/`

- [ ] **Step 1: 写失败测试**

```go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"alexGo-cloud/pkg/auth"
)

func TestAccessLogFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zapcore.InfoLevel)
	old := loggerForTest
	loggerForTest = zap.New(core)
	t.Cleanup(func() { loggerForTest = old })

	r := gin.New()
	r.Use(func(c *gin.Context) { // fake Auth：写入 claims
		c.Set("claims", &auth.Claims{UserID: 7, TenantID: 3})
	})
	r.Use(AccessLog())
	r.GET("/api/admin/test", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/test?x=1", nil)
	r.ServeHTTP(w, req)

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	fields := map[string]any{}
	for _, f := range entries[0].Context {
		fields[f.Key] = f.String
	}
	if fields["method"] != "GET" || fields["path"] != "/api/admin/test" {
		t.Fatalf("fields = %v", fields)
	}
	if fields["user_id"] != "7" || fields["tenant_id"] != "3" {
		t.Fatalf("user/tenant missing: %v", fields)
	}
	if fields["status"] != "200" {
		t.Fatalf("status = %v", fields["status"])
	}
}

func TestAccessLogLevelByStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zapcore.DebugLevel)
	old := loggerForTest
	loggerForTest = zap.New(core)
	t.Cleanup(func() { loggerForTest = old })

	r := gin.New()
	r.Use(AccessLog())
	r.GET("/err", func(c *gin.Context) { c.Status(500) })
	r.GET("/bad", func(c *gin.Context) { c.Status(404) })
	r.GET("/health", func(c *gin.Context) { c.Status(200) })

	for _, path := range []string{"/err", "/bad", "/health"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	}
	if len(logs.All()) != 2 { // /health 被排除
		t.Fatalf("entries = %d, want 2", len(logs.All()))
	}
	levels := map[string]zapcore.Level{}
	for _, e := range logs.All() {
		p, _ := e.ContextMap()["path"].(string)
		levels[p] = e.Level
	}
	if levels["/err"] != zapcore.ErrorLevel || levels["/bad"] != zapcore.WarnLevel {
		t.Fatalf("levels = %v", levels)
	}
}
```

测试辅助：`pkg/middleware` 需要一个可替换的 logger 变量。实现 Step 3 时在 `access_log.go` 提供 `var loggerForTest = pkglogger.Log`，AccessLog 内使用 `loggerForTest`（生产路径初始化后 `pkglogger.Log` 非 nil；nil 时降级 `c.Next()` 不打日志）。import 别名注意：测试文件里变量名 `loggerForTest` 与实现一致。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./pkg/middleware/ -run TestAccessLog -v`
Expected: FAIL（AccessLog 未定义）

- [ ] **Step 3: 实现 `access_log.go`**

```go
package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"alexGo-cloud/pkg/auth"
	pkglogger "alexGo-cloud/pkg/logger"
)

// loggerForTest 供单测替换 observer；生产始终用 pkglogger.Log。
var loggerForTest = pkglogger.Log

func accessLogSkip(path string) bool {
	switch path {
	case "/health", "/health/ready", "/metrics":
		return true
	}
	return len(path) >= len("/debug/pprof/") && path[:len("/debug/pprof/")] == "/debug/pprof/"
}

// AccessLog 结构化访问日志。必须挂在链尾（OTEL 之后、Auth 之后），
// 否则 trace_id/user_id 恒空。
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		if accessLogSkip(c.Request.URL.Path) {
			c.Next()
			return
		}
		start := time.Now()
		c.Next()
		if loggerForTest == nil {
			return
		}
		latency := time.Since(start)
		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("query", truncateQuery(c.Request.URL.RawQuery, 2048)),
			zap.Int("status", c.Writer.Status()),
			zap.Int64("latency_ms", latency.Milliseconds()),
			zap.String("client_ip", c.ClientIP()),
		}
		if cl, _ := c.Get("claims"); cl != nil {
			if claims, ok := cl.(*auth.Claims); ok && claims != nil {
				fields = append(fields,
					zap.Uint64("user_id", claims.UserID),
					zap.Uint64("tenant_id", claims.TenantID))
			}
		}
		if sc := trace.SpanFromContext(c.Request.Context()).SpanContext(); sc.HasTraceID() {
			fields = append(fields, zap.String("trace_id", sc.TraceID().String()))
		}
		status := c.Writer.Status()
		switch {
		case status >= 500:
			loggerForTest.Error("http request", fields...)
		case status >= 400:
			loggerForTest.Warn("http request", fields...)
		default:
			loggerForTest.Info("http request", fields...)
		}
	}
}

func truncateQuery(q string, max int) string {
	if len(q) <= max {
		return q
	}
	return q[:max] + "..."
}
```

依赖确认：`go.mod` 已有 `go.opentelemetry.io/otel`（trace.go 在用）；`auth.Claims` 定义于 `pkg/auth/auth.go:3`，字段 `UserID uint64` / `TenantID uint64`（已核对，与上面 `zap.Uint64` 一致）。

- [ ] **Step 4: 改 `alexgo-server/server/http.go` 中间件链**

在 `r.Use(...)` 链中：删除 `middleware.Logger(),` 一行（约 77 行），在 `trace.OTELMiddleware(),` 之后追加 `middleware.AccessLog(),`。保持注释风格，给 AccessLog 加一行注释：`// AccessLog：结构化访问日志（状态码/耗时/trace_id），排除健康检查端点。`

- [ ] **Step 5: 跑测试确认通过（含 http.go 既有单测）**

Run: `go test ./pkg/middleware/ ./alexgo-server/... -v`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add pkg/middleware/access_log.go pkg/middleware/access_log_test.go alexgo-server/server/http.go
git commit -m "feat(middleware): AccessLog 结构化访问日志——替换 Logger，补状态码/耗时/trace_id"
```

---

### Task 6: 自定义 Recovery（替换 gin.Recovery）

**Files:**
- Modify: `pkg/middleware/middleware.go`（Recovery 函数体替换；Logger 函数删除）
- Test: `pkg/middleware/recovery_test.go`
- Modify: `alexgo-server/server/http.go:72`（已是 `middleware.Recovery()`，无需改调用点，只确认）

**Interfaces:**
- Consumes: `pkg/resp`（`Fail`）、`pkg/errors`（`New`/`CodeInternal`）、zap
- Produces: `func Recovery() gin.HandlerFunc`——panic 时 zap.Error 带 stack + 回 `{code:50000,...}` 信封（HTTP 500）

- [ ] **Step 1: 写失败测试**

```go
package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestRecoveryWritesEnvelopeAndLogs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zapcore.InfoLevel)
	old := loggerForTest
	loggerForTest = zap.New(core)
	t.Cleanup(func() { loggerForTest = old })

	r := gin.New()
	r.Use(Recovery())
	r.GET("/panic", func(c *gin.Context) { panic("boom detail") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", w.Code)
	}
	var e struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatal(err)
	}
	if e.Code != 50000 {
		t.Fatalf("code = %d, want 50000", e.Code)
	}
	found := false
	for _, entry := range logs.All() {
		if entry.Level == zapcore.ErrorLevel {
			found = true
		}
	}
	if !found {
		t.Fatal("no error-level panic log")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./pkg/middleware/ -run TestRecovery -v`
Expected: FAIL（新 Recovery 未实现——旧 gin.Recovery 不会写信封）

- [ ] **Step 3: 实现——改 `middleware.go` 的 `Recovery()`，删除 `Logger()`**

顺序注意：必须**先 `resp.Fail` 写信封、再 `c.Abort()`**（Abort 后 gin 拒绝再写 body）：

```go
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				if loggerForTest != nil {
					loggerForTest.Error("panic recovered",
						zap.Any("panic", rec),
						zap.String("path", c.Request.URL.Path),
						zap.ByteString("stack", debug.Stack()))
				}
				resp.Fail(c, errs.New(errs.CodeInternal, nil))
				c.Abort()
			}
		}()
		c.Next()
	}
}
```

imports：`"runtime/debug"`、`"alexGo-cloud/pkg/errors"`、`"alexGo-cloud/pkg/resp"`、zap。同时删除文件里的 `Logger()` 函数（Task 5 已从 http.go 摘除调用；若有其他引用 `grep -rn "middleware.Logger" .` 必须为 0）。`Trace()` 占位保留不动。

- [ ] **Step 4: 跑测试确认通过 + 全仓确认无 Logger 引用**

Run: `go test ./pkg/middleware/ ./alexgo-server/... -v && ! grep -rn "middleware.Logger(" --include='*.go' .`
Expected: PASS，grep 无输出

- [ ] **Step 5: 提交**

```bash
git add pkg/middleware/middleware.go pkg/middleware/recovery_test.go
git commit -m "feat(middleware): 自定义 Recovery——panic 落 zap 栈日志 + 统一 50000 信封"
```

---

### Task 7: system/basic.go 全迁（dept/post/dict/config/notice，78 处）

**Files:**
- Modify: `modules/system/controller/admin/basic.go`（全部 handler）
- Test: 现有 `modules/system/controller/admin/notice_read_test.go` 适配（其余断言 200 状态码不受影响，逐个跑）

**Interfaces:**
- Consumes: `pkg/resp`（OK/Fail）、`pkg/errors`（`New`/`CodeXxx`）
- Produces: 无新接口；全部 handler 出站响应变信封

**迁移映射表（本 Task 与后续所有 controller Task 通用）**

| 旧写法 | 新写法 |
|---|---|
| `ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})` + `return` | `resp.Fail(ctx, err)` + `return` |
| `ctx.JSON(http.StatusBadRequest, gin.H{"error": "..."})`（显式业务拒绝） | `resp.Fail(ctx, errs.New(<语义码>, nil))` + `return` |
| `ShouldBindJSON 失败 → 400 gin.H{"error": err.Error()}` | `resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))` + `return` |
| `ctx.JSON(http.StatusOK, gin.H{"data": x})` | `resp.OK(ctx, x)` |
| `ctx.JSON(http.StatusOK, gin.H{"status": "ok"})` | `resp.OK(ctx, nil)` |
| `ctx.JSON(http.StatusOK, gin.H{"data": items, "total": total})` | `resp.Page(ctx, items, total, req.PageReq)`（先 Defaults()） |
| service 返回的裸 error（fmt.Errorf） | 原样透传 `resp.Fail(ctx, err)`（兜底 50000）；语义明确的错误在 **service 层**换 `errs.New(errs.CodeXxx, nil)`（本 Task 只在已能明确语义处替换，如「不存在」「已存在」） |

- [ ] **Step 1: 迁移前跑基线测试**

Run: `go test ./modules/system/... -count=1`
Expected: PASS（记录基线，后续不得倒退）

- [ ] **Step 2: 按映射表逐个 handler 迁移 basic.go**

示例（ListDepts，文件内所有 handler 同构套用）：

```go
// 旧
func (c *BasicController) ListDepts(ctx *gin.Context) {
	list, err := c.deptSvc.List(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": list})
}

// 新
func (c *BasicController) ListDepts(ctx *gin.Context) {
	list, err := c.deptSvc.List(ctx.Request.Context())
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, list)
}
```

Create/Update/Delete 类（原 `{"status":"ok"}`）：

```go
resp.OK(ctx, nil)
```

参数/资源语义错误示例（原 `ctx.JSON(http.StatusBadRequest, gin.H{"error": "parent not found"})` 之类字符串分支）：查 `grep -n 'gin.H{"error"' modules/system/controller/admin/basic.go` 列出全部显式错误分支，逐个映射语义码：
- 「不存在」→ `errs.New(errs.CodeUserNotFound, nil)` 或新建对应 3xxxx 码（如部门不存在→新增 `CodeDeptNotFound=30010`，同步加注册表文案与 Code 常量——**加码时同步更新 `pkg/errors/code.go` 常量 + registry + `code_test.go` 的导出常量清单**）
- 「已存在」→ 对应 `CodeXxxTaken`
- 校验类 → `CodeParamInvalid`

迁移完成后删除 `basic.go` 中不再使用的 import（`net/http`、标准库 `errors` 视用法）。

- [ ] **Step 3: 跑 system 测试并适配 notice_read_test.go**

Run: `go test ./modules/system/... -count=1`
若 `notice_read_test.go` 断言旧 body（如 `want unauthorized` / 旧 `{"data":...}` 结构）：把断言结构体改成信封（`Code/Msg/Data`），401 断言改查 `code==20001`。数据字段从裸数组变 `Data struct{...}` 的注意：notice ListMine 是非分页 `{"data": list}` → `resp.OK` 后 `data` 仍是裸数组，**形状不变**，多数断言只需把外层 struct 加 `Code` 字段。

- [ ] **Step 4: 全测试绿**

Run: `CGO_ENABLED=0 go test ./...`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add modules/system/controller/admin/basic.go modules/system/controller/admin/notice_read_test.go pkg/errors/
git commit -m "refactor(system): basic 控制器全迁统一信封——dept/post/dict/config/notice"
```

---

### Task 8: system 其余 controller 迁移（user/role/menu/tenant/auth/audit/app/vben）

**Files:**
- Modify: `modules/system/controller/admin/user.go`（15 处）
- Modify: `modules/system/controller/admin/role.go`（12）
- Modify: `modules/system/controller/admin/menu.go`（12）
- Modify: `modules/system/controller/admin/tenant.go`（11）
- Modify: `modules/system/controller/admin/auth.go`（13）
- Modify: `modules/system/controller/admin/audit.go`（4）
- Modify: `modules/system/controller/admin/controller.go`（2）
- Modify: `modules/system/controller/app/controller.go`（4）
- Modify: `modules/system/controller/vben/controller.go`（1）
- Test: 上述包内现有 `*_test.go`（controller_test.go、vben/controller_test.go 等）按需适配断言

**Interfaces:**
- Consumes: 同 Task 7 映射表 + 分页（user 列表若分页则 `resp.Page`；以实际 handler 为准——当前 user 列表是 `{"data": items}` 全量返回，保持 `resp.OK(ctx, items)`，**不顺手改分页**，分页改造只动已有 page/size 入参的接口）
- Produces: 无新接口

**禁改文件（本 Task 完成后 `git diff` 断言为空）：**
- `modules/system/oauth2/controller/protocol.go`

- [ ] **Step 1: 逐文件按映射表迁移（8 个文件同构套用，无新形态）**

auth.go 特殊点：登录失败显式 401/403 分支改：

```go
// 旧
ctx.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
// 新
resp.Fail(ctx, errs.New(errs.CodeUnauthorized, nil))
```

tenant.go 特殊点：「租户已停用」类业务分支改 `errs.New(errs.CodeTenantDisabled, nil)`。

user.go / role.go 特殊点：「已存在」类检查（如 CreateUser 里查重）改对应 `CodeXxxTaken`；service 已返回对应 error 的直接透传。

- [ ] **Step 2: 跑测试 + 适配断言**

Run: `go test ./modules/system/... -count=1`
适配规则同 Task 7 Step 3。

- [ ] **Step 3: 断言 oauth2 protocol 未动**

Run: `git diff --name-only | grep protocol && exit 1 || exit 0`
Expected: 退出码 0（无输出）

- [ ] **Step 4: 全测试绿**

Run: `CGO_ENABLED=0 go test ./...`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add modules/system/controller/
git commit -m "refactor(system): user/role/menu/tenant/auth/audit/app/vben 控制器全迁统一信封"
```

---

### Task 9: infra codegen 控制器迁移 + codegen 分页改造

**Files:**
- Modify: `modules/system/infra/controller/admin/codegen_controller.go`（11 处）
- Modify: `modules/system/infra/repository/codegen_repo.go:41`（手写 Offset/Limit → `repo.Paginate`）
- Test: `modules/system/infra/controller/admin/codegen_controller_test.go` 适配

**Interfaces:**
- Consumes: `pkg/resp`、`pkg/repo.Paginate[T]`（Task 4）
- Produces: codegen 仓库列表接口响应变 `data:{list,total,page,size}`；仓库函数签名保持 `(page, size int) ([]T, int64, error)`（内部换 Paginate，签名不变，service 不动）

- [ ] **Step 1: codegen_repo.go 分页替换**

```go
// 旧（约 30-45 行区域）
err := q.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&items).Error
// + 先 Count(&total) 的两段式

// 新
return repo.Paginate[T](ctx, q.Order("id desc"), page, size)
```

`ctx` 以函数现有入参为准（repo 方法若无 ctx，用 `context.Background()` 并保持签名不变——**不改签名**，避免动 service/controller 接口）。

- [ ] **Step 2: codegen_controller.go 按映射表迁移**

列表类 handler（原 `{"data": items, "total": total}`）改 `resp.Page`；controller 侧加分页入参 struct：

```go
type listReq struct {
	resp.PageReq `form:",inline"`
	// 已有 query 参数（如 table_name）按现有绑定 struct 内嵌 PageReq 并 Defaults()
}
```

若现有列表 handler 已有 `page`/`size` query 绑定，直接换用 `resp.PageReq` + `Defaults()`；预览/下载类保持 `resp.OK` 或自定义（ZIP 下载走 blob，Content-Type 非 JSON 的分支**只改其 JSON 错误分支**，文件流分支不动）。

- [ ] **Step 3: 跑测试并适配 codegen_controller_test.go**

Run: `go test ./modules/system/infra/... -count=1`
信封断言适配：分页响应解包结构从 `{data:[...], total:N}` 变 `{code:0, data:{list, total, page, size}}`。

- [ ] **Step 4: 全测试绿**

Run: `CGO_ENABLED=0 go test ./...`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add modules/system/infra/
git commit -m "refactor(infra): codegen 控制器迁统一信封 + repo.Paginate"
```

---

### Task 10: member/order/oauth2-admin 迁移 + auth 中间件信封 + 分页改造

**Files:**
- Modify: `modules/member/controller/admin/member_user.go`（8 处）
- Modify: `modules/member/repository/member_repo.go:59` → `repo.Paginate`
- Modify: `modules/member/controller/app/member_auth.go`（4 处）
- Modify: `modules/order/controller/admin/controller.go`（5 处）
- Modify: `modules/system/oauth2/controller/admin/client.go`（14 处）
- Modify: `modules/system/oauth2/repository/client.go:56` → `repo.Paginate`
- Modify: `pkg/middleware/auth.go`（`AbortWithStatusJSON` 401/403 → 信封）
- Test: `member_user_test.go`、`member_auth_test.go`、`client_test.go`、`oauth2/e2e_test.go` 适配

**Interfaces:**
- Consumes: `pkg/resp`、`pkg/repo.Paginate`、`pkg/errors`
- Produces: member/oauth2 client 列表响应变 `data:{list,total,page,size}`；auth 中间件 401/403 body 变 `{code:20001|20003,msg,data:null}`（**HTTP status 不变**，前端 rejected 链不受影响）

- [ ] **Step 1: 两个 repo 换 Paginate**（同 Task 9 Step 1 模式：`member_repo.go` 保持 `Order("id desc")`；`client.go` 保持 `Order("id ASC")`）

- [ ] **Step 2: 三个 controller 目录按映射表迁移**

member_user 列表（原 `{"data": items, "total": total}`）→ `resp.Page`；oauth2 admin client 列表同理。member_auth（`captcha_uuid`/登录）成功/失败分支逐个换；「验证码错误」显式分支用 `errs.New(errs.CodeCaptchaInvalid, nil)`（原 400）。

auth.go：

```go
// 旧
c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
// 新（保持 Abort 语义）
resp.Fail(c, errs.New(errs.CodeUnauthorized, nil))
c.Abort()
```

403 同理换 `CodeForbidden`。

- [ ] **Step 3: 禁改断言 + 测试适配**

Run: `git diff --name-only | grep 'oauth2/controller/protocol' && exit 1 || exit 0`
Run: `go test ./modules/... ./pkg/... -count=1`
适配：`member_auth_test.go` / `client_test.go` / `e2e_test.go` 中断言 `{"data":...}`、`{"error":...}` 的结构体加信封字段；分页断言改 `data.list`。`e2e_test.go` 若断言 401 body `unauthorized` 字符串 → 改查 `code==20001`。

- [ ] **Step 4: 全测试绿**

Run: `CGO_ENABLED=0 go test ./...`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add modules/member modules/order modules/system/oauth2/controller/admin pkg/middleware/auth.go
git commit -m "refactor: member/order/oauth2-admin 控制器 + auth 中间件迁统一信封，分页收敛"
```

---

### Task 11: 前端适配（信封收紧 + 列表页 .list）

**Files:**
- Modify: `apps/web-antd/src/api/request.ts:91`（`successCode` 收紧为 `code === 0`）
- Modify: `apps/web-antd/src/api/admin.ts`（`{status:"ok"}` 类调用点判 void；`listOauth2Clients` 374-377 行分页改真分页）
- Modify: `apps/web-antd/src/api/infra.ts`（6 行注释所述 workaround 移除）
- Modify: `apps/web-antd/src/views/**` 受影响列表页（consumers of 上面两个 api + member 列表页）
- Test: `pnpm run typecheck`；`pnpm --filter @vben/web-antd test`（如有拦截器单测）

**Interfaces:**
- Consumes: 后端 Task 7-10 完成的信封；拦截器返回 `data`（即 `{list,total,page,size}` 或裸数据）
- Produces: 前端全部 API 调用点兼容新信封；列表页服务端分页

- [ ] **Step 1: 迁移完备性检查（Review Focus #1）**

Run: `grep -rn 'gin.H{"data"\|gin.H{"status"\|gin.H{"error"' modules --include='*.go' | grep -v protocol.go | grep -v _test`
Expected: 无输出（全迁完成）。若有残留，先补齐再继续——**严禁在有残留时收紧 successCode**。

- [ ] **Step 2: 收紧 successCode**

```ts
// apps/web-antd/src/api/request.ts 约 79-91 行
// 旧：successCode: (code: any) => code === undefined || code === 0,
// 新（先删掉双信封兼容注释块，改为）：
successCode: (code: any) => code === 0,
```

- [ ] **Step 3: 改造 oauth2 clients 列表为真分页**

`admin.ts`：

```ts
// 旧
// 响应 {data,total} 被拦截器解出 data 后 total 丢失 → 页面本地切页，这里一次拉满上限 200。
export function listOauth2Clients() {
  return requestClient.get<Oauth2Client[]>(
    '/admin/oauth2/clients?page=1&size=200',
  );
}
// 新
export function listOauth2ClientsPage(params: { page: number; size: number }) {
  return requestClient.get<{ list: Oauth2Client[]; total: number }>(
    '/admin/oauth2/clients',
    { params },
  );
}
```

同步改消费该函数的 oauth2 客户端页面（`grep -rn "listOauth2Clients" apps/web-antd/src`）：接 `{list,total}`，删本地切页与 `slice` workaround。`infra.ts` 第 6 行注释块一并删除并按同模式改。

- [ ] **Step 4: `{status:"ok"}` 消费点改造**

Run: `grep -rn "status" apps/web-antd/src/api --include='*.ts' | grep -v "//"` 找出所有读返回 `status` 字段的调用（如 `resetUserPassword` 的 `Promise<{status:string}>`）→ 返回类型改 `Promise<void>`，调用点删除 `.status` 读取。

- [ ] **Step 5: typecheck + 相关视图手验**

Run: `pnpm run typecheck`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add apps/web-antd/src
git commit -m "refactor(web): 信封收紧 code===0 + 列表页真分页，废 200 上限 workaround"
```

---

### Task 12: roadmap 更新 + 全量回归冒烟

**Files:**
- Modify: `docs/roadmap.md`（P1 表新增一行并标 ✅）
- Verify only: 全仓测试 + 冒烟清单

- [ ] **Step 1: roadmap 记账**

P1 表追加：

```markdown
| P1-5 | **HTTP 基座规范化** | 统一信封 {code,msg,data} + 五位错误码 + Paginate[T] + DateTime + AccessLog/Recovery（spec：`docs/superpowers/specs/2026-10-15-http-foundation-design.md`） | ✅ |
```

状态总览 P1 计数从 `0/4` 改 `1/5`。

- [ ] **Step 2: 全量测试**

Run: `CGO_ENABLED=0 go test ./... && pnpm run typecheck`
Expected: 双绿

- [ ] **Step 3: 冒烟（本地起服务，按 `docs/usage.md` 的启动方式）**

逐项手验（每项录结果）：
1. 登录（vben 后台）→ 401 时错误提示可读（信封 msg）
2. 用户列表：翻页（page=2）返回不同行、total 正确
3. 创建重名用户 → 弹「用户名已存在」（code 30001）
4. 故意传错参数（POST 缺 username）→「参数校验失败」
5. oauth2 SSO 全流程仍通（protocol 端点未动）
6. 访问日志：启动日志可见 `http request` 条目含 status/latency_ms/trace_id

- [ ] **Step 4: 提交**

```bash
git add docs/roadmap.md
git commit -m "docs(roadmap): P1-5 HTTP 基座规范化收口"
```

---

## Self-Review 记录

- **Spec 覆盖**：信封（T3）、错误码（T2）、DateTime（T1）、分页信封+Paginate（T3/T4/T9/T10）、AccessLog（T5）、Recovery（T6）、前端（T11）、豁免清单（Global Constraints + T8/T10 diff 断言）、roadmap（T12）——全覆盖
- **占位符扫描**：T7/T8/T10 中「按映射表逐 handler 迁移」以完整映射表 + 具体示例承载（78/45 处逐行贴代码不现实，映射表穷尽了全部 7 种旧形态）
- **类型一致性**：`errs.New(Code, error)`、`resp.Page[T](c, []T, int64, PageReq)`、`repo.Paginate[T](ctx, *gorm.DB, int, int)` 各 Task 引用一致
- **Review Focus**：5 条均有归属 Task 与测试（见各 Task 标注）
