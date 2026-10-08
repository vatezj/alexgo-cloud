package token

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/pkg/config"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&accessToken{}); err != nil {
		t.Fatal(err)
	}
	// 夹具补最小维表：Issue 查 tenants.status，Validate 回填查 system_users。
	for _, ddl := range []string{
		`CREATE TABLE tenants (id INTEGER PRIMARY KEY, status INTEGER NOT NULL DEFAULT 1, deleted INTEGER NOT NULL DEFAULT 0)`,
		`INSERT INTO tenants (id, status) VALUES (1, 1), (2, 0)`,
		`CREATE TABLE system_users (id INTEGER PRIMARY KEY, username TEXT, dept_id INTEGER NOT NULL DEFAULT 0, deleted INTEGER NOT NULL DEFAULT 0)`,
		`INSERT INTO system_users (id, username, dept_id) VALUES (9, 'alice', 7)`,
	} {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatal(err)
		}
	}
	cfg := &config.Config{}
	cfg.Auth.AccessExpireHour = 2
	cfg.Auth.RefreshExpireDay = 7
	return NewService(db, cfg)
}

func mustIssue(t *testing.T, s *Service) *Issued {
	t.Helper()
	issued, err := s.Issue(context.Background(), IssueParams{
		UserID: 9, UserType: UserTypeAdmin, TenantID: 1, ClientID: "alexgo-admin",
	})
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	return issued
}

func TestIssue_Validate_Roundtrip(t *testing.T) {
	s := newTestService(t)
	issued := mustIssue(t, s)
	if issued.AccessToken == "" || issued.RefreshToken == "" || issued.ExpiresIn <= 0 {
		t.Fatalf("issued = %+v, want full fields", issued)
	}
	claims, err := s.Validate(context.Background(), issued.AccessToken)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if claims.UserID != 9 || claims.UserType != UserTypeAdmin || claims.TenantID != 1 {
		t.Errorf("claims = %+v", claims)
	}
}

func TestValidate_MissingToken(t *testing.T) {
	s := newTestService(t)
	if _, err := s.Validate(context.Background(), "nope"); err == nil {
		t.Error("unknown token must fail")
	}
}

// 刷新必须轮换：旧 refresh 失效、旧 access 失效、新 token 可用。
func TestRefresh_Rotates(t *testing.T) {
	s := newTestService(t)
	issued := mustIssue(t, s)
	next, err := s.Refresh(context.Background(), issued.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if next.AccessToken == issued.AccessToken || next.RefreshToken == issued.RefreshToken {
		t.Error("refresh must rotate both tokens")
	}
	if _, err := s.Validate(context.Background(), issued.AccessToken); err == nil {
		t.Error("old access must be invalid after refresh")
	}
	if _, err := s.Validate(context.Background(), next.AccessToken); err != nil {
		t.Errorf("new access must validate: %v", err)
	}
	if _, err := s.Refresh(context.Background(), issued.RefreshToken); err == nil {
		t.Error("old refresh must be single-use")
	}
}

// 注销必须同时打掉本地缓存（否则 60s 内仍放行）。
func TestRevoke_InvalidatesCache(t *testing.T) {
	s := newTestService(t)
	issued := mustIssue(t, s)
	if _, err := s.Validate(context.Background(), issued.AccessToken); err != nil {
		t.Fatal(err) // 先填缓存
	}
	if err := s.Revoke(context.Background(), issued.AccessToken); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}
	if _, err := s.Validate(context.Background(), issued.AccessToken); err == nil {
		t.Error("revoked token must fail even with warm cache")
	}
}

// 踢人：撤销该用户全部 token。
func TestRevokeAll(t *testing.T) {
	s := newTestService(t)
	a := mustIssue(t, s)
	b := mustIssue(t, s)
	if err := s.RevokeAll(context.Background(), UserTypeAdmin, 9); err != nil {
		t.Fatalf("RevokeAll() error = %v", err)
	}
	for _, tk := range []string{a.AccessToken, b.AccessToken} {
		if _, err := s.Validate(context.Background(), tk); err == nil {
			t.Error("all user tokens must be revoked")
		}
	}
}

// 过期 token 拒绝（直接改库模拟过期，避免 sleep）。
func TestValidate_Expired(t *testing.T) {
	s := newTestService(t)
	issued := mustIssue(t, s)
	past := time.Now().Add(-time.Minute)
	if err := s.db.Model(&accessToken{}).
		Where("access_token = ?", issued.AccessToken).
		Update("expires_time", past).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.Validate(context.Background(), issued.AccessToken); err == nil {
		t.Error("expired token must fail")
	}
}

// 刷新窗口按 create_time+refreshExpireDay：access 已过期但创建时间在 7 天内 → 仍可刷新。
// （若实现误用 expires_time 校验，本测试必须变红。）
func TestRefresh_AccessExpiredButWithinWindow(t *testing.T) {
	s := newTestService(t)
	issued := mustIssue(t, s)
	past := time.Now().Add(-time.Minute)
	if err := s.db.Model(&accessToken{}).
		Where("access_token = ?", issued.AccessToken).
		Update("expires_time", past).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.Validate(context.Background(), issued.AccessToken); err == nil {
		t.Fatal("access must be invalid")
	}
	next, err := s.Refresh(context.Background(), issued.RefreshToken)
	if err != nil {
		t.Fatalf("refresh must still work within %d-day window: %v", cfgRefreshDays, err)
	}
	if next.AccessToken == "" {
		t.Error("empty new token")
	}
}

const cfgRefreshDays = 7
