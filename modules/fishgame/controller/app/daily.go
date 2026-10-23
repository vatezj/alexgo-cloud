package app

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/fishgame/middleware"
	"alexGo-cloud/modules/fishgame/service"
)

// DailyController 每日循环三端点（裸 JSON，不调 pkg/resp）。
type DailyController struct {
	svc *service.DailyService
}

func NewDailyController(svc *service.DailyService) *DailyController {
	return &DailyController{svc: svc}
}

// DailyConfig GET /config/daily → {"luck":{...},"fish_king":{...}}。
func (c *DailyController) DailyConfig(ctx *gin.Context) {
	out, err := c.svc.DailyConfig(ctx.Request.Context(), time.Now())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	ctx.JSON(http.StatusOK, out)
}

// Sign POST /daily/sign body {"date","streak"} → {"ok","bait"}。
// streak 作对账参考，服务端权威计算；date 解析失败 400。
func (c *DailyController) Sign(ctx *gin.Context) {
	var body struct {
		Date   string `json:"date" binding:"required"`
		Streak int    `json:"streak"`
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
	got, bait, err := c.svc.Sign(ctx.Request.Context(), p.ID, body.Date)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "bad_date"})
		return
	}
	if !got {
		ctx.JSON(http.StatusOK, gin.H{"ok": false})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"ok": true, "bait": bait})
}

// Basket GET /basket?since=unix → {"bait","capped"}。
func (c *DailyController) Basket(ctx *gin.Context) {
	since := time.Now().Unix()
	if v := ctx.Query("since"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "bad_since"})
			return
		}
		since = n
	}
	bait, capped, err := c.svc.Basket(since)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"bait": bait, "capped": capped})
}
