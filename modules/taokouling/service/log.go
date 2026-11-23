package service

import (
	"context"

	"alexGo-cloud/modules/taokouling/model"
	"alexGo-cloud/modules/taokouling/repository"
	"alexGo-cloud/pkg/tenant"
)

type LogService interface {
	Create(ctx context.Context, entity *model.Log) error
	List(ctx context.Context) ([]*model.Log, error)
	GetByID(ctx context.Context, id uint64) (*model.Log, error)
	Update(ctx context.Context, entity *model.Log) error
	Delete(ctx context.Context, id uint64) error
}

type logService struct {
	repo repository.LogRepository
}

func NewLogService(repo repository.LogRepository) LogService {
	return &logService{repo: repo}
}

func (s *logService) Create(ctx context.Context, entity *model.Log) error {
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	return s.repo.Create(ctx, entity)
}

func (s *logService) List(ctx context.Context) ([]*model.Log, error) {
	return s.repo.List(ctx, tenant.TenantIDFromContext(ctx))
}

func (s *logService) GetByID(ctx context.Context, id uint64) (*model.Log, error) {
	return s.repo.GetByID(ctx, tenant.TenantIDFromContext(ctx), id)
}

func (s *logService) Update(ctx context.Context, entity *model.Log) error {
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	return s.repo.Update(ctx, entity)
}

func (s *logService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, tenant.TenantIDFromContext(ctx), id)
}
