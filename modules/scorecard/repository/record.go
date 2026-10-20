package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"alexGo-cloud/modules/scorecard/model"
	"alexGo-cloud/pkg/repo"
	"alexGo-cloud/pkg/tenant"
)

// RecordRepository 战绩数据访问。批量 Create 供结算全员一次落库；
// 租户隔离靠 gormplugin。
type RecordRepository interface {
	Create(ctx context.Context, recs []*model.Record) error // 批量（结算全员）
	ListByUser(ctx context.Context, userID uint64, page, size int) ([]*model.Record, int64, error)
	GetByID(ctx context.Context, id uint64) (*model.Record, error) // 软查：不存在/跨租户 → (nil, nil)
	Delete(ctx context.Context, id uint64) error
	ListByRoom(ctx context.Context, roomID uint64) ([]*model.Record, error)
	StatsByUser(ctx context.Context, userID uint64) (total int64, win int64, lose int64, draw int64, err error)
	ListAdmin(ctx context.Context, userID, roomID uint64, page, size int) ([]*model.Record, int64, error) // userID/roomID 0 = 不过滤
}

type recordRepo struct {
	db *gorm.DB
}

func NewRecordRepository(db *gorm.DB) RecordRepository {
	return &recordRepo{db: db}
}

// Create 批量插入。slice 逐行由 gormplugin 回填 tenant_id（fill_create 处理 slice）。
func (r *recordRepo) Create(ctx context.Context, recs []*model.Record) error {
	if len(recs) == 0 {
		return nil
	}
	for _, rec := range recs {
		if rec.TenantID == 0 {
			rec.TenantID = tenant.TenantIDFromContext(ctx)
		}
	}
	return r.db.WithContext(ctx).Create(recs).Error
}

// ListByUser 本人战绩分页（created_at 倒序，idx_record_user_created 覆盖）。
func (r *recordRepo) ListByUser(ctx context.Context, userID uint64, page, size int) ([]*model.Record, int64, error) {
	q := r.db.Model(&model.Record{}).
		Where("user_id = ?", userID).Order("created_at desc, id desc")
	return repo.Paginate[*model.Record](ctx, q, page, size)
}

// GetByID 明细软查：不存在与跨租户统一 (nil, nil)，由 service 映射 30110。
func (r *recordRepo) GetByID(ctx context.Context, id uint64) (*model.Record, error) {
	var out model.Record
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &out, err
}

func (r *recordRepo) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Record{}).Error
}

// ListByRoom 房间全部战绩（结算核对/房间维度展示，不分页）。
func (r *recordRepo) ListByRoom(ctx context.Context, roomID uint64) ([]*model.Record, error) {
	var list []*model.Record
	err := r.db.WithContext(ctx).
		Where("room_id = ?", roomID).Order("id asc").Find(&list).Error
	return list, err
}

// StatsByUser 个人统计聚合：总分 + 胜/负/平场次（score 单列聚合，免拉全表）。
func (r *recordRepo) StatsByUser(ctx context.Context, userID uint64) (total, win, lose, draw int64, err error) {
	err = r.db.WithContext(ctx).Model(&model.Record{}).Where("user_id = ?", userID).
		Select("COALESCE(SUM(score),0) AS total,"+
			"COALESCE(SUM(CASE WHEN score>0 THEN 1 ELSE 0 END),0) AS win,"+
			"COALESCE(SUM(CASE WHEN score<0 THEN 1 ELSE 0 END),0) AS lose,"+
			"COALESCE(SUM(CASE WHEN score=0 THEN 1 ELSE 0 END),0) AS draw").
		Row().Scan(&total, &win, &lose, &draw)
	return
}

// ListAdmin 监管端战绩分页。userID/roomID 传 0 表示该维度不过滤。
func (r *recordRepo) ListAdmin(ctx context.Context, userID, roomID uint64, page, size int) ([]*model.Record, int64, error) {
	q := r.db.Model(&model.Record{}).Order("id desc")
	if userID != 0 {
		q = q.Where("user_id = ?", userID)
	}
	if roomID != 0 {
		q = q.Where("room_id = ?", roomID)
	}
	return repo.Paginate[*model.Record](ctx, q, page, size)
}
