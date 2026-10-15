package admin

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/member/service"
	errs "alexGo-cloud/pkg/errors"
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
