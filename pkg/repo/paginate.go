// Package repo 提供 gorm 数据访问公共 helper。
package repo

import (
	"context"

	"gorm.io/gorm"
)

// Paginate 执行分页查询：Count + Offset + Limit + Find。
// Where/Order 由调用方链好；本函数不注入默认 Order。page/size ≤0 时按 1/10 兜底
// （正常路径 controller 侧 Defaults() 已保证，此处仅防御）。
func Paginate[T any](ctx context.Context, q *gorm.DB, page, size int) ([]T, int64, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 10
	}
	var total int64
	if err := q.WithContext(ctx).Session(&gorm.Session{}).Model(new(T)).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []T
	if err := q.WithContext(ctx).Session(&gorm.Session{}).
		Offset((page - 1) * size).
		Limit(size).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
