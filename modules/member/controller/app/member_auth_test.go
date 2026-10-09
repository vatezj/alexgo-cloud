package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"alexGo-cloud/modules/member/model"
	"alexGo-cloud/modules/member/repository"
	"alexGo-cloud/modules/member/service"
	"alexGo-cloud/pkg/token"
)

// stubRepo：Login 只需要 GetByMobile 命中一个启用账号（真实 bcrypt 哈希，保证密码分支可达）。
type stubRepo struct{ user *model.MemberUser }

func (s *stubRepo) GetByMobile(context.Context, uint64, string) (*model.MemberUser, error) {
	return s.user, nil
}
func (s *stubRepo) Create(context.Context, *model.MemberUser) error { return nil }
func (s *stubRepo) Update(context.Context, *model.MemberUser) error { return nil }
func (s *stubRepo) List(context.Context, uint64, int, int) ([]*model.MemberUser, int64, error) {
	return nil, 0, nil
}
func (s *stubRepo) CountByTenant(context.Context, uint64) (int64, error) { return 0, nil }
func (s *stubRepo) UpdateStatus(context.Context, uint64, uint64, int) error {
	return nil
}

var _ repository.MemberRepository = (*stubRepo)(nil)

// badIssuer 模拟签发故障（gRPC 不可达 / 熔断）。
type badIssuer struct{}

func (badIssuer) Issue(context.Context, token.IssueParams) (*token.Issued, error) {
	return nil, context.DeadlineExceeded
}
func (badIssuer) Refresh(context.Context, string) (*token.Issued, error) {
	return nil, context.DeadlineExceeded
}
func (badIssuer) Revoke(context.Context, string) error { return context.DeadlineExceeded }
func (badIssuer) RevokeAll(context.Context, token.UserType, uint64) error {
	return context.DeadlineExceeded
}

// okIssuer 正常签发（成功路径用）。
type okIssuer struct{}

func (okIssuer) Issue(context.Context, token.IssueParams) (*token.Issued, error) {
	return &token.Issued{AccessToken: "a", RefreshToken: "r", ExpiresIn: 7200}, nil
}
func (okIssuer) Refresh(context.Context, string) (*token.Issued, error) {
	return &token.Issued{AccessToken: "a2", RefreshToken: "r2", ExpiresIn: 7200}, nil
}
func (okIssuer) Revoke(context.Context, string) error                 { return nil }
func (okIssuer) RevokeAll(context.Context, token.UserType, uint64) error { return nil }

func enabledRepo(t *testing.T) *stubRepo {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("pw123456"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return &stubRepo{user: &model.MemberUser{ID: 1, Mobile: "13800000000", Status: 1, Password: string(hash)}}
}

// doLogin 走 httptest 完整路由（复刻 module 注册的 POST /api/app/member/auth/login）。
func doLogin(t *testing.T, svc service.MemberService, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	c := NewAuthController(svc)
	r.POST("/api/app/member/auth/login", c.Login)
	req := httptest.NewRequest(http.MethodPost, "/api/app/member/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// I2：issuer 未装配（nil）→ 503 auth service unavailable（不是 401）。
func TestLogin_NilIssuer_503(t *testing.T) {
	svc := service.NewMemberService(enabledRepo(t), nil, nil)
	w := doLogin(t, svc, `{"mobile":"13800000000","password":"pw123456"}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body=%s", w.Code, w.Body.String())
	}
	if w.Body.String() != `{"error":"auth service unavailable"}` {
		t.Errorf("body = %s", w.Body.String())
	}
}

// I2：坏 issuer（gRPC 故障）→ 503。
func TestLogin_BadIssuer_503(t *testing.T) {
	svc := service.NewMemberService(enabledRepo(t), badIssuer{}, nil)
	w := doLogin(t, svc, `{"mobile":"13800000000","password":"pw123456"}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body=%s", w.Code, w.Body.String())
	}
}

// I2：坏密码是凭据错误 → 401（与 503 判别：签发故障绝不冒充密码错误，反之亦然）。
func TestLogin_BadPassword_401(t *testing.T) {
	svc := service.NewMemberService(enabledRepo(t), badIssuer{}, nil)
	w := doLogin(t, svc, `{"mobile":"13800000000","password":"wrong-pass"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body=%s", w.Code, w.Body.String())
	}
}

// 正常签发 → 200 且返回 token 三元组。
func TestLogin_OK_200(t *testing.T) {
	svc := service.NewMemberService(enabledRepo(t), okIssuer{}, nil)
	w := doLogin(t, svc, `{"mobile":"13800000000","password":"pw123456"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"token":"a"`) {
		t.Errorf("body = %s, want token payload", w.Body.String())
	}
}
