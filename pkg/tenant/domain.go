package tenant

import (
	"context"

	"gorm.io/gorm"
)

// NewDomainLookup 基于 tenants 表实现域名解析（表全局，故直接查库，
// 不依赖 modules 层——system 与 member 两服务都能注入同一实现）。
//
// 过滤条件与租户可用性一致：deleted=0 且 status=1；
// 未命中返回 (0, nil)——表示“该域名无租户”，走平台租户历史行为。
//
// 注意：该 raw 查询无模型（st.Schema == nil），gorm 隔离插件天然不参与；
// 即便参与，tenants 也在插件白名单内（cmd/main.go 注册时 ExemptTables 含 "tenants"）。
func NewDomainLookup(db *gorm.DB) DomainLookup {
	return func(ctx context.Context, host string) (uint64, error) {
		if db == nil || host == "" {
			return 0, nil
		}
		var id uint64
		err := db.WithContext(ctx).
			Raw("SELECT id FROM tenants WHERE domain = ? AND deleted = 0 AND status = 1", host).
			Scan(&id).Error
		if err != nil {
			return 0, err
		}
		return id, nil
	}
}
