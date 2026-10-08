package main

import (
	"go.uber.org/fx"

	"alexGo-cloud/modules/system"
	"alexGo-cloud/modules/system/grpcserver"
	"alexGo-cloud/pkg/client"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/database"
	"alexGo-cloud/pkg/logger"
	"alexGo-cloud/pkg/trace"
)

func main() {
	logger.Init()
	if logger.Log != nil {
		defer func() { _ = logger.Log.Sync() }()
	}

	fx.New(
		fx.Provide(
			config.LoadGlobalConfig,
			database.NewDB,
			client.NewGRPCConn,
		),
		fx.Invoke(trace.InitTracer),
		system.FxModule,
		fx.Invoke(grpcserver.StartGRPCServer),
	).Run()
}
