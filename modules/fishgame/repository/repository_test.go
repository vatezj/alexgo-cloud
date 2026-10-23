package repository

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/fishgame/model"
	"alexGo-cloud/pkg/tenant/gormplugin"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&model.Player{}, &model.Session{}, &model.Fish{}, &model.Dex{},
		&model.TrackEvent{}, &model.Sign{}, &model.Friend{}, &model.Config{}, &model.Share{},
	); err != nil {
		t.Fatal(err)
	}
	if err := gormplugin.Register(db, gormplugin.Options{ExemptTables: []string{"tenants", "casbin_rule"}}); err != nil {
		t.Fatal(err)
	}
	return db
}

// 游戏端 ctx：无租户注入（tid=0 钉底）。
func gameCtx() context.Context { return context.Background() }

func TestPlayer_UpsertByOpenID_Idempotent(t *testing.T) {
	db := newTestDB(t)
	repo := NewPlayerRepository(db)
	ctx := gameCtx()

	p1, err := repo.UpsertByOpenID(ctx, "openid-abc", "微信用户abcd", "http://a/1.png")
	if err != nil {
		t.Fatal(err)
	}
	if p1.ID == 0 || p1.TenantID != 0 {
		t.Fatalf("首次插入应有 ID 且 tenant_id=0: %+v", p1)
	}

	p2, err := repo.UpsertByOpenID(ctx, "openid-abc", "新昵称", "http://a/2.png")
	if err != nil {
		t.Fatal(err)
	}
	if p2.ID != p1.ID {
		t.Fatalf("同 openid 应更新不新增: p1=%d p2=%d", p1.ID, p2.ID)
	}
	if p2.Nick != "新昵称" {
		t.Fatalf("nick 应更新为新昵称, got %q", p2.Nick)
	}
	var n int64
	if err := db.Model(&model.Player{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("应只有 1 行, got %d", n)
	}
}

func TestSession_GetByToken_HitAndExpireMiss(t *testing.T) {
	db := newTestDB(t)
	repo := NewSessionRepository(db)
	ctx := gameCtx()

	tok := "tok-valid"
	s := &model.Session{TenantID: 0, Token: tok, PlayerID: 1, ExpireAt: time.Now().Add(24 * time.Hour)}
	if err := repo.Create(ctx, s); err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetByToken(ctx, tok)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.PlayerID != 1 {
		t.Fatalf("有效 token 应命中: %+v", got)
	}

	expired := &model.Session{TenantID: 0, Token: "tok-expired", PlayerID: 2, ExpireAt: time.Now().Add(-time.Minute)}
	if err := repo.Create(ctx, expired); err != nil {
		t.Fatal(err)
	}
	miss, err := repo.GetByToken(ctx, "tok-expired")
	if err != nil {
		t.Fatal(err)
	}
	if miss != nil {
		t.Fatalf("过期 token 应 miss: %+v", miss)
	}
}

func TestDex_UpsertCatch_FirstThenIncrement(t *testing.T) {
	db := newTestDB(t)
	repo := NewDexRepository(db)
	ctx := gameCtx()

	first, err := repo.UpsertCatch(ctx, 1, "inkfish")
	if err != nil {
		t.Fatal(err)
	}
	if !first {
		t.Fatalf("首次插入应 firstInsert=true")
	}

	second, err := repo.UpsertCatch(ctx, 1, "inkfish")
	if err != nil {
		t.Fatal(err)
	}
	if second {
		t.Fatalf("二次应 firstInsert=false")
	}

	var d model.Dex
	if err := db.Where("player_id = ? AND fish_id = ?", 1, "inkfish").First(&d).Error; err != nil {
		t.Fatal(err)
	}
	if d.Count != 2 {
		t.Fatalf("count 应为 2, got %d", d.Count)
	}

	n, err := repo.CountByPlayer(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("点亮数应 1（行数 where count≥1）, got %d", n)
	}
}

func TestDex_MergeReport_MaxWins(t *testing.T) {
	db := newTestDB(t)
	repo := NewDexRepository(db)
	ctx := gameCtx()

	// 已有 catch count=3
	if _, err := repo.UpsertCatch(ctx, 1, "inkfish"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpsertCatch(ctx, 1, "inkfish"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpsertCatch(ctx, 1, "inkfish"); err != nil {
		t.Fatal(err)
	}

	// 上报 5 > 3 → 5
	if err := repo.MergeReport(ctx, 1, map[string]int{"inkfish": 5, "unknown_fish": 9}); err != nil {
		t.Fatal(err)
	}
	var d model.Dex
	if err := db.Where("player_id = ? AND fish_id = ?", 1, "inkfish").First(&d).Error; err != nil {
		t.Fatal(err)
	}
	if d.Count != 5 {
		t.Fatalf("max 合并应 5, got %d", d.Count)
	}

	// 上报 2 < 5 → 仍 5
	if err := repo.MergeReport(ctx, 1, map[string]int{"inkfish": 2}); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("player_id = ? AND fish_id = ?", 1, "inkfish").First(&d).Error; err != nil {
		t.Fatal(err)
	}
	if d.Count != 5 {
		t.Fatalf("低报不应覆盖, got %d", d.Count)
	}
}

func TestTrackEvent_BatchInsertIgnoreDup(t *testing.T) {
	db := newTestDB(t)
	repo := NewTrackEventRepository(db)
	ctx := gameCtx()

	events := []*model.TrackEvent{
		{TenantID: 0, PlayerID: 1, Name: "catch", T: 100, Props: `{"fish_id":"inkfish"}`, DedupKey: "dup-key-1"},
		{TenantID: 0, PlayerID: 1, Name: "catch", T: 100, Props: `{"fish_id":"inkfish"}`, DedupKey: "dup-key-1"}, // 重复
		{TenantID: 0, PlayerID: 1, Name: "sign_in", T: 101, Props: `{}`, DedupKey: "dup-key-2"},
	}
	insertedIdx, err := repo.BatchInsertIgnoreDup(ctx, events)
	if err != nil {
		t.Fatal(err)
	}
	if len(insertedIdx) != 2 {
		t.Fatalf("应只插入 2 条（去重）, got %v", insertedIdx)
	}
	var n int64
	if err := db.Model(&model.TrackEvent{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("库内应 2 条, got %d", n)
	}
}

func TestFriend_AddPair_Bidirectional(t *testing.T) {
	db := newTestDB(t)
	repo := NewFriendRepository(db)
	ctx := gameCtx()

	if err := repo.AddPair(ctx, 1, 2); err != nil {
		t.Fatal(err)
	}
	// 幂等：再插一次不报错、不重复
	if err := repo.AddPair(ctx, 1, 2); err != nil {
		t.Fatal(err)
	}

	ids, err := repo.ListFriendIDs(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != 2 {
		t.Fatalf("1 的好友应 [2], got %v", ids)
	}
	ids, err = repo.ListFriendIDs(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != 1 {
		t.Fatalf("2 的好友应 [1], got %v", ids)
	}

	ok, err := repo.Exists(ctx, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("Exists(1,2) 应 true")
	}
	ok, err = repo.Exists(ctx, 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatalf("Exists(1,3) 应 false")
	}
}

func TestFish_ListActive_StatusFilter(t *testing.T) {
	db := newTestDB(t)
	repo := NewFishRepository(db)
	ctx := gameCtx()

	for _, f := range []*model.Fish{
		{TenantID: 0, FishID: "inkfish", Name: "墨鱼", Rarity: "normal", Period: "any", Depth: "any", Sort: 1, Status: 1},
		{TenantID: 0, FishID: "off_fish", Name: "停用鱼", Rarity: "normal", Period: "any", Depth: "any", Sort: 2, Status: 0},
	} {
		if err := repo.Create(ctx, f); err != nil {
			t.Fatal(err)
		}
	}

	list, err := repo.ListActive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].FishID != "inkfish" {
		t.Fatalf("ListActive 应只返回 status=1, got %+v", list)
	}

	got, err := repo.GetByFishID(ctx, "inkfish")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Name != "墨鱼" {
		t.Fatalf("GetByFishID 软查应命中: %+v", got)
	}
	miss, err := repo.GetByFishID(ctx, "nope")
	if err != nil {
		t.Fatal(err)
	}
	if miss != nil {
		t.Fatalf("未知 fish_id 应 nil: %+v", miss)
	}
}

func TestSign_GetByPlayerDate(t *testing.T) {
	db := newTestDB(t)
	repo := NewSignRepository(db)
	ctx := gameCtx()

	s := &model.Sign{TenantID: 0, PlayerID: 1, Date: "2026-10-23", Streak: 3, Bait: 5}
	if err := repo.Create(ctx, s); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByPlayerDate(ctx, 1, "2026-10-23")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Streak != 3 || got.Bait != 5 {
		t.Fatalf("应命中签到: %+v", got)
	}
	miss, err := repo.GetByPlayerDate(ctx, 1, "2026-10-22")
	if err != nil {
		t.Fatal(err)
	}
	if miss != nil {
		t.Fatalf("无记录应 nil: %+v", miss)
	}
}
