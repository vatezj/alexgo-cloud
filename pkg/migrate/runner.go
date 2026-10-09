package migrate

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/logger"
)

type Source interface {
	GetMigrationPath() string
	GetModuleName() string
}

type Runner struct {
	db      *gorm.DB
	cfg     *config.Config
	sources []Source
}

func NewRunner(db *gorm.DB, cfg *config.Config, sources []Source) *Runner {
	return &Runner{db: db, cfg: cfg, sources: sources}
}

func (r *Runner) Run() error {
	if r.db == nil {
		return fmt.Errorf("db is nil")
	}
	if r.cfg == nil {
		return fmt.Errorf("cfg is nil")
	}
	if !r.cfg.Migrate.Auto {
		return nil
	}

	for _, src := range r.sources {
		// 每个 source 使用独立连接池，原因有二：
		// 1. golang-migrate 的 mysql driver 要求连接串带 multiStatements=true
		//    （见 driver 文档），否则多语句迁移文件整批下发会报 ERROR 1064；
		//    该参数只加在迁移专用池上，业务池（gorm）保持原样。
		// 2. m.Close() 会关闭 driver 持有的 *sql.DB；若复用 gorm 的共享池，
		//    第一个 source 迁移完共享池就被关掉，后续 source 直接报
		//    "sql: database is closed"，启动失败。
		migDB, err := sql.Open("mysql", withMultiStatements(r.cfg.Database.DSN))
		if err != nil {
			return fmt.Errorf("open migration db: %w", err)
		}
		driver, err := mysql.WithInstance(migDB, &mysql.Config{
			MigrationsTable: "schema_migrations_" + src.GetModuleName(),
		})
		if err != nil {
			_ = migDB.Close()
			return err
		}
		m, err := migrate.NewWithDatabaseInstance(
			"file://"+src.GetMigrationPath(),
			"mysql",
			driver,
		)
		if err != nil {
			_ = migDB.Close()
			if logger.Log != nil {
				logger.Log.Error("migrate init failed", zap.String("module", src.GetModuleName()), zap.Error(err))
			}
			continue
		}

		upErr := m.Up()
		if upErr != nil && upErr != migrate.ErrNoChange {
			if logger.Log != nil {
				logger.Log.Error("migrate up failed", zap.String("module", src.GetModuleName()), zap.Error(upErr))
			}
		} else {
			if logger.Log != nil {
				logger.Log.Info("migrate success", zap.String("module", src.GetModuleName()))
			}
		}

		_, _ = m.Close()
	}

	return nil
}

// withMultiStatements 幂等地给迁移 DSN 追加 multiStatements=true。
func withMultiStatements(dsn string) string {
	if strings.Contains(dsn, "multiStatements=") {
		return dsn
	}
	if strings.Contains(dsn, "?") {
		return dsn + "&multiStatements=true"
	}
	return dsn + "?multiStatements=true"
}
