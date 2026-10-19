package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/service"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/excel"
)

// stubUserService：只关心 ListUsers 的桩（嵌入接口，其余方法不触达）。
type stubUserService struct {
	service.UserService
	users []*model.User
	err   error
}

func (s *stubUserService) ListUsers(context.Context) ([]*model.User, error) {
	return s.users, s.err
}

func doExport(t *testing.T, svc service.UserService) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	c := NewUserController(svc)
	r.GET("/api/admin/system/users/export", c.ExportUsers)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/admin/system/users/export", nil))
	return w
}

// 成功路径：200 + xlsx MIME + Disposition(users_) + 读回表头/首行关键字段。
func TestExportUsers_OK_XlsxRoundTrip(t *testing.T) {
	when := time.Date(2026, 10, 19, 8, 30, 0, 0, time.UTC)
	svc := &stubUserService{users: []*model.User{{
		ID: 1, Username: "alice", Nickname: "爱丽丝", Email: "a@x.com",
		Mobile: "13800000000", Sex: 2, Status: 1, DeptID: 7, Remark: "备注",
		LoginIP: "1.2.3.4", LoginDate: &when, CreatedAt: when,
	}}}
	w := doExport(t, svc)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != excel.MIME {
		t.Errorf("Content-Type = %q, want %q", ct, excel.MIME)
	}
	cd := w.Header().Get("Content-Disposition")
	if !strings.HasPrefix(cd, "attachment") || !strings.Contains(cd, "users_") {
		t.Errorf("Content-Disposition = %q", cd)
	}
	f, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer f.Close()
	rows, err := f.GetRows("用户")
	if err != nil {
		t.Fatalf("GetRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0][0] != "ID" || rows[0][1] != "用户名" || len(rows[0]) != 12 {
		t.Errorf("header = %v, want 12 cols starting ID/用户名", rows[0])
	}
	if rows[1][1] != "alice" || rows[1][2] != "爱丽丝" || rows[1][10] != "2026-10-19T08:30:00Z" {
		t.Errorf("row1 = %v", rows[1])
	}
}

// 超限：Build 在写字节前拒 → 信封 400/30013（RF② controller 侧半边；体非 PK zip 头）。
func TestExportUsers_TooManyRows_Envelope400(t *testing.T) {
	users := make([]*model.User, excel.MaxExportRows+1)
	for i := range users {
		users[i] = &model.User{ID: uint64(i + 1)}
	}
	w := doExport(t, &stubUserService{users: users})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%.80s", w.Code, w.Body.String())
	}
	var env struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("body must be JSON envelope: %v", err)
	}
	if env.Code != int(errs.CodeExportTooManyRows) || env.Msg != "导出行数超过上限" {
		t.Errorf("envelope = %d/%q, want 30013/导出行数超过上限", env.Code, env.Msg)
	}
	if bytes.HasPrefix(w.Body.Bytes(), []byte("PK")) {
		t.Error("30013 response must not start with xlsx zip magic")
	}
}

// 取行 DB 错 → 50000 信封（写流前，安全）。
func TestExportUsers_ListError_Envelope(t *testing.T) {
	w := doExport(t, &stubUserService{err: errs.New(errs.CodeInternal, nil)})
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}
