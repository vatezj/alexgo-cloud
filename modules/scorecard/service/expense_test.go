package service

import (
	"context"
	"testing"

	"alexGo-cloud/pkg/tenant"
)

func setupRoom(t *testing.T, teaPercent float64, auto bool, maxLose, target int) (svc RoomService, ctx context.Context, roomID uint64, loser, winner uint64) {
	t.Helper()
	svc, _ = newRoomServiceForTest(t)
	ctx = tenant.WithTenantID(context.Background(), 1)
	snap, err := svc.Create(ctx, 100, CreateRoomReq{TeaPercent: teaPercent, TeaAutoEnabled: auto, MaxLoseScore: maxLose, TeaTargetAmount: target})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Join(ctx, 200, snap.RoomCode); err != nil {
		t.Fatal(err)
	}
	return svc, ctx, snap.ID, 100, 200
}

func TestExpense_ConservationWithTea(t *testing.T) {
	svc, ctx, roomID, loser, winner := setupRoom(t, 10, true, 0, 0) // 10% 茶饭
	res, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 100, ToUserID: winner, ClientRequestID: "r1"})
	if err != nil {
		t.Fatal(err)
	}
	if res.TeaCharged != 10 {
		t.Fatalf("tea = round(100*10%%) = 10, got %d", res.TeaCharged)
	}
	m := memberScoreMap(t, svc, ctx, roomID)
	if m[loser] != -100 || m[winner] != 90 {
		t.Fatalf("守恒破坏: %v", m)
	}
	if res.Snapshot.TeaPoolTotal != 10 {
		t.Fatalf("pool: %+v", res.Snapshot)
	}
}

func TestExpense_IdempotentReplay(t *testing.T) {
	svc, ctx, roomID, loser, winner := setupRoom(t, 0, false, 0, 0)
	r1, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 30, ToUserID: winner, ClientRequestID: "dup"})
	if err != nil {
		t.Fatal(err)
	}
	r2, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 30, ToUserID: winner, ClientRequestID: "dup"})
	if err != nil {
		t.Fatal(err)
	}
	if r2.LogID != r1.LogID {
		t.Fatalf("幂等重放应同 log: %d vs %d", r1.LogID, r2.LogID)
	}
	m := memberScoreMap(t, svc, ctx, roomID)
	if m[loser] != -30 || m[winner] != 30 {
		t.Fatalf("重放重复扣分: %v", m)
	}
}

func TestExpense_VersionIncrementsSerially(t *testing.T) {
	svc, ctx, roomID, loser, winner := setupRoom(t, 0, false, 0, 0)
	r1, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 10, ToUserID: winner, ClientRequestID: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	r2, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 10, ToUserID: winner, ClientRequestID: "v2"})
	if err != nil {
		t.Fatal(err)
	}
	if r2.Snapshot.Version != r1.Snapshot.Version+1 {
		t.Fatalf("version 应单调 +1: %d → %d", r1.Snapshot.Version, r2.Snapshot.Version)
	}
	// 走到 SaveVersioned 被拒的 30112 路径在 repo 单测构造
	//（TestRoomRepo_SaveVersioned_VersionConflict，Task 5）：sqlite 无法模拟
	// 真并发，FOR UPDATE 串行化在 MySQL 集成环境生效，version 是第二道防线。
}

func TestExpense_PendingAppliesNextRound(t *testing.T) {
	svc, ctx, roomID, loser, winner := setupRoom(t, 10, true, 0, 0)
	if _, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 100, ToUserID: winner, ClientRequestID: "old"}); err != nil {
		t.Fatal(err)
	}
	// 改配置 20%：当前把之后的下一笔才生效
	if _, err := svc.UpdateConfig(ctx, 100, roomID, ConfigReq{TeaPercent: ptr(20.0)}); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 100, ToUserID: winner, ClientRequestID: "new"})
	if err != nil {
		t.Fatal(err)
	}
	if res.TeaCharged != 20 {
		t.Fatalf("从下把生效失败: tea=%d", res.TeaCharged)
	}
}

func TestUpdateConfig_PercentOnlyKeepsOtherFields(t *testing.T) {
	svc, ctx, roomID, loser, winner := setupRoom(t, 10, true, 50, 30) // percent=10 auto maxLose=50 target=30
	if _, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 100, ToUserID: winner, ClientRequestID: "p1"}); err != nil {
		t.Fatal(err)
	}
	// 只改 percent：pending 首次暂存先种子当前 target/maxLose，applyPending 不得清零
	if _, err := svc.UpdateConfig(ctx, 100, roomID, ConfigReq{TeaPercent: ptr(20.0)}); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 100, ToUserID: winner, ClientRequestID: "p2"})
	if err != nil {
		t.Fatal(err)
	}
	if res.TeaCharged != 20 {
		t.Fatalf("percent 20%% 应生效: tea=%d", res.TeaCharged)
	}
	if res.Snapshot.TeaTargetAmount != 30 || res.Snapshot.MaxLoseScore != 50 {
		t.Fatalf("部分编辑清场了其他配置: target=%d maxLose=%d",
			res.Snapshot.TeaTargetAmount, res.Snapshot.MaxLoseScore)
	}
}

func TestUpdateConfig_AutoToggleKeepsNumerics(t *testing.T) {
	svc, ctx, roomID, loser, winner := setupRoom(t, 10, true, 50, 0) // percent=10 auto maxLose=50
	// 只关自动开关：不进 pending，数值项不动
	if _, err := svc.UpdateConfig(ctx, 100, roomID, ConfigReq{TeaAutoEnabled: ptr(false)}); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 100, ToUserID: winner, ClientRequestID: "a1"})
	if err != nil {
		t.Fatal(err)
	}
	if res.TeaCharged != 0 {
		t.Fatalf("自动关后不应抽茶: tea=%d", res.TeaCharged)
	}
	if res.Snapshot.TeaPercent != 10 || res.Snapshot.MaxLoseScore != 50 {
		t.Fatalf("开关误清数值: percent=%v maxLose=%d", res.Snapshot.TeaPercent, res.Snapshot.MaxLoseScore)
	}
}

func TestExpense_TargetReachedAutoOff(t *testing.T) {
	svc, ctx, roomID, loser, winner := setupRoom(t, 50, true, 0, 30) // 目标 30
	res, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 100, ToUserID: winner, ClientRequestID: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if res.TeaCharged != 50 || res.Snapshot.TeaPoolTotal != 50 {
		t.Fatalf("%+v", res)
	}
	if res.Snapshot.TeaAutoEnabled {
		t.Fatal("达标应自动关闭自动抽取")
	}
}

func TestExpense_MaxLoseFrequencyControl(t *testing.T) {
	svc, ctx, roomID, loser, winner := setupRoom(t, 0, false, 50, 0)
	res1, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 50, ToUserID: winner, ClientRequestID: "m1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res1.AlarmedUserIDs) != 1 || res1.AlarmedUserIDs[0] != loser {
		t.Fatalf("首次达上限应全房提醒: %+v", res1)
	}
	if _, err := svc.Expense(ctx, winner, roomID, ExpenseReq{Score: 30, ToUserID: loser, ClientRequestID: "m2"}); err != nil {
		t.Fatal(err)
	}
	// loser 此时是赢家不涨累计输分；再让 loser 输一笔超限 → 仅日志不进 AlarmedUserIDs
	res3, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 30, ToUserID: winner, ClientRequestID: "m3"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res3.AlarmedUserIDs) != 0 {
		t.Fatalf("二次超限不应重复全房提醒: %+v", res3)
	}
}

func TestExpense_DiyTeaFromSource(t *testing.T) {
	svc, ctx, roomID, loser, winner := setupRoom(t, 0, false, 0, 0) // 手动模式
	res, err := svc.Expense(ctx, loser, roomID, ExpenseReq{
		Score: 100, Tea: 20, TeaFromUserID: winner, ToUserID: winner, ClientRequestID: "d1",
	})
	if err != nil {
		t.Fatal(err)
	}
	m := memberScoreMap(t, svc, ctx, roomID)
	// DIY：输家 -100，赢家 +100-20（来源=赢家），守恒
	if m[loser] != -100 || m[winner] != 80 || res.TeaCharged != 20 {
		t.Fatalf("DIY 口径错: %v tea=%d", m, res.TeaCharged)
	}
}
