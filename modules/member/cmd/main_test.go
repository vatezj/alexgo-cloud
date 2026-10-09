package main

import (
	"context"
	"strings"
	"testing"

	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/token"
)

// TestMember_GraphValidates 正向干跑：member-server 装配图完整
// （dig dry-run：不执行构造器/Invoke，不需要配置文件与 DB）。
func TestMember_GraphValidates(t *testing.T) {
	cfg := &config.Config{}
	if err := fx.ValidateApp(append(options(cfg, false), fx.NopLogger)...); err != nil {
		t.Fatalf("member 入口图干跑失败: %v", err)
	}
}

// TestMember_MigrateOnly_GraphValidates 正向干跑：--migrate-only（不起 HTTP）图完整。
func TestMember_MigrateOnly_GraphValidates(t *testing.T) {
	cfg := &config.Config{}
	if err := fx.ValidateApp(append(options(cfg, true), fx.NopLogger)...); err != nil {
		t.Fatalf("member migrate-only 图干跑失败: %v", err)
	}
}

// TestMember_RequiresAccountLimitChecker 反向干跑：剔除 tenant.NewAccountLimitChecker
// 后 NewMemberService（三参）消费 tenant.AccountLimitChecker 必须报缺——
// 证明额度检查器是入口真实依赖而非挂空（勿改回两参/勿丢 Provide）。
func TestMember_RequiresAccountLimitChecker(t *testing.T) {
	err := fx.ValidateApp(append(baseOptions(&config.Config{}, false), fx.NopLogger)...)
	if err == nil {
		t.Fatal("剔除额度检查器的图必须不可满足")
	}
	if !strings.Contains(err.Error(), "tenant.AccountLimitChecker") {
		t.Fatalf("error = %v, want missing tenant.AccountLimitChecker", err)
	}
}

// TestTokenIssuerProvider_NilConnFailsFast typed-nil 裁决（T11）：conn 为 nil 时
// 必须先判具体指针再包装——返回 nilIssuer（fail-fast），而非包着 typed-nil 的
// gRPC 客户端（接口判空恒 false，调用即 panic）。
func TestTokenIssuerProvider_NilConnFailsFast(t *testing.T) {
	got := tokenIssuerProvider(nil)
	if _, ok := got.(nilIssuer); !ok {
		t.Fatalf("nil conn 应返回 nilIssuer, got %T", got)
	}
	// 各方法必须返回错误（登录接口据此映射 503），不得 panic。
	if _, err := got.Issue(context.Background(), token.IssueParams{}); err == nil {
		t.Fatal("nilIssuer.Issue 必须返回 error")
	}
	if _, err := got.Refresh(context.Background(), ""); err == nil {
		t.Fatal("nilIssuer.Refresh 必须返回 error")
	}
	if err := got.Revoke(context.Background(), ""); err == nil {
		t.Fatal("nilIssuer.Revoke 必须返回 error")
	}
	if err := got.RevokeAll(context.Background(), token.UserTypeMember, 1); err == nil {
		t.Fatal("nilIssuer.RevokeAll 必须返回 error")
	}
}

// TestTokenIssuerProvider_WrapsRealConn 非 nil conn 走 gRPC 客户端包装（非 nilIssuer）。
func TestTokenIssuerProvider_WrapsRealConn(t *testing.T) {
	// grpc.NewClient 懒连接：仅构造，不发起 I/O。
	conn, err := grpc.NewClient("127.0.0.1:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	got := tokenIssuerProvider(conn)
	if _, ok := got.(nilIssuer); ok {
		t.Fatal("非 nil conn 不应返回 nilIssuer")
	}
}
