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
}

type roleService struct {
	roleRepo     repository.RoleRepository
	roleMenuRepo repository.RoleMenuRepository
}

type menuService struct {
	menuRepo repository.MenuRepository
}

type permissionService struct {
	roleRepo     repository.RoleRepository
	menuRepo     repository.MenuRepository
	userRoleRepo repository.UserRoleRepository
	roleMenuRepo repository.RoleMenuRepository
	enforcer     *casbin.Enforcer
}

func NewRoleService(roleRepo repository.RoleRepository, roleMenuRepo repository.RoleMenuRepository) RoleService {
	return &roleService{roleRepo: roleRepo, roleMenuRepo: roleMenuRepo}
}

func NewMenuService(menuRepo repository.MenuRepository) MenuService {
	return &menuService{menuRepo: menuRepo}
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
func (s *roleService) Delete(ctx context.Context, id uint64) error {
	tid := tenant.TenantIDFromContext(ctx)
	r, err := s.roleRepo.GetByID(ctx, tid, id)
	if err != nil {
		return err
	}
	if r.Type == 1 {
		return fmt.Errorf("system role (type=1) cannot be deleted")
	}
	return s.roleRepo.Delete(ctx, tid, id)
}

func (s *roleService) AssignMenus(ctx context.Context, roleID uint64, menuIDs []uint64) error {
	tid := tenant.TenantIDFromContext(ctx)
	return s.roleMenuRepo.SetRoleMenus(ctx, tid, roleID, menuIDs)
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
	return m, nil
}

func (s *menuService) Delete(ctx context.Context, id uint64) error {
	tid := tenant.TenantIDFromContext(ctx)
	return s.menuRepo.Delete(ctx, tid, id)
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

func (s *permissionService) EnsureUserRolePolicy(ctx context.Context, username string, roles []*model.Role) error {
	if s.enforcer == nil || username == "" {
		return nil
	}
	_ = s.enforcer.LoadPolicy()
	existing, _ := s.enforcer.GetRolesForUser(username)
	for _, r := range existing {
		_, _ = s.enforcer.DeleteRoleForUser(username, r)
	}
	for _, r := range roles {
		_, _ = s.enforcer.AddRoleForUser(username, r.Code)
	}
	return s.enforcer.SavePolicy()
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
