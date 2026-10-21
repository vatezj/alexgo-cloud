package social

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"

	errs "alexGo-cloud/pkg/errors"
)

// === 夹具（每用例独立内存库：SocialApp + gormplugin，见 repository_test.go）===

type appcfgEnv struct {
	tid  uint64
	db   *gorm.DB
	repo SocialRepository
	reg  *AdapterRegistry
	svc  *AppConfigService
	ctx  context.Context
}

// newAppcfgEnv 建库并预注册微信两平台适配器（Exists 真实可用；
// 未知平台用例传 "qq" 不在注册表）。
func newAppcfgEnv(t *testing.T, tid uint64) *appcfgEnv {
	t.Helper()
	db := newTestDB(t)
	reg := &AdapterRegistry{byPlatform: map[string]PlatformAdapter{
		PlatformWechatMP:  &stubAdapter{platform: PlatformWechatMP},
		PlatformWechatWeb: &stubAdapter{platform: PlatformWechatWeb},
	}}
	repo := NewSocialRepository(db)
	return &appcfgEnv{
		tid:  tid,
		db:   db,
		repo: repo,
		reg:  reg,
		svc:  NewAppConfigService(AppConfigDeps{Repo: repo, Adapters: reg}),
		ctx:  ctxTid(tid),
	}
}

// seed 直插一条渠道配置（绕过 svc 规则，只铺前置数据）。
func (e *appcfgEnv) seed(t *testing.T, platform, secret string) *SocialApp {
	t.Helper()
	app := &SocialApp{
		TenantID: e.tid, Platform: platform,
		AppID: "appid-" + platform, Secret: secret, Status: 1,
	}
	if err := e.repo.CreateApp(e.ctx, app); err != nil {
		t.Fatal(err)
	}
	return app
}

func (e *appcfgEnv) count(t *testing.T) int64 {
	t.Helper()
	var n int64
	if err := e.db.Model(&SocialApp{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

// === Create（brief Step 1 用例逐字）===

func TestAppConfig_Create_UnknownPlatform_10001(t *testing.T) {
	e := newAppcfgEnv(t, 1)

	err := e.svc.Create(e.ctx, &SocialApp{Platform: "qq", AppID: "a", Secret: "s", Status: 1})
	if !errs.Is(err, errs.CodeParamInvalid) {
		t.Fatalf("err = %v, want CodeParamInvalid(10001)", err)
	}
	if n := e.count(t); n != 0 {
		t.Errorf("行数 = %d, want 0（未知平台不落库）", n)
	}
}

func TestAppConfig_Create_MissingAppIDOrSecret_10002(t *testing.T) {
	cases := []struct {
		name string
		app  *SocialApp
	}{
		{"missing_appid", &SocialApp{Platform: PlatformWechatMP, AppID: "", Secret: "s", Status: 1}},
		{"missing_secret", &SocialApp{Platform: PlatformWechatMP, AppID: "a", Secret: "", Status: 1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newAppcfgEnv(t, 1)
			err := e.svc.Create(e.ctx, c.app)
			if !errs.Is(err, errs.CodeParamMissing) {
				t.Fatalf("err = %v, want CodeParamMissing(10002)", err)
			}
			if n := e.count(t); n != 0 {
				t.Errorf("行数 = %d, want 0（缺必填参数不落库）", n)
			}
		})
	}
}

func TestAppConfig_Create_DupPlatform_10001(t *testing.T) {
	e := newAppcfgEnv(t, 1)
	e.seed(t, PlatformWechatMP, "first-secret")

	err := e.svc.Create(e.ctx, &SocialApp{
		Platform: PlatformWechatMP, AppID: "second-appid", Secret: "second-secret", Status: 1,
	})
	if !errs.Is(err, errs.CodeParamInvalid) {
		t.Fatalf("err = %v, want CodeParamInvalid(10001)", err)
	}
	if n := e.count(t); n != 1 {
		t.Fatalf("行数 = %d, want 1（重复创建不插行）", n)
	}
	got, gerr := e.repo.GetApp(e.ctx, e.tid, PlatformWechatMP)
	if gerr != nil {
		t.Fatal(gerr)
	}
	if got.AppID != "appid-wechat_mp" || got.Secret != "first-secret" {
		t.Errorf("既有行被改动: appid=%q secret=%q, want 首行原值", got.AppID, got.Secret)
	}
}

func TestAppConfig_Create_OK_SecretStoredPlain(t *testing.T) {
	const tid = uint64(1)
	e := newAppcfgEnv(t, tid)
	// 入参带越权 ID/TenantID（JSON 绑定可带 id 字段）：服务必须归位后落库
	app := &SocialApp{
		ID: 999, TenantID: 777,
		Platform: PlatformWechatMP, AppID: "wx-123", Secret: "plain-secret-456", Status: 1,
	}
	if err := e.svc.Create(e.ctx, app); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if app.ID == 999 {
		t.Errorf("ID = %d, want 归 0 后自增（入参 999 不应原样落库）", app.ID)
	}
	if app.TenantID != tid {
		t.Errorf("TenantID = %d, want %d（ctx 租户显式注入，CreateApp 走 IgnoreTenant）", app.TenantID, tid)
	}
	got, err := e.repo.GetApp(e.ctx, tid, PlatformWechatMP)
	if err != nil {
		t.Fatalf("落库后取回: %v", err)
	}
	if got.Secret != "plain-secret-456" {
		t.Errorf("Secret = %q, want 明文原样存储 %q", got.Secret, "plain-secret-456")
	}
	if got.TenantID != tid || got.AppID != "wx-123" || got.Status != 1 {
		t.Errorf("行 = %+v, want tenant=1 appid=wx-123 status=1", got)
	}
}

// brief 规则：Status ∉ {0,1} → errs.CodeParamInvalid（Step 1 未列名，自审补覆盖）。
func TestAppConfig_Create_StatusInvalid_10001(t *testing.T) {
	e := newAppcfgEnv(t, 1)

	err := e.svc.Create(e.ctx, &SocialApp{Platform: PlatformWechatMP, AppID: "a", Secret: "s", Status: 2})
	if !errs.Is(err, errs.CodeParamInvalid) {
		t.Fatalf("err = %v, want CodeParamInvalid(10001)", err)
	}
	if n := e.count(t); n != 0 {
		t.Errorf("行数 = %d, want 0", n)
	}
}

// === Update（brief Step 1 用例逐字）===

func TestAppConfig_Update_SecretEmpty_KeepsOld(t *testing.T) {
	const tid = uint64(1)
	e := newAppcfgEnv(t, tid)
	seeded := e.seed(t, PlatformWechatMP, "old-secret")

	err := e.svc.Update(e.ctx, &SocialApp{
		ID: seeded.ID, Platform: PlatformWechatMP,
		AppID: "new-appid", Secret: "", Status: 0,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, gerr := e.repo.GetApp(e.ctx, tid, PlatformWechatMP)
	if gerr != nil {
		t.Fatal(gerr)
	}
	if got.Secret != "old-secret" {
		t.Errorf("Secret = %q, want 保留旧值 %q（留空不改机密）", got.Secret, "old-secret")
	}
	if got.AppID != "new-appid" {
		t.Errorf("AppID = %q, want new-appid（其余字段正常更新）", got.AppID)
	}
	if got.Status != 0 {
		t.Errorf("Status = %d, want 0", got.Status)
	}
}

func TestAppConfig_Update_UnknownPlatform_10001(t *testing.T) {
	const tid = uint64(1)
	e := newAppcfgEnv(t, tid)
	seeded := e.seed(t, PlatformWechatMP, "sec")

	err := e.svc.Update(e.ctx, &SocialApp{
		ID: seeded.ID, Platform: "qq", AppID: "a", Secret: "s", Status: 1,
	})
	if !errs.Is(err, errs.CodeParamInvalid) {
		t.Fatalf("err = %v, want CodeParamInvalid(10001)", err)
	}
	got, gerr := e.repo.GetApp(e.ctx, tid, PlatformWechatMP)
	if gerr != nil {
		t.Fatal(gerr)
	}
	if got.AppID != "appid-wechat_mp" || got.Secret != "sec" {
		t.Errorf("拒绝后行 = %+v, want 原值不变", got)
	}
}

func TestAppConfig_Update_CrossTenant_10001(t *testing.T) {
	const tid = uint64(1)
	e := newAppcfgEnv(t, tid)
	seeded := e.seed(t, PlatformWechatMP, "sec")

	// 访问者 ctx 租户=2：GetAppByID(2, id) 跨租户不可见 → ErrRecordNotFound → 10001
	err := e.svc.Update(ctxTid(2), &SocialApp{
		ID: seeded.ID, Platform: PlatformWechatMP, AppID: "hacked", Secret: "hacked", Status: 1,
	})
	if !errs.Is(err, errs.CodeParamInvalid) {
		t.Fatalf("err = %v, want CodeParamInvalid(10001)", err)
	}
	got, gerr := e.repo.GetApp(e.ctx, tid, PlatformWechatMP)
	if gerr != nil {
		t.Fatal(gerr)
	}
	if got.AppID != "appid-wechat_mp" || got.Secret != "sec" || got.TenantID != tid {
		t.Errorf("跨租户更新后行 = %+v, want 原值不变", got)
	}
}

func TestAppConfig_Update_PlatformTakenByOtherRow_10001(t *testing.T) {
	const tid = uint64(1)
	e := newAppcfgEnv(t, tid)
	mp := e.seed(t, PlatformWechatMP, "sec-mp")
	web := e.seed(t, PlatformWechatWeb, "sec-web")

	// 把 mp 行改成 wechat_web：已被 web 行（不同 ID）占用 → 10001
	err := e.svc.Update(e.ctx, &SocialApp{
		ID: mp.ID, Platform: PlatformWechatWeb, AppID: "mp-appid", Secret: "", Status: 1,
	})
	if !errs.Is(err, errs.CodeParamInvalid) {
		t.Fatalf("err = %v, want CodeParamInvalid(10001)", err)
	}
	gotMP, merr := e.repo.GetApp(e.ctx, tid, PlatformWechatMP)
	if merr != nil {
		t.Fatal(merr)
	}
	if gotMP.Platform != PlatformWechatMP || gotMP.AppID != "appid-wechat_mp" {
		t.Errorf("mp 行 = %+v, want 未被改动", gotMP)
	}
	gotWeb, werr := e.repo.GetApp(e.ctx, tid, PlatformWechatWeb)
	if werr != nil {
		t.Fatal(werr)
	}
	if gotWeb.ID != web.ID || gotWeb.Secret != "sec-web" {
		t.Errorf("web 行 = %+v, want 原行原值（不被覆盖）", gotWeb)
	}
}

// brief 规则：Update ID==0 → errs.CodeParamInvalid（Step 1 未列名，自审补覆盖）。
func TestAppConfig_Update_IDZero_10001(t *testing.T) {
	e := newAppcfgEnv(t, 1)

	err := e.svc.Update(e.ctx, &SocialApp{
		ID: 0, Platform: PlatformWechatMP, AppID: "a", Secret: "s", Status: 1,
	})
	if !errs.Is(err, errs.CodeParamInvalid) {
		t.Fatalf("err = %v, want CodeParamInvalid(10001)", err)
	}
}

// brief 规则：Update AppID=="" → errs.CodeParamMissing（Step 1 未列名，自审补覆盖）。
func TestAppConfig_Update_MissingAppID_10002(t *testing.T) {
	e := newAppcfgEnv(t, 1)
	seeded := e.seed(t, PlatformWechatMP, "sec")

	err := e.svc.Update(e.ctx, &SocialApp{
		ID: seeded.ID, Platform: PlatformWechatMP, AppID: "", Secret: "s", Status: 1,
	})
	if !errs.Is(err, errs.CodeParamMissing) {
		t.Fatalf("err = %v, want CodeParamMissing(10002)", err)
	}
}

// === List（brief Step 1 用例逐字）===

func TestAppConfig_List_PlatformFilter(t *testing.T) {
	const tid = uint64(1)
	e := newAppcfgEnv(t, tid)
	e.seed(t, PlatformWechatMP, "s1")
	e.seed(t, PlatformWechatWeb, "s2")
	// 跨租户对照行：不得入列
	if err := e.repo.CreateApp(ctxTid(2), &SocialApp{
		TenantID: 2, Platform: PlatformWechatMP, AppID: "t2-appid", Secret: "t2", Status: 1,
	}); err != nil {
		t.Fatal(err)
	}

	// platform=="" → 全平台
	all, total, err := e.svc.List(e.ctx, 1, 10, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 2 || len(all) != 2 {
		t.Fatalf("all total=%d len=%d, want 2/2（tenant=1 全平台，跨租户行不入列）", total, len(all))
	}
	// 指定平台 → 只回该平台
	only, ot, err := e.svc.List(e.ctx, 1, 10, PlatformWechatWeb)
	if err != nil {
		t.Fatalf("List(platform): %v", err)
	}
	if ot != 1 || len(only) != 1 {
		t.Fatalf("filtered total=%d len=%d, want 1/1", ot, len(only))
	}
	if only[0].Platform != PlatformWechatWeb || only[0].TenantID != tid {
		t.Errorf("行 = %+v, want platform=wechat_web tenant=1", only[0])
	}
}

// === Delete（brief Step 1 用例逐字）===

func TestAppConfig_Delete_OK(t *testing.T) {
	const tid = uint64(1)
	e := newAppcfgEnv(t, tid)
	seeded := e.seed(t, PlatformWechatMP, "sec")

	if err := e.svc.Delete(e.ctx, seeded.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := e.repo.GetAppByID(e.ctx, tid, seeded.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("删除后 err = %v, want ErrRecordNotFound", err)
	}
	// brief Delete 规则：RowsAffected==0（repo 回 ErrRecordNotFound）→ 10001
	if err := e.svc.Delete(e.ctx, seeded.ID); !errs.Is(err, errs.CodeParamInvalid) {
		t.Errorf("重复删除 err = %v, want CodeParamInvalid(10001)", err)
	}
}

// 自审补覆盖：跨租户 Delete 不可见 → 10001 且原行存活。
func TestAppConfig_Delete_CrossTenant_10001(t *testing.T) {
	const tid = uint64(1)
	e := newAppcfgEnv(t, tid)
	seeded := e.seed(t, PlatformWechatMP, "sec")

	if err := e.svc.Delete(ctxTid(2), seeded.ID); !errs.Is(err, errs.CodeParamInvalid) {
		t.Fatalf("err = %v, want CodeParamInvalid(10001)", err)
	}
	if _, err := e.repo.GetAppByID(e.ctx, tid, seeded.ID); err != nil {
		t.Errorf("跨租户删除后原行丢失: %v", err)
	}
}
