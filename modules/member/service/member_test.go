package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"alexGo-cloud/modules/member/model"
	"alexGo-cloud/pkg/token"
)

type fakeIssuer struct{ revoked []string }

func (f *fakeIssuer) Issue(_ context.Context, p token.IssueParams) (*token.Issued, error) {
	if p.UserType != token.UserTypeMember {
		return nil, errors.New("wrong user type")
	}
	return &token.Issued{AccessToken: "ma", RefreshToken: "mr", ExpiresIn: 7200}, nil
}
func (f *fakeIssuer) Refresh(context.Context, string) (*token.Issued, error) {
	return &token.Issued{AccessToken: "ma2", RefreshToken: "mr2", ExpiresIn: 7200}, nil
}
func (f *fakeIssuer) Revoke(_ context.Context, at string) error {
	f.revoked = append(f.revoked, at)
	return nil
}
func (f *fakeIssuer) RevokeAll(context.Context, token.UserType, uint64) error { return nil }

// memRepo：进程内 map 实现 MemberRepository（测试专用）。
type memRepo struct {
	byMobile map[string]*model.MemberUser
	nextID   uint64
}

func newMemRepo() *memRepo { return &memRepo{byMobile: map[string]*model.MemberUser{}} }

func (m *memRepo) GetByMobile(_ context.Context, tid uint64, mobile string) (*model.MemberUser, error) {
	u, ok := m.byMobile[key(tid, mobile)]
	if !ok {
		return nil, errors.New("not found")
	}
	return u, nil
}
func (m *memRepo) Create(_ context.Context, u *model.MemberUser) error {
	m.nextID++
	u.ID = m.nextID
	m.byMobile[key(u.TenantID, u.Mobile)] = u
	return nil
}
func (m *memRepo) Update(_ context.Context, u *model.MemberUser) error {
	m.byMobile[key(u.TenantID, u.Mobile)] = u
	return nil
}
func (m *memRepo) List(_ context.Context, tid uint64, page, size int) ([]*model.MemberUser, int64, error) {
	var out []*model.MemberUser
	for _, u := range m.byMobile {
		if u.TenantID == tid {
			out = append(out, u)
		}
	}
	return out, int64(len(out)), nil
}
func (m *memRepo) CountByTenant(_ context.Context, tid uint64) (int64, error) {
	var n int64
	for _, u := range m.byMobile {
		if u.TenantID == tid {
			n++
		}
	}
	return n, nil
}

// UpdateStatus 按 tenant+id 局部更新状态（Disable 语义）。
func (m *memRepo) UpdateStatus(_ context.Context, tid, id uint64, status int) error {
	for _, u := range m.byMobile {
		if u.TenantID == tid && u.ID == id {
			u.Status = status
			return nil
		}
	}
	return errors.New("not found")
}

func key(tid uint64, mobile string) string { return fmt.Sprintf("%d:%s", tid, mobile) }

func TestRegister_ThenLogin(t *testing.T) {
	repo := newMemRepo()
	iss := &fakeIssuer{}
	svc := NewMemberService(repo, iss)

	u, err := svc.Register(context.Background(), "13800000000", "pw123456", "", "1.2.3.4")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if u.Mobile != "13800000000" || u.Status != 1 {
		t.Errorf("u = %+v", u)
	}

	res, err := svc.Login(context.Background(), "13800000000", "pw123456", "1.2.3.4")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if res.AccessToken != "ma" || res.ExpiresIn != 7200 {
		t.Errorf("res = %+v", res)
	}
}

func TestRegister_DuplicateMobile(t *testing.T) {
	repo := newMemRepo()
	svc := NewMemberService(repo, &fakeIssuer{})
	// 注：brief 原文密码为 "pw"，与服务端 min 8 校验冲突（首个 Register 即失败）；
	// 本测试意图是手机号查重，改用合法密码 pw123456。
	if _, err := svc.Register(context.Background(), "13800000000", "pw123456", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Register(context.Background(), "13800000000", "pw123456", "", ""); err == nil {
		t.Error("duplicate mobile must fail")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	repo := newMemRepo()
	svc := NewMemberService(repo, &fakeIssuer{})
	if _, err := svc.Register(context.Background(), "13800000000", "pw123456", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Login(context.Background(), "13800000000", "bad", ""); err == nil {
		t.Error("wrong password must fail")
	}
}

func TestLogout_Revoke(t *testing.T) {
	iss := &fakeIssuer{}
	svc := NewMemberService(newMemRepo(), iss)
	if err := svc.Logout(context.Background(), "ma"); err != nil {
		t.Fatal(err)
	}
	if len(iss.revoked) != 1 {
		t.Errorf("revoked = %v", iss.revoked)
	}
}
