package service

import (
	"context"
	"errors"
	"testing"

	"alexGo-cloud/modules/taokouling/model"
	"alexGo-cloud/pkg/tenant"
)

type fakeChannelRepo struct {
	data map[uint64]*model.Channel
	seq  uint64
}

var errChannelRecordNotFound = errors.New("record not found")

func (f *fakeChannelRepo) Create(_ context.Context, e *model.Channel) error {
	f.seq++
	e.ID = f.seq
	f.data[e.ID] = e
	return nil
}

func (f *fakeChannelRepo) List(_ context.Context, _ uint64) ([]*model.Channel, error) {
	out := make([]*model.Channel, 0, len(f.data))
	for _, e := range f.data {
		out = append(out, e)
	}
	return out, nil
}

func (f *fakeChannelRepo) GetByID(_ context.Context, _ uint64, id uint64) (*model.Channel, error) {
	if e, ok := f.data[id]; ok {
		return e, nil
	}
	return nil, errChannelRecordNotFound
}

func (f *fakeChannelRepo) Update(_ context.Context, e *model.Channel) error {
	f.data[e.ID] = e
	return nil
}

func (f *fakeChannelRepo) Delete(_ context.Context, _ uint64, id uint64) error {
	delete(f.data, id)
	return nil
}

func TestChannelService_CRUD(t *testing.T) {
	svc := NewChannelService(&fakeChannelRepo{data: map[uint64]*model.Channel{}})
	ctx := context.Background()
	ctx = tenant.WithTenantID(ctx, 42)

	e := &model.Channel{}
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
