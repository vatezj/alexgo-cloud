package admin

import (
	"strconv"
	"strings"

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

func (c *RoleController) List(ctx *gin.Context) {
	items, err := c.roleSvc.List(ctx.Request.Context())
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, items)
}

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
		// 同租户 code 查重在 service 内（返回裸错误串），按语义映射「已存在」→ 30003。
		if strings.Contains(err.Error(), "already exists") {
			resp.Fail(ctx, errs.New(errs.CodeRoleTaken, nil))
			return
		}
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, item)
}

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
