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

	ok, err := e.Enforce("1:admin", "/api/admin/system/users", "GET")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("tenant1 admin must access system/users after assign")
	}

	// 跨租户：同 code 角色不属于租户 2 的用户。
	ok, _ = e.Enforce("2:admin", "/api/admin/system/users", "GET")
	if ok {
		t.Error("tenant2 must NOT inherit tenant1's role policy")
	}

	// 只分配了 user/role 两个权限 → menus 路由不放行。
	ok, _ = e.Enforce("1:admin", "/api/admin/system/menus", "GET")
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

	ok, _ := e.Enforce("1:ops", "/api/admin/system/roles", "GET")
	if ok {
		t.Error("stale policy must be removed on rebuild")
	}
	ok, _ = e.Enforce("1:ops", "/api/admin/system/users", "GET")
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
