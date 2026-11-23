package taokouling

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"alexGo-cloud/alexgo-server/server"
	"alexGo-cloud/modules/taokouling/controller/admin"
	"alexGo-cloud/modules/taokouling/controller/app"
	"alexGo-cloud/modules/taokouling/repository"
	"alexGo-cloud/modules/taokouling/service"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/migrate"
)

// FxModule taokouling 模块的 fx 装配（newmodule 生成，业务逻辑请手工扩展）。
var FxModule = fx.Module("taokouling",
	fx.Provide(
		repository.NewChannelRepository,
		service.NewChannelService,
		admin.NewAdminChannelController,
		app.NewAppChannelController,
		repository.NewLogRepository,
		service.NewLogService,
		admin.NewAdminLogController,
		app.NewAppLogController,
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

	Cfg              *config.Config
	ChannelAdminCtrl *admin.AdminChannelController
	ChannelAppCtrl   *app.AppChannelController
	LogAdminCtrl     *admin.AdminLogController
	LogAppCtrl       *app.AppLogController
}

type taokoulingModule struct {
	cfg              *config.Config
	ChannelAdminCtrl *admin.AdminChannelController
	ChannelAppCtrl   *app.AppChannelController
	LogAdminCtrl     *admin.AdminLogController
	LogAppCtrl       *app.AppLogController
}

// NewModule 聚合控制器，产出 server.Module（挂进 group:"modules"）。
func NewModule(p ModuleParams) server.Module {
	return &taokoulingModule{
		cfg:              p.Cfg,
		ChannelAdminCtrl: p.ChannelAdminCtrl,
		ChannelAppCtrl:   p.ChannelAppCtrl,
		LogAdminCtrl:     p.LogAdminCtrl,
		LogAppCtrl:       p.LogAppCtrl,
	}
}

// RegisterRoutes 挂载 taokouling 路由。入参是 /api group（见 server/http.go），
// 故 admin 用 /admin/taokouling、app 用 /app/taokouling（member 基准双前缀）。
// config 开关：modules.taokouling: false 时跳过注册。
func (m *taokoulingModule) RegisterRoutes(r *gin.RouterGroup) {
	if m.cfg != nil {
		if on, ok := m.cfg.Modules["taokouling"]; ok && !on {
			return
		}
	}
	// Channel 的 CRUD 路由由 codegen 生成的 module_register_channel.go 声明，这里接线。
	RegisterChannelDBRoutes(r, m.ChannelAdminCtrl)
	// Log 的 CRUD 路由由 codegen 生成的 module_register_log.go 声明，这里接线。
	RegisterLogDBRoutes(r, m.LogAdminCtrl)

	appGroup := r.Group("/app/taokouling")
	appGroup.GET("/channels/ping", m.ChannelAppCtrl.Ping)
	appGroup.GET("/logs/ping", m.LogAppCtrl.Ping)
	// 跨模块接口映射（如 token.Issuer）归入口 main.go，模块内不重复 Provide——
	// 需要时在此上方参照 modules/member/module.go 补 fx.Provide 映射。
}
