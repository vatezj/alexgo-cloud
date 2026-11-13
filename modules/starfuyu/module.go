package starfuyu

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"alexGo-cloud/alexgo-server/server"
	"alexGo-cloud/modules/starfuyu/controller/admin"
	"alexGo-cloud/modules/starfuyu/controller/app"
	"alexGo-cloud/modules/starfuyu/repository"
	"alexGo-cloud/modules/starfuyu/service"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/migrate"
)

// FxModule starfuyu 模块的 fx 装配（newmodule 生成，业务逻辑请手工扩展）。
// Task 3 级联：四实体（Profile/Bazi/Divination/Fortune）的脚手架 service/admin
// 控制器已随仓储覆盖删除，Task 4+ 重建；Task 5 级联：limit 脚手架全链
// （admin CRUD 控制器 / module_register / LimitRepository）删除，服务层换为
// RequestLimit 限流三服务，admin 面由 Task 11 重建；app Ping 控制器保留。
var FxModule = fx.Module("starfuyu",
	fx.Provide(
		repository.NewProfileRepository,
		app.NewAppProfileController,
		repository.NewBaziRepository,
		app.NewAppBaziController,
		repository.NewDivinationRepository,
		app.NewAppDivinationController,
		repository.NewFortuneRepository,
		service.NewDeepSeekFetcher, // FortuneFetcher（Task 4 抓取器，Task 10 控制器消费）
		service.NewFortuneService,  // FortuneService（Task 4）
		app.NewAppFortuneController,
		repository.NewRequestLimitRepository, // Task 3 deferred Provide，Task 5 补
		service.NewLimitService,              // LimitService.Allow（Task 5 每日限流）
		service.NewProfileService,            // ProfileService（Task 5）
		service.NewBaziService,               // BaziService（Task 5）
		service.NewDeepSeekBaziCalculator,    // BaziCalculator（Task 5 DeepSeek 实现）
		repository.NewSignCacheRepository,    // Task 3 deferred Provide，Task 6 补
		service.NewDivinationService,         // DivinationService（Task 6 签文缓存 + 求签/历史）
		service.NewDeepSeekSignInterpreter,   // SignInterpreter（Task 6 DeepSeek 实现）
		service.NewImageService,              // ImageService（Task 7 运势生图，storage.Driver 由 system 模块提供）
		// Task 8：公众号文章 + 微信 MP 客户端（appid/secret 从 cfg 注入；driver 读封面字节）。
		func(cfg *config.Config) service.MPClient {
			return service.NewWeChatMPClient(cfg.Starfuyu.WechatMp.AppID, cfg.Starfuyu.WechatMp.AppSecret)
		},
		service.NewArticleService, // ArticleService（Task 8 草稿箱上传）
		app.NewAppLimitController,
		service.NewTaskRunner, // TaskRunner（Task 9 定时任务编排）
		// Task 10：C 端 13 端点合一控制器（Bearer member 自鉴权，spec §4.1）。
		// token.Validator 的接口映射归入口（main.go），模块内不重复 Provide。
		app.NewAppController,
		// Task 11：admin 8 端点（casbin 全局中间件鉴权，handler 不取 claims）
		// + 公开图片读（storage.Driver 由 system 模块提供）。
		admin.NewAdminController,
		admin.NewPublicController,
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
	// 定时任务（Task 9）：OnStart 起 robfig/cron（5 段标准 spec + CRON_TZ=Asia/Shanghai），
	// OnStop c.Stop() 等在跑任务收尾——细节见 cron.go。
	fx.Invoke(cronLifecycle),
)

// ModuleParams 聚合依赖（fx.In）。
type ModuleParams struct {
	fx.In

	Cfg               *config.Config
	ProfileAppCtrl    *app.AppProfileController
	BaziAppCtrl       *app.AppBaziController
	DivinationAppCtrl *app.AppDivinationController
	FortuneAppCtrl    *app.AppFortuneController
	LimitAppCtrl      *app.AppLimitController
	AppCtrl           *app.AppController      // Task 10：C 端 13 端点
	AdminCtrl         *admin.AdminController  // Task 11：admin 8 端点（spec §4.2）
	PublicCtrl        *admin.PublicController // Task 11：公开图片读（spec §4.3）
}

type starfuyuModule struct {
	cfg               *config.Config
	ProfileAppCtrl    *app.AppProfileController
	BaziAppCtrl       *app.AppBaziController
	DivinationAppCtrl *app.AppDivinationController
	FortuneAppCtrl    *app.AppFortuneController
	LimitAppCtrl      *app.AppLimitController
	AppCtrl           *app.AppController
	AdminCtrl         *admin.AdminController
	PublicCtrl        *admin.PublicController
}

// NewModule 聚合控制器，产出 server.Module（挂进 group:"modules"）。
func NewModule(p ModuleParams) server.Module {
	return &starfuyuModule{
		cfg:               p.Cfg,
		ProfileAppCtrl:    p.ProfileAppCtrl,
		BaziAppCtrl:       p.BaziAppCtrl,
		DivinationAppCtrl: p.DivinationAppCtrl,
		FortuneAppCtrl:    p.FortuneAppCtrl,
		LimitAppCtrl:      p.LimitAppCtrl,
		AppCtrl:           p.AppCtrl,
		AdminCtrl:         p.AdminCtrl,
		PublicCtrl:        p.PublicCtrl,
	}
}

// RegisterRoutes 挂载 starfuyu 三段路由。入参是 /api group（见 server/http.go），
// 故 admin 用 /admin/starfuyu、app 用 /app/starfuyu（member 基准双前缀）、
// 公开读用 /public/starfuyu（免鉴权）。
// config 开关：modules.starfuyu: false 时跳过注册——判断置于所有 group 创建之前。
func (m *starfuyuModule) RegisterRoutes(r *gin.RouterGroup) {
	if m.cfg != nil {
		if on, ok := m.cfg.Modules["starfuyu"]; ok && !on {
			return
		}
	}
	// 公开读（免鉴权，spec §4.3）：入参是 /api group → /api/public/starfuyu/image/*key。
	// *key 通配：storage key（fortune/v2/{date}/{c}.jpg）含斜杠，:key 单段截不住。
	r.GET("/public/starfuyu/image/*key", m.PublicCtrl.Image)

	// admin 段（spec §4.2）：统一信封 + casbin 按 permissionRoutes 前缀鉴权
	// （starfuyu:fortune / starfuyu:task，Task 1 已登记；handler 不取 claims）。
	adminG := r.Group("/admin/starfuyu")
	adminG.GET("/fortunes", m.AdminCtrl.ListFortunes)
	adminG.GET("/fortunes/:date", m.AdminCtrl.FortuneByDate)
	adminG.GET("/fortunes/:date/article", m.AdminCtrl.FortuneArticle)
	adminG.POST("/fortunes/:date/upload-mp", m.AdminCtrl.UploadMP)
	adminG.POST("/tasks/fetch-fortune", m.AdminCtrl.TaskFetchFortune)
	adminG.POST("/tasks/generate-images", m.AdminCtrl.TaskGenerateImages)
	adminG.POST("/tasks/execute-all", m.AdminCtrl.TaskExecuteAll)
	adminG.GET("/limits", m.AdminCtrl.Limits)

	appGroup := r.Group("/app/starfuyu")
	appGroup.GET("/profiles/ping", m.ProfileAppCtrl.Ping)
	appGroup.GET("/bazis/ping", m.BaziAppCtrl.Ping)
	appGroup.GET("/divinations/ping", m.DivinationAppCtrl.Ping)
	appGroup.GET("/fortunes/ping", m.FortuneAppCtrl.Ping)
	appGroup.GET("/limits/ping", m.LimitAppCtrl.Ping)
	// C 端业务 13 端点（Task 10，spec §4.1）。静态段（/fortune/today、/fortune/my/*、
	// /fortune/image/*）先于 /fortune/:date 注册——gin 静态优先，照 goLucky 原路由。
	// /divination/sign/:number 是唯一公开端点：handler 内不挂 claims。
	appGroup.GET("/fortune/today", m.AppCtrl.FortuneToday)
	appGroup.GET("/fortune/:date", m.AppCtrl.FortuneByDate)
	appGroup.GET("/fortune/my/today", m.AppCtrl.FortuneMyToday)
	appGroup.GET("/fortune/my/:date", m.AppCtrl.FortuneMyByDate)
	appGroup.GET("/fortune/image/:constellation/:date", m.AppCtrl.FortuneImage)
	appGroup.POST("/bazi/calculate", m.AppCtrl.BaziCalculate)
	appGroup.GET("/bazi/my", m.AppCtrl.BaziMy)
	appGroup.PUT("/bazi/my", m.AppCtrl.BaziUpdateMy)
	appGroup.GET("/divination/sign/:number", m.AppCtrl.DivinationSign)
	appGroup.POST("/divination/draw", m.AppCtrl.DivinationDraw)
	appGroup.GET("/divination/history", m.AppCtrl.DivinationHistory)
	appGroup.GET("/profile", m.AppCtrl.ProfileGet)
	appGroup.PUT("/profile", m.AppCtrl.ProfilePut)
	// 跨模块接口映射（如 token.Issuer）归入口 main.go，模块内不重复 Provide——
	// 需要时在此上方参照 modules/member/module.go 补 fx.Provide 映射。
}
