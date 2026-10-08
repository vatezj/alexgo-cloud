package service

import (
	"context"
	"time"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/repository"
	"alexGo-cloud/pkg/tenant"
)

type AuditService interface {
	RecordLogin(ctx context.Context, username string, userID uint64, ip string, ua string, success bool, msg string) error
	RecordOperate(ctx context.Context, userID uint64, username string, method string, path string, status int, latencyMs int64, errMsg string) error
	ListLogin(ctx context.Context, limit int) ([]*model.LoginLog, error)
	ListOperate(ctx context.Context, limit int) ([]*model.OperateLog, error)
}

type auditService struct {
	repo repository.AuditRepository
}

func NewAuditService(repo repository.AuditRepository) AuditService {
	return &auditService{repo: repo}
}

func (s *auditService) RecordLogin(ctx context.Context, username string, userID uint64, ip string, ua string, success bool, msg string) error {
	tid := tenant.TenantIDFromContext(ctx)
	ok := 0
	if success {
		ok = 1
	}
	return s.repo.CreateLogin(ctx, &model.LoginLog{
		TenantID:  tid,
		Username:  username,
		UserID:    userID,
		IP:        ip,
		UserAgent: ua,
		Success:   ok,
		Message:   msg,
		CreatedAt: time.Now(),
	})
}

func (s *auditService) RecordOperate(ctx context.Context, userID uint64, username string, method string, path string, status int, latencyMs int64, errMsg string) error {
	tid := tenant.TenantIDFromContext(ctx)
	return s.repo.CreateOperate(ctx, &model.OperateLog{
		TenantID:  tid,
		UserID:    userID,
		Username:  username,
		Method:    method,
		Path:      path,
		Status:    status,
		LatencyMs: latencyMs,
		Error:     errMsg,
		CreatedAt: time.Now(),
	})
}

func (s *auditService) ListLogin(ctx context.Context, limit int) ([]*model.LoginLog, error) {
	return s.repo.ListLogin(ctx, tenant.TenantIDFromContext(ctx), limit)
}

func (s *auditService) ListOperate(ctx context.Context, limit int) ([]*model.OperateLog, error) {
	return s.repo.ListOperate(ctx, tenant.TenantIDFromContext(ctx), limit)
}

