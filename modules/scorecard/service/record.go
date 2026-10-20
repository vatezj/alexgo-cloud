package service

import (
	"context"

	"alexGo-cloud/modules/scorecard/model"
	"alexGo-cloud/modules/scorecard/repository"
	"alexGo-cloud/pkg/tenant"
)

type RecordService interface {
	Create(ctx context.Context, entity *model.Record) error
	List(ctx context.Context) ([]*model.Record, error)
	GetByID(ctx context.Context, id uint64) (*model.Record, error)
	Update(ctx context.Context, entity *model.Record) error
	Delete(ctx context.Context, id uint64) error
}

type recordService struct {
	repo repository.RecordRepository
}

func NewRecordService(repo repository.RecordRepository) RecordService {
	return &recordService{repo: repo}
}

func (s *recordService) Create(ctx context.Context, entity *model.Record) error {
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	return s.repo.Create(ctx, entity)
}

func (s *recordService) List(ctx context.Context) ([]*model.Record, error) {
	return s.repo.List(ctx, tenant.TenantIDFromContext(ctx))
}

func (s *recordService) GetByID(ctx context.Context, id uint64) (*model.Record, error) {
	return s.repo.GetByID(ctx, tenant.TenantIDFromContext(ctx), id)
}

func (s *recordService) Update(ctx context.Context, entity *model.Record) error {
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	return s.repo.Update(ctx, entity)
}

func (s *recordService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, tenant.TenantIDFromContext(ctx), id)
}
