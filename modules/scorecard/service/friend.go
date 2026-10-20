package service

import (
	"context"

	"alexGo-cloud/modules/scorecard/model"
	"alexGo-cloud/modules/scorecard/repository"
	"alexGo-cloud/pkg/tenant"
)

type FriendService interface {
	Create(ctx context.Context, entity *model.Friend) error
	List(ctx context.Context) ([]*model.Friend, error)
	GetByID(ctx context.Context, id uint64) (*model.Friend, error)
	Update(ctx context.Context, entity *model.Friend) error
	Delete(ctx context.Context, id uint64) error
}

type friendService struct {
	repo repository.FriendRepository
}

func NewFriendService(repo repository.FriendRepository) FriendService {
	return &friendService{repo: repo}
}

func (s *friendService) Create(ctx context.Context, entity *model.Friend) error {
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	return s.repo.Create(ctx, entity)
}

func (s *friendService) List(ctx context.Context) ([]*model.Friend, error) {
	return s.repo.List(ctx, tenant.TenantIDFromContext(ctx))
}

func (s *friendService) GetByID(ctx context.Context, id uint64) (*model.Friend, error) {
	return s.repo.GetByID(ctx, tenant.TenantIDFromContext(ctx), id)
}

func (s *friendService) Update(ctx context.Context, entity *model.Friend) error {
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	return s.repo.Update(ctx, entity)
}

func (s *friendService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, tenant.TenantIDFromContext(ctx), id)
}
