package repository

import (
	"context"

	"gorm.io/gorm"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/pkg/tenant/gormplugin"
)

type RoleRepository interface {
	List(ctx context.Context, tenantID uint64) ([]*model.Role, error)
	// ListAll 返回全部租户的角色（平台侧全量重建 Casbin 策略用，tid=0 入口）。
	ListAll(ctx context.Context) ([]*model.Role, error)
	GetByID(ctx context.Context, tenantID uint64, id uint64) (*model.Role, error)
	GetByCode(ctx context.Context, tenantID uint64, code string) (*model.Role, error)
	Create(ctx context.Context, r *model.Role) error
	Update(ctx context.Context, r *model.Role) error
	Delete(ctx context.Context, tenantID uint64, id uint64) error
}

type MenuRepository interface {
	List(ctx context.Context, tenantID uint64) ([]*model.Menu, error)
	// ListAll 返回全部租户的菜单（全局重建 Casbin 策略用，无租户过滤）。
	ListAll(ctx context.Context) ([]*model.Menu, error)
	GetByID(ctx context.Context, tenantID uint64, id uint64) (*model.Menu, error)
	Create(ctx context.Context, m *model.Menu) error
	Update(ctx context.Context, m *model.Menu) error
	Delete(ctx context.Context, tenantID uint64, id uint64) error
}

type UserRoleRepository interface {
	SetUserRoles(ctx context.Context, tenantID uint64, userID uint64, roleIDs []uint64) error
	ListRoleIDsByUser(ctx context.Context, tenantID uint64, userID uint64) ([]uint64, error)
}

type RoleMenuRepository interface {
	SetRoleMenus(ctx context.Context, tenantID uint64, roleID uint64, menuIDs []uint64) error
	ListMenuIDsByRoleIDs(ctx context.Context, tenantID uint64, roleIDs []uint64) ([]uint64, error)
}

type roleRepo struct{ db *gorm.DB }
type menuRepo struct{ db *gorm.DB }
type userRoleRepo struct{ db *gorm.DB }
type roleMenuRepo struct{ db *gorm.DB }

func NewRoleRepository(db *gorm.DB) RoleRepository { return &roleRepo{db: db} }
func NewMenuRepository(db *gorm.DB) MenuRepository { return &menuRepo{db: db} }
func NewUserRoleRepository(db *gorm.DB) UserRoleRepository { return &userRoleRepo{db: db} }
func NewRoleMenuRepository(db *gorm.DB) RoleMenuRepository { return &roleMenuRepo{db: db} }

func (r *roleRepo) List(ctx context.Context, tenantID uint64) ([]*model.Role, error) {
	var items []*model.Role
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("id desc").Find(&items).Error
	return items, err
}

// ListAll 返回全部租户的角色。IgnoreTenant 确保即便 ctx 带租户编号也不被隔离插件过滤，
// 保证“全量”语义（平台侧重建 Casbin 策略用）。
func (r *roleRepo) ListAll(ctx context.Context) ([]*model.Role, error) {
	var items []*model.Role
	err := r.db.WithContext(gormplugin.IgnoreTenant(ctx)).Find(&items).Error
	return items, err
}

func (r *roleRepo) GetByID(ctx context.Context, tenantID uint64, id uint64) (*model.Role, error) {
	var item model.Role
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *roleRepo) GetByCode(ctx context.Context, tenantID uint64, code string) (*model.Role, error) {
	var item model.Role
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND code = ?", tenantID, code).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *roleRepo) Create(ctx context.Context, item *model.Role) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *roleRepo) Update(ctx context.Context, item *model.Role) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *roleRepo) Delete(ctx context.Context, tenantID uint64, id uint64) error {
	return r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).Delete(&model.Role{}).Error
}

func (r *menuRepo) List(ctx context.Context, tenantID uint64) ([]*model.Menu, error) {
	var items []*model.Menu
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("sort asc, id asc").
		Find(&items).Error
	return items, err
}

// ListAll 返回全部租户的菜单。IgnoreTenant 确保不被隔离插件按 ctx 租户过滤，
// 保证“全量”语义（全局重建 Casbin 策略用）。
func (r *menuRepo) ListAll(ctx context.Context) ([]*model.Menu, error) {
	var items []*model.Menu
	err := r.db.WithContext(gormplugin.IgnoreTenant(ctx)).
		Order("sort asc, id asc").
		Find(&items).Error
	return items, err
}

func (r *menuRepo) GetByID(ctx context.Context, tenantID uint64, id uint64) (*model.Menu, error) {
	var item model.Menu
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *menuRepo) Create(ctx context.Context, item *model.Menu) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *menuRepo) Update(ctx context.Context, item *model.Menu) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *menuRepo) Delete(ctx context.Context, tenantID uint64, id uint64) error {
	return r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).Delete(&model.Menu{}).Error
}

func (r *userRoleRepo) SetUserRoles(ctx context.Context, tenantID uint64, userID uint64, roleIDs []uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tenant_id = ? AND user_id = ?", tenantID, userID).Delete(&model.UserRole{}).Error; err != nil {
			return err
		}
		for _, rid := range roleIDs {
			item := model.UserRole{UserID: userID, RoleID: rid, TenantID: tenantID}
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *userRoleRepo) ListRoleIDsByUser(ctx context.Context, tenantID uint64, userID uint64) ([]uint64, error) {
	var items []model.UserRole
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND user_id = ?", tenantID, userID).Find(&items).Error; err != nil {
		return nil, err
	}
	out := make([]uint64, 0, len(items))
	for _, it := range items {
		out = append(out, it.RoleID)
	}
	return out, nil
}

func (r *roleMenuRepo) SetRoleMenus(ctx context.Context, tenantID uint64, roleID uint64, menuIDs []uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tenant_id = ? AND role_id = ?", tenantID, roleID).Delete(&model.RoleMenu{}).Error; err != nil {
			return err
		}
		for _, mid := range menuIDs {
			item := model.RoleMenu{RoleID: roleID, MenuID: mid, TenantID: tenantID}
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *roleMenuRepo) ListMenuIDsByRoleIDs(ctx context.Context, tenantID uint64, roleIDs []uint64) ([]uint64, error) {
	if len(roleIDs) == 0 {
		return nil, nil
	}
	var items []model.RoleMenu
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND role_id IN ?", tenantID, roleIDs).
		Find(&items).Error; err != nil {
		return nil, err
	}
	out := make([]uint64, 0, len(items))
	for _, it := range items {
		out = append(out, it.MenuID)
	}
	return out, nil
}

