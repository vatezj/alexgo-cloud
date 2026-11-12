package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"alexGo-cloud/modules/starfuyu/model"
	"alexGo-cloud/pkg/repo"
)

// FortuneRepository 每日星座运势内容（brief 钉死签名，Task 4 服务层唯一数据入口）。
// 内容表：读写钉 tenant_id=0（决定 6）——写路径显式置 0；读路径不猜租户，
// 由 service 层 tenant.WithTenantID(ctx, 0) 包裹（Task 4）。
type FortuneRepository interface {
	// GetByDate 当日全部运势；调用方保证 ctx tid=0。无数据返回空列表。
	GetByDate(ctx context.Context, date string) ([]*model.Fortune, error)
	// CountByDate 当日行数（抓取补缺判定：==12 已满）。
	CountByDate(ctx context.Context, date string) (int64, error)
	// GetOne 单星座运势；miss → (nil, nil)。
	GetOne(ctx context.Context, date, constellation string) (*model.Fortune, error)
	// UpsertBatch 整批落库：(tenant_id,date,constellation) 冲突更新全部业务列，失败不落。
	UpsertBatch(ctx context.Context, rows []*model.Fortune) error
	// ListAdmin 管理端分页：date/constellation 非空才过滤。
	ListAdmin(ctx context.Context, date, constellation string, page, size int) ([]*model.Fortune, int64, error)
}

type fortuneRepo struct{ db *gorm.DB }

func NewFortuneRepository(db *gorm.DB) FortuneRepository { return &fortuneRepo{db: db} }

func (r *fortuneRepo) GetByDate(ctx context.Context, date string) ([]*model.Fortune, error) {
	var list []*model.Fortune
	err := r.db.WithContext(ctx).
		Where("date = ?", date).
		Order("id asc").
		Find(&list).Error
	return list, err
}

func (r *fortuneRepo) CountByDate(ctx context.Context, date string) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.Fortune{}).
		Where("date = ?", date).
		Count(&n).Error
	return n, err
}

func (r *fortuneRepo) GetOne(ctx context.Context, date, constellation string) (*model.Fortune, error) {
	var out model.Fortune
	err := r.db.WithContext(ctx).
		Where("date = ? AND constellation = ?", date, constellation).
		First(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *fortuneRepo) UpsertBatch(ctx context.Context, rows []*model.Fortune) error {
	if len(rows) == 0 {
		return nil
	}
	for _, row := range rows {
		row.TenantID = 0 // 内容表恒 0（决定 6）
		row.ID = 0       // 冲突目标是 (tenant_id,date,constellation)，清主键防 PK 抢先命中
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "tenant_id"}, {Name: "date"}, {Name: "constellation"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"daily_overview", "overall_score", "love_score", "career_score",
			"wealth_score", "health_score", "lucky_color", "lucky_number",
			"summary", "love_tip", "career_tip", "wealth_tip", "health_tip", "updated_at",
		}),
	}).Create(rows).Error
}

func (r *fortuneRepo) ListAdmin(ctx context.Context, date, constellation string, page, size int) ([]*model.Fortune, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.Fortune{}).Order("date desc, constellation asc")
	if date != "" {
		q = q.Where("date = ?", date)
	}
	if constellation != "" {
		q = q.Where("constellation = ?", constellation)
	}
	return repo.Paginate[*model.Fortune](ctx, q, page, size)
}
