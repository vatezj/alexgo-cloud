package service

import (
	"context"
	"testing"

	"github.com/casbin/casbin/v2"
	casbinmodel "github.com/casbin/casbin/v2/model"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/pkg/auth"
	"alexGo-cloud/pkg/tenant"
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
	// 用户 sub 带 user_type 维度（C1）：{tid}:{ut}:{username}，与中间件/EnsureUserRolePolicy 同构。
	if _, err := e.AddRoleForUser("1:1:alice", "1:role:admin"); err != nil {
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
	if len(g) != 1 || g[0][0] != "1:1:alice" || g[0][1] != "1:role:admin" {
		t.Errorf("g links must survive rebuild, got %v", g)
	}
}

// C3：种子补齐的两组按钮菜单（system:tenant / member:user）必须真的落到 admin 角色的
// p 策略上——此前种子缺这两组 → 租户/会员管理接口开箱全员 403。
// 不跑 seed（unit）：构造与 seed 同形的菜单列 + admin 角色 + g 绑定 → rebuildRolePolicies。
func TestRebuild_TenantAndMemberManageMenus(t *testing.T) {
	e := newMemEnforcer(t)
	role := &model.Role{ID: 1, Code: "admin", TenantID: 1}
	menus := []*model.Menu{
		{ID: 1, Permission: "system:tenant:manage", Type: "button"},
		{ID: 2, Permission: "member:user:manage", Type: "button"},
	}
	if err := rebuildRolePolicies(context.Background(), e, role, menus); err != nil {
		t.Fatalf("rebuild error = %v", err)
	}
	// 管理员用户 sub（user_type=1）经 g 绑定走角色策略，与中间件/seed 同构。
	if _, err := e.AddRoleForUser("1:1:admin", "1:role:admin"); err != nil {
		t.Fatal(err)
	}

	for _, obj := range []string{"/api/admin/system/tenants", "/api/admin/member/users"} {
		ok, err := e.Enforce("1:1:admin", obj, "GET")
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Errorf("admin must GET %s after seed menus assigned", obj)
		}
	}
	// 集合路径的 item 子路径同样放行（keyMatch2 的 /* 变体）。
	if ok, _ := e.Enforce("1:1:admin", "/api/admin/system/tenants/2", "PUT"); !ok {
		t.Error("tenant item route must be covered by /* variant")
	}
	// 只给了这两组菜单 → 未覆盖的路由仍拒（判别：不是通配放行）。
	if ok, _ := e.Enforce("1:1:admin", "/api/admin/system/menus", "GET"); ok {
		t.Error("unassigned route must be denied")
	}
}

// C1：EnsureUserRolePolicy 的用户 sub 必须带 user_type 维度（管理员 ut=1），
// 与中间件 auth.UserSub 同构；否则 member 昵称撞管理员用户名即经 g(x,x) 提权。
func TestEnsureUserRolePolicy_SubHasUserType(t *testing.T) {
	e := newMemEnforcer(t)
	// 仓储在本用例不触达（Ensure 只消费 enforcer + ctx tenant），nil 即可。
	svc := NewPermissionService(nil, nil, nil, nil, e)
	ctx := tenant.WithTenantID(context.Background(), 1)
	roles := []*model.Role{{ID: 1, Code: "admin", TenantID: 1}}

	if err := svc.EnsureUserRolePolicy(ctx, "alice", roles); err != nil {
		t.Fatalf("EnsureUserRolePolicy error = %v", err)
	}
	g, err := e.GetGroupingPolicy()
	if err != nil {
		t.Fatal(err)
	}
	if len(g) != 1 || g[0][0] != "1:1:alice" || g[0][1] != "1:role:admin" {
		t.Errorf("g = %v, want [[1:1:alice 1:role:admin]]", g)
	}
	// member 形态（ut=2）与管理员 sub 永不相等——用 UserSub 形态钉住判别点。
	if auth.UserSub(1, 2, "alice", 0) == g[0][0] {
		t.Error("member sub must differ from admin sub for same name")
	}
}

// order 模块启用后：角色按 "order:order:*" 权限必须拿到 /api/admin/order/orders
//（集合 + item 两形态），且未授权的 system 路由仍拒绝——判别表项真加进去了、
// rebuild 不再对 order 按钮权限静默丢弃。
func TestRebuild_OrderRoutes(t *testing.T) {
	e := newMemEnforcer(t)
	role := &model.Role{ID: 1, Code: "admin", TenantID: 1}
	menus := []*model.Menu{{ID: 1, Permission: "order:order:*", Type: "menu"}}
	if err := rebuildRolePolicies(context.Background(), e, role, menus); err != nil {
		t.Fatalf("rebuild error = %v", err)
	}
	if _, err := e.AddRoleForUser("1:1:admin", "1:role:admin"); err != nil {
		t.Fatal(err)
	}
	for _, obj := range []string{"/api/admin/order/orders", "/api/admin/order/orders/123"} {
		if ok, _ := e.Enforce("1:1:admin", obj, "GET"); !ok {
			t.Errorf("order route %s must be allowed", obj)
		}
	}
	if ok, _ := e.Enforce("1:1:admin", "/api/admin/system/users", "GET"); ok {
		t.Error("unassigned route must be denied")
	}
}
