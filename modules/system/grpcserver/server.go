package grpcserver

import (
	"context"
	"net"

	"go.uber.org/fx"
	"google.golang.org/grpc"

	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/logger"
)

type Registrar interface {
	Register(server *grpc.Server)
}

type GRPCServerParams struct {
	fx.In

	LC  fx.Lifecycle
	Cfg *config.Config
	// Registrars 聚合 group:"grpc_registrars"。注意：dig 值组不允许 optional 标记
	//（`optional:"true"` 会在构图期硬报 "value groups cannot be optional"）——
	// 空组自然解析为空切片，语义与 optional 等价，无需也不能加 optional。
	Registrars []Registrar `group:"grpc_registrars"`
}

func StartGRPCServer(p GRPCServerParams) error {
	if p.Cfg.Deployment.Mode != "micro" {
		return nil // mono 模式不起 gRPC（本地直调 token.Service，无需监听）
	}
	addr := p.Cfg.Server.GRPCAddr
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	s := grpc.NewServer()
	for _, r := range p.Registrars {
		r.Register(s)
	}

	p.LC.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if logger.Log != nil {
				logger.Log.Info("gRPC server listening on " + addr)
			}
			go func() {
				_ = s.Serve(lis)
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			s.GracefulStop()
			return nil
		},
	})

	return nil
}
