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
	// ListNameTSince 窗口内 name+t 投影（看板聚合，spec §4.9）。
	ListNameTSince(ctx context.Context, startT int64) ([]NameT, error)
	// ExistsFishRef props 引用 fish_id（删除前引用检查 → 30402）。
	ExistsFishRef(ctx context.Context, fishID string) (bool, error)
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

// NameT 看板聚合的最小投影（name,t 列，spec §4.9 近 7 日窗口）。
type NameT struct {
	Name string
	T    int64
}

// ListNameTSince startT 起（含）所有事件的 name+t 投影，按 t 升序。
// 窗口聚合（按 name 汇总 / 按日分桶）由 service 在内存完成——7 日窗口有界，
// 且本地日期分桶在 mysql（生产）与 sqlite（测试）间无方言差异。
func (r *trackEventRepo) ListNameTSince(ctx context.Context, startT int64) ([]NameT, error) {
	var rows []NameT
	err := r.db.WithContext(ctx).Model(&model.TrackEvent{}).
		Select("name, t").Where("t >= ?", startT).Order("t asc").Scan(&rows).Error
	return rows, err
}

// ExistsFishRef 事件 props 是否引用指定 fish_id（删除前引用检查，spec §4.2 → 30402）。
// canonical props 形如 {"fish_id":"inkfish",...}，按字面片段匹配；宁可误报不可漏报。
func (r *trackEventRepo) ExistsFishRef(ctx context.Context, fishID string) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.TrackEvent{}).
		Where("props LIKE ?", `%"fish_id":"`+fishID+`"%`).Count(&n).Error
	return n > 0, err
}
