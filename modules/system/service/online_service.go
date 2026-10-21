package service

import (
	"context"
	"fmt"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/repository"
	"alexGo-cloud/pkg/auth"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/token"
)

// ImpersonateClientID 代登录签发的专用 client_id：token 行可辨识、可审计；
// 本期踢人端点仅管管理员会话（member 行 Kick → 30203，撤销手段不在本期范围）。
const ImpersonateClientID = "impersonate-admin"

// OnlineSessionVO 列表行（token 明文不出接口，只给行 id）。
type OnlineSessionVO struct {
	ID             uint64 `json:"id"`
	UserID         uint64 `json:"user_id"`
	Username       string `json:"username"`
	ClientID       string `json:"client_id"`
	IsImpersonated bool   `json:"is_impersonated"` // 管理员行恒 false（代登录是 member token）；保留字段供范围扩展
	IssuedAt       string `json:"issued_at"`       // RFC3339
	ExpiresAt      string `json:"expires_at"`
}

type OnlineService interface {
	List(ctx context.Context, page, size int) ([]*OnlineSessionVO, int64, error)
	Kick(ctx context.Context, operator *auth.Claims, sessionID uint64) error
	Impersonate(ctx context.Context, operator *auth.Claims, ip, userAgent string, targetUserID uint64) (*token.Issued, error)
}

type onlineService struct {
	repo   repository.OnlineRepository
	issuer token.Issuer
	audit  repository.AuditRepository
}

func NewOnlineService(repo repository.OnlineRepository, issuer token.Issuer, audit repository.AuditRepository) OnlineService {
	return &onlineService{repo: repo, issuer: issuer, audit: audit}
}

func (s *onlineService) List(ctx context.Context, page, size int) ([]*OnlineSessionVO, int64, error) {
	rows, total, err := s.repo.ListAdmin(ctx, page, size)
	if err != nil {
		return nil, 0, err
	}
	vos := make([]*OnlineSessionVO, 0, len(rows))
	for _, row := range rows {
		vos = append(vos, &OnlineSessionVO{
			ID:             row.ID,
			UserID:         row.UserID,
			Username:       row.Username,
			ClientID:       row.ClientID,
			IsImpersonated: row.ClientID == ImpersonateClientID,
			IssuedAt:       row.IssuedAt.Format("2006-01-02T15:04:05Z07:00"),
			ExpiresAt:      row.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	return vos, total, nil
}

// Kick 踢人：按行 id 吊销。禁止踢自己（防把自己锁外面）；非管理员会话/查无/跨租户 → 30203。
func (s *onlineService) Kick(ctx context.Context, operator *auth.Claims, sessionID uint64) error {
	row, err := s.repo.GetByID(ctx, sessionID)
	if err != nil {
		return err
	}
	if row == nil || row.UserID == 0 {
		return errs.New(errs.CodeSessionNotFound, nil)
	}
	// user_type=1（管理员）之外的行不在本页范围——以「补名查得到 system_users」
	// 不足以判类型，故依赖 Task 1 已投影的 UserType 列。
	if row.UserType != token.UserTypeAdmin {
		return errs.New(errs.CodeSessionNotFound, nil)
	}
	if operator.UserID == row.UserID {
		return errs.New(errs.CodeCannotKickSelf, nil)
	}
	return s.issuer.Revoke(ctx, row.AccessToken)
}

// Impersonate 代登录到会员：签 member token（client_id=impersonate-admin）+ login_logs 审计。
// 停用会员拒签（30201）——给停用账号发 token 等于绕过登录状态门；停用租户由 Issue 内部拒（透传）。
func (s *onlineService) Impersonate(ctx context.Context, operator *auth.Claims, ip, userAgent string, targetUserID uint64) (*token.Issued, error) {
	m, err := s.repo.GetMemberForImpersonate(ctx, targetUserID)
	if err != nil {
		return nil, err
	}
	if m == nil || m.Deleted || m.Status != 1 {
		return nil, errs.New(errs.CodeImpersonateMemberNotFound, nil)
	}
	issued, err := s.issuer.Issue(ctx, token.IssueParams{
		UserID:   m.ID,
		UserType: token.UserTypeMember,
		TenantID: m.TenantID,
		ClientID: ImpersonateClientID,
	})
	if err != nil {
		return nil, err // 租户停用等内部错误透传（与正常登录一致）
	}
	if err := s.audit.CreateLogin(ctx, &model.LoginLog{
		TenantID:  m.TenantID,
		Username:  m.Nickname,
		UserID:    m.ID,
		IP:        ip,
		UserAgent: userAgent,
		Success:   1,
		Message:   fmt.Sprintf("代登录: 管理员%d/%s → 会员%d/%s", operator.UserID, operator.Username, m.ID, m.Nickname),
	}); err != nil {
		return nil, err
	}
	return issued, nil
}
