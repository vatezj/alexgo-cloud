package starfuyu

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/starfuyu/controller/admin"
	"alexGo-cloud/modules/starfuyu/controller/app"
	"alexGo-cloud/modules/starfuyu/model"
	"alexGo-cloud/modules/starfuyu/repository"
	"alexGo-cloud/modules/starfuyu/service"
	"alexGo-cloud/pkg/config"
	errs "alexGo-cloud/pkg/errors"
	mw "alexGo-cloud/pkg/middleware"
	"alexGo-cloud/pkg/storage"
	"alexGo-cloud/pkg/tenant/gormplugin"
	"alexGo-cloud/pkg/token"
)

// ---- 外部依赖测试替身（DeepSeek / token 校验在这些用例中不触达）----

// stubFetcher service.FortuneFetcher 替身（抓取不在整模块路由用例内触发）。
type stubFetcher struct{}

func (stubFetcher) FetchFortunes(context.Context, string) ([]*model.Fortune, *model.DailyHoroscope, error) {
	return nil, nil, errors.New("stub: fetcher 不应在路由用例中被调用")
}

// stubCalc service.BaziCalculator 替身。
type stubCalc struct{}

func (stubCalc) Calculate(context.Context, string, string, string) (*model.BaziInfo, error) {
	return nil, errors.New("stub: calculator 不应在路由用例中被调用")
}

// stubInterp service.SignInterpreter 替身。
type stubInterp struct{}

func (stubInterp) SignContent(context.Context, int) (string, error) {
	return "", errors.New("stub: interpreter 不应在路由用例中被调用")
}

func (stubInterp) Interpret(context.Context, *model.BaziInfo, string, string, string) (*service.DivinationResult, error) {
	return nil, errors.New("stub: interpreter 不应在路由用例中被调用")
}

// stubValidator token.Validator 替身（app 段 401 走 claims 空 token 短路，不进 Validate）。
type stubValidator struct{}

func (stubValidator) Validate(context.Context, string) (*token.Claims, error) {
	return nil, errors.New("stub: bad token")
}

// envelope 统一信封断言的最小形状 {code, msg, data}。
type envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// newModuleTestRouter 全装配路由（照 fishgame module_test：sqlite 内存 + 真服务 +
// 外部依赖 stub）。返回 router 与 storage driver（public image 用例要预置文件）。
// withAuth 手工挂全局 auth 中间件（mw.NewAuthMiddleware）——仅 admin 401 用例需要；
// 开关关闭用例必须不挂（中间件在 noRoute 链上也会执行，会把 404 变 401）。
func newModuleTestRouter(t *testing.T, cfg *config.Config, withAuth bool) (*gin.Engine, storage.Driver) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&model.Profile{}, &model.Fortune{}, &model.Bazi{},
		&model.Divination{}, &model.RequestLimit{}, &model.SignCache{},
	); err != nil {
		t.Fatal(err)
	}
	if err := gormplugin.Register(db, gormplugin.Options{ExemptTables: []string{"tenants", "casbin_rule"}}); err != nil {
		t.Fatal(err)
	}
	driver, err := storage.NewLocalDriver(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	fortuneRepo := repository.NewFortuneRepository(db)
	fortunes := service.NewFortuneService(fortuneRepo, repository.NewDailyRepository(db), stubFetcher{})
	images := service.NewImageService(fortunes, driver, cfg)
	mp := service.NewWeChatMPClient("", "")
	articles := service.NewArticleService(fortunes, images, mp, driver, cfg)
	tasks := service.NewTaskRunner(fortunes, images)
	limitRepo := repository.NewRequestLimitRepository(db)
	limitSvc := service.NewLimitService(limitRepo, cfg)
	profiles := service.NewProfileService(repository.NewProfileRepository(db))
	bazis := service.NewBaziService(repository.NewBaziRepository(db), limitSvc, stubCalc{})
	divs := service.NewDivinationService(
		repository.NewSignCacheRepository(db), repository.NewDivinationRepository(db),
		repository.NewBaziRepository(db), limitSvc, stubInterp{})

	m := NewModule(ModuleParams{
		Cfg:               cfg,
		ProfileAppCtrl:    app.NewAppProfileController(),
		BaziAppCtrl:       app.NewAppBaziController(),
		DivinationAppCtrl: app.NewAppDivinationController(),
		FortuneAppCtrl:    app.NewAppFortuneController(),
		LimitAppCtrl:      app.NewAppLimitController(),
		AppCtrl:           app.NewAppController(fortunes, profiles, bazis, divs, images, stubValidator{}),
		AdminCtrl:         admin.NewAdminController(fortunes, tasks, articles, limitRepo),
		PublicCtrl:        admin.NewPublicController(driver),
	})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	if withAuth {
		r.Use(mw.NewAuthMiddleware(mw.AuthDeps{}))
	}
	m.RegisterRoutes(r.Group("/api"))
	return r, driver
}

func doReq(r *gin.Engine, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(method, path, nil))
	return w
}

// cfgOn / cfgOff 模块开关两种取值。
func cfgOn() *config.Config {
	return &config.Config{Modules: map[string]bool{"starfuyu": true}}
}

func cfgOff() *config.Config {
	return &config.Config{Modules: map[string]bool{"starfuyu": false}}
}

// TestRegisterRoutes_ModuleOff_All404 brief Step 1：switch off → 三段（app/admin/public）
// 全 404（不挂 auth 中间件，避免中间件在 noRoute 链上把 404 变 401）。
func TestRegisterRoutes_ModuleOff_All404(t *testing.T) {
	r, _ := newModuleTestRouter(t, cfgOff(), false)
	for _, path := range []string{
		"/api/app/starfuyu/fortune/today",
		"/api/admin/starfuyu/fortunes",
		"/api/admin/starfuyu/limits",
		"/api/public/starfuyu/image/fortune/2026-11-12/aries.jpg",
	} {
		w := doReq(r, http.MethodGet, path)
		if w.Code != http.StatusNotFound {
			t.Fatalf("switch off %s 应 404, got %d body=%s", path, w.Code, w.Body.String())
		}
	}
}

// TestRegisterRoutes_On_AppGate401 switch on → app 段无 token → 401 + 20001
// （handler 内 claims 自鉴权，不依赖全局中间件）。
func TestRegisterRoutes_On_AppGate401(t *testing.T) {
	r, _ := newModuleTestRouter(t, cfgOn(), false)
	w := doReq(r, http.MethodGet, "/api/app/starfuyu/fortune/today")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("app 段无 token 应 401, got %d body=%s", w.Code, w.Body.String())
	}
	var env envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil || env.Code != int(errs.CodeUnauthorized) {
		t.Fatalf("应 401 + code 20001: err=%v env=%+v body=%s", err, env, w.Body.String())
	}
}

// TestRegisterRoutes_On_AdminRegistered200 switch on → admin 段路由已注册：
// 不挂 auth 中间件时 handler 直达 → 200（空分页信封），证非 404。
func TestRegisterRoutes_On_AdminRegistered200(t *testing.T) {
	r, _ := newModuleTestRouter(t, cfgOn(), false)
	w := doReq(r, http.MethodGet, "/api/admin/starfuyu/fortunes")
	if w.Code != http.StatusOK {
		t.Fatalf("admin 段已注册应 200, got %d body=%s", w.Code, w.Body.String())
	}
	var env envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil || env.Code != 0 {
		t.Fatalf("应信封 code=0: err=%v body=%s", err, w.Body.String())
	}
}

// TestRegisterRoutes_On_AdminNoToken401 手工挂全局 auth 中间件（mw.Auth 同款）：
// admin 段无 token → 401 + 20001（路由已注册经 200 用例另证，此处证鉴权门形状）。
func TestRegisterRoutes_On_AdminNoToken401(t *testing.T) {
	r, _ := newModuleTestRouter(t, cfgOn(), true)
	w := doReq(r, http.MethodGet, "/api/admin/starfuyu/fortunes")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("admin 段无 token 应 401, got %d body=%s", w.Code, w.Body.String())
	}
	var env envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil || env.Code != int(errs.CodeUnauthorized) {
		t.Fatalf("应 401 + code 20001: err=%v body=%s", err, w.Body.String())
	}
}

// TestRegisterRoutes_On_PublicImage200 public 段免鉴权：预置 storage 文件 → 200 inline 图片。
func TestRegisterRoutes_On_PublicImage200(t *testing.T) {
	r, driver := newModuleTestRouter(t, cfgOn(), false)
	if err := driver.Save(t.Context(), "fortune/2026-11-12/aries.jpg", strings.NewReader("JPEGDATA")); err != nil {
		t.Fatal(err)
	}
	w := doReq(r, http.MethodGet, "/api/public/starfuyu/image/fortune/2026-11-12/aries.jpg")
	if w.Code != http.StatusOK {
		t.Fatalf("public image 应 200, got %d body=%s", w.Code, w.Body.String())
	}
	if w.Body.String() != "JPEGDATA" {
		t.Fatalf("image bytes=%q want JPEGDATA", w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/jpeg" {
		t.Fatalf("Content-Type=%q want image/jpeg", ct)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "inline") {
		t.Fatalf("Content-Disposition=%q want inline", cd)
	}
}

// TestRegisterRoutes_On_PublicImageMiss30010 public 段 miss → 400 + 30010。
func TestRegisterRoutes_On_PublicImageMiss30010(t *testing.T) {
	r, _ := newModuleTestRouter(t, cfgOn(), false)
	w := doReq(r, http.MethodGet, "/api/public/starfuyu/image/fortune/2026-11-12/nope.jpg")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("miss 应 400, got %d body=%s", w.Code, w.Body.String())
	}
	var env envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil || env.Code != int(errs.CodeFileNotFound) {
		t.Fatalf("miss 应 400 + code 30010: err=%v body=%s", err, w.Body.String())
	}
}

// TestRegisterRoutes_On_PublicImageTraversal30010 路径穿越（.. 段 / 非 fortune 前缀）
// → 400 + 30010（不泄露存在性；驱动 resolve 双保险之外的 controller 层前缀校验）。
func TestRegisterRoutes_On_PublicImageTraversal30010(t *testing.T) {
	r, driver := newModuleTestRouter(t, cfgOn(), false)
	// 驱动根下放一个越权目标：若前缀校验失效，穿越读会命中它。
	if err := driver.Save(t.Context(), "secret.txt", strings.NewReader("TOPSECRET")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"/api/public/starfuyu/image/../../secret.txt",
		"/api/public/starfuyu/image/secret.txt",
	} {
		w := doReq(r, http.MethodGet, path)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("traversal %s 应 400, got %d body=%s", path, w.Code, w.Body.String())
		}
		var env envelope
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil || env.Code != int(errs.CodeFileNotFound) {
			t.Fatalf("traversal %s 应 400 + 30010: err=%v body=%s", path, err, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "TOPSECRET") {
			t.Fatalf("traversal %s 泄露了文件内容", path)
		}
	}
}
