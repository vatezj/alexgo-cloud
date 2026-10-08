package migrate

import (
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
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
	if r.cfg != nil && !r.cfg.Migrate.Auto {
		return nil
	}
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}

	for _, src := range r.sources {
		driver, err := mysql.WithInstance(sqlDB, &mysql.Config{
			MigrationsTable: "schema_migrations_" + src.GetModuleName(),
		})
		if err != nil {
			return err
		}
		m, err := migrate.NewWithDatabaseInstance(
			"file://"+src.GetMigrationPath(),
			"mysql",
			driver,
		)
		if err != nil {
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
