package app

import (
	"github.com/gin-gonic/gin"

	"alexGo-cloud/pkg/resp"
)

type AppProfileController struct{}

func NewAppProfileController() *AppProfileController {
	return &AppProfileController{}
}

func (c *AppProfileController) Ping(ctx *gin.Context) {
	resp.OK(ctx, nil)
}
