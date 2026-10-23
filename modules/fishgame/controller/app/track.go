package app

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/fishgame/middleware"
	"alexGo-cloud/modules/fishgame/service"
)

// TrackController 埋点入口（裸 JSON：204 无 body，信封豁免）。
type TrackController struct {
	svc *service.TrackService
}

func NewTrackController(svc *service.TrackService) *TrackController {
	return &TrackController{svc: svc}
}

// Track POST /track body {"events":[...]} → 204。
// 单条非法跳过、整批非法仍 204（契约只认 2xx）；失败 500 裸 error。
func (c *TrackController) Track(ctx *gin.Context) {
	var body struct {
		Events []service.TrackEventReq `json:"events"`
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
	if err := c.svc.Track(ctx.Request.Context(), &p, body.Events); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	ctx.Status(http.StatusNoContent)
}
