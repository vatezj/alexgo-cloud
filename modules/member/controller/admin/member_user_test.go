package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/member/model"
	"alexGo-cloud/modules/member/service"
)

// stubSvc：只关心 UpdateStatus 的状态与参数传递。
type stubSvc struct {
	service.MemberService
	gotID     uint64
	gotStatus int
	err       error
}

func (s *stubSvc) Disable(_ context.Context, id uint64, status int) error {
	if s.err != nil {
		return s.err
	}
	s.gotID, s.gotStatus = id, status
	return nil
}

func doStatus(t *testing.T, svc service.MemberService, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	c := NewUserController(svc)
	r.PUT("/api/admin/member/users/:id/status", c.UpdateStatus)
	req := httptest.NewRequest(http.MethodPut, "/api/admin/member/users/5/status", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// T5 deferred 关闭：空 body / 缺 status 字段 → 400（绝不默认成 0=停用）。
func TestUpdateStatus_MissingStatus_400(t *testing.T) {
	for _, body := range []string{`{}`, ``} {
		svc := &stubSvc{}
		w := doStatus(t, svc, body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("body=%q status = %d, want 400", body, w.Code)
		}
		if svc.gotID != 0 {
			t.Errorf("Disable called on invalid input: id=%d", svc.gotID)
		}
	}
}

// status ∉ {0,1} → 400。
func TestUpdateStatus_OutOfRange_400(t *testing.T) {
	for _, body := range []string{`{"status":2}`, `{"status":-1}`} {
		svc := &stubSvc{}
		w := doStatus(t, svc, body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("body=%q status = %d, want 400; body=%s", body, w.Code, w.Body.String())
		}
	}
}

// 合法值 0/1 → 200，且参数原样传到 service。
func TestUpdateStatus_OK(t *testing.T) {
	for _, tc := range []struct {
		body string
		want int
	}{{`{"status":0}`, 0}, {`{"status":1}`, 1}} {
		svc := &stubSvc{}
		w := doStatus(t, svc, tc.body)
		if w.Code != http.StatusOK {
			t.Errorf("body=%q status = %d, want 200; body=%s", tc.body, w.Code, w.Body.String())
		}
		if svc.gotID != 5 || svc.gotStatus != tc.want {
			t.Errorf("Disable(%d, %d), want (5, %d)", svc.gotID, svc.gotStatus, tc.want)
		}
	}
}

// service 层的 ErrInvalidStatus 归一为 400（双层校验的兜底路径）。
func TestUpdateStatus_ServiceInvalidStatus_400(t *testing.T) {
	svc := &stubSvc{err: service.ErrInvalidStatus}
	w := doStatus(t, svc, `{"status":0}`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// 非法 id → 400。
func TestUpdateStatus_InvalidID_400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	c := NewUserController(&stubSvc{})
	r.PUT("/api/admin/member/users/:id/status", c.UpdateStatus)
	req := httptest.NewRequest(http.MethodPut, "/api/admin/member/users/abc/status", strings.NewReader(`{"status":0}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// List 正常返回（controller 不吞错、结构含 data/total）。
func TestList_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	c := NewUserController(&stubListSvc{items: []*model.MemberUser{{ID: 1, Mobile: "138"}}, total: 1})
	r.GET("/api/admin/member/users", c.List)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/admin/member/users?page=1&size=10", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"total":1`) {
		t.Errorf("body = %s", w.Body.String())
	}
}

type stubListSvc struct {
	service.MemberService
	items []*model.MemberUser
	total int64
}

func (s *stubListSvc) List(context.Context, int, int) ([]*model.MemberUser, int64, error) {
	return s.items, s.total, nil
}
