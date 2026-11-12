package service

import (
	"context"
	"errors"
	"testing"

	"alexGo-cloud/modules/starfuyu/model"
	"alexGo-cloud/pkg/tenant"
)

type fakeFortuneRepo struct {
	data map[uint64]*model.Fortune
	seq  uint64
}

var errFortuneRecordNotFound = errors.New("record not found")

func (f *fakeFortuneRepo) Create(_ context.Context, e *model.Fortune) error {
	f.seq++
	e.ID = f.seq
	f.data[e.ID] = e
	return nil
}

func (f *fakeFortuneRepo) List(_ context.Context, _ uint64) ([]*model.Fortune, error) {
	out := make([]*model.Fortune, 0, len(f.data))
	for _, e := range f.data {
		out = append(out, e)
	}
	return out, nil
}

func (f *fakeFortuneRepo) GetByID(_ context.Context, _ uint64, id uint64) (*model.Fortune, error) {
	if e, ok := f.data[id]; ok {
		return e, nil
	}
	return nil, errFortuneRecordNotFound
}

func (f *fakeFortuneRepo) Update(_ context.Context, e *model.Fortune) error {
	f.data[e.ID] = e
	return nil
}

func (f *fakeFortuneRepo) Delete(_ context.Context, _ uint64, id uint64) error {
	delete(f.data, id)
	return nil
}

func TestFortuneService_CRUD(t *testing.T) {
	svc := NewFortuneService(&fakeFortuneRepo{data: map[uint64]*model.Fortune{}})
	ctx := context.Background()
	ctx = tenant.WithTenantID(ctx, 42)

	e := &model.Fortune{}
	if err := svc.Create(ctx, e); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if e.TenantID != 42 {
		t.Fatalf("Create 应从 ctx 打戳租户 42, got %d", e.TenantID)
	}
	if e.ID == 0 {
		t.Fatal("Create should assign ID")
	}
	if _, err := svc.GetByID(ctx, e.ID); err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if err := svc.Update(ctx, e); err != nil {
		t.Fatalf("Update: %v", err)
	}
	list, err := svc.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("List: err=%v len=%d", err, len(list))
	}
	if err := svc.Delete(ctx, e.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if list, _ = svc.List(ctx); len(list) != 0 {
		t.Fatalf("after Delete len=%d", len(list))
	}
}
