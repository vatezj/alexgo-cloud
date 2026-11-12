package service

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/starfuyu/model"
	"alexGo-cloud/modules/starfuyu/repository"
	"alexGo-cloud/pkg/config"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/tenant/gormplugin"
)

// newTask5DB sqlite 内存库 + 六模型 AutoMigrate + gormplugin（照 repository_test 惯例，
// Task 5 的 limit/bazi/profile 三组测试共用）。
func newTask5DB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&model.Profile{}, &model.Fortune{}, &model.Bazi{},
		&model.Divination{}, &model.RequestLimit{}, &model.SignCache{},
	); err != nil {
		t.Fatal(err)
	}
	if err := gormplugin.Register(db, gormplugin.Options{ExemptTables: []string{"tenants", "casbin_rule"}}); err != nil {
		t.Fatal(err)
	}
	return db
}

// TestLimit_AllowBlocksOverDaily brief 用例：cfg limit=2，Allow 第 1/2 次 nil，第 3 次 30503；
// 超限那次的计数不回滚（新 count > limit 判定，增量保留在行上）。
func TestLimit_AllowBlocksOverDaily(t *testing.T) {
	db := newTask5DB(t)
	repo := repository.NewRequestLimitRepository(db)
	cfg := &config.Config{}
	cfg.Starfuyu.Limits.BaziCalculateDaily = 2
	svc := NewLimitService(repo, cfg)
	ctx := context.Background()

	for i := 1; i <= 2; i++ {
		if err := svc.Allow(ctx, 7, TypeBaziCalculate); err != nil {
			t.Fatalf("第 %d 次 Allow 应 nil, got %v", i, err)
		}
	}
	err := svc.Allow(ctx, 7, TypeBaziCalculate)
	if !errs.Is(err, errs.CodeStarfuyuLimitExceeded) {
		t.Fatalf("第 3 次 Allow 应 30503, got %v", err)
	}

	var row model.RequestLimit
	if dbErr := db.Where("member_user_id = ? AND type = ?", 7, TypeBaziCalculate).First(&row).Error; dbErr != nil {
		t.Fatalf("读限流行 err = %v", dbErr)
	}
	if row.Count != 3 {
		t.Fatalf("超限调用不回滚计数, count = %d, want 3", row.Count)
	}
}

// TestLimit_UsesShanghaiDate brief 用例：TZ=UTC 下限流 key 日期仍 == TodaySH()（沪时区日历日）。
func TestLimit_UsesShanghaiDate(t *testing.T) {
	t.Setenv("TZ", "UTC")
	db := newTask5DB(t)
	repo := repository.NewRequestLimitRepository(db)
	svc := NewLimitService(repo, &config.Config{})
	ctx := context.Background()

	if err := svc.Allow(ctx, 9, TypeBaziCalculate); err != nil {
		t.Fatalf("首次 Allow 应 nil, got %v", err)
	}
	var row model.RequestLimit
	if dbErr := db.Where("member_user_id = ? AND type = ?", 9, TypeBaziCalculate).First(&row).Error; dbErr != nil {
		t.Fatalf("读限流行 err = %v", dbErr)
	}
	if row.Date != TodaySH() {
		t.Fatalf("限流 key 日期 = %q, want TodaySH() %q", row.Date, TodaySH())
	}
}

// TestLimit_DefaultsFiveAndTen brief 缺省：cfg <=0 时 bazi=5、divination=10。
func TestLimit_DefaultsFiveAndTen(t *testing.T) {
	db := newTask5DB(t)
	repo := repository.NewRequestLimitRepository(db)
	svc := NewLimitService(repo, &config.Config{})
	ctx := context.Background()

	for i := 1; i <= 5; i++ {
		if err := svc.Allow(ctx, 11, TypeBaziCalculate); err != nil {
			t.Fatalf("bazi 第 %d 次应 nil（缺省 5）, got %v", i, err)
		}
	}
	if err := svc.Allow(ctx, 11, TypeBaziCalculate); !errs.Is(err, errs.CodeStarfuyuLimitExceeded) {
		t.Fatalf("bazi 第 6 次应 30503, got %v", err)
	}
	for i := 1; i <= 10; i++ {
		if err := svc.Allow(ctx, 11, TypeDivinationDraw); err != nil {
			t.Fatalf("divination 第 %d 次应 nil（缺省 10）, got %v", i, err)
		}
	}
	if err := svc.Allow(ctx, 11, TypeDivinationDraw); !errs.Is(err, errs.CodeStarfuyuLimitExceeded) {
		t.Fatalf("divination 第 11 次应 30503, got %v", err)
	}
}
