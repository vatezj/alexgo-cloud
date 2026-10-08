package service

import (
	"context"

	"alexGo-cloud/modules/{{.Module}}/model"
	"alexGo-cloud/modules/{{.Module}}/repository"
)

type {{.ServiceName}}Service interface {
	Create(ctx context.Context, entity *model.{{.ServiceName}}) error
	List(ctx context.Context) ([]*model.{{.ServiceName}}, error)
	GetByID(ctx context.Context, id uint64) (*model.{{.ServiceName}}, error)
	Update(ctx context.Context, entity *model.{{.ServiceName}}) error
	Delete(ctx context.Context, id uint64) error
}

type {{.LowerName}}Service struct {
	repo repository.{{.ServiceName}}Repository
}

func New{{.ServiceName}}Service(repo repository.{{.ServiceName}}Repository) {{.ServiceName}}Service {
	return &{{.LowerName}}Service{repo: repo}
}

func (s *{{.LowerName}}Service) Create(ctx context.Context, entity *model.{{.ServiceName}}) error {
	return s.repo.Create(ctx, entity)
}

func (s *{{.LowerName}}Service) List(ctx context.Context) ([]*model.{{.ServiceName}}, error) {
	return s.repo.List(ctx)
}

func (s *{{.LowerName}}Service) GetByID(ctx context.Context, id uint64) (*model.{{.ServiceName}}, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *{{.LowerName}}Service) Update(ctx context.Context, entity *model.{{.ServiceName}}) error {
	return s.repo.Update(ctx, entity)
}

func (s *{{.LowerName}}Service) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, id)
}

