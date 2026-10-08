package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/casbin/casbin/v2"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/repository"
	"alexGo-cloud/pkg/tenant"
)

type RoleService interface {
	List(ctx context.Context) ([]*model.Role, error)
	Create(ctx context.Context, p RoleCreateParams) (*model.Role, error)
	Delete(ctx context.Context, id uint64) error
	AssignMenus(ctx context.Context, roleID uint64, menuIDs []uint64) error
}

// RoleCreateParams 角色创建参数。
// DataScope 合法域 1-5（1全部 2自定义 3本部门 4本部门及以下 5仅本人），0 → 默认 1；
// Type 一律 2=自定义（一期不开放系统内置角色的创建）；Status 恒 1。
type RoleCreateParams struct {
	Code             string
	Name             string
	Remark           string
	Sort             int
	DataScope        int
	DataScopeDeptIDs string
}

type MenuService interface {
	List(ctx context.Context) ([]*model.Menu, error)
	Create(ctx context.Context, m *model.Menu) (*model.Menu, error)
	Delete(ctx context.Context, id uint64) error
	Tree(ctx context.Context, menus []*model.Menu) []*model.Menu
}

type PermissionService interface {
	UserRoles(ctx context.Context, userID uint64) ([]*model.Role, error)
	UserMenus(ctx context.Context, userID uint64) ([]*model.Menu, error)
	UserPermCodes(ctx context.Context, userID uint64) ([]string, error)
	UserRoutes(ctx context.Context, userID uint64) ([]*VbenRoute, error)
	EnsureUserRolePolicy(ctx context.Context, username string, roles []*model.Role) error
	// RebuildPolicies：按当前 role_menus 全量重建 Casbin p 策略（启动/菜单变更用）。
	RebuildPolicies(ctx context.Context) error
	// RebuildRolePolicies：单角色重建（AssignMenus/Delete 触发）。
	RebuildRolePolicies(ctx context.Context, roleID uint64) error
}

type roleService struct {
	roleRepo     repository.RoleRepository
	roleMenuRepo repository.RoleMenuRepository
	permSvc      PermissionService
}

type menuService struct {
	menuRepo repository.MenuRepository
	permSvc  PermissionService
}

type permissionService struct {
	roleRepo     repository.RoleRepository
	menuRepo     repository.MenuRepository
	userRoleRepo repository.UserRoleRepository
	roleMenuRepo repository.RoleMenuRepository
	enforcer     *casbin.Enforcer
}

// NewRoleService 注入 permSvc 用于角色变更后重建 Casbin 策略。
// 无环：NewPermissionService 依赖仓储与 enforcer，不依赖 roleService。
func NewRoleService(roleRepo repository.RoleRepository, roleMenuRepo repository.RoleMenuRepository, permSvc PermissionService) RoleService {
	return &roleService{roleRepo: roleRepo, roleMenuRepo: roleMenuRepo, permSvc: permSvc}
}

func NewMenuService(menuRepo repository.MenuRepository, permSvc PermissionService) MenuService {
	return &menuService{menuRepo: menuRepo, permSvc: permSvc}
}

func NewPermissionService(
	roleRepo repository.RoleRepository,
	menuRepo repository.MenuRepository,
	userRoleRepo repository.UserRoleRepository,
	roleMenuRepo repository.RoleMenuRepository,
	enforcer *casbin.Enforcer,
) PermissionService {
	return &permissionService{
		roleRepo:     roleRepo,
		menuRepo:     menuRepo,
		userRoleRepo: userRoleRepo,
		roleMenuRepo: roleMenuRepo,
		enforcer:     enforcer,
	}
}

func (s *roleService) List(ctx context.Context) ([]*model.Role, error) {
	tid := tenant.TenantIDFromContext(ctx)
	return s.roleRepo.List(ctx, tid)
}

func (s *roleService) Create(ctx context.Context, p RoleCreateParams) (*model.Role, error) {
	if p.Code == "" || p.Name == "" {
		return nil, fmt.Errorf("code/name required")
	}
	// data_scope 默认 1（全部），超出 1-5 合法域则拒绝
	if p.DataScope == 0 {
		p.DataScope = 1
	}
	if p.DataScope < 1 || p.DataScope > 5 {
		return nil, fmt.Errorf("invalid data_scope: %d", p.DataScope)
	}
	tid := tenant.TenantIDFromContext(ctx)
	// 同租户内 code 唯一
	if _, err := s.roleRepo.GetByCode(ctx, tid, p.Code); err == nil {
		return nil, fmt.Errorf("role code already exists")
	}
	now := time.Now()
	// type 恒 2=自定义，status 恒 1=启用
	r := &model.Role{
		Code: p.Code, Name: p.Name, Remark: p.Remark, Sort: p.Sort,
		DataScope: p.DataScope, DataScopeDeptIDs: p.DataScopeDeptIDs,
		Type: 2, Status: 1, TenantID: tid,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.roleRepo.Create(ctx, r); err != nil {
		return nil, err
	}
	return r, nil
}

// Delete 删除角色；系统内置角色（type=1）禁止删除。
// 删除成功后全量重建 Casbin 策略（角色已没，其 p 策略随 RemoveFilteredPolicy 清除）。
func (s *roleService) Delete(ctx context.Context, id uint64) error {
	tid := tenant.TenantIDFromContext(ctx)
	r, err := s.roleRepo.GetByID(ctx, tid, id)
	if err != nil {
		return err
	}
	if r.Type == 1 {
		return fmt.Errorf("system role (type=1) cannot be deleted")
	}
	if err := s.roleRepo.Delete(ctx, tid, id); err != nil {
		return err
	}
	if s.permSvc != nil {
		return s.permSvc.RebuildPolicies(ctx)
	}
	return nil
}

// AssignMenus 分配菜单后单角色重建 Casbin 策略（菜单增减立即反映到路由权限）。
func (s *roleService) AssignMenus(ctx context.Context, roleID uint64, menuIDs []uint64) error {
	tid := tenant.TenantIDFromContext(ctx)
	if err := s.roleMenuRepo.SetRoleMenus(ctx, tid, roleID, menuIDs); err != nil {
		return err
	}
	if s.permSvc != nil {
		return s.permSvc.RebuildRolePolicies(ctx, roleID)
	}
	return nil
}

func (s *menuService) List(ctx context.Context) ([]*model.Menu, error) {
	tid := tenant.TenantIDFromContext(ctx)
	return s.menuRepo.List(ctx, tid)
}

func (s *menuService) Create(ctx context.Context, m *model.Menu) (*model.Menu, error) {
	if m == nil {
		return nil, fmt.Errorf("menu nil")
	}
	tid := tenant.TenantIDFromContext(ctx)
	now := time.Now()
	m.TenantID = tid
	if m.Status == 0 {
		m.Status = 1
	}
	m.CreatedAt = now
	m.UpdatedAt = now
	if err := s.menuRepo.Create(ctx, m); err != nil {
		return nil, err
	}
	if s.permSvc != nil {
		if err := s.permSvc.RebuildPolicies(ctx); err != nil {
			return nil, err
		}
	}
	return m, nil
}

// Delete 删除菜单后全量重建 Casbin 策略（被删菜单的权限随之失效，防残留）。
func (s *menuService) Delete(ctx context.Context, id uint64) error {
	tid := tenant.TenantIDFromContext(ctx)
	if err := s.menuRepo.Delete(ctx, tid, id); err != nil {
		return err
	}
	if s.permSvc != nil {
		return s.permSvc.RebuildPolicies(ctx)
	}
	return nil
}

func (s *menuService) Tree(ctx context.Context, menus []*model.Menu) []*model.Menu {
	if len(menus) == 0 {
		return nil
	}
	children := map[uint64][]*model.Menu{}
	var roots []*model.Menu
	for _, m := range menus {
		if m.ParentID == 0 {
			roots = append(roots, m)
			continue
		}
		children[m.ParentID] = append(children[m.ParentID], m)
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i].Sort < roots[j].Sort })
	var walk func(n *model.Menu)
	walk = func(n *model.Menu) {
		kids := children[n.ID]
		sort.Slice(kids, func(i, j int) bool { return kids[i].Sort < kids[j].Sort })
		for _, c := range kids {
			walk(c)
		}
	}
	for _, r := range roots {
		walk(r)
	}
	return roots
}

func (s *permissionService) UserRoles(ctx context.Context, userID uint64) ([]*model.Role, error) {
	tid := tenant.TenantIDFromContext(ctx)
	roleIDs, err := s.userRoleRepo.ListRoleIDsByUser(ctx, tid, userID)
	if err != nil {
		return nil, err
	}
	if len(roleIDs) == 0 {
		return nil, nil
	}
	all, err := s.roleRepo.List(ctx, tid)
	if err != nil {
		return nil, err
	}
	m := map[uint64]*model.Role{}
	for _, r := range all {
		m[r.ID] = r
	}
	var out []*model.Role
	for _, id := range roleIDs {
		if r, ok := m[id]; ok {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *permissionService) UserMenus(ctx context.Context, userID uint64) ([]*model.Menu, error) {
	tid := tenant.TenantIDFromContext(ctx)
	roleIDs, err := s.userRoleRepo.ListRoleIDsByUser(ctx, tid, userID)
	if err != nil {
		return nil, err
	}
	menuIDs, err := s.roleMenuRepo.ListMenuIDsByRoleIDs(ctx, tid, roleIDs)
	if err != nil {
		return nil, err
	}
	all, err := s.menuRepo.List(ctx, tid)
	if err != nil {
		return nil, err
	}
	if len(menuIDs) == 0 {
		return nil, nil
	}
	set := map[uint64]bool{}
	for _, id := range menuIDs {
		set[id] = true
	}
	var out []*model.Menu
	for _, m := range all {
		if set[m.ID] {
			out = append(out, m)
		}
	}
	return out, nil
}

func (s *permissionService) UserPermCodes(ctx context.Context, userID uint64) ([]string, error) {
	menus, err := s.UserMenus(ctx, userID)
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, m := range menus {
		if m.Permission != "" {
			set[m.Permission] = true
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

func (s *permissionService) UserRoutes(ctx context.Context, userID uint64) ([]*VbenRoute, error) {
	menus, err := s.UserMenus(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(menus) == 0 {
		return nil, nil
	}

	var enabled []*model.Menu
	for _, m := range menus {
		if m.Status == 1 {
			enabled = append(enabled, m)
		}
	}

	children := map[uint64][]*model.Menu{}
	for _, m := range enabled {
		children[m.ParentID] = append(children[m.ParentID], m)
	}
	for k := range children {
		sort.Slice(children[k], func(i, j int) bool {
			if children[k][i].Sort == children[k][j].Sort {
				return children[k][i].ID < children[k][j].ID
			}
			return children[k][i].Sort < children[k][j].Sort
		})
	}

	var build func(parentID uint64) []*VbenRoute
	build = func(parentID uint64) []*VbenRoute {
		ms := children[parentID]
		if len(ms) == 0 {
			return nil
		}
		var out []*VbenRoute
		for _, m := range ms {
			if strings.EqualFold(m.Type, "button") {
				continue
			}
			r := &VbenRoute{
				Path:      m.Path,
				Name:      routeName(m.Name, m.Path, m.ID),
				Component: routeComponent(m),
				Meta: VbenRouteMeta{
					Title:   m.Name,
					Icon:    m.Icon,
					OrderNo: m.Sort,
				},
			}
			if m.Permission != "" {
				r.Meta.Permissions = []string{m.Permission}
			}
			r.Children = build(m.ID)
			out = append(out, r)
		}
		return out
	}

	return build(0), nil
}

// RebuildPolicies 按 role_menus **全局**重建 Casbin p 策略。
// 重建是全局一致性操作（RebuildAllPolicies 先清空全部 p），故恒取全租户 scope：
// roles 用 ListAll、menus 用 ListAll，完全忽略 ctx 的租户编号——否则单租户的
// 菜单/角色变更会误删其他租户的策略。逐角色装配菜单时仍用该角色自身 tid
// 查 role_menu（role_menu 行是租户隔离的）。
// 注：ListMenuIDsByRoleIDs 现签名为聚合扁平切片（非 roleID→menuIDs 映射），
// 故按角色逐个查询以获得精确的 role→menus 映射。
func (s *permissionService) RebuildPolicies(ctx context.Context) error {
	if s.enforcer == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	roles, err := s.roleRepo.ListAll(ctx)
	if err != nil {
		return err
	}
	menus, err := s.menuRepo.ListAll(ctx)
	if err != nil {
		return err
	}
	byID := make(map[uint64]*model.Menu, len(menus))
	for _, m := range menus {
		if m != nil {
			byID[m.ID] = m
		}
	}
	roleMenus := make(map[uint64][]*model.Menu, len(roles))
	for _, r := range roles {
		menuIDs, merr := s.roleMenuRepo.ListMenuIDsByRoleIDs(ctx, r.TenantID, []uint64{r.ID})
		if merr != nil {
			return merr
		}
		if len(menuIDs) == 0 {
			continue
		}
		var bound []*model.Menu
		for _, mid := range menuIDs {
			if m, ok := byID[mid]; ok {
				bound = append(bound, m)
			}
		}
		roleMenus[r.ID] = bound
	}
	// 空 roles 也照走：RebuildAllPolicies 会清空全部 p（防残留）后不加任何规则。
	return RebuildAllPolicies(ctx, s.enforcer, roles, roleMenus)
}

// RebuildRolePolicies 单角色重建（AssignMenus 触发）。
// 只清该角色 sub 的 p 再按其菜单重建，不清全局，故不涉跨租户误删；
// 菜单实体用 ListAll 取（按 menuIDs 过滤），避免与 ctx 租户耦合。
func (s *permissionService) RebuildRolePolicies(ctx context.Context, roleID uint64) error {
	if s.enforcer == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	tid := tenant.TenantIDFromContext(ctx)
	r, err := s.roleRepo.GetByID(ctx, tid, roleID)
	if err != nil {
		return err
	}
	menuIDs, err := s.roleMenuRepo.ListMenuIDsByRoleIDs(ctx, r.TenantID, []uint64{r.ID})
	if err != nil {
		return err
	}
	var menus []*model.Menu
	if len(menuIDs) > 0 {
		all, merr := s.menuRepo.ListAll(ctx)
		if merr != nil {
			return merr
		}
		idset := make(map[uint64]bool, len(menuIDs))
		for _, mid := range menuIDs {
			idset[mid] = true
		}
		for _, m := range all {
			if m != nil && idset[m.ID] {
				menus = append(menus, m)
			}
		}
	}
	return rebuildRolePolicies(ctx, s.enforcer, r, menus)
}

// EnsureUserRolePolicy 绑定用户→角色的 g 关系，主体与角色 sub 均带租户前缀
//（{tid}:{username} → {tid}:{roleCode}），与中间件 sub 构造一致。
func (s *permissionService) EnsureUserRolePolicy(ctx context.Context, username string, roles []*model.Role) error {
	if s.enforcer == nil || username == "" {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := loadPolicy(s.enforcer); err != nil {
		return err
	}
	tid := tenant.TenantIDFromContext(ctx)
	userSub := fmt.Sprintf("%d:%s", tid, username)
	existing, err := s.enforcer.GetRolesForUser(userSub)
	if err != nil {
		return err
	}
	for _, r := range existing {
		if _, err := s.enforcer.DeleteRoleForUser(userSub, r); err != nil {
			return err
		}
	}
	for _, r := range roles {
		if _, err := s.enforcer.AddRoleForUser(userSub, roleSub(r.TenantID, r.Code)); err != nil {
			return err
		}
	}
	return savePolicy(s.enforcer)
}

type VbenRoute struct {
	Path      string      `json:"path"`
	Name      string      `json:"name"`
	Component string      `json:"component"`
	Meta      VbenRouteMeta `json:"meta"`
	Children  []*VbenRoute `json:"children,omitempty"`
}

type VbenRouteMeta struct {
	Title       string   `json:"title"`
	Icon        string   `json:"icon,omitempty"`
	OrderNo     int      `json:"orderNo,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

func routeComponent(m *model.Menu) string {
	if strings.EqualFold(m.Type, "dir") {
		return "LAYOUT"
	}
	if m.Component != "" {
		return m.Component
	}
	return "LAYOUT"
}

func routeName(title, path string, id uint64) string {
	base := strings.TrimSpace(path)
	base = strings.Trim(base, "/")
	base = strings.ReplaceAll(base, "/", "_")
	base = strings.ReplaceAll(base, "-", "_")
	base = strings.ReplaceAll(base, ":", "_")
	base = strings.ReplaceAll(base, ".", "_")
	if base == "" {
		base = strings.ReplaceAll(strings.TrimSpace(title), " ", "_")
	}
	if base == "" {
		base = fmt.Sprintf("route_%d", id)
	}
	return base
}
