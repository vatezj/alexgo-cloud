package auth_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"alexGo-cloud/pkg/auth"
	"alexGo-cloud/pkg/config"
)

func cfgWithSecret(secret string) *config.Config {
	c := &config.Config{}
	c.System.JWTSecret = secret
	return c
}

func TestGenerateParse_Roundtrip(t *testing.T) {
	cfg := cfgWithSecret("test-secret")
	token, err := auth.GenerateToken(42, "alice", 7, cfg)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	// 兼容 "Bearer <token>" 与裸 token 两种输入。
	for _, in := range []string{token, "Bearer " + token} {
		claims, err := auth.ParseToken(in, cfg)
		if err != nil {
			t.Fatalf("ParseToken(%q) error = %v", in, err)
		}
		if claims.UserID != 42 || claims.Username != "alice" || claims.TenantID != 7 {
			t.Errorf("claims = %+v, want {42 alice 7}", claims)
		}
	}
}

func TestGenerateToken_EmptySecret(t *testing.T) {
	if _, err := auth.GenerateToken(1, "u", 1, cfgWithSecret("")); err == nil {
		t.Error("GenerateToken with empty secret should fail")
	}
}

func TestParseToken_EmptyInputs(t *testing.T) {
	cfg := cfgWithSecret("test-secret")
	for _, in := range []string{"", "Bearer ", "   "} {
		if _, err := auth.ParseToken(in, cfg); err == nil {
			t.Errorf("ParseToken(%q) should fail", in)
		}
	}
}

func TestParseToken_WrongSecret(t *testing.T) {
	token, err := auth.GenerateToken(1, "alice", 1, cfgWithSecret("secret-a"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := auth.ParseToken(token, cfgWithSecret("secret-b")); err == nil {
		t.Error("token signed with another secret must be rejected")
	}
}

// 过期 token 必须被拒（伪造一个 exp 已过期的合法签名 token）。
func TestParseToken_Expired(t *testing.T) {
	cfg := cfgWithSecret("test-secret")
	type expClaims struct {
		auth.Claims
		jwt.RegisteredClaims
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, expClaims{
		Claims: auth.Claims{UserID: 1, Username: "alice"},
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	}).SignedString([]byte(cfg.System.JWTSecret))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := auth.ParseToken(signed, cfg); err == nil {
		t.Error("expired token must be rejected")
	}
}

// alg=none 伪造 token 必须被拒（签名算法必须是 HMAC）。
func TestParseToken_AlgNoneRejected(t *testing.T) {
	cfg := cfgWithSecret("test-secret")
	type noneClaims struct {
		auth.Claims
		jwt.RegisteredClaims
	}
	forged, err := jwt.NewWithClaims(jwt.SigningMethodNone, noneClaims{
		Claims: auth.Claims{UserID: 1, Username: "admin"},
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := auth.ParseToken(forged, cfg); err == nil {
		t.Error("alg=none token must be rejected")
	}
}

func TestParseToken_EmptySecretConfig(t *testing.T) {
	if _, err := auth.ParseToken("whatever", cfgWithSecret("")); err == nil {
		t.Error("empty secret config should fail")
	}
}
