package service

import (
	"context"
	"time"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/repository"
	"alexGo-cloud/pkg/tenant"
)

type DeptService interface {
	List(ctx context.Context) ([]*model.Dept, error)
	Create(ctx context.Context, d *model.Dept) (*model.Dept, error)
	Update(ctx context.Context, d *model.Dept) error
	Delete(ctx context.Context, id uint64) error
}

type PostService interface {
	List(ctx context.Context) ([]*model.Post, error)
	Create(ctx context.Context, p *model.Post) (*model.Post, error)
	Update(ctx context.Context, p *model.Post) error
	Delete(ctx context.Context, id uint64) error
}

type DictService interface {
	ListTypes(ctx context.Context) ([]*model.DictType, error)
	CreateType(ctx context.Context, t *model.DictType) (*model.DictType, error)
	UpdateType(ctx context.Context, t *model.DictType) error
	DeleteType(ctx context.Context, id uint64) error
	ListDatas(ctx context.Context, typeID uint64) ([]*model.DictData, error)
	CreateData(ctx context.Context, d *model.DictData) (*model.DictData, error)
	UpdateData(ctx context.Context, d *model.DictData) error
	DeleteData(ctx context.Context, id uint64) error
}

type ConfigService interface {
	List(ctx context.Context) ([]*model.SystemConfig, error)
	Create(ctx context.Context, c *model.SystemConfig) (*model.SystemConfig, error)
	Update(ctx context.Context, c *model.SystemConfig) error
	Delete(ctx context.Context, id uint64) error
}

type NoticeService interface {
	List(ctx context.Context) ([]*model.SystemNotice, error)
	Create(ctx context.Context, n *model.SystemNotice) (*model.SystemNotice, error)
	Update(ctx context.Context, n *model.SystemNotice) error
	Delete(ctx context.Context, id uint64) error
}

type deptService struct{ repo repository.DeptRepository }
type postService struct{ repo repository.PostRepository }
type dictService struct{ repo repository.DictRepository }
type configService struct{ repo repository.ConfigRepository }
type noticeService struct{ repo repository.NoticeRepository }

func NewDeptService(repo repository.DeptRepository) DeptService       { return &deptService{repo: repo} }
func NewPostService(repo repository.PostRepository) PostService       { return &postService{repo: repo} }
func NewDictService(repo repository.DictRepository) DictService       { return &dictService{repo: repo} }
func NewConfigService(repo repository.ConfigRepository) ConfigService { return &configService{repo: repo} }
func NewNoticeService(repo repository.NoticeRepository) NoticeService { return &noticeService{repo: repo} }

func (s *deptService) List(ctx context.Context) ([]*model.Dept, error) {
	return s.repo.List(ctx, tenant.TenantIDFromContext(ctx))
}
func (s *deptService) Create(ctx context.Context, d *model.Dept) (*model.Dept, error) {
	tid := tenant.TenantIDFromContext(ctx)
	now := time.Now()
	d.TenantID = tid
	if d.Status == 0 {
		d.Status = 1
	}
	d.CreatedAt = now
	d.UpdatedAt = now
	if err := s.repo.Create(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}
func (s *deptService) Update(ctx context.Context, d *model.Dept) error {
	tid := tenant.TenantIDFromContext(ctx)
	d.TenantID = tid
	d.UpdatedAt = time.Now()
	return s.repo.Update(ctx, d)
}
func (s *deptService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, tenant.TenantIDFromContext(ctx), id)
}

func (s *postService) List(ctx context.Context) ([]*model.Post, error) {
	return s.repo.List(ctx, tenant.TenantIDFromContext(ctx))
}
func (s *postService) Create(ctx context.Context, p *model.Post) (*model.Post, error) {
	tid := tenant.TenantIDFromContext(ctx)
	now := time.Now()
	p.TenantID = tid
	if p.Status == 0 {
		p.Status = 1
	}
	p.CreatedAt = now
	p.UpdatedAt = now
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}
func (s *postService) Update(ctx context.Context, p *model.Post) error {
	tid := tenant.TenantIDFromContext(ctx)
	p.TenantID = tid
	p.UpdatedAt = time.Now()
	return s.repo.Update(ctx, p)
}
func (s *postService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, tenant.TenantIDFromContext(ctx), id)
}

func (s *dictService) ListTypes(ctx context.Context) ([]*model.DictType, error) {
	return s.repo.ListTypes(ctx, tenant.TenantIDFromContext(ctx))
}
func (s *dictService) CreateType(ctx context.Context, t *model.DictType) (*model.DictType, error) {
	tid := tenant.TenantIDFromContext(ctx)
	now := time.Now()
	t.TenantID = tid
	if t.Status == 0 {
		t.Status = 1
	}
	t.CreatedAt = now
	t.UpdatedAt = now
	if err := s.repo.CreateType(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}
func (s *dictService) UpdateType(ctx context.Context, t *model.DictType) error {
	tid := tenant.TenantIDFromContext(ctx)
	t.TenantID = tid
	t.UpdatedAt = time.Now()
	return s.repo.UpdateType(ctx, t)
}
func (s *dictService) DeleteType(ctx context.Context, id uint64) error {
	return s.repo.DeleteType(ctx, tenant.TenantIDFromContext(ctx), id)
}
func (s *dictService) ListDatas(ctx context.Context, typeID uint64) ([]*model.DictData, error) {
	return s.repo.ListDatas(ctx, tenant.TenantIDFromContext(ctx), typeID)
}
func (s *dictService) CreateData(ctx context.Context, d *model.DictData) (*model.DictData, error) {
	tid := tenant.TenantIDFromContext(ctx)
	now := time.Now()
	d.TenantID = tid
	if d.Status == 0 {
		d.Status = 1
	}
	d.CreatedAt = now
	d.UpdatedAt = now
	if err := s.repo.CreateData(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}
func (s *dictService) UpdateData(ctx context.Context, d *model.DictData) error {
	tid := tenant.TenantIDFromContext(ctx)
	d.TenantID = tid
	d.UpdatedAt = time.Now()
	return s.repo.UpdateData(ctx, d)
}
func (s *dictService) DeleteData(ctx context.Context, id uint64) error {
	return s.repo.DeleteData(ctx, tenant.TenantIDFromContext(ctx), id)
}

func (s *configService) List(ctx context.Context) ([]*model.SystemConfig, error) {
	return s.repo.List(ctx, tenant.TenantIDFromContext(ctx))
}
func (s *configService) Create(ctx context.Context, c *model.SystemConfig) (*model.SystemConfig, error) {
	tid := tenant.TenantIDFromContext(ctx)
	now := time.Now()
	c.TenantID = tid
	c.CreatedAt = now
	c.UpdatedAt = now
	if err := s.repo.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}
func (s *configService) Update(ctx context.Context, c *model.SystemConfig) error {
	tid := tenant.TenantIDFromContext(ctx)
	c.TenantID = tid
	c.UpdatedAt = time.Now()
	return s.repo.Update(ctx, c)
}
func (s *configService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, tenant.TenantIDFromContext(ctx), id)
}

func (s *noticeService) List(ctx context.Context) ([]*model.SystemNotice, error) {
	return s.repo.List(ctx, tenant.TenantIDFromContext(ctx))
}
func (s *noticeService) Create(ctx context.Context, n *model.SystemNotice) (*model.SystemNotice, error) {
	tid := tenant.TenantIDFromContext(ctx)
	now := time.Now()
	n.TenantID = tid
	if n.Status == 0 {
		n.Status = 1
	}
	n.CreatedAt = now
	n.UpdatedAt = now
	if err := s.repo.Create(ctx, n); err != nil {
		return nil, err
	}
	return n, nil
}
func (s *noticeService) Update(ctx context.Context, n *model.SystemNotice) error {
	tid := tenant.TenantIDFromContext(ctx)
	n.TenantID = tid
	n.UpdatedAt = time.Now()
	return s.repo.Update(ctx, n)
}
func (s *noticeService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, tenant.TenantIDFromContext(ctx), id)
}
