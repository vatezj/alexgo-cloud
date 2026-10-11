package oauth2

import (
	"go.uber.org/fx"

	"alexGo-cloud/pkg/migrate"
)

// FxModule oauth2 模块注册（D7：硬编码进两个 main.go）。
// 照抄 modules/infra/module.go：迁移源注解进 group:"migration_sources"（ResultTags）。
var FxModule = fx.Module("oauth2",
	fx.Provide(
		fx.Annotate(
			NewMigrationSource,
			fx.As(new(migrate.Source)),
			fx.ResultTags(`group:"migration_sources"`),
		),
	),
)
