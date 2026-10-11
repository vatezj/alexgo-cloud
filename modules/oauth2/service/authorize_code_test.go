package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/oauth2/model"
	"alexGo-cloud/modules/oauth2/repository"
)

func newTestCodes(t *testing.T) (*AuthorizeCodeService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Oauth2AuthorizeCode{}); err != nil {
		t.Fatal(err)
	}
	return NewAuthorizeCodeService(repository.NewAuthorizeCodeRepository(db)), db
}

func baseCodeParams() IssueCodeParams {
	return IssueCodeParams{
		ClientID: "spa", UserID: 3, Username: "alice",
		AccountType: AccountTypeSystem, TenantID: 0,
		RedirectURI: "http://localhost:5173/callback", Scopes: "all",
	}
}

func TestExchangeCode_OK(t *testing.T) {
	svc, _ := newTestCodes(t)
	code, err := svc.Issue(context.Background(), baseCodeParams())
	if err != nil {
		t.Fatal(err)
	}
	if code == "" {
		t.Fatal("Issue 返回空 code")
	}
	row, err := svc.Exchange(context.Background(), ExchangeParams{
		Code: code, ClientID: "spa", RedirectURI: "http://localhost:5173/callback",
	})
	if err != nil {
		t.Fatal(err)
	}
	if row.UserID != 3 || row.Username != "alice" || row.AccountType != AccountTypeSystem {
		t.Errorf("row = %+v", row)
	}
	// 成功消费后重放 → 拒
	if _, err := svc.Exchange(context.Background(), ExchangeParams{
		Code: code, ClientID: "spa", RedirectURI: "http://localhost:5173/callback",
	}); !errors.Is(err, ErrCodeInvalid) {
		t.Fatalf("重放应 ErrCodeInvalid, got %v", err)
	}
}

func TestExchangeCode_Expired(t *testing.T) {
	svc, db := newTestCodes(t)
	code, err := svc.Issue(context.Background(), baseCodeParams())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Oauth2AuthorizeCode{}).
		Where("code = ?", code).
		Update("expires_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Exchange(context.Background(), ExchangeParams{
		Code: code, ClientID: "spa", RedirectURI: "http://localhost:5173/callback",
	}); !errors.Is(err, ErrCodeInvalid) {
		t.Fatalf("过期应 ErrCodeInvalid, got %v", err)
	}
}

// Review Focus #4：首次尝试即消费——即便首次参数错，第二次全对也拒（CAS）。
func TestExchangeCode_RevokedAfterFirstAttempt(t *testing.T) {
	svc, _ := newTestCodes(t)
	code, err := svc.Issue(context.Background(), baseCodeParams())
	if err != nil {
		t.Fatal(err)
	}
	// 第一次：client 不匹配 → 拒，且已消费
	if _, err := svc.Exchange(context.Background(), ExchangeParams{
		Code: code, ClientID: "other", RedirectURI: "http://localhost:5173/callback",
	}); !errors.Is(err, ErrCodeInvalid) {
		t.Fatalf("client 不匹配应 ErrCodeInvalid, got %v", err)
	}
	// 第二次：参数全对也拒
	if _, err := svc.Exchange(context.Background(), ExchangeParams{
		Code: code, ClientID: "spa", RedirectURI: "http://localhost:5173/callback",
	}); !errors.Is(err, ErrCodeInvalid) {
		t.Fatalf("首错后应已消费, got %v", err)
	}
}

func TestExchangeCode_RedirectMismatch(t *testing.T) {
	svc, _ := newTestCodes(t)
	code, err := svc.Issue(context.Background(), baseCodeParams())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Exchange(context.Background(), ExchangeParams{
		Code: code, ClientID: "spa", RedirectURI: "http://evil.example/steal",
	}); !errors.Is(err, ErrCodeInvalid) {
		t.Fatalf("redirect 不匹配应 ErrCodeInvalid, got %v", err)
	}
}

func TestExchangeCode_PKCE(t *testing.T) {
	svc, _ := newTestCodes(t)
	verifier := "correct-verifier-0123456789"
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	p := baseCodeParams()
	p.CodeChallenge, p.CodeChallengeMethod = challenge, "S256"
	code, err := svc.Issue(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	// 错 verifier（首错即消费，后续全对也拒——本例只验错拒）
	if _, err := svc.Exchange(context.Background(), ExchangeParams{
		Code: code, ClientID: "spa", RedirectURI: "http://localhost:5173/callback",
		CodeVerifier: "wrong-verifier",
	}); !errors.Is(err, ErrCodeInvalid) {
		t.Fatalf("错 verifier 应 ErrCodeInvalid, got %v", err)
	}
	// 空 verifier
	code2, _ := svc.Issue(context.Background(), p)
	if _, err := svc.Exchange(context.Background(), ExchangeParams{
		Code: code2, ClientID: "spa", RedirectURI: "http://localhost:5173/callback",
	}); !errors.Is(err, ErrCodeInvalid) {
		t.Fatalf("空 verifier 应 ErrCodeInvalid, got %v", err)
	}
	// 正确 verifier
	code3, _ := svc.Issue(context.Background(), p)
	if _, err := svc.Exchange(context.Background(), ExchangeParams{
		Code: code3, ClientID: "spa", RedirectURI: "http://localhost:5173/callback",
		CodeVerifier: verifier,
	}); err != nil {
		t.Fatalf("正确 verifier 应过, got %v", err)
	}
	// method 非 S256：Exchange 也拒（authorize 侧已 400，双保险）
	p2 := baseCodeParams()
	p2.CodeChallenge, p2.CodeChallengeMethod = challenge, "plain"
	code4, _ := svc.Issue(context.Background(), p2)
	if _, err := svc.Exchange(context.Background(), ExchangeParams{
		Code: code4, ClientID: "spa", RedirectURI: "http://localhost:5173/callback",
		CodeVerifier: verifier,
	}); !errors.Is(err, ErrCodeInvalid) {
		t.Fatalf("method plain 应 ErrCodeInvalid, got %v", err)
	}
}
