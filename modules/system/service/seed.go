package service

import (
	"context"
	"strings"
	"time"

	"github.com/casbin/casbin/v2"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/repository"
	"alexGo-cloud/pkg/auth"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/logger"
	"alexGo-cloud/pkg/token"
)

type SeederParams struct {
	fx.In

	LC       fx.Lifecycle
	DB       *gorm.DB
	Cfg      *config.Config
	Users    repository.UserRepository
	Roles    repository.RoleRepository
	Menus    repository.MenuRepository
	UserRole repository.UserRoleRepository
	RoleMenu repository.RoleMenuRepository
	// Perm 用于把种子角色/菜单按统一路径重灌 Casbin p 策略（与 AssignMenus 同源，避免两套逻辑漂移）。
	Perm     PermissionService `optional:"true"`
	Enforcer *casbin.Enforcer  `optional:"true"`
}

func StartSeeder(p SeederParams) {
	p.LC.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				_ = seed(ctx, p)
			}()
			return nil
		},
	})
}

func seed(ctx context.Context, p SeederParams) error {
	if p.Cfg == nil {
		return nil
	}

	username := p.Cfg.System.DefaultAdminUsername
	password := p.Cfg.System.DefaultAdminPassword
	roleCode := p.Cfg.System.DefaultAdminRole
	if username == "" || password == "" || roleCode == "" {
		return nil
	}

	tid := uint64(0)
	now := time.Now()

	u, err := p.Users.GetByUsername(ctx, tid, username)
	if err != nil {
		hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		u = &model.User{
			Username:     username,
			Nickname:     "管理员",
			PasswordHash: string(hash),
			Status:       1,
			TenantID:     tid,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		_ = p.Users.Create(ctx, u)
	}

	r, err := p.Roles.GetByCode(ctx, tid, roleCode)
	if err != nil {
		r = &model.Role{
			Code:      roleCode,
			Name:      "管理员",
			Type:      1, // 系统内置：删除守卫（type=1 不可删）依赖此值
			DataScope: 1, // 全部数据范围：T10 加载器只认 1-5，0 会退化为仅本人
			Status:    1,
			TenantID:  tid,
			CreatedAt: now,
			UpdatedAt: now,
		}
		_ = p.Roles.Create(ctx, r)
	}

	_ = p.UserRole.SetUserRoles(ctx, tid, u.ID, []uint64{r.ID})

	menus, _ := p.Menus.List(ctx, tid)
	// 迁移 20261009000001 先于本函数插入工作台/订单 5 行，fresh 库 len(menus)!=0
	// 会被误判"已播种"→ 11 个系统菜单永远缺失（Option A，spec errata 4）。
	if !hasSystemDir(menus) {
		root := &model.Menu{
			ParentID:   0,
			Type:       "dir",
			Name:       "系统管理",
			Path:       "/system",
			Component:  "",
			Icon:       "lucide:settings",
			Permission: "system",
			Sort:       10,
			Status:     1,
			TenantID:   tid,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		_ = p.Menus.Create(ctx, root)

		users := &model.Menu{
			ParentID:   root.ID,
			Type:       "menu",
			Name:       "用户管理",
			Path:       "/system/users",
			Component:  "views/system/SystemUsersPage",
			Icon:       "lucide:user",
			Permission: "system:user:list",
			Sort:       1,
			Status:     1,
			TenantID:   tid,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		_ = p.Menus.Create(ctx, users)

		roles := &model.Menu{
			ParentID:   root.ID,
			Type:       "menu",
			Name:       "角色管理",
			Path:       "/system/roles",
			Component:  "views/system/SystemRolesPage",
			Icon:       "lucide:users",
			Permission: "system:role:*",
			Sort:       2,
			Status:     1,
			TenantID:   tid,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		_ = p.Menus.Create(ctx, roles)

		menus := &model.Menu{
			ParentID:   root.ID,
			Type:       "menu",
			Name:       "菜单管理",
			Path:       "/system/menus",
			Component:  "views/system/SystemMenusPage",
			Icon:       "lucide:list-tree",
			Permission: "system:menu:*",
			Sort:       3,
			Status:     1,
			TenantID:   tid,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		_ = p.Menus.Create(ctx, menus)

		depts := &model.Menu{
			ParentID:   root.ID,
			Type:       "menu",
			Name:       "部门管理",
			Path:       "/system/depts",
			Component:  "views/system/SystemDeptsPage",
			Icon:       "lucide:building-2",
			Permission: "system:dept:*",
			Sort:       4,
			Status:     1,
			TenantID:   tid,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		_ = p.Menus.Create(ctx, depts)

		posts := &model.Menu{
			ParentID:   root.ID,
			Type:       "menu",
			Name:       "岗位管理",
			Path:       "/system/posts",
			Component:  "views/system/SystemPostsPage",
			Icon:       "lucide:id-card",
			Permission: "system:post:*",
			Sort:       5,
			Status:     1,
			TenantID:   tid,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		_ = p.Menus.Create(ctx, posts)

		dicts := &model.Menu{
			ParentID:   root.ID,
			Type:       "menu",
			Name:       "字典管理",
			Path:       "/system/dict",
			Component:  "views/system/SystemDictPage",
			Icon:       "lucide:book-open",
			Permission: "system:dict:*",
			Sort:       6,
			Status:     1,
			TenantID:   tid,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		_ = p.Menus.Create(ctx, dicts)

		cfgs := &model.Menu{
			ParentID:   root.ID,
			Type:       "menu",
			Name:       "系统参数",
			Path:       "/system/configs",
			Component:  "views/system/SystemConfigsPage",
			Icon:       "lucide:settings-2",
			Permission: "system:config:*",
			Sort:       7,
			Status:     1,
			TenantID:   tid,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		_ = p.Menus.Create(ctx, cfgs)

		notices := &model.Menu{
			ParentID:   root.ID,
			Type:       "menu",
			Name:       "通知公告",
			Path:       "/system/notices",
			Component:  "views/system/SystemNoticesPage",
			Icon:       "lucide:bell",
			Permission: "system:notice:*",
			Sort:       8,
			Status:     1,
			TenantID:   tid,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		_ = p.Menus.Create(ctx, notices)

		loginLogs := &model.Menu{
			ParentID:   root.ID,
			Type:       "menu",
			Name:       "登录日志",
			Path:       "/system/logins",
			Component:  "views/system/SystemLoginLogsPage",
			Icon:       "lucide:history",
			Permission: "system:log:login",
			Sort:       9,
			Status:     1,
			TenantID:   tid,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		_ = p.Menus.Create(ctx, loginLogs)

		operateLogs := &model.Menu{
			ParentID:   root.ID,
			Type:       "menu",
			Name:       "操作日志",
			Path:       "/system/operates",
			Component:  "views/system/SystemOperateLogsPage",
			Icon:       "lucide:file-text",
			Permission: "system:log:operate",
			Sort:       10,
			Status:     1,
			TenantID:   tid,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		_ = p.Menus.Create(ctx, operateLogs)
	}

	allMenus, _ := p.Menus.List(ctx, tid)

	// 按钮菜单补齐（T9/C3 移交）：permissionRoutes 含 system:auth / system:tenant /
	// member:user 三组路由；若无任何菜单的 permPrefix 命中它们，启动重灌后这些路由会全员 403
	//（C3：租户管理与会员管理接口开箱不可用）。
	// 幂等：任一菜单 permPrefix 已命中则跳过，兼容已播种过菜单的库；
	// 三组均进入 allMenus → SetRoleMenus（admin 种子角色拿到全部管理接口）。
	var rootID uint64
	for _, m := range allMenus {
		// 迁移先插了 /dashboard、/order 两个 ParentID==0 目录——按"第一个 dir"取
		// 会把 3 个按钮挂到工作台下；锚定 /system 目录本身。
		if m.Path == "/system" && strings.EqualFold(m.Type, "dir") {
			rootID = m.ID
			break
		}
	}
	for _, b := range []struct {
		perm string
		name string
		icon string
		sort int
	}{
		{perm: "system:auth:profile", name: "登录鉴权", icon: "key", sort: 11},
		{perm: "system:tenant:manage", name: "租户管理", icon: "cluster", sort: 12},
		{perm: "member:user:manage", name: "会员管理", icon: "team", sort: 13},
	} {
		// permPrefix 取前两段（"system:auth:profile"→"system:auth"），比对必须同尺度，
		// 否则永远不命中 → 每次启动重复插菜单（破坏幂等）。
		if hasPermPrefix(allMenus, permPrefix(b.perm)) {
			continue
		}
		btn := &model.Menu{
			ParentID:   rootID,
			Type:       "button",
			Name:       b.name,
			Path:       "",
			Component:  "",
			Icon:       b.icon,
			Permission: b.perm,
			Sort:       b.sort,
			Status:     1,
			TenantID:   tid,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if err := p.Menus.Create(ctx, btn); err == nil {
			allMenus = append(allMenus, btn)
		}
	}

	menuIDs := make([]uint64, 0, len(allMenus))
	for _, m := range allMenus {
		menuIDs = append(menuIDs, m.ID)
	}
	_ = p.RoleMenu.SetRoleMenus(ctx, tid, r.ID, menuIDs)

	if p.Enforcer != nil {
		_ = p.Enforcer.LoadPolicy()
		// 种子角色策略走统一重建（与 AssignMenus 同一路径，避免两套逻辑漂移）；
		// 移除旧的裸 sub 通配策略 roleCode|"/api/admin/*"（会绕过前缀隔离）。
		if p.Perm != nil {
			if rerr := p.Perm.RebuildPolicies(ctx); rerr != nil && logger.Log != nil {
				logger.Log.Warn("rebuild policies failed", zap.Error(rerr))
			}
		}
		// 用户→角色 g 绑定带租户 + user_type 维度（{tid}:{ut}:{username} → {tid}:{roleCode}），
		// 与中间件 sub 同构（C1：种子管理员是 user_type=1，缺维度会被 member 昵称撞 sub 提权）。
		if _, aerr := p.Enforcer.AddRoleForUser(
			auth.UserSub(tid, int(token.UserTypeAdmin), username, 0),
			roleSub(tid, roleCode),
		); aerr != nil && logger.Log != nil {
			logger.Log.Warn("seed role link failed", zap.Error(aerr))
		}
		if serr := p.Enforcer.SavePolicy(); serr != nil && logger.Log != nil {
			logger.Log.Warn("save policy failed", zap.Error(serr))
		}
	}

	if logger.Log != nil {
		logger.Log.Info("seed default admin done")
	}

	return nil
}

// hasPermPrefix 判断菜单集中是否已有任一菜单的 permission 前缀命中给定前缀
//（permPrefix 取前两段，故 "system:auth:profile" 与 "system:auth:refresh" 同组）。
func hasPermPrefix(menus []*model.Menu, prefix string) bool {
	for _, m := range menus {
		if m != nil && permPrefix(m.Permission) == prefix {
			return true
		}
	}
	return false
}

// hasSystemDir 判断 /system 目录是否已存在——seed 菜单创建门控（Option A）。
// 迁移可在 seed 之前向 menus 插行（工作台/订单），"表非空"不再等于"已播种"。
func hasSystemDir(menus []*model.Menu) bool {
	for _, m := range menus {
		if m != nil && m.Path == "/system" && strings.EqualFold(m.Type, "dir") {
			return true
		}
	}
	return false
}
