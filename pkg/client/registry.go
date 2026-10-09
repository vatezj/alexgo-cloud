package client

import (
	"fmt"

	"alexGo-cloud/pkg/config"
)

// GetServiceAddress 解析 gRPC 目标地址。
// 优先级：显式 system_grpc_addr 配置 > microservice.enabled 旧路径（consul 演进项，
// 当前返回本机占位）——deployment.mode=micro 的双服务形态下即便未开旧开关，
// 只要配置了地址即可拨号（make run-member 的 SYSTEM_GRPC_ADDR 即走此分支）。
func GetServiceAddress(cfg *config.Config, serviceName string) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("config is nil")
	}
	_ = serviceName // consul 服务发现未接入：serviceName 暂为占位参数
	if cfg.SystemGRPCAddr != "" {
		return cfg.SystemGRPCAddr, nil
	}
	if cfg.Microservice.Enabled {
		// 旧演进路径：consul 注册尚未接入，暂返回本机占位地址。
		return "localhost:50051", nil
	}
	return "", fmt.Errorf("no grpc address: set system_grpc_addr or enable microservice.enabled")
}
