package service

import (
	"context"
	"testing"

	"alexGo-cloud/modules/scorecard/model"
	errs "alexGo-cloud/pkg/errors"
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

// 票4：幂等键房间内唯一但可跨用户撞键——同 client_request_id 被他人抢先后，
// 本人的记分请求不得被吞（回放他人结果），应拒绝；键的原主重放不受影响。
func TestExpense_IdempotentKeyMustBelongToRequester(t *testing.T) {
	svc, ctx, roomID, loser, winner := setupRoom(t, 0, false, 0, 0)
	if _, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 30, ToUserID: winner, ClientRequestID: "shared"}); err != nil {
		t.Fatal(err)
	}
	// 他人已占用同键：本人请求被拒（否则本人支出被吞、回放成他人结果）。
	if _, err := svc.Expense(ctx, winner, roomID, ExpenseReq{Score: 10, ToUserID: loser, ClientRequestID: "shared"}); err == nil {
		t.Fatal("他人已占用的幂等键应拒绝")
	}
	// 键的原主重放仍正常。
	r, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 30, ToUserID: winner, ClientRequestID: "shared"})
	if err != nil || r == nil {
		t.Fatalf("原主重放应正常: %v", err)
	}
	m := memberScoreMap(t, svc, ctx, roomID)
	if m[loser] != -30 || m[winner] != 30 {
		t.Fatalf("被拒请求不应记分: %v", m)
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

// 票2：type-6 首提醒按最近进房流水 id 下界判定——退房重进（新会话）后
// 再次达上限须重新全房提醒，不被上一会话的老提醒日志吞掉。
func TestExpense_MaxLoseAlarmResetsAcrossSessions(t *testing.T) {
	svc, ctx, roomID, loser, winner := setupRoom(t, 0, false, 50, 0)
	res1, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 50, ToUserID: winner, ClientRequestID: "s1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res1.AlarmedUserIDs) != 1 || res1.AlarmedUserIDs[0] != loser {
		t.Fatalf("首次达上限应提醒: %+v", res1)
	}
	// 退房（硬删成员行，Leave 同款）+ 重进 = 新会话。
	if err := svc.(*roomService).db.WithContext(ctx).
		Where("room_id = ? AND user_id = ?", roomID, uint64(loser)).
		Delete(&model.RoomUser{}).Error; err != nil {
		t.Fatal(err)
	}
	snap, err := svc.Snapshot(ctx, winner, roomID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Join(ctx, loser, snap.RoomCode); err != nil {
		t.Fatal(err)
	}
	// 新会话再达上限：须重新全房提醒。
	res2, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 50, ToUserID: winner, ClientRequestID: "s2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res2.AlarmedUserIDs) != 1 || res2.AlarmedUserIDs[0] != loser {
		t.Fatalf("新会话再达上限应重新提醒: %+v", res2)
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

// TestExpense_DiyTeaFromNonMemberRejected 终审 Important #2：DIY 来源非在房成员 →
// 30113 拒绝，且分数/池/流水原样（AddScore 0 行静默 → 不校验会守恒破坏）。
func TestExpense_DiyTeaFromNonMemberRejected(t *testing.T) {
	svc, ctx, roomID, loser, winner := setupRoom(t, 0, false, 0, 0) // 手动模式
	// 用户 300 真实存在（是另一间房的房主），但不在本房 —— 比纯假 ID 更硬。
	if _, err := svc.Create(ctx, 300, CreateRoomReq{}); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Expense(ctx, loser, roomID, ExpenseReq{
		Score: 100, Tea: 20, TeaFromUserID: 300, ToUserID: winner, ClientRequestID: "n1",
	})
	if !errs.Is(err, errs.CodeScorecardTargetNotMember) {
		t.Fatalf("非成员来源应 30113, got %v", err)
	}
	// 守恒钉死：成员分全 0、池 0、无 type=1 流水（事务整体回滚/未写）。
	if m := memberScoreMap(t, svc, ctx, roomID); m[loser] != 0 || m[winner] != 0 {
		t.Fatalf("拒绝后分数被动过: %v", m)
	}
	snap, err := svc.Snapshot(ctx, loser, roomID)
	if err != nil {
		t.Fatal(err)
	}
	if snap.TeaPoolTotal != 0 {
		t.Fatalf("拒绝后池被动过: %d", snap.TeaPoolTotal)
	}
	// 拒绝后不落 type=1 记分流水（开房/进房的 type=3 是既有流水，不应受影响）。
	logs, _, err := svc.Logs(ctx, loser, roomID, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range logs {
		if l.Type == 1 {
			t.Fatalf("拒绝后仍落了记分流水: %+v", l)
		}
	}
}

// TestExpense_TeaOverLimitRejected 终审 #2：单笔 Tea 超上限 → 10001，且不动房。
func TestExpense_TeaOverLimitRejected(t *testing.T) {
	svc, ctx, roomID, loser, winner := setupRoom(t, 0, false, 0, 0)
	_, err := svc.Expense(ctx, loser, roomID, ExpenseReq{
		Score: 100, Tea: maxTeaPerExpense + 1, TeaFromUserID: winner, ToUserID: winner,
		ClientRequestID: "o1",
	})
	if !errs.Is(err, errs.CodeParamInvalid) {
		t.Fatalf("Tea 超限应 10001, got %v", err)
	}
	if m := memberScoreMap(t, svc, ctx, roomID); m[loser] != 0 || m[winner] != 0 {
		t.Fatalf("超限拒绝后分数被动过: %v", m)
	}
	// 临界值（=上限）仍放行，保险丝不是业务规则。
	if _, err := svc.Expense(ctx, loser, roomID, ExpenseReq{
		Score: 100, Tea: maxTeaPerExpense, ToUserID: winner, ClientRequestID: "o2",
	}); err != nil {
		t.Fatalf("上限值应放行: %v", err)
	}
}

// TestExpense_TeaOnly 只记茶（score=0）：来源扣 T 入池、无分数转移、to_user_id=0 放行。
func TestExpense_TeaOnly(t *testing.T) {
	svc, ctx, roomID, loser, winner := setupRoom(t, 0, false, 0, 0) // 手动模式
	res, err := svc.Expense(ctx, loser, roomID, ExpenseReq{
		Score: 0, Tea: 20, TeaFromUserID: 0, ToUserID: 0, ClientRequestID: "t1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.TeaCharged != 20 || res.Snapshot.TeaPoolTotal != 20 {
		t.Fatalf("只记茶口径错: %+v", res)
	}
	m := memberScoreMap(t, svc, ctx, roomID)
	// 来源缺省=支出方（loser）：仅 loser -20，无赢家转移
	if m[loser] != -20 || m[winner] != 0 {
		t.Fatalf("只记茶分数转移错: %v", m)
	}
	// 流水 type=1、score=0、to=0、tea=20
	logs, _, err := svc.Logs(ctx, loser, roomID, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, l := range logs {
		if l.Type == 1 {
			found = true
			if l.Score != 0 || l.ToUserID != 0 || l.Tea != 20 || l.FromUserID != loser {
				t.Fatalf("只记茶流水字段错: %+v", l)
			}
		}
	}
	if !found {
		t.Fatal("只记茶未落 type=1 流水")
	}
}

// TestExpense_TeaOnlyRejectsEmpty score=0 且 tea=0 → 10001，不动任何账目。
func TestExpense_TeaOnlyRejectsEmpty(t *testing.T) {
	svc, ctx, roomID, loser, _ := setupRoom(t, 0, false, 0, 0)
	_, err := svc.Expense(ctx, loser, roomID, ExpenseReq{
		Score: 0, Tea: 0, ClientRequestID: "e1",
	})
	if !errs.Is(err, errs.CodeParamInvalid) {
		t.Fatalf("空请求应 10001, got %v", err)
	}
	if m := memberScoreMap(t, svc, ctx, roomID); m[loser] != 0 {
		t.Fatalf("拒绝后分数被动过: %v", m)
	}
}

// TestExpense_TeaOnlyAutoModeRejected 自动抽茶模式 score=0：tea=round(0)=0 →
// 事务内拒（req.Tea 在 auto 分支不生效），不落空流水、不推进账目。
func TestExpense_TeaOnlyAutoModeRejected(t *testing.T) {
	svc, ctx, roomID, loser, winner := setupRoom(t, 10, true, 0, 0) // 自动模式
	_, err := svc.Expense(ctx, loser, roomID, ExpenseReq{
		Score: 0, Tea: 5, ToUserID: winner, ClientRequestID: "a0",
	})
	if !errs.Is(err, errs.CodeParamInvalid) {
		t.Fatalf("自动模式只记茶应 10001, got %v", err)
	}
	if m := memberScoreMap(t, svc, ctx, roomID); m[loser] != 0 || m[winner] != 0 {
		t.Fatalf("拒绝后分数被动过: %v", m)
	}
	logs, _, err := svc.Logs(ctx, loser, roomID, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range logs {
		if l.Type == 1 {
			t.Fatalf("拒绝后仍落了记分流水: %+v", l)
		}
	}
}

// TestExpense_TeaOnlyFromNonMemberRejected 只记茶来源非房内成员 → 30113，全量回滚。
func TestExpense_TeaOnlyFromNonMemberRejected(t *testing.T) {
	svc, ctx, roomID, loser, _ := setupRoom(t, 0, false, 0, 0)
	if _, err := svc.Create(ctx, 300, CreateRoomReq{}); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Expense(ctx, loser, roomID, ExpenseReq{
		Score: 0, Tea: 20, TeaFromUserID: 300, ClientRequestID: "t2",
	})
	if !errs.Is(err, errs.CodeScorecardTargetNotMember) {
		t.Fatalf("非成员来源应 30113, got %v", err)
	}
	if m := memberScoreMap(t, svc, ctx, roomID); m[loser] != 0 {
		t.Fatalf("拒绝后分数被动过: %v", m)
	}
	snap, err := svc.Snapshot(ctx, loser, roomID)
	if err != nil {
		t.Fatal(err)
	}
	if snap.TeaPoolTotal != 0 {
		t.Fatalf("拒绝后池被动过: %d", snap.TeaPoolTotal)
	}
}

// TestExpense_ScorePositiveStillRequiresToUserID score>0 回归：to=0 仍拒（只记茶放行不松动记分约束）。
func TestExpense_ScorePositiveStillRequiresToUserID(t *testing.T) {
	svc, ctx, roomID, loser, _ := setupRoom(t, 0, false, 0, 0)
	_, err := svc.Expense(ctx, loser, roomID, ExpenseReq{
		Score: 10, Tea: 0, ClientRequestID: "s0",
	})
	if !errs.Is(err, errs.CodeParamInvalid) {
		t.Fatalf("score>0 且 to=0 应 10001, got %v", err)
	}
}
