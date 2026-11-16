package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"alexGo-cloud/modules/starfuyu/model"
	"alexGo-cloud/modules/starfuyu/repository"
	"alexGo-cloud/pkg/config"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/storage"
	"alexGo-cloud/pkg/tenant"
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
	// 用户反馈：图片版外层不叠微信自带边距（终审修复波：曾误改到单星座分支）。
	if !strings.Contains(html, "margin: 0 auto; padding: 0") {
		t.Errorf("图片版外层应 padding:0（收窄左右边距）")
	}
	// 用户反馈：微信会给缺省表格画单元格边框——table/td/img 三处必须显式 border:0。
	if !strings.Contains(html, `border="0"`) {
		t.Errorf("table 缺 border=\"0\"（微信会画默认单元格框）")
	}
	if strings.Count(html, "border: 0") < 13 { // td×12 + img×12 合并样式里至少 12 处
		t.Errorf("td/img 显式 border:0 不足 12 处")
	}
	// 预览文件本地打开：无 meta charset 会乱码（实测 Playwright 瞎猜编码）。
	if !strings.Contains(html, `<meta charset="utf-8">`) {
		t.Errorf("HTML 缺 meta charset utf-8")
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

	// 用户反馈 2026-11-16：格上不再画「星座名（日期范围）」小标——海报自带大星座名。
	if strings.Contains(html, "（3月21日-4月19日）") {
		t.Errorf("格上不应再画星座名+日期范围小标")
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
	// spec 非目标：单星座文字版版式不动（含外层 padding 20px——终审修复波回滚误改）。
	if !strings.Contains(html, "padding: 20px") {
		t.Fatalf("单星座文字版外层应保持 padding:20px（非目标不动）")
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

// realisticRows 预览采样专用：12 星座真实感文案（DeepSeek 密钥空、库无数据时的
// 视觉验收入口；仅 TestArticlePreviewForVisual 使用，不参与任何断言）。
func realisticRows(date string) []*model.Fortune {
	consts := ConstellationList
	summaries := [12]string{
		"整体节奏轻快，工作上有贵人点拨，感情宜主动表达",
		"财运稳步上扬，适合处理账务，感情需要耐心经营",
		"思维活跃灵感多，社交场合如鱼得水，忌三心二意",
		"家庭氛围温馨，情绪宜收放有度，健康留意肠胃",
		"气场全开的一天，展现领导力同时别忘了倾听",
		"细节决定成败，适合复盘整理，感情别太较真",
		"人际和谐的一天，合作谈判顺利，注意权衡取舍",
		"洞察力在线，适合深度沟通，防小人远离是非",
		"行动力满满，短途出行有惊喜，理财见好就收",
		"事业稳步推进，加班也别忘吃饭，感情宜主动",
		"创意点子频出，适合头脑风暴，别忽略身边人",
		"直觉敏锐感受细腻，艺术灵感佳，防情绪内耗",
	}
	advices := [6]string{
		"上午适合签约谈判与整理", "下午适合推进搁置的计划", "重要决定放在头脑清醒时",
		"多倾听少表态，三思后行", "适合整理房间断舍离", "主动联系老朋友会带来好运",
	}
	avoids := [6]string{
		"避免冲动消费与投资", "别在背后议论他人", "忌熬夜与暴饮暴食",
		"不要轻易许下承诺", "避免情绪化做决定", "忌与人争一时长短",
	}
	loveTips := [6]string{
		"多与伴侣进行深度交流", "单身者可留意身边人", "制造一点小惊喜加分",
		"给彼此留一点空间", "坦诚说出内心感受", "一起做顿饭增进感情",
	}
	careerTips := [6]string{
		"项目推进顺利，注意细节", "适合汇报展示阶段成果", "同事协作是今日关键",
		"把大任务拆成小步骤", "新技能学习事半功倍", "会议发言提前打腹稿",
	}
	wealthTips := [6]string{
		"偏财小有惊喜，见好就收", "正财稳定，适合做预算", "避免大额非必要支出",
		"留意账单防止漏缴", "投资宜稳不宜急", "理性消费拒绝冲动种草",
	}
	healthTips := [6]string{
		"注意规律作息补充水分", "久坐记得起身拉伸", "留意肩颈放松眼睛",
		"饮食清淡肠胃无负担", "傍晚适合散步解压", "保证七小时睡眠",
	}
	reunionTips := [6]string{
		"旧缘或有回音，宜冷静观察再定", "先把自己的生活理顺再谈缘分", "回忆美好时也别忘现实",
		"复合需双方都愿意改变", "给过去一个温柔的句号", "联系前先想清楚想要什么",
	}
	singleTips := [6]string{
		"桃花在朋友聚会中出现，多留意", "放慢节奏舒服自在最吸引人", "提升自己比急着脱单重要",
		"兴趣社群里有同频的人", "别为了寂寞将就", "展现真实笑容最加分",
	}
	jobTips := [6]string{
		"面试表现稳，薪资可再谈谈看", "突出自身心态放平稳", "更新简历留意新机会",
		"内推比海投更有效", "试用期多问多学", "把项目经历量化成成果",
	}
	slogans := [12]string{
		"贵人点拨节奏轻快，上午签约趁势办",
		"稳步向前好运自来，把计划落到实处",
		"灵感在线表达流畅，重要的话今天说",
		"家庭是你的充电站，早点回家吃饭",
		"气场全开自带光芒，记得照顾他人感受",
		"细节里藏着好运，慢慢来比较快",
		"左右逢源合作顺利，平衡就是赢",
		"洞察先机静水流深，远离是非之地",
		"说走就走好运开路，见好就收是智慧",
		"稳扎稳打步步为营，身体是本钱",
		"脑洞大开打破常规，别忽略身边温柔",
		"直觉是你的指南针，别让情绪跑了偏",
	}
	colors := [6]model.LuckyColor{
		{Chinese: "红色", English: "Red", Hex: "#E74C3C"},
		{Chinese: "蓝色", English: "Blue", Hex: "#3498DB"},
		{Chinese: "金色", English: "Gold", Hex: "#F1C40F"},
		{Chinese: "绿色", English: "Green", Hex: "#2ECC71"},
		{Chinese: "紫色", English: "Purple", Hex: "#9B59B6"},
		{Chinese: "橙色", English: "Orange", Hex: "#E67E22"},
	}
	todosSets := [3]model.DailyTodos{
		{"给久未联系的朋友发条问候消息", "整理桌面和待办清单半小时", "睡前拉伸十分钟放松肩颈"},
		{"记账复盘本周开销", "给绿植浇浇水晒晒太阳", "读十页书再睡"},
		{"把拖着的邮件回掉", "午休散步十五分钟", "对家人说一句感谢"},
	}
	rows := make([]*model.Fortune, 0, 12)
	for i, c := range consts {
		rows = append(rows, &model.Fortune{
			TenantID: 0, Date: date, Constellation: c,
			OverallScore:       68 + (i*5)%25,
			LoveScore:          60 + (i*7)%35,
			CareerScore:        62 + (i*3)%33,
			WealthScore:        58 + (i*11)%37,
			HealthScore:        65 + (i*4)%30,
			LuckyNumber:        i + 1,
			LuckyColor:         colors[i%6],
			Summary:            summaries[i],
			MatchConstellation: consts[(i+1)%12],
			Advice:             advices[i%6],
			Avoid:              avoids[i%6],
			LoveTip:            loveTips[i%6],
			CareerTip:          careerTips[i%6],
			WealthTip:          wealthTips[i%6],
			HealthTip:          healthTips[i%6],
			ReunionTip:         reunionTips[i%6],
			SingleTip:          singleTips[i%6],
			JobTip:             jobTips[i%6],
			ReunionScore:       60 + (i*9)%35,
			SingleScore:        62 + (i*5)%33,
			JobScore:           64 + (i*7)%31,
			DailyTodos:         todosSets[i%3],
			Slogan:             slogans[i],
		})
	}
	return rows
}

// TestArticlePreviewForVisual 图片版软文预览采样：真渲染 12 张 v3 海报到稳定目录，
// BuildHTML 出整版 HTML（img src 换 file:// 本地路径）→ /tmp/starfuyu-article.html，
// 供浏览器人工对照参考图验收。文案用 realisticRows（真实感，非占位）。
// go test -short 跳过（CI 不落盘）；本地 -run 显式跑。
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
	rows := realisticRows(date)
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

	// 2) 整版 HTML：真实感行 + 参考图同款开篇（重点星象/总述），img src 换 file://。
	f := newArticleFixture(t, 0)
	svc := NewArticleService(
		&stubArticleFortunes{rows: rows},
		&stubArticleImages{key: "x.jpg"},
		f.mp, f.drv, f.cfg,
		&stubArticleDaily{row: &model.DailyHoroscope{
			Astro: "月水星，月土六合，木火合",
			Intro: "这天是一个积极热情有能量的日子，办事情能够把握好分寸与人情之间的关系，在金钱和感情方面都有好消息。",
		}},
	)
	html, err := svc.BuildHTML(ctx, date, "")
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

// TestUploadRealDraftManual 真实上传公众号草稿箱：内存库 realisticRows + 真海报渲染 +
// 真微信密钥（alexgo-server/configs/config.yaml，不回显不落断言）→ UploadToMP。
// 手动用例：go test -short 跳过；wechat_mp 密钥未配 → Skip；线上库零写入。
func TestUploadRealDraftManual(t *testing.T) {
	if testing.Short() {
		t.Skip("真实上传：-short 跳过")
	}
	// 双闸：全量跑测试也不误发——只有显式 STARFUYU_REAL_UPLOAD=1 才打真微信。
	if os.Getenv("STARFUYU_REAL_UPLOAD") != "1" {
		t.Skip("真实上传需 STARFUYU_REAL_UPLOAD=1 显式开启")
	}
	// 读仓库根 config（LoadGlobalConfig 固定路径相对 cwd），读完切回 service 包 cwd。
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(filepath.Join(wd, "..", "..", "..")); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadGlobalConfig()
	_ = os.Chdir(wd) // 立刻切回：后续 data/ 相对路径按 service 包 cwd 解析
	if err != nil {
		t.Fatalf("LoadGlobalConfig err = %v", err)
	}
	if cfg.Starfuyu.WechatMp.AppID == "" || cfg.Starfuyu.WechatMp.AppSecret == "" {
		t.Skip("wechat_mp app_id/app_secret 未配置")
	}

	const date = "2026-11-16"
	ctx := context.Background()

	// 1) 内存库 + 你验收过的真实感数据 + 当日星象（线上库零写入）。
	db := newTestDB(t)
	repo := repository.NewFortuneRepository(db)
	dailyRepo := repository.NewDailyRepository(db)
	rows := realisticRows(date)
	if err := repo.UpsertBatch(ctx, rows); err != nil {
		t.Fatalf("seed rows err = %v", err)
	}
	if err := dailyRepo.Upsert(ctx, &model.DailyHoroscope{
		Date:  date,
		Astro: "月水星，月土六合，木火合",
		Intro: "这天是一个积极热情有能量的日子，办事情能够把握好分寸与人情之间的关系，在金钱和感情方面都有好消息。",
	}); err != nil {
		t.Fatalf("seed daily err = %v", err)
	}
	fortuneSvc := NewFortuneService(repo, dailyRepo, &stubFetcher{rows: rows})

	// 2) 真渲染 12 张 v3 海报（临时 storage）。
	imgDrv, err := storage.NewLocalDriver(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	imgCfg := &config.Config{}
	imgCfg.Starfuyu.FontFile = "../../../data/fonts/AlibabaPuHuiTi-3-85-Bold.ttf"
	imgCfg.Starfuyu.BgFile = "../../../data/starfuyu/bg.png"
	imgCfg.Starfuyu.CuteDir = "../../../data/starfuyu/cute/cutout"
	imgSvc := NewImageService(fortuneSvc, imgDrv, imgCfg)

	// 3) 真微信客户端 + 图片版整版上传（草稿箱，不群发）。
	mp := NewWeChatMPClient(cfg.Starfuyu.WechatMp.AppID, cfg.Starfuyu.WechatMp.AppSecret)
	articleSvc := NewArticleService(fortuneSvc, imgSvc, mp, imgDrv, cfg, dailyRepo)

	up, err := articleSvc.UploadToMP(ctx, date)
	if err != nil {
		t.Fatalf("UploadToMP err = %v", err)
	}
	for _, a := range up {
		t.Logf("已入草稿箱: title=%q media_id=%s", a.Title, a.MediaID)
	}
	if len(up) != 1 {
		t.Fatalf("篇数 = %d, want 1", len(up))
	}
}

// TestBuildHTML_DailyReadPinnedTidZero daily 读取必须钉 tid=0（contentCtx）：
// 请求 ctx 带非零租户时仍读到日级星象——否则 gormplugin 按 tid 过滤必 miss，
// 软文静默丢「重点星象/开篇」两块（终审 Important-1）。
func TestBuildHTML_DailyReadPinnedTidZero(t *testing.T) {
	const date = "2026-11-16"
	db := newTestDB(t)
	dailyRepo := repository.NewDailyRepository(db)
	if err := dailyRepo.Upsert(context.Background(), &model.DailyHoroscope{
		Date: date, Astro: "木星入庙顺行", Intro: "总述段落。",
	}); err != nil {
		t.Fatalf("seed daily err = %v", err)
	}
	f := newArticleFixture(t, 0)
	svc := NewArticleService(
		&stubArticleFortunes{rows: twelveRows(date, "租")},
		&stubArticleImages{key: "x.jpg"},
		f.mp, f.drv, f.cfg, dailyRepo)

	ctx := tenant.WithTenantID(context.Background(), 7) // 非零租户请求
	html, err := svc.BuildHTML(ctx, date, "")
	if err != nil {
		t.Fatalf("BuildHTML err = %v, want nil", err)
	}
	if !strings.Contains(html, "重点星象：木星入庙顺行") {
		t.Fatalf("非零租户 ctx 下 daily 应仍可读（tid=0 钉死），HTML 缺重点星象块")
	}
}

// TestUploadToMP_DailyReadPinnedTidZero 同上，钉 UploadToMP 路径的 daily 读取。
func TestUploadToMP_DailyReadPinnedTidZero(t *testing.T) {
	const date = "2026-11-16"
	db := newTestDB(t)
	dailyRepo := repository.NewDailyRepository(db)
	if err := dailyRepo.Upsert(context.Background(), &model.DailyHoroscope{
		Date: date, Astro: "金星合月", Intro: "上传路径总述。",
	}); err != nil {
		t.Fatalf("seed daily err = %v", err)
	}
	f := newArticleFixture(t, 0)
	svc := NewArticleService(
		&stubArticleFortunes{rows: twelveRows(date, "租")},
		&stubArticleImages{key: "fortune/v3/" + date + "/白羊座.jpg"},
		f.mp, f.drv, f.cfg, dailyRepo)

	ctx := tenant.WithTenantID(context.Background(), 7)
	if _, err := svc.UploadToMP(ctx, date); err != nil {
		t.Fatalf("UploadToMP err = %v", err)
	}
	var body struct {
		Articles []struct {
			Content string `json:"content"`
		} `json:"articles"`
	}
	if err := json.Unmarshal(f.st.lastDraftBody, &body); err != nil {
		t.Fatalf("parse draft body: %v", err)
	}
	if !strings.Contains(body.Articles[0].Content, "重点星象：金星合月") {
		t.Fatalf("UploadToMP 路径非零租户下 daily 应仍可读，content 缺重点星象")
	}
}
