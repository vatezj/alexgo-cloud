package service

import (
	"testing"

	"gorm.io/gorm"

	"alexGo-cloud/modules/fishgame/model"
	"alexGo-cloud/modules/fishgame/repository"
)

func newTrackService(t *testing.T) (*TrackService, *DexService, *gorm.DB) {
	t.Helper()
	db := newTestDB(t)
	for _, f := range []*model.Fish{
		{TenantID: 0, FishID: "inkfish", Name: "Inkfish", Rarity: "normal", Status: 1, Sort: 1},
		{TenantID: 0, FishID: "ghostfin", Name: "Ghostfin", Rarity: "rare", Status: 1, Sort: 2},
	} {
		if err := repository.NewFishRepository(db).Create(t.Context(), f); err != nil {
			t.Fatal(err)
		}
	}
	fishRepo := repository.NewFishRepository(db)
	dex := NewDexService(fishRepo, repository.NewDexRepository(db))
	track := NewTrackService(
		repository.NewTrackEventRepository(db),
		fishRepo,
		repository.NewDexRepository(db),
	)
	return track, dex, db
}

func countTracks(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&model.TrackEvent{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func TestTrackService_Dedup_SameEventTwice(t *testing.T) {
	track, _, db := newTrackService(t)
	p := &model.Player{TenantID: 0, OpenID: "openid-trk", Nick: "T"}
	if err := db.Create(p).Error; err != nil {
		t.Fatal(err)
	}
	evs := []TrackEventReq{
		{Name: "sign_in", T: 1000, Props: map[string]any{"date": "2026-10-23"}},
	}
	if err := track.Track(t.Context(), p, evs); err != nil {
		t.Fatal(err)
	}
	if err := track.Track(t.Context(), p, evs); err != nil {
		t.Fatal(err)
	}
	if n := countTracks(t, db); n != 1 {
		t.Fatalf("同事件应只 1 行, got %d", n)
	}
}

func TestTrackService_Dedup_PropsKeyOrder(t *testing.T) {
	track, _, db := newTrackService(t)
	p := &model.Player{TenantID: 0, OpenID: "openid-order", Nick: "O"}
	if err := db.Create(p).Error; err != nil {
		t.Fatal(err)
	}
	if err := track.Track(t.Context(), p, []TrackEventReq{
		{Name: "catch", T: 2000, Props: map[string]any{"a": 1, "b": 2}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := track.Track(t.Context(), p, []TrackEventReq{
		{Name: "catch", T: 2000, Props: map[string]any{"b": 2, "a": 1}},
	}); err != nil {
		t.Fatal(err)
	}
	if n := countTracks(t, db); n != 1 {
		t.Fatalf("canonical JSON 应去重, got %d", n)
	}
}

func TestTrackService_Truncate200(t *testing.T) {
	track, _, db := newTrackService(t)
	p := &model.Player{TenantID: 0, OpenID: "openid-trunc", Nick: "U"}
	if err := db.Create(p).Error; err != nil {
		t.Fatal(err)
	}
	evs := make([]TrackEventReq, 250)
	for i := range evs {
		evs[i] = TrackEventReq{Name: "use_bait", T: int64(3000 + i)}
	}
	if err := track.Track(t.Context(), p, evs); err != nil {
		t.Fatal(err)
	}
	if n := countTracks(t, db); n != 200 {
		t.Fatalf("应截 200 条, got %d", n)
	}
}

func TestTrackService_SkipInvalidEvent(t *testing.T) {
	track, _, db := newTrackService(t)
	p := &model.Player{TenantID: 0, OpenID: "openid-skip", Nick: "S"}
	if err := db.Create(p).Error; err != nil {
		t.Fatal(err)
	}
	evs := []TrackEventReq{
		{Name: "", T: 4000},
		{Name: "sign_in", T: 0},
		{Name: "sign_in", T: 4001},
	}
	if err := track.Track(t.Context(), p, evs); err != nil {
		t.Fatal(err)
	}
	if n := countTracks(t, db); n != 1 {
		t.Fatalf("应只落 1 条合法事件, got %d", n)
	}
}

func TestTrackService_Catch_DerivesDex(t *testing.T) {
	track, dex, db := newTrackService(t)
	p := &model.Player{TenantID: 0, OpenID: "openid-catch", Nick: "C"}
	if err := db.Create(p).Error; err != nil {
		t.Fatal(err)
	}
	if err := track.Track(t.Context(), p, []TrackEventReq{
		{Name: "catch", T: 5000, Props: map[string]any{"kind": "fish", "fish_id": "inkfish"}},
		{Name: "catch", T: 5001, Props: map[string]any{"kind": "fish", "fish_id": "no_such"}},
		{Name: "catch", T: 5002, Props: map[string]any{"kind": "empty"}},
	}); err != nil {
		t.Fatal(err)
	}
	list, err := dex.ListPlayer(t.Context(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].FishID != "inkfish" || list[0].Count != 1 {
		t.Fatalf("dex = %+v", list)
	}
}

func TestTrackService_Catch_DedupDoesNotDoubleIncrement(t *testing.T) {
	track, dex, db := newTrackService(t)
	p := &model.Player{TenantID: 0, OpenID: "openid-dedup-inc", Nick: "D"}
	if err := db.Create(p).Error; err != nil {
		t.Fatal(err)
	}
	evs := []TrackEventReq{
		{Name: "catch", T: 6000, Props: map[string]any{"kind": "fish", "fish_id": "inkfish"}},
	}
	if err := track.Track(t.Context(), p, evs); err != nil {
		t.Fatal(err)
	}
	if err := track.Track(t.Context(), p, evs); err != nil {
		t.Fatal(err)
	}
	list, err := dex.ListPlayer(t.Context(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Count != 1 {
		t.Fatalf("重发不应膨胀: %+v", list)
	}
}
