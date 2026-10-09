package system

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"alexGo-cloud/alexgo-server/server"
	"alexGo-cloud/modules/system/controller/admin"
	"alexGo-cloud/modules/system/controller/app"
	"alexGo-cloud/modules/system/controller/vben"
	"alexGo-cloud/modules/system/grpcserver"
	"alexGo-cloud/modules/system/repository"
	"alexGo-cloud/modules/system/service"
	"alexGo-cloud/pkg/audit"
	"alexGo-cloud/pkg/logger"
	"alexGo-cloud/pkg/migrate"
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/token"
)

// tokenServiceRegistrar 把 TokenServiceImpl 放进 group:"grpc_registrars"
// （由 grpcserver.StartGRPCServer 聚合注册；mono 模式该 group 无人消费、惰性不构造）。
// 提取为包级变量：正/反向干跑测试与 FxModule 共用同一对象，防装配漂移。
var tokenServiceRegistrar = fx.Annotate(
	grpcserver.NewTokenServiceImpl,
	fx.As(new(grpcserver.Registrar)),
	fx.ResultTags(`group:"grpc_registrars"`),
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
		vben.NewController,
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
		// T11：TokenService 的 gRPC Registrar（As 版本单条，同时满足组注册）。
		tokenServiceRegistrar,
	),
	fx.Invoke(service.StartSeeder),
	// 过期令牌清扫 ticker（T11，仿 outbox.StartRelay 的 Fx 生命周期模式）。
	// 关键：OnStart 的 ctx 在启动完成即被 cancel——goroutine 不能监听它（会秒退），
	// 改为独立 runCtx，并在 OnStop 时 cancel 保证退出（OnStart 只负责起 goroutine）。
	fx.Invoke(func(lc fx.Lifecycle, svc *token.Service) {
		runCtx, cancel := context.WithCancel(context.Background())
		lc.Append(fx.Hook{
			OnStart: func(context.Context) error {
				go func() {
					ticker := time.NewTicker(time.Hour)
					defer ticker.Stop()
					for {
						select {
						case <-runCtx.Done():
							return
						case <-ticker.C:
							// 扫描用 Background：不受请求/生命周期 ctx 影响。
							if err := svc.SweepExpired(context.Background()); err != nil && logger.Log != nil {
								logger.Log.Warn("token sweep failed", zap.Error(err))
							}
						}
					}
				}()
				return nil
			},
			OnStop: func(context.Context) error {
				cancel()
				return nil
			},
		})
	}),
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
	vbenCtrl   *vben.Controller
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
	vbenCtrl *vben.Controller,
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
		vbenCtrl:   vbenCtrl,
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

	// vben-admin 自读端点（/api/auth/login|logout|codes、/api/user/info、/api/menu/all）：
	// 挂在 /api 根组（r 已带 /api 前缀），与 admin/app 组无路径冲突。
	m.vbenCtrl.Register(r)
}
