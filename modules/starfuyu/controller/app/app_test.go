package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/starfuyu/model"
	"alexGo-cloud/modules/starfuyu/service"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/token"
)

// ---- 测试桩（stubValidator 照 scorecard controller/app/app_test.go）----

// stubValidator token.Validator 测试桩：按 access token 串返回固定 claims。
type stubValidator struct {
	byToken map[string]*token.Claims
}

func (s stubValidator) Validate(_ context.Context, accessToken string) (*token.Claims, error) {
	if cl, ok := s.byToken[accessToken]; ok {
		return cl, nil
	}
	return nil, errors.New("stub: bad token")
}

// stubFortune service.FortuneService 桩：GetByDate/GetOne 可配；
// GetByDate 记录 ctx 租户（断言 claims 把 token 租户写回了请求 ctx）。
type stubFortune struct {
	getByDate func(ctx context.Context, date string) ([]*model.Fortune, error)
	getOne    func(ctx context.Context, date, constellation string) (*model.Fortune, error)
	tenant    uint64 // 最近一次 GetByDate 读到的 ctx 租户
}

func (s *stubFortune) Fetch(context.Context, string, bool) (bool, error) { return false, nil }

func (s *stubFortune) GetByDate(ctx context.Context, date string) ([]*model.Fortune, error) {
	s.tenant = tenant.TenantIDFromContext(ctx)
	if s.getByDate != nil {
		return s.getByDate(ctx, date)
	}
	return []*model.Fortune{}, nil // 缺省「无数据」：空列表而非 nil
}

func (s *stubFortune) GetOne(ctx context.Context, date, constellation string) (*model.Fortune, error) {
	if s.getOne != nil {
		return s.getOne(ctx, date, constellation)
	}
	return &model.Fortune{Date: date, Constellation: constellation}, nil
}

func (s *stubFortune) ListAdmin(context.Context, string, string, int, int) ([]*model.Fortune, int64, error) {
	return []*model.Fortune{}, 0, nil
}

// stubProfile service.ProfileService 桩：GetMy 缺省「未设星座」（nil, nil）。
type stubProfile struct {
	getMy func(ctx context.Context, memberUserID uint64) (*model.Profile, error)
	setMy func(ctx context.Context, memberUserID uint64, constellation string) (*model.Profile, error)
}

func (s *stubProfile) GetMy(ctx context.Context, memberUserID uint64) (*model.Profile, error) {
	if s.getMy != nil {
		return s.getMy(ctx, memberUserID)
	}
	return nil, nil
}

func (s *stubProfile) SetMy(ctx context.Context, memberUserID uint64, constellation string) (*model.Profile, error) {
	if s.setMy != nil {
		return s.setMy(ctx, memberUserID, constellation)
	}
	return &model.Profile{MemberUserID: memberUserID, Constellation: constellation}, nil
}

// stubBazi service.BaziService 桩：Calculate/UpdateMy 缺省成功（超限用例自行配错误）。
type stubBazi struct {
	calculate func(ctx context.Context, memberUserID uint64, req service.BaziReq) (*model.Bazi, error)
}

func (s *stubBazi) Calculate(ctx context.Context, memberUserID uint64, req service.BaziReq) (*model.Bazi, error) {
	if s.calculate != nil {
		return s.calculate(ctx, memberUserID, req)
	}
	return &model.Bazi{MemberUserID: memberUserID, BirthDate: req.BirthDate}, nil
}

func (s *stubBazi) GetMy(context.Context, uint64) (*model.Bazi, error) {
	return &model.Bazi{}, nil
}

func (s *stubBazi) UpdateMy(ctx context.Context, memberUserID uint64, req service.BaziReq) (*model.Bazi, error) {
	return s.Calculate(ctx, memberUserID, req)
}

// stubDivination service.DivinationService 桩：GetSign 缺省按签号回固定签文。
type stubDivination struct {
	getSign func(ctx context.Context, number int) (string, error)
	draw    func(ctx context.Context, memberUserID uint64, question, category string) (*model.Divination, error)
}

func (s *stubDivination) GetSign(ctx context.Context, number int) (string, error) {
	if s.getSign != nil {
		return s.getSign(ctx, number)
	}
	return "签文内容", nil
}

func (s *stubDivination) Draw(ctx context.Context, memberUserID uint64, question, category string) (*model.Divination, error) {
	if s.draw != nil {
		return s.draw(ctx, memberUserID, question, category)
	}
	return &model.Divination{MemberUserID: memberUserID, Question: question, Category: category}, nil
}

func (s *stubDivination) History(context.Context, uint64, int, int) ([]*model.Divination, int64, error) {
	return []*model.Divination{}, 0, nil
}

// stubImage service.ImageService 桩：Ensure 记录入参（钉 date/constellation 参数顺序）。
type stubImage struct {
	ensure       func(ctx context.Context, date, constellation string) (string, error)
	gotDate      string
	gotConstella string
}

func (s *stubImage) Ensure(ctx context.Context, date, constellation string) (string, error) {
	s.gotDate, s.gotConstella = date, constellation
	if s.ensure != nil {
		return s.ensure(ctx, date, constellation)
	}
	return "fortune/v3/" + date + "/" + constellation + ".jpg", nil
}

func (s *stubImage) GenerateAll(context.Context, string) (int, error) { return 0, nil }

// ---- 路由与请求辅助 ----

// testEnv 桩环境：一份控制器 + 各服务桩（供用例改配行为）。
type testEnv struct {
	router  *gin.Engine
	fortune *stubFortune
	profile *stubProfile
	bazi    *stubBazi
	divi    *stubDivination
	image   *stubImage
}

const (
	tokMember = "tok-member" // member，租户 7
	tokAdmin  = "tok-admin"  // admin，应被 403 拒
)

// newTestRouter 装 spec §4.1 全部 13 端点（清单与 module.go RegisterRoutes 一致——
// 注册即钉死 gin 静态段与 :param 共存不 panic）；不挂 Tenant 中间件，ctx 租户
// 只能来自 claims 写回，正好单独断言写回行为。
func newTestRouter(t *testing.T) *testEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	env := &testEnv{
		fortune: &stubFortune{},
		profile: &stubProfile{},
		bazi:    &stubBazi{},
		divi:    &stubDivination{},
		image:   &stubImage{},
	}
	ctrl := NewAppController(env.fortune, env.profile, env.bazi, env.divi, env.image,
		stubValidator{byToken: map[string]*token.Claims{
			tokMember: {UserID: 100, Username: "m", UserType: token.UserTypeMember, TenantID: 7},
			tokAdmin:  {UserID: 1, Username: "a", UserType: token.UserTypeAdmin, TenantID: 1},
		}})

	r := gin.New()
	app := r.Group("/api/app/starfuyu")
	app.GET("/fortune/today", ctrl.FortuneToday)
	app.GET("/fortune/:date", ctrl.FortuneByDate)
	app.GET("/fortune/my/today", ctrl.FortuneMyToday)
	app.GET("/fortune/my/:date", ctrl.FortuneMyByDate)
	app.GET("/fortune/image/:constellation/:date", ctrl.FortuneImage)
	app.POST("/bazi/calculate", ctrl.BaziCalculate)
	app.GET("/bazi/my", ctrl.BaziMy)
	app.PUT("/bazi/my", ctrl.BaziUpdateMy)
	app.GET("/divination/sign/:number", ctrl.DivinationSign) // 公开：不挂 claims
	app.POST("/divination/draw", ctrl.DivinationDraw)
	app.GET("/divination/history", ctrl.DivinationHistory)
	app.GET("/profile", ctrl.ProfileGet)
	app.PUT("/profile", ctrl.ProfilePut)
	env.router = r
	return env
}

// envelope 统一信封（resp.OK/Fail）。
type envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func doReq(r *gin.Engine, method, path, tokenStr, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if tokenStr != "" {
		req.Header.Set("Authorization", "Bearer "+tokenStr)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) envelope {
	t.Helper()
	var env envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v body=%s", err, w.Body.String())
	}
	return env
}

// wantCode 断言 HTTP status + 信封 code。
func wantCode(t *testing.T, w *httptest.ResponseRecorder, httpStatus, code int) {
	t.Helper()
	if w.Code != httpStatus {
		t.Fatalf("http=%d want %d body=%s", w.Code, httpStatus, w.Body.String())
	}
	if env := decode(t, w); env.Code != code {
		t.Fatalf("code=%d want %d body=%s", env.Code, code, w.Body.String())
	}
}

// ---- 鉴权门（Review Focus #4）----

// TestAuthGate_NoToken401 无 Authorization → 401 + code 20001。
func TestAuthGate_NoToken401(t *testing.T) {
	env := newTestRouter(t)
	w := doReq(env.router, http.MethodGet, "/api/app/starfuyu/fortune/today", "", "")
	wantCode(t, w, http.StatusUnauthorized, int(errs.CodeUnauthorized))
}

// TestAuthGate_BadToken401 Validate 失败 → 401 20001。
func TestAuthGate_BadToken401(t *testing.T) {
	env := newTestRouter(t)
	w := doReq(env.router, http.MethodGet, "/api/app/starfuyu/fortune/today", "bad-token", "")
	wantCode(t, w, http.StatusUnauthorized, int(errs.CodeUnauthorized))
}

// TestAuthGate_AdminForbidden403 UserType=admin → 403 20003（C 端只收 member）。
func TestAuthGate_AdminForbidden403(t *testing.T) {
	env := newTestRouter(t)
	w := doReq(env.router, http.MethodGet, "/api/app/starfuyu/fortune/today", tokAdmin, "")
	wantCode(t, w, http.StatusForbidden, int(errs.CodeForbidden))
}

// TestAuthGate_MemberTenantWriteBack member 成功 → 200，且后续 handler（服务桩）
// 读到的 ctx tenant == claims.TenantID（无中间件兜底，写回是唯一租户来源）。
func TestAuthGate_MemberTenantWriteBack(t *testing.T) {
	env := newTestRouter(t)
	w := doReq(env.router, http.MethodGet, "/api/app/starfuyu/fortune/today", tokMember, "")
	wantCode(t, w, http.StatusOK, 0)
	if env.fortune.tenant != 7 {
		t.Fatalf("ctx tenant=%d want claims.TenantID=7", env.fortune.tenant)
	}
}

// ---- 关键路径 ----

// TestFortuneToday_EmptyList200 无数据 → 200 空列表（data 是 [] 不是 null）。
func TestFortuneToday_EmptyList200(t *testing.T) {
	env := newTestRouter(t)
	w := doReq(env.router, http.MethodGet, "/api/app/starfuyu/fortune/today", tokMember, "")
	wantCode(t, w, http.StatusOK, 0)
	env2 := decode(t, w)
	var list []any
	if err := json.Unmarshal(env2.Data, &list); err != nil {
		t.Fatalf("data 应是数组: %v data=%s", err, env2.Data)
	}
	if len(list) != 0 {
		t.Fatalf("无数据应空列表, got %d rows", len(list))
	}
}

// TestDivinationSign_Public200 GET /divination/sign/42 免 token → 200（公开端点）。
func TestDivinationSign_Public200(t *testing.T) {
	env := newTestRouter(t)
	w := doReq(env.router, http.MethodGet, "/api/app/starfuyu/divination/sign/42", "", "")
	wantCode(t, w, http.StatusOK, 0)
	var data struct {
		SignNumber  int    `json:"sign_number"`
		SignContent string `json:"sign_content"`
	}
	if err := json.Unmarshal(decode(t, w).Data, &data); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	if data.SignNumber != 42 || data.SignContent == "" {
		t.Fatalf("sign 应含 sign_number=42 与非空签文, got %+v", data)
	}
}

// TestBaziCalculate_LimitExceeded400 服务层超限 30503 → HTTP 400 + code 30503。
func TestBaziCalculate_LimitExceeded400(t *testing.T) {
	env := newTestRouter(t)
	env.bazi.calculate = func(context.Context, uint64, service.BaziReq) (*model.Bazi, error) {
		return nil, errs.New(errs.CodeStarfuyuLimitExceeded, nil)
	}
	w := doReq(env.router, http.MethodPost, "/api/app/starfuyu/bazi/calculate", tokMember,
		`{"birth_date":"1990-01-01","birth_time":"12:00:00","birth_place":"北京"}`)
	wantCode(t, w, http.StatusBadRequest, int(errs.CodeStarfuyuLimitExceeded))
}

// ---- 组合逻辑（brief 钉死）----

// TestFortuneMyToday_NoConstellation30505 未设星座 → 30505（fortune/my 组合第一步）。
func TestFortuneMyToday_NoConstellation30505(t *testing.T) {
	env := newTestRouter(t) // profile.getMy 缺省 nil
	w := doReq(env.router, http.MethodGet, "/api/app/starfuyu/fortune/my/today", tokMember, "")
	wantCode(t, w, http.StatusBadRequest, int(errs.CodeStarfuyuConstellationMissing))
}

// TestProfileGet_UnsetEmpty200 GET /profile 未设星座 → 200 + constellation:""（不报错）。
func TestProfileGet_UnsetEmpty200(t *testing.T) {
	env := newTestRouter(t)
	w := doReq(env.router, http.MethodGet, "/api/app/starfuyu/profile", tokMember, "")
	wantCode(t, w, http.StatusOK, 0)
	var data struct {
		Constellation string `json:"constellation"`
	}
	if err := json.Unmarshal(decode(t, w).Data, &data); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	if data.Constellation != "" {
		t.Fatalf("未设星座应 constellation=\"\", got %q", data.Constellation)
	}
}

// TestFortuneImage_URLComposition 图 URL 组合：Ensure(date,constellation) →
// data.url == /api/public/starfuyu/image/<key>；并钉路由参数顺序（:constellation/:date
// 与 Ensure 入参顺序相反，防写反）。
func TestFortuneImage_URLComposition(t *testing.T) {
	env := newTestRouter(t)
	w := doReq(env.router, http.MethodGet,
		"/api/app/starfuyu/fortune/image/aries/2026-11-12", tokMember, "")
	wantCode(t, w, http.StatusOK, 0)
	if env.image.gotDate != "2026-11-12" || env.image.gotConstella != "aries" {
		t.Fatalf("Ensure(date,constellation) 入参写反: date=%q constellation=%q",
			env.image.gotDate, env.image.gotConstella)
	}
	var data struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(decode(t, w).Data, &data); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	const want = "/api/public/starfuyu/image/fortune/v3/2026-11-12/aries.jpg"
	if data.URL != want {
		t.Fatalf("url=%q want %q", data.URL, want)
	}
}

// TestFortuneByDate_BadDate10001 :date 非法（非 YYYY-MM-DD）→ 400 + 10001（spec §4.1）。
func TestFortuneByDate_BadDate10001(t *testing.T) {
	env := newTestRouter(t)
	w := doReq(env.router, http.MethodGet, "/api/app/starfuyu/fortune/2026-13-01", tokMember, "")
	wantCode(t, w, http.StatusBadRequest, int(errs.CodeParamInvalid))
}
