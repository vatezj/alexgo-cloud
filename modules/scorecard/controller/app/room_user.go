package app

import (
	"github.com/gin-gonic/gin"

	"alexGo-cloud/pkg/resp"
)

type AppRoomUserController struct{}

func NewAppRoomUserController() *AppRoomUserController {
	return &AppRoomUserController{}
}

func (c *AppRoomUserController) Ping(ctx *gin.Context) {
	resp.OK(ctx, nil)
}
