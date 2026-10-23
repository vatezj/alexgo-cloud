package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"alexGo-cloud/modules/fishgame/model"
	"alexGo-cloud/pkg/repo"
)

// FishRepository 图鉴条目数据访问。
type FishRepository interface {
	ListActive(ctx context.Context) ([]*model.Fish, error) // status=1 按 sort 升序
	ListAdmin(ctx context.Context, rarity string, status int, page, size int) ([]*model.Fish, int64, error)
	GetByFishID(ctx context.Context, fishID string) (*model.Fish, error) // 软查
	GetByID(ctx context.Context, id uint64) (*model.Fish, error)         // 软查
	Create(ctx context.Context, f *model.Fish) error
	Update(ctx context.Context, f *model.Fish) error
	Delete(ctx context.Context, id uint64) error
}

type fishRepo struct{ db *gorm.DB }

func NewFishRepository(db *gorm.DB) FishRepository { return &fishRepo{db: db} }

func (r *fishRepo) ListActive(ctx context.Context) ([]*model.Fish, error) {
	var list []*model.Fish
	err := r.db.WithContext(ctx).Where("status = ?", 1).Order("sort asc").Find(&list).Error
	return list, err
}

func (r *fishRepo) ListAdmin(ctx context.Context, rarity string, status int, page, size int) ([]*model.Fish, int64, error) {
	q := r.db.Model(&model.Fish{}).Order("sort asc")
	if rarity != "" {
		q = q.Where("rarity = ?", rarity)
	}
	if status >= 0 {
		q = q.Where("status = ?", status)
	}
	return repo.Paginate[*model.Fish](ctx, q, page, size)
}

func (r *fishRepo) GetByFishID(ctx context.Context, fishID string) (*model.Fish, error) {
	var out model.Fish
	err := r.db.WithContext(ctx).Where("fish_id = ?", fishID).First(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &out, err
}

func (r *fishRepo) GetByID(ctx context.Context, id uint64) (*model.Fish, error) {
	var out model.Fish
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &out, err
}

func (r *fishRepo) Create(ctx context.Context, f *model.Fish) error {
	if f.TenantID == 0 {
		f.TenantID = 0
	}
	return r.db.WithContext(ctx).Create(f).Error
}

func (r *fishRepo) Update(ctx context.Context, f *model.Fish) error {
	return r.db.WithContext(ctx).Save(f).Error
}

func (r *fishRepo) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Fish{}).Error
}
