package app

import (
	"github.com/gin-gonic/gin"

	"alexGo-cloud/pkg/resp"
)

type AppFortuneController struct{}

func NewAppFortuneController() *AppFortuneController {
	return &AppFortuneController{}
}

func (c *AppFortuneController) Ping(ctx *gin.Context) {
	resp.OK(ctx, nil)
}
