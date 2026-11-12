package service

import (
	"context"
	"testing"

	"alexGo-cloud/modules/starfuyu/repository"
	errs "alexGo-cloud/pkg/errors"
)

// TestProfile_SetMyRejectsUnknownConstellation brief 用例："十三座" → 10001 且不落库；
// "白羊座" upsert 后 GetMy 命中；未设星座 GetMy → (nil, nil)（spec §4.1）。
func TestProfile_SetMyRejectsUnknownConstellation(t *testing.T) {
	db := newTask5DB(t)
	svc := NewProfileService(repository.NewProfileRepository(db))
	ctx := context.Background()

	if _, err := svc.SetMy(ctx, 7, "十三座"); !errs.Is(err, errs.CodeParamInvalid) {
		t.Fatalf(`"十三座" 应 10001, got %v`, err)
	}
	if p, err := svc.GetMy(ctx, 7); err != nil || p != nil {
		t.Fatalf("拒绝非法星座后 GetMy 应 (nil, nil), got (%v, %v)", p, err)
	}

	if _, err := svc.SetMy(ctx, 7, "白羊座"); err != nil {
		t.Fatalf(`"白羊座" SetMy err = %v, want nil`, err)
	}
	hit, err := svc.GetMy(ctx, 7)
	if err != nil || hit == nil {
		t.Fatalf("SetMy 后 GetMy err = %v, row = %v", err, hit)
	}
	if hit.Constellation != "白羊座" || hit.MemberUserID != 7 {
		t.Fatalf("GetMy 回读不对: %+v", hit)
	}

	// 覆盖更新同 member 只有一行。
	if _, err := svc.SetMy(ctx, 7, "天蝎座"); err != nil {
		t.Fatalf("二次 SetMy err = %v, want nil", err)
	}
	var n int64
	if err := db.Table("starfuyu_profile").Where("member_user_id = ?", 7).Count(&n).Error; err != nil {
		t.Fatalf("count profile: %v", err)
	}
	if n != 1 {
		t.Fatalf("重复 SetMy 后行数 = %d, want 1", n)
	}
}
