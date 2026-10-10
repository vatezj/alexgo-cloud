package service

import (
	"context"
	"errors"
	"testing"

	"alexGo-cloud/modules/order/model"
)

type fakeCodegenDemoItemRepo struct {
	data map[uint64]*model.CodegenDemoItem
	seq  uint64
}

var errCodegenDemoItemRecordNotFound = errors.New("record not found")

func (f *fakeCodegenDemoItemRepo) Create(_ context.Context, e *model.CodegenDemoItem) error {
	f.seq++
	e.ID = f.seq
	f.data[e.ID] = e
	return nil
}

func (f *fakeCodegenDemoItemRepo) List(_ context.Context, _ uint64) ([]*model.CodegenDemoItem, error) {
	out := make([]*model.CodegenDemoItem, 0, len(f.data))
	for _, e := range f.data {
		out = append(out, e)
	}
	return out, nil
}

func (f *fakeCodegenDemoItemRepo) GetByID(_ context.Context, _ uint64, id uint64) (*model.CodegenDemoItem, error) {
	if e, ok := f.data[id]; ok {
		return e, nil
	}
	return nil, errCodegenDemoItemRecordNotFound
}

func (f *fakeCodegenDemoItemRepo) Update(_ context.Context, e *model.CodegenDemoItem) error {
	f.data[e.ID] = e
	return nil
}

func (f *fakeCodegenDemoItemRepo) Delete(_ context.Context, _ uint64, id uint64) error {
	delete(f.data, id)
	return nil
}

func TestCodegenDemoItemService_CRUD(t *testing.T) {
	svc := NewCodegenDemoItemService(&fakeCodegenDemoItemRepo{data: map[uint64]*model.CodegenDemoItem{}})
	ctx := context.Background()

	e := &model.CodegenDemoItem{}
	if err := svc.Create(ctx, e); err != nil {
		t.Fatalf("Create: %v", err)
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
