package tenant

import "context"

// AccountLimitChecker 租户账号额度检查（窄接口）。
// 实现方为 system 模块的 TenantService；member 经 FX 注入消费本接口，
// 避免 member → system 的直接 import（跨模块依赖收敛到共享层 pkg/tenant）。
// 实现方在 system 模块的 FxModule 中 Provide 映射，入口不重复 Provide。
type AccountLimitChecker interface {
	// CheckAccountLimit 校验 tenantID 下账号数未超 account_limit；
	// account_limit<=0（-1 不限）或 tenantID=0（平台租户）直接放行。
	// 触额时返回非 nil error，调用方原样透传。
	CheckAccountLimit(ctx context.Context, tenantID uint64) error
}
