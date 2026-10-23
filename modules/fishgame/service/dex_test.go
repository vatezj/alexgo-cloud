package service

import (
	"testing"

	"alexGo-cloud/modules/fishgame/model"
)

func TestDexService_Report_MaxMerge(t *testing.T) {
	track, dex, db := newTrackService(t)
	p := &model.Player{TenantID: 0, OpenID: "openid-report", Nick: "R"}
	if err := db.Create(p).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := track.Track(t.Context(), p, []TrackEventReq{
			{Name: "catch", T: int64(7000 + i), Props: map[string]any{"kind": "fish", "fish_id": "inkfish"}},
		}); err != nil {
			t.Fatal(err)
		}
	}
	total, err := dex.Report(t.Context(), p.ID, map[string]int{"inkfish": 5, "unknown_x": 9})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Fatalf("点亮数应 1, got %d", total)
	}
	list, err := dex.ListPlayer(t.Context(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Count != 5 {
		t.Fatalf("max 合并应 5, got %+v", list)
	}

	if _, err := dex.Report(t.Context(), p.ID, map[string]int{"inkfish": 2}); err != nil {
		t.Fatal(err)
	}
	list, err = dex.ListPlayer(t.Context(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if list[0].Count != 5 {
		t.Fatalf("上报 2 不应回退, got %d", list[0].Count)
	}
}

func TestDexService_Table_StatusFilter(t *testing.T) {
	_, dex, db := newTrackService(t)
	if err := db.Create(&model.Fish{
		TenantID: 0, FishID: "off_x", Name: "Off", Rarity: "normal", Status: 0, Sort: 9,
	}).Error; err != nil {
		t.Fatal(err)
	}
	rows, err := dex.Table(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("应只 2 条启用, got %d", len(rows))
	}
	for _, r := range rows {
		if r.ID == "off_x" {
			t.Fatal("停用条目不应出现")
		}
		if r.Name == "" || r.Rarity == "" {
			t.Fatalf("字段应齐: %+v", r)
		}
	}
}
