package service

import (
	"context"
	"fmt"

	"alexGo-cloud/modules/order/model"
	"alexGo-cloud/modules/order/repository"
	"alexGo-cloud/pkg/tenant"
)

type CodegenDemoMenuService interface {
	Create(ctx context.Context, entity *model.CodegenDemoMenu) error
	List(ctx context.Context) ([]*model.CodegenDemoMenu, error)
	GetByID(ctx context.Context, id uint64) (*model.CodegenDemoMenu, error)
	Update(ctx context.Context, entity *model.CodegenDemoMenu) error
	Delete(ctx context.Context, id uint64) error
}

type codegendemomenuService struct {
	repo repository.CodegenDemoMenuRepository
}

func NewCodegenDemoMenuService(repo repository.CodegenDemoMenuRepository) CodegenDemoMenuService {
	return &codegendemomenuService{repo: repo}
}

// validateParent 树表父级校验（spec §10.2）：parent_id=0 是根直接放行；
// 否则父必须存在且非自身。不加 Children 字段、不加 GetTree，List 保持平铺。
func (s *codegendemomenuService) validateParent(ctx context.Context, entity *model.CodegenDemoMenu) error {
	if entity.ParentID == 0 {
		return nil
	}
	if entity.ParentID == entity.ID {
		return fmt.Errorf("cannot set self (id=%d) as parent", entity.ID)
	}
	_, err := s.repo.GetByID(ctx, tenant.TenantIDFromContext(ctx), entity.ParentID)
	if err != nil {
		return fmt.Errorf("parent id=%d: %w", entity.ParentID, err)
	}
	return nil
}

func (s *codegendemomenuService) Create(ctx context.Context, entity *model.CodegenDemoMenu) error {
	if err := s.validateParent(ctx, entity); err != nil {
		return err
	}
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	return s.repo.Create(ctx, entity)
}

func (s *codegendemomenuService) List(ctx context.Context) ([]*model.CodegenDemoMenu, error) {
	return s.repo.List(ctx, tenant.TenantIDFromContext(ctx))
}

func (s *codegendemomenuService) GetByID(ctx context.Context, id uint64) (*model.CodegenDemoMenu, error) {
	return s.repo.GetByID(ctx, tenant.TenantIDFromContext(ctx), id)
}

func (s *codegendemomenuService) Update(ctx context.Context, entity *model.CodegenDemoMenu) error {
	if err := s.validateParent(ctx, entity); err != nil {
		return err
	}
	entity.TenantID = tenant.TenantIDFromContext(ctx)
	return s.repo.Update(ctx, entity)
}

func (s *codegendemomenuService) Delete(ctx context.Context, id uint64) error {
	has, err := s.repo.HasChildren(ctx, tenant.TenantIDFromContext(ctx), id)
	if err != nil {
		return err
	}
	if has {
		return fmt.Errorf("cannot delete id=%d: has children", id)
	}
	return s.repo.Delete(ctx, tenant.TenantIDFromContext(ctx), id)
}
