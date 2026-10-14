package service

import (
	"context"
	"errors"
	"testing"

	"alexGo-cloud/modules/order/model"
	"alexGo-cloud/pkg/tenant"
)

type fakeCodegenDemoMenuRepo struct {
	data map[uint64]*model.CodegenDemoMenu
	seq  uint64
}

var errCodegenDemoMenuRecordNotFound = errors.New("record not found")

func (f *fakeCodegenDemoMenuRepo) Create(_ context.Context, e *model.CodegenDemoMenu) error {
	f.seq++
	e.ID = f.seq
	f.data[e.ID] = e
	return nil
}

func (f *fakeCodegenDemoMenuRepo) List(_ context.Context, _ uint64) ([]*model.CodegenDemoMenu, error) {
	out := make([]*model.CodegenDemoMenu, 0, len(f.data))
	for _, e := range f.data {
		out = append(out, e)
	}
	return out, nil
}

func (f *fakeCodegenDemoMenuRepo) GetByID(_ context.Context, _ uint64, id uint64) (*model.CodegenDemoMenu, error) {
	if e, ok := f.data[id]; ok {
		return e, nil
	}
	return nil, errCodegenDemoMenuRecordNotFound
}

func (f *fakeCodegenDemoMenuRepo) Update(_ context.Context, e *model.CodegenDemoMenu) error {
	f.data[e.ID] = e
	return nil
}

func (f *fakeCodegenDemoMenuRepo) Delete(_ context.Context, _ uint64, id uint64) error {
	delete(f.data, id)
	return nil
}

// HasChildren 语义镜像：data 里存在 ParentID==id 的行即为有子。
func (f *fakeCodegenDemoMenuRepo) HasChildren(_ context.Context, _ uint64, id uint64) (bool, error) {
	for _, e := range f.data {
		if e.ParentID == id {
			return true, nil
		}
	}
	return false, nil
}

func TestCodegenDemoMenuService_CRUD(t *testing.T) {
	svc := NewCodegenDemoMenuService(&fakeCodegenDemoMenuRepo{data: map[uint64]*model.CodegenDemoMenu{}})
	ctx := context.Background()
	ctx = tenant.WithTenantID(ctx, 42)

	e := &model.CodegenDemoMenu{}
	if err := svc.Create(ctx, e); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if e.TenantID != 42 {
		t.Fatalf("Create 应从 ctx 打戳租户 42, got %d", e.TenantID)
	}
	if e.ID == 0 {
		t.Fatal("Create should assign ID")
	}
	if _, err := svc.GetByID(ctx, e.ID); err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if err := svc.Update(ctx, e); err != nil {
		t.Fatalf("Update: %v", err)
	}
	list, err := svc.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("List: err=%v len=%d", err, len(list))
	}
	if err := svc.Delete(ctx, e.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if list, _ = svc.List(ctx); len(list) != 0 {
		t.Fatalf("after Delete len=%d", len(list))
	}
}

// TestCodegenDemoMenuService_Tree 钉树语义（spec §10.2，Review Focus ⑤）：
// 父不存在拒绝、自引用拒绝、挂后代成环拒绝、合法改挂（无关子树/根）、
// 有子禁删、无子可删。
func TestCodegenDemoMenuService_Tree(t *testing.T) {
	svc := NewCodegenDemoMenuService(&fakeCodegenDemoMenuRepo{data: map[uint64]*model.CodegenDemoMenu{}})
	ctx := context.Background()
	ctx = tenant.WithTenantID(ctx, 42)

	if err := svc.Create(ctx, &model.CodegenDemoMenu{ParentID: 999}); err == nil {
		t.Fatal("父不存在应拒绝")
	}
	root := &model.CodegenDemoMenu{}
	if err := svc.Create(ctx, root); err != nil {
		t.Fatalf("Create root: %v", err)
	}
	child := &model.CodegenDemoMenu{ParentID: root.ID}
	if err := svc.Create(ctx, child); err != nil {
		t.Fatalf("Create child: %v", err)
	}
	child.ParentID = child.ID
	if err := svc.Update(ctx, child); err == nil {
		t.Fatal("自引用应拒绝")
	}
	child.ParentID = root.ID
	if err := svc.Update(ctx, child); err != nil {
		t.Fatalf("恢复父级: %v", err)
	}
	// 合法改挂：无关子树多级上溯不得误伤（祖先链不含自身即放行）。
	other := &model.CodegenDemoMenu{}
	if err := svc.Create(ctx, other); err != nil {
		t.Fatalf("Create other: %v", err)
	}
	otherChild := &model.CodegenDemoMenu{ParentID: other.ID}
	if err := svc.Create(ctx, otherChild); err != nil {
		t.Fatalf("Create otherChild: %v", err)
	}
	child.ParentID = otherChild.ID
	if err := svc.Update(ctx, child); err != nil {
		t.Fatalf("改挂无关子树: %v", err)
	}
	child.ParentID = 0
	if err := svc.Update(ctx, child); err != nil {
		t.Fatalf("改挂根: %v", err)
	}
	// 成环拒（Final Review Fix 1）：child 改挂自己的后代 grandchild → A→B→A。
	// 环成员会从 root/孤儿构树法里消失，前端不可编辑/不可删。
	grandchild := &model.CodegenDemoMenu{ParentID: child.ID}
	if err := svc.Create(ctx, grandchild); err != nil {
		t.Fatalf("Create grandchild: %v", err)
	}
	child.ParentID = grandchild.ID
	if err := svc.Update(ctx, child); err == nil {
		t.Fatal("父级改挂自身后代（成环）应拒绝")
	}
	child.ParentID = root.ID
	if err := svc.Update(ctx, child); err != nil {
		t.Fatalf("改回 root: %v", err)
	}
	if err := svc.Delete(ctx, root.ID); err == nil {
		t.Fatal("有子禁删")
	}
	if err := svc.Delete(ctx, child.ID); err == nil {
		t.Fatal("有子禁删（孙尚存）")
	}
	if err := svc.Delete(ctx, grandchild.ID); err != nil {
		t.Fatalf("Delete grandchild: %v", err)
	}
	if err := svc.Delete(ctx, child.ID); err != nil {
		t.Fatalf("Delete child: %v", err)
	}
	if err := svc.Delete(ctx, root.ID); err != nil {
		t.Fatalf("Delete root: %v", err)
	}
	if err := svc.Delete(ctx, otherChild.ID); err != nil {
		t.Fatalf("Delete otherChild: %v", err)
	}
	if err := svc.Delete(ctx, other.ID); err != nil {
		t.Fatalf("Delete other: %v", err)
	}
}
