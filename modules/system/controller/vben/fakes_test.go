package vben

import (
	"context"

	"alexGo-cloud/modules/system/model"
	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
	"alexGo-cloud/modules/system/service"
)

// fakeAuth 实现缩编后的 service.AuthService（Login 已下线）：Logout 记账。
type fakeAuth struct {
	logoutCount int
}

func (f *fakeAuth) Refresh(_ context.Context, _ string) (*service.LoginResult, error) {
	return nil, nil
}

func (f *fakeAuth) Logout(_ context.Context, _ string) error {
	f.logoutCount++
	return nil
}

// fakeUnified 实现 oauth2svc.UnifiedLogin：Login 按注入值返回并记账（断言 AccountType/ClientID）。
type fakeUnified struct {
	res    *oauth2svc.LoginResult
	err    error
	logins []oauth2svc.LoginParams
}

func (f *fakeUnified) Login(_ context.Context, p oauth2svc.LoginParams) (*oauth2svc.LoginResult, *oauth2svc.AuthIdentity, error) {
	f.logins = append(f.logins, p)
	if f.err != nil {
		return nil, nil, f.err
	}
	return f.res, &oauth2svc.AuthIdentity{UserID: 7, Username: p.Username, AccountType: p.AccountType}, nil
}
func (f *fakeUnified) AuthorizeLogin(_ context.Context, p oauth2svc.LoginParams) (*oauth2svc.AuthIdentity, error) {
	f.logins = append(f.logins, p)
	if f.err != nil {
		return nil, f.err
	}
	return &oauth2svc.AuthIdentity{UserID: 7, Username: p.Username, AccountType: p.AccountType}, nil
}
func (f *fakeUnified) IssueFor(_ context.Context, _ *oauth2svc.AuthIdentity, _ string) (*oauth2svc.LoginResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.res, nil
}

// fakePerm 实现 service.PermissionService（本阶段 Login 不触达，返回空）。
type fakePerm struct {
	codes  []string
	routes []*service.VbenRoute
	roles  []*model.Role
}

func (f *fakePerm) UserRoles(context.Context, uint64) ([]*model.Role, error) { return f.roles, nil }

func (f *fakePerm) UserMenus(context.Context, uint64) ([]*model.Menu, error) { return nil, nil }

func (f *fakePerm) UserPermCodes(context.Context, uint64) ([]string, error) { return f.codes, nil }

func (f *fakePerm) UserRoutes(context.Context, uint64) ([]*service.VbenRoute, error) {
	return f.routes, nil
}

func (f *fakePerm) EnsureUserRolePolicy(context.Context, string, []*model.Role) error { return nil }

func (f *fakePerm) RebuildPolicies(context.Context) error { return nil }

func (f *fakePerm) RebuildRolePolicies(context.Context, uint64) error { return nil }

// fakeUser 实现 service.UserService：GetUserByID 按注入值返回。
type fakeUser struct {
	user *model.User
	err  error
}

// ListUsers 签名以 service.UserService 接口为准（仅 ctx，无 map 参数）。
func (f *fakeUser) ListUsers(context.Context) ([]*model.User, error) {
	return nil, nil
}

func (f *fakeUser) GetUserByID(context.Context, uint64) (*model.User, error) {
	return f.user, f.err
}

func (f *fakeUser) CreateUser(context.Context, string, string, string) (*model.User, error) {
	return nil, nil
}

func (f *fakeUser) UpdateUser(context.Context, uint64, string, int) error { return nil }

func (f *fakeUser) ResetPassword(context.Context, uint64, string) error { return nil }

func (f *fakeUser) SetRoles(context.Context, uint64, []uint64) error { return nil }
