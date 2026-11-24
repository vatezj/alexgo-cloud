package admin

import (
	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/taokouling/model"
	"alexGo-cloud/modules/taokouling/service"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/resp"
)

// T4b 机械适配：新仓储 ConversionLogRepository 仅 Append/List，
// 脚手架 Get/Update/Delete 无对应 service 方法，随死代码删除（T14 重写时整体替换）。
type AdminLogController struct {
	svc service.LogService
}

func NewAdminLogController(svc service.LogService) *AdminLogController {
	return &AdminLogController{svc: svc}
}

// List 全量列表（无 page/size 入参）→ resp.OK 裸数组，页面 localPage 本地切页。
func (c *AdminLogController) List(ctx *gin.Context) {
	list, err := c.svc.List(ctx.Request.Context())
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, list)
}

func (c *AdminLogController) Create(ctx *gin.Context) {
	var req model.ConversionLog
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if err := c.svc.Create(ctx.Request.Context(), &req); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}
