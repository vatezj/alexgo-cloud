package server

import (
	"context"
	"net/http"
	"net/http/pprof"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/fx"

	"github.com/casbin/casbin/v2"

	"alexGo-cloud/pkg/audit"
	"alexGo-cloud/pkg/circuitbreaker"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/limiter"
	"alexGo-cloud/pkg/logger"
	"alexGo-cloud/pkg/middleware"
	"alexGo-cloud/pkg/monitor"
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
	Breaker *circuitbreaker.CircuitBreaker `optional:"true"`
	OperateRecorder audit.OperateRecorder `optional:"true"`
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
		// Tenant：从 Header 注入租户上下文（X-Tenant-ID），用于 SaaS 多租户隔离/审计/限流维度等。
		middleware.TenantMiddleware(),
		// Logger：结构化日志（可在这里对接 trace id / request id）。
		middleware.Logger(),
		// PrometheusMiddleware：记录 http_requests_total（method/path/status），用于错误率/流量监控。
		monitor.PrometheusMiddleware(),
		// RateLimitAndBreaker：入口防护，避免高并发/故障导致雪崩。
		middleware.RateLimitAndBreaker(p.Cfg, p.RateLimiter, p.Breaker),
		// AuthMiddleware：对 /api/admin/** 进行 JWT + Casbin 校验（可通过 enforcer=nil 或配置禁用）。
		middleware.AuthMiddleware(p.Cfg, p.Enforcer),
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

	// 健康检查：不依赖 DB/Redis/NATS 是否可用（生产中可扩展为更严格的 readiness）。
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "alexGo-cloud"})
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
// - /health：健康检查（liveness/readiness 探针）
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
