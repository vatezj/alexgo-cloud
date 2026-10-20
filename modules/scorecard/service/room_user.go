package service

import (
	"context"

	"alexGo-cloud/modules/scorecard/model"
	"alexGo-cloud/modules/scorecard/repository"
	"alexGo-cloud/pkg/tenant"
)

type RoomUserService interface {
	Create(ctx context.Context, entity *model.RoomUser) error
	List(ctx context.Context) ([]*model.RoomUser, error)
	GetByID(ctx context.Context, id uint64) (*model.RoomUser, error)
	Update(ctx context.Context, entity *model.RoomUser) error
	Delete(ctx context.Context, id uint64) error
}

type roomuserService struct {
	repo repository.RoomUserRepository
}

func NewRoomUserService(repo repository.RoomUserRepository) RoomUserService {
	return &roomuserService{repo: repo}
}

func (s *roomuserService) Create(ctx context.Context, entity *model.RoomUser) error {
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	return s.repo.Create(ctx, entity)
}

func (s *roomuserService) List(ctx context.Context) ([]*model.RoomUser, error) {
	return s.repo.List(ctx, tenant.TenantIDFromContext(ctx))
}

func (s *roomuserService) GetByID(ctx context.Context, id uint64) (*model.RoomUser, error) {
	return s.repo.GetByID(ctx, tenant.TenantIDFromContext(ctx), id)
}

func (s *roomuserService) Update(ctx context.Context, entity *model.RoomUser) error {
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	return s.repo.Update(ctx, entity)
}

func (s *roomuserService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, tenant.TenantIDFromContext(ctx), id)
}
