package fishgame

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"alexGo-cloud/alexgo-server/server"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/migrate"
)

// FxModule fishgame 模块的 fx 装配。
//
// 脚手架期的实体 service/admin/app 控制器与前端 CRUD 视图已随 Task 1 删除
// （scorecard 先例：脚手架产物全部重写/删除）；Task 5-9 将按业务重建
// Auth/Session/Track/Dex/Friend/Share 服务与合一版 app/admin 控制器。
var FxModule = fx.Module("fishgame",
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

	Cfg *config.Config
}

type fishgameModule struct {
	cfg *config.Config
}

// NewModule 产出 server.Module（挂进 group:"modules"）。
func NewModule(p ModuleParams) server.Module {
	return &fishgameModule{cfg: p.Cfg}
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
	// Task 5 起：v1 段（auth/session + 中间件）与 admin 段陆续接线。
}
