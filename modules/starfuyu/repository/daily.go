package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"alexGo-cloud/modules/starfuyu/model"
)

// DailyRepository 当日全局星象/总述（图片版软文 §3.1；每日一条，内容表恒 tid=0）。
type DailyRepository interface {
	// GetByDate 当日行；miss → (nil, false, nil)。
	GetByDate(ctx context.Context, date string) (*model.DailyHoroscope, bool, error)
	// Upsert (tenant_id,date) 冲突更新 astro/intro；写路径显式置 tid=0。
	Upsert(ctx context.Context, row *model.DailyHoroscope) error
}

type dailyRepo struct{ db *gorm.DB }

func NewDailyRepository(db *gorm.DB) DailyRepository { return &dailyRepo{db: db} }

func (r *dailyRepo) GetByDate(ctx context.Context, date string) (*model.DailyHoroscope, bool, error) {
	var out model.DailyHoroscope
	err := r.db.WithContext(ctx).
		Where("date = ?", date).
		First(&out).Error
	if err == gorm.ErrRecordNotFound {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &out, true, nil
}

func (r *dailyRepo) Upsert(ctx context.Context, row *model.DailyHoroscope) error {
	row.TenantID = 0 // 内容表恒 0
	row.ID = 0       // 冲突目标 (tenant_id,date)，清主键防 PK 抢先命中
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "tenant_id"}, {Name: "date"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"astro", "intro", "updated_at",
		}),
	}).Create(row).Error
}
