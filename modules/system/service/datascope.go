package service

import (
	"context"
	"strconv"
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
	deptRepo     repository.DeptRepository
}

// NewDataScopeLoader 构造加载器（依赖经 modules/system/module.go 提供，
// 以 tenant.ScopeLoader 接口映射进 Fx 图；member 端不提供 → nil → 不注入）。
// deptID 由中间件从 token claims 传入，不需要 userRepo 查表——不注入无用依赖。
func NewDataScopeLoader(
	roleRepo repository.RoleRepository,
	userRoleRepo repository.UserRoleRepository,
	deptRepo repository.DeptRepository,
) *DataScopeLoader {
	return &DataScopeLoader{
		roleQuery:    roleRepo,
		userRoleRepo: userRoleRepo,
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
// deptID=0（未挂部门）在查库**之前**短路：dept-less 用户每请求不应白付一次全表查询；
// 查询失败 → nil。插件对 mode 4 空集合做 fail-closed（显式 1=0，与 mode 2 空集合同处理）——
// 故 dept 树异常时方向是"更少数据"，不会放开成租户内全量。
//
// seen 判重（环安全）：parent_id 数据可能成环（自指 10→10、互指 20↔21），
// 无 visited 集会让 out/queue 无限增长——mode4 用户的每次请求都会卡死在中间件。
func (l *DataScopeLoader) descendants(ctx context.Context, tid, deptID uint64) []uint64 {
	if deptID == 0 {
		return nil
	}
	all, err := l.deptRepo.List(ctx, tid)
	if err != nil {
		return nil
	}
	children := map[uint64][]uint64{}
	for _, d := range all {
		children[d.ParentID] = append(children[d.ParentID], d.ID)
	}
	seen := map[uint64]bool{deptID: true}
	out := []uint64{deptID}
	queue := []uint64{deptID}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, ch := range children[cur] {
			if seen[ch] {
				continue // 环（含自指/互指）判重，防止中间件死循环
			}
			seen[ch] = true
			out = append(out, ch)
			queue = append(queue, ch)
		}
	}
	return out
}

// parseUintList 解析逗号分隔的部门编号串（容忍空格/空段；严格整段解析——
// "1abc"/"1.5"/溢出 一律整段拒绝，不做前缀部分解析；非正数丢弃）。
func parseUintList(s string) []uint64 {
	var out []uint64
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.ParseUint(p, 10, 64)
		if err == nil && v > 0 {
			out = append(out, v)
		}
	}
	return out
}
