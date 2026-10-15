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

// List 列出全部租户。
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

// Create 新建租户（status 恒 1=启用；account_limit 缺省 0 → 归一为 -1 不限）。
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

// Update 更新租户（name/domain/status/account_limit）。
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

// Delete 软删租户。
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
