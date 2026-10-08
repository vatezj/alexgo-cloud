package admin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/{{.Module}}/model"
	"alexGo-cloud/modules/{{.Module}}/service"
)

type Admin{{.Entity}}Controller struct {
	svc service.{{.Entity}}Service
}

func NewAdmin{{.Entity}}Controller(svc service.{{.Entity}}Service) *Admin{{.Entity}}Controller {
	return &Admin{{.Entity}}Controller{svc: svc}
}

func (c *Admin{{.Entity}}Controller) List(ctx *gin.Context) {
	list, err := c.svc.List(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": list})
}

func (c *Admin{{.Entity}}Controller) Create(ctx *gin.Context) {
	var req model.{{.Entity}}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := c.svc.Create(ctx.Request.Context(), &req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (c *Admin{{.Entity}}Controller) Delete(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := c.svc.Delete(ctx.Request.Context(), id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

