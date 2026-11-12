package service

import (
	"context"

	"alexGo-cloud/modules/starfuyu/model"
	"alexGo-cloud/modules/starfuyu/repository"
	"alexGo-cloud/pkg/tenant"
)

type ProfileService interface {
	Create(ctx context.Context, entity *model.Profile) error
	List(ctx context.Context) ([]*model.Profile, error)
	GetByID(ctx context.Context, id uint64) (*model.Profile, error)
	Update(ctx context.Context, entity *model.Profile) error
	Delete(ctx context.Context, id uint64) error
}

type profileService struct {
	repo repository.ProfileRepository
}

func NewProfileService(repo repository.ProfileRepository) ProfileService {
	return &profileService{repo: repo}
}

func (s *profileService) Create(ctx context.Context, entity *model.Profile) error {
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	return s.repo.Create(ctx, entity)
}

func (s *profileService) List(ctx context.Context) ([]*model.Profile, error) {
	return s.repo.List(ctx, tenant.TenantIDFromContext(ctx))
}

func (s *profileService) GetByID(ctx context.Context, id uint64) (*model.Profile, error) {
	return s.repo.GetByID(ctx, tenant.TenantIDFromContext(ctx), id)
}

func (s *profileService) Update(ctx context.Context, entity *model.Profile) error {
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	return s.repo.Update(ctx, entity)
}

func (s *profileService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, tenant.TenantIDFromContext(ctx), id)
}
