package main

import (
	"flag"
	"fmt"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"gorm.io/gorm"

	"alexGo-cloud/alexgo-server/server"
	"alexGo-cloud/modules/member"
	"alexGo-cloud/modules/order"
	"alexGo-cloud/modules/system"
	"alexGo-cloud/modules/system/grpcserver"
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
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/tenant/gormplugin"
	"alexGo-cloud/pkg/token"
	"alexGo-cloud/pkg/trace"
)

// main 是 alexGo-cloud 的 system-server 入口，支持双运行模式：
//
//   - mono 模式（默认，deployment.mode != "micro"）：单进程装配 system+order+member
//     全部模块（make run 一键全起），token 签发/校验都用本地 token.Service；
//   - micro 模式（DEPLOYMENT_MODE=micro，make run-system）：本进程只装 system+order，
//     member 路由由 modules/member/cmd 独立进程提供（make run-member），
//     会员登录经 gRPC TokenService 委托本进程签发。
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

	// 配置提前加载：模块装配（mono/micro）在 fx 构建期就需要 deployment.mode，
	// 无法等 fx 内 Provide 再分支——所以此处直接加载并 fx.Supply，
	// 原先 fx.Provide(config.LoadGlobalConfig) 移除（*config.Config 全局唯一来源）。
	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		if logger.Log != nil {
			logger.Log.Fatal("load config failed", zap.Error(err))
		}
		panic(err)
	}

	// fx.New(...) 会构建依赖图并按 Lifecycle 启动；Run() 会阻塞直到接收到退出信号。
	fx.New(options(cfg, *migrateOnly)...).Run()
}

// options 装配入口 fx 清单（正/反向干跑测试复用同一清单，防止测试与入口漂移）。
// 接口映射单独切片（ifaceOptions）：反向干跑测试剔除之——token.Issuer 由模块
// 非 optional 消费直接报缺；token.Validator 的生产消费方是 optional 参数（缺了图仍成立），
// 故其反向判别用测试内真实消费者构造（见 TestBaseOptions_RequiresTokenValidator）。
func options(cfg *config.Config, migrateOnly bool) []fx.Option {
	return append(baseOptions(cfg, migrateOnly), ifaceOptions()...)
}

// ifaceOptions 入口级接口映射：fx 按具体类型 *token.Service 提供、不会自动满足接口，
// 故显式 Provide、全局唯一——登录签发（system NewAuthService / member NewMemberService）
// 与中间件校验分别消费，system/member 两模块共用（member-server 入口同样写一份）。
func ifaceOptions() []fx.Option {
	return []fx.Option{
		fx.Provide(
			func(s *token.Service) token.Issuer { return s },
			func(s *token.Service) token.Validator { return s },
		),
	}
}

// baseOptions 基础装配：基础设施 Provider/Invoke + system/order 模块
// （member 模块按部署模式条件追加；接口映射见 ifaceOptions）。
func baseOptions(cfg *config.Config, migrateOnly bool) []fx.Option {
	// fx.Provide：声明依赖构建方式（“怎么构造”）。
	// fx.Invoke：声明副作用入口（“启动时做什么”）。
	//
	// 这里的顺序不影响依赖解析，但“阅读顺序”建议从基础设施 → 应用层 → 模块层。
	opts := []fx.Option{
		// 配置：提前加载后经 Supply 注入（见 main），此处不再 Provide 加载器。
		fx.Supply(cfg),
		fx.Provide(
			// 数据库连接：GORM + 连接池 + Fx OnStop 优雅关闭。
			database.NewDB,
			// 租户域名解析（Host → tenant_id），供 TenantMiddleware 注入（可选，nil 时只认 X-Tenant-ID 头）。
			tenant.NewDomainLookup,
			// OAuth2 令牌服务：同时作为 Issuer（签发）与 Validator（中间件校验）；
			// 接口映射见 ifaceOptions。
			token.NewService,
			// gRPC 连接：deployment.mode=micro 或 microservice.enabled 任一开启即拨号
			// （地址解析 system_grpc_addr 优先，见 pkg/client.GetServiceAddress），否则返回 nil。
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
		// 租户字段隔离插件：对 DB 挂 INSERT 填充/查询过滤回调。
		// 必须用 fx.Invoke 而不是再 Provide 一个 *gorm.DB——同一类型两个 provider 会与
		// database.NewDB 冲突（FX 直接报错）。Invoke 在依赖图构建后执行，
		// 放在 Provide 之后、server.StartHTTPServer 的 Invoke 之前，
		// 保证任何 HTTP 请求进入前插件已挂载。
		fx.Invoke(func(db *gorm.DB) error {
			return gormplugin.Register(db, gormplugin.Options{
				ExemptTables: []string{"tenants", "casbin_rule"},
			})
		}),
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
		// gRPC TokenService：仅 micro 模式实际监听（StartGRPCServer 内部按 deployment.mode 门控，
		// mono 直接 return——单进程模式 Token 走本地直调，无需监听）。
		fx.Invoke(grpcserver.StartGRPCServer),
		// 模块装配：每个模块将自身作为 server.Module 注册到 group:"modules"。
		system.FxModule,
		order.FxModule,
	}
	// mono 模式（deployment.mode != "micro"）：member 路由同进程注册，
	// TokenIssuer/AccountLimitChecker 都由本进程本地满足；
	// micro 模式：member 由 modules/member/cmd 独立启动，本进程不装其路由。
	if cfg.Deployment.Mode != "micro" {
		opts = append(opts, member.FxModule)
	}
	if !migrateOnly {
		// 统一 HTTP Server（Gin + 中间件 + 模块路由注册）。
		opts = append(opts, fx.Invoke(server.StartHTTPServer))
	}
	return opts
}
