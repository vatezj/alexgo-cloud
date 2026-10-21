package social

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWechatWeb_ExchangeOK(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sns/oauth2/access_token" {
			t.Errorf("path = %s", r.URL.Path)
		}
		gotQuery = r.URL.RawQuery
		w.Write([]byte(`{"access_token":"AT","openid":"OID","unionid":"UID"}`))
	}))
	defer srv.Close()
	web := &WechatWeb{baseURL: srv.URL, client: srv.Client()}
	idn, err := web.Exchange(context.Background(), &SocialApp{AppID: "a", Secret: "s"}, "code-1")
	if err != nil {
		t.Fatal(err)
	}
	if idn.OpenID != "OID" || idn.UnionID != "UID" {
		t.Errorf("Identity = %+v", idn)
	}
	for _, want := range []string{"appid=a", "secret=s", "code=code-1", "grant_type=authorization_code"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query 缺 %q: %s", want, gotQuery)
		}
	}
}

func TestWechatWeb_ExchangeErrcode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errcode":40029,"errmsg":"invalid code"}`))
	}))
	defer srv.Close()
	web := &WechatWeb{baseURL: srv.URL, client: srv.Client()}
	if _, err := web.Exchange(context.Background(), &SocialApp{AppID: "a", Secret: "s"}, "bad"); err == nil {
		t.Error("errcode≠0 应返回 error（Review Focus #2）")
	}
}

func TestAdapterRegistry_HasBothWechat(t *testing.T) {
	r := NewAdapterRegistry()
	if !r.Exists(PlatformWechatMP) || !r.Exists(PlatformWechatWeb) {
		t.Error("registry 应含微信两平台")
	}
}
