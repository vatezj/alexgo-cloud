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

// DailyConfig godoc
//
//	@Summary		每日配置（运势 + 鱼王）
//	@Description	运营覆盖优先，否则确定性自算；响应 {"luck":{"period","depth_bias"},"fish_king":{"fish_id","week"}}。
//	@Tags			fishgame-app
//	@Produce		json
//	@Security		X-Session
//	@Success		200	{object}	service.DailyConfigResponse
//	@Failure		401	{object}	map[string]string
//	@Failure		500	{object}	map[string]string
//	@Router			/v1/config/daily [get]
func (c *DailyController) DailyConfig(ctx *gin.Context) {
	out, err := c.svc.DailyConfig(ctx.Request.Context(), time.Now())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	ctx.JSON(http.StatusOK, out)
}

// Sign godoc
//
//	@Summary		每日签到
//	@Description	streak 作对账参考，服务端权威计算连击；同日重复 {"ok":false}；date 必须 YYYY-MM-DD。
//	@Tags			fishgame-app
//	@Accept			json
//	@Produce		json
//	@Security		X-Session
//	@Param			body	body		object		true	"签到请求"	Example({"date": "2026-10-23", "streak": 2})
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]string
//	@Failure		401		{object}	map[string]string
//	@Router			/v1/daily/sign [post]
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

// Basket godoc
//
//	@Summary		茶篓产出（纯计算，不落表）
//	@Description	elapsed = now-since → bait=min(elapsed/2400,12)，elapsed≥8h 时 capped=true；since 缺省为服务端当前时刻。
//	@Tags			fishgame-app
//	@Produce		json
//	@Security		X-Session
//	@Param			since	query		integer		false	"客户端本地起算点（unix 秒）"	Example(1761200000)
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]string
//	@Failure		401		{object}	map[string]string
//	@Router			/v1/basket [get]
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
