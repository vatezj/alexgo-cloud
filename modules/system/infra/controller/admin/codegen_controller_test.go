package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

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
		// 与真实 repo.GetTable 一致：First 未命中 → gorm.ErrRecordNotFound
		return nil, nil, gorm.ErrRecordNotFound
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

// codeOf 解析统一信封的 code 字段（resp.OK/Fail/Page 出站；旧 {"status":"ok"}/{"error":...}
// 断言迁移为查 code——0 成功、10001 参数、30007 数据表不存在、50000 系统兜底）。
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

// 列表信封：data:{list,total,page,size}（原 {data,total} 平铺）。
func TestListTables_Envelope(t *testing.T) {
	r := newTestRouter(&stubService{})
	w := doJSON(t, r, "GET", "/api/admin/infra/codegen/tables?page=1&size=10", "")
	if w.Code != 200 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body)
	}
	if got := codeOf(t, w); got != 0 {
		t.Errorf("envelope code=%d, want 0; body=%s", got, w.Body)
	}
	var body struct {
		Data struct {
			List []struct {
				TableName string `json:"table_name"`
			} `json:"list"`
			Total int64 `json:"total"`
			Page  int   `json:"page"`
			Size  int   `json:"size"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	d := body.Data
	if d.Total != 1 || d.Page != 1 || d.Size != 10 || len(d.List) != 1 || d.List[0].TableName != "order_items" {
		t.Errorf("page envelope mismatch: %+v, body=%s", d, w.Body)
	}
}

// 信封铁律：列表 data:{list,total,page,size}；详情 data；变更 data:null（code 0）；
// 失败 code+msg（语义码）。
func TestEnvelopes_AllShapes(t *testing.T) {
	r := newTestRouter(&stubService{})

	w := doJSON(t, r, "GET", "/api/admin/infra/codegen/tables/1", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"data"`) {
		t.Errorf("detail: %d %s", w.Code, w.Body)
	}
	if got := codeOf(t, w); got != 0 {
		t.Errorf("detail code=%d, want 0; body=%s", got, w.Body)
	}

	w = doJSON(t, r, "POST", "/api/admin/infra/codegen/import", `{"table_name":"order_items"}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"data"`) {
		t.Errorf("import: %d %s", w.Code, w.Body)
	}
	if got := codeOf(t, w); got != 0 {
		t.Errorf("import code=%d, want 0; body=%s", got, w.Body)
	}

	w = doJSON(t, r, "DELETE", "/api/admin/infra/codegen/tables/1", "")
	if w.Code != 200 {
		t.Errorf("delete: %d %s", w.Code, w.Body)
	}
	if got := codeOf(t, w); got != 0 {
		t.Errorf("delete code=%d, want 0; body=%s", got, w.Body)
	}

	w = doJSON(t, r, "POST", "/api/admin/infra/codegen/tables/1/sync", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"added"`) {
		t.Errorf("sync: %d %s", w.Code, w.Body)
	}
	if got := codeOf(t, w); got != 0 {
		t.Errorf("sync code=%d, want 0; body=%s", got, w.Body)
	}
}

// 业务拒绝/不存在 → 400 + 语义码（30007 数据表不存在）；绝不 404（仓库无此惯例）。
func TestErrors_400Never404(t *testing.T) {
	r := newTestRouter(&stubService{importErr: cgmodel.ErrTableNotFound})
	w := doJSON(t, r, "POST", "/api/admin/infra/codegen/import", `{"table_name":"ghost"}`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("import notfound: code=%d, want 400", w.Code)
	}
	if got := codeOf(t, w); got != 30007 {
		t.Errorf("import notfound envelope code=%d, want 30007; body=%s", got, w.Body)
	}

	r2 := newTestRouter(&stubService{})
	w2 := doJSON(t, r2, "GET", "/api/admin/infra/codegen/tables/999", "")
	if w2.Code != http.StatusBadRequest {
		t.Errorf("detail missing: code=%d, want 400", w2.Code)
	}
	if got := codeOf(t, w2); got != 30007 {
		t.Errorf("detail missing envelope code=%d, want 30007; body=%s", got, w2.Body)
	}
}

// 空 table_name → 400 + 10001，且不触达 service。
func TestImport_ValidationBeforeService(t *testing.T) {
	stub := &stubService{}
	r := newTestRouter(stub)
	w := doJSON(t, r, "POST", "/api/admin/infra/codegen/import", `{"table_name":""}`)
	if w.Code != 400 {
		t.Errorf("code=%d", w.Code)
	}
	if got := codeOf(t, w); got != 10001 {
		t.Errorf("envelope code=%d, want 10001; body=%s", got, w.Body)
	}
	if stub.gotImport != "" {
		t.Error("空表名不应调用 service")
	}
}

// M3：preview 端点（取代 generate 占位）——信封 code 0 + data:[{path,content}]。
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
	if got := codeOf(t, w); got != 0 {
		t.Errorf("envelope code=%d, want 0; body=%s", got, w.Body)
	}
	body := w.Body.String()
	for _, want := range []string{`"data"`, `"path":`, `"content":`, "modules/infra/model/order_item.go", "package model"} {
		if !strings.Contains(body, want) {
			t.Errorf("body 缺 %q: %s", want, body)
		}
	}
}

// 空/缺 table_ids → 400 + 10001，绝不静默当全表（Review Focus #3）。
func TestPreview_EmptyIDs_400(t *testing.T) {
	r := newTestRouter(&stubService{})
	for _, payload := range []string{"", "{}", `{"table_ids":[]}`} {
		w := doJSON(t, r, "POST", "/api/admin/infra/codegen/preview", payload)
		if w.Code != 400 {
			t.Errorf("payload=%q code=%d body=%s", payload, w.Code, w.Body)
		}
		if got := codeOf(t, w); got != 10001 {
			t.Errorf("payload=%q envelope code=%d, want 10001; body=%s", payload, got, w.Body)
		}
	}
	w := doJSON(t, r, "POST", "/api/admin/infra/codegen/preview", `{"table_ids":[1]}`)
	if w.Code != 200 { // stub 空 files → data:[]（合法信封，不算错）
		t.Errorf("合法空渲染 code=%d body=%s", w.Code, w.Body)
	}
	if got := codeOf(t, w); got != 0 {
		t.Errorf("合法空渲染 envelope code=%d, want 0; body=%s", got, w.Body)
	}
}

// download：zip 流 + 两个头；业务错误 → 400 + 语义码（Review Focus #4）。
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

	// 与真实 generate() 聚合一致：not found 以 %w 包装上抛
	errStub := newTestRouter(&stubService{downloadErr: fmt.Errorf("table 9: %w", gorm.ErrRecordNotFound)})
	w = doJSON(t, errStub, "POST", "/api/admin/infra/codegen/download", `{"table_ids":[9]}`)
	if w.Code != 400 {
		t.Errorf("download err code=%d body=%s", w.Code, w.Body)
	}
	if got := codeOf(t, w); got != 30007 {
		t.Errorf("download err envelope code=%d, want 30007; body=%s", got, w.Body)
	}
}

// db-tables：导入弹窗下拉数据源，信封 code 0 + data 字符串数组；
// 元数据不可达（读库类）→ 500 + code 50000，绝不 404。
func TestDBTables_Envelope(t *testing.T) {
	r := newTestRouter(&stubService{})
	w := doJSON(t, r, "GET", "/api/admin/infra/codegen/db-tables", "")
	if w.Code != 200 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body)
	}
	if got := codeOf(t, w); got != 0 {
		t.Errorf("envelope code=%d, want 0; body=%s", got, w.Body)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"data"`) || !strings.Contains(body, "orders") {
		t.Errorf("body=%s", body)
	}
}

func TestDBTables_MetadataDown_500(t *testing.T) {
	r := newTestRouter(&stubService{dbTablesErr: errors.New("metadata unreachable")})
	w := doJSON(t, r, "GET", "/api/admin/infra/codegen/db-tables", "")
	if w.Code != http.StatusInternalServerError {
		t.Errorf("code=%d body=%s", w.Code, w.Body)
	}
	if got := codeOf(t, w); got != 50000 {
		t.Errorf("envelope code=%d, want 50000; body=%s", got, w.Body)
	}
}
