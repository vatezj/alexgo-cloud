package service

import (
	"context"

	"alexGo-cloud/modules/scorecard/model"
	"alexGo-cloud/modules/scorecard/repository"
	"alexGo-cloud/pkg/tenant"
)

type FriendBlackService interface {
	Create(ctx context.Context, entity *model.FriendBlack) error
	List(ctx context.Context) ([]*model.FriendBlack, error)
	GetByID(ctx context.Context, id uint64) (*model.FriendBlack, error)
	Update(ctx context.Context, entity *model.FriendBlack) error
	Delete(ctx context.Context, id uint64) error
}

type friendblackService struct {
	repo repository.FriendBlackRepository
}

func NewFriendBlackService(repo repository.FriendBlackRepository) FriendBlackService {
	return &friendblackService{repo: repo}
}

func (s *friendblackService) Create(ctx context.Context, entity *model.FriendBlack) error {
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	return s.repo.Create(ctx, entity)
}

func (s *friendblackService) List(ctx context.Context) ([]*model.FriendBlack, error) {
	return s.repo.List(ctx, tenant.TenantIDFromContext(ctx))
}

func (s *friendblackService) GetByID(ctx context.Context, id uint64) (*model.FriendBlack, error) {
	return s.repo.GetByID(ctx, tenant.TenantIDFromContext(ctx), id)
}

func (s *friendblackService) Update(ctx context.Context, entity *model.FriendBlack) error {
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	return s.repo.Update(ctx, entity)
}

func (s *friendblackService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, tenant.TenantIDFromContext(ctx), id)
}
