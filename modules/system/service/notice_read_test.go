package service

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/repository"
	"alexGo-cloud/pkg/tenant"
)

// setupNoticeService：sqlite :memory: + 公告/已读回执表，真 repo（抄 setupMenuService 模式）。
// 返回 db 供行数断言（幂等、跨租户回执隔离都要数行）。
func setupNoticeService(t *testing.T) (NoticeService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.SystemNotice{}, &model.SystemNoticeRead{}); err != nil {
		t.Fatal(err)
	}
	return NewNoticeService(repository.NewNoticeRepository(db)), db
}

// seedNotice 走 service.Create 造一行公告（自带租户/时间戳），返回 id。
func seedNotice(t *testing.T, svc NoticeService, ctx context.Context, title string) uint64 {
	t.Helper()
	n, err := svc.Create(ctx, &model.SystemNotice{Title: title, Content: "内容-" + title})
	if err != nil {
		t.Fatalf("seed notice %q: %v", title, err)
	}
	return n.ID
}

// 读状态合并：ListMine 按 id desc 返回，read_status 逐条正确（0 未读 / 1 已读）。
func TestNoticeService_ListMine_ReadStatus(t *testing.T) {
	svc, _ := setupNoticeService(t)
	ctx := tenant.WithTenantID(context.Background(), 1)
	idA := seedNotice(t, svc, ctx, "A")
	seedNotice(t, svc, ctx, "B")

	if err := svc.MarkRead(ctx, 7, idA); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	items, err := svc.ListMine(ctx, 7)
	if err != nil {
		t.Fatalf("ListMine: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	// 列表 id desc：B 后建（id 大）在前、未读；A 在后、已读
	if items[0].Title != "B" || items[0].ReadStatus != 0 {
		t.Errorf("items[0] = %s/read=%d, want B/0", items[0].Title, items[0].ReadStatus)
	}
	if items[1].Title != "A" || items[1].ReadStatus != 1 {
		t.Errorf("items[1] = %s/read=%d, want A/1", items[1].Title, items[1].ReadStatus)
	}
}

// 幂等（Review Focus 1）：重复标记不报错，库中恰一行。
func TestNoticeService_MarkRead_Idempotent(t *testing.T) {
	svc, db := setupNoticeService(t)
	ctx := tenant.WithTenantID(context.Background(), 1)
	id := seedNotice(t, svc, ctx, "A")

	for i := 0; i < 3; i++ {
		if err := svc.MarkRead(ctx, 7, id); err != nil {
			t.Fatalf("MarkRead #%d: %v", i+1, err)
		}
	}
	var rows int64
	if err := db.Model(&model.SystemNoticeRead{}).Count(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Errorf("receipt rows = %d, want 1", rows)
	}
}

// 不存在与跨租户都返回 ErrNoticeNotFound（Review Focus 2 前半）。
func TestNoticeService_MarkRead_NotFound(t *testing.T) {
	svc, _ := setupNoticeService(t)
	ctx1 := tenant.WithTenantID(context.Background(), 1)
	ctx2 := tenant.WithTenantID(context.Background(), 2)

	if err := svc.MarkRead(ctx1, 7, 999); !errors.Is(err, ErrNoticeNotFound) {
		t.Fatalf("missing id err = %v, want ErrNoticeNotFound", err)
	}
	crossID := seedNotice(t, svc, ctx2, "T2")
	if err := svc.MarkRead(ctx1, 7, crossID); !errors.Is(err, ErrNoticeNotFound) {
		t.Fatalf("cross-tenant err = %v, want ErrNoticeNotFound", err)
	}
}

// 全部已读：本租户全变已读；别租户不受影响、回执行数只等于本租户公告数（Review Focus 2 后半）。
func TestNoticeService_MarkAllRead(t *testing.T) {
	svc, db := setupNoticeService(t)
	ctx1 := tenant.WithTenantID(context.Background(), 1)
	ctx2 := tenant.WithTenantID(context.Background(), 2)
	seedNotice(t, svc, ctx1, "A")
	seedNotice(t, svc, ctx1, "B")
	seedNotice(t, svc, ctx2, "T2")

	if err := svc.MarkAllRead(ctx1, 7); err != nil {
		t.Fatalf("MarkAllRead: %v", err)
	}
	items, err := svc.ListMine(ctx1, 7)
	if err != nil {
		t.Fatalf("ListMine: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	for _, it := range items {
		if it.ReadStatus != 1 {
			t.Errorf("notice %q read=%d, want 1 after MarkAllRead", it.Title, it.ReadStatus)
		}
	}
	other, err := svc.ListMine(ctx2, 7)
	if err != nil {
		t.Fatalf("ListMine tenant2: %v", err)
	}
	if len(other) != 1 || other[0].ReadStatus != 0 {
		t.Errorf("tenant2 items = %+v, want 1 条且未读", other)
	}
	var rows int64
	if err := db.Model(&model.SystemNoticeRead{}).Count(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if rows != 2 {
		t.Errorf("receipt rows = %d, want 2（仅租户1 两条）", rows)
	}
}
