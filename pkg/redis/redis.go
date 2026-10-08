package redis

import (
	"context"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"

	"alexGo-cloud/pkg/config"
)

type Params struct {
	fx.In

	LC  fx.Lifecycle
	Cfg *config.Config
}

func NewClient(p Params) (*redis.Client, error) {
	if p.Cfg == nil || !p.Cfg.Redis.Enabled {
		return nil, nil
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     p.Cfg.Redis.Addr,
		Password: p.Cfg.Redis.Password,
		DB:       p.Cfg.Redis.DB,
	})

	p.LC.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return rdb.Close()
		},
	})

	return rdb, nil
}
