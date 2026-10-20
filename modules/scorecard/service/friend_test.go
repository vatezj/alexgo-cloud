package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	membersvc "alexGo-cloud/modules/member/service"
	"alexGo-cloud/modules/scorecard/model"
	"alexGo-cloud/modules/scorecard/repository"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/tenant/gormplugin"
)

// newFriendServiceForTest 独立内存库（好友链路不依赖房间数据）。
// 相对 brief 原文的最小适配（详见 task-11-report 偏差节）：
//  1. 原文调用 tenant.WithTenantID 却未 import —— 补 `"alexGo-cloud/pkg/tenant"`；
//  2. 原文构造与返回两个互不相干的 fakeBriefs{}（均无 rec），返回值无法观测 ——
//     改为同一 stub 值（值接收者 + 指针 rec 字段，注入服务与返回值共享记录器），
//     供 List 用例断言 GetBriefs 的租户 ctx 与批量 ids（T6 快照用例先例）；
//  3. gormplugin.Register 的 error 显式检查（与 newRoomServiceForTest 对齐）。
func newFriendServiceForTest(t *testing.T) (FriendService, fakeBriefs) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Friend{}, &model.FriendBlack{}, &model.Record{}); err != nil {
		t.Fatal(err)
	}
	if err := gormplugin.Register(db, gormplugin.Options{ExemptTables: []string{"tenants", "casbin_rule"}}); err != nil {
		t.Fatal(err)
	}
	stub := fakeBriefs{rec: &briefRec{}}
	return NewFriendService(db, repository.NewFriendRepository(db),
		repository.NewFriendBlackRepository(db), repository.NewRecordRepository(db), stub), stub
}

func TestFriend_AddBlackRank(t *testing.T) {
	fs, briefs := newFriendServiceForTest(t)
	ctx := tenant.WithTenantID(context.Background(), 1)
	_ = briefs // fake BriefProvider
	if err := fs.Add(ctx, 1, 2); err != nil {
		t.Fatal(err)
	}
	if err := fs.Add(ctx, 1, 2); err == nil { // 重复 → 30109
		t.Fatal("重复加好友应拒")
	}
	if err := fs.SetBlack(ctx, 2, 1, true); err != nil {
		t.Fatal(err)
	}
	// 拉黑后：1 再加 3 被 3 拉黑不了——此处断言 Exists 双向判定
	if err := fs.Add(ctx, 1, 2); err == nil { // 关系已存在仍 30109，先 Remove 再 Add 才测拉黑拦截
		t.Fatal("重复")
	}
	if err := fs.Remove(ctx, 1, 2); err != nil {
		t.Fatal(err)
	}
	if err := fs.Add(ctx, 1, 2); err == nil {
		t.Fatal("拉黑期间加好友应 30108")
	}
	if err := fs.SetBlack(ctx, 2, 1, false); err != nil {
		t.Fatal(err)
	}
	if err := fs.Add(ctx, 1, 2); err != nil {
		t.Fatal(err)
	}
	if err := fs.SetRank(ctx, 1, 2, false); err != nil {
		t.Fatal(err)
	}
	items, err := fs.List(ctx, 1)
	if err != nil || len(items) != 1 || items[0].RankEnabled {
		t.Fatalf("排名开关: %+v err=%v", items, err)
	}
}

// TestFriend_ListRankBriefsTenant 排名列表：total_score 降序（同分 user_id 升序）、
// 昵称批量补齐且 GetBriefs 透传租户编号 ctx（T6 TestSnapshot_BriefsCarryTenantCtx 先例）、
// WinRate=win/(win+lose)（分母 0 → 0）、IsBlocked 取「我拉黑对方」的单向行、
// 我自己的战绩不进好友榜。
func TestFriend_ListRankBriefsTenant(t *testing.T) {
	fs, briefs := newFriendServiceForTest(t)
	ctx := tenant.WithTenantID(context.Background(), 1)
	for _, id := range []uint64{2, 3, 4} {
		if err := fs.Add(ctx, 1, id); err != nil {
			t.Fatal(err)
		}
	}
	// 战绩：u3=50（1胜）；u2=30-10=20（1胜1负 → 0.5）；u4 无战绩；u1 自己 100 不进榜。
	inner := fs.(*friendService)
	if err := inner.record.Create(ctx, []*model.Record{
		{TenantID: 1, RoomID: 9, UserID: 3, Score: 50, SettleType: 1},
		{TenantID: 1, RoomID: 9, UserID: 2, Score: 30, SettleType: 1},
		{TenantID: 1, RoomID: 9, UserID: 2, Score: -10, SettleType: 1},
		{TenantID: 1, RoomID: 9, UserID: 1, Score: 100, SettleType: 1},
	}); err != nil {
		t.Fatal(err)
	}
	items, err := fs.List(ctx, 1)
	if err != nil || len(items) != 3 {
		t.Fatalf("好友排名: %+v err=%v", items, err)
	}
	// total_score 降序：u3=50 → u2=20 → u4=0
	if items[0].UserID != 3 || items[0].TotalScore != 50 || items[0].WinRate != 1 {
		t.Fatalf("榜首应 u3(50): %+v", items[0])
	}
	if items[1].UserID != 2 || items[1].TotalScore != 20 || items[1].WinRate != 0.5 {
		t.Fatalf("次席应 u2(20, 胜率0.5): %+v", items[1])
	}
	if items[2].UserID != 4 || items[2].TotalScore != 0 || items[2].WinRate != 0 {
		t.Fatalf("末席应 u4(0): %+v", items[2])
	}
	for _, it := range items {
		if it.Nickname != fmt.Sprintf("u%d", it.UserID) {
			t.Fatalf("昵称应由 BriefProvider 批量补齐: %+v", it)
		}
		if !it.RankEnabled {
			t.Fatalf("Add 后默认参与排名: %+v", it)
		}
		if it.IsBlocked {
			t.Fatalf("未拉黑不应置位: %+v", it)
		}
	}
	// GetBriefs 批量 + 租户 ctx（List 是本用例最后一次调用 → ids 为全部好友）
	if briefs.rec == nil || briefs.rec.tenantID != 1 {
		t.Fatalf("GetBriefs 应携带租户编号 1, got %+v", briefs.rec)
	}
	if len(briefs.rec.ids) != 3 || briefs.rec.ids[0] != 2 || briefs.rec.ids[1] != 3 || briefs.rec.ids[2] != 4 {
		t.Fatalf("GetBriefs 应按好友行批量传 id, got %v", briefs.rec.ids)
	}
	// 我拉黑 u4 → IsBlocked 置位，行仍返回（spec §friend 未定义按拉黑过滤，只带标记）
	if err := fs.SetBlack(ctx, 1, 4, true); err != nil {
		t.Fatal(err)
	}
	items, err = fs.List(ctx, 1)
	if err != nil || len(items) != 3 {
		t.Fatalf("拉黑后列表仍 3 行: %+v err=%v", items, err)
	}
	for _, it := range items {
		if it.IsBlocked != (it.UserID == 4) {
			t.Fatalf("IsBlocked 应取我方拉黑行: %+v", it)
		}
	}
}

// TestFriend_AddValidation 校验与错误码钉死：自加/零值 10001、对方不存在 30002、
// 重复 30109（关系与拉黑并存时给 30109 —— 给定用例注释「关系已存在仍 30109」）、
// 无关系拉黑拦截 30108；Remove 双向删行、SetBlack 幂等、SetRank 单行空更新无害。
func TestFriend_AddValidation(t *testing.T) {
	fs, _ := newFriendServiceForTest(t)
	ctx := tenant.WithTenantID(context.Background(), 1)
	inner := fs.(*friendService)

	if err := fs.Add(ctx, 5, 5); !errs.Is(err, errs.CodeParamInvalid) {
		t.Fatalf("自加应 10001, got %v", err)
	}
	if err := fs.Add(ctx, 0, 5); !errs.Is(err, errs.CodeParamInvalid) {
		t.Fatalf("零值应 10001, got %v", err)
	}

	// 对方不存在：fakeBriefs 按 id 恒造行，换空桩测 30002。
	svcMissing := NewFriendService(inner.db, repository.NewFriendRepository(inner.db),
		repository.NewFriendBlackRepository(inner.db), repository.NewRecordRepository(inner.db), missingBriefs{})
	if err := svcMissing.Add(ctx, 1, 404); !errs.Is(err, errs.CodeUserNotFound) {
		t.Fatalf("对方不存在应 30002, got %v", err)
	}

	// 重复加 → 30109。
	if err := fs.Add(ctx, 1, 2); err != nil {
		t.Fatal(err)
	}
	if err := fs.Add(ctx, 1, 2); !errs.Is(err, errs.CodeScorecardFriendTaken) {
		t.Fatalf("重复加好友应 30109, got %v", err)
	}
	// 关系存在 + 双方拉黑并存 → 30109（给定用例注释钉的语义：先报已存在）。
	if err := fs.SetBlack(ctx, 2, 1, true); err != nil {
		t.Fatal(err)
	}
	if err := fs.Add(ctx, 1, 2); !errs.Is(err, errs.CodeScorecardFriendTaken) {
		t.Fatalf("关系在先应 30109, got %v", err)
	}
	// Remove 双向删行（Add 对称：两行同生同灭，否则重建时反向行撞唯一键）。
	if err := fs.Remove(ctx, 1, 2); err != nil {
		t.Fatal(err)
	}
	if r, err := inner.friend.Get(ctx, 1, 2); err != nil || r != nil {
		t.Fatalf("我方行应删净: %v err=%v", r, err)
	}
	if r, err := inner.friend.Get(ctx, 2, 1); err != nil || r != nil {
		t.Fatalf("反向行应删净: %v err=%v", r, err)
	}
	// 拉黑仍在 → 30108。
	if err := fs.Add(ctx, 1, 2); !errs.Is(err, errs.CodeScorecardBlacklisted) {
		t.Fatalf("拉黑期间加好友应 30108, got %v", err)
	}
	// SetBlack(true) 幂等、解除（false）幂等，之后可正常重建。
	if err := fs.SetBlack(ctx, 2, 1, true); err != nil {
		t.Fatalf("重复拉黑应幂等: %v", err)
	}
	if err := fs.SetBlack(ctx, 2, 1, false); err != nil {
		t.Fatal(err)
	}
	if err := fs.SetBlack(ctx, 2, 1, false); err != nil {
		t.Fatalf("重复解除应幂等: %v", err)
	}
	if err := fs.Add(ctx, 1, 2); err != nil {
		t.Fatal(err)
	}
	// SetRank 只动我方单行；对不存在的组合是无害空更新（无独立错误码，幂等语义）。
	if err := fs.SetRank(ctx, 1, 77, false); err != nil {
		t.Fatalf("空更新应无害: %v", err)
	}
}

// TestFriend_AdminList 监管端分页直查（Task 13 消费面）：total 计全量、size 截断、
// 跨租户行不可见（tenant plugin 隔离，Paginate 内 WithContext(ctx)）。
func TestFriend_AdminList(t *testing.T) {
	fs, _ := newFriendServiceForTest(t)
	ctx := tenant.WithTenantID(context.Background(), 1)
	if err := fs.Add(ctx, 1, 2); err != nil { // 双向 2 行（tenant 1）
		t.Fatal(err)
	}
	ctx2 := tenant.WithTenantID(context.Background(), 2)
	if err := fs.Add(ctx2, 8, 9); err != nil { // 另一租户 2 行，不应混入
		t.Fatal(err)
	}
	list, total, err := fs.AdminList(ctx, 1, 10)
	if err != nil || total != 2 || len(list) != 2 {
		t.Fatalf("监管分页: total=%d len=%d err=%v", total, len(list), err)
	}
	list, total, err = fs.AdminList(ctx, 1, 1)
	if err != nil || total != 2 || len(list) != 1 {
		t.Fatalf("size=1 截断但 total 计全量: total=%d len=%d err=%v", total, len(list), err)
	}
}

// missingBriefs 空简报桩：GetBriefs 无命中（fakeBriefs 按 id 恒造行，测不出 30002）。
type missingBriefs struct{}

func (missingBriefs) GetBriefs(context.Context, []uint64) ([]membersvc.UserBrief, error) {
	return nil, nil
}
