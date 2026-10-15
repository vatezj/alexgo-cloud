package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/service"
	"alexGo-cloud/pkg/auth"
)

// fakeNoticeSvc：可编程 NoticeService 桩（本包无 testify，手写）。
type fakeNoticeSvc struct {
	items    []*service.NoticeItem
	readErr  error  // 非 nil 时 MarkRead/MarkAllRead 返回
	markID   uint64 // 最近一次 MarkRead 的 noticeID
	markUser uint64 // 最近一次 MarkRead/MarkAllRead 的 userID
	markAll  bool
}

func (f *fakeNoticeSvc) List(context.Context) ([]*model.SystemNotice, error) {
	return nil, nil
}
func (f *fakeNoticeSvc) Create(_ context.Context, n *model.SystemNotice) (*model.SystemNotice, error) {
	return n, nil
}
func (f *fakeNoticeSvc) Update(context.Context, *model.SystemNotice) error { return nil }
func (f *fakeNoticeSvc) Delete(context.Context, uint64) error              { return nil }
func (f *fakeNoticeSvc) ListMine(context.Context, uint64) ([]*service.NoticeItem, error) {
	return f.items, nil
}
func (f *fakeNoticeSvc) MarkRead(_ context.Context, userID, noticeID uint64) error {
	if f.readErr != nil {
		return f.readErr
	}
	f.markID, f.markUser = noticeID, userID
	return nil
}
func (f *fakeNoticeSvc) MarkAllRead(_ context.Context, userID uint64) error {
	if f.readErr != nil {
		return f.readErr
	}
	f.markUser, f.markAll = userID, true
	return nil
}

// newNoticeRouter 按 module.go:211-214 + 新增三行的同形顺序装配**全部七个**公告路由
// （静态段 mine/read-all 与参数段 :id 混挂——Review Focus 4：树构建 panic 会让本用例全灭）。
// claims 为 nil 模拟中间件缺失。同包测试直接构造结构体字面量，绕开 NewBasicController 的 5 依赖。
func newNoticeRouter(svc service.NoticeService, claims *auth.Claims) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	ctrl := &BasicController{noticeSvc: svc}
	g := r.Group("/api/admin/system")
	withClaims := func(h gin.HandlerFunc) gin.HandlerFunc {
		return func(c *gin.Context) {
			if claims != nil {
				c.Set("claims", claims)
			}
			h(c)
		}
	}
	g.GET("/notices", withClaims(ctrl.ListNotices))
	g.POST("/notices", withClaims(ctrl.CreateNotice))
	g.PUT("/notices", withClaims(ctrl.UpdateNotice))
	g.DELETE("/notices/:id", withClaims(ctrl.DeleteNotice))
	g.GET("/notices/mine", withClaims(ctrl.ListMyNotices))
	g.POST("/notices/read-all", withClaims(ctrl.ReadAllNotices))
	g.POST("/notices/:id/read", withClaims(ctrl.MarkNoticeRead))
	return r
}

func doReq(r *gin.Engine, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(method, path, nil))
	return w
}

// codeOf 解析统一信封的 code 字段（resp.OK/Fail 出站；旧 {"status":"ok"}/{"error":...} 断言
// 迁移为查 code——0 成功、10001 参数、20001 未登录、30006 公告不存在）。
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

// mine 信封：data 平铺公告字段 + read_status（NoticeItem 内嵌指针展开）。
func TestListMyNotices_OK(t *testing.T) {
	svc := &fakeNoticeSvc{items: []*service.NoticeItem{
		{SystemNotice: &model.SystemNotice{ID: 1, Title: "A", Content: "c"}, ReadStatus: 1},
		{SystemNotice: &model.SystemNotice{ID: 2, Title: "B", Content: "c"}, ReadStatus: 0},
	}}
	w := doReq(newNoticeRouter(svc, &auth.Claims{UserID: 42, TenantID: 1}),
		http.MethodGet, "/api/admin/system/notices/mine")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Code int `json:"code"`
		Data []struct {
			ID         uint64 `json:"id"`
			Title      string `json:"title"`
			ReadStatus int    `json:"read_status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Code != 0 {
		t.Errorf("code = %d, want 0", resp.Code)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("data len = %d, want 2", len(resp.Data))
	}
	if resp.Data[0].Title != "A" || resp.Data[0].ReadStatus != 1 ||
		resp.Data[1].Title != "B" || resp.Data[1].ReadStatus != 0 {
		t.Errorf("data = %+v, want A/1 B/0", resp.Data)
	}
}

// Review Focus 3：缺 claims 必须 401，不 500 不 panic。
func TestListMyNotices_NoClaims_401(t *testing.T) {
	w := doReq(newNoticeRouter(&fakeNoticeSvc{}, nil),
		http.MethodGet, "/api/admin/system/notices/mine")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body=%s", w.Code, w.Body.String())
	}
	if got := codeOf(t, w); got != 20001 {
		t.Errorf("code = %d, want 20001; body=%s", got, w.Body.String())
	}
}

func TestMarkNoticeRead_OK(t *testing.T) {
	svc := &fakeNoticeSvc{}
	w := doReq(newNoticeRouter(svc, &auth.Claims{UserID: 42}),
		http.MethodPost, "/api/admin/system/notices/5/read")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if svc.markID != 5 || svc.markUser != 42 {
		t.Errorf("MarkRead(notice=%d, user=%d), want (5, 42)", svc.markID, svc.markUser)
	}
	if got := codeOf(t, w); got != 0 {
		t.Errorf("code = %d, want 0; body=%s", got, w.Body.String())
	}
}

func TestMarkNoticeRead_NotFound_400(t *testing.T) {
	svc := &fakeNoticeSvc{readErr: service.ErrNoticeNotFound}
	w := doReq(newNoticeRouter(svc, &auth.Claims{UserID: 42}),
		http.MethodPost, "/api/admin/system/notices/5/read")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
	if got := codeOf(t, w); got != 30006 {
		t.Errorf("code = %d, want 30006; body=%s", got, w.Body.String())
	}
}

func TestMarkNoticeRead_ZeroID_400(t *testing.T) {
	w := doReq(newNoticeRouter(&fakeNoticeSvc{}, &auth.Claims{UserID: 42}),
		http.MethodPost, "/api/admin/system/notices/0/read")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d body = %s, want 400", w.Code, w.Body.String())
		return
	}
	if got := codeOf(t, w); got != 10001 {
		t.Errorf("code = %d, want 10001; body=%s", got, w.Body.String())
	}
}

func TestReadAllNotices_OK(t *testing.T) {
	svc := &fakeNoticeSvc{}
	w := doReq(newNoticeRouter(svc, &auth.Claims{UserID: 42}),
		http.MethodPost, "/api/admin/system/notices/read-all")
	if w.Code != http.StatusOK || !svc.markAll || svc.markUser != 42 {
		t.Errorf("status = %d markAll = %v user = %d, want 200/true/42", w.Code, svc.markAll, svc.markUser)
	}
	if got := codeOf(t, w); got != 0 {
		t.Errorf("code = %d, want 0; body=%s", got, w.Body.String())
	}
}
