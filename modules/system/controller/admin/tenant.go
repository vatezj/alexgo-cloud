package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/service"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/resp"
)

// TenantController 租户管理（admin/system/tenants CRUD）。
type TenantController struct {
	svc service.TenantService
}

// NewTenantController 构造控制器。
func NewTenantController(svc service.TenantService) *TenantController {
	return &TenantController{svc: svc}
}

// List godoc
//
//	@Summary		租户列表
//	@Description	权限 system:tenant；全量返回不分页。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Produce		json
//	@Security		Bearer
//	@Success		200	{object}	map[string]interface{}
//	@Router			/admin/system/tenants [get]
func (c *TenantController) List(ctx *gin.Context) {
	items, err := c.svc.List(ctx.Request.Context())
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, items)
}

// tenantRequest 创建/更新租户的请求体。
type tenantRequest struct {
	Name         string `json:"name"`
	Domain       string `json:"domain"`
	Status       int    `json:"status"`
	AccountLimit int    `json:"account_limit"`
}

// Create godoc
//
//	@Summary		新建租户
//	@Description	权限 system:tenant；status 恒 1=启用；account_limit 缺省 0 → 归一为 -1 不限。
//	@Tags			system-admin
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			body	body	object	true	"租户信息"	Example({"name": "示例租户", "domain": "example.com", "account_limit": 0})
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Router			/admin/system/tenants [post]
func (c *TenantController) Create(ctx *gin.Context) {
	var req tenantRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if req.AccountLimit == 0 {
		req.AccountLimit = -1
	}
	item, err := c.svc.Create(ctx.Request.Context(), req.Name, req.Domain, req.AccountLimit)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, item)
}

// Update godoc
//
//	@Summary		更新租户
//	@Description	权限 system:tenant；name/domain/status/account_limit 全量更新；id 非法 → 10001。
//	@Tags			system-admin
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			id		path	integer	true	"租户 id"
//	@Param			body	body	object	true	"租户信息"	Example({"name": "示例租户", "domain": "example.com", "status": 1, "account_limit": -1})
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Router			/admin/system/tenants/{id} [put]
func (c *TenantController) Update(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	var req tenantRequest
	if id == 0 || ctx.ShouldBindJSON(&req) != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, nil))
		return
	}
	if err := c.svc.Update(ctx.Request.Context(), id, req.Name, req.Domain, req.Status, req.AccountLimit); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

// Delete godoc
//
//	@Summary		删除租户（软删）
//	@Description	权限 system:tenant；id 非法 → 10001。
//	@Tags			system-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			id	path	integer	true	"租户 id"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Router			/admin/system/tenants/{id} [delete]
func (c *TenantController) Delete(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, nil))
		return
	}
	if err := c.svc.Delete(ctx.Request.Context(), id); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}
