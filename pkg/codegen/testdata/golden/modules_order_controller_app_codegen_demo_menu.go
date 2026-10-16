package app

import (
	"github.com/gin-gonic/gin"

	"alexGo-cloud/pkg/resp"
)

type AppCodegenDemoMenuController struct{}

func NewAppCodegenDemoMenuController() *AppCodegenDemoMenuController {
	return &AppCodegenDemoMenuController{}
}

func (c *AppCodegenDemoMenuController) Ping(ctx *gin.Context) {
	resp.OK(ctx, nil)
}
