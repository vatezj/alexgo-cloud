package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/service"
	"alexGo-cloud/pkg/resp"
)

type AuditController struct {
	auditSvc service.AuditService
}

func NewAuditController(auditSvc service.AuditService) *AuditController {
	return &AuditController{auditSvc: auditSvc}
}

// ListLogin godoc
//
//	@Summary		登录日志列表
//	@Description	权限 system:log；limit 控制返回条数（query string Atoi，非 page/size 分页）。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			limit	query	integer	false	"返回条数上限"	Example(20)
//	@Success		200		{object}	map[string]interface{}
//	@Router			/admin/system/logs/login [get]
func (c *AuditController) ListLogin(ctx *gin.Context) {
	limit, _ := strconv.Atoi(ctx.Query("limit"))
	list, err := c.auditSvc.ListLogin(ctx.Request.Context(), limit)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, list)
}

// ListOperate godoc
//
//	@Summary		操作日志列表
//	@Description	权限 system:log；limit 控制返回条数（query string Atoi，非 page/size 分页）。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			limit	query	integer	false	"返回条数上限"	Example(20)
//	@Success		200		{object}	map[string]interface{}
//	@Router			/admin/system/logs/operate [get]
func (c *AuditController) ListOperate(ctx *gin.Context) {
	limit, _ := strconv.Atoi(ctx.Query("limit"))
	list, err := c.auditSvc.ListOperate(ctx.Request.Context(), limit)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, list)
}
