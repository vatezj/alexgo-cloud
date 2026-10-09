package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"alexGo-cloud/pkg/client"
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

// TestIssuerWiredOverGRPCInMicroMode 双服务形态主路径（验收判据“member 经 gRPC
// 委托签发”）：deployment.mode=micro + 显式地址 → NewGRPCConn 必须拨号（非 nil），
// tokenIssuerProvider 包装为真实 gRPC Issuer——即便 microservice.enabled=false。
func TestIssuerWiredOverGRPCInMicroMode(t *testing.T) {
	cfg := &config.Config{SystemGRPCAddr: "127.0.0.1:50051"}
	cfg.Deployment.Mode = "micro"

	conn, err := client.NewGRPCConn(client.GRPCConnParams{Cfg: cfg})
	if err != nil {
		t.Fatalf("NewGRPCConn err = %v, want nil", err)
	}
	if conn == nil {
		t.Fatal("micro 模式必须拨号：conn 不能为 nil（否则恒 nilIssuer → 登录 503）")
	}
	t.Cleanup(func() { _ = conn.Close() })

	if _, ok := tokenIssuerProvider(conn).(nilIssuer); ok {
		t.Fatal("micro 模式 Issuer 必须是 gRPC 客户端而非 nilIssuer")
	}
}

// TestIssuerNilConnWhenBothFlagsOff mono 默认（两开关皆关、无地址）→ NewGRPCConn
// 返回 typed-nil conn → tokenIssuerProvider 判空命中 nilIssuer（生产同路径）。
func TestIssuerNilConnWhenBothFlagsOff(t *testing.T) {
	conn, err := client.NewGRPCConn(client.GRPCConnParams{Cfg: &config.Config{}})
	if err != nil {
		t.Fatalf("NewGRPCConn err = %v, want nil", err)
	}
	if conn != nil {
		_ = conn.Close()
		t.Fatal("mono 默认必须返回 nil conn")
	}
	if _, ok := tokenIssuerProvider(conn).(nilIssuer); !ok {
		t.Fatalf("typed-nil conn 必须得到 nilIssuer, got %T", tokenIssuerProvider(conn))
	}
}

// TestRunMemberRecipeEnv_Wiring Makefile run-member 开箱路径（review 判定的 503 链路）：
// 仅 recipe 环境（HTTP_ADDR/SYSTEM_GRPC_ADDR，DEPLOYMENT_MODE 置空等效 unset——
// 连 recipe 的 DEPLOYMENT_MODE 都不给，只靠入口强制）、loader 默认 deployment.mode=
// "mono" 的真实默认值下，走入口自身的 loadConfig()（加载+无条件强制 micro）后
// NewGRPCConn 必须拨号。测试不得手动置 Mode="micro"（只允许复用入口的强制逻辑）。
func TestRunMemberRecipeEnv_Wiring(t *testing.T) {
	// 隔离环境：applyEnvOverrides 忽略空值 → DEPLOYMENT_MODE="" 等效未设置；
	// HTTP_ADDR/SYSTEM_GRPC_ADDR 与 Makefile run-member recipe 逐字一致。
	t.Setenv("DEPLOYMENT_MODE", "")
	t.Setenv("HTTP_ADDR", ":8081")
	t.Setenv("SYSTEM_GRPC_ADDR", "127.0.0.1:50051")

	// config.yaml 是仓内相对路径：make 运行目录即仓库根，测试切到同目录
	//（LoadGlobalConfig 在包目录下会因相对路径读不到配置而报错）。
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir("../../.."); err != nil {
		t.Fatal(err)
	}

	// 缺陷前提锚点：入口强制之前，loader 默认 deployment.mode 恒为 "mono"
	//（viper SetDefault；stock config.yaml 亦无 deployment 段）——空值回退即死代码。
	raw, err := config.LoadGlobalConfig()
	if err != nil {
		t.Fatal(err)
	}
	if raw.Deployment.Mode != "mono" {
		t.Fatalf("loader 默认应为 mono（本用例前提，若变更需重审入口强制逻辑）, got %q", raw.Deployment.Mode)
	}

	// 入口同款路径（loadConfig = LoadGlobalConfig + 无条件强制 micro）。
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Deployment.Mode != "micro" {
		t.Fatalf("入口必须固化 micro, got %q", cfg.Deployment.Mode)
	}
	if cfg.Server.HTTPAddr != ":8081" {
		t.Fatalf("recipe HTTP_ADDR 未生效: %q", cfg.Server.HTTPAddr)
	}
	if cfg.SystemGRPCAddr != "127.0.0.1:50051" {
		t.Fatalf("recipe SYSTEM_GRPC_ADDR 未生效: %q", cfg.SystemGRPCAddr)
	}

	conn, err := client.NewGRPCConn(client.GRPCConnParams{Cfg: cfg})
	if err != nil {
		t.Fatalf("NewGRPCConn err = %v, want nil", err)
	}
	if conn == nil {
		t.Fatal("开箱 run-member 必须拨号：nil conn → nilIssuer → 登录 503（review Important）")
	}
	t.Cleanup(func() { _ = conn.Close() })
}
