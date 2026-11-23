package app

import (
	"github.com/gin-gonic/gin"

	"alexGo-cloud/pkg/resp"
)

type AppChannelController struct{}

func NewAppChannelController() *AppChannelController {
	return &AppChannelController{}
}

func (c *AppChannelController) Ping(ctx *gin.Context) {
	resp.OK(ctx, nil)
}
