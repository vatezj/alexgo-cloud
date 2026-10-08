package service

import (
	"context"
	"testing"

	"github.com/casbin/casbin/v2"
	casbinmodel "github.com/casbin/casbin/v2/model"

	"alexGo-cloud/modules/system/model"
)

// 内存 Casbin（与 pkg/auth/casbin.go 同款 matcher），无 DB adapter。
func newMemEnforcer(t *testing.T) *casbin.Enforcer {
	t.Helper()
	m, err := casbinmodel.NewModelFromString(`
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && keyMatch2(r.obj, p.obj) && regexMatch(r.act, p.act)
`)
	if err != nil {
		t.Fatal(err)
	}
	e, err := casbin.NewEnforcer(m)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func menusWithPerms() []*model.Menu {
	return []*model.Menu{
		{ID: 1, Permission: "system:user:list", Type: "button"},
		{ID: 2, Permission: "system:role:list", Type: "button"},
	}
}

// 分配菜单 → 角色立即获得对应路由权限；角色 sub 带租户前缀。
func TestRebuildRolePolicies(t *testing.T) {
	e := newMemEnforcer(t)
	role := &model.Role{ID: 1, Code: "admin", TenantID: 1}

	if err := rebuildRolePolicies(context.Background(), e, role, menusWithPerms()); err != nil {
		t.Fatalf("rebuild error = %v", err)
	}

	ok, err := e.Enforce("1:role:admin", "/api/admin/system/users", "GET")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("tenant1 admin must access system/users after assign")
	}

	// 跨租户：同 code 角色不属于租户 2 的用户。
	ok, _ = e.Enforce("2:role:admin", "/api/admin/system/users", "GET")
	if ok {
		t.Error("tenant2 must NOT inherit tenant1's role policy")
	}

	// 只分配了 user/role 两个权限 → menus 路由不放行。
	ok, _ = e.Enforce("1:role:admin", "/api/admin/system/menus", "GET")
	if ok {
		t.Error("unassigned route must be denied")
	}
}

// 移除权限（重建时菜单变少）→ 旧策略必须清掉（防残留）。
func TestRebuild_ClearsStale(t *testing.T) {
	e := newMemEnforcer(t)
	role := &model.Role{ID: 1, Code: "ops", TenantID: 1}
	_ = rebuildRolePolicies(context.Background(), e, role, menusWithPerms())

	// 只剩 user 权限
	_ = rebuildRolePolicies(context.Background(), e, role, menusWithPerms()[:1])

	ok, _ := e.Enforce("1:role:ops", "/api/admin/system/roles", "GET")
	if ok {
		t.Error("stale policy must be removed on rebuild")
	}
	ok, _ = e.Enforce("1:role:ops", "/api/admin/system/users", "GET")
	if !ok {
		t.Error("current policy must remain")
	}
}

func TestPermPrefix(t *testing.T) {
	cases := map[string]string{
		"system:user:list":   "system:user",
		"system:log:operate": "system:log",
		"member:user:list":   "member:user",
		"solo":               "solo", // 段数不足原样返回
	}
	for in, want := range cases {
		if got := permPrefix(in); got != want {
			t.Errorf("permPrefix(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestRebuildAllPolicies 全量重建路径（C1 回归）：清空各租户陈旧 p、按 roleMenus
// 重建当前规则、不跨租户串、且保留 g 绑定。此前该路径零测试导致 RemoveFilteredPolicy(0)
//（casbin v2.135 拒绝空 fieldValues）带病上线。
func TestRebuildAllPolicies(t *testing.T) {
	e := newMemEnforcer(t)
	// 陈旧策略：tenant1 + tenant2 各一条（重建后必须都被清掉）。
	if _, err := e.AddPolicy("1:role:old", "/api/admin/system/users", ".*"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.AddPolicy("2:role:old", "/api/admin/system/users", ".*"); err != nil {
		t.Fatal(err)
	}
	// 预置一条 g 绑定：p-only clear 证明（重建后必须仍在）。
	if _, err := e.AddRoleForUser("1:alice", "1:role:admin"); err != nil {
		t.Fatal(err)
	}

	roles := []*model.Role{
		{ID: 1, Code: "admin", TenantID: 1},
		{ID: 2, Code: "ops", TenantID: 2},
	}
	roleMenus := map[uint64][]*model.Menu{
		1: {{ID: 1, Permission: "system:user:list", Type: "button"}},
		2: {{ID: 2, Permission: "system:role:list", Type: "button"}},
	}

	if err := RebuildAllPolicies(context.Background(), e, roles, roleMenus); err != nil {
		t.Fatalf("RebuildAllPolicies error = %v", err)
	}

	// 陈旧规则（两个租户）必须清除。
	if ok, _ := e.Enforce("1:role:old", "/api/admin/system/users", "GET"); ok {
		t.Error("stale tenant1 policy must be cleared")
	}
	if ok, _ := e.Enforce("2:role:old", "/api/admin/system/users", "GET"); ok {
		t.Error("stale tenant2 policy must be cleared")
	}
	// 当前规则就位。
	if ok, _ := e.Enforce("1:role:admin", "/api/admin/system/users", "GET"); !ok {
		t.Error("tenant1 admin must reach user route after rebuild")
	}
	if ok, _ := e.Enforce("2:role:ops", "/api/admin/system/roles", "GET"); !ok {
		t.Error("tenant2 ops must reach role route after rebuild")
	}
	// 不跨租户串：tenant2 不能触及仅 tenant1 授予的 user 路由。
	if ok, _ := e.Enforce("2:role:ops", "/api/admin/system/users", "GET"); ok {
		t.Error("cross-tenant bleed: tenant2 must not reach tenant1-only route")
	}
	// g 绑定必须存活（p-only clear 证明）。
	g, err := e.GetGroupingPolicy()
	if err != nil {
		t.Fatal(err)
	}
	if len(g) != 1 || g[0][0] != "1:alice" || g[0][1] != "1:role:admin" {
		t.Errorf("g links must survive rebuild, got %v", g)
	}
}
