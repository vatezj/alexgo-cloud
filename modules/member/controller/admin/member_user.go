package admin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/member/service"
)

// UserController admin 端会员管理接口（列表 / 启停）。
type UserController struct {
	svc service.MemberService
}

// NewUserController 构造控制器（fx 注入 MemberService）。
func NewUserController(svc service.MemberService) *UserController { return &UserController{svc: svc} }

// List 分页列出当前租户会员。
func (c *UserController) List(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(ctx.DefaultQuery("size", "20"))
	items, total, err := c.svc.List(ctx.Request.Context(), page, size)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": items, "total": total})
}

// UpdateStatus 启用/停用会员（PUT /users/:id/status）。
func (c *UserController) UpdateStatus(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	var req struct {
		Status int `json:"status"`
	}
	if id == 0 || ctx.ShouldBindJSON(&req) != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}
	if err := c.svc.Disable(ctx.Request.Context(), id, req.Status); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}
