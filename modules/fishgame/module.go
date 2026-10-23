package fishgame

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"alexGo-cloud/alexgo-server/server"
	"alexGo-cloud/modules/fishgame/controller/app"
	"alexGo-cloud/modules/fishgame/middleware"
	"alexGo-cloud/modules/fishgame/repository"
	"alexGo-cloud/modules/fishgame/service"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/migrate"
)

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
		app.NewAuthController,
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

	Cfg       *config.Config
	AuthCtrl  *app.AuthController
	SessionMW gin.HandlerFunc
}

type fishgameModule struct {
	cfg       *config.Config
	authCtrl  *app.AuthController
	sessionMW gin.HandlerFunc
}

// NewModule 产出 server.Module（挂进 group:"modules"）。
func NewModule(p ModuleParams) server.Module {
	return &fishgameModule{cfg: p.Cfg, authCtrl: p.AuthCtrl, sessionMW: p.SessionMW}
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
	v1 := r.Group("/v1")
	// login 不挂中间件（自鉴权入口）。
	v1.POST("/auth/login", m.authCtrl.Login)
	// Task 6-8 业务路由挂 v1Auth（X-Session）。
	_ = v1.Group("", m.sessionMW)
}
