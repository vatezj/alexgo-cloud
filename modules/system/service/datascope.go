package service

import (
	"context"
	"fmt"
	"strings"

	"alexGo-cloud/modules/system/repository"
	"alexGo-cloud/pkg/tenant"
)

// DataScopeLoader：tenant.ScopeLoader 的实现——读用户角色的 roles.data_scope，
// 在所有角色里取"最宽松"档（数值越小越宽松：1>2>3>4>5），再按档位补齐部门集合。
//
// 一期取舍：登录后每请求调用、结果不缓存（角色调整即时生效；租户内部门量级小）。
// 构造依赖全部来自 system 模块自己的仓储（module.go Provide）；类型导出仅为让
// module.go 能写 `func(*service.DataScopeLoader) tenant.ScopeLoader` 映射
// （接口 tenant.ScopeLoader 定义在 pkg/tenant，middleware 只消费不实现）。
type DataScopeLoader struct {
	roleQuery    repository.RoleRepository
	userRoleRepo repository.UserRoleRepository
	userRepo     repository.UserRepository
	deptRepo     repository.DeptRepository
}

// NewDataScopeLoader 构造加载器（依赖经 modules/system/module.go 提供，
// 以 tenant.ScopeLoader 接口映射进 Fx 图；member 端不提供 → nil → 不注入）。
func NewDataScopeLoader(
	roleRepo repository.RoleRepository,
	userRoleRepo repository.UserRoleRepository,
	userRepo repository.UserRepository,
	deptRepo repository.DeptRepository,
) *DataScopeLoader {
	return &DataScopeLoader{
		roleQuery:    roleRepo,
		userRoleRepo: userRoleRepo,
		userRepo:     userRepo,
		deptRepo:     deptRepo,
	}
}

// Load 按用户计算数据范围：先取其全部角色，再取最宽松档；
// GetByID 单角色失败只跳过该角色（不让一个坏角色打挂整请求）。
func (l *DataScopeLoader) Load(ctx context.Context, userID, deptID uint64) (tenant.DataScope, error) {
	tid := tenant.TenantIDFromContext(ctx)
	roleIDs, err := l.userRoleRepo.ListRoleIDsByUser(ctx, tid, userID)
	if err != nil {
		return tenant.DataScope{}, err
	}
	// 从最严开始，逐角色放宽：5 → 1 越来越宽松，最终 best 即最宽松档。
	best := 5
	var customIDs string
	for _, rid := range roleIDs {
		r, err := l.roleQuery.GetByID(ctx, tid, rid)
		if err != nil {
			continue
		}
		// 只认 1-5：0（历史数据/未设置）不参与放宽；>5 的越界值同理被 best=5 初值挡住。
		if r.DataScope >= 1 && r.DataScope < best {
			best = r.DataScope
		}
		if r.DataScope == 2 && r.DataScopeDeptIDs != "" {
			customIDs = r.DataScopeDeptIDs
		}
	}
	ds := tenant.DataScope{Mode: best, UserID: userID, DeptID: deptID}
	switch best {
	case 2: // 自定义："1,2,3" 逗号分隔；空集合 → 插件按"看不到"处理
		ds.DeptIDs = parseUintList(customIDs)
	case 3:
		// 本部门：插件直接用 ds.DeptID 等值条件，无需集合
	case 4:
		// 本部门及以下：集合由部门树算好，插件只做 IN
		ds.DeptIDs = l.descendants(ctx, tid, deptID)
	}
	return ds, nil
}

// descendants 从 deptID 出发沿 parent_id 树收集自身+全部后代。
// depts 全量一次查、内存建树——租户内部门量级小，避免递归 SQL（跨库方言差异）。
// deptID=0（未挂部门）或查询失败 → nil。
//
// 注意空集合语义（brief 原文的非对称，勿"顺手修"）：mode 2 空集合 → 插件显式 1=0
// （看不到）；mode 4 空集合 → 插件不加部门条件（租户内不限）。
// 故 dept 查询失败时 mode 4 是 fail-open——该取舍记录在 task-10-report concerns。
func (l *DataScopeLoader) descendants(ctx context.Context, tid, deptID uint64) []uint64 {
	all, err := l.deptRepo.List(ctx, tid)
	if err != nil || deptID == 0 {
		return nil
	}
	children := map[uint64][]uint64{}
	for _, d := range all {
		children[d.ParentID] = append(children[d.ParentID], d.ID)
	}
	out := []uint64{deptID}
	queue := []uint64{deptID}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, ch := range children[cur] {
			out = append(out, ch)
			queue = append(queue, ch)
		}
	}
	return out
}

// parseUintList 解析逗号分隔的部门编号串（容忍空格/空段/非法段/非正数，一律丢弃）。
func parseUintList(s string) []uint64 {
	var out []uint64
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		var v uint64
		if _, err := fmt.Sscanf(p, "%d", &v); err == nil && v > 0 {
			out = append(out, v)
		}
	}
	return out
}
