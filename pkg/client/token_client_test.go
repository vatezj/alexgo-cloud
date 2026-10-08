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

func dialBuf(t *testing.T, srv *stubTokenServer) *grpc.ClientConn {
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
