package admin

import (
	"errors"
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
// T5 deferred 关闭：status 用指针 bind——字段缺失（空 body/`{}`）或值 ∉ {0,1} 一律 400，
// 不再出现"空 body 默认停用"；service 层再兜一层 ErrInvalidStatus 校验。
func (c *UserController) UpdateStatus(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	var req struct {
		Status *int `json:"status"`
	}
	if id == 0 || ctx.ShouldBindJSON(&req) != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}
	if req.Status == nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "status required"})
		return
	}
	// 控制器先行拒非法值（与 service 双层校验，controller 直测也 400 不依赖 service 实现）。
	if *req.Status != 0 && *req.Status != 1 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "status must be 0 or 1"})
		return
	}
	if err := c.svc.Disable(ctx.Request.Context(), id, *req.Status); err != nil {
		if errors.Is(err, service.ErrInvalidStatus) {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}
