package admin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/infra/model"
	"alexGo-cloud/pkg/codegen/builder"
	cgmodel "alexGo-cloud/pkg/codegen/model"
)

// stubService 记录调用，按 case 返回。
type stubService struct {
	importErr   error
	syncRes     *model.SyncResult
	syncErr     error
	gotImport   string
	saved       []*model.CodegenColumn
	dbTablesErr error
	// —— M3 新增 ——
	previewFiles []cgmodel.GeneratedFile
	previewErr   error
	zipBytes     []byte
	downloadErr  error
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
func (s *stubService) ListDBTables(context.Context) ([]string, error) {
	if s.dbTablesErr != nil {
		return nil, s.dbTablesErr
	}
	return []string{"orders", "menus"}, nil
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

func (s *stubService) Preview(context.Context, []uint64) ([]cgmodel.GeneratedFile, error) {
	if s.previewErr != nil {
		return nil, s.previewErr
	}
	return s.previewFiles, nil
}

func (s *stubService) Download(context.Context, []uint64) ([]byte, error) {
	if s.downloadErr != nil {
		return nil, s.downloadErr
	}
	return s.zipBytes, nil
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

// M3：preview 端点（取代 generate 占位）——信封 {data:[{path,content}]}。
func TestPreview_Envelope(t *testing.T) {
	stub := &stubService{previewFiles: []cgmodel.GeneratedFile{{
		Path:    "modules/infra/model/order_item.go",
		Content: []byte("package model\n"),
	}}}
	r := newTestRouter(stub)
	w := doJSON(t, r, "POST", "/api/admin/infra/codegen/preview", `{"table_ids":[1]}`)
	if w.Code != 200 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body)
	}
	body := w.Body.String()
	for _, want := range []string{`"data"`, `"path":`, `"content":`, "modules/infra/model/order_item.go", "package model"} {
		if !strings.Contains(body, want) {
			t.Errorf("body 缺 %q: %s", want, body)
		}
	}
}

// 空/缺 table_ids → 400，绝不静默当全表（Review Focus #3）。
func TestPreview_EmptyIDs_400(t *testing.T) {
	r := newTestRouter(&stubService{})
	for _, payload := range []string{"", "{}", `{"table_ids":[]}`} {
		w := doJSON(t, r, "POST", "/api/admin/infra/codegen/preview", payload)
		if w.Code != 400 {
			t.Errorf("payload=%q code=%d body=%s", payload, w.Code, w.Body)
		}
	}
	w := doJSON(t, r, "POST", "/api/admin/infra/codegen/preview", `{"table_ids":[1]}`)
	if w.Code != 200 { // stub 空 files → data:[]（合法信封，不算错）
		t.Errorf("合法空渲染 code=%d body=%s", w.Code, w.Body)
	}
}

// download：zip 流 + 两个头；业务错误 → 400（Review Focus #4）。
func TestDownload_ZipStream(t *testing.T) {
	r := newTestRouter(&stubService{zipBytes: []byte("PK\x03\x04fake-entry")})
	w := doJSON(t, r, "POST", "/api/admin/infra/codegen/download", `{"table_ids":[1]}`)
	if w.Code != 200 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/zip" {
		t.Errorf("Content-Type=%q", ct)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, `filename="codegen.zip"`) {
		t.Errorf("Content-Disposition=%q", cd)
	}
	if !strings.HasPrefix(w.Body.String(), "PK") {
		t.Errorf("body 前缀=%q", w.Body.String()[:min(4, w.Body.Len())])
	}

	errStub := newTestRouter(&stubService{downloadErr: errors.New("table 9: not found")})
	w = doJSON(t, errStub, "POST", "/api/admin/infra/codegen/download", `{"table_ids":[9]}`)
	if w.Code != 400 {
		t.Errorf("download err code=%d body=%s", w.Code, w.Body)
	}
}

// db-tables：导入弹窗下拉数据源，信封 data 字符串数组；
// 元数据不可达（读库类）→ 500 + error，绝不 404。
func TestDBTables_Envelope(t *testing.T) {
	r := newTestRouter(&stubService{})
	w := doJSON(t, r, "GET", "/api/admin/infra/codegen/db-tables", "")
	if w.Code != 200 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"data"`) || !strings.Contains(body, "orders") {
		t.Errorf("body=%s", body)
	}
}

func TestDBTables_MetadataDown_500(t *testing.T) {
	r := newTestRouter(&stubService{dbTablesErr: errors.New("metadata unreachable")})
	w := doJSON(t, r, "GET", "/api/admin/infra/codegen/db-tables", "")
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), `"error"`) {
		t.Errorf("code=%d body=%s", w.Code, w.Body)
	}
}
