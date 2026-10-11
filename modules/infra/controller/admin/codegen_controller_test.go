package admin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/infra/model"
	"alexGo-cloud/pkg/codegen/builder"
	cgmodel "alexGo-cloud/pkg/codegen/model"
)

// stubService 记录调用，按 case 返回。
type stubService struct {
	importErr error
	syncRes   *model.SyncResult
	syncErr   error
	gotImport string
	saved     []*model.CodegenColumn
}

func (s *stubService) ListTables(context.Context, int, int) ([]*model.CodegenTable, int64, error) {
	return []*model.CodegenTable{{ID: 1, Name: "order_items", ClassName: "OrderItem"}}, 1, nil
}
func (s *stubService) GetTable(_ context.Context, id uint64) (*model.CodegenTable, []*model.CodegenColumn, error) {
	if id != 1 {
		return nil, nil, errors.New("record not found")
	}
	return &model.CodegenTable{ID: 1, Name: "order_items"},
		[]*model.CodegenColumn{{ID: 10, TableID: 1, Name: "id", IsPK: true}}, nil
}
func (s *stubService) ImportTable(_ context.Context, name string) (*model.CodegenTable, error) {
	s.gotImport = name
	if s.importErr != nil {
		return nil, s.importErr
	}
	return &model.CodegenTable{ID: 1, Name: name}, nil
}
func (s *stubService) UpdateTable(context.Context, *model.CodegenTable) error { return nil }
func (s *stubService) DeleteTable(context.Context, uint64) error              { return nil }
func (s *stubService) SaveColumns(_ context.Context, _ uint64, cols []*model.CodegenColumn) error {
	s.saved = cols
	return nil
}
func (s *stubService) ListColumns(_ context.Context, _ uint64) ([]*model.CodegenColumn, error) {
	return []*model.CodegenColumn{{ID: 10, TableID: 1, Name: "id"}}, nil
}
func (s *stubService) Sync(context.Context, uint64) (*model.SyncResult, error) {
	if s.syncErr != nil {
		return nil, s.syncErr
	}
	if s.syncRes != nil {
		return s.syncRes, nil
	}
	// 真实 service 成功时从不返回 (nil, nil)
	return &model.SyncResult{}, nil
}

func newTestRouter(stub *stubService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	c := NewCodegenController(stub, builder.Options{Module: "infra"})
	g := r.Group("/api/admin/infra/codegen")
	c.RegisterRoutes(g)
	return r
}

func doJSON(t *testing.T, r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestListTables_Envelope(t *testing.T) {
	r := newTestRouter(&stubService{})
	w := doJSON(t, r, "GET", "/api/admin/infra/codegen/tables?page=1&size=10", "")
	if w.Code != 200 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"total":1`) || !strings.Contains(body, "order_items") {
		t.Errorf("body=%s", body)
	}
}

// 信封铁律：列表 data+total；详情 data；变更 status ok；错误 error 字段。
func TestEnvelopes_AllShapes(t *testing.T) {
	r := newTestRouter(&stubService{})

	w := doJSON(t, r, "GET", "/api/admin/infra/codegen/tables/1", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"data"`) {
		t.Errorf("detail: %d %s", w.Code, w.Body)
	}

	w = doJSON(t, r, "POST", "/api/admin/infra/codegen/import", `{"table_name":"order_items"}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"data"`) {
		t.Errorf("import: %d %s", w.Code, w.Body)
	}

	w = doJSON(t, r, "DELETE", "/api/admin/infra/codegen/tables/1", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"ok"`) {
		t.Errorf("delete: %d %s", w.Code, w.Body)
	}

	w = doJSON(t, r, "POST", "/api/admin/infra/codegen/tables/1/sync", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"added"`) {
		t.Errorf("sync: %d %s", w.Code, w.Body)
	}
}

// 业务拒绝/不存在 → 400 + error；绝不 404（仓库无此惯例）。
func TestErrors_400Never404(t *testing.T) {
	r := newTestRouter(&stubService{importErr: cgmodel.ErrTableNotFound})
	w := doJSON(t, r, "POST", "/api/admin/infra/codegen/import", `{"table_name":"ghost"}`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("import notfound: code=%d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"error"`) {
		t.Errorf("body=%s", w.Body)
	}

	r2 := newTestRouter(&stubService{})
	w2 := doJSON(t, r2, "GET", "/api/admin/infra/codegen/tables/999", "")
	if w2.Code != http.StatusBadRequest {
		t.Errorf("detail missing: code=%d, want 400", w2.Code)
	}
}

// 空 table_name → 400，且不触达 service。
func TestImport_ValidationBeforeService(t *testing.T) {
	stub := &stubService{}
	r := newTestRouter(stub)
	w := doJSON(t, r, "POST", "/api/admin/infra/codegen/import", `{"table_name":""}`)
	if w.Code != 400 {
		t.Errorf("code=%d", w.Code)
	}
	if stub.gotImport != "" {
		t.Error("空表名不应调用 service")
	}
}

// generate 端点是 M3 占位：返回 500 + 明确文案，不 panic。
func TestGenerate_ReservedForM3(t *testing.T) {
	r := newTestRouter(&stubService{})
	w := doJSON(t, r, "GET", "/api/admin/infra/codegen/tables/1/generate", "")
	if w.Code != 500 || !strings.Contains(w.Body.String(), "M3") {
		t.Errorf("code=%d body=%s", w.Code, w.Body)
	}
}
