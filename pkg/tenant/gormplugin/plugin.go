// Package gormplugin 为 GORM 挂载租户字段隔离：
// INSERT 自动填 tenant_id，SELECT/UPDATE/DELETE 自动追加 tenant_id 条件。
//
// 设计边界（为什么这样做）：
// - tid=0 不注入：平台/未解析租户保持历史行为（仓库层已有显式过滤，插件是隔离下限不是唯一手段）；
// - 无 TenantID 字段的模型跳过：存量表零影响；
// - 白名单（tenants/casbin_rule 等全局表）跳过；
// - IgnoreTenant 显式放行：平台侧全量查询的唯一通道。
//
// ctx key 约定：租户编号一律复用 pkg/tenant 的 key（WithTenantID/TenantIDFromContext），
// 避免出现两套互不相认的 key；本包只定义自己的 ignoreKey。
package gormplugin

import (
	"context"
	"fmt"
	"reflect"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"

	"alexGo-cloud/pkg/tenant"
)

type ctxKey int

// ignoreKey 是 gormplugin 私有的忽略标记 key（租户编号 key 全部来自 pkg/tenant）。
const ignoreKey ctxKey = iota

// IgnoreTenant 标记该 ctx 的后续 GORM 操作跳过租户注入（平台侧全量查询）。
// 可与 tenant.WithTenantID 叠加使用：即便 ctx 里有租户编号也强制全量。
func IgnoreTenant(ctx context.Context) context.Context {
	return context.WithValue(ctx, ignoreKey, true)
}

// Options 插件注册选项。
type Options struct {
	// ExemptTables 白名单表名（全局表，如 tenants、casbin_rule），完全不参与租户注入。
	ExemptTables []string
}

// ignored 返回该 ctx 是否被 IgnoreTenant 显式放行。
func ignored(ctx context.Context) bool {
	v, _ := ctx.Value(ignoreKey).(bool)
	return v
}

// Register 给 db 挂 Create/Query/Update/Delete 回调：
// Create 仅在 tenant_id 为零值时回填；Query/Update/Delete 追加 tenant_id 等值条件。
func Register(db *gorm.DB, opts Options) error {
	exempt := map[string]bool{}
	for _, t := range opts.ExemptTables {
		exempt[t] = true
	}

	// tenantField 返回需要参与注入的 TenantID 字段；返回 nil 表示该语句应跳过注入。
	// 判定顺序：schema/ctx 就绪 → IgnoreTenant → 白名单 → 模型有无字段 → tid≠0。
	// 表名一律取 st.Schema.Table（回调期 st.Table 可能为空）。
	tenantField := func(st *gorm.Statement) *schema.Field {
		if st == nil || st.Schema == nil || st.Context == nil {
			return nil
		}
		if ignored(st.Context) {
			return nil
		}
		// 表名判定：以 st.Schema.Table 为准（回调期 st.Table 可能为空）；
		// 若调用方显式 .Table(...) 覆盖过，两个名字任一命中白名单都放行
		//（白名单是安全阀，宁可多放不可漏放）。
		if exempt[st.Schema.Table] || (st.Table != "" && exempt[st.Table]) {
			return nil
		}
		f := st.Schema.LookUpField("TenantID")
		if f == nil {
			return nil
		}
		if tenant.TenantIDFromContext(st.Context) == 0 {
			return nil
		}
		return f
	}

	// SELECT/UPDATE/DELETE：追加 tenant_id 条件。
	// 回调时机选 Before("gorm:query/update/delete")：此时 Statement.Schema 已解析、SQL 尚未构建。
	addFilter := func(db *gorm.DB) {
		st := db.Statement
		if tenantField(st) == nil {
			return
		}
		tid := tenant.TenantIDFromContext(st.Context)
		st.AddClause(clause.Where{Exprs: []clause.Expression{
			clause.Eq{Column: clause.Column{Name: "tenant_id"}, Value: tid},
		}})
	}

	// INSERT：tenant_id 为零值时回填（显式赋值的租户编号不覆盖）。
	fillInsert := func(db *gorm.DB) {
		st := db.Statement
		f := tenantField(st)
		if f == nil {
			return
		}
		tid := tenant.TenantIDFromContext(st.Context)
		// 逐行回填：单条 Create 是 struct，批量 Create 是 slice；map 目标跳过（无字段寻址能力）。
		rv := st.ReflectValue
		switch rv.Kind() {
		case reflect.Slice:
			for i := 0; i < rv.Len(); i++ {
				if elem := rv.Index(i); elem.Kind() == reflect.Struct {
					setIfZero(st.Context, f, elem, tid)
				}
			}
		case reflect.Struct:
			setIfZero(st.Context, f, rv, tid)
		}
	}

	if err := db.Callback().Query().Before("gorm:query").Register("tenant:filter_query", addFilter); err != nil {
		return fmt.Errorf("tenant plugin query: %w", err)
	}
	if err := db.Callback().Update().Before("gorm:update").Register("tenant:filter_update", addFilter); err != nil {
		return fmt.Errorf("tenant plugin update: %w", err)
	}
	if err := db.Callback().Delete().Before("gorm:delete").Register("tenant:filter_delete", addFilter); err != nil {
		return fmt.Errorf("tenant plugin delete: %w", err)
	}
	if err := db.Callback().Create().Before("gorm:create").Register("tenant:fill_create", fillInsert); err != nil {
		return fmt.Errorf("tenant plugin create: %w", err)
	}
	return nil
}

// setIfZero 仅当字段当前为零值时写入 v（显式赋值的 tenant_id 不覆盖）。
//
// gorm v1.31 的 Field.ValueOf/Set 是函数字段：
//
//	ValueOf func(context.Context, reflect.Value) (value interface{}, zero bool)
//	Set     func(context.Context, reflect.Value, interface{}) error
//
// 传入的 reflect.Value 为“结构体值”（指针已解引用、可寻址）。
func setIfZero(ctx context.Context, f *schema.Field, rv reflect.Value, v uint64) {
	if f == nil {
		return
	}
	cur, zero := f.ValueOf(ctx, rv)
	if !zero && cur != nil {
		return
	}
	_ = f.Set(ctx, rv, v)
}
