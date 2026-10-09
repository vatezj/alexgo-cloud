package main

import (
	"context"
	"flag"
	"fmt"

	"go.uber.org/fx"
	"google.golang.org/grpc"
	"gorm.io/gorm"

	"alexGo-cloud/alexgo-server/server"
	"alexGo-cloud/modules/member"
	"alexGo-cloud/pkg/client"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/database"
	"alexGo-cloud/pkg/logger"
	"alexGo-cloud/pkg/migrate"
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/tenant/gormplugin"
	"alexGo-cloud/pkg/token"
	"alexGo-cloud/pkg/trace"
)

// main 是 member-server 入口：member 模块的独立进程（micro 模式，make run-member）。
// 与 system-server（alexgo-server/cmd）的差异：
//   - HTTP 端口 :8081（HTTP_ADDR env 覆盖）；
//   - 无 Casbin/审计（member 无 RBAC）；
//   - token.Issuer = gRPC 客户端（委托 system-server 签发，失败即登录 503）；
//   - 校验（Validator）= 本地 token.Service 读共享 token 表（同库约束，spec §9.10）；
//   - 账号额度 = pkg/tenant 的 DB 窄实现（system 模块不在本进程）；
//   - ScopeLoader 不提供（由 system 模块提供，optional nil 安全）。
func main() {
	// --migrate-only：用于 CI/CD 或 K8s Job，只做 schema migrate，不起 HTTP。
	migrateOnly := flag.Bool("migrate-only", false, "run migrations only")
	flag.Parse()

	logger.Init()
	if logger.Log != nil {
		defer func() { _ = logger.Log.Sync() }()
	}

	// 配置提前加载 + fx.Supply（与 system-server 入口同一模式）。
	cfg, err := loadConfig()
	if err != nil {
		panic(err)
	}

	fx.New(options(cfg, *migrateOnly)...).Run()
}

// loadConfig 加载配置并固化 member-server 的形态身份；
// 入口与接线测试（TestRunMemberRecipeEnv_Wiring）共用同一路径，防"测试手动置
// Mode 绕过真实默认值"的漂移。
func loadConfig() (*config.Config, error) {
	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		return nil, err
	}
	// member-server 二进制的身份就是 micro 形态：无论配置文件/环境写什么，
	// 本进程必然按双服务模式运行（mono 模式该入口不参与）。
	// 必须无条件覆盖：loader 对 deployment.mode 有 viper 默认值 "mono"，
	// `Mode == ""` 空值回退是死代码（config 加载永远返回 "mono"）——review 已判。
	cfg.Deployment.Mode = "micro"
	return cfg, nil
}

// options 装配 member-server fx 清单（正/反向干跑测试复用同一清单，防止测试与入口漂移）。
// 账号额度检查器单独追加：反向干跑测试剔除之，证明 NewMemberService 真实消费
// tenant.AccountLimitChecker（三参签名不可回退）。
func options(cfg *config.Config, migrateOnly bool) []fx.Option {
	opts := baseOptions(cfg, migrateOnly)
	// system 模块不在本进程 → 其 FxModule 的 TenantService 映射缺席，
	// 用 pkg/tenant 的 DB 窄实现补齐（语义与 system 侧 CheckAccountLimit 一致）。
	opts = append(opts, fx.Provide(tenant.NewAccountLimitChecker))
	return opts
}

// baseOptions 基础装配：基础设施 Provider/Invoke + member 模块（不含额度检查器）。
func baseOptions(cfg *config.Config, migrateOnly bool) []fx.Option {
	opts := []fx.Option{
		// 配置：提前加载后经 Supply 注入（见 main）。
		fx.Supply(cfg),
		fx.Provide(
			// 数据库连接：GORM + 连接池 + Fx OnStop 优雅关闭。
			database.NewDB,
			// gRPC 连接：deployment.mode=micro 或 microservice.enabled 任一开启即拨号
			// （地址解析 system_grpc_addr 优先）；两开关皆关 → nil conn → nilIssuer fail-fast。
			client.NewGRPCConn,
			// 租户域名解析（Host → tenant_id）：HTTPServerParams.TenantDomainLookup
			// optional 可为 nil，但仍 Provide 以支持域名解析（与 system-server 同款）。
			tenant.NewDomainLookup,
			// OAuth2 令牌服务：本地 Validator（读共享 token 表）。
			token.NewService,
			func(s *token.Service) token.Validator { return s },
			// Issuer：委托 system-server 签发；conn 为 nil 时 Issuer 各方法返回错误 → 登录 503。
			// 注意顺序（T11 裁决）：必须先用具体指针 conn == nil 判断再包装——
			// typed-nil *grpc.ClientConn 直接判 ==nil 为真可拦截；若先包成 token.Issuer
			// 接口再判空，接口恒非 nil（动态类型非 nil）→ 调用即 panic。
			tokenIssuerProvider,
			// migrate.Runner：收集模块 migration_sources，启动时统一执行。
			fx.Annotate(
				migrate.NewRunner,
				fx.ParamTags("", "", `group:"migration_sources"`),
			),
		),
		// 租户字段隔离插件：与 system 侧同款 ExemptTables（tenants/casbin_rule 全局表放行）。
		// 必须 fx.Invoke（不能再 Provide 一个 *gorm.DB，会与 database.NewDB 冲突），
		// 放 Provide 之后、StartHTTPServer 的 Invoke 之前——任何请求进入前插件已挂载。
		fx.Invoke(func(db *gorm.DB) error {
			return gormplugin.Register(db, gormplugin.Options{
				ExemptTables: []string{"tenants", "casbin_rule"},
			})
		}),
		// OpenTelemetry TracerProvider 初始化（支持 OTLP exporter）。
		fx.Invoke(trace.InitTracer),
		// 迁移：默认启动时执行（migrate.auto 控制）。
		fx.Invoke(func(r *migrate.Runner) error { return r.Run() }),
		member.FxModule,
	}
	if !migrateOnly {
		// 统一 HTTP Server（Gin + 中间件 + 模块路由注册）；本进程只有 member 路由。
		opts = append(opts, fx.Invoke(server.StartHTTPServer))
	}
	return opts
}

// tokenIssuerProvider 产出签发器：conn 为 nil（mono 形态两开关皆关）时返回
// nilIssuer fail-fast，否则包装 gRPC 客户端委托 system-server 签发。
func tokenIssuerProvider(conn *grpc.ClientConn) token.Issuer {
	if conn == nil {
		return nilIssuer{}
	}
	return client.NewTokenIssuer(conn)
}

// nilIssuer：gRPC 未连接时的 fail-fast 实现（所有操作返回错误，登录接口映射 503）。
type nilIssuer struct{}

func (nilIssuer) Issue(context.Context, token.IssueParams) (*token.Issued, error) {
	return nil, fmt.Errorf("member-server: system grpc not connected")
}
func (nilIssuer) Refresh(context.Context, string) (*token.Issued, error) {
	return nil, fmt.Errorf("member-server: system grpc not connected")
}
func (nilIssuer) Revoke(context.Context, string) error {
	return fmt.Errorf("member-server: system grpc not connected")
}
func (nilIssuer) RevokeAll(context.Context, token.UserType, uint64) error {
	return fmt.Errorf("member-server: system grpc not connected")
}

// 编译期断言：确保实现满足接口。
var _ token.Issuer = nilIssuer{}
