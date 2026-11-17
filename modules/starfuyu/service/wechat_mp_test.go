package service

import (
	"alexGo-cloud/pkg/config"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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

// TestWeChatMPClient_ListDrafts batchget 200 → total + 条目（media_id + 标题）。
func TestWeChatMPClient_ListDrafts(t *testing.T) {
	cli := newMPForTest(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/cgi-bin/token"):
			_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":7200}`))
		case strings.Contains(r.URL.Path, "/cgi-bin/draft/batchget"):
			_, _ = w.Write([]byte(`{"total_count":2,"item":[` +
				`{"media_id":"m-1","content":{"news_item":[{"title":"十二星座每日好运｜2026年11月18日"}]}},` +
				`{"media_id":"m-2","content":{"title":"十二星座每日好运｜2026年11月17日"}}]}`))
		default:
			t.Errorf("意外路径 %s", r.URL.Path)
		}
	})

	total, items, err := cli.ListDrafts(context.Background(), 0, 20)
	if err != nil {
		t.Fatalf("ListDrafts err = %v, want nil", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("total=%d items=%d, want 2/2", total, len(items))
	}
	if items[0].MediaID != "m-1" || items[0].Title() != "十二星座每日好运｜2026年11月18日" {
		t.Fatalf("条目解析不符: %+v", items[0])
	}
}

// TestWeChatMPClient_DeleteDraft draft/delete 200 → nil；errcode≠0 → 30507。
func TestWeChatMPClient_DeleteDraft(t *testing.T) {
	var deleted string
	cli := newMPForTest(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/cgi-bin/token"):
			_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":7200}`))
		case strings.Contains(r.URL.Path, "/cgi-bin/draft/delete"):
			var body struct {
				MediaID string `json:"media_id"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			deleted = body.MediaID
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
		default:
			t.Errorf("意外路径 %s", r.URL.Path)
		}
	})

	if err := cli.DeleteDraft(context.Background(), "m-del-1"); err != nil {
		t.Fatalf("DeleteDraft err = %v, want nil", err)
	}
	if deleted != "m-del-1" {
		t.Fatalf("请求体 media_id = %q, want m-del-1", deleted)
	}

	cli2 := newMPForTest(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/cgi-bin/token") {
			_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":7200}`))
			return
		}
		_, _ = w.Write([]byte(`{"errcode":40008,"errmsg":"invalid media_id"}`))
	})
	if err := cli2.DeleteDraft(context.Background(), "nope"); !errs.Is(err, errs.CodeStarfuyuMpUploadFailed) {
		t.Fatalf("errcode≠0 应 30507, got %v", err)
	}
}

// TestDraftManageRealManual 公众号草稿管理工具（真实 API，环境闸）：
//
//	STARFUYU_REAL_UPLOAD=1 + STARFUYU_DRAFT_LIST=1          → 分页列出全部草稿（只读）
//	STARFUYU_REAL_UPLOAD=1 + STARFUYU_DRAFT_DELETE_IDS=a,b   → 少量 id 逐个删除
//	STARFUYU_REAL_UPLOAD=1 + STARFUYU_DRAFT_DELETE_FILE=path → 批量删（每行一个 id；每成功
//	  一条即从文件抹行——失败/中断后重跑同命令即断点续删；遇错即停）。删除不可逆。
//
// 都不设 → Skip。
func TestDraftManageRealManual(t *testing.T) {
	if os.Getenv("STARFUYU_REAL_UPLOAD") != "1" {
		t.Skip("真实 API 需 STARFUYU_REAL_UPLOAD=1")
	}
	listMode := os.Getenv("STARFUYU_DRAFT_LIST") == "1"
	deleteIDs := os.Getenv("STARFUYU_DRAFT_DELETE_IDS")
	deleteFile := os.Getenv("STARFUYU_DRAFT_DELETE_FILE")
	if !listMode && deleteIDs == "" && deleteFile == "" {
		t.Skip("需 STARFUYU_DRAFT_LIST=1 / STARFUYU_DRAFT_DELETE_IDS / STARFUYU_DRAFT_DELETE_FILE")
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(filepath.Join(wd, "..", "..", "..")); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadGlobalConfig()
	_ = os.Chdir(wd)
	if err != nil {
		t.Fatalf("LoadGlobalConfig err = %v", err)
	}
	if cfg.Starfuyu.WechatMp.AppID == "" {
		t.Skip("wechat_mp 未配置")
	}
	mp := NewWeChatMPClient(cfg.Starfuyu.WechatMp.AppID, cfg.Starfuyu.WechatMp.AppSecret)
	ctx := context.Background()

	if listMode {
		offset, total := 0, -1
		idx := 0
		for offset < total || total == -1 {
			got, items, err := mp.ListDrafts(ctx, offset, 20)
			if err != nil {
				t.Fatalf("ListDrafts(offset=%d) err = %v", offset, err)
			}
			total = got
			for _, it := range items {
				idx++
				t.Logf("%2d | %s | %s", idx, it.MediaID, it.Title())
			}
			offset += len(items)
			if len(items) == 0 {
				break
			}
		}
		t.Logf("共 %d 条", idx)
		return
	}

	// 文件模式：逐行删、成功即从文件抹行（断点续删），遇错即停并保留剩余。
	if deleteFile != "" {
		raw, err := os.ReadFile(deleteFile)
		if err != nil {
			t.Fatalf("读删除清单 err = %v", err)
		}
		var ids []string
		for _, l := range strings.Split(string(raw), "\n") {
			if l = strings.TrimSpace(l); l != "" {
				ids = append(ids, l)
			}
		}
		// 单批上限：干净退出防止 go test -timeout 硬杀丢进度（分批循环用）。
		limit := len(ids)
		if v := os.Getenv("STARFUYU_DRAFT_DELETE_LIMIT"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n < limit {
				limit = n
			}
		}
		// 限频间隔（ms）：45009 日配额 + 高频触发都有先例，默认 300ms 保守降频。
		pause := 300 * time.Millisecond
		if v := os.Getenv("STARFUYU_DRAFT_DELETE_PAUSE_MS"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 {
				pause = time.Duration(n) * time.Millisecond
			}
		}
		remaining := ids
		done := 0
		for i := 0; i < limit && len(remaining) > 0; i++ {
			id := remaining[0]
			if err := mp.DeleteDraft(ctx, id); err != nil {
				_ = os.WriteFile(deleteFile, []byte(strings.Join(remaining, "\n")+"\n"), 0o644)
				t.Fatalf("第 %d 条失败，剩余 %d 条保留在 %s: id=%s err=%v",
					done+1, len(remaining), deleteFile, id, err)
			}
			remaining = remaining[1:]
			done++
			if done%25 == 0 {
				_ = os.WriteFile(deleteFile, []byte(strings.Join(remaining, "\n")+"\n"), 0o644)
			}
			if pause > 0 {
				time.Sleep(pause)
			}
		}
		_ = os.WriteFile(deleteFile, []byte(strings.Join(remaining, "\n")+"\n"), 0o644)
		t.Logf("本批删除 %d 条，剩余 %d 条", done, len(remaining))
		return
	}

	for _, id := range strings.Split(deleteIDs, ",") {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if err := mp.DeleteDraft(ctx, id); err != nil {
			t.Fatalf("DeleteDraft(%s) err = %v", id, err)
		}
		t.Logf("已删除: %s", id)
	}
}
