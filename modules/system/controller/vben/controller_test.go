package vben

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/service"
	"alexGo-cloud/pkg/auth"
)

func newTestRouter(auth *fakeAuth, audit *fakeAudit, perm *fakePerm, user *fakeUser) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// Register 挂在 /api 根组上（全路径 /api/auth/login），与 server 装配一致。
	NewController(auth, audit, perm, user).Register(r.Group("/api"))
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
	auth := &fakeAuth{loginRes: &service.LoginResult{
		AccessToken: "at-1", RefreshToken: "rt-1", ExpiresIn: 7200,
	}}
	audit := &fakeAudit{}
	w := postJSON(newTestRouter(auth, audit, &fakePerm{}, &fakeUser{}),
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
	if len(audit.successes) != 1 || !audit.successes[0] {
		t.Errorf("audit successes = %v, want [true]", audit.successes)
	}
}

func TestLogin_WrongPassword_401Envelope(t *testing.T) {
	auth := &fakeAuth{loginErr: errInvalidCredentials}
	audit := &fakeAudit{}
	w := postJSON(newTestRouter(auth, audit, &fakePerm{}, &fakeUser{}),
		"/api/auth/login", `{"username":"admin","password":"nope"}`)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Code  int    `json:"code"`
		Data  any    `json:"data"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Code != 401 || resp.Error == "" || resp.Data != nil {
		t.Errorf("envelope = %+v, want {code:401, data:null, error:非空}", resp)
	}
	if len(audit.successes) != 1 || audit.successes[0] {
		t.Errorf("audit successes = %v, want [false]", audit.successes)
	}
}

func TestLogin_BadBody_400(t *testing.T) {
	w := postJSON(newTestRouter(&fakeAuth{}, &fakeAudit{}, &fakePerm{}, &fakeUser{}),
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
	r := newTestRouter(authSvc, &fakeAudit{}, &fakePerm{}, &fakeUser{})
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
	r := newTestRouter(authSvc, &fakeAudit{}, &fakePerm{}, &fakeUser{})
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
		NewController(&fakeAuth{}, &fakeAudit{}, &fakePerm{codes: []string{"order:order:*", "system:role:*"}}, &fakeUser{}).Codes(c)
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
	ctrl := NewController(&fakeAuth{}, &fakeAudit{}, &fakePerm{codes: nil}, &fakeUser{})
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
	ctrl := NewController(&fakeAuth{}, &fakeAudit{}, &fakePerm{
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
	ctrl := NewController(&fakeAuth{}, &fakeAudit{}, &fakePerm{}, &fakeUser{})
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
	ctrl := NewController(&fakeAuth{}, &fakeAudit{}, &fakePerm{
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
	ctrl := NewController(&fakeAuth{}, &fakeAudit{}, &fakePerm{routes: nil}, &fakeUser{})
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
	// 强制修复：brief 原文用 &fakeAuth{}（loginRes 为 nil 且 err 为 nil），
	// 循环里 POST /api/auth/login 会让 Login 解引用 res.AccessToken → nil panic；
	// 仅补 fixture 的 loginRes，断言保持逐字节不变。
	r := newTestRouter(&fakeAuth{loginRes: &service.LoginResult{AccessToken: "at", RefreshToken: "rt", ExpiresIn: 1}}, &fakeAudit{}, &fakePerm{}, &fakeUser{})
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
