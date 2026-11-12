package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/starfuyu/model"
	"alexGo-cloud/modules/starfuyu/repository"
	"alexGo-cloud/modules/starfuyu/service"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/storage"
)

// ---- 测试替身 ----

// stubFortune service.FortuneService 替身：ListAdmin/GetByDate 可配，记录入参。
type stubFortune struct {
	listRows         []*model.Fortune
	listTotal        int64
	getByDateRows    []*model.Fortune
	gotListDate      string
	gotListConstella string
	gotListPage      int
	gotListSize      int
	gotGetByDateDate string
}

func (s *stubFortune) Fetch(context.Context, string, bool) (bool, error) { return false, nil }

func (s *stubFortune) GetByDate(_ context.Context, date string) ([]*model.Fortune, error) {
	s.gotGetByDateDate = date
	return s.getByDateRows, nil
}

func (s *stubFortune) GetOne(context.Context, string, string) (*model.Fortune, error) {
	return &model.Fortune{}, nil
}

func (s *stubFortune) ListAdmin(_ context.Context, date, constellation string, page, size int) ([]*model.Fortune, int64, error) {
	s.gotListDate, s.gotListConstella = date, constellation
	s.gotListPage, s.gotListSize = page, size
	return s.listRows, s.listTotal, nil
}

// stubTasks service.TaskRunner 替身：记录三个 date-aware 方法入参
// （FetchToday 等 Today* 委托同参，不单独断言）。
type stubTasks struct {
	gotFetchDate  string
	gotFetchForce bool
	gotGenDate    string
	gotExecDate   string
	gotExecForce  bool
}

func (s *stubTasks) FetchToday(ctx context.Context, force bool) error {
	return s.FetchDate(ctx, service.TodaySH(), force)
}

func (s *stubTasks) GenerateImagesToday(ctx context.Context) error {
	return s.GenerateImagesDate(ctx, service.TodaySH())
}

func (s *stubTasks) ExecuteAll(ctx context.Context, force bool) error {
	return s.ExecuteAllDate(ctx, service.TodaySH(), force)
}

func (s *stubTasks) FetchDate(_ context.Context, date string, force bool) error {
	s.gotFetchDate, s.gotFetchForce = date, force
	return nil
}

func (s *stubTasks) GenerateImagesDate(_ context.Context, date string) error {
	s.gotGenDate = date
	return nil
}

func (s *stubTasks) ExecuteAllDate(_ context.Context, date string, force bool) error {
	s.gotExecDate, s.gotExecForce = date, force
	return nil
}

// stubArticle service.ArticleService 替身。
type stubArticle struct {
	html          string
	gotUploadDate string
}

func (s *stubArticle) BuildHTML(context.Context, string, string) (string, error) {
	return s.html, nil
}

func (s *stubArticle) UploadToMP(_ context.Context, date string) ([]service.UploadedArticle, error) {
	s.gotUploadDate = date
	return []service.UploadedArticle{{Title: "十二星座每日好运", MediaID: "media-1"}}, nil
}

// stubImage service.ImageService 替身（ArticleService 构造依赖；空密钥短路路径不触达）。
type stubImage struct{}

func (stubImage) Ensure(context.Context, string, string) (string, error) { return "", nil }
func (stubImage) GenerateAll(context.Context, string) (int, error)       { return 0, nil }

// stubLimits repository.RequestLimitRepository 替身（/limits 聚合读）。
type stubLimits struct {
	rows    []repository.TodayLimitRow
	gotDate string
}

func (s *stubLimits) CheckAndIncrement(context.Context, uint64, string, string) (int, error) {
	return 1, nil
}

func (s *stubLimits) AdminTodaySummary(_ context.Context, date string) ([]repository.TodayLimitRow, error) {
	s.gotDate = date
	return s.rows, nil
}

// ---- 路由与请求辅助 ----

// newRouter 起 admin 八路由（与 module.go RegisterRoutes admin 段同形状）。
func newRouter(t *testing.T, ctrl *AdminController) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/admin/starfuyu")
	g.GET("/fortunes", ctrl.ListFortunes)
	g.GET("/fortunes/:date", ctrl.FortuneByDate)
	g.GET("/fortunes/:date/article", ctrl.FortuneArticle)
	g.POST("/fortunes/:date/upload-mp", ctrl.UploadMP)
	g.POST("/tasks/fetch-fortune", ctrl.TaskFetchFortune)
	g.POST("/tasks/generate-images", ctrl.TaskGenerateImages)
	g.POST("/tasks/execute-all", ctrl.TaskExecuteAll)
	g.GET("/limits", ctrl.Limits)
	return r
}

// envelope 统一信封断言的最小形状 {code, msg, data}。
type envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// doJSON 发请求并解析信封；body 非空时带 Content-Type: application/json。
func doJSON(t *testing.T, r *gin.Engine, method, path, body string) (*httptest.ResponseRecorder, envelope) {
	t.Helper()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var env envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("信封解析失败: %v body=%s", err, w.Body.String())
	}
	return w, env
}

// wantCode 断言 HTTP status + 信封 code。
func wantCode(t *testing.T, w *httptest.ResponseRecorder, env envelope, httpStatus, code int) {
	t.Helper()
	if w.Code != httpStatus {
		t.Fatalf("http=%d want %d body=%s", w.Code, httpStatus, w.Body.String())
	}
	if env.Code != code {
		t.Fatalf("code=%d want %d body=%s", env.Code, code, w.Body.String())
	}
}

// ---- 用例 ----

// TestListFortunes_PageEnvelope brief Step 1：resp.Page 形状 {list,total,page,size}，
// query 筛选（date/constellation/page/size）透传 ListAdmin。
func TestListFortunes_PageEnvelope(t *testing.T) {
	fortune := &stubFortune{
		listRows:  []*model.Fortune{{Date: "2026-11-12", Constellation: "白羊座"}},
		listTotal: 42,
	}
	r := newRouter(t, NewAdminController(fortune, &stubTasks{}, &stubArticle{}, &stubLimits{}))

	w, env := doJSON(t, r, http.MethodGet,
		"/admin/starfuyu/fortunes?page=2&size=5&date=2026-11-12&constellation=白羊座", "")
	wantCode(t, w, env, http.StatusOK, 0)

	var data struct {
		List  []any `json:"list"`
		Total int64 `json:"total"`
		Page  int   `json:"page"`
		Size  int   `json:"size"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("data 解析失败: %v data=%s", err, env.Data)
	}
	if data.Page != 2 || data.Size != 5 || data.Total != 42 || len(data.List) != 1 {
		t.Fatalf("page 形状不对: %+v", data)
	}
	if fortune.gotListDate != "2026-11-12" || fortune.gotListConstella != "白羊座" ||
		fortune.gotListPage != 2 || fortune.gotListSize != 5 {
		t.Fatalf("ListAdmin 入参未透传: date=%q constellation=%q page=%d size=%d",
			fortune.gotListDate, fortune.gotListConstella, fortune.gotListPage, fortune.gotListSize)
	}
}

// TestListFortunes_BadDate10001 query date 非法（非 YYYY-MM-DD）→ 400 + 10001。
func TestListFortunes_BadDate10001(t *testing.T) {
	r := newRouter(t, NewAdminController(&stubFortune{}, &stubTasks{}, &stubArticle{}, &stubLimits{}))
	w, env := doJSON(t, r, http.MethodGet, "/admin/starfuyu/fortunes?date=2026-13-01", "")
	wantCode(t, w, env, http.StatusBadRequest, 10001)
}

// TestFortuneByDate_Empty30501 指定日期无运势行 → 400 + 30501（spec §7 admin 详情）。
func TestFortuneByDate_Empty30501(t *testing.T) {
	fortune := &stubFortune{getByDateRows: []*model.Fortune{}}
	r := newRouter(t, NewAdminController(fortune, &stubTasks{}, &stubArticle{}, &stubLimits{}))
	w, env := doJSON(t, r, http.MethodGet, "/admin/starfuyu/fortunes/2026-11-12", "")
	wantCode(t, w, env, http.StatusBadRequest, 30501)
	if fortune.gotGetByDateDate != "2026-11-12" {
		t.Fatalf("GetByDate date=%q want 2026-11-12", fortune.gotGetByDateDate)
	}
}

// TestFortuneByDate_Ok200 有行 → 200 data 为数组。
func TestFortuneByDate_Ok200(t *testing.T) {
	fortune := &stubFortune{getByDateRows: []*model.Fortune{{Date: "2026-11-12", Constellation: "白羊座"}}}
	r := newRouter(t, NewAdminController(fortune, &stubTasks{}, &stubArticle{}, &stubLimits{}))
	w, env := doJSON(t, r, http.MethodGet, "/admin/starfuyu/fortunes/2026-11-12", "")
	wantCode(t, w, env, http.StatusOK, 0)
	var list []any
	if err := json.Unmarshal(env.Data, &list); err != nil || len(list) != 1 {
		t.Fatalf("data 应为 1 元素数组: %v data=%s", err, env.Data)
	}
}

// TestFortuneByDate_BadDate10001 :date 非法 → 400 + 10001。
func TestFortuneByDate_BadDate10001(t *testing.T) {
	r := newRouter(t, NewAdminController(&stubFortune{}, &stubTasks{}, &stubArticle{}, &stubLimits{}))
	w, env := doJSON(t, r, http.MethodGet, "/admin/starfuyu/fortunes/2026-13-01", "")
	wantCode(t, w, env, http.StatusBadRequest, 10001)
}

// TestFortuneArticle_ReturnsHTML 文章预览 → 200 data.html 为生成的 HTML。
func TestFortuneArticle_ReturnsHTML(t *testing.T) {
	articles := &stubArticle{html: "<div>十二星座运势</div>"}
	r := newRouter(t, NewAdminController(&stubFortune{}, &stubTasks{}, articles, &stubLimits{}))
	w, env := doJSON(t, r, http.MethodGet, "/admin/starfuyu/fortunes/2026-11-12/article", "")
	wantCode(t, w, env, http.StatusOK, 0)
	var data struct {
		HTML string `json:"html"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("data 解析失败: %v data=%s", err, env.Data)
	}
	if data.HTML != "<div>十二星座运势</div>" {
		t.Fatalf("html=%q", data.HTML)
	}
}

// TestUploadMP_EmptyCreds30507 brief Step 1：真 ArticleService 空密钥 → 立即
// 30507 不打网络（controller 透传信封，HTTP 400）。
func TestUploadMP_EmptyCreds30507(t *testing.T) {
	driver, err := storage.NewLocalDriver(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	realArticle := service.NewArticleService(
		&stubFortune{}, stubImage{}, service.NewWeChatMPClient("", ""), driver, &config.Config{})
	r := newRouter(t, NewAdminController(&stubFortune{}, &stubTasks{}, realArticle, &stubLimits{}))

	w, env := doJSON(t, r, http.MethodPost, "/admin/starfuyu/fortunes/2026-11-12/upload-mp", "")
	wantCode(t, w, env, http.StatusBadRequest, 30507)
}

// TestFetchFortune_ForcePassthrough brief Step 1：force/date 语义透传 TaskRunner。
func TestFetchFortune_ForcePassthrough(t *testing.T) {
	tasks := &stubTasks{}
	r := newRouter(t, NewAdminController(&stubFortune{}, tasks, &stubArticle{}, &stubLimits{}))

	w, env := doJSON(t, r, http.MethodPost, "/admin/starfuyu/tasks/fetch-fortune",
		`{"date":"2026-11-01","force":true}`)
	wantCode(t, w, env, http.StatusOK, 0)
	if tasks.gotFetchDate != "2026-11-01" || !tasks.gotFetchForce {
		t.Fatalf("FetchDate 入参: date=%q force=%v want 2026-11-01/true",
			tasks.gotFetchDate, tasks.gotFetchForce)
	}
}

// TestFetchFortune_DefaultDate 空 body / {} → date 缺省 TodaySH、force=false。
func TestFetchFortune_DefaultDate(t *testing.T) {
	for _, body := range []string{"", "{}"} {
		tasks := &stubTasks{}
		r := newRouter(t, NewAdminController(&stubFortune{}, tasks, &stubArticle{}, &stubLimits{}))
		w, env := doJSON(t, r, http.MethodPost, "/admin/starfuyu/tasks/fetch-fortune", body)
		wantCode(t, w, env, http.StatusOK, 0)
		if tasks.gotFetchDate != service.TodaySH() || tasks.gotFetchForce {
			t.Fatalf("body=%q 默认入参: date=%q force=%v want %q/false",
				body, tasks.gotFetchDate, tasks.gotFetchForce, service.TodaySH())
		}
	}
}

// TestFetchFortune_BadDate10001 body date 非法 → 400 + 10001。
func TestFetchFortune_BadDate10001(t *testing.T) {
	r := newRouter(t, NewAdminController(&stubFortune{}, &stubTasks{}, &stubArticle{}, &stubLimits{}))
	w, env := doJSON(t, r, http.MethodPost, "/admin/starfuyu/tasks/fetch-fortune", `{"date":"yesterday"}`)
	wantCode(t, w, env, http.StatusBadRequest, 10001)
}

// TestGenerateImages_DatePassthrough generate-images 透传 date。
func TestGenerateImages_DatePassthrough(t *testing.T) {
	tasks := &stubTasks{}
	r := newRouter(t, NewAdminController(&stubFortune{}, tasks, &stubArticle{}, &stubLimits{}))
	w, env := doJSON(t, r, http.MethodPost, "/admin/starfuyu/tasks/generate-images", `{"date":"2026-11-01"}`)
	wantCode(t, w, env, http.StatusOK, 0)
	if tasks.gotGenDate != "2026-11-01" {
		t.Fatalf("GenerateImagesDate date=%q want 2026-11-01", tasks.gotGenDate)
	}
}

// TestExecuteAll_ForceDatePassthrough execute-all 透传 date+force。
func TestExecuteAll_ForceDatePassthrough(t *testing.T) {
	tasks := &stubTasks{}
	r := newRouter(t, NewAdminController(&stubFortune{}, tasks, &stubArticle{}, &stubLimits{}))
	w, env := doJSON(t, r, http.MethodPost, "/admin/starfuyu/tasks/execute-all",
		`{"date":"2026-11-01","force":true}`)
	wantCode(t, w, env, http.StatusOK, 0)
	if tasks.gotExecDate != "2026-11-01" || !tasks.gotExecForce {
		t.Fatalf("ExecuteAllDate 入参: date=%q force=%v want 2026-11-01/true",
			tasks.gotExecDate, tasks.gotExecForce)
	}
}

// TestLimits_ReturnsRows /limits → 200 聚合行数组，date 取 TodaySH。
func TestLimits_ReturnsRows(t *testing.T) {
	limits := &stubLimits{rows: []repository.TodayLimitRow{
		{Type: "bazi_calculate", Count: 3},
		{Type: "divination_draw", Count: 7},
	}}
	r := newRouter(t, NewAdminController(&stubFortune{}, &stubTasks{}, &stubArticle{}, limits))
	w, env := doJSON(t, r, http.MethodGet, "/admin/starfuyu/limits", "")
	wantCode(t, w, env, http.StatusOK, 0)
	if limits.gotDate != service.TodaySH() {
		t.Fatalf("AdminTodaySummary date=%q want %q", limits.gotDate, service.TodaySH())
	}
	var rows []repository.TodayLimitRow
	if err := json.Unmarshal(env.Data, &rows); err != nil || len(rows) != 2 {
		t.Fatalf("data 应为 2 行聚合: %v data=%s", err, env.Data)
	}
}

// TestLimits_NilRowsEmptyArray 无消耗 → data 是 [] 不是 null。
func TestLimits_NilRowsEmptyArray(t *testing.T) {
	r := newRouter(t, NewAdminController(&stubFortune{}, &stubTasks{}, &stubArticle{}, &stubLimits{}))
	w, env := doJSON(t, r, http.MethodGet, "/admin/starfuyu/limits", "")
	wantCode(t, w, env, http.StatusOK, 0)
	if string(env.Data) != "[]" {
		t.Fatalf("空聚合 data=%s want []", env.Data)
	}
}
