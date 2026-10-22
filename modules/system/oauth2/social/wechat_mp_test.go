package social

import (
	"context"
	"encoding/json"
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

// ---- Task 2：StableToken / getwxacodeunlimit（小程序码）----

// roundTripFunc 让测试能内联构造 http.RoundTripper。
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// TestStableToken_CachesAndReturns 200 → token；按 appid 进程内缓存，
// 二次调用不再打 /cgi-bin/stable_token（计数断言）。
func TestStableToken_CachesAndReturns(t *testing.T) {
	var hits int
	mp, _ := newMP(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cgi-bin/stable_token" {
			t.Errorf("path = %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		hits++
		var body struct {
			GrantType string `json:"grant_type"`
			AppID     string `json:"appid"`
			Secret    string `json:"secret"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if body.GrantType != "client_credential" || body.AppID != "my-app" || body.Secret != "my-secret" {
			t.Errorf("body = %+v", body)
		}
		w.Write([]byte(`{"access_token":"AT-1","expires_in":7200}`))
	})
	app := &SocialApp{AppID: "my-app", Secret: "my-secret"}
	tok, err := mp.StableToken(context.Background(), app)
	if err != nil {
		t.Fatal(err)
	}
	if tok != "AT-1" {
		t.Fatalf("token = %q, want AT-1", tok)
	}
	tok2, err := mp.StableToken(context.Background(), app)
	if err != nil {
		t.Fatal(err)
	}
	if tok2 != "AT-1" {
		t.Fatalf("cached token = %q, want AT-1", tok2)
	}
	if hits != 1 {
		t.Fatalf("二次调用应命中进程内缓存: stable_token hits = %d", hits)
	}
}

// TestWxacode_ReturnsPNG 假 server 返回 PNG 字节 → 原样透传（含请求参数透传断言）。
func TestWxacode_ReturnsPNG(t *testing.T) {
	const png = "\x89PNG\r\n\x1a\nfake-png-bytes"
	var reqBody struct {
		Scene      string `json:"scene"`
		Page       string `json:"page"`
		EnvVersion string `json:"env_version"`
		CheckPath  bool   `json:"check_path"`
	}
	var gotToken string
	mp, _ := newMP(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/stable_token":
			w.Write([]byte(`{"access_token":"AT-1","expires_in":7200}`))
		case "/wxa/getwxacodeunlimit":
			gotToken = r.URL.Query().Get("access_token")
			if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
				t.Errorf("decode body: %v", err)
			}
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte(png))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	got, err := mp.Wxacode(context.Background(), &SocialApp{AppID: "my-app", Secret: "my-secret"},
		"qr=ABCD", "pages/index/index", "trial")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != png {
		t.Fatalf("PNG 字节应原样透传: %q", got)
	}
	if reqBody.Scene != "qr=ABCD" || reqBody.Page != "pages/index/index" || reqBody.EnvVersion != "trial" {
		t.Errorf("body args = %+v", reqBody)
	}
	if reqBody.CheckPath {
		t.Errorf("check_path 应为 false: %+v", reqBody)
	}
	if gotToken != "AT-1" {
		t.Errorf("access_token query = %q", gotToken)
	}
}

// TestWxacode_WechatErr_MapsError errcode≠0 → error 上抛（不泄 secret）。
func TestWxacode_WechatErr_MapsError(t *testing.T) {
	const secret = "secret-errcode-XYZ"
	mp, _ := newMP(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/stable_token":
			w.Write([]byte(`{"access_token":"AT-1","expires_in":7200}`))
		case "/wxa/getwxacodeunlimit":
			w.Write([]byte(`{"errcode":41030,"errmsg":"invalid page"}`))
		}
	})
	_, err := mp.Wxacode(context.Background(), &SocialApp{AppID: "a", Secret: secret},
		"qr=ABCD", "pages/index/index", "trial")
	if err == nil {
		t.Fatal("errcode≠0 应上抛 error")
	}
	if !strings.Contains(err.Error(), "errcode=41030") {
		t.Errorf("error 应带 errcode: %v", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("error 泄露 secret: %v", err)
	}
}

// TestWxacode_SecretNotInError getwxacodeunlimit 传输层失败（*url.Error 含
// access_token 查询串 URL）→ 沿用 wechat_mp.go 剥离先例，error 串不含 secret/URL。
func TestWxacode_SecretNotInError(t *testing.T) {
	const secret = "super-secret-WX-ABC"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cgi-bin/stable_token" {
			t.Errorf("stable_token 假 server 不应被打: %s", r.URL.Path)
		}
		w.Write([]byte(`{"access_token":"AT-1","expires_in":7200}`))
	}))
	t.Cleanup(srv.Close)
	mp := &WechatMP{
		baseURL: srv.URL,
		client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path == "/wxa/getwxacodeunlimit" {
				return nil, errors.New("dial tcp: connection refused")
			}
			return http.DefaultTransport.RoundTrip(req)
		})},
	}
	_, err := mp.Wxacode(context.Background(), &SocialApp{AppID: "my-app", Secret: secret},
		"qr=ABCD", "pages/index/index", "trial")
	if err == nil {
		t.Fatal("传输层失败应返回 error")
	}
	msg := err.Error()
	if strings.Contains(msg, secret) {
		t.Errorf("error 泄露 secret: %s", msg)
	}
	if strings.Contains(msg, "access_token=") {
		t.Errorf("error 含 access_token 查询串: %s", msg)
	}
	if !strings.Contains(msg, "request failed (network)") {
		t.Errorf("error 应标明 network 失败: %s", msg)
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
