package repository

import (
	"context"

	"alexGo-cloud/modules/system/model"

	"gorm.io/gorm"
)

type UserRepository interface {
	List(ctx context.Context, tenantID uint64) ([]*model.User, error)
	GetByID(ctx context.Context, tenantID uint64, id uint64) (*model.User, error)
	GetByUsername(ctx context.Context, tenantID uint64, username string) (*model.User, error)
	Create(ctx context.Context, u *model.User) error
	Update(ctx context.Context, u *model.User) error
}

type userRepo struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) UserRepository {
	return &userRepo{db: db}
}

func (r *userRepo) List(ctx context.Context, tenantID uint64) ([]*model.User, error) {
	var users []*model.User
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Find(&users).Error
	return users, err
}

func (r *userRepo) GetByID(ctx context.Context, tenantID uint64, id uint64) (*model.User, error) {
	var u model.User
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&u, id).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepo) GetByUsername(ctx context.Context, tenantID uint64, username string) (*model.User, error) {
	var u model.User
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND username = ?", tenantID, username).First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepo) Create(ctx context.Context, u *model.User) error {
	return r.db.WithContext(ctx).Create(u).Error
}

func (r *userRepo) Update(ctx context.Context, u *model.User) error {
	return r.db.WithContext(ctx).Save(u).Error
}
