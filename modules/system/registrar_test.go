package system

import (
	"strings"
	"testing"

	"go.uber.org/fx"
	"google.golang.org/grpc"

	"alexGo-cloud/modules/system/grpcserver"
	"alexGo-cloud/pkg/token"
)

// registrarParams 与 StartGRPCServer 消费 group:"grpc_registrars" 的形态一致
//（dig 值组禁用 optional——空组即空切片）。
type registrarParams struct {
	fx.In
	Registrars []grpcserver.Registrar `group:"grpc_registrars"`
}

// TestTokenServiceRegistrar_InGroup 正向干跑：tokenServiceRegistrar 必须把
// *TokenServiceImpl 放进 group，且 Register 后 gRPC server 上出现 TokenService 服务。
//（fx.New 内执行 Invoke——构造器真实运行，仅不启生命周期。）
func TestTokenServiceRegistrar_InGroup(t *testing.T) {
	var regs []grpcserver.Registrar
	app := fx.New(
		fx.Provide(
			tokenServiceRegistrar,
			// 构造期不触库（NewService 仅保存指针），无需真实 DB。
			func() *token.Service { return token.NewService(nil, nil) },
		),
		fx.Invoke(func(p registrarParams) { regs = p.Registrars }),
		fx.NopLogger,
	)
	if err := app.Err(); err != nil {
		t.Fatalf("fx 装配失败: %v", err)
	}
	if len(regs) != 1 {
		t.Fatalf("registrars = %d, want 1", len(regs))
	}
	if _, ok := regs[0].(*grpcserver.TokenServiceImpl); !ok {
		t.Fatalf("registrar type = %T, want *grpcserver.TokenServiceImpl", regs[0])
	}
	srv := grpc.NewServer()
	regs[0].Register(srv)
	if _, ok := srv.GetServiceInfo()["alexgo.system.rpc.TokenService"]; !ok {
		t.Error("TokenService 未注册到 gRPC server")
	}
	srv.Stop()
}

// TestTokenServiceRegistrar_RequiresTokenService 反向干跑：不提供 *token.Service 时
// group 成员构造不可满足——证明 TokenServiceImpl 真实入组并被消费（而非挂空组）。
func TestTokenServiceRegistrar_RequiresTokenService(t *testing.T) {
	err := fx.ValidateApp(
		fx.Provide(tokenServiceRegistrar),
		fx.Invoke(func(p registrarParams) { _ = p }),
		fx.NopLogger,
	)
	if err == nil {
		t.Fatal("缺 *token.Service 的图必须不可满足")
	}
	if !strings.Contains(err.Error(), "*token.Service") {
		t.Fatalf("error = %v, want missing *token.Service", err)
	}
}
