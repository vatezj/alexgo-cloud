package trace

import (
	"context"
	"os"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.uber.org/fx"
)

// TracerParams 注入 Fx 生命周期。
type TracerParams struct {
	fx.In

	LC fx.Lifecycle
}

// InitTracer 初始化 OpenTelemetry TracerProvider，并在进程退出时 Shutdown。
//
// 导出方式：
// - 默认：仅在进程内生成 trace（不导出）
// - 当设置环境变量 OTEL_EXPORTER_OTLP_ENDPOINT 时：
//   - 使用 OTLP gRPC exporter 进行导出（常见接收端：OTel Collector / Jaeger(all-in-one)/Tempo）
//
// 生产建议：
// - 通过 env 控制采样率（TraceIdRatioBased）与 exporter endpoint（可扩展到 OTEL SDK 配置体系）
// - 配置 Resource 属性（service.name / service.version / deployment.environment 等）
func InitTracer(p TracerParams) {
	ctx := context.Background()
	// OTLP exporter endpoint，例如：
	// - Jaeger all-in-one: "localhost:4317"
	// - OTel Collector: "otel-collector.monitoring:4317"
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	var opts []sdktrace.TracerProviderOption
	// Resource：服务标识，用于在可观测平台聚合与过滤。
	res, _ := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("alexGo-cloud"),
		),
	)
	opts = append(opts, sdktrace.WithResource(res))

	if endpoint != "" {
		// WithInsecure：示例环境直连；生产建议使用 TLS 与鉴权。
		exp, err := otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint(endpoint),
			otlptracegrpc.WithInsecure(),
		)
		if err == nil {
			// Batcher：异步批量上报，减少对请求延迟的影响。
			opts = append(opts, sdktrace.WithBatcher(exp))
		}
	}

	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)

	p.LC.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return tp.Shutdown(ctx)
		},
	})
}

// OTELMiddleware 为 Gin 注入 Trace（生成 span，传播 trace context）。
func OTELMiddleware() gin.HandlerFunc {
	return otelgin.Middleware("alexGo-cloud")
}
