package service

import (
	"context"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/repository"
	"alexGo-cloud/pkg/tenant"
)

// recordingPermSvc：只记录 RebuildPolicies 调用次数的 PermissionService 桩，
// 其余方法与菜单编辑无关，返回零值即可。
type recordingPermSvc struct {
	rebuilds int
}

func (f *recordingPermSvc) UserRoles(context.Context, uint64) ([]*model.Role, error) {
	return nil, nil
}
func (f *recordingPermSvc) UserMenus(context.Context, uint64) ([]*model.Menu, error) {
	return nil, nil
}
func (f *recordingPermSvc) UserPermCodes(context.Context, uint64) ([]string, error) {
	return nil, nil
}
func (f *recordingPermSvc) UserRoutes(context.Context, uint64) ([]*VbenRoute, error) {
	return nil, nil
}
func (f *recordingPermSvc) EnsureUserRolePolicy(context.Context, string, []*model.Role) error {
	return nil
}
func (f *recordingPermSvc) RebuildPolicies(context.Context) error {
	f.rebuilds++
	return nil
}
func (f *recordingPermSvc) RebuildRolePolicies(context.Context, uint64) error {
	return nil
}

// setupMenuService：sqlite :memory: + Menu 表，返回服务与记录桩。
func setupMenuService(t *testing.T) (MenuService, *recordingPermSvc) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Menu{}); err != nil {
		t.Fatal(err)
	}
	perm := &recordingPermSvc{}
	return NewMenuService(repository.NewMenuRepository(db), perm), perm
}

// createMenuRow 走 service.Create 造一行（自带租户/时间戳），返回 id。
func createMenuRow(t *testing.T, svc MenuService, ctx context.Context, m *model.Menu) uint64 {
	t.Helper()
	created, err := svc.Create(ctx, m)
	if err != nil {
		t.Fatalf("create %q: %v", m.Name, err)
	}
	return created.ID
}

// 正常更新：字段覆盖落库、UpdatedAt 前移、权限变更触发策略重建；Status 显式 0 保留为停用。
func TestMenuUpdate_Success(t *testing.T) {
	svc, perm := setupMenuService(t)
	ctx := tenant.WithTenantID(context.Background(), 1)

	id := createMenuRow(t, svc, ctx, &model.Menu{
		Type: "menu", Name: "旧名", Path: "/old", Permission: "system:menu:list", Sort: 1, Status: 1,
	})
	baseRebuilds := perm.rebuilds // Create 造数触发的次数，更新断言以其为基线

	updated, err := svc.Update(ctx, &model.Menu{
		ID: id, ParentID: 0, Type: "menu", Name: "新名", Path: "/new",
		Component: "views/x", Icon: "lucide:x", Permission: "system:menu:all",
		Sort: 9, Status: 0,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "新名" || updated.Path != "/new" || updated.Permission != "system:menu:all" || updated.Sort != 9 {
		t.Errorf("fields not applied: %+v", updated)
	}
	if updated.Status != 0 {
		t.Errorf("status = %d, want 0（显式停用不应被默认成启用）", updated.Status)
	}
	if !updated.UpdatedAt.After(updated.CreatedAt) {
		t.Errorf("UpdatedAt %v 未晚于 CreatedAt %v", updated.UpdatedAt, updated.CreatedAt)
	}
	// Create 本身也会重建一次，Update 后应比造数时多 1
	if perm.rebuilds != baseRebuilds+1 {
		t.Errorf("RebuildPolicies 次数 = %d, want %d", perm.rebuilds, baseRebuilds+1)
	}

	// 重新读库确认真落盘
	list, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Name != "新名" {
		t.Errorf("list after update = %+v, want one row named 新名", list)
	}
}

// 父节点校验：parent 指向自身、或指向自己的后代 → 拒绝且不落库、不重建策略。
func TestMenuUpdate_RejectsCycle(t *testing.T) {
	svc, perm := setupMenuService(t)
	ctx := tenant.WithTenantID(context.Background(), 1)

	rootID := createMenuRow(t, svc, ctx, &model.Menu{Type: "dir", Name: "根", Status: 1})
	childID := createMenuRow(t, svc, ctx, &model.Menu{Type: "menu", Name: "子", ParentID: rootID, Status: 1})
	baseRebuilds := perm.rebuilds

	if _, err := svc.Update(ctx, &model.Menu{ID: rootID, ParentID: rootID, Type: "dir", Name: "根", Status: 1}); err == nil || !strings.Contains(err.Error(), "itself") {
		t.Errorf("parent=self: err = %v, want 含 itself 的错误", err)
	}
	if _, err := svc.Update(ctx, &model.Menu{ID: rootID, ParentID: childID, Type: "dir", Name: "根", Status: 1}); err == nil || !strings.Contains(err.Error(), "descendant") {
		t.Errorf("parent=child: err = %v, want 含 descendant 的错误", err)
	}
	if perm.rebuilds != baseRebuilds {
		t.Errorf("校验失败不应重建策略，rebuilds = %d, want %d", perm.rebuilds, baseRebuilds)
	}

	// 落库的仍是原值
	list, _ := svc.List(ctx)
	for _, m := range list {
		if m.ID == rootID && (m.ParentID != 0 || m.Name != "根") {
			t.Errorf("校验失败后根行被改动: %+v", m)
		}
	}
}

// 合法改挂：根节点改挂到另一个目录下 → 允许。
func TestMenuUpdate_ReparentToSibling(t *testing.T) {
	svc, _ := setupMenuService(t)
	ctx := tenant.WithTenantID(context.Background(), 1)

	a := createMenuRow(t, svc, ctx, &model.Menu{Type: "dir", Name: "A", Status: 1})
	b := createMenuRow(t, svc, ctx, &model.Menu{Type: "dir", Name: "B", Status: 1})
	m := createMenuRow(t, svc, ctx, &model.Menu{Type: "menu", Name: "M", ParentID: a, Status: 1})

	if _, err := svc.Update(ctx, &model.Menu{ID: m, ParentID: b, Type: "menu", Name: "M", Status: 1}); err != nil {
		t.Fatalf("reparent: %v", err)
	}
	list, _ := svc.List(ctx)
	for _, row := range list {
		if row.ID == m && row.ParentID != b {
			t.Errorf("parent_id = %d, want %d", row.ParentID, b)
		}
	}
}

// 不存在 / 缺 id → 报错。
func TestMenuUpdate_InvalidID(t *testing.T) {
	svc, _ := setupMenuService(t)
	ctx := tenant.WithTenantID(context.Background(), 1)

	if _, err := svc.Update(ctx, &model.Menu{ID: 0}); err == nil {
		t.Error("id=0: want error, got nil")
	}
	if _, err := svc.Update(ctx, nil); err == nil {
		t.Error("nil menu: want error, got nil")
	}
	if _, err := svc.Update(ctx, &model.Menu{ID: 999, Name: "x", Status: 1}); err == nil {
		t.Error("id=999: want error, got nil")
	}
}
