package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"alexGo-cloud/modules/starfuyu/model"
)

// SignCacheRepository 签文按号持久缓存（brief 钉死签名，Task 6 服务层唯一数据入口）。
// 内容表：签文全局稳定（决定 15）——写路径显式 tenant_id=0；读路径不猜租户，
// 由 service 层包裹 ctx（Task 6）。
type SignCacheRepository interface {
	// Get 按签号取缓存；miss → (nil, nil)。
	Get(ctx context.Context, number int) (*model.SignCache, error)
	// Create 首访落缓存；同号冲突不覆盖首签（同号稳定），冲突后重读返回已存在行。
	Create(ctx context.Context, number int, content string) (*model.SignCache, error)
}

type signCacheRepo struct{ db *gorm.DB }

func NewSignCacheRepository(db *gorm.DB) SignCacheRepository { return &signCacheRepo{db: db} }

func (r *signCacheRepo) Get(ctx context.Context, number int) (*model.SignCache, error) {
	var out model.SignCache
	err := r.db.WithContext(ctx).Where("sign_number = ?", number).First(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *signCacheRepo) Create(ctx context.Context, number int, content string) (*model.SignCache, error) {
	row := &model.SignCache{TenantID: 0, SignNumber: number, SignContent: content}
	// 冲突目标 (tenant_id,sign_number)；DoNothing 保首签不被覆盖（决定 15：同号稳定）。
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "sign_number"}},
		DoNothing: true,
	}).Create(row).Error
	if err != nil {
		return nil, err
	}
	// 冲突重读：二次 Create 返回已存在行内容（部分驱动回填 ID 不可靠，按签号重读）。
	return r.Get(ctx, number)
}
