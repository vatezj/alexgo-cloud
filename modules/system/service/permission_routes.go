package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/casbin/casbin/v2"

	"alexGo-cloud/modules/system/model"
)

// permissionRoutes：permission 前缀 → 后端路由模板（keyMatch2）。
// 一期采用前缀粗粒度（spec §9 决策 8：二期按路由注册表精确到 method）。
// 键必须与菜单 permission 字段的前两段一致；新增 admin 路由时同步维护本表。
//
// 每条“集合路由”额外带一条 `/*` 变体：中间件取的是 gin 的 FullPath()
//（如 "/api/admin/system/users/:id"），而 keyMatch2 的模式在 key2（p.obj）侧，
// 集合路径 "users" 无法匹配 item 路径 "users/:id"——若只留集合精确路径，
// 种子通配策略（"/api/admin/*"）被移除后所有 UPDATE/DELETE/POST :id 路由会 403。
// `/*` 经 keyMatch2 转为 "/.*"，同时覆盖集合与其全部子路径。
var permissionRoutes = map[string][]string{
	"system:user":   {"/api/admin/system/users", "/api/admin/system/users/*"},
	"system:role":   {"/api/admin/system/roles", "/api/admin/system/roles/*"},
	"system:menu":   {"/api/admin/system/menus", "/api/admin/system/menus/*"},
	"system:dept":   {"/api/admin/system/depts", "/api/admin/system/depts/*"},
	"system:post":   {"/api/admin/system/posts", "/api/admin/system/posts/*"},
	"system:dict":   {"/api/admin/system/dict/types", "/api/admin/system/dict/types/*", "/api/admin/system/dict/datas", "/api/admin/system/dict/datas/*"},
	"system:config": {"/api/admin/system/configs", "/api/admin/system/configs/*"},
	"system:notice": {"/api/admin/system/notices", "/api/admin/system/notices/*"},
	"system:log":    {"/api/admin/system/logs/login", "/api/admin/system/logs/operate"},
	"system:auth":   {"/api/admin/system/auth/profile", "/api/admin/system/auth/refresh", "/api/admin/system/auth/logout"},
	"system:tenant": {"/api/admin/system/tenants", "/api/admin/system/tenants/*"},
	"member:user":   {"/api/admin/member/users", "/api/admin/member/users/*"},
}

// permPrefix 取 permission 的前两段作为路由分组键："system:user:list" → "system:user"。
// 段数不足（如 "solo"）时原样返回。
func permPrefix(permission string) string {
	parts := strings.Split(permission, ":")
	if len(parts) >= 2 {
		return parts[0] + ":" + parts[1]
	}
	return permission
}

// roleSub：Casbin 主体一律带租户前缀，杜绝跨租户同 code 串策略。
func roleSub(tenantID uint64, roleCode string) string {
	return fmt.Sprintf("%d:%s", tenantID, roleCode)
}

// savePolicy 落盘；无 adapter（内存 enforcer，单测）时跳过——SavePolicy 对 nil adapter 会 panic。
func savePolicy(e *casbin.Enforcer) error {
	if e == nil || e.GetAdapter() == nil {
		return nil
	}
	return e.SavePolicy()
}

// loadPolicy 从 adapter 载入；无 adapter（内存 enforcer，单测）时跳过——LoadPolicy 对 nil adapter 会 panic。
func loadPolicy(e *casbin.Enforcer) error {
	if e == nil || e.GetAdapter() == nil {
		return nil
	}
	return e.LoadPolicy()
}

// RebuildAllPolicies 全量重建：roleMenus 为 roleID → 该角色的菜单集合。
// 启动重灌与菜单变更调用；调用方负责从 role_menus 表装配 roleMenus。
func RebuildAllPolicies(ctx context.Context, e *casbin.Enforcer,
	roles []*model.Role, roleMenus map[uint64][]*model.Menu) error {
	if e == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, err := e.RemoveFilteredPolicy(0); err != nil {
		return err
	}
	for _, r := range roles {
		if err := rebuildRolePoliciesWithoutClear(ctx, e, r, roleMenus[r.ID]); err != nil {
			return err
		}
	}
	return savePolicy(e)
}

// rebuildRolePolicies 单角色重建：先清掉该角色 sub 的全部旧 p 策略（防残留），再按当前菜单重建。
func rebuildRolePolicies(ctx context.Context, e *casbin.Enforcer, r *model.Role, menus []*model.Menu) error {
	if e == nil || r == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, err := e.RemoveFilteredPolicy(0, roleSub(r.TenantID, r.Code)); err != nil {
		return err
	}
	if err := rebuildRolePoliciesWithoutClear(ctx, e, r, menus); err != nil {
		return err
	}
	return savePolicy(e)
}

// rebuildRolePoliciesWithoutClear 只增不删（供全量重建复用，避免逐角色 SavePolicy）。
// ctx 保留参数以便后续 adapter 操作；当前叶函数不消费，故不做 nil 重赋值（避免 ineffectual assignment）。
func rebuildRolePoliciesWithoutClear(_ context.Context, e *casbin.Enforcer, r *model.Role, menus []*model.Menu) error {
	if e == nil || r == nil {
		return nil
	}
	sub := roleSub(r.TenantID, r.Code)
	seen := map[string]bool{}
	for _, m := range menus {
		if m == nil || m.Permission == "" {
			continue
		}
		routes, ok := permissionRoutes[permPrefix(m.Permission)]
		if !ok {
			continue
		}
		for _, route := range routes {
			key := sub + "|" + route
			if seen[key] {
				continue
			}
			seen[key] = true
			if _, err := e.AddPolicy(sub, route, ".*"); err != nil {
				return err
			}
		}
	}
	return nil
}
