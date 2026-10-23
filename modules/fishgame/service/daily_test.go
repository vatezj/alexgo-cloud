package service

import (
	"testing"
	"time"

	"gorm.io/gorm"

	"alexGo-cloud/modules/fishgame/model"
	"alexGo-cloud/modules/fishgame/repository"
)

func newDailyService(t *testing.T) (*DailyService, *gorm.DB) {
	t.Helper()
	db := newTestDB(t)
	// 种两条启用鱼（鱼王池用）+ 一条停用
	for _, f := range []*model.Fish{
		{TenantID: 0, FishID: "rare_a", Name: "Rare A", Rarity: "rare", Status: 1, Sort: 1},
		{TenantID: 0, FishID: "epic_b", Name: "Epic B", Rarity: "epic", Status: 1, Sort: 2},
		{TenantID: 0, FishID: "normal_c", Name: "Normal C", Rarity: "normal", Status: 1, Sort: 3},
		{TenantID: 0, FishID: "off_d", Name: "Off D", Rarity: "rare", Status: 0, Sort: 4},
	} {
		if err := repository.NewFishRepository(db).Create(t.Context(), f); err != nil {
			t.Fatal(err)
		}
	}
	return NewDailyService(
		repository.NewConfigRepository(db),
		repository.NewFishRepository(db),
		repository.NewSignRepository(db),
	), db
}

func TestDailyService_DailyConfig_NoOverride_SelfCalc(t *testing.T) {
	svc, _ := newDailyService(t)
	cfg, err := svc.DailyConfig(t.Context(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	switch cfg.Luck.Period {
	case "day", "dusk", "night", "any":
	default:
		t.Fatalf("period 非法: %q", cfg.Luck.Period)
	}
	if cfg.Luck.DepthBias < 0 || cfg.Luck.DepthBias > 0.2 {
		t.Fatalf("depth_bias 越界: %v", cfg.Luck.DepthBias)
	}
	if cfg.FishKing.FishID == "" || cfg.FishKing.Week == "" {
		t.Fatalf("fish_king 应完整: %+v", cfg.FishKing)
	}
	// 自算鱼王必须在启用池内
	if cfg.FishKing.FishID != "rare_a" && cfg.FishKing.FishID != "epic_b" {
		t.Fatalf("自算鱼王应 rare/epic 池内: %q", cfg.FishKing.FishID)
	}
}

func TestDailyService_DailyConfig_OverrideApplied(t *testing.T) {
	svc, db := newDailyService(t)
	// luck 覆盖
	if err := db.Create(&model.Config{
		TenantID: 0, Type: "luck", Period: "night", DepthBias: 0.15, Status: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	// fish_king 覆盖（合法 id）
	if err := db.Create(&model.Config{
		TenantID: 0, Type: "fish_king", FishID: "epic_b", Status: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	cfg, err := svc.DailyConfig(t.Context(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Luck.Period != "night" || cfg.Luck.DepthBias != 0.15 {
		t.Fatalf("应吃覆盖 luck: %+v", cfg.Luck)
	}
	if cfg.FishKing.FishID != "epic_b" {
		t.Fatalf("应吃覆盖 fish_king: %q", cfg.FishKing.FishID)
	}
}

func TestDailyService_DailyConfig_InvalidFishKingOverride_Fallback(t *testing.T) {
	svc, db := newDailyService(t)
	if err := db.Create(&model.Config{
		TenantID: 0, Type: "fish_king", FishID: "not_exist", Status: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	cfg, err := svc.DailyConfig(t.Context(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	// 二次过滤：非法 id 回退自算
	if cfg.FishKing.FishID == "not_exist" {
		t.Fatal("非法覆盖 id 不应下发")
	}
	if cfg.FishKing.FishID != "rare_a" && cfg.FishKing.FishID != "epic_b" {
		t.Fatalf("应回退自算池内: %q", cfg.FishKing.FishID)
	}
}

func TestDailyService_Sign(t *testing.T) {
	svc, db := newDailyService(t)
	p := &model.Player{TenantID: 0, OpenID: "openid-sign", Nick: "签"}
	if err := db.Create(p).Error; err != nil {
		t.Fatal(err)
	}

	// 首日 streak=1 → bait=1
	ok, bait, err := svc.Sign(t.Context(), p.ID, "2026-10-20")
	if err != nil || !ok || bait != 1 {
		t.Fatalf("首日: ok=%v bait=%d err=%v", ok, bait, err)
	}

	// 同日重复
	ok, _, err = svc.Sign(t.Context(), p.ID, "2026-10-20")
	if err != nil || ok {
		t.Fatalf("同日应 ok=false: ok=%v err=%v", ok, err)
	}

	// 隔 1 天 → streak=2 → bait=2
	ok, bait, err = svc.Sign(t.Context(), p.ID, "2026-10-21")
	if err != nil || !ok || bait != 2 {
		t.Fatalf("隔1天: ok=%v bait=%d err=%v", ok, bait, err)
	}

	// 隔 ≥2 天 → 归 1 → bait=1
	ok, bait, err = svc.Sign(t.Context(), p.ID, "2026-10-25")
	if err != nil || !ok || bait != 1 {
		t.Fatalf("隔≥2天: ok=%v bait=%d err=%v", ok, bait, err)
	}

	// streak 3 → bait=3；4-6 → 3；7+ → 5
	// 10-26 streak=2 bait=2
	if ok, bait, err = svc.Sign(t.Context(), p.ID, "2026-10-26"); err != nil || !ok || bait != 2 {
		t.Fatalf("streak2: bait=%d err=%v", bait, err)
	}
	// 10-27 streak=3 bait=3
	if ok, bait, err = svc.Sign(t.Context(), p.ID, "2026-10-27"); err != nil || !ok || bait != 3 {
		t.Fatalf("streak3: bait=%d err=%v", bait, err)
	}
	// 10-28 streak=4 bait=3
	if ok, bait, err = svc.Sign(t.Context(), p.ID, "2026-10-28"); err != nil || !ok || bait != 3 {
		t.Fatalf("streak4: bait=%d err=%v", bait, err)
	}
	// 10-30 streak=6 bait=3（跳过 10-29 不算——不对，10-29 streak=5 bait=3）
	if ok, bait, err = svc.Sign(t.Context(), p.ID, "2026-10-29"); err != nil || !ok || bait != 3 {
		t.Fatalf("streak5: bait=%d err=%v", bait, err)
	}
	// 10-30 streak=6 bait=3
	if ok, bait, err = svc.Sign(t.Context(), p.ID, "2026-10-30"); err != nil || !ok || bait != 3 {
		t.Fatalf("streak6: bait=%d err=%v", bait, err)
	}
	// 10-31 streak=7 bait=5
	if ok, bait, err = svc.Sign(t.Context(), p.ID, "2026-10-31"); err != nil || !ok || bait != 5 {
		t.Fatalf("streak7: bait=%d err=%v", bait, err)
	}
}

func TestDailyService_Basket(t *testing.T) {
	svc, _ := newDailyService(t)
	now := time.Now().Unix()

	// since=now → 0
	bait, capped, err := svc.Basket(now)
	if err != nil || bait != 0 || capped {
		t.Fatalf("now: bait=%d capped=%v err=%v", bait, capped, err)
	}
	// since=now-2400*3 → 3
	bait, capped, err = svc.Basket(now - 2400*3)
	if err != nil || bait != 3 || capped {
		t.Fatalf("3 件: bait=%d capped=%v err=%v", bait, capped, err)
	}
	// since=now-2400*20 → 12 capped（8h=28800 > 2400*12）
	bait, capped, err = svc.Basket(now - 2400*20)
	if err != nil || bait != 12 || !capped {
		t.Fatalf("超时: bait=%d capped=%v err=%v", bait, capped, err)
	}
	// 负 since 钳 0
	bait, capped, err = svc.Basket(now + 9999)
	if err != nil || bait != 0 || capped {
		t.Fatalf("负 elapsed: bait=%d capped=%v", bait, capped)
	}
	// 恰满 8h 不算 capped（elapsed == 28800 → capped false？brief: elapsed >= 8h capped=true）
	// brief: capped = elapsed >= 28800；客户端注释"恰满 8h 不算"。
	// Ruling: 用 brief >= 口径（服务端契约文档优先）。
	bait, capped, err = svc.Basket(now - 28800)
	if err != nil || bait != 12 || !capped {
		t.Fatalf("恰 8h: bait=%d capped=%v", bait, capped)
	}
}
