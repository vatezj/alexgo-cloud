package app

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type AppCodegenDemoItemController struct{}

func NewAppCodegenDemoItemController() *AppCodegenDemoItemController {
	return &AppCodegenDemoItemController{}
}

func (c *AppCodegenDemoItemController) Ping(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}
