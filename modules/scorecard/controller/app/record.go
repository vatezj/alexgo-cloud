package app

import (
	"github.com/gin-gonic/gin"

	"alexGo-cloud/pkg/resp"
)

type AppRecordController struct{}

func NewAppRecordController() *AppRecordController {
	return &AppRecordController{}
}

func (c *AppRecordController) Ping(ctx *gin.Context) {
	resp.OK(ctx, nil)
}
