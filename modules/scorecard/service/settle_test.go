package service

import (
	"testing"

	errs "alexGo-cloud/pkg/errors"
)

// 说明：brief 原文 import context/pkg/tenant 未被引用（setupRoom 内部已带租户上下文），
// 为可编译移除。

func TestSettle_WritesRecordsForAll(t *testing.T) {
	svc, ctx, roomID, loser, winner := setupRoom(t, 0, false, 0, 0)
	if _, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 40, ToUserID: winner, ClientRequestID: "s1"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settle(ctx, 100, roomID); err != nil {
		t.Fatal(err)
	}
	// 结算后：记分被拒（TestExpense_RejectedAfterSettle 覆盖）
	// record 两行：loser -40 / winner +40
	recs := listRecordsOfRoom(t, svc, ctx, roomID)
	if len(recs) != 2 {
		t.Fatalf("全员 record 行, got %d", len(recs))
	}
}

func TestSettle_NonOwnerRejected(t *testing.T) {
	svc, ctx, roomID, _, _ := setupRoom(t, 0, false, 0, 0)
	if err := svc.Settle(ctx, 200, roomID); !errs.Is(err, errs.CodeScorecardNotOwner) {
		t.Fatalf("非房主应 30105, got %v", err)
	}
}

func TestExpense_RejectedAfterSettle(t *testing.T) {
	svc, ctx, roomID, loser, winner := setupRoom(t, 0, false, 0, 0)
	if err := svc.Settle(ctx, 100, roomID); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 1, ToUserID: winner, ClientRequestID: "z"})
	if !errs.Is(err, errs.CodeScorecardRoomSettled) {
		t.Fatalf("已结算应 30104, got %v", err)
	}
}

func TestLeave_OwnerAutoSettles(t *testing.T) {
	svc, ctx, roomID, _, _ := setupRoom(t, 0, false, 0, 0)
	if err := svc.Leave(ctx, 100, roomID); err != nil {
		t.Fatal(err)
	}
	// 房主退房 → 先结算（全员 record settle_type=2）→ 房主行移除
	recs := listRecordsOfRoom(t, svc, ctx, roomID)
	if len(recs) != 2 { // 100 与 200 各一行
		t.Fatalf("房主退房自动结算应写全员 record, got %d", len(recs))
	}
	snap, err := svc.Snapshot(ctx, 200, roomID)
	if err != nil || snap.Status != 1 {
		t.Fatalf("房间应已结算只读: %v %+v", err, snap)
	}
}

func TestLeave_ZeroDataWritesEmptyRecord(t *testing.T) {
	svc, ctx, roomID, _, _ := setupRoom(t, 0, false, 0, 0)
	if err := svc.Leave(ctx, 200, roomID); err != nil {
		t.Fatal(err)
	}
	recs := listRecordsOfRoom(t, svc, ctx, roomID)
	if len(recs) != 1 || recs[0].Score != 0 || recs[0].SettleType != 4 {
		t.Fatalf("零数据空战绩: %+v", recs)
	}
}

// TestRejoin_ResetsCumLose brief 原文让 loser(=100=房主) 退房后重进；但房主退房自动
// 结算（status=1），Join 对已结算房拒 30104——重进语义只对普通成员成立。最小适配：
// 改为普通成员 200 输分 → 退房 → 重进，断言清零不变。
func TestRejoin_ResetsCumLose(t *testing.T) {
	svc, ctx, roomID, owner, member := setupRoom(t, 0, false, 30, 0)
	if _, err := svc.Expense(ctx, member, roomID, ExpenseReq{Score: 30, ToUserID: owner, ClientRequestID: "j1"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Leave(ctx, member, roomID); err != nil {
		t.Fatal(err)
	}
	snap, err := svc.Join(ctx, member, mustCode(t, svc, ctx, roomID))
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range snap.Members {
		if m.UserID == member && (m.CumLoseScore != 0 || m.CurScore != 0) {
			t.Fatalf("重进应清零: %+v", m)
		}
	}
}

func TestTransferOwner_Matrix(t *testing.T) {
	svc, ctx, roomID, _, _ := setupRoom(t, 0, false, 0, 0)
	// 非房主发起 → 30105
	if err := svc.TransferOwner(ctx, 200, roomID, 100); !errs.Is(err, errs.CodeScorecardNotOwner) {
		t.Fatalf("非房主: %v", err)
	}
	// 目标不在房 → 30113
	if err := svc.TransferOwner(ctx, 100, roomID, 999); !errs.Is(err, errs.CodeScorecardTargetNotMember) {
		t.Fatalf("目标不在房: %v", err)
	}
	// 合法转让
	if err := svc.TransferOwner(ctx, 100, roomID, 200); err != nil {
		t.Fatal(err)
	}
	snap, _ := svc.Snapshot(ctx, 200, roomID)
	if snap.CreatorID != 200 {
		t.Fatalf("房主未切换: %+v", snap)
	}
	// 原房主变普通玩家后再转让 → 30105
	if err := svc.TransferOwner(ctx, 100, roomID, 200); err == nil {
		t.Fatal("原房主应失去权限")
	}
}
