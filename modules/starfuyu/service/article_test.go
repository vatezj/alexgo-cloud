package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"alexGo-cloud/modules/starfuyu/model"
	"alexGo-cloud/modules/starfuyu/repository"
	"alexGo-cloud/pkg/config"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/storage"
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

// stubArticleDaily DailyRepository 替身：row==nil 模拟 miss（降级渲染）。
type stubArticleDaily struct {
	row *model.DailyHoroscope
}

func (s *stubArticleDaily) GetByDate(context.Context, string) (*model.DailyHoroscope, bool, error) {
	if s.row == nil {
		return nil, false, nil
	}
	return s.row, true, nil
}

func (s *stubArticleDaily) Upsert(context.Context, *model.DailyHoroscope) error { return nil }

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

// fakeWeChatStats 假微信调用统计：端点命中次数、draft 请求体、uploadimg 第 N 次失败注入。
type fakeWeChatStats struct {
	uploadimg      int32
	material       int32
	draft          int32
	lastDraftBody  []byte
	uploadimgFailN int32 // 0=不失败；N=第 N 次 uploadimg 返回 errcode≠0
}

// fakeWeChat 假微信：token / 素材 / 正文图 / 草稿四端点；draftErrCode≠0 时草稿报错。
func fakeWeChat(t *testing.T, draftErrCode int) (*httptest.Server, *fakeWeChatStats) {
	t.Helper()
	st := &fakeWeChatStats{}
	mux := http.NewServeMux()
	mux.HandleFunc("/cgi-bin/token", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"tok-test","expires_in":7200}`))
	})
	mux.HandleFunc("/cgi-bin/material/add_material", func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&st.material, 1)
		_, _ = w.Write([]byte(`{"media_id":"thumb-1","url":"https://mmbiz.qpic.cn/x.jpg"}`))
	})
	mux.HandleFunc("/cgi-bin/media/uploadimg", func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&st.uploadimg, 1)
		if st.uploadimgFailN != 0 && n == st.uploadimgFailN {
			_, _ = w.Write([]byte(`{"errcode":40004,"errmsg":"invalid media"}`))
			return
		}
		_, _ = w.Write([]byte(`{"url":"https://mmbiz.qpic.cn/img` + itoa(int(n)) + `"}`))
	})
	mux.HandleFunc("/cgi-bin/draft/add", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&st.draft, 1)
		body, _ := io.ReadAll(r.Body)
		st.lastDraftBody = body
		if draftErrCode != 0 {
			_, _ = w.Write([]byte(`{"errcode":` + itoa(draftErrCode) + `,"errmsg":"biz err"}`))
			return
		}
		_, _ = w.Write([]byte(`{"media_id":"draft-1"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, st
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
	st  *fakeWeChatStats
}

func newArticleFixture(t *testing.T, draftErrCode int) *articleFixture {
	t.Helper()
	srv, st := fakeWeChat(t, draftErrCode)
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
		&stubArticleDaily{row: &model.DailyHoroscope{
			Astro: "月水星，月土六合，木火合",
			Intro: "这天是一个积极热情有能量的日子，办事情能够把握好分寸。",
		}},
	)
	return &articleFixture{svc: svc, mp: mp, drv: drv, cfg: cfg, srv: srv, st: st}
}

// TestBuildHTML_ContainsAllConstellations 图片版整版：12 星座名 + 12 张 v3 海报
// 相对公开读路径 + 日期徽章 + 底部星福柚（旧文字版断言随版式替换）。
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
		want := `/api/public/starfuyu/image/fortune/v3/` + date + `/` + c + `.jpg`
		if !strings.Contains(html, want) {
			t.Errorf("整版 HTML 缺海报 img src %q", want)
		}
	}
	if !strings.Contains(html, "2026/11/12") {
		t.Errorf("整版 HTML 缺日期徽章 2026/11/12")
	}
	if !strings.Contains(html, "星福柚") {
		t.Errorf("整版 HTML 缺底部关注区「星福柚」")
	}
}

// TestBuildHTML_ElementGroups 四胶囊顺序火土风水；每组成员顺序钉死；
// 星座名旁带 constellationMonth 日期范围。
func TestBuildHTML_ElementGroups(t *testing.T) {
	const date = "2026-11-12"
	f := newArticleFixture(t, 0)
	html, err := f.svc.BuildHTML(context.Background(), date, "")
	if err != nil {
		t.Fatalf("BuildHTML err = %v, want nil", err)
	}

	// 四象胶囊顺序。
	last := -1
	for _, pill := range []string{"火象星座", "土象星座", "风象星座", "水象星座"} {
		idx := strings.Index(html, pill)
		if idx < 0 {
			t.Fatalf("缺四象胶囊 %q", pill)
		}
		if idx <= last {
			t.Fatalf("四象胶囊顺序错：%q 出现在前一个之前", pill)
		}
		last = idx
	}

	// 每组成员 img 顺序（火：白羊→狮子→射手；其余同理）。
	groups := [][]string{
		{"白羊座", "狮子座", "射手座"},
		{"金牛座", "处女座", "摩羯座"},
		{"双子座", "天秤座", "水瓶座"},
		{"巨蟹座", "天蝎座", "双鱼座"},
	}
	for _, g := range groups {
		prev := -1
		for _, c := range g {
			idx := strings.Index(html, `/fortune/v3/`+date+`/`+c+`.jpg`)
			if idx < 0 {
				t.Fatalf("缺 %s 海报 img", c)
			}
			if idx <= prev {
				t.Fatalf("组内顺序错：%s 应在前一成员之后", c)
			}
			prev = idx
		}
	}

	// 每格名字带日期范围（示例：白羊座）。
	if !strings.Contains(html, "白羊座（3月21日-4月19日）") {
		t.Errorf("缺星座名+日期范围小标（白羊座）")
	}
}

// TestBuildHTML_MissingDailyDegrades daily miss → 无「重点星象」块，
// 但日期徽章、四胶囊、12 img 齐（降级不失败）。
func TestBuildHTML_MissingDailyDegrades(t *testing.T) {
	const date = "2026-11-12"
	f := newArticleFixture(t, 0)
	f.svc = NewArticleService(
		&stubArticleFortunes{rows: twelveRows(date, "文")},
		&stubArticleImages{key: "x.jpg"},
		f.mp, f.drv, f.cfg,
		&stubArticleDaily{row: nil},
	)

	html, err := f.svc.BuildHTML(context.Background(), date, "")
	if err != nil {
		t.Fatalf("BuildHTML err = %v, want nil", err)
	}
	if strings.Contains(html, "重点星象") {
		t.Errorf("daily miss 不应画重点星象块")
	}
	if !strings.Contains(html, date[:4]+"/"+date[5:7]+"/"+date[8:]) {
		t.Errorf("缺日期徽章")
	}
	for _, pill := range []string{"火象星座", "土象星座", "风象星座", "水象星座"} {
		if !strings.Contains(html, pill) {
			t.Errorf("缺胶囊 %q", pill)
		}
	}
	n := strings.Count(html, `/fortune/v3/`+date+`/`)
	if n != 12 {
		t.Errorf("img 数 = %d, want 12", n)
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
	// 图片版流程：正文图 uploadimg 12 次、封面永久素材 1 次。
	if n := atomic.LoadInt32(&f.st.uploadimg); n != 12 {
		t.Fatalf("uploadimg 次数 = %d, want 12", n)
	}
	if n := atomic.LoadInt32(&f.st.material); n != 1 {
		t.Fatalf("add_material 次数 = %d, want 1", n)
	}
}

// TestUploadToMP_ContentUsesCDNUrls 草稿 content 必须是微信 CDN URL：
// 12 个 mmbiz.qpic.cn、不含预览用 /api/public/ 相对路径、含四象胶囊文案。
func TestUploadToMP_ContentUsesCDNUrls(t *testing.T) {
	f := newArticleFixture(t, 0)
	if _, err := f.svc.UploadToMP(context.Background(), "2026-11-12"); err != nil {
		t.Fatalf("UploadToMP err = %v, want nil", err)
	}
	var body struct {
		Articles []struct {
			Content string `json:"content"`
		} `json:"articles"`
	}
	if err := json.Unmarshal(f.st.lastDraftBody, &body); err != nil {
		t.Fatalf("解析 draft body 失败: %v", err)
	}
	if len(body.Articles) != 1 {
		t.Fatalf("draft articles = %d, want 1", len(body.Articles))
	}
	content := body.Articles[0].Content
	if n := strings.Count(content, "https://mmbiz.qpic.cn/img"); n != 12 {
		t.Errorf("content CDN 图数 = %d, want 12", n)
	}
	if strings.Contains(content, "/api/public/") {
		t.Errorf("content 混入预览相对路径 /api/public/")
	}
	for _, pill := range []string{"火象星座", "土象星座", "风象星座", "水象星座"} {
		if !strings.Contains(content, pill) {
			t.Errorf("content 缺四象胶囊 %q", pill)
		}
	}
}

// TestUploadToMP_ContentUploadFailNoDraft 第 3 张正文图上传失败 → 30507 且草稿零调用（不发半成品）。
func TestUploadToMP_ContentUploadFailNoDraft(t *testing.T) {
	f := newArticleFixture(t, 0)
	f.st.uploadimgFailN = 3
	got, err := f.svc.UploadToMP(context.Background(), "2026-11-12")
	if !errs.Is(err, errs.CodeStarfuyuMpUploadFailed) {
		t.Fatalf("正文图失败应 30507, got %v", err)
	}
	if got != nil {
		t.Fatalf("失败不得返回篇目: %+v", got)
	}
	if n := atomic.LoadInt32(&f.st.draft); n != 0 {
		t.Fatalf("draft 调用 = %d, want 0（不发半成品）", n)
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
		&stubArticleDaily{},
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

// TestArticlePreviewForVisual 图片版软文预览采样：真渲染 12 张 v3 海报到稳定目录，
// BuildHTML 出整版 HTML（img src 换 file:// 本地路径）→ /tmp/starfuyu-article.html，
// 供浏览器人工对照参考图验收。go test -short 跳过（CI 不落盘）；本地 -run 显式跑。
func TestArticlePreviewForVisual(t *testing.T) {
	if testing.Short() {
		t.Skip("视觉采样：-short 跳过")
	}
	const date = "2026-11-16"
	ctx := context.Background()

	// 1) 真渲染 12 张海报到稳定目录（t.TempDir 测试结束即删，file:// 会失效）。
	root := "/tmp/starfuyu-article-preview"
	if err := os.RemoveAll(root); err != nil {
		t.Fatalf("清理预览目录 err = %v", err)
	}
	imgDrv, err := storage.NewLocalDriver(root)
	if err != nil {
		t.Fatalf("NewLocalDriver err = %v", err)
	}
	db := newTestDB(t)
	repo := repository.NewFortuneRepository(db)
	rows := twelveRows(date, "预")
	if err := repo.UpsertBatch(ctx, rows); err != nil {
		t.Fatalf("seed rows err = %v", err)
	}
	imgCfg := &config.Config{}
	imgCfg.Starfuyu.FontFile = "../../../data/fonts/AlibabaPuHuiTi-3-85-Bold.ttf"
	imgCfg.Starfuyu.BgFile = "../../../data/starfuyu/bg.png"
	imgCfg.Starfuyu.CuteDir = "../../../data/starfuyu/cute/cutout"
	imgSvc := NewImageService(
		NewFortuneService(repo, repository.NewDailyRepository(db), &stubFetcher{rows: rows}),
		imgDrv, imgCfg)
	if _, err := imgSvc.GenerateAll(ctx, date); err != nil {
		t.Fatalf("GenerateAll err = %v", err)
	}

	// 2) 整版 HTML（stub 文章数据）+ img src 换本地 file:// 路径。
	f := newArticleFixture(t, 0)
	html, err := f.svc.BuildHTML(ctx, date, "")
	if err != nil {
		t.Fatalf("BuildHTML err = %v", err)
	}
	html = strings.ReplaceAll(html,
		"/api/public/starfuyu/image/fortune/v3/",
		"file://"+root+"/fortune/v3/")
	const out = "/tmp/starfuyu-article.html"
	if err := os.WriteFile(out, []byte(html), 0o644); err != nil {
		t.Fatalf("写预览 HTML err = %v", err)
	}
	t.Logf("软文预览已落盘: %s（海报目录 %s）", out, root)
}
