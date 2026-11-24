package taokouling

import (
	"math"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"alexGo-cloud/alexgo-server/server"
	"alexGo-cloud/modules/taokouling/client"
	// 契约钉死 provides 形如 app.NewAppController / admin.NewAdminController（照 starfuyu 记法）；
	// 本模块 controller 为单包平铺（app.go/admin.go/app_test.go 同包），故同一包以两个别名导入。
	admin "alexGo-cloud/modules/taokouling/controller"
	app "alexGo-cloud/modules/taokouling/controller"
	"alexGo-cloud/modules/taokouling/repository"
	"alexGo-cloud/modules/taokouling/service"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/migrate"
)

// FxModule taokouling 模块的 fx 装配（Task 13/14 收口：C 端双端点 + admin 五端点，
// 脚手架 channel/log 控制器与 module_register_* 路由已随 R9 删除）。
var FxModule = fx.Module("taokouling",
	fx.Provide(
		repository.NewChannelRepository,
		repository.NewConversionLogRepository,
		// 四客户端：超时/重试按 cfg 归一（timeout<=0 → 5s/3s、maxRetries<=0 → 1）。
		func(cfg *config.Config) *client.ZhetaokeClient {
			z := cfg.Taokouling.Zhetaoke
			return client.NewZhetaokeClient(z.BaseURL, z.Appkey, z.Sid, z.ParsePID,
				normTimeout(z.Timeout, 5*time.Second), normRetries(z.MaxRetries))
		},
		func(cfg *config.Config) *client.HaodankuClient {
			h := cfg.Taokouling.Haodanku
			return client.NewHaodankuClient(h.BaseURL,
				normTimeout(h.Timeout, 5*time.Second), normRetries(h.MaxRetries))
		},
		func(cfg *config.Config) *client.TopClient {
			t := cfg.Taokouling.Top
			return client.NewTopClient(t.BaseURL, t.Appkey, t.Secret, t.UserID,
				normTimeout(t.Timeout, 5*time.Second), normRetries(t.MaxRetries))
		},
		func(cfg *config.Config) *client.ShortURLClient {
			s := cfg.Taokouling.ShortURL
			// 短链固定单次尝试（失败由调用方回退长链），仅超时归一（3s）。
			return client.NewShortURLClient(s.BaseURL, s.APIKey, normTimeout(s.Timeout, 3*time.Second))
		},
		// 客户端具体类型 → service 编排接口（ConvertParams 按接口消费；dig 不自动匹配接口）。
		func(z *client.ZhetaokeClient) service.TklParser { return z },
		func(h *client.HaodankuClient) service.RatesClient { return h },
		func(t *client.TopClient) service.TopClient { return t },
		func(s *client.ShortURLClient) service.Shortener { return s },
		func(r *repository.ChannelRepository) service.ChannelStore { return r },
		func(r *repository.ConversionLogRepository) service.LogStore { return r },
		// 进程内缓存与按 IP 限流（时钟 time.Now；enabled=false → MaxInt 等效放行）。
		func(cfg *config.Config) *service.TTLCache {
			return service.NewTTLCache(time.Now)
		},
		func(cfg *config.Config) *service.RateLimiter {
			rl := cfg.Taokouling.RateLimit
			window := time.Duration(rl.WindowSeconds) * time.Second
			if window <= 0 {
				window = 60 * time.Second
			}
			if !rl.Enabled {
				return service.NewRateLimiter(math.MaxInt, window, time.Now)
			}
			limit := rl.Limit
			if limit <= 0 {
				limit = 10
			}
			return service.NewRateLimiter(limit, window, time.Now)
		},
		// 编排服务与两端控制器（ConversionService 结构化满足 app.Converter）。
		service.NewConversionService,
		func(s *service.ConversionService) app.Converter { return s },
		app.NewAppController,
		admin.NewAdminController,
	),
	fx.Provide(
		fx.Annotate(
			NewModule,
			fx.As(new(server.Module)),
			fx.ResultTags(`group:"modules"`),
		),
	),
	fx.Provide(
		fx.Annotate(
			NewMigrationSource,
			fx.As(new(migrate.Source)),
			fx.ResultTags(`group:"migration_sources"`),
		),
	),
)

// normTimeout 超时归一：<=0 → def（客户端默认 5s，shorturl 3s）。
func normTimeout(sec int, def time.Duration) time.Duration {
	if sec <= 0 {
		return def
	}
	return time.Duration(sec) * time.Second
}

// normRetries 重试归一：<=0 → 1（契约钉死，配置不可关成 0 次）。
func normRetries(n int) int {
	if n <= 0 {
		return 1
	}
	return n
}

// ModuleParams 聚合依赖（fx.In，照 starfuyu）。
type ModuleParams struct {
	fx.In

	Cfg       *config.Config
	AppCtrl   *app.AppController
	AdminCtrl *admin.AdminController
}

type taokoulingModule struct {
	cfg       *config.Config
	AppCtrl   *app.AppController
	AdminCtrl *admin.AdminController
}

// NewModule 聚合控制器，产出 server.Module（挂进 group:"modules"）。
func NewModule(p ModuleParams) server.Module {
	return &taokoulingModule{
		cfg:       p.Cfg,
		AppCtrl:   p.AppCtrl,
		AdminCtrl: p.AdminCtrl,
	}
}

// RegisterRoutes 挂载 taokouling 路由。入参是 /api group（见 server/http.go），
// 故 admin 用 /admin/taokouling、app 用 /app/taokouling。
// config 开关：modules.taokouling: false 时跳过注册——判断置于所有 group 创建之前。
func (m *taokoulingModule) RegisterRoutes(r *gin.RouterGroup) {
	if m.cfg != nil {
		if on, ok := m.cfg.Modules["taokouling"]; ok && !on {
			return
		}
	}
	// admin 段（casbin 按 permissionRoutes /api/admin/taokouling/* 前缀鉴权，handler 不取 claims）。
	adminG := r.Group("/admin/taokouling")
	adminG.GET("/channels", m.AdminCtrl.ListChannels)
	adminG.POST("/channels", m.AdminCtrl.CreateChannel)
	adminG.PUT("/channels/:id", m.AdminCtrl.UpdateChannel)
	adminG.DELETE("/channels/:id", m.AdminCtrl.DeleteChannel)
	adminG.GET("/logs", m.AdminCtrl.ListLogs)

	// C 端双端点（公开，无 Bearer；限流前置在 handler 内，spec §4/§6）。
	appGroup := r.Group("/app/taokouling")
	appGroup.POST("/parse", m.AppCtrl.Parse)
	appGroup.POST("/convert", m.AppCtrl.Convert)
}
