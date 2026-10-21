package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/member/model"
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/tenant/gormplugin"
)

// ListAll 租户隔离：真注册 gormplugin（生产装配，main.go 同款 Options）——裸
// sqlite/memRepo 的 Where 自测钉不住插件层（B 子项目终审 C2 教训，RF③）。
func TestListAll_TenantIsolation_UnderPlugin(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.MemberUser{}); err != nil {
		t.Fatal(err)
	}
	if err := gormplugin.Register(db, gormplugin.Options{
		ExemptTables: []string{"tenants", "casbin_rule"},
	}); err != nil {
		t.Fatal(err)
	}
	repo := NewMemberRepository(db)

	ctx1 := tenant.WithTenantID(context.Background(), 1)
	ctx2 := tenant.WithTenantID(context.Background(), 2)
	for _, u := range []*model.MemberUser{
		{Nickname: "甲", Mobile: "138", TenantID: 1},
		{Nickname: "乙", Mobile: "139", TenantID: 1},
	} {
		if err := repo.Create(ctx1, u); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}
	if err := repo.Create(ctx2, &model.MemberUser{Nickname: "丙", Mobile: "1380", TenantID: 2}); err != nil {
		t.Fatalf("Create tenant2: %v", err)
	}

	got, err := repo.ListAll(ctx1, 1)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListAll count = %d, want 2 (tenant 1 only)", len(got))
	}
	for _, u := range got {
		if u.TenantID != 1 {
			t.Errorf("cross-tenant leak: id=%d tenant=%d", u.ID, u.TenantID)
		}
	}
}

// GetByID：按 tenant+id 直取、不带 deleted=0（软删行照常返回由调用方按
// Deleted 判定）；主键全局唯一故两租户同 id 不可能——钉住的边界是跨租户 id
// 查不到 + tid=0（插件沉默不注入）靠显式 tenant_id 条件挡非平台行。
func TestGetByID_TenantIsolation_IncludeSoftDeleted(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.MemberUser{}); err != nil {
		t.Fatal(err)
	}
	if err := gormplugin.Register(db, gormplugin.Options{
		ExemptTables: []string{"tenants", "casbin_rule"},
	}); err != nil {
		t.Fatal(err)
	}
	repo := NewMemberRepository(db)

	ctx0 := tenant.WithTenantID(context.Background(), 0)
	ctx1 := tenant.WithTenantID(context.Background(), 1)
	ctx2 := tenant.WithTenantID(context.Background(), 2)

	u1 := &model.MemberUser{Nickname: "甲", Mobile: "13800000001", TenantID: 1}
	u2 := &model.MemberUser{Nickname: "乙", Mobile: "13800000002", TenantID: 2}
	p0 := &model.MemberUser{Nickname: "平台", Mobile: "13800000000", TenantID: 0}
	if err := repo.Create(ctx1, u1); err != nil {
		t.Fatalf("Create tenant1: %v", err)
	}
	if err := repo.Create(ctx2, u2); err != nil {
		t.Fatalf("Create tenant2: %v", err)
	}
	// tid=0 平台行：ctx0 → 插件不回填，tenant_id 保持 0
	if err := repo.Create(ctx0, p0); err != nil {
		t.Fatalf("Create tenant0: %v", err)
	}

	// happy path：tenant+id 命中
	got, err := repo.GetByID(ctx1, 1, u1.ID)
	if err != nil || got.ID != u1.ID || got.TenantID != 1 {
		t.Fatalf("GetByID = %+v, %v, want id=%d tenant=1", got, err, u1.ID)
	}

	// 跨租户 id 查不到 → ErrRecordNotFound
	if _, err := repo.GetByID(ctx1, 1, u2.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("跨租户 GetByID err = %v, want ErrRecordNotFound", err)
	}

	// tid=0（插件不注入）：显式 tenant_id=0 条件仍挡住租户行
	if _, err := repo.GetByID(ctx0, 0, u1.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("tid=0 读到租户行 err = %v, want ErrRecordNotFound", err)
	}
	// tid=0 正读：平台行可取回
	if u, err := repo.GetByID(ctx0, 0, p0.ID); err != nil || u.TenantID != 0 {
		t.Errorf("tid=0 GetByID = %+v, %v, want tenant=0", u, err)
	}

	// 不存在 id
	if _, err := repo.GetByID(ctx1, 1, 999999); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("不存在 GetByID err = %v, want ErrRecordNotFound", err)
	}

	// 软删行仍可取回（GetByID 不带 deleted=0），Deleted 字段交调用方判定
	if err := db.WithContext(ctx1).Model(&model.MemberUser{}).
		Where("id = ?", u1.ID).Update("deleted", true).Error; err != nil {
		t.Fatalf("标记软删: %v", err)
	}
	soft, err := repo.GetByID(ctx1, 1, u1.ID)
	if err != nil {
		t.Fatalf("软删行 GetByID: %v", err)
	}
	if !soft.Deleted {
		t.Errorf("软删行 Deleted = false, want true（GetByID 不带 deleted=0 过滤）")
	}
	if soft.ID != u1.ID || soft.TenantID != 1 {
		t.Errorf("软删行 = id=%d tenant=%d, want id=%d tenant=1", soft.ID, soft.TenantID, u1.ID)
	}
}
