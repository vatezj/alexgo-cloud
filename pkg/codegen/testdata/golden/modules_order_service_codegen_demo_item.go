package service

import (
	"context"

	"alexGo-cloud/modules/order/model"
	"alexGo-cloud/modules/order/repository"
	"alexGo-cloud/pkg/tenant"
)

type CodegenDemoItemService interface {
	Create(ctx context.Context, entity *model.CodegenDemoItem) error
	List(ctx context.Context) ([]*model.CodegenDemoItem, error)
	GetByID(ctx context.Context, id uint64) (*model.CodegenDemoItem, error)
	Update(ctx context.Context, entity *model.CodegenDemoItem) error
	Delete(ctx context.Context, id uint64) error
}

type codegendemoitemService struct {
	repo repository.CodegenDemoItemRepository
}

func NewCodegenDemoItemService(repo repository.CodegenDemoItemRepository) CodegenDemoItemService {
	return &codegendemoitemService{repo: repo}
}

func (s *codegendemoitemService) Create(ctx context.Context, entity *model.CodegenDemoItem) error {
	return s.repo.Create(ctx, entity)
}

func (s *codegendemoitemService) List(ctx context.Context) ([]*model.CodegenDemoItem, error) {
	return s.repo.List(ctx, tenant.TenantIDFromContext(ctx))
}

func (s *codegendemoitemService) GetByID(ctx context.Context, id uint64) (*model.CodegenDemoItem, error) {
	return s.repo.GetByID(ctx, tenant.TenantIDFromContext(ctx), id)
}

func (s *codegendemoitemService) Update(ctx context.Context, entity *model.CodegenDemoItem) error {
	return s.repo.Update(ctx, entity)
}

func (s *codegendemoitemService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, tenant.TenantIDFromContext(ctx), id)
}
