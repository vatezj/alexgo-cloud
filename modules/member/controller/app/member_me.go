package app

import (
	"strings"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/member/service"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/resp"
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/token"
)

// MeController app 端会员个人资料接口（GET/PUT me；Bearer 自鉴权，与 scorecard
// controller/app/app.go 同构——登录态中间件放行后 handler 内 claims 取 userID）。
type MeController struct {
	svc       service.MemberService
	validator token.Validator
}

// NewMeController 构造控制器（fx 注入 MemberService + token.Validator；
// Validator 的接口映射归入口 main.go，模块内不重复 Provide）。
func NewMeController(svc service.MemberService, validator token.Validator) *MeController {
	return &MeController{svc: svc, validator: validator}
}

// claims 解析 Bearer 并校验 member 身份；成功后把 token 内租户写回请求 ctx
// （与 scorecard app.go claims 同一 API——伪造 X-Tenant-ID 头无法跨租户）；
// 失败已写响应，ok=false 直接 return。
func (c *MeController) claims(ctx *gin.Context) (*token.Claims, bool) {
	raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(ctx.GetHeader("Authorization")), "Bearer "))
	if raw == "" {
		resp.Fail(ctx, errs.New(errs.CodeUnauthorized, nil))
		return nil, false
	}
	cl, err := c.validator.Validate(ctx.Request.Context(), raw)
	if err != nil {
		resp.Fail(ctx, errs.New(errs.CodeUnauthorized, nil))
		return nil, false
	}
	if cl.UserType != token.UserTypeMember {
		resp.Fail(ctx, errs.New(errs.CodeForbidden, nil))
		return nil, false
	}
	ctx.Request = ctx.Request.WithContext(tenant.WithTenantID(ctx.Request.Context(), cl.TenantID))
	return cl, true
}

// Me godoc
//
//	@Summary		当前登录会员资料
//	@Description	凭 Authorization Bearer 自鉴权（member 身份校验，非 X-Session）。data 键名 user_id/nickname/avatar/created_at 逐字（小程序前端消费面）。
//	@Tags			member-app
//	@Produce		json
//	@Security		Bearer
//	@Success		200	{object}	map[string]interface{}
//	@Failure		401	{object}	map[string]interface{}
//	@Router			/app/member/me [get]
func (c *MeController) Me(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	p, err := c.svc.Me(ctx.Request.Context(), cl.UserID)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, p)
}

// updateMeRequest 指针字段：缺省 nil=不改；nickname 空串由 service 拒（10001），
// avatar 空串=清空（合法）。
type updateMeRequest struct {
	Nickname *string `json:"nickname"`
	Avatar   *string `json:"avatar"`
}

// UpdateMe godoc
//
//	@Summary		部分更新会员资料
//	@Description	凭 Bearer 自鉴权。指针字段缺省 nil=不改；nickname 空串 → 10001；avatar 空串=清空（合法）。
//	@Tags			member-app
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			body	body	object	true	"要改的字段（可只给一项）"	Example({"nickname": "阿飞", "avatar": "https://cdn.example.com/a.png"})
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		401		{object}	map[string]interface{}
//	@Router			/app/member/me [put]
func (c *MeController) UpdateMe(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	var req updateMeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	p, err := c.svc.UpdateMe(ctx.Request.Context(), cl.UserID, req.Nickname, req.Avatar)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, p)
}
