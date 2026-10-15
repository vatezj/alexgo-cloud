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

func (c *AuditController) ListLogin(ctx *gin.Context) {
	limit, _ := strconv.Atoi(ctx.Query("limit"))
	list, err := c.auditSvc.ListLogin(ctx.Request.Context(), limit)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, list)
}

func (c *AuditController) ListOperate(ctx *gin.Context) {
	limit, _ := strconv.Atoi(ctx.Query("limit"))
	list, err := c.auditSvc.ListOperate(ctx.Request.Context(), limit)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, list)
}
