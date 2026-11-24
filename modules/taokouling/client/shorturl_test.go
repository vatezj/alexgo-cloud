package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

const (
	suAPIKey   = "test-su-key"
	suLongURL  = "https://www.taoquan.ink/pages/search/search_info?keyword=测试"
	suShortURL = "http://simo.im/x"
)

func newShortURLTestClient(baseURL string) *ShortURLClient {
	return NewShortURLClient(baseURL, suAPIKey, 3*time.Second)
}

// TestShortURL_ShortenSuccess：GET 带 format=json/url/apikey → 返回 {"url":...}。
func TestShortURL_ShortenSuccess(t *testing.T) {
	var count atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		q := r.URL.Query()
		if got := q.Get("format"); got != "json" {
			t.Errorf("format = %q, want json", got)
		}
		if got := q.Get("url"); got != suLongURL {
			t.Errorf("url = %q, want %q", got, suLongURL)
		}
		if got := q.Get("apikey"); got != suAPIKey {
			t.Errorf("apikey = %q, want %q", got, suAPIKey)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"url":"` + suShortURL + `"}`))
	}))
	defer srv.Close()

	got, err := newShortURLTestClient(srv.URL).Shorten(context.Background(), suLongURL)
	if err != nil {
		t.Fatalf("Shorten err = %v", err)
	}
	if got != suShortURL {
		t.Errorf("shortURL = %q, want %q", got, suShortURL)
	}
	if n := count.Load(); n != 1 {
		t.Errorf("request count = %d, want 1", n)
	}
}

// TestShortURL_MissingURLField：响应缺 url 字段 → err。
func TestShortURL_MissingURLField(t *testing.T) {
	var count atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"msg":"ok"}`))
	}))
	defer srv.Close()

	_, err := newShortURLTestClient(srv.URL).Shorten(context.Background(), suLongURL)
	if err == nil {
		t.Fatal("Shorten err = nil, want error on missing url field")
	}
	if n := count.Load(); n != 1 {
		t.Errorf("request count = %d, want 1", n)
	}
}
