package app

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/fishgame/service"
)

// PublicController 免鉴权公开读端点（spec §4.5：/api/public/fishgame/share/:key）。
type PublicController struct {
	svc *service.ShareService
}

func NewPublicController(svc *service.ShareService) *PublicController {
	return &PublicController{svc: svc}
}

// Share GET /public/fishgame/share/:key → 200 inline 图字节；miss 裸 404。
func (c *PublicController) Share(ctx *gin.Context) {
	key := ctx.Param("key")
	mime, data, err := c.svc.Get(ctx.Request.Context(), key)
	if err != nil {
		if errors.Is(err, service.ErrShareNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	ctx.Header("Content-Disposition", "inline")
	ctx.Data(http.StatusOK, mime, data)
}
