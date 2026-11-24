package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const (
	hkPID      = "mm_test_pid"
	hkFormKey  = "test-hk-key"
	hkTBName   = "测试昵称"
	hkItemID   = "123456789012"
	hkItemDesc = "面料舒服不掉色"
	hkClickURL = "https://s.click.taobao.com/e?pid=mm_test_pid"
)

func newHaodankuTestClient(baseURL string, maxRetries int) *HaodankuClient {
	return NewHaodankuClient(baseURL, 5*time.Second, maxRetries)
}

// TestHaodanku_RatesSuccess：POST /ratesurl 表单含四键，code=1 → 返回链接与 couponmoney=30。
func TestHaodanku_RatesSuccess(t *testing.T) {
	var count atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/ratesurl" {
			t.Errorf("path = %q, want /ratesurl", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q, want application/x-www-form-urlencoded", ct)
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm err = %v", err)
			return
		}
		want := map[string]string{
			"pid":     hkPID,
			"apikey":  hkFormKey,
			"tb_name": hkTBName,
			"itemid":  hkItemID,
		}
		for k, v := range want {
			if got := r.PostForm.Get(k); got != v {
				t.Errorf("form[%q] = %q, want %q", k, got, v)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"data":{"coupon_click_url":"` + hkClickURL + `","couponmoney":30}}`))
	}))
	defer srv.Close()

	clickURL, coupon, err := newHaodankuTestClient(srv.URL, 1).RatesURL(
		context.Background(), hkPID, hkFormKey, hkTBName, hkItemID)
	if err != nil {
		t.Fatalf("RatesURL err = %v", err)
	}
	if clickURL != hkClickURL {
		t.Errorf("clickURL = %q, want %q", clickURL, hkClickURL)
	}
	if coupon != 30 {
		t.Errorf("coupon = %d, want 30", coupon)
	}
	if n := count.Load(); n != 1 {
		t.Errorf("request count = %d, want 1", n)
	}
}

// TestHaodanku_RatesCodeZero：code=0 → ErrNoRates，计数==1（业务失败不重试）。
func TestHaodanku_RatesCodeZero(t *testing.T) {
	var count atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0}`))
	}))
	defer srv.Close()

	_, _, err := newHaodankuTestClient(srv.URL, 1).RatesURL(
		context.Background(), hkPID, hkFormKey, hkTBName, hkItemID)
	if !errors.Is(err, ErrNoRates) {
		t.Errorf("err = %v, want ErrNoRates", err)
	}
	if n := count.Load(); n != 1 {
		t.Errorf("request count = %d, want 1（业务失败不重试）", n)
	}
}

// TestHaodanku_ItemDesc：正常取 data.itemdesc；连续 500 → err 且计数==2（HTTPHelper 重试一次）。
func TestHaodanku_ItemDesc(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var count atomic.Int64
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count.Add(1)
			if r.Method != http.MethodGet {
				t.Errorf("method = %s, want GET", r.Method)
			}
			// 原项目固定 HAODANKYAPIKEY（index.php define）——路径段钉死。
			if r.URL.Path != "/item_detail/apikey/vate/itemid/"+hkItemID {
				t.Errorf("path = %q, want /item_detail/apikey/vate/itemid/%s", r.URL.Path, hkItemID)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"itemdesc":"` + hkItemDesc + `"}}`))
		}))
		defer srv.Close()

		desc, err := newHaodankuTestClient(srv.URL, 1).ItemDesc(context.Background(), hkItemID)
		if err != nil {
			t.Fatalf("ItemDesc err = %v", err)
		}
		if desc != hkItemDesc {
			t.Errorf("desc = %q, want %q", desc, hkItemDesc)
		}
		if n := count.Load(); n != 1 {
			t.Errorf("request count = %d, want 1", n)
		}
	})

	t.Run("5xx retry once then fail", func(t *testing.T) {
		var count atomic.Int64
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count.Add(1)
			http.Error(w, "boom", http.StatusInternalServerError)
		}))
		defer srv.Close()

		_, err := newHaodankuTestClient(srv.URL, 1).ItemDesc(context.Background(), hkItemID)
		if err == nil {
			t.Fatal("ItemDesc err = nil, want error after retry exhausted")
		}
		if errors.Is(err, ErrNoRates) {
			t.Errorf("err = %v, 不应为换链业务错误 ErrNoRates（5xx 属传输层失败）", err)
		}
		if !strings.Contains(err.Error(), "500") {
			t.Errorf("err = %v, want 含 HTTP 状态码 500", err)
		}
		if n := count.Load(); n != 2 {
			t.Errorf("request count = %d, want 2", n)
		}
	})
}
