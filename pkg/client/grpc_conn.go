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
	if p.Cfg == nil || !p.Cfg.Microservice.Enabled {
		return nil, nil
	}
	addr, err := GetServiceAddress(p.Cfg, "system")
	if err != nil {
		return nil, err
	}
	// NewClient 取代已弃用的 Dial（golangci-lint SA1019）：同样懒连接，不做 I/O。
	return grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
}
