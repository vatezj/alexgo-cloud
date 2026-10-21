package controller

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/oauth2/social"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/resp"
)

// AppConfiger AppConfigService 的 admin 门面（controller 只依赖接口，测试可 stub）。
type AppConfiger interface {
	List(ctx context.Context, page, size int, platform string) ([]*social.SocialApp, int64, error)
	Create(ctx context.Context, app *social.SocialApp) error
	Update(ctx context.Context, app *social.SocialApp) error
	Delete(ctx context.Context, id uint64) error
}

// AdminController admin 渠道配置接口（casbin social:app 放行，见
// permissionRoutes；统一信封 pkg/resp）。
type AdminController struct {
	svc AppConfiger
}

func NewAdminController(svc AppConfiger) *AdminController {
	return &AdminController{svc: svc}
}

// appRequest Create/Update 请求体（主控裁决：必须的请求结构体）：
// model.SocialApp.Secret 带 json:"-"（model.go），encoding/json 绑定它时整体
// 跳过该字段——若直接绑 *model.SocialApp，Create 的 secret 永远收不到（service
// 恒 10002），Update 无法经 API 轮换 secret。明文 secret 由外层同名字段
// （深度 0，json:"secret"）单独收取，绑定后手动写回 model（先例：
// oauth2/controller/admin/client.go createClientRequest「json:"-" 不参与绑定，
// 手动写回」）。响应侧无需拆分结构：resp.OK(model) 序列化时 json:"-" 使
// secret 恒不回显。
type appRequest struct {
	social.SocialApp
	Secret string `json:"secret"`
}

// toModel 搬字段：外层 Secret 写回嵌入 model（json:"-" 未参与绑定）。
func (r appRequest) toModel() social.SocialApp {
	app := r.SocialApp
	app.Secret = r.Secret
	return app
}

// List GET /admin/social/apps?page&size&platform → data:{list,total,page,size}。
// 分页入参统一 resp.PageReq（binding max=100）；platform=="" 全平台视图。
func (a *AdminController) List(ctx *gin.Context) {
	var req resp.PageReq
	if err := ctx.ShouldBindQuery(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	req.Defaults()
	items, total, err := a.svc.List(ctx.Request.Context(), req.Page, req.Size, ctx.Query("platform"))
	if err != nil {
		resp.Fail(ctx, err) // 读库类 → 500 裸透传
		return
	}
	resp.Page(ctx, items, total, req)
}

// Create POST /admin/social/apps → data 回显（secret 不回显）。
// appid/secret 缺失与平台查重等规则在 service（10002/10001）；租户与 ID 归位
// 同样归 service（T8 app.TenantID = tid 注入），controller 只搬 body 字段。
func (a *AdminController) Create(ctx *gin.Context) {
	var req appRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	app := req.toModel()
	app.ID = 0 // 新建忽略请求 id（service 亦归 0，双保险）
	if err := a.svc.Create(ctx.Request.Context(), &app); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, &app)
}

// Update PUT /admin/social/apps/:id → data 回显；id 非法/bind 错 400。
// body 带 secret = 轮换新值；不带 = 空串 → service 保留库中旧值
// （编辑表单留空不改机密）。路径 id 优先于 body 里的 id。
func (a *AdminController) Update(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil || id == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	var req appRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	app := req.toModel()
	app.ID = id
	if err := a.svc.Update(ctx.Request.Context(), &app); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, &app)
}

// Delete DELETE /admin/social/apps/:id → 信封 code 0。
func (a *AdminController) Delete(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil || id == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if err := a.svc.Delete(ctx.Request.Context(), id); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}
