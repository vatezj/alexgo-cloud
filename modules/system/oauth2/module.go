package oauth2

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"alexGo-cloud/alexgo-server/server"
	"alexGo-cloud/modules/system/oauth2/controller"
	"alexGo-cloud/modules/system/oauth2/controller/admin"
	"alexGo-cloud/modules/system/oauth2/repository"
	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
	"alexGo-cloud/modules/system/oauth2/social"
	"alexGo-cloud/pkg/migrate"
)

// FxModule oauth2 模块装配（D7：硬编码进两个 main.go）。
// 照抄 modules/system/infra/module.go：迁移源注解进 group:"migration_sources"，
// NewModule 注解进 group:"modules"（ResultTags 不是 ParamTags）。
var FxModule = fx.Module("oauth2",
	fx.Provide(
		fx.Annotate(
			NewMigrationSource,
			fx.As(new(migrate.Source)),
			fx.ResultTags(`group:"migration_sources"`),
		),
		oauth2svc.NewCaptchaService,
		repository.NewClientRepository,
		repository.NewAuthorizeCodeRepository,
		oauth2svc.NewClientService,
		oauth2svc.NewAuthorizeCodeService,
		oauth2svc.NewUnifiedLogin,
		oauth2svc.NewTokenService,
		// 第三方登录（微信一期，Task 6）：仓储/适配器注册表/登录门面。
		// SocialDeps.Members 由 member 模块映射（member/module.go）；
		// system 微服务进程无 member 模块 → optional 为 nil。
		social.NewSocialRepository,
		social.NewAdapterRegistry,
		social.NewSocialService,
		controller.NewProtocolController,
		admin.NewClientController,
		fx.Annotate(
			NewModule,
			fx.As(new(server.Module)),
			fx.ResultTags(`group:"modules"`),
		),
	),
)

type oauth2Module struct {
	proto   *controller.ProtocolController
	clients *admin.ClientController
}

// NewModule 聚合控制器，产出 server.Module（挂进 group:"modules"）。
func NewModule(proto *controller.ProtocolController, clients *admin.ClientController) server.Module {
	return &oauth2Module{proto: proto, clients: clients}
}

// RegisterRoutes 入参是 /api group（见 server/http.go，infra 同款），
// 故路径写相对前缀 /oauth2/*。
func (m *oauth2Module) RegisterRoutes(r *gin.RouterGroup) {
	m.proto.Register(r)
	g := r.Group("/admin/oauth2")
	g.GET("/clients", m.clients.List)
	g.POST("/clients", m.clients.Create)
	g.PUT("/clients/:id", m.clients.Update)
	g.DELETE("/clients/:id", m.clients.Delete)
}
