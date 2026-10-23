package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/service"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/resp"
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

// List godoc
//
//	@Summary		角色列表（全量）
//	@Description	权限 system:role；当前租户全量角色，无分页。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Produce		json
//	@Security		Bearer
//	@Success		200	{object}	map[string]interface{}
//	@Router			/admin/system/roles [get]
func (c *RoleController) List(ctx *gin.Context) {
	items, err := c.roleSvc.List(ctx.Request.Context())
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, items)
}

// Create godoc
//
//	@Summary		新建角色
//	@Description	权限 system:role；code 查重冲突 → CodeRoleTaken。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			body	body		object		true	"角色信息"	Example({"code": "editor", "name": "编辑", "remark": "", "sort": 0, "data_scope": 1, "data_scope_dept_ids": ""})
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Router			/admin/system/roles [post]
func (c *RoleController) Create(ctx *gin.Context) {
	var req createRoleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	item, err := c.roleSvc.Create(ctx.Request.Context(), service.RoleCreateParams{
		Code: req.Code, Name: req.Name, Remark: req.Remark, Sort: req.Sort,
		DataScope: req.DataScope, DataScopeDeptIDs: req.DataScopeDeptIDs,
	})
	if err != nil {
		resp.Fail(ctx, err) // code 查重已在 service 哨兵化（CodeRoleTaken 经 resp.Fail 直传）
		return
	}
	resp.OK(ctx, item)
}

// Delete godoc
//
//	@Summary		删除角色
//	@Description	权限 system:role。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			id	path	integer	true	"角色 ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	map[string]interface{}
//	@Router			/admin/system/roles/{id} [delete]
func (c *RoleController) Delete(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, nil))
		return
	}
	if err := c.roleSvc.Delete(ctx.Request.Context(), id); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

// AssignMenus godoc
//
//	@Summary		分配角色菜单（全量覆盖）
//	@Description	权限 system:role；menu_ids 全量替换该角色的菜单授权。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			id	path	integer		true	"角色 ID"
//	@Param			body	body		object		true	"菜单 ID 列表"	Example({"menu_ids": [1, 2, 3]})
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Router			/admin/system/roles/{id}/menus [post]
func (c *RoleController) AssignMenus(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, nil))
		return
	}
	var req assignMenusRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if err := c.roleSvc.AssignMenus(ctx.Request.Context(), id, req.MenuIDs); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}
