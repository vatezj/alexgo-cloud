//go:build integration

package oauth2_test

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"alexGo-cloud/modules/member"
	membermodel "alexGo-cloud/modules/member/model"
	memberrepo "alexGo-cloud/modules/member/repository"
	membersvc "alexGo-cloud/modules/member/service"
	"alexGo-cloud/modules/system"
	sysmodel "alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/oauth2"
	"alexGo-cloud/modules/system/oauth2/controller"
	oauth2model "alexGo-cloud/modules/system/oauth2/model"
	oauth2repo "alexGo-cloud/modules/system/oauth2/repository"
	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
	sysrepo "alexGo-cloud/modules/system/repository"
	sysvc "alexGo-cloud/modules/system/service"
	"alexGo-cloud/pkg/auth"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/middleware"
	"alexGo-cloud/pkg/migrate"
	"alexGo-cloud/pkg/token"
)

// tokenResp /oauth2/token 200 响应形状（service.TokenResponse 的 JSON 视图）。
type tokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
}

// TestE2E_PasswordGrant_AdminAPI_200：system 账号 password grant 换 token 后
// 可访问受 Casbin 保护的 admin 路由（Review Focus 3 正例）。
// allowPing 必须在拿 token 之后——登录内部 EnsureUserRolePolicy 会 loadPolicy
// 重载，事前加的内存策略会被洗掉（裁决 4）。
func TestE2E_PasswordGrant_AdminAPI_200(t *testing.T) {
	r, e := newE2E(t)
	w := postJSON(t, r, "/api/oauth2/token", map[string]string{
		"grant_type":   "password",
		"client_id":    "alexgo-admin",
		"username":     "e2eadmin",
		"password":     "pw123456",
		"account_type": "system",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("password grant 200, got %d body=%s", w.Code, w.Body.String())
	}
	var tok tokenResp
	if err := json.Unmarshal(w.Body.Bytes(), &tok); err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken == "" || tok.TokenType != "Bearer" {
		t.Fatalf("access_token/token_type 异常: %+v", tok)
	}
	allowPing(t, e)
	ping := doReq(t, r, http.MethodGet, "/api/admin/system/e2e/ping", tok.AccessToken)
	if ping.Code != http.StatusOK || !strings.Contains(ping.Body.String(), "pong") {
		t.Fatalf("admin ping 200, got %d body=%s", ping.Code, ping.Body.String())
	}
}

// TestE2E_MemberToken_AdminAPI_403：member 账号拿到合法 token，
// 但 userType=2 无 admin 策略 → 访问 admin 路由 403（Review Focus 3 跨账号类型冒用）。
func TestE2E_MemberToken_AdminAPI_403(t *testing.T) {
	r, _ := newE2E(t)
	w := postJSON(t, r, "/api/oauth2/token", map[string]string{
		"grant_type":    "password",
		"client_id":     "e2e-member",
		"client_secret": "e2e-member-secret",
		"username":      "13800000001",
		"password":      "mempw123",
		"account_type":  "member",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("member password grant 200, got %d body=%s", w.Code, w.Body.String())
	}
	var tok tokenResp
	if err := json.Unmarshal(w.Body.Bytes(), &tok); err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken == "" {
		t.Fatal("member access_token 空")
	}
	ping := doReq(t, r, http.MethodGet, "/api/admin/system/e2e/ping", tok.AccessToken)
	if ping.Code != http.StatusForbidden {
		t.Fatalf("member 访问 admin 路由 403, got %d body=%s", ping.Code, ping.Body.String())
	}
}

// TestE2E_ClientCredentials_Public_401：公开客户端（库中 secret 空）走机机
// 凭证 → 401 invalid_client（RFC 信封），绝不降级放行。
func TestE2E_ClientCredentials_Public_401(t *testing.T) {
	r, _ := newE2E(t)
	w := postJSON(t, r, "/api/oauth2/token", map[string]string{
		"grant_type": "client_credentials",
		"client_id":  "e2e-pub-m2m",
	})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("公开机机 401, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "invalid_client") {
		t.Fatalf("body 含 invalid_client, got %s", w.Body.String())
	}
}

// TestE2E_ClientCredentials_M2M_200NoRefresh：机密 client 机机凭证 200，
// 恒无 refresh_token（D9），且机机 token 不得进 admin API（403，
// Review Focus 3 的 client-token 用例）。
func TestE2E_ClientCredentials_M2M_200NoRefresh(t *testing.T) {
	r, _ := newE2E(t)
	w := postJSON(t, r, "/api/oauth2/token", map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     "e2e-m2m",
		"client_secret": "e2e-secret",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("机机 200, got %d body=%s", w.Code, w.Body.String())
	}
	var tok tokenResp
	if err := json.Unmarshal(w.Body.Bytes(), &tok); err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken == "" {
		t.Fatal("机机 access_token 空")
	}
	if tok.RefreshToken != "" {
		t.Fatalf("机机不该带 refresh_token, got %q", tok.RefreshToken)
	}
	ping := doReq(t, r, http.MethodGet, "/api/admin/system/e2e/ping", tok.AccessToken)
	if ping.Code != http.StatusForbidden {
		t.Fatalf("机机 token 访问 admin 路由 403, got %d body=%s", ping.Code, ping.Body.String())
	}
}

// TestE2E_AuthCode_FullFlow：授权码全流——GET authorize 302 登录页 →
// POST authorize/login 302 回跳带 code/state → POST token 换 JWT 全通。
func TestE2E_AuthCode_FullFlow(t *testing.T) {
	r, _ := newE2E(t)
	verifier, challenge := pkcePair(t)
	redirect := "http://localhost:5173/callback"

	q := url.Values{}
	q.Set("client_id", "alexgo-admin")
	q.Set("redirect_uri", redirect)
	q.Set("response_type", "code")
	q.Set("state", "st-full")
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	w := doReq(t, r, http.MethodGet, "/api/oauth2/authorize?"+q.Encode(), "")
	if w.Code != http.StatusFound {
		t.Fatalf("authorize 302, got %d body=%s", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); !strings.HasPrefix(loc, "/auth/login?") {
		t.Fatalf("Location = %q, want 前缀 /auth/login?", loc)
	}

	w2 := postJSON(t, r, "/api/oauth2/authorize/login", map[string]string{
		"client_id":             "alexgo-admin",
		"redirect_uri":          redirect,
		"state":                 "st-full",
		"response_type":         "code",
		"username":              "e2eadmin",
		"password":              "pw123456",
		"account_type":          "system",
		"code_challenge":        challenge,
		"code_challenge_method": "S256",
	})
	if w2.Code != http.StatusFound {
		t.Fatalf("authorize/login 302, got %d body=%s", w2.Code, w2.Body.String())
	}
	loc2 := w2.Header().Get("Location")
	if !strings.HasPrefix(loc2, redirect+"?") {
		t.Fatalf("Location = %q, want 前缀 %s?", loc2, redirect)
	}
	code := locParam(t, w2, "code")
	if st := locParam(t, w2, "state"); st != "st-full" {
		t.Fatalf("state = %q, want st-full", st)
	}

	w3 := postJSON(t, r, "/api/oauth2/token", map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     "alexgo-admin",
		"code":          code,
		"redirect_uri":  redirect,
		"code_verifier": verifier,
	})
	if w3.Code != http.StatusOK {
		t.Fatalf("exchange 200, got %d body=%s", w3.Code, w3.Body.String())
	}
	var tok tokenResp
	if err := json.Unmarshal(w3.Body.Bytes(), &tok); err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken == "" || tok.TokenType != "Bearer" {
		t.Fatalf("exchange 响应异常: %+v", tok)
	}
}

// TestE2E_RedirectTamper_400：redirect_uri 篡改——authorize 与 authorize/login
// 两侧都 400 且绝不带 Location（开放重定向防线，Review Focus 1）；
// login 侧凭据合法也必须 400，钉死「白名单先于凭据」的顺序。
func TestE2E_RedirectTamper_400(t *testing.T) {
	r, _ := newE2E(t)
	evil := "http://evil.example/cb"
	_, challenge := pkcePair(t)

	q := url.Values{}
	q.Set("client_id", "alexgo-admin")
	q.Set("redirect_uri", evil)
	q.Set("response_type", "code")
	q.Set("state", "st-evil")
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	w := doReq(t, r, http.MethodGet, "/api/oauth2/authorize?"+q.Encode(), "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("authorize 篡改 400, got %d body=%s", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc != "" {
		t.Fatalf("400 不得带 Location, got %q", loc)
	}
	if !strings.Contains(w.Body.String(), "invalid_request") {
		t.Fatalf("body 含 invalid_request, got %s", w.Body.String())
	}

	w2 := postJSON(t, r, "/api/oauth2/authorize/login", map[string]string{
		"client_id":             "alexgo-admin",
		"redirect_uri":          evil,
		"state":                 "st-evil",
		"response_type":         "code",
		"username":              "e2eadmin",
		"password":              "pw123456",
		"account_type":          "system",
		"code_challenge":        challenge,
		"code_challenge_method": "S256",
	})
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("authorize/login 篡改 400, got %d body=%s", w2.Code, w2.Body.String())
	}
	if loc := w2.Header().Get("Location"); loc != "" {
		t.Fatalf("400 不得带 Location, got %q", loc)
	}
	if !strings.Contains(w2.Body.String(), "invalid_request") {
		t.Fatalf("body 含 invalid_request, got %s", w2.Body.String())
	}
}

// TestE2E_CodeReplay_400：授权码一次性——首次 exchange 200 后重放
// 必须 400 invalid_grant（Review Focus 4；CAS 单次消费）。
func TestE2E_CodeReplay_400(t *testing.T) {
	r, _ := newE2E(t)
	verifier, challenge := pkcePair(t)
	redirect := "http://localhost:5173/callback"

	wLogin := postJSON(t, r, "/api/oauth2/authorize/login", map[string]string{
		"client_id":             "alexgo-admin",
		"redirect_uri":          redirect,
		"state":                 "st-replay",
		"response_type":         "code",
		"username":              "e2eadmin",
		"password":              "pw123456",
		"account_type":          "system",
		"code_challenge":        challenge,
		"code_challenge_method": "S256",
	})
	if wLogin.Code != http.StatusFound {
		t.Fatalf("authorize/login 302, got %d body=%s", wLogin.Code, wLogin.Body.String())
	}
	code := locParam(t, wLogin, "code")

	exchange := map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     "alexgo-admin",
		"code":          code,
		"redirect_uri":  redirect,
		"code_verifier": verifier,
	}
	w1 := postJSON(t, r, "/api/oauth2/token", exchange)
	if w1.Code != http.StatusOK {
		t.Fatalf("首次 exchange 200, got %d body=%s", w1.Code, w1.Body.String())
	}
	w2 := postJSON(t, r, "/api/oauth2/token", exchange)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("重放 400, got %d body=%s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "invalid_grant") {
		t.Fatalf("body 含 invalid_grant, got %s", w2.Body.String())
	}
}

// chdirRepoRoot 切到仓库根：Runner 迁移源路径 file://modules/... 相对进程 cwd
// 解析，本包位于 modules/system/oauth2（距根三级）。幂等：已在根则不上跳。
func chdirRepoRoot(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
		return
	}
	root := filepath.Clean(filepath.Join(wd, "..", "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("chdirRepoRoot 上跳后不在仓库根（缺 go.mod）：%s", root)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
}

// mustBcrypt 造测试口令哈希（MinCost：7 个用例各自造哈希，压启动耗时，裁决 6）。
func mustBcrypt(t *testing.T, pw string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return string(h)
}

// newE2E 搭最小可跑栈：切根 → 三迁移源建表（裁决 1）→ 清残留并 seed e2e 数据
// （裁决 2，绝不碰种子行）→ 手工 DI（不经 fx）→ 鉴权中间件 + 受保护桩 + 协议路由。
func newE2E(t *testing.T) (*gin.Engine, *casbin.Enforcer) {
	t.Helper()
	chdirRepoRoot(t)
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		t.Skip("DB_DSN not set; skip integration test")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("mysql unreachable: %v", err)
	}

	cfg := &config.Config{}
	cfg.Auth.Mode = "token"
	cfg.Auth.Captcha.Enabled = false
	cfg.Auth.LoginPage = "/auth/login"
	cfg.Auth.AccessExpireHour = 2
	cfg.Auth.RefreshExpireDay = 7
	cfg.Migrate.Auto = true
	cfg.Database.DSN = dsn

	runner := migrate.NewRunner(db, cfg, []migrate.Source{
		oauth2.NewMigrationSource(), // oauth2_client / oauth2_authorize_code + 种子 client
		system.NewMigrationSource(), // system_users / roles / menus / token 表
		member.NewMigrationSource(), // member_user
	})
	if err := runner.Run(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// 自造自销：跑前清上次中断残留，收尾再清——只动 e2e 专属行（裁决 2）。
	purge := func() {
		for _, sql := range []string{
			"DELETE FROM system_users WHERE username='e2eadmin'",
			"DELETE FROM member_user WHERE mobile='13800000001'",
			"DELETE FROM oauth2_client WHERE client_id IN ('e2e-m2m','e2e-member','e2e-pub-m2m')",
		} {
			_ = db.Exec(sql).Error
		}
	}
	purge()
	t.Cleanup(purge)

	if err := db.Create(&sysmodel.User{
		Username:     "e2eadmin",
		Nickname:     "e2e 管理员",
		PasswordHash: mustBcrypt(t, "pw123456"),
		Status:       1,
		TenantID:     0,
	}).Error; err != nil {
		t.Fatalf("seed e2eadmin: %v", err)
	}
	if err := db.Create(&membermodel.MemberUser{
		Mobile:   "13800000001",
		Nickname: "e2e 会员",
		Password: mustBcrypt(t, "mempw123"),
		Status:   1,
		TenantID: 0,
	}).Error; err != nil {
		t.Fatalf("seed e2e member: %v", err)
	}
	for _, c := range []oauth2model.Oauth2Client{
		{ClientID: "e2e-m2m", ClientSecret: mustBcrypt(t, "e2e-secret"),
			GrantTypes: "client_credentials", AccountTypes: "system", Scopes: "all", Activity: 1},
		{ClientID: "e2e-member", ClientSecret: mustBcrypt(t, "e2e-member-secret"),
			GrantTypes: "password,refresh_token", AccountTypes: "member", Scopes: "all", Activity: 1},
		// 公开机机：secret 空 + grants 含 client_credentials——先过 AllowsGrant
		// 才会到 401「公开客户端不支持机机凭证」分支（裁决 7）。
		{ClientID: "e2e-pub-m2m", ClientSecret: "",
			GrantTypes: "client_credentials", AccountTypes: "system", Scopes: "all", Activity: 1},
	} {
		if err := db.Create(&c).Error; err != nil {
			t.Fatalf("seed client %s: %v", c.ClientID, err)
		}
	}

	enforcer, err := auth.NewCasbinEnforcer(db)
	if err != nil {
		t.Fatalf("casbin: %v", err)
	}
	issuer := token.NewService(db, cfg) // 同时充当 Issuer 与 Validator
	captcha := oauth2svc.NewCaptchaService(oauth2svc.CaptchaDeps{Cfg: cfg})
	clients := oauth2svc.NewClientService(oauth2repo.NewClientRepository(db))
	codes := oauth2svc.NewAuthorizeCodeService(oauth2repo.NewAuthorizeCodeRepository(db))
	userAuth := sysvc.NewUserAuthenticator(
		sysrepo.NewRepository(db),
		sysvc.NewPermissionService(
			sysrepo.NewRoleRepository(db),
			sysrepo.NewMenuRepository(db),
			sysrepo.NewUserRoleRepository(db),
			sysrepo.NewRoleMenuRepository(db),
			enforcer,
		),
	)
	memberAuth := membersvc.NewMemberService(memberrepo.NewMemberRepository(db), issuer, nil)
	unified := oauth2svc.NewUnifiedLogin(oauth2svc.UnifiedDeps{
		Cfg:        cfg,
		Issuer:     issuer,
		Captcha:    captcha,
		UserAuth:   userAuth,
		MemberAuth: memberAuth,
	}) // Auditor/JWTGen 省略——optional 且 nil 安全（裁决 5）
	tokens := oauth2svc.NewTokenService(oauth2svc.TokenServiceDeps{
		Cfg: cfg, Clients: clients, Codes: codes, Login: unified, Issuer: issuer,
	})
	proto := controller.NewProtocolController(controller.ProtocolDeps{
		Captcha: captcha, Codes: codes, Clients: clients, Cfg: cfg,
		Login: unified, Tokens: tokens, Validator: issuer,
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api", middleware.NewAuthMiddleware(middleware.AuthDeps{
		Cfg:       cfg,
		Enforcer:  enforcer,
		Validator: issuer,
	})) // ScopeLoader 省略（nil 跳过数据权限装载）
	// 受保护桩：真实 system 全量路由归 fx 接线红线（裁决 3），
	// e2e 只验中间件 + Casbin 的 200/403 通断。
	api.GET("/admin/system/e2e/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": "pong"})
	})
	proto.Register(api)
	return router, enforcer
}

// allowPing 给 e2eadmin 授受保护桩的读策略。**必须在拿到 token 之后调用**：
// 登录内部 EnsureUserRolePolicy 起手 loadPolicy 重载，事前加的内存策略会被
// 整批洗掉（裁决 4）。重复 AddPolicy 幂等（已存在返回 false，不报错）。
func allowPing(t *testing.T, e *casbin.Enforcer) {
	t.Helper()
	sub := auth.UserSub(0, 1, "e2eadmin", 0) // tenant 0 / UserTypeAdmin=1 / e2eadmin
	if _, err := e.AddPolicy(sub, "/api/admin/system/e2e/ping", "GET"); err != nil {
		t.Fatalf("AddPolicy: %v", err)
	}
}

// postJSON POST JSON 体（Content-Type: application/json），返回响应记录。
func postJSON(t *testing.T, r *gin.Engine, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// doReq 发任意方法请求；bearer 非空时带 Authorization: Bearer。
func doReq(t *testing.T, r *gin.Engine, method, path, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	r.ServeHTTP(w, req)
	return w
}

// pkcePair 生成 RFC 7636 对：32B crypto/rand 的 base64url verifier +
// 其 SHA-256 的 base64url challenge（S256）。
func pkcePair(t *testing.T) (verifier, challenge string) {
	t.Helper()
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	verifier = base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge
}

// locParam 从响应 Location 取 query 参数（无 Location / 缺参即 Fatal——
// 这两个断言本身就是用例意图的一部分）。
func locParam(t *testing.T, w *httptest.ResponseRecorder, key string) string {
	t.Helper()
	loc := w.Header().Get("Location")
	if loc == "" {
		t.Fatal("响应无 Location")
	}
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("解析 Location %q: %v", loc, err)
	}
	v := u.Query().Get(key)
	if v == "" {
		t.Fatalf("Location 缺参数 %q: %q", key, loc)
	}
	return v
}
