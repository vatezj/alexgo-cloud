package server

import (
	"context"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/fx"
	"gorm.io/gorm"

	"github.com/casbin/casbin/v2"

	"alexGo-cloud/pkg/audit"
	"alexGo-cloud/pkg/circuitbreaker"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/limiter"
	"alexGo-cloud/pkg/logger"
	"alexGo-cloud/pkg/middleware"
	"alexGo-cloud/pkg/monitor"
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/token"
	"alexGo-cloud/pkg/trace"
)

// Module 是“模块化单体”的核心抽象：每个业务模块实现 RegisterRoutes，把自己的 HTTP 路由挂到统一 Gin RouterGroup 上。
//
// 单体模式下：模块之间通过 Go 接口直接调用（零 RPC 开销）。
// 微服务演进时：模块可以拆出独立进程（例如 gRPC server），但在单体内仍保留相同接口，靠 Fx 条件注入切换实现。
type Module interface {
	RegisterRoutes(r *gin.RouterGroup)
}

// HTTPServerParams 使用 Fx.In 进行参数注入。
//
// 约定：
// - Enforcer/RateLimiter/Breaker 都是 optional，表示可以通过配置关闭并返回 nil。
// - Modules 通过 group:"modules" 聚合：每个模块以 group 的形式向这里“注册自己”。
type HTTPServerParams struct {
	fx.In

	LC  fx.Lifecycle
	Cfg *config.Config
	// Enforcer：Casbin RBAC 引擎；nil 表示不做权限校验（仅做 JWT 校验或完全放行，取决于中间件实现）。
	Enforcer *casbin.Enforcer `optional:"true"`
	// RateLimiter：Redis Token Bucket 限流器；nil 表示禁用限流（Allow() 会默认放行）。
	RateLimiter *limiter.RateLimiter `optional:"true"`
	// Breaker：熔断器；nil 表示禁用熔断（请求不会因熔断提前失败）。
	Breaker         *circuitbreaker.CircuitBreaker `optional:"true"`
	OperateRecorder audit.OperateRecorder          `optional:"true"`
	// TokenValidator：mode=token 时的令牌校验器（fx 由 token.NewService 提供）。
	TokenValidator token.Validator `optional:"true"`
	// TenantDomainLookup：域名→租户解析（nil 则只认 X-Tenant-ID 头；fx 由 tenant.NewDomainLookup 提供）。
	TenantDomainLookup tenant.DomainLookup `optional:"true"`
	// ScopeLoader：数据权限加载器（T10，fx 由 system 模块的 NewDataScopeLoader 映射提供）。
	// optional：member 端不提供 → nil → AuthMiddleware 不注入 scope（只剩租户隔离，安全缺省）。
	ScopeLoader tenant.ScopeLoader `optional:"true"`
	// DB：用于 /health/ready 探活（fx 由 database.NewDB 注入）。
	DB *gorm.DB
	// Modules：模块列表（group 聚合）。
	Modules []Module `group:"modules"`
}

// newRouter 构建完整的 Gin 路由（中间件链 + 模块路由 + 运维端点）。
// 抽出为独立函数以便在单测中直接构造路由（pprof 开关、health 行为等）。
func newRouter(p HTTPServerParams) *gin.Engine {
	r := gin.New()
	r.Use(
		// Recovery：防止 panic 直接把进程打挂（对外返回 500）。
		middleware.Recovery(),
		// Tenant：解析租户并注入 context（X-Tenant-ID 头优先，其次 Host 域名 → tenant_id），
		// 供 SaaS 多租户隔离（gorm 插件）/审计/限流维度使用。
		middleware.NewTenantMiddleware(p.TenantDomainLookup),
		// Logger：结构化日志（可在这里对接 trace id / request id）。
		middleware.Logger(),
		// PrometheusMiddleware：记录 http_requests_total（method/path/status），用于错误率/流量监控。
		monitor.PrometheusMiddleware(),
		// RateLimitAndBreaker：入口防护，避免高并发/故障导致雪崩。
		middleware.RateLimitAndBreaker(p.Cfg, p.RateLimiter, p.Breaker),
		// AuthMiddleware：对 /api/admin/** 做 Token/JWT 双模式校验 + 管理员门槛 + 可选 Casbin
		// （/api/app/** 全部公开：member refresh/logout 是 possession-based，controller 自行校验）。
		middleware.NewAuthMiddleware(middleware.AuthDeps{
			Cfg: p.Cfg, Enforcer: p.Enforcer, Validator: p.TokenValidator, ScopeLoader: p.ScopeLoader,
		}),
		middleware.OperateLogMiddleware(p.OperateRecorder),
		// OTELMiddleware：OpenTelemetry trace，支持把链路导出到 Jaeger/Tempo/OTLP Collector。
		trace.OTELMiddleware(),
		// Trace：占位，用于未来对接更完整的 trace/span 属性补全。
		middleware.Trace(),
	)

	// /api 下由各模块注册自己的 admin/app 路由。
	api := r.Group("/api")
	for _, m := range p.Modules {
		m.RegisterRoutes(api)
	}

	// 健康检查（liveness）：不依赖 DB/Redis/NATS 是否可用，进程活着即 200。
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "alexGo-cloud"})
	})
	// readiness：依赖探活。DB ping 失败返回 503，供 K8s readinessProbe 摘流。
	// 注意：livenessProbe 继续使用 /health（不依赖 DB），避免 DB 故障触发全员重启。
	r.GET("/health/ready", func(c *gin.Context) {
		if p.DB == nil {
			c.JSON(http.StatusOK, gin.H{"status": "ok", "db": "unconfigured"})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		sqlDB, err := p.DB.DB()
		if err == nil {
			err = sqlDB.PingContext(ctx)
		}
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "db": "down"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "db": "up"})
	})
	// /metrics：Prometheus 拉取点（Prometheus server scrape）。
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	// pprof：默认关闭（server.pprof_enabled=true 才挂载），
	// 避免生产环境通过业务端口泄露运行时信息。
	if p.Cfg.Server.PprofEnabled {
		r.GET("/debug/pprof/", gin.WrapF(pprof.Index))
		r.GET("/debug/pprof/cmdline", gin.WrapF(pprof.Cmdline))
		r.GET("/debug/pprof/profile", gin.WrapF(pprof.Profile))
		r.GET("/debug/pprof/symbol", gin.WrapF(pprof.Symbol))
		r.GET("/debug/pprof/trace", gin.WrapF(pprof.Trace))
	}
	return r
}

// StartHTTPServer 构建 Gin Router、挂载全局中间件、聚合注册模块路由，并通过 Fx Lifecycle 托管 http.Server。
//
// 路由结构约定：
// - /api/**：业务 API（模块自己注册）
// - /health：liveness 探针（不依赖 DB）
// - /health/ready：readiness 探针（ping DB，失败 503）
// - /metrics：Prometheus 指标
// - /debug/pprof/**：pprof 性能分析（默认关闭，由 server.pprof_enabled 控制）
func StartHTTPServer(p HTTPServerParams) {
	r := newRouter(p)

	srv := &http.Server{Addr: p.Cfg.Server.HTTPAddr, Handler: r}

	p.LC.Append(fx.Hook{
		// OnStart：非阻塞启动 http server。
		OnStart: func(ctx context.Context) error {
			if logger.Log != nil {
				logger.Log.Info("HTTP server starting on " + p.Cfg.Server.HTTPAddr)
			}
			go func() {
				_ = srv.ListenAndServe()
			}()
			return nil
		},
		// OnStop：优雅关闭，等待 in-flight 请求结束（由 ctx 控制超时）。
		OnStop: func(ctx context.Context) error {
			if logger.Log != nil {
				logger.Log.Info("HTTP server shutting down...")
			}
			return srv.Shutdown(ctx)
		},
	})
}
