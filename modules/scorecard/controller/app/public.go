package app

import (
	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/scorecard/service"
	"alexGo-cloud/pkg/resp"
)

// PublicController 免鉴权公开读端点（/api/public/scorecard/**）：
// 分享卡可能在登录前就被消费（冷启动转发），不走 AppController.claims 自鉴权。
// auth 中间件只拦 /api/admin/**，公开段天然放行（pkg/middleware/auth.go）。
type PublicController struct {
	shareCfg *service.ShareConfigService
}

// NewPublicController 构造（fx 注入 ShareConfigService）。
func NewPublicController(shareCfg *service.ShareConfigService) *PublicController {
	return &PublicController{shareCfg: shareCfg}
}

// ShareConfig godoc
//
//	@Summary		三页分享卡配置（免鉴权）
//	@Description	统一 pkg/resp 信封，data = {index,simulator,salary} → {title,imageUrl}。
//	@Description	值来自系统参数 scorecard.share.*（后台可改）；key 缺失/JSON 非法回退内置默认。
//	@Tags			scorecard-app
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}
//	@Router			/public/scorecard/share-config [get]
func (c *PublicController) ShareConfig(ctx *gin.Context) {
	resp.OK(ctx, c.shareCfg.Get(ctx.Request.Context()))
}
