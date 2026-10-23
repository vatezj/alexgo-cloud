package app

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/fishgame/middleware"
	"alexGo-cloud/modules/fishgame/service"
)

// FriendController 好友两端点（裸 JSON，不调 pkg/resp）。
type FriendController struct {
	svc *service.FriendService
}

func NewFriendController(svc *service.FriendService) *FriendController {
	return &FriendController{svc: svc}
}

// Add godoc
//
//	@Summary		添加好友（按 openid）
//	@Description	自身 → self_friend；目标不存在 → target_not_found；成功 {"ok":true}。
//	@Tags			fishgame-app
//	@Accept			json
//	@Produce		json
//	@Security		X-Session
//	@Param			body	body		object		true	"目标玩家"	Example({"openid": "oABC123"})
//	@Success		200		{object}	map[string]bool
//	@Failure		400		{object}	map[string]string
//	@Failure		401		{object}	map[string]string
//	@Router			/v1/friends [post]
func (c *FriendController) Add(ctx *gin.Context) {
	var body struct {
		OpenID string `json:"openid" binding:"required"`
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
	err := c.svc.Add(ctx.Request.Context(), &p, body.OpenID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSelfFriend):
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "self_friend"})
		case errors.Is(err, service.ErrTargetNotFound):
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "target_not_found"})
		default:
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		}
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"ok": true})
}

// Rank godoc
//
//	@Summary		好友榜（双口径）
//	@Description	好友源非空 → 好友 ∪ 自己；空 → 同租户全体；dex 降序、nick 升序决胜。
//	@Tags			fishgame-app
//	@Produce		json
//	@Security		X-Session
//	@Success		200	{object}	map[string]interface{}
//	@Failure		401	{object}	map[string]string
//	@Failure		500	{object}	map[string]string
//	@Router			/v1/friends/rank [get]
func (c *FriendController) Rank(ctx *gin.Context) {
	p, ok := middleware.PlayerFrom(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	rows, err := c.svc.Rank(ctx.Request.Context(), &p)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"list": rows})
}
