package service

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"alexGo-cloud/modules/system/model"
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

func enabledUser() *model.User {
	return &model.User{ID: 3, Username: "alice", Status: 1, PasswordHash: hashOf("pw")}
}

func hashOf(pw string) string {
	h, _ := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(h)
}

func TestLogin_IssuesToken(t *testing.T) {
	cfg := &config.Config{}
	cfg.Auth.Mode = "token"
	iss := &fakeIssuer{issued: &token.Issued{AccessToken: "a1", RefreshToken: "r1", ExpiresIn: 7200}}
	svc := NewAuthService(cfg, &fakeUserRepo{user: enabledUser()}, fakePerm{}, iss)

	res, u, err := svc.Login(context.Background(), "alice", "pw")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if res.AccessToken != "a1" || res.RefreshToken != "r1" || res.ExpiresIn != 7200 {
		t.Errorf("res = %+v", res)
	}
	if u.ID != 3 {
		t.Errorf("user = %+v", u)
	}
	if iss.issueCalls != 1 {
		t.Errorf("Issue calls = %d, want 1", iss.issueCalls)
	}
}

func TestLogin_BadPassword_NoIssue(t *testing.T) {
	cfg := &config.Config{}
	iss := &fakeIssuer{issued: &token.Issued{AccessToken: "a1", RefreshToken: "r1", ExpiresIn: 7200}}
	svc := NewAuthService(cfg, &fakeUserRepo{user: enabledUser()}, fakePerm{}, iss)
	if _, _, err := svc.Login(context.Background(), "alice", "wrong"); err == nil {
		t.Fatal("wrong password must fail")
	}
	// 判别性断言：密码错误绝不允许触碰签发器。
	if iss.issueCalls != 0 {
		t.Errorf("Issue calls = %d, want 0（must not issue on bad password）", iss.issueCalls)
	}
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
