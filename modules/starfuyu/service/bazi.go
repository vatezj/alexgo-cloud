package service

import (
	"context"

	"alexGo-cloud/modules/starfuyu/model"
	"alexGo-cloud/modules/starfuyu/repository"
	"alexGo-cloud/pkg/tenant"
)

type BaziService interface {
	Create(ctx context.Context, entity *model.Bazi) error
	List(ctx context.Context) ([]*model.Bazi, error)
	GetByID(ctx context.Context, id uint64) (*model.Bazi, error)
	Update(ctx context.Context, entity *model.Bazi) error
	Delete(ctx context.Context, id uint64) error
}

type baziService struct {
	repo repository.BaziRepository
}

func NewBaziService(repo repository.BaziRepository) BaziService {
	return &baziService{repo: repo}
}

func (s *baziService) Create(ctx context.Context, entity *model.Bazi) error {
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	return s.repo.Create(ctx, entity)
}

func (s *baziService) List(ctx context.Context) ([]*model.Bazi, error) {
	return s.repo.List(ctx, tenant.TenantIDFromContext(ctx))
}

func (s *baziService) GetByID(ctx context.Context, id uint64) (*model.Bazi, error) {
	return s.repo.GetByID(ctx, tenant.TenantIDFromContext(ctx), id)
}

func (s *baziService) Update(ctx context.Context, entity *model.Bazi) error {
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	return s.repo.Update(ctx, entity)
}

func (s *baziService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, tenant.TenantIDFromContext(ctx), id)
}
