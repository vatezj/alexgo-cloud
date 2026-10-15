package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/token"
)

type stubUserAuth struct {
	ident *AuthIdentity
	err   error
	calls int
}

func (s *stubUserAuth) Authenticate(_ context.Context, _, _ string) (*AuthIdentity, error) {
	s.calls++
	return s.ident, s.err
}

type stubMemberAuth struct {
	ident  *AuthIdentity
	err    error
	calls  int
	lastIP string
}

func (s *stubMemberAuth) Authenticate(_ context.Context, _, _, ip string) (*AuthIdentity, error) {
	s.calls++
	s.lastIP = ip
	return s.ident, s.err
}

type stubAuditor struct {
	success []bool
	msgs    []string
}

func (s *stubAuditor) RecordLogin(_ context.Context, _ string, _ uint64, _, _ string, success bool, msg string) error {
	s.success = append(s.success, success)
	s.msgs = append(s.msgs, msg)
	return nil
}

type stubJWTGen struct {
	tk    string
	err   error
	calls int
}

func (s *stubJWTGen) Issue(_ context.Context, _ uint64, _ string, _ uint64) (string, error) {
	s.calls++
	return s.tk, s.err
}

type stubIssuer struct {
	issued *token.Issued
	err    error
	calls  []token.IssueParams
}

func (s *stubIssuer) Issue(_ context.Context, p token.IssueParams) (*token.Issued, error) {
	s.calls = append(s.calls, p)
	return s.issued, s.err
}
func (s *stubIssuer) Refresh(context.Context, string) (*token.Issued, error)  { return nil, nil }
func (s *stubIssuer) Revoke(context.Context, string) error                    { return nil }
func (s *stubIssuer) RevokeAll(context.Context, token.UserType, uint64) error { return nil }

func newTestCfg(mode string) *config.Config {
	cfg := &config.Config{}
	cfg.Auth.Mode = mode
	return cfg
}

func okIssuer() *stubIssuer {
	return &stubIssuer{issued: &token.Issued{AccessToken: "a", RefreshToken: "r", ExpiresIn: 60}}
}

// Review Focus #2：登录链路确实经过 captcha Verify（开关双路径与一次性由 Task 3 钉）。
func TestLogin_CaptchaTogglePaths(t *testing.T) {
	ident := &AuthIdentity{UserID: 3, Username: "alice", AccountType: AccountTypeSystem}

	// 开关关：无 uuid/code 也放行，全链路走通。
	audOff := &stubAuditor{}
	off := NewUnifiedLogin(UnifiedDeps{
		Cfg: newTestCfg("token"), Issuer: okIssuer(),
		Captcha: newTestCaptcha(false), UserAuth: &stubUserAuth{ident: ident}, Auditor: audOff,
	})
	res, _, err := off.Login(context.Background(), LoginParams{Username: "alice", Password: "pw"})
	if err != nil || res.AccessToken != "a" {
		t.Fatalf("disabled 开关应放行, res=%+v err=%v", res, err)
	}
	if len(audOff.success) != 1 || audOff.success[0] != true {
		t.Errorf("审计 = %v, want [true]", audOff.success)
	}

	// 开关开：错码 → ErrCaptchaInvalid；不路由、不签发、审计 false 恰一次。
	ua := &stubUserAuth{ident: ident}
	iss := &stubIssuer{}
	aud := &stubAuditor{}
	on := NewUnifiedLogin(UnifiedDeps{
		Cfg: newTestCfg("token"), Issuer: iss,
		Captcha: newTestCaptcha(true), UserAuth: ua, Auditor: aud,
	})
	_, _, err = on.Login(context.Background(), LoginParams{
		Username: "alice", Password: "pw", CaptchaUUID: "x", CaptchaCode: "0000",
	})
	if !errors.Is(err, ErrCaptchaInvalid) {
		t.Fatalf("应 ErrCaptchaInvalid, got %v", err)
	}
	if ua.calls != 0 || len(iss.calls) != 0 {
		t.Errorf("captcha 失败不得路由/签发: userAuth=%d issuer=%d", ua.calls, len(iss.calls))
	}
	if len(aud.success) != 1 || aud.success[0] != false {
		t.Errorf("审计 = %v, want [false] 恰一次", aud.success)
	}
	if !strings.HasPrefix(aud.msgs[0], "oauth2: ") {
		t.Errorf("审计 msg 应带 oauth2: 前缀, got %q", aud.msgs[0])
	}
}

func TestLogin_WrongPassword_AuditFalseOnce(t *testing.T) {
	ua := &stubUserAuth{err: ErrInvalidCredentials}
	iss := &stubIssuer{}
	aud := &stubAuditor{}
	ul := NewUnifiedLogin(UnifiedDeps{
		Cfg: newTestCfg("token"), Issuer: iss, UserAuth: ua, Auditor: aud,
	})
	_, _, err := ul.Login(context.Background(), LoginParams{Username: "a", Password: "bad"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("应 ErrInvalidCredentials, got %v", err)
	}
	if len(aud.success) != 1 || aud.success[0] != false {
		t.Errorf("审计 = %v, want [false] 恰一次", aud.success)
	}
	if len(iss.calls) != 0 {
		t.Errorf("凭据失败不得签发, issuer calls=%d", len(iss.calls))
	}
}

func TestLogin_AccountTypeUnsupported_NoAudit(t *testing.T) {
	aud := &stubAuditor{}
	ul := NewUnifiedLogin(UnifiedDeps{
		Cfg: newTestCfg("token"), Issuer: okIssuer(),
		UserAuth: &stubUserAuth{ident: &AuthIdentity{UserID: 1}}, Auditor: aud,
	})
	_, _, err := ul.Login(context.Background(), LoginParams{
		AccountType: "admin", Username: "x", Password: "y",
	})
	if !errors.Is(err, ErrAccountTypeUnsupported) {
		t.Fatalf("应 ErrAccountTypeUnsupported, got %v", err)
	}
	if len(aud.success) != 0 {
		t.Errorf("类型不支持先于审计, got %v", aud.success)
	}
}

func TestLogin_EmptyAccountTypeDefaultsSystem(t *testing.T) {
	ua := &stubUserAuth{ident: &AuthIdentity{UserID: 3, Username: "alice", AccountType: AccountTypeSystem}}
	ma := &stubMemberAuth{}
	ul := NewUnifiedLogin(UnifiedDeps{
		Cfg: newTestCfg("token"), Issuer: okIssuer(),
		UserAuth: ua, MemberAuth: ma, Auditor: &stubAuditor{},
	})
	if _, _, err := ul.Login(context.Background(), LoginParams{Username: "alice", Password: "pw"}); err != nil {
		t.Fatal(err)
	}
	if ua.calls != 1 || ma.calls != 0 {
		t.Errorf("缺省应路由 system: user=%d member=%d", ua.calls, ma.calls)
	}
}

func TestLogin_IssuerNil_WrapsErrIssuance(t *testing.T) {
	aud := &stubAuditor{}
	ul := NewUnifiedLogin(UnifiedDeps{
		Cfg: newTestCfg("token"), Issuer: nil,
		UserAuth: &stubUserAuth{ident: &AuthIdentity{UserID: 3, AccountType: AccountTypeSystem}},
		Auditor:  aud,
	})
	_, _, err := ul.Login(context.Background(), LoginParams{Username: "a", Password: "b"})
	if !errors.Is(err, ErrIssuance) {
		t.Fatalf("Issuer nil 应 wrap ErrIssuance, got %v", err)
	}
	if len(aud.success) != 1 || aud.success[0] != false {
		t.Errorf("签发失败审计 = %v, want [false]", aud.success)
	}
}

func TestIssueFor_JwtMode_SystemOnly(t *testing.T) {
	jw := &stubJWTGen{tk: "jwt-token"}
	iss := okIssuer()
	ul := NewUnifiedLogin(UnifiedDeps{Cfg: newTestCfg("jwt"), Issuer: iss, JWTGen: jw})

	res, err := ul.IssueFor(context.Background(),
		&AuthIdentity{UserID: 3, Username: "alice", AccountType: AccountTypeSystem}, "alexgo-admin")
	if err != nil {
		t.Fatal(err)
	}
	if res.AccessToken != "jwt-token" || res.RefreshToken != "" || res.ExpiresIn != 0 {
		t.Errorf("jwt 模式 system 结果 = %+v", res)
	}
	if jw.calls != 1 || len(iss.calls) != 0 {
		t.Errorf("应只走 JWTGen: jwt=%d issuer=%d", jw.calls, len(iss.calls))
	}

	// D2：jwt 回滚只针对 system；member 仍签不透明令牌。
	res2, err := ul.IssueFor(context.Background(),
		&AuthIdentity{UserID: 9, Username: "m", AccountType: AccountTypeMember}, "alexgo-app")
	if err != nil {
		t.Fatal(err)
	}
	if res2.AccessToken != "a" || res2.RefreshToken != "r" {
		t.Errorf("member jwt 模式应不透明, got %+v", res2)
	}
	if len(iss.calls) != 1 || iss.calls[0].UserType != token.UserTypeMember {
		t.Errorf("issuer calls = %+v", iss.calls)
	}
}

// D9：机机恒不透明——jwt 模式也不走 JWTGen，refresh 强制空。
func TestIssueFor_ClientOpaque(t *testing.T) {
	jw := &stubJWTGen{}
	iss := okIssuer()
	ul := NewUnifiedLogin(UnifiedDeps{Cfg: newTestCfg("jwt"), Issuer: iss, JWTGen: jw})

	res, err := ul.IssueFor(context.Background(),
		&AuthIdentity{AccountType: AccountTypeClient, Username: "e2e-m2m"}, "e2e-m2m")
	if err != nil {
		t.Fatal(err)
	}
	if res.RefreshToken != "" {
		t.Errorf("机机不得带 refresh, got %q", res.RefreshToken)
	}
	if jw.calls != 0 {
		t.Errorf("client 不得走 JWTGen, calls=%d", jw.calls)
	}
	if len(iss.calls) != 1 {
		t.Fatalf("issuer calls = %d", len(iss.calls))
	}
	p := iss.calls[0]
	if p.UserType != token.UserTypeClient || p.UserID != 0 || p.TenantID != 0 || p.ClientID != "e2e-m2m" {
		t.Errorf("IssueParams = %+v", p)
	}
}

func TestIssueFor_IssueError_Wraps(t *testing.T) {
	iss := &stubIssuer{err: errors.New("db down")}
	ul := NewUnifiedLogin(UnifiedDeps{Cfg: newTestCfg("token"), Issuer: iss})
	_, err := ul.IssueFor(context.Background(),
		&AuthIdentity{UserID: 3, AccountType: AccountTypeSystem}, "alexgo-admin")
	if !errors.Is(err, ErrIssuance) {
		t.Fatalf("底层错误应 wrap ErrIssuance, got %v", err)
	}
}

func TestAuthorizeLogin_NoIssuance_AuditOnce(t *testing.T) {
	iss := okIssuer()
	aud := &stubAuditor{}
	ul := NewUnifiedLogin(UnifiedDeps{
		Cfg: newTestCfg("token"), Issuer: iss,
		UserAuth: &stubUserAuth{ident: &AuthIdentity{UserID: 3, Username: "alice", AccountType: AccountTypeSystem}},
		Auditor:  aud,
	})
	ident, err := ul.AuthorizeLogin(context.Background(), LoginParams{
		Username: "alice", Password: "pw", ClientID: "alexgo-admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ident.UserID != 3 {
		t.Errorf("ident = %+v", ident)
	}
	if len(iss.calls) != 0 {
		t.Errorf("AuthorizeLogin 不得签发, issuer calls=%d", len(iss.calls))
	}
	if len(aud.success) != 1 || aud.success[0] != true {
		t.Errorf("审计 = %v, want [true] 恰一次", aud.success)
	}
}
