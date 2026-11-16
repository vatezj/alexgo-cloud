package repository

import (
	"testing"

	"alexGo-cloud/modules/starfuyu/model"
)

// TestDailyRepo_UpsertAndGet 首插 → GetByDate 命中；同 date 改 astro/intro 再 Upsert →
// 覆盖更新且 ID 不变；未知 date → (nil, false, nil)。
func TestDailyRepo_UpsertAndGet(t *testing.T) {
	db := newTestDB(t)
	repo := NewDailyRepository(db)
	ctx := baseCtx()

	miss, ok, err := repo.GetByDate(ctx, "2026-11-16")
	if err != nil {
		t.Fatalf("GetByDate miss err = %v, want nil", err)
	}
	if ok || miss != nil {
		t.Fatalf("GetByDate miss = (%+v, %v), want (nil, false)", miss, ok)
	}

	in := &model.DailyHoroscope{
		Date:  "2026-11-16",
		Astro: "月水星，月土六合",
		Intro: "这天是积极热情有能量的日子。",
	}
	if err := repo.Upsert(ctx, in); err != nil {
		t.Fatalf("首次 Upsert err = %v", err)
	}

	got, ok, err := repo.GetByDate(ctx, "2026-11-16")
	if err != nil || !ok {
		t.Fatalf("GetByDate hit = (%+v, %v, %v), want ok", got, ok, err)
	}
	if got.Astro != "月水星，月土六合" || got.Intro != "这天是积极热情有能量的日子。" {
		t.Fatalf("命中行字段不符: %+v", got)
	}
	firstID := got.ID

	// 同 date 再 Upsert：覆盖 astro/intro、主键不变。
	if err := repo.Upsert(ctx, &model.DailyHoroscope{
		Date:  "2026-11-16",
		Astro: "木星入庙",
		Intro: "换个总述。",
	}); err != nil {
		t.Fatalf("二次 Upsert err = %v", err)
	}
	got2, ok, err := repo.GetByDate(ctx, "2026-11-16")
	if err != nil || !ok {
		t.Fatalf("二次 GetByDate = (%+v, %v, %v)", got2, ok, err)
	}
	if got2.Astro != "木星入庙" || got2.Intro != "换个总述。" {
		t.Fatalf("二次 Upsert 未覆盖: %+v", got2)
	}
	if got2.ID != firstID {
		t.Fatalf("二次 Upsert ID 变了: %d → %d（应冲突更新不换行）", firstID, got2.ID)
	}
}
