package service

import (
	"context"

	"alexGo-cloud/modules/starfuyu/model"
	"alexGo-cloud/modules/starfuyu/repository"
	"alexGo-cloud/pkg/tenant"
)

type DivinationService interface {
	Create(ctx context.Context, entity *model.Divination) error
	List(ctx context.Context) ([]*model.Divination, error)
	GetByID(ctx context.Context, id uint64) (*model.Divination, error)
	Update(ctx context.Context, entity *model.Divination) error
	Delete(ctx context.Context, id uint64) error
}

type divinationService struct {
	repo repository.DivinationRepository
}

func NewDivinationService(repo repository.DivinationRepository) DivinationService {
	return &divinationService{repo: repo}
}

func (s *divinationService) Create(ctx context.Context, entity *model.Divination) error {
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	return s.repo.Create(ctx, entity)
}

func (s *divinationService) List(ctx context.Context) ([]*model.Divination, error) {
	return s.repo.List(ctx, tenant.TenantIDFromContext(ctx))
}

func (s *divinationService) GetByID(ctx context.Context, id uint64) (*model.Divination, error) {
	return s.repo.GetByID(ctx, tenant.TenantIDFromContext(ctx), id)
}

func (s *divinationService) Update(ctx context.Context, entity *model.Divination) error {
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	return s.repo.Update(ctx, entity)
}

func (s *divinationService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, tenant.TenantIDFromContext(ctx), id)
}
