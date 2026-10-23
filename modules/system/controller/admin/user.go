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

// Create godoc
//
//	@Summary		新建用户
//	@Description	权限 system:user；username 查重冲突 → CodeUsernameTaken。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			body	body		object		true	"用户信息"	Example({"username": "alice", "nickname": "爱丽丝", "password": "secret"})
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Router			/admin/system/users [post]
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

// Update godoc
//
//	@Summary		更新用户（昵称/状态）
//	@Description	权限 system:user；仅改 nickname/status，不支持改用户名/密码。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			id	path	integer		true	"用户 ID"
//	@Param			body	body		object		true	"昵称与状态"	Example({"nickname": "新昵称", "status": 1})
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Router			/admin/system/users/{id} [put]
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

// ResetPassword godoc
//
//	@Summary		重置用户密码
//	@Description	权限 system:user；直接设置新明文密码（service 内哈希）。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			id	path	integer		true	"用户 ID"
//	@Param			body	body		object		true	"新密码"	Example({"password": "new-secret"})
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Router			/admin/system/users/{id}/reset-password [post]
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

// SetRoles godoc
//
//	@Summary		设置用户角色（全量覆盖）
//	@Description	权限 system:user；role_ids 全量替换该用户角色集合。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			id	path	integer		true	"用户 ID"
//	@Param			body	body		object		true	"角色 ID 列表"	Example({"role_ids": [1, 2]})
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Router			/admin/system/users/{id}/roles [post]
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

// ExportUsers godoc
//
//	@Summary		导出用户 xlsx
//	@Description	权限 system:user；导出当前租户全量用户为 xlsx 流（非 JSON 信封）。Build 失败 → 400 信封（30013）。
//	@Tags			system-admin
//	@Produce		octet-stream
//	@Security		Bearer
//	@Success		200	{file}	file
//	@Failure		400	{object}	map[string]interface{}
//	@Router			/admin/system/users/export [get]
//
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
