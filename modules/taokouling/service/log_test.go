package service

import (
	"context"
	"testing"

	"alexGo-cloud/modules/taokouling/model"
	"alexGo-cloud/modules/taokouling/repository"
	"alexGo-cloud/pkg/tenant"
)

type fakeLogRepo struct {
	data map[uint64]*model.ConversionLog
	seq  uint64
}

func (f *fakeLogRepo) Append(_ context.Context, e *model.ConversionLog) error {
	f.seq++
	e.ID = f.seq
	f.data[e.ID] = e
	return nil
}

func (f *fakeLogRepo) List(_ context.Context, _ repository.LogFilter) ([]model.ConversionLog, int64, error) {
	out := make([]model.ConversionLog, 0, len(f.data))
	for _, e := range f.data {
		out = append(out, *e)
	}
	return out, int64(len(out)), nil
}

func TestLogService_CreateList(t *testing.T) {
	svc := NewLogService(&fakeLogRepo{data: map[uint64]*model.ConversionLog{}})
	ctx := context.Background()
	ctx = tenant.WithTenantID(ctx, 42)

	e := &model.ConversionLog{}
	if err := svc.Create(ctx, e); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if e.TenantID != 42 {
		t.Fatalf("Create 应从 ctx 打戳租户 42, got %d", e.TenantID)
	}
	if e.ID == 0 {
		t.Fatal("Create should assign ID")
	}
	list, err := svc.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("List: err=%v len=%d", err, len(list))
	}
}
