package service

import (
	"context"

	"alexGo-cloud/modules/scorecard/model"
	"alexGo-cloud/modules/scorecard/repository"
	"alexGo-cloud/pkg/tenant"
)

type RoomService interface {
	Create(ctx context.Context, entity *model.Room) error
	List(ctx context.Context) ([]*model.Room, error)
	GetByID(ctx context.Context, id uint64) (*model.Room, error)
	Update(ctx context.Context, entity *model.Room) error
	Delete(ctx context.Context, id uint64) error
}

type roomService struct {
	repo repository.RoomRepository
}

func NewRoomService(repo repository.RoomRepository) RoomService {
	return &roomService{repo: repo}
}

func (s *roomService) Create(ctx context.Context, entity *model.Room) error {
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	return s.repo.Create(ctx, entity)
}

func (s *roomService) List(ctx context.Context) ([]*model.Room, error) {
	return s.repo.List(ctx, tenant.TenantIDFromContext(ctx))
}

func (s *roomService) GetByID(ctx context.Context, id uint64) (*model.Room, error) {
	return s.repo.GetByID(ctx, tenant.TenantIDFromContext(ctx), id)
}

func (s *roomService) Update(ctx context.Context, entity *model.Room) error {
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	return s.repo.Update(ctx, entity)
}

func (s *roomService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, tenant.TenantIDFromContext(ctx), id)
}
