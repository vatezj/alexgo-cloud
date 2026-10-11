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

	"alexGo-cloud/modules/oauth2/model"
	"alexGo-cloud/modules/oauth2/repository"
	oauth2svc "alexGo-cloud/modules/oauth2/service"
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

// List 信封 {data,total}；secret 恒不回显（json:"-"）。
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
	var body struct {
		Data  []map[string]any `json:"data"`
		Total int64            `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data) != 2 || body.Total != 2 {
		t.Errorf("data=%d total=%d, want 2/2", len(body.Data), body.Total)
	}
	if strings.Contains(w.Body.String(), "client_secret") {
		t.Errorf("列表不得出现 client_secret 字段: %s", w.Body.String())
	}
}

// Create 坏 body → 400 {"error"}（admin 信封，非 RFC）。
func TestClients_Create_400BadBody(t *testing.T) {
	r, _ := newClientRouter(t)
	w := doJSON(r, http.MethodPost, "/api/admin/oauth2/clients", "not-json")
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), `"error"`) {
		t.Errorf("code=%d body=%s", w.Code, w.Body.String())
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
	if !strings.Contains(w.Body.String(), `"client_secret":"plain-1"`) {
		t.Errorf("创建响应须一次性回显明文 secret: %s", w.Body.String())
	}
	w = doJSON(r, http.MethodGet, "/api/admin/oauth2/clients", "")
	if strings.Contains(w.Body.String(), "plain-1") {
		t.Errorf("列表绝不回显明文 secret: %s", w.Body.String())
	}
}

// Delete → {"status":"ok"}，随后列表归零。
func TestClients_Delete_OK(t *testing.T) {
	r, svc := newClientRouter(t)
	if err := svc.Create(context.Background(), &model.Oauth2Client{
		ClientID: "gone", GrantTypes: "password",
	}); err != nil {
		t.Fatal(err)
	}
	w := doJSON(r, http.MethodGet, "/api/admin/oauth2/clients", "")
	var list struct {
		Data []struct {
			ID uint64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Data) != 1 {
		t.Fatalf("seed list: %s", w.Body.String())
	}
	w = doJSON(r, http.MethodDelete,
		"/api/admin/oauth2/clients/"+strconv.FormatUint(list.Data[0].ID, 10), "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"status":"ok"`) {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	w = doJSON(r, http.MethodGet, "/api/admin/oauth2/clients", "")
	if !strings.Contains(w.Body.String(), `"total":0`) {
		t.Errorf("delete 后 total: %s", w.Body.String())
	}
}
