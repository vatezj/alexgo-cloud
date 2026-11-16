package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	errs "alexGo-cloud/pkg/errors"
)

// newMPForTest 构造指向 httptest 的 WeChatMPClient。
func newMPForTest(t *testing.T, handler http.HandlerFunc) *wechatMPClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	cli := NewWeChatMPClient("wx-app", "wx-secret").(*wechatMPClient)
	cli.baseURL = srv.URL
	return cli
}

// TestWeChatMPClient_AddDraft_Success 草稿 200 → media_id；请求体为 articles JSON。
func TestWeChatMPClient_AddDraft_Success(t *testing.T) {
	var gotBody string
	cli := newMPForTest(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/cgi-bin/token"):
			_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":7200}`))
		case strings.HasSuffix(r.URL.Path, "/cgi-bin/draft/add"):
			b, _ := io.ReadAll(r.Body)
			gotBody = string(b)
			if r.Header.Get("Content-Type") == "" {
				t.Errorf("草稿请求缺 Content-Type")
			}
			_, _ = w.Write([]byte(`{"media_id":"mid-42"}`))
		default:
			t.Errorf("意外路径 %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})

	mid, err := cli.AddDraft(context.Background(), []MPArticle{{
		Title: "标题", Content: "<p>内容</p>", Digest: "摘要", Author: "作者",
		ShowCoverPic: 1, ThumbMediaID: "thumb-1",
	}})
	if err != nil {
		t.Fatalf("AddDraft err = %v, want nil", err)
	}
	if mid != "mid-42" {
		t.Fatalf("media_id = %q, want mid-42", mid)
	}
	if !strings.Contains(gotBody, "标题") || !strings.Contains(gotBody, "thumb-1") {
		t.Fatalf("请求体应含 title/thumb_media_id, got %s", gotBody)
	}
}

// TestWeChatMPClient_ErrCode30507 草稿 errcode≠0 → 30507。
func TestWeChatMPClient_ErrCode30507(t *testing.T) {
	cli := newMPForTest(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/cgi-bin/token"):
			_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":7200}`))
		default:
			_, _ = w.Write([]byte(`{"errcode":40001,"errmsg":"invalid credential"}`))
		}
	})

	_, err := cli.AddDraft(context.Background(), []MPArticle{{Title: "t", Content: "c"}})
	if !errs.Is(err, errs.CodeStarfuyuMpUploadFailed) {
		t.Fatalf("errcode≠0 应为 30507, got %v", err)
	}
}

// TestWeChatMPClient_TokenErrCode30507 token 端点 errcode≠0 → 30507。
func TestWeChatMPClient_TokenErrCode30507(t *testing.T) {
	cli := newMPForTest(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"errcode":40125,"errmsg":"invalid appsecret"}`))
	})

	_, err := cli.UploadImageMaterial(context.Background(), []byte{0xFF, 0xD8})
	if !errs.Is(err, errs.CodeStarfuyuMpUploadFailed) {
		t.Fatalf("token errcode≠0 应为 30507, got %v", err)
	}
}

// TestWeChatMPClient_UploadImageMaterial_Success 素材 200 → media_id（收 bytes 非路径）。
func TestWeChatMPClient_UploadImageMaterial_Success(t *testing.T) {
	var hits int32
	cli := newMPForTest(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/cgi-bin/token"):
			atomic.AddInt32(&hits, 1)
			_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":7200}`))
		case strings.Contains(r.URL.Path, "/cgi-bin/material/add_material"):
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Errorf("ParseMultipartForm: %v", err)
			}
			_, _, err := r.FormFile("media")
			if err != nil {
				t.Errorf("缺 media 文件字段: %v", err)
			}
			_, _ = w.Write([]byte(`{"media_id":"thumb-9"}`))
		default:
			t.Errorf("意外路径 %s", r.URL.Path)
		}
	})

	mid, err := cli.UploadImageMaterial(context.Background(), []byte{0x89, 0x50, 0x4E, 0x47})
	if err != nil {
		t.Fatalf("UploadImageMaterial err = %v, want nil", err)
	}
	if mid != "thumb-9" {
		t.Fatalf("media_id = %q, want thumb-9", mid)
	}

	// 第二次调用应命中 token 缓存（提前 5 分钟刷新），token 端点仍只打 1 次。
	if _, err = cli.UploadImageMaterial(context.Background(), []byte{0x89}); err != nil {
		t.Fatalf("二次 Upload err = %v", err)
	}
	if n := atomic.LoadInt32(&hits); n != 1 {
		t.Fatalf("token 端点命中 = %d, want 1（缓存）", n)
	}
}

// TestWeChatMPClient_HTTPFail30507 网络层失败（连接拒绝）→ 30507。
func TestWeChatMPClient_HTTPFail30507(t *testing.T) {
	cli := newMPForTest(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	// 关掉服务器制造连接错误。
	cli.baseURL = "http://127.0.0.1:1" // 保留无效端口

	_, err := cli.AddDraft(context.Background(), []MPArticle{{Title: "t", Content: "c"}})
	if !errs.Is(err, errs.CodeStarfuyuMpUploadFailed) {
		t.Fatalf("HTTP 失败应为 30507, got %v", err)
	}
}

// TestWeChatMPClient_EmptyCredsNoNetwork 空 appid/secret → 30507 且不发请求。
func TestWeChatMPClient_EmptyCredsNoNetwork(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = w.Write([]byte(`{"access_token":"should-not","expires_in":7200}`))
	}))
	defer srv.Close()

	cli := NewWeChatMPClient("", "").(*wechatMPClient)
	cli.baseURL = srv.URL

	_, err := cli.AddDraft(context.Background(), []MPArticle{{Title: "t", Content: "c"}})
	if !errs.Is(err, errs.CodeStarfuyuMpUploadFailed) {
		t.Fatalf("空密钥应为 30507, got %v", err)
	}
	if n := atomic.LoadInt32(&hits); n != 0 {
		t.Fatalf("空密钥不得打网络, hits = %d, want 0", n)
	}
}

// TestWeChatMPClient_UploadContentImage_Success uploadimg 200 → 回微信 CDN URL。
func TestWeChatMPClient_UploadContentImage_Success(t *testing.T) {
	cli := newMPForTest(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/cgi-bin/token"):
			_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":7200}`))
		case strings.Contains(r.URL.Path, "/cgi-bin/media/uploadimg"):
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Errorf("ParseMultipartForm: %v", err)
			}
			if _, _, err := r.FormFile("media"); err != nil {
				t.Errorf("缺 media 文件字段: %v", err)
			}
			_, _ = w.Write([]byte(`{"url":"https://mmbiz.qpic.cn/img1"}`))
		default:
			t.Errorf("意外路径 %s", r.URL.Path)
		}
	})

	u, err := cli.UploadContentImage(context.Background(), []byte{0x89, 0x50})
	if err != nil {
		t.Fatalf("UploadContentImage err = %v, want nil", err)
	}
	if u != "https://mmbiz.qpic.cn/img1" {
		t.Fatalf("url = %q, want https://mmbiz.qpic.cn/img1", u)
	}
}

// TestWeChatMPClient_UploadContentImage_ErrCode30507 errcode≠0 → 30507；空字节 → 30507。
func TestWeChatMPClient_UploadContentImage_ErrCode30507(t *testing.T) {
	cli := newMPForTest(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/cgi-bin/token"):
			_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":7200}`))
		default:
			_, _ = w.Write([]byte(`{"errcode":40004,"errmsg":"invalid media type"}`))
		}
	})
	if _, err := cli.UploadContentImage(context.Background(), []byte{0x89}); !errs.Is(err, errs.CodeStarfuyuMpUploadFailed) {
		t.Fatalf("errcode≠0 应 30507, got %v", err)
	}
	if _, err := cli.UploadContentImage(context.Background(), nil); !errs.Is(err, errs.CodeStarfuyuMpUploadFailed) {
		t.Fatalf("空字节应 30507, got %v", err)
	}
}
