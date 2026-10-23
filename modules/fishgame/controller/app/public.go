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

// Share godoc
//
//	@Summary		分享图公开读（免鉴权）
//	@Description	按 key 直出图片字节（Content-Disposition: inline）；miss 404 {"error":"not_found"}。
//	@Tags			fishgame-app
//	@Produce		json
//	@Param			key	path		string		true	"上传返回 URL 中的 key"
//	@Success		200	{file}		string
//	@Failure		404	{object}	map[string]string
//	@Router			/public/fishgame/share/{key} [get]
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
