package app

import (
	"github.com/gin-gonic/gin"

	"alexGo-cloud/pkg/resp"
)

type AppBaziController struct{}

func NewAppBaziController() *AppBaziController {
	return &AppBaziController{}
}

func (c *AppBaziController) Ping(ctx *gin.Context) {
	resp.OK(ctx, nil)
}
