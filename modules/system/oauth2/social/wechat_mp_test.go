package social

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newMP(t *testing.T, handler http.HandlerFunc) (*WechatMP, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &WechatMP{baseURL: srv.URL, client: srv.Client()}, srv
}

func TestWechatMP_ExchangeOK(t *testing.T) {
	var gotQuery string
	mp, _ := newMP(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sns/jscode2session" {
			t.Errorf("path = %s", r.URL.Path)
		}
		gotQuery = r.URL.RawQuery
		w.Write([]byte(`{"openid":"OID","unionid":"UID","session_key":"k"}`))
	})
	idn, err := mp.Exchange(context.Background(), &SocialApp{AppID: "my-app", Secret: "my-secret"}, "code-1")
	if err != nil {
		t.Fatal(err)
	}
	if idn.OpenID != "OID" || idn.UnionID != "UID" {
		t.Errorf("Identity = %+v", idn)
	}
	for _, want := range []string{"appid=my-app", "secret=my-secret", "js_code=code-1", "grant_type=authorization_code"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query 缺 %q: %s", want, gotQuery)
		}
	}
}

func TestWechatMP_ExchangeErrcode(t *testing.T) {
	mp, _ := newMP(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errcode":40029,"errmsg":"invalid code"}`))
	})
	if _, err := mp.Exchange(context.Background(), &SocialApp{AppID: "a", Secret: "s"}, "bad"); err == nil {
		t.Error("errcode≠0 应返回 error（Review Focus #2）")
	}
}

func TestWechatMP_ExchangeEmptyOpenID(t *testing.T) {
	mp, _ := newMP(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"unionid":"UID"}`))
	})
	if _, err := mp.Exchange(context.Background(), &SocialApp{AppID: "a", Secret: "s"}, "c"); err == nil {
		t.Error("空 openid 应返回 error")
	}
}
