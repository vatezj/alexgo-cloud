package grpcserver

import (
	"context"
	"net"
	"testing"
	"time"

	"go.uber.org/fx"

	"alexGo-cloud/pkg/config"
)

// freeAddr 取一个当前空闲的 127.0.0.1 端口（先听再关，测试间存在极小竞态窗口，可接受）。
func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

// newGateApp 按部署模式装配 StartGRPCServer（fx.Invoke 在 fx.New 期执行，错误落在 Err()）。
func newGateApp(t *testing.T, mode, addr string) *fx.App {
	t.Helper()
	cfg := &config.Config{}
	cfg.Deployment.Mode = mode
	cfg.Server.GRPCAddr = addr
	app := fx.New(
		fx.Supply(cfg),
		fx.Invoke(StartGRPCServer),
		fx.NopLogger,
	)
	t.Cleanup(func() { _ = app.Stop(context.Background()) })
	return app
}

// mono 模式不起 gRPC：门控必须先于 net.Listen 返回 nil——
// 故意给非法监听地址，若门控缺失则 listen 报错（本测试变红）。
func TestStartGRPCServer_MonoSkipsListen(t *testing.T) {
	app := newGateApp(t, "mono", "256.256.256.256:99999")
	if err := app.Err(); err != nil {
		t.Fatalf("mono 模式不应尝试监听，err = %v", err)
	}
}

// mono 模式下即使地址合法也不得有 TCP 监听（本地直调 token.Service，无需监听）。
func TestStartGRPCServer_MonoNotListening(t *testing.T) {
	addr := freeAddr(t)
	app := newGateApp(t, "mono", addr)
	if err := app.Err(); err != nil {
		t.Fatalf("mono 装配失败: %v", err)
	}
	if err := app.Start(context.Background()); err != nil {
		t.Fatalf("Start() = %v", err)
	}
	conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		t.Fatalf("mono 模式不得监听 %s", addr)
	}
}

// micro 模式必须真实监听（system-server 的 gRPC 入口）。
func TestStartGRPCServer_MicroListens(t *testing.T) {
	addr := freeAddr(t)
	app := newGateApp(t, "micro", addr)
	if err := app.Err(); err != nil {
		t.Fatalf("micro 装配失败: %v", err)
	}
	if err := app.Start(context.Background()); err != nil {
		t.Fatalf("Start() = %v", err)
	}
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("micro 模式必须监听 %s: %v", addr, err)
	}
	_ = conn.Close()
}

// 门控放行方向：micro 模式下 listen 的真实失败必须原样返回（不被门控吞掉）。
func TestStartGRPCServer_MicroListenErrorPropagates(t *testing.T) {
	app := newGateApp(t, "micro", "256.256.256.256:99999")
	if err := app.Err(); err == nil {
		t.Fatal("micro 模式非法地址应报监听错误")
	}
}
