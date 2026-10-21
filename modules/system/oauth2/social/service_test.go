package social

import (
	"context"
	"errors"
	"strings"
	"testing"

	"gorm.io/gorm"

	membermodel "alexGo-cloud/modules/member/model"
	memberrepo "alexGo-cloud/modules/member/repository"
	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/token"
)

// === 夹具（brief Step 1 逐字）===

type fakeIssuer struct {
	last token.IssueParams
	err  error
}

func (f *fakeIssuer) Issue(ctx context.Context, p token.IssueParams) (*token.Issued, error) {
	f.last = p
	if f.err != nil {
		return nil, f.err
	}
	return &token.Issued{AccessToken: "at", RefreshToken: "rt", ExpiresIn: 3600}, nil
}

func (f *fakeIssuer) Refresh(context.Context, string) (*token.Issued, error)  { return nil, nil }
func (f *fakeIssuer) Revoke(context.Context, string) error                    { return nil }
func (f *fakeIssuer) RevokeAll(context.Context, token.UserType, uint64) error { return nil }

type fakeAuditor struct{ msgs []string }

func (f *fakeAuditor) RecordLogin(ctx context.Context, username string, userID uint64, ip, ua string, success bool, msg string) error {
	f.msgs = append(f.msgs, msg)
	return nil
}

type stubAdapter struct {
	platform string
	idn      *Identity
	err      error
	calls    int
}

func (a *stubAdapter) Platform() string { return a.platform }
func (a *stubAdapter) Exchange(ctx context.Context, app *SocialApp, code string) (*Identity, error) {
	a.calls++
	return a.idn, a.err
}

// stubLimit 额度检查器桩（brief 用例 TestLogin_QuotaError_Propagates 所需）。
type stubLimit struct{ err error }

func (l *stubLimit) CheckAccountLimit(context.Context, uint64) error { return l.err }

// === 环境（每用例独立内存库：SocialApp + MemberIdentity + member_user）===

func newLoginDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := newTestDB(t) // SocialApp + MemberIdentity + gormplugin（repository_test.go）
	if err := db.AutoMigrate(&membermodel.MemberUser{}); err != nil {
		t.Fatal(err)
	}
	return db
}

type loginEnv struct {
	tid     uint64
	db      *gorm.DB
	repo    SocialRepository
	mem     memberrepo.MemberRepository
	issuer  *fakeIssuer
	auditor *fakeAuditor
	adp     *stubAdapter
	reg     *AdapterRegistry
	ctx     context.Context
}

func newLoginEnv(t *testing.T, tid uint64) *loginEnv {
	t.Helper()
	db := newLoginDB(t)
	adp := &stubAdapter{platform: PlatformWechatMP}
	return &loginEnv{
		tid:     tid,
		db:      db,
		repo:    NewSocialRepository(db),
		mem:     memberrepo.NewMemberRepository(db),
		issuer:  &fakeIssuer{},
		auditor: &fakeAuditor{},
		adp:     adp,
		reg:     &AdapterRegistry{byPlatform: map[string]PlatformAdapter{PlatformWechatMP: adp}},
		ctx:     ctxTid(tid),
	}
}

// svc 组装 SocialService；members/limit 传 nil 模拟 optional 依赖未接线。
func (e *loginEnv) svc(members MemberStore, limit tenant.AccountLimitChecker) *SocialService {
	return NewSocialService(SocialDeps{
		Repo:     e.repo,
		Members:  members,
		Adapters: e.reg,
		Issuer:   e.issuer,
		Auditor:  e.auditor,
		Limit:    limit,
	})
}

func (e *loginEnv) defaultSvc() *SocialService { return e.svc(e.mem, nil) }

func (e *loginEnv) seedApp(t *testing.T, platform string, status int) {
	t.Helper()
	if err := e.repo.CreateApp(e.ctx, &SocialApp{
		TenantID: e.tid, Platform: platform, AppID: "wx-appid", Secret: "sec", Status: status,
	}); err != nil {
		t.Fatal(err)
	}
}

func (e *loginEnv) seedIdentity(t *testing.T, platform, openid, unionid string, memberID uint64) {
	t.Helper()
	if err := e.repo.CreateIdentity(e.ctx, &MemberIdentity{
		TenantID: e.tid, MemberUserID: memberID, Platform: platform, OpenID: openid, UnionID: unionid,
	}); err != nil {
		t.Fatal(err)
	}
}

func (e *loginEnv) seedMember(t *testing.T, id uint64, mut func(*membermodel.MemberUser)) {
	t.Helper()
	u := &membermodel.MemberUser{ID: id, TenantID: e.tid, Nickname: "n", Status: 1}
	if mut != nil {
		mut(u)
	}
	if err := e.mem.Create(e.ctx, u); err != nil {
		t.Fatal(err)
	}
}

// === 用例 ===

func TestLogin_IdentityHit_IssuesAndAuditsOnce(t *testing.T) {
	const tid = uint64(1)
	e := newLoginEnv(t, tid)
	e.seedApp(t, PlatformWechatMP, 1)
	e.seedIdentity(t, PlatformWechatMP, "OID", "", 100)
	e.seedMember(t, 100, nil)
	e.adp.idn = &Identity{OpenID: "OID"}

	res, err := e.defaultSvc().Login(e.ctx, PlatformWechatMP, "code-1", "1.2.3.4", "ua-1")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if res.AccessToken != "at" || res.RefreshToken != "rt" || res.ExpiresIn != 3600 {
		t.Errorf("result = %+v, want at/rt/3600", res)
	}
	want := token.IssueParams{
		UserID: 100, TenantID: tid, ClientID: "alexgo-app", UserType: token.UserTypeMember,
	}
	if e.issuer.last != want {
		t.Errorf("IssueParams = %+v, want %+v", e.issuer.last, want)
	}
	if len(e.auditor.msgs) != 1 {
		t.Fatalf("审计条数 = %d, want 1（恰一次）: %v", len(e.auditor.msgs), e.auditor.msgs)
	}
	if !strings.Contains(e.auditor.msgs[0], "微信小程序登录") || !strings.Contains(e.auditor.msgs[0], "OID") {
		t.Errorf("审计 msg = %q, want 含 \"微信小程序登录\" 与 openid", e.auditor.msgs[0])
	}
}

func TestLogin_MemberDeletedOrDisabled_Rejected(t *testing.T) {
	const tid = uint64(1)
	cases := []struct {
		name string
		mut  func(*membermodel.MemberUser)
	}{
		{"deleted", func(u *membermodel.MemberUser) { u.Deleted = true }},
		{"disabled", func(u *membermodel.MemberUser) { u.Status = 0 }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newLoginEnv(t, tid)
			e.seedApp(t, PlatformWechatMP, 1)
			e.seedIdentity(t, PlatformWechatMP, "OID", "", 100)
			e.seedMember(t, 100, c.mut)
			e.adp.idn = &Identity{OpenID: "OID"}

			_, err := e.defaultSvc().Login(e.ctx, PlatformWechatMP, "code", "ip", "ua")
			if !errs.Is(err, errs.CodeUserNotFound) {
				t.Fatalf("err = %v, want CodeUserNotFound(30002)", err)
			}
			if e.issuer.last != (token.IssueParams{}) {
				t.Errorf("失败登录不应签发，IssueParams = %+v", e.issuer.last)
			}
		})
	}
}

func TestLogin_UnionIDBackfill(t *testing.T) {
	const tid = uint64(1)
	e := newLoginEnv(t, tid)
	e.seedApp(t, PlatformWechatMP, 1)
	e.seedIdentity(t, PlatformWechatWeb, "B", "U", 100) // 存量：网页端已绑 member 100
	e.seedMember(t, 100, nil)
	e.adp.idn = &Identity{OpenID: "A", UnionID: "U"} // 小程序端同人不同 openid

	if _, err := e.defaultSvc().Login(e.ctx, PlatformWechatMP, "code", "ip", "ua"); err != nil {
		t.Fatalf("Login: %v", err)
	}
	idn, err := e.repo.GetIdentity(e.ctx, tid, PlatformWechatMP, "A")
	if err != nil {
		t.Fatalf("补建行未落库: %v", err)
	}
	if idn.MemberUserID != 100 || idn.UnionID != "U" || idn.TenantID != tid {
		t.Errorf("补建行 = %+v, want member=100 unionid=U tenant=1", idn)
	}
	if e.issuer.last.UserID != 100 {
		t.Errorf("IssueParams.UserID = %d, want 100（签发给同人既有会员）", e.issuer.last.UserID)
	}
}

func TestLogin_EmptyUnionID_NotMatched(t *testing.T) { // Review Focus #6
	const tid = uint64(1)
	e := newLoginEnv(t, tid)
	e.seedApp(t, PlatformWechatMP, 1)
	e.seedIdentity(t, PlatformWechatWeb, "B", "", 100) // 存量 unionid 空
	e.seedMember(t, 100, nil)
	e.adp.idn = &Identity{OpenID: "A", UnionID: ""} // unionid 空、openid 不同 → 不是同人

	if _, err := e.defaultSvc().Login(e.ctx, PlatformWechatMP, "code", "ip", "ua"); err != nil {
		t.Fatalf("Login: %v", err)
	}
	idn, err := e.repo.GetIdentity(e.ctx, tid, PlatformWechatMP, "A")
	if err != nil {
		t.Fatalf("空壳绑定行未落库: %v", err)
	}
	if idn.MemberUserID == 100 {
		t.Errorf("unionid='' 不应补绑到 member 100，got member=%d", idn.MemberUserID)
	}
	if e.issuer.last.UserID == 100 {
		t.Errorf("应签发给新空壳而非 member 100，got UserID=%d", e.issuer.last.UserID)
	}
	// 存量行不被改指
	old, err := e.repo.GetIdentity(e.ctx, tid, PlatformWechatWeb, "B")
	if err != nil || old.MemberUserID != 100 {
		t.Errorf("存量行 = %+v, err=%v, want member=100", old, err)
	}
	// 空壳应是新注册会员（member 行数 2）
	var n int64
	if err := e.db.Model(&membermodel.MemberUser{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("member 行数 = %d, want 2", n)
	}
}

func TestLogin_NotConfigured_30301_NoAdapterCall(t *testing.T) {
	e := newLoginEnv(t, 1) // 无 SocialApp 行

	_, err := e.defaultSvc().Login(e.ctx, PlatformWechatMP, "code", "ip", "ua")
	if !errs.Is(err, errs.CodeSocialAppNotConfigured) {
		t.Fatalf("err = %v, want CodeSocialAppNotConfigured(30301)", err)
	}
	if e.adp.calls != 0 {
		t.Errorf("Exchange 调用 = %d, want 0（未配置不耗 code 配额）", e.adp.calls)
	}
}

func TestLogin_Disabled_30301(t *testing.T) {
	e := newLoginEnv(t, 1)
	e.seedApp(t, PlatformWechatMP, 0) // Status=0 停用

	_, err := e.defaultSvc().Login(e.ctx, PlatformWechatMP, "code", "ip", "ua")
	if !errs.Is(err, errs.CodeSocialAppNotConfigured) {
		t.Fatalf("err = %v, want CodeSocialAppNotConfigured(30301)", err)
	}
	if e.adp.calls != 0 {
		t.Errorf("Exchange 调用 = %d, want 0（停用不耗 code 配额）", e.adp.calls)
	}
}

func TestLogin_ExchangeErr_30302(t *testing.T) {
	e := newLoginEnv(t, 1)
	e.seedApp(t, PlatformWechatMP, 1)
	e.adp.err = errors.New("wechat: 40029 invalid code")

	_, err := e.defaultSvc().Login(e.ctx, PlatformWechatMP, "bad-code", "ip", "ua")
	if !errs.Is(err, errs.CodeSocialExchangeFailed) {
		t.Fatalf("err = %v, want CodeSocialExchangeFailed(30302)", err)
	}
	if e.adp.calls != 1 {
		t.Errorf("Exchange 调用 = %d, want 1", e.adp.calls)
	}
}

func TestLogin_AutoRegisterShell_SanitizesNickname(t *testing.T) { // Review Focus #5
	cases := []struct {
		name   string
		nick   string
		openID string
		want   string // "" 表示只断言非空且无 ':'
	}{
		{"colon-stripped", "微:信:用", "open-abcdef12", "微信用"},
		{"empty-placeholder", "", "open-abcdef12", "微信用户ef12"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			const tid = uint64(1)
			e := newLoginEnv(t, tid)
			e.seedApp(t, PlatformWechatMP, 1)
			e.adp.idn = &Identity{OpenID: c.openID, Nickname: c.nick}

			if _, err := e.defaultSvc().Login(e.ctx, PlatformWechatMP, "code", "ip", "ua"); err != nil {
				t.Fatalf("Login: %v", err)
			}
			idn, err := e.repo.GetIdentity(e.ctx, tid, PlatformWechatMP, c.openID)
			if err != nil {
				t.Fatalf("空壳绑定行未落库: %v", err)
			}
			u, err := e.mem.GetByID(e.ctx, tid, idn.MemberUserID)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(u.Nickname, ":") {
				t.Errorf("nickname = %q, 不得含 ':'（casbin 命名空间撞车）", u.Nickname)
			}
			if u.Nickname == "" {
				t.Fatal("nickname 为空（token.Validate 拒空昵称）")
			}
			if c.want != "" && u.Nickname != c.want {
				t.Errorf("nickname = %q, want %q", u.Nickname, c.want)
			}
			if e.issuer.last.UserID != u.ID {
				t.Errorf("IssueParams.UserID = %d, want 空壳 id %d", e.issuer.last.UserID, u.ID)
			}
		})
	}
}

func TestLogin_QuotaError_Propagates(t *testing.T) {
	e := newLoginEnv(t, 1)
	e.seedApp(t, PlatformWechatMP, 1)
	e.adp.idn = &Identity{OpenID: "NEW", Nickname: "nick"}
	quotaErr := errors.New("account limit reached (5)")

	_, err := e.svc(e.mem, &stubLimit{err: quotaErr}).Login(e.ctx, PlatformWechatMP, "code", "ip", "ua")
	if !errors.Is(err, quotaErr) {
		t.Fatalf("err = %v, want 原样透传 %v", err, quotaErr)
	}
	var n int64
	if err := e.db.Model(&membermodel.MemberUser{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("额度拒绝后 member 行数 = %d, want 0（不应落空壳）", n)
	}
}

func TestLogin_IssuerErr_WrapsErrIssuance(t *testing.T) {
	e := newLoginEnv(t, 1)
	e.seedApp(t, PlatformWechatMP, 1)
	e.seedIdentity(t, PlatformWechatMP, "OID", "", 100)
	e.seedMember(t, 100, nil)
	e.adp.idn = &Identity{OpenID: "OID"}
	e.issuer.err = errors.New("token db down")

	_, err := e.defaultSvc().Login(e.ctx, PlatformWechatMP, "code", "ip", "ua")
	if !errors.Is(err, oauth2svc.ErrIssuance) {
		t.Fatalf("err = %v, want wraps ErrIssuance", err)
	}
	if len(e.auditor.msgs) != 0 {
		t.Errorf("签发失败不得记成功审计: %v", e.auditor.msgs)
	}
}

func TestLogin_MembersNil_ErrAuthenticatorMissing(t *testing.T) {
	e := newLoginEnv(t, 1)
	e.seedApp(t, PlatformWechatMP, 1)
	e.seedIdentity(t, PlatformWechatMP, "OID", "", 100)
	e.adp.idn = &Identity{OpenID: "OID"}

	_, err := e.svc(nil, nil).Login(e.ctx, PlatformWechatMP, "code", "ip", "ua")
	if !errors.Is(err, oauth2svc.ErrAuthenticatorMissing) {
		t.Fatalf("err = %v, want ErrAuthenticatorMissing", err)
	}
}

// === Task 7：绑定 / 手机号合并 / 解绑守门（brief Step 1 用例逐字）===

func TestBindSocial_PlatformUnknown_10001(t *testing.T) {
	e := newLoginEnv(t, 1)

	err := e.defaultSvc().BindSocial(e.ctx, 100, "qq", "code-1")
	if !errs.Is(err, errs.CodeParamInvalid) {
		t.Fatalf("err = %v, want CodeParamInvalid(10001)", err)
	}
	if e.adp.calls != 0 {
		t.Errorf("Exchange 调用 = %d, want 0（未知平台不耗 code 配额）", e.adp.calls)
	}
}

func TestBindSocial_NotConfigured_30301(t *testing.T) {
	e := newLoginEnv(t, 1) // 无 SocialApp 行

	err := e.defaultSvc().BindSocial(e.ctx, 100, PlatformWechatMP, "code-1")
	if !errs.Is(err, errs.CodeSocialAppNotConfigured) {
		t.Fatalf("err = %v, want CodeSocialAppNotConfigured(30301)", err)
	}
	if e.adp.calls != 0 {
		t.Errorf("Exchange 调用 = %d, want 0（未配置不耗 code 配额）", e.adp.calls)
	}
}

func TestBindSocial_ExchangeErr_30302(t *testing.T) {
	const tid = uint64(1)
	e := newLoginEnv(t, tid)
	e.seedApp(t, PlatformWechatMP, 1)
	e.adp.err = errors.New("wechat: 40029 invalid code")

	err := e.defaultSvc().BindSocial(e.ctx, 100, PlatformWechatMP, "bad-code")
	if !errs.Is(err, errs.CodeSocialExchangeFailed) {
		t.Fatalf("err = %v, want CodeSocialExchangeFailed(30302)", err)
	}
	if e.adp.calls != 1 {
		t.Errorf("Exchange 调用 = %d, want 1", e.adp.calls)
	}
	var n int64
	if err := e.db.Model(&MemberIdentity{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("绑定行数 = %d, want 0（换取失败不落行）", n)
	}
}

func TestBindSocial_AlreadyBoundSelf_Idempotent(t *testing.T) {
	const tid = uint64(1)
	e := newLoginEnv(t, tid)
	e.seedApp(t, PlatformWechatMP, 1)
	e.seedIdentity(t, PlatformWechatMP, "OID", "U", 100)
	e.adp.idn = &Identity{OpenID: "OID", UnionID: "U"}

	if err := e.defaultSvc().BindSocial(e.ctx, 100, PlatformWechatMP, "code-1"); err != nil {
		t.Fatalf("已绑自己应幂等 nil, got %v", err)
	}
	var n int64
	if err := e.db.Model(&MemberIdentity{}).
		Where("tenant_id = ? AND platform = ? AND openid = ?", tid, PlatformWechatMP, "OID").
		Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("绑定行数 = %d, want 1（不重复插行）", n)
	}
}

func TestBindSocial_BoundOther_30303(t *testing.T) {
	const tid = uint64(1)
	e := newLoginEnv(t, tid)
	e.seedApp(t, PlatformWechatMP, 1)
	e.seedIdentity(t, PlatformWechatMP, "OID", "", 200)
	e.adp.idn = &Identity{OpenID: "OID"}

	err := e.defaultSvc().BindSocial(e.ctx, 100, PlatformWechatMP, "code-1")
	if !errs.Is(err, errs.CodeSocialIdentityBound) {
		t.Fatalf("err = %v, want CodeSocialIdentityBound(30303)", err)
	}
	idn, gerr := e.repo.GetIdentity(e.ctx, tid, PlatformWechatMP, "OID")
	if gerr != nil || idn.MemberUserID != 200 {
		t.Errorf("绑定行 = %+v err=%v, want member=200（30303 不迁移）", idn, gerr)
	}
}

func TestBindSocial_Unbound_Binds(t *testing.T) {
	const tid = uint64(1)
	e := newLoginEnv(t, tid)
	e.seedApp(t, PlatformWechatMP, 1)
	e.adp.idn = &Identity{OpenID: "NEW", UnionID: "U"}

	if err := e.defaultSvc().BindSocial(e.ctx, 100, PlatformWechatMP, "code-1"); err != nil {
		t.Fatalf("BindSocial: %v", err)
	}
	idn, err := e.repo.GetIdentity(e.ctx, tid, PlatformWechatMP, "NEW")
	if err != nil {
		t.Fatalf("绑定行未落库: %v", err)
	}
	if idn.MemberUserID != 100 || idn.TenantID != tid || idn.UnionID != "U" {
		t.Errorf("绑定行 = %+v, want member=100 tenant=1 unionid=U", idn)
	}
}

func TestBindMobile_FormatInvalid_10001(t *testing.T) {
	e := newLoginEnv(t, 1)
	e.seedMember(t, 100, nil)

	err := e.defaultSvc().BindMobile(e.ctx, 100, "123")
	if !errs.Is(err, errs.CodeParamInvalid) {
		t.Fatalf("err = %v, want CodeParamInvalid(10001)", err)
	}
}

func TestBindMobile_AlreadyHasMobile_30304(t *testing.T) {
	const tid = uint64(1)
	e := newLoginEnv(t, tid)
	e.seedMember(t, 100, func(u *membermodel.MemberUser) { u.Mobile = "13800000000" })

	err := e.defaultSvc().BindMobile(e.ctx, 100, "13900000000")
	if !errs.Is(err, errs.CodeSocialMobileBound) {
		t.Fatalf("err = %v, want CodeSocialMobileBound(30304)", err)
	}
	u, gerr := e.mem.GetByID(e.ctx, tid, 100)
	if gerr != nil {
		t.Fatal(gerr)
	}
	if u.Mobile != "13800000000" {
		t.Errorf("mobile = %q, want 原号不变（30304 不改绑）", u.Mobile)
	}
}

func TestBindMobile_Free_SetsMobile(t *testing.T) {
	const tid = uint64(1)
	e := newLoginEnv(t, tid)
	e.seedMember(t, 100, func(u *membermodel.MemberUser) { u.Password = "bcrypt-hash" })

	if err := e.defaultSvc().BindMobile(e.ctx, 100, "13900139000"); err != nil {
		t.Fatalf("BindMobile: %v", err)
	}
	u, err := e.mem.GetByID(e.ctx, tid, 100)
	if err != nil {
		t.Fatal(err)
	}
	if u.Mobile != "13900139000" {
		t.Errorf("mobile = %q, want 13900139000", u.Mobile)
	}
	if u.Deleted {
		t.Error("手机号空闲绑定成功不应软删")
	}
	byMobile, berr := e.mem.GetByMobile(e.ctx, tid, "13900139000")
	if berr != nil || byMobile.ID != 100 {
		t.Errorf("GetByMobile = %+v err=%v, want id=100", byMobile, berr)
	}
}

func TestBindMobile_Taken_NonShell_30305(t *testing.T) { // Review Focus #4：非空壳（有手机号或密码）拒绝
	const tid = uint64(1)
	e := newLoginEnv(t, tid)
	e.seedMember(t, 100, func(u *membermodel.MemberUser) { u.Password = "bcrypt-hash" }) // 非空壳：有密码
	e.seedMember(t, 200, func(u *membermodel.MemberUser) { u.Mobile = "13900139000" })
	e.seedIdentity(t, PlatformWechatMP, "OID", "", 100)

	err := e.defaultSvc().BindMobile(e.ctx, 100, "13900139000")
	if !errs.Is(err, errs.CodeSocialMergeRefused) {
		t.Fatalf("err = %v, want CodeSocialMergeRefused(30305)", err)
	}
	// 不误伤：未软删、identity 不改指、目标手机号不动
	me, merr := e.mem.GetByID(e.ctx, tid, 100)
	if merr != nil {
		t.Fatal(merr)
	}
	if me.Deleted {
		t.Error("30305 拒绝后当前账号不应软删")
	}
	idn, ierr := e.repo.GetIdentity(e.ctx, tid, PlatformWechatMP, "OID")
	if ierr != nil || idn.MemberUserID != 100 {
		t.Errorf("identity = %+v err=%v, want member=100（拒绝不迁移）", idn, ierr)
	}
	target, terr := e.mem.GetByID(e.ctx, tid, 200)
	if terr != nil || target.Mobile != "13900139000" {
		t.Errorf("目标 = %+v err=%v, want 手机号不变", target, terr)
	}
}

func TestBindMobile_Taken_Shell_Merges(t *testing.T) {
	// 空壳 me(无 mobile 无 password, 有 identity wechat_mp) + 目标 member 有 mobile
	//   → Reassign: me 的 identity 全指目标；me.Deleted=true
	//   → 跨租户同 member_user_id 的 identity 不被改指（seed tid=2 对照行）
	//   → 软删后 GetByMobile(tid, mobile) 只回目标行
	const tid = uint64(1)
	const otherTid = uint64(2)
	e := newLoginEnv(t, tid)
	e.seedMember(t, 100, nil) // 空壳：Mobile="" Password=""
	e.seedMember(t, 200, func(u *membermodel.MemberUser) { u.Mobile = "13900139000" })
	e.seedIdentity(t, PlatformWechatMP, "OID", "", 100)
	// 跨租户对照行：同 member_user_id、tid=2
	if err := e.repo.CreateIdentity(ctxTid(otherTid), &MemberIdentity{
		TenantID: otherTid, MemberUserID: 100, Platform: PlatformWechatMP, OpenID: "OID-T2",
	}); err != nil {
		t.Fatal(err)
	}

	if err := e.defaultSvc().BindMobile(e.ctx, 100, "13900139000"); err != nil {
		t.Fatalf("空壳合并应成功: %v", err)
	}
	// tid=1 identity 改指目标
	idn, err := e.repo.GetIdentity(e.ctx, tid, PlatformWechatMP, "OID")
	if err != nil || idn.MemberUserID != 200 {
		t.Errorf("identity = %+v err=%v, want member=200（改指目标）", idn, err)
	}
	// 跨租户对照行不被改指（显式 tenant 过滤）
	other, oerr := e.repo.GetIdentity(ctxTid(otherTid), otherTid, PlatformWechatMP, "OID-T2")
	if oerr != nil || other.MemberUserID != 100 {
		t.Errorf("跨租户行 = %+v err=%v, want member=100（不改指）", other, oerr)
	}
	// 空壳软删
	me, merr := e.mem.GetByID(e.ctx, tid, 100)
	if merr != nil {
		t.Fatal(merr)
	}
	if !me.Deleted {
		t.Error("合并后空壳应软删 me.Deleted=true")
	}
	// 软删后 GetByMobile 只回目标行
	byMobile, berr := e.mem.GetByMobile(e.ctx, tid, "13900139000")
	if berr != nil || byMobile.ID != 200 {
		t.Errorf("GetByMobile = %+v err=%v, want 只回目标 id=200", byMobile, berr)
	}
	// 目标行未被误伤
	target, terr := e.mem.GetByID(e.ctx, tid, 200)
	if terr != nil || target.Deleted || target.Mobile != "13900139000" {
		t.Errorf("目标 = %+v err=%v, want 存活且手机号不变", target, terr)
	}
}

func TestBindMobile_MembersNil_ErrAuthenticatorMissing(t *testing.T) {
	e := newLoginEnv(t, 1)

	err := e.svc(nil, nil).BindMobile(e.ctx, 100, "13900139000")
	if !errors.Is(err, oauth2svc.ErrAuthenticatorMissing) {
		t.Fatalf("err = %v, want ErrAuthenticatorMissing", err)
	}
}

func TestUnbind_NoIdentity_Idempotent(t *testing.T) {
	e := newLoginEnv(t, 1)
	e.seedMember(t, 100, nil)

	if err := e.defaultSvc().UnbindSocial(e.ctx, 100, PlatformWechatMP); err != nil {
		t.Fatalf("无该平台绑定解绑应幂等 nil, got %v", err)
	}
}

func TestUnbind_HasPassword_Allows(t *testing.T) {
	const tid = uint64(1)
	e := newLoginEnv(t, tid)
	e.seedMember(t, 100, func(u *membermodel.MemberUser) { u.Password = "bcrypt-hash" })
	e.seedIdentity(t, PlatformWechatMP, "OID", "", 100)

	if err := e.defaultSvc().UnbindSocial(e.ctx, 100, PlatformWechatMP); err != nil {
		t.Fatalf("有密码应允许解绑: %v", err)
	}
	if _, err := e.repo.GetIdentity(e.ctx, tid, PlatformWechatMP, "OID"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("解绑后 err = %v, want ErrRecordNotFound", err)
	}
}

func TestUnbind_OtherPlatformRemains_Allows(t *testing.T) {
	const tid = uint64(1)
	e := newLoginEnv(t, tid)
	e.seedMember(t, 100, nil) // 无密码，但另有一平台绑定
	e.seedIdentity(t, PlatformWechatMP, "OID-MP", "", 100)
	e.seedIdentity(t, PlatformWechatWeb, "OID-WEB", "", 100)

	if err := e.defaultSvc().UnbindSocial(e.ctx, 100, PlatformWechatMP); err != nil {
		t.Fatalf("尚有其他平台可解绑: %v", err)
	}
	if _, err := e.repo.GetIdentity(e.ctx, tid, PlatformWechatMP, "OID-MP"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("被解绑行 err = %v, want ErrRecordNotFound", err)
	}
	rest, err := e.repo.ListIdentities(e.ctx, tid, 100)
	if err != nil || len(rest) != 1 || rest[0].Platform != PlatformWechatWeb {
		t.Errorf("剩余绑定 = %+v err=%v, want 仅 wechat_web", rest, err)
	}
}

func TestUnbind_LastLoginMethod_30306(t *testing.T) { // Review Focus #7：Password=="" 且 identities 只剩本平台
	const tid = uint64(1)
	e := newLoginEnv(t, tid)
	e.seedMember(t, 100, nil) // 无密码
	e.seedIdentity(t, PlatformWechatMP, "OID", "", 100)

	err := e.defaultSvc().UnbindSocial(e.ctx, 100, PlatformWechatMP)
	if !errs.Is(err, errs.CodeSocialUnbindLast) {
		t.Fatalf("err = %v, want CodeSocialUnbindLast(30306)", err)
	}
	if _, gerr := e.repo.GetIdentity(e.ctx, tid, PlatformWechatMP, "OID"); gerr != nil {
		t.Errorf("30306 守门拒绝后绑定行不得删除: %v", gerr)
	}
}
