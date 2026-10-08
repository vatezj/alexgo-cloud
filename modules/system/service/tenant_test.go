package service

import (
	"context"
	"errors"
	"testing"

	"alexGo-cloud/modules/system/model"
)

type memTenantRepo struct {
	byID   map[uint64]*model.Tenant
	nextID uint64
	limits map[uint64]int // tenantID -> accountLimit（供额度测试）
	counts map[uint64]int64
}

func newMemTenantRepo() *memTenantRepo {
	return &memTenantRepo{
		byID: map[uint64]*model.Tenant{}, limits: map[uint64]int{},
		counts: map[uint64]int64{},
	}
}

func (m *memTenantRepo) List(context.Context) ([]*model.Tenant, error) {
	var out []*model.Tenant
	for _, v := range m.byID {
		out = append(out, v)
	}
	return out, nil
}
func (m *memTenantRepo) GetByID(_ context.Context, id uint64) (*model.Tenant, error) {
	v, ok := m.byID[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return v, nil
}
func (m *memTenantRepo) Create(_ context.Context, t *model.Tenant) error {
	m.nextID++
	t.ID = m.nextID
	m.byID[t.ID] = t
	return nil
}
func (m *memTenantRepo) Update(_ context.Context, t *model.Tenant) error {
	m.byID[t.ID] = t
	return nil
}
func (m *memTenantRepo) Delete(_ context.Context, id uint64) error {
	delete(m.byID, id)
	return nil
}
func (m *memTenantRepo) CountAccounts(_ context.Context, tenantID uint64) (int64, error) {
	return m.counts[tenantID], nil
}

func TestTenantService_CreateList(t *testing.T) {
	repo := newMemTenantRepo()
	svc := NewTenantService(repo)
	created, err := svc.Create(context.Background(), "Acme", "acme.example.com", -1)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == 0 || created.Status != 1 {
		t.Errorf("created = %+v（status 必须 1=启用）", created)
	}
	list, err := svc.List(context.Background())
	if err != nil || len(list) != 1 {
		t.Errorf("list = %v, err = %v", list, err)
	}
}

// 超额必须拒绝——服务层统一拦（system 建用户 + member 注册两处调用）。
func TestTenantService_CheckAccountLimit(t *testing.T) {
	repo := newMemTenantRepo()
	svc := NewTenantService(repo)
	tn, _ := svc.Create(context.Background(), "Acme", "", 2) // 额度 2
	repo.counts[tn.ID] = 2
	if err := svc.CheckAccountLimit(context.Background(), tn.ID); err == nil {
		t.Error("over limit must fail")
	}
	repo.counts[tn.ID] = 1
	if err := svc.CheckAccountLimit(context.Background(), tn.ID); err != nil {
		t.Errorf("under limit must pass: %v", err)
	}
	repo.counts[tn.ID] = 2
	tn.AccountLimit = -1 // 不限
	repo.byID[tn.ID] = tn
	if err := svc.CheckAccountLimit(context.Background(), tn.ID); err != nil {
		t.Errorf("limit=-1 must pass: %v", err)
	}
}
