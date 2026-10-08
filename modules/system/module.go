package system

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"alexGo-cloud/alexgo-server/server"
	"alexGo-cloud/modules/system/controller/admin"
	"alexGo-cloud/modules/system/controller/app"
	"alexGo-cloud/modules/system/repository"
	"alexGo-cloud/modules/system/service"
	"alexGo-cloud/pkg/audit"
	"alexGo-cloud/pkg/migrate"
	"alexGo-cloud/pkg/tenant"
)

var FxModule = fx.Module("system",
	fx.Provide(
		repository.NewRepository,
		service.NewService,
		admin.NewAdminController,
		admin.NewUserController,
		repository.NewDeptRepository,
		repository.NewPostRepository,
		repository.NewDictRepository,
		repository.NewConfigRepository,
		repository.NewNoticeRepository,
		service.NewDeptService,
		service.NewPostService,
		service.NewDictService,
		service.NewConfigService,
		service.NewNoticeService,
		admin.NewBasicController,
		repository.NewAuditRepository,
		service.NewAuditService,
		func(svc service.AuditService) audit.OperateRecorder { return svc },
		repository.NewRoleRepository,
		repository.NewMenuRepository,
		repository.NewUserRoleRepository,
		repository.NewRoleMenuRepository,
		service.NewAuthService,
		service.NewRoleService,
		service.NewMenuService,
		service.NewPermissionService,
		admin.NewAuthController,
		admin.NewRoleController,
		admin.NewMenuController,
		admin.NewAuditController,
		app.NewAppController,
		// 租户管理 + 账号额度（Task 7）。
		repository.NewTenantRepository,
		service.NewTenantService,
		admin.NewTenantController,
		// 窄接口映射：member 模块经 pkg/tenant.AccountLimitChecker 消费，
		// 实现归 system（唯一 Provide 处，入口不重复）。
		func(svc service.TenantService) tenant.AccountLimitChecker { return svc },
		// 数据权限加载器（T10）：concrete → tenant.ScopeLoader 窄接口映射，
		// AuthDeps 经 HTTPServerParams(optional) 消费；member 端不提供该实现。
		service.NewDataScopeLoader,
		func(l *service.DataScopeLoader) tenant.ScopeLoader { return l },
	),
	fx.Invoke(service.StartSeeder),
	fx.Provide(
		fx.Annotate(
			NewSystemModule,
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

type systemModule struct {
	adminCtrl  *admin.AdminController
	userCtrl   *admin.UserController
	authCtrl   *admin.AuthController
	roleCtrl   *admin.RoleController
	menuCtrl   *admin.MenuController
	basicCtrl  *admin.BasicController
	auditCtrl  *admin.AuditController
	appCtrl    *app.AppController
	tenantCtrl *admin.TenantController
}

func NewSystemModule(
	adminCtrl *admin.AdminController,
	userCtrl *admin.UserController,
	authCtrl *admin.AuthController,
	roleCtrl *admin.RoleController,
	menuCtrl *admin.MenuController,
	basicCtrl *admin.BasicController,
	auditCtrl *admin.AuditController,
	appCtrl *app.AppController,
	tenantCtrl *admin.TenantController,
) server.Module {
	return &systemModule{
		adminCtrl:  adminCtrl,
		userCtrl:   userCtrl,
		authCtrl:   authCtrl,
		roleCtrl:   roleCtrl,
		menuCtrl:   menuCtrl,
		basicCtrl:  basicCtrl,
		auditCtrl:  auditCtrl,
		appCtrl:    appCtrl,
		tenantCtrl: tenantCtrl,
	}
}

func (m *systemModule) RegisterRoutes(r *gin.RouterGroup) {
	adminGroup := r.Group("/admin/system")
	adminGroup.GET("/users", m.adminCtrl.ListUsers)
	adminGroup.POST("/users", m.userCtrl.Create)
	adminGroup.PUT("/users/:id", m.userCtrl.Update)
	adminGroup.POST("/users/:id/reset-password", m.userCtrl.ResetPassword)
	adminGroup.POST("/users/:id/roles", m.userCtrl.SetRoles)
	adminGroup.GET("/auth/profile", m.authCtrl.Profile)
	adminGroup.POST("/auth/refresh", m.authCtrl.Refresh)
	adminGroup.POST("/auth/logout", m.authCtrl.Logout)
	adminGroup.GET("/roles", m.roleCtrl.List)
	adminGroup.POST("/roles", m.roleCtrl.Create)
	adminGroup.DELETE("/roles/:id", m.roleCtrl.Delete)
	adminGroup.POST("/roles/:id/menus", m.roleCtrl.AssignMenus)
	adminGroup.GET("/menus", m.menuCtrl.List)
	adminGroup.POST("/menus", m.menuCtrl.Create)
	adminGroup.DELETE("/menus/:id", m.menuCtrl.Delete)
	adminGroup.GET("/depts", m.basicCtrl.ListDepts)
	adminGroup.POST("/depts", m.basicCtrl.CreateDept)
	adminGroup.PUT("/depts", m.basicCtrl.UpdateDept)
	adminGroup.DELETE("/depts/:id", m.basicCtrl.DeleteDept)
	adminGroup.GET("/posts", m.basicCtrl.ListPosts)
	adminGroup.POST("/posts", m.basicCtrl.CreatePost)
	adminGroup.PUT("/posts", m.basicCtrl.UpdatePost)
	adminGroup.DELETE("/posts/:id", m.basicCtrl.DeletePost)
	adminGroup.GET("/dict/types", m.basicCtrl.ListDictTypes)
	adminGroup.POST("/dict/types", m.basicCtrl.CreateDictType)
	adminGroup.PUT("/dict/types", m.basicCtrl.UpdateDictType)
	adminGroup.DELETE("/dict/types/:id", m.basicCtrl.DeleteDictType)
	adminGroup.GET("/dict/datas", m.basicCtrl.ListDictDatas)
	adminGroup.POST("/dict/datas", m.basicCtrl.CreateDictData)
	adminGroup.PUT("/dict/datas", m.basicCtrl.UpdateDictData)
	adminGroup.DELETE("/dict/datas/:id", m.basicCtrl.DeleteDictData)
	adminGroup.GET("/configs", m.basicCtrl.ListConfigs)
	adminGroup.POST("/configs", m.basicCtrl.CreateConfig)
	adminGroup.PUT("/configs", m.basicCtrl.UpdateConfig)
	adminGroup.DELETE("/configs/:id", m.basicCtrl.DeleteConfig)
	adminGroup.GET("/notices", m.basicCtrl.ListNotices)
	adminGroup.POST("/notices", m.basicCtrl.CreateNotice)
	adminGroup.PUT("/notices", m.basicCtrl.UpdateNotice)
	adminGroup.DELETE("/notices/:id", m.basicCtrl.DeleteNotice)
	adminGroup.GET("/logs/login", m.auditCtrl.ListLogin)
	adminGroup.GET("/logs/operate", m.auditCtrl.ListOperate)
	adminGroup.GET("/tenants", m.tenantCtrl.List)
	adminGroup.POST("/tenants", m.tenantCtrl.Create)
	adminGroup.PUT("/tenants/:id", m.tenantCtrl.Update)
	adminGroup.DELETE("/tenants/:id", m.tenantCtrl.Delete)

	appGroup := r.Group("/app/system")
	appGroup.POST("/auth/login", m.appCtrl.Login)
	appGroup.POST("/auth/refresh", m.authCtrl.Refresh)
	appGroup.POST("/auth/logout", m.authCtrl.Logout)
}
