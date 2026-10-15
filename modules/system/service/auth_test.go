package service

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"alexGo-cloud/modules/system/model"
	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
	"alexGo-cloud/pkg/auth"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/token"
)

type fakeIssuer struct {
	issued     *token.Issued
	issueErr   error
	issueCalls int // Issue 被调用次数：断言失败路径绝不签发
	revoked    []string
}

func (f *fakeIssuer) Issue(_ context.Context, p token.IssueParams) (*token.Issued, error) {
	f.issueCalls++
	if f.issueErr != nil {
		return nil, f.issueErr
	}
	return f.issued, nil
}
func (f *fakeIssuer) Refresh(context.Context, string) (*token.Issued, error) {
	return f.issued, nil
}
func (f *fakeIssuer) Revoke(_ context.Context, at string) error {
	f.revoked = append(f.revoked, at)
	return nil
}
func (f *fakeIssuer) RevokeAll(context.Context, token.UserType, uint64) error { return nil }

type fakeUserRepo struct{ user *model.User }

func (f *fakeUserRepo) List(context.Context, uint64) ([]*model.User, error) { return nil, nil }
func (f *fakeUserRepo) GetByID(context.Context, uint64, uint64) (*model.User, error) {
	return nil, errors.New("not found")
}
func (f *fakeUserRepo) GetByUsername(_ context.Context, _ uint64, _ string) (*model.User, error) {
	if f.user == nil {
		return nil, errors.New("not found")
	}
	return f.user, nil
}
func (f *fakeUserRepo) Create(context.Context, *model.User) error { return nil }
func (f *fakeUserRepo) Update(context.Context, *model.User) error { return nil }

// fakePerm implements PermissionService zero-values（Login 内仅调用 EnsureUserRolePolicy）。
type fakePerm struct{}

func (fakePerm) UserRoles(context.Context, uint64) ([]*model.Role, error)          { return nil, nil }
func (fakePerm) UserMenus(context.Context, uint64) ([]*model.Menu, error)          { return nil, nil }
func (fakePerm) UserPermCodes(context.Context, uint64) ([]string, error)           { return nil, nil }
func (fakePerm) UserRoutes(context.Context, uint64) ([]*VbenRoute, error)          { return nil, nil }
func (fakePerm) EnsureUserRolePolicy(context.Context, string, []*model.Role) error { return nil }
func (fakePerm) RebuildPolicies(context.Context) error                             { return nil }
func (fakePerm) RebuildRolePolicies(context.Context, uint64) error                 { return nil }

func enabledUser() *model.User {
	return &model.User{ID: 3, Username: "alice", Status: 1, PasswordHash: hashOf("pw")}
}

func hashOf(pw string) string {
	h, _ := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(h)
}

func TestLogout_Revoke(t *testing.T) {
	iss := &fakeIssuer{}
	svc := NewAuthService(&config.Config{}, &fakeUserRepo{}, fakePerm{}, iss)
	if err := svc.Logout(context.Background(), "a1"); err != nil {
		t.Fatal(err)
	}
	if len(iss.revoked) != 1 || iss.revoked[0] != "a1" {
		t.Errorf("revoked = %v", iss.revoked)
	}
}

// TestLogin_JwtRollbackMode：回滚开关端到端——mode=jwt 时统一登录必须签发旧静态
// JWT（auth.ParseToken 可解析），且绝不触碰 token.Issuer；Refresh 快速拒绝、Logout 幂等。
func TestLogin_JwtRollbackMode(t *testing.T) {
	cfg := &config.Config{}
	cfg.Auth.Mode = "jwt"
	cfg.System.JWTSecret = "rollback-secret"
	iss := &fakeIssuer{issued: &token.Issued{AccessToken: "a1", RefreshToken: "r1", ExpiresIn: 7200}}
	ul := oauth2svc.NewUnifiedLogin(oauth2svc.UnifiedDeps{
		Cfg:      cfg,
		Issuer:   iss,
		Captcha:  oauth2svc.NewCaptchaService(oauth2svc.CaptchaDeps{Cfg: cfg}),
		UserAuth: NewUserAuthenticator(&fakeUserRepo{user: enabledUser()}, fakePerm{}),
		JWTGen:   NewJWTGenerator(cfg),
	})

	res, _, err := ul.Login(context.Background(), oauth2svc.LoginParams{
		AccountType: oauth2svc.AccountTypeSystem,
		Username:    "alice",
		Password:    "pw",
		ClientID:    "alexgo-admin",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	// 判别性①：jwt 回滚绝不走不透明令牌签发器。
	if iss.issueCalls != 0 {
		t.Errorf("Issue calls = %d, want 0（jwt rollback must not use token.Issuer）", iss.issueCalls)
	}
	// 判别性②：返回的 token 必须能被中间件 jwt 分支解析且 claims 正确。
	claims, perr := auth.ParseToken(res.AccessToken, cfg)
	if perr != nil {
		t.Fatalf("issued token must parse as legacy static JWT: %v", perr)
	}
	if claims.UserID != 3 || claims.Username != "alice" {
		t.Errorf("claims = %+v", claims)
	}
	// 判别性③：Refresh 快速拒绝 / Logout 幂等 —— 归属未缩的 AuthService。
	svc := NewAuthService(cfg, &fakeUserRepo{}, fakePerm{}, iss)
	if _, rerr := svc.Refresh(context.Background(), "r1"); rerr == nil {
		t.Error("Refresh must be rejected in jwt mode")
	}
	if lerr := svc.Logout(context.Background(), res.AccessToken); lerr != nil {
		t.Errorf("Logout() in jwt mode = %v, want nil", lerr)
	}
}

func TestRefresh_ReturnsLoginResult(t *testing.T) {
	iss := &fakeIssuer{issued: &token.Issued{AccessToken: "a2", RefreshToken: "r2", ExpiresIn: 3600}}
	svc := NewAuthService(&config.Config{}, &fakeUserRepo{}, fakePerm{}, iss)
	res, err := svc.Refresh(context.Background(), "r1")
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if res.AccessToken != "a2" || res.RefreshToken != "r2" || res.ExpiresIn != 3600 {
		t.Errorf("res = %+v", res)
	}
}
