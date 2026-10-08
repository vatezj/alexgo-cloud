package auth

import (
	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"gorm.io/gorm"
)

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
