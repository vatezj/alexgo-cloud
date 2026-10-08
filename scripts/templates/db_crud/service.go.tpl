package service

import (
	"context"

	"alexGo-cloud/modules/{{.Module}}/model"
	"alexGo-cloud/modules/{{.Module}}/repository"
{{- if .HasTenant }}
	"alexGo-cloud/pkg/tenant"
{{- end }}
)

type {{.Entity}}Service interface {
	Create(ctx context.Context, entity *model.{{.Entity}}) error
	List(ctx context.Context) ([]*model.{{.Entity}}, error)
	GetByID(ctx context.Context, id uint64) (*model.{{.Entity}}, error)
	Update(ctx context.Context, entity *model.{{.Entity}}) error
	Delete(ctx context.Context, id uint64) error
}

type {{ lower .Entity }}Service struct {
	repo repository.{{.Entity}}Repository
}

func New{{.Entity}}Service(repo repository.{{.Entity}}Repository) {{.Entity}}Service {
	return &{{ lower .Entity }}Service{repo: repo}
}

func (s *{{ lower .Entity }}Service) Create(ctx context.Context, entity *model.{{.Entity}}) error {
	return s.repo.Create(ctx, entity)
}

func (s *{{ lower .Entity }}Service) List(ctx context.Context) ([]*model.{{.Entity}}, error) {
	{{- if .HasTenant }}
	return s.repo.List(ctx, tenant.TenantIDFromContext(ctx))
	{{- else }}
	return s.repo.List(ctx)
	{{- end }}
}

func (s *{{ lower .Entity }}Service) GetByID(ctx context.Context, id uint64) (*model.{{.Entity}}, error) {
	{{- if .HasTenant }}
	return s.repo.GetByID(ctx, tenant.TenantIDFromContext(ctx), id)
	{{- else }}
	return s.repo.GetByID(ctx, id)
	{{- end }}
}

func (s *{{ lower .Entity }}Service) Update(ctx context.Context, entity *model.{{.Entity}}) error {
	return s.repo.Update(ctx, entity)
}

func (s *{{ lower .Entity }}Service) Delete(ctx context.Context, id uint64) error {
	{{- if .HasTenant }}
	return s.repo.Delete(ctx, tenant.TenantIDFromContext(ctx), id)
	{{- else }}
	return s.repo.Delete(ctx, id)
	{{- end }}
}

