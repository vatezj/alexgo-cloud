package service

import (
	"context"
	"testing"

	"alexGo-cloud/modules/scorecard/model"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/tenant"
)

func TestCreate_GenerateCodeAndSnapshot(t *testing.T) {
	svc, _ := newRoomServiceForTest(t)
	ctx := tenant.WithTenantID(context.Background(), 1)
	snap, err := svc.Create(ctx, 100, CreateRoomReq{TeaPercent: 10, TeaAutoEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.RoomCode) != 4 {
		t.Fatalf("房号应 4 位, got %q", snap.RoomCode)
	}
	if snap.CreatorID != 100 || snap.Version != 1 || len(snap.Members) != 1 || !snap.Members[0].IsCreator {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}
}

func TestCreate_CustomCodeConflictRetry(t *testing.T) {
	svc, _ := newRoomServiceForTest(t)
	ctx := tenant.WithTenantID(context.Background(), 1)
	if _, err := svc.Create(ctx, 100, CreateRoomReq{RoomCode: "AB12"}); err != nil {
		t.Fatal(err)
	}
	// 同租户重复自定义房号：生成冲突耗尽 → 30106
	_, err := svc.Create(ctx, 101, CreateRoomReq{RoomCode: "AB12"})
	if !errs.Is(err, errs.CodeScorecardRoomTaken) {
		t.Fatalf("want 30106, got %v", err)
	}
}

func TestJoin_IdempotentAndSessionReset(t *testing.T) {
	svc, db := newRoomServiceForTest(t)
	ctx := tenant.WithTenantID(context.Background(), 1)
	snap, _ := svc.Create(ctx, 100, CreateRoomReq{})
	if _, err := svc.Join(ctx, 200, snap.RoomCode); err != nil {
		t.Fatal(err)
	}
	// 幂等：已在房，返回现状不重复加人
	snap2, err := svc.Join(ctx, 200, snap.RoomCode)
	if err != nil || len(snap2.Members) != 2 {
		t.Fatalf("join idempotent fail: %v %+v", err, snap2)
	}
	// 造数模拟退房后重进（Leave 在 Task 8 实现，此处直接删成员行）：
	// 重进应重新入房且累计清零
	if err := db.WithContext(ctx).Where("room_id = ? AND user_id = ?", snap.ID, uint64(200)).
		Delete(&model.RoomUser{}).Error; err != nil {
		t.Fatal(err)
	}
	snap3, err := svc.Join(ctx, 200, snap.RoomCode)
	if err != nil || len(snap3.Members) != 2 {
		t.Fatalf("重进失败: %v %+v", err, snap3)
	}
	for _, m := range snap3.Members {
		if m.UserID == 200 && (m.CurScore != 0 || m.CumLoseScore != 0) {
			t.Fatalf("重进应清零: %+v", m)
		}
	}
}

func TestSnapshot_CrossTenantDenied(t *testing.T) {
	svc, _ := newRoomServiceForTest(t)
	ctx1 := tenant.WithTenantID(context.Background(), 1)
	ctx2 := tenant.WithTenantID(context.Background(), 2)
	snap, _ := svc.Create(ctx1, 100, CreateRoomReq{})
	_, err := svc.Snapshot(ctx2, 100, snap.ID)
	if !errs.Is(err, errs.CodeScorecardRoomNotFound) {
		t.Fatalf("跨租户应 30101, got %v", err)
	}
}

// TestSnapshot_BriefsCarryTenantCtx 快照昵称补齐是 BriefProvider 的跨模块消费点：
// GetBriefs 必须透传租户编号 ctx（T3 评审 Minor #1 明确 deferred 至 Task 6 落断言）。
// 顺带钉住自定义房号真实落库（30106 冲突测试只覆盖错误码，不覆盖房号值）。
func TestSnapshot_BriefsCarryTenantCtx(t *testing.T) {
	svc, _ := newRoomServiceForTest(t)
	ctx := tenant.WithTenantID(context.Background(), 7)
	snap, err := svc.Create(ctx, 100, CreateRoomReq{RoomCode: "ZZ99"})
	if err != nil {
		t.Fatal(err)
	}
	if snap.RoomCode != "ZZ99" {
		t.Fatalf("自定义房号应原样落库, got %q", snap.RoomCode)
	}
	if _, err := svc.Join(ctx, 200, snap.RoomCode); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Snapshot(ctx, 100, snap.ID)
	if err != nil {
		t.Fatal(err)
	}
	rec := svc.(*roomService).briefs.(fakeBriefs).rec
	if rec == nil {
		t.Fatal("testhelper 应注入带记录器的 fakeBriefs")
	}
	if rec.tenantID != 7 {
		t.Fatalf("GetBriefs 应携带租户编号 7, got %d", rec.tenantID)
	}
	if len(rec.ids) != 2 || rec.ids[0] != 100 || rec.ids[1] != 200 {
		t.Fatalf("GetBriefs 应按成员批量传 id, got %v", rec.ids)
	}
	if got.Members[0].Nickname != "u100" || got.Members[1].Nickname != "u200" {
		t.Fatalf("昵称应由 BriefProvider 批量补齐: %+v", got.Members)
	}
}
