package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/pkg/config"
)

func TestPprof_NotMountedByDefault(t *testing.T) {
	cfg := &config.Config{} // pprof_enabled 零值 = false
	r := newRouter(HTTPServerParams{Cfg: cfg})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/debug/pprof/", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 when pprof disabled", w.Code)
	}
}

func TestPprof_MountedWhenEnabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.PprofEnabled = true
	r := newRouter(HTTPServerParams{Cfg: cfg})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/debug/pprof/cmdline", nil))
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 when pprof enabled", w.Code)
	}
}

// swaggerModules：每模块独立 spec 实例的挂载清单（与 http.go swaggerSpecs 保持一致）。
var swaggerModules = []string{"fishgame", "system", "scorecard", "member", "order"}

// swagger：swagger_enabled 零值 = false → 不挂载（404）。
func TestSwagger_NotMountedByDefault(t *testing.T) {
	cfg := &config.Config{} // swagger_enabled 零值 = false
	r := newRouter(HTTPServerParams{Cfg: cfg})

	for _, m := range swaggerModules {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/swagger/"+m+"/index.html", nil))
		if w.Code != http.StatusNotFound {
			t.Errorf("/swagger/%s/index.html status = %d, want 404 when swagger disabled", m, w.Code)
		}
	}
}

// swagger：显式开启 → 每模块 UI 页面 200（与 pprof 同款开关模式）。
func TestSwagger_ModuleMountedWhenEnabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.SwaggerEnabled = true
	r := newRouter(HTTPServerParams{Cfg: cfg})

	for _, m := range swaggerModules {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/swagger/"+m+"/index.html", nil))
		if w.Code != http.StatusOK {
			t.Errorf("/swagger/%s/index.html status = %d, want 200 when swagger enabled", m, w.Code)
		}
	}
}

// swagger：实例隔离——各模块 doc.json 只含自己的 tag，互不串味。
func TestSwagger_ModuleSpecIsolated(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.SwaggerEnabled = true
	r := newRouter(HTTPServerParams{Cfg: cfg})

	cases := []struct {
		module   string
		wantTag  string
		wantNone string
	}{
		{"fishgame", "fishgame-admin", "system-admin"},
		{"system", "system-admin", "fishgame-admin"},
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/swagger/"+c.module+"/doc.json", nil))
		if w.Code != http.StatusOK {
			t.Errorf("/swagger/%s/doc.json status = %d, want 200", c.module, w.Code)
			continue
		}
		body := w.Body.String()
		if !strings.Contains(body, c.wantTag) {
			t.Errorf("/swagger/%s/doc.json missing tag %q", c.module, c.wantTag)
		}
		if strings.Contains(body, c.wantNone) {
			t.Errorf("/swagger/%s/doc.json unexpectedly contains tag %q", c.module, c.wantNone)
		}
	}
}

// swagger：/swagger 根路径重定向到首个模块 UI。
func TestSwagger_RootRedirectsToFirstModule(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.SwaggerEnabled = true
	r := newRouter(HTTPServerParams{Cfg: cfg})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/swagger", nil))
	if w.Code != http.StatusFound {
		t.Errorf("/swagger status = %d, want 302 redirect", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/swagger/fishgame/index.html" {
		t.Errorf("Location = %q, want /swagger/fishgame/index.html", loc)
	}
}

// liveness：不依赖 DB，永远 200（进程活着即可）。
func TestHealth_LivenessWithoutDB(t *testing.T) {
	r := newRouter(HTTPServerParams{Cfg: &config.Config{}})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/health", nil))
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

// readiness：DB 未配置（nil）→ 200 unconfigured（无依赖可探，不阻塞启动）。
func TestHealth_ReadyWithoutDB(t *testing.T) {
	r := newRouter(HTTPServerParams{Cfg: &config.Config{}})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/health/ready", nil))
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if got := w.Body.String(); !strings.Contains(got, "unconfigured") {
		t.Errorf("body = %s, want contain \"unconfigured\"", got)
	}
}

// readiness：DB 正常 → 200。
func TestHealth_ReadyWithLiveDB(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	r := newRouter(HTTPServerParams{Cfg: &config.Config{}, DB: db})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/health/ready", nil))
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
}

// readiness：DB 连接已关闭 → 503（K8s 摘流，但 liveness 仍 200 不触发重启）。
func TestHealth_ReadyWhenDBDown(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	_ = sqlDB.Close()

	r := newRouter(HTTPServerParams{Cfg: &config.Config{}, DB: db})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/health/ready", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}

	// 同一时刻 liveness 仍必须 200。
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest("GET", "/health", nil))
	if w2.Code != http.StatusOK {
		t.Errorf("liveness status = %d, want 200 while db down", w2.Code)
	}
}
