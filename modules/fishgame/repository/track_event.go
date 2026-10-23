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
	// BatchInsertIgnoreDup ON DUPLICATE KEY IGNORE（dedup_key 冲突跳过），返回实际插入条数。
	BatchInsertIgnoreDup(ctx context.Context, events []*model.TrackEvent) (int, error)
	ListAdmin(ctx context.Context, name string, playerID uint64, startT, endT int64, page, size int) ([]*model.TrackEvent, int64, error)
}

type trackEventRepo struct{ db *gorm.DB }

func NewTrackEventRepository(db *gorm.DB) TrackEventRepository { return &trackEventRepo{db: db} }

func (r *trackEventRepo) BatchInsertIgnoreDup(ctx context.Context, events []*model.TrackEvent) (int, error) {
	if len(events) == 0 {
		return 0, nil
	}
	for _, e := range events {
		if e.TenantID == 0 {
			e.TenantID = 0
		}
	}
	before := int64(0)
	if err := r.db.WithContext(ctx).Model(&model.TrackEvent{}).Count(&before).Error; err != nil {
		return 0, err
	}
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(events).Error
	if err != nil {
		return 0, err
	}
	var after int64
	if err := r.db.WithContext(ctx).Model(&model.TrackEvent{}).Count(&after).Error; err != nil {
		return 0, err
	}
	return int(after - before), nil
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
