package client

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"alexGo-cloud/modules/system/api/rpc"
	"alexGo-cloud/pkg/token"
)

// fake 的 token 服务端实现（不走 DB：直接回定值，验证 gRPC 通路与参数映射）。
type stubTokenServer struct {
	rpc.UnimplementedTokenServiceServer
	gotUserID int64
}

func (s *stubTokenServer) IssueToken(_ context.Context, req *rpc.IssueTokenRequest) (*rpc.IssueTokenResponse, error) {
	s.gotUserID = req.GetUserId()
	return &rpc.IssueTokenResponse{AccessToken: "ga", RefreshToken: "gr", ExpiresIn: 60}, nil
}

func dialBuf(t *testing.T, srv rpc.TokenServiceServer) *grpc.ClientConn {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	rpc.RegisterTokenServiceServer(s, srv)
	go func() { _ = s.Serve(lis) }()
	t.Cleanup(s.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestTokenIssuer_OverGRPC(t *testing.T) {
	stub := &stubTokenServer{}
	conn := dialBuf(t, stub)
	iss := NewTokenIssuer(conn)

	issued, err := iss.Issue(context.Background(), token.IssueParams{
		UserID: 42, UserType: token.UserTypeMember, TenantID: 1, ClientID: "alexgo-app",
	})
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if issued.AccessToken != "ga" || issued.ExpiresIn != 60 {
		t.Errorf("issued = %+v", issued)
	}
	if stub.gotUserID != 42 {
		t.Errorf("user_id mapped to %d, want 42", stub.gotUserID)
	}
}

// gRPC 不可达 → Issue 报错（登录层将映射 503，绝不降级）。
func TestTokenIssuer_Unreachable(t *testing.T) {
	iss := NewTokenIssuer(nil)
	if _, err := iss.Issue(context.Background(), token.IssueParams{UserID: 1}); err == nil {
		t.Error("nil conn must error")
	}
}

// ---- I6：Refresh/Revoke 的 gRPC 穿透（对称于 Issue 用例） ----

// refreshStub 记录 RefreshToken 请求映射并回传响应。
type refreshStub struct {
	rpc.UnimplementedTokenServiceServer
	gotRefresh string
}

func (s *refreshStub) RefreshToken(_ context.Context, req *rpc.RefreshTokenRequest) (*rpc.IssueTokenResponse, error) {
	s.gotRefresh = req.GetRefreshToken()
	return &rpc.IssueTokenResponse{AccessToken: "ra", RefreshToken: "rr", ExpiresIn: 120}, nil
}

func TestTokenIssuer_RefreshOverGRPC(t *testing.T) {
	stub := &refreshStub{}
	iss := NewTokenIssuer(dialBuf(t, stub))

	out, err := iss.Refresh(context.Background(), "old-refresh")
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if stub.gotRefresh != "old-refresh" {
		t.Errorf("refresh_token mapped to %q, want old-refresh", stub.gotRefresh)
	}
	if out.AccessToken != "ra" || out.RefreshToken != "rr" || out.ExpiresIn != 120 {
		t.Errorf("issued = %+v, want rpc response mapped back", out)
	}
}

// revokeStub 记录 RevokeToken 请求映射。
type revokeStub struct {
	rpc.UnimplementedTokenServiceServer
	gotAccess string
}

func (s *revokeStub) RevokeToken(_ context.Context, req *rpc.RevokeTokenRequest) (*rpc.RevokeTokenResponse, error) {
	s.gotAccess = req.GetAccessToken()
	return &rpc.RevokeTokenResponse{Ok: true}, nil
}

func TestTokenIssuer_RevokeOverGRPC(t *testing.T) {
	stub := &revokeStub{}
	iss := NewTokenIssuer(dialBuf(t, stub))

	if err := iss.Revoke(context.Background(), "the-access"); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}
	if stub.gotAccess != "the-access" {
		t.Errorf("access_token mapped to %q, want the-access", stub.gotAccess)
	}
}

// RevokeAll 显式不支持（一期声明的行为钉住）。
func TestTokenIssuer_RevokeAllUnsupported(t *testing.T) {
	iss := NewTokenIssuer(dialBuf(t, &revokeStub{}))
	if err := iss.RevokeAll(context.Background(), token.UserTypeMember, 1); err == nil {
		t.Error("RevokeAll over grpc must be rejected")
	}
}
