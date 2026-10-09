package vben

import (
	"context"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/service"
)

// fakeAuth 实现 service.AuthService：Login 按注入值返回；Logout 记账。
type fakeAuth struct {
	loginRes    *service.LoginResult
	loginErr    error
	logoutCount int
}

func (f *fakeAuth) Login(_ context.Context, _, _ string) (*service.LoginResult, *model.User, error) {
	if f.loginErr != nil {
		return nil, nil, f.loginErr
	}
	return f.loginRes, &model.User{ID: 7, Username: "admin"}, nil
}

func (f *fakeAuth) Refresh(_ context.Context, _ string) (*service.LoginResult, error) {
	return nil, nil
}

func (f *fakeAuth) Logout(_ context.Context, _ string) error {
	f.logoutCount++
	return nil
}

// fakeAudit 实现 service.AuditService：记录 RecordLogin 的 success 序列。
type fakeAudit struct {
	successes []bool
}

func (f *fakeAudit) RecordLogin(_ context.Context, _ string, _ uint64, _, _ string, success bool, _ string) error {
	f.successes = append(f.successes, success)
	return nil
}

func (f *fakeAudit) RecordOperate(context.Context, uint64, string, string, string, int, int64, string) error {
	return nil
}

func (f *fakeAudit) ListLogin(context.Context, int) ([]*model.LoginLog, error) { return nil, nil }

func (f *fakeAudit) ListOperate(context.Context, int) ([]*model.OperateLog, error) { return nil, nil }

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
