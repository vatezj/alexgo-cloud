package database

import (
	"context"
	"fmt"
	"time"

	"alexGo-cloud/pkg/config"

	"go.uber.org/fx"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type DBParams struct {
	fx.In

	LC  fx.Lifecycle
	Cfg *config.Config
}

func NewDB(p DBParams) (*gorm.DB, error) {
	if p.Cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if p.Cfg.Database.DSN == "" {
		return nil, fmt.Errorf("database.dsn is empty")
	}

	db, err := gorm.Open(mysql.Open(p.Cfg.Database.DSN), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	maxOpen := p.Cfg.Database.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 100
	}
	maxIdle := p.Cfg.Database.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = 10
	}
	lifetimeSec := p.Cfg.Database.ConnMaxLifetimeSecond
	if lifetimeSec <= 0 {
		lifetimeSec = 3600
	}

	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(time.Duration(lifetimeSec) * time.Second)

	p.LC.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return sqlDB.Close()
		},
	})

	return db, nil
}
