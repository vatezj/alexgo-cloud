package grpcserver

import (
	"context"

	"google.golang.org/grpc"

	"alexGo-cloud/modules/system/api/rpc"
	"alexGo-cloud/pkg/token"
)

// TokenServiceImpl 把 pkg/token 暴露为 gRPC（micro 模式下 member-server 的委托入口）。
type TokenServiceImpl struct {
	rpc.UnimplementedTokenServiceServer
	svc *token.Service
}

func NewTokenServiceImpl(svc *token.Service) *TokenServiceImpl {
	return &TokenServiceImpl{svc: svc}
}

// Register 供 grpcserver.StartGRPCServer 的 group:"grpc_registrars" 聚合调用。
func (s *TokenServiceImpl) Register(server *grpc.Server) {
	rpc.RegisterTokenServiceServer(server, s)
}

func (s *TokenServiceImpl) IssueToken(ctx context.Context, req *rpc.IssueTokenRequest) (*rpc.IssueTokenResponse, error) {
	issued, err := s.svc.Issue(ctx, token.IssueParams{
		UserID:   uint64(req.GetUserId()),
		UserType: token.UserType(req.GetUserType()),
		TenantID: uint64(req.GetTenantId()),
		ClientID: req.GetClientId(),
	})
	if err != nil {
		return nil, err
	}
	return &rpc.IssueTokenResponse{
		AccessToken: issued.AccessToken, RefreshToken: issued.RefreshToken, ExpiresIn: issued.ExpiresIn,
	}, nil
}

func (s *TokenServiceImpl) RefreshToken(ctx context.Context, req *rpc.RefreshTokenRequest) (*rpc.IssueTokenResponse, error) {
	issued, err := s.svc.Refresh(ctx, req.GetRefreshToken())
	if err != nil {
		return nil, err
	}
	return &rpc.IssueTokenResponse{
		AccessToken: issued.AccessToken, RefreshToken: issued.RefreshToken, ExpiresIn: issued.ExpiresIn,
	}, nil
}

func (s *TokenServiceImpl) RevokeToken(ctx context.Context, req *rpc.RevokeTokenRequest) (*rpc.RevokeTokenResponse, error) {
	if err := s.svc.Revoke(ctx, req.GetAccessToken()); err != nil {
		return nil, err
	}
	return &rpc.RevokeTokenResponse{Ok: true}, nil
}
