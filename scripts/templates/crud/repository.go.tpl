package repository

import (
	"context"

	"alexGo-cloud/modules/{{.Module}}/model"
	"gorm.io/gorm"
)

type {{.ServiceName}}Repository interface {
	Create(ctx context.Context, entity *model.{{.ServiceName}}) error
	List(ctx context.Context) ([]*model.{{.ServiceName}}, error)
	GetByID(ctx context.Context, id uint64) (*model.{{.ServiceName}}, error)
	Update(ctx context.Context, entity *model.{{.ServiceName}}) error
	Delete(ctx context.Context, id uint64) error
}

type {{.LowerName}}Repo struct {
	db *gorm.DB
}

func New{{.ServiceName}}Repository(db *gorm.DB) {{.ServiceName}}Repository {
	return &{{.LowerName}}Repo{db: db}
}

func (r *{{.LowerName}}Repo) Create(ctx context.Context, entity *model.{{.ServiceName}}) error {
	return r.db.WithContext(ctx).Create(entity).Error
}

func (r *{{.LowerName}}Repo) List(ctx context.Context) ([]*model.{{.ServiceName}}, error) {
	var list []*model.{{.ServiceName}}
	err := r.db.WithContext(ctx).Find(&list).Error
	return list, err
}

func (r *{{.LowerName}}Repo) GetByID(ctx context.Context, id uint64) (*model.{{.ServiceName}}, error) {
	var entity model.{{.ServiceName}}
	err := r.db.WithContext(ctx).First(&entity, id).Error
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

func (r *{{.LowerName}}Repo) Update(ctx context.Context, entity *model.{{.ServiceName}}) error {
	return r.db.WithContext(ctx).Save(entity).Error
}

func (r *{{.LowerName}}Repo) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.{{.ServiceName}}{}, id).Error
}

