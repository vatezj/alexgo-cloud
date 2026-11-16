package admin

import (
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/starfuyu/repository"
	"alexGo-cloud/modules/starfuyu/service"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/resp"
	"alexGo-cloud/pkg/storage"
)

// AdminController admin 端 starfuyu 接口（spec §4.2，8 端点）：casbin 已按
// permissionRoutes 前缀鉴权，handler 不取 claims（scorecard admin 同构）。
type AdminController struct {
	fortunes service.FortuneService
	tasks    service.TaskRunner
	articles service.ArticleService
	limits   repository.RequestLimitRepository
}

// NewAdminController 装配 admin 控制器。
// limits 直收 RequestLimitRepository：/limits 映射 AdminTodaySummary（brief 钉死的
// 数据源，查询本身已有 repository_test 覆盖；不为此扩 LimitService 接口）。
func NewAdminController(fortunes service.FortuneService, tasks service.TaskRunner,
	articles service.ArticleService, limits repository.RequestLimitRepository) *AdminController {
	return &AdminController{fortunes: fortunes, tasks: tasks, articles: articles, limits: limits}
}

// dateParam 校验 :date 路径参数为 YYYY-MM-DD（非法 10001）；通过返回原串。
func dateParam(ctx *gin.Context) (string, bool) {
	raw := ctx.Param("date")
	if _, err := time.Parse("2006-01-02", raw); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return "", false
	}
	return raw, true
}

// ---- 运势（starfuyu:fortune）----

// ListFortunes godoc
//
//	@Summary		运势分页
//	@Description	权限 starfuyu:fortune；统一 pkg/resp 信封。date/constellation 缺省不过滤（spec §4.2）。
//	@Tags			starfuyu-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			page	query		integer	false	"页码，默认 1"	Example(1)
//	@Param			size	query		integer	false	"每页条数，默认 10，≤100"	Example(10)
//	@Param			date	query		string	false	"YYYY-MM-DD 精确过滤"	Example(2026-11-12)
//	@Param			constellation	query		string	false	"星座精确过滤"	Example(白羊座)
//	@Success		200		{object}	map[string]interface{}
//	@Router			/admin/starfuyu/fortunes [get]
func (c *AdminController) ListFortunes(ctx *gin.Context) {
	var req struct {
		resp.PageReq
		Date          string `form:"date"`
		Constellation string `form:"constellation"`
	}
	if err := ctx.ShouldBindQuery(&req); err != nil { // PageReq binding max=100 挡超限
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	req.Defaults()
	if req.Date != "" {
		if _, err := time.Parse("2006-01-02", req.Date); err != nil {
			resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
			return
		}
	}
	list, total, err := c.fortunes.ListAdmin(ctx.Request.Context(),
		req.Date, req.Constellation, req.Page, req.Size)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.Page(ctx, list, total, req.PageReq)
}

// FortuneByDate godoc
//
//	@Summary		指定日期全星座运势
//	@Description	权限 starfuyu:fortune；无行 → 30501（spec §7 admin 详情）。
//	@Tags			starfuyu-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			date	path	string	true	"YYYY-MM-DD"	Example(2026-11-12)
//	@Success		200		{object}	map[string]interface{}
//	@Router			/admin/starfuyu/fortunes/{date} [get]
func (c *AdminController) FortuneByDate(ctx *gin.Context) {
	date, ok := dateParam(ctx)
	if !ok {
		return
	}
	list, err := c.fortunes.GetByDate(ctx.Request.Context(), date)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	if len(list) == 0 {
		resp.Fail(ctx, errs.New(errs.CodeStarfuyuFortuneNotFound, nil))
		return
	}
	resp.OK(ctx, list)
}

// FortuneArticle godoc
//
//	@Summary		公众号文章 HTML 预览
//	@Description	权限 starfuyu:fortune；整版（12 星座）HTML 生成，不上传（spec §4.2）。
//	@Tags			starfuyu-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			date	path	string	true	"YYYY-MM-DD"	Example(2026-11-12)
//	@Success		200		{object}	map[string]interface{}
//	@Router			/admin/starfuyu/fortunes/{date}/article [get]
func (c *AdminController) FortuneArticle(ctx *gin.Context) {
	date, ok := dateParam(ctx)
	if !ok {
		return
	}
	html, err := c.articles.BuildHTML(ctx.Request.Context(), date, "")
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, gin.H{"html": html})
}

// UploadMP godoc
//
//	@Summary		文章 + 封面素材传公众号草稿箱
//	@Description	权限 starfuyu:fortune；返回每篇 {title,media_id}。密钥空配 → 30507 不打网络（spec §11.4）。
//	@Tags			starfuyu-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			date	path	string	true	"YYYY-MM-DD"	Example(2026-11-12)
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Router			/admin/starfuyu/fortunes/{date}/upload-mp [post]
func (c *AdminController) UploadMP(ctx *gin.Context) {
	date, ok := dateParam(ctx)
	if !ok {
		return
	}
	list, err := c.articles.UploadToMP(ctx.Request.Context(), date)
	if err != nil {
		resp.Fail(ctx, err) // 30507（空密钥）等 BizError 原样透传
		return
	}
	resp.OK(ctx, list)
}

// ---- 手动任务（starfuyu:task）----

// taskBody 三个任务端点的公共入参（全部可选：缺省 date=今日、force=false）。
type taskBody struct {
	Date  string `json:"date"`
	Force bool   `json:"force"`
}

// bindTask 绑定任务 body。空 body（io.EOF）按零值缺省——所有字段可选，
// curl -X POST 裸触发等价 {}；date 缺省 TodaySH、非法 → 10001。
func bindTask(ctx *gin.Context) (taskBody, bool) {
	var body taskBody
	if err := ctx.ShouldBindJSON(&body); err != nil && !errors.Is(err, io.EOF) {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return body, false
	}
	if body.Date == "" {
		body.Date = service.TodaySH()
	} else if _, err := time.Parse("2006-01-02", body.Date); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return body, false
	}
	return body, true
}

// TaskFetchFortune godoc
//
//	@Summary		手动抓取指定日运势
//	@Description	权限 starfuyu:task；body date 缺省今日，force:true 覆盖重抓（决定 17）；同步返回（决定 20）。
//	@Tags			starfuyu-admin
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Success		200	{object}	map[string]interface{}
//	@Router			/admin/starfuyu/tasks/fetch-fortune [post]
func (c *AdminController) TaskFetchFortune(ctx *gin.Context) {
	body, ok := bindTask(ctx)
	if !ok {
		return
	}
	if err := c.tasks.FetchDate(ctx.Request.Context(), body.Date, body.Force); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, gin.H{"date": body.Date})
}

// TaskGenerateImages godoc
//
//	@Summary		手动生成指定日运势图
//	@Description	权限 starfuyu:task；body date 缺省今日，生成 12 星座图；同步返回。
//	@Tags			starfuyu-admin
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Success		200	{object}	map[string]interface{}
//	@Router			/admin/starfuyu/tasks/generate-images [post]
func (c *AdminController) TaskGenerateImages(ctx *gin.Context) {
	body, ok := bindTask(ctx)
	if !ok {
		return
	}
	if err := c.tasks.GenerateImagesDate(ctx.Request.Context(), body.Date); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, gin.H{"date": body.Date})
}

// TaskExecuteAll godoc
//
//	@Summary		抓取 + 生图一条龙
//	@Description	权限 starfuyu:task；先 Fetch 后 Generate（调用序同 cron），body 同 fetch-fortune；同步返回。
//	@Tags			starfuyu-admin
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Success		200	{object}	map[string]interface{}
//	@Router			/admin/starfuyu/tasks/execute-all [post]
func (c *AdminController) TaskExecuteAll(ctx *gin.Context) {
	body, ok := bindTask(ctx)
	if !ok {
		return
	}
	if err := c.tasks.ExecuteAllDate(ctx.Request.Context(), body.Date, body.Force); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, gin.H{"date": body.Date})
}

// Limits godoc
//
//	@Summary		今日限流消耗概览
//	@Description	权限 starfuyu:task；按 type 聚合 SUM(count)（运维排查用，spec §4.2）。
//	@Tags			starfuyu-admin
//	@Produce		json
//	@Security		Bearer
//	@Success		200	{object}	map[string]interface{}
//	@Router			/admin/starfuyu/limits [get]
func (c *AdminController) Limits(ctx *gin.Context) {
	rows, err := c.limits.AdminTodaySummary(ctx.Request.Context(), service.TodaySH())
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	if rows == nil {
		rows = []repository.TodayLimitRow{} // 空聚合 data=[] 不是 null
	}
	resp.OK(ctx, rows)
}

// ---- 公开读（spec §4.3；与 AdminController 同文件：brief 文件清单只列 admin.go）----

// PublicController 免鉴权公开读端点（/api/public/starfuyu/image/*key）。
// storage key 形如 fortune/v3/{date}/{constellation}.jpg 含斜杠，必须用 *key 通配。
type PublicController struct {
	driver storage.Driver
}

func NewPublicController(driver storage.Driver) *PublicController {
	return &PublicController{driver: driver}
}

// Image godoc
//
//	@Summary		运势图公开读（免鉴权）
//	@Description	storage key → inline 图片；穿越/前缀不符/不存在统一 30010（不泄露存在性，spec §4.3）。
//	@Tags			starfuyu-public
//	@Produce		json
//	@Param			key	path	string	true	"storage key（fortune/v3/{date}/{constellation}.jpg）"
//	@Success		200	{file}	string
//	@Failure		400	{object}	map[string]interface{}
//	@Router			/public/starfuyu/image/{key} [get]
func (c *PublicController) Image(ctx *gin.Context) {
	key := strings.TrimPrefix(ctx.Param("key"), "/") // *key 通配含前导 /
	clean := filepath.Clean(key)
	// 防穿越（brief：filepath.Clean + 前缀校验）：拒绝对路径/越出根的 ".."，
	// 且只放行 fortune/ 前缀——storage 根为多模块共享，缺前缀校验会泄露
	// 其他模块的文件。驱动 resolve 是第二道保险。
	if key == "" || filepath.IsAbs(clean) || clean == ".." ||
		strings.HasPrefix(clean, ".."+string(filepath.Separator)) ||
		!strings.HasPrefix(clean, "fortune/") {
		resp.Fail(ctx, errs.New(errs.CodeFileNotFound, nil))
		return
	}
	rc, err := c.driver.Open(ctx.Request.Context(), clean)
	if err != nil {
		resp.Fail(ctx, errs.New(errs.CodeFileNotFound, err))
		return
	}
	defer func() { _ = rc.Close() }()
	ctx.Header("Content-Type", mimeByExt(clean))
	ctx.Header("Content-Disposition", "inline")
	ctx.Header("X-Content-Type-Options", "nosniff")
	ctx.Status(http.StatusOK)
	_, _ = io.Copy(ctx.Writer, rc) // 头已发，拷贝失败无法回信封——与 infra file serve 同豁免
}

// mimeByExt 按扩展名给 inline 图片 Content-Type（ImageService 产出恒 .jpg，兜底 octet-stream）。
func mimeByExt(key string) string {
	switch strings.ToLower(filepath.Ext(key)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}
