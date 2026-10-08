package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

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
