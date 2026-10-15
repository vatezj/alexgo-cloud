package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"alexGo-cloud/modules/system/oauth2/model"
	"alexGo-cloud/modules/system/oauth2/repository"
)

// CodeTTL 授权码有效期 2 分钟（D15 补充语义）。
const CodeTTL = 2 * time.Minute

// ErrCodeInvalid 授权码过期/已消费/client 或 redirect 不匹配/PKCE 失败——
// 统一对外 400 invalid_grant（不区分原因，防探测）。
var ErrCodeInvalid = errors.New("authorize code invalid")

type IssueCodeParams struct {
	ClientID            string
	UserID              uint64
	Username            string
	AccountType         string
	TenantID            uint64
	RedirectURI         string
	Scopes              string
	CodeChallenge       string
	CodeChallengeMethod string
}

type ExchangeParams struct {
	Code         string
	ClientID     string
	RedirectURI  string
	CodeVerifier string
}

type AuthorizeCodeService struct {
	repo *repository.AuthorizeCodeRepository
}

func NewAuthorizeCodeService(repo *repository.AuthorizeCodeRepository) *AuthorizeCodeService {
	return &AuthorizeCodeService{repo: repo}
}

// Issue 生成授权码：32B crypto/rand → base64url（无填充，D15 补充语义）。
func (s *AuthorizeCodeService) Issue(ctx context.Context, p IssueCodeParams) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("authorize code rand: %w", err)
	}
	code := base64.RawURLEncoding.EncodeToString(buf)
	row := &model.Oauth2AuthorizeCode{
		Code:                code,
		ClientID:            p.ClientID,
		UserID:              p.UserID,
		Username:            p.Username,
		AccountType:         p.AccountType,
		OwnerTenantID:       p.TenantID,
		RedirectURI:         p.RedirectURI,
		Scopes:              p.Scopes,
		CodeChallenge:       p.CodeChallenge,
		CodeChallengeMethod: p.CodeChallengeMethod,
		ExpiresAt:           time.Now().Add(CodeTTL),
		Revoked:             false,
	}
	if err := s.repo.Create(ctx, row); err != nil {
		return "", fmt.Errorf("authorize code store: %w", err)
	}
	return code, nil
}

// Exchange 换发校验。顺序固定：
// 载入 → **MarkRevoked 先行**（false → ErrCodeInvalid；首次尝试即消费，无论后续校验对错，
// CAS 单次消费，D15 补充语义）→ 过期 → client 归属 → redirect 精确匹配 → PKCE。
// 任何失败统一 ErrCodeInvalid。
func (s *AuthorizeCodeService) Exchange(ctx context.Context, p ExchangeParams) (*model.Oauth2AuthorizeCode, error) {
	row, err := s.repo.GetByCode(ctx, p.Code)
	if err != nil || row == nil {
		return nil, ErrCodeInvalid
	}
	ok, err := s.repo.MarkRevoked(ctx, p.Code)
	if err != nil {
		return nil, fmt.Errorf("authorize code revoke: %w", err)
	}
	if !ok {
		return nil, ErrCodeInvalid // 已被首次尝试消费（含并发重放）
	}
	if time.Now().After(row.ExpiresAt) {
		return nil, ErrCodeInvalid
	}
	if row.ClientID != p.ClientID {
		return nil, ErrCodeInvalid
	}
	if row.RedirectURI != p.RedirectURI || p.RedirectURI == "" {
		return nil, ErrCodeInvalid // 精确匹配，失效/篡改一律拒（D15 补充语义）
	}
	if row.CodeChallenge != "" {
		if row.CodeChallengeMethod != "S256" {
			return nil, ErrCodeInvalid
		}
		if p.CodeVerifier == "" {
			return nil, ErrCodeInvalid
		}
		sum := sha256.Sum256([]byte(p.CodeVerifier))
		if base64.RawURLEncoding.EncodeToString(sum[:]) != row.CodeChallenge {
			return nil, ErrCodeInvalid
		}
	}
	return row, nil
}
