package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/service"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/resp"
)

type MenuController struct {
	menuSvc service.MenuService
}

func NewMenuController(menuSvc service.MenuService) *MenuController {
	return &MenuController{menuSvc: menuSvc}
}

// List godoc
//
//	@Summary		菜单列表（全量树）
//	@Description	权限 system:menu；当前租户全量菜单树，无分页。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Produce		json
//	@Security		Bearer
//	@Success		200	{object}	map[string]interface{}
//	@Router			/admin/system/menus [get]
func (c *MenuController) List(ctx *gin.Context) {
	items, err := c.menuSvc.List(ctx.Request.Context())
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, items)
}

// Create godoc
//
//	@Summary		新建菜单
//	@Description	权限 system:menu。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			body	body	model.Menu	true	"菜单"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Router			/admin/system/menus [post]
func (c *MenuController) Create(ctx *gin.Context) {
	var req model.Menu
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	item, err := c.menuSvc.Create(ctx.Request.Context(), &req)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, item)
}

// Update godoc
//
//	@Summary		更新菜单
//	@Description	权限 system:menu；路径 :id 覆盖 body 内 ID。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			id	path	integer		true	"菜单 ID"
//	@Param			body	body		model.Menu	true	"菜单"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Router			/admin/system/menus/{id} [put]
func (c *MenuController) Update(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, nil))
		return
	}
	var req model.Menu
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	req.ID = id
	item, err := c.menuSvc.Update(ctx.Request.Context(), &req)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, item)
}

// Delete godoc
//
//	@Summary		删除菜单
//	@Description	权限 system:menu。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			id	path	integer	true	"菜单 ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	map[string]interface{}
//	@Router			/admin/system/menus/{id} [delete]
func (c *MenuController) Delete(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, nil))
		return
	}
	if err := c.menuSvc.Delete(ctx.Request.Context(), id); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}
