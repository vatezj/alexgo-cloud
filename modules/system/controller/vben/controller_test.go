package vben

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/model"
	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
	"alexGo-cloud/modules/system/service"
	"alexGo-cloud/pkg/auth"
)

func newTestRouter(auth *fakeAuth, unified *fakeUnified, perm *fakePerm, user *fakeUser) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// Register 挂在 /api 根组上（全路径 /api/auth/login），与 server 装配一致。
	NewController(unified, auth, perm, user).Register(r.Group("/api"))
	return r
}

func postJSON(r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestLogin_Success_Envelope(t *testing.T) {
	unified := &fakeUnified{res: &oauth2svc.LoginResult{AccessToken: "at-1", RefreshToken: "rt-1", ExpiresIn: 7200}}
	w := postJSON(newTestRouter(&fakeAuth{}, unified, &fakePerm{}, &fakeUser{}),
		"/api/auth/login", `{"username":"admin","password":"admin123"}`)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Code  int     `json:"code"`
		Error *string `json:"error"`
		Data  struct {
			AccessToken  string `json:"accessToken"`
			RefreshToken string `json:"refreshToken"`
			ExpiresIn    int64  `json:"expiresIn"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Code != 0 || resp.Error != nil {
		t.Errorf("envelope = %+v, want code=0 error=null", resp)
	}
	if resp.Data.AccessToken != "at-1" || resp.Data.RefreshToken != "rt-1" || resp.Data.ExpiresIn != 7200 {
		t.Errorf("data = %+v, want vben accessToken/refreshToken/expiresIn", resp.Data)
	}
	if len(unified.logins) != 1 ||
		unified.logins[0].AccountType != oauth2svc.AccountTypeSystem ||
		unified.logins[0].ClientID != "alexgo-admin" {
		t.Errorf("logins = %+v, want 恰一次且 account_type=system client_id=alexgo-admin", unified.logins)
	}
}

func TestLogin_WrongPassword_401Envelope(t *testing.T) {
	unified := &fakeUnified{err: oauth2svc.ErrInvalidCredentials}
	w := postJSON(newTestRouter(&fakeAuth{}, unified, &fakePerm{}, &fakeUser{}),
		"/api/auth/login", `{"username":"admin","password":"nope"}`)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body=%s", w.Code, w.Body.String())
	}
	// 信封适配：vben 旧自定义信封 {code:401,error:...} → 统一信封 {code,msg,data}；
	// HTTP 401 不变，code 精确 20001（未登录或登录已失效），文案走注册表。
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data any    `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Code != 20001 || resp.Msg == "" || resp.Data != nil {
		t.Errorf("envelope = %+v, want {code:20001, msg:非空, data:null}", resp)
	}
}

func TestLogin_CaptchaInvalid_400(t *testing.T) {
	unified := &fakeUnified{err: oauth2svc.ErrCaptchaInvalid}
	w := postJSON(newTestRouter(&fakeAuth{}, unified, &fakePerm{}, &fakeUser{}),
		"/api/auth/login", `{"username":"admin","password":"x","captcha_uuid":"u","captcha_code":"y"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
	// 信封适配：captcha 显式分支 → 30005（验证码错误），HTTP 400 不变。
	if !strings.Contains(w.Body.String(), `"code":30005`) {
		t.Errorf("body = %s", w.Body.String())
	}
}

func TestLogin_BadBody_400(t *testing.T) {
	w := postJSON(newTestRouter(&fakeAuth{}, &fakeUnified{}, &fakePerm{}, &fakeUser{}),
		"/api/auth/login", `not-json`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
}

// withClaims 复刻中间件注入（Task 4 之前测试自注入）。
func withClaims(r *gin.Engine, path string, cl *auth.Claims, h gin.HandlerFunc) {
	r.GET(path, func(c *gin.Context) {
		if cl != nil {
			c.Set("claims", cl)
		}
		h(c)
	})
}

func TestLogout_NoToken_Still200(t *testing.T) {
	authSvc := &fakeAuth{}
	r := newTestRouter(authSvc, &fakeUnified{res: &oauth2svc.LoginResult{AccessToken: "at", RefreshToken: "rt", ExpiresIn: 1}}, &fakePerm{}, &fakeUser{})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200（no-op 也要成功）; body=%s", w.Code, w.Body.String())
	}
	if authSvc.logoutCount != 0 {
		t.Errorf("logout called %d times without token, want 0", authSvc.logoutCount)
	}
	if !strings.Contains(w.Body.String(), `"code":0`) {
		t.Errorf("body = %s, want envelope code 0", w.Body.String())
	}
}

func TestLogout_WithBearer_CallsService(t *testing.T) {
	authSvc := &fakeAuth{}
	r := newTestRouter(authSvc, &fakeUnified{res: &oauth2svc.LoginResult{AccessToken: "at", RefreshToken: "rt", ExpiresIn: 1}}, &fakePerm{}, &fakeUser{})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer the-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || authSvc.logoutCount != 1 {
		t.Errorf("status = %d logoutCount = %d, want 200/1", w.Code, authSvc.logoutCount)
	}
}

func TestCodes_ReturnsRawPermStrings(t *testing.T) {
	// 强制修复：newTestRouter 经 Register 已挂 /api/auth/codes，withClaims 再注册
	// 同路径会触发 gin 重复路由 panic；改用裸引擎（与本 brief 其余 3 个自注入测试同款）。
	gin.SetMode(gin.TestMode)
	r := gin.New()
	withClaims(r, "/api/auth/codes", &auth.Claims{UserID: 9, Username: "alice"}, func(c *gin.Context) {
		NewController(&fakeUnified{}, &fakeAuth{}, &fakePerm{codes: []string{"order:order:*", "system:role:*"}}, &fakeUser{}).Codes(c)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/auth/codes", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Code int      `json:"code"`
		Data []string `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Code != 0 || len(resp.Data) != 2 || resp.Data[0] != "order:order:*" {
		t.Errorf("resp = %+v, want raw perm strings in data", resp)
	}
}

func TestCodes_NilCodes_ReturnEmptyArray(t *testing.T) {
	ctrl := NewController(&fakeUnified{}, &fakeAuth{}, &fakePerm{codes: nil}, &fakeUser{})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/auth/codes", func(c *gin.Context) {
		c.Set("claims", &auth.Claims{UserID: 1})
		ctrl.Codes(c)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/auth/codes", nil))
	// data 必须是 [] 而非 null：vben accessStore 期望数组。
	if !strings.Contains(w.Body.String(), `"data":[]`) {
		t.Errorf("body = %s, want data:[] (not null)", w.Body.String())
	}
}

func TestUserInfo_StringUserIDAndRequiredFields(t *testing.T) {
	ctrl := NewController(&fakeUnified{}, &fakeAuth{}, &fakePerm{
		roles: []*model.Role{{Code: "admin"}, {Code: "ops"}},
	}, &fakeUser{user: &model.User{Nickname: "管理员", Avatar: "http://a/x.png"}})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/user/info", func(c *gin.Context) {
		c.Set("claims", &auth.Claims{UserID: 9, Username: "alice"})
		ctrl.UserInfo(c)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/user/info", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			UserID   string   `json:"userId"`
			Username string   `json:"username"`
			RealName string   `json:"realName"`
			Avatar   string   `json:"avatar"`
			Roles    []string `json:"roles"`
			Desc     *string  `json:"desc"`
			HomePath *string  `json:"homePath"`
			Token    *string  `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	d := resp.Data
	if d.UserID != "9" {
		t.Errorf("userId = %q (%T), want string \"9\"（vben BasicUserInfo.userId: string）", d.UserID, d.UserID)
	}
	if d.Username != "alice" || d.RealName != "管理员" || d.Avatar != "http://a/x.png" {
		t.Errorf("data = %+v, want username/nickname/avatar 映射", d)
	}
	if len(d.Roles) != 2 || d.Roles[0] != "admin" {
		t.Errorf("roles = %v, want [admin ops]", d.Roles)
	}
	if d.Desc == nil || d.HomePath == nil || d.Token == nil {
		t.Errorf("desc/homePath/token 必须存在（UserInfo 三必填），got %+v", d)
	}
}

func TestUserInfo_NoClaims_401(t *testing.T) {
	ctrl := NewController(&fakeUnified{}, &fakeAuth{}, &fakePerm{}, &fakeUser{})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/user/info", ctrl.UserInfo) // 无 claims 注入
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/user/info", nil))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401（claims 缺失防御）", w.Code)
	}
}

func TestMenuAll_LAYOUTBlankedAndTreeKept(t *testing.T) {
	ctrl := NewController(&fakeUnified{}, &fakeAuth{}, &fakePerm{
		routes: []*service.VbenRoute{
			{
				Path: "/system", Name: "system", Component: "LAYOUT",
				Meta: service.VbenRouteMeta{Title: "系统管理", OrderNo: 10},
				Children: []*service.VbenRoute{
					{
						Path: "/system/users", Name: "system_users",
						Component: "views/system/SystemUsersPage",
						Meta:      service.VbenRouteMeta{Title: "用户管理", OrderNo: 1},
					},
				},
			},
		},
	}, &fakeUser{})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/menu/all", func(c *gin.Context) {
		c.Set("claims", &auth.Claims{UserID: 9})
		ctrl.MenuAll(c)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/menu/all", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, "LAYOUT") {
		t.Errorf("body 含 LAYOUT（vben convertRoutes 会报 component invalid）: %s", body)
	}
	if !strings.Contains(body, `"component":""`) {
		t.Errorf("目录 component 必须置空: %s", body)
	}
	if !strings.Contains(body, `"component":"views/system/SystemUsersPage"`) {
		t.Errorf("叶子 component 必须原样保留: %s", body)
	}
	if !strings.Contains(body, `"path":"/system/users"`) || !strings.Contains(body, `"title":"用户管理"`) {
		t.Errorf("树结构/meta 丢失: %s", body)
	}
}

func TestMenuAll_NilRoutes_EmptyArray(t *testing.T) {
	ctrl := NewController(&fakeUnified{}, &fakeAuth{}, &fakePerm{routes: nil}, &fakeUser{})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/menu/all", func(c *gin.Context) {
		c.Set("claims", &auth.Claims{UserID: 9})
		ctrl.MenuAll(c)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/menu/all", nil))
	if !strings.Contains(w.Body.String(), `"data":[]`) {
		t.Errorf("body = %s, want data:[]（nil 转空数组，vben fetchMenuListAsync 期望数组）", w.Body.String())
	}
}

// Register 必须挂齐 5 条路由：任一缺失在 vben 运行期表现为 404 → 登录流程卡死。
func TestRegister_MountsAllFiveRoutes(t *testing.T) {
	// fixture 必须给 unified 注入非 nil 的 LoginResult：循环里 POST /api/auth/login
	// 会让 Login 解引用 res.AccessToken → nil panic（断言保持逐字节不变）。
	r := newTestRouter(&fakeAuth{}, &fakeUnified{res: &oauth2svc.LoginResult{AccessToken: "at", RefreshToken: "rt", ExpiresIn: 1}}, &fakePerm{}, &fakeUser{})
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/api/auth/login"},
		{http.MethodPost, "/api/auth/logout"},
		{http.MethodGet, "/api/auth/codes"},
		{http.MethodGet, "/api/user/info"},
		{http.MethodGet, "/api/menu/all"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader("{}"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code == http.StatusNotFound {
			t.Errorf("%s %s 未挂载（gin 404）", tc.method, tc.path)
		}
	}
}
