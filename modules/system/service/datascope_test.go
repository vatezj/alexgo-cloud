package service

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/pkg/tenant"
)

// memUserRoleRepo：UserRoleRepository 测试替身（按 user→roleIDs 内存映射）。
type memUserRoleRepo struct {
	byUser map[uint64][]uint64
}

func (m *memUserRoleRepo) SetUserRoles(_ context.Context, _ uint64, userID uint64, roleIDs []uint64) error {
	m.byUser[userID] = roleIDs
	return nil
}

func (m *memUserRoleRepo) ListRoleIDsByUser(_ context.Context, _, userID uint64) ([]uint64, error) {
	return m.byUser[userID], nil
}

// memDeptRepo：DeptRepository 测试替身。
type memDeptRepo struct {
	depts []*model.Dept
	err   error
}

func (m *memDeptRepo) List(_ context.Context, tid uint64) ([]*model.Dept, error) {
	if m.err != nil {
		return nil, m.err
	}
	var out []*model.Dept
	for _, d := range m.depts {
		if d.TenantID == tid {
			out = append(out, d)
		}
	}
	return out, nil
}
func (m *memDeptRepo) Create(_ context.Context, d *model.Dept) error {
	m.depts = append(m.depts, d)
	return nil
}
func (m *memDeptRepo) Update(_ context.Context, d *model.Dept) error { return nil }
func (m *memDeptRepo) Delete(_ context.Context, _ uint64, _ uint64) error {
	return nil
}

func scopedCtx() context.Context { return tenant.WithTenantID(context.Background(), 1) }

// seed 超管角色（DataScope=1，见 seed.go）→ 加载器得 Mode 1 全部。
func TestDataScopeLoader_SeedAdmin_Mode1(t *testing.T) {
	roles := newMemRoleRepo()
	admin := &model.Role{ID: 1, Code: "admin", DataScope: 1, TenantID: 1}
	if err := roles.Create(scopedCtx(), admin); err != nil {
		t.Fatal(err)
	}
	ur := &memUserRoleRepo{byUser: map[uint64][]uint64{9: {1}}}
	l := NewDataScopeLoader(roles, ur, &memDeptRepo{})

	ds, err := l.Load(scopedCtx(), 9, 77)
	if err != nil {
		t.Fatal(err)
	}
	if ds.Mode != 1 || ds.UserID != 9 || ds.DeptID != 77 {
		t.Errorf("seed admin ds = %+v, want Mode 1", ds)
	}
}

// 多角色取最宽松档（数值越小越宽松）；自定义档附带部门集合。
func TestDataScopeLoader_BestOfRoles_Mode2(t *testing.T) {
	roles := newMemRoleRepo()
	for _, r := range []*model.Role{
		{ID: 1, Code: "a", DataScope: 4, TenantID: 1},
		{ID: 2, Code: "b", DataScope: 2, DataScopeDeptIDs: "20,21", TenantID: 1},
		{ID: 3, Code: "c", DataScope: 5, TenantID: 1},
	} {
		if err := roles.Create(scopedCtx(), r); err != nil {
			t.Fatal(err)
		}
	}
	ur := &memUserRoleRepo{byUser: map[uint64][]uint64{9: {1, 2, 3}}}
	l := NewDataScopeLoader(roles, ur, &memDeptRepo{})

	ds, err := l.Load(scopedCtx(), 9, 10)
	if err != nil {
		t.Fatal(err)
	}
	if ds.Mode != 2 {
		t.Errorf("mode = %d, want 2（最宽松）", ds.Mode)
	}
	if !reflect.DeepEqual(ds.DeptIDs, []uint64{20, 21}) {
		t.Errorf("deptIDs = %v, want [20 21]", ds.DeptIDs)
	}
}

// Mode 4：沿 parent_id 树收集自身+全部后代（内存建树，非本支部门不进集合）。
func TestDataScopeLoader_Mode4_Descendants(t *testing.T) {
	roles := newMemRoleRepo()
	if err := roles.Create(scopedCtx(), &model.Role{ID: 1, Code: "a", DataScope: 4, TenantID: 1}); err != nil {
		t.Fatal(err)
	}
	ur := &memUserRoleRepo{byUser: map[uint64][]uint64{9: {1}}}
	// 树：10 → 11 → 12；20 → 21（旁支）
	dept := &memDeptRepo{depts: []*model.Dept{
		{ID: 10, ParentID: 0, TenantID: 1},
		{ID: 11, ParentID: 10, TenantID: 1},
		{ID: 12, ParentID: 11, TenantID: 1},
		{ID: 20, ParentID: 0, TenantID: 1},
		{ID: 21, ParentID: 20, TenantID: 1},
	}}
	l := NewDataScopeLoader(roles, ur, dept)

	ds, err := l.Load(scopedCtx(), 9, 10)
	if err != nil {
		t.Fatal(err)
	}
	if ds.Mode != 4 {
		t.Errorf("mode = %d, want 4", ds.Mode)
	}
	want := []uint64{10, 11, 12}
	if len(ds.DeptIDs) != len(want) {
		t.Fatalf("descendants = %v, want %v", ds.DeptIDs, want)
	}
	set := map[uint64]bool{}
	for _, id := range ds.DeptIDs {
		set[id] = true
	}
	for _, id := range want {
		if !set[id] {
			t.Errorf("descendants = %v, missing %d", ds.DeptIDs, id)
		}
	}
	if set[20] || set[21] {
		t.Errorf("descendants = %v, must not include sibling branch", ds.DeptIDs)
	}
}

// 无角色 → Mode 5 仅本人（最严兜底）。
func TestDataScopeLoader_NoRoles_Mode5(t *testing.T) {
	l := NewDataScopeLoader(newMemRoleRepo(), &memUserRoleRepo{byUser: map[uint64][]uint64{}},
		&memDeptRepo{})
	ds, err := l.Load(scopedCtx(), 9, 77)
	if err != nil {
		t.Fatal(err)
	}
	if ds.Mode != 5 || ds.UserID != 9 {
		t.Errorf("ds = %+v, want Mode 5 / UserID 9", ds)
	}
}

// Mode 2 但自定义集合为空 → DeptIDs 空（插件侧显式"看不到"）。
func TestDataScopeLoader_Mode2_NoCustomIDs(t *testing.T) {
	roles := newMemRoleRepo()
	if err := roles.Create(scopedCtx(), &model.Role{ID: 1, Code: "a", DataScope: 2, TenantID: 1}); err != nil {
		t.Fatal(err)
	}
	ur := &memUserRoleRepo{byUser: map[uint64][]uint64{9: {1}}}
	l := NewDataScopeLoader(roles, ur, &memDeptRepo{})

	ds, err := l.Load(scopedCtx(), 9, 10)
	if err != nil {
		t.Fatal(err)
	}
	if ds.Mode != 2 || len(ds.DeptIDs) != 0 {
		t.Errorf("ds = %+v, want Mode 2 with empty dept set", ds)
	}
}

// 部门树查询失败 → Mode 4 退化为空集合，插件侧 fail-closed（显式 1=0，
// 见 plugin_test 的 TestDataScope_Mode4Empty_FailsClosed），且不 panic、错误不上抛。
func TestDataScopeLoader_Mode4_DeptListError(t *testing.T) {
	roles := newMemRoleRepo()
	if err := roles.Create(scopedCtx(), &model.Role{ID: 1, Code: "a", DataScope: 4, TenantID: 1}); err != nil {
		t.Fatal(err)
	}
	ur := &memUserRoleRepo{byUser: map[uint64][]uint64{9: {1}}}
	l := NewDataScopeLoader(roles, ur, &memDeptRepo{err: errors.New("db down")})

	ds, err := l.Load(scopedCtx(), 9, 10)
	if err != nil {
		t.Fatal(err)
	}
	if ds.Mode != 4 || len(ds.DeptIDs) != 0 {
		t.Errorf("ds = %+v, want Mode 4 with empty descendants on dept error", ds)
	}
}

func TestParseUintList(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []uint64
	}{
		{"1,2,3", []uint64{1, 2, 3}},
		{" 1 , 2 ,x, ,3 ", []uint64{1, 2, 3}},
		{"", nil},
		{"0,-1,abc", nil},
	} {
		got := parseUintList(tc.in)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("parseUintList(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
