package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/fishgame/model"
	"alexGo-cloud/modules/fishgame/repository"
	"alexGo-cloud/pkg/tenant/gormplugin"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Player{}, &model.Session{}); err != nil {
		t.Fatal(err)
	}
	if err := gormplugin.Register(db, gormplugin.Options{ExemptTables: []string{"tenants", "casbin_rule"}}); err != nil {
		t.Fatal(err)
	}
	return db
}

// seedPlayerSession 落一个 player + 未过期 session，返回 token。
func seedPlayerSession(t *testing.T, db *gorm.DB, expire time.Time) string {
	t.Helper()
	p, err := repository.NewPlayerRepository(db).UpsertByOpenID(t.Context(), "openid-mw", "测试", "")
	if err != nil {
		t.Fatal(err)
	}
	s := &model.Session{TenantID: 0, Token: "tok-valid", PlayerID: p.ID, ExpireAt: expire}
	if err := repository.NewSessionRepository(db).Create(t.Context(), s); err != nil {
		t.Fatal(err)
	}
	return s.Token
}

func newMWRouter(db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected",
		SessionAuth(repository.NewPlayerRepository(db), repository.NewSessionRepository(db)),
		func(c *gin.Context) {
			p, ok := c.Get("fishgame_player")
			if !ok {
				c.Status(http.StatusInternalServerError)
				return
			}
			c.JSON(http.StatusOK, gin.H{"player": p})
		},
	)
	return r
}

func TestSessionAuth_MissingHeader_401(t *testing.T) {
	db := newTestDB(t)
	r := newMWRouter(db)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/protected", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "unauthorized" {
		t.Fatalf("body = %v", body)
	}
}

func TestSessionAuth_Valid_SessionSetsPlayer(t *testing.T) {
	db := newTestDB(t)
	token := seedPlayerSession(t, db, time.Now().Add(24*time.Hour))
	r := newMWRouter(db)
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("X-Session", token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
}

func TestSessionAuth_Expired_401(t *testing.T) {
	db := newTestDB(t)
	_ = seedPlayerSession(t, db, time.Now().Add(-time.Hour))
	r := newMWRouter(db)
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("X-Session", "tok-valid")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", w.Code)
	}
}
