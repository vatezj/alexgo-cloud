package service

import (
	"context"

	"alexGo-cloud/modules/taokouling/model"
	"alexGo-cloud/modules/taokouling/repository"
	"alexGo-cloud/pkg/tenant"
)

type ChannelService interface {
	Create(ctx context.Context, entity *model.Channel) error
	List(ctx context.Context) ([]*model.Channel, error)
	GetByID(ctx context.Context, id uint64) (*model.Channel, error)
	Update(ctx context.Context, entity *model.Channel) error
	Delete(ctx context.Context, id uint64) error
}

type channelService struct {
	repo repository.ChannelRepository
}

func NewChannelService(repo repository.ChannelRepository) ChannelService {
	return &channelService{repo: repo}
}

func (s *channelService) Create(ctx context.Context, entity *model.Channel) error {
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	return s.repo.Create(ctx, entity)
}

func (s *channelService) List(ctx context.Context) ([]*model.Channel, error) {
	return s.repo.List(ctx, tenant.TenantIDFromContext(ctx))
}

func (s *channelService) GetByID(ctx context.Context, id uint64) (*model.Channel, error) {
	return s.repo.GetByID(ctx, tenant.TenantIDFromContext(ctx), id)
}

func (s *channelService) Update(ctx context.Context, entity *model.Channel) error {
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	return s.repo.Update(ctx, entity)
}

func (s *channelService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, tenant.TenantIDFromContext(ctx), id)
}
