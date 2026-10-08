package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/casbin/casbin/v2"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/repository"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/logger"
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
	if len(menus) == 0 {
		root := &model.Menu{
			ParentID:   0,
			Type:       "dir",
			Name:       "系统管理",
			Path:       "/system",
			Component:  "",
			Icon:       "settings",
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
			Icon:       "user",
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
			Icon:       "team",
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
			Icon:       "menu",
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
			Icon:       "apartment",
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
			Icon:       "idcard",
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
			Icon:       "book",
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
			Icon:       "setting",
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
			Icon:       "bell",
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
			Icon:       "history",
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
			Icon:       "profile",
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

	// system:auth 按钮菜单（T9 移交）：permissionRoutes 含 profile/refresh/logout 三条
	// /auth 路由；若无任何菜单 permission 前缀命中 system:auth，启动重灌后这三条路由会全员 403。
	// 幂等：已存在（任一菜单 permPrefix=="system:auth"）则跳过，兼容已播种过菜单的库。
	hasAuthMenu := false
	for _, m := range allMenus {
		if permPrefix(m.Permission) == "system:auth" {
			hasAuthMenu = true
			break
		}
	}
	if !hasAuthMenu {
		var rootID uint64
		for _, m := range allMenus {
			if m.ParentID == 0 && strings.EqualFold(m.Type, "dir") {
				rootID = m.ID
				break
			}
		}
		authBtn := &model.Menu{
			ParentID:   rootID,
			Type:       "button",
			Name:       "登录鉴权",
			Path:       "",
			Component:  "",
			Icon:       "key",
			Permission: "system:auth:profile",
			Sort:       11,
			Status:     1,
			TenantID:   tid,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if err := p.Menus.Create(ctx, authBtn); err == nil {
			allMenus = append(allMenus, authBtn)
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
		// 用户→角色 g 绑定带租户前缀（{tid}:{username} → {tid}:{roleCode}），与中间件 sub 同构。
		if _, aerr := p.Enforcer.AddRoleForUser(
			fmt.Sprintf("%d:%s", tid, username),
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
