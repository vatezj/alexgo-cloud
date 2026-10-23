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

// Table GET /dex/table → {"list":[...]}（只发 status=1）。
func (c *DexController) Table(ctx *gin.Context) {
	rows, err := c.svc.Table(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"list": rows})
}

// Report POST /dex/report body {"dex":{"<fish_id>":N}} → {"ok":true,"dex":总数}。
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
