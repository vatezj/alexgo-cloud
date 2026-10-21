package social

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/tenant/gormplugin"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := gormplugin.Register(db, gormplugin.Options{}); err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&SocialApp{}, &MemberIdentity{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func ctxTid(tid uint64) context.Context { return tenant.WithTenantID(context.Background(), tid) }

// 注：方法签名以 brief 的 Interfaces 块为准（tenantID 是显式参数）；下列骨架调用
// 补传与 ctx 同值的 tenantID 实参，seed/断言保持骨架原文语义。
func TestRepo_TenantIsolation(t *testing.T) {
	db := newTestDB(t)
	repo := NewSocialRepository(db)
	// seed：tid 0/1/2 各一份 wechat_mp 配置
	for _, tid := range []uint64{0, 1, 2} {
		if err := repo.CreateApp(ctxTid(1), &SocialApp{TenantID: tid, Platform: PlatformWechatMP, AppID: "app-" + string(rune('0'+tid)), Secret: "s", Status: 1}); err != nil {
			t.Fatal(err)
		}
	}
	// tid=0 只回 tid=0 行（Review Focus #1：插件对 tid=0 沉默不注入）
	app, err := repo.GetApp(ctxTid(0), 0, PlatformWechatMP)
	if err != nil {
		t.Fatal(err)
	}
	if app.TenantID != 0 {
		t.Errorf("tid=0 读到跨租户行: tenant=%d", app.TenantID)
	}
	// tid=1 只回 tid=1 行
	app, err = repo.GetApp(ctxTid(1), 1, PlatformWechatMP)
	if err != nil {
		t.Fatal(err)
	}
	if app.TenantID != 1 {
		t.Errorf("tid=1 读到跨租户行: tenant=%d", app.TenantID)
	}
	// GetAppByID 跨租户不可见 → ErrRecordNotFound
	if _, err := repo.GetAppByID(ctxTid(1), 1, 3); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("跨租户 GetAppByID err = %v, want ErrRecordNotFound", err)
	}
}

func TestRepo_IdentityLookupAndReassign(t *testing.T) {
	db := newTestDB(t)
	repo := NewSocialRepository(db)
	// tid=1: member 100 有 wechat_mp openid=A；member 200 有 wechat_web openid=B
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(repo.CreateIdentity(ctxTid(1), &MemberIdentity{TenantID: 1, MemberUserID: 100, Platform: PlatformWechatMP, OpenID: "A", UnionID: "U1"}))
	must(repo.CreateIdentity(ctxTid(1), &MemberIdentity{TenantID: 1, MemberUserID: 200, Platform: PlatformWechatWeb, OpenID: "B", UnionID: ""}))
	must(repo.CreateIdentity(ctxTid(2), &MemberIdentity{TenantID: 2, MemberUserID: 100, Platform: PlatformWechatMP, OpenID: "A", UnionID: "U1"}))

	idn, err := repo.GetIdentity(ctxTid(1), 1, PlatformWechatMP, "A")
	if err != nil || idn.MemberUserID != 100 {
		t.Fatalf("GetIdentity = %+v, %v", idn, err)
	}
	// unionid 空串不参与匹配（Review Focus #6）
	if _, err := repo.GetIdentityByUnion(ctxTid(1), 1, ""); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("unionid='' 查询 err = %v, want ErrRecordNotFound", err)
	}
	// tid=1 有 U1（member 100），应命中且 tenant=1
	u1, err := repo.GetIdentityByUnion(ctxTid(1), 1, "U1")
	if err != nil || u1.TenantID != 1 || u1.MemberUserID != 100 {
		t.Fatalf("GetIdentityByUnion = %+v, %v", u1, err)
	}
	// tid=3 无该 unionid → 不串租户
	if _, err := repo.GetIdentityByUnion(ctxTid(3), 3, "U1"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("tid=3 GetIdentityByUnion err = %v, want ErrRecordNotFound", err)
	}
	// ReassignIdentities 显式 tenant 过滤：tid=1 改指 100→300，不影响 tid=2 同 member 行
	n, err := repo.ReassignIdentities(ctxTid(1), 1, 100, 300)
	if err != nil || n != 1 {
		t.Fatalf("Reassign = %d, %v", n, err)
	}
	idn2, _ := repo.GetIdentity(ctxTid(2), 2, PlatformWechatMP, "A")
	if idn2.MemberUserID != 100 {
		t.Errorf("跨租户 identity 被改指: %d", idn2.MemberUserID)
	}
	// DeleteIdentity 只删 (tid, member, platform)
	if n, err := repo.DeleteIdentity(ctxTid(1), 1, 200, PlatformWechatWeb); err != nil || n != 1 {
		t.Fatalf("DeleteIdentity = %d, %v", n, err)
	}
}

// TestRepo_AppCRUD 覆盖 CreateApp/GetApp/GetAppByID/UpdateApp/DeleteApp 的
// happy path + 未找到 + 跨租户写不可见；tid=0 生命周期单独钉住（插件对 tid=0
// 沉默不注入，写路径全靠显式 tenant_id 条件）。
// 每个 subtest 独立建库：(tenant_id, platform) 唯一，跨 subtest 共享库会撞唯一键。
func TestRepo_AppCRUD(t *testing.T) {

	t.Run("create_get", func(t *testing.T) {
		repo := NewSocialRepository(newTestDB(t))
		if err := repo.CreateApp(ctxTid(1), &SocialApp{TenantID: 1, Platform: PlatformWechatMP, AppID: "mp-1", Secret: "s1", Status: 1}); err != nil {
			t.Fatal(err)
		}
		app, err := repo.GetApp(ctxTid(1), 1, PlatformWechatMP)
		if err != nil || app.AppID != "mp-1" || app.TenantID != 1 {
			t.Fatalf("GetApp = %+v, %v", app, err)
		}
		// create_time/update_time 零值补 now（模型字段非 CreatedAt/UpdatedAt，gorm 不自动跟踪）
		if app.CreateTime.IsZero() || app.UpdateTime.IsZero() {
			t.Errorf("CreateApp 未填时间: create=%v update=%v", app.CreateTime, app.UpdateTime)
		}
		byID, err := repo.GetAppByID(ctxTid(1), 1, app.ID)
		if err != nil || byID.AppID != "mp-1" {
			t.Fatalf("GetAppByID = %+v, %v", byID, err)
		}
	})

	t.Run("get_not_found", func(t *testing.T) {
		repo := NewSocialRepository(newTestDB(t))
		if _, err := repo.GetApp(ctxTid(1), 1, "douyin"); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Errorf("GetApp 未知 platform err = %v, want ErrRecordNotFound", err)
		}
		if _, err := repo.GetAppByID(ctxTid(1), 1, 99999); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Errorf("GetAppByID 不存在 err = %v, want ErrRecordNotFound", err)
		}
	})

	t.Run("update", func(t *testing.T) {
		repo := NewSocialRepository(newTestDB(t))
		if err := repo.CreateApp(ctxTid(1), &SocialApp{TenantID: 1, Platform: PlatformWechatWeb, AppID: "web-1", Secret: "s1", Status: 1}); err != nil {
			t.Fatal(err)
		}
		cur, err := repo.GetApp(ctxTid(1), 1, PlatformWechatWeb)
		if err != nil {
			t.Fatal(err)
		}
		upd := *cur
		upd.AppID = "web-1-renamed"
		upd.Secret = "s2"
		upd.Status = 0
		if err := repo.UpdateApp(ctxTid(1), &upd); err != nil {
			t.Fatalf("UpdateApp: %v", err)
		}
		got, err := repo.GetApp(ctxTid(1), 1, PlatformWechatWeb)
		if err != nil {
			t.Fatal(err)
		}
		if got.AppID != "web-1-renamed" || got.Secret != "s2" || got.Status != 0 {
			t.Errorf("UpdateApp 后 = %+v, want AppID=web-1-renamed Secret=s2 Status=0", got)
		}
	})

	t.Run("update_cross_tenant", func(t *testing.T) {
		repo := NewSocialRepository(newTestDB(t))
		if err := repo.CreateApp(ctxTid(1), &SocialApp{TenantID: 1, Platform: PlatformWechatMP, AppID: "mp-a", Secret: "s", Status: 1}); err != nil {
			t.Fatal(err)
		}
		cur, err := repo.GetApp(ctxTid(1), 1, PlatformWechatMP)
		if err != nil {
			t.Fatal(err)
		}
		// ctx=tenant2 + 显式 tenant_id=1 → 条件矛盾，0 行 → ErrRecordNotFound，且原行不被改
		upd := *cur
		upd.AppID = "hacked"
		if err := repo.UpdateApp(ctxTid(2), &upd); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("跨租户 UpdateApp err = %v, want ErrRecordNotFound", err)
		}
		got, err := repo.GetApp(ctxTid(1), 1, PlatformWechatMP)
		if err != nil || got.AppID != "mp-a" {
			t.Errorf("跨租户 UpdateApp 泄漏: got=%+v, %v, want AppID=mp-a", got, err)
		}
	})

	t.Run("update_not_found", func(t *testing.T) {
		repo := NewSocialRepository(newTestDB(t))
		if err := repo.UpdateApp(ctxTid(1), &SocialApp{TenantID: 1, ID: 99999, Platform: PlatformWechatMP, AppID: "x", Secret: "x", Status: 1}); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Errorf("不存在 UpdateApp err = %v, want ErrRecordNotFound", err)
		}
	})

	t.Run("delete", func(t *testing.T) {
		repo := NewSocialRepository(newTestDB(t))
		if err := repo.CreateApp(ctxTid(1), &SocialApp{TenantID: 1, Platform: PlatformWechatWeb, AppID: "web-del", Secret: "s", Status: 1}); err != nil {
			t.Fatal(err)
		}
		cur, err := repo.GetApp(ctxTid(1), 1, PlatformWechatWeb)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.DeleteApp(ctxTid(1), 1, cur.ID); err != nil {
			t.Fatalf("DeleteApp: %v", err)
		}
		if _, err := repo.GetApp(ctxTid(1), 1, PlatformWechatWeb); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Errorf("删除后 GetApp err = %v, want ErrRecordNotFound", err)
		}
	})

	t.Run("delete_cross_tenant", func(t *testing.T) {
		repo := NewSocialRepository(newTestDB(t))
		if err := repo.CreateApp(ctxTid(1), &SocialApp{TenantID: 1, Platform: PlatformWechatWeb, AppID: "web-keep", Secret: "s", Status: 1}); err != nil {
			t.Fatal(err)
		}
		cur, err := repo.GetApp(ctxTid(1), 1, PlatformWechatWeb)
		if err != nil {
			t.Fatal(err)
		}
		// ctx=tenant2 + 显式 tenant_id=1 → 不可见，删不掉
		if err := repo.DeleteApp(ctxTid(2), 1, cur.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("跨租户 DeleteApp err = %v, want ErrRecordNotFound", err)
		}
		if _, err := repo.GetApp(ctxTid(1), 1, PlatformWechatWeb); err != nil {
			t.Errorf("跨租户 DeleteApp 误删本租户行: %v", err)
		}
	})

	t.Run("delete_not_found", func(t *testing.T) {
		repo := NewSocialRepository(newTestDB(t))
		if err := repo.DeleteApp(ctxTid(1), 1, 99999); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Errorf("不存在 DeleteApp err = %v, want ErrRecordNotFound", err)
		}
	})

	t.Run("tid0_lifecycle", func(t *testing.T) {
		repo := NewSocialRepository(newTestDB(t))
		if err := repo.CreateApp(ctxTid(0), &SocialApp{TenantID: 0, Platform: PlatformWechatMP, AppID: "mp-0", Secret: "s", Status: 1}); err != nil {
			t.Fatal(err)
		}
		app, err := repo.GetApp(ctxTid(0), 0, PlatformWechatMP)
		if err != nil || app.TenantID != 0 || app.AppID != "mp-0" {
			t.Fatalf("tid=0 GetApp = %+v, %v", app, err)
		}
		upd := *app
		upd.Status = 0
		if err := repo.UpdateApp(ctxTid(0), &upd); err != nil {
			t.Fatalf("tid=0 UpdateApp: %v", err)
		}
		got, err := repo.GetApp(ctxTid(0), 0, PlatformWechatMP)
		if err != nil || got.Status != 0 {
			t.Fatalf("tid=0 更新后 = %+v, %v, want Status=0", got, err)
		}
		if err := repo.DeleteApp(ctxTid(0), 0, app.ID); err != nil {
			t.Fatalf("tid=0 DeleteApp: %v", err)
		}
		if _, err := repo.GetApp(ctxTid(0), 0, PlatformWechatMP); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Errorf("tid=0 删除后 GetApp err = %v, want ErrRecordNotFound", err)
		}
	})
}

// TestRepo_ListApps 分页 + platform 等值筛选 + platform=="" 不筛 + 租户隔离。
// (tenant_id, platform) 唯一 → 每租户多行必须靠不同 platform（model 注明
// platform 可扩展 douyin/alipay），seed：tid1 = mp/web/douyin 三份，tid2 = mp 一份。
func TestRepo_ListApps(t *testing.T) {
	db := newTestDB(t)
	repo := NewSocialRepository(db)
	for _, a := range []*SocialApp{
		{TenantID: 1, Platform: PlatformWechatMP, AppID: "m1", Secret: "s", Status: 1},
		{TenantID: 1, Platform: PlatformWechatWeb, AppID: "w1", Secret: "s", Status: 1},
		{TenantID: 1, Platform: "douyin", AppID: "d1", Secret: "s", Status: 1},
		{TenantID: 2, Platform: PlatformWechatMP, AppID: "x1", Secret: "s", Status: 1},
	} {
		if err := repo.CreateApp(ctxTid(1), a); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("page1", func(t *testing.T) {
		list, total, err := repo.ListApps(ctxTid(1), 1, 1, 2, "")
		if err != nil {
			t.Fatal(err)
		}
		if total != 3 || len(list) != 2 {
			t.Fatalf("ListApps page1 = %d 行 / total %d, want 2 / 3", len(list), total)
		}
		for _, a := range list {
			if a.TenantID != 1 {
				t.Errorf("越权行: %+v", a)
			}
		}
	})

	t.Run("page2", func(t *testing.T) {
		list, total, err := repo.ListApps(ctxTid(1), 1, 2, 2, "")
		if err != nil {
			t.Fatal(err)
		}
		if total != 3 || len(list) != 1 {
			t.Fatalf("ListApps page2 = %d 行 / total %d, want 1 / 3", len(list), total)
		}
		// 两页合起来是 3 条互不重复的行（分页不重不漏）
		p1, _, err := repo.ListApps(ctxTid(1), 1, 1, 2, "")
		if err != nil {
			t.Fatal(err)
		}
		seen := map[uint64]bool{}
		for _, a := range append(p1, list...) {
			if seen[a.ID] {
				t.Errorf("分页重复行: id=%d", a.ID)
			}
			seen[a.ID] = true
		}
		if len(seen) != 3 {
			t.Errorf("两页去重后 = %d 行, want 3", len(seen))
		}
	})

	t.Run("filter_platform", func(t *testing.T) {
		list, total, err := repo.ListApps(ctxTid(1), 1, 1, 10, PlatformWechatMP)
		if err != nil {
			t.Fatal(err)
		}
		// 不筛时 total=3，筛 wechat_mp 只剩 1 行 → 等值筛选生效
		if total != 1 || len(list) != 1 {
			t.Fatalf("ListApps platform=wechat_mp = %d 行 / total %d, want 1 / 1", len(list), total)
		}
		if list[0].Platform != PlatformWechatMP || list[0].AppID != "m1" || list[0].TenantID != 1 {
			t.Errorf("筛错行: %+v", list[0])
		}
	})

	t.Run("tenant_isolation_tid2", func(t *testing.T) {
		list, total, err := repo.ListApps(ctxTid(2), 2, 1, 10, "")
		if err != nil {
			t.Fatal(err)
		}
		if total != 1 || len(list) != 1 {
			t.Fatalf("tid=2 ListApps = %d 行 / total %d, want 1 / 1", len(list), total)
		}
		if list[0].AppID != "x1" || list[0].TenantID != 2 {
			t.Errorf("tid=2 ListApps 串租户: %+v", list[0])
		}
	})

	t.Run("tid0_isolation", func(t *testing.T) {
		list, total, err := repo.ListApps(ctxTid(0), 0, 1, 10, "")
		if err != nil {
			t.Fatal(err)
		}
		if total != 0 || len(list) != 0 {
			t.Fatalf("tid=0 ListApps = %d 行 / total %d, want 0 / 0（本库无 tid=0 配置）", len(list), total)
		}
	})
}

// TestRepo_ListIdentities 逐成员列绑定行：tenant 恒筛 + tid=0 双向隔离
// （tid=0 只见平台行，tid≠0 见不到 tid=0 行——插件对 tid=0 沉默不注入）。
func TestRepo_ListIdentities(t *testing.T) {
	db := newTestDB(t)
	repo := NewSocialRepository(db)
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(repo.CreateIdentity(ctxTid(1), &MemberIdentity{TenantID: 1, MemberUserID: 100, Platform: PlatformWechatMP, OpenID: "A", UnionID: "U1"}))
	must(repo.CreateIdentity(ctxTid(1), &MemberIdentity{TenantID: 1, MemberUserID: 100, Platform: PlatformWechatWeb, OpenID: "C", UnionID: ""}))
	must(repo.CreateIdentity(ctxTid(1), &MemberIdentity{TenantID: 1, MemberUserID: 200, Platform: PlatformWechatMP, OpenID: "B", UnionID: ""}))
	must(repo.CreateIdentity(ctxTid(2), &MemberIdentity{TenantID: 2, MemberUserID: 100, Platform: PlatformWechatMP, OpenID: "A", UnionID: "U1"}))
	must(repo.CreateIdentity(ctxTid(0), &MemberIdentity{TenantID: 0, MemberUserID: 1, Platform: PlatformWechatMP, OpenID: "Z", UnionID: ""}))

	t.Run("member_two_rows", func(t *testing.T) {
		list, err := repo.ListIdentities(ctxTid(1), 1, 100)
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 2 {
			t.Fatalf("ListIdentities(1,100) = %d 行, want 2", len(list))
		}
		for _, idn := range list {
			if idn.TenantID != 1 || idn.MemberUserID != 100 {
				t.Errorf("越权行: %+v", idn)
			}
		}
	})

	t.Run("tenant2_only", func(t *testing.T) {
		list, err := repo.ListIdentities(ctxTid(2), 2, 100)
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 1 || list[0].TenantID != 2 {
			t.Fatalf("ListIdentities(2,100) = %+v, want 1 行 tenant=2", list)
		}
	})

	t.Run("empty_member", func(t *testing.T) {
		list, err := repo.ListIdentities(ctxTid(1), 1, 999)
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 0 {
			t.Fatalf("ListIdentities(1,999) = %d 行, want 0", len(list))
		}
	})

	t.Run("tid0_isolation", func(t *testing.T) {
		// tid=0 正读：平台行可见
		idn, err := repo.GetIdentity(ctxTid(0), 0, PlatformWechatMP, "Z")
		if err != nil || idn.TenantID != 0 {
			t.Fatalf("tid=0 GetIdentity = %+v, %v", idn, err)
		}
		// tid=1 读不到 tid=0 行（同 platform/openid 撞名也必须落空）
		if _, err := repo.GetIdentity(ctxTid(1), 1, PlatformWechatMP, "Z"); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Errorf("tid=1 读到 tid=0 行 err = %v, want ErrRecordNotFound", err)
		}
		// tid=0 列表只见 tid=0 行
		list, err := repo.ListIdentities(ctxTid(0), 0, 1)
		if err != nil || len(list) != 1 || list[0].TenantID != 0 {
			t.Fatalf("tid=0 ListIdentities = %+v, %v, want 1 行 tenant=0", list, err)
		}
	})
}
