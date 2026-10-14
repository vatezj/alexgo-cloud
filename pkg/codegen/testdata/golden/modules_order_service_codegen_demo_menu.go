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
// 否则父必须存在、非自身，且不得是本节点的后代——挂子孙会成环
// （A→B→A），环成员从 root/孤儿构树法里消失，前端不可编辑/不可删。
// 不加 Children 字段、不加 GetTree，List 保持平铺。
func (s *codegendemomenuService) validateParent(ctx context.Context, entity *model.CodegenDemoMenu) error {
	if entity.ParentID == 0 {
		return nil
	}
	if entity.ParentID == entity.ID {
		return fmt.Errorf("cannot set self (id=%d) as parent", entity.ID)
	}
	parent, err := s.repo.GetByID(ctx, tenant.TenantIDFromContext(ctx), entity.ParentID)
	if err != nil {
		return fmt.Errorf("parent id=%d: %w", entity.ParentID, err)
	}
	if entity.ID == 0 {
		return nil // Create 尚未分配 id，祖先链不可能回到自身
	}
	// 新父是本节点后代 ⇔ 新父的祖先链含 entity.ID。沿 ParentID 逐级上溯：
	// 命中 entity.ID 即成环拒绝；visited 防既有脏数据里的旧环死循环；
	// 祖先链中断（孤儿脏数据）视为非后代放行，父存在性已由上面保证。
	seen := map[uint64]bool{parent.ID: true}
	for cur := parent; cur.ParentID != 0; {
		if cur.ParentID == entity.ID {
			return fmt.Errorf("cannot set parent id=%d: descendant of id=%d (cycle)", entity.ParentID, entity.ID)
		}
		if seen[cur.ParentID] {
			break // 旧环不含本节点，停步即可
		}
		seen[cur.ParentID] = true
		next, err := s.repo.GetByID(ctx, tenant.TenantIDFromContext(ctx), cur.ParentID)
		if err != nil {
			break
		}
		cur = next
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
