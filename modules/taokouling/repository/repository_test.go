package repository

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/taokouling/model"
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/tenant/gormplugin"
)

// newTestDB sqlite 内存库 + 两模型 AutoMigrate + gormplugin（照 starfuyu repository_test 模式）。
// sqlite 不跑迁移 SQL——各测试自种默认行，不依赖种子数据。
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Channel{}, &model.ConversionLog{}); err != nil {
		t.Fatal(err)
	}
	if err := gormplugin.Register(db, gormplugin.Options{ExemptTables: []string{"tenants", "casbin_rule"}}); err != nil {
		t.Fatal(err)
	}
	return db
}

// baseCtx 无租户注入（tid=0）：repo 自钉 0 后的缺省测试路径。
func baseCtx() context.Context { return context.Background() }

// seedDefaultChannel 测试自种默认行（partner=” host=” status=1，走 repo.Create → tenant_id 恒 0）。
func seedDefaultChannel(t *testing.T, repo *ChannelRepository) *model.Channel {
	t.Helper()
	ch := &model.Channel{Name: "默认渠道", Partner: "", Host: "", Status: 1}
	if err := repo.Create(baseCtx(), ch); err != nil {
		t.Fatalf("种默认行 err = %v", err)
	}
	if ch.ID == 0 {
		t.Fatalf("种默认行未回填 ID: %+v", ch)
	}
	return ch
}

// TestChannelRepo_SelfPinsZero R1b：(a) baseCtx GetDefault 命中默认行；
// (b) ctx 带 tid=999 GetDefault 仍命中同一行；(c) ctx 999 下 Append 落库 tenant_id=0。
func TestChannelRepo_SelfPinsZero(t *testing.T) {
	db := newTestDB(t)
	chRepo := NewChannelRepository(db)
	logRepo := NewConversionLogRepository(db)
	seedDefaultChannel(t, chRepo)

	// (a) GetDefault(context.Background()) 返回默认行。
	got, err := chRepo.GetDefault(context.Background())
	if err != nil || got == nil {
		t.Fatalf("(a) GetDefault(baseCtx) err=%v row=%+v, want 命中默认行", err, got)
	}
	if got.TenantID != 0 || got.Partner != "" || got.Host != "" || got.Status != 1 {
		t.Fatalf("(a) 默认行形状不对: %+v", got)
	}

	// (b) GetDefault(tid=999) 也返回同一默认行（repo 首行自钉 0，双条件不 fail-closed）。
	got999, err := chRepo.GetDefault(tenant.WithTenantID(context.Background(), 999))
	if err != nil || got999 == nil {
		t.Fatalf("(b) GetDefault(tid=999) err=%v row=%+v, want 命中同一默认行", err, got999)
	}
	if got999.ID != got.ID || got999.TenantID != 0 {
		t.Fatalf("(b) 应返回同一 tid=0 默认行: got=%+v want.ID=%d", got999, got.ID)
	}

	// (c) Append 在 ctx 999 下 → 行持久化 tenant_id==0。
	rec := &model.ConversionLog{Endpoint: "parse", Partner: "", InputText: "https://t.tb.cn/x",
		ExtractedTkl: "tkl", Status: 1, DurationMs: 5}
	if err := logRepo.Append(tenant.WithTenantID(context.Background(), 999), rec); err != nil {
		t.Fatalf("(c) Append(tid=999) err = %v", err)
	}
	if rec.ID == 0 || rec.TenantID != 0 {
		t.Fatalf("(c) Append 后 rec 形状不对（TenantID 应被钉 0）: %+v", rec)
	}
	var stored model.ConversionLog
	if err := db.First(&stored, rec.ID).Error; err != nil {
		t.Fatalf("(c) 回读落库行 err = %v", err)
	}
	if stored.TenantID != 0 {
		t.Fatalf("(c) 落库 tenant_id 应恒 0, got %d", stored.TenantID)
	}
}

// TestChannelRepo_CompositeUniqueAllowsHostChannels 复合唯一键语义：
// 默认行 + 两条不同 host 行全部成功；重复三元组 ("","") 报错且不落行。
func TestChannelRepo_CompositeUniqueAllowsHostChannels(t *testing.T) {
	db := newTestDB(t)
	repo := NewChannelRepository(db)

	seedDefaultChannel(t, repo) // ("", "")
	h1 := &model.Channel{Name: "host1", Partner: "", Host: "song.vate.ren", Status: 1}
	h2 := &model.Channel{Name: "host2", Partner: "", Host: "taoquan.luojin.ink", Status: 1}
	if err := repo.Create(baseCtx(), h1); err != nil {
		t.Fatalf("host 行1 err = %v", err)
	}
	if err := repo.Create(baseCtx(), h2); err != nil {
		t.Fatalf("host 行2 err = %v", err)
	}
	var n int64
	if err := db.Model(&model.Channel{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("默认行+两条 host 行应共 3 行, got %d", n)
	}

	// 重复 (tenant_id,partner,host) 三元组 → 复合唯一键拦截。
	dup := &model.Channel{Name: "dup", Partner: "", Host: "", Status: 1}
	if err := repo.Create(baseCtx(), dup); err == nil {
		t.Fatalf("重复三元组 (0,\"\",\"\") 应报错, got nil")
	}
	if err := db.Model(&model.Channel{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("重复插入失败后行数应仍为 3, got %d", n)
	}
}

// TestChannelRepo_EnabledFiltersAndIdOrder 停用行不可被
// GetEnabledByPartner/Host/Default 命中；同 partner/host 两启用行取 id 小者；
// 空 partner/host 直接 (nil,nil)。
func TestChannelRepo_EnabledFiltersAndIdOrder(t *testing.T) {
	db := newTestDB(t)
	repo := NewChannelRepository(db)

	// 停用默认行 → GetDefault 不命中；启用品后命中（复合唯一键仅允许一条 ("","")）。
	disabledDefault := &model.Channel{Name: "默认渠道", Status: 0}
	if err := repo.Create(baseCtx(), disabledDefault); err != nil {
		t.Fatalf("种停用默认行 err = %v", err)
	}
	if got, err := repo.GetDefault(baseCtx()); err != nil || got != nil {
		t.Fatalf("停用默认行不应被 GetDefault 命中: got=%+v err=%v", got, err)
	}
	disabledDefault.Status = 1
	if err := repo.Update(baseCtx(), disabledDefault); err != nil {
		t.Fatalf("Update 启用默认行 err = %v", err)
	}
	gotDef, err := repo.GetDefault(baseCtx())
	if err != nil || gotDef == nil || gotDef.ID != disabledDefault.ID || gotDef.Status != 1 {
		t.Fatalf("启用后 GetDefault 应命中: got=%+v err=%v", gotDef, err)
	}

	// 同 partner：停用行 id 更小但不可被命中 → 取启用行。
	pOff := &model.Channel{Name: "p-off", Partner: "abc", Host: "h-off", Status: 0}
	pOn := &model.Channel{Name: "p-on", Partner: "abc", Host: "h-on", Status: 1}
	if err := repo.Create(baseCtx(), pOff); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(baseCtx(), pOn); err != nil {
		t.Fatal(err)
	}
	gotP, err := repo.GetEnabledByPartner(baseCtx(), "abc")
	if err != nil || gotP == nil {
		t.Fatalf("GetEnabledByPartner(abc) err=%v row=%+v, want 命中", err, gotP)
	}
	if gotP.ID != pOn.ID {
		t.Fatalf("停用行(id=%d)不可命中: got id=%d want %d", pOff.ID, gotP.ID, pOn.ID)
	}

	// 同 partner 两行都启用 → id asc 取小者（两行 host 必须不同，复合唯一键约束）。
	pA := &model.Channel{Name: "p-a", Partner: "def", Host: "h1", Status: 1}
	pB := &model.Channel{Name: "p-b", Partner: "def", Host: "h2", Status: 1}
	if err := repo.Create(baseCtx(), pA); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(baseCtx(), pB); err != nil {
		t.Fatal(err)
	}
	gotDef2, err := repo.GetEnabledByPartner(baseCtx(), "def")
	if err != nil || gotDef2 == nil {
		t.Fatalf("GetEnabledByPartner(def) err=%v row=%+v, want 命中", err, gotDef2)
	}
	if gotDef2.ID != pA.ID {
		t.Fatalf("同 partner 两启用行应取 id 小者: got=%d want %d", gotDef2.ID, pA.ID)
	}

	// host 侧同理：停用行 id 更小不可命中。
	hOff := &model.Channel{Name: "h-off-row", Partner: "px", Host: "shared.h", Status: 0}
	hOn := &model.Channel{Name: "h-on-row", Partner: "py", Host: "shared.h", Status: 1}
	if err := repo.Create(baseCtx(), hOff); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(baseCtx(), hOn); err != nil {
		t.Fatal(err)
	}
	gotH, err := repo.GetEnabledByHost(baseCtx(), "shared.h")
	if err != nil || gotH == nil {
		t.Fatalf("GetEnabledByHost(shared.h) err=%v row=%+v, want 命中", err, gotH)
	}
	if gotH.ID != hOn.ID {
		t.Fatalf("停用 host 行(id=%d)不可命中: got id=%d want %d", hOff.ID, gotH.ID, hOn.ID)
	}

	// 同 host 两启用行（partner 不同，三元组不撞）→ id asc 取小者。
	hA := &model.Channel{Name: "h-a", Partner: "qa", Host: "multi.h", Status: 1}
	hB := &model.Channel{Name: "h-b", Partner: "qb", Host: "multi.h", Status: 1}
	if err := repo.Create(baseCtx(), hA); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(baseCtx(), hB); err != nil {
		t.Fatal(err)
	}
	gotH2, err := repo.GetEnabledByHost(baseCtx(), "multi.h")
	if err != nil || gotH2 == nil {
		t.Fatalf("GetEnabledByHost(multi.h) err=%v row=%+v, want 命中", err, gotH2)
	}
	if gotH2.ID != hA.ID {
		t.Fatalf("同 host 两启用行应取 id 小者: got=%d want %d", gotH2.ID, hA.ID)
	}

	// 空 partner / host → (nil, nil)。
	if got, err := repo.GetEnabledByPartner(baseCtx(), ""); err != nil || got != nil {
		t.Fatalf("GetEnabledByPartner(\"\") = %+v, %v; want nil, nil", got, err)
	}
	if got, err := repo.GetEnabledByHost(baseCtx(), ""); err != nil || got != nil {
		t.Fatalf("GetEnabledByHost(\"\") = %+v, %v; want nil, nil", got, err)
	}
}

// TestChannelRepo_ExistsPairExclude (tang,"") 已存在：
// excludeID=0 → true；excludeID=该行 id → false；其他组合 → false。
func TestChannelRepo_ExistsPairExclude(t *testing.T) {
	db := newTestDB(t)
	repo := NewChannelRepository(db)
	seedDefaultChannel(t, repo)

	row := &model.Channel{Name: "tang", Partner: "tang", Host: "", Status: 1}
	if err := repo.Create(baseCtx(), row); err != nil {
		t.Fatalf("种 (tang,\"\") 行 err = %v", err)
	}
	if row.ID == 0 {
		t.Fatalf("行未回填 ID: %+v", row)
	}

	if ok, err := repo.ExistsPair(baseCtx(), "tang", "", 0); err != nil || !ok {
		t.Fatalf("ExistsPair(tang,\"\",0) = %v, %v; want true, nil", ok, err)
	}
	if ok, err := repo.ExistsPair(baseCtx(), "tang", "", row.ID); err != nil || ok {
		t.Fatalf("ExistsPair(tang,\"\",exclude self) = %v, %v; want false, nil", ok, err)
	}
	if ok, err := repo.ExistsPair(baseCtx(), "tang", "other.host", 0); err != nil || ok {
		t.Fatalf("ExistsPair(tang,other.host,0) = %v, %v; want false, nil", ok, err)
	}
	if ok, err := repo.ExistsPair(baseCtx(), "nobody", "", 0); err != nil || ok {
		t.Fatalf("ExistsPair(nobody,\"\",0) = %v, %v; want false, nil", ok, err)
	}
}

// TestLogRepo_AppendForcesZeroAndListFilters Append 强制 tid=0（即使 ctx 999）；
// List：Status=-1 不过滤、partner/status/时间/item 筛选、id DESC、total 正确、分页独立。
func TestLogRepo_AppendForcesZeroAndListFilters(t *testing.T) {
	db := newTestDB(t)
	repo := NewConversionLogRepository(db)

	mk := func(partner string, status int8, item string, at time.Time) *model.ConversionLog {
		return &model.ConversionLog{
			Endpoint: "parse", Partner: partner, InputText: "https://t.tb.cn/x",
			ExtractedTkl: "tkl", ItemID: item, Status: status,
			CreatedAt: at, UpdatedAt: at,
		}
	}
	r1 := mk("p1", 1, "item1", time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC))
	r2 := mk("p1", 2, "item2", time.Date(2026, 11, 1, 11, 0, 0, 0, time.UTC))
	r3 := mk("p2", 1, "item1", time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC))
	r4 := mk("p1", 1, "item3", time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC))

	// ctx 999 下 Append 也强制 tenant_id=0；其余走 baseCtx。
	if err := repo.Append(tenant.WithTenantID(baseCtx(), 999), r1); err != nil {
		t.Fatalf("Append(tid=999) err = %v", err)
	}
	if r1.TenantID != 0 {
		t.Fatalf("Append 后 rec.TenantID 应被钉 0, got %d", r1.TenantID)
	}
	for i, rec := range []*model.ConversionLog{r2, r3, r4} {
		if err := repo.Append(baseCtx(), rec); err != nil {
			t.Fatalf("Append[%d] err = %v", i+2, err)
		}
	}
	var rows []model.ConversionLog
	if err := db.Order("id ASC").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 {
		t.Fatalf("落库应 4 行, got %d", len(rows))
	}
	for _, row := range rows {
		if row.TenantID != 0 {
			t.Fatalf("落库 tenant_id 应恒 0, got %d (id=%d)", row.TenantID, row.ID)
		}
	}

	// Status=-1 不过滤 + id DESC + total。
	all, total, err := repo.List(baseCtx(), LogFilter{Status: -1, Page: 1, Size: 10})
	if err != nil {
		t.Fatalf("List(all) err = %v", err)
	}
	if total != 4 || len(all) != 4 {
		t.Fatalf("List(all) total=%d len=%d, want 4/4", total, len(all))
	}
	wantOrder := []uint64{r4.ID, r3.ID, r2.ID, r1.ID}
	for i, row := range all {
		if row.ID != wantOrder[i] {
			t.Fatalf("List(all) 应按 id DESC: got[%d].ID=%d want %d (ids=%v)", i, row.ID, wantOrder[i], wantOrder)
		}
	}

	// partner 筛选。
	p1s, total, err := repo.List(baseCtx(), LogFilter{Partner: "p1", Status: -1, Page: 1, Size: 10})
	if err != nil || total != 3 || len(p1s) != 3 {
		t.Fatalf("List(p1) total=%d len=%d err=%v, want 3/3/nil", total, len(p1s), err)
	}
	if p1s[0].ID != r4.ID || p1s[2].ID != r1.ID {
		t.Fatalf("List(p1) id DESC 顺序不对: %+v", []uint64{p1s[0].ID, p1s[1].ID, p1s[2].ID})
	}

	// status 筛选。
	s1, total, err := repo.List(baseCtx(), LogFilter{Status: 1, Page: 1, Size: 10})
	if err != nil || total != 3 || len(s1) != 3 {
		t.Fatalf("List(status=1) total=%d len=%d err=%v, want 3/3/nil", total, len(s1), err)
	}
	for _, row := range s1 {
		if row.Status != 1 {
			t.Fatalf("status 筛选混入非 1 行: %+v", row)
		}
	}

	// 时间窗（Start/End 非零值才过滤）。
	win, total, err := repo.List(baseCtx(), LogFilter{
		Start:  time.Date(2026, 11, 1, 10, 30, 0, 0, time.UTC),
		End:    time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC),
		Status: -1, Page: 1, Size: 10,
	})
	if err != nil || total != 2 || len(win) != 2 {
		t.Fatalf("List(时间窗) total=%d len=%d err=%v, want 2/2/nil", total, len(win), err)
	}
	if win[0].ID != r3.ID || win[1].ID != r2.ID {
		t.Fatalf("时间窗行不对: got ids=%v want [%d %d]", []uint64{win[0].ID, win[1].ID}, r3.ID, r2.ID)
	}

	// item_id 筛选。
	items, total, err := repo.List(baseCtx(), LogFilter{ItemID: "item1", Status: -1, Page: 1, Size: 10})
	if err != nil || total != 2 || len(items) != 2 {
		t.Fatalf("List(item1) total=%d len=%d err=%v, want 2/2/nil", total, len(items), err)
	}

	// 分页 + total 独立。
	page1, total, err := repo.List(baseCtx(), LogFilter{Status: -1, Page: 1, Size: 2})
	if err != nil || total != 4 || len(page1) != 2 {
		t.Fatalf("List 第一页 total=%d len=%d err=%v, want 4/2/nil", total, len(page1), err)
	}
	if page1[0].ID != r4.ID || page1[1].ID != r3.ID {
		t.Fatalf("第一页应 [r4 r3], got [%d %d]", page1[0].ID, page1[1].ID)
	}
	page2, total2, err := repo.List(baseCtx(), LogFilter{Status: -1, Page: 2, Size: 2})
	if err != nil || total2 != 4 || len(page2) != 2 {
		t.Fatalf("List 第二页 total=%d len=%d err=%v, want 4/2/nil", total2, len(page2), err)
	}
	if page2[0].ID != r2.ID || page2[1].ID != r1.ID {
		t.Fatalf("第二页应 [r2 r1], got [%d %d]", page2[0].ID, page2[1].ID)
	}
}
