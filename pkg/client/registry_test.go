package client

import (
	"strings"
	"testing"

	"alexGo-cloud/pkg/config"
)

// 显式 system_grpc_addr 优先：即便 microservice.enabled=false 也返回配置地址
// （deployment.mode=micro 双服务形态的主路径：make run-member 传 SYSTEM_GRPC_ADDR）。
func TestGetServiceAddress_ExplicitAddrWins(t *testing.T) {
	cfg := &config.Config{SystemGRPCAddr: "1.2.3.4:50051"}
	got, err := GetServiceAddress(cfg, "system")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if got != "1.2.3.4:50051" {
		t.Fatalf("addr = %q, want 1.2.3.4:50051", got)
	}
}

// 两开关皆关且无地址 → 明确报错（不静默给占位地址）。
func TestGetServiceAddress_NoAddrNoFlagErrors(t *testing.T) {
	if _, err := GetServiceAddress(&config.Config{}, "system"); err == nil {
		t.Fatal("无地址且旧开关关闭必须报错")
	}
}

// 旧路径回退：microservice.enabled=true 且无显式地址 → 本机占位 localhost:50051。
func TestGetServiceAddress_LegacyFallback(t *testing.T) {
	cfg := &config.Config{}
	cfg.Microservice.Enabled = true
	got, err := GetServiceAddress(cfg, "system")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if got != "localhost:50051" {
		t.Fatalf("addr = %q, want localhost:50051", got)
	}
}

// nil 配置 → 报错（非 panic）。
func TestGetServiceAddress_NilConfig(t *testing.T) {
	if _, err := GetServiceAddress(nil, "system"); err == nil {
		t.Fatal("nil cfg 必须报错")
	}
}

// deployment.mode=micro + 显式地址 → 真实拨号（grpc.NewClient 懒连接，无需对端在线）。
func TestNewGRPCConn_MicroModeDials(t *testing.T) {
	cfg := &config.Config{SystemGRPCAddr: "127.0.0.1:50051"}
	cfg.Deployment.Mode = "micro"

	conn, err := NewGRPCConn(GRPCConnParams{Cfg: cfg})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if conn == nil {
		t.Fatal("micro 模式必须拨号：conn 不能为 nil（否则 member 恒 nilIssuer → 登录 503）")
	}
	t.Cleanup(func() { _ = conn.Close() })
}

// 旧开关单独开启（无显式地址）也拨号（走 localhost 占位回退）。
func TestNewGRPCConn_LegacyFlagDials(t *testing.T) {
	cfg := &config.Config{}
	cfg.Microservice.Enabled = true

	conn, err := NewGRPCConn(GRPCConnParams{Cfg: cfg})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if conn == nil {
		t.Fatal("microservice.enabled=true 必须拨号")
	}
	t.Cleanup(func() { _ = conn.Close() })
}

// 两开关皆关（mono 默认）→ (nil, nil)：调用方按 typed-nil 判空 fail-fast。
func TestNewGRPCConn_BothOffReturnsNil(t *testing.T) {
	conn, err := NewGRPCConn(GRPCConnParams{Cfg: &config.Config{}})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if conn != nil {
		_ = conn.Close()
		t.Fatal("mono 默认（两开关皆关）必须返回 nil conn")
	}
}

// nil 配置 → (nil, nil)（保持原行为）。
func TestNewGRPCConn_NilConfigReturnsNil(t *testing.T) {
	conn, err := NewGRPCConn(GRPCConnParams{})
	if err != nil || conn != nil {
		t.Fatalf("got (%v, %v), want (nil, nil)", conn, err)
	}
}

// 编译期防回归：错误文案提及两个可操作开关（便于排障）。
func TestGetServiceAddress_ErrorIsActionable(t *testing.T) {
	_, err := GetServiceAddress(&config.Config{}, "system")
	if err == nil || !strings.Contains(err.Error(), "system_grpc_addr") {
		t.Fatalf("error = %v, want 提示 system_grpc_addr", err)
	}
}
