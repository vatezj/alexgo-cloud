package app

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type AppCodegenDemoMenuController struct{}

func NewAppCodegenDemoMenuController() *AppCodegenDemoMenuController {
	return &AppCodegenDemoMenuController{}
}

func (c *AppCodegenDemoMenuController) Ping(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}
