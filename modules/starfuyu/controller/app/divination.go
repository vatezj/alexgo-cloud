package app

import (
	"github.com/gin-gonic/gin"

	"alexGo-cloud/pkg/resp"
)

type AppDivinationController struct{}

func NewAppDivinationController() *AppDivinationController {
	return &AppDivinationController{}
}

func (c *AppDivinationController) Ping(ctx *gin.Context) {
	resp.OK(ctx, nil)
}
