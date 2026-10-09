package tenant

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// AccountLimitChecker 租户账号额度检查（窄接口）。
// 跨模块依赖收敛到共享层 pkg/tenant，避免 member → system 的直接 import。
// 实现有两个（按进程装配，互斥出现）：
//   - system 模块 FxModule 映射 TenantService（mono / system-server 进程，入口不重复 Provide）；
//   - NewAccountLimitChecker 的 DB 窄实现（member-server 独立进程，system 模块不在本进程，
//     由 modules/member/cmd 入口 Provide）。
type AccountLimitChecker interface {
	// CheckAccountLimit 校验 tenantID 下账号数未超 account_limit；
	// account_limit<=0（-1 不限）或 tenantID=0（平台租户）直接放行。
	// 触额时返回非 nil error，调用方原样透传。
	CheckAccountLimit(ctx context.Context, tenantID uint64) error
}

// dbAccountLimitChecker 是 AccountLimitChecker 的 DB 窄实现：
// 与 system 侧 TenantService.CheckAccountLimit 同库直查、语义一致——
//   - tid==0（平台租户）直接放行，不查库；
//   - 查 tenants 取 account_limit（deleted=0；行不存在/已软删 → "tenant not found" 错误）；
//   - limit<=0（-1 不限）放行；
//   - 计数 system_users + member_user（tenant_id=tid 且 deleted=0），
//     member 表可能不存在（纯 system 场景）→ 缺表按 0 计，与 system 侧 CountAccounts 同款容错。
type dbAccountLimitChecker struct{ db *gorm.DB }

// NewAccountLimitChecker 构造 DB 实现（返回接口类型，供入口直接 fx.Provide）。
func NewAccountLimitChecker(db *gorm.DB) AccountLimitChecker {
	return &dbAccountLimitChecker{db: db}
}

func (c *dbAccountLimitChecker) CheckAccountLimit(ctx context.Context, tenantID uint64) error {
	if c.db == nil {
		return fmt.Errorf("account limit checker: db not configured")
	}
	if tenantID == 0 {
		return nil // 平台租户不限
	}
	// raw/Pluck 查询无模型（st.Schema == nil），租户隔离插件天然不参与；
	// tenants 本身也在插件 ExemptTables 白名单内（入口注册时声明）。
	var limits []int64
	if err := c.db.WithContext(ctx).Table("tenants").
		Where("id = ? AND deleted = 0", tenantID).
		Pluck("account_limit", &limits).Error; err != nil {
		return err
	}
	if len(limits) == 0 {
		return fmt.Errorf("tenant not found: %d", tenantID)
	}
	limit := limits[0]
	if limit <= 0 {
		return nil // 0/-1 不限
	}
	var n int64
	if err := c.db.WithContext(ctx).Table("system_users").
		Where("tenant_id = ? AND deleted = 0", tenantID).Count(&n).Error; err != nil {
		return err
	}
	var m int64
	if err := c.db.WithContext(ctx).Table("member_user").
		Where("tenant_id = ? AND deleted = 0", tenantID).Count(&m).Error; err == nil {
		n += m // member 缺表按 0 容错（错误吞掉，与 system 侧 CountAccounts 一致）
	}
	if n >= limit {
		return fmt.Errorf("account limit reached (%d)", limit)
	}
	return nil
}
