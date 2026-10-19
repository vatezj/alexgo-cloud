package admin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/service"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/excel"
	"alexGo-cloud/pkg/resp"
)

type UserController struct {
	userSvc service.UserService
}

func NewUserController(userSvc service.UserService) *UserController {
	return &UserController{userSvc: userSvc}
}

type createUserRequest struct {
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Password string `json:"password"`
}

type updateUserRequest struct {
	Nickname string `json:"nickname"`
	Status   int    `json:"status"`
}

type resetPasswordRequest struct {
	Password string `json:"password"`
}

type setRolesRequest struct {
	RoleIDs []uint64 `json:"role_ids"`
}

func (c *UserController) Create(ctx *gin.Context) {
	var req createUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	u, err := c.userSvc.CreateUser(ctx.Request.Context(), req.Username, req.Nickname, req.Password)
	if err != nil {
		resp.Fail(ctx, err) // 查重已在 service 哨兵化（CodeUsernameTaken 经 resp.Fail 直传）
		return
	}
	resp.OK(ctx, u)
}

func (c *UserController) Update(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, nil))
		return
	}
	var req updateUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if err := c.userSvc.UpdateUser(ctx.Request.Context(), id, req.Nickname, req.Status); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

func (c *UserController) ResetPassword(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, nil))
		return
	}
	var req resetPasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if err := c.userSvc.ResetPassword(ctx.Request.Context(), id, req.Password); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

func (c *UserController) SetRoles(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, nil))
		return
	}
	var req setRolesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if err := c.userSvc.SetRoles(ctx.Request.Context(), id, req.RoleIDs); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

// ExportUsers 导出当前租户全量用户 xlsx（GET /users/export）。
// 先取行、再 Build、最后 ctx.Data——Build 返回 error 时一个字节未写，信封安全
// （RF② 先查后写；codegen download 同款豁免只发生在 ctx.Data 之后）。
func (c *UserController) ExportUsers(ctx *gin.Context) {
	users, err := c.userSvc.ListUsers(ctx.Request.Context())
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	spec := excel.Spec[*model.User]{
		Sheet:    "用户",
		FileName: excel.FileName("users"),
		Columns: []excel.Column[*model.User]{
			{Title: "ID", Cell: func(u *model.User) any { return u.ID }},
			{Title: "用户名", Cell: func(u *model.User) any { return u.Username }},
			{Title: "昵称", Cell: func(u *model.User) any { return u.Nickname }},
			{Title: "邮箱", Cell: func(u *model.User) any { return u.Email }},
			{Title: "手机", Cell: func(u *model.User) any { return u.Mobile }},
			{Title: "性别", Cell: func(u *model.User) any { return u.Sex }},
			{Title: "状态", Cell: func(u *model.User) any { return u.Status }},
			{Title: "部门ID", Cell: func(u *model.User) any { return u.DeptID }},
			{Title: "备注", Cell: func(u *model.User) any { return u.Remark }},
			{Title: "登录IP", Cell: func(u *model.User) any { return u.LoginIP }},
			{Title: "登录时间", Cell: func(u *model.User) any { return excel.TimeRFC3339(u.LoginDate) }},
			{Title: "创建时间", Cell: func(u *model.User) any { return excel.TimeRFC3339(&u.CreatedAt) }},
		},
		Rows: users,
	}
	data, err := excel.Build(spec)
	if err != nil {
		resp.Fail(ctx, err) // 30013 → 400 信封（statusFor 3xxxx 自动映射）
		return
	}
	ctx.Header("Content-Disposition", excel.Disposition(spec.FileName))
	ctx.Data(http.StatusOK, excel.MIME, data)
}
