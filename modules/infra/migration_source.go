package infra

import "alexGo-cloud/pkg/migrate"

type migrationSource struct{}

func NewMigrationSource() migrate.Source {
	return &migrationSource{}
}

// GetMigrationPath 迁移 SQL 所在目录（相对仓库根，Runner 以 file:// 打开）。
func (s *migrationSource) GetMigrationPath() string {
	return "modules/infra/migrations"
}

// GetModuleName 决定 schema_migrations_<name> 记录表。
func (s *migrationSource) GetModuleName() string {
	return "infra"
}
