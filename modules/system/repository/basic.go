package repository

import (
	"context"

	"alexGo-cloud/modules/system/model"
	"gorm.io/gorm"
)

type DeptRepository interface {
	List(ctx context.Context, tenantID uint64) ([]*model.Dept, error)
	Create(ctx context.Context, d *model.Dept) error
	Update(ctx context.Context, d *model.Dept) error
	Delete(ctx context.Context, tenantID uint64, id uint64) error
}

type PostRepository interface {
	List(ctx context.Context, tenantID uint64) ([]*model.Post, error)
	Create(ctx context.Context, p *model.Post) error
	Update(ctx context.Context, p *model.Post) error
	Delete(ctx context.Context, tenantID uint64, id uint64) error
}

type DictRepository interface {
	ListTypes(ctx context.Context, tenantID uint64) ([]*model.DictType, error)
	CreateType(ctx context.Context, t *model.DictType) error
	UpdateType(ctx context.Context, t *model.DictType) error
	DeleteType(ctx context.Context, tenantID uint64, id uint64) error
	ListDatas(ctx context.Context, tenantID uint64, typeID uint64) ([]*model.DictData, error)
	CreateData(ctx context.Context, d *model.DictData) error
	UpdateData(ctx context.Context, d *model.DictData) error
	DeleteData(ctx context.Context, tenantID uint64, id uint64) error
}

type ConfigRepository interface {
	List(ctx context.Context, tenantID uint64) ([]*model.SystemConfig, error)
	Create(ctx context.Context, c *model.SystemConfig) error
	Update(ctx context.Context, c *model.SystemConfig) error
	Delete(ctx context.Context, tenantID uint64, id uint64) error
}

type NoticeRepository interface {
	List(ctx context.Context, tenantID uint64) ([]*model.SystemNotice, error)
	Create(ctx context.Context, n *model.SystemNotice) error
	Update(ctx context.Context, n *model.SystemNotice) error
	Delete(ctx context.Context, tenantID uint64, id uint64) error
}

type deptRepo struct{ db *gorm.DB }
type postRepo struct{ db *gorm.DB }
type dictRepo struct{ db *gorm.DB }
type configRepo struct{ db *gorm.DB }
type noticeRepo struct{ db *gorm.DB }

func NewDeptRepository(db *gorm.DB) DeptRepository       { return &deptRepo{db: db} }
func NewPostRepository(db *gorm.DB) PostRepository       { return &postRepo{db: db} }
func NewDictRepository(db *gorm.DB) DictRepository       { return &dictRepo{db: db} }
func NewConfigRepository(db *gorm.DB) ConfigRepository   { return &configRepo{db: db} }
func NewNoticeRepository(db *gorm.DB) NoticeRepository   { return &noticeRepo{db: db} }

func (r *deptRepo) List(ctx context.Context, tenantID uint64) ([]*model.Dept, error) {
	var list []*model.Dept
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("sort asc, id asc").Find(&list).Error
	return list, err
}
func (r *deptRepo) Create(ctx context.Context, d *model.Dept) error { return r.db.WithContext(ctx).Create(d).Error }
func (r *deptRepo) Update(ctx context.Context, d *model.Dept) error { return r.db.WithContext(ctx).Save(d).Error }
func (r *deptRepo) Delete(ctx context.Context, tenantID uint64, id uint64) error {
	return r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).Delete(&model.Dept{}).Error
}

func (r *postRepo) List(ctx context.Context, tenantID uint64) ([]*model.Post, error) {
	var list []*model.Post
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("sort asc, id asc").Find(&list).Error
	return list, err
}
func (r *postRepo) Create(ctx context.Context, p *model.Post) error { return r.db.WithContext(ctx).Create(p).Error }
func (r *postRepo) Update(ctx context.Context, p *model.Post) error { return r.db.WithContext(ctx).Save(p).Error }
func (r *postRepo) Delete(ctx context.Context, tenantID uint64, id uint64) error {
	return r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).Delete(&model.Post{}).Error
}

func (r *dictRepo) ListTypes(ctx context.Context, tenantID uint64) ([]*model.DictType, error) {
	var list []*model.DictType
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("id desc").Find(&list).Error
	return list, err
}
func (r *dictRepo) CreateType(ctx context.Context, t *model.DictType) error { return r.db.WithContext(ctx).Create(t).Error }
func (r *dictRepo) UpdateType(ctx context.Context, t *model.DictType) error { return r.db.WithContext(ctx).Save(t).Error }
func (r *dictRepo) DeleteType(ctx context.Context, tenantID uint64, id uint64) error {
	return r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).Delete(&model.DictType{}).Error
}
func (r *dictRepo) ListDatas(ctx context.Context, tenantID uint64, typeID uint64) ([]*model.DictData, error) {
	var list []*model.DictData
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND type_id = ?", tenantID, typeID).Order("sort asc, id asc").Find(&list).Error
	return list, err
}
func (r *dictRepo) CreateData(ctx context.Context, d *model.DictData) error { return r.db.WithContext(ctx).Create(d).Error }
func (r *dictRepo) UpdateData(ctx context.Context, d *model.DictData) error { return r.db.WithContext(ctx).Save(d).Error }
func (r *dictRepo) DeleteData(ctx context.Context, tenantID uint64, id uint64) error {
	return r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).Delete(&model.DictData{}).Error
}

func (r *configRepo) List(ctx context.Context, tenantID uint64) ([]*model.SystemConfig, error) {
	var list []*model.SystemConfig
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("id desc").Find(&list).Error
	return list, err
}
func (r *configRepo) Create(ctx context.Context, c *model.SystemConfig) error { return r.db.WithContext(ctx).Create(c).Error }
func (r *configRepo) Update(ctx context.Context, c *model.SystemConfig) error { return r.db.WithContext(ctx).Save(c).Error }
func (r *configRepo) Delete(ctx context.Context, tenantID uint64, id uint64) error {
	return r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).Delete(&model.SystemConfig{}).Error
}

func (r *noticeRepo) List(ctx context.Context, tenantID uint64) ([]*model.SystemNotice, error) {
	var list []*model.SystemNotice
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("id desc").Find(&list).Error
	return list, err
}
func (r *noticeRepo) Create(ctx context.Context, n *model.SystemNotice) error { return r.db.WithContext(ctx).Create(n).Error }
func (r *noticeRepo) Update(ctx context.Context, n *model.SystemNotice) error { return r.db.WithContext(ctx).Save(n).Error }
func (r *noticeRepo) Delete(ctx context.Context, tenantID uint64, id uint64) error {
	return r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).Delete(&model.SystemNotice{}).Error
}
