package admin

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/service"
	errs "alexGo-cloud/pkg/errors"
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
		// 查重在 service 内（返回裸错误串），此处按语义映射「已存在」→ 30001。
		if strings.Contains(err.Error(), "already exists") {
			resp.Fail(ctx, errs.New(errs.CodeUsernameTaken, nil))
			return
		}
		resp.Fail(ctx, err)
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
