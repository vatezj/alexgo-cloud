package member

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"alexGo-cloud/alexgo-server/server"
	"alexGo-cloud/modules/member/controller/admin"
	"alexGo-cloud/modules/member/controller/app"
	"alexGo-cloud/modules/member/repository"
	"alexGo-cloud/modules/member/service"
	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
	"alexGo-cloud/modules/system/oauth2/social"
	"alexGo-cloud/pkg/migrate"
)

// FxModule member 模块的 fx 装配。
// 注意：token.Issuer / token.Validator 的接口映射归入口（alexgo-server/cmd/main.go），
// 模块内不重复 Provide；NewMemberService 的 issuer 参数由入口映射满足。
var FxModule = fx.Module("member",
	fx.Provide(
		repository.NewMemberRepository,
		service.NewMemberService,
		app.NewAuthController,
		admin.NewUserController,
	),
	// member 凭据校验窄接口映射：统一登录（Task 8）按 oauth2svc.MemberAuthenticator 消费。
	fx.Provide(
		func(svc service.MemberService) oauth2svc.MemberAuthenticator { return svc },
	),
	// member 摘要窄只读接口映射：scorecard 等业务模块按 service.BriefProvider 消费。
	fx.Provide(
		func(svc service.MemberService) service.BriefProvider { return svc },
	),
	// 第三方登录会员窄接口映射（Task 6）：social.SocialService 按 MemberStore 消费。
	// 无 import 循环：social 只 import member/model；micro system 进程无本 Provide → optional 为 nil。
	fx.Provide(func(r repository.MemberRepository) social.MemberStore { return r }),
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

type memberModule struct {
	authCtrl *app.AuthController
	userCtrl *admin.UserController
}

// NewModule 聚合控制器，产出 server.Module（挂进 group:"modules"）。
func NewModule(authCtrl *app.AuthController, userCtrl *admin.UserController) server.Module {
	return &memberModule{authCtrl: authCtrl, userCtrl: userCtrl}
}

// RegisterRoutes 挂载 member 路由。入参是 /api group（见 server/http.go），
// 故这里用相对前缀 /app/member、/admin/member（与 system 模块同构）。
func (m *memberModule) RegisterRoutes(r *gin.RouterGroup) {
	appGroup := r.Group("/app/member")
	appGroup.POST("/auth/register", m.authCtrl.Register)
	appGroup.POST("/auth/login", m.authCtrl.Login)
	appGroup.POST("/auth/refresh", m.authCtrl.Refresh)
	appGroup.POST("/auth/logout", m.authCtrl.Logout)

	adminGroup := r.Group("/admin/member")
	adminGroup.GET("/users", m.userCtrl.List)
	adminGroup.GET("/users/export", m.userCtrl.ExportMemberUsers)
	adminGroup.PUT("/users/:id/status", m.userCtrl.UpdateStatus)
}
