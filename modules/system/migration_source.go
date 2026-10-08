package system

import "alexGo-cloud/pkg/migrate"

type migrationSource struct{}

func NewMigrationSource() migrate.Source {
	return &migrationSource{}
}

func (s *migrationSource) GetMigrationPath() string {
	return "modules/system/migrations"
}

func (s *migrationSource) GetModuleName() string {
	return "system"
}
