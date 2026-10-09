package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"

	"alexGo-cloud/modules/member/model"
	"alexGo-cloud/pkg/tenant"
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
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	var all []*model.MemberUser
	for _, u := range m.byMobile {
		if u.TenantID == tid {
			all = append(all, u)
		}
	}
	// 与真实仓储同形：id desc 排序 + offset/limit（分页语义可被断言）。
	sort.Slice(all, func(i, j int) bool { return all[i].ID > all[j].ID })
	total := int64(len(all))
	start := (page - 1) * size
	if start >= len(all) {
		return []*model.MemberUser{}, total, nil
	}
	end := start + size
	if end > len(all) {
		end = len(all)
	}
	return all[start:end], total, nil
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
	svc := NewMemberService(repo, iss, nil) // nil limit：未启用额度检查

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
	svc := NewMemberService(repo, &fakeIssuer{}, nil)
	// 注：brief 原文密码为 "pw"，与服务端 min 8 校验冲突（首个 Register 即失败）；
	// 本测试意图是手机号查重，改用合法密码 pw123456。
	if _, err := svc.Register(context.Background(), "13800000000", "pw123456", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Register(context.Background(), "13800000000", "pw123456", "", ""); err == nil {
		t.Error("duplicate mobile must fail")
	}
}

// 昵称含 ':' 必须拒绝（I2 提权修复）：角色 sub 命名空间为 "{tid}:role:{code}"，
// 若昵称可含 ':'，注册 "role:admin" 会得到用户 sub "{tid}:role:admin"，
// 与管理员角色 sub 撞车 → casbin g(x,x) 恒等 → 继承全部管理员策略。
func TestRegister_NicknameWithColon_Rejected(t *testing.T) {
	repo := newMemRepo()
	svc := NewMemberService(repo, &fakeIssuer{}, nil)
	if _, err := svc.Register(context.Background(), "13800000000", "pw123456", "role:admin", "1.2.3.4"); err == nil {
		t.Error("nickname containing ':' must be rejected")
	}
	// 合法昵称仍可注册。
	if _, err := svc.Register(context.Background(), "13800000001", "pw123456", "role-admin", "1.2.3.4"); err != nil {
		t.Errorf("normal nickname must register: %v", err)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	repo := newMemRepo()
	svc := NewMemberService(repo, &fakeIssuer{}, nil)
	if _, err := svc.Register(context.Background(), "13800000000", "pw123456", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Login(context.Background(), "13800000000", "bad", ""); err == nil {
		t.Error("wrong password must fail")
	}
}

func TestLogout_Revoke(t *testing.T) {
	iss := &fakeIssuer{}
	svc := NewMemberService(newMemRepo(), iss, nil)
	if err := svc.Logout(context.Background(), "ma"); err != nil {
		t.Fatal(err)
	}
	if len(iss.revoked) != 1 {
		t.Errorf("revoked = %v", iss.revoked)
	}
}

// fakeLimit 额度检查器（pkg/tenant.AccountLimitChecker 的测试替身）。
type fakeLimit struct{ err error }

func (f *fakeLimit) CheckAccountLimit(context.Context, uint64) error { return f.err }

// 注册触额必须被拒——额度错误原样返回，且不落库（system 建用户走同一检查）。
func TestRegister_AccountLimitReached(t *testing.T) {
	repo := newMemRepo()
	want := errors.New("account limit reached (2)")
	svc := NewMemberService(repo, &fakeIssuer{}, &fakeLimit{err: want})

	_, err := svc.Register(context.Background(), "13800000000", "pw123456", "", "1.2.3.4")
	if !errors.Is(err, want) {
		t.Fatalf("Register() err = %v, want %v", err, want)
	}
	// 被拒的注册不得落库：同手机号应仍可注册（额度放行后成功）。
	svc2 := NewMemberService(repo, &fakeIssuer{}, &fakeLimit{err: nil})
	if _, err := svc2.Register(context.Background(), "13800000000", "pw123456", "", "1.2.3.4"); err != nil {
		t.Fatalf("Register() under limit error = %v", err)
	}
}

// brokenIssuer 模拟签发侧故障（gRPC 不可达 / issuer 未装配）。
type brokenIssuer struct{ fail bool }

func (b *brokenIssuer) Issue(context.Context, token.IssueParams) (*token.Issued, error) {
	if b.fail {
		return nil, errors.New("grpc unavailable")
	}
	return &token.Issued{AccessToken: "a", RefreshToken: "r", ExpiresIn: 1}, nil
}
func (b *brokenIssuer) Refresh(context.Context, string) (*token.Issued, error) {
	return nil, errors.New("grpc unavailable")
}
func (b *brokenIssuer) Revoke(context.Context, string) error  { return errors.New("grpc unavailable") }
func (b *brokenIssuer) RevokeAll(context.Context, token.UserType, uint64) error {
	return errors.New("grpc unavailable")
}

// I2：凭据校验通过但签发失败 → 错误必须包着 ErrIssuance 哨兵（controller 据此映射 503，
// 而不是把 infra 故障报成 401"密码错误"）。
func TestLogin_IssuanceFailure_WrapsSentinel(t *testing.T) {
	repo := newMemRepo()
	svc := NewMemberService(repo, &brokenIssuer{fail: true}, nil)
	if _, err := svc.Register(context.Background(), "13800000000", "pw123456", "", "1.2.3.4"); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Login(context.Background(), "13800000000", "pw123456", "1.2.3.4")
	if err == nil {
		t.Fatal("login must fail when issuer fails")
	}
	if !errors.Is(err, ErrIssuance) {
		t.Errorf("err = %v, want wrapped ErrIssuance", err)
	}
	// 坏密码是凭据错误，绝不包哨兵（controller 才能区分 401/503）。
	if _, err := svc.Login(context.Background(), "13800000000", "bad-pass", ""); err == nil {
		t.Fatal("wrong password must fail")
	} else if errors.Is(err, ErrIssuance) {
		t.Errorf("bad password must NOT wrap ErrIssuance, got %v", err)
	}
}

// I2：issuer 未装配（nil）同样按签发故障处理 → 哨兵，而非 panic。
func TestLogin_NilIssuer_WrapsSentinel(t *testing.T) {
	repo := newMemRepo()
	svc := NewMemberService(repo, nil, nil)
	if _, err := svc.Register(context.Background(), "13800000000", "pw123456", "", "1.2.3.4"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Login(context.Background(), "13800000000", "pw123456", ""); err == nil {
		t.Fatal("nil issuer must fail")
	} else if !errors.Is(err, ErrIssuance) {
		t.Errorf("err = %v, want wrapped ErrIssuance", err)
	}
}

// I6/T5：List 按租户过滤 + total 汇总（memRepo 只回当前租户的数据）。
func TestList_FiltersByTenant(t *testing.T) {
	repo := newMemRepo()
	svc := NewMemberService(repo, &fakeIssuer{}, nil)
	// 当前 ctx 无租户（tid=0）注册两个。
	if _, err := svc.Register(context.Background(), "13800000000", "pw123456", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Register(context.Background(), "13800000001", "pw123456", "", ""); err != nil {
		t.Fatal(err)
	}
	// 另一租户注册一个：不得出现在 tid=0 的列表里。
	ctxOther := tenant.WithTenantID(context.Background(), 2)
	svcOther := NewMemberService(repo, &fakeIssuer{}, nil)
	if _, err := svcOther.Register(ctxOther, "13800000002", "pw123456", "", ""); err != nil {
		t.Fatal(err)
	}

	items, total, err := svc.List(context.Background(), 1, 20)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) != 2 || total != 2 {
		t.Errorf("List = %d items / total %d, want 2/2 (tenant filter)", len(items), total)
	}
	for _, u := range items {
		if u.TenantID != 0 {
			t.Errorf("cross-tenant leak: %s (tenant %d) in tid=0 list", u.Mobile, u.TenantID)
		}
	}
	// 分页：size=1 → 仅 1 条，total 仍为全量。
	page1, total1, err := svc.List(context.Background(), 1, 1)
	if err != nil {
		t.Fatalf("List(page=1,size=1) error = %v", err)
	}
	if len(page1) != 1 || total1 != 2 {
		t.Errorf("paged List = %d items / total %d, want 1/2", len(page1), total1)
	}
}

// I6/T5：Disable 改状态 + status 边界校验（仅 0/1，其余返回 ErrInvalidStatus）。
func TestDisable_StatusFlipAndBounds(t *testing.T) {
	repo := newMemRepo()
	svc := NewMemberService(repo, &fakeIssuer{}, nil)
	u, err := svc.Register(context.Background(), "13800000000", "pw123456", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Disable(context.Background(), u.ID, 0); err != nil {
		t.Fatalf("Disable(0) error = %v", err)
	}
	if u.Status != 0 {
		t.Errorf("status = %d, want 0 after disable", u.Status)
	}
	// 停用后不可登录（状态位语义）。
	if _, err := svc.Login(context.Background(), "13800000000", "pw123456", ""); err == nil {
		t.Error("disabled member must not login")
	}
	// 重新启用。
	if err := svc.Disable(context.Background(), u.ID, 1); err != nil {
		t.Fatalf("Disable(1) error = %v", err)
	}
	if u.Status != 1 {
		t.Errorf("status = %d, want 1 after enable", u.Status)
	}
	// 边界：非 0/1 一律拒绝且不落库（T5 deferred）。
	for _, bad := range []int{-1, 2, 99} {
		if err := svc.Disable(context.Background(), u.ID, bad); !errors.Is(err, ErrInvalidStatus) {
			t.Errorf("Disable(%d) err = %v, want ErrInvalidStatus", bad, err)
		}
	}
	if u.Status != 1 {
		t.Errorf("status changed to %d by invalid call, want 1", u.Status)
	}
}
