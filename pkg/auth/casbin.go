package auth

import (
	"fmt"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"gorm.io/gorm"
)

// UserSub 构造 Casbin 用户主体："{tenantID}:{userType}:{name}"；
// name 为空时退化为 "{tenantID}:{userType}:{userID}"（sub 必须非空且稳定）。
//
// 为什么必须带 user_type 维度（C1 提权修复）：member 的 name 取昵称、admin 的 name 取用户名，
// 二者可以相同（如昵称 "admin"）。若 sub 只有 "{tid}:{name}"，成员昵称 "admin" 与种子管理员
// 的用户 sub 撞车 → casbin 恒等 matcher g(x,x)=true → 成员继承管理员全部角色策略（开箱提权）。
// 三处构造点必须同构：中间件 Enforce、system 的 EnsureUserRolePolicy、seed 的 AddRoleForUser；
// 角色侧仍是独立命名空间 roleSub（"{tid}:role:{code}"），不受本函数影响。
//
// userType 用 int 承接 token.UserType 的字面值（1管理员/2会员）：pkg/auth 保持零业务依赖，
// 调用方传 int(token.UserTypeAdmin) 等即可。
func UserSub(tenantID uint64, userType int, name string, userID uint64) string {
	if name == "" {
		return fmt.Sprintf("%d:%d:%d", tenantID, userType, userID)
	}
	return fmt.Sprintf("%d:%d:%s", tenantID, userType, name)
}

// NewCasbinEnforcer 创建 Casbin RBAC 引擎（Model + Adapter）。
//
// Adapter：
// - 使用 gorm-adapter 把 policy 存在数据库中，便于动态管理权限（而不是写死在代码里）。
//
// Model（RBAC）：
// - sub：主体（用户/角色）
// - obj：资源（通常是路由 FullPath，如 /api/admin/system/users）
// - act：动作（HTTP method，如 GET/POST）
//
// 注意：
// - 这里仅创建 Enforcer，不主动加载初始 policy。
// - 生产可在迁移/启动时初始化默认角色与策略，或提供管理接口维护策略。
func NewCasbinEnforcer(db *gorm.DB) (*casbin.Enforcer, error) {
	a, err := gormadapter.NewAdapterByDB(db)
	if err != nil {
		return nil, err
	}

	m, err := model.NewModelFromString(`
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
		return nil, err
	}

	e, err := casbin.NewEnforcer(m, a)
	if err != nil {
		return nil, err
	}
	_ = e.LoadPolicy()
	return e, nil
}
