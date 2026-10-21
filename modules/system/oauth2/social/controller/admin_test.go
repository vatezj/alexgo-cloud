package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/system/oauth2/social"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/middleware"
	"alexGo-cloud/pkg/tenant"
)

// === 夹具 ===

// stubAppConfiger AppConfiger 测试桩：func 字段由各用例注入，
// 收到的参数落 got* 字段供断言。
type stubAppConfiger struct {
	list   func(ctx context.Context, page, size int, platform string) ([]*social.SocialApp, int64, error)
	create func(ctx context.Context, app *social.SocialApp) error
	update func(ctx context.Context, app *social.SocialApp) error
	del    func(ctx context.Context, id uint64) error

	gotPage     int
	gotSize     int
	gotPlatform string
	gotApp      *social.SocialApp
	gotID       uint64
	gotTenant   uint64
}

func (s *stubAppConfiger) List(ctx context.Context, page, size int, platform string) ([]*social.SocialApp, int64, error) {
	s.gotPage, s.gotSize, s.gotPlatform = page, size, platform
	return s.list(ctx, page, size, platform)
}

func (s *stubAppConfiger) Create(ctx context.Context, app *social.SocialApp) error {
	s.gotApp = app
	s.gotTenant = tenant.TenantIDFromContext(ctx)
	return s.create(ctx, app)
}

func (s *stubAppConfiger) Update(ctx context.Context, app *social.SocialApp) error {
	s.gotApp = app
	s.gotTenant = tenant.TenantIDFromContext(ctx)
	return s.update(ctx, app)
}

func (s *stubAppConfiger) Delete(ctx context.Context, id uint64) error {
	s.gotID = id
	s.gotTenant = tenant.TenantIDFromContext(ctx)
	return s.del(ctx, id)
}

// newAdminRouter 装 admin 路由（路径与 module.go RegisterRoutes 同款），
// 前挂真实 TenantMiddleware（X-Tenant-ID 头解析，与 http.go 链序一致）。
func newAdminRouter(t *testing.T, svc AppConfiger) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	a := NewAdminController(svc)
	r := gin.New()
	r.Use(middleware.NewTenantMiddleware(nil))
	soc := r.Group("/api/admin/social")
	soc.GET("/apps", a.List)
	soc.POST("/apps", a.Create)
	soc.PUT("/apps/:id", a.Update)
	soc.DELETE("/apps/:id", a.Delete)
	return r
}

// doReq 带任意头发请求。
func doReq(r *gin.Engine, method, path, body string, hdr map[string]string) *httptest.ResponseRecorder {
	req := newReq(method, path, body)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	return serve(r, req)
}

// seedApp 直插一条渠道配置（铺 Update/Delete 前置数据，绕过 svc 规则）。
func seedApp(t *testing.T, db *gorm.DB, tid uint64, platform, secret string) *social.SocialApp {
	t.Helper()
	row := &social.SocialApp{
		TenantID: tid, Platform: platform,
		AppID: "app-" + platform, Secret: secret, Status: 1,
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	return row
}

// loadApp 按 id 读回库中行。
func loadApp(t *testing.T, db *gorm.DB, id uint64) *social.SocialApp {
	t.Helper()
	var row social.SocialApp
	if err := db.First(&row, id).Error; err != nil {
		t.Fatalf("load app %d: %v", id, err)
	}
	return &row
}

// === stub 版（控制器行为） ===

// TestAdminList_ReturnsJSON_NoSecretInBody 分页信封 + platform 透传 +
// secret 恒不回显（Review Focus #3：stub 行带明文 Secret，body 不得出现其值）。
func TestAdminList_ReturnsJSON_NoSecretInBody(t *testing.T) {
	stub := &stubAppConfiger{list: func(_ context.Context, _, _ int, _ string) ([]*social.SocialApp, int64, error) {
		return []*social.SocialApp{{
			ID: 1, TenantID: 3, Platform: social.PlatformWechatMP,
			AppID: "wx123", Secret: "sk-live-TOPSECRET", Status: 1,
		}}, 1, nil
	}}
	r := newAdminRouter(t, stub)
	w := doJSON(r, http.MethodGet, "/api/admin/social/apps?page=1&size=10&platform=wechat_mp", "")

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200; body=%s", w.Code, w.Body.String())
	}
	e := decode(t, w)
	if e.Code != 0 {
		t.Errorf("envelope code=%d, want 0; body=%s", e.Code, w.Body.String())
	}
	if stub.gotPage != 1 || stub.gotSize != 10 || stub.gotPlatform != "wechat_mp" {
		t.Errorf("page/size/platform = %d/%d/%q, want 1/10/wechat_mp",
			stub.gotPage, stub.gotSize, stub.gotPlatform)
	}
	body := w.Body.String()
	if contains := strings.Contains(body, "sk-live-TOPSECRET"); contains {
		t.Errorf("列表响应泄漏 secret 值: %s", body)
	}
	if strings.Contains(body, `"secret"`) {
		t.Errorf("列表响应不得出现 secret 字段名（json:\"-\"): %s", body)
	}
}

// TestAdminCreate_BodyNeverEchoesSecret 主控裁决：请求体带 secret 须经
// 手动写回进入 model（否则 json:\"-\" 使 Create 恒 10002）；响应侧恒不回显。
func TestAdminCreate_BodyNeverEchoesSecret(t *testing.T) {
	stub := &stubAppConfiger{create: func(_ context.Context, _ *social.SocialApp) error { return nil }}
	r := newAdminRouter(t, stub)
	w := doJSON(r, http.MethodPost, "/api/admin/social/apps",
		`{"platform":"wechat_mp","appid":"wx123","secret":"sk-NEW-PLAIN","status":1}`)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200; body=%s", w.Code, w.Body.String())
	}
	if e := decode(t, w); e.Code != 0 {
		t.Errorf("envelope code=%d, want 0; body=%s", e.Code, w.Body.String())
	}
	if stub.gotApp == nil {
		t.Fatal("svc.Create 未被调用")
	}
	if stub.gotApp.Secret != "sk-NEW-PLAIN" {
		t.Errorf("model.Secret=%q, want sk-NEW-PLAIN（body secret 必须手动写回 model，否则 service 恒 10002）",
			stub.gotApp.Secret)
	}
	if stub.gotApp.Platform != social.PlatformWechatMP || stub.gotApp.AppID != "wx123" || stub.gotApp.Status != 1 {
		t.Errorf("app = %+v, want platform=wechat_mp appid=wx123 status=1", stub.gotApp)
	}
	body := w.Body.String()
	if strings.Contains(body, "sk-NEW-PLAIN") {
		t.Errorf("Create 响应不得回显 secret: %s", body)
	}
	if strings.Contains(body, `"secret"`) {
		t.Errorf("Create 响应不得出现 secret 字段名: %s", body)
	}
}

// TestAdminCreate_PassesTenantFromCtx 请求 ctx 带租户（X-Tenant-ID=42 经
// TenantMiddleware 解析）→ svc.Create 收到同一 ctx（T8 在 service 内注入
// app.TenantID，controller 不碰 TenantID）。
func TestAdminCreate_PassesTenantFromCtx(t *testing.T) {
	stub := &stubAppConfiger{create: func(_ context.Context, _ *social.SocialApp) error { return nil }}
	r := newAdminRouter(t, stub)
	w := doReq(r, http.MethodPost, "/api/admin/social/apps",
		`{"platform":"wechat_mp","appid":"wx123","secret":"s1","status":1}`,
		map[string]string{"X-Tenant-ID": "42"})

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200; body=%s", w.Code, w.Body.String())
	}
	if stub.gotTenant != 42 {
		t.Errorf("svc.Create ctx tenant=%d, want 42", stub.gotTenant)
	}
	// 裁决：service 注租户，controller 只搬字段——TenantID 不得由 controller 乱写。
	if stub.gotApp.TenantID != 0 {
		t.Errorf("model.TenantID=%d, want 0（租户注入归 service，controller 不得自行赋值）", stub.gotApp.TenantID)
	}
}

// TestAdminUpdate_IDFromPath PUT /apps/7 → app.ID==7（路径 id 优先），
// body 带 secret 即轮换值（主控裁决：Update 必须能经 API 轮换 secret）。
func TestAdminUpdate_IDFromPath(t *testing.T) {
	stub := &stubAppConfiger{update: func(_ context.Context, _ *social.SocialApp) error { return nil }}
	r := newAdminRouter(t, stub)
	w := doJSON(r, http.MethodPut, "/api/admin/social/apps/7",
		`{"id":99,"platform":"wechat_web","appid":"wx77","secret":"sk-ROTATED","status":1}`)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200; body=%s", w.Code, w.Body.String())
	}
	if e := decode(t, w); e.Code != 0 {
		t.Errorf("envelope code=%d, want 0; body=%s", e.Code, w.Body.String())
	}
	if stub.gotApp == nil {
		t.Fatal("svc.Update 未被调用")
	}
	if stub.gotApp.ID != 7 {
		t.Errorf("app.ID=%d, want 7（路径参数，body id 忽略）", stub.gotApp.ID)
	}
	if stub.gotApp.Secret != "sk-ROTATED" {
		t.Errorf("app.Secret=%q, want sk-ROTATED（轮换值必须进 model）", stub.gotApp.Secret)
	}
}

// TestAdminUpdate_InvalidID_10001 PUT /apps/abc（非法 id）→ 400 + 10001。
func TestAdminUpdate_InvalidID_10001(t *testing.T) {
	stub := &stubAppConfiger{update: func(_ context.Context, _ *social.SocialApp) error {
		t.Error("非法 id 不应调用 svc.Update")
		return nil
	}}
	r := newAdminRouter(t, stub)
	w := doJSON(r, http.MethodPut, "/api/admin/social/apps/abc", `{"platform":"wechat_mp"}`)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400; body=%s", w.Code, w.Body.String())
	}
	if got := decode(t, w).Code; got != int(errs.CodeParamInvalid) {
		t.Errorf("code=%d, want 10001", got)
	}
}

// TestAdminDelete_IDFromPath DELETE /apps/7 → svc.Delete 收到 id=7。
func TestAdminDelete_IDFromPath(t *testing.T) {
	stub := &stubAppConfiger{del: func(_ context.Context, _ uint64) error { return nil }}
	r := newAdminRouter(t, stub)
	w := doJSON(r, http.MethodDelete, "/api/admin/social/apps/7", "")

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200; body=%s", w.Code, w.Body.String())
	}
	if e := decode(t, w); e.Code != 0 {
		t.Errorf("envelope code=%d, want 0; body=%s", e.Code, w.Body.String())
	}
	if stub.gotID != 7 {
		t.Errorf("svc.Delete id=%d, want 7", stub.gotID)
	}
}

// === 真 service + sqlite（主控裁决：请求体 secret 必须真落库 / 真轮换） ===

// newDBAdminRouter 真链路：sqlite :memory: + 真 AppConfigService（真仓储 +
// 真适配器注册表 Exists）+ 同款路由与 TenantMiddleware。
func newDBAdminRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&social.SocialApp{}); err != nil {
		t.Fatal(err)
	}
	repo := social.NewSocialRepository(db)
	svc := social.NewAppConfigService(social.AppConfigDeps{Repo: repo, Adapters: social.NewAdapterRegistry()})
	return newAdminRouter(t, svc), db
}

// TestAdminCreate_SecretLandsInDB POST body 带 secret → 落库明文（经
// controller 手动写回 → service 规则链 → repo），且响应不回显。
func TestAdminCreate_SecretLandsInDB(t *testing.T) {
	r, db := newDBAdminRouter(t)
	w := doReq(r, http.MethodPost, "/api/admin/social/apps",
		`{"platform":"wechat_mp","appid":"wx-1","secret":"sk-DB-PLAIN","status":1}`,
		map[string]string{"X-Tenant-ID": "42"})

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200; body=%s", w.Code, w.Body.String())
	}
	if e := decode(t, w); e.Code != 0 {
		t.Fatalf("envelope code=%d, want 0; body=%s", e.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "sk-DB-PLAIN") {
		t.Errorf("Create 响应不得回显 secret: %s", w.Body.String())
	}
	var row social.SocialApp
	if err := db.Where("platform = ?", social.PlatformWechatMP).First(&row).Error; err != nil {
		t.Fatalf("load created row: %v", err)
	}
	if row.Secret != "sk-DB-PLAIN" {
		t.Errorf("落库 secret=%q, want sk-DB-PLAIN（请求体 secret 必须落库）", row.Secret)
	}
	if row.AppID != "wx-1" || row.Status != 1 {
		t.Errorf("appid/status = %q/%d, want wx-1/1", row.AppID, row.Status)
	}
	if row.TenantID != 42 {
		t.Errorf("tenant_id=%d, want 42（T8 service 内按 ctx 注入）", row.TenantID)
	}
}

// TestAdminUpdate_SecretRotatesInDB PUT 带 secret → 库中轮换为新值；
// 再次 PUT 不带 secret → 保留（不改机密），仍为轮换后的值。
func TestAdminUpdate_SecretRotatesInDB(t *testing.T) {
	r, db := newDBAdminRouter(t)
	seeded := seedApp(t, db, 42, social.PlatformWechatMP, "sk-OLD")
	hdr := map[string]string{"X-Tenant-ID": "42"}
	path := "/api/admin/social/apps/" + strconv.FormatUint(seeded.ID, 10)

	w := doReq(r, http.MethodPut, path,
		`{"platform":"wechat_mp","appid":"wx-1","secret":"sk-ROT-DB","status":1}`, hdr)
	if w.Code != http.StatusOK {
		t.Fatalf("rotate: status=%d, want 200; body=%s", w.Code, w.Body.String())
	}
	if e := decode(t, w); e.Code != 0 {
		t.Fatalf("rotate: envelope code=%d, want 0; body=%s", e.Code, w.Body.String())
	}
	if got := loadApp(t, db, seeded.ID); got.Secret != "sk-ROT-DB" {
		t.Errorf("轮换后落库 secret=%q, want sk-ROT-DB", got.Secret)
	}
	if strings.Contains(w.Body.String(), "sk-ROT-DB") {
		t.Errorf("Update 响应不得回显 secret: %s", w.Body.String())
	}

	// 不带 secret = 留空保留旧值（编辑表单留空不改机密），经 API 路径同样成立。
	w = doReq(r, http.MethodPut, path,
		`{"platform":"wechat_mp","appid":"wx-1","status":1}`, hdr)
	if w.Code != http.StatusOK {
		t.Fatalf("keep: status=%d, want 200; body=%s", w.Code, w.Body.String())
	}
	if got := loadApp(t, db, seeded.ID); got.Secret != "sk-ROT-DB" {
		t.Errorf("留空更新后 secret=%q, want 保持 sk-ROT-DB", got.Secret)
	}
}
