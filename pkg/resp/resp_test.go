package resp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/logger"
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

// 终审修复①：BizError 的 cause（gin 校验细节如字段名）必须进日志，
// 且响应体仍为三键信封、cause 不外泄。newCtx 的 Request 为 nil——顺带钉住
// 路径读取的 nil guard（否则在 logger.Log 非 nil 时单测 panic）。
func TestFailBizErrorCauseInLogNotBody(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	old := logger.Log
	logger.Log = zap.New(core)
	t.Cleanup(func() { logger.Log = old })

	c, w := newCtx()
	cause := `gin: binding failed for field "username"`
	Fail(c, errs.New(errs.CodeParamInvalid, errors.New(cause)))

	// 响应侧：HTTP 400 + {code,msg,data:null}，body 无 cause。
	if w.Code != http.StatusBadRequest {
		t.Fatalf("http = %d", w.Code)
	}
	var e envelope
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatal(err)
	}
	if e.Code != int(errs.CodeParamInvalid) || e.Msg != "参数校验失败" || e.Data != nil {
		t.Fatalf("envelope = %+v", e)
	}
	if contains(w.Body.String(), "binding failed") {
		t.Fatalf("cause leaked to client: %s", w.Body.String())
	}

	// 日志侧：cause 作为 error 字段出现在日志输出。
	found := false
	for _, entry := range logs.All() {
		v, ok := entry.ContextMap()["error"]
		if ok && contains(fmt.Sprintf("%v", v), "binding failed") {
			found = true
		}
	}
	if !found {
		t.Fatalf("cause not logged, entries = %+v", logs.All())
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
