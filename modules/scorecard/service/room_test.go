package service

import (
	"context"
	"errors"
	"testing"

	"alexGo-cloud/modules/scorecard/model"
	"alexGo-cloud/pkg/tenant"
)

type fakeRoomRepo struct {
	data map[uint64]*model.Room
	seq  uint64
}

var errRoomRecordNotFound = errors.New("record not found")

func (f *fakeRoomRepo) Create(_ context.Context, e *model.Room) error {
	f.seq++
	e.ID = f.seq
	f.data[e.ID] = e
	return nil
}

func (f *fakeRoomRepo) List(_ context.Context, _ uint64) ([]*model.Room, error) {
	out := make([]*model.Room, 0, len(f.data))
	for _, e := range f.data {
		out = append(out, e)
	}
	return out, nil
}

func (f *fakeRoomRepo) GetByID(_ context.Context, _ uint64, id uint64) (*model.Room, error) {
	if e, ok := f.data[id]; ok {
		return e, nil
	}
	return nil, errRoomRecordNotFound
}

func (f *fakeRoomRepo) Update(_ context.Context, e *model.Room) error {
	f.data[e.ID] = e
	return nil
}

func (f *fakeRoomRepo) Delete(_ context.Context, _ uint64, id uint64) error {
	delete(f.data, id)
	return nil
}

func TestRoomService_CRUD(t *testing.T) {
	svc := NewRoomService(&fakeRoomRepo{data: map[uint64]*model.Room{}})
	ctx := context.Background()
	ctx = tenant.WithTenantID(ctx, 42)

	e := &model.Room{}
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
