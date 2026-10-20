package service

import (
	"context"

	"alexGo-cloud/modules/scorecard/model"
	"alexGo-cloud/modules/scorecard/repository"
	"alexGo-cloud/pkg/tenant"
)

type RoomLogService interface {
	Create(ctx context.Context, entity *model.RoomLog) error
	List(ctx context.Context) ([]*model.RoomLog, error)
	GetByID(ctx context.Context, id uint64) (*model.RoomLog, error)
	Update(ctx context.Context, entity *model.RoomLog) error
	Delete(ctx context.Context, id uint64) error
}

type roomlogService struct {
	repo repository.RoomLogRepository
}

func NewRoomLogService(repo repository.RoomLogRepository) RoomLogService {
	return &roomlogService{repo: repo}
}

func (s *roomlogService) Create(ctx context.Context, entity *model.RoomLog) error {
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	return s.repo.Create(ctx, entity)
}

func (s *roomlogService) List(ctx context.Context) ([]*model.RoomLog, error) {
	return s.repo.List(ctx, tenant.TenantIDFromContext(ctx))
}

func (s *roomlogService) GetByID(ctx context.Context, id uint64) (*model.RoomLog, error) {
	return s.repo.GetByID(ctx, tenant.TenantIDFromContext(ctx), id)
}

func (s *roomlogService) Update(ctx context.Context, entity *model.RoomLog) error {
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	return s.repo.Update(ctx, entity)
}

func (s *roomlogService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, tenant.TenantIDFromContext(ctx), id)
}
