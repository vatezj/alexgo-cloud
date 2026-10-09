package token

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
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

// Service 不做进程内缓存：校验恒查库（yudao 同款）。
// 为什么不用 TTL 缓存：① cache-aside 存在 TOCTOU——Validate 读库后、写缓存前，
// 可与 Revoke 交错把已吊销 token 重新写回；② 多进程部署时（member-server 校验、
// system-server 踢人），进程内缓存让"踢人立即失效"最长延迟 60s，违背 spec 验收项。
// 单表唯一索引点查的代价可接受，正确性优先。
type Service struct {
	db  *gorm.DB
	cfg *config.Config
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

// Validate 查库校验（DB 为权威，无进程内缓存）。username/dept_id 从对应用户表加载
//（raw SQL 避免 pkg 依赖 modules 层），用户不存在视为无效。
func (s *Service) Validate(ctx context.Context, accessTokenStr string) (*Claims, error) {
	if accessTokenStr == "" {
		return nil, errors.New("token: empty")
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
	// I3：停用租户的存量 token 必须失效——Issue 时的租户检查只拦"停用后新签发"，
	// 停用前签发的 token 仍会通过上面的查询；这里按校验时点复查（与 Issue 同款 raw SQL）。
	// tenant_id=0（平台租户/未解析）与 Issue 对齐，跳过检查。
	if row.TenantID != 0 {
		var status int
		if err := s.db.WithContext(ctx).
			Raw("SELECT status FROM tenants WHERE id = ? AND deleted = 0", row.TenantID).
			Scan(&status).Error; err != nil {
			return nil, fmt.Errorf("token: tenant lookup: %w", err)
		}
		if status != 1 {
			return nil, fmt.Errorf("token: tenant disabled")
		}
	}
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
	// 单次使用：旧 access 立即失效（删行），旧 refresh 随行删除一并作废。
	if derr := s.db.WithContext(ctx).Where("id = ?", row.ID).Delete(&accessToken{}).Error; derr != nil {
		return nil, derr
	}
	return s.Issue(ctx, IssueParams{
		UserID: row.UserID, UserType: row.UserType,
		TenantID: row.TenantID, ClientID: row.ClientID,
	})
}

func (s *Service) Revoke(ctx context.Context, accessTokenStr string) error {
	return s.db.WithContext(ctx).
		Where("access_token = ?", accessTokenStr).
		Delete(&accessToken{}).Error
}

func (s *Service) RevokeAll(ctx context.Context, userType UserType, userID uint64) error {
	return s.db.WithContext(ctx).
		Where("user_id = ? AND user_type = ?", userID, userType).
		Delete(&accessToken{}).Error
}

// SweepExpired 清理过期超过 7 天的行（由调用方以 ticker 驱动，见 Task 11 注册）。
func (s *Service) SweepExpired(ctx context.Context) error {
	// 保留期与刷新窗口对齐（refreshExpire()，默认 7 天）：行存活期间 refresh 可用，
	// 超窗后才清扫——改 refresh_expire_day 配置时两者同步漂移。
	return s.db.WithContext(ctx).
		Where("expires_time < ?", time.Now().Add(-s.refreshExpire())).
		Delete(&accessToken{}).Error
}
