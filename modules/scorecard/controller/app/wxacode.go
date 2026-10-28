package app

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"alexGo-cloud/modules/system/oauth2/social"
	"alexGo-cloud/pkg/config"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/resp"
)

// defaultWxacodeEnvVersion 小程序码 env_version 默认值：体验版 "trial"
// （page 未正式发布时也能出码）。可经 scorecard.wxacode_env_version 配置覆盖
// （release=正式版 / develop=开发版）；正式发版改配 release。
const defaultWxacodeEnvVersion = "trial"

// wxEnvVersion 从全局配置解析小程序码 env_version：未配/空串回退默认 trial
// （与历史硬编码行为一致），否则透传配置值。
func wxEnvVersion(cfg *config.Config) string {
	if cfg == nil || cfg.Scorecard.WxacodeEnvVersion == "" {
		return defaultWxacodeEnvVersion
	}
	return cfg.Scorecard.WxacodeEnvVersion
}

// wxacodePage 小程序码落地页（Task 12 首页消费 scene）。
const wxacodePage = "pages/index/index"

// wxacodeCacheTTL 小程序码进程内按 scene 缓存时长（spec §3.9.1 短 TTL，量小防频调）。
// 键为完整 scene（含 inv 请求者）：同房两人取码各自打微信，不互串缓存。
const wxacodeCacheTTL = 5 * time.Minute

// wxacodeCacheEntry 小程序码缓存条目（PNG 字节原样缓存）。
type wxacodeCacheEntry struct {
	png   []byte
	expAt time.Time
}

// Wxacode godoc
//
//	@Summary		房间小程序码
//	@Description	200 直出 image/png 字节（非 JSON 信封），错误路径仍走 pkg/resp 五位码信封：30101 房不存在、30102 非成员、50001 渠道未配置/停用、50000 微信侧失败（平台错误只进日志不外泄）。scene=qr+房号&inv+请求者UserID，进程内按 scene 缓存 5min（防跨用户串码）。
//	@Tags			scorecard-app
//	@Produce		png
//	@Security		Bearer
//	@Param			id	path	integer	true	"房间 ID"	Example(1)
//	@Success		200		{file}		string
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		401		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/app/scorecard/rooms/{id}/wxacode [get]
func (c *AppController) Wxacode(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	id, ok := pathID(ctx, "id")
	if !ok {
		return
	}
	// 房不存在/跨租户 30101、非成员 30102（快照组装的成员昵称此处不消费，只取房号）。
	snap, err := c.rooms.Snapshot(ctx.Request.Context(), cl.UserID, id)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	// scene = qr+房号&inv+请求者UserID（前端按 qr= 前缀解析进房 + inv 静默绑好友；
	// 缓存键含 inv，同房两人取码不互串）。
	scene := fmt.Sprintf("qr=%s&inv=%d", snap.RoomCode, cl.UserID)
	if png, ok := c.cachedWxacode(scene); ok {
		ctx.Data(http.StatusOK, "image/png", png)
		return
	}
	// 渠道配置：social 登录取配置同款 GetApp(tenant, "wechat_mp")；未配置/停用 → 500 明确 msg。
	app, err := c.apps.GetApp(ctx.Request.Context(), cl.TenantID, social.PlatformWechatMP)
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && app.Status != 1) {
		resp.Fail(ctx, errs.New(errs.CodeWxacodeNotConfigured, err))
		return
	}
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	// scene 前缀 qr= 由控制器拼（房号+inv 取自房行与 claims，social 层只透传）。
	png, err := c.wx.Wxacode(ctx.Request.Context(), app, scene, wxacodePage, c.wxEnvVersion)
	if err != nil {
		// 微信侧失败：平台错误（errcode 等）只进日志、不外泄（resp.Fail 对非 BizError
		// 记 Error 日志 + 回泛化 50000），前端按契约降级房号展示。
		resp.Fail(ctx, err)
		return
	}
	c.storeWxacode(scene, png)
	ctx.Data(http.StatusOK, "image/png", png)
}

// cachedWxacode 命中未过期缓存 → (PNG 字节, true)。
func (c *AppController) cachedWxacode(scene string) ([]byte, bool) {
	c.wxMu.Lock()
	defer c.wxMu.Unlock()
	e, ok := c.wxCache[scene]
	if !ok || !time.Now().Before(e.expAt) {
		return nil, false
	}
	return e.png, true
}

// storeWxacode 写入/覆盖按 scene 的缓存条目（键含 inv 请求者，天然按人隔离）。
func (c *AppController) storeWxacode(scene string, png []byte) {
	c.wxMu.Lock()
	defer c.wxMu.Unlock()
	c.wxCache[scene] = wxacodeCacheEntry{png: png, expAt: time.Now().Add(wxacodeCacheTTL)}
}
