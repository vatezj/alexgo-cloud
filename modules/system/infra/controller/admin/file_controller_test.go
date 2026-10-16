package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/infra/model"
	"alexGo-cloud/modules/system/infra/service"
	errs "alexGo-cloud/pkg/errors"
)

// fileStub 记录调用、按 case 返回（直接实现 service.FileService）。
type fileStub struct {
	upload      *model.SystemFile
	uploadErr   error
	gotIn       service.UploadInput
	getByKey    *model.SystemFile
	getByKeyErr error
	openRc      io.ReadCloser
	openErr     error
	getByID     *model.SystemFile
	getByIDErr  error
	deleteErr   error
}

func (s *fileStub) Upload(_ context.Context, in service.UploadInput) (*model.SystemFile, error) {
	s.gotIn = in
	return s.upload, s.uploadErr
}
func (s *fileStub) GetByID(context.Context, uint64) (*model.SystemFile, error) {
	return s.getByID, s.getByIDErr
}
func (s *fileStub) GetByKey(context.Context, string) (*model.SystemFile, error) {
	return s.getByKey, s.getByKeyErr
}
func (s *fileStub) Open(context.Context, *model.SystemFile) (io.ReadCloser, error) {
	return s.openRc, s.openErr
}
func (s *fileStub) Delete(context.Context, uint64) error { return s.deleteErr }

// newFileTestRouter 同时挂 admin 与 public 两组——照真实模块的相对前缀。
func newFileTestRouter(stub *fileStub) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	c := NewFileController(stub)
	c.RegisterAdminRoutes(r.Group("/api/admin/infra/file"))
	c.RegisterPublicRoutes(r.Group("/api/public/infra/file"))
	return r
}

func multipartReq(t *testing.T, path string, filename string, content []byte, form map[string]string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatal(err)
	}
	for k, v := range form {
		if err := w.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

// 上传信封形状：code=0，data 五字段；public=true → public_url 带 key；
// public=false → public_url 空串。multipart 表单域（public/biz_type）透传 service。
func TestUpload_EnvelopeShape(t *testing.T) {
	stub := &fileStub{upload: &model.SystemFile{
		ID: 7, Name: "a.jpg", Size: 4, Mime: "image/jpeg",
		Public: true, PublicKey: strings.Repeat("a", 32),
	}}
	r := newFileTestRouter(stub)
	req := multipartReq(t, "/api/admin/infra/file/upload", "a.jpg", []byte("x"), map[string]string{
		"public": "true", "biz_type": "notice",
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body)
	}
	var env struct {
		Code int `json:"code"`
		Data struct {
			ID        uint64 `json:"id"`
			Name      string `json:"name"`
			Size      int64  `json:"size"`
			Mime      string `json:"mime"`
			PublicURL string `json:"public_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Code != 0 || env.Data.ID != 7 || env.Data.PublicURL != "/api/public/infra/file/"+strings.Repeat("a", 32) {
		t.Errorf("envelope = %+v, want code=0 with public_url", env)
	}
	if !stub.gotIn.Public || stub.gotIn.BizType != "notice" || stub.gotIn.Name != "a.jpg" {
		t.Errorf("service input = %+v, want Public=true BizType=notice", stub.gotIn)
	}
	// 缺 file 字段 → 10002
	w2 := httptest.NewRecorder()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("public", "false")
	_ = mw.Close()
	req2 := httptest.NewRequest("POST", "/api/admin/infra/file/upload", &buf)
	req2.Header.Set("Content-Type", mw.FormDataContentType())
	r.ServeHTTP(w2, req2)
	if code, _ := codeOfBody(t, w2); code != 10002 {
		t.Errorf("missing file envelope code = %d, want 10002", code)
	}
	// service 30011 直传
	stubErr := &fileStub{uploadErr: errs.New(errs.CodeFileTypeNotAllowed, nil)}
	r2 := newFileTestRouter(stubErr)
	w3 := httptest.NewRecorder()
	r2.ServeHTTP(w3, multipartReq(t, "/api/admin/infra/file/upload", "a.svg", []byte("x"), nil))
	if code, _ := codeOfBody(t, w3); code != 30011 || w3.Code != 400 {
		t.Errorf("mime reject: http=%d code=%d, want 400/30011", w3.Code, code)
	}
}

// public 读：免鉴权（无 Authorization 头）、流式透传 + nosniff/inline/Cache-Control；
// 私有/不存在统一 30010。
func TestPublicGet_HeadersAndEnvelope(t *testing.T) {
	meta := &model.SystemFile{Name: "照片.jpg", Mime: "image/jpeg", PublicKey: strings.Repeat("b", 32), Public: true}
	stub := &fileStub{
		getByKey: meta,
		openRc:   io.NopCloser(bytes.NewReader([]byte{0xFF, 0xD8, 0xFF})),
	}
	r := newFileTestRouter(stub)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/public/infra/file/"+meta.PublicKey, nil))
	if w.Code != 200 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body)
	}
	if got := w.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Errorf("Content-Type = %q, want image/jpeg (存库探测值，不信扩展名)", got)
	}
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("nosniff missing")
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "inline") {
		t.Errorf("Content-Disposition = %q, want inline", cd)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "public, max-age=3600" {
		t.Errorf("Cache-Control = %q", cc)
	}
	if w.Body.Len() != 3 {
		t.Errorf("body len = %d, want 3 (stream passthrough)", w.Body.Len())
	}

	// private/不存在 → 30010 信封（流不输出）
	stub2 := &fileStub{getByKeyErr: errs.New(errs.CodeFileNotFound, nil)}
	r2 := newFileTestRouter(stub2)
	w2 := httptest.NewRecorder()
	r2.ServeHTTP(w2, httptest.NewRequest("GET", "/api/public/infra/file/"+strings.Repeat("c", 32), nil))
	if code, _ := codeOfBody(t, w2); code != 30010 || w2.Code != 400 {
		t.Errorf("missing/private: http=%d code=%d, want 400/30010", w2.Code, code)
	}
}

// 受控下载：attachment disposition + 流式字节；DELETE 动作信封 data:null。
func TestAdminDownloadAndDelete(t *testing.T) {
	meta := &model.SystemFile{Name: "报告.pdf", Mime: "application/pdf", RelPath: "x"}
	stub := &fileStub{
		getByID: meta,
		openRc:  io.NopCloser(bytes.NewReader([]byte("%PDF-1.4"))),
	}
	r := newFileTestRouter(stub)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/admin/infra/file/5", nil))
	if w.Code != 200 {
		t.Fatalf("download status=%d body=%s", w.Code, w.Body)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment") {
		t.Errorf("Content-Disposition = %q, want attachment", cd)
	}
	if !strings.HasPrefix(w.Body.String(), "%PDF") {
		t.Errorf("body = %q, want pdf bytes", w.Body.String()[:min(4, w.Body.Len())])
	}

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest("DELETE", "/api/admin/infra/file/5", nil))
	if code, data := codeOfBody(t, w2); code != 0 || data != "null" {
		t.Errorf("delete envelope code=%d data=%s, want 0/null", code, data)
	}
}

// codeOfBody 解析信封 code 与 data 原文（controller 测试内联小工具）。
func codeOfBody(t *testing.T, w *httptest.ResponseRecorder) (int, string) {
	t.Helper()
	var env struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	return env.Code, string(env.Data)
}
