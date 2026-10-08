package admin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/service"
)

type RoleController struct {
	roleSvc service.RoleService
}

func NewRoleController(roleSvc service.RoleService) *RoleController {
	return &RoleController{roleSvc: roleSvc}
}

type createRoleRequest struct {
	Code             string `json:"code"`
	Name             string `json:"name"`
	Remark           string `json:"remark"`
	Sort             int    `json:"sort"`
	DataScope        int    `json:"data_scope"`
	DataScopeDeptIDs string `json:"data_scope_dept_ids"`
}

type assignMenusRequest struct {
	MenuIDs []uint64 `json:"menu_ids"`
}

func (c *RoleController) List(ctx *gin.Context) {
	items, err := c.roleSvc.List(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": items})
}

func (c *RoleController) Create(ctx *gin.Context) {
	var req createRoleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := c.roleSvc.Create(ctx.Request.Context(), service.RoleCreateParams{
		Code: req.Code, Name: req.Name, Remark: req.Remark, Sort: req.Sort,
		DataScope: req.DataScope, DataScopeDeptIDs: req.DataScopeDeptIDs,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": item})
}

func (c *RoleController) Delete(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := c.roleSvc.Delete(ctx.Request.Context(), id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (c *RoleController) AssignMenus(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req assignMenusRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := c.roleSvc.AssignMenus(ctx.Request.Context(), id, req.MenuIDs); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

