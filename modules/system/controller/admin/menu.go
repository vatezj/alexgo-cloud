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

func (c *MenuController) List(ctx *gin.Context) {
	items, err := c.menuSvc.List(ctx.Request.Context())
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, items)
}

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
