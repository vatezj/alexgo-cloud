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
