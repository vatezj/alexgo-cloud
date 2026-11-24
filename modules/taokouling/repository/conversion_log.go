package repository

import (
	"context"
	"time"

	"alexGo-cloud/modules/taokouling/model"
	"alexGo-cloud/pkg/tenant"
	"gorm.io/gorm"
)

// ConversionLogRepository 转换记录仓储（平台级数据，repo 自钉 tid=0，R1b）。
type ConversionLogRepository struct {
	db *gorm.DB
}

// NewConversionLogRepository 构造转换记录仓储。
func NewConversionLogRepository(db *gorm.DB) *ConversionLogRepository {
	return &ConversionLogRepository{db: db}
}

// LogFilter 转换记录筛选：Status=-1 不过滤；时间零值不过滤；空串不过滤。
type LogFilter struct {
	Start, End      time.Time
	Partner, ItemID string
	Status          int
	Page, Size      int
}

// Append 追加转换记录（强制 rec.TenantID=0）。
func (r *ConversionLogRepository) Append(ctx context.Context, rec *model.ConversionLog) error {
	ctx = tenant.WithTenantID(ctx, 0) // 平台级数据，repo 自钉 0
	rec.TenantID = 0
	return r.db.WithContext(ctx).Create(rec).Error
}

// List 按 LogFilter 分页列出（id DESC，total 独立计数）。
func (r *ConversionLogRepository) List(ctx context.Context, f LogFilter) ([]model.ConversionLog, int64, error) {
	ctx = tenant.WithTenantID(ctx, 0) // 平台级数据，repo 自钉 0
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Size < 1 {
		f.Size = 10
	}
	where := func(db *gorm.DB) *gorm.DB {
		db = db.Where("tenant_id = ?", 0)
		if f.Status != -1 {
			db = db.Where("status = ?", f.Status)
		}
		if f.Partner != "" {
			db = db.Where("partner = ?", f.Partner)
		}
		if f.ItemID != "" {
			db = db.Where("item_id = ?", f.ItemID)
		}
		if !f.Start.IsZero() {
			db = db.Where("created_at >= ?", f.Start)
		}
		if !f.End.IsZero() {
			db = db.Where("created_at <= ?", f.End)
		}
		return db
	}
	var total int64
	if err := where(r.db.WithContext(ctx).Model(&model.ConversionLog{})).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []model.ConversionLog
	err := where(r.db.WithContext(ctx).Model(&model.ConversionLog{})).
		Order("id DESC").Offset((f.Page - 1) * f.Size).Limit(f.Size).Find(&list).Error
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
