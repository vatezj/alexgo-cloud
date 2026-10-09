package client

import (
	"alexGo-cloud/pkg/config"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type GRPCConnParams struct {
	fx.In

	Cfg *config.Config
}

func NewGRPCConn(p GRPCConnParams) (*grpc.ClientConn, error) {
	// 拨号门控看“形态”而非单一旧开关：deployment.mode=micro（双服务形态，
	// member-server 委托 system-server 签发）与 microservice.enabled（consul 演进旧开关）
	// 任一开启即拨号；两者皆关（mono 默认）返回 nil conn → 调用方 fail-fast 兜底。
	// 地址解析统一走 GetServiceAddress（显式 system_grpc_addr 优先）。
	if p.Cfg == nil || (!p.Cfg.Microservice.Enabled && p.Cfg.Deployment.Mode != "micro") {
		return nil, nil
	}
	addr, err := GetServiceAddress(p.Cfg, "system")
	if err != nil {
		return nil, err
	}
	// NewClient 取代已弃用的 Dial（golangci-lint SA1019）：同样懒连接，不做 I/O。
	return grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
}
