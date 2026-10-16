package app

import (
	"github.com/gin-gonic/gin"

	"alexGo-cloud/pkg/resp"
)

type AppCodegenDemoItemController struct{}

func NewAppCodegenDemoItemController() *AppCodegenDemoItemController {
	return &AppCodegenDemoItemController{}
}

func (c *AppCodegenDemoItemController) Ping(ctx *gin.Context) {
	resp.OK(ctx, nil)
}
