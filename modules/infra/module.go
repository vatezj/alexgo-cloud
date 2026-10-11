package infra

import (
	"go.uber.org/fx"

	"alexGo-cloud/pkg/migrate"
)

// FxModule infra 模块的 fx 装配。系统级功能：不做租户隔离（spec §1）。
// M2 先挂迁移源；三层与 admin 路由在 Task 7 追加。
var FxModule = fx.Module("infra",
	fx.Provide(
		fx.Annotate(
			NewMigrationSource,
			fx.As(new(migrate.Source)),
			fx.ResultTags(`group:"migration_sources"`),
		),
	),
)
