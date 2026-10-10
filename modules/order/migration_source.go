package order

import "alexGo-cloud/pkg/migrate"

// migrationSource 把 order 模块的 SQL 迁移目录注册进全局迁移器（fx group:"migration_sources"）。
type migrationSource struct{}

// NewMigrationSource 返回 order 模块的迁移源。
func NewMigrationSource() migrate.Source { return &migrationSource{} }

// GetMigrationPath 迁移 SQL 所在目录（相对仓库根）。
func (s *migrationSource) GetMigrationPath() string { return "modules/order/migrations" }

// GetModuleName 模块名（决定 schema_migrations_<name> 记录表）。
func (s *migrationSource) GetModuleName() string { return "order" }
