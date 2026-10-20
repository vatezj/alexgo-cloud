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
