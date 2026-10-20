package app

import (
	"github.com/gin-gonic/gin"

	"alexGo-cloud/pkg/resp"
)

type AppRoomLogController struct{}

func NewAppRoomLogController() *AppRoomLogController {
	return &AppRoomLogController{}
}

func (c *AppRoomLogController) Ping(ctx *gin.Context) {
	resp.OK(ctx, nil)
}
