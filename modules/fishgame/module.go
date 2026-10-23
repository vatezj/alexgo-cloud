package fishgame

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"alexGo-cloud/alexgo-server/server"
	"alexGo-cloud/modules/fishgame/controller/admin"
	"alexGo-cloud/modules/fishgame/controller/app"
	"alexGo-cloud/modules/fishgame/middleware"
	"alexGo-cloud/modules/fishgame/repository"
	"alexGo-cloud/modules/fishgame/service"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/migrate"
	"alexGo-cloud/pkg/storage"
	"alexGo-cloud/pkg/tenant"
)

// pinTidZero 游戏端不变式：ctx tenant 钉回 0（终审 Important#4）。
// 全局 NewTenantMiddleware 会把客户端可控的 X-Tenant-ID 注入 ctx；ctx tid≠0 时
// gormplugin 被激活，写侧 setIfZero 会把仓库显式 TenantID:0 覆盖成分家租户，
// 同 openid 可裂多行（uk 是 tenant_id+openid）。插件对 tid=0 静默不激活，
// 故钉 0 即完整关闭——仓库查询无需显式 tenant 过滤（照 scorecard 惯例）。
func pinTidZero(c *gin.Context) {
	c.Request = c.Request.WithContext(tenant.WithTenantID(c.Request.Context(), 0))
	c.Next()
}

// FxModule fishgame 模块的 fx 装配。
var FxModule = fx.Module("fishgame",
	fx.Provide(
		repository.NewPlayerRepository,
		repository.NewSessionRepository,
		repository.NewFishRepository,
		repository.NewDexRepository,
		repository.NewTrackEventRepository,
		repository.NewSignRepository,
		repository.NewFriendRepository,
		repository.NewConfigRepository,
		repository.NewShareRepository,
		// 微信 client：appid/secret 来自 cfg.Fishgame.Wechat（联调前可为空）。
		func(cfg *config.Config) *service.WechatClient {
			return service.NewWechatClient(cfg.Fishgame.Wechat.Appid, cfg.Fishgame.Wechat.Secret)
		},
		service.NewAuthService,
		service.NewDailyService,
		service.NewTrackService,
		service.NewDexService,
		service.NewFriendService,
		// FriendSource 抽象：一期库内关系（微信关系链留位，spec §4.6）。
		fx.Annotate(
			service.NewStoredFriendSource,
			fx.As(new(service.FriendSource)),
		),
		// ShareService：public_base_url 从 cfg 取（storage.Driver 由 system 模块提供）。
		func(cfg *config.Config, shares repository.ShareRepository, driver storage.Driver) *service.ShareService {
			return service.NewShareService(shares, driver, cfg.Fishgame.PublicBaseURL)
		},
		service.NewAdminService,
		app.NewAuthController,
		app.NewDailyController,
		app.NewTrackController,
		app.NewDexController,
		app.NewFriendController,
		app.NewShareController,
		app.NewPublicController,
		admin.NewAdminController,
		middleware.SessionAuth,
	),
	fx.Provide(
		fx.Annotate(
			NewModule,
			fx.As(new(server.Module)),
			fx.ResultTags(`group:"modules"`),
		),
	),
	fx.Provide(
		fx.Annotate(
			NewMigrationSource,
			fx.As(new(migrate.Source)),
			fx.ResultTags(`group:"migration_sources"`),
		),
	),
)

// ModuleParams 聚合依赖（fx.In）。
type ModuleParams struct {
	fx.In

	Cfg        *config.Config
	AuthCtrl   *app.AuthController
	DailyCtrl  *app.DailyController
	TrackCtrl  *app.TrackController
	DexCtrl    *app.DexController
	FriendCtrl *app.FriendController
	ShareCtrl  *app.ShareController
	PublicCtrl *app.PublicController
	AdminCtrl  *admin.AdminController
	SessionMW  gin.HandlerFunc
}

type fishgameModule struct {
	cfg        *config.Config
	authCtrl   *app.AuthController
	dailyCtrl  *app.DailyController
	trackCtrl  *app.TrackController
	dexCtrl    *app.DexController
	friendCtrl *app.FriendController
	shareCtrl  *app.ShareController
	publicCtrl *app.PublicController
	adminCtrl  *admin.AdminController
	sessionMW  gin.HandlerFunc
}

// NewModule 产出 server.Module（挂进 group:"modules"）。
func NewModule(p ModuleParams) server.Module {
	return &fishgameModule{
		cfg:        p.Cfg,
		authCtrl:   p.AuthCtrl,
		dailyCtrl:  p.DailyCtrl,
		trackCtrl:  p.TrackCtrl,
		dexCtrl:    p.DexCtrl,
		friendCtrl: p.FriendCtrl,
		shareCtrl:  p.ShareCtrl,
		publicCtrl: p.PublicCtrl,
		adminCtrl:  p.AdminCtrl,
		sessionMW:  p.SessionMW,
	}
}

// RegisterRoutes 挂载 fishgame 路由。入参是 /api group（见 server/http.go），
// 契约客户端段挂 /v1（裸 JSON）、admin 段挂 /admin/fishgame（统一信封 + casbin）。
// config 开关：modules.fishgame: false 时跳过注册。
func (m *fishgameModule) RegisterRoutes(r *gin.RouterGroup) {
	if m.cfg != nil {
		if on, ok := m.cfg.Modules["fishgame"]; ok && !on {
			return
		}
	}
	// pinTidZero：游戏段全量钉 tid=0（含 login/公开读），防 X-Tenant-ID 扰动。
	v1 := r.Group("/v1", pinTidZero)
	// login 不挂中间件（自鉴权入口）。
	v1.POST("/auth/login", m.authCtrl.Login)
	// Task 6-8 业务路由挂 v1Auth（X-Session）。
	v1Auth := v1.Group("", m.sessionMW)
	v1Auth.GET("/config/daily", m.dailyCtrl.DailyConfig)
	v1Auth.POST("/daily/sign", m.dailyCtrl.Sign)
	v1Auth.GET("/basket", m.dailyCtrl.Basket)
	v1Auth.POST("/track", m.trackCtrl.Track)
	v1Auth.GET("/dex/table", m.dexCtrl.Table)
	v1Auth.POST("/dex/report", m.dexCtrl.Report)
	v1Auth.POST("/friends", m.friendCtrl.Add)
	v1Auth.GET("/friends/rank", m.friendCtrl.Rank)
	v1Auth.POST("/share/image", m.shareCtrl.Upload)
	// 公开读（免鉴权）：RegisterRoutes 入参是 /api group → 完整路径 /api/public/fishgame/share/:key。
	r.GET("/public/fishgame/share/:key", pinTidZero, m.publicCtrl.Share)

	// admin 段（spec §4.2）：统一信封 + casbin 按 permissionRoutes 前缀鉴权。
	admin := r.Group("/admin/fishgame")
	admin.GET("/fishes", m.adminCtrl.ListFish)
	admin.POST("/fishes", m.adminCtrl.CreateFish)
	admin.PUT("/fishes/:id", m.adminCtrl.UpdateFish)
	admin.DELETE("/fishes/:id", m.adminCtrl.DeleteFish)
	admin.GET("/configs", m.adminCtrl.ListConfig)
	admin.POST("/configs", m.adminCtrl.CreateConfig)
	admin.PUT("/configs/:id", m.adminCtrl.UpdateConfig)
	admin.DELETE("/configs/:id", m.adminCtrl.DeleteConfig)
	admin.GET("/events", m.adminCtrl.ListEvents)
	admin.GET("/players", m.adminCtrl.ListPlayers)
	admin.GET("/players/:id", m.adminCtrl.PlayerDetail)
	admin.GET("/players/:id/rank", m.adminCtrl.PlayerRank)
	admin.GET("/stats", m.adminCtrl.Stats)
}
