package repository

import (
	"context"

	"gorm.io/gorm"

	"alexGo-cloud/modules/system/oauth2/model"
)

// ClientRepository oauth2_client 访问层（D4：模型无 tenant_id/deleted → 租户插件跳过、硬删除）。
type ClientRepository struct {
	db *gorm.DB
}

func NewClientRepository(db *gorm.DB) *ClientRepository {
	return &ClientRepository{db: db}
}

func (r *ClientRepository) GetByClientID(ctx context.Context, clientID string) (*model.Oauth2Client, error) {
	var c model.Oauth2Client
	if err := r.db.WithContext(ctx).Where("client_id = ?", clientID).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *ClientRepository) GetByID(ctx context.Context, id uint64) (*model.Oauth2Client, error) {
	var c model.Oauth2Client
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *ClientRepository) Create(ctx context.Context, c *model.Oauth2Client) error {
	return r.db.WithContext(ctx).Create(c).Error
}

func (r *ClientRepository) Update(ctx context.Context, c *model.Oauth2Client) error {
	return r.db.WithContext(ctx).Save(c).Error
}

// Delete 硬删（D4）；不存在返回 nil——删除幂等。
func (r *ClientRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Oauth2Client{}).Error
}

func (r *ClientRepository) List(ctx context.Context, page, size int) ([]*model.Oauth2Client, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&model.Oauth2Client{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []*model.Oauth2Client
	err := r.db.WithContext(ctx).
		Order("id ASC").Offset((page - 1) * size).Limit(size).
		Find(&rows).Error
	return rows, total, err
}
