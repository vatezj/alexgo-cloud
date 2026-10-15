package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/system/oauth2/model"
	"alexGo-cloud/modules/system/oauth2/repository"
	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
)

func newClientRouter(t *testing.T) (*gin.Engine, *oauth2svc.ClientService) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Oauth2Client{}); err != nil {
		t.Fatal(err)
	}
	svc := oauth2svc.NewClientService(repository.NewClientRepository(db))
	c := NewClientController(svc)
	r := gin.New()
	g := r.Group("/api/admin/oauth2")
	g.GET("/clients", c.List)
	g.POST("/clients", c.Create)
	g.PUT("/clients/:id", c.Update)
	g.DELETE("/clients/:id", c.Delete)
	return r, svc
}

func doJSON(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// codeOf 解析统一信封的 code 字段（resp.OK/Fail/Page 出站；旧 {"status":"ok"}/{"error":...}
// 断言迁移为查 code——0 成功、10001 参数）。
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

// clientListBody 统一分页信封 data:{list,total,page,size}（原 {"data":items,"total":total}）。
type clientListBody struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		List  []map[string]any `json:"list"`
		Total int64            `json:"total"`
		Page  int              `json:"page"`
		Size  int              `json:"size"`
	} `json:"data"`
}

// List 信封 data:{list,total,page,size}；secret 恒不回显（json:"-"）。
func TestClients_ListEnvelope(t *testing.T) {
	r, svc := newClientRouter(t)
	for _, id := range []string{"c1", "c2"} {
		if err := svc.Create(context.Background(), &model.Oauth2Client{
			ClientID: id, GrantTypes: "password,refresh_token",
		}); err != nil {
			t.Fatal(err)
		}
	}
	w := doJSON(r, http.MethodGet, "/api/admin/oauth2/clients?page=1&size=20", "")
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var body clientListBody
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != 0 || body.Msg != "ok" {
		t.Errorf("code/msg = %d/%q, want 0/ok; body=%s", body.Code, body.Msg, w.Body.String())
	}
	if len(body.Data.List) != 2 || body.Data.Total != 2 ||
		body.Data.Page != 1 || body.Data.Size != 20 {
		t.Errorf("list=%d total=%d page=%d size=%d, want 2/2/1/20",
			len(body.Data.List), body.Data.Total, body.Data.Page, body.Data.Size)
	}
	if strings.Contains(w.Body.String(), "client_secret") {
		t.Errorf("列表不得出现 client_secret 字段: %s", w.Body.String())
	}
}

// Create 坏 body → 400（信封 code 精确 10001，非 RFC）。
func TestClients_Create_400BadBody(t *testing.T) {
	r, _ := newClientRouter(t)
	w := doJSON(r, http.MethodPost, "/api/admin/oauth2/clients", "not-json")
	if w.Code != http.StatusBadRequest {
		t.Errorf("code=%d body=%s", w.Code, w.Body.String())
	}
	if got := codeOf(t, w); got != 10001 {
		t.Errorf("code = %d, want 10001; body=%s", got, w.Body.String())
	}
}

// 裁决 2：明文 secret 创建时一次性回显，此后列表绝不出现。
func TestClients_Create_OK_SecretEchoOnce(t *testing.T) {
	r, _ := newClientRouter(t)
	w := doJSON(r, http.MethodPost, "/api/admin/oauth2/clients",
		`{"client_id":"c1","client_secret":"plain-1","grant_types":"password,refresh_token"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if got := codeOf(t, w); got != 0 {
		t.Errorf("code = %d, want 0; body=%s", got, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"client_secret":"plain-1"`) {
		t.Errorf("创建响应须一次性回显明文 secret: %s", w.Body.String())
	}
	w = doJSON(r, http.MethodGet, "/api/admin/oauth2/clients", "")
	if strings.Contains(w.Body.String(), "plain-1") {
		t.Errorf("列表绝不回显明文 secret: %s", w.Body.String())
	}
}

// Delete → 信封 code 0（原 {"status":"ok"}），随后列表归零。
func TestClients_Delete_OK(t *testing.T) {
	r, svc := newClientRouter(t)
	if err := svc.Create(context.Background(), &model.Oauth2Client{
		ClientID: "gone", GrantTypes: "password",
	}); err != nil {
		t.Fatal(err)
	}
	w := doJSON(r, http.MethodGet, "/api/admin/oauth2/clients", "")
	var list clientListBody
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Data.List) != 1 {
		t.Fatalf("seed list: %s", w.Body.String())
	}
	seedID, _ := list.Data.List[0]["id"].(float64)
	w = doJSON(r, http.MethodDelete,
		"/api/admin/oauth2/clients/"+strconv.FormatUint(uint64(seedID), 10), "")
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if got := codeOf(t, w); got != 0 {
		t.Fatalf("delete code = %d, want 0; body=%s", got, w.Body.String())
	}
	w = doJSON(r, http.MethodGet, "/api/admin/oauth2/clients", "")
	var after clientListBody
	if err := json.Unmarshal(w.Body.Bytes(), &after); err != nil {
		t.Fatal(err)
	}
	if after.Data.Total != 0 || len(after.Data.List) != 0 {
		t.Errorf("delete 后 total/list = %d/%d, want 0/0; body=%s",
			after.Data.Total, len(after.Data.List), w.Body.String())
	}
}
