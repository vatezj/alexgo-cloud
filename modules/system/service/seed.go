package service

import (
	"context"
	"time"

	"github.com/casbin/casbin/v2"
	"golang.org/x/crypto/bcrypt"
	"go.uber.org/fx"
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
	Enforcer *casbin.Enforcer `optional:"true"`
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
			Status:    1,
			TenantID:   tid,
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
	menuIDs := make([]uint64, 0, len(allMenus))
	for _, m := range allMenus {
		menuIDs = append(menuIDs, m.ID)
	}
	_ = p.RoleMenu.SetRoleMenus(ctx, tid, r.ID, menuIDs)

	if p.Enforcer != nil {
		_ = p.Enforcer.LoadPolicy()
		_, _ = p.Enforcer.AddPolicy(roleCode, "/api/admin/*", ".*")
		_, _ = p.Enforcer.AddRoleForUser(username, roleCode)
		_ = p.Enforcer.SavePolicy()
	}

	if logger.Log != nil {
		logger.Log.Info("seed default admin done")
	}

	return nil
}
