package infra

import (
	"context"
	"database/sql"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	"go.uber.org/fx"

	"alexGo-cloud/alexgo-server/server"
	"alexGo-cloud/modules/infra/controller/admin"
	"alexGo-cloud/modules/infra/repository"
	"alexGo-cloud/modules/infra/service"
	"alexGo-cloud/pkg/codegen/builder"
	"alexGo-cloud/pkg/codegen/metadata"
	cgmodel "alexGo-cloud/pkg/codegen/model"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/migrate"
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

// newCodegenService 组装业务层，注入统一构建选项。
func newCodegenService(reader metadata.MetadataReader, repo repository.CodegenRepository, opts builder.Options) service.CodegenService {
	return service.NewCodegenService(reader, repo, opts)
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

type infraModule struct {
	codegenCtrl *admin.CodegenController
}

// NewModule 聚合控制器，产出 server.Module（挂进 group:"modules"）。
func NewModule(codegenCtrl *admin.CodegenController) server.Module {
	return &infraModule{codegenCtrl: codegenCtrl}
}

// RegisterRoutes 挂载 infra 路由。入参是 /api group（见 server/http.go），
// 故这里用相对前缀 /admin/infra/codegen。
func (m *infraModule) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/admin/infra/codegen")
	m.codegenCtrl.RegisterRoutes(g)
}
