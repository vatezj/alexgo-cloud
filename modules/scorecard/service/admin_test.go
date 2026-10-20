package service

import (
	"testing"

	errs "alexGo-cloud/pkg/errors"
)

// setupRoom 见 expense_test.go。
func TestAdminClose(t *testing.T) {
	svc, ctx, roomID, _, _ := setupRoom(t, 0, false, 0, 0)
	if err := svc.AdminClose(ctx, roomID); err != nil {
		t.Fatal(err)
	}
	room := svc.(*roomService)
	got, _ := room.room.GetByID(ctx, roomID)
	if got == nil || got.Status != 2 {
		t.Fatalf("强制关房: %+v", got)
	}
	// 已关闭再关 → 30103
	if err := svc.AdminClose(ctx, roomID); !errs.Is(err, errs.CodeScorecardRoomClosed) {
		t.Fatalf("重复关闭应 30103, got %v", err)
	}
}

// TestAdminListRooms_RoomCodeFilter 终审 Important #3（spec §4.2 房间列表筛选）：
// room_code 精确等值——命中回本房、不命中空列表；空 roomCode 与 status<0 仍不过滤。
func TestAdminListRooms_RoomCodeFilter(t *testing.T) {
	svc, ctx, roomID, _, _ := setupRoom(t, 0, false, 0, 0)
	code := mustCode(t, svc, ctx, roomID)

	// 命中：房号精确等值 → 单条本房。
	list, total, err := svc.AdminListRooms(ctx, -1, code, 1, 10)
	if err != nil || total != 1 || len(list) != 1 || list[0].ID != roomID {
		t.Fatalf("按房号命中失败: total=%d list=%v err=%v", total, list, err)
	}
	// 不命中："0000" 不在房号字符集（去易混 0/1）内，恒无匹配。
	list, total, err = svc.AdminListRooms(ctx, -1, "0000", 1, 10)
	if err != nil || total != 0 || len(list) != 0 {
		t.Fatalf("按房号不命中应空: total=%d list=%v err=%v", total, list, err)
	}
	// status 与 room_code 可叠加：本房进行中，按 status=2（已关闭）+ 房号 → 空。
	list, total, err = svc.AdminListRooms(ctx, 2, code, 1, 10)
	if err != nil || total != 0 || len(list) != 0 {
		t.Fatalf("status+房号叠加过滤失败: total=%d list=%v err=%v", total, list, err)
	}
	// 空房号回归：不过滤，仍能查到本房。
	_, total, err = svc.AdminListRooms(ctx, -1, "", 1, 10)
	if err != nil || total != 1 {
		t.Fatalf("空房号应不过滤: total=%d err=%v", total, err)
	}
}
