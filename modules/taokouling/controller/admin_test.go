package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/taokouling/model"
	"alexGo-cloud/modules/taokouling/repository"
	"alexGo-cloud/pkg/tenant/gormplugin"
)

// newTestDB sqlite 内存库 + 两模型 AutoMigrate + gormplugin（照 repository_test.go helper）。
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Channel{}, &model.ConversionLog{}); err != nil {
		t.Fatal(err)
	}
	if err := gormplugin.Register(db, gormplugin.Options{ExemptTables: []string{"tenants", "casbin_rule"}}); err != nil {
		t.Fatal(err)
	}
	return db
}

// newAdminRouter 起 admin 五路由（与 module.go RegisterRoutes admin 段同形状）。
func newAdminRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	db := newTestDB(t)
	c := NewAdminController(repository.NewChannelRepository(db), repository.NewConversionLogRepository(db))
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/channels", c.ListChannels)
	r.POST("/channels", c.CreateChannel)
	r.PUT("/channels/:id", c.UpdateChannel)
	r.DELETE("/channels/:id", c.DeleteChannel)
	r.GET("/logs", c.ListLogs)
	return r, db
}

// doReq 发请求并解析 {code,msg,data} 信封。
func doReq(t *testing.T, r *gin.Engine, method, path, body string) (int, int, json.RawMessage) {
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
	var env struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("信封解析失败: %v body=%s", err, w.Body.String())
	}
	return w.Code, env.Code, env.Data
}

// TestAdminChannel_CreateDup10001 create once → 200；same partner+host again → 400 code 10001。
func TestAdminChannel_CreateDup10001(t *testing.T) {
	r, _ := newAdminRouter(t)
	body := `{"name":"p1","partner":"kp","host":"h1","pid":"mm_1","apikey":"ak","tb_name":"tb","status":1}`

	status, code, _ := doReq(t, r, http.MethodPost, "/channels", body)
	if status != http.StatusOK || code != 0 {
		t.Fatalf("首次创建应 200/code0: status=%d code=%d", status, code)
	}

	status, code, _ = doReq(t, r, http.MethodPost, "/channels", body)
	if status != http.StatusBadRequest || code != 10001 {
		t.Fatalf("重复 partner+host 应 400/10001: status=%d code=%d", status, code)
	}
}

// TestAdminChannel_DeleteDefaultRejected seed default row → DELETE → 400 code 10001，row still present。
func TestAdminChannel_DeleteDefaultRejected(t *testing.T) {
	r, db := newAdminRouter(t)
	def := &model.Channel{Name: "默认渠道", Partner: "", Host: "", Status: 1}
	if err := repository.NewChannelRepository(db).Create(t.Context(), def); err != nil {
		t.Fatal(err)
	}

	status, code, _ := doReq(t, r, http.MethodDelete, "/channels/"+itoa(def.ID), "")
	if status != http.StatusBadRequest || code != 10001 {
		t.Fatalf("默认渠道删除应 400/10001: status=%d code=%d", status, code)
	}

	var n int64
	if err := db.Model(&model.Channel{}).Where("id = ?", def.ID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("默认行应仍在库: count=%d", n)
	}
}

// TestAdminLogs_PageShape append 2 rows → GET /logs → 200，body has list/total/page/size keys。
func TestAdminLogs_PageShape(t *testing.T) {
	r, db := newAdminRouter(t)
	logs := repository.NewConversionLogRepository(db)
	for i := 0; i < 2; i++ {
		if err := logs.Append(t.Context(), &model.ConversionLog{
			Endpoint: "parse", Status: 1, InputText: "tkl",
		}); err != nil {
			t.Fatal(err)
		}
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/logs?page=1&size=10", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("应 200: status=%d body=%s", w.Code, w.Body.String())
	}
	var env struct {
		Code int `json:"code"`
		Data struct {
			List  []any `json:"list"`
			Total int64 `json:"total"`
			Page  int   `json:"page"`
			Size  int   `json:"size"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("解析失败: %v body=%s", err, w.Body.String())
	}
	if env.Code != 0 {
		t.Fatalf("code 应 0: %d", env.Code)
	}
	if env.Data.Total != 2 || len(env.Data.List) != 2 {
		t.Fatalf("应 2 行: total=%d list=%d", env.Data.Total, len(env.Data.List))
	}
	if env.Data.Page != 1 || env.Data.Size != 10 {
		t.Fatalf("page/size 应 1/10: %d/%d", env.Data.Page, env.Data.Size)
	}
	// list/total/page/size 键存在性（结构体反序列化即证明键存在；再验原始 JSON 键）。
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(raw["data"], &data); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"list", "total", "page", "size"} {
		if _, ok := data[k]; !ok {
			t.Fatalf("data 缺键 %s: %s", k, w.Body.String())
		}
	}
}

// TestAdminLogs_DateOnlyEndIncludesFullDay date-only end 补全日：
// 种今日 15:00（本地）行，start=end=今日（date-only）→ 行必须返回
// （旧实现 end 解析为当日 00:00，`created_at <= ?` 会漏掉整个 end 当日）。
func TestAdminLogs_DateOnlyEndIncludesFullDay(t *testing.T) {
	r, db := newAdminRouter(t)
	logs := repository.NewConversionLogRepository(db)

	now := time.Now()
	at := time.Date(now.Year(), now.Month(), now.Day(), 15, 0, 0, 0, time.Local)
	rec := &model.ConversionLog{
		Endpoint: "parse", Status: 1, InputText: "tkl",
		CreatedAt: at,
	}
	if err := logs.Append(t.Context(), rec); err != nil {
		t.Fatal(err)
	}
	// 钉死 seed：跳过 gorm 自动回填，保证 created_at 就是今日 15:00 本地墙钟。
	if err := db.Model(&model.ConversionLog{}).
		Where("id = ?", rec.ID).UpdateColumn("created_at", at).Error; err != nil {
		t.Fatal(err)
	}

	day := at.Format("2006-01-02")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/logs?start="+day+"&end="+day, nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("应 200: status=%d body=%s", w.Code, w.Body.String())
	}
	var env struct {
		Code int `json:"code"`
		Data struct {
			List  []map[string]any `json:"list"`
			Total int64            `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("解析失败: %v body=%s", err, w.Body.String())
	}
	if env.Code != 0 || env.Data.Total != 1 || len(env.Data.List) != 1 {
		t.Fatalf("date-only 同日区间应含当日 15:00 行: code=%d total=%d list=%d body=%s",
			env.Code, env.Data.Total, len(env.Data.List), w.Body.String())
	}
}

// itoa uint64 → 十进制串（测试小工具）。
func itoa(id uint64) string {
	if id == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for id > 0 {
		i--
		buf[i] = byte('0' + id%10)
		id /= 10
	}
	return string(buf[i:])
}
