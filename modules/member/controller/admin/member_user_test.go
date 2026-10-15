package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/member/model"
	"alexGo-cloud/modules/member/service"
)

// codeOf 解析统一信封的 code 字段（resp.OK/Fail/Page 出站；旧 {"status":"ok"}/{"error":...}
// 断言迁移为查 code——0 成功、10001 参数、10002 缺参）。
func codeOf(t *testing.T, w *httptest.ResponseRecorder) int {
	t.Helper()
	var body struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal envelope: %v, body=%s", err, w.Body.String())
	}
	return body.Code
}

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
// 信封适配（不弱化）：`{}` 绑定成功但缺字段 → code 10002（缺参）；空 body 绑定失败 → 10001。
func TestUpdateStatus_MissingStatus_400(t *testing.T) {
	for _, tc := range []struct {
		body string
		code int
	}{{`{}`, 10002}, {``, 10001}} {
		svc := &stubSvc{}
		w := doStatus(t, svc, tc.body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("body=%q status = %d, want 400", tc.body, w.Code)
		}
		if got := codeOf(t, w); got != tc.code {
			t.Errorf("body=%q code = %d, want %d; body=%s", tc.body, got, tc.code, w.Body.String())
		}
		if svc.gotID != 0 {
			t.Errorf("Disable called on invalid input: id=%d", svc.gotID)
		}
	}
}

// status ∉ {0,1} → 400（10001 参数校验）。
func TestUpdateStatus_OutOfRange_400(t *testing.T) {
	for _, body := range []string{`{"status":2}`, `{"status":-1}`} {
		svc := &stubSvc{}
		w := doStatus(t, svc, body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("body=%q status = %d, want 400; body=%s", body, w.Code, w.Body.String())
		}
		if got := codeOf(t, w); got != 10001 {
			t.Errorf("body=%q code = %d, want 10001; body=%s", body, got, w.Body.String())
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
		if got := codeOf(t, w); got != 0 || !strings.Contains(w.Body.String(), `"code":0`) {
			t.Errorf("body=%q code = %d, want 0（信封键须在场）; body=%s", tc.body, got, w.Body.String())
		}
		if svc.gotID != 5 || svc.gotStatus != tc.want {
			t.Errorf("Disable(%d, %d), want (5, %d)", svc.gotID, svc.gotStatus, tc.want)
		}
	}
}

// service 层的 ErrInvalidStatus 归一为 400（双层校验的兜底路径）→ 10001。
func TestUpdateStatus_ServiceInvalidStatus_400(t *testing.T) {
	svc := &stubSvc{err: service.ErrInvalidStatus}
	w := doStatus(t, svc, `{"status":0}`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
	if got := codeOf(t, w); got != 10001 {
		t.Errorf("code = %d, want 10001; body=%s", got, w.Body.String())
	}
}

// 非法 id → 400（10001）。
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
	if got := codeOf(t, w); got != 10001 {
		t.Errorf("code = %d, want 10001; body=%s", got, w.Body.String())
	}
}

// List 正常返回：统一分页信封 data:{list,total,page,size}（原 {"data":items,"total":total}）。
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
	var resp struct {
		Code int `json:"code"`
		Data struct {
			List []struct {
				ID     uint64 `json:"id"`
				Mobile string `json:"mobile"`
			} `json:"list"`
			Total int64 `json:"total"`
			Page  int   `json:"page"`
			Size  int   `json:"size"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Code != 0 {
		t.Errorf("code = %d, want 0; body=%s", resp.Code, w.Body.String())
	}
	if len(resp.Data.List) != 1 || resp.Data.List[0].Mobile != "138" {
		t.Errorf("list = %+v, want [{1 138}]", resp.Data.List)
	}
	if resp.Data.Total != 1 || resp.Data.Page != 1 || resp.Data.Size != 10 {
		t.Errorf("total/page/size = %d/%d/%d, want 1/1/10",
			resp.Data.Total, resp.Data.Page, resp.Data.Size)
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
