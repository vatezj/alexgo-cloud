package service

import (
	"context"

	"alexGo-cloud/modules/starfuyu/model"
	"alexGo-cloud/modules/starfuyu/repository"
	"alexGo-cloud/pkg/tenant"
)

type FortuneService interface {
	Create(ctx context.Context, entity *model.Fortune) error
	List(ctx context.Context) ([]*model.Fortune, error)
	GetByID(ctx context.Context, id uint64) (*model.Fortune, error)
	Update(ctx context.Context, entity *model.Fortune) error
	Delete(ctx context.Context, id uint64) error
}

type fortuneService struct {
	repo repository.FortuneRepository
}

func NewFortuneService(repo repository.FortuneRepository) FortuneService {
	return &fortuneService{repo: repo}
}

func (s *fortuneService) Create(ctx context.Context, entity *model.Fortune) error {
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	return s.repo.Create(ctx, entity)
}

func (s *fortuneService) List(ctx context.Context) ([]*model.Fortune, error) {
	return s.repo.List(ctx, tenant.TenantIDFromContext(ctx))
}

func (s *fortuneService) GetByID(ctx context.Context, id uint64) (*model.Fortune, error) {
	return s.repo.GetByID(ctx, tenant.TenantIDFromContext(ctx), id)
}

func (s *fortuneService) Update(ctx context.Context, entity *model.Fortune) error {
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	return s.repo.Update(ctx, entity)
}

func (s *fortuneService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, tenant.TenantIDFromContext(ctx), id)
}
