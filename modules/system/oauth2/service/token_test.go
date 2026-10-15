package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/system/oauth2/model"
	"alexGo-cloud/modules/system/oauth2/repository"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/token"
)

// stubUnified TokenService 视角的 UnifiedLogin 桩：Login 与 IssueFor 共用 res/err，
// 记账供断言（Review Focus #3：账号类型白名单落点）。
type stubUnified struct {
	res    *LoginResult
	ident  *AuthIdentity
	err    error
	logins []LoginParams
	issued []issueCall
}

type issueCall struct {
	ident    *AuthIdentity
	clientID string
}

func (s *stubUnified) Login(_ context.Context, p LoginParams) (*LoginResult, *AuthIdentity, error) {
	s.logins = append(s.logins, p)
	id := s.ident
	if id == nil {
		id = &AuthIdentity{UserID: 3, Username: p.Username, AccountType: p.AccountType}
	}
	return s.res, id, s.err
}

func (s *stubUnified) AuthorizeLogin(_ context.Context, p LoginParams) (*AuthIdentity, error) {
	s.logins = append(s.logins, p)
	if s.err != nil {
		return nil, s.err
	}
	return &AuthIdentity{UserID: 3, Username: p.Username, AccountType: p.AccountType}, nil
}

func (s *stubUnified) IssueFor(_ context.Context, ident *AuthIdentity, clientID string) (*LoginResult, error) {
	s.issued = append(s.issued, issueCall{ident: ident, clientID: clientID})
	return s.res, s.err
}

// refreshIssuer 在 stubIssuer（unified_login_test.go 同包）基础上补 Refresh。
type refreshIssuer struct {
	stubIssuer
	refreshed  *token.Issued
	refreshErr error
	refreshN   int
}

func (s *refreshIssuer) Refresh(_ context.Context, _ string) (*token.Issued, error) {
	s.refreshN++
	if s.refreshErr != nil {
		return nil, s.refreshErr
	}
	return s.refreshed, nil
}

func newTestTokenSvc(t *testing.T, mode string) (*TokenService, *ClientService, *AuthorizeCodeService, *stubUnified, *refreshIssuer) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Oauth2Client{}, &model.Oauth2AuthorizeCode{}); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Auth.Mode = mode
	clients := NewClientService(repository.NewClientRepository(db))
	codes := NewAuthorizeCodeService(repository.NewAuthorizeCodeRepository(db))
	ul := &stubUnified{res: &LoginResult{AccessToken: "a", RefreshToken: "r", ExpiresIn: 60}}
	iss := &refreshIssuer{refreshed: &token.Issued{AccessToken: "a2", RefreshToken: "r2", ExpiresIn: 60}}
	svc := NewTokenService(TokenServiceDeps{Cfg: cfg, Clients: clients, Codes: codes, Login: ul, Issuer: iss})
	return svc, clients, codes, ul, iss
}

func seedAdminClient(t *testing.T, clients *ClientService) {
	t.Helper()
	if err := clients.Create(context.Background(), &model.Oauth2Client{
		ClientID:     "alexgo-admin",
		GrantTypes:   "authorization_code,password,refresh_token",
		AccountTypes: "system",
		RedirectURIs: "http://localhost:5173/callback",
	}); err != nil {
		t.Fatal(err)
	}
}

func TestExchange_Password_OK(t *testing.T) {
	svc, clients, _, ul, _ := newTestTokenSvc(t, "token")
	seedAdminClient(t, clients)
	res, perr := svc.Exchange(context.Background(), &TokenRequest{
		GrantType: GrantPassword, ClientID: "alexgo-admin",
		Username: "alice", Password: "pw",
	})
	if perr != nil {
		t.Fatalf("perr = %v", perr)
	}
	if res.AccessToken != "a" || res.TokenType != "Bearer" {
		t.Errorf("res = %+v", res)
	}
	if len(ul.logins) != 1 {
		t.Fatalf("logins = %d", len(ul.logins))
	}
	lp := ul.logins[0]
	if lp.AccountType != AccountTypeSystem || lp.ClientID != "alexgo-admin" {
		t.Errorf("login params = %+v", lp)
	}
}

// Review Focus #3：client 账号类型白名单——admin client 换不了 member token。
func TestExchange_Password_AccountTypeNotAllowed(t *testing.T) {
	svc, clients, _, ul, _ := newTestTokenSvc(t, "token")
	seedAdminClient(t, clients)
	res, perr := svc.Exchange(context.Background(), &TokenRequest{
		GrantType: GrantPassword, ClientID: "alexgo-admin",
		Username: "13800000000", Password: "pw", AccountType: AccountTypeMember,
	})
	if res != nil {
		t.Errorf("应拒, res = %+v", res)
	}
	if perr == nil || perr.Status != 400 || perr.Code != "unauthorized_client" {
		t.Fatalf("perr = %+v", perr)
	}
	if len(ul.logins) != 0 {
		t.Errorf("白名单拒后不得进登录, logins=%d", len(ul.logins))
	}
}

func TestExchange_Password_UnsupportedGrant(t *testing.T) {
	svc, clients, _, _, _ := newTestTokenSvc(t, "token")
	if err := clients.Create(context.Background(), &model.Oauth2Client{
		ClientID: "code-only", GrantTypes: "authorization_code", AccountTypes: "system",
		RedirectURIs: "http://localhost:5173/callback",
	}); err != nil {
		t.Fatal(err)
	}
	_, perr := svc.Exchange(context.Background(), &TokenRequest{
		GrantType: GrantPassword, ClientID: "code-only", Username: "a", Password: "b",
	})
	if perr == nil || perr.Status != 400 || perr.Code != "unsupported_grant_type" {
		t.Fatalf("perr = %+v", perr)
	}
}

func TestExchange_ClientAuth_Unknown_401(t *testing.T) {
	svc, _, _, _, _ := newTestTokenSvc(t, "token")
	res, perr := svc.Exchange(context.Background(), &TokenRequest{
		GrantType: GrantPassword, ClientID: "nope", Username: "a", Password: "b",
	})
	if res != nil {
		t.Errorf("res = %+v", res)
	}
	if perr == nil || perr.Status != 401 || perr.Code != "invalid_client" {
		t.Fatalf("perr = %+v", perr)
	}
}

// Review Focus #3：公开客户端禁机机凭证。
func TestExchange_ClientCredentials_Public_401(t *testing.T) {
	svc, clients, _, _, _ := newTestTokenSvc(t, "token")
	if err := clients.Create(context.Background(), &model.Oauth2Client{
		ClientID: "pub-m2m", GrantTypes: "client_credentials", AccountTypes: "system",
	}); err != nil {
		t.Fatal(err)
	}
	_, perr := svc.Exchange(context.Background(), &TokenRequest{
		GrantType: GrantClientCredentials, ClientID: "pub-m2m",
	})
	if perr == nil || perr.Status != 401 || perr.Code != "invalid_client" {
		t.Fatalf("perr = %+v", perr)
	}
}

func TestExchange_ClientCredentials_OK(t *testing.T) {
	svc, clients, _, ul, _ := newTestTokenSvc(t, "token")
	if err := clients.Create(context.Background(), &model.Oauth2Client{
		ClientID: "m2m", ClientSecret: "s3cret",
		GrantTypes: "client_credentials", AccountTypes: "system",
	}); err != nil {
		t.Fatal(err)
	}
	res, perr := svc.Exchange(context.Background(), &TokenRequest{
		GrantType: GrantClientCredentials, ClientID: "m2m", ClientSecret: "s3cret",
	})
	if perr != nil {
		t.Fatalf("perr = %v", perr)
	}
	if res.AccessToken != "a" || res.TokenType != "Bearer" {
		t.Errorf("res = %+v", res)
	}
	if len(ul.issued) != 1 {
		t.Fatalf("issued = %d", len(ul.issued))
	}
	if ul.issued[0].ident.AccountType != AccountTypeClient || ul.issued[0].ident.Username != "m2m" {
		t.Errorf("ident = %+v", ul.issued[0].ident)
	}
}

func TestExchange_Refresh_OK(t *testing.T) {
	svc, clients, _, _, iss := newTestTokenSvc(t, "token")
	seedAdminClient(t, clients)
	res, perr := svc.Exchange(context.Background(), &TokenRequest{
		GrantType: GrantRefreshToken, ClientID: "alexgo-admin", RefreshToken: "old-r",
	})
	if perr != nil {
		t.Fatalf("perr = %v", perr)
	}
	if res.AccessToken != "a2" || res.RefreshToken != "r2" || iss.refreshN != 1 {
		t.Errorf("res=%+v refreshN=%d", res, iss.refreshN)
	}
}

func TestExchange_Refresh_JwtMode_400(t *testing.T) {
	svc, clients, _, _, _ := newTestTokenSvc(t, "jwt")
	seedAdminClient(t, clients)
	_, perr := svc.Exchange(context.Background(), &TokenRequest{
		GrantType: GrantRefreshToken, ClientID: "alexgo-admin", RefreshToken: "old-r",
	})
	if perr == nil || perr.Status != 400 || perr.Code != "invalid_grant" {
		t.Fatalf("perr = %+v", perr)
	}
}

// T6 缺口收口：空 refresh_token 必须拒（防空串命中 client 行的空 refresh_token 值）。
func TestExchange_Refresh_EmptyToken_400(t *testing.T) {
	svc, clients, _, _, iss := newTestTokenSvc(t, "token")
	seedAdminClient(t, clients)
	_, perr := svc.Exchange(context.Background(), &TokenRequest{
		GrantType: GrantRefreshToken, ClientID: "alexgo-admin", RefreshToken: "",
	})
	if perr == nil || perr.Status != 400 || perr.Code != "invalid_grant" {
		t.Fatalf("perr = %+v", perr)
	}
	if iss.refreshN != 0 {
		t.Errorf("空 token 不得触达 Issuer.Refresh, refreshN=%d", iss.refreshN)
	}
}

func TestExchange_AuthCode_OK(t *testing.T) {
	svc, clients, codes, ul, _ := newTestTokenSvc(t, "token")
	seedAdminClient(t, clients)
	code, err := codes.Issue(context.Background(), IssueCodeParams{
		ClientID: "alexgo-admin", UserID: 3, Username: "alice",
		AccountType: AccountTypeSystem, TenantID: 0,
		RedirectURI: "http://localhost:5173/callback", Scopes: "all",
	})
	if err != nil {
		t.Fatal(err)
	}
	res, perr := svc.Exchange(context.Background(), &TokenRequest{
		GrantType: GrantAuthorizationCode, ClientID: "alexgo-admin",
		Code: code, RedirectURI: "http://localhost:5173/callback",
	})
	if perr != nil {
		t.Fatalf("perr = %v", perr)
	}
	if res.AccessToken != "a" || res.TokenType != "Bearer" {
		t.Errorf("res = %+v", res)
	}
	if len(ul.issued) != 1 {
		t.Fatalf("issued = %d", len(ul.issued))
	}
	id := ul.issued[0].ident
	if id.UserID != 3 || id.Username != "alice" || id.AccountType != AccountTypeSystem {
		t.Errorf("ident = %+v", id)
	}
	if ul.issued[0].clientID != "alexgo-admin" {
		t.Errorf("clientID = %q", ul.issued[0].clientID)
	}
}

func TestExchange_AuthCode_BadCode_400(t *testing.T) {
	svc, clients, _, _, _ := newTestTokenSvc(t, "token")
	seedAdminClient(t, clients)
	_, perr := svc.Exchange(context.Background(), &TokenRequest{
		GrantType: GrantAuthorizationCode, ClientID: "alexgo-admin",
		Code: "bogus", RedirectURI: "http://localhost:5173/callback",
	})
	if perr == nil || perr.Status != 400 || perr.Code != "invalid_grant" {
		t.Fatalf("perr = %+v", perr)
	}
}

// Review Focus #3（授权码支）：账号类型白名单同样拦 authorization_code 换 token——
// member 授权码 + system-only client → 400 unauthorized_client，且不得签发。
// CAS 语义：Exchange 的 MarkRevoked 先于白名单检查，故白名单失败当次码即被消费，
// 重放必须 400 invalid_grant（钉死「消费不回滚」的实际行为）。
func TestExchange_AuthCode_AccountTypeNotAllowed(t *testing.T) {
	svc, clients, codes, ul, _ := newTestTokenSvc(t, "token")
	seedAdminClient(t, clients) // AccountTypes: "system"
	redirect := "http://localhost:5173/callback"
	code, err := codes.Issue(context.Background(), IssueCodeParams{
		ClientID: "alexgo-admin", UserID: 5, Username: "13800000001",
		AccountType: AccountTypeMember, TenantID: 0,
		RedirectURI: redirect, Scopes: "all",
	})
	if err != nil {
		t.Fatal(err)
	}
	req := &TokenRequest{
		GrantType: GrantAuthorizationCode, ClientID: "alexgo-admin",
		Code: code, RedirectURI: redirect,
	}

	res, perr := svc.Exchange(context.Background(), req)
	if res != nil {
		t.Errorf("应拒, res = %+v", res)
	}
	if perr == nil || perr.Status != 400 || perr.Code != "unauthorized_client" {
		t.Fatalf("perr = %+v, want 400 unauthorized_client", perr)
	}
	if len(ul.issued) != 0 {
		t.Errorf("白名单拒后不得签发, issued = %d", len(ul.issued))
	}

	// 码已在白名单检查前被 CAS 消费：重放 → invalid_grant（而非再次 unauthorized_client）。
	_, perr2 := svc.Exchange(context.Background(), req)
	if perr2 == nil || perr2.Status != 400 || perr2.Code != "invalid_grant" {
		t.Fatalf("重放 perr = %+v, want 400 invalid_grant", perr2)
	}
}

func TestLoginProtoErr_Mapping(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{ErrInvalidCredentials, 400, "invalid_grant"},
		{ErrUserDisabled, 400, "invalid_grant"},
		{ErrCaptchaInvalid, 400, "captcha_invalid"},
		{ErrIssuance, 500, "server_error"},
		{ErrAuthenticatorMissing, 500, "server_error"},
		{ErrAccountTypeUnsupported, 400, "invalid_request"},
		{errors.New("boom"), 500, "server_error"},
	}
	for _, c := range cases {
		perr := LoginProtoErr(c.err)
		if perr.Status != c.status || perr.Code != c.code {
			t.Errorf("LoginProtoErr(%v) = %+v, want %d/%s", c.err, perr, c.status, c.code)
		}
	}
	// wrap 链也要命中（IssueFor 的 fmt.Errorf("%w: ...")）
	wrapped := fmt.Errorf("%w: issuer down", ErrIssuance)
	if p := LoginProtoErr(wrapped); p.Status != 500 || p.Code != "server_error" {
		t.Errorf("wrap 链 = %+v", p)
	}
}
