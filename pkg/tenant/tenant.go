package tenant

import (
	"context"
	"strconv"
)

// ctxKey 用于避免 context key 冲突（推荐用私有类型而不是 string）。
type ctxKey struct{}

// WithTenantID 把 tenantID 写入 context，供下游 Service/Repository/中间件读取。
func WithTenantID(ctx context.Context, tenantID uint64) context.Context {
	return context.WithValue(ctx, ctxKey{}, tenantID)
}

// TenantIDFromContext 从 context 读取 tenantID。
// 如果不存在，则返回 0（代表“未指定租户”）。
func TenantIDFromContext(ctx context.Context) uint64 {
	v := ctx.Value(ctxKey{})
	if id, ok := v.(uint64); ok {
		return id
	}
	return 0
}

// ParseTenantID 把 Header 文本解析为 uint64。
// 解析失败返回 0。
func ParseTenantID(s string) uint64 {
	if s == "" {
		return 0
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}
