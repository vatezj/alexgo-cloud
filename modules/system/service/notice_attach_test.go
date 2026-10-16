package service

import (
	"context"
	"testing"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/pkg/tenant"
)

// 附件字段往返：Create 原样存、List 原样取（spec §2：后端不解析 JSON 内容）。
func TestNoticeService_AttachmentsRoundTrip(t *testing.T) {
	svc, _ := setupNoticeService(t)
	ctx := tenant.WithTenantID(context.Background(), 1)
	att := `[{"id":7,"name":"照片.jpg","size":4,"url":"/api/public/infra/file/aaaa"}]`

	n, err := svc.Create(ctx, &model.SystemNotice{Title: "带附件", Content: "c", Attachments: att})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	list, err := svc.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("List: n=%d err=%v", len(list), err)
	}
	if list[0].Attachments != att {
		t.Errorf("Attachments = %q, want round-trip exact", list[0].Attachments)
	}
	// Update 覆盖也原样存
	n2 := *n
	n2.Attachments = `[{"id":8,"name":"b.pdf","size":9,"url":"/api/public/infra/file/bbbb"}]`
	if err := svc.Update(ctx, &n2); err != nil {
		t.Fatalf("Update: %v", err)
	}
	list, _ = svc.List(ctx)
	if list[0].Attachments != n2.Attachments {
		t.Errorf("Attachments after update = %q, want %q", list[0].Attachments, n2.Attachments)
	}
}
