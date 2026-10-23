package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"alexGo-cloud/modules/fishgame/model"
	"alexGo-cloud/pkg/repo"
)

// TrackEventRepository 埋点事实流：追加型 + dedup_key 唯一防重发。
type TrackEventRepository interface {
	// BatchInsertIgnoreDup 逐条 OnConflict DoNothing；返回实际插入的下标（与入参对齐，
	// 供 catch→dex 推导只对首次插入递增，防重发膨胀）。
	BatchInsertIgnoreDup(ctx context.Context, events []*model.TrackEvent) ([]int, error)
	ListAdmin(ctx context.Context, name string, playerID uint64, startT, endT int64, page, size int) ([]*model.TrackEvent, int64, error)
}

type trackEventRepo struct{ db *gorm.DB }

func NewTrackEventRepository(db *gorm.DB) TrackEventRepository { return &trackEventRepo{db: db} }

func (r *trackEventRepo) BatchInsertIgnoreDup(ctx context.Context, events []*model.TrackEvent) ([]int, error) {
	inserted := make([]int, 0, len(events))
	for i, e := range events {
		if e.TenantID == 0 {
			e.TenantID = 0
		}
		res := r.db.WithContext(ctx).
			Clauses(clause.OnConflict{DoNothing: true}).
			Create(e)
		if res.Error != nil {
			return inserted, res.Error
		}
		if res.RowsAffected > 0 {
			inserted = append(inserted, i)
		}
	}
	return inserted, nil
}

func (r *trackEventRepo) ListAdmin(ctx context.Context, name string, playerID uint64, startT, endT int64, page, size int) ([]*model.TrackEvent, int64, error) {
	q := r.db.Model(&model.TrackEvent{}).Order("id desc")
	if name != "" {
		q = q.Where("name = ?", name)
	}
	if playerID > 0 {
		q = q.Where("player_id = ?", playerID)
	}
	if startT > 0 {
		q = q.Where("t >= ?", startT)
	}
	if endT > 0 {
		q = q.Where("t <= ?", endT)
	}
	return repo.Paginate[*model.TrackEvent](ctx, q, page, size)
}
