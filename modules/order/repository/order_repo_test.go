package repository

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/order/model"
	systemmodel "alexGo-cloud/modules/system/model"
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/tenant/gormplugin"
)

// List 租户隔离：真注册 gormplugin（生产装配，main.go 同款 Options）——裸
// sqlite 的 Where 自测钉不住插件层（member 同款教训，RF③）。
//
// 关键背景：Order 模型无 TenantID 字段（orders 表无 tenant_id 列）→ 插件
// 明文跳过不接管（tid=0 也不注入），租户隔离必须靠 repo 层显式
// user_id IN (SELECT id FROM system_users WHERE tenant_id = ?)。
// 本测试因此同时钉住：显式过滤在插件在位时仍正确。
func TestList_TenantIsolation_UnderPlugin(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Order{}, &systemmodel.User{}); err != nil {
		t.Fatal(err)
	}
	if err := gormplugin.Register(db, gormplugin.Options{
		ExemptTables: []string{"tenants", "casbin_rule"},
	}); err != nil {
		t.Fatal(err)
	}
	repo := NewOrderRepository(db)

	ctx1 := tenant.WithTenantID(context.Background(), 1)
	ctx2 := tenant.WithTenantID(context.Background(), 2)
	// system_users 两租户各一用户（User 带 TenantID 字段，插件接管 INSERT；
	// 显式赋值不被覆盖）。订单经 user_id 挂到各自用户。
	u1 := &systemmodel.User{Username: "u1", TenantID: 1}
	u2 := &systemmodel.User{Username: "u2", TenantID: 2}
	if err := db.WithContext(ctx1).Create(u1).Error; err != nil {
		t.Fatalf("create user1: %v", err)
	}
	if err := db.WithContext(ctx2).Create(u2).Error; err != nil {
		t.Fatalf("create user2: %v", err)
	}
	for _, o := range []*model.Order{
		{UserID: u1.ID, OrderNo: "T1-A"},
		{UserID: u1.ID, OrderNo: "T1-B"},
	} {
		if err := repo.Create(ctx1, o); err != nil {
			t.Fatalf("Create tenant1: %v", err)
		}
	}
	if err := repo.Create(ctx2, &model.Order{UserID: u2.ID, OrderNo: "T2-C"}); err != nil {
		t.Fatalf("Create tenant2: %v", err)
	}

	got1, err := repo.List(ctx1)
	if err != nil {
		t.Fatalf("List(ctx1): %v", err)
	}
	if len(got1) != 2 {
		t.Fatalf("List(ctx1) count = %d, want 2 (tenant 1 only)", len(got1))
	}
	for _, o := range got1 {
		if o.UserID != u1.ID {
			t.Errorf("cross-tenant leak in ctx1: id=%d user_id=%d", o.ID, o.UserID)
		}
	}

	got2, err := repo.List(ctx2)
	if err != nil {
		t.Fatalf("List(ctx2): %v", err)
	}
	if len(got2) != 1 {
		t.Fatalf("List(ctx2) count = %d, want 1 (tenant 2 only)", len(got2))
	}
	for _, o := range got2 {
		if o.UserID != u2.ID {
			t.Errorf("cross-tenant leak in ctx2: id=%d user_id=%d", o.ID, o.UserID)
		}
	}
}
