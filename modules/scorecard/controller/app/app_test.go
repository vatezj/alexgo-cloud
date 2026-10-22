package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/scorecard/model"
	"alexGo-cloud/modules/scorecard/repository"
	"alexGo-cloud/modules/scorecard/service"
	"alexGo-cloud/modules/system/oauth2/social"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/middleware"
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/tenant/gormplugin"
	"alexGo-cloud/pkg/token"
)

// TestClaims_MissingToken401 无 token 拒绝（原有冒烟）。
func TestClaims_MissingToken401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/x", nil)
	ctrl := &AppController{}
	if _, ok := ctrl.claims(c); ok {
		t.Fatal("无 token 应拒绝")
	}
}

// stubValidator token.Validator 测试桩：按 access token 串返回固定 claims（多租户共用一个桩）。
type stubValidator struct {
	byToken map[string]*token.Claims
}

func (s stubValidator) Validate(_ context.Context, accessToken string) (*token.Claims, error) {
	if cl, ok := s.byToken[accessToken]; ok {
		return cl, nil
	}
	return nil, errors.New("stub: bad token")
}

// stubAppGetter social.AppGetter 测试桩：返回固定 SocialApp 或错误。
type stubAppGetter struct {
	app   *social.SocialApp
	err   error
	calls int
}

func (s *stubAppGetter) GetApp(_ context.Context, _ uint64, _ string) (*social.SocialApp, error) {
	s.calls++
	return s.app, s.err
}

// stubWxacoder social.Wxacoder 测试桩：返回固定 PNG 字节并记录入参（计数断言缓存）。
type stubWxacoder struct {
	png   []byte
	err   error
	calls int
	scene string
	page  string
	env   string
}

func (s *stubWxacoder) Wxacode(_ context.Context, _ *social.SocialApp, scene, page, envVersion string) ([]byte, error) {
	s.calls++
	s.scene, s.page, s.env = scene, page, envVersion
	return s.png, s.err
}

// newAppRouter 装一套真实链路：sqlite :memory: + 租户插件 + TenantMiddleware（按
// X-Tenant-ID 头解析，伪造入口）+ app 控制器（Bearer 自鉴权）。
// tokenA → 租户 1 user100；tokenC → 租户 1 user300；tokenB → 租户 2 user200。
// 返回 ctrl 供小程序码测试替换 social 桩（wxacode.go 注入面）。
func newAppRouter(t *testing.T) (*gin.Engine, *gorm.DB, service.RoomService, *AppController) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Room{}, &model.RoomUser{}, &model.RoomLog{},
		&model.Record{}, &model.Friend{}, &model.FriendBlack{}); err != nil {
		t.Fatal(err)
	}
	if err := gormplugin.Register(db, gormplugin.Options{ExemptTables: []string{"tenants", "casbin_rule"}}); err != nil {
		t.Fatal(err)
	}
	roomRepo := repository.NewRoomRepository(db)
	rooms := service.NewRoomService(db, roomRepo, repository.NewRoomUserRepository(db),
		repository.NewRoomLogRepository(db), repository.NewRecordRepository(db),
		nil /* briefs：缺席降级空昵称 */, repository.NewFriendBlackRepository(db))
	records := service.NewRecordService(repository.NewRecordRepository(db))
	friends := service.NewFriendService(db, repository.NewFriendRepository(db),
		repository.NewFriendBlackRepository(db), repository.NewRecordRepository(db), nil)
	ctrl := NewAppController(rooms, records, friends, stubValidator{byToken: map[string]*token.Claims{
		"tokA": {UserID: 100, Username: "a", UserType: token.UserTypeMember, TenantID: 1},
		"tokC": {UserID: 300, Username: "c", UserType: token.UserTypeMember, TenantID: 1},
		"tokB": {UserID: 200, Username: "b", UserType: token.UserTypeMember, TenantID: 2},
	}}, &stubAppGetter{app: &social.SocialApp{AppID: "wx-app", Secret: "wx-secret", Status: 1}},
		&stubWxacoder{png: []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}})
	r := gin.New()
	// 真实链路：Tenant 先于 handler（http.go 链序 Recovery → Tenant → … → handler）。
	r.Use(middleware.NewTenantMiddleware(nil))
	r.POST("/api/app/scorecard/rooms", ctrl.CreateRoom)
	r.GET("/api/app/scorecard/rooms/:id/snapshot", ctrl.Snapshot)
	r.GET("/api/app/scorecard/rooms/:id/wxacode", ctrl.Wxacode)
	return r, db, rooms, ctrl
}

// codeOf 解析统一信封 code（0 成功）。
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

func doReq(r *gin.Engine, method, path, tokenStr, fakeTenant, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	if fakeTenant != "" {
		req.Header.Set("X-Tenant-ID", fakeTenant)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestCreateRoom_TokenTenantBeatsForgedHeader 终审 Important #1：A 租户 token +
// 伪造 X-Tenant-ID: 2 开房 → 房落 A 租户（tenant_id=1），且以租户 2 上下文读不到。
func TestCreateRoom_TokenTenantBeatsForgedHeader(t *testing.T) {
	r, db, rooms, _ := newAppRouter(t)
	w := doReq(r, http.MethodPost, "/api/app/scorecard/rooms", "tokA", "2",
		`{"room_code":"ABCD"}`)
	if w.Code != http.StatusOK || codeOf(t, w) != 0 {
		t.Fatalf("开房应成功: http=%d body=%s", w.Code, w.Body.String())
	}
	var row model.Room
	if err := db.Where("room_code = ?", "ABCD").First(&row).Error; err != nil {
		t.Fatalf("查房行: %v", err)
	}
	if row.TenantID != 1 {
		t.Fatalf("伪造头改写了租户: tenant_id=%d, want 1", row.TenantID)
	}
	// B 租户上下文查该房 → 跨租户不可见（30101）。
	got, err := rooms.Snapshot(tenant.WithTenantID(context.Background(), 2), 100, row.ID)
	if !errs.Is(err, errs.CodeScorecardRoomNotFound) || got != nil {
		t.Fatalf("B 租户应 30101: snap=%+v err=%v", got, err)
	}
}

// TestSnapshot_TokenTenantBlocksCrossTenantRead 终审 Important #1 反向：B 租户真房，
// A 租户 token 伪造 X-Tenant-ID: 2 也读不到（token 租户覆盖头）。
func TestSnapshot_TokenTenantBlocksCrossTenantRead(t *testing.T) {
	r, db, _, _ := newAppRouter(t)
	// B 租户自己开房（tokenB + 头 2）。
	if w := doReq(r, http.MethodPost, "/api/app/scorecard/rooms", "tokB", "2",
		`{"room_code":"EFGH"}`); w.Code != http.StatusOK || codeOf(t, w) != 0 {
		t.Fatalf("B 租户开房应成功: %d %s", w.Code, w.Body.String())
	}
	var row model.Room
	if err := db.Where("room_code = ?", "EFGH").First(&row).Error; err != nil {
		t.Fatalf("查房行: %v", err)
	}
	// A token + 伪造头 2 → 仍按租户 1 过滤 → 30101 房不存在。
	snapPath := "/api/app/scorecard/rooms/" + itoa(row.ID) + "/snapshot"
	w := doReq(r, http.MethodGet, snapPath, "tokA", "2", "")
	if w.Code != http.StatusBadRequest || codeOf(t, w) != 30101 {
		t.Fatalf("跨租户读应 30101: http=%d body=%s", w.Code, w.Body.String())
	}
	// 同房对本租户可见（对照组，确保 30101 不是别的原因）。
	w = doReq(r, http.MethodGet, snapPath, "tokB", "2", "")
	if w.Code != http.StatusOK || codeOf(t, w) != 0 {
		t.Fatalf("本租户读应成功: http=%d body=%s", w.Code, w.Body.String())
	}
}

func itoa(v uint64) string { return strconv.FormatUint(v, 10) }

// ---- Task 2：房间小程序码（GET /rooms/:id/wxacode → 200 image/png 直出）----

// createRoomAs 以指定 token 开房并返回落库房行。
func createRoomAs(t *testing.T, r *gin.Engine, db *gorm.DB, tokenStr, code string) model.Room {
	t.Helper()
	if w := doReq(r, http.MethodPost, "/api/app/scorecard/rooms", tokenStr, "",
		`{"room_code":"`+code+`"}`); w.Code != http.StatusOK || codeOf(t, w) != 0 {
		t.Fatalf("开房应成功: %d %s", w.Code, w.Body.String())
	}
	var row model.Room
	if err := db.Where("room_code = ?", code).First(&row).Error; err != nil {
		t.Fatalf("查房行: %v", err)
	}
	return row
}

func TestWxacode_OK_PNG(t *testing.T) {
	r, db, _, ctrl := newAppRouter(t)
	row := createRoomAs(t, r, db, "tokA", "WXYZ")
	wx := &stubWxacoder{png: []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 1, 2, 3}}
	ctrl.wx = wx
	pngPath := "/api/app/scorecard/rooms/" + itoa(row.ID) + "/wxacode"
	w := doReq(r, http.MethodGet, pngPath, "tokA", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("取码应 200: http=%d body=%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "image/png") {
		t.Fatalf("Content-Type = %q, want image/png", ct)
	}
	if !bytes.Equal(w.Body.Bytes(), wx.png) {
		t.Fatalf("PNG 字节应原样直出: %v", w.Body.Bytes())
	}
	// scene 前缀 qr= 由控制器拼（房号来自房行）；page/envVersion 按常量契约。
	if wx.scene != "qr=WXYZ" || wx.page != "pages/index/index" || wx.env != "trial" {
		t.Fatalf("取码入参: scene=%q page=%q env=%q", wx.scene, wx.page, wx.env)
	}
	// 进程内按房号缓存 5min：二次请求命中缓存，不再打微信取码。
	w2 := doReq(r, http.MethodGet, pngPath, "tokA", "", "")
	if w2.Code != http.StatusOK || !bytes.Equal(w2.Body.Bytes(), wx.png) {
		t.Fatalf("缓存命中应同样直出: http=%d len=%d", w2.Code, w2.Body.Len())
	}
	if wx.calls != 1 {
		t.Fatalf("二次请求应命中缓存: wxacode calls = %d", wx.calls)
	}
}

func TestWxacode_RoomNotFound_30101(t *testing.T) {
	r, _, _, _ := newAppRouter(t)
	w := doReq(r, http.MethodGet, "/api/app/scorecard/rooms/999999/wxacode", "tokA", "", "")
	if w.Code != http.StatusBadRequest || codeOf(t, w) != 30101 {
		t.Fatalf("房不存在应 30101: http=%d body=%s", w.Code, w.Body.String())
	}
}

// TestWxacode_NotMember_30102 非成员不可取码（防外人批量取码）。
func TestWxacode_NotMember_30102(t *testing.T) {
	r, db, _, _ := newAppRouter(t)
	// tokA（user100，租户 1）开房；tokC（同租户 user300，非成员）取码 → 30102。
	row := createRoomAs(t, r, db, "tokA", "QRST")
	w := doReq(r, http.MethodGet, "/api/app/scorecard/rooms/"+itoa(row.ID)+"/wxacode", "tokC", "", "")
	if w.Code != http.StatusBadRequest || codeOf(t, w) != 30102 {
		t.Fatalf("非成员取码应 30102: http=%d body=%s", w.Code, w.Body.String())
	}
}

// TestWxacode_AppNotConfigured_500 渠道未配置/停用 → HTTP 500 + 明确 msg（非泛化系统繁忙）。
func TestWxacode_AppNotConfigured_500(t *testing.T) {
	r, db, _, ctrl := newAppRouter(t)
	row := createRoomAs(t, r, db, "tokA", "IJKL")
	ctrl.apps = &stubAppGetter{err: gorm.ErrRecordNotFound}
	w := doReq(r, http.MethodGet, "/api/app/scorecard/rooms/"+itoa(row.ID)+"/wxacode", "tokA", "", "")
	if w.Code != http.StatusInternalServerError || codeOf(t, w) != int(errs.CodeWxacodeNotConfigured) {
		t.Fatalf("未配置渠道应 500/50001: http=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "未配置") {
		t.Fatalf("500 应带明确 msg: %s", w.Body.String())
	}
}
