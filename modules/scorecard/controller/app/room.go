package app

import (
	"github.com/gin-gonic/gin"

	"alexGo-cloud/pkg/resp"
)

type AppRoomController struct{}

func NewAppRoomController() *AppRoomController {
	return &AppRoomController{}
}

func (c *AppRoomController) Ping(ctx *gin.Context) {
	resp.OK(ctx, nil)
}
