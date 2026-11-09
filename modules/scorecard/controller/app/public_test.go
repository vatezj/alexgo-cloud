package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/scorecard/service"
	sysmodel "alexGo-cloud/modules/system/model"
	sysrepo "alexGo-cloud/modules/system/repository"
	syssvc "alexGo-cloud/modules/system/service"
)

// TestPublicController_ShareConfig 免鉴权端点：空库返回三页内置默认（信封 code=0，
// data 键 = index/simulator/salary，各含 title/imageUrl）。
func TestPublicController_ShareConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&sysmodel.SystemConfig{}); err != nil {
		t.Fatal(err)
	}
	shareCfg := service.NewShareConfigService(syssvc.NewConfigService(sysrepo.NewConfigRepository(db)))

	r := gin.New()
	r.GET("/api/public/scorecard/share-config", NewPublicController(shareCfg).ShareConfig)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/public/scorecard/share-config", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP %d, body=%s", w.Code, w.Body.String())
	}

	var body struct {
		Code int `json:"code"`
		Data struct {
			Index     map[string]string `json:"index"`
			Simulator map[string]string `json:"simulator"`
			Salary    map[string]string `json:"salary"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v, body=%s", err, w.Body.String())
	}
	if body.Code != 0 {
		t.Fatalf("code 应为 0，实际 %d", body.Code)
	}
	for page, m := range map[string]map[string]string{
		"index": body.Data.Index, "simulator": body.Data.Simulator, "salary": body.Data.Salary,
	} {
		if m == nil || m["title"] == "" {
			t.Errorf("page=%s 缺 title: %+v", page, m)
		}
		if _, ok := m["imageUrl"]; !ok {
			t.Errorf("page=%s 缺 imageUrl 键（空串可接受，键不能缺）", page)
		}
	}
}
