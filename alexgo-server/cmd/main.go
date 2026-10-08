package main

import (
	"flag"
	"fmt"
	"time"

	"go.uber.org/fx"
	"google.golang.org/grpc"

	"alexGo-cloud/alexgo-server/server"
	"alexGo-cloud/modules/order"
	"alexGo-cloud/modules/system"
	systemservice "alexGo-cloud/modules/system/service"
	"alexGo-cloud/pkg/auth"
	"alexGo-cloud/pkg/circuitbreaker"
	"alexGo-cloud/pkg/client"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/database"
	"alexGo-cloud/pkg/limiter"
	"alexGo-cloud/pkg/logger"
	"alexGo-cloud/pkg/migrate"
	"alexGo-cloud/pkg/mq"
	"alexGo-cloud/pkg/outbox"
	appredis "alexGo-cloud/pkg/redis"
	"alexGo-cloud/pkg/token"
	"alexGo-cloud/pkg/trace"
)

// main 是 alexGo-cloud 单体模式的统一启动入口。
//
// 设计目标：
// 1) 单体一键启动：HTTP + 模块路由 + 迁移 + 可观测性，默认都在这里装配。
// 2) 零成本演进：当开启 microservice.enabled 时，通过 Fx Decorate 把本地接口实现替换为 gRPC 客户端实现。
// 3) 生产可控：通过配置开关启用/禁用 outbox、NATS、Redis、限流熔断等能力。
//
// 运行方式：
// - 默认：启动 HTTP 服务（同时执行迁移 + 启动 Outbox Relay 等后台任务）
// - --migrate-only：仅执行迁移与后台初始化，不启动 HTTP（适合 initJob / 发布前迁移）
func main() {
	// --migrate-only：用于 CI/CD 或 K8s Job，只做 schema migrate，不起 HTTP。
	migrateOnly := flag.Bool("migrate-only", false, "run migrations only")
	flag.Parse()

	// 初始化全局 logger（Zap），后续所有模块共用同一实例。
	logger.Init()
	if logger.Log != nil {
		defer func() { _ = logger.Log.Sync() }()
	}

	opts := []fx.Option{
		// fx.Provide：声明依赖构建方式（“怎么构造”）。
		// fx.Invoke：声明副作用入口（“启动时做什么”）。
		//
		// 这里的顺序不影响依赖解析，但“阅读顺序”建议从基础设施 → 应用层 → 模块层。
		fx.Provide(
			// 配置加载：全局 config.yaml + modules/<name>/configs/config.yaml 命名空间合并。
			config.LoadGlobalConfig,
			// 数据库连接：GORM + 连接池 + Fx OnStop 优雅关闭。
			database.NewDB,
			// OAuth2 令牌服务：同时作为 Issuer（签发）与 Validator（中间件校验）。
			token.NewService,
			// 当 microservice.enabled=true 时可能需要创建 gRPC 连接（否则返回 nil）。
			client.NewGRPCConn,
			// Casbin Enforcer：用于 RBAC 权限校验（AuthMiddleware 内可选启用）。
			auth.NewCasbinEnforcer,
			// Redis Client：用于限流等分布式状态（未启用时返回 nil）。
			appredis.NewClient,
			// Redis Token Bucket 限流器（底层靠 Lua 脚本保证原子性）。
			limiter.NewRateLimiter,
			// 熔断器：当下游/自身错误率持续高时“快速失败”，避免雪崩。
			// OpenSecond 表示熔断保持打开的时间窗口。
			func(cfg *config.Config) *circuitbreaker.CircuitBreaker {
				if cfg == nil || !cfg.Breaker.Enabled {
					return nil
				}
				return circuitbreaker.NewCircuitBreaker(cfg.Breaker.Threshold, time.Duration(cfg.Breaker.OpenSecond)*time.Second)
			},
			// NATS JetStream Broker：Outbox Relay 会把事件发布到 NATS（未启用时返回 nil）。
			mq.NewNATSBroker,
			// Outbox Relay：轮询 outbox_events，把 pending 事件可靠发布并更新状态。
			outbox.NewRelay,
			// migrate.Runner：收集所有模块的 migration_sources，在启动时统一执行。
			fx.Annotate(
				migrate.NewRunner,
				fx.ParamTags("", "", `group:"migration_sources"`),
			),
		),
		// 启动 Outbox Relay（后台 goroutine + ticker）。是否启用由配置 outbox.enabled 控制。
		fx.Invoke(outbox.StartRelay),
		// Decorate：在依赖图构建完成后“替换某个类型的最终实现”。
		// 这里用于把 system.UserService（本地实现）在微服务模式下替换为 gRPC 客户端实现。
		fx.Decorate(func(cfg *config.Config, local systemservice.UserService, conn *grpc.ClientConn) (systemservice.UserService, error) {
			if cfg == nil || !cfg.Microservice.Enabled {
				// 单体模式：直接用本地实现（跨模块调用零开销）。
				return local, nil
			}
			if conn == nil {
				// 微服务模式：必须存在 gRPC 连接（否则配置与运行时不一致）。
				return nil, fmt.Errorf("microservice enabled but grpc conn is nil")
			}
			// 微服务模式：返回 gRPC client（实现同一接口），调用方无需改代码。
			return client.NewUserGRPCClient(conn), nil
		}),
		// OpenTelemetry TracerProvider 初始化（支持 OTLP exporter）。
		fx.Invoke(trace.InitTracer),
		// 迁移：默认启动时执行；生产可通过 migrate.auto 配置控制是否启用。
		fx.Invoke(func(r *migrate.Runner) error { return r.Run() }),
		// 模块装配：每个模块将自身作为 server.Module 注册到 group:"modules"。
		system.FxModule,
		order.FxModule,
	}

	if !*migrateOnly {
		// 统一 HTTP Server（Gin + 中间件 + 模块路由注册）。
		opts = append(opts, fx.Invoke(server.StartHTTPServer))
	}

	// fx.New(...) 会构建依赖图并按 Lifecycle 启动；Run() 会阻塞直到接收到退出信号。
	fx.New(opts...).Run()
}
