package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/repository"
	"alexGo-cloud/pkg/auth"
	"alexGo-cloud/pkg/config"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/tenant/gormplugin"
	"alexGo-cloud/pkg/token"
)

// newOnlineTestEnv：sqlite 内存库 + gormplugin 注册（跨租户测试依赖插件注入）。
// 注意：插件只在 ctx 带 tenant_id 时生效（tenant.WithTenantID）。
func newOnlineTestEnv(t *testing.T) (OnlineService, *gorm.DB, *token.Service) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := gormplugin.Register(db, gormplugin.Options{}); err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&repository.OnlineSession{}, &repository.MemberBrief{}, &model.LoginLog{}); err != nil {
		t.Fatal(err)
	}
	for _, ddl := range []string{
		`CREATE TABLE tenants (id INTEGER PRIMARY KEY, status INTEGER NOT NULL DEFAULT 1, deleted INTEGER NOT NULL DEFAULT 0)`,
		`INSERT INTO tenants (id, status) VALUES (1, 1), (2, 1), (3, 0)`,
		`CREATE TABLE system_users (id INTEGER PRIMARY KEY, username TEXT, dept_id INTEGER NOT NULL DEFAULT 0, deleted INTEGER NOT NULL DEFAULT 0)`,
		`INSERT INTO system_users (id, username, dept_id) VALUES (9, 'alice', 7), (10, 'bob', 0), (88, 'ghost', 0)`,
		`UPDATE system_users SET deleted = 1 WHERE id = 88`,
		// OnlineSession 是列表投影（不带 refresh_token/scopes/deleted），AutoMigrate 建出的表
		// 缺这三列：token.Issue 必写 refresh_token/scopes，ListAdmin 过滤需 deleted。
		// 生产 DDL（migrations/20261008000003_oauth2_access_token.up.sql）三列齐备，此处补齐对齐。
		`ALTER TABLE system_oauth2_access_token ADD COLUMN refresh_token TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE system_oauth2_access_token ADD COLUMN scopes TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE system_oauth2_access_token ADD COLUMN deleted INTEGER NOT NULL DEFAULT 0`,
	} {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatal(err)
		}
	}
	cfg := &config.Config{}
	cfg.Auth.AccessExpireHour = 2
	cfg.Auth.RefreshExpireDay = 7
	ts := token.NewService(db, cfg)
	svc := NewOnlineService(repository.NewOnlineRepository(db), ts, repository.NewAuditRepository(db))
	return svc, db, ts
}

// ctxTid 带租户上下文（gormplugin 注入依据）。
func ctxTid(tid uint64) context.Context {
	return tenant.WithTenantID(context.Background(), tid)
}

func mustIssueOnline(t *testing.T, ts *token.Service, tid, uid uint64, ut token.UserType) *token.Issued {
	t.Helper()
	issued, err := ts.Issue(ctxTid(tid), token.IssueParams{UserID: uid, UserType: ut, TenantID: tid, ClientID: "vben-web"})
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	return issued
}

func TestOnlineService_List_OnlyAdminAndAlive(t *testing.T) {
	svc, db, ts := newOnlineTestEnv(t)
	// tid=1 管理员会话 ×2、会员会话 ×1（不得出现）、已过期管理员会话 ×1（不得出现）
	mustIssueOnline(t, ts, 1, 9, token.UserTypeAdmin)
	mustIssueOnline(t, ts, 1, 10, token.UserTypeAdmin)
	mustIssueOnline(t, ts, 1, 88, token.UserTypeAdmin) // 已删管理员：行可见、username 空串
	mustIssueOnline(t, ts, 1, 9, token.UserTypeMember)
	past := time.Now().Add(-time.Hour)
	if err := db.WithContext(ctxTid(1)).Exec(
		`INSERT INTO system_oauth2_access_token (user_id, user_type, access_token, refresh_token, client_id, scopes, expires_time, create_time, tenant_id)
		 VALUES (9, 1, 'expired-token', 'r', 'vben-web', 'all', ?, ?, 1)`, past, time.Now()).Error; err != nil {
		t.Fatal(err)
	}
	// tid=2 的管理员会话（tid=1 不可见）
	mustIssueOnline(t, ts, 2, 10, token.UserTypeAdmin)

	vos, total, err := svc.List(ctxTid(1), 1, 2)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	// 期望可见：tid=1 未过期管理员会话 3 条（含已删用户 ghost 行）
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(vos) != 2 {
		t.Fatalf("len(vos) = %d, want 2 (size=2)", len(vos))
	}
	// username 补查：ghost 行留空
	sawGhost := false
	for _, vo := range vos {
		if vo.UserID == 88 {
			sawGhost = true
			if vo.Username != "" {
				t.Errorf("deleted user username = %q, want empty", vo.Username)
			}
		}
	}
	if !sawGhost {
		t.Error("deleted user row (uid=88) missing from page 1")
	}
}

func TestOnlineService_Kick_OK(t *testing.T) {
	svc, _, ts := newOnlineTestEnv(t)
	issued := mustIssueOnline(t, ts, 1, 9, token.UserTypeAdmin)
	op := &auth.Claims{UserID: 99, Username: "root", TenantID: 1, UserType: 1}
	// 找到该 token 的行 id
	var row repository.OnlineSession
	_ = row
	// 经 List 拿行 id（token 明文不进 service API）
	vos, _, err := svc.List(ctxTid(1), 1, 100)
	if err != nil || len(vos) != 1 {
		t.Fatalf("List() = %d rows, err %v", len(vos), err)
	}
	if err := svc.Kick(ctxTid(1), op, vos[0].ID); err != nil {
		t.Fatalf("Kick() error = %v", err)
	}
	if _, err := ts.Validate(ctxTid(1), issued.AccessToken); err == nil {
		t.Error("kicked token must fail Validate immediately")
	}
	// 踢完列表空
	_, total, _ := svc.List(ctxTid(1), 1, 10)
	if total != 0 {
		t.Errorf("total after kick = %d, want 0", total)
	}
}

func TestOnlineService_Kick_Self(t *testing.T) {
	svc, _, ts := newOnlineTestEnv(t)
	mustIssueOnline(t, ts, 1, 9, token.UserTypeAdmin)
	vos, _, _ := svc.List(ctxTid(1), 1, 10)
	op := &auth.Claims{UserID: 9, TenantID: 1, UserType: 1}
	err := svc.Kick(ctxTid(1), op, vos[0].ID)
	if !errs.Is(err, errs.CodeCannotKickSelf) {
		t.Errorf("Kick(self) err = %v, want 30202", err)
	}
}

func TestOnlineService_Kick_CrossTenant(t *testing.T) {
	svc, _, ts := newOnlineTestEnv(t)
	mustIssueOnline(t, ts, 2, 10, token.UserTypeAdmin) // tid=2 的会话
	// tid=1 视角：List 看不见；按 id 直踢也必须 30203（插件过滤 → GetByID 查无）
	// 先用 tid=2 自己 List 拿真实行 id
	vos, _, err := svc.List(ctxTid(2), 1, 10)
	if err != nil || len(vos) != 1 {
		t.Fatalf("List(tid=2) = %d, err %v", len(vos), err)
	}
	op := &auth.Claims{UserID: 9, TenantID: 1, UserType: 1}
	if err := svc.Kick(ctxTid(1), op, vos[0].ID); !errs.Is(err, errs.CodeSessionNotFound) {
		t.Errorf("cross-tenant Kick err = %v, want 30203", err)
	}
}

func TestOnlineService_Kick_RaceDeleted(t *testing.T) {
	svc, db, ts := newOnlineTestEnv(t)
	mustIssueOnline(t, ts, 1, 9, token.UserTypeAdmin)
	vos, _, _ := svc.List(ctxTid(1), 1, 10)
	if err := db.WithContext(ctxTid(1)).Exec(`DELETE FROM system_oauth2_access_token WHERE id = ?`, vos[0].ID).Error; err != nil {
		t.Fatal(err)
	}
	op := &auth.Claims{UserID: 99, TenantID: 1, UserType: 1}
	if err := svc.Kick(ctxTid(1), op, vos[0].ID); !errs.Is(err, errs.CodeSessionNotFound) {
		t.Errorf("race Kick err = %v, want 30203", err)
	}
}

func seedMember(t *testing.T, db *gorm.DB, id uint64, tid uint64, status int, deleted bool) {
	t.Helper()
	d := 0
	if deleted {
		d = 1
	}
	// member_user 由 MemberBrief AutoMigrate 产出，无 create_time/update_time 列（brief 原 SQL 需适配）。
	if err := db.Exec(
		`INSERT INTO member_user (id, nickname, status, deleted, tenant_id) VALUES (?, ?, ?, ?, ?)`,
		id, "会员"+string(rune('0'+id%10)), status, d, tid).Error; err != nil {
		t.Fatal(err)
	}
}

func TestOnlineService_Impersonate_OK(t *testing.T) {
	svc, db, ts := newOnlineTestEnv(t)
	seedMember(t, db, 501, 1, 1, false)
	op := &auth.Claims{UserID: 9, Username: "root", TenantID: 1, UserType: 1}
	issued, err := svc.Impersonate(ctxTid(1), op, "127.0.0.1", "test-ua", 501)
	if err != nil {
		t.Fatalf("Impersonate() error = %v", err)
	}
	claims, err := ts.Validate(ctxTid(1), issued.AccessToken)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if claims.UserID != 501 || claims.UserType != token.UserTypeMember || claims.TenantID != 1 {
		t.Errorf("claims = %+v", claims)
	}
	// token 行 client_id 可辨
	var row repository.OnlineSession
	if err := db.WithContext(ctxTid(1)).Where("access_token = ?", issued.AccessToken).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.ClientID != ImpersonateClientID {
		t.Errorf("client_id = %q, want %q", row.ClientID, ImpersonateClientID)
	}
	// login_logs 审计行
	var log model.LoginLog
	if err := db.Where("user_id = ?", 501).Order("id DESC").First(&log).Error; err != nil {
		t.Fatalf("login log not written: %v", err)
	}
	if log.IP != "127.0.0.1" || log.UserAgent != "test-ua" || log.Success != 1 {
		t.Errorf("login log = %+v", log)
	}
	if !strings.Contains(log.Message, "root") || !strings.Contains(log.Message, "501") {
		t.Errorf("login log message = %q, want operator+target", log.Message)
	}
}

func TestOnlineService_Impersonate_MissingOrDisabled(t *testing.T) {
	svc, db, _ := newOnlineTestEnv(t)
	seedMember(t, db, 502, 1, 0, false) // 停用
	seedMember(t, db, 503, 1, 1, true)  // 已删
	op := &auth.Claims{UserID: 9, TenantID: 1, UserType: 1}
	if err := mustImpersonateErr(svc, ctxTid(1), op, 501); !errs.Is(err, errs.CodeImpersonateMemberNotFound) {
		t.Errorf("missing member err = %v, want 30201", err)
	}
	if err := mustImpersonateErr(svc, ctxTid(1), op, 502); !errs.Is(err, errs.CodeImpersonateMemberNotFound) {
		t.Errorf("disabled member err = %v, want 30201", err)
	}
	if err := mustImpersonateErr(svc, ctxTid(1), op, 503); !errs.Is(err, errs.CodeImpersonateMemberNotFound) {
		t.Errorf("deleted member err = %v, want 30201", err)
	}
}

func mustImpersonateErr(svc OnlineService, ctx context.Context, op *auth.Claims, uid uint64) error {
	_, err := svc.Impersonate(ctx, op, "", "", uid)
	return err
}

func TestOnlineService_Impersonate_CrossTenantMemberRejected(t *testing.T) {
	svc, db, _ := newOnlineTestEnv(t)
	seedMember(t, db, 504, 2, 1, false) // tid=2 的会员
	op := &auth.Claims{UserID: 9, TenantID: 1, UserType: 1}
	if err := mustImpersonateErr(svc, ctxTid(1), op, 504); !errs.Is(err, errs.CodeImpersonateMemberNotFound) {
		t.Errorf("cross-tenant member err = %v, want 30201", err)
	}
}

func TestOnlineService_Impersonate_DisabledTenant(t *testing.T) {
	svc, db, _ := newOnlineTestEnv(t)
	seedMember(t, db, 505, 3, 1, false) // tid=3 租户停用（夹具 tenants 3=status 0）
	op := &auth.Claims{UserID: 9, TenantID: 3, UserType: 1}
	_, err := svc.Impersonate(ctxTid(3), op, "", "", 505)
	if err == nil {
		t.Fatal("disabled tenant must fail")
	}
	if errs.Is(err, errs.CodeImpersonateMemberNotFound) || errs.Is(err, errs.CodeSessionNotFound) || errs.Is(err, errs.CodeCannotKickSelf) {
		t.Errorf("tenant-disabled err = %v, want transparent non-3020x", err)
	}
}
