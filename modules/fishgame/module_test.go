package fishgame

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/fishgame/controller/admin"
	"alexGo-cloud/modules/fishgame/controller/app"
	"alexGo-cloud/modules/fishgame/middleware"
	"alexGo-cloud/modules/fishgame/model"
	"alexGo-cloud/modules/fishgame/repository"
	"alexGo-cloud/modules/fishgame/service"
	"alexGo-cloud/pkg/config"
	mw "alexGo-cloud/pkg/middleware"
	"alexGo-cloud/pkg/storage"
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/tenant/gormplugin"
)

// newModuleTestRouter 全装配路由（照 module.go RegisterRoutes 形状手工拼，
// 覆盖终审 Important#5：login 免会话可达 / 401 裸 JSON / 成功响应无信封）。
func newModuleTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&model.Player{}, &model.Session{}, &model.Fish{}, &model.Dex{},
		&model.TrackEvent{}, &model.Sign{}, &model.Friend{}, &model.Config{}, &model.Share{},
	); err != nil {
		t.Fatal(err)
	}
	if err := gormplugin.Register(db, gormplugin.Options{ExemptTables: []string{"tenants", "casbin_rule"}}); err != nil {
		t.Fatal(err)
	}

	players := repository.NewPlayerRepository(db)
	sessions := repository.NewSessionRepository(db)
	fishes := repository.NewFishRepository(db)
	dex := repository.NewDexRepository(db)
	events := repository.NewTrackEventRepository(db)
	signs := repository.NewSignRepository(db)
	friends := repository.NewFriendRepository(db)
	configs := repository.NewConfigRepository(db)
	shares := repository.NewShareRepository(db)
	driver, err := storage.NewLocalDriver(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	friendSvc := service.NewFriendService(service.NewStoredFriendSource(friends, players), players, friends, dex)
	shareSvc := service.NewShareService(shares, driver, "")
	cfg := &config.Config{}

	m := NewModule(ModuleParams{
		Cfg: cfg,
		AuthCtrl: app.NewAuthController(
			service.NewAuthService(service.NewWechatClient("", ""), players, sessions)),
		DailyCtrl:   app.NewDailyController(service.NewDailyService(configs, fishes, signs)),
		TrackCtrl:   app.NewTrackController(service.NewTrackService(events, fishes, dex)),
		DexCtrl:     app.NewDexController(service.NewDexService(fishes, dex)),
		FriendCtrl:  app.NewFriendController(friendSvc),
		ShareCtrl:   app.NewShareController(shareSvc, cfg),
		PublicCtrl:  app.NewPublicController(shareSvc),
		AdminCtrl:   admin.NewAdminController(service.NewAdminService(fishes, configs, events, players, dex, friendSvc)),
		SessionMW:   middleware.SessionAuth(players, sessions),
	})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	m.RegisterRoutes(r.Group("/api"))
	return r, db
}

// TestRegisterRoutes_LoginReachableWithoutSession 终审 Important#5 / Review Focus#2：
// login 挂在 session 中间件之外——空 body 应到 Login 的 400 invalid_body，
// 而不是被中间件截成 401 unauthorized。
func TestRegisterRoutes_LoginReachableWithoutSession(t *testing.T) {
	r, _ := newModuleTestRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "invalid_body") {
		t.Fatalf("login 应可达（400 invalid_body），got %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "unauthorized") {
		t.Fatalf("login 不应被 session 中间件拦截: %s", w.Body.String())
	}
}

// TestRegisterRoutes_NoSession401BareJSON 终审 Important#5 / Review Focus#1：
// 契约端点缺 session → 401 裸 JSON，顶层无 code 键（信封泄漏即客户端形状校验炸）。
func TestRegisterRoutes_NoSession401BareJSON(t *testing.T) {
	r, _ := newModuleTestRouter(t)
	for _, path := range []string{
		"/api/v1/config/daily", "/api/v1/basket", "/api/v1/dex/table", "/api/v1/friends/rank",
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s 缺 session 应 401, got %d", path, w.Code)
		}
		var m map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
			t.Fatalf("%s 应是 JSON: %v", path, err)
		}
		if _, has := m["code"]; has {
			t.Fatalf("%s 裸 JSON 不应有 code 键: %s", path, w.Body.String())
		}
		if m["error"] != "unauthorized" {
			t.Fatalf("%s 应 {\"error\":\"unauthorized\"}: %s", path, w.Body.String())
		}
	}
}

// TestRegisterRoutes_SuccessNoEnvelope 带真实会话打 /basket：200 成功路径同样无信封。
func TestRegisterRoutes_SuccessNoEnvelope(t *testing.T) {
	r, db := newModuleTestRouter(t)
	p, err := repository.NewPlayerRepository(db).UpsertByOpenID(t.Context(), "asm-o", "装配", "")
	if err != nil {
		t.Fatal(err)
	}
	token, err := service.NewSessionService(repository.NewSessionRepository(db),
		repository.NewPlayerRepository(db)).Issue(t.Context(), p.ID)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/basket?since=0", nil)
	req.Header.Set("X-Session", token)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("带会话 /basket 应 200, got %d %s", w.Code, w.Body.String())
	}
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if _, has := m["code"]; has {
		t.Fatalf("成功响应不应有 code 键: %s", w.Body.String())
	}
}

// TestPinTidZero_OverridesHeader 终审 Important#4：游戏端不变式——ctx tenant 钉回 0，
// X-Tenant-ID 头不得扰动 gormplugin（防同 openid 裂多行）。
func TestPinTidZero_OverridesHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(mw.NewTenantMiddleware(nil))
	r.GET("/x", pinTidZero, func(c *gin.Context) {
		c.String(http.StatusOK, strconv.FormatUint(tenant.TenantIDFromContext(c.Request.Context()), 10))
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Tenant-ID", "5")
	r.ServeHTTP(w, req)
	if w.Body.String() != "0" {
		t.Fatalf("pinTidZero 后 tid 应 0, got %s", w.Body.String())
	}
}
