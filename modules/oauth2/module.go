package oauth2

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"alexGo-cloud/alexgo-server/server"
	"alexGo-cloud/modules/oauth2/controller"
	"alexGo-cloud/modules/oauth2/repository"
	oauth2svc "alexGo-cloud/modules/oauth2/service"
	"alexGo-cloud/pkg/migrate"
)

// FxModule oauth2 模块装配（D7：硬编码进两个 main.go）。照抄 modules/infra/module.go：
// 迁移源注解进 group:"migration_sources"，NewModule 注解进 group:"modules"（均 ResultTags）。
var FxModule = fx.Module("oauth2",
	fx.Provide(
		fx.Annotate(
			NewMigrationSource,
			fx.As(new(migrate.Source)),
			fx.ResultTags(`group:"migration_sources"`),
		),
		oauth2svc.NewCaptchaService,
		controller.NewProtocolController,
		repository.NewClientRepository,
		oauth2svc.NewClientService,
		fx.Annotate(
			NewModule,
			fx.As(new(server.Module)),
			fx.ResultTags(`group:"modules"`),
		),
	),
)

type oauth2Module struct {
	proto *controller.ProtocolController
}

// NewModule 聚合控制器，产出 server.Module（挂进 group:"modules"）。
func NewModule(proto *controller.ProtocolController) server.Module {
	return &oauth2Module{proto: proto}
}

// RegisterRoutes 入参是 /api group（server/http.go 统一挂载，infra 同款），
// 故路径写相对前缀 /oauth2/*；内部不再 r.Group("/api")。
func (m *oauth2Module) RegisterRoutes(r *gin.RouterGroup) {
	m.proto.Register(r)
}
