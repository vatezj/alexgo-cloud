# 用户体系一期（双用户 + Token + 多租户 + 权限对齐 + 双服务骨架）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 落地已评审通过的用户体系 spec 一期：双用户分表、OAuth2 Token 体系（单体/双服务双模式）、SaaS 多租户隔离、权限管理对齐（Casbin 同步/data_scope/租户前缀）、member-server 双服务骨架。

**Architecture:** 两个运行模式共用同一套模块代码（`fx.Decorate` 切换 `token.Issuer` 本地/gRPC 实现）；mono 为默认一键启动，micro 为部署形态（system-server :8080 + member-server :8081，前缀分流、无网关）；同库不同进程；能力全部放 `pkg/`（token、tenant 插件、sms 预留位）。

**Tech Stack:** Go 1.25、Gin、Uber FX、GORM(MySQL)、glebarez/sqlite(测试)、Casbin(gorm-adapter)、gRPC+protobuf、golang-migrate。

**Spec:** `docs/superpowers/specs/2026-10-08-user-system-design.md`（已评审通过；本计划是其一期范围的实施论证——spec 冲突以 spec 为准，本计划记录的偏差见 Global Constraints）

## Global Constraints

- 项目根：`/Users/alex/Desktop/goWork/alexGo-cloud`，**直接在 `main` 分支提交**（不建功能分支），每任务一次 commit
- 本地环境：裸 `go build`/`go test` 有既有 macOS SDK 链接错误 → **一律 `CGO_ENABLED=0`**；`golangci-lint` 未装本地（CI 装且 errcheck 默认开——**所有 error 返回值必须处理**，禁止 `_ =` 风格除非显式注释理由）
- **测试零外部依赖**：sqlite 内存库（glebarez，已在 go.mod）、内存 Casbin、fake Issuer/Validator、bufconn gRPC；**不依赖 MySQL/Redis/网络**
- 中文注释、解释"为什么"；文件级 doc comment 与现有风格一致
- **状态位统一 `1=启用 0=停用`**（与现有 `users.Status==1` 才可登录一致；spec DDL 注释里"0开启1停用"以此为准改写）
- **`deleted` 列一律 `TINYINT(1)`（不是 TINYINT(1)）**、一期仅落库不启用 gorm 软删——TINYINT(1) 经 go-sql-driver 返回原始字节，GORM bool 字段扫描必然报错（T1 审查源码级证实）；对 spec "bit(1)+gorm.DeletedAt" 的记录性偏差
- 环境变量经 `applyEnvOverrides` 显式覆盖（viper Unmarshal 不读隐式 env）：新增 `DEPLOYMENT_MODE`、`SYSTEM_GRPC_ADDR`
- `protoc` 本机未装：Task 11 用 `brew install protobuf` + `go install protoc-gen-go protoc-gen-go-grpc`（**brew 为工作区外系统改动，执行时向用户披露**）
- 新命名：spec 的 `TokenIssuer` → 代码接口 `token.Issuer`；gRPC 服务名 `TokenService`
- 既有 7 个测试包必须保持全绿；改动既有测试（如 auth 中间件）必须在同任务内完成

## Review Focus

以下 5 类故障最可能咬人，各有归属任务的测试钉住：

1. **Token 全生命周期**：签发/校验/刷新（轮换）/注销/踢人（RevokeAll）/过期拒绝/缓存与注销一致性 → Task 2、4
2. **gorm 租户隔离插件边界**：跨租户不可见、tid=0 不过滤（保持现状）、白名单表跳过、无 TenantID 字段模型跳过、IgnoreTenant 放行 → Task 6
3. **Casbin 策略同步与租户前缀**：AssignMenus 后角色立即有/无权限、`{tid}:{user}` 前缀使跨租户同 code 角色不串、启动重灌清掉旧裸 sub 策略 → Task 9
4. **data_scope 五档注入**：1全部/2自定义/3本部门/4本部门及以下/5仅本人 各自的查询结果正确、白名单豁免 → Task 10
5. **双模式一致性**：mono 本地签发与 micro gRPC 委托产出可用等价 token；gRPC 不可达时 member 登录 503 不降级；两模式对同一 token 校验一致 → Task 11、12

---

### Task 1: 系统侧数据库迁移 + 模型适配

**Files:**
- Create: `modules/system/migrations/20261008000001_rename_users.up.sql` / `.down.sql`
- Create: `modules/system/migrations/20261008000002_system_users_columns.up.sql` / `.down.sql`
- Create: `modules/system/migrations/20261008000003_oauth2_access_token.up.sql` / `.down.sql`
- Create: `modules/system/migrations/20261008000004_tenants.up.sql` / `.down.sql`
- Create: `modules/system/migrations/20261008000005_roles_menus_columns.up.sql` / `.down.sql`
- Modify: `modules/system/model/model.go`（User）、`modules/system/model/rbac.go`（Role/Menu）

**Interfaces:**
- Consumes: 现有 `model.User`（users 表）、`roles/menus` 表
- Produces: 表 `system_users`（含 spec §3.1 全部列）、`system_oauth2_access_token`（spec §3.3）、`tenants`（spec §3.4）；`roles`/`menus` 补列（spec §3.7）；`model.User.TableName() == "system_users"`；`model.Role`/`model.Menu` 新字段——后续所有任务依赖

- [ ] **Step 1: 写失败测试（表名锚点）**

创建 `modules/system/model/model_name_test.go`：

```go
package model

import "testing"

// users 表已重命名为 system_users，TableName 必须跟随（否则迁移后 GORM 全部打到不存在的表）。
func TestUserTableName(t *testing.T) {
	if got := (User{}).TableName(); got != "system_users" {
		t.Errorf("User.TableName() = %q, want system_users", got)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `CGO_ENABLED=0 go test ./modules/system/model/ -v -count=1`
Expected: FAIL — `User.TableName() = "users"`（零值 TableName 默认表名是 `users`，加 `system_users` 断言后红）

- [ ] **Step 3: 写迁移 SQL（up/down 成对）**

`20261008000001_rename_users.up.sql`：

```sql
RENAME TABLE `users` TO `system_users`;
```

`20261008000001_rename_users.down.sql`：

```sql
RENAME TABLE `system_users` TO `users`;
```

`20261008000002_system_users_columns.up.sql`：

```sql
ALTER TABLE `system_users`
  ADD COLUMN `remark`     VARCHAR(500) NOT NULL DEFAULT '' COMMENT '备注' AFTER `nickname`,
  ADD COLUMN `dept_id`    BIGINT       NOT NULL DEFAULT 0  COMMENT '部门ID' AFTER `remark`,
  ADD COLUMN `post_ids`   VARCHAR(255) NOT NULL DEFAULT '' COMMENT '岗位ID数组' AFTER `dept_id`,
  ADD COLUMN `email`      VARCHAR(50)  NOT NULL DEFAULT '' COMMENT '邮箱' AFTER `post_ids`,
  ADD COLUMN `mobile`     VARCHAR(11)  NOT NULL DEFAULT '' COMMENT '手机号' AFTER `email`,
  ADD COLUMN `sex`        TINYINT      NOT NULL DEFAULT 0  COMMENT '性别 0未知 1男 2女' AFTER `mobile`,
  ADD COLUMN `avatar`     VARCHAR(100) NOT NULL DEFAULT '' COMMENT '头像' AFTER `sex`,
  ADD COLUMN `login_ip`   VARCHAR(50)  NOT NULL DEFAULT '' COMMENT '最近登录IP' AFTER `status`,
  ADD COLUMN `login_date` DATETIME     NULL COMMENT '最近登录时间' AFTER `login_ip`,
  ADD COLUMN `deleted`    TINYINT(1)       NOT NULL DEFAULT 0  COMMENT '是否删除（一期不启用软删，仅落列）' AFTER `login_date`,
  ADD COLUMN `creator`    VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '创建者' AFTER `deleted`,
  ADD COLUMN `updater`    VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '更新者' AFTER `creator`;
```

`20261008000002_system_users_columns.down.sql`：

```sql
ALTER TABLE `system_users`
  DROP COLUMN `remark`, DROP COLUMN `dept_id`, DROP COLUMN `post_ids`,
  DROP COLUMN `email`, DROP COLUMN `mobile`, DROP COLUMN `sex`,
  DROP COLUMN `avatar`, DROP COLUMN `login_ip`, DROP COLUMN `login_date`,
  DROP COLUMN `deleted`, DROP COLUMN `creator`, DROP COLUMN `updater`;
```

`20261008000003_oauth2_access_token.up.sql`：

```sql
CREATE TABLE IF NOT EXISTS `system_oauth2_access_token` (
  `id`            BIGINT       NOT NULL AUTO_INCREMENT,
  `user_id`       BIGINT       NOT NULL COMMENT '用户编号',
  `user_type`     TINYINT      NOT NULL COMMENT '用户类型 1管理员 2会员',
  `access_token`  VARCHAR(255) NOT NULL COMMENT '访问令牌',
  `refresh_token` VARCHAR(32)  NOT NULL DEFAULT '' COMMENT '刷新令牌',
  `client_id`     VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '客户端编号',
  `scopes`        VARCHAR(255) NOT NULL DEFAULT '' COMMENT '授权范围',
  `expires_time`  DATETIME     NOT NULL COMMENT '过期时间',
  `deleted`       TINYINT(1)       NOT NULL DEFAULT 0,
  `creator`       VARCHAR(64)  NOT NULL DEFAULT '',
  `create_time`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updater`       VARCHAR(64)  NOT NULL DEFAULT '',
  `update_time`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `tenant_id`     BIGINT       NOT NULL DEFAULT 0 COMMENT '租户编号',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_access_token` (`access_token`),
  UNIQUE KEY `uk_refresh_token` (`refresh_token`),
  KEY `idx_user` (`user_id`, `user_type`),
  KEY `idx_expires` (`expires_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='OAuth2 访问令牌表';
```

`20261008000003_oauth2_access_token.down.sql`：

```sql
DROP TABLE IF EXISTS `system_oauth2_access_token`;
```

`20261008000004_tenants.up.sql`：

```sql
CREATE TABLE IF NOT EXISTS `tenants` (
  `id`            BIGINT      NOT NULL AUTO_INCREMENT,
  `name`          VARCHAR(64) NOT NULL COMMENT '租户名称',
  `package_id`    BIGINT      NOT NULL DEFAULT 0  COMMENT '租户套餐编号（预留）',
  `status`        TINYINT     NOT NULL DEFAULT 1  COMMENT '状态 1启用 0停用',
  `expire_time`   DATETIME    NULL COMMENT '过期时间（NULL=永久）',
  `account_limit` INT         NOT NULL DEFAULT -1 COMMENT '账号额度 -1不限',
  `domain`        VARCHAR(64) NOT NULL DEFAULT '' COMMENT '绑定域名（登录解析用，可空）',
  `deleted`       TINYINT(1)      NOT NULL DEFAULT 0,
  `creator`       VARCHAR(64) NOT NULL DEFAULT '',
  `create_time`   DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updater`       VARCHAR(64) NOT NULL DEFAULT '',
  `update_time`   DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_domain` (`domain`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='租户表';
```

`20261008000004_tenants.down.sql`：

```sql
DROP TABLE IF EXISTS `tenants`;
```

`20261008000005_roles_menus_columns.up.sql`：

```sql
ALTER TABLE `roles`
  ADD COLUMN `sort`                INT          NOT NULL DEFAULT 0  COMMENT '显示顺序' AFTER `name`,
  ADD COLUMN `data_scope`          TINYINT      NOT NULL DEFAULT 1  COMMENT '数据范围 1全部 2自定义 3本部门 4本部门及以下 5仅本人' AFTER `sort`,
  ADD COLUMN `data_scope_dept_ids` VARCHAR(500) NOT NULL DEFAULT '' COMMENT '自定义数据范围部门ID数组' AFTER `data_scope`,
  ADD COLUMN `type`                TINYINT      NOT NULL DEFAULT 2  COMMENT '角色类型 1系统内置 2自定义' AFTER `data_scope_dept_ids`,
  ADD COLUMN `remark`              VARCHAR(500) NOT NULL DEFAULT '' COMMENT '备注' AFTER `type`,
  ADD COLUMN `deleted`             TINYINT(1)       NOT NULL DEFAULT 0  COMMENT '是否删除' AFTER `remark`,
  ADD COLUMN `creator`             VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '创建者' AFTER `deleted`,
  ADD COLUMN `updater`             VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '更新者' AFTER `creator`;

ALTER TABLE `menus`
  ADD COLUMN `deleted` TINYINT(1)      NOT NULL DEFAULT 0  COMMENT '是否删除' AFTER `status`,
  ADD COLUMN `creator` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '创建者' AFTER `deleted`,
  ADD COLUMN `updater` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '更新者' AFTER `creator`;
```

`20261008000005_roles_menus_columns.down.sql`：

```sql
ALTER TABLE `roles`
  DROP COLUMN `sort`, DROP COLUMN `data_scope`, DROP COLUMN `data_scope_dept_ids`,
  DROP COLUMN `type`, DROP COLUMN `remark`, DROP COLUMN `deleted`,
  DROP COLUMN `creator`, DROP COLUMN `updater`;

ALTER TABLE `menus`
  DROP COLUMN `deleted`, DROP COLUMN `creator`, DROP COLUMN `updater`;
```

- [ ] **Step 4: 更新模型**

`modules/system/model/model.go` 的 `User` 改为：

```go
type User struct {
	ID           uint64    `gorm:"primaryKey" json:"id"`
	Username     string    `json:"username"`
	Nickname     string    `json:"nickname"`
	PasswordHash string    `json:"-"`
	Remark       string    `json:"remark"`
	DeptID       uint64    `json:"dept_id"`
	PostIDs      string    `json:"post_ids"`
	Email        string    `json:"email"`
	Mobile       string    `json:"mobile"`
	Sex          int       `json:"sex"`
	Avatar       string    `json:"avatar"`
	Status       int       `json:"status"`
	LoginIP      string    `json:"login_ip"`
	LoginDate    *time.Time `json:"login_date"`
	Deleted      bool      `gorm:"column:deleted" json:"-"`
	TenantID     uint64    `json:"tenant_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TableName：users 已在迁移 20261008000001 重命名为 system_users。
func (User) TableName() string { return "system_users" }
```

（保留原字段顺序语义；`time` 已在 import。）

`modules/system/model/rbac.go` 的 `Role` 改为：

```go
type Role struct {
	ID              uint64 `gorm:"primaryKey" json:"id"`
	Code            string `json:"code"`
	Name            string `json:"name"`
	Sort            int    `json:"sort"`
	DataScope       int    `json:"data_scope"`
	DataScopeDeptIDs string `json:"data_scope_dept_ids"`
	Type            int    `json:"type"`
	Remark          string `json:"remark"`
	Status          int    `json:"status"`
	Deleted         bool   `gorm:"column:deleted" json:"-"`
	TenantID        uint64 `json:"tenant_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
```

`Menu` 追加字段（其余不动）：

```go
	Deleted   bool   `gorm:"column:deleted" json:"-"`
	Updater   string `json:"updater,omitempty"`
	Creator   string `json:"creator,omitempty"`
```

- [ ] **Step 5: 排查旧表名引用**

Run: `grep -rn '"users"\|`users`\|FROM users\|Table("users"' --include="*.go" . | grep -v system_users`
Expected: 无输出（若有，逐处改为 system_users 或依赖 TableName）

- [ ] **Step 6: 验证**

Run: `CGO_ENABLED=0 go build ./... && go vet ./... && CGO_ENABLED=0 go test ./... -count=1`
如本机 3306 有可用 MySQL（`nc -z 127.0.0.1 3306` 成功）：`make migrate` 并确认无报错；否则在报告中注明"迁移未在本地执行，由有 MySQL 的环境执行"
Expected: 构建/vet/测试全绿；`TestUserTableName` PASS

- [ ] **Step 7: Commit**

```bash
git add modules/system/
git commit -m "feat(system): rename users->system_users, add token/tenants tables, enhance roles/menus columns"
```

---

### Task 2: pkg/token 核心（签发/校验/刷新/注销 + 清扫）

**Files:**
- Create: `pkg/token/token.go`（类型与接口）、`pkg/token/service.go`（实现）、`pkg/token/service_test.go`
- Modify: `pkg/config/config.go`（Auth/Deployment 字段）、`pkg/config/loader.go`（defaults + env）

**Interfaces:**
- Consumes: 表 `system_oauth2_access_token`（Task 1）、`tenants`（Task 1）、`config.Config`
- Produces（后续任务的唯一依赖，签名必须逐字一致）：

```go
package token

type UserType int8

const (
	UserTypeAdmin  UserType = 1
	UserTypeMember UserType = 2
)

type Claims struct {
	UserID   uint64
	Username string
	UserType UserType
	TenantID uint64
	DeptID   uint64 // 仅管理员有意义，会员为 0
}

type IssueParams struct {
	UserID   uint64
	UserType UserType
	TenantID uint64
	ClientID string
}

type Issued struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64 // 秒
}

type Issuer interface {
	Issue(ctx context.Context, p IssueParams) (*Issued, error)
	Refresh(ctx context.Context, refreshToken string) (*Issued, error)
	Revoke(ctx context.Context, accessToken string) error
	RevokeAll(ctx context.Context, userType UserType, userID uint64) error
}

type Validator interface {
	Validate(ctx context.Context, accessToken string) (*Claims, error)
}

func NewService(db *gorm.DB, cfg *config.Config) *Service // *Service 同时实现 Issuer 与 Validator
```

- [ ] **Step 1: 写失败测试**

创建 `pkg/token/service_test.go`：

```go
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

// 注销后立即失效（无缓存层，直接反映 DB 行删除——踢人立即失效的验收项）。
func TestRevoke_InvalidatesCache(t *testing.T) {
	s := newTestService(t)
	issued := mustIssue(t, s)
	if _, err := s.Validate(context.Background(), issued.AccessToken); err != nil {
		t.Fatal(err) // 撤销前必须可用
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

// 刷新窗口必须有上界：create_time 超过 refreshExpireDay → Refresh 必须失败
// （若实现删除窗口校验行，本测试必须变红——钉死 refresh_expire_day 非死配置）。
func TestRefresh_WindowExceeded(t *testing.T) {
	s := newTestService(t)
	issued := mustIssue(t, s)
	tooOld := time.Now().AddDate(0, 0, -(cfgRefreshDays + 1))
	if err := s.db.Model(&accessToken{}).
		Where("access_token = ?", issued.AccessToken).
		Update("create_time", tooOld).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.Refresh(context.Background(), issued.RefreshToken); err == nil {
		t.Error("refresh beyond window must fail")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `CGO_ENABLED=0 go test ./pkg/token/ -v -count=1`
Expected: 编译失败 `undefined: Service/NewService/accessToken`

- [ ] **Step 3: 实现**

`pkg/token/token.go`：上方 Step 1 Produces 块中的全部类型与接口（逐字），外加 doc comment：

```go
// Package token 实现 yudao 式 OAuth2 不透明令牌：随机串落库、DB 为权威，
// 支持签发/校验/刷新（轮换）/注销/踢人。单体模式本地直调本包；
// 双服务模式 member-server 经 gRPC 委托 system-server 调用本包（token.Issuer 接口是切换点）。
package token
```

（context/gorm/config import 依接口签名补齐。）

`pkg/token/service.go`：

```go
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
		// Scan 无行时 status 保持 0 → 视为停用/不存在，拒绝。
		if err := s.db.WithContext(ctx).
			Raw("SELECT status FROM tenants WHERE id = ? AND deleted = 0", p.TenantID).
			Scan(&status).Error; err != nil || status != 1 {
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
```

**注意（实现者必须处理）**：上面 Issue 的租户检查写了两遍 Scan——**只保留一个**，最终形态为：

```go
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
```

（`Scan` 查不到行时 `status` 保持零值 0，天然拒绝；`gorm.io/gorm` 的 `Raw().Scan()` 对空结果不报错。）

同文件继续实现 `Validate` / `Refresh` / `Revoke` / `RevokeAll` / `SweepExpired`：

```go
// Validate 查缓存→查库→回填缓存。username/dept_id 从对应用户表加载
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
	_ = s.Revoke(ctx, row.AccessToken)
	issued, err := s.Issue(ctx, IssueParams{
		UserID: row.UserID, UserType: row.UserType,
		TenantID: row.TenantID, ClientID: row.ClientID,
	})
	if err != nil {
		return nil, err
	}
	return issued, nil
}
```

Refresh 的最终形态（窗口校验按 create_time + refreshExpireDay；直接删旧行再签新行，不经过 Revoke，错误路径单一）：

```go
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
```

`pkg/config/config.go` 顶层 `Config` 新增两段（`mapstructure` 对齐）：

```go
	// Auth：OAuth2 令牌与认证模式。
	Auth struct {
		Mode            string `mapstructure:"mode"` // token | jwt（回滚开关）
		AccessExpireHour int    `mapstructure:"access_expire_hour"`
		RefreshExpireDay int    `mapstructure:"refresh_expire_day"`
	} `mapstructure:"auth"`

	// Deployment：mono=单体一键启动（默认）；micro=双服务部署形态。
	Deployment struct {
		Mode string `mapstructure:"mode"`
	} `mapstructure:"deployment"`

	// SystemGRPCAddr：member-server → system-server TokenService 的地址。
	SystemGRPCAddr string `mapstructure:"system_grpc_addr"`
```

`pkg/config/loader.go`：`SetDefault` 区追加 + `applyEnvOverrides` 追加：

```go
	v.SetDefault("auth.mode", "token")
	v.SetDefault("auth.access_expire_hour", 2)
	v.SetDefault("auth.refresh_expire_day", 7)
	v.SetDefault("deployment.mode", "mono")
	v.SetDefault("system_grpc_addr", "127.0.0.1:50051")
```

```go
		{"DEPLOYMENT_MODE", func(v string) { cfg.Deployment.Mode = v }},
		{"SYSTEM_GRPC_ADDR", func(v string) { cfg.SystemGRPCAddr = v }},
```

（沿用现有 `applyEnvOverrides` 的 pairs 结构追加两行。）

- [ ] **Step 4: 运行确认通过**

Run: `CGO_ENABLED=0 go test ./pkg/token/ ./pkg/config/ -v -count=1`
Expected: token 6 个测试 PASS；config 既有 env 覆盖测试仍 PASS

- [ ] **Step 5: 全量验证**

Run: `CGO_ENABLED=0 go build ./... && go vet ./... && CGO_ENABLED=0 go test ./... -count=1`
Expected: 全绿（注意 errcheck：vet 通过但 CI 的 golangci-lint 更严，禁用裸 `_ =` 丢错误）

- [ ] **Step 6: Commit**

```bash
git add pkg/token/ pkg/config/
git commit -m "feat(token): opaque OAuth2 token service with rotate/revoke/cache; auth+deployment config"
```


---

### Task 3: AuthMiddleware 接入 Token 校验（双模式）

**Files:**
- Modify: `pkg/middleware/auth.go`（重构为 `NewAuthMiddleware(AuthDeps)`）、`pkg/auth/auth.go`（Claims 加字段）
- Modify: `alexgo-server/server/http.go`（装配处）、`pkg/middleware/auth_test.go`（适配 + 新测试）
- Modify: `alexgo-server/cmd/main.go`（提供 token.Service 注入 deps——仅装配，不改行为）

**Interfaces:**
- Consumes: `token.Validator`（Task 2）、`cfg.Auth.Mode`
- Produces:

```go
// pkg/middleware/auth.go
type AuthDeps struct {
	Cfg       *config.Config
	Enforcer  *casbin.Enforcer  // nil=不做 Casbin
	Validator token.Validator   // mode=token 时必须非 nil；mode=jwt 时忽略
}

func NewAuthMiddleware(d AuthDeps) gin.HandlerFunc
```

`auth.Claims` 增加字段：`UserType int`、`DeptID uint64`（json tag `user_type`、`dept_id`）。

- [ ] **Step 1: 写失败测试（token 模式 + jwt 模式回归）**

`pkg/middleware/auth_test.go` **保留**现有 `newRouter`/用例结构，做三处修改：

1. 所有 `middleware.AuthMiddleware(cfg, enf)` 调用改为：

```go
r.Use(middleware.NewAuthMiddleware(middleware.AuthDeps{Cfg: cfg, Enforcer: enf, Validator: fakeValidator{claimsFor(cfg)}))
```

其中测试文件顶部新增：

```go
type fakeValidator struct {
	claims *token.Claims
	err    error
}

func (f fakeValidator) Validate(_ context.Context, _ string) (*token.Claims, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.claims, nil
}
```

2. 现有用例的 cfg 设为 **jwt 模式**（保持旧路径覆盖）：`cfg.Auth.Mode = "jwt"` 加进 `testCfg()`。
3. 新增 token 模式用例（追加到同文件）：

```go
// token 模式：合法 validator → 200 且 claims 注入（含 user_type/dept_id）；
// validator 报错 → 401。
func TestAuthMiddleware_TokenMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testCfg()
	cfg.Auth.Mode = "token"

	v := fakeValidator{claims: &token.Claims{
		UserID: 9, Username: "alice", UserType: token.UserTypeAdmin, TenantID: 1, DeptID: 77,
	}}
	r := gin.New()
	r.Use(middleware.NewAuthMiddleware(middleware.AuthDeps{Cfg: cfg, Validator: v}))
	r.GET("/api/admin/system/users", func(c *gin.Context) {
		a, _ := c.Get("claims")
		cl := a.(*auth.Claims)
		c.JSON(200, gin.H{"ut": cl.UserType, "dept": cl.DeptID})
	})

	req := httptest.NewRequest("GET", "/api/admin/system/users", nil)
	req.Header.Set("Authorization", "Bearer whatever")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	if w.Body.String() != `{"dept":77,"ut":1}` {
		t.Errorf("body = %s", w.Body.String())
	}

	// validator 失败 → 401
	r2 := gin.New()
	r2.Use(middleware.NewAuthMiddleware(middleware.AuthDeps{
		Cfg: cfg, Validator: fakeValidator{err: errors.New("bad")},
	}))
	r2.GET("/api/admin/system/users", func(c *gin.Context) { c.Status(200) })
	w2 := httptest.NewRecorder()
	r2.ServeHTTP(w2, httptest.NewRequest("GET", "/api/admin/system/users", nil))
	if w2.Code != 401 {
		t.Errorf("status = %d, want 401", w2.Code)
	}
}
```

（import 需补 `context`、`errors`、`alexGo-cloud/pkg/token`。）

- [ ] **Step 2: 运行确认失败**

Run: `CGO_ENABLED=0 go test ./pkg/middleware/ -v -count=1`
Expected: 编译失败 `undefined: NewAuthMiddleware/AuthDeps`

- [ ] **Step 3: 实现**

`pkg/auth/auth.go` 的 `Claims` 改为：

```go
type Claims struct {
	UserID   uint64 `json:"user_id"`
	Username string `json:"username"`
	TenantID uint64 `json:"tenant_id"`
	UserType int    `json:"user_type"` // 1管理员 2会员（token 模式填充；jwt 模式为 1）
	DeptID   uint64 `json:"dept_id"`   // 仅管理员
}
```

`pkg/middleware/auth.go` 整文件替换为：

```go
package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"

	"alexGo-cloud/pkg/auth"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/token"
)

// AuthDeps 聚合鉴权中间件的可选依赖。
//
// 双模式（cfg.Auth.Mode）：
// - token（默认）：不透明令牌经 Validator 查库校验，claims 含 user_type/dept_id；
// - jwt：兼容旧路径（静态密钥 JWT），作为回滚开关。
// Enforcer/Validator 均可为 nil：nil Enforcer 跳过 Casbin；mode=token 但 Validator 为 nil
// 属装配错误，直接 501 提示（fail-closed，绝不放行）。
type AuthDeps struct {
	Cfg       *config.Config
	Enforcer  *casbin.Enforcer
	Validator token.Validator
}

func NewAuthMiddleware(d AuthDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		// 基础运维接口：默认放行（生产可结合网关/内网隔离）。
		if path == "/health" || path == "/health/ready" || path == "/metrics" || strings.HasPrefix(path, "/debug/pprof/") {
			c.Next()
			return
		}
		if !strings.HasPrefix(path, "/api/admin/") && !strings.HasPrefix(path, "/api/app/member/") {
			// 约定扩展：/api/app/member/** 的登录态接口同样需要校验（登录/注册端点本身在放行清单，见下）。
			if !isPublicAppPath(path) {
				c.Next()
				return
			}
			c.Next()
			return
		}
		... // 完整逻辑见 Step 3a/3b 展开
	}
}
```

**Step 3a（上面的函数体——完整形态，替换 `...`）**：

```go
func NewAuthMiddleware(d AuthDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if path == "/health" || path == "/health/ready" || path == "/metrics" ||
			strings.HasPrefix(path, "/debug/pprof/") {
			c.Next()
			return
		}
		// 公开路径：非 /api/admin/** 且非需登录的 app 接口一律放行。
		// /api/app/** 中仅本清单需要登录态（登录/注册等入口是公开的）。
		if !strings.HasPrefix(path, "/api/admin/") && !appAuthRequired(path) {
			c.Next()
			return
		}

		tokenStr := c.GetHeader("Authorization")
		claims, err := parseClaims(d, tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Set("claims", claims)

		if d.Enforcer != nil {
			// sub 带租户前缀：{tenantId}:{username}，杜绝跨租户同名角色串策略（块⑦）。
			sub := fmt.Sprintf("%d:%s", claims.TenantID, claims.Username)
			if claims.Username == "" {
				sub = fmt.Sprintf("%d:%d", claims.TenantID, claims.UserID)
			}
			obj := c.FullPath()
			if obj == "" {
				obj = path
			}
			ok, eerr := d.Enforcer.Enforce(sub, obj, c.Request.Method)
			if eerr != nil || !ok {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
		}
		c.Next()
	}
}

// appAuthRequired：app 端需要登录态的路径前缀（一期只有 member 的登出/刷新；
// 登录、注册、send-sms-code 等入口公开）。
func appAuthRequired(path string) bool {
	return strings.HasPrefix(path, "/api/app/member/auth/logout") ||
		strings.HasPrefix(path, "/api/app/member/auth/refresh")
}

func parseClaims(d AuthDeps, tokenStr string) (*auth.Claims, error) {
	if d.Cfg != nil && d.Cfg.Auth.Mode == "jwt" {
		return auth.ParseToken(tokenStr, d.Cfg) // 旧路径：Bearer JWT
	}
	// token 模式（默认）
	if d.Validator == nil {
		return nil, fmt.Errorf("auth: validator not wired (mode=token)")
	}
	if tokenStr == "" {
		return nil, fmt.Errorf("auth: empty token")
	}
	raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(tokenStr), "Bearer "))
	tc, err := d.Validator.Validate(context.Background(), raw)
	if err != nil {
		return nil, err
	}
	return &auth.Claims{
		UserID:   tc.UserID,
		Username: tc.Username,
		TenantID: tc.TenantID,
		UserType: int(tc.UserType),
		DeptID:   tc.DeptID,
	}, nil
}
```

（import 补 `context`；`auth.ParseToken` 已有。注意 jwt 模式下 `ParseToken` 接受裸 token/Bearer 两种，行为不变。）

**Step 3b（装配）**：`alexgo-server/server/http.go` 的 `HTTPServerParams` 增加：

```go
	// TokenValidator：mode=token 时的令牌校验器（fx 由 token.NewService 提供）。
	TokenValidator token.Validator `optional:"true"`
```

`StartHTTPServer` 中间件链里把

```go
		middleware.AuthMiddleware(p.Cfg, p.Enforcer),
```

替换为

```go
		middleware.NewAuthMiddleware(middleware.AuthDeps{Cfg: p.Cfg, Enforcer: p.Enforcer, Validator: p.TokenValidator}),
```

（import 补 `alexGo-cloud/pkg/token`。）

**Step 3c（提供者）**：`alexgo-server/cmd/main.go` 的 `fx.Provide` 块新增（紧邻 `database.NewDB` 之后）：

```go
			// OAuth2 令牌服务：同时作为 Issuer（签发）与 Validator（中间件校验）。
			token.NewService,
```

（import `alexGo-cloud/pkg/token`；`token.NewService(db, cfg)` 的参数 FX 自动注入 `*gorm.DB`/`*config.Config`。注意 main 当前用 `fx.Supply(cfg)` 还是 `Provide(LoadGlobalConfig)`：**保持现状**（Provide），Task 12 才改 Supply。）

- [ ] **Step 4: 运行确认通过**

Run: `CGO_ENABLED=0 go test ./pkg/middleware/ ./pkg/auth/ -v -count=1`
Expected: 旧用例（jwt 模式）+ 新 token 模式用例全 PASS

- [ ] **Step 5: 全量验证并检查其他调用点**

Run: `grep -rn "AuthMiddleware(" --include="*.go" . | grep -v NewAuthMiddleware`
Expected: 无残留旧调用（若有——逐处改 `NewAuthMiddleware(AuthDeps{...})`）
Run: `CGO_ENABLED=0 go build ./... && go vet ./... && CGO_ENABLED=0 go test ./... -count=1`
Expected: 全绿

- [ ] **Step 6: Commit**

```bash
git add pkg/middleware/ pkg/auth/ alexgo-server/ cmd 2>/dev/null; git add pkg/middleware/ pkg/auth/ alexgo-server/
git commit -m "feat(auth): AuthDeps middleware supporting token/jwt modes with tenant-prefixed casbin sub"
```

---

### Task 4: 登录签发改造（LoginResult + refresh/logout 端点）

**Files:**
- Modify: `modules/system/service/auth.go`（Login 签名 + 签发走 token.Issuer）
- Modify: `modules/system/controller/app/controller.go`（响应结构）
- Modify: `modules/system/controller/admin/auth.go`（新增 Refresh/Logout handler）
- Modify: `modules/system/module.go`（新路由）
- Create: `modules/system/service/auth_test.go`

**Interfaces:**
- Consumes: `token.Issuer`（Task 2）、`auth.Claims`（Task 3）
- Produces:

```go
// modules/system/service
type LoginResult struct {
	AccessToken  string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type AuthService interface {
	Login(ctx context.Context, username, password string) (*LoginResult, *model.User, error)
	Refresh(ctx context.Context, refreshToken string) (*LoginResult, error)
	Logout(ctx context.Context, accessToken string) error
}
func NewAuthService(cfg *config.Config, userRepo repository.UserRepository, permSvc PermissionService, issuer token.Issuer) AuthService
```

- [ ] **Step 1: 写失败测试**

创建 `modules/system/service/auth_test.go`：

```go
package service

import (
	"context"
	"errors"
	"testing"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/pkg/token"
)

type fakeIssuer struct {
	issued  *token.Issued
	issueErr error
	revoked []string
}

func (f *fakeIssuer) Issue(_ context.Context, p token.IssueParams) (*token.Issued, error) {
	if f.issueErr != nil {
		return nil, f.issueErr
	}
	return f.issued, nil
}
func (f *fakeIssuer) Refresh(context.Context, string) (*token.Issued, error) {
	return f.issued, nil
}
func (f *fakeIssuer) Revoke(_ context.Context, at string) error {
	f.revoked = append(f.revoked, at)
	return nil
}
func (f *fakeIssuer) RevokeAll(context.Context, token.UserType, uint64) error { return nil }

type fakeUserRepo struct{ user *model.User }

func (f *fakeUserRepo) List(context.Context, uint64) ([]*model.User, error) { return nil, nil }
func (f *fakeUserRepo) GetByID(context.Context, uint64, uint64) (*model.User, error) {
	return nil, errors.New("not found")
}
func (f *fakeUserRepo) GetByUsername(_ context.Context, _ uint64, _ string) (*model.User, error) {
	if f.user == nil {
		return nil, errors.New("not found")
	}
	return f.user, nil
}
func (f *fakeUserRepo) Create(context.Context, *model.User) error { return nil }
func (f *fakeUserRepo) Update(context.Context, *model.User) error { return nil }

// fakePerm implements PermissionService zero-values（Login 内仅调用 EnsureUserRolePolicy）。
type fakePerm struct{}

func (fakePerm) UserRoles(context.Context, uint64) ([]*model.Role, error)          { return nil, nil }
func (fakePerm) UserMenus(context.Context, uint64) ([]*model.Menu, error)          { return nil, nil }
func (fakePerm) UserPermCodes(context.Context, uint64) ([]string, error)           { return nil, nil }
func (fakePerm) UserRoutes(context.Context, uint64) ([]*VbenRoute, error)          { return nil, nil }
func (fakePerm) EnsureUserRolePolicy(context.Context, string, []*model.Role) error { return nil }

func enabledUser() *model.User {
	return &model.User{ID: 3, Username: "alice", Status: 1, PasswordHash: hashOf("pw")}
}

func hashOf(pw string) string {
	h, _ := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(h)
}
```

（import 补 `golang.org/x/crypto/bcrypt`。）

用例部分：

```go
func TestLogin_IssuesToken(t *testing.T) {
	cfg := &config.Config{}
	cfg.Auth.Mode = "token"
	iss := &fakeIssuer{issued: &token.Issued{AccessToken: "a1", RefreshToken: "r1", ExpiresIn: 7200}}
	svc := NewAuthService(cfg, &fakeUserRepo{user: enabledUser()}, fakePerm{}, iss)

	res, u, err := svc.Login(context.Background(), "alice", "pw")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if res.AccessToken != "a1" || res.RefreshToken != "r1" || res.ExpiresIn != 7200 {
		t.Errorf("res = %+v", res)
	}
	if u.ID != 3 {
		t.Errorf("user = %+v", u)
	}
}

func TestLogin_BadPassword_NoIssue(t *testing.T) {
	cfg := &config.Config{}
	iss := &fakeIssuer{}
	svc := NewAuthService(cfg, &fakeUserRepo{user: enabledUser()}, fakePerm{}, iss)
	if _, _, err := svc.Login(context.Background(), "alice", "wrong"); err == nil {
		t.Fatal("wrong password must fail")
	}
	if iss.issued != nil {
		t.Error("must not issue on bad password（issueErr guard）")
	}
}

func TestLogout_Revoke(t *testing.T) {
	iss := &fakeIssuer{}
	svc := NewAuthService(&config.Config{}, &fakeUserRepo{}, fakePerm{}, iss)
	if err := svc.Logout(context.Background(), "a1"); err != nil {
		t.Fatal(err)
	}
	if len(iss.revoked) != 1 || iss.revoked[0] != "a1" {
		t.Errorf("revoked = %v", iss.revoked)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `CGO_ENABLED=0 go test ./modules/system/service/ -run TestLogin -v`
Expected: 编译失败（Login 签名/NewAuthService 参数不符）

- [ ] **Step 3: 实现 service**

`modules/system/service/auth.go` 整文件替换：

```go
package service

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/repository"
	"alexGo-cloud/pkg/auth"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/token"
)

// LoginResult 是双端登录的统一响应（controller 直接序列化）。
type LoginResult struct {
	AccessToken  string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type AuthService interface {
	Login(ctx context.Context, username, password string) (*LoginResult, *model.User, error)
	Refresh(ctx context.Context, refreshToken string) (*LoginResult, error)
	Logout(ctx context.Context, accessToken string) error
}

type authService struct {
	cfg      *config.Config
	userRepo repository.UserRepository
	permSvc  PermissionService
	issuer   token.Issuer
}

func NewAuthService(cfg *config.Config, userRepo repository.UserRepository, permSvc PermissionService, issuer token.Issuer) AuthService {
	return &authService{cfg: cfg, userRepo: userRepo, permSvc: permSvc, issuer: issuer}
}

func (s *authService) clientID() string { return "alexgo-admin" }

func toResult(i *token.Issued) *LoginResult {
	return &LoginResult{AccessToken: i.AccessToken, RefreshToken: i.RefreshToken, ExpiresIn: i.ExpiresIn}
}

func (s *authService) Login(ctx context.Context, username, password string) (*LoginResult, *model.User, error) {
	if username == "" || password == "" {
		return nil, nil, fmt.Errorf("username or password is empty")
	}
	tid := tenant.TenantIDFromContext(ctx)
	u, err := s.userRepo.GetByUsername(ctx, tid, username)
	if err != nil {
		return nil, nil, err
	}
	if u.Status != 1 {
		return nil, nil, fmt.Errorf("user disabled")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, nil, fmt.Errorf("invalid credentials")
	}
	if s.permSvc != nil {
		roles, _ := s.permSvc.UserRoles(ctx, u.ID)
		_ = s.permSvc.EnsureUserRolePolicy(ctx, u.Username, roles)
	}
	issued, err := s.issuer.Issue(ctx, token.IssueParams{
		UserID: u.ID, UserType: token.UserTypeAdmin, TenantID: tid, ClientID: s.clientID(),
	})
	if err != nil {
		return nil, nil, err
	}
	return toResult(issued), u, nil
}

func (s *authService) Refresh(ctx context.Context, refreshToken string) (*LoginResult, error) {
	issued, err := s.issuer.Refresh(ctx, refreshToken)
	if err != nil {
		return nil, err
	}
	return toResult(issued), nil
}

func (s *authService) Logout(ctx context.Context, accessToken string) error {
	return s.issuer.Revoke(ctx, accessToken)
}
```

（`permSvc.EnsureUserRolePolicy` 的 `_ =` 若 golangci 报 errcheck——**必须改**为：

```go
		if _, perr := s.permSvc.UserRoles(ctx, u.ID); true {
			_ = perr
		}
```

不对——正确处理：错误显式判断并忽略需注释；采用：

```go
	if s.permSvc != nil {
		roles, _ := s.permSvc.UserRoles(ctx, u.ID) // 权限同步失败不阻断登录（下次请求仍会走中间件校验）
		if perr := s.permSvc.EnsureUserRolePolicy(ctx, u.Username, roles); perr != nil && logger.Log != nil {
			logger.Log.Warn("ensure user role policy failed", zap.Error(perr))
		}
	}
```

import 补 `alexGo-cloud/pkg/logger`、`go.uber.org/zap`。`UserRoles` 的 `roles, _` 同样会被 errcheck 扫吗——`_, _ =` 形式或改：

```go
		roles, rerr := s.permSvc.UserRoles(ctx, u.ID)
		if rerr != nil && logger.Log != nil {
			logger.Log.Warn("load user roles failed", zap.Error(rerr))
		}
```

以最终 golangci-lint 通过为准。）

- [ ] **Step 4: controller 与路由**

`modules/system/controller/app/controller.go` 的 `Login` 成功分支改为：

```go
	res, u, err := ctrl.authSvc.Login(c.Request.Context(), req.Username, req.Password)
	... // 错误/审计逻辑不变
	c.JSON(http.StatusOK, gin.H{
		"token": res.AccessToken, "refresh_token": res.RefreshToken, "expires_in": res.ExpiresIn,
	})
```

（注意原变量 `token, u, err :=` 改名 `res, u, err :=`；`gin.H{"token": token}` 中的 token 现在是字符串名冲突——改用 `res` 后无冲突。）

`modules/system/controller/admin/auth.go` 追加两个 handler：

```go
type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// Refresh 用 refresh_token 换取新令牌对。
func (c *AuthController) Refresh(ctx *gin.Context) {
	var req refreshRequest
	if err := ctx.ShouldBindJSON(&req); err != nil || req.RefreshToken == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "refresh_token required"})
		return
	}
	res, err := c.authSvc.Refresh(ctx.Request.Context(), req.RefreshToken)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"token": res.AccessToken, "refresh_token": res.RefreshToken, "expires_in": res.ExpiresIn,
	})
}

// Logout 注销当前 access token（从 Authorization 头取）。
func (c *AuthController) Logout(ctx *gin.Context) {
	raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(ctx.GetHeader("Authorization")), "Bearer "))
	if raw == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "token required"})
		return
	}
	if err := c.authSvc.Logout(ctx.Request.Context(), raw); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}
```

（`AuthController` 需要 `authSvc` 字段——**检查其现有结构体**：若无，`NewAuthController` 增加 `authSvc service.AuthService` 参数并在 `module.go` 的 Provide/构造处传入；import 补 `strings`、`service` 已有。）

`modules/system/module.go` 的 `RegisterRoutes` 中 appGroup 追加：

```go
		appGroup.POST("/auth/refresh", m.authCtrl.Refresh)
		appGroup.POST("/auth/logout", m.authCtrl.Logout)
```

adminGroup 同样追加（管理端同端点）：

```go
		adminGroup.POST("/auth/refresh", m.authCtrl.Refresh)
		adminGroup.POST("/auth/logout", m.authCtrl.Logout)
```

- [ ] **Step 5: 运行确认通过**

Run: `CGO_ENABLED=0 go test ./modules/system/... -v -count=1`
Expected: auth service 测试 PASS（若 `system` 包编译因 AuthController 依赖变化报错，同步修 module.go 装配）

- [ ] **Step 6: 全量验证 + 前端兼容性检查**

Run: `grep -rn "\.token\|refresh_token" admin-web/src/views/LoginPage.vue admin-web/src/api/admin.ts | head`
Expected: 登录页读 `res.token` —— 响应新增字段向后兼容，**无需改前端**（若发现读取整个对象结构体类型报错，按实际补 `refresh_token?: string`）
Run: `CGO_ENABLED=0 go build ./... && go vet ./... && CGO_ENABLED=0 go test ./... -count=1`
Expected: 全绿

- [ ] **Step 7: Commit**

```bash
git add modules/system/
git commit -m "feat(system): login issues opaque tokens, add refresh/logout endpoints"
```

---

### Task 5: member 模块骨架（model/repo/service/controller + 独立迁移 + mono 装配）

**Files:**
- Create: `modules/member/migrations/20261008000001_init_member.up.sql` / `.down.sql`
- Create: `modules/member/migration_source.go`
- Create: `modules/member/model/member_user.go`
- Create: `modules/member/repository/member_repo.go`
- Create: `modules/member/service/member.go`（+ 测试 `member_test.go`）
- Create: `modules/member/controller/app/member_auth.go`、`controller/admin/member_user.go`
- Create: `modules/member/module.go`
- Modify: `alexgo-server/cmd/main.go`（装配 member.FxModule——Task 12 才按模式拆分，本任务先无条件装入）

**Interfaces:**
- Consumes: `token.Issuer`（Task 2/3 已提供）、租户上下文（现有 TenantMiddleware）
- Produces:

```go
// modules/member/model
type MemberUser struct {
	ID         uint64    `gorm:"primaryKey" json:"id"`
	Nickname   string    `json:"nickname"`
	Avatar     string    `json:"avatar"`
	Status     int       `json:"status"` // 1启用 0停用
	Mobile     string    `json:"mobile"`
	Password   string    `json:"-"` // bcrypt
	RegisterIP string    `json:"register_ip"`
	LoginIP    string    `json:"login_ip"`
	LoginDate  *time.Time `json:"login_date"`
	Deleted    bool      `gorm:"column:deleted" json:"-"`
	TenantID   uint64    `json:"tenant_id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
func (MemberUser) TableName() string { return "member_user" }

// modules/member/service
type LoginResult struct { AccessToken, RefreshToken string; ExpiresIn int64 } // 与 system 同形
type MemberService interface {
	Register(ctx context.Context, mobile, password, nickname, ip string) (*model.MemberUser, error)
	Login(ctx context.Context, mobile, password, ip string) (*LoginResult, error)
	Refresh(ctx context.Context, refreshToken string) (*LoginResult, error)
	Logout(ctx context.Context, accessToken string) error
	List(ctx context.Context, page, size int) ([]*model.MemberUser, int64, error)
	Disable(ctx context.Context, id uint64, status int) error
}
```

路由（`module.go` RegisterRoutes）：
- `POST /api/app/member/auth/register`
- `POST /api/app/member/auth/login`
- `POST /api/app/member/auth/refresh`、`POST /api/app/member/auth/logout`
- `GET /api/admin/member/users`、`PUT /api/admin/member/users/:id/status`

- [ ] **Step 1: 写失败测试**

创建 `modules/member/service/member_test.go`：

```go
package service

import (
	"context"
	"errors"
	"testing"

	"alexGo-cloud/modules/member/model"
	"alexGo-cloud/pkg/token"
)

type fakeIssuer struct{ revoked []string }

func (f *fakeIssuer) Issue(_ context.Context, p token.IssueParams) (*token.Issued, error) {
	if p.UserType != token.UserTypeMember {
		return nil, errors.New("wrong user type")
	}
	return &token.Issued{AccessToken: "ma", RefreshToken: "mr", ExpiresIn: 7200}, nil
}
func (f *fakeIssuer) Refresh(context.Context, string) (*token.Issued, error) {
	return &token.Issued{AccessToken: "ma2", RefreshToken: "mr2", ExpiresIn: 7200}, nil
}
func (f *fakeIssuer) Revoke(_ context.Context, at string) error {
	f.revoked = append(f.revoked, at)
	return nil
}
func (f *fakeIssuer) RevokeAll(context.Context, token.UserType, uint64) error { return nil }

// memRepo：进程内 map 实现 MemberRepository（测试专用）。
type memRepo struct {
	byMobile map[string]*model.MemberUser
	nextID   uint64
}

func newMemRepo() *memRepo { return &memRepo{byMobile: map[string]*model.MemberUser{}} }

func (m *memRepo) GetByMobile(_ context.Context, tid uint64, mobile string) (*model.MemberUser, error) {
	u, ok := m.byMobile[key(tid, mobile)]
	if !ok {
		return nil, errors.New("not found")
	}
	return u, nil
}
func (m *memRepo) Create(_ context.Context, u *model.MemberUser) error {
	m.nextID++
	u.ID = m.nextID
	m.byMobile[key(u.TenantID, u.Mobile)] = u
	return nil
}
func (m *memRepo) Update(_ context.Context, u *model.MemberUser) error {
	m.byMobile[key(u.TenantID, u.Mobile)] = u
	return nil
}
func (m *memRepo) List(_ context.Context, tid uint64, page, size int) ([]*model.MemberUser, int64, error) {
	var out []*model.MemberUser
	for _, u := range m.byMobile {
		if u.TenantID == tid {
			out = append(out, u)
		}
	}
	return out, int64(len(out)), nil
}
func (m *memRepo) CountByTenant(_ context.Context, tid uint64) (int64, error) {
	var n int64
	for _, u := range m.byMobile {
		if u.TenantID == tid {
			n++
		}
	}
	return n, nil
}

func key(tid uint64, mobile string) string { return fmt.Sprintf("%d:%s", tid, mobile) }

func TestRegister_ThenLogin(t *testing.T) {
	repo := newMemRepo()
	iss := &fakeIssuer{}
	svc := NewMemberService(repo, iss)

	u, err := svc.Register(context.Background(), "13800000000", "pw123456", "", "1.2.3.4")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if u.Mobile != "13800000000" || u.Status != 1 {
		t.Errorf("u = %+v", u)
	}

	res, err := svc.Login(context.Background(), "13800000000", "pw123456", "1.2.3.4")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if res.AccessToken != "ma" || res.ExpiresIn != 7200 {
		t.Errorf("res = %+v", res)
	}
}

func TestRegister_DuplicateMobile(t *testing.T) {
	repo := newMemRepo()
	svc := NewMemberService(repo, &fakeIssuer{})
	if _, err := svc.Register(context.Background(), "13800000000", "pw", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Register(context.Background(), "13800000000", "pw", "", ""); err == nil {
		t.Error("duplicate mobile must fail")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	repo := newMemRepo()
	svc := NewMemberService(repo, &fakeIssuer{})
	if _, err := svc.Register(context.Background(), "13800000000", "pw123456", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Login(context.Background(), "13800000000", "bad", ""); err == nil {
		t.Error("wrong password must fail")
	}
}

func TestLogout_Revoke(t *testing.T) {
	iss := &fakeIssuer{}
	svc := NewMemberService(newMemRepo(), iss)
	if err := svc.Logout(context.Background(), "ma"); err != nil {
		t.Fatal(err)
	}
	if len(iss.revoked) != 1 {
		t.Errorf("revoked = %v", iss.revoked)
	}
}
```

（import 补 `fmt`。mobile 校验：`1[3-9]\d{9}` 正则，测试手机号合法。）

- [ ] **Step 2: 运行确认失败**

Run: `CGO_ENABLED=0 go test ./modules/member/... -v -count=1`
Expected: 编译失败 `undefined: NewMemberService/MemberRepository`

- [ ] **Step 3: 迁移与模型**

`modules/member/migrations/20261008000001_init_member.up.sql`：

```sql
CREATE TABLE IF NOT EXISTS `member_user` (
  `id`          BIGINT       NOT NULL AUTO_INCREMENT,
  `nickname`    VARCHAR(30)  NOT NULL DEFAULT '' COMMENT '用户昵称',
  `avatar`      VARCHAR(255) NOT NULL DEFAULT '' COMMENT '用户头像',
  `status`      TINYINT      NOT NULL DEFAULT 1  COMMENT '状态 1启用 0停用',
  `mobile`      VARCHAR(11)  NOT NULL DEFAULT '' COMMENT '用户手机号（登录账号）',
  `password`    VARCHAR(100) NOT NULL DEFAULT '' COMMENT '密码 bcrypt，可为空=未设密码',
  `register_ip` VARCHAR(32)  NOT NULL DEFAULT '' COMMENT '注册IP',
  `login_ip`    VARCHAR(50)  NOT NULL DEFAULT '' COMMENT '最近登录IP',
  `login_date`  DATETIME     NULL COMMENT '最近登录时间',
  `deleted`     TINYINT(1)       NOT NULL DEFAULT 0  COMMENT '是否删除（一期不启用软删）',
  `creator`     VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '创建者',
  `create_time` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updater`     VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '更新者',
  `update_time` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `tenant_id`   BIGINT       NOT NULL DEFAULT 0  COMMENT '租户编号',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_mobile_tenant` (`mobile`, `tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='会员用户表';
```

down：`DROP TABLE IF EXISTS \`member_user\`;`

`modules/member/model/member_user.go`：上方 Produces 的 `MemberUser` 结构（import time）。

`modules/member/migration_source.go`：

```go
package member

import "alexGo-cloud/pkg/migrate"

type migrationSource struct{}

func NewMigrationSource() migrate.Source { return &migrationSource{} }

func (s *migrationSource) GetMigrationPath() string { return "modules/member/migrations" }
func (s *migrationSource) GetModuleName() string    { return "member" }
```

- [ ] **Step 4: 仓储**

`modules/member/repository/member_repo.go`：

```go
package repository

import (
	"context"

	"gorm.io/gorm"

	"alexGo-cloud/modules/member/model"
)

// MemberRepository 会员用户仓储。
type MemberRepository interface {
	GetByMobile(ctx context.Context, tenantID uint64, mobile string) (*model.MemberUser, error)
	Create(ctx context.Context, u *model.MemberUser) error
	Update(ctx context.Context, u *model.MemberUser) error
	List(ctx context.Context, tenantID uint64, page, size int) ([]*model.MemberUser, int64, error)
	CountByTenant(ctx context.Context, tenantID uint64) (int64, error)
}

type memberRepo struct{ db *gorm.DB }

func NewMemberRepository(db *gorm.DB) MemberRepository { return &memberRepo{db: db} }

func (r *memberRepo) GetByMobile(ctx context.Context, tid uint64, mobile string) (*model.MemberUser, error) {
	var u model.MemberUser
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND mobile = ? AND deleted = 0", tid, mobile).
		First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *memberRepo) Create(ctx context.Context, u *model.MemberUser) error {
	return r.db.WithContext(ctx).Create(u).Error
}

func (r *memberRepo) Update(ctx context.Context, u *model.MemberUser) error {
	return r.db.WithContext(ctx).Save(u).Error
}

func (r *memberRepo) List(ctx context.Context, tid uint64, page, size int) ([]*model.MemberUser, int64, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	var total int64
	q := r.db.WithContext(ctx).Model(&model.MemberUser{}).Where("tenant_id = ? AND deleted = 0", tid)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []*model.MemberUser
	err := q.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&items).Error
	return items, total, err
}

func (r *memberRepo) CountByTenant(ctx context.Context, tid uint64) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.MemberUser{}).
		Where("tenant_id = ? AND deleted = 0", tid).Count(&n).Error
	return n, err
}
```

- [ ] **Step 5: 服务**

`modules/member/service/member.go`：

```go
package service

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"golang.org/x/crypto/bcrypt"

	"alexGo-cloud/modules/member/model"
	"alexGo-cloud/modules/member/repository"
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/token"
)

var mobileRe = regexp.MustCompile(`^1[3-9]\d{9}$`)

type LoginResult struct {
	AccessToken  string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type MemberService interface {
	Register(ctx context.Context, mobile, password, nickname, ip string) (*model.MemberUser, error)
	Login(ctx context.Context, mobile, password, ip string) (*LoginResult, error)
	Refresh(ctx context.Context, refreshToken string) (*LoginResult, error)
	Logout(ctx context.Context, accessToken string) error
	List(ctx context.Context, page, size int) ([]*model.MemberUser, int64, error)
	Disable(ctx context.Context, id uint64, status int) error
}

type memberService struct {
	repo   repository.MemberRepository
	issuer token.Issuer
}

func NewMemberService(repo repository.MemberRepository, issuer token.Issuer) MemberService {
	return &memberService{repo: repo, issuer: issuer}
}

func toResult(i *token.Issued) *LoginResult {
	return &LoginResult{AccessToken: i.AccessToken, RefreshToken: i.RefreshToken, ExpiresIn: i.ExpiresIn}
}

func (s *memberService) Register(ctx context.Context, mobile, password, nickname, ip string) (*model.MemberUser, error) {
	if !mobileRe.MatchString(mobile) {
		return nil, fmt.Errorf("invalid mobile")
	}
	if len(password) < 8 {
		return nil, fmt.Errorf("password too short (min 8)")
	}
	tid := tenant.TenantIDFromContext(ctx)
	if _, err := s.repo.GetByMobile(ctx, tid, mobile); err == nil {
		return nil, fmt.Errorf("mobile already registered")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	if nickname == "" {
		// 无昵称时用手机号尾 4 位生成占位昵称。
		nickname = "用户" + mobile[len(mobile)-4:]
	}
	now := time.Now()
	u := &model.MemberUser{
		Nickname: nickname, Status: 1, Mobile: mobile,
		Password: string(hash), RegisterIP: ip, LoginDate: &now,
		TenantID: tid,
	}
	if err := s.repo.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *memberService) Login(ctx context.Context, mobile, password, ip string) (*LoginResult, error) {
	tid := tenant.TenantIDFromContext(ctx)
	u, err := s.repo.GetByMobile(ctx, tid, mobile)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	if u.Status != 1 {
		return nil, fmt.Errorf("user disabled")
	}
	if bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)) != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	now := time.Now()
	u.LoginIP, u.LoginDate = ip, &now
	if err := s.repo.Update(ctx, u); err != nil {
		return nil, err
	}
	issued, err := s.issuer.Issue(ctx, token.IssueParams{
		UserID: u.ID, UserType: token.UserTypeMember, TenantID: tid, ClientID: "alexgo-app",
	})
	if err != nil {
		return nil, err
	}
	return toResult(issued), nil
}

func (s *memberService) Refresh(ctx context.Context, refreshToken string) (*LoginResult, error) {
	issued, err := s.issuer.Refresh(ctx, refreshToken)
	if err != nil {
		return nil, err
	}
	return toResult(issued), nil
}

func (s *memberService) Logout(ctx context.Context, accessToken string) error {
	return s.issuer.Revoke(ctx, accessToken)
}

func (s *memberService) List(ctx context.Context, page, size int) ([]*model.MemberUser, int64, error) {
	return s.repo.List(ctx, tenant.TenantIDFromContext(ctx), page, size)
}

// Disable 启用/停用会员（status: 1启用 0停用）；停用不吊销既有 token——
// 中间件 Validate 查用户存在性，但状态检查在 login 层——既有 token 生命周期内仍有效
// （一期取舍：踢人场景由 RevokeAll 覆盖，本接口后续接 RevokeAll 联动）。
func (s *memberService) Disable(ctx context.Context, id uint64, status int) error {
	// 仓储无按 ID 直取——用 Update 语义按 tenant+id 局部更新。
	return s.repo.UpdateStatus(ctx, tenant.TenantIDFromContext(ctx), id, status)
}
```

（**接口一致性**：`Disable` 用到 `UpdateStatus`——把 `MemberRepository` 接口与 memRepo 同步加方法：

```go
	UpdateStatus(ctx context.Context, tenantID, id uint64, status int) error
```

实现：

```go
func (r *memberRepo) UpdateStatus(ctx context.Context, tid, id uint64, status int) error {
	return r.db.WithContext(ctx).
		Model(&model.MemberUser{}).
		Where("tenant_id = ? AND id = ? AND deleted = 0", tid, id).
		Update("status", status).Error
}
```

memRepo 测试实现：

```go
func (m *memRepo) UpdateStatus(_ context.Context, tid, id uint64, status int) error {
	for _, u := range m.byMobile {
		if u.TenantID == tid && u.ID == id {
			u.Status = status
			return nil
		}
	}
	return errors.New("not found")
}
```

同时 Step 5 正文的 `Disable` 保持上面形态。）

- [ ] **Step 6: 控制器与模块**

`modules/member/controller/app/member_auth.go`：

```go
package app

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/member/service"
)

type AuthController struct {
	svc service.MemberService
}

func NewAuthController(svc service.MemberService) *AuthController { return &AuthController{svc: svc} }

type registerRequest struct {
	Mobile   string `json:"mobile"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
}

type loginRequest struct {
	Mobile   string `json:"mobile"`
	Password string `json:"password"`
}

func errJSON(c *gin.Context, code int, err error) {
	c.JSON(code, gin.H{"error": err.Error()})
}

func (c *AuthController) Register(ctx *gin.Context) {
	var req registerRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		errJSON(ctx, http.StatusBadRequest, err)
		return
	}
	u, err := c.svc.Register(ctx.Request.Context(), req.Mobile, req.Password, req.Nickname, ctx.ClientIP())
	if err != nil {
		errJSON(ctx, http.StatusBadRequest, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": u})
}

func (c *AuthController) Login(ctx *gin.Context) {
	var req loginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		errJSON(ctx, http.StatusBadRequest, err)
		return
	}
	res, err := c.svc.Login(ctx.Request.Context(), req.Mobile, req.Password, ctx.ClientIP())
	if err != nil {
		errJSON(ctx, http.StatusUnauthorized, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"token": res.AccessToken, "refresh_token": res.RefreshToken, "expires_in": res.ExpiresIn,
	})
}

func (c *AuthController) Refresh(ctx *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil || req.RefreshToken == "" {
		errJSON(ctx, http.StatusBadRequest, err)
		return
	}
	res, err := c.svc.Refresh(ctx.Request.Context(), req.RefreshToken)
	if err != nil {
		errJSON(ctx, http.StatusUnauthorized, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"token": res.AccessToken, "refresh_token": res.RefreshToken, "expires_in": res.ExpiresIn,
	})
}

func (c *AuthController) Logout(ctx *gin.Context) {
	raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(ctx.GetHeader("Authorization")), "Bearer "))
	if raw == "" {
		errJSON(ctx, http.StatusBadRequest, fmt.Errorf("token required"))
		return
	}
	if err := c.svc.Logout(ctx.Request.Context(), raw); err != nil {
		errJSON(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}
```

（import 补 `fmt`。`Refresh` 中 bind err 为 nil 但 token 空时 `errJSON(..., err)` 的 err 是 nil——**修正**：显式 `fmt.Errorf("refresh_token required")`。）

`modules/member/controller/admin/member_user.go`：

```go
package admin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/member/service"
)

type UserController struct {
	svc service.MemberService
}

func NewUserController(svc service.MemberService) *UserController { return &UserController{svc: svc} }

func (c *UserController) List(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(ctx.DefaultQuery("size", "20"))
	items, total, err := c.svc.List(ctx.Request.Context(), page, size)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": items, "total": total})
}

func (c *UserController) UpdateStatus(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	var req struct {
		Status int `json:"status"`
	}
	if id == 0 || ctx.ShouldBindJSON(&req) != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}
	if err := c.svc.Disable(ctx.Request.Context(), id, req.Status); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}
```

`modules/member/module.go`：

```go
package member

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"alexGo-cloud/alexgo-server/server"
	"alexGo-cloud/modules/member/controller/admin"
	"alexGo-cloud/modules/member/controller/app"
	"alexGo-cloud/modules/member/repository"
	"alexGo-cloud/modules/member/service"
	"alexGo-cloud/pkg/migrate"
)

var FxModule = fx.Module("member",
	fx.Provide(
		repository.NewMemberRepository,
		service.NewMemberService,
		app.NewAuthController,
		admin.NewUserController,
	),
	fx.Provide(
		fx.Annotate(
			NewModule,
			fx.As(new(server.Module)),
			fx.ResultTags(`group:"modules"`),
		),
	),
	fx.Provide(
		fx.Annotate(
			NewMigrationSource,
			fx.As(new(migrate.Source)),
			fx.ResultTags(`group:"migration_sources"`),
		),
	),
)

type memberModule struct {
	authCtrl *app.AuthController
	userCtrl *admin UserController
}

func NewModule(authCtrl *app.AuthController, userCtrl *admin.UserController) server.Module {
	return &memberModule{authCtrl: authCtrl, userCtrl: userCtrl}
}

func (m *memberModule) RegisterRoutes(r *gin.RouterGroup) {
	appGroup := r.Group("/api/app/member")
	appGroup.POST("/auth/register", m.authCtrl.Register)
	appGroup.POST("/auth/login", m.authCtrl.Login)
	appGroup.POST("/auth/refresh", m.authCtrl.Refresh)
	appGroup.POST("/auth/logout", m.authCtrl.Logout)

	adminGroup := r.Group("/api/admin/member")
	adminGroup.GET("/users", m.userCtrl.List)
	adminGroup.PUT("/users/:id/status", m.userCtrl.UpdateStatus)
}
```

（注意 `admin UserController` 打错为 `admin UserController` → 正确为 `admin *admin.UserController`；以编译为准。）

**重要**：`RegisterRoutes` 的入参——现有 `server.Module` 接口是 `RegisterRoutes(r *gin.RouterGroup)`，而 system 模块拿到的是 `/api` group 后自己 `r.Group("/admin/system")`。**核对** `alexgo-server/server/http.go`：`api := r.Group("/api"); m.RegisterRoutes(api)` —— system 里写的是 `r.Group("/admin/system")`（相对 /api）。**member 模块保持同构**：`r.Group("/app/member")` 与 `r.Group("/admin/member")`（去掉前缀 `/api`）。上面代码按此修正：

```go
func (m *memberModule) RegisterRoutes(r *gin.RouterGroup) {
	appGroup := r.Group("/app/member")
	...
	adminGroup := r.Group("/admin/member")
	...
}
```

- [ ] **Step 7: mono 装配 + 验证**

`alexgo-server/cmd/main.go` 的 fx 选项中（`order.FxModule` 旁）追加：

```go
		member.FxModule,
```

import `alexGo-cloud/modules/member`。

Run: `CGO_ENABLED=0 go build ./... && go vet ./... && CGO_ENABLED=0 go test ./modules/member/... -v -count=1 && CGO_ENABLED=0 go test ./... -count=1`
Expected: member 4 测试 PASS，全量绿；`grep member migrations` 确认 `migration_sources` group 已聚合（`pkg/migrate` 收集逻辑现有）

- [ ] **Step 8: Commit**

```bash
git add modules/member/ alexgo-server/cmd/main.go
git commit -m "feat(member): member module skeleton with register/login/refresh/logout and admin list"
```


---

### Task 6: 租户解析升级 + gorm 查询隔离插件

**Files:**
- Modify: `pkg/tenant/tenant.go`（域名解析上下文）
- Modify: `pkg/middleware/tenant.go`（`NewTenantMiddleware(lookup)`）
- Create: `pkg/tenant/gormplugin/plugin.go`、`pkg/tenant/gormplugin/plugin_test.go`
- Modify: `alexgo-server/server/http.go`（装配 lookup + 注册插件）、`alexgo-server/cmd/main.go`（插件挂到 *gorm.DB）
- Create: `modules/system/controller/admin/tenant.go` 挪到 Task 7——本任务只做解析+隔离

**Interfaces:**
- Consumes: `tenants` 表（Task 1）、`database.NewDB` 的 `*gorm.DB`
- Produces:

```go
// pkg/tenant
type DomainLookup func(ctx context.Context, host string) (tenantID uint64, err error)

// pkg/middleware
func NewTenantMiddleware(lookup DomainLookup) gin.HandlerFunc // lookup 可为 nil（仅 header）

// pkg/tenant/gormplugin
type Options struct {
	ExemptTables []string // 如 tenants、casbin_rule
}
func Register(db *gorm.DB, opts Options) error // 挂 Create/Query/Update/Delete 回调
func IgnoreTenant(ctx context.Context) context.Context // 显式跳过注入
```

- [ ] **Step 1: 写失败测试**

创建 `pkg/tenant/gormplugin/plugin_test.go`：

```go
package gormplugin

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type row struct {
	ID       uint64 `gorm:"primaryKey"`
	Name     string
	TenantID uint64 `gorm:"column:tenant_id"`
}

func (row) TableName() string { return "rows" }

type noTenantRow struct {
	ID   uint64 `gorm:"primaryKey"`
	Name string
}

func (noTenantRow) TableName() string { return "no_tenant_rows" }

func setup(t *testing.T, opts Options) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&row{}, &noTenantRow{}); err != nil {
		t.Fatal(err)
	}
	if err := Register(db, opts); err != nil {
		t.Fatal(err)
	}
	return db
}

// INSERT 自动填 tenant_id；SELECT 只见本租户行。
func TestIsolation_CrossTenant(t *testing.T) {
	db := setup(t, Options{})

	ctx1 := WithTenantID(context.Background(), 1)
	ctx2 := WithTenantID(context.Background(), 2)

	if err := db.WithContext(ctx1).Create(&row{Name: "a"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx2).Create(&row{Name: "b"}).Error; err != nil {
		t.Fatal(err)
	}

	var got []row
	if err := db.WithContext(ctx1).Find(&got).Error; err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "a" || got[0].TenantID != 1 {
		t.Fatalf("tenant1 sees %+v, want only own row with injected tenant_id", got)
	}

	var got2 []row
	if err := db.WithContext(ctx2).Find(&got2).Error; err != nil {
		t.Fatal(err)
	}
	if len(got2) != 1 || got2[0].Name != "b" {
		t.Fatalf("tenant2 sees %+v", got2)
	}
}

// tid=0（未解析/平台）：不过滤不填充——保持历史行为。
func TestTenantZero_NoInjection(t *testing.T) {
	db := setup(t, Options{})
	if err := db.WithContext(context.Background()).Create(&row{Name: "x"}).Error; err != nil {
		t.Fatal(err)
	}
	var got []row
	if err := db.WithContext(context.Background()).Find(&got).Error; err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].TenantID != 0 {
		t.Fatalf("got %+v", got)
	}
}

// 无 TenantID 字段的模型完全不受影响。
func TestNoTenantField_Skip(t *testing.T) {
	db := setup(t, Options{})
	if err := db.WithContext(WithTenantID(context.Background(), 9)).Create(&noTenantRow{Name: "n"}).Error; err != nil {
		t.Fatal(err)
	}
	var got []noTenantRow
	if err := db.WithContext(WithTenantID(context.Background(), 9)).Find(&got).Error; err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %+v", got)
	}
}

// 白名单表跳过过滤。
func TestExemptTable(t *testing.T) {
	db := setup(t, Options{ExemptTables: []string{"rows"}})
	if err := db.WithContext(WithTenantID(context.Background(), 1)).Create(&row{Name: "e", TenantID: 77}).Error; err != nil {
		t.Fatal(err)
	}
	var got []row
	if err := db.WithContext(WithTenantID(context.Background(), 2)).Find(&got).Error; err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].TenantID != 77 {
		t.Fatalf("exempt table must not be filtered: %+v", got)
	}
}

// IgnoreTenant 显式放行（平台侧全量查询）。
func TestIgnoreTenant(t *testing.T) {
	db := setup(t, Options{})
	if err := db.WithContext(WithTenantID(context.Background(), 1)).Create(&row{Name: "a"}).Error; err != nil {
		t.Fatal(err)
	}
	var got []row
	if err := db.WithContext(WithTenantID(context.Background(), 1)).Find(&got).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(IgnoreTenant(context.Background())).Find(&got).Error; err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("IgnoreTenant sees %+v, want all", got)
	}
}
```

（`WithTenantID` 是 `pkg/tenant` 现有函数——测试里 import `"alexGo-cloud/pkg/tenant"` 并用 `tenant.WithTenantID`；为阅读连贯此处省略前缀，**实现时按实际包名补**。）

- [ ] **Step 2: 运行确认失败**

Run: `CGO_ENABLED=0 go test ./pkg/tenant/... -v -count=1`
Expected: 编译失败 `undefined: Register/IgnoreTenant`

- [ ] **Step 3: 实现插件**

创建 `pkg/tenant/gormplugin/plugin.go`：

```go
// Package gormplugin 为 GORM 挂载租户字段隔离：
// INSERT 自动填 tenant_id，SELECT/UPDATE/DELETE 自动追加 tenant_id 条件。
//
// 设计边界（为什么这样做）：
// - tid=0 不注入：平台/未解析租户保持历史行为（仓库层已有显式过滤，插件是隔离下限不是唯一手段）；
// - 无 TenantID 字段的模型跳过：存量表零影响；
// - 白名单（tenants/casbin_rule 等全局表）跳过；
// - IgnoreTenant 显式放行：平台侧全量查询的唯一通道。
package gormplugin

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ctxKey int

const (
	tenantKey ctxKey = iota
	ignoreKey
)

// WithTenantID / IgnoreTenant 的 ctx 构造（与 pkg/tenant.WithTenantID 兼容：
// 直接复用 pkg/tenant 的 key 语义——为避免双 key，本包直接引用 pkg/tenant）。
```

**（实现裁决——必须照此收敛，避免两套 ctx key）**：`pkg/tenant/gormplugin` 不自造 key，全部复用 `pkg/tenant`：

```go
package gormplugin

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"alexGo-cloud/pkg/tenant"
)

type ctxKey int

const ignoreKey ctxKey = iota

// IgnoreTenant 标记该 ctx 的后续 GORM 操作跳过租户注入（平台侧全量查询）。
func IgnoreTenant(ctx context.Context) context.Context {
	return context.WithValue(ctx, ignoreKey, true)
}

type Options struct {
	ExemptTables []string
}

func Register(db *gorm.DB, opts Options) error {
	exempt := map[string]bool{}
	for _, t := range opts.ExemptTables {
		exempt[t] = true
	}

	// SELECT/UPDATE/DELETE：追加 tenant_id 条件。
	// 回调时机选 Before("gorm:query")：此时 Statement.Schema 与 clauses 已就绪。
	addFilter := func(db *gorm.DB) {
		st := db.Statement
		if st.Schema == nil || st.Context == nil {
			return
		}
		if st.Context.Value(ignoreKey) == true {
			return
		}
		if exempt[st.Table] {
			return
		}
		if st.Schema.LookUpField("TenantID") == nil {
			return
		}
		tid := tenant.TenantIDFromContext(st.Context)
		if tid == 0 {
			return
		}
		st.AddClause(clause.Where{Exprs: []clause.Expression{
			clause.Eq{Column: clause.Column{Name: "tenant_id"}, Value: tid},
		}})
	}

	// INSERT：tenant_id 为零值时回填。
	fillInsert := func(db *gorm.DB) {
		st := db.Statement
		if st.Schema == nil || st.Context == nil || st.Context.Value(ignoreKey) == true {
			return
		}
		if exempt[st.Table] || st.Schema.LookUpField("TenantID") == nil {
			return
		}
		tid := tenant.TenantIDFromContext(st.Context)
		if tid == 0 {
			return
		}
		if f := st.Schema.LookUpField("TenantID"); f != nil {
			if v, _ := f.ValueOf(st.Context, st.Dest); v == nil || v.(uint64) == 0 {
				_ = f.Set(st.Context, st.Dest, tid)
			}
		}
	}

	if err := db.Callback().Query().Before("gorm:query").Register("tenant:filter_query", addFilter); err != nil {
		return fmt.Errorf("tenant plugin query: %w", err)
	}
	if err := db.Callback().Update().Before("gorm:update").Register("tenant:filter_update", addFilter); err != nil {
		return fmt.Errorf("tenant plugin update: %w", err)
	}
	if err := db.Callback().Delete().Before("gorm:delete").Register("tenant:filter_delete", addFilter); err != nil {
		return fmt.Errorf("tenant plugin delete: %w", err)
	}
	if err := db.Callback().Create().Before("gorm:create").Register("tenant:fill_create", fillInsert); err != nil {
		return fmt.Errorf("tenant plugin create: %w", err)
	}
	return nil
}
```

（`f.ValueOf/Set` 的签名以 gorm v1.31 实际 API 为准：`f.ValueOf(ctx, dest) (value interface{}, found bool)`、`f.Set(ctx, dest, value) error`——实现时 `go doc gorm.io/gorm/schema.Field` 核对，行为不变：仅当目标字段为 0 时回填。`st.Table` 在回调期可能为空——改为 `st.Schema.Table`（schema 一定有）：过滤函数里统一用 `st.Schema.Table`。）

- [ ] **Step 4: 域名解析 + 中间件**

`pkg/tenant/tenant.go` 追加（现有 `WithTenantID`/`TenantIDFromContext` 不动）：

```go
// DomainLookup 按请求域名解析租户编号（nil/未命中返回 0）。
type DomainLookup func(ctx context.Context, host string) (uint64, error)
```

`pkg/middleware/tenant.go` 整文件替换：

```go
package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/pkg/tenant"
)

// NewTenantMiddleware 解析租户并注入 context，优先级：
// 1) 显式 X-Tenant-ID 头（App/开发直连）；
// 2) Host 域名匹配 tenants.domain（SaaS Web；lookup 为 nil 时跳过）；
// 3) 都没有 → 0（平台租户，历史行为）。
func NewTenantMiddleware(lookup tenant.DomainLookup) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := tenant.ParseTenantID(c.GetHeader("X-Tenant-ID"))
		if id == 0 && lookup != nil {
			host := c.Request.Host
			if h, _, ok := strings.Cut(host, ":"); ok {
				host = h // 去端口再匹配
			}
			if resolved, err := lookup(c.Request.Context(), host); err == nil {
				id = resolved
			}
		}
		c.Request = c.Request.WithContext(tenant.WithTenantID(c.Request.Context(), id))
		c.Next()
	}
}
```

`alexgo-server/server/http.go`：中间件链中 `middleware.TenantMiddleware(),` 改为 `middleware.NewTenantMiddleware(p.TenantDomainLookup),`，`HTTPServerParams` 增加：

```go
	// TenantDomainLookup：域名→租户解析（nil 则只认 X-Tenant-ID 头）。
	TenantDomainLookup tenant.DomainLookup `optional:"true"`
```

（import `alexGo-cloud/pkg/tenant`。）

**域名解析实现**（system 模块提供，两服务共用逻辑）：创建 `pkg/tenant/domain.go`：

```go
package tenant

import (
	"context"

	"gorm.io/gorm"
)

// NewDomainLookup 基于 tenants 表实现域名解析（表全局，故直接查库，
// 不依赖 modules 层——system 与 member 两服务都能注入同一实现）。
func NewDomainLookup(db *gorm.DB) DomainLookup {
	return func(ctx context.Context, host string) (uint64, error) {
		if db == nil || host == "" {
			return 0, nil
		}
		var id uint64
		err := db.WithContext(ctx).
			Raw("SELECT id FROM tenants WHERE domain = ? AND deleted = 0 AND status = 1", host).
			Scan(&id).Error
		if err != nil {
			return 0, err
		}
		return id, nil
	}
}
```

**注意**：`tenants` 查询**不得**被插件过滤（它是全局表）——插件白名单在 `cmd/main.go` 注册时包含 `"tenants"`（见 Step 5）。且该 raw 查询无模型，插件不参与（Raw 回调仍走 Query callback？——GORM Raw 也会触发 query callbacks，但 `st.Schema == nil` → 早退，安全）。

- [ ] **Step 5: 装配插件与 lookup**

`alexgo-server/cmd/main.go`：
1. `fx.Provide` 中 `database.NewDB` 之后追加：

```go
			// 租户域名解析（Host → tenant_id），供 TenantMiddleware 注入。
			tenant.NewDomainLookup,
			// 租户字段隔离插件：对 DB 挂 INSERT 填充/查询过滤回调。
			func(db *gorm.DB) (*gorm.DB, error) {
				if err := gormplugin.Register(db, gormplugin.Options{
					ExemptTables: []string{"tenants", "casbin_rule"},
				}); err != nil {
					return nil, err
				}
				return db, nil
			},
```

（import `gorm.io/gorm`、`alexGo-cloud/pkg/tenant`、`alexGo-cloud/pkg/tenant/gormplugin`。**注意**：`database.NewDB` 返回 `(*gorm.DB, error)`——再包一层同签名 provider 会**重复注册同一类型**冲突。FX 中同一类型两个 provider 报错。正确做法：插件注册放进 `database.NewDB` 内部不合适（pkg/database 不该依赖 tenant）。**改用 fx.Invoke**：

```go
			fx.Invoke(func(db *gorm.DB) error {
				return gormplugin.Register(db, gormplugin.Options{
					ExemptTables: []string{"tenants", "casbin_rule"},
				})
			}),
```

`fx.Invoke` 在依赖图构建后执行、早于路由启动（Invoke 顺序在 options 列表中位于 StartHTTPServer 之前即可——把该 Invoke 放在 Provide 块之后、`server.StartHTTPServer` 的 Invoke 之前）。）

2. `HTTPServerParams` 装配处（`StartHTTPServer` 调用由 fx 自动注入）无需改——`TenantDomainLookup` optional 由 `tenant.NewDomainLookup` provider 满足。

`modules/system/controller/admin/tenant.go`——本任务不做（Task 7）。

- [ ] **Step 6: 验证**

Run: `CGO_ENABLED=0 go test ./pkg/tenant/... ./pkg/middleware/ -v -count=1`
Expected: 插件 5 个测试 PASS；middleware 既有测试 PASS
Run: `CGO_ENABLED=0 go build ./... && go vet ./... && CGO_ENABLED=0 go test ./... -count=1`
Expected: 全绿（重点观察既有仓储测试/种子是否受插件影响——`casbin_rule`/`tenants` 已豁免，其余带 tenant_id 表在 tid=0 时不注入，行为不变）

- [ ] **Step 7: Commit**

```bash
git add pkg/tenant/ pkg/middleware/tenant.go alexgo-server/
git commit -m "feat(tenant): domain-based tenant resolution and gorm isolation plugin"
```

---

### Task 7: 租户管理 CRUD + 停用拒绝 + account_limit

**Files:**
- Create: `modules/system/repository/tenant.go`
- Create: `modules/system/service/tenant.go`（+ 测试 `tenant_test.go`）
- Create: `modules/system/controller/admin/tenant.go`
- Modify: `modules/system/module.go`（Provide + 路由）
- Modify: `modules/system/service/user.go`（创建用户时 account_limit 校验——**先读现有 CreateUser 实现再改**）
- Modify: `modules/member/service/member.go`（Register 时 account_limit 校验）

**Interfaces:**
- Consumes: `tenants` 表（Task 1）、`pkg/token.Issue` 的租户状态检查（Task 2 已挡签发）
- Produces: 路由
  - `GET /api/admin/system/tenants`、`POST /api/admin/system/tenants`、`PUT /api/admin/system/tenants/:id`、`DELETE /api/admin/system/tenants/:id`
  - `service.TenantService`：`List/Create/Update/Delete`

- [ ] **Step 1: 写失败测试**

创建 `modules/system/service/tenant_test.go`：

```go
package service

import (
	"context"
	"errors"
	"testing"

	"alexGo-cloud/modules/system/model"
)

type memTenantRepo struct {
	byID   map[uint64]*model.Tenant
	nextID uint64
	limits map[uint64]int // tenantID -> accountLimit（供额度测试）
	counts map[uint64]int64
}

func newMemTenantRepo() *memTenantRepo {
	return &memTenantRepo{
		byID: map[uint64]*model.Tenant{}, limits: map[uint64]int{},
		counts: map[uint64]int64{},
	}
}

func (m *memTenantRepo) List(context.Context) ([]*model.Tenant, error) {
	var out []*model.Tenant
	for _, v := range m.byID {
		out = append(out, v)
	}
	return out, nil
}
func (m *memTenantRepo) GetByID(_ context.Context, id uint64) (*model.Tenant, error) {
	v, ok := m.byID[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return v, nil
}
func (m *memTenantRepo) Create(_ context.Context, t *model.Tenant) error {
	m.nextID++
	t.ID = m.nextID
	m.byID[t.ID] = t
	return nil
}
func (m *memTenantRepo) Update(_ context.Context, t *model.Tenant) error {
	m.byID[t.ID] = t
	return nil
}
func (m *memTenantRepo) Delete(_ context.Context, id uint64) error {
	delete(m.byID, id)
	return nil
}
func (m *memTenantRepo) CountAccounts(_ context.Context, tenantID uint64) (int64, error) {
	return m.counts[tenantID], nil
}

func TestTenantService_CreateList(t *testing.T) {
	repo := newMemTenantRepo()
	svc := NewTenantService(repo)
	created, err := svc.Create(context.Background(), "Acme", "acme.example.com", -1)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == 0 || created.Status != 1 {
		t.Errorf("created = %+v（status 必须 1=启用）", created)
	}
	list, err := svc.List(context.Background())
	if err != nil || len(list) != 1 {
		t.Errorf("list = %v, err = %v", list, err)
	}
}

// 超额必须拒绝——服务层统一拦（system 建用户 + member 注册两处调用）。
func TestTenantService_CheckAccountLimit(t *testing.T) {
	repo := newMemTenantRepo()
	svc := NewTenantService(repo)
	tn, _ := svc.Create(context.Background(), "Acme", "", 2) // 额度 2
	repo.counts[tn.ID] = 2
	if err := svc.CheckAccountLimit(context.Background(), tn.ID); err == nil {
		t.Error("over limit must fail")
	}
	repo.counts[tn.ID] = 1
	if err := svc.CheckAccountLimit(context.Background(), tn.ID); err != nil {
		t.Errorf("under limit must pass: %v", err)
	}
	repo.counts[tn.ID] = 2
	tn.AccountLimit = -1 // 不限
	repo.byID[tn.ID] = tn
	if err := svc.CheckAccountLimit(context.Background(), tn.ID); err != nil {
		t.Errorf("limit=-1 must pass: %v", err)
	}
}
```

（`model.Tenant` 见 Step 3；`NewTenantService(repo)` 单参或带 dep 按实现。）

- [ ] **Step 2: 运行确认失败**

Run: `CGO_ENABLED=0 go test ./modules/system/service/ -run TestTenant -v`
Expected: 编译失败

- [ ] **Step 3: 模型/仓储/服务/控制器**

`modules/system/model/tenant.go`：

```go
package model

import "time"

// Tenant 租户（全局表，不带 tenant_id）。
type Tenant struct {
	ID           uint64     `gorm:"primaryKey" json:"id"`
	Name         string     `json:"name"`
	PackageID    uint64     `gorm:"column:package_id" json:"package_id"`
	Status       int        `json:"status"` // 1启用 0停用
	ExpireTime   *time.Time `gorm:"column:expire_time" json:"expire_time"`
	AccountLimit int        `gorm:"column:account_limit" json:"account_limit"`
	Domain       string     `json:"domain"`
	Deleted      bool       `gorm:"column:deleted" json:"-"`
	Creator      string     `json:"creator,omitempty"`
	Updater      string     `json:"updater,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (Tenant) TableName() string { return "tenants" }
```

`modules/system/repository/tenant.go`：

```go
package repository

import (
	"context"

	"gorm.io/gorm"

	"alexGo-cloud/modules/system/model"
)

type TenantRepository interface {
	List(ctx context.Context) ([]*model.Tenant, error)
	GetByID(ctx context.Context, id uint64) (*model.Tenant, error)
	Create(ctx context.Context, t *model.Tenant) error
	Update(ctx context.Context, t *model.Tenant) error
	Delete(ctx context.Context, id uint64) error
	CountAccounts(ctx context.Context, tenantID uint64) (int64, error)
}

type tenantRepo struct{ db *gorm.DB }

func NewTenantRepository(db *gorm.DB) TenantRepository { return &tenantRepo{db: db} }

func (r *tenantRepo) List(ctx context.Context) ([]*model.Tenant, error) {
	var items []*model.Tenant
	err := r.db.WithContext(ctx).Where("deleted = 0").Order("id desc").Find(&items).Error
	return items, err
}

func (r *tenantRepo) GetByID(ctx context.Context, id uint64) (*model.Tenant, error) {
	var item model.Tenant
	err := r.db.WithContext(ctx).Where("id = ? AND deleted = 0", id).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *tenantRepo) Create(ctx context.Context, t *model.Tenant) error {
	return r.db.WithContext(ctx).Create(t).Error
}

func (r *tenantRepo) Update(ctx context.Context, t *model.Tenant) error {
	return r.db.WithContext(ctx).Save(t).Error
}

func (r *tenantRepo) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Model(&model.Tenant{}).
		Where("id = ?", id).Update("deleted", 1).Error
}

// CountAccounts 统计租户下 system_users + member_user 账号数（raw SQL：
// member 表可能不存在于纯 system 场景——用存在性容错，缺表按 0 计）。
func (r *tenantRepo) CountAccounts(ctx context.Context, tenantID uint64) (int64, error) {
	var n int64
	if err := r.db.WithContext(ctx).Table("system_users").
		Where("tenant_id = ? AND deleted = 0", tenantID).Count(&n).Error; err != nil {
		return 0, err
	}
	var m int64
	err := r.db.WithContext(ctx).Table("member_user").
		Where("tenant_id = ? AND deleted = 0", tenantID).Count(&m).Error
	if err == nil {
		n += m
	}
	return n, nil
}
```

`modules/system/service/tenant.go`：

```go
package service

import (
	"context"
	"fmt"
	"strings"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/repository"
)

type TenantService interface {
	List(ctx context.Context) ([]*model.Tenant, error)
	Create(ctx context.Context, name, domain string, accountLimit int) (*model.Tenant, error)
	Update(ctx context.Context, id uint64, name, domain string, status, accountLimit int) error
	Delete(ctx context.Context, id uint64) error
	// CheckAccountLimit：account_limit>0 时校验租户账号数未超限（system 建用户与 member 注册共用）。
	CheckAccountLimit(ctx context.Context, tenantID uint64) error
}

type tenantService struct {
	repo repository.TenantRepository
}

func NewTenantService(repo repository.TenantRepository) TenantService {
	return &tenantService{repo: repo}
}

func (s *tenantService) List(ctx context.Context) ([]*model.Tenant, error) {
	return s.repo.List(ctx)
}

func (s *tenantService) Create(ctx context.Context, name, domain string, accountLimit int) (*model.Tenant, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("tenant name required")
	}
	t := &model.Tenant{
		Name: name, Domain: domain, Status: 1,
		AccountLimit: accountLimit, // 默认 -1 不限时由调用方传
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *tenantService) Update(ctx context.Context, id uint64, name, domain string, status, accountLimit int) error {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	t.Name, t.Domain, t.Status, t.AccountLimit = name, domain, status, accountLimit
	return s.repo.Update(ctx, t)
}

func (s *tenantService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, id)
}

func (s *tenantService) CheckAccountLimit(ctx context.Context, tenantID uint64) error {
	if tenantID == 0 {
		return nil // 平台租户不限
	}
	t, err := s.repo.GetByID(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("tenant not found: %w", err)
	}
	if t.AccountLimit <= 0 {
		return nil
	}
	n, err := s.repo.CountAccounts(ctx, tenantID)
	if err != nil {
		return err
	}
	if n >= int64(t.AccountLimit) {
		return fmt.Errorf("account limit reached (%d)", t.AccountLimit)
	}
	return nil
}
```

`modules/system/controller/admin/tenant.go`：

```go
package admin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/service"
)

type TenantController struct {
	svc service.TenantService
}

func NewTenantController(svc service.TenantService) *TenantController {
	return &TenantController{svc: svc}
}

func (c *TenantController) List(ctx *gin.Context) {
	items, err := c.svc.List(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": items})
}

type tenantRequest struct {
	Name         string `json:"name"`
	Domain       string `json:"domain"`
	Status       int    `json:"status"`
	AccountLimit int    `json:"account_limit"`
}

func (c *TenantController) Create(ctx *gin.Context) {
	var req tenantRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.AccountLimit == 0 {
		req.AccountLimit = -1
	}
	item, err := c.svc.Create(ctx.Request.Context(), req.Name, req.Domain, req.AccountLimit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": item})
}

func (c *TenantController) Update(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	var req tenantRequest
	if id == 0 || ctx.ShouldBindJSON(&req) != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}
	if err := c.svc.Update(ctx.Request.Context(), id, req.Name, req.Domain, req.Status, req.AccountLimit); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (c *TenantController) Delete(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := c.svc.Delete(ctx.Request.Context(), id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}
```

- [ ] **Step 4: 装配与路由**

`modules/system/module.go`：
1. `FxModule` 的 `fx.Provide` 追加：`repository.NewTenantRepository, service.NewTenantService, admin.NewTenantController`
2. `systemModule` 结构体加 `tenantCtrl *admin.TenantController`，`NewSystemModule` 参数同步加
3. `RegisterRoutes` 的 adminGroup 追加：

```go
		adminGroup.GET("/tenants", m.tenantCtrl.List)
		adminGroup.POST("/tenants", m.tenantCtrl.Create)
		adminGroup.PUT("/tenants/:id", m.tenantCtrl.Update)
		adminGroup.DELETE("/tenants/:id", m.tenantCtrl.Delete)
```

**额度接入**：
1. `modules/system/service/user.go`：先 `Read` 现有 `CreateUser` 实现，在写库前插入：

```go
	if err := s.tenantSvc.CheckAccountLimit(ctx, tenant.TenantIDFromContext(ctx)); err != nil {
		return nil, err
	}
```

`userService` 增加字段 `tenantSvc TenantService`，`NewUserService` 增参（**读取现有构造签名后最小化改动**；FX 自动注入）。

2. `modules/member/service/member.go` 的 `Register` 在 `GetByMobile` 查重后插入同样调用——`memberService` 增加 `tenantSvc` 依赖。**member 模块依赖 system 的 TenantService**：跨模块直接 import system service 会破坏模块独立性。**收敛做法**：定义窄接口于 member 侧：

```go
// modules/member/service/account_limit.go
type AccountLimitChecker interface {
	CheckAccountLimit(ctx context.Context, tenantID uint64) error
}
```

`NewMemberService(repo, issuer, limit AccountLimitChecker)`——mono/micro 下 system 模块都提供同一实现，FX 按接口注入（`fx.As`）：system FxModule 追加

```go
		func(s service.TenantService) AccountLimitChecker { return s }, // package member/service 引用？
```

——**不行**（循环方向：system 引用 member 类型）。**最终收敛**：把窄接口放共享层 `pkg/tenant`：

```go
// pkg/tenant/account.go
// AccountLimitChecker 租户账号额度检查（实现方为 system 模块，member 经 FX 注入）。
type AccountLimitChecker interface {
	CheckAccountLimit(ctx context.Context, tenantID uint64) error
}
```

system FxModule Provide：`func(svc service.TenantService) tenant.AccountLimitChecker { return svc }`
member `NewMemberService(repo, issuer, limit tenant.AccountLimitChecker)`（`limit` 可 nil → 跳过检查）。

（测试中直接传 fake。）

- [ ] **Step 5: 验证**

Run: `CGO_ENABLED=0 go test ./modules/system/service/ ./modules/member/... -v -count=1`
Expected: Tenant 2 测试 + member 既有测试 PASS
Run: `CGO_ENABLED=0 go build ./... && go vet ./... && CGO_ENABLED=0 go test ./... -count=1`
Expected: 全绿（UserService 构造变化若编译报错，同步 module.go 提供处）

- [ ] **Step 6: Commit**

```bash
git add modules/system/ modules/member/ pkg/tenant/
git commit -m "feat(tenant): tenant management CRUD, account limit enforcement"
```


---

### Task 8: 角色字段增强（data_scope/type/sort）+ 系统角色删除守卫

**Files:**
- Modify: `modules/system/service/rbac.go`（RoleService.Create/Delete 签名与守卫）
- Modify: `modules/system/controller/admin/role.go`（请求结构扩展）
- Create: `modules/system/service/role_scope_test.go`

**Interfaces:**
- Consumes: `roles` 补列（Task 1）
- Produces:

```go
type RoleService interface {
	List(ctx context.Context) ([]*model.Role, error)
	Create(ctx context.Context, p RoleCreateParams) (*model.Role, error)
	Delete(ctx context.Context, id uint64) error
	AssignMenus(ctx context.Context, roleID uint64, menuIDs []uint64) error
}

type RoleCreateParams struct {
	Code, Name, Remark  string
	Sort                int
	DataScope           int    // 1全部 2自定义 3本部门 4本部门及以下 5仅本人；0 → 默认 1
	DataScopeDeptIDs    string
}
```

（`type` 一律 2=自定义——一期不开放系统内置角色的创建；`Status` 默认 1。）

- [ ] **Step 1: 写失败测试**

创建 `modules/system/service/role_scope_test.go`：

```go
package service

import (
	"context"
	"errors"
	"testing"

	"alexGo-cloud/modules/system/model"
)

type memRoleRepo struct {
	byID   map[uint64]*model.Role
	nextID uint64
}

func newMemRoleRepo() *memRoleRepo { return &memRoleRepo{byID: map[uint64]*model.Role{}} }

func (m *memRoleRepo) List(_ context.Context, tid uint64) ([]*model.Role, error) {
	var out []*model.Role
	for _, r := range m.byID {
		if r.TenantID == tid {
			out = append(out, r)
		}
	}
	return out, nil
}
func (m *memRoleRepo) GetByID(_ context.Context, tid, id uint64) (*model.Role, error) {
	r, ok := m.byID[id]
	if !ok || r.TenantID != tid {
		return nil, errors.New("not found")
	}
	return r, nil
}
func (m *memRoleRepo) GetByCode(_ context.Context, tid uint64, code string) (*model.Role, error) {
	for _, r := range m.byID {
		if r.TenantID == tid && r.Code == code {
			return r, nil
		}
	}
	return nil, errors.New("not found")
}
func (m *memRoleRepo) Create(_ context.Context, r *model.Role) error {
	m.nextID++
	r.ID = m.nextID
	m.byID[r.ID] = r
	return nil
}
func (m *memRoleRepo) Update(_ context.Context, r *model.Role) error { m.byID[r.ID] = r; return nil }
func (m *memRoleRepo) Delete(_ context.Context, _ uint64, id uint64) error {
	delete(m.byID, id)
	return nil
}

type memRoleMenuRepo struct{}

func (memRoleMenuRepo) SetRoleMenus(context.Context, uint64, uint64, []uint64) error { return nil }
func (memRoleMenuRepo) ListMenuIDsByRoleIDs(context.Context, uint64, []uint64) ([]uint64, error) {
	return nil, nil
}

func TestRoleCreate_DefaultsAndParams(t *testing.T) {
	repo := newMemRoleRepo()
	svc := NewRoleService(repo, memRoleMenuRepo{})

	r, err := svc.Create(context.Background(), RoleCreateParams{
		Code: "sales", Name: "销售", DataScope: 3, Sort: 5, Remark: "r",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if r.DataScope != 3 || r.Sort != 5 || r.Type != 2 || r.Status != 1 || r.Remark != "r" {
		t.Errorf("role = %+v", r)
	}

	// DataScope=0 → 默认 1（全部）
	r2, err := svc.Create(context.Background(), RoleCreateParams{Code: "ops", Name: "运维"})
	if err != nil {
		t.Fatal(err)
	}
	if r2.DataScope != 1 || r2.Type != 2 {
		t.Errorf("defaults = %+v", r2)
	}

	// 非法 data_scope 拒绝
	if _, err := svc.Create(context.Background(), RoleCreateParams{
		Code: "x", Name: "x", DataScope: 9,
	}); err == nil {
		t.Error("invalid data_scope must fail")
	}
}

// 系统内置角色（type=1）禁止删除。
func TestRoleDelete_SystemRoleGuard(t *testing.T) {
	repo := newMemRoleRepo()
	svc := NewRoleService(repo, memRoleMenuRepo{})
	r, _ := svc.Create(context.Background(), RoleCreateParams{Code: "sys", Name: "系统"})
	r.Type = 1
	repo.byID[r.ID] = r

	if err := svc.Delete(context.Background(), r.ID); err == nil {
		t.Error("system role (type=1) must not be deletable")
	}
	// 自定义角色可删
	r2, _ := svc.Create(context.Background(), RoleCreateParams{Code: "custom", Name: "自定义"})
	if err := svc.Delete(context.Background(), r2.ID); err != nil {
		t.Errorf("custom role delete: %v", err)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `CGO_ENABLED=0 go test ./modules/system/service/ -run TestRole -v`
Expected: 编译失败（Create 签名未变）

- [ ] **Step 3: 实现**

`modules/system/service/rbac.go` 修改：

1. `RoleService` 接口按 Produces 替换（加 `RoleCreateParams`）。
2. `roleService.Create` 替换：

```go
type RoleCreateParams struct {
	Code           string
	Name           string
	Remark         string
	Sort           int
	DataScope      int
	DataScopeDeptIDs string
}

func (s *roleService) Create(ctx context.Context, p RoleCreateParams) (*model.Role, error) {
	if p.Code == "" || p.Name == "" {
		return nil, fmt.Errorf("code/name required")
	}
	if p.DataScope == 0 {
		p.DataScope = 1
	}
	if p.DataScope < 1 || p.DataScope > 5 {
		return nil, fmt.Errorf("invalid data_scope: %d", p.DataScope)
	}
	tid := tenant.TenantIDFromContext(ctx)
	if _, err := s.roleRepo.GetByCode(ctx, tid, p.Code); err == nil {
		return nil, fmt.Errorf("role code already exists")
	}
	r := &model.Role{
		Code: p.Code, Name: p.Name, Remark: p.Remark, Sort: p.Sort,
		DataScope: p.DataScope, DataScopeDeptIDs: p.DataScopeDeptIDs,
		Type: 2, Status: 1, TenantID: tid,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := s.roleRepo.Create(ctx, r); err != nil {
		return nil, err
	}
	return r, nil
}
```

（`time` 已 import；检查 `roleRepo.Create` 是否会自动填时间——以现有实现为准，重复赋值无害。）

3. `roleService.Delete` 替换（加守卫）：

```go
func (s *roleService) Delete(ctx context.Context, id uint64) error {
	tid := tenant.TenantIDFromContext(ctx)
	r, err := s.roleRepo.GetByID(ctx, tid, id)
	if err != nil {
		return err
	}
	if r.Type == 1 {
		return fmt.Errorf("system role (type=1) cannot be deleted")
	}
	return s.roleRepo.Delete(ctx, tid, id)
}
```

4. `modules/system/controller/admin/role.go` 的 `createRoleRequest` 替换：

```go
type createRoleRequest struct {
	Code           string `json:"code"`
	Name           string `json:"name"`
	Remark         string `json:"remark"`
	Sort           int    `json:"sort"`
	DataScope      int    `json:"data_scope"`
	DataScopeDeptIDs string `json:"data_scope_dept_ids"`
}
```

`Create` handler 改为：

```go
	item, err := c.roleSvc.Create(ctx.Request.Context(), service.RoleCreateParams{
		Code: req.Code, Name: req.Name, Remark: req.Remark, Sort: req.Sort,
		DataScope: req.DataScope, DataScopeDeptIDs: req.DataScopeDeptIDs,
	})
```

5. **既有调用点排查**：`grep -rn "\.Create(ctx" modules/system --include="*.go" | grep -i role`、seed 是否调用 `roleSvc.Create`——若有，改为 `RoleCreateParams{...}` 形式。

- [ ] **Step 4: 验证**

Run: `CGO_ENABLED=0 go build ./... && go vet ./... && CGO_ENABLED=0 go test ./modules/system/... -v -count=1 && CGO_ENABLED=0 go test ./... -count=1`
Expected: 3 个新测试 PASS，全量绿

- [ ] **Step 5: Commit**

```bash
git add modules/system/
git commit -m "feat(system): role data_scope/type/sort fields with system-role delete guard"
```

---

### Task 9: Casbin 策略同步 + 租户 sub 前缀 + 启动重灌

**Files:**
- Create: `modules/system/service/permission_routes.go`（permission→路由映射 + rebuild）
- Modify: `modules/system/service/rbac.go`（AssignMenus/Delete 触发 rebuild；EnsureUserRolePolicy 前缀化）
- Modify: `modules/system/service/seed.go`（种子策略前缀化 + 末尾全量重灌）
- Modify: `pkg/middleware/auth.go`（sub 前缀——**Task 3 已写入**，本任务复核）
- Modify: `pkg/middleware/auth_test.go`（casbin 用例改前缀 sub）
- Create: `modules/system/service/policy_sync_test.go`

**Interfaces:**
- Consumes: `role_menus`/`roles`/`menus`、Casbin enforcer（gorm-adapter）、Task 3 的 `{tid}:{username}` sub
- Produces:

```go
// permission_routes.go
func permPrefix(permission string) string          // "system:user:list" → "system:user"
var permissionRoutes = map[string][]string{...}    // 见 Step 3
func rebuildRolePolicies(ctx context.Context, e *casbin.Enforcer, r *model.Role, menus []*model.Menu) error
func RebuildAllPolicies(ctx context.Context, e *casbin.Enforcer, roles []*model.Role, menus []*model.Menu) error
func roleSub(tenantID uint64, roleCode string) string // "1:admin"
```

- [ ] **Step 1: 写失败测试**

创建 `modules/system/service/policy_sync_test.go`：

```go
package service

import (
	"context"
	"testing"

	"github.com/casbin/casbin/v2"
	casbinmodel "github.com/casbin/casbin/v2/model"

	"alexGo-cloud/modules/system/model"
)

// 内存 Casbin（与 pkg/auth/casbin.go 同款 matcher），无 DB adapter。
func newMemEnforcer(t *testing.T) *casbin.Enforcer {
	t.Helper()
	m, err := casbinmodel.NewModelFromString(`
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && keyMatch2(r.obj, p.obj) && regexMatch(r.act, p.act)
`)
	if err != nil {
		t.Fatal(err)
	}
	e, err := casbin.NewEnforcer(m)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func menusWithPerms() []*model.Menu {
	return []*model.Menu{
		{ID: 1, Permission: "system:user:list", Type: "button"},
		{ID: 2, Permission: "system:role:list", Type: "button"},
	}
}

// 分配菜单 → 角色立即获得对应路由权限；角色 sub 带租户前缀。
func TestRebuildRolePolicies(t *testing.T) {
	e := newMemEnforcer(t)
	role := &model.Role{ID: 1, Code: "admin", TenantID: 1}

	if err := rebuildRolePolicies(context.Background(), e, role, menusWithPerms()); err != nil {
		t.Fatalf("rebuild error = %v", err)
	}

	ok, err := e.Enforce("1:admin", "/api/admin/system/users", "GET")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("tenant1 admin must access system/users after assign")
	}

	// 跨租户：同 code 角色不属于租户 2 的用户。
	ok, _ = e.Enforce("2:admin", "/api/admin/system/users", "GET")
	if ok {
		t.Error("tenant2 must NOT inherit tenant1's role policy")
	}

	// 只分配了 user/role 两个权限 → menus 路由不放行。
	ok, _ = e.Enforce("1:admin", "/api/admin/system/menus", "GET")
	if ok {
		t.Error("unassigned route must be denied")
	}
}

// 移除权限（重建时菜单变少）→ 旧策略必须清掉（防残留）。
func TestRebuild_ClearsStale(t *testing.T) {
	e := newMemEnforcer(t)
	role := &model.Role{ID: 1, Code: "ops", TenantID: 1}
	_ = rebuildRolePolicies(context.Background(), e, role, menusWithPerms())

	// 只剩 user 权限
	_ = rebuildRolePolicies(context.Background(), e, role, menusWithPerms()[:1])

	ok, _ := e.Enforce("1:ops", "/api/admin/system/roles", "GET")
	if ok {
		t.Error("stale policy must be removed on rebuild")
	}
	ok, _ = e.Enforce("1:ops", "/api/admin/system/users", "GET")
	if !ok {
		t.Error("current policy must remain")
	}
}

func TestPermPrefix(t *testing.T) {
	cases := map[string]string{
		"system:user:list":   "system:user",
		"system:log:operate": "system:log",
		"member:user:list":   "member:user",
		"solo":               "solo", // 段数不足原样返回
	}
	for in, want := range cases {
		if got := permPrefix(in); got != want {
			t.Errorf("permPrefix(%q) = %q, want %q", in, got, want)
		}
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `CGO_ENABLED=0 go test ./modules/system/service/ -run "TestRebuild|TestPermPrefix" -v`
Expected: 编译失败 `undefined: rebuildRolePolicies/permPrefix`

- [ ] **Step 3: 实现映射与重建**

创建 `modules/system/service/permission_routes.go`：

```go
package service

import (
	"context"
	"strings"

	"github.com/casbin/casbin/v2"

	"alexGo-cloud/modules/system/model"
)

// permissionRoutes：permission 前缀 → 后端路由模板（keyMatch2）。
// 一期采用前缀粗粒度（spec §9 决策 8：二期按路由注册表精确到 method）。
// 键必须与菜单 permission 字段的前两段一致；新增 admin 路由时同步维护本表。
var permissionRoutes = map[string][]string{
	"system:user":   {"/api/admin/system/users"},
	"system:role":   {"/api/admin/system/roles"},
	"system:menu":   {"/api/admin/system/menus"},
	"system:dept":   {"/api/admin/system/depts"},
	"system:post":   {"/api/admin/system/posts"},
	"system:dict":   {"/api/admin/system/dict/types", "/api/admin/system/dict/datas"},
	"system:config": {"/api/admin/system/configs"},
	"system:notice": {"/api/admin/system/notices"},
	"system:log":    {"/api/admin/system/logs/login", "/api/admin/system/logs/operate"},
	"system:auth":   {"/api/admin/system/auth/profile", "/api/admin/system/auth/refresh", "/api/admin/system/auth/logout"},
	"system:tenant": {"/api/admin/system/tenants"},
	"member:user":   {"/api/admin/member/users"},
}

func permPrefix(permission string) string {
	parts := strings.Split(permission, ":")
	if len(parts) >= 2 {
		return parts[0] + ":" + parts[1]
	}
	return permission
}

// roleSub：Casbin 主体一律带租户前缀，杜绝跨租户同 code 串策略。
// import 补 "fmt"。
func roleSub(tenantID uint64, roleCode string) string {
	return fmt.Sprintf("%d:%s", tenantID, roleCode)
}
```

继续同文件——rebuild 函数族**最终形态**（`rebuildRolePolicies` 与测试签名一致；全量重建携带 role→menus 映射）：

```go
// RebuildAllPolicies 全量重建：roleMenus 为 roleID → 该角色的菜单集合。
// 启动重灌与菜单变更调用；调用方负责从 role_menus 表装配 roleMenus。
func RebuildAllPolicies(ctx context.Context, e *casbin.Enforcer,
	roles []*model.Role, roleMenus map[uint64][]*model.Menu) error {
	if e == nil {
		return nil
	}
	if _, err := e.RemoveFilteredPolicy(0); err != nil {
		return err
	}
	for _, r := range roles {
		if err := rebuildRolePoliciesWithoutClear(ctx, e, r, roleMenus[r.ID]); err != nil {
			return err
		}
	}
	return e.SavePolicy()
}
```

配套把 `rebuildRolePolicies` 拆成两层（**测试直接测 `rebuildRolePolicies`，签名保持 Step 1 不变**）：

```go
func rebuildRolePolicies(ctx context.Context, e *casbin.Enforcer, r *model.Role, menus []*model.Menu) error {
	if e == nil || r == nil {
		return nil
	}
	if _, err := e.RemoveFilteredPolicy(0, roleSub(r.TenantID, r.Code)); err != nil {
		return err
	}
	if err := rebuildRolePoliciesWithoutClear(ctx, e, r, menus); err != nil {
		return err
	}
	return e.SavePolicy()
}

// rebuildRolePoliciesWithoutClear 只增不删（供全量重建复用，避免逐角色 SavePolicy）。
func rebuildRolePoliciesWithoutClear(ctx context.Context, e *casbin.Enforcer, r *model.Role, menus []*model.Menu) error {
	if e == nil || r == nil {
		return nil
	}
	sub := roleSub(r.TenantID, r.Code)
	seen := map[string]bool{}
	for _, m := range menus {
		if m.Permission == "" {
			continue
		}
		routes, ok := permissionRoutes[permPrefix(m.Permission)]
		if !ok {
			continue
		}
		for _, route := range routes {
			key := sub + "|" + route
			if seen[key] {
				continue
			}
			seen[key] = true
			if _, err := e.AddPolicy(sub, route, ".*"); err != nil {
				return err
			}
		}
	}
	return nil
}
```

（`ctx` 暂未使用——保留参数以便后续 adapter 操作；若 errcheck/unused 报警，用 `_ = ctx` 不行——改为函数内 `if ctx == nil { ctx = context.Background() }` 消费掉。）

- [ ] **Step 4: 触发点接线**

`modules/system/service/rbac.go`：

1. `permissionService` 增加方法（接口 `PermissionService` 同步追加）：

```go
	// RebuildPolicies：按当前 role_menus 全量重建 Casbin p 策略（启动/菜单变更用）。
	RebuildPolicies(ctx context.Context) error
	// RebuildRolePolicies：单角色重建（AssignMenus/Delete 触发）。
	RebuildRolePolicies(ctx context.Context, roleID uint64) error
```

实现：

```go
func (s *permissionService) RebuildPolicies(ctx context.Context) error {
	if s.enforcer == nil {
		return nil
	}
	tid := tenant.TenantIDFromContext(ctx) // 0 = 全租户
	var roles []*model.Role
	var err error
	if tid == 0 {
		roles, err = s.roleRepo.ListAll(ctx) // 新增仓储方法，见下
	} else {
		roles, err = s.roleRepo.List(ctx, tid)
	}
	if err != nil {
		return err
	}
	menus, err := s.menuRepo.List(ctx, tid)
	if err != nil {
		return err
	}
	// 装配 roleID → menus
	roleIDs := make([]uint64, 0, len(roles))
	for _, r := range roles {
		roleIDs = append(roleIDs, r.ID)
	}
	menuIDsByRole, err := s.roleMenuRepo.ListMenuIDsByRoleIDs(ctx, roleIDs)
	if err != nil {
		return err
	}
	byID := map[uint64]*model.Menu{}
	for _, m := range menus {
		byID[m.ID] = m
	}
	roleMenus := map[uint64][]*model.Menu{}
	for roleID, mids := range menuIDsByRole {
		for _, mid := range mids {
			if m, ok := byID[mid]; ok {
				roleMenus[roleID] = append(roleMenus[roleID], m)
			}
		}
	}
	return RebuildAllPolicies(ctx, s.enforcer, roles, roleMenus)
}
```

**接口对齐注意**：现有 `ListMenuIDsByRoleIDs(ctx, tenantID, roleIDs)` 是**逐角色返回还是聚合**——读 `repository/rbac.go` 实际签名后适配；若返回 `map[uint64][]uint64`（roleID→menuIDs）直接用；若是聚合切片则改为逐角色循环调用。**以现签名为准，保持行为**。

`ListAll` 新增（`RoleRepository` 接口 + 实现 + memRoleRepo 测试同步）：

```go
	ListAll(ctx context.Context) ([]*model.Role, error)
```

```go
func (r *roleRepo) ListAll(ctx context.Context) ([]*model.Role, error) {
	var items []*model.Role
	err := r.db.WithContext(ctx).Find(&items).Error
	return items, err
}
```

（memRoleRepo：返回全部。）

2. `RebuildRolePolicies(ctx, roleID)` 实现：取角色 → 取该角色 menuIDs → 取菜单实体 → `rebuildRolePolicies`。

3. **触发点**：

```go
func (s *roleService) AssignMenus(ctx context.Context, roleID uint64, menuIDs []uint64) error {
	tid := tenant.TenantIDFromContext(ctx)
	if err := s.roleMenuRepo.SetRoleMenus(ctx, tid, roleID, menuIDs); err != nil {
		return err
	}
	if s.permSvc != nil { // 见下方构造调整
		return s.permSvc.RebuildRolePolicies(ctx, roleID)
	}
	return nil
}
```

`roleService` 增加 `permSvc PermissionService` 依赖——**注意循环**：`NewPermissionService` 依赖 roleRepo 等，`NewRoleService` 依赖 permSvc 无环（permSvc 不依赖 roleService）✓。`NewRoleService(roleRepo, roleMenuRepo, permSvc)`，module.go 提供处同步（FX 自动解析）。

同理 `roleService.Delete` 成功后调用 `permSvc.RebuildPolicies(ctx)`（角色删了，其策略由 RemoveFilteredPolicy 清——更精确：删除前取 role，删除后 `e.RemoveFilteredPolicy(0, roleSub)`；**简化**：Delete 后 `RebuildPolicies`）。`menuService.Create/Delete` 成功后同样 `RebuildPolicies`——`menuService` 也要注入 `permSvc`。

4. `EnsureUserRolePolicy` 前缀化（同文件）：

```go
func (s *permissionService) EnsureUserRolePolicy(ctx context.Context, username string, roles []*model.Role) error {
	if s.enforcer == nil || username == "" {
		return nil
	}
	if err := s.enforcer.LoadPolicy(); err != nil {
		return err
	}
	tid := tenant.TenantIDFromContext(ctx)
	userSub := fmt.Sprintf("%d:%s", tid, username)
	existing, _ := s.enforcer.GetRolesForUser(userSub)
	for _, r := range existing {
		if _, err := s.enforcer.DeleteRoleForUser(userSub, r); err != nil {
			return err
		}
	}
	for _, r := range roles {
		if _, err := s.enforcer.AddRoleForUser(userSub, roleSub(r.TenantID, r.Code)); err != nil {
			return err
		}
	}
	return s.enforcer.SavePolicy()
}
```

5. **seed.go**（`StartSeeder` 末尾）替换现有 `AddPolicy/AddRoleForUser` 段为前缀化 + 全量重灌：

```go
	if p.Enforcer != nil {
		_ = p.Enforcer.LoadPolicy()
		tid := tenant.TenantIDFromContext(ctx)
		// 种子角色策略走统一重建（与 AssignMenus 同一路径，避免两套逻辑漂移）。
		if rerr := p.Perm.RebuildPolicies(ctx); rerr != nil && logger.Log != nil {
			logger.Log.Warn("rebuild policies failed", zap.Error(rerr))
		}
		if _, aerr := p.Enforcer.AddRoleForUser(
			fmt.Sprintf("%d:%s", tid, username),
			roleSub(tid, roleCode),
		); aerr != nil && logger.Log != nil {
			logger.Log.Warn("seed role link failed", zap.Error(aerr))
		}
		if serr := p.Enforcer.SavePolicy(); serr != nil && logger.Log != nil {
			logger.Log.Warn("save policy failed", zap.Error(serr))
		}
	}
```

（`seed.go` 现有 `p.Perm` 字段名以实际为准——读文件后适配；`roleSub` 在同包 service 可直接用。**注意**：种子在 `tid` 上下文——确认 seed 的 ctx 是否带 tenant（现有 seed 用 `tid` 变量——从 ctx 取或常量，保持原逻辑。）

- [ ] **Step 5: 中间件测试适配**

`pkg/middleware/auth_test.go` 的 Casbin 用例（`TestAuthMiddleware_CasbinDeny_403`/`CasbinAllow_200`）：
1. 策略 subject 改前缀：`enf.AddPolicy("1:alice", "/api/admin/system/users", "GET")`
2. fake/validator claims 的 `TenantID: 1`、`Username: "alice"`（token 模式用例里配）
3. 用例 cfg 用 jwt 模式时，jwt token 的 claims TenantID 也必须是 1（`auth.GenerateToken(uid, "alice", 1, cfg)`——现有签名 tenantID 参数已是 1 ✓）

- [ ] **Step 6: 验证**

Run: `CGO_ENABLED=0 go build ./... && go vet ./... && CGO_ENABLED=0 go test ./modules/system/... ./pkg/middleware/ -v -count=1`
Expected: policy 3 测试 + middleware 全量 PASS
Run: `CGO_ENABLED=0 go test ./... -count=1`
Expected: 全绿

- [ ] **Step 7: Commit**

```bash
git add modules/system/ pkg/middleware/
git commit -m "feat(rbac): role_menus to casbin policy sync, tenant-prefixed subjects, startup rebuild"
```

---

### Task 10: data_scope 数据权限查询注入

**Files:**
- Create: `pkg/tenant/datascope.go`（ctx 结构与构造）
- Modify: `pkg/tenant/gormplugin/plugin.go`（查询回调叠加 scope 条件）
- Modify: `pkg/middleware/auth.go`（`AuthDeps` 加 `ScopeLoader`，中间件计算并注入）
- Create: `modules/system/service/datascope.go`（ScopeLoader 实现：读角色 data_scope + 部门树）
- Modify: `pkg/tenant/gormplugin/plugin_test.go`（五档断言）
- Modify: `modules/system/module.go`（Provide ScopeLoader）

**Interfaces:**
- Consumes: `roles.data_scope`（Task 8）、`system_users.dept_id`、`depts.parent_id`、`token.Claims.DeptID`（Task 2/3）
- Produces:

```go
// pkg/tenant
type DataScope struct {
	Mode       int      // 1全部 2自定义 3本部门 4本部门及以下 5仅本人
	UserID     uint64
	DeptID     uint64
	DeptIDs    []uint64 // mode=2 的自定义集合 / mode=3,4 计算结果
}
func WithDataScope(ctx context.Context, ds DataScope) context.Context
func DataContext(ctx context.Context) (DataScope, bool)

// pkg/middleware
type ScopeLoader interface {
	// Load 根据用户取"最宽松"数据范围（登录后每请求调用，结果不缓存——一期取舍）。
	Load(ctx context.Context, userID, deptID uint64) (tenant.DataScope, error)
}
// AuthDeps 增加字段：ScopeLoader ScopeLoader（nil=不注入，member 端/关闭时）

// pkg/tenant/gormplugin：查询回调在租户过滤之后叠加：
// - Mode1：不加条件
// - Mode2：dept_id IN (DeptIDs)
// - Mode3：dept_id = DeptID
// - Mode4：dept_id IN (DeptIDs)（本部门及以下集合由 Loader 算好）
// - Mode5：id = UserID
// 仅当模型有 DeptID 字段且未 IgnoreTenant 时生效。
```

- [ ] **Step 1: 写失败测试（插件五档）**

`pkg/tenant/gormplugin/plugin_test.go` 追加：

```go
type empRow struct {
	ID       uint64 `gorm:"primaryKey"`
	Name     string
	DeptID   uint64 `gorm:"column:dept_id"`
	TenantID uint64 `gorm:"column:tenant_id"`
}

func (empRow) TableName() string { return "emp_rows" }

func setupEmp(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&empRow{}); err != nil {
		t.Fatal(err)
	}
	if err := Register(db, Options{}); err != nil {
		t.Fatal(err)
	}
	seed := []empRow{
		{Name: "u1", DeptID: 10, TenantID: 1},
		{Name: "u2", DeptID: 20, TenantID: 1},
		{Name: "u3", DeptID: 21, TenantID: 1},
	}
	if err := db.Create(&seed).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func names(t *testing.T, db *gorm.DB, ctx context.Context) []string {
	t.Helper()
	var rows []empRow
	if err := db.WithContext(ctx).Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Name)
	}
	sort.Strings(out)
	return out
}

func baseCtx() context.Context {
	return tenant.WithTenantID(context.Background(), 1)
}

func TestDataScope_AllModes(t *testing.T) {
	db := setupEmp(t)

	// Mode 1 全部
	ctx := tenant.WithDataScope(baseCtx(), tenant.DataScope{Mode: 1, UserID: 1, DeptID: 10})
	if got := names(t, db, ctx); len(got) != 3 {
		t.Errorf("mode1 = %v, want 3 rows", got)
	}

	// Mode 2 自定义 [20,21]
	ctx = tenant.WithDataScope(baseCtx(), tenant.DataScope{Mode: 2, UserID: 1, DeptIDs: []uint64{20, 21}})
	if got := names(t, db, ctx); len(got) != 2 || got[0] != "u2" || got[1] != "u3" {
		t.Errorf("mode2 = %v", got)
	}

	// Mode 3 本部门（dept 10）
	ctx = tenant.WithDataScope(baseCtx(), tenant.DataScope{Mode: 3, UserID: 1, DeptID: 10})
	if got := names(t, db, ctx); len(got) != 1 || got[0] != "u1" {
		t.Errorf("mode3 = %v", got)
	}

	// Mode 4 本部门及以下（Loader 算出 10,11 —— 种子没有 dept 11 的人，只有 10）
	ctx = tenant.WithDataScope(baseCtx(), tenant.DataScope{Mode: 4, UserID: 1, DeptID: 10, DeptIDs: []uint64{10, 11}})
	if got := names(t, db, ctx); len(got) != 1 || got[0] != "u1" {
		t.Errorf("mode4 = %v", got)
	}

	// Mode 5 仅本人（user id=1 → 只有 u1 是 seed 第一行 id=1）
	ctx = tenant.WithDataScope(baseCtx(), tenant.DataScope{Mode: 5, UserID: 1, DeptID: 10})
	if got := names(t, db, ctx); len(got) != 1 || got[0] != "u1" {
		t.Errorf("mode5 = %v", got)
	}

	// 无 scope 注入（老行为）：不过滤
	if got := names(t, db, baseCtx()); len(got) != 3 {
		t.Errorf("no scope = %v", got)
	}
}

// 没有 DeptID 字段的模型不受 data scope 影响。
func TestDataScope_NoDeptField_Skip(t *testing.T) {
	db := setup(t, Options{})
	ctx := tenant.WithDataScope(WithTenantID(context.Background(), 1),
		tenant.DataScope{Mode: 5, UserID: 999})
	if err := db.WithContext(ctx).Create(&row{Name: "n"}).Error; err != nil {
		t.Fatal(err)
	}
	var got []row
	if err := db.WithContext(ctx).Find(&got).Error; err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Errorf("got %+v", got)
	}
}
```

（import 补 `sort`、`alexGo-cloud/pkg/tenant`；`setup`/`row` 是 Task 6 已有 helper。**Mode5 断言依赖 seed 第一行 id=1**——sqlite 自增从 1 ✓。）

- [ ] **Step 2: 运行确认失败**

Run: `CGO_ENABLED=0 go test ./pkg/tenant/... -run TestDataScope -v`
Expected: 编译失败 `undefined: WithDataScope/DataContext` 或断言失败（scope 未生效）

- [ ] **Step 3: 实现**

`pkg/tenant/datascope.go`：

```go
package tenant

import "context"

type dsKey int

const dataScopeKey dsKey = iota

// DataScope 请求级数据权限（由 AuthMiddleware 计算后注入，gormplugin 消费）。
type DataScope struct {
	Mode    int // 1全部 2自定义 3本部门 4本部门及以下 5仅本人
	UserID  uint64
	DeptID  uint64
	DeptIDs []uint64
}

func WithDataScope(ctx context.Context, ds DataScope) context.Context {
	return context.WithValue(ctx, dataScopeKey, ds)
}

func DataContext(ctx context.Context) (DataScope, bool) {
	v, ok := ctx.Value(dataScopeKey).(DataScope)
	return v, ok
}
```

`pkg/tenant/gormplugin/plugin.go` 的 `addFilter` 在租户条件追加后叠加 scope（同一回调内顺序执行）：

```go
		// data_scope：仅对带 dept_id 字段的模型生效；无 scope 注入则保持现状。
		if ds, ok := tenant.DataContext(st.Context); ok && st.Schema.LookUpField("DeptID") != nil {
			switch ds.Mode {
			case 2:
				if len(ds.DeptIDs) > 0 {
					st.AddClause(clause.Where{Exprs: []clause.Expression{
						clause.IN{Column: clause.Column{Name: "dept_id"}, Values: deptValues(ds.DeptIDs)},
					}})
				} else {
					// 自定义集合为空 → 按"看不到"处理（显式 false 条件）。
					st.AddClause(clause.Where{Exprs: []clause.Expression{
						clause.Expr{SQL: "1 = 0"},
					}})
				}
			case 3:
				st.AddClause(clause.Where{Exprs: []clause.Expression{
					clause.Eq{Column: clause.Column{Name: "dept_id"}, Value: ds.DeptID},
				}})
			case 4:
				if len(ds.DeptIDs) > 0 {
					st.AddClause(clause.Where{Exprs: []clause.Expression{
						clause.IN{Column: clause.Column{Name: "dept_id"}, Values: deptValues(ds.DeptIDs)},
					}})
				}
			case 5:
				st.AddClause(clause.Where{Exprs: []clause.Expression{
					clause.Eq{Column: clause.Column{Name: "id"}, Value: ds.UserID},
				}})
			}
			// Mode 1 / 0：不加条件
		}
```

辅助：

```go
func deptValues(ids []uint64) []interface{} {
	out := make([]interface{}, 0, len(ids))
	for _, id := range ids {
		out = append(out, id)
	}
	return out
}
```

（注意：**只有 Query 回调加 scope**——Update/Delete 不加（管理操作按主键+租户已够；scope 是列表可见性的下限，避免把管理端 Update 误杀）。在 `addFilter` 内区分：Query 走完整逻辑，Update/Delete 只走租户部分——实现方式：`addFilter` 接参数 `withScope bool`，`db.Callback().Query().Before("gorm:query").Register(..., func(db){addFilter(db, true)})`，Update/Delete 传 false。）

- [ ] **Step 4: ScopeLoader + 中间件注入**

`modules/system/service/datascope.go`：

```go
package service

import (
	"context"
	"fmt"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/repository"
	"alexGo-cloud/pkg/tenant"
)

// dataScopeLoader：读用户角色的 data_scope，取最宽松档（数值越小越宽松：1>2>3>4>5）。
type dataScopeLoader struct {
	roleRepo     repository.UserRoleRepository
	roleQuery    repository.RoleRepository
	menuless     struct{} // 占位防误加
	userRepo     repository.UserRepository
	deptRepo     repository.DeptRepository
	userRoleRepo repository.UserRoleRepository
}

// NewDataScopeLoader 构造（依赖经 module.go 提供）。
func NewDataScopeLoader(
	roleRepo repository.RoleRepository,
	userRoleRepo repository.UserRoleRepository,
	userRepo repository.UserRepository,
	deptRepo repository.DeptRepository,
) *dataScopeLoader {
	return &dataScopeLoader{
		roleQuery: roleRepo, userRoleRepo: userRoleRepo,
		userRepo: userRepo, deptRepo: deptRepo,
	}
}
```

**（实现裁决——结构体字段去重）**：上面 struct 有冗余（menuless/roleRepo 冗余），收敛为：

```go
type dataScopeLoader struct {
	roleQuery    repository.RoleRepository
	userRoleRepo repository.UserRoleRepository
	userRepo     repository.UserRepository
	deptRepo     repository.DeptRepository
}

func (l *dataScopeLoader) Load(ctx context.Context, userID, deptID uint64) (tenant.DataScope, error) {
	tid := tenant.TenantIDFromContext(ctx)
	roleIDs, err := l.userRoleRepo.ListRoleIDsByUser(ctx, tid, userID)
	if err != nil {
		return tenant.DataScope{}, err
	}
	// 取所有角色实体
	best := 5 // 从最严开始，逐角色放宽
	var customIDs string
	for _, rid := range roleIDs {
		r, err := l.roleQuery.GetByID(ctx, tid, rid)
		if err != nil {
			continue
		}
		if r.DataScope >= 1 && r.DataScope < best {
			best = r.DataScope
		}
		if r.DataScope == 2 && r.DataScopeDeptIDs != "" {
			customIDs = r.DataScopeDeptIDs
		}
	}
	ds := tenant.DataScope{Mode: best, UserID: userID, DeptID: deptID}
	switch best {
	case 2: // 自定义："1,2,3" 逗号分隔
		ds.DeptIDs = parseUintList(customIDs)
	case 3:
		// dept_ids 不需要——插件用 ds.DeptID
	case 4:
		ds.DeptIDs = l.descendants(ctx, tid, deptID)
	}
	return ds, nil
}

// descendants：从 deptID 出发沿 parent_id 树收集自身+全部后代（depts 全量一次查，内存建树——租户内部门量级小，避免递归 SQL）。
func (l *dataScopeLoader) descendants(ctx context.Context, tid, deptID uint64) []uint64 {
	all, err := l.deptRepo.List(ctx, tid)
	if err != nil || deptID == 0 {
		return nil
	}
	children := map[uint64][]uint64{}
	for _, d := range all {
		children[d.ParentID] = append(children[d.ParentID], d.ID)
	}
	out := []uint64{deptID}
	queue := []uint64{deptID}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, ch := range children[cur] {
			out = append(out, ch)
			queue = append(queue, ch)
		}
	}
	return out
}

func parseUintList(s string) []uint64 {
	var out []uint64
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		var v uint64
		if _, err := fmt.Sscanf(p, "%d", &v); err == nil && v > 0 {
			out = append(out, v)
		}
	}
	return out
}
```

（import 补 `strings`；`DeptRepository.List` 签名——**读 `repository/basic.go` 实际接口后对齐**（现有 `NewDeptRepository` 在 module.go 提供，方法名以实际为准，可能已是 `List(ctx, tid)`）。`NewDataScopeLoader` 返回具体类型——module.go Provide 接口 `middleware.ScopeLoader`：加一层

```go
		func(l *dataScopeLoader) middleware.ScopeLoader { return l },
```

——**不行**（system 引用 middleware 包造成依赖方向问题吗？module.go 已 import server/gin，import middleware 无环——可行。更干净：ScopeLoader 接口定义放 `pkg/tenant`（与 DataScope 同处）：

```go
// pkg/tenant/datascope.go
type ScopeLoader interface {
	Load(ctx context.Context, userID, deptID uint64) (DataScope, error)
}
```

`AuthDeps.ScopeLoader tenant.ScopeLoader`——middleware 已 import tenant ✓，system service 实现该接口 ✓，member 模块不提供（nil）。**采用此方案**。）

`pkg/middleware/auth.go`：

1. `AuthDeps` 加 `ScopeLoader tenant.ScopeLoader`（注释：nil=不注入数据范围，member 端与未启用时）
2. 中间件 `c.Set("claims", ...)` 之后追加：

```go
		if d.ScopeLoader != nil && claims.UserType == int(token.UserTypeAdmin) && claims.DeptID != 0 {
			if ds, err := d.ScopeLoader.Load(c.Request.Context(), claims.UserID, claims.DeptID); err == nil {
				c.Request = c.Request.WithContext(tenant.WithDataScope(c.Request.Context(), ds))
			}
			// Load 失败不阻断请求：无 scope = 只剩租户隔离（fail-safe 方向是"更少数据"还是"更多"？
			// 失败时注入 Mode=5（仅本人）更安全——收敛为：
		}
```

**（实现裁决——失败方向）**：Load 失败注入 Mode5：

```go
		if d.ScopeLoader != nil && claims.UserType == int(token.UserTypeAdmin) {
			ds := tenant.DataScope{Mode: 5, UserID: claims.UserID, DeptID: claims.DeptID} // 默认最严
			if loaded, err := d.ScopeLoader.Load(c.Request.Context(), claims.UserID, claims.DeptID); err == nil {
				ds = loaded
			}
			c.Request = c.Request.WithContext(tenant.WithDataScope(c.Request.Context(), ds))
		}
```

（DeptID=0 的管理员（超管）：Loader 对 dept 0 + 无角色返回 Mode 5 且 DeptID=0——mode5 走 `id=userID`，超管只看到自己？**种子 admin 有角色 data_scope=1**（seed 建角色时 DataScope 需为 1——**检查 seed**：种子角色 `Create` 旧签名没有 DataScope → 默认 1 ✓（Task 8 的 Create 默认 1）。故 admin 走 Mode1 全部 ✓。**同时**：mode5 且 DeptID=0 时 `id=userID` 仍合理。）

3. `http.go` 装配：`AuthDeps{...}` 加 `ScopeLoader: p.ScopeLoader`——`HTTPServerParams` 增加 `ScopeLoader tenant.ScopeLoader \`optional:"true"\``。
4. `modules/system/module.go` Provide `NewDataScopeLoader` + 接口映射 `tenant.ScopeLoader`。

- [ ] **Step 5: 验证**

Run: `CGO_ENABLED=0 go test ./pkg/tenant/... ./pkg/middleware/ ./modules/system/... -v -count=1`
Expected: data scope 5 档测试 PASS，其余不回归
Run: `CGO_ENABLED=0 go build ./... && go vet ./... && CGO_ENABLED=0 go test ./... -count=1`
Expected: 全绿

- [ ] **Step 6: Commit**

```bash
git add pkg/tenant/ pkg/middleware/ alexgo-server/ modules/system/
git commit -m "feat(tenant): data_scope query injection with per-request scope loader"
```


---

### Task 11: gRPC TokenService（proto + 服务端 + 客户端）

**Files:**
- Create: `modules/system/api/rpc/token.proto`
- Create: `modules/system/api/rpc/token.pb.go`、`token_grpc.pb.go`（生成后**提交**）
- Create: `modules/system/grpcserver/token_service.go`（Registrar 实现）
- Create: `pkg/client/token_client.go`（实现 `token.Issuer`）
- Modify: `modules/system/module.go`（Provide Registrar + 启动清扫 ticker）
- Modify: `alexgo-server/server/http.go`（无需改——gRPC 由 grpcserver 启动）
- Modify: `Makefile`（`proto` 含新 proto——`gen_proto.sh` 已 walk modules/**/*.proto，自动覆盖）

**Interfaces:**
- Consumes: `token.Issuer`/`token.Service`（Task 2）、`grpcserver.Registrar` + `group:"grpc_registrars"`（现有）
- Produces: `TokenService{IssueToken,RefreshToken,RevokeToken}` gRPC；`client.NewTokenClient(conn) token.Issuer`

- [ ] **Step 1: 工具链准备（披露项）**

```bash
# protoc 本机未装：brew 安装（工作区外系统改动，完成后在报告中披露）
brew install protobuf
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
export PATH="$(go env GOPATH)/bin:$PATH"
protoc --version
```

- [ ] **Step 2: 写 proto 并生成**

`modules/system/api/rpc/token.proto`：

```protobuf
syntax = "proto3";

package alexgo.system.rpc;
option go_package = "alexGo-cloud/modules/system/api/rpc";

// TokenService 是 OAuth2 令牌的跨服务契约：member-server 委托 system-server 签发/刷新/注销。
service TokenService {
  rpc IssueToken(IssueTokenRequest) returns (IssueTokenResponse);
  rpc RefreshToken(RefreshTokenRequest) returns (IssueTokenResponse);
  rpc RevokeToken(RevokeTokenRequest) returns (RevokeTokenResponse);
}

message IssueTokenRequest {
  int64 user_id = 1;
  int32 user_type = 2; // 1管理员 2会员
  int64 tenant_id = 3;
  string client_id = 4;
}

message IssueTokenResponse {
  string access_token = 1;
  string refresh_token = 2;
  int64 expires_in = 3;
}

message RefreshTokenRequest {
  string refresh_token = 1;
}

message RevokeTokenRequest {
  string access_token = 1;
}

message RevokeTokenResponse {
  bool ok = 1;
}
```

生成：

```bash
protoc --proto_path=. --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  modules/system/api/rpc/token.proto
ls modules/system/api/rpc/   # token.pb.go + token_grpc.pb.go
```

- [ ] **Step 3: 服务端实现**

`modules/system/grpcserver/token_service.go`：

```go
package grpcserver

import (
	"context"

	"alexGo-cloud/modules/system/api/rpc"
	"alexGo-cloud/pkg/token"
)

// TokenServiceImpl 把 pkg/token 暴露为 gRPC（micro 模式下 member-server 的委托入口）。
type TokenServiceImpl struct {
	rpc.UnimplementedTokenServiceServer
	svc *token.Service
}

func NewTokenServiceImpl(svc *token.Service) *TokenServiceImpl {
	return &TokenServiceImpl{svc: svc}
}

// Register 供 grpcserver.StartGRPCServer 的 group:"grpc_registrars" 聚合调用。
func (s *TokenServiceImpl) Register(server *grpc.Server) {
	rpc.RegisterTokenServiceServer(server, s)
}

func (s *TokenServiceImpl) IssueToken(ctx context.Context, req *rpc.IssueTokenRequest) (*rpc.IssueTokenResponse, error) {
	issued, err := s.svc.Issue(ctx, token.IssueParams{
		UserID:   uint64(req.GetUserId()),
		UserType: token.UserType(req.GetUserType()),
		TenantID: uint64(req.GetTenantId()),
		ClientID: req.GetClientId(),
	})
	if err != nil {
		return nil, err
	}
	return &rpc.IssueTokenResponse{
		AccessToken: issued.AccessToken, RefreshToken: issued.RefreshToken, ExpiresIn: issued.ExpiresIn,
	}, nil
}

func (s *TokenServiceImpl) RefreshToken(ctx context.Context, req *rpc.RefreshTokenRequest) (*rpc.IssueTokenResponse, error) {
	issued, err := s.svc.Refresh(ctx, req.GetRefreshToken())
	if err != nil {
		return nil, err
	}
	return &rpc.IssueTokenResponse{
		AccessToken: issued.AccessToken, RefreshToken: issued.RefreshToken, ExpiresIn: issued.ExpiresIn,
	}, nil
}

func (s *TokenServiceImpl) RevokeToken(ctx context.Context, req *rpc.RevokeTokenRequest) (*rpc.RevokeTokenResponse, error) {
	if err := s.svc.Revoke(ctx, req.GetAccessToken()); err != nil {
		return nil, err
	}
	return &rpc.RevokeTokenResponse{Ok: true}, nil
}
```

（import 补 `google.golang.org/grpc`。）

`modules/system/module.go` FxModule `fx.Provide` 追加：

```go
		grpcserver.NewTokenServiceImpl,
		fx.Annotate(
			grpcserver.NewTokenServiceImpl,
			fx.As(new(grpcserver.Registrar)),
			fx.ResultTags(`group:"grpc_registrars"`),
		),
```

（**二选一即可**：`As(Registrar)` 版本——同时满足组注册；`NewTokenServiceImpl` 原类型不必单独 Provide，除非 client 依赖。**采用 As 版本单条**；若 TokenServiceImpl 其他处需要原类型再补 Provide。import `modules/system/grpcserver`。）

**gRPC 启动门控（仅 micro）**：`modules/system/grpcserver/server.go` 的 `StartGRPCServer` 开头加：

```go
	if p.Cfg.Deployment.Mode != "micro" {
		return nil // mono 模式不起 gRPC（本地直调 token.Service，无需监听）
	}
```

**过期清扫 ticker**：`modules/system/module.go` FxModule `fx.Invoke` 追加（仿 `outbox.StartRelay` 模式，内联实现）：

```go
	fx.Invoke(func(lc fx.Lifecycle, svc *token.Service, cfg *config.Config) {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				go func() {
					ticker := time.NewTicker(time.Hour)
					defer ticker.Stop()
					for {
						select {
						case <-ctx.Done():
							return
						case <-ticker.C:
							if err := svc.SweepExpired(context.Background()); err != nil && logger.Log != nil {
								logger.Log.Warn("token sweep failed", zap.Error(err))
							}
						}
					}
				}()
				return nil
			},
		})
	}),
```

（import `time`、`config`、`logger`、`zap`——module.go 已有部分，核对补齐。注意 fx.Lifecycle 的 OnStart ctx 在启动完成即 cancel——**必须用独立 context**：goroutine 内 `context.Background()` ✓（sweep 调用已用 Background）；`<-ctx.Done()` 会立刻返回导致 goroutine 秒退——**改为监听外部信号不可得**。**修正**：该 hook 的 OnStart ctx 不复用，用 `lc` 注册 OnStop cancel：

```go
		fx.Invoke(func(lc fx.Lifecycle, svc *token.Service) {
			runCtx, cancel := context.WithCancel(context.Background())
			lc.Append(fx.Hook{
				OnStart: func(context.Context) error {
					go func() {
						ticker := time.NewTicker(time.Hour)
						defer ticker.Stop()
						for {
							select {
							case <-runCtx.Done():
								return
							case <-ticker.C:
								if err := svc.SweepExpired(context.Background()); err != nil && logger.Log != nil {
									logger.Log.Warn("token sweep failed", zap.Error(err))
								}
							}
						}
					}()
					return nil
				},
				OnStop: func(context.Context) error { cancel(); return nil },
			})
		}),
```

**采用此修正版**。）

- [ ] **Step 4: 客户端实现**

`pkg/client/token_client.go`：

```go
package client

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"alexGo-cloud/modules/system/api/rpc"
	"alexGo-cloud/pkg/token"
)

// NewTokenIssuer 连接 system-server 的 TokenService，返回可用作 member-server
// 签发入口的 token.Issuer。调用方（fx）负责管理连接生命周期（grpc_conn.go 现有模式）。
func NewTokenIssuer(conn grpc.ClientConnInterface) token.Issuer {
	return &tokenGRPCClient{conn: conn}
}

type tokenGRPCClient struct {
	conn grpc.ClientConnInterface
}

func (c *tokenGRPCClient) client() rpc.TokenServiceClient {
	return rpc.NewTokenServiceClient(c.conn)
}

func (c *tokenGRPCClient) Issue(ctx context.Context, p token.IssueParams) (*token.Issued, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("token: grpc conn is nil")
	}
	resp, err := c.client().IssueToken(ctx, &rpc.IssueTokenRequest{
		UserId: int64(p.UserID), UserType: int32(p.UserType),
		TenantId: int64(p.TenantID), ClientId: p.ClientID,
	})
	if err != nil {
		return nil, err // gRPC 故障 → 登录接口 503（controller 层映射），不降级
	}
	return &token.Issued{
		AccessToken: resp.GetAccessToken(), RefreshToken: resp.GetRefreshToken(), ExpiresIn: resp.GetExpiresIn(),
	}, nil
}

func (c *tokenGRPCClient) Refresh(ctx context.Context, refreshToken string) (*token.Issued, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("token: grpc conn is nil")
	}
	resp, err := c.client().RefreshToken(ctx, &rpc.RefreshTokenRequest{RefreshToken: refreshToken})
	if err != nil {
		return nil, err
	}
	return &token.Issued{
		AccessToken: resp.GetAccessToken(), RefreshToken: resp.GetRefreshToken(), ExpiresIn: resp.GetExpiresIn(),
	}, nil
}

func (c *tokenGRPCClient) Revoke(ctx context.Context, accessToken string) error {
	if c.conn == nil {
		return fmt.Errorf("token: grpc conn is nil")
	}
	_, err := c.client().RevokeToken(ctx, &rpc.RevokeTokenRequest{AccessToken: accessToken})
	return err
}

func (c *tokenGRPCClient) RevokeAll(_ context.Context, _ token.UserType, _ uint64) error {
	return fmt.Errorf("token: RevokeAll not supported over grpc（踢人接口仅 system-server 本地使用）")
}

// 编译期断言：确保客户端满足接口。
var _ token.Issuer = (*tokenGRPCClient)(nil)
```

（`insecure` import 未用则删；连接建立方式读 `pkg/client/grpc_conn.go`——若 `NewGRPCConn` 返回 `*grpc.ClientConn`，`NewTokenIssuer` 接口化签名保持 `grpc.ClientConnInterface` 兼容。**踢人（RevokeAll）只在 system-server 本地**——member 会员管理"禁用+踢人"一期不做联动（Task 5 注释已声明）。）

**连接提供**：`client.NewGRPCConn` 现有实现——读它；若未启用 micro 返回 nil（现有语义）。`member-server` 进程将用 `NewTokenIssuer(conn)` 提供 `token.Issuer`（Task 12）。

- [ ] **Step 5: bufconn 集成测试**

创建 `pkg/client/token_client_test.go`：

```go
package client

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"

	"alexGo-cloud/modules/system/api/rpc"
	"alexGo-cloud/modules/system/grpcserver"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/token"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// fake 的 token 服务端实现（不走 DB：直接回定值，验证 gRPC 通路与参数映射）。
type stubTokenServer struct {
	rpc.UnimplementedTokenServiceServer
	gotUserID int64
}

func (s *stubTokenServer) IssueToken(_ context.Context, req *rpc.IssueTokenRequest) (*rpc.IssueTokenResponse, error) {
	s.gotUserID = req.GetUserId()
	return &rpc.IssueTokenResponse{AccessToken: "ga", RefreshToken: "gr", ExpiresIn: 60}, nil
}

func dialBuf(t *testing.T, srv *stubTokenServer) *grpc.ClientConn {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	rpc.RegisterTokenServiceServer(s, srv)
	go func() { _ = s.Serve(lis) }()
	t.Cleanup(s.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestTokenIssuer_OverGRPC(t *testing.T) {
	stub := &stubTokenServer{}
	conn := dialBuf(t, stub)
	iss := NewTokenIssuer(conn)

	issued, err := iss.Issue(context.Background(), token.IssueParams{
		UserID: 42, UserType: token.UserTypeMember, TenantID: 1, ClientID: "alexgo-app",
	})
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if issued.AccessToken != "ga" || issued.ExpiresIn != 60 {
		t.Errorf("issued = %+v", issued)
	}
	if stub.gotUserID != 42 {
		t.Errorf("user_id mapped to %d, want 42", stub.gotUserID)
	}
}

// gRPC 不可达 → Issue 报错（登录层将映射 503，绝不降级）。
func TestTokenIssuer_Unreachable(t *testing.T) {
	iss := NewTokenIssuer(nil)
	if _, err := iss.Issue(context.Background(), token.IssueParams{UserID: 1}); err == nil {
		t.Error("nil conn must error")
	}
}
```

（import 补 `credentials/insecure`；`grpcserver` 未用则删；本测试验证**契约通路**——真实 `TokenServiceImpl` 的单测在 `modules/system/grpcserver/token_service_test.go` 补充：用 sqlite `token.Service` + bufconn 走完整 Issue→Validate：

```go
// modules/system/grpcserver/token_service_test.go
func TestTokenService_IssueThenValidate(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	_ = db.AutoMigrate(/* 租户与 token 表结构：直接建 accessToken 需导出问题——
	   token 包 accessToken 未导出。改为 AutoMigrate 用原生 SQL 建表： */
	)
	...
}
```

**实现裁决（避免导出问题）**：`token` 包补一个**测试专用导出**不优雅——改在 `pkg/token/service_test.go` 已覆盖 Issue→Validate；grpcserver 层只测**参数映射**（stub db 不需要）：`TokenServiceImpl` 直接构造 `&token.Service{}` 不可行（无导出构造带 db）。**收敛**：grpcserver 测试用 `token.NewService(sqliteDB, cfg)` + 手工 SQL 建 `tenants` 与 token 表（`CREATE TABLE` 在测试里执行——表结构照 Task 1 DDL 简化版）：

```go
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
			expires_time DATETIME, deleted INTEGER DEFAULT 0, tenant_id INTEGER)`,
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
	claims, err := svc.Validate(context.Background(), resp.GetAccessToken())
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if claims.UserID != 7 || claims.Username != "alice" || claims.DeptID != 3 {
		t.Errorf("claims = %+v", claims)
	}
}
```

（`token.Service` 的 `accessToken` GORM 模型 `TableName()` 已映射表名——手工 DDL 与模型列对齐即可。）

- [ ] **Step 6: 验证**

Run: `bash -n scripts/gen_proto.sh && CGO_ENABLED=0 go build ./... && go vet ./... && CGO_ENABLED=0 go test ./pkg/client/ ./modules/system/... -v -count=1 && CGO_ENABLED=0 go test ./... -count=1`
Expected: 全绿；`git status` 显示 `token.pb.go/token_grpc.pb.go` 待提交

- [ ] **Step 7: Commit**

```bash
git add modules/system/api/rpc/ modules/system/grpcserver/ modules/system/module.go pkg/client/
git commit -m "feat(rpc): TokenService gRPC contract, server registrar and issuer client"
```

---

### Task 12: member-server 独立进程 + 双运行模式

**Files:**
- Create: `modules/member/cmd/main.go`
- Modify: `alexgo-server/cmd/main.go`（cfg 提前加载 + 按模式条件装配 + mono 装 member）
- Modify: `modules/member/module.go`（补 `fx.Provide(token issuer 装配)`——issuer 由入口提供）
- Modify: `Makefile`（`run`（mono）、`run-system`、`run-member`）
- Create: `pkg/middleware/http_stack.go`？——**不建**：member cmd 内联组装（见 Step 2）

**Interfaces:**
- Consumes: `deployment.mode`（Task 2 配置）、`client.NewTokenIssuer`（Task 11）、`token.NewService`（Task 2）、各模块 FxModule
- Produces: `modules/member/cmd` 二进制（:8081）；mono `make run` 单进程含全部路由

- [ ] **Step 1: mono 入口改造（先失败测试——构建即测试）**

`alexgo-server/cmd/main.go` 整文件替换：

```go
package main

import (
	"flag"
	"fmt"
	"time"

	"go.uber.org/fx"
	"google.golang.org/grpc"

	"alexGo-cloud/alexgo-server/server"
	"alexGo-cloud/modules/member"
	"alexGo-cloud/modules/order"
	"alexGo-cloud/modules/system"
	systemservice "alexGo-cloud/modules/system/service"
	"alexGo-cloud/pkg/auth"
	"alexGo-cloud/pkg/circuitbreaker"
	"alexGo-cloud/pkg/client"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/database"
	"alexGo-cloud/pkg/limiter"
	"alexGo-cloud/pkg/logger"
	"alexGo-cloud/pkg/migrate"
	"alexGo-cloud/pkg/mq"
	"alexGo-cloud/pkg/outbox"
	appredis "alexGo-cloud/pkg/redis"
	"alexGo-cloud/pkg/token"
	"alexGo-cloud/pkg/trace"
)

// main 是 alexGo-cloud 的 system-server 入口：
// - mono 模式（默认）：单进程装配 system+order+member 全部模块（make run 一键全起）；
// - micro 模式：本进程只装 system+order，member 由 modules/member/cmd 独立启动，
//   会员登录经 gRPC TokenService 委托本进程签发。
func main() {
	migrateOnly := flag.Bool("migrate-only", false, "run migrations only")
	flag.Parse()

	logger.Init()
	if logger.Log != nil {
		defer func() { _ = logger.Log.Sync() }()
	}

	// 配置提前加载：模块装配（mono/micro）在 fx 构建期就需要 deployment.mode，
	// 无法等 fx 内 Provide 再分支——所以此处直接加载并 fx.Supply。
	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		if logger.Log != nil {
			logger.Log.Fatal("load config failed", zap.Error(err))
		}
		panic(err)
	}
	mono := cfg.Deployment.Mode != "micro"

	opts := []fx.Option{
		fx.Supply(cfg),
		fx.Provide(
			database.NewDB,
			client.NewGRPCConn,
			auth.NewCasbinEnforcer,
			appredis.NewClient,
			limiter.NewRateLimiter,
			func(cfg *config.Config) *circuitbreaker.CircuitBreaker {
				if cfg == nil || !cfg.Breaker.Enabled {
					return nil
				}
				return circuitbreaker.NewCircuitBreaker(cfg.Breaker.Threshold, time.Duration(cfg.Breaker.OpenSecond)*time.Second)
			},
			mq.NewNATSBroker,
			outbox.NewRelay,
			token.NewService,
			fx.Annotate(
				migrate.NewRunner,
				fx.ParamTags("", "", `group:"migration_sources"`),
			),
		),
		fx.Invoke(outbox.StartRelay),
		fx.Decorate(func(cfg *config.Config, local systemservice.UserService, conn *grpc.ClientConn) (systemservice.UserService, error) {
			if cfg == nil || !cfg.Microservice.Enabled {
				return local, nil
			}
			if conn == nil {
				return nil, fmt.Errorf("microservice enabled but grpc conn is nil")
			}
			return client.NewUserGRPCClient(conn), nil
		}),
		fx.Invoke(trace.InitTracer),
		fx.Invoke(func(r *migrate.Runner) error { return r.Run() }),
		system.FxModule,
		order.FxModule,
	}
	if mono {
		// 单体模式：member 路由同进程注册，TokenIssuer 用本地 token.Service。
		opts = append(opts, member.FxModule)
	}
	if !*migrateOnly {
		opts = append(opts, fx.Invoke(server.StartHTTPServer))
	}
	fx.New(opts...).Run()
}
```

（import 补 `go.uber.org/zap`；**原 `config.LoadGlobalConfig` 从 fx.Provide 移除**——`fx.Supply(cfg)` 供全部注入点。其余 Provider 与原文件保持一致（`auth.NewCasbinEnforcer` 等原文照抄，**读现文件核对完整清单**——原文件还有 `trace.InitTracer` Invoke、`pprof` 等已在此；以"现文件减 LoadGlobalConfig、加 Supply/token.NewService/member 条件"为改动原则，不要丢 Provider。）

**注意**：`member.FxModule` 的 `service.NewMemberService(repo, issuer, limit)` 需要 `token.Issuer`——mono 下由 `token.NewService` 提供 `*token.Service`，**需要接口映射**（`*token.Service` 同时实现 Issuer，但 FX 按具体类型 `*token.Service` 提供，`token.Issuer` 接口无人提供）。member FxModule 追加：

```go
		func(s *token.Service) token.Issuer { return s },
```

（`modules/member/module.go` import `alexGo-cloud/pkg/token`。system 模块的 `NewAuthService(..., issuer token.Issuer)` 同理——**system FxModule 也要这条 Provide**（或入口统一 Provide 一次——**收敛**：入口 main 里 fx.Provide 加：

```go
			func(s *token.Service) token.Issuer { return s },
```

全局唯一，两模块共用。**放入口，不放模块**——member-server 入口同样要写（见 Step 2）。）

- [ ] **Step 2: member-server 入口**

`modules/member/cmd/main.go`：

```go
package main

import (
	"flag"

	"go.uber.org/fx"

	"alexGo-cloud/alexgo-server/server"
	"alexGo-cloud/modules/member"
	"alexGo-cloud/pkg/client"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/database"
	"alexGo-cloud/pkg/logger"
	"alexGo-cloud/pkg/migrate"
	"alexGo-cloud/pkg/token"
	"alexGo-cloud/pkg/trace"
)

// member-server：member 模块的独立进程（micro 模式）。
// 与 system-server 的差异：
// - HTTP 端口 :8081（HTTP_ADDR env 覆盖）；
// - 无 Casbin/审计（member 无 RBAC）；
// - token.Issuer = gRPC 客户端（委托 system-server，失败即登录 503）；
// - 校验（Validator）= 本地 token.Service 读共享 token 表（同库约束，spec §9.10）。
func main() {
	migrateOnly := flag.Bool("migrate-only", false, "run migrations only")
	flag.Parse()

	logger.Init()
	if logger.Log != nil {
		defer func() { _ = logger.Log.Sync() }()
	}

	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		panic(err)
	}
	if cfg.Deployment.Mode == "" {
		cfg.Deployment.Mode = "micro"
	}

	opts := []fx.Option{
		fx.Supply(cfg),
		fx.Provide(
			database.NewDB,
			client.NewGRPCConn,
			token.NewService, // Validator：本地校验共享 token 表
			func(s *token.Service) token.Validator { return s },
			func(conn *grpc.ClientConn) token.Issuer {
				// 委托 system-server 签发；conn 为 nil 时 Issuer 各方法返回错误 → 登录 503。
				if conn == nil {
					return nilIssuer{}
				}
				return client.NewTokenIssuer(conn)
			},
			fx.Annotate(
				migrate.NewRunner,
				fx.ParamTags("", "", `group:"migration_sources"`),
			),
		),
		fx.Invoke(trace.InitTracer),
		fx.Invoke(func(r *migrate.Runner) error { return r.Run() }),
		member.FxModule,
	}
	if !*migrateOnly {
		opts = append(opts, fx.Invoke(server.StartHTTPServer))
	}
	fx.New(opts...).Run()
}

// nilIssuer：gRPC 未连接时的 fail-fast 实现（所有操作返回错误，登录接口映射 503）。
type nilIssuer struct{}

func (nilIssuer) Issue(context.Context, token.IssueParams) (*token.Issued, error) {
	return nil, fmt.Errorf("member-server: system grpc not connected")
}
func (nilIssuer) Refresh(context.Context, string) (*token.Issued, error) {
	return nil, fmt.Errorf("member-server: system grpc not connected")
}
func (nilIssuer) Revoke(context.Context, string) error {
	return fmt.Errorf("member-server: system grpc not connected")
}
func (nilIssuer) RevokeAll(context.Context, token.UserType, uint64) error {
	return fmt.Errorf("member-server: system grpc not connected")
}
```

（import 补 `context`、`fmt`、`google.golang.org/grpc`。**HTTP_ADDR**：`config` 已 BindEnv HTTP_ADDR → Makefile 传 `HTTP_ADDR=:8081`。**gin 装配**：`server.StartHTTPServer` 复用——member 进程的 `HTTPServerParams`：`Enforcer` optional 无提供→nil ✓；`TokenValidator` 提供 ✓；`ScopeLoader` 不提供→nil ✓；`TenantDomainLookup` 需要——`tenant.NewDomainLookup` 也要 Provide（本入口 fx.Provide 补 `tenant.NewDomainLookup`）。**AuditRecorder** optional 无提供 ✓。**Casbin Enforcer**：system FxModule 不在本进程，`auth.NewCasbinEnforcer` 也不 Provide ✓。）

**中间件注意**：`server.StartHTTPServer` 的链里 `RateLimitAndBreaker`、`AuthMiddleware` 均 nil-safe ✓；`NewTenantMiddleware` 需要 lookup——`HTTPServerParams.TenantDomainLookup` optional 为 nil 时只认头 ✓，但仍 Provide 以支持域名解析。

- [ ] **Step 3: Makefile**

追加：

```make
run: ## mono 模式：单进程启动全部模块（默认）
	go run ./alexgo-server/cmd/main.go

run-system: ## micro 模式 system-server（含 gRPC TokenService）
	DEPLOYMENT_MODE=micro go run ./alexgo-server/cmd/main.go

run-member: ## micro 模式 member-server（:8081，委托 system 签发）
	HTTP_ADDR=:8081 SYSTEM_GRPC_ADDR=127.0.0.1:50051 go run ./modules/member/cmd/main.go
```

（现有 `run` 目标替换为带注释版本；`DEPLOYMENT_MODE`/`SYSTEM_GRPC_ADDR` 经 `applyEnvOverrides` 生效——Task 2 已加。**注意 `run` 原目标的 recipe 保持 `go run ./alexgo-server/cmd/main.go` 不变**。）

- [ ] **Step 4: 验证**

```bash
CGO_ENABLED=0 go build ./alexgo-server/cmd ./modules/member/cmd && echo BUILD_OK
CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go test ./... -count=1
# mono 一键启动冒烟（本机有 MySQL 时；否则报告注明跳过）：
nc -z 127.0.0.1 3306 && (timeout 20 make run > /tmp/mono.log 2>&1 &) && sleep 12 && \
  curl -sf localhost:8080/health && curl -sf localhost:8080/api/app/member/auth/login -X POST -H 'Content-Type: application/json' -d '{"mobile":"13800000000","password":"x"}' -o /dev/null -w "%{http_code}\n" || true
```

（member 登录在 mono 下**必须可达**（401/400 均为可达，502/连接拒绝为不可达）。）

- [ ] **Step 5: Commit**

```bash
git add alexgo-server/cmd/main.go modules/member/ Makefile
git commit -m "feat(deploy): mono default single-process startup, member-server standalone entry"
```

---

### Task 13: 路径分流与部署清单（vite/compose/k8s/helm 双服务）

**Files:**
- Modify: `admin-web/vite.config.ts`（member 前缀可配上游）
- Modify: `alexgo-server/Dockerfile`（双二进制）
- Modify: `deployments/docker-compose/docker-compose.yml`（member 服务 + micro 环境）
- Modify: `deployments/kubernetes/deployment.yaml`（env DEPLOYMENT_MODE=micro）、新增 `deployment-member.yaml`、`service-member.yaml`、`ingress.yaml`（新建 path 分流）
- Modify: `deployments/kubernetes/configmap.yaml`（config.yaml 加 `deployment.mode: micro`）
- Modify: `deployments/helm/alexgo-cloud/templates/`（deployment-member、service-member、ingress 两路径、configmap 加 mode）、`values*.yaml`（member 段）

**Interfaces:**
- Consumes: member 二进制 :8081、`/api/app/member`+`/api/admin/member` 前缀（spec §2.3）
- Produces: 三处（vite/compose/k8s/helm）前缀规则一致

- [ ] **Step 1: vite 分流**

`admin-web/vite.config.ts` 的 `server.proxy` 替换：

```ts
  server: {
    port: 5174,
    proxy: {
      // member 前缀可独立上游：mono（默认）两服务同进程 → 全部指 8080；
      // micro 本地联调：MEMBER_PROXY=http://localhost:8081 npm run dev
      '/api/app/member': { target: process.env.MEMBER_PROXY || 'http://localhost:8080', changeOrigin: true },
      '/api/admin/member': { target: process.env.MEMBER_PROXY || 'http://localhost:8080', changeOrigin: true },
      '/api': 'http://localhost:8080',
      '/swagger': 'http://localhost:8080',
    },
  },
```

（Vite 代理键按**最长前缀优先**匹配——member 规则在前 ✓。`process.env` 在 vite.config 里可用（Node 环境）。）

- [ ] **Step 2: Dockerfile 双二进制**

`alexgo-server/Dockerfile` 的 builder 段改为：

```dockerfile
RUN go build -o /out/alexgo-server ./alexgo-server/cmd && \
    go build -o /out/member-server ./modules/member/cmd
```

runtime 段 COPY 两个二进制：

```dockerfile
COPY --from=builder /out/alexgo-server /app/alexgo-server
COPY --from=builder /out/member-server /app/member-server
```

（ENTRYPOINT 保持 alexgo-server；member 用 command 覆盖。）

- [ ] **Step 3: docker-compose**

```yaml
  alexgo:
    build: .
    ports:
      - "8080:8080"
    depends_on:
      - mysql
    environment:
      - DB_DSN=root:password@tcp(mysql:3306)/alexgo?charset=utf8mb4&parseTime=True&loc=Local
      - DEPLOYMENT_MODE=micro
      - SYSTEM_GRPC_ADDR=alexgo:50051
    # grpc_addr 默认 :50051 已监听，micro 模式自动开启

  member:
    build: .
    command: ["/app/member-server"]
    ports:
      - "8081:8081"
    depends_on:
      - mysql
      - alexgo
    environment:
      - DB_DSN=root:password@tcp(mysql:3306)/alexgo?charset=utf8mb4&parseTime=True&loc=Local
      - HTTP_ADDR=:8081
      - DEPLOYMENT_MODE=micro
      - SYSTEM_GRPC_ADDR=alexgo:50051
```

- [ ] **Step 4: K8s**

`configmap.yaml` 内嵌 config.yaml 的 `server:` 段后加：

```yaml
    deployment:
      mode: micro
```

新增 `deployments/kubernetes/deployment-member.yaml`：

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: alexgo-member
  namespace: alexgo-cloud
spec:
  replicas: 2
  selector:
    matchLabels:
      app: alexgo-member
  template:
    metadata:
      labels:
        app: alexgo-member
    spec:
      containers:
        - name: member
          image: alexgo-cloud:latest
          command: ["/app/member-server"]
          ports:
            - containerPort: 8081
          env:
            - name: HTTP_ADDR
              value: ":8081"
            - name: DB_DSN
              valueFrom:
                secretKeyRef:
                  name: alexgo-secrets
                  key: DB_DSN
            - name: JWT_SECRET
              valueFrom:
                secretKeyRef:
                  name: alexgo-secrets
                  key: JWT_SECRET
            - name: DEPLOYMENT_MODE
              value: "micro"
            - name: SYSTEM_GRPC_ADDR
              value: "alexgo-system:50051"
          readinessProbe:
            httpGet:
              path: /health/ready
              port: 8081
            initialDelaySeconds: 10
            periodSeconds: 10
            timeoutSeconds: 5
          livenessProbe:
            httpGet:
              path: /health
              port: 8081
            initialDelaySeconds: 30
            periodSeconds: 10
            timeoutSeconds: 5
          volumeMounts:
            - name: alexgo-config
              mountPath: /app/alexgo-server/configs
              readOnly: true
      volumes:
        - name: alexgo-config
          configMap:
            name: alexgo-config
```

新增 `deployments/kubernetes/service-member.yaml`：

```yaml
apiVersion: v1
kind: Service
metadata:
  name: alexgo-member
  namespace: alexgo-cloud
spec:
  selector:
    app: alexgo-member
  ports:
    - name: http
      port: 8080
      targetPort: 8081
```

新增 `deployments/kubernetes/ingress.yaml`（path 分流——spec §2.3）：

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: alexgo-ingress
  namespace: alexgo-cloud
spec:
  ingressClassName: nginx
  rules:
    - host: alexgo.example.com
      http:
        paths:
          - path: /api/app/member
            pathType: Prefix
            backend:
              service:
                name: alexgo-member
                port:
                  number: 8080
          - path: /api/admin/member
            pathType: Prefix
            backend:
              service:
                name: alexgo-member
                port:
                  number: 8080
          - path: /
            pathType: Prefix
            backend:
              service:
                name: alexgo-system   # 现有 service 名以实际为准（kustomize service.yaml metadata.name）
                port:
                  number: 8080
```

（**先读 `deployments/kubernetes/service.yaml` 与 `ingress.yaml` 现名**——现有 ingress 若已存在则**修改**它而非新建；service 名对齐。`kustomization.yaml` resources 加三个新文件。）

`deployment.yaml`（system）容器 env 追加：

```yaml
            - name: DEPLOYMENT_MODE
              value: "micro"
```

- [ ] **Step 5: Helm**

`templates/configmap.yaml` 的 server 段后加：

```yaml
    deployment:
      mode: {{ .Values.deployment.mode | default "micro" }}
```

`values.yaml` 加：

```yaml
deployment:
  mode: micro   # 本地/单机演示可改 mono

member:
  replicaCount: 2
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
    limits:
      cpu: 400m
      memory: 512Mi
```

（values-dev/gray/prod 按需覆盖 `member.replicaCount`；**至少**在 values-prod.yaml 加 `member.replicaCount: 3`。）

`templates/deployment-member.yaml`（以现有 deployment.yaml 为模板改：name/member、image 同、command `["/app/member-server"]`、port 8081、env HTTP_ADDR/DEPLOYMENT_MODE/SYSTEM_GRPC_ADDR=`{{ include "alexgo-cloud.fullname" . }}-system:50051`——**system service 名以现有 service.yaml 的 fullname 为准**）。
`templates/service-member.yaml`（对照现有 service.yaml：selector `app: ...-member`、port 8080→targetPort 8081）。
`templates/ingress.yaml`：在现有 rules 的 paths 里**前面**插入两条 member 路径（backend service 用 member service fullname），`/` 保持原 backend。

（system deployment 的 env 也加 DEPLOYMENT_MODE=micro。）

- [ ] **Step 6: 验证**

```bash
helm lint deployments/helm/alexgo-cloud
helm template t deployments/helm/alexgo-cloud | grep -c "alexgo-member"   # 期望 > 0
kubectl kustomize deployments/kubernetes > /tmp/k8s.yaml && grep -c "alexgo-member" /tmp/k8s.yaml
grep -rn "DEPLOYMENT_MODE\|SYSTEM_GRPC_ADDR" deployments/ | wc -l
```

Expected: lint 0 失败；member 资源渲染齐全；两环境变量注入齐备

- [ ] **Step 7: Commit**

```bash
git add admin-web/vite.config.ts alexgo-server/Dockerfile deployments/
git commit -m "feat(deploy): path-prefix routing for member service across vite/compose/k8s/helm"
```

---

### Task 14: 全链路收尾（文档同步 + 终验）

**Files:**
- Modify: `README.md`（运行模式、新端点、member 服务）
- Modify: `docs/architecture.md`（§5 中间件、§7 配置、§8 测试、§9 部署对应小节）
- Modify: `docs/superpowers/specs/2026-10-08-user-system-design.md`（顶部状态改"一期已实施"）

**Interfaces:**
- Consumes: Task 1–13 全部交付
- Produces: 文档与代码一致；计划闭环

- [ ] **Step 1: README 更新**

1. 「快速开始」后追加小节：

```markdown
## 运行模式

| 模式 | 命令 | 说明 |
| --- | --- | --- |
| mono（默认） | `make run` | 单进程全部模块（system+order+member），Token 本地签发，端口 :8080 |
| micro（部署形态） | `make run-system` + `make run-member` | system-server :8080(+gRPC :50051) 与 member-server :8081，member 经 gRPC 委托签发 |

配置：`deployment.mode`（mono|micro）；member 上游地址 `SYSTEM_GRPC_ADDR`。
micro 本地联调前端分流：`MEMBER_PROXY=http://localhost:8081 npm run dev`。
```

2. 内置功能表的登录行更新：`账号密码 / Token 签发刷新注销（OAuth2 不透明令牌）/ 会员 mobile 注册登录`
3. 命令参考加 `run-system`、`run-member`

- [ ] **Step 2: architecture.md 同步**

- §5 中间件链：AuthMiddleware 描述改"token/jwt 双模式 + `{tenant}:{user}` Casbin sub + data_scope 注入"
- §7 配置：加 `auth.*`、`deployment.mode`、`system_grpc_addr` 及 `applyEnvOverrides` 新增两键
- §8 测试：包数从 7 → 实际数（`go test ./... | grep -c ok`），替身加"sqlite 隔离插件、bufconn gRPC"
- §9 部署：双服务 + 前缀分流 + Secret（已写）补 member 一行

- [ ] **Step 3: spec 状态标注**

spec 顶部 `> **状态**：待评审（Review Draft）` 改为：

```markdown
> **状态**：已评审通过；一期（块①②③⑦ + 双服务骨架 + 双运行模式）已实施（见 git log）
> 二/三期（块④⑤⑥：短信/三方/小程序）待排期
```

- [ ] **Step 4: 终验（计划收尾验证，全部任务完成后执行一次）**

```bash
CGO_ENABLED=0 go build ./... && go vet ./... && CGO_ENABLED=0 go test ./... -count=1 -race
make generate-all 2>&1 | tail -3   # 本机无 protoc 时该步预期失败——注明由 CI 覆盖；CRUD 部分单独跑：make generate-crud
make generate-crud && git status --porcelain   # 期望无变更（生成器跳过现有实体）
helm lint deployments/helm/alexgo-cloud
kubectl kustomize deployments/kubernetes > /dev/null && echo KUSTOMIZE_OK
git status --porcelain   # 期望为空
```

Expected: build/vet/test（含 -race）全绿；生成器 no-op；helm/kustomize 渲染 OK；工作区干净

- [ ] **Step 5: Commit**

```bash
git add README.md docs/
git commit -m "docs: sync README/architecture/spec for user-system phase 1 delivery"
```

---

## 计划自查（Self-Review 记录）

1. **Spec 覆盖**：块①（T1/T4/T5）块②（T2/T3/T4/T11）块③（T6/T7）块⑦（T8/T9/T10）双服务骨架（T11/T12/T13）双运行模式（T12/T13）——spec §8 一期清单全覆盖；块④⑤⑥ 明确不在本期 ✓
2. **占位符扫描**：无 TBD/TODO；两处"实现裁决"（token 租户检查去重、RebuildAllPolicies 签名、ScopeLoader 接口放 pkg/tenant、dataScopeLoader 字段去重）均为已给出最终形态的显式修正 ✓
3. **类型一致性**：`token.Issuer/Validator/Claims`（T2 定义 → T3/T4/T5/T11/T12 消费签名一致）；`AuthDeps`（T3 定义 → T10 加字段）；`RoleCreateParams`（T8 → controller）；`tenant.DataScope/ScopeLoader`（T10 定义 → 插件/中间件/模块消费）✓
4. **Review Focus 映射**：F1→T2/T4 测试；F2→T6 测试；F3→T9 测试；F4→T10 测试；F5→T11/T12 验证 ✓
5. **依赖顺序**：T1→T2→T3→T4→T5→T6→T7→T8→T9→T10→T11→T12→T13→T14 单链，无回环；T6 插件在 T10 扩展而非重写；T9 依赖 T3 的 sub 前缀与 T8 的 data_scope 字段 ✓
