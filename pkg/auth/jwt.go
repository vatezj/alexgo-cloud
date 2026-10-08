package auth

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"alexGo-cloud/pkg/config"
)

// jwtClaims 是 JWT 的完整 claims 结构：
// - 业务字段：Claims（user_id/username/tenant_id）
// - 标准字段：jwt.RegisteredClaims（exp/iat 等）
type jwtClaims struct {
	Claims
	jwt.RegisteredClaims
}

// GenerateToken 生成 JWT（HS256）。
//
// 生产建议：
// - secret 存储在安全介质（K8s Secret/外部密钥系统），不要写入仓库。
// - 可增加 issuer/audience/jti 等字段。
func GenerateToken(userID uint64, username string, tenantID uint64, cfg *config.Config) (string, error) {
	if cfg == nil || cfg.System.JWTSecret == "" {
		return "", fmt.Errorf("jwt secret is empty")
	}
	now := time.Now()
	claims := jwtClaims{
		Claims: Claims{
			UserID:   userID,
			Username: username,
			TenantID: tenantID,
		},
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.System.JWTSecret))
}

// ParseToken 解析并校验 JWT，返回业务 Claims。
//
// 输入兼容：
// - "Bearer <token>"
// - "<token>"
//
// 校验点：
// - 签名算法必须是 HMAC
// - exp/iat 等由 jwt 库基于 RegisteredClaims 校验
func ParseToken(tokenStr string, cfg *config.Config) (*Claims, error) {
	if cfg == nil || cfg.System.JWTSecret == "" {
		return nil, fmt.Errorf("jwt secret is empty")
	}
	tokenStr = strings.TrimSpace(tokenStr)
	tokenStr = strings.TrimPrefix(tokenStr, "Bearer ")
	tokenStr = strings.TrimSpace(tokenStr)
	if tokenStr == "" {
		return nil, fmt.Errorf("token is empty")
	}

	var out jwtClaims
	_, err := jwt.ParseWithClaims(tokenStr, &out, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(cfg.System.JWTSecret), nil
	})
	if err != nil {
		return nil, err
	}
	return &out.Claims, nil
}
