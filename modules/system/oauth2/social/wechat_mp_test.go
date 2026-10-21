package social

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// errTransport 恒定返回传输层错误，模拟 DNS/连接失败。
// http.Client 会把它包成 *url.Error（含完整请求 URL），用于验证错误文本不泄 secret。
type errTransport struct{ err error }

func (e errTransport) RoundTrip(*http.Request) (*http.Response, error) { return nil, e.err }

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

// 终审 Important：client.Do 失败时 *url.Error 文本含完整 URL（含 secret），
// 修复后错误只保留内层网络错误，不得出现 secret 值或完整 query URL。
func TestWechatMP_ExchangeTransportErrorNoSecretLeak(t *testing.T) {
	const secret = "super-secret-MP-XYZ"
	mp := &WechatMP{
		baseURL: "https://api.weixin.qq.com",
		client:  &http.Client{Transport: errTransport{err: errors.New("dial tcp: connection refused")}},
	}
	_, err := mp.Exchange(context.Background(), &SocialApp{AppID: "my-app", Secret: secret}, "code-1")
	if err == nil {
		t.Fatal("传输层失败应返回 error")
	}
	msg := err.Error()
	if strings.Contains(msg, secret) {
		t.Errorf("error 泄露 secret: %s", msg)
	}
	if strings.Contains(msg, "secret=") {
		t.Errorf("error 含 query 参数: %s", msg)
	}
	// 保留网络故障诊断信息
	if !strings.Contains(msg, "request failed (network)") {
		t.Errorf("error 应标明 network 失败: %s", msg)
	}
}
