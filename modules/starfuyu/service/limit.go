package service

import (
	"context"

	"alexGo-cloud/modules/starfuyu/model"
	"alexGo-cloud/modules/starfuyu/repository"
	"alexGo-cloud/pkg/tenant"
)

type LimitService interface {
	Create(ctx context.Context, entity *model.Limit) error
	List(ctx context.Context) ([]*model.Limit, error)
	GetByID(ctx context.Context, id uint64) (*model.Limit, error)
	Update(ctx context.Context, entity *model.Limit) error
	Delete(ctx context.Context, id uint64) error
}

type limitService struct {
	repo repository.LimitRepository
}

func NewLimitService(repo repository.LimitRepository) LimitService {
	return &limitService{repo: repo}
}

func (s *limitService) Create(ctx context.Context, entity *model.Limit) error {
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	return s.repo.Create(ctx, entity)
}

func (s *limitService) List(ctx context.Context) ([]*model.Limit, error) {
	return s.repo.List(ctx, tenant.TenantIDFromContext(ctx))
}

func (s *limitService) GetByID(ctx context.Context, id uint64) (*model.Limit, error) {
	return s.repo.GetByID(ctx, tenant.TenantIDFromContext(ctx), id)
}

func (s *limitService) Update(ctx context.Context, entity *model.Limit) error {
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	return s.repo.Update(ctx, entity)
}

func (s *limitService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, tenant.TenantIDFromContext(ctx), id)
}
