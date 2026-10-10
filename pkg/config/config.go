package config

// Config 是整个应用的统一配置对象。
//
// 读取来源（按优先级覆盖）：
// 1) 环境变量（viper.AutomaticEnv + BindEnv）
// 2) alexgo-server/configs/config.yaml
// 3) 各模块 modules/<module>/configs/config.yaml 以命名空间合并（例如 system.jwt_secret → system.jwt_secret）
//
// 设计原则：
// - 配置项可按功能域分组（server/database/mq/outbox/...），便于 Helm values 与 GitOps 管理。
// - “可选依赖”通过 Enabled 开关控制：关闭时相关组件返回 nil 并在中间件/服务里做 graceful no-op。
type Config struct {
	// Server：服务端口等监听信息。
	Server struct {
		// HTTPAddr：HTTP 监听地址，示例 ":8080"。
		HTTPAddr string `mapstructure:"http_addr"`
		// GRPCAddr：gRPC 监听地址（用于模块拆分后的独立 gRPC server），示例 ":50051"。
		GRPCAddr string `mapstructure:"grpc_addr"`
		// PprofEnabled：是否挂载 /debug/pprof/**。
		// 生产默认关闭：pprof 端点会泄露命令行参数、堆栈与运行时信息，应仅在本地/内网调试时开启。
		PprofEnabled bool `mapstructure:"pprof_enabled"`
	} `mapstructure:"server"`

	// Database：数据库连接信息与连接池参数。
	Database struct {
		// DSN：GORM MySQL DSN。
		DSN string `mapstructure:"dsn"`
		// MaxOpenConns / MaxIdleConns / ConnMaxLifetimeSecond：连接池参数（不配置则使用代码默认值）。
		MaxOpenConns          int `mapstructure:"max_open_conns"`
		MaxIdleConns          int `mapstructure:"max_idle_conns"`
		ConnMaxLifetimeSecond int `mapstructure:"conn_max_lifetime_second"`
	} `mapstructure:"database"`

	// Modules：模块开关。单体模式下“引入即启动”，这里用开关决定是否注册模块路由。
	Modules map[string]bool `mapstructure:"modules"`

	// Migrate：是否在启动时自动执行数据库迁移。
	Migrate struct {
		Auto bool `mapstructure:"auto"`
	} `mapstructure:"migrate"`

	// Microservice：微服务演进开关。
	// enabled=true 时，某些接口会被 Decorate 为 gRPC 客户端实现。
	Microservice struct {
		Enabled  bool   `mapstructure:"enabled"`
		Registry string `mapstructure:"registry"`
	} `mapstructure:"microservice"`

	// Redis：Redis 连接信息（限流等功能依赖）。
	Redis struct {
		Enabled  bool   `mapstructure:"enabled"`
		Addr     string `mapstructure:"addr"`
		Password string `mapstructure:"password"`
		DB       int    `mapstructure:"db"`
	} `mapstructure:"redis"`

	// MQ：消息系统配置（当前实现 NATS JetStream）。
	MQ struct {
		NATS struct {
			Enabled bool   `mapstructure:"enabled"`
			URL     string `mapstructure:"url"`
			Stream  string `mapstructure:"stream"`
			// SubjectPrefix：stream 订阅的 subject pattern，例如 "alexgo.>"。
			SubjectPrefix string `mapstructure:"subject_prefix"`
		} `mapstructure:"nats"`
	} `mapstructure:"mq"`

	// Outbox：Transactional Outbox Relay 配置。
	Outbox struct {
		Enabled        bool `mapstructure:"enabled"`
		BatchSize      int  `mapstructure:"batch_size"`
		IntervalSecond int  `mapstructure:"interval_second"`
	} `mapstructure:"outbox"`

	// Limiter：入口限流（Token Bucket）配置。
	Limiter struct {
		Enabled bool `mapstructure:"enabled"`
		Rate    int  `mapstructure:"rate"`
		Burst   int  `mapstructure:"burst"`
	} `mapstructure:"limiter"`

	// Breaker：熔断器配置。
	Breaker struct {
		Enabled    bool `mapstructure:"enabled"`
		Threshold  int  `mapstructure:"threshold"`
		OpenSecond int  `mapstructure:"open_second"`
	} `mapstructure:"breaker"`

	// System：system 模块配置（示例：JWT secret、默认角色等）。
	System struct {
		JWTSecret        string `mapstructure:"jwt_secret"`
		DefaultAdminRole string `mapstructure:"default_admin_role"`
		DefaultAdminUsername string `mapstructure:"default_admin_username"`
		DefaultAdminPassword string `mapstructure:"default_admin_password"`
	} `mapstructure:"system"`

	// Auth：OAuth2 令牌与认证模式。
	Auth struct {
		Mode            string `mapstructure:"mode"` // token | jwt（回滚开关）
		AccessExpireHour int    `mapstructure:"access_expire_hour"`
		RefreshExpireDay int    `mapstructure:"refresh_expire_day"`
	} `mapstructure:"auth"`

	// Deployment：mono=单体一键启动（默认）；micro=双服务部署形态。
	Deployment struct {
		Mode string `mapstructure:"mode"`
	} `mapstructure:"deployment"`

	// SystemGRPCAddr：member-server → system-server TokenService 的地址。
	SystemGRPCAddr string `mapstructure:"system_grpc_addr"`

	// Codegen：代码生成器（modules/infra + tools/codegen）配置。
	Codegen struct {
		// UnitTestEnable：生成代码时是否附带单元测试骨架（默认开启）。
		UnitTestEnable bool `mapstructure:"unit_test_enable"`
	} `mapstructure:"codegen"`
}
