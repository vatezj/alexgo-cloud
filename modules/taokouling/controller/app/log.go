package app

import (
	"github.com/gin-gonic/gin"

	"alexGo-cloud/pkg/resp"
)

type AppLogController struct{}

func NewAppLogController() *AppLogController {
	return &AppLogController{}
}

func (c *AppLogController) Ping(ctx *gin.Context) {
	resp.OK(ctx, nil)
}
