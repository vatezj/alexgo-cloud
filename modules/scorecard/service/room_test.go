package service

import (
	"context"
	"testing"
	"time"

	"gorm.io/gorm"

	"alexGo-cloud/modules/scorecard/model"
	"alexGo-cloud/modules/scorecard/repository"
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

// 票1：Create 多步写（room → member → log）任一步失败须整体回滚，
// 不得残留房行烧掉自定义房号。
func TestCreate_PartialFailureRollsBack(t *testing.T) {
	svc, db := newRoomServiceForTest(t)
	ctx := tenant.WithTenantID(context.Background(), 1)
	// 砍掉成员表制造第二步（member.Create）失败。
	if err := db.Exec(`DROP TABLE room_user`).Error; err != nil {
		t.Fatal(err)
	}
	_, err := svc.Create(ctx, 100, CreateRoomReq{RoomCode: "AB12"})
	if err == nil {
		t.Fatal("成员表缺失时 Create 应失败")
	}
	var n int64
	if err := db.Model(&model.Room{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("失败后不应残留房行（自定义房号被烧）: n=%d", n)
	}
}

// 票1：Join 多步写（member → log → expire 续期）任一步失败须整体回滚，
// 不得残留无流水的成员行。
func TestJoin_PartialFailureRollsBack(t *testing.T) {
	svc, db := newRoomServiceForTest(t)
	ctx := tenant.WithTenantID(context.Background(), 1)
	snap, err := svc.Create(ctx, 100, CreateRoomReq{})
	if err != nil {
		t.Fatal(err)
	}
	// 砍掉流水表制造第二步（log.Create）失败。
	if err := db.Exec(`DROP TABLE room_log`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Join(ctx, 200, snap.RoomCode); err == nil {
		t.Fatal("流水表缺失时 Join 应失败")
	}
	var n int64
	if err := db.WithContext(ctx).Model(&model.RoomUser{}).
		Where("room_id = ? AND user_id = ?", snap.ID, uint64(200)).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("失败后不应残留成员行: n=%d", n)
	}
}

// 票1：UpdateConfig 状态门——已结算 30104 / 已关闭 30103 拒绝改配置。
func TestUpdateConfig_StatusGate(t *testing.T) {
	svc, db := newRoomServiceForTest(t)
	ctx := tenant.WithTenantID(context.Background(), 1)
	settled, err := svc.Create(ctx, 100, CreateRoomReq{RoomCode: "GS01"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Settle(ctx, 100, settled.ID); err != nil {
		t.Fatal(err)
	}
	auto := true
	if _, err := svc.UpdateConfig(ctx, 100, settled.ID, ConfigReq{TeaAutoEnabled: &auto}); !errs.Is(err, errs.CodeScorecardRoomSettled) {
		t.Fatalf("已结算应 30104, got %v", err)
	}
	closed, err := svc.Create(ctx, 100, CreateRoomReq{RoomCode: "GS02"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Model(&model.Room{}).Where("id = ?", closed.ID).
		UpdateColumn("status", 2).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateConfig(ctx, 100, closed.ID, ConfigReq{TeaAutoEnabled: &auto}); !errs.Is(err, errs.CodeScorecardRoomClosed) {
		t.Fatalf("已关闭应 30103, got %v", err)
	}
}

// 票3：自定义房号格式 ^[A-Z0-9]{4,8}$，非法串 10001 拒，不落库
//（原任意串直落 varchar(8)，超长 500）。
func TestCreate_CustomCodeFormatValidation(t *testing.T) {
	svc, _ := newRoomServiceForTest(t)
	ctx := tenant.WithTenantID(context.Background(), 1)
	for _, code := range []string{"abc9", "AB", "ABCDEFGHI", "AB C", "abcd", "AB12中文"} {
		if _, err := svc.Create(ctx, 100, CreateRoomReq{RoomCode: code}); !errs.Is(err, errs.CodeParamInvalid) {
			t.Fatalf("非法房号 %q 应 10001, got %v", code, err)
		}
	}
	// 合法：4-8 位大写字母/数字。
	for _, code := range []string{"AB12", "ABCDEF12"} {
		if _, err := svc.Create(ctx, 100, CreateRoomReq{RoomCode: code}); err != nil {
			t.Fatalf("合法房号 %q 被拒: %v", code, err)
		}
	}
}

// 票5：快照读路径（非事务）不抢 FOR UPDATE 行锁——轮询热点；只有事务内才锁行。
type spyRoomRepo struct {
	repository.RoomRepository
	forUpdateCalls int
}

func (s *spyRoomRepo) GetForUpdate(ctx context.Context, tx *gorm.DB, id uint64) (*model.Room, error) {
	s.forUpdateCalls++
	return s.RoomRepository.GetForUpdate(ctx, tx, id)
}

func TestBuildSnapshot_NonTxSkipsForUpdate(t *testing.T) {
	svc, db := newRoomServiceForTest(t)
	ctx := tenant.WithTenantID(context.Background(), 1)
	snap, err := svc.Create(ctx, 100, CreateRoomReq{})
	if err != nil {
		t.Fatal(err)
	}
	rs := svc.(*roomService)
	spy := &spyRoomRepo{RoomRepository: rs.room}
	rs.room = spy
	// 非事务读（快照接口轮询热点）：不走 GetForUpdate。
	if _, err := rs.buildSnapshot(ctx, db, snap.ID, 100, false); err != nil {
		t.Fatal(err)
	}
	if spy.forUpdateCalls != 0 {
		t.Fatalf("非事务快照不应 GetForUpdate, got %d", spy.forUpdateCalls)
	}
	// 事务内（Expense 等写路径）：必须锁行。
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, err := rs.buildSnapshot(ctx, tx, snap.ID, 100, true)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if spy.forUpdateCalls != 1 {
		t.Fatalf("事务内快照应 GetForUpdate 一次, got %d", spy.forUpdateCalls)
	}
}

// TestSweepExpired_ClosesAndLogs 超时关房：把 expire_time 拨到过去后 SweepExpired
// 关闭该房（status=2）并写 type=8 状态变更流水（系统级扫描，各房带自己的 TenantID）。
func TestSweepExpired_ClosesAndLogs(t *testing.T) {
	svc, _ := newRoomServiceForTest(t)
	ctx := tenant.WithTenantID(context.Background(), 1)
	snap, err := svc.Create(ctx, 100, CreateRoomReq{})
	if err != nil {
		t.Fatal(err)
	}
	roomSvc := svc.(*roomService)
	// 把 expire_time 拨到过去（直改列绕过 version 乐观锁，模拟超时）。
	if err := roomSvc.db.WithContext(ctx).Model(&model.Room{}).Where("id = ?", snap.ID).
		UpdateColumn("expire_time", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	n, err := svc.SweepExpired(ctx)
	if err != nil || n != 1 {
		t.Fatalf("sweep: n=%d err=%v", n, err)
	}
	got, err := roomSvc.room.GetByID(ctx, snap.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Status != 2 {
		t.Fatalf("应关闭: %+v", got)
	}
	logs, _, err := roomSvc.log.ListByRoom(ctx, snap.ID, 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	hasStatusLog := false
	for _, l := range logs {
		if l.Type == 8 {
			if l.TenantID != 1 {
				t.Fatalf("type=8 流水应带房自身 TenantID=1, got %d", l.TenantID)
			}
			hasStatusLog = true
		}
	}
	if !hasStatusLog {
		t.Fatal("缺 type=8 状态变更日志")
	}
}

// TestSweepExpired_SkipsActiveAndSettled 只关逾期进行中房：未到期/已结算/已关闭不被触碰。
func TestSweepExpired_SkipsActiveAndSettled(t *testing.T) {
	svc, _ := newRoomServiceForTest(t)
	ctx := tenant.WithTenantID(context.Background(), 1)
	// 房 A：进行中且未到期（expire=now+2h）。
	a, err := svc.Create(ctx, 100, CreateRoomReq{RoomCode: "AAAA"})
	if err != nil {
		t.Fatal(err)
	}
	// 房 B：已结算（不逾期也不该被动）。
	b, err := svc.Create(ctx, 100, CreateRoomReq{RoomCode: "BBBB"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Settle(ctx, 100, b.ID); err != nil {
		t.Fatal(err)
	}
	n, err := svc.SweepExpired(ctx)
	if err != nil || n != 0 {
		t.Fatalf("无逾期房应 n=0: n=%d err=%v", n, err)
	}
	roomSvc := svc.(*roomService)
	gotA, err := roomSvc.room.GetByID(ctx, a.ID)
	if err != nil || gotA == nil || gotA.Status != 0 {
		t.Fatalf("未到期房不应被关: %+v err=%v", gotA, err)
	}
	gotB, err := roomSvc.room.GetByID(ctx, b.ID)
	if err != nil || gotB == nil || gotB.Status != 1 {
		t.Fatalf("已结算房不应被改状态: %+v err=%v", gotB, err)
	}
}
