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

// Add POST /friends body {"openid":"..."} → {"ok":true}；400 裸 error。
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

// Rank GET /friends/rank → {"list":[{"nick","dex"}]}。
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
