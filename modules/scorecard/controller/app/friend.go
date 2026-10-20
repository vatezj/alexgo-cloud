package app

import (
	"github.com/gin-gonic/gin"

	"alexGo-cloud/pkg/resp"
)

type AppFriendController struct{}

func NewAppFriendController() *AppFriendController {
	return &AppFriendController{}
}

func (c *AppFriendController) Ping(ctx *gin.Context) {
	resp.OK(ctx, nil)
}
