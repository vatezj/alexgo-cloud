package main

import (
	"go.uber.org/fx"
	"gorm.io/gorm"

	"alexGo-cloud/modules/system"
	"alexGo-cloud/modules/system/grpcserver"
	"alexGo-cloud/pkg/client"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/database"
	"alexGo-cloud/pkg/logger"
	"alexGo-cloud/pkg/tenant/gormplugin"
	"alexGo-cloud/pkg/token"
	"alexGo-cloud/pkg/trace"
)

// options 装配 micro gRPC 入口的 fx 清单；干跑测试（grpc_main_test）复用同一清单，
// 防止测试与入口漂移。
func options() []fx.Option {
	return []fx.Option{
		fx.Provide(
			config.LoadGlobalConfig,
			database.NewDB,
			client.NewGRPCConn,
			// TokenServiceImpl（group:"grpc_registrars"）与模块内清扫 ticker 都消费
			// *token.Service——system-server 在本进程本地签发（micro 模式唯一签发方）。
			token.NewService,
		),
		// 租户字段隔离插件（T6 移交）：与 HTTP 入口同款 ExemptTables——本进程的
		// seeder/仓储写读同样要租户隔离。必须 fx.Invoke（再 Provide 一个 *gorm.DB
		// 会与 database.NewDB 冲突），放 Provide 后、StartGRPCServer Invoke 前。
		fx.Invoke(func(db *gorm.DB) error {
			return gormplugin.Register(db, gormplugin.Options{
				ExemptTables: []string{"tenants", "casbin_rule"},
			})
		}),
		fx.Invoke(trace.InitTracer),
		system.FxModule,
		fx.Invoke(grpcserver.StartGRPCServer),
	}
}

func main() {
	logger.Init()
	if logger.Log != nil {
		defer func() { _ = logger.Log.Sync() }()
	}

	fx.New(options()...).Run()
}
