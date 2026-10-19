package admin

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/member/model"
	"alexGo-cloud/modules/member/service"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/excel"
	"alexGo-cloud/pkg/resp"
)

// UserController admin 端会员管理接口（列表 / 启停）。
type UserController struct {
	svc service.MemberService
}

// NewUserController 构造控制器（fx 注入 MemberService）。
func NewUserController(svc service.MemberService) *UserController { return &UserController{svc: svc} }

// List 分页列出当前租户会员 → 统一分页信封 data:{list,total,page,size}。
func (c *UserController) List(ctx *gin.Context) {
	var req resp.PageReq
	if err := ctx.ShouldBindQuery(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	req.Defaults()
	items, total, err := c.svc.List(ctx.Request.Context(), req.Page, req.Size)
	if err != nil {
		resp.Fail(ctx, err) // 读库类 → 500 裸透传
		return
	}
	resp.Page(ctx, items, total, req)
}

// UpdateStatus 启用/停用会员（PUT /users/:id/status）。
// T5 deferred 关闭：status 用指针 bind——字段缺失（空 body/`{}`）或值 ∉ {0,1} 一律 400，
// 不再出现"空 body 默认停用"；service 层再兜一层 ErrInvalidStatus 校验。
func (c *UserController) UpdateStatus(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, nil))
		return
	}
	var req struct {
		Status *int `json:"status"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if req.Status == nil {
		resp.Fail(ctx, errs.New(errs.CodeParamMissing, nil))
		return
	}
	// 控制器先行拒非法值（与 service 双层校验，controller 直测也 400 不依赖 service 实现）。
	if *req.Status != 0 && *req.Status != 1 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, nil))
		return
	}
	if err := c.svc.Disable(ctx.Request.Context(), id, *req.Status); err != nil {
		if errors.Is(err, service.ErrInvalidStatus) {
			resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
			return
		}
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

// ExportMemberUsers 导出当前租户全量会员 xlsx（GET /users/export）。
// 租户隔离双层：svc.Export 取 ctx 租户 + repo.ListAll Where + gormplugin 经 ctx
// 照常作用；Build 失败字节未写，信封安全（RF②）。
func (c *UserController) ExportMemberUsers(ctx *gin.Context) {
	members, err := c.svc.Export(ctx.Request.Context())
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	spec := excel.Spec[*model.MemberUser]{
		Sheet:    "会员",
		FileName: excel.FileName("member_users"),
		Columns: []excel.Column[*model.MemberUser]{
			{Title: "ID", Cell: func(u *model.MemberUser) any { return u.ID }},
			{Title: "昵称", Cell: func(u *model.MemberUser) any { return u.Nickname }},
			{Title: "手机", Cell: func(u *model.MemberUser) any { return u.Mobile }},
			{Title: "状态", Cell: func(u *model.MemberUser) any { return u.Status }},
			{Title: "注册IP", Cell: func(u *model.MemberUser) any { return u.RegisterIP }},
			{Title: "登录IP", Cell: func(u *model.MemberUser) any { return u.LoginIP }},
			{Title: "登录时间", Cell: func(u *model.MemberUser) any { return excel.TimeRFC3339(u.LoginDate) }},
			{Title: "创建时间", Cell: func(u *model.MemberUser) any { return excel.TimeRFC3339(&u.CreatedAt) }},
		},
		Rows: members,
	}
	data, err := excel.Build(spec)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	ctx.Header("Content-Disposition", excel.Disposition(spec.FileName))
	ctx.Data(http.StatusOK, excel.MIME, data)
}
