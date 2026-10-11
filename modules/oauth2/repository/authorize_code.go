package repository

import (
	"context"

	"gorm.io/gorm"

	"alexGo-cloud/modules/oauth2/model"
)

// AuthorizeCodeRepository 授权码访问层（D4：OwnerTenantID → tenant_id 列）。
type AuthorizeCodeRepository struct {
	db *gorm.DB
}

func NewAuthorizeCodeRepository(db *gorm.DB) *AuthorizeCodeRepository {
	return &AuthorizeCodeRepository{db: db}
}

func (r *AuthorizeCodeRepository) Create(ctx context.Context, row *model.Oauth2AuthorizeCode) error {
	return r.db.WithContext(ctx).Create(row).Error
}

func (r *AuthorizeCodeRepository) GetByCode(ctx context.Context, code string) (*model.Oauth2AuthorizeCode, error) {
	var row model.Oauth2AuthorizeCode
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// MarkRevoked CAS 单次消费：只有未消费的行能翻转成功。
// 返回 false（RowsAffected=0）= 已被首次尝试消费——授权码「首错即消费」的落点。
func (r *AuthorizeCodeRepository) MarkRevoked(ctx context.Context, code string) (bool, error) {
	res := r.db.WithContext(ctx).
		Model(&model.Oauth2AuthorizeCode{}).
		Where("code = ? AND revoked = 0", code).
		Update("revoked", true)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}
