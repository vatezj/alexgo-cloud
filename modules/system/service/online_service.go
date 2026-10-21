package service

import (
	"context"
	"errors"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/repository"
	"alexGo-cloud/pkg/auth"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/token"
)

// ImpersonateClientID 代登录签发的专用 client_id：token 行可辨识、可审计、可单独踢。
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

// Kick / Impersonate：Task 2/3 实现。接口三方法本任务即声明完毕。
func (s *onlineService) Kick(ctx context.Context, operator *auth.Claims, sessionID uint64) error {
	return errors.New("online: Kick not implemented")
}

func (s *onlineService) Impersonate(ctx context.Context, operator *auth.Claims, ip, userAgent string, targetUserID uint64) (*token.Issued, error) {
	return nil, errors.New("online: Impersonate not implemented")
}

var _ = model.LoginLog{} // audit 接口消费方，Task 3 用；先钉 import 防 Task 3 改头部
var _ = errs.Is          // 错误码判定（30201/30202/30203），Task 2/3 用；先钉 import
