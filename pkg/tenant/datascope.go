package tenant

import "context"

type dsKey int

const dataScopeKey dsKey = iota

// DataScope 请求级数据权限（由 AuthMiddleware 计算后注入，gormplugin 消费）。
//
// 五档语义（Mode）：
//  1. 全部：不加条件
//  2. 自定义：DeptIDs 为显式部门集合（集合为空 → 插件按"看不到"处理）
//  3. 本部门：插件用 DeptID 等值条件
//  4. 本部门及以下：DeptIDs 由 ScopeLoader 沿部门树算好（插件只做 IN）
//  5. 仅本人：插件用 id = UserID
//
// 约定：scope 只作用于 Query（列表可见性下限），Update/Delete 不消费——
// 避免把管理端按主键的维护操作误杀；无 scope 注入则保持老行为（仅租户隔离）。
type DataScope struct {
	Mode    int // 1全部 2自定义 3本部门 4本部门及以下 5仅本人
	UserID  uint64
	DeptID  uint64
	DeptIDs []uint64 // mode=2 的自定义集合 / mode=3,4 计算结果
}

// ScopeLoader 按用户计算"最宽松"数据范围（登录后每请求调用，一期不做缓存）。
//
// 放在 pkg/tenant 与 DataScope 同处：middleware（注入方）与系统模块（实现方）
// 都消费它，避免 system 反向依赖 pkg/middleware 造成依赖方向问题。
// 实现方为 modules/system 的 dataScopeLoader；不提供该依赖时（member 端/未启用）
// 上游以 optional nil 装配，中间件不注入 scope。
type ScopeLoader interface {
	Load(ctx context.Context, userID, deptID uint64) (DataScope, error)
}

// WithDataScope 把本请求的数据权限写入 context。
func WithDataScope(ctx context.Context, ds DataScope) context.Context {
	return context.WithValue(ctx, dataScopeKey, ds)
}

// DataContext 读取数据权限；第二个返回值表示是否注入过（false = 老行为，不过滤）。
func DataContext(ctx context.Context) (DataScope, bool) {
	v, ok := ctx.Value(dataScopeKey).(DataScope)
	return v, ok
}
