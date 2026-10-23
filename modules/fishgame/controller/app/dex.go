package app

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/fishgame/middleware"
	"alexGo-cloud/modules/fishgame/service"
)

// DexController 图鉴两端点（裸 JSON，不调 pkg/resp）。
type DexController struct {
	svc *service.DexService
}

func NewDexController(svc *service.DexService) *DexController {
	return &DexController{svc: svc}
}

// Table godoc
//
//	@Summary		图鉴表（只发启用条目）
//	@Description	响应 {"list":[{id,name,rarity,period,depth,flavor}]}，仅 status=1。
//	@Tags			fishgame-app
//	@Produce		json
//	@Security		X-Session
//	@Success		200	{object}	map[string]interface{}
//	@Failure		401	{object}	map[string]string
//	@Failure		500	{object}	map[string]string
//	@Router			/v1/dex/table [get]
func (c *DexController) Table(ctx *gin.Context) {
	rows, err := c.svc.Table(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"list": rows})
}

// Report godoc
//
//	@Summary		图鉴进度上报（客户端镜像对账）
//	@Description	body {"dex":{"<fish_id>":N}}；服务端双路合并取 max（非 sum），响应 {"ok":true,"dex":总数}。
//	@Tags			fishgame-app
//	@Accept			json
//	@Produce		json
//	@Security		X-Session
//	@Param			body	body		object		true	"点亮镜像"	Example({"dex": {"crucian": 2, "carp": 1}})
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]string
//	@Failure		401		{object}	map[string]string
//	@Router			/v1/dex/report [post]
func (c *DexController) Report(ctx *gin.Context) {
	var body struct {
		Dex map[string]int `json:"dex"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	p, ok := middleware.PlayerFrom(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	total, err := c.svc.Report(ctx.Request.Context(), p.ID, body.Dex)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"ok": true, "dex": total})
}
