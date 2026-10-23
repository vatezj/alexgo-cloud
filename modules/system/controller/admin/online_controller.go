package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/service"
	"alexGo-cloud/pkg/auth"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/resp"
)

// OnlineController 在线会话监管（列表/踢人/代登录）。
// claims 取中间件注入（照 auth.go Profile）：Kick 需操作者身份禁踢自己，Impersonate 需审计操作者。
type OnlineController struct {
	svc service.OnlineService
}

func NewOnlineController(svc service.OnlineService) *OnlineController {
	return &OnlineController{svc: svc}
}

func onlineClaims(c *gin.Context) (*auth.Claims, bool) {
	v, ok := c.Get("claims")
	if !ok {
		return nil, false
	}
	claims, ok := v.(*auth.Claims)
	if !ok || claims == nil {
		return nil, false
	}
	return claims, true
}

// List godoc
//
//	@Summary		在线管理员会话分页
//	@Description	权限 system:online；本租户未过期管理员会话，统一 pkg/resp 信封。明文 token 不出接口。
//	@Tags			system-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			page	query	integer	false	"页码，默认 1"	Example(1)
//	@Param			size	query	integer	false	"每页条数，默认 10"	Example(10)
//	@Success		200		{object}	map[string]interface{}
//	@Router			/admin/system/online [get]
func (c *OnlineController) List(ctx *gin.Context) {
	var req resp.PageReq
	if err := ctx.ShouldBindQuery(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	req.Defaults()
	list, total, err := c.svc.List(ctx.Request.Context(), req.Page, req.Size)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.Page(ctx, list, total, req)
}

// Kick godoc
//
//	@Summary		踢下线（按行吊销会话）
//	@Description	权限 system:online；按会话行 id 吊销，明文 token 不出接口；禁踢自己。
//	@Tags			system-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			id	path	integer	true	"会话行 id"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	map[string]interface{}
//	@Router			/admin/system/online/{id} [delete]
func (c *OnlineController) Kick(ctx *gin.Context) {
	claims, ok := onlineClaims(ctx)
	if !ok {
		resp.Fail(ctx, errs.New(errs.CodeUnauthorized, nil))
		return
	}
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if err := c.svc.Kick(ctx.Request.Context(), claims, id); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

type impersonateReq struct {
	UserID uint64 `json:"user_id" binding:"required"`
}

// Impersonate godoc
//
//	@Summary		代登录到会员
//	@Description	权限 system:online；签发会员侧会话（client_id=impersonate-admin），写审计。data={access_token, refresh_token, expires_in}。
//	@Tags			system-admin
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			body	body	object	true	"目标会员"	Example({"user_id": 1})
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Router			/admin/system/online/impersonate [post]
func (c *OnlineController) Impersonate(ctx *gin.Context) {
	claims, ok := onlineClaims(ctx)
	if !ok {
		resp.Fail(ctx, errs.New(errs.CodeUnauthorized, nil))
		return
	}
	var req impersonateReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	issued, err := c.svc.Impersonate(ctx.Request.Context(), claims, ctx.ClientIP(), ctx.GetHeader("User-Agent"), req.UserID)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	// token.Issued 无 json tag（pkg/token/token.go:30 直出 Go 字段名），此处包一层
	// snake_case 出站契约：{access_token, refresh_token, expires_in}（不改 pkg/token 导出 struct）。
	resp.OK(ctx, gin.H{
		"access_token":  issued.AccessToken,
		"refresh_token": issued.RefreshToken,
		"expires_in":    issued.ExpiresIn,
	})
}
