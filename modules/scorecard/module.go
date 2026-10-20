package scorecard

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"alexGo-cloud/alexgo-server/server"
	"alexGo-cloud/modules/scorecard/controller/app"
	"alexGo-cloud/modules/scorecard/repository"
	"alexGo-cloud/modules/scorecard/service"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/logger"
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
		// briefs（member.BriefProvider）标 optional：micro 模式（DEPLOYMENT_MODE=micro）
		// 本进程不装 member.FxModule，图里无该接口映射——RoomService 仍须可装配
		// （buildSnapshot 对 nil briefs 降级空昵称）。mono 模式由 member 映射注入。
		fx.Annotate(
			service.NewRoomService,
			fx.ParamTags("", "", "", "", "", `optional:"true"`, ""),
		),
		// RecordService 直产接口类型（同 RoomService 模式）：db + RecordRepository 均可解析，
		// Task 12/13 控制器按 service.RecordService 注入。
		service.NewRecordService,
		// FriendService 好友服务（Task 11）：briefs（member.BriefProvider）标 optional
		// 同 RoomService——micro 模式本进程不装 member.FxModule，图里无该接口映射；
		// 缺席时 Add 降级跳过对方存在校验、List 昵称降级留空。Task 12/13 控制器消费。
		fx.Annotate(
			service.NewFriendService,
			fx.ParamTags("", "", "", "", `optional:"true"`),
		),
		// Task 12 合一版 app 控制器（Bearer 自鉴权）。token.Validator 的接口映射归
		// 入口（alexgo-server/cmd/main.go ifaceOptions），模块内不重复 Provide。
		// admin 端合一控制器 Task 13 落地后补 Provide。
		app.NewAppController,
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
	// 超时关房 ticker（2h TTL 心跳不续期，spec §4.2）：照 modules/system/module.go
	// token SweepExpired 的 Fx 生命周期模式——OnStart 的 ctx 在启动完成即被 cancel，
	// goroutine 不能监听它（会秒退），改独立 runCtx 并在 OnStop cancel 保证退出。
	// 扫描用 Background：不受请求/生命周期 ctx 影响；错误只记 warn 不致命，下轮再扫。
	fx.Invoke(func(lc fx.Lifecycle, svc service.RoomService) {
		runCtx, cancel := context.WithCancel(context.Background())
		lc.Append(fx.Hook{
			OnStart: func(context.Context) error {
				go func() {
					ticker := time.NewTicker(time.Minute)
					defer ticker.Stop()
					for {
						select {
						case <-runCtx.Done():
							return
						case <-ticker.C:
							if _, err := svc.SweepExpired(context.Background()); err != nil && logger.Log != nil {
								logger.Log.Warn("scorecard sweep failed", zap.Error(err))
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
)

// ModuleParams 聚合依赖（fx.In）。
type ModuleParams struct {
	fx.In

	Cfg     *config.Config
	AppCtrl *app.AppController
}

type scorecardModule struct {
	cfg     *config.Config
	appCtrl *app.AppController
}

// NewModule 聚合控制器，产出 server.Module（挂进 group:"modules"）。
func NewModule(p ModuleParams) server.Module {
	return &scorecardModule{
		cfg:     p.Cfg,
		appCtrl: p.AppCtrl,
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

	app := r.Group("/app/scorecard")
	app.POST("/rooms", m.appCtrl.CreateRoom)
	app.POST("/rooms/join", m.appCtrl.JoinRoom)
	app.GET("/rooms/:id/snapshot", m.appCtrl.Snapshot)
	app.PUT("/rooms/:id/config", m.appCtrl.UpdateConfig)
	app.POST("/rooms/:id/expense", m.appCtrl.Expense)
	app.GET("/rooms/:id/logs", m.appCtrl.Logs)
	app.POST("/rooms/:id/settle", m.appCtrl.Settle)
	app.POST("/rooms/:id/leave", m.appCtrl.Leave)
	app.POST("/rooms/:id/transfer-owner", m.appCtrl.TransferOwner)
	app.GET("/records", m.appCtrl.MyRecords)
	app.GET("/records/:recordId", m.appCtrl.RecordDetail)
	app.DELETE("/records/:recordId", m.appCtrl.DeleteRecord)
	app.GET("/stat", m.appCtrl.Stat)
	app.GET("/friends", m.appCtrl.ListFriends)
	app.POST("/friends", m.appCtrl.AddFriend)
	app.DELETE("/friends/:userId", m.appCtrl.RemoveFriend)
	app.POST("/friends/:userId/black", m.appCtrl.SetFriendBlack)
	app.PUT("/friends/:userId/rank", m.appCtrl.SetFriendRank)
	// 跨模块接口映射（如 token.Issuer）归入口 main.go，模块内不重复 Provide——
	// 需要时在此上方参照 modules/member/module.go 补 fx.Provide 映射。
}
