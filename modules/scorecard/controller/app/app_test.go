package app

import (
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

// newAppRouter 装一套真实链路：sqlite :memory: + 租户插件 + TenantMiddleware（按
// X-Tenant-ID 头解析，伪造入口）+ app 控制器（Bearer 自鉴权）。
// tokenA → 租户 1，tokenB → 租户 2。
func newAppRouter(t *testing.T) (*gin.Engine, *gorm.DB, service.RoomService) {
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
	records := service.NewRecordService(db, repository.NewRecordRepository(db))
	friends := service.NewFriendService(db, repository.NewFriendRepository(db),
		repository.NewFriendBlackRepository(db), repository.NewRecordRepository(db), nil)
	ctrl := NewAppController(rooms, records, friends, stubValidator{byToken: map[string]*token.Claims{
		"tokA": {UserID: 100, Username: "a", UserType: token.UserTypeMember, TenantID: 1},
		"tokB": {UserID: 200, Username: "b", UserType: token.UserTypeMember, TenantID: 2},
	}})
	r := gin.New()
	// 真实链路：Tenant 先于 handler（http.go 链序 Recovery → Tenant → … → handler）。
	r.Use(middleware.NewTenantMiddleware(nil))
	r.POST("/api/app/scorecard/rooms", ctrl.CreateRoom)
	r.GET("/api/app/scorecard/rooms/:id/snapshot", ctrl.Snapshot)
	return r, db, rooms
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
	r, db, rooms := newAppRouter(t)
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
	r, db, _ := newAppRouter(t)
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
