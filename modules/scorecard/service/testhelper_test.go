package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/member/service"
	"alexGo-cloud/modules/scorecard/model"
	"alexGo-cloud/modules/scorecard/repository"
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/tenant/gormplugin"
)

// briefRec fakeBriefs 的调用记录（值接收者下经指针字段共享，供跨模块消费点断言）。
type briefRec struct {
	tenantID uint64
	ids      []uint64
}

// fakeBriefs BriefProvider 测试桩：按 id 生成占位昵称；rec 非 nil 时记录调用上下文。
// 值接收者保持值形态可用（Task 11 以 fakeBriefs{} 传值/返回）。
type fakeBriefs struct {
	rec *briefRec
}

func (f fakeBriefs) GetBriefs(ctx context.Context, ids []uint64) ([]service.UserBrief, error) {
	if f.rec != nil {
		f.rec.tenantID = tenant.TenantIDFromContext(ctx)
		f.rec.ids = append([]uint64(nil), ids...)
	}
	out := make([]service.UserBrief, 0, len(ids))
	for _, id := range ids {
		out = append(out, service.UserBrief{ID: id, Nickname: fmt.Sprintf("u%d", id), Status: 1})
	}
	return out, nil
}

func newRoomServiceForTest(t *testing.T) (RoomService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Room{}, &model.RoomUser{}, &model.RoomLog{},
		&model.Record{}, &model.Friend{}, &model.FriendBlack{}); err != nil {
		t.Fatal(err)
	}
	if err := gormplugin.Register(db, gormplugin.Options{ExemptTables: []string{"tenants", "casbin_rule"}}); err != nil {
		t.Fatal(err)
	}
	roomRepo := repository.NewRoomRepository(db)
	svc := NewRoomService(db, roomRepo, repository.NewRoomUserRepository(db),
		repository.NewRoomLogRepository(db), fakeBriefs{rec: &briefRec{}}, repository.NewFriendBlackRepository(db))
	return svc, db
}

// memberScoreMap 快照成员分 → user_id → cur_score。
func memberScoreMap(t *testing.T, svc RoomService, ctx context.Context, roomID uint64) map[uint64]int {
	t.Helper()
	snap, err := svc.Snapshot(ctx, snapCreatorID(t, svc, ctx, roomID), roomID)
	if err != nil {
		t.Fatal(err)
	}
	m := make(map[uint64]int, len(snap.Members))
	for _, u := range snap.Members {
		m[u.UserID] = u.CurScore
	}
	return m
}

func snapCreatorID(t *testing.T, svc RoomService, ctx context.Context, roomID uint64) uint64 {
	t.Helper()
	room, err := svc.(*roomService).room.GetByID(ctx, roomID)
	if err != nil || room == nil {
		t.Fatalf("room: %v", err)
	}
	return room.CreatorID
}

// listRecordsOfRoom 读该房全部 record 行（测试白盒，直接查库）。
func listRecordsOfRoom(t *testing.T, svc RoomService, ctx context.Context, roomID uint64) []*model.Record {
	t.Helper()
	var rows []*model.Record
	if err := svc.(*roomService).db.WithContext(ctx).Where("room_id = ?", roomID).Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	return rows
}

// mustCode 取房间当前房号。
func mustCode(t *testing.T, svc RoomService, ctx context.Context, roomID uint64) string {
	t.Helper()
	room, err := svc.(*roomService).room.GetByID(ctx, roomID)
	if err != nil || room == nil {
		t.Fatalf("room: %v", err)
	}
	return room.RoomCode
}

func ptr[T any](v T) *T { return &v }
