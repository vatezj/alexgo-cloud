package service

import (
	"strconv"
	"testing"
	"time"

	"gorm.io/gorm"

	"alexGo-cloud/modules/fishgame/model"
	"alexGo-cloud/modules/fishgame/repository"
	errs "alexGo-cloud/pkg/errors"
)

func newAdminService(t *testing.T) (*AdminService, *gorm.DB) {
	t.Helper()
	db := newTestDB(t)
	players := repository.NewPlayerRepository(db)
	friends := repository.NewFriendRepository(db)
	dex := repository.NewDexRepository(db)
	friend := NewFriendService(NewStoredFriendSource(friends, players), players, friends, dex)
	svc := NewAdminService(
		repository.NewFishRepository(db),
		repository.NewConfigRepository(db),
		repository.NewTrackEventRepository(db),
		players,
		dex,
		friend,
	)
	return svc, db
}

func seedFish(t *testing.T, db *gorm.DB, fishID, rarity string, status int) *model.Fish {
	t.Helper()
	f := &model.Fish{TenantID: 0, FishID: fishID, Name: fishID, Rarity: rarity,
		Period: "any", Depth: "any", Sort: 1, Status: status}
	if err := repository.NewFishRepository(db).Create(t.Context(), f); err != nil {
		t.Fatal(err)
	}
	return f
}

func codeOf(t *testing.T, err error) errs.Code {
	t.Helper()
	be, ok := errs.As(err)
	if !ok {
		t.Fatalf("应 *BizError, got %v", err)
	}
	return be.Code
}

// ---- fish ----

func TestAdminService_ListFish_FilterAndPage(t *testing.T) {
	svc, db := newAdminService(t)
	seedFish(t, db, "inkfish", "normal", 1)
	seedFish(t, db, "ghostfin", "rare", 1)
	seedFish(t, db, "oldfish", "rare", 0)

	list, total, err := svc.ListFish(t.Context(), "rare", -1, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("rarity=rare 应 2 行, total=%d len=%d", total, len(list))
	}

	list, total, err = svc.ListFish(t.Context(), "rare", 1, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || list[0].FishID != "ghostfin" {
		t.Fatalf("rarity+status 组合筛错: total=%d %+v", total, list)
	}

	list, _, err = svc.ListFish(t.Context(), "", -1, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("page=2&size=1 应 1 行, got %d", len(list))
	}
}

func TestAdminService_CreateFish_OK(t *testing.T) {
	svc, _ := newAdminService(t)
	f, err := svc.CreateFish(t.Context(), FishInput{
		FishID: "inkfish", Name: "墨鱼", Rarity: "normal",
		Period: "day,night", Depth: "mid", Sort: 1, Status: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if f.ID == 0 || f.FishID != "inkfish" {
		t.Fatalf("应落库并回填 id: %+v", f)
	}
}

func TestAdminService_CreateFish_DuplicateFishID(t *testing.T) {
	svc, db := newAdminService(t)
	seedFish(t, db, "inkfish", "normal", 1)
	_, err := svc.CreateFish(t.Context(), FishInput{FishID: "inkfish", Name: "x", Rarity: "normal", Period: "any", Depth: "any"})
	if got := codeOf(t, err); got != errs.CodeFishgameFishNotFound {
		t.Fatalf("重复 fish_id 应 30401, got %d (%v)", got, err)
	}
}

func TestAdminService_CreateFish_EnumInvalid(t *testing.T) {
	svc, _ := newAdminService(t)
	cases := []struct {
		name string
		in   FishInput
	}{
		{"rarity", FishInput{FishID: "a", Rarity: "legend", Period: "any", Depth: "any"}},
		{"period", FishInput{FishID: "a", Rarity: "normal", Period: "day,evil", Depth: "any"}},
		{"depth", FishInput{FishID: "a", Rarity: "normal", Period: "any", Depth: "abyss"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := svc.CreateFish(t.Context(), c.in)
			if got := codeOf(t, err); got != errs.CodeParamInvalid {
				t.Fatalf("枚举非法应 10001, got %d (%v)", got, err)
			}
		})
	}
}

func TestAdminService_UpdateFish_NotFound(t *testing.T) {
	svc, _ := newAdminService(t)
	_, err := svc.UpdateFish(t.Context(), 999, FishInput{FishID: "x", Rarity: "normal", Period: "any", Depth: "any"})
	if got := codeOf(t, err); got != errs.CodeFishgameFishNotFound {
		t.Fatalf("更新查无应 30401, got %d (%v)", got, err)
	}
}

func TestAdminService_DeleteFish_OK(t *testing.T) {
	svc, db := newAdminService(t)
	f := seedFish(t, db, "inkfish", "normal", 1)
	if err := svc.DeleteFish(t.Context(), f.ID); err != nil {
		t.Fatal(err)
	}
	var n int64
	if err := db.Model(&model.Fish{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("应删行, 剩 %d", n)
	}
}

func TestAdminService_DeleteFish_ReferencedByDex(t *testing.T) {
	svc, db := newAdminService(t)
	f := seedFish(t, db, "inkfish", "normal", 1)
	seedDex(t, db, 1, "inkfish", 3)
	if err := svc.DeleteFish(t.Context(), f.ID); err == nil {
		t.Fatal("被 dex 引用应拒删")
	} else if got := codeOf(t, err); got != errs.CodeFishgameFishReferenced {
		t.Fatalf("应 30402, got %d (%v)", got, err)
	}
}

func TestAdminService_DeleteFish_ReferencedByEvent(t *testing.T) {
	svc, db := newAdminService(t)
	f := seedFish(t, db, "inkfish", "normal", 1)
	ev := &model.TrackEvent{TenantID: 0, PlayerID: 1, Name: "catch", T: 1000,
		Props: `{"fish_id":"inkfish","kind":"fish"}`, DedupKey: "ref-1"}
	if err := db.Create(ev).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteFish(t.Context(), f.ID); err == nil {
		t.Fatal("被事件引用应拒删")
	} else if got := codeOf(t, err); got != errs.CodeFishgameFishReferenced {
		t.Fatalf("应 30402, got %d (%v)", got, err)
	}
}

// ---- config ----

func TestAdminService_CreateConfig_FishKingInvalid(t *testing.T) {
	svc, db := newAdminService(t)
	// 不存在的 fish_id
	_, err := svc.CreateConfig(t.Context(), ConfigInput{
		Type: "fish_king", FishID: "ghost", Week: "2026-W43", Status: 1,
	})
	if got := codeOf(t, err); got != errs.CodeFishgameFishKingInvalid {
		t.Fatalf("fish_id 不存在应 30403, got %d (%v)", got, err)
	}
	// 停用的 fish_id
	seedFish(t, db, "disabled", "rare", 0)
	_, err = svc.CreateConfig(t.Context(), ConfigInput{
		Type: "fish_king", FishID: "disabled", Week: "2026-W43", Status: 1,
	})
	if got := codeOf(t, err); got != errs.CodeFishgameFishKingInvalid {
		t.Fatalf("fish_id 停用应 30403, got %d (%v)", got, err)
	}
}

func TestAdminService_CreateConfig_LuckValidation(t *testing.T) {
	svc, db := newAdminService(t)
	// depth_bias > 0.3 拒绝
	_, err := svc.CreateConfig(t.Context(), ConfigInput{
		Type: "luck", Period: "day", DepthBias: 0.5, Status: 1,
	})
	if got := codeOf(t, err); got != errs.CodeParamInvalid {
		t.Fatalf("depth_bias>0.3 应 10001, got %d (%v)", got, err)
	}
	// 合法 luck 落库
	c, err := svc.CreateConfig(t.Context(), ConfigInput{
		Type: "luck", Period: "night", DepthBias: 0.2, Status: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.ID == 0 {
		t.Fatal("luck 配置应落库")
	}
	var n int64
	if err := db.Model(&model.Config{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("应 1 行, got %d", n)
	}
}

func TestAdminService_UpdateAndDeleteConfig_NotFound(t *testing.T) {
	svc, _ := newAdminService(t)
	_, err := svc.UpdateConfig(t.Context(), 999, ConfigInput{Type: "luck", Period: "day", Status: 1})
	if got := codeOf(t, err); got != errs.CodeFishgameConfigNotFound {
		t.Fatalf("更新查无应 30404, got %d (%v)", got, err)
	}
	err = svc.DeleteConfig(t.Context(), 999)
	if got := codeOf(t, err); got != errs.CodeFishgameConfigNotFound {
		t.Fatalf("删除查无应 30404, got %d (%v)", got, err)
	}
}

func TestAdminService_ListConfig_ByType(t *testing.T) {
	svc, db := newAdminService(t)
	if err := repository.NewConfigRepository(db).Create(t.Context(),
		&model.Config{TenantID: 0, Type: "luck", Period: "day", Status: 1}); err != nil {
		t.Fatal(err)
	}
	if err := repository.NewConfigRepository(db).Create(t.Context(),
		&model.Config{TenantID: 0, Type: "fish_king", FishID: "inkfish", Week: "2026-W43", Status: 1}); err != nil {
		t.Fatal(err)
	}
	list, total, err := svc.ListConfig(t.Context(), "luck", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || list[0].Type != "luck" {
		t.Fatalf("type 筛选错: total=%d %+v", total, list)
	}
}

// ---- events ----

func TestAdminService_ListEvents_FilterDesc(t *testing.T) {
	svc, db := newAdminService(t)
	mk := func(name string, playerID uint64, tt int64, key string) {
		t.Helper()
		e := &model.TrackEvent{TenantID: 0, PlayerID: playerID, Name: name, T: tt,
			Props: `{}`, DedupKey: key}
		if err := db.Create(e).Error; err != nil {
			t.Fatal(err)
		}
	}
	mk("catch", 1, 100, "e1")
	mk("catch", 2, 200, "e2")
	mk("sign_in", 1, 300, "e3")

	list, total, err := svc.ListEvents(t.Context(), "catch", 0, 0, 0, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("name=catch 应 2 行, got %d", total)
	}
	if list[0].T != 200 || list[1].T != 100 {
		t.Fatalf("应 id 倒序: %+v", []int64{list[0].T, list[1].T})
	}

	_, total, err = svc.ListEvents(t.Context(), "", 1, 150, 0, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Fatalf("player+起始时间筛应 1 行, got %d", total)
	}

	_, total, err = svc.ListEvents(t.Context(), "", 0, 0, 150, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Fatalf("截止时间筛应 1 行（t≤150 只剩 e1）, got %d", total)
	}
}

// ---- players ----

func TestAdminService_ListPlayers_FuzzyAndDex(t *testing.T) {
	svc, db := newAdminService(t)
	a := seedPlayer(t, db, "openid-alpha", "阿尔法")
	b := seedPlayer(t, db, "openid-beta", "贝塔")
	seedDex(t, db, a.ID, "inkfish", 3)
	seedDex(t, db, a.ID, "ghostfin", 1)
	seedDex(t, db, b.ID, "inkfish", 1)

	list, total, err := svc.ListPlayers(t.Context(), "alpha", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || list[0].ID != a.ID || list[0].Dex != 2 {
		t.Fatalf("openid 模糊搜错: total=%d %+v", total, list)
	}

	list, total, err = svc.ListPlayers(t.Context(), "贝", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || list[0].ID != b.ID || list[0].Dex != 1 {
		t.Fatalf("nick 模糊搜错: total=%d %+v", total, list)
	}

	_, total, err = svc.ListPlayers(t.Context(), "", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("空关键词应全体 2 行, got %d", total)
	}
}

func TestAdminService_PlayerDetail(t *testing.T) {
	svc, db := newAdminService(t)
	p := seedPlayer(t, db, "openid-a", "玩家甲")
	seedDex(t, db, p.ID, "inkfish", 3)
	seedDex(t, db, p.ID, "ghostfin", 0) // count=0 不算点亮
	for i := 0; i < 25; i++ {
		e := &model.TrackEvent{TenantID: 0, PlayerID: p.ID, Name: "catch", T: int64(1000 + i),
			Props: `{}`, DedupKey: "detail-" + string(rune('a'+i))}
		if err := db.Create(e).Error; err != nil {
			t.Fatal(err)
		}
	}

	d, err := svc.PlayerDetail(t.Context(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if d.Player.OpenID != "openid-a" {
		t.Fatalf("基本信息错: %+v", d.Player)
	}
	if len(d.Lit) != 1 || d.Lit[0].FishID != "inkfish" || d.Lit[0].Count != 3 {
		t.Fatalf("点亮明细应过滤 count>=1: %+v", d.Lit)
	}
	if len(d.Events) != 20 {
		t.Fatalf("最近事件应 20 条, got %d", len(d.Events))
	}
	if d.Events[0].T != 1024 {
		t.Fatalf("事件应倒序（最新在前）: 首条 T=%d", d.Events[0].T)
	}
}

func TestAdminService_PlayerDetail_NotFound(t *testing.T) {
	svc, _ := newAdminService(t)
	_, err := svc.PlayerDetail(t.Context(), 999)
	if got := codeOf(t, err); got != errs.CodeUserNotFound {
		t.Fatalf("玩家查无应 30002, got %d (%v)", got, err)
	}
}

func TestAdminService_PlayerRank_Position(t *testing.T) {
	svc, db := newAdminService(t)
	a := seedPlayer(t, db, "openid-a", "aaa")
	b := seedPlayer(t, db, "openid-b", "bbb")
	c := seedPlayer(t, db, "openid-c", "ccc")
	seedDex(t, db, a.ID, "inkfish", 1)
	seedDex(t, db, b.ID, "inkfish", 3)
	seedDex(t, db, b.ID, "ghostfin", 1)
	seedDex(t, db, c.ID, "inkfish", 2)
	seedDex(t, db, c.ID, "ghostfin", 1)

	// 空好友 → 全体口径：点亮数=行数 bbb(2)=ccc(2) 按 nick 决胜，aaa(1) 垫底
	pos, err := svc.PlayerRank(t.Context(), c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pos.Position != 2 || pos.Total != 3 {
		t.Fatalf("ccc 应第 2/共 3: %+v", pos)
	}

	pos, err = svc.PlayerRank(t.Context(), b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pos.Position != 1 {
		t.Fatalf("bbb 点亮最多应第 1: %+v", pos)
	}
}

func TestAdminService_PlayerRank_NotFound(t *testing.T) {
	svc, _ := newAdminService(t)
	_, err := svc.PlayerRank(t.Context(), 999)
	if got := codeOf(t, err); got != errs.CodeUserNotFound {
		t.Fatalf("玩家查无应 30002, got %d (%v)", got, err)
	}
}

// ---- stats ----

func TestAdminService_Stats(t *testing.T) {
	svc, db := newAdminService(t)
	now := time.Now()

	old := seedPlayer(t, db, "openid-old", "老玩家")
	if err := db.Model(&model.Player{}).Where("id = ?", old.ID).
		Update("created_at", now.Add(-48*time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	seedPlayer(t, db, "openid-new", "新玩家") // created_at = 今天

	seq := 0
	mk := func(name string, tt int64) {
		t.Helper()
		seq++
		e := &model.TrackEvent{TenantID: 0, PlayerID: 1, Name: name, T: tt,
			Props: `{}`, DedupKey: "st-" + strconv.Itoa(seq)}
		if err := db.Create(e).Error; err != nil {
			t.Fatal(err)
		}
	}
	d1 := now.Add(-24 * time.Hour)
	d2 := now.Add(-48 * time.Hour)
	mk("catch", d1.Unix())
	mk("catch", d1.Unix())
	mk("empty", d1.Unix())
	mk("escape", d2.Unix())
	mk("sign_in", d1.Unix())
	mk("sign_in", d1.Unix())
	mk("sign_in", d2.Unix())
	mk("basket_claim", d1.Unix())
	// 终审 Important#2：客户端实际上报 ad_request/ad_rewarded/ad_skip/ad_fail，
	// 不存在名为 ad 的事件——广告计数须聚合真实事件名。
	mk("ad_request", d1.Unix())
	mk("ad_rewarded", d1.Unix())
	mk("ad_skip", d2.Unix())
	mk("page_view", d2.Unix())
	mk("catch", now.Add(-8*24*time.Hour).Unix()) // 窗外，不计

	st, err := svc.Stats(t.Context(), now)
	if err != nil {
		t.Fatal(err)
	}
	if st.PlayerTotal != 2 {
		t.Fatalf("玩家总数应 2, got %d", st.PlayerTotal)
	}
	if st.PlayerToday != 1 {
		t.Fatalf("今日新增应 1, got %d", st.PlayerToday)
	}
	if st.EventCounts["catch"] != 2 {
		t.Fatalf("catch 近7日应 2（窗外不计）, got %d", st.EventCounts["catch"])
	}
	if st.EventCounts["page_view"] != 1 {
		t.Fatalf("page_view 应 1, got %d", st.EventCounts["page_view"])
	}

	wantFunnel := map[string]int64{"catch": 2, "empty": 1, "escape": 1, "broken": 0}
	for k, want := range wantFunnel {
		if st.Funnel[k] != want {
			t.Fatalf("funnel[%s] 应 %d, got %d", k, want, st.Funnel[k])
		}
	}
	if r := st.FunnelRatio["catch"]; r < 0.49 || r > 0.51 {
		t.Fatalf("catch 占比应 0.5, got %f", r)
	}
	if st.SignCount != 3 || st.BasketCount != 1 || st.AdCount != 3 {
		t.Fatalf("签到/领筐/广告应 3/1/3（ad_* 三事件聚合）, got %d/%d/%d",
			st.SignCount, st.BasketCount, st.AdCount)
	}

	// 按日趋势：d1 的 catch 应聚成一条 count=2
	day := time.Unix(d1.Unix(), 0).Format("2006-01-02")
	var found bool
	for _, row := range st.Daily {
		if row.Day == day && row.Name == "catch" {
			found = true
			if row.Count != 2 {
				t.Fatalf("%s catch 应 2, got %d", day, row.Count)
			}
		}
	}
	if !found {
		t.Fatalf("Daily 应含 %s catch: %+v", day, st.Daily)
	}

	// 错误路径不吞：不存在的不该出现
	if _, ok := st.EventCounts["bogus"]; ok {
		t.Fatal("不应有 bogus 事件")
	}
}
