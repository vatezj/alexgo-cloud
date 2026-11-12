package app

import (
	"github.com/gin-gonic/gin"

	"alexGo-cloud/pkg/resp"
)

type AppLimitController struct{}

func NewAppLimitController() *AppLimitController {
	return &AppLimitController{}
}

func (c *AppLimitController) Ping(ctx *gin.Context) {
	resp.OK(ctx, nil)
}
