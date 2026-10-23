package app

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/fishgame/middleware"
	"alexGo-cloud/modules/fishgame/service"
	"alexGo-cloud/pkg/config"
)

// ShareController 分享图上传（multipart 字段 image，裸 JSON {"url"}）。
type ShareController struct {
	svc *service.ShareService
	cfg *config.Config
}

func NewShareController(svc *service.ShareService, cfg *config.Config) *ShareController {
	return &ShareController{svc: svc, cfg: cfg}
}

// Upload POST /share/image multipart image → {"url":"<绝对 URL>"}。
// base 优先级：cfg.Fishgame.PublicBaseURL → https + 请求 Host。
func (c *ShareController) Upload(ctx *gin.Context) {
	p, ok := middleware.PlayerFrom(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	fh, err := ctx.FormFile("image")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	f, err := fh.Open()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	defer f.Close()

	base := ""
	if c.cfg != nil {
		base = c.cfg.Fishgame.PublicBaseURL
	}
	if base == "" {
		base = "https://" + ctx.Request.Host
	}
	url, err := c.svc.Upload(ctx.Request.Context(), p.ID, fh.Filename, base, f)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrShareExt):
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "file_type_not_allowed"})
		case errors.Is(err, service.ErrShareTooLarge):
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "file_too_large"})
		default:
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		}
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"url": url})
}
