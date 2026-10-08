package token

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"

	"alexGo-cloud/pkg/config"
)

// accessToken 映射表 system_oauth2_access_token（一期不启用软删，deleted 仅占位）。
type accessToken struct {
	ID           uint64    `gorm:"primaryKey"`
	UserID       uint64    `gorm:"column:user_id"`
	UserType     UserType  `gorm:"column:user_type"`
	AccessToken  string    `gorm:"column:access_token;size:255"`
	RefreshToken string    `gorm:"column:refresh_token;size:32"`
	ClientID     string    `gorm:"column:client_id;size:64"`
	Scopes       string    `gorm:"column:scopes;size:255"`
	ExpiresTime  time.Time `gorm:"column:expires_time"`
	CreatedAt    time.Time `gorm:"column:create_time"` // 刷新窗口基准（表 DEFAULT CURRENT_TIMESTAMP）
	TenantID     uint64    `gorm:"column:tenant_id"`
}

func (accessToken) TableName() string { return "system_oauth2_access_token" }

// cacheEntry：进程内缓存（60s TTL）。为什么需要缓存：校验是每请求路径，
// 直查 DB 会让鉴权延迟绑定 DB；注销/刷新时同步删除对应 key 保证不放行已吊销 token。
type cacheEntry struct {
	claims   *Claims
	expireAt time.Time
}

type Service struct {
	db    *gorm.DB
	cfg   *config.Config
	cache sync.Map // access_token -> cacheEntry
}

// NewService 构造 Service；*Service 同时实现 Issuer 与 Validator。
func NewService(db *gorm.DB, cfg *config.Config) *Service {
	return &Service{db: db, cfg: cfg}
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *Service) accessExpire() time.Duration {
	h := s.cfg.Auth.AccessExpireHour
	if h <= 0 {
		h = 2
	}
	return time.Duration(h) * time.Hour
}

func (s *Service) refreshExpire() time.Duration {
	d := s.cfg.Auth.RefreshExpireDay
	if d <= 0 {
		d = 7
	}
	return time.Duration(d) * 24 * time.Hour
}

// Issue 签发令牌。tenant_id=0（平台租户/未解析）跳过租户状态检查；
// 租户存在且停用则拒绝签发——这是 SaaS 停用租户的第一道闸。
func (s *Service) Issue(ctx context.Context, p IssueParams) (*Issued, error) {
	if p.UserID == 0 || p.UserType == 0 {
		return nil, fmt.Errorf("token: invalid issue params")
	}
	if p.TenantID != 0 {
		var status int
		err := s.db.WithContext(ctx).
			Raw("SELECT status FROM tenants WHERE id = ? AND deleted = 0", p.TenantID).
			Scan(&status).Error
		if err != nil {
			return nil, fmt.Errorf("token: tenant lookup: %w", err)
		}
		if status != 1 {
			return nil, fmt.Errorf("token: tenant disabled or not found")
		}
	}
	access, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	refresh, err := randomToken(24) // 32 字符以内（列宽 varchar(32)，base64url 24字节→32字符）
	if err != nil {
		return nil, err
	}
	row := accessToken{
		UserID:       p.UserID,
		UserType:     p.UserType,
		AccessToken:  access,
		RefreshToken: refresh,
		ClientID:     p.ClientID,
		Scopes:       "all",
		ExpiresTime:  time.Now().Add(s.accessExpire()),
		TenantID:     p.TenantID,
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, err
	}
	return &Issued{AccessToken: access, RefreshToken: refresh, ExpiresIn: int64(s.accessExpire().Seconds())}, nil
}

// Validate 查缓存→查库→回填缓存。username/dept_id 从对应用户表加载
//（raw SQL 避免 pkg 依赖 modules 层），用户不存在视为无效。
func (s *Service) Validate(ctx context.Context, accessTokenStr string) (*Claims, error) {
	if accessTokenStr == "" {
		return nil, errors.New("token: empty")
	}
	if v, ok := s.cache.Load(accessTokenStr); ok {
		e := v.(cacheEntry)
		if time.Now().Before(e.expireAt) {
			return e.claims, nil
		}
		s.cache.Delete(accessTokenStr)
	}
	var row accessToken
	err := s.db.WithContext(ctx).
		Where("access_token = ?", accessTokenStr).
		First(&row).Error
	if err != nil {
		return nil, fmt.Errorf("token: invalid")
	}
	if time.Now().After(row.ExpiresTime) {
		return nil, fmt.Errorf("token: expired")
	}
	claims := &Claims{
		UserID:   row.UserID,
		UserType: row.UserType,
		TenantID: row.TenantID,
	}
	switch row.UserType {
	case UserTypeAdmin:
		var u struct {
			Username string
			DeptID   uint64 `gorm:"column:dept_id"`
		}
		if err := s.db.WithContext(ctx).
			Raw("SELECT username, dept_id FROM system_users WHERE id = ? AND deleted = 0", row.UserID).
			Scan(&u).Error; err != nil || u.Username == "" {
			return nil, fmt.Errorf("token: user not found")
		}
		claims.Username, claims.DeptID = u.Username, u.DeptID
	case UserTypeMember:
		var nickname string
		if err := s.db.WithContext(ctx).
			Raw("SELECT nickname FROM member_user WHERE id = ? AND deleted = 0", row.UserID).
			Scan(&nickname).Error; err != nil || nickname == "" {
			return nil, fmt.Errorf("token: user not found")
		}
		claims.Username = nickname
	default:
		return nil, fmt.Errorf("token: unknown user type")
	}
	// 缓存 TTL 60s，但不越过 token 自身过期时刻。
	ttl := 60 * time.Second
	if left := time.Until(row.ExpiresTime); left < ttl {
		ttl = left
	}
	s.cache.Store(accessTokenStr, cacheEntry{claims: claims, expireAt: time.Now().Add(ttl)})
	return claims, nil
}

// Refresh 轮换：刷新窗口 = create_time + refreshExpireDay（不是 access 的 expires_time——
// access 过期后 7 天内仍可刷新，与 SweepExpired 的 7 天保留期对齐），旧 refresh 单次使用。
func (s *Service) Refresh(ctx context.Context, refreshToken string) (*Issued, error) {
	var row accessToken
	err := s.db.WithContext(ctx).Where("refresh_token = ?", refreshToken).First(&row).Error
	if err != nil || time.Since(row.CreatedAt) > s.refreshExpire() {
		return nil, fmt.Errorf("token: invalid refresh token")
	}
	// 单次使用：旧 access 立即失效（清缓存 + 删行），旧 refresh 随行删除一并作废。
	s.cache.Delete(row.AccessToken)
	if derr := s.db.WithContext(ctx).Where("id = ?", row.ID).Delete(&accessToken{}).Error; derr != nil {
		return nil, derr
	}
	return s.Issue(ctx, IssueParams{
		UserID: row.UserID, UserType: row.UserType,
		TenantID: row.TenantID, ClientID: row.ClientID,
	})
}

func (s *Service) Revoke(ctx context.Context, accessTokenStr string) error {
	s.cache.Delete(accessTokenStr)
	return s.db.WithContext(ctx).
		Where("access_token = ?", accessTokenStr).
		Delete(&accessToken{}).Error
}

func (s *Service) RevokeAll(ctx context.Context, userType UserType, userID uint64) error {
	var tokens []string
	if err := s.db.WithContext(ctx).
		Model(&accessToken{}).
		Where("user_id = ? AND user_type = ?", userID, userType).
		Pluck("access_token", &tokens).Error; err != nil {
		return err
	}
	for _, tk := range tokens {
		s.cache.Delete(tk)
	}
	return s.db.WithContext(ctx).
		Where("user_id = ? AND user_type = ?", userID, userType).
		Delete(&accessToken{}).Error
}

// SweepExpired 清理过期超过 7 天的行（由调用方以 ticker 驱动，见 Task 11 注册）。
func (s *Service) SweepExpired(ctx context.Context) error {
	return s.db.WithContext(ctx).
		Where("expires_time < ?", time.Now().Add(-7*24*time.Hour)).
		Delete(&accessToken{}).Error
}
