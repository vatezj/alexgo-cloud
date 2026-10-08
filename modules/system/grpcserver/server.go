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

	LC         fx.Lifecycle
	Cfg        *config.Config
	Registrars []Registrar `group:"grpc_registrars" optional:"true"`
}

func StartGRPCServer(p GRPCServerParams) error {
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
