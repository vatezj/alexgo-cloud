package infra

import (
	"context"
	"database/sql"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	"go.uber.org/fx"

	"alexGo-cloud/alexgo-server/server"
	"alexGo-cloud/modules/system/infra/controller/admin"
	"alexGo-cloud/modules/system/infra/repository"
	"alexGo-cloud/modules/system/infra/service"
	"alexGo-cloud/pkg/codegen/builder"
	"alexGo-cloud/pkg/codegen/metadata"
	cgmodel "alexGo-cloud/pkg/codegen/model"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/migrate"
	"alexGo-cloud/pkg/storage"
)

// FxModule infra 模块装配：迁移源 + 三层 + admin 路由。系统级：不做租户隔离。
var FxModule = fx.Module("infra",
	fx.Provide(
		fx.Annotate(
			NewMigrationSource,
			fx.As(new(migrate.Source)),
			fx.ResultTags(`group:"migration_sources"`),
		),
		newBuilderOptions,
		repository.NewCodegenRepository,
		newMetadataReader,
		newCodegenService,
		admin.NewCodegenController,
		newStorageDriver,
		repository.NewFileRepository,
		newFileService,
		admin.NewFileController,
		fx.Annotate(
			NewModule,
			fx.As(new(server.Module)),
			fx.ResultTags(`group:"modules"`),
		),
	),
)

// newBuilderOptions codegen 构建选项（单值，service 与 controller 共用）。
func newBuilderOptions() builder.Options {
	return builder.Options{
		Module:       "infra",
		TemplateType: cgmodel.TemplateTypeSingle,
		FrontType:    cgmodel.FrontTypeVben5Antd,
	}
}

// newCodegenService 组装业务层，注入统一构建选项与单测骨架开关。
func newCodegenService(
	reader metadata.MetadataReader,
	repo repository.CodegenRepository,
	opts builder.Options,
	cfg *config.Config,
) service.CodegenService {
	unitTest := true
	if cfg != nil {
		unitTest = cfg.Codegen.UnitTestEnable
	}
	return service.NewCodegenService(reader, repo, opts, unitTest)
}

// newMetadataReader 用配置 DSN 构造 MySQL 元数据读取器（information_schema 查询）。
// 连接惰性建立；进程退出时关闭 sql.DB。
func newMetadataReader(lc fx.Lifecycle, cfg *config.Config) (metadata.MetadataReader, error) {
	db, err := sql.Open("mysql", cfg.Database.DSN)
	if err != nil {
		return nil, err
	}
	dsn, err := mysql.ParseDSN(cfg.Database.DSN)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	lc.Append(fx.Hook{OnStop: func(context.Context) error { return db.Close() }})
	return metadata.NewMySQLReader(db, dsn.DBName), nil
}

// newStorageDriver 本地盘驱动：构造即 Abs+MkdirAll。不做 Enabled 开关——
// 本地盘无外部依赖（spec §1，与 Redis/MQ 可选依赖不同），YAGNI。
func newStorageDriver(cfg *config.Config) (storage.Driver, error) {
	root := "./data/uploads"
	if cfg != nil && cfg.Storage.Root != "" {
		root = cfg.Storage.Root
	}
	return storage.NewLocalDriver(root)
}

// newFileService 组装文件服务（配置注入大小上限与白名单覆盖）。
func newFileService(
	repo repository.FileRepository,
	driver storage.Driver,
	cfg *config.Config,
) service.FileService {
	return service.NewFileService(repo, driver, cfg)
}

type infraModule struct {
	codegenCtrl *admin.CodegenController
	fileCtrl    *admin.FileController
}

// NewModule 聚合控制器，产出 server.Module（挂进 group:"modules"）。
func NewModule(codegenCtrl *admin.CodegenController, fileCtrl *admin.FileController) server.Module {
	return &infraModule{codegenCtrl: codegenCtrl, fileCtrl: fileCtrl}
}

// RegisterRoutes 挂载 infra 路由。入参是 /api group（见 server/http.go），
// 故这里用相对前缀：admin 受控组 + public 免鉴权组。
func (m *infraModule) RegisterRoutes(r *gin.RouterGroup) {
	m.codegenCtrl.RegisterRoutes(r.Group("/admin/infra/codegen"))
	m.fileCtrl.RegisterAdminRoutes(r.Group("/admin/infra/file"))
	m.fileCtrl.RegisterPublicRoutes(r.Group("/public/infra/file"))
}
