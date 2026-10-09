package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// LoadGlobalConfig 加载并合并配置，返回最终可用的 *Config。
//
// 合并策略：
// 1) 先加载全局配置：alexgo-server/configs/config.yaml
// 2) 读取 modules 开关：modules.<name>=true 的模块会继续加载 modules/<name>/configs/config.yaml
// 3) 模块配置以“命名空间”合并：例如 system/config.yaml 中的 jwt_secret 会合并到 key "system.jwt_secret"
//
// 环境变量覆盖：
// - viper.AutomaticEnv() + BindEnv：HTTP_ADDR / GRPC_ADDR / NATS_URL / REDIS_ADDR
// - applyEnvOverrides（Unmarshal 之后）：DB_DSN / JWT_SECRET / REDIS_PASSWORD / DEPLOYMENT_MODE / SYSTEM_GRPC_ADDR
//   （密钥类与部署形态类，保证压过模块 yaml——viper Unmarshal 不读隐式 env，必须显式覆盖）
//
// 容器友好：
// - 当 config 文件不存在时不会报错（ConfigFileNotFoundError 直接忽略），便于仅靠 env 启动。
func LoadGlobalConfig() (*Config, error) {
	v := viper.New()
	// 把 server.http_addr 映射到 SERVER_HTTP_ADDR 的形式，便于在 K8s/Compose 通过 env 注入。
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 默认值：保证“开箱即用”（即使没有配置文件/没有 env）。
	v.SetDefault("server.http_addr", ":8080")
	v.SetDefault("server.grpc_addr", ":50051")
	v.SetDefault("server.pprof_enabled", false)
	v.SetDefault("migrate.auto", true)
	v.SetDefault("outbox.enabled", true)
	v.SetDefault("outbox.batch_size", 100)
	v.SetDefault("outbox.interval_second", 5)
	v.SetDefault("mq.nats.enabled", false)
	v.SetDefault("mq.nats.url", "nats://127.0.0.1:4222")
	v.SetDefault("mq.nats.stream", "ALEXGO")
	v.SetDefault("mq.nats.subject_prefix", "alexgo.>")
	v.SetDefault("redis.enabled", false)
	v.SetDefault("redis.addr", "127.0.0.1:6379")
	v.SetDefault("redis.db", 0)
	v.SetDefault("limiter.enabled", false)
	v.SetDefault("limiter.rate", 100)
	v.SetDefault("limiter.burst", 200)
	v.SetDefault("breaker.enabled", false)
	v.SetDefault("breaker.threshold", 10)
	v.SetDefault("breaker.open_second", 30)
	v.SetDefault("system.default_admin_username", "admin")
	v.SetDefault("system.default_admin_password", "admin123")
	v.SetDefault("auth.mode", "token")
	v.SetDefault("auth.access_expire_hour", 2)
	v.SetDefault("auth.refresh_expire_day", 7)
	v.SetDefault("deployment.mode", "mono")
	v.SetDefault("system_grpc_addr", "127.0.0.1:50051")

	// 常用 env 覆盖：兼容 docker-compose / k8s secret 注入。
	_ = v.BindEnv("server.http_addr", "HTTP_ADDR")
	_ = v.BindEnv("server.grpc_addr", "GRPC_ADDR")
	_ = v.BindEnv("mq.nats.url", "NATS_URL")
	_ = v.BindEnv("redis.addr", "REDIS_ADDR")

	// 配置文件路径使用项目内固定位置，保证开发体验一致。
	v.SetConfigFile("alexgo-server/configs/config.yaml")
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		// 允许配置文件不存在：便于仅依赖环境变量启动（容器/平台更常见）。
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read global config failed: %w", err)
		}
	}

	// 读取 modules 开关，决定需要加载哪些模块的本地配置。
	modulesRaw := v.GetStringMap("modules")
	for modName, val := range modulesRaw {
		on, ok := val.(bool)
		if !ok || !on {
			continue
		}
		modPath := filepath.Join("modules", modName, "configs", "config.yaml")
		if _, err := os.Stat(modPath); err != nil {
			continue
		}

		// 每个模块一个 viper，用于读模块的 config.yaml。
		mv := viper.New()
		mv.SetConfigFile(modPath)
		mv.SetConfigType("yaml")
		if err := mv.ReadInConfig(); err != nil {
			continue
		}

		prefix := modName + "."
		for k, val := range mv.AllSettings() {
			v.Set(prefix+k, val)
		}
	}

	var cfg Config
	// Unmarshal：将 viper key-value 映射到结构体（mapstructure tag 控制字段映射）。
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config failed: %w", err)
	}
	applyEnvOverrides(&cfg)
	return &cfg, nil
}

// applyEnvOverrides 在 Unmarshal 之后用显式环境变量覆盖安全敏感配置。
//
// 为什么 DB_DSN 这类密钥 env 光靠 AutomaticEnv/BindEnv 不生效：
//  1. 没有显式绑定，且名字对不上——AutomaticEnv + EnvKeyReplacer 是按 viper key 做
//     “`.`→`_`” 转写来查 env 的（database.dsn → DATABASE_DSN），并不会查 DB_DSN；
//  2. 模块配置合并使用 v.Set()，其优先级高于 env，即使绑定了名字，env 也会输给模块 yaml。
//
// 因此对 K8s Secret / Compose 注入的密钥变量，在这里做最终覆盖（仅非空时生效）。
func applyEnvOverrides(cfg *Config) {
	pairs := []struct {
		env string
		set func(v string)
	}{
		{"DB_DSN", func(v string) { cfg.Database.DSN = v }},
		{"JWT_SECRET", func(v string) { cfg.System.JWTSecret = v }},
		{"REDIS_PASSWORD", func(v string) { cfg.Redis.Password = v }},
		{"DEPLOYMENT_MODE", func(v string) { cfg.Deployment.Mode = v }},
		{"SYSTEM_GRPC_ADDR", func(v string) { cfg.SystemGRPCAddr = v }},
	}
	for _, p := range pairs {
		if v := os.Getenv(p.env); v != "" {
			p.set(v)
		}
	}
}
