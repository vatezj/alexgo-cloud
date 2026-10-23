package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"alexGo-cloud/modules/fishgame/model"
	"alexGo-cloud/modules/fishgame/repository"
	"alexGo-cloud/modules/fishgame/service"
)

// newRouter 起全五组路由（与 module.go RegisterRoutes 的 admin 段同形状）。
func newRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	db := newAdminTestDB(t)
	players := repository.NewPlayerRepository(db)
	friends := repository.NewFriendRepository(db)
	dex := repository.NewDexRepository(db)
	friend := service.NewFriendService(service.NewStoredFriendSource(friends, players),
		players, friends, dex)
	svc := service.NewAdminService(
		repository.NewFishRepository(db),
		repository.NewConfigRepository(db),
		repository.NewTrackEventRepository(db),
		players, dex, friend,
	)
	c := NewAdminController(svc)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/admin/fishgame")
	g.GET("/fishes", c.ListFish)
	g.POST("/fishes", c.CreateFish)
	g.PUT("/fishes/:id", c.UpdateFish)
	g.DELETE("/fishes/:id", c.DeleteFish)
	g.GET("/configs", c.ListConfig)
	g.POST("/configs", c.CreateConfig)
	g.PUT("/configs/:id", c.UpdateConfig)
	g.DELETE("/configs/:id", c.DeleteConfig)
	g.GET("/events", c.ListEvents)
	g.GET("/players", c.ListPlayers)
	g.GET("/players/:id", c.PlayerDetail)
	g.GET("/players/:id/rank", c.PlayerRank)
	g.GET("/stats", c.Stats)
	return r, db
}

// envelope 统一信封断言的最小形状 {code, msg, data}。
type envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func doJSON(t *testing.T, r *gin.Engine, method, path, body string) (*httptest.ResponseRecorder, envelope) {
	t.Helper()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var env envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("信封解析失败: %v body=%s", err, w.Body.String())
	}
	return w, env
}

func seedFishRow(t *testing.T, db *gorm.DB, fishID, rarity string, status int) {
	t.Helper()
	f := &model.Fish{TenantID: 0, FishID: fishID, Name: fishID, Rarity: rarity,
		Period: "any", Depth: "any", Sort: 1, Status: status}
	if err := repository.NewFishRepository(db).Create(t.Context(), f); err != nil {
		t.Fatal(err)
	}
}

// TestAdmin_ListFish_EnvelopeAndFilter 信封 + rarity 透传（casbin 段标准形状）。
func TestAdmin_ListFish_EnvelopeAndFilter(t *testing.T) {
	r, db := newRouter(t)
	seedFishRow(t, db, "inkfish", "normal", 1)
	seedFishRow(t, db, "ghostfin", "rare", 1)

	w, env := doJSON(t, r, http.MethodGet, "/admin/fishgame/fishes?rarity=rare&page=1&size=10", "")
	if w.Code != http.StatusOK || env.Code != 0 {
		t.Fatalf("应 200/code0: http=%d code=%d body=%s", w.Code, env.Code, w.Body.String())
	}
	var data struct {
		List  []map[string]any `json:"list"`
		Total int64            `json:"total"`
		Page  int              `json:"page"`
		Size  int              `json:"size"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("data 解析: %v", err)
	}
	if data.Total != 1 || len(data.List) != 1 || data.List[0]["fish_id"] != "ghostfin" {
		t.Fatalf("rarity 筛选透传错: %+v", data)
	}
	if data.Page != 1 || data.Size != 10 {
		t.Fatalf("分页缺省错: page=%d size=%d", data.Page, data.Size)
	}
}

// TestAdmin_CreateFish_Duplicate_400 查重 → 信封 30401 + HTTP 400（statusFor 304xx→400）。
func TestAdmin_CreateFish_Duplicate_400(t *testing.T) {
	r, db := newRouter(t)
	seedFishRow(t, db, "inkfish", "normal", 1)

	w, env := doJSON(t, r, http.MethodPost, "/admin/fishgame/fishes",
		`{"fish_id":"inkfish","name":"x","rarity":"normal","period":"any","depth":"any"}`)
	if w.Code != http.StatusBadRequest || env.Code != 30401 {
		t.Fatalf("重复应 400/30401: http=%d code=%d body=%s", w.Code, env.Code, w.Body.String())
	}
}

// TestAdmin_CreateFish_EnumInvalid 枚举非法 → 10001/400。
func TestAdmin_CreateFish_EnumInvalid(t *testing.T) {
	r, _ := newRouter(t)
	w, env := doJSON(t, r, http.MethodPost, "/admin/fishgame/fishes",
		`{"fish_id":"a","name":"x","rarity":"legend","period":"any","depth":"any"}`)
	if w.Code != http.StatusBadRequest || env.Code != 10001 {
		t.Fatalf("枚举应 400/10001: http=%d code=%d", w.Code, env.Code)
	}
}

// TestAdmin_DeleteFish_Referenced_400 引用拒删 → 400/30402。
func TestAdmin_DeleteFish_Referenced_400(t *testing.T) {
	r, db := newRouter(t)
	seedFishRow(t, db, "inkfish", "normal", 1)
	var f model.Fish
	if err := db.Where("fish_id = ?", "inkfish").First(&f).Error; err != nil {
		t.Fatal(err)
	}
	if err := repository.NewDexRepository(db).MergeReport(t.Context(), 1, map[string]int{"inkfish": 1}); err != nil {
		t.Fatal(err)
	}
	w, env := doJSON(t, r, http.MethodDelete, "/admin/fishgame/fishes/"+strconv.FormatUint(f.ID, 10), "")
	if w.Code != http.StatusBadRequest || env.Code != 30402 {
		t.Fatalf("引用拒删应 400/30402: http=%d code=%d body=%s", w.Code, env.Code, w.Body.String())
	}
}

// TestAdmin_Stats_OK 看板信封 + 关键字段在位。
func TestAdmin_Stats_OK(t *testing.T) {
	r, _ := newRouter(t)
	w, env := doJSON(t, r, http.MethodGet, "/admin/fishgame/stats", "")
	if w.Code != http.StatusOK || env.Code != 0 {
		t.Fatalf("应 200/code0: http=%d code=%d body=%s", w.Code, env.Code, w.Body.String())
	}
	var data struct {
		PlayerTotal int64            `json:"player_total"`
		Funnel      map[string]int64 `json:"funnel"`
		SignCount   int64            `json:"sign_count"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("data 解析: %v", err)
	}
	if _, ok := data.Funnel["catch"]; !ok {
		t.Fatalf("funnel 应含 catch: %+v", data.Funnel)
	}
}

// TestAdmin_PlayerRank_NotFound_400 玩家查无 → 400/30002（信封段映射）。
func TestAdmin_PlayerRank_NotFound_400(t *testing.T) {
	r, _ := newRouter(t)
	w, env := doJSON(t, r, http.MethodGet, "/admin/fishgame/players/999/rank", "")
	if w.Code != http.StatusBadRequest || env.Code != 30002 {
		t.Fatalf("查无应 400/30002: http=%d code=%d body=%s", w.Code, env.Code, w.Body.String())
	}
}

// TestAdmin_ListPlayers_Defaults 空参 → page=1 size=10 缺省透传。
func TestAdmin_ListPlayers_Defaults(t *testing.T) {
	r, _ := newRouter(t)
	w, env := doJSON(t, r, http.MethodGet, "/admin/fishgame/players", "")
	if w.Code != http.StatusOK || env.Code != 0 {
		t.Fatalf("应 200/code0: http=%d code=%d", w.Code, env.Code)
	}
	var data struct {
		Page int `json:"page"`
		Size int `json:"size"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatal(err)
	}
	if data.Page != 1 || data.Size != 10 {
		t.Fatalf("分页缺省错: %+v", data)
	}
}
