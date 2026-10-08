package grpcserver

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/system/api/rpc"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/token"
)

// TestTokenServiceImpl_IssueValidate 手工 DDL 走完整 Issue→Validate 端到端：
// token.accessToken 模型未导出（无法跨包 AutoMigrate），故按 Task 1 表结构建表。
// 列与模型对齐：模型有 create_time（GORM INSERT 必带），DDL 必须提供该列。
func TestTokenServiceImpl_IssueValidate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, ddl := range []string{
		`CREATE TABLE tenants (id INTEGER PRIMARY KEY, status INTEGER, deleted INTEGER DEFAULT 0)`,
		`INSERT INTO tenants (id, status) VALUES (1, 1)`,
		`CREATE TABLE system_oauth2_access_token (
			id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER, user_type INTEGER,
			access_token TEXT UNIQUE, refresh_token TEXT UNIQUE, client_id TEXT, scopes TEXT,
			expires_time DATETIME, create_time DATETIME, deleted INTEGER DEFAULT 0, tenant_id INTEGER)`,
		`CREATE TABLE system_users (id INTEGER PRIMARY KEY, username TEXT, dept_id INTEGER, deleted INTEGER DEFAULT 0)`,
		`INSERT INTO system_users (id, username, dept_id) VALUES (7, 'alice', 3)`,
	} {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatal(err)
		}
	}
	cfg := &config.Config{}
	cfg.Auth.AccessExpireHour = 2
	cfg.Auth.RefreshExpireDay = 7
	svc := token.NewService(db, cfg)
	impl := NewTokenServiceImpl(svc)

	resp, err := impl.IssueToken(context.Background(), &rpc.IssueTokenRequest{
		UserId: 7, UserType: 1, TenantId: 1, ClientId: "alexgo-admin",
	})
	if err != nil {
		t.Fatalf("IssueToken() error = %v", err)
	}
	if resp.GetAccessToken() == "" || resp.GetExpiresIn() != 7200 {
		t.Errorf("resp = %+v, want access_token 与 2h expires_in", resp)
	}
	claims, err := svc.Validate(context.Background(), resp.GetAccessToken())
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if claims.UserID != 7 || claims.Username != "alice" || claims.DeptID != 3 {
		t.Errorf("claims = %+v", claims)
	}
}
