package service

import (
	"context"

	"alexGo-cloud/modules/taokouling/model"
	"alexGo-cloud/pkg/tenant"
)

// ChannelRepo 渠道仓储依赖（T4b 机械适配：匹配 repository.ChannelRepository 新签名，
// 去掉旧接口的 tenantID 形参；结构化满足，T11 重写时整体替换）。
type ChannelRepo interface {
	Create(ctx context.Context, ch *model.Channel) error
	List(ctx context.Context, page, size int) ([]model.Channel, int64, error)
	GetByID(ctx context.Context, id uint64) (*model.Channel, error)
	Update(ctx context.Context, ch *model.Channel) error
	Delete(ctx context.Context, id uint64) error
}

type ChannelService interface {
	Create(ctx context.Context, entity *model.Channel) error
	List(ctx context.Context) ([]*model.Channel, error)
	GetByID(ctx context.Context, id uint64) (*model.Channel, error)
	Update(ctx context.Context, entity *model.Channel) error
	Delete(ctx context.Context, id uint64) error
}

type channelService struct {
	repo ChannelRepo
}

func NewChannelService(repo ChannelRepo) ChannelService {
	return &channelService{repo: repo}
}

func (s *channelService) Create(ctx context.Context, entity *model.Channel) error {
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	return s.repo.Create(ctx, entity)
}

func (s *channelService) List(ctx context.Context) ([]*model.Channel, error) {
	// 脚手架语义为全量列表（页面本地切页）；新仓储恒分页，取大页近似全量。
	list, _, err := s.repo.List(ctx, 1, 1000)
	if err != nil {
		return nil, err
	}
	out := make([]*model.Channel, 0, len(list))
	for i := range list {
		out = append(out, &list[i])
	}
	return out, nil
}

func (s *channelService) GetByID(ctx context.Context, id uint64) (*model.Channel, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *channelService) Update(ctx context.Context, entity *model.Channel) error {
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	return s.repo.Update(ctx, entity)
}

func (s *channelService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, id)
}
