package scorecard

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"alexGo-cloud/alexgo-server/server"
	"alexGo-cloud/modules/scorecard/controller/admin"
	"alexGo-cloud/modules/scorecard/controller/app"
	"alexGo-cloud/modules/scorecard/repository"
	"alexGo-cloud/modules/scorecard/service"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/migrate"
)

// FxModule scorecard 模块的 fx 装配（newmodule 生成，业务逻辑请手工扩展）。
var FxModule = fx.Module("scorecard",
	fx.Provide(
		repository.NewRoomRepository,
		service.NewRoomService,
		admin.NewAdminRoomController,
		app.NewAppRoomController,
		repository.NewRoomUserRepository,
		service.NewRoomUserService,
		admin.NewAdminRoomUserController,
		app.NewAppRoomUserController,
		repository.NewRoomLogRepository,
		service.NewRoomLogService,
		admin.NewAdminRoomLogController,
		app.NewAppRoomLogController,
		repository.NewRecordRepository,
		service.NewRecordService,
		admin.NewAdminRecordController,
		app.NewAppRecordController,
		repository.NewFriendRepository,
		service.NewFriendService,
		admin.NewAdminFriendController,
		app.NewAppFriendController,
		repository.NewFriendBlackRepository,
		service.NewFriendBlackService,
		admin.NewAdminFriendBlackController,
		app.NewAppFriendBlackController,
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

	Cfg                  *config.Config
	RoomAdminCtrl        *admin.AdminRoomController
	RoomAppCtrl          *app.AppRoomController
	RoomUserAdminCtrl    *admin.AdminRoomUserController
	RoomUserAppCtrl      *app.AppRoomUserController
	RoomLogAdminCtrl     *admin.AdminRoomLogController
	RoomLogAppCtrl       *app.AppRoomLogController
	RecordAdminCtrl      *admin.AdminRecordController
	RecordAppCtrl        *app.AppRecordController
	FriendAdminCtrl      *admin.AdminFriendController
	FriendAppCtrl        *app.AppFriendController
	FriendBlackAdminCtrl *admin.AdminFriendBlackController
	FriendBlackAppCtrl   *app.AppFriendBlackController
}

type scorecardModule struct {
	cfg                  *config.Config
	RoomAdminCtrl        *admin.AdminRoomController
	RoomAppCtrl          *app.AppRoomController
	RoomUserAdminCtrl    *admin.AdminRoomUserController
	RoomUserAppCtrl      *app.AppRoomUserController
	RoomLogAdminCtrl     *admin.AdminRoomLogController
	RoomLogAppCtrl       *app.AppRoomLogController
	RecordAdminCtrl      *admin.AdminRecordController
	RecordAppCtrl        *app.AppRecordController
	FriendAdminCtrl      *admin.AdminFriendController
	FriendAppCtrl        *app.AppFriendController
	FriendBlackAdminCtrl *admin.AdminFriendBlackController
	FriendBlackAppCtrl   *app.AppFriendBlackController
}

// NewModule 聚合控制器，产出 server.Module（挂进 group:"modules"）。
func NewModule(p ModuleParams) server.Module {
	return &scorecardModule{
		cfg:                  p.Cfg,
		RoomAdminCtrl:        p.RoomAdminCtrl,
		RoomAppCtrl:          p.RoomAppCtrl,
		RoomUserAdminCtrl:    p.RoomUserAdminCtrl,
		RoomUserAppCtrl:      p.RoomUserAppCtrl,
		RoomLogAdminCtrl:     p.RoomLogAdminCtrl,
		RoomLogAppCtrl:       p.RoomLogAppCtrl,
		RecordAdminCtrl:      p.RecordAdminCtrl,
		RecordAppCtrl:        p.RecordAppCtrl,
		FriendAdminCtrl:      p.FriendAdminCtrl,
		FriendAppCtrl:        p.FriendAppCtrl,
		FriendBlackAdminCtrl: p.FriendBlackAdminCtrl,
		FriendBlackAppCtrl:   p.FriendBlackAppCtrl,
	}
}

// RegisterRoutes 挂载 scorecard 路由。入参是 /api group（见 server/http.go），
// 故 admin 用 /admin/scorecard、app 用 /app/scorecard（member 基准双前缀）。
// config 开关：modules.scorecard: false 时跳过注册。
func (m *scorecardModule) RegisterRoutes(r *gin.RouterGroup) {
	if m.cfg != nil {
		if on, ok := m.cfg.Modules["scorecard"]; ok && !on {
			return
		}
	}
	// Room 的 CRUD 路由由 codegen 生成的 module_register_room.go 声明，这里接线。
	RegisterRoomDBRoutes(r, m.RoomAdminCtrl)
	// RoomUser 的 CRUD 路由由 codegen 生成的 module_register_room_user.go 声明，这里接线。
	RegisterRoomUserDBRoutes(r, m.RoomUserAdminCtrl)
	// RoomLog 的 CRUD 路由由 codegen 生成的 module_register_room_log.go 声明，这里接线。
	RegisterRoomLogDBRoutes(r, m.RoomLogAdminCtrl)
	// Record 的 CRUD 路由由 codegen 生成的 module_register_record.go 声明，这里接线。
	RegisterRecordDBRoutes(r, m.RecordAdminCtrl)
	// Friend 的 CRUD 路由由 codegen 生成的 module_register_friend.go 声明，这里接线。
	RegisterFriendDBRoutes(r, m.FriendAdminCtrl)
	// FriendBlack 的 CRUD 路由由 codegen 生成的 module_register_friend_black.go 声明，这里接线。
	RegisterFriendBlackDBRoutes(r, m.FriendBlackAdminCtrl)

	appGroup := r.Group("/app/scorecard")
	appGroup.GET("/rooms/ping", m.RoomAppCtrl.Ping)
	appGroup.GET("/room_users/ping", m.RoomUserAppCtrl.Ping)
	appGroup.GET("/room_logs/ping", m.RoomLogAppCtrl.Ping)
	appGroup.GET("/records/ping", m.RecordAppCtrl.Ping)
	appGroup.GET("/friends/ping", m.FriendAppCtrl.Ping)
	appGroup.GET("/friend_blacks/ping", m.FriendBlackAppCtrl.Ping)
	// 跨模块接口映射（如 token.Issuer）归入口 main.go，模块内不重复 Provide——
	// 需要时在此上方参照 modules/member/module.go 补 fx.Provide 映射。
}
