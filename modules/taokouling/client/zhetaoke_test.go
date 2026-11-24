package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

const (
	testAppkey = "test-appkey"
	testSID    = "test-sid"
	testPID    = "test-pid"
	testTkl    = "6fj0Y1HoO3U"
)

// successPayload 为 spec §4.2 A 结构：item_id 是 JSON number。
const successPayload = `{"tbk_privilege_get_response":{"result":{"data":{"item_id":123456789012}}}}`

const errorPayload = `{"error_response":{"code":"10001","msg":"无效的appkey参数"}}`

func newTestClient(baseURL string, maxRetries int) *ZhetaokeClient {
	return NewZhetaokeClient(baseURL, testAppkey, testSID, testPID, 5*time.Second, maxRetries)
}

func TestZhetaoke_ParseSuccess(t *testing.T) {
	var count atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/open_gaoyongzhuanlian_tkl.ashx" {
			t.Errorf("path = %q, want /api/open_gaoyongzhuanlian_tkl.ashx", r.URL.Path)
		}
		q := r.URL.Query()
		if got := q.Get("signurl"); got != "0" {
			t.Errorf("signurl = %q, want \"0\"", got)
		}
		if got := q.Get("tkl"); got != testTkl {
			t.Errorf("tkl = %q, want %q", got, testTkl)
		}
		if q.Get("appkey") == "" {
			t.Error("appkey missing")
		}
		if q.Get("sid") == "" {
			t.Error("sid missing")
		}
		if q.Get("pid") == "" {
			t.Error("pid missing")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(successPayload))
	}))
	defer srv.Close()

	itemID, err := newTestClient(srv.URL, 1).ParseTkl(context.Background(), testTkl)
	if err != nil {
		t.Fatalf("ParseTkl err = %v", err)
	}
	if itemID != "123456789012" {
		t.Errorf("itemID = %q, want %q", itemID, "123456789012")
	}
	if n := count.Load(); n != 1 {
		t.Errorf("request count = %d, want 1", n)
	}
}

func TestZhetaoke_ErrorResponse(t *testing.T) {
	var count atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(errorPayload))
	}))
	defer srv.Close()

	_, err := newTestClient(srv.URL, 1).ParseTkl(context.Background(), testTkl)
	if !errors.Is(err, ErrParseFailed) {
		t.Errorf("err = %v, want ErrParseFailed", err)
	}
	if n := count.Load(); n != 1 {
		t.Errorf("request count = %d, want 1（业务失败不重试）", n)
	}
}

func TestZhetaoke_MissingPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	_, err := newTestClient(srv.URL, 1).ParseTkl(context.Background(), testTkl)
	if !errors.Is(err, ErrParseFailed) {
		t.Errorf("err = %v, want ErrParseFailed", err)
	}
}

// TestZhetaoke_RetryOnceOn5xx：第 1 次 500、第 2 次 200 → 成功，请求计数 == 2（首次 + 恰好一次重试）。
func TestZhetaoke_RetryOnceOn5xx(t *testing.T) {
	var count atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if count.Add(1) == 1 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(successPayload))
	}))
	defer srv.Close()

	itemID, err := newTestClient(srv.URL, 1).ParseTkl(context.Background(), testTkl)
	if err != nil {
		t.Fatalf("ParseTkl err = %v, want success after one retry", err)
	}
	if itemID != "123456789012" {
		t.Errorf("itemID = %q, want %q", itemID, "123456789012")
	}
	if n := count.Load(); n != 2 {
		t.Errorf("request count = %d, want 2", n)
	}
}

// TestZhetaoke_FailAfterRetry：连续 500 → 出错，请求计数 == 2（重试在一次后停止）。
func TestZhetaoke_FailAfterRetry(t *testing.T) {
	var count atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := newTestClient(srv.URL, 1).ParseTkl(context.Background(), testTkl)
	if err == nil {
		t.Fatal("ParseTkl err = nil, want error after retry exhausted")
	}
	if errors.Is(err, ErrParseFailed) {
		t.Errorf("err = %v, 不应为业务错误 ErrParseFailed（5xx 属传输层失败）", err)
	}
	if n := count.Load(); n != 2 {
		t.Errorf("request count = %d, want 2", n)
	}
}

// TestZhetaoke_BusinessFailNoRetry：error_response → ErrParseFailed 且计数 == 1。
func TestZhetaoke_BusinessFailNoRetry(t *testing.T) {
	var count atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(errorPayload))
	}))
	defer srv.Close()

	_, err := newTestClient(srv.URL, 1).ParseTkl(context.Background(), testTkl)
	if !errors.Is(err, ErrParseFailed) {
		t.Errorf("err = %v, want ErrParseFailed", err)
	}
	if n := count.Load(); n != 1 {
		t.Errorf("request count = %d, want 1", n)
	}
}
