package oauth2

// NewMigrationSource 注册 oauth2 迁移源（runner 按 GetModuleName 建 schema_migrations_oauth2 记录表）。
func NewMigrationSource() *MigrationSource { return &MigrationSource{} }

type MigrationSource struct{}

func (s *MigrationSource) GetMigrationPath() string { return "modules/oauth2/migrations" }
func (s *MigrationSource) GetModuleName() string    { return "oauth2" }
