package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestWechat(t *testing.T, handler http.HandlerFunc) *WechatClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	cli := NewWechatClient("wx-test-appid", "wx-test-secret")
	cli.baseURL = srv.URL
	return cli
}

func TestWechatClient_Exchange_Success(t *testing.T) {
	cli := newTestWechat(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sns/jscode2session" {
			t.Errorf("path = %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("appid") != "wx-test-appid" || q.Get("js_code") != "code-1" {
			t.Errorf("query = %v", q)
		}
		w.Write([]byte(`{"openid":"openid-xyz","session_key":"sk"}`))
	})
	openid, err := cli.Exchange(t.Context(), "code-1")
	if err != nil {
		t.Fatal(err)
	}
	if openid != "openid-xyz" {
		t.Fatalf("openid = %q", openid)
	}
}

func TestWechatClient_Exchange_ErrCode(t *testing.T) {
	cli := newTestWechat(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errcode":40029,"errmsg":"invalid code"}`))
	})
	_, err := cli.Exchange(t.Context(), "bad")
	if err == nil {
		t.Fatal("errcode≠0 应返回 error")
	}
}

func TestWechatClient_Exchange_Timeout(t *testing.T) {
	cli := newTestWechat(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Write([]byte(`{"openid":"late"}`))
	})
	cli.client.Timeout = 50 * time.Millisecond
	_, err := cli.Exchange(t.Context(), "slow")
	if err == nil {
		t.Fatal("超时应返回 error")
	}
}
