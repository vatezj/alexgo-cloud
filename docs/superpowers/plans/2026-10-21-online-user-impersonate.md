# 在线用户 + 代登录（P1-3）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 管理员在线会话分页列表 + 踢人（Revoke 立即失效）+ 代登录到会员（专用 client_id 标记 + login_logs 审计），含前端页面与菜单种子。

**Architecture:** 全部落 system 模块：`modules/system/repository/online_repo.go`（token 表分页 + member_user 点查）→ `modules/system/service/online_service.go`（List/Kick/Impersonate，注入 `token.Issuer`）→ `modules/system/controller/admin/online_controller.go`（三端点统一信封）。租户隔离靠既有 `pkg/tenant/gormplugin`（模型带 TenantID 字段即自动注入），**禁止 JOIN**（插件追加的非限定 `tenant_id` 在 JOIN 下会歧义），username 用分批补查。

**Tech Stack:** Go 1.x + gin + gorm（glebarez/sqlite 内存库测试）+ casbin 权限三链路；前端 vben antd（Table 服务端真分页，照 ScorecardRoomsPage 范式）。

**Spec:** `docs/superpowers/specs/2026-10-21-online-user-impersonate-design.md`

## Global Constraints

- 直接落 main，不建功能分支，不 push。
- 统一信封 `resp.OK/Fail`；分页 `resp.PageReq`（size≤100）+ `resp.Page`；错误码走 `pkg/errors`（1xxxx 参数 / 2xxxx 认证 / 3xxxx 业务 / 5xxxx 系统）。
- 新增业务错误码：**30201/30202/30203**（registry 中文文案写死）。
- 代登录 token 标记：`client_id = "impersonate-admin"`（包常量 `ImpersonateClientID`）。
- access_token 明文**不出接口**（列表只回行 id；踢人按行 id）。
- 禁词红线（scorecard 终审教训，本仓库全 diff 适用）：代码/文案不得出现 bet/cash/rake/money/钱/赌注/抽成/赌。
- 新增 admin 路由必须同步 `permissionRoutes`（缺登记 = 全 403）。
- 前端权限码经 `v-access:code` 精确匹配菜单 permission（页面 `system:online:*` 三码）。
- 回归门禁：`go test ./...` 全绿 + `pnpm typecheck` 绿。
- 根 `pnpm lint` 在本机（Node 22.14）不可用（engines 卡 22.18，roadmap E-3 已知）——单文件用 `npx eslint` 替代，不作为本计划门禁。

## Review Focus

以下五类输入/条件最可能咬到人，评审时优先核对拥有它的任务的测试：

1. **分页 Count 在「username 分批补查」拆分后是否仍准**（token 行分页 total 只数 token 表）→ Task 1 `TestOnlineService_List` 断言 total=2（页内只回 1 条时 total 仍为 2）。
2. **踢人竞态**（行刚被 refresh 轮换删除或 sweep 清理）→ Task 2 `TestOnlineService_Kick_RaceDeleted`：插行后手工删行再 Kick → 30203。
3. **跨租户防线**（A 租户管理员踢/看不到 B 租户会话；对 B 租户会员代登录）→ Task 2 `TestOnlineService_Kick_CrossTenant`、Task 3 `TestOnlineService_Impersonate_CrossTenantMemberRejected`：ctx 带 tid=1，目标行/会员 tid=2 → 30203/30201（gormplugin 自动过滤；测试用**裸 sqlite 时无插件**，故这两个测试必须用 `gormplugin.Register` 注册后的 db——见 Task 1 测试夹具说明）。
4. **停用租户/停用会员**（Issue 内部拒停用租户；停用会员不得签 token）→ Task 3 `TestOnlineService_Impersonate_DisabledMember`（status=0 → 30201）与 `TestOnlineService_Impersonate_DisabledTenant`（tenants.status=0 → 非 nil err 且非 3020x 业务码，透传）。
5. **权限三链路缺一即 403**（permissionRoutes 漏登记 / migration 漏按钮 / v-access 码不一致）→ Task 4 `permission_routes_test.go` 既有机制自动兜底新键（跑测试确认）；Task 5 migration 注释钉死三码 `system:online:list/kick/impersonate` 与 `permissionRoutes["system:online"]` 同源；Task 6 页面 `v-access` 字符串与之一致。

---

### Task 1: OnlineRepository + OnlineService.List（含错误码与测试夹具）

**Files:**
- Modify: `pkg/errors/code.go`（常量 + registry）
- Create: `modules/system/repository/online_repo.go`
- Create: `modules/system/service/online_service.go`
- Test: `modules/system/service/online_service_test.go`

**Interfaces:**
- Consumes: `pkg/token`（`UserType`、`Issuer`）、`pkg/repo.Paginate`、`pkg/tenant/gormplugin`（测试注册）、`pkg/errors`
- Produces（后续任务依赖，签名必须一致）:
  - `repository.OnlineRepository`（接口：`ListAdmin(ctx, page, size int) ([]*OnlineSession, int64, error)`、`GetByID(ctx, id uint64) (*OnlineSession, error)`、`GetMemberForImpersonate(ctx, userID uint64) (*MemberBrief, error)`）
  - `repository.OnlineSession`（gorm struct，表 `system_oauth2_access_token`：`ID/UserID/UserType/ClientID/AccessToken/IssuedAt(create_time)/ExpiresAt(expires_time)/TenantID` + `Username string` 非列字段）
  - `repository.MemberBrief`（表 `member_user`：`ID/Nickname/Status/TenantID/Deleted`）
  - `service.OnlineService` 接口（本任务先实现 `List`；`Kick`/`Impersonate` 在 Task 2/3 加进接口——**本任务就把三方法写进接口声明**，未实现的方法返回 `errors.New("not implemented")` 占位，避免 Task 2/3 改接口引发连锁 diff）
  - `service.ImpersonateClientID = "impersonate-admin"`（包常量）
  - `service.OnlineSessionVO`（json：`id,user_id,username,client_id,is_impersonated,issued_at,expires_at`）

- [ ] **Step 1: 加错误码**

`pkg/errors/code.go` 常量块 3xxxx 段末尾（`CodeScorecardTargetNotMember` 之后）追加：

```go
// CodeImpersonateMemberNotFound online：代登录目标会员不存在/已删除/已停用。
CodeImpersonateMemberNotFound Code = 30201
// CodeCannotKickSelf online：不能踢出自己的会话。
CodeCannotKickSelf Code = 30202
// CodeSessionNotFound online：在线会话不存在（或已非管理员会话）。
CodeSessionNotFound Code = 30203
```

registry 追加：

```go
CodeImpersonateMemberNotFound: "目标会员不存在或不可用",
CodeCannotKickSelf:            "不能踢出自己的会话",
CodeSessionNotFound:           "会话不存在",
```

- [ ] **Step 2: 写 repository（online_repo.go）**

```go
package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"alexGo-cloud/pkg/repo"
	"alexGo-cloud/pkg/token"
)

// OnlineSession 投影：只取列表/踢人需要的列，不取 refresh_token（最小暴露）。
// Username 非表列，由 ListAdmin 分批补查填充。
type OnlineSession struct {
	ID          uint64         `gorm:"column:id" json:"id"`
	UserID      uint64         `gorm:"column:user_id" json:"user_id"`
	UserType    token.UserType `gorm:"column:user_type" json:"user_type"` // Task 2 Kick 校验 user_type=1 依赖此列投影
	ClientID    string         `gorm:"column:client_id" json:"client_id"`
	AccessToken string         `gorm:"column:access_token" json:"-"`
	IssuedAt    time.Time      `gorm:"column:create_time" json:"issued_at"`
	ExpiresAt   time.Time      `gorm:"column:expires_time" json:"expires_at"`
	TenantID    uint64         `gorm:"column:tenant_id" json:"tenant_id"`
	Username    string         `gorm:"-" json:"username"` // 分批补查，非表列
}

func (OnlineSession) TableName() string { return "system_oauth2_access_token" }

// MemberBrief 代登录目标的最小画像（member_user 表，system 侧只读点查，
// 照 tenant.go CountAccounts 的 raw Table 先例——不 import modules/member）。
type MemberBrief struct {
	ID       uint64 `gorm:"column:id"`
	Nickname string `gorm:"column:nickname"`
	Status   int    `gorm:"column:status"` // 1启用 0停用
	Deleted  bool   `gorm:"column:deleted"`
	TenantID uint64 `gorm:"column:tenant_id"`
}

func (MemberBrief) TableName() string { return "member_user" }

type OnlineRepository interface {
	// ListAdmin 本租户未过期管理员会话分页（gormplugin 自动追加 tenant_id）。
	ListAdmin(ctx context.Context, page, size int) ([]*OnlineSession, int64, error)
	// GetByID 按行 id 点查（含过期行——踢过期行等价清理）；跨租户经插件过滤后查无。
	GetByID(ctx context.Context, id uint64) (*OnlineSession, error)
	// GetMemberForImpersonate 代登录目标点查（含已删/停用行，交 service 判定）。
	GetMemberForImpersonate(ctx context.Context, userID uint64) (*MemberBrief, error)
}

type onlineRepo struct{ db *gorm.DB }

func NewOnlineRepository(db *gorm.DB) OnlineRepository {
	return &onlineRepo{db: db}
}

func (r *onlineRepo) ListAdmin(ctx context.Context, page, size int) ([]*OnlineSession, int64, error) {
	// 禁止 JOIN：gormplugin 追加的非限定 tenant_id 在 JOIN system_users 时列名歧义。
	// 先分页 token 行（Count 只数本表，准确），再按页内 user_id 批量补 username。
	q := r.db.WithContext(ctx).Model(&OnlineSession{}).
		Where("user_type = ? AND expires_time > ? AND deleted = ?", 1, time.Now(), false).
		Order("id DESC")
	rows, total, err := repo.Paginate[OnlineSession](ctx, q, page, size)
	if err != nil || len(rows) == 0 {
		return rows, total, err
	}
	ids := make([]uint64, 0, len(rows))
	seen := map[uint64]bool{}
	for _, row := range rows {
		if !seen[row.UserID] {
			seen[row.UserID] = true
			ids = append(ids, row.UserID)
		}
	}
	var users []struct {
		ID       uint64 `gorm:"column:id"`
		Username string `gorm:"column:username"`
	}
	if err := r.db.WithContext(ctx).Model(&struct {
		ID       uint64 `gorm:"column:id"`
		Username string `gorm:"column:username"`
	}{}).Table("system_users").
		Select("id, username").
		Where("id IN ? AND deleted = 0", ids).
		Find(&users).Error; err != nil {
		return nil, 0, err
	}
	nameOf := map[uint64]string{}
	for _, u := range users {
		nameOf[u.ID] = u.Username
	}
	for _, row := range rows {
		row.Username = nameOf[row.UserID] // 已删用户留空串，前端渲染 "-"
	}
	return rows, total, nil
}

func (r *onlineRepo) GetByID(ctx context.Context, id uint64) (*OnlineSession, error) {
	var row OnlineSession
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil // 查无/跨租户：交 service 判 30203
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *onlineRepo) GetMemberForImpersonate(ctx context.Context, userID uint64) (*MemberBrief, error) {
	var m MemberBrief
	err := r.db.WithContext(ctx).Where("id = ?", userID).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}
```

注意：`ListAdmin` 里补 username 的匿名 struct 查询——gorm 对无模型的 `Table("system_users")` + `Find(&users)`，schema 取自 dest 匿名 struct（无 TenantID 字段 → 插件跳过注入，正确：system_users 补名按 id IN 页内集合，不泄漏列表）。若实现时 `Model(&struct{}{})` 与 `Find(&users)` 组合在本仓库 gorm 版本下行为异常，可改为 `r.db.WithContext(ctx).Table("system_users").Select("id, username").Where(...).Find(&users)` 直写（先例：`tenant.go:64` 的 `Table("member_user")` 直查）。

- [ ] **Step 3: 写 service 骨架 + List（online_service.go）**

```go
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
```

- [ ] **Step 4: 写测试夹具 + List 失败测试**

`modules/system/service/online_service_test.go`：

```go
package service

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/repository"
	"alexGo-cloud/pkg/auth"
	"alexGo-cloud/pkg/config"
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
	_ = sawGhost
}
```

（`tenant.WithTenantID` 的实际函数名以 `pkg/tenant/tenant.go` 为准——实现时 grep `func With`/`TenantIDFromContext` 对齐；若无现成构造器则在测试内用插件接受的 ctx 键直接 set，或给 `pkg/tenant` 补一个导出构造函数——**只允许加构造函数，不改插件语义**。）

- [ ] **Step 5: 跑测试确认失败（接口已有实现，此时应只差 username/过滤细节——红灯起点）**

Run: `go test ./modules/system/service/ -run TestOnlineService_List -v`
Expected：编译错误或 FAIL（实现未完时红；若直接绿说明夹具方向对，继续 Step 6 前先确认用例覆盖了 ghost/过期/会员混入）。

- [ ] **Step 6: 实现缺口使测试过（如 username 补查、过滤条件已在 Step 2/3 写完则直接绿）**

Run: `go test ./modules/system/service/ -run TestOnlineService -v`
Expected: PASS（List 用例；Kick/Impersonate 尚无用例）

- [ ] **Step 7: Commit**

```bash
git add pkg/errors/code.go modules/system/repository/online_repo.go modules/system/service/online_service.go modules/system/service/online_service_test.go
git commit -m "feat(online): 在线会话 repo + List 分页（P1-3 T1）"
```

---

### Task 2: OnlineService.Kick

**Files:**
- Modify: `modules/system/service/online_service.go`（删 Kick 占位）
- Test: `modules/system/service/online_service_test.go`（追加用例）

**Interfaces:**
- Consumes: Task 1 的 `OnlineRepository.GetByID`、`token.Issuer.Revoke`、`errs.CodeCannotKickSelf`/`CodeSessionNotFound`
- Produces: `Kick(ctx context.Context, operator *auth.Claims, sessionID uint64) error`（可被 controller 直接调用；语义：跨租户/查无 → 30203；非 user_type=1 → 30203；自己 → 30202；成功 → Revoke 物理删行）

- [ ] **Step 1: 写失败测试**

追加到 `online_service_test.go`：

```go
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
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./modules/system/service/ -run TestOnlineService_Kick -v`
Expected: FAIL（占位实现返回 "not implemented"，`errs.Is` 不成立）

- [ ] **Step 3: 实现 Kick**

替换 `online_service.go` 中 Kick 占位：

```go
// Kick 踢人：按行 id 吊销。禁止踢自己（防把自己锁外面）；非管理员会话/查无/跨租户 → 30203。
func (s *onlineService) Kick(ctx context.Context, operator *auth.Claims, sessionID uint64) error {
	row, err := s.repo.GetByID(ctx, sessionID)
	if err != nil {
		return err
	}
	if row == nil || row.UserID == 0 {
		return errs.New(errs.CodeSessionNotFound, nil)
	}
	// user_type=1（管理员）之外的行不在本页范围——OnlineSession 未投影 user_type 列，
	// 以「补名查得到 system_users」不足以判类型，故投影列在 Task 1 已含 UserType：
	if row.UserType != token.UserTypeAdmin {
		return errs.New(errs.CodeSessionNotFound, nil)
	}
	if operator.UserID == row.UserID {
		return errs.New(errs.CodeCannotKickSelf, nil)
	}
	return s.issuer.Revoke(ctx, row.AccessToken)
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./modules/system/service/ -run TestOnlineService -v`
Expected: PASS（List + Kick 全部用例；`go vet ./modules/system/...` 干净）

- [ ] **Step 5: Commit**

```bash
git add modules/system/service/online_service.go modules/system/service/online_service_test.go
git commit -m "feat(online): Kick 踢人（禁踢自己/跨租户 30203/竞态兜底）（P1-3 T2）"
```

---

### Task 3: OnlineService.Impersonate（代登录 + login_logs 审计）

**Files:**
- Modify: `modules/system/service/online_service.go`（删 Impersonate 占位）
- Test: `modules/system/service/online_service_test.go`（追加用例）

**Interfaces:**
- Consumes: `GetMemberForImpersonate`、`token.Issuer.Issue`（`IssueParams{ClientID: ImpersonateClientID}`）、`repository.AuditRepository.CreateLogin`、`errs.CodeImpersonateMemberNotFound`
- Produces: `Impersonate(ctx, operator *auth.Claims, ip, userAgent string, targetUserID uint64) (*token.Issued, error)`——controller 侧响应体 `{access_token, refresh_token, expires_in}` 直接取 `token.Issued`

- [ ] **Step 1: 写失败测试**

追加（夹具补充：member_user 由 `MemberBrief` AutoMigrate 产出，需灌数据行）：

```go
func seedMember(t *testing.T, db *gorm.DB, id uint64, tid uint64, status int, deleted bool) {
	t.Helper()
	d := 0
	if deleted {
		d = 1
	}
	if err := db.Exec(
		`INSERT INTO member_user (id, nickname, status, deleted, tenant_id, create_time, update_time) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, "会员"+string(rune('0'+id%10)), status, d, tid, time.Now(), time.Now()).Error; err != nil {
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
	seedMember(t, db, 502, 1, 0, false)   // 停用
	seedMember(t, db, 503, 1, 1, true)    // 已删
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
```

（顶部补 `strings` import；`model.LoginLog` 已在夹具 import。）

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./modules/system/service/ -run TestOnlineService_Impersonate -v`
Expected: FAIL（占位 "not implemented"）

- [ ] **Step 3: 实现 Impersonate**

替换占位：

```go
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
```

（补 `fmt` import；删掉文件尾的 `var _ = model.LoginLog{}` 占位行——本步已真实使用。）

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./modules/system/service/ -run TestOnlineService -v`
Expected: PASS（List + Kick + Impersonate 全部）

- [ ] **Step 5: Commit**

```bash
git add modules/system/service/online_service.go modules/system/service/online_service_test.go
git commit -m "feat(online): Impersonate 代登录（client_id 标记 + login_logs 审计）（P1-3 T3）"
```

---

### Task 4: OnlineController + module 挂载 + permissionRoutes

**Files:**
- Create: `modules/system/controller/admin/online_controller.go`
- Test: `modules/system/controller/admin/online_controller_test.go`
- Modify: `modules/system/module.go`（fx.Provide 三条 + struct 字段 + RegisterRoutes 三行）
- Modify: `modules/system/service/permission_routes.go`（`system:online` 键）

**Interfaces:**
- Consumes: Task 1-3 的 `service.OnlineService`（三方法齐）、`auth.Claims`（`ctx.Get("claims")`，照 `admin/auth.go:32`）、`resp.PageReq/Page/OK/Fail`
- Produces: 三端点 handler（module.go 路由形状）：`GET /admin/system/online`、`DELETE /admin/system/online/:id`、`POST /admin/system/online/impersonate`

- [ ] **Step 1: 写 controller 失败测试**

`online_controller_test.go`（gin 手装 + httptest，照 `notice_read_test.go`）：

```go
package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/service"
	"alexGo-cloud/pkg/auth"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/resp"
	"alexGo-cloud/pkg/token"
)

// fakeOnlineSvc 控制桩：按用例切返回值。
type fakeOnlineSvc struct {
	list     []*service.OnlineSessionVO
	listErr  error
	total    int64
	kickErr  error
	kickedID uint64
	imp      *token.Issued
	impErr   error
	impUID   uint64
}

func (f *fakeOnlineSvc) List(ctx context.Context, page, size int) ([]*service.OnlineSessionVO, int64, error) {
	return f.list, f.total, f.listErr
}
func (f *fakeOnlineSvc) Kick(ctx context.Context, operator *auth.Claims, sessionID uint64) error {
	f.kickedID = sessionID
	return f.kickErr
}
func (f *fakeOnlineSvc) Impersonate(ctx context.Context, operator *auth.Claims, ip, userAgent string, targetUserID uint64) (*token.Issued, error) {
	f.impUID = targetUserID
	return f.imp, f.impErr
}

func newOnlineRouter(svc service.OnlineService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// 注入 claims 的桩中间件（与 pkg/middleware/auth.go 的 c.Set("claims", ...) 同键）
	r.Use(func(c *gin.Context) {
		c.Set("claims", &auth.Claims{UserID: 9, Username: "root", TenantID: 1, UserType: 1})
	})
	ctrl := NewOnlineController(svc)
	// 与 module.go RegisterRoutes 同形作回归
	r.GET("/api/admin/system/online", ctrl.List)
	r.DELETE("/api/admin/system/online/:id", ctrl.Kick)
	r.POST("/api/admin/system/online/impersonate", ctrl.Impersonate)
	return r
}

func doOnlineReq(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	var rd *strings.Reader = strings.NewReader(body)
	req := httptest.NewRequest(method, path, rd)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

type envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func TestOnlineController_List(t *testing.T) {
	fake := &fakeOnlineSvc{
		list: []*service.OnlineSessionVO{{ID: 1, UserID: 9, Username: "alice", ClientID: "vben-web"}},
		total: 1,
	}
	w := doOnlineReq(newOnlineRouter(fake), http.MethodGet, "/api/admin/system/online?page=1&size=10", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	var env envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Code != 0 {
		t.Errorf("code = %d, msg %s", env.Code, env.Msg)
	}
	var data struct {
		List  []service.OnlineSessionVO `json:"list"`
		Total int64                     `json:"total"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.List) != 1 || data.Total != 1 || data.List[0].Username != "alice" {
		t.Errorf("data = %+v", data)
	}
	if strings.Contains(w.Body.String(), "access_token") {
		t.Error("response must not contain access_token field name from VO (明文防线)")
	}
}

func TestOnlineController_Kick(t *testing.T) {
	fake := &fakeOnlineSvc{}
	w := doOnlineReq(newOnlineRouter(fake), http.MethodDelete, "/api/admin/system/online/7", "")
	if w.Code != http.StatusOK || fake.kickedID != 7 {
		t.Errorf("status = %d, kickedID = %d", w.Code, fake.kickedID)
	}
	// 30202 透传
	fake.kickErr = errs.New(errs.CodeCannotKickSelf, nil)
	w = doOnlineReq(newOnlineRouter(fake), http.MethodDelete, "/api/admin/system/online/7", "")
	var env envelope
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	if env.Code != 30202 {
		t.Errorf("code = %d, want 30202", env.Code)
	}
}

func TestOnlineController_Kick_BadID(t *testing.T) {
	fake := &fakeOnlineSvc{}
	w := doOnlineReq(newOnlineRouter(fake), http.MethodDelete, "/api/admin/system/online/abc", "")
	var env envelope
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	if env.Code != errs.CodeParamInvalid {
		t.Errorf("code = %d, want 10001", env.Code)
	}
}

func TestOnlineController_Impersonate(t *testing.T) {
	fake := &fakeOnlineSvc{imp: &token.Issued{AccessToken: "a", RefreshToken: "r", ExpiresIn: 7200}}
	w := doOnlineReq(newOnlineRouter(fake), http.MethodPost, "/api/admin/system/online/impersonate", `{"user_id":501}`)
	if w.Code != http.StatusOK || fake.impUID != 501 {
		t.Fatalf("status = %d, impUID = %d, body %s", w.Code, fake.impUID, w.Body.String())
	}
	var env envelope
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	var data struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	_ = json.Unmarshal(env.Data, &data)
	if data.AccessToken != "a" || data.ExpiresIn != 7200 {
		t.Errorf("data = %+v", data)
	}
	// 30201 透传
	fake.impErr = errs.New(errs.CodeImpersonateMemberNotFound, nil)
	fake.imp = nil
	w = doOnlineReq(newOnlineRouter(fake), http.MethodPost, "/api/admin/system/online/impersonate", `{"user_id":502}`)
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	if env.Code != 30201 {
		t.Errorf("code = %d, want 30201", env.Code)
	}
}
```

- [ ] **Step 2: 跑测试确认失败（编译错：NewOnlineController 未定义）**

Run: `go test ./modules/system/controller/admin/ -run TestOnlineController -v`
Expected: 编译失败 `undefined: NewOnlineController`

- [ ] **Step 3: 写 controller**

`online_controller.go`：

```go
package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/service"
	"alexGo-cloud/pkg/auth"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/resp"
)

// OnlineController 在线会话监管（列表/踢人/代登录）。
// claims 取中间件注入（照 auth.go Profile）：Kick 需操作者身份禁踢自己，Impersonate 需审计操作者。
type OnlineController struct {
	svc service.OnlineService
}

func NewOnlineController(svc service.OnlineService) *OnlineController {
	return &OnlineController{svc: svc}
}

func onlineClaims(c *gin.Context) (*auth.Claims, bool) {
	v, ok := c.Get("claims")
	if !ok {
		return nil, false
	}
	claims, ok := v.(*auth.Claims)
	if !ok || claims == nil {
		return nil, false
	}
	return claims, true
}

// List GET /admin/system/online：本租户未过期管理员会话分页。
func (c *OnlineController) List(ctx *gin.Context) {
	var req resp.PageReq
	if err := ctx.ShouldBindQuery(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	req.Defaults()
	list, total, err := c.svc.List(ctx.Request.Context(), req.Page, req.Size)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.Page(ctx, list, total, req)
}

// Kick DELETE /admin/system/online/:id：按行 id 吊销（明文 token 不出接口）。
func (c *OnlineController) Kick(ctx *gin.Context) {
	claims, ok := onlineClaims(ctx)
	if !ok {
		resp.Fail(ctx, errs.New(errs.CodeUnauthorized, nil))
		return
	}
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if err := c.svc.Kick(ctx.Request.Context(), claims, id); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

type impersonateReq struct {
	UserID uint64 `json:"user_id" binding:"required"`
}

// Impersonate POST /admin/system/online/impersonate：代登录到会员（client_id=impersonate-admin）。
func (c *OnlineController) Impersonate(ctx *gin.Context) {
	claims, ok := onlineClaims(ctx)
	if !ok {
		resp.Fail(ctx, errs.New(errs.CodeUnauthorized, nil))
		return
	}
	var req impersonateReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	issued, err := c.svc.Impersonate(ctx.Request.Context(), claims, ctx.ClientIP(), ctx.GetHeader("User-Agent"), req.UserID)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, issued) // {access_token, refresh_token, expires_in}（token.Issued json tag）
}
```

注意：`token.Issued` 的 json 字段名——`pkg/token/token.go` 的 struct 若无 json tag，响应会是 `AccessToken`（Go 首字母大写）。**实现时检查**：若无 tag，在 controller 侧包一层响应 struct：

```go
resp.OK(ctx, gin.H{
	"access_token":  issued.AccessToken,
	"refresh_token": issued.RefreshToken,
	"expires_in":    issued.ExpiresIn,
})
```

并同步 Task 4 Step 1 测试断言（测试已按 snake_case 断言）。**默认采用 gin.H 包装**（不改 pkg/token 的导出 struct json 契约）。

- [ ] **Step 4: module.go 挂载 + fx**

`modules/system/module.go` 三处：
1. `fx.Provide` 块尾部（`tokenServiceRegistrar,` 之前）加：

```go
// P1-3 在线用户 + 代登录。
repository.NewOnlineRepository,
service.NewOnlineService,
admin.NewOnlineController,
```

2. `systemModule` struct 加字段 `onlineCtrl *admin.OnlineController`，`NewModule` 构造函数参数加 `onlineCtrl *admin.OnlineController`，字面量加 `onlineCtrl: onlineCtrl,`。
3. `RegisterRoutes` admin 组 `adminGroup.DELETE("/tenants/:id", ...)` 之后加：

```go
// P1-3 在线用户：静态段 /online/impersonate 与参数段 /online/:id 不同 method，无 gin 冲突。
adminGroup.GET("/online", m.onlineCtrl.List)
adminGroup.DELETE("/online/:id", m.onlineCtrl.Kick)
adminGroup.POST("/online/impersonate", m.onlineCtrl.Impersonate)
```

`permission_routes.go` map 中 `"system:log"` 行后加：

```go
"system:online": {"/api/admin/system/online", "/api/admin/system/online/*"},
```

- [ ] **Step 5: 跑测试确认通过 + 全模块回归**

Run: `go test ./modules/system/... -v -run 'Online|PermissionRoutes'`
Expected: PASS（controller 用例 + permission_routes 既有测试含新键断言——若该测试是全键遍历则自动覆盖；跑 `go test ./modules/system/service/ -run PermissionRoutes -v` 确认无 FAIL）

- [ ] **Step 6: Commit**

```bash
git add modules/system/controller/admin/online_controller.go modules/system/controller/admin/online_controller_test.go modules/system/module.go modules/system/service/permission_routes.go
git commit -m "feat(online): 三端点 controller + module 挂载 + permissionRoutes（P1-3 T4）"
```

---

### Task 5: 菜单/按钮 migration SQL

**Files:**
- Create: `modules/system/migrations/20261021000001_online_user_menus.up.sql`

**Interfaces:**
- Consumes: Task 4 的 `permissionRoutes["system:online"]`（注释钉同源）；menus 表结构照 `20261011000003_oauth2_client_menu.up.sql`（COALESCE 父目录 + NOT EXISTS 幂等 + role_menus 绑 admin）
- Produces: 菜单「在线用户」+ 按钮 `system:online:kick` / `system:online:impersonate`（前端 v-access 消费）

- [ ] **Step 1: 写 migration SQL**

```sql
-- P1-3 在线用户 + 代登录菜单种子（抄 20261011000003 oauth2_client_menu 模式）。
-- 幂等：全部 NOT EXISTS 守卫，可重复跑。
-- 权限码 system:online:list / :kick / :impersonate 必须与 permission_routes.go 的
-- "system:online" 键同源（prefix 前两段 system:online），否则本迁移产出死权限（403）。
-- 父目录「系统管理」由 StartSeeder 在迁移之后创建（fresh 库），故 parent 用
-- COALESCE 兜底 0——已有库挂系统管理下，fresh 库落根（seed 尾部 SetRoleMenus 全量绑定）。

-- 1) 页面菜单「在线用户」
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, COALESCE((SELECT p.`id` FROM `menus` p WHERE p.`tenant_id`=0 AND p.`name`='系统管理' AND p.`type`='dir' AND p.`deleted`=0 LIMIT 1), 0),
       'menu', '在线用户', '/system/online-users', 'views/system/SystemOnlineUsersPage', 'lucide:monitor', 'system:online:list', 95, 1, 0, NOW(), NOW()
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND path='/system/online-users' AND deleted=0);

-- 2) 按钮：踢人 / 代登录
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, m.`id`, 'button', t.`name`, '', '', '', t.`perm`, t.`sort`, 1, 0, NOW(), NOW()
FROM `menus` m
JOIN (
  SELECT '踢人' AS `name`, 'system:online:kick' AS perm, 1 AS `sort`
  UNION ALL SELECT '代登录', 'system:online:impersonate', 2
) t
WHERE m.`tenant_id`=0 AND m.`permission`='system:online:list' AND m.`type`='menu' AND m.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` b WHERE b.`tenant_id`=0 AND b.`permission`=t.`perm` AND b.`deleted`=0);

-- 3) admin 角色挂菜单
INSERT INTO `role_menus` (`role_id`,`menu_id`,`tenant_id`)
SELECT r.`id`, m.`id`, 0
FROM `menus` m
JOIN `roles` r ON r.`tenant_id`=0 AND r.`code`='admin' AND r.`deleted`=0
WHERE m.`tenant_id`=0 AND m.`deleted`=0
  AND (m.`path`='/system/online-users' OR m.`permission` LIKE 'system:online%')
  AND NOT EXISTS (SELECT 1 FROM `role_menus` rm WHERE rm.`tenant_id`=0 AND rm.`role_id`=r.`id` AND rm.`menu_id`=m.`id`);
```

- [ ] **Step 2: 幂等验证（本机 MySQL 可达时）**

Run: `make migrate && make migrate`（跑两遍）
Expected: 两次都 0 错误；第二遍 NOT EXISTS 全跳过（零新行）。若本机 MySQL 不可达，跳过实跑，在 Commit message 注明「migration 未实跑，随下次启动自动应用」。

- [ ] **Step 3: Commit**

```bash
git add modules/system/migrations/20261021000001_online_user_menus.up.sql
git commit -m "feat(online): 在线用户菜单/按钮种子 migration（P1-3 T5）"
```

---

### Task 6: 前端 API + 在线用户页

**Files:**
- Modify: `apps/web-antd/src/api/admin.ts`（新段：online 三 API + listMemberUsersApi）
- Create: `apps/web-antd/src/views/system/SystemOnlineUsersPage.vue`

**Interfaces:**
- Consumes: Task 4 三端点；`GET /admin/member/users?page&size`（member 模块既有，返回 `resp.Page` 形状 `{list,total,page,size}`，item 含 `id/nickname/mobile/status`）；页面 component 解析走 `import.meta.glob('../views/**/*.vue')` 自动（菜单 migration 指定 `views/system/SystemOnlineUsersPage`，零路由改动）
- Produces: `listOnlineApi/kickOnlineApi/impersonateApi/listMemberUsersApi`（后续无消费者，本任务自用）

- [ ] **Step 1: api/admin.ts 新段**（文件尾追加；interface 放同段）

```ts
// ---- 在线用户（P1-3） ----
export interface OnlineSession {
  id: number;
  user_id: number;
  username: string;
  client_id: string;
  is_impersonated: boolean;
  issued_at: string;
  expires_at: string;
}

export function listOnlineApi(params: Record<string, any>) {
  return requestClient.get<{ list: OnlineSession[]; total: number }>(
    '/admin/system/online',
    { params },
  );
}

export function kickOnlineApi(id: number): Promise<void> {
  return requestClient.delete(`/admin/system/online/${id}`);
}

export function impersonateApi(userId: number) {
  return requestClient.post<{
    access_token: string;
    refresh_token: string;
    expires_in: number;
  }>('/admin/system/online/impersonate', { user_id: userId });
}

export interface MemberBrief {
  id: number;
  nickname: string;
  mobile: string;
  status: number;
}

export function listMemberUsersApi(params: Record<string, any>) {
  return requestClient.get<{ list: MemberBrief[]; total: number }>(
    '/admin/member/users',
    { params },
  );
}
```

- [ ] **Step 2: 页面 `SystemOnlineUsersPage.vue`**

照 `ScorecardRoomsPage.vue` 的 Table + 服务端分页范式（不用 vxeGrid，与最近一批页面保持一致）：

```vue
<script setup lang="ts">
import { onMounted, ref } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Button,
  Input,
  message,
  Modal,
  Popconfirm,
  Table,
  TableColumn,
  Tag,
} from 'ant-design-vue';

import {
  impersonateApi,
  kickOnlineApi,
  listMemberUsersApi,
  listOnlineApi,
} from '#/api/admin';

const loading = ref(false);
const list = ref<any[]>([]);
const total = ref(0);
const pagination = ref({ current: 1, pageSize: 20 });

async function load() {
  loading.value = true;
  try {
    const res = await listOnlineApi({
      page: pagination.value.current,
      size: pagination.value.pageSize,
    });
    list.value = (res as any).list ?? res;
    total.value = (res as any).total ?? 0;
  } finally {
    loading.value = false;
  }
}

async function onKick(row: any) {
  await kickOnlineApi(row.id);
  message.success('已踢出，该会话立即失效');
  await load();
}

// ---- 代登录 modal ----
const impOpen = ref(false);
const impLoading = ref(false);
const memberKeyword = ref('');
const memberList = ref<any[]>([]);
const selectedMember = ref<any>(null);

async function searchMembers() {
  impLoading.value = true;
  try {
    const res = await listMemberUsersApi({ page: 1, size: 50 });
    const rows = (res as any).list ?? [];
    const kw = memberKeyword.value.trim().toLowerCase();
    memberList.value = kw
      ? rows.filter(
          (m: any) =>
            String(m.nickname ?? '').toLowerCase().includes(kw) ||
            String(m.mobile ?? '').includes(kw),
        )
      : rows;
  } finally {
    impLoading.value = false;
  }
}

function openImpersonate() {
  selectedMember.value = null;
  memberKeyword.value = '';
  memberList.value = [];
  impOpen.value = true;
  searchMembers();
}

async function confirmImpersonate() {
  if (!selectedMember.value) {
    message.warning('先选择目标会员');
    return;
  }
  // token 不回显、不落盘（spec §4：仅签发+审计；浏览器身份切换留后续小票）
  await impersonateApi(selectedMember.value.id);
  message.success('代登录成功，会话已生效（可在登录日志审计）');
  impOpen.value = false;
  await load();
}

onMounted(load);
</script>

<template>
  <Page auto-content-height>
    <div class="p-4">
      <div class="mb-3 flex gap-2">
        <Button type="primary" :loading="loading" @click="load">刷新</Button>
      </div>
      <Table
        :data-source="list"
        :loading="loading"
        :pagination="{
          ...pagination,
          total,
          onChange: (p: number, ps: number) => {
            pagination.current = p;
            pagination.pageSize = ps;
            load();
          },
        }"
        row-key="id"
      >
        <TableColumn title="ID" data-index="id" :width="80" />
        <TableColumn title="用户名" data-index="username">
          <template #default="{ record }">
            {{ record.username || '-' }}
          </template>
        </TableColumn>
        <TableColumn title="用户ID" data-index="user_id" :width="90" />
        <TableColumn title="客户端" data-index="client_id">
          <template #default="{ record }">
            <Tag v-if="record.is_impersonated" color="orange">代登录</Tag>
            <template v-else>{{ record.client_id || '-' }}</template>
          </template>
        </TableColumn>
        <TableColumn title="签发时间" data-index="issued_at" />
        <TableColumn title="过期时间" data-index="expires_at" />
        <TableColumn title="操作" :width="180">
          <template #default="{ record }">
            <Popconfirm
              title="确认踢出该会话？踢出后立即失效。"
              @confirm="onKick(record)"
            >
              <Button
                v-access:code="['system:online:kick']"
                danger
                size="small"
              >
                踢出
              </Button>
            </Popconfirm>
            <Button
              v-access:code="['system:online:impersonate']"
              class="ml-2"
              size="small"
              @click="openImpersonate"
            >
              代登录
            </Button>
          </template>
        </TableColumn>
      </Table>

      <Modal
        v-model:open="impOpen"
        title="代登录到会员"
        @ok="confirmImpersonate"
      >
        <Input
          v-model:value="memberKeyword"
          allow-clear
          class="mb-3"
          placeholder="按昵称/手机过滤（前 50 条）"
          @press-enter="searchMembers"
        />
        <Table
          :columns="[
            { title: 'ID', dataIndex: 'id', width: 80 },
            { title: '昵称', dataIndex: 'nickname' },
            { title: '手机', dataIndex: 'mobile' },
          ]"
          :data-source="memberList"
          :loading="impLoading"
          :pagination="false"
          row-key="id"
          size="small"
          :row-selection="{
            type: 'radio',
            onSelect: (r: any) => {
              selectedMember = r;
            },
          }"
        />
      </Modal>
    </div>
  </Page>
</template>
```

- [ ] **Step 3: 类型检查 + 单文件 lint**

Run: `pnpm typecheck`
Expected: 绿（新文件无 any 逃逸报错——Table record 默认 any 可过）

Run: `npx eslint apps/web-antd/src/views/system/SystemOnlineUsersPage.vue apps/web-antd/src/api/admin.ts`
Expected: 0 error（若 prettier 格式告警，按提示 `npx prettier --write` 两文件后复跑）

- [ ] **Step 4: Commit**

```bash
git add apps/web-antd/src/api/admin.ts apps/web-antd/src/views/system/SystemOnlineUsersPage.vue
git commit -m "feat(online): 前端在线用户页 + API（P1-3 T6）"
```

---

### Task 7: 全量回归 + docs 回填

**Files:**
- Modify: `docs/roadmap.md`（P1-3 行 → ✅ + 提交号）
- Modify: `docs/modules.md`（§3 system 域回填在线用户/代登录小节，照 §4.7 文风）
- Modify: `docs/usage.md`（新节：在线用户页与代登录用法，含「token 不回显」说明）

**Interfaces:**
- Consumes: Task 1-6 全部产物
- Produces: 文档现状与代码一致；roadmap 进度更新

- [ ] **Step 1: 全量测试**

Run: `go test ./...`
Expected: 全绿（53+ 包，含新增 online 用例）

Run: `pnpm typecheck`
Expected: 绿

- [ ] **Step 2: 禁词红线扫描**

Run: `git diff main -- . | grep -inE 'bet|cash|rake|money|钱|赌注|抽成|赌' || echo CLEAN`
Expected: `CLEAN`（本计划所有新增文案已人工核过）

- [ ] **Step 3: docs 回填**

- `docs/roadmap.md` P1-3 行状态列改：`✅ <commit>`（填本任务链最终 commit 号；实现中先写 `✅` 占位，Step 5 commit 前用 `git log --oneline` 回填 T6 commit 号）。
- `docs/modules.md`：§3 system 域（登录与令牌 §2.2 附近合适位置，或监管页聚集处）加小节「在线用户与代登录」：三端点表、权限码三枚、`client_id=impersonate-admin` 标记、踢人禁踢自己、login_logs 审计 message 形状、明确不做（会员在线/IP-UA/浏览器切换）。
- `docs/usage.md`：新节「6.x 在线用户」：菜单路径（系统管理→在线用户）、踢人操作、代登录操作（选会员→确认→toast；token 不回显的边界说明 + 审计查 login_logs）。

- [ ] **Step 4: 终审前自查 commit**

```bash
git add docs/roadmap.md docs/modules.md docs/usage.md
git commit -m "docs(online): P1-3 回填 roadmap/modules/usage"
```

- [ ] **Step 5: 回填 roadmap 提交号并 amend**

上一步 commit 的 roadmap 行若仍是占位，执行：改文件填 T4/T6 commit 号 → `git add docs/roadmap.md && git commit --amend --no-edit`（仅当上一步 commit 未被其它工作依赖；本计划内 Step 4 是链尾，安全）。

- [ ] **Step 6: 最终状态确认**

Run: `git log --oneline -8`
Expected: T1..T7 七笔提交清晰可回溯。
