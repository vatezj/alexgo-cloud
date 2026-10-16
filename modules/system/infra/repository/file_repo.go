package repository

import (
	"context"

	"gorm.io/gorm"

	"alexGo-cloud/modules/system/infra/model"
)

// FileRepository 元数据 CRUD。未命中一律 gorm.ErrRecordNotFound（service 层转 30010）。
type FileRepository interface {
	Create(ctx context.Context, f *model.SystemFile) error
	// GetByID 受控读：按租户隔离。
	GetByID(ctx context.Context, tenantID, id uint64) (*model.SystemFile, error)
	// GetByKey public 读：**唯一不带租户过滤的查询**——公告附件跨端可见是产品语义
	// （spec §2），单租户测试全绿不代表这里可以加 tenant 条件。
	GetByKey(ctx context.Context, key string) (*model.SystemFile, error)
	// Delete 按租户隔离硬删行；0 行影响返回 ErrRecordNotFound。
	Delete(ctx context.Context, tenantID, id uint64) error
}

type fileRepo struct{ db *gorm.DB }

// NewFileRepository 构造文件元数据仓库。
func NewFileRepository(db *gorm.DB) FileRepository { return &fileRepo{db: db} }

func (r *fileRepo) Create(ctx context.Context, f *model.SystemFile) error {
	return r.db.WithContext(ctx).Create(f).Error
}

func (r *fileRepo) GetByID(ctx context.Context, tenantID, id uint64) (*model.SystemFile, error) {
	var f model.SystemFile
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).First(&f).Error
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *fileRepo) GetByKey(ctx context.Context, key string) (*model.SystemFile, error) {
	var f model.SystemFile
	err := r.db.WithContext(ctx).Where("public_key = ?", key).First(&f).Error
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *fileRepo) Delete(ctx context.Context, tenantID, id uint64) error {
	res := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).Delete(&model.SystemFile{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
