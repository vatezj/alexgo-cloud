package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"alexGo-cloud/modules/starfuyu/model"
	"alexGo-cloud/pkg/config"
	errs "alexGo-cloud/pkg/errors"
)

// stubArticleFortunes ArticleService 测试替身：预置行直出。
type stubArticleFortunes struct {
	rows []*model.Fortune
}

func (s *stubArticleFortunes) Fetch(context.Context, string, bool) (bool, error) {
	return false, nil
}

func (s *stubArticleFortunes) GetByDate(context.Context, string) ([]*model.Fortune, error) {
	return s.rows, nil
}

func (s *stubArticleFortunes) GetOne(_ context.Context, _, constellation string) (*model.Fortune, error) {
	for _, r := range s.rows {
		if r.Constellation == constellation {
			return r, nil
		}
	}
	return nil, errs.New(errs.CodeStarfuyuFortuneNotFound, nil)
}

func (s *stubArticleFortunes) ListAdmin(context.Context, string, string, int, int) ([]*model.Fortune, int64, error) {
	return nil, 0, nil
}

// stubArticleImages ImageService 替身：Ensure 固定返回 key。
type stubArticleImages struct {
	key string
	err error
}

func (s *stubArticleImages) Ensure(context.Context, string, string) (string, error) {
	return s.key, s.err
}

func (s *stubArticleImages) GenerateAll(context.Context, string) (int, error) {
	return 12, nil
}

// stubMPDriver storage.Driver 替身：Open 固定返回预置字节。
type stubMPDriver struct {
	data  []byte
	opens int
}

func (d *stubMPDriver) Save(context.Context, string, io.Reader) error { return nil }

func (d *stubMPDriver) Open(context.Context, string) (io.ReadCloser, error) {
	d.opens++
	return io.NopCloser(bytes.NewReader(d.data)), nil
}

func (d *stubMPDriver) Delete(context.Context, string) error { return nil }

// fakeWeChat 假微信：token / 素材 / 草稿三端点；draftErrCode≠0 时草稿报错。
func fakeWeChat(t *testing.T, draftErrCode int) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/cgi-bin/token", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"tok-test","expires_in":7200}`))
	})
	mux.HandleFunc("/cgi-bin/material/add_material", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"media_id":"thumb-1","url":"https://mmbiz.qpic.cn/x.jpg"}`))
	})
	mux.HandleFunc("/cgi-bin/draft/add", func(w http.ResponseWriter, _ *http.Request) {
		if draftErrCode != 0 {
			_, _ = w.Write([]byte(`{"errcode":` + itoa(draftErrCode) + `,"errmsg":"biz err"}`))
			return
		}
		_, _ = w.Write([]byte(`{"media_id":"draft-1"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// articleFixture 组装 ArticleService：stub 运势/图片 + 真 WeChatMPClient（打假微信）。
type articleFixture struct {
	svc ArticleService
	mp  *wechatMPClient
	drv *stubMPDriver
	cfg *config.Config
	srv *httptest.Server
}

func newArticleFixture(t *testing.T, draftErrCode int) *articleFixture {
	t.Helper()
	srv := fakeWeChat(t, draftErrCode)
	cfg := &config.Config{}
	cfg.Starfuyu.WechatMp.AppID = "wx-test-appid"
	cfg.Starfuyu.WechatMp.AppSecret = "wx-test-secret"
	mp := NewWeChatMPClient("wx-test-appid", "wx-test-secret").(*wechatMPClient)
	mp.baseURL = srv.URL
	drv := &stubMPDriver{data: []byte{0xFF, 0xD8, 0xFF, 0xD9}}
	svc := NewArticleService(
		&stubArticleFortunes{rows: twelveRows("2026-11-12", "文")},
		&stubArticleImages{key: "fortune/2026-11-12/白羊座.jpg"},
		mp,
		drv,
		cfg,
	)
	return &articleFixture{svc: svc, mp: mp, drv: drv, cfg: cfg, srv: srv}
}

// TestBuildHTML_ContainsAllConstellations stub 12 条 → 整版 HTML 含 12 星座名与分数 stars 字样。
func TestBuildHTML_ContainsAllConstellations(t *testing.T) {
	const date = "2026-11-12"
	f := newArticleFixture(t, 0)

	html, err := f.svc.BuildHTML(context.Background(), date, "")
	if err != nil {
		t.Fatalf("BuildHTML err = %v, want nil", err)
	}
	if len(html) == 0 {
		t.Fatalf("BuildHTML 返回空串")
	}
	for _, c := range ConstellationList {
		if !strings.Contains(html, c) {
			t.Errorf("整版 HTML 缺星座名 %q", c)
		}
	}
	// 分数 stars 字样：goLucky starsToString 渲染为「N分」。
	if !strings.Contains(html, "分") {
		t.Errorf("整版 HTML 缺分数 stars 字样（N分）")
	}
	if !strings.Contains(html, "整体运势") {
		t.Errorf("整版 HTML 缺整体运势段")
	}
}

// TestBuildHTML_SingleConstellation 非空 constellation → 只含该星座。
func TestBuildHTML_SingleConstellation(t *testing.T) {
	f := newArticleFixture(t, 0)

	html, err := f.svc.BuildHTML(context.Background(), "2026-11-12", "金牛座")
	if err != nil {
		t.Fatalf("BuildHTML single err = %v, want nil", err)
	}
	if !strings.Contains(html, "金牛座") {
		t.Fatalf("单星座 HTML 缺目标星座名")
	}
	if strings.Contains(html, "白羊座") {
		t.Fatalf("单星座 HTML 不得含其他星座名")
	}
}

// TestUploadToMP_FailureMaps30507 假微信草稿接口 errcode≠0 → 30507。
func TestUploadToMP_FailureMaps30507(t *testing.T) {
	f := newArticleFixture(t, 40007)

	got, err := f.svc.UploadToMP(context.Background(), "2026-11-12")
	if !errs.Is(err, errs.CodeStarfuyuMpUploadFailed) {
		t.Fatalf("草稿 errcode≠0 应为 30507, got %v", err)
	}
	if got != nil {
		t.Fatalf("失败应返回 nil 列表, got %+v", got)
	}
}

// TestUploadToMP_SuccessReturnsMediaIDs 假接口全 200 → 默认整版 1 篇（controller 裁定）。
func TestUploadToMP_SuccessReturnsMediaIDs(t *testing.T) {
	f := newArticleFixture(t, 0)

	got, err := f.svc.UploadToMP(context.Background(), "2026-11-12")
	if err != nil {
		t.Fatalf("UploadToMP err = %v, want nil", err)
	}
	if len(got) != 1 {
		t.Fatalf("默认整版篇数 = %d, want 1", len(got))
	}
	if got[0].MediaID != "draft-1" {
		t.Fatalf("MediaID = %q, want draft-1", got[0].MediaID)
	}
	if got[0].Title == "" {
		t.Fatalf("Title 不得为空")
	}
	if f.drv.opens == 0 {
		t.Fatalf("封面素材应经 storage.Driver.Open 读字节")
	}
}

// TestUploadToMP_EmptyCreds30507_NoNetwork 密钥空配 → 立即 30507，不打网络（验收 §11.4）。
func TestUploadToMP_EmptyCreds30507_NoNetwork(t *testing.T) {
	f := newArticleFixture(t, 0)
	f.cfg.Starfuyu.WechatMp.AppID = ""
	f.cfg.Starfuyu.WechatMp.AppSecret = ""
	// 重建 svc（cfg 是构造时拷入的字段引用，直接改 struct 内 cfg 即可——
	// NewArticleService 存的是同一指针）。
	svc := NewArticleService(
		&stubArticleFortunes{rows: twelveRows("2026-11-12", "空配")},
		&stubArticleImages{key: "fortune/2026-11-12/白羊座.jpg"},
		f.mp,
		f.drv,
		f.cfg,
	)

	got, err := svc.UploadToMP(context.Background(), "2026-11-12")
	if !errs.Is(err, errs.CodeStarfuyuMpUploadFailed) {
		t.Fatalf("空密钥应为 30507, got %v", err)
	}
	if got != nil {
		t.Fatalf("空密钥失败应 nil, got %+v", got)
	}
	// 未配置密钥不得触发任何微信调用（opens 为 0 表示连封面都没读——
	// 短路发生在网络与素材流水线之前）。
	if f.drv.opens != 0 {
		t.Fatalf("空密钥不得读素材, opens = %d, want 0", f.drv.opens)
	}
}
