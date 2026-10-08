package service

import (
	"context"
	"errors"
	"testing"

	"alexGo-cloud/modules/system/model"
)

type memRoleRepo struct {
	byID   map[uint64]*model.Role
	nextID uint64
}

func newMemRoleRepo() *memRoleRepo { return &memRoleRepo{byID: map[uint64]*model.Role{}} }

func (m *memRoleRepo) List(_ context.Context, tid uint64) ([]*model.Role, error) {
	var out []*model.Role
	for _, r := range m.byID {
		if r.TenantID == tid {
			out = append(out, r)
		}
	}
	return out, nil
}
func (m *memRoleRepo) ListAll(context.Context) ([]*model.Role, error) {
	var out []*model.Role
	for _, r := range m.byID {
		out = append(out, r)
	}
	return out, nil
}
func (m *memRoleRepo) GetByID(_ context.Context, tid, id uint64) (*model.Role, error) {
	r, ok := m.byID[id]
	if !ok || r.TenantID != tid {
		return nil, errors.New("not found")
	}
	return r, nil
}
func (m *memRoleRepo) GetByCode(_ context.Context, tid uint64, code string) (*model.Role, error) {
	for _, r := range m.byID {
		if r.TenantID == tid && r.Code == code {
			return r, nil
		}
	}
	return nil, errors.New("not found")
}
func (m *memRoleRepo) Create(_ context.Context, r *model.Role) error {
	m.nextID++
	r.ID = m.nextID
	m.byID[r.ID] = r
	return nil
}
func (m *memRoleRepo) Update(_ context.Context, r *model.Role) error { m.byID[r.ID] = r; return nil }
func (m *memRoleRepo) Delete(_ context.Context, _ uint64, id uint64) error {
	delete(m.byID, id)
	return nil
}

type memRoleMenuRepo struct{}

func (memRoleMenuRepo) SetRoleMenus(context.Context, uint64, uint64, []uint64) error { return nil }
func (memRoleMenuRepo) ListMenuIDsByRoleIDs(context.Context, uint64, []uint64) ([]uint64, error) {
	return nil, nil
}

func TestRoleCreate_DefaultsAndParams(t *testing.T) {
	repo := newMemRoleRepo()
	svc := NewRoleService(repo, memRoleMenuRepo{}, nil)

	r, err := svc.Create(context.Background(), RoleCreateParams{
		Code: "sales", Name: "销售", DataScope: 3, Sort: 5, Remark: "r",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if r.DataScope != 3 || r.Sort != 5 || r.Type != 2 || r.Status != 1 || r.Remark != "r" {
		t.Errorf("role = %+v", r)
	}

	// DataScope=0 → 默认 1（全部）
	r2, err := svc.Create(context.Background(), RoleCreateParams{Code: "ops", Name: "运维"})
	if err != nil {
		t.Fatal(err)
	}
	if r2.DataScope != 1 || r2.Type != 2 {
		t.Errorf("defaults = %+v", r2)
	}

	// 非法 data_scope 拒绝
	if _, err := svc.Create(context.Background(), RoleCreateParams{
		Code: "x", Name: "x", DataScope: 9,
	}); err == nil {
		t.Error("invalid data_scope must fail")
	}
}

// 系统内置角色（type=1）禁止删除。
func TestRoleDelete_SystemRoleGuard(t *testing.T) {
	repo := newMemRoleRepo()
	svc := NewRoleService(repo, memRoleMenuRepo{}, nil)
	r, _ := svc.Create(context.Background(), RoleCreateParams{Code: "sys", Name: "系统"})
	r.Type = 1
	repo.byID[r.ID] = r

	if err := svc.Delete(context.Background(), r.ID); err == nil {
		t.Error("system role (type=1) must not be deletable")
	}
	// 自定义角色可删
	r2, _ := svc.Create(context.Background(), RoleCreateParams{Code: "custom", Name: "自定义"})
	if err := svc.Delete(context.Background(), r2.ID); err != nil {
		t.Errorf("custom role delete: %v", err)
	}
}
