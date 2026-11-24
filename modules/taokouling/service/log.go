package service

import (
	"context"

	"alexGo-cloud/modules/taokouling/model"
	"alexGo-cloud/modules/taokouling/repository"
	"alexGo-cloud/pkg/tenant"
)

// LogRepo 转换记录仓储依赖（T4b 机械适配：匹配 repository.ConversionLogRepository 新签名；
// 脚手架旧 CRUD 中 GetByID/Update/Delete 在新仓储无对应方法，随死代码删除，T12 重写时整体替换）。
type LogRepo interface {
	Append(ctx context.Context, rec *model.ConversionLog) error
	List(ctx context.Context, f repository.LogFilter) ([]model.ConversionLog, int64, error)
}

type LogService interface {
	Create(ctx context.Context, entity *model.ConversionLog) error
	List(ctx context.Context) ([]*model.ConversionLog, error)
}

type logService struct {
	repo LogRepo
}

func NewLogService(repo LogRepo) LogService {
	return &logService{repo: repo}
}

func (s *logService) Create(ctx context.Context, entity *model.ConversionLog) error {
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	return s.repo.Append(ctx, entity)
}

func (s *logService) List(ctx context.Context) ([]*model.ConversionLog, error) {
	// 脚手架语义为全量列表（页面本地切页）；新仓储恒分页，取大页近似全量。
	list, _, err := s.repo.List(ctx, repository.LogFilter{Status: -1, Page: 1, Size: 1000})
	if err != nil {
		return nil, err
	}
	out := make([]*model.ConversionLog, 0, len(list))
	for i := range list {
		out = append(out, &list[i])
	}
	return out, nil
}
