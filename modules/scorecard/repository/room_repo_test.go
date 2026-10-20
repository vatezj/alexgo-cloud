package repository

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/scorecard/model"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/tenant/gormplugin"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Room{}, &model.RoomUser{}, &model.RoomLog{}, &model.Record{}, &model.Friend{}, &model.FriendBlack{}); err != nil {
		t.Fatal(err)
	}
	if err := gormplugin.Register(db, gormplugin.Options{ExemptTables: []string{"tenants", "casbin_rule"}}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestRoom_GetByID_TenantIsolation(t *testing.T) {
	db := newTestDB(t)
	repo := NewRoomRepository(db)
	ctx1 := tenant.WithTenantID(context.Background(), 1)
	ctx2 := tenant.WithTenantID(context.Background(), 2)
	r := &model.Room{RoomCode: "AB12", TenantID: 1, Status: 0}
	if err := db.WithContext(ctx1).Create(r).Error; err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByID(ctx2, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("跨租户应不可见，got %+v", got)
	}
}

func TestRoomRepo_SaveVersioned_VersionConflict(t *testing.T) {
	db := newTestDB(t)
	repo := NewRoomRepository(db)
	ctx := tenant.WithTenantID(context.Background(), 1)
	r := &model.Room{RoomCode: "XY99", TenantID: 1, Status: 0}
	if err := db.WithContext(ctx).Create(r).Error; err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByID(ctx, r.ID) // 持有过期快照（version=0）
	if err != nil || got == nil {
		t.Fatalf("get: %v", err)
	}
	// 模拟并发者已提交：库内 version 推进到 3
	if err := db.WithContext(ctx).Model(&model.Room{}).Where("id = ?", r.ID).
		UpdateColumn("version", 3).Error; err != nil {
		t.Fatal(err)
	}
	got.RoomCode = "XY98"
	err = repo.SaveVersioned(ctx, db, got) // 携带 version=0 的 WHERE 命中 0 行
	if !errs.Is(err, errs.CodeScorecardVersionConflict) {
		t.Fatalf("过期 version 应 30112, got %v", err)
	}
}
