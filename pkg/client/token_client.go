package client

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"alexGo-cloud/modules/system/api/rpc"
	"alexGo-cloud/pkg/token"
)

// NewTokenIssuer 连接 system-server 的 TokenService，返回可用作 member-server
// 签发入口的 token.Issuer。调用方（fx）负责管理连接生命周期（grpc_conn.go 现有模式）。
func NewTokenIssuer(conn grpc.ClientConnInterface) token.Issuer {
	return &tokenGRPCClient{conn: conn}
}

type tokenGRPCClient struct {
	conn grpc.ClientConnInterface
}

func (c *tokenGRPCClient) client() rpc.TokenServiceClient {
	return rpc.NewTokenServiceClient(c.conn)
}

func (c *tokenGRPCClient) Issue(ctx context.Context, p token.IssueParams) (*token.Issued, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("token: grpc conn is nil")
	}
	resp, err := c.client().IssueToken(ctx, &rpc.IssueTokenRequest{
		UserId: int64(p.UserID), UserType: int32(p.UserType),
		TenantId: int64(p.TenantID), ClientId: p.ClientID,
	})
	if err != nil {
		return nil, err // gRPC 故障 → 登录接口 503（controller 层映射），不降级
	}
	return &token.Issued{
		AccessToken: resp.GetAccessToken(), RefreshToken: resp.GetRefreshToken(), ExpiresIn: resp.GetExpiresIn(),
	}, nil
}

func (c *tokenGRPCClient) Refresh(ctx context.Context, refreshToken string) (*token.Issued, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("token: grpc conn is nil")
	}
	resp, err := c.client().RefreshToken(ctx, &rpc.RefreshTokenRequest{RefreshToken: refreshToken})
	if err != nil {
		return nil, err
	}
	return &token.Issued{
		AccessToken: resp.GetAccessToken(), RefreshToken: resp.GetRefreshToken(), ExpiresIn: resp.GetExpiresIn(),
	}, nil
}

func (c *tokenGRPCClient) Revoke(ctx context.Context, accessToken string) error {
	if c.conn == nil {
		return fmt.Errorf("token: grpc conn is nil")
	}
	_, err := c.client().RevokeToken(ctx, &rpc.RevokeTokenRequest{AccessToken: accessToken})
	return err
}

// RevokeAll 显式不支持：踢人接口仅 system-server 本地使用
//（member 会员管理"禁用+踢人"一期不做联动——Task 5 注释已声明）。
func (c *tokenGRPCClient) RevokeAll(_ context.Context, _ token.UserType, _ uint64) error {
	return fmt.Errorf("token: RevokeAll not supported over grpc（踢人接口仅 system-server 本地使用）")
}

// 编译期断言：确保客户端满足接口。
var _ token.Issuer = (*tokenGRPCClient)(nil)
