package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx/fxtest"

	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/job"
)

// newJobRouter 装 /admin/jobs 两路由（与 module.go RegisterRoutes 同形作回归）。
// 不挂 auth 中间件：用例只验 handler 信封与参数语义；鉴权由 casbin 中间件全局负责。
func newJobRouter(t *testing.T, m *job.Manager) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	ctrl := NewJobController(m)
	g := r.Group("/api")
	g.GET("/admin/jobs", ctrl.List)
	g.POST("/admin/jobs/:name/trigger", ctrl.Trigger)
	return r
}

func doJobReq(r *gin.Engine, method, path string) (*httptest.ResponseRecorder, envelope) {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(method, path, nil))
	var env envelope
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	return w, env
}

// TestJobList_ReturnsRegistered 列表返回已注册任务（统一信封 code=0 + 数组 data）。
func TestJobList_ReturnsRegistered(t *testing.T) {
	m := job.NewManager(&config.Config{Modules: map[string]bool{"demo": true}})
	m.Register("demo.a", "@every 1m", func(context.Context) error { return nil })
	w, env := doJobReq(newJobRouter(t, m), http.MethodGet, "/api/admin/jobs")
	if w.Code != http.StatusOK || env.Code != 0 {
		t.Fatalf("GET /admin/jobs = %d/%d, want 200/0", w.Code, env.Code)
	}
	var list []job.Status
	if err := json.Unmarshal(env.Data, &list); err != nil {
		t.Fatalf("data 解码失败: %v", err)
	}
	if len(list) != 1 || list[0].Name != "demo.a" {
		t.Fatalf("list = %+v, want 含 demo.a", list)
	}
}

// TestJobTrigger_NotFound30601 未注册 name → 信封 code=30601。
func TestJobTrigger_NotFound30601(t *testing.T) {
	m := job.NewManager(&config.Config{Modules: map[string]bool{"demo": true}})
	w, env := doJobReq(newJobRouter(t, m), http.MethodPost, "/api/admin/jobs/nope/trigger")
	if env.Code != 30601 {
		t.Fatalf("trigger 未注册 = %d/%d, want 200/30601", w.Code, env.Code)
	}
}

// TestJobTrigger_OK 同步执行一次 → code=0 且回显 name。
func TestJobTrigger_OK(t *testing.T) {
	m := job.NewManager(&config.Config{Modules: map[string]bool{"demo": true}})
	called := 0
	m.Register("demo.a", "@every 1m", func(context.Context) error { called++; return nil })
	// 需启动才有 enabled；用 BindLifecycle + fxtest 起停。
	lc := fxtest.NewLifecycle(t)
	job.BindLifecycle(lc, m)
	lc.RequireStart()
	defer lc.RequireStop()

	w, env := doJobReq(newJobRouter(t, m), http.MethodPost, "/api/admin/jobs/demo.a/trigger")
	if w.Code != http.StatusOK || env.Code != 0 {
		t.Fatalf("trigger = %d/%d, want 200/0", w.Code, env.Code)
	}
	if called != 1 {
		t.Fatalf("called = %d, want 1", called)
	}
}
