package app

import (
	"github.com/gin-gonic/gin"

	"alexGo-cloud/pkg/resp"
)

type AppFriendBlackController struct{}

func NewAppFriendBlackController() *AppFriendBlackController {
	return &AppFriendBlackController{}
}

func (c *AppFriendBlackController) Ping(ctx *gin.Context) {
	resp.OK(ctx, nil)
}
