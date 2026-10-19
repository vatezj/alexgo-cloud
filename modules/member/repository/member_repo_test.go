package repository

import (
	"context"
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
