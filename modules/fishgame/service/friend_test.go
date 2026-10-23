package service

import (
	"errors"
	"testing"

	"gorm.io/gorm"

	"alexGo-cloud/modules/fishgame/model"
	"alexGo-cloud/modules/fishgame/repository"
)

func seedPlayer(t *testing.T, db *gorm.DB, openid, nick string) *model.Player {
	t.Helper()
	p, err := repository.NewPlayerRepository(db).UpsertByOpenID(t.Context(), openid, nick, "")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func newFriendService(t *testing.T) (*FriendService, *gorm.DB) {
	t.Helper()
	db := newTestDB(t)
	players := repository.NewPlayerRepository(db)
	friends := repository.NewFriendRepository(db)
	dex := repository.NewDexRepository(db)
	svc := NewFriendService(NewStoredFriendSource(friends, players), players, friends, dex)
	return svc, db
}

func TestFriendService_Add_TargetNotFound(t *testing.T) {
	svc, _ := newFriendService(t)
	self := &model.Player{ID: 1, OpenID: "openid-a"}
	err := svc.Add(t.Context(), self, "openid-ghost")
	if !errors.Is(err, ErrTargetNotFound) {
		t.Fatalf("应 ErrTargetNotFound, got %v", err)
	}
}

func TestFriendService_Add_SelfFriend(t *testing.T) {
	svc, _ := newFriendService(t)
	self := &model.Player{ID: 1, OpenID: "openid-a"}
	err := svc.Add(t.Context(), self, "openid-a")
	if !errors.Is(err, ErrSelfFriend) {
		t.Fatalf("应 ErrSelfFriend, got %v", err)
	}
}

func TestFriendService_Add_Idempotent(t *testing.T) {
	svc, db := newFriendService(t)
	self := seedPlayer(t, db, "openid-a", "A")
	other := seedPlayer(t, db, "openid-b", "B")
	if err := svc.Add(t.Context(), self, other.OpenID); err != nil {
		t.Fatal(err)
	}
	if err := svc.Add(t.Context(), self, other.OpenID); err != nil {
		t.Fatalf("幂等再加应无错: %v", err)
	}
	var n int64
	if err := db.Model(&model.Friend{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("双向应 2 行, got %d", n)
	}
}

func TestFriendService_Rank_FriendsNonEmpty(t *testing.T) {
	svc, db := newFriendService(t)
	self := seedPlayer(t, db, "openid-a", "self")
	f1 := seedPlayer(t, db, "openid-b", "buddy")
	_ = seedPlayer(t, db, "openid-c", "outsider")

	if err := repository.NewFriendRepository(db).AddPair(t.Context(), self.ID, f1.ID); err != nil {
		t.Fatal(err)
	}
	seedDex(t, db, self.ID, "inkfish", 2)
	seedDex(t, db, f1.ID, "inkfish", 5)
	seedDex(t, db, f1.ID, "ghostfin", 1)
	// outsider 点亮更多但不在好友 ∪ 自己
	_ = seedPlayer(t, db, "openid-d", "rival")
	rival := seedPlayer(t, db, "openid-e", "rival2")
	seedDex(t, db, rival.ID, "inkfish", 9)

	rows, err := svc.Rank(t.Context(), self)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("应好友∪自己 2 行, got %+v", rows)
	}
	if rows[0].Nick != "buddy" || rows[0].Dex != 2 {
		t.Fatalf("buddy 2 点亮应居首: %+v", rows)
	}
	if rows[1].Nick != "self" || rows[1].Dex != 1 {
		t.Fatalf("self 1 点亮次之: %+v", rows)
	}
}

func TestFriendService_Rank_EmptyFallsBackToAll(t *testing.T) {
	svc, db := newFriendService(t)
	a := seedPlayer(t, db, "openid-a", "aaa")
	b := seedPlayer(t, db, "openid-b", "bbb")
	_ = seedPlayer(t, db, "openid-c", "ccc") // 0 点亮垫底

	seedDex(t, db, a.ID, "inkfish", 1)
	seedDex(t, db, b.ID, "inkfish", 3)
	seedDex(t, db, b.ID, "ghostfin", 1) // 点亮数 = 条目行数，bbb 2 条 > aaa 1 条

	rows, err := svc.Rank(t.Context(), a)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("空好友应全体 3 行, got %+v", rows)
	}
	if rows[0].Nick != "bbb" || rows[0].Dex != 2 {
		t.Fatalf("应按 dex 降序: %+v", rows)
	}
	if rows[1].Nick != "aaa" {
		t.Fatalf("自己应第 2: %+v", rows)
	}
	if rows[2].Nick != "ccc" || rows[2].Dex != 0 {
		t.Fatalf("0 点亮垫底: %+v", rows)
	}
}

// seedDex 直插点亮行（count≥1，绕过 UpsertCatch 逐次递增的啰嗦）。
func seedDex(t *testing.T, db *gorm.DB, playerID uint64, fishID string, count int) {
	t.Helper()
	var d model.Dex
	err := db.Where("player_id = ? AND fish_id = ?", playerID, fishID).First(&d).Error
	if err == nil {
		if err := db.Model(&model.Dex{}).Where("id = ?", d.ID).Update("count", count).Error; err != nil {
			t.Fatal(err)
		}
		return
	}
	d = model.Dex{TenantID: 0, PlayerID: playerID, FishID: fishID, Count: count}
	if err := db.Create(&d).Error; err != nil {
		t.Fatal(err)
	}
}
