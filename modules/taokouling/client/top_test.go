package client

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const (
	topTestAppkey = "25922261"
	topTestSecret = "s3cret"
	topTestUserID = "1060631727"
)

func newTestTopClient(baseURL string, maxRetries int) *TopClient {
	return NewTopClient(baseURL, topTestAppkey, topTestSecret, topTestUserID, 5*time.Second, maxRetries)
}

// TestTopSign_DeterministicAndOrdered：签名按 key 字节升序拼 key+value（value 原始串不 urlencode），
// sign = md5(secret + 拼接串 + secret) 小写 hex；sign 键自身剔除不参与。
func TestTopSign_DeterministicAndOrdered(t *testing.T) {
	params := map[string]string{
		"sys":    "taobao.tbk.tpwd.create",
		"appkey": "25922261",
		"ext":    "{}",
		"zeta":   "1",
		"sign":   "IGNORED-MUST-NOT-PARTICIPATE",
	}
	// 手写拼接串字面量：key 按字节升序 appkey < ext < sys < zeta（sign 已剔除，不出现）。
	// 每项 = key+value 原始串直拼、无分隔符（如 "sys"+"taobao.tbk.tpwd.create"）。
	concat := "appkey25922261" +
		"ext{}" +
		"systaobao.tbk.tpwd.create" +
		"zeta1"
	sum := md5.Sum([]byte(topTestSecret + concat + topTestSecret))
	want := hex.EncodeToString(sum[:])

	if got := topSign(params, topTestSecret); got != want {
		t.Errorf("topSign = %q, want %q", got, want)
	}

	// sign 键即使存在于 map 中也不参与：与剔除 sign 的 map 结果一致。
	delete(params, "sign")
	if got := topSign(params, topTestSecret); got != want {
		t.Errorf("topSign（剔除 sign 后）= %q, want %q", got, want)
	}
}

// TestTop_TpwdCreateSuccess：请求体为 x-www-form-urlencoded 且含 sys/sign/ext；data.model 即返回值。
func TestTop_TpwdCreateSuccess(t *testing.T) {
	var count atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); !strings.Contains(ct, "application/x-www-form-urlencoded") {
			t.Errorf("Content-Type = %q, want application/x-www-form-urlencoded", ct)
		}
		raw, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(raw), "sys=taobao.tbk.tpwd.create") {
			t.Errorf("body 缺少 sys=taobao.tbk.tpwd.create: %s", raw)
		}
		if !strings.Contains(string(raw), "ext=%7B%7D") {
			t.Errorf("body 缺少 ext=%%7B%%7D: %s", raw)
		}
		form, err := url.ParseQuery(string(raw))
		if err != nil {
			t.Fatalf("解析请求体失败: %v", err)
		}
		if form.Get("sign") == "" {
			t.Error("sign 缺失或为空")
		}
		if form.Get("appkey") != topTestAppkey {
			t.Errorf("appkey = %q, want %q", form.Get("appkey"), topTestAppkey)
		}
		if form.Get("user_id") != topTestUserID {
			t.Errorf("user_id = %q, want %q", form.Get("user_id"), topTestUserID)
		}
		if form.Get("text") != "好物推荐" {
			t.Errorf("text = %q", form.Get("text"))
		}
		if form.Get("url") != "https://s.click.taobao.com/abc" {
			t.Errorf("url = %q", form.Get("url"))
		}
		if form.Get("logo") != "https://img.example/logo.jpg" {
			t.Errorf("logo = %q", form.Get("logo"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"model":"￥abc123￥"}}`))
	}))
	defer srv.Close()

	got, err := newTestTopClient(srv.URL, 1).TpwdCreate(
		context.Background(), "好物推荐", "https://s.click.taobao.com/abc", "https://img.example/logo.jpg")
	if err != nil {
		t.Fatalf("TpwdCreate err = %v", err)
	}
	if got != "￥abc123￥" {
		t.Errorf("model = %q, want %q", got, "￥abc123￥")
	}
	if n := count.Load(); n != 1 {
		t.Errorf("request count = %d, want 1", n)
	}
}

// TestTop_TpwdCreateMissingModel：缺 data.model → 业务失败 error，maxRetries=1 下计数 == 1（不重试）。
func TestTop_TpwdCreateMissingModel(t *testing.T) {
	var count atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer srv.Close()

	_, err := newTestTopClient(srv.URL, 1).TpwdCreate(
		context.Background(), "t", "https://s.click.taobao.com/abc", "")
	if err == nil {
		t.Fatal("TpwdCreate err = nil, want error（缺 data.model）")
	}
	if n := count.Load(); n != 1 {
		t.Errorf("request count = %d, want 1（业务失败不重试）", n)
	}
}

// TestTop_TpwdCreateOmitEmptyLogo：logo 为空时请求体不携带 logo 键（也不进签名集合）。
func TestTop_TpwdCreateOmitEmptyLogo(t *testing.T) {
	var count atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		raw, _ := io.ReadAll(r.Body)
		form, err := url.ParseQuery(string(raw))
		if err != nil {
			t.Fatalf("解析请求体失败: %v", err)
		}
		if _, ok := form["logo"]; ok {
			t.Errorf("logo 为空时不应出现在请求体: %s", raw)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"model":"￥xyz￥"}}`))
	}))
	defer srv.Close()

	got, err := newTestTopClient(srv.URL, 1).TpwdCreate(
		context.Background(), "t", "https://s.click.taobao.com/abc", "")
	if err != nil {
		t.Fatalf("TpwdCreate err = %v", err)
	}
	if got != "￥xyz￥" {
		t.Errorf("model = %q", got)
	}
	if n := count.Load(); n != 1 {
		t.Errorf("request count = %d, want 1", n)
	}
}

// TestTop_ItemInfoSuccess：spec §7.2 形状 results.n_tbk_item 数组一项 → Item；
// 价格兼容 JSON 字符串与数字；裸 results（无外层包装）与空数组一并覆盖。
func TestTop_ItemInfoSuccess(t *testing.T) {
	// 真实 TOP 形状：外层 tbk_item_info_get_response 包装；价格一个字符串一个数字。
	const wrappedPayload = `{"tbk_item_info_get_response":{"results":{"n_tbk_item":[` +
		`{"title":"标题","pict_url":"https://img.example/p.jpg","zk_final_price":"19.9","reserve_price":29.9}]}}}`

	var count atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		raw, _ := io.ReadAll(r.Body)
		form, err := url.ParseQuery(string(raw))
		if err != nil {
			t.Fatalf("解析请求体失败: %v", err)
		}
		if form.Get("sys") != "taobao.tbk.item.info.get" {
			t.Errorf("sys = %q", form.Get("sys"))
		}
		if form.Get("num_iids") != "1234567890" {
			t.Errorf("num_iids = %q", form.Get("num_iids"))
		}
		if form.Get("platform") != "1" {
			t.Errorf("platform = %q", form.Get("platform"))
		}
		if form.Get("ip") != "203.0.113.7" {
			t.Errorf("ip = %q", form.Get("ip"))
		}
		if form.Get("sign") == "" {
			t.Error("sign 缺失或为空")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(wrappedPayload))
	}))
	defer srv.Close()

	item, err := newTestTopClient(srv.URL, 1).ItemInfo(context.Background(), "1234567890", "203.0.113.7")
	if err != nil {
		t.Fatalf("ItemInfo err = %v", err)
	}
	want := Item{Title: "标题", PictURL: "https://img.example/p.jpg", ZkPrice: 19.9, ReservePrice: 29.9}
	if item != want {
		t.Errorf("Item = %+v, want %+v", item, want)
	}
	if n := count.Load(); n != 1 {
		t.Errorf("request count = %d, want 1", n)
	}

	t.Run("裸 results 无外层包装", func(t *testing.T) {
		srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"results":{"n_tbk_item":[` +
				`{"title":"标题","pict_url":"https://img.example/p.jpg","zk_final_price":19.9,"reserve_price":"29.9"}]}}`))
		}))
		defer srv2.Close()

		got, err := newTestTopClient(srv2.URL, 0).ItemInfo(context.Background(), "1234567890", "203.0.113.7")
		if err != nil {
			t.Fatalf("ItemInfo err = %v", err)
		}
		if got != want {
			t.Errorf("Item = %+v, want %+v", got, want)
		}
	})

	t.Run("空数组 → error", func(t *testing.T) {
		srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"results":{"n_tbk_item":[]}}`))
		}))
		defer srv2.Close()

		if _, err := newTestTopClient(srv2.URL, 1).ItemInfo(context.Background(), "1234567890", "203.0.113.7"); err == nil {
			t.Fatal("ItemInfo err = nil, want error（空数组）")
		}
	})
}
