package repository

import (
	"context"

	"alexGo-cloud/modules/{{.Module}}/model"
	"gorm.io/gorm"
)

type {{.Entity}}Repository interface {
	Create(ctx context.Context, entity *model.{{.Entity}}) error
	List(ctx context.Context{{ if .HasTenant }}, tenantID uint64{{ end }}) ([]*model.{{.Entity}}, error)
	GetByID(ctx context.Context{{ if .HasTenant }}, tenantID uint64{{ end }}, id uint64) (*model.{{.Entity}}, error)
	Update(ctx context.Context, entity *model.{{.Entity}}) error
	Delete(ctx context.Context{{ if .HasTenant }}, tenantID uint64{{ end }}, id uint64) error
}

type {{ lower .Entity }}Repo struct {
	db *gorm.DB
}

func New{{.Entity}}Repository(db *gorm.DB) {{.Entity}}Repository {
	return &{{ lower .Entity }}Repo{db: db}
}

func (r *{{ lower .Entity }}Repo) Create(ctx context.Context, entity *model.{{.Entity}}) error {
	return r.db.WithContext(ctx).Create(entity).Error
}

func (r *{{ lower .Entity }}Repo) List(ctx context.Context{{ if .HasTenant }}, tenantID uint64{{ end }}) ([]*model.{{.Entity}}, error) {
	var list []*model.{{.Entity}}
	q := r.db.WithContext(ctx).Model(&model.{{.Entity}}{})
	{{- if .HasTenant }}
	q = q.Where("tenant_id = ?", tenantID)
	{{- end }}
	err := q.Find(&list).Error
	return list, err
}

func (r *{{ lower .Entity }}Repo) GetByID(ctx context.Context{{ if .HasTenant }}, tenantID uint64{{ end }}, id uint64) (*model.{{.Entity}}, error) {
	var entity model.{{.Entity}}
	q := r.db.WithContext(ctx).Model(&model.{{.Entity}}{})
	{{- if .HasTenant }}
	q = q.Where("tenant_id = ?", tenantID)
	{{- end }}
	err := q.Where("id = ?", id).First(&entity).Error
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

func (r *{{ lower .Entity }}Repo) Update(ctx context.Context, entity *model.{{.Entity}}) error {
	return r.db.WithContext(ctx).Save(entity).Error
}

func (r *{{ lower .Entity }}Repo) Delete(ctx context.Context{{ if .HasTenant }}, tenantID uint64{{ end }}, id uint64) error {
	q := r.db.WithContext(ctx).Model(&model.{{.Entity}}{})
	{{- if .HasTenant }}
	q = q.Where("tenant_id = ?", tenantID)
	{{- end }}
	return q.Where("id = ?", id).Delete(&model.{{.Entity}}{}).Error
}

