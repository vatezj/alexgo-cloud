package main

import (
	"go.uber.org/fx"

	"alexGo-cloud/modules/system"
	"alexGo-cloud/modules/system/grpcserver"
	"alexGo-cloud/pkg/client"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/database"
	"alexGo-cloud/pkg/logger"
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
