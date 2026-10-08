package app

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type App{{.ServiceName}}Controller struct{}

func NewApp{{.ServiceName}}Controller() *App{{.ServiceName}}Controller {
	return &App{{.ServiceName}}Controller{}
}

func (c *App{{.ServiceName}}Controller) Ping(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

