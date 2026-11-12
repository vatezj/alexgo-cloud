package starfuyu

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"alexGo-cloud/alexgo-server/server"
	"alexGo-cloud/modules/starfuyu/controller/admin"
	"alexGo-cloud/modules/starfuyu/controller/app"
	"alexGo-cloud/modules/starfuyu/repository"
	"alexGo-cloud/modules/starfuyu/service"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/migrate"
)

// FxModule starfuyu 模块的 fx 装配（newmodule 生成，业务逻辑请手工扩展）。
var FxModule = fx.Module("starfuyu",
	fx.Provide(
		repository.NewProfileRepository,
		service.NewProfileService,
		admin.NewAdminProfileController,
		app.NewAppProfileController,
		repository.NewBaziRepository,
		service.NewBaziService,
		admin.NewAdminBaziController,
		app.NewAppBaziController,
		repository.NewDivinationRepository,
		service.NewDivinationService,
		admin.NewAdminDivinationController,
		app.NewAppDivinationController,
		repository.NewFortuneRepository,
		service.NewFortuneService,
		admin.NewAdminFortuneController,
		app.NewAppFortuneController,
		repository.NewLimitRepository,
		service.NewLimitService,
		admin.NewAdminLimitController,
		app.NewAppLimitController,
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

	Cfg                 *config.Config
	ProfileAdminCtrl    *admin.AdminProfileController
	ProfileAppCtrl      *app.AppProfileController
	BaziAdminCtrl       *admin.AdminBaziController
	BaziAppCtrl         *app.AppBaziController
	DivinationAdminCtrl *admin.AdminDivinationController
	DivinationAppCtrl   *app.AppDivinationController
	FortuneAdminCtrl    *admin.AdminFortuneController
	FortuneAppCtrl      *app.AppFortuneController
	LimitAdminCtrl      *admin.AdminLimitController
	LimitAppCtrl        *app.AppLimitController
}

type starfuyuModule struct {
	cfg                 *config.Config
	ProfileAdminCtrl    *admin.AdminProfileController
	ProfileAppCtrl      *app.AppProfileController
	BaziAdminCtrl       *admin.AdminBaziController
	BaziAppCtrl         *app.AppBaziController
	DivinationAdminCtrl *admin.AdminDivinationController
	DivinationAppCtrl   *app.AppDivinationController
	FortuneAdminCtrl    *admin.AdminFortuneController
	FortuneAppCtrl      *app.AppFortuneController
	LimitAdminCtrl      *admin.AdminLimitController
	LimitAppCtrl        *app.AppLimitController
}

// NewModule 聚合控制器，产出 server.Module（挂进 group:"modules"）。
func NewModule(p ModuleParams) server.Module {
	return &starfuyuModule{
		cfg:                 p.Cfg,
		ProfileAdminCtrl:    p.ProfileAdminCtrl,
		ProfileAppCtrl:      p.ProfileAppCtrl,
		BaziAdminCtrl:       p.BaziAdminCtrl,
		BaziAppCtrl:         p.BaziAppCtrl,
		DivinationAdminCtrl: p.DivinationAdminCtrl,
		DivinationAppCtrl:   p.DivinationAppCtrl,
		FortuneAdminCtrl:    p.FortuneAdminCtrl,
		FortuneAppCtrl:      p.FortuneAppCtrl,
		LimitAdminCtrl:      p.LimitAdminCtrl,
		LimitAppCtrl:        p.LimitAppCtrl,
	}
}

// RegisterRoutes 挂载 starfuyu 路由。入参是 /api group（见 server/http.go），
// 故 admin 用 /admin/starfuyu、app 用 /app/starfuyu（member 基准双前缀）。
// config 开关：modules.starfuyu: false 时跳过注册。
func (m *starfuyuModule) RegisterRoutes(r *gin.RouterGroup) {
	if m.cfg != nil {
		if on, ok := m.cfg.Modules["starfuyu"]; ok && !on {
			return
		}
	}
	// Profile 的 CRUD 路由由 codegen 生成的 module_register_profile.go 声明，这里接线。
	RegisterProfileDBRoutes(r, m.ProfileAdminCtrl)
	// Bazi 的 CRUD 路由由 codegen 生成的 module_register_bazi.go 声明，这里接线。
	RegisterBaziDBRoutes(r, m.BaziAdminCtrl)
	// Divination 的 CRUD 路由由 codegen 生成的 module_register_divination.go 声明，这里接线。
	RegisterDivinationDBRoutes(r, m.DivinationAdminCtrl)
	// Fortune 的 CRUD 路由由 codegen 生成的 module_register_fortune.go 声明，这里接线。
	RegisterFortuneDBRoutes(r, m.FortuneAdminCtrl)
	// Limit 的 CRUD 路由由 codegen 生成的 module_register_limit.go 声明，这里接线。
	RegisterLimitDBRoutes(r, m.LimitAdminCtrl)

	appGroup := r.Group("/app/starfuyu")
	appGroup.GET("/profiles/ping", m.ProfileAppCtrl.Ping)
	appGroup.GET("/bazis/ping", m.BaziAppCtrl.Ping)
	appGroup.GET("/divinations/ping", m.DivinationAppCtrl.Ping)
	appGroup.GET("/fortunes/ping", m.FortuneAppCtrl.Ping)
	appGroup.GET("/limits/ping", m.LimitAppCtrl.Ping)
	// 跨模块接口映射（如 token.Issuer）归入口 main.go，模块内不重复 Provide——
	// 需要时在此上方参照 modules/member/module.go 补 fx.Provide 映射。
}
