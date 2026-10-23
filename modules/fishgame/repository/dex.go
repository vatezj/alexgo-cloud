package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"alexGo-cloud/modules/fishgame/model"
)

// DexRepository 图鉴进度：双路写（catch 单加一 / report max 合并）。
type DexRepository interface {
	// UpsertCatch 钓获一次：首次插 count=1 返回 true；已有行 count=count+1 返回 false。
	UpsertCatch(ctx context.Context, playerID uint64, fishID string) (firstInsert bool, err error)
	// MergeReport max 合并：逐条 count=GREATEST(旧, 上报)；未知 fish_id 由调用方过滤，repo 层只 upsert。
	MergeReport(ctx context.Context, playerID uint64, counts map[string]int) error
	CountByPlayer(ctx context.Context, playerID uint64) (int64, error) // 点亮数 = 行数 where count≥1
	ListByPlayer(ctx context.Context, playerID uint64) ([]*model.Dex, error)
}

type dexRepo struct{ db *gorm.DB }

func NewDexRepository(db *gorm.DB) DexRepository { return &dexRepo{db: db} }

func (r *dexRepo) UpsertCatch(ctx context.Context, playerID uint64, fishID string) (bool, error) {
	first := true
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var d model.Dex
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("player_id = ? AND fish_id = ?", playerID, fishID).
			First(&d).Error
		if err == nil {
			first = false
			return tx.Model(&model.Dex{}).Where("id = ?", d.ID).
				UpdateColumn("count", gorm.Expr("count + 1")).Error
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}
		return tx.Create(&model.Dex{TenantID: 0, PlayerID: playerID, FishID: fishID, Count: 1}).Error
	})
	return first, err
}

func (r *dexRepo) MergeReport(ctx context.Context, playerID uint64, counts map[string]int) error {
	if len(counts) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for fishID, n := range counts {
			if n <= 0 {
				continue
			}
			var d model.Dex
			err := tx.Where("player_id = ? AND fish_id = ?", playerID, fishID).First(&d).Error
			if err == gorm.ErrRecordNotFound {
				if err := tx.Create(&model.Dex{TenantID: 0, PlayerID: playerID, FishID: fishID, Count: n}).Error; err != nil {
					return err
				}
				continue
			}
			if err != nil {
				return err
			}
			if n > d.Count {
				if err := tx.Model(&model.Dex{}).Where("id = ?", d.ID).
					UpdateColumn("count", n).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (r *dexRepo) CountByPlayer(ctx context.Context, playerID uint64) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.Dex{}).
		Where("player_id = ? AND count >= 1", playerID).Count(&n).Error
	return n, err
}

func (r *dexRepo) ListByPlayer(ctx context.Context, playerID uint64) ([]*model.Dex, error) {
	var list []*model.Dex
	err := r.db.WithContext(ctx).Where("player_id = ?", playerID).Order("fish_id asc").Find(&list).Error
	return list, err
}
