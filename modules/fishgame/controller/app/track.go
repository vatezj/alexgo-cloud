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

// Track godoc
//
//	@Summary		埋点上报（批量）
//	@Description	单条非法跳过、整批非法仍 204（契约只认 2xx）；dedup_key=sha1(openid|name|t|canonical JSON) 幂等。catch 事件驱动图鉴点亮。
//	@Tags			fishgame-app
//	@Accept			json
//	@Security		X-Session
//	@Param			body	body		object		true	"事件批次"	Example({"events": [{"name": "catch", "t": 1761200000, "props": {"fish_id": "crucian", "result": "success"}}]})
//	@Success		204
//	@Failure		400		{object}	map[string]string
//	@Failure		401		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Router			/v1/track [post]
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
