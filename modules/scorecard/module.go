package scorecard

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"alexGo-cloud/alexgo-server/server"
	"alexGo-cloud/modules/scorecard/controller/app"
	"alexGo-cloud/modules/scorecard/repository"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/migrate"
)

// FxModule scorecard 模块的 fx 装配。
//
// 脚手架期的实体 service/admin 控制器已随 Task 5 repository 层重写删除
// （旧 repo 接口不再提供 CRUD 方法，残留 service 编译不过——plan Task 12
// 预定的同一清理因接口变更提前至此）；Task 6-13 将按业务接口重建
// RoomService/RecordService/FriendService 与合一版 app/admin 控制器。
var FxModule = fx.Module("scorecard",
	fx.Provide(
		repository.NewRoomRepository,
		repository.NewRoomUserRepository,
		repository.NewRoomLogRepository,
		repository.NewRecordRepository,
		repository.NewFriendRepository,
		repository.NewFriendBlackRepository,
		app.NewAppRoomController,
		app.NewAppRoomUserController,
		app.NewAppRoomLogController,
		app.NewAppRecordController,
		app.NewAppFriendController,
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

	Cfg                *config.Config
	RoomAppCtrl        *app.AppRoomController
	RoomUserAppCtrl    *app.AppRoomUserController
	RoomLogAppCtrl     *app.AppRoomLogController
	RecordAppCtrl      *app.AppRecordController
	FriendAppCtrl      *app.AppFriendController
	FriendBlackAppCtrl *app.AppFriendBlackController
}

type scorecardModule struct {
	cfg                *config.Config
	RoomAppCtrl        *app.AppRoomController
	RoomUserAppCtrl    *app.AppRoomUserController
	RoomLogAppCtrl     *app.AppRoomLogController
	RecordAppCtrl      *app.AppRecordController
	FriendAppCtrl      *app.AppFriendController
	FriendBlackAppCtrl *app.AppFriendBlackController
}

// NewModule 聚合控制器，产出 server.Module（挂进 group:"modules"）。
func NewModule(p ModuleParams) server.Module {
	return &scorecardModule{
		cfg:                p.Cfg,
		RoomAppCtrl:        p.RoomAppCtrl,
		RoomUserAppCtrl:    p.RoomUserAppCtrl,
		RoomLogAppCtrl:     p.RoomLogAppCtrl,
		RecordAppCtrl:      p.RecordAppCtrl,
		FriendAppCtrl:      p.FriendAppCtrl,
		FriendBlackAppCtrl: p.FriendBlackAppCtrl,
	}
}

// RegisterRoutes 挂载 scorecard 路由。入参是 /api group（见 server/http.go），
// admin 路由待 Task 13 合一版控制器落地后重挂；app 端 18 业务接口 Task 12 落地。
func (m *scorecardModule) RegisterRoutes(r *gin.RouterGroup) {
	if m.cfg != nil {
		if on, ok := m.cfg.Modules["scorecard"]; ok && !on {
			return
		}
	}

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
