package main

import (
	"strings"
	"testing"

	"go.uber.org/fx"

	"alexGo-cloud/modules/system"
	"alexGo-cloud/modules/system/grpcserver"
	"alexGo-cloud/pkg/client"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/database"
	"alexGo-cloud/pkg/trace"
)

// TestGRPCMain_GraphValidates 正向干跑：入口装配清单依赖图完整
//（dig dry-run：不执行构造器/Invoke，不需要配置文件与 DB）。
func TestGRPCMain_GraphValidates(t *testing.T) {
	if err := fx.ValidateApp(append(options(), fx.NopLogger)...); err != nil {
		t.Fatalf("fx.ValidateApp = %v", err)
	}
}

// TestGRPCMain_RequiresTokenService 反向干跑：故意剔除 token.NewService →
// *token.Service 缺失必须报错（registrar group 与清扫 ticker 的消费是真实的）。
func TestGRPCMain_RequiresTokenService(t *testing.T) {
	err := fx.ValidateApp(
		fx.Provide(config.LoadGlobalConfig, database.NewDB, client.NewGRPCConn), // 剔除 token.NewService
		fx.Invoke(trace.InitTracer),
		system.FxModule,
		fx.Invoke(grpcserver.StartGRPCServer),
		fx.NopLogger,
	)
	if err == nil {
		t.Fatal("缺 token.NewService 的入口图必须不可满足")
	}
	if !strings.Contains(err.Error(), "*token.Service") {
		t.Fatalf("error = %v, want missing *token.Service", err)
	}
}
