# 统一登录（OAuth2 授权服务 + 三端兼容 + SSO）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新建 `modules/system/oauth2` 授权服务（授权码+PKCE / password / refresh_token / client_credentials 四种 grant、图形验证码开关、client 管理），三套既有登录端点降级为兼容层，web-antd 登录页改走授权码 SSO，users/members 表与 Casbin 鉴权零改动。

**Architecture:** 认证统一、存储分开——oauth2 模块只做「谁有权拿到 token」的守门人，凭据校验走 `UserAuthenticator`（system 实现）与 `MemberAuthenticator`（member 实现）两个 Go 接口；token 模式走 `pkg/token` 不透明令牌（中间件/Casbin 零改动），jwt 回滚模式由 system 提供 `JWTGenerator`。协议手写（不引 go-oauth2），唯一新依赖 `base64Captcha`。

**Tech Stack:** Go 1.25、gin、uber fx（`group:"modules"` / `group:"migration_sources"`）、gorm + MySQL/glebarez/sqlite、golang-migrate、casbin、go-redis v9（optional）、`github.com/mojocn/base64Captcha@v1.3.8`、vue3/vben5（apps/web-antd）。

**Spec:** `docs/superpowers/specs/2026-10-11-unified-login-design.md`（§9 勘误 D1–D15 覆盖前文冲突处）

## Global Constraints

- 中文注释与文档；代码标识符英文。
- TDD RED→GREEN 强制：每个行为先写失败测试、亲眼看它失败（因缺符号/断言不符），再写最小实现。
- 直接在 main 上提交，**不建功能分支、未经用户指示不推送**。
- 全量回归：`CGO_ENABLED=0 go test ./...` 必须全绿；接线红线 `CGO_ENABLED=0 go test ./alexgo-server/cmd/ ./modules/member/cmd/`。
- 集成测试：`DB_DSN=$(grep 'dsn:' alexgo-server/configs/config.yaml | sed 's/.*dsn: "//;s/".*//') go test -tags=integration ./modules/system/oauth2/... ./modules/system/infra/migrations/`（`modules/system/migrations/` 无 Go 文件，不在命令内——T16 的菜单种子集成测试在 `modules/system/infra/migrations/migrate_test.go`）。
- 前端：`pnpm -C apps/web-antd typecheck`。
- 不改 users/members 表结构；不动 Casbin 匹配器与中间件公开路径规则（`/api/oauth2/*` 已天然公开）。
- 唯一新依赖 `github.com/mojocn/base64Captcha@v1.3.8`；不引 go-oauth2。
- redirect_uri **精确匹配**，失效/篡改一律 400，**绝不 302**（防开放重定向）。
- 授权码 TTL 2 分钟、32B base64url、**首次 exchange 尝试即 MarkRevoked**（CAS 单次消费，无论 verifier 对错）。
- 协议端点 RFC 6749 信封 `{"error","error_description"}`；admin 端点维持 `{error}` / `{data,total}` / `{status:"ok"}` 与 400/500 惯例。
- 迁移幂等测试 = 直跑 SQL 文件双次（multiStatements=true），不只依赖 runner 双跑（golang-migrate Up 只跑 pending）。

## Review Focus

（每条挂到拥有该代码的任务测试上；执行时逐条核对。）

1. **redirect_uri 篡改 / 开放重定向**：authorize、authorize/login、exchange 三处都必须 400 且响应无 Location 头 → Task 9（`TestAuthorize_Validate_RedirectMismatch_No302`）、Task 11（`TestAuthorizeLogin_RedirectTamper_400NoLocation`）、Task 15（e2e `TestE2E_RedirectTamper_400`）。
2. **验证码开关双路径 + 挑战一次性**：enabled=false 跳过校验、enabled=true 错码拒绝、同 uuid 二次使用拒绝 → Task 3（`TestCaptcha_DisabledSkip / TestCaptcha_VerifyPaths / TestCaptcha_SingleUse`）、Task 8（`TestLogin_CaptchaTogglePaths`）、Task 11（`TestToken_CaptchaInvalid_400`）。
3. **跨账号类型冒用**：system client 换 member token → 400 unauthorized_client；member/client token 打 `/api/admin/*` → 403；公开 client 走 client_credentials → 401 → Task 7（`TestClientService_Authenticate_PublicSkipBcrypt`）、Task 10（`TestExchange_Password_AccountTypeNotAllowed`）、Task 15（e2e 两例 403 + 401）。
4. **授权码过期 / 重放 / verifier 错且首错即消费**：过期 400 invalid_grant；同 code 第二次 400；verifier 错的**第一次**尝试就使 code 失效 → Task 9（`TestExchangeCode_Expired / TestExchangeCode_RevokedAfterFirstAttempt`）、Task 15（e2e 重放 400）。
5. **三端兼容响应结构回归 + jwt 回滚模式**：vben `{code,data,error,message}`、app 三元组（非 captcha 错保持 500 怪癖）、member 三元组+503 哨兵；mode=jwt 时 system 登录签 JWT、Refresh 拒绝 → Task 12（既有契约测试改造后全绿 + `TestAppLogin_Quirks`、`TestLogin_JwtRollbackMode` 迁移版）。

---

## 设计裁决（spec §9 展开，计划按此执行）

| # | 裁决 |
|---|---|
| D1 | 协议层手写；唯一新依赖 `github.com/mojocn/base64Captcha@v1.3.8`。 |
| D2 | token 模式（默认）走 `pkg/token` 不透明令牌；jwt 模式下 system 登录用 system 提供的 `JWTGenerator`（wrap `auth.GenerateToken`），无 refresh。 |
| D3 | 全部新端点挂 `/api/oauth2/*`（Module 只拿 /api 组；中间件公开规则 `!strings.HasPrefix(path,"/api/admin/")` 已放行）。 |
| D4 | 只建两表：`oauth2_client`（无 tenant_id/deleted，硬删）、`oauth2_authorize_code`（租户列 Go 字段 `OwnerTenantID`；含 `username` 列供 jwt 模式授权码流）。签发台账复用 `system_oauth2_access_token`。 |
| D5 | TTL 沿用 `auth.access_expire_hour=2` / `auth.refresh_expire_day=7`；config 只加 `auth.captcha.enabled`（默认 false）与 `auth.login_page`（默认 `/auth/login`）。 |
| D6 | 验证码存 `pkg/redis`（可用则 `captcha:{uuid}` TTL 2 分钟，否则内存 store）；deps 中 Redis 标 `optional:"true"`（member 进程无 redis 也能装配）。 |
| D7 | `oauth2.FxModule` 硬编码进 `alexgo-server/cmd/main.go` 与 `modules/member/cmd/main.go`。 |
| D8 | 不加逐端点限流，复用全局 RateLimitAndBreaker。 |
| D9 | 新增 `token.UserTypeClient=3`；`Issue` 允许 `UserID==0` 当且仅当 type 3 且 ClientID 非空；`Validate` 对 type 3 以 ClientID 为 username（无库查询）；机机 token 无 refresh，jwt 模式也签不透明。 |
| D10 | 种子 client：`alexgo-admin`（secret 空=公开，grants `authorization_code,password,refresh_token`，account_types `system`，redirect `http://localhost:5173/callback\nhttp://localhost:8080/callback`）与 `alexgo-app`（机密，grants `password,refresh_token`，account_types `member`）。库中 secret 空 → 跳过 bcrypt。 |
| D11 | app 登录真实路径 `POST /api/app/system/auth/login`（spec §1 笔误勘正）。 |
| D12 | 验证码字段 `captcha_uuid` + `captcha_code`。 |
| D13 | 审计移入统一登录服务（每登录恰一次，msg 前缀 `"oauth2: "`）；兼容层 controller 删除内联审计。 |
| D14 | refresh 不做 client 绑定：只校验 client 存在+活跃+grants 含 `refresh_token`。 |
| D15 | 前端 SSO：登录页造 state/verifier/S256 → sessionStorage → `fetch POST /api/oauth2/authorize/login`（redirect:'follow'，从 `resp.url` 提取 code 比对 state）→ push `/callback` → 校验 session → `POST /api/oauth2/token` → completeLogin。固定 client `alexgo-admin`、redirect `origin+/callback`。 |

**统一登录语义（Task 8 冻结）**：`Login` 顺序 = 账号类型校验（`system|member` 否则 `ErrAccountTypeUnsupported`，先于 captcha/审计）→ captcha（enabled 时 Verify，先于查库，失败审计 false 一次）→ 按类型路由 UserAuth/MemberAuth（nil → `ErrAuthenticatorMissing`）→ `IssueFor` 成功才审计 true、失败审计 false——**每恰一次审计**。`IssueFor`：Issuer==nil → `ErrIssuance`；`AccountType==client` → 恒 Issue{UserID:0, UserType:3, TenantID:0, ClientID} 且 RefreshToken=""（jwt 模式也不例外）；system+mode==jwt → JWTGen（nil → wrap ErrIssuance）返回 `{AccessToken}` refresh 空 expires 0；否则按类型 opaque Issue；任何 Issue 错 → wrap `ErrIssuance`。

**TokenService 错误映射（loginProtoErr）**：`ErrInvalidCredentials`→400 `invalid_grant`「账号或密码错误」；`ErrUserDisabled`→400 `invalid_grant`「账号已停用」；`ErrCaptchaInvalid`→400 `captcha_invalid`；`ErrIssuance|ErrAuthenticatorMissing`→500 `server_error`「签发服务不可用」；`ErrAccountTypeUnsupported`→400 `invalid_request`；default→500 `server_error`。

---

### Task 1: config 扩展（auth.captcha.enabled + auth.login_page）

**Files:**
- Modify: `pkg/config/config.go`（Auth struct，约 99-103 行）
- Modify: `pkg/config/loader.go`（defaults，约 55-57 行 `auth.mode` 之后）
- Test: `pkg/config/loader_test.go`

**Interfaces:**
- Consumes: 既有 `config.Config`、`LoadGlobalConfig()`、`chdirRepoRoot(t)` 测试助手。
- Produces: `cfg.Auth.Captcha.Enabled bool`、`cfg.Auth.LoginPage string`（Task 3 captcha、Task 11 protocol 消费）。

- [ ] **Step 1: 写失败测试**

在 `pkg/config/loader_test.go` 追加（该文件是 `package config_test`，已 import `testing` 与 chdirRepoRoot）：

```go
func TestLoad_AuthCaptchaAndLoginPageDefaults(t *testing.T) {
	chdirRepoRoot(t)
	t.Setenv("DB_DSN", "")
	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Auth.Captcha.Enabled {
		t.Errorf("auth.captcha.enabled 默认应为 false, got true")
	}
	if cfg.Auth.LoginPage != "/auth/login" {
		t.Errorf("auth.login_page 默认 = %q, want /auth/login", cfg.Auth.LoginPage)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `CGO_ENABLED=0 go test ./pkg/config/ -run TestLoad_AuthCaptcha -v`
Expected: FAIL —— `cfg.Auth.Captcha undefined`（编译错误，缺字段即红）。

- [ ] **Step 3: 最小实现**

`pkg/config/config.go` Auth struct（保留既有 Mode/AccessExpireHour/RefreshExpireDay）追加：

```go
	// Captcha 图形验证码开关（spec §9 D5）：false 时登录跳过校验。
	Captcha struct {
		Enabled bool `mapstructure:"enabled"`
	} `mapstructure:"captcha"`
	// LoginPage authorize 未认证时 302 的登录页路径（vben LOGIN_PATH 实际值）。
	LoginPage string `mapstructure:"login_page"`
```

`pkg/config/loader.go` defaults（`v.SetDefault("auth.refresh_expire_day", 7)` 之后）追加：

```go
	v.SetDefault("auth.captcha.enabled", false)
	v.SetDefault("auth.login_page", "/auth/login")
```

- [ ] **Step 4: 运行确认通过**

Run: `CGO_ENABLED=0 go test ./pkg/config/ -v`
Expected: PASS（新增测试 + 既有 loader 测试全绿）。

- [ ] **Step 5: Commit**

```bash
git add pkg/config/config.go pkg/config/loader.go pkg/config/loader_test.go
git commit -m "feat(config): auth.captcha.enabled 开关与 auth.login_page"
```

---

### Task 2: oauth2 模块骨架 + 两表迁移 + 种子 client + 双 main 装配 + 幂等

**Files:**
- Create: `modules/system/oauth2/module.go`
- Create: `modules/system/oauth2/migration_source.go`
- Create: `modules/system/oauth2/migrations/20261011000001_oauth2_tables.up.sql`
- Create: `modules/system/oauth2/migrations/20261011000002_oauth2_seed_clients.up.sql`
- Create: `modules/system/oauth2/model/client.go`、`modules/system/oauth2/model/authorize_code.go`
- Modify: `alexgo-server/cmd/main.go`（opts 列表约 174-176 行，插在 `infra.FxModule` 之前）
- Modify: `modules/member/cmd/main.go`（约 119 行 `member.FxModule,` 之前插 `oauth2.FxModule,`）
- Test: `modules/system/oauth2/migrations/migrate_test.go`（`//go:build integration`）

**Interfaces:**
- Consumes: `pkg/migrate.Source` 接口（`GetMigrationPath() string`、`GetModuleName() string`）；infra 骨架 `modules/system/infra/migration_source.go` 模板；fx `group:"modules"` / `group:"migration_sources"` 范式。
- Produces: `oauth2.FxModule`（fx.Option，两 main 消费）；`oauth2.NewMigrationSource()`；表 `oauth2_client`、`oauth2_authorize_code`；种子 client 两行；`model.Oauth2Client`、`model.Oauth2AuthorizeCode`（Task 7/9 消费）。

- [ ] **Step 1: 写失败测试**

`modules/system/oauth2/migrations/migrate_test.go`：

```go
//go:build integration

package migrations

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"alexGo-cloud/pkg/migrate"
)

func chdirRepoRoot(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
		return
	}
	root := filepath.Clean(filepath.Join(wd, "..", "..", ".."))
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
}

// runOAuth2 用 DB_DSN 构造 Runner 只跑 oauth2 迁移源。
func runOAuth2(t *testing.T) (*gorm.DB, *migrate.Runner) {
	t.Helper()
	chdirRepoRoot(t)
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		t.Skip("DB_DSN not set; skip integration test")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("mysql unreachable: %v", err)
	}
	cfg := &config.Config{}
	cfg.Migrate.Auto = true
	cfg.Database.DSN = dsn
	runner := migrate.NewRunner(db, cfg, []migrate.Source{oauth2.NewMigrationSource()})
	return db, runner
}
```

注意 import 需补 `"alexGo-cloud/modules/system/oauth2"` 与 `"alexGo-cloud/pkg/config"`（上面骨架省略，实现时补齐）。测试用例：

```go
// 双表建立 + 双跑稳定；oauth2_client 无 tenant_id/deleted（D4 硬删）。
func TestOAuth2Migrations_UpTwiceStable(t *testing.T) {
	db, runner := runOAuth2(t)
	if err := runner.Run(); err != nil {
		t.Fatalf("run #1: %v", err)
	}
	for _, tbl := range []string{"oauth2_client", "oauth2_authorize_code"} {
		if !db.Migrator().HasTable(tbl) {
			t.Fatalf("表 %s 未建立", tbl)
		}
	}
	if db.Migrator().HasColumn("oauth2_client", "tenant_id") || db.Migrator().HasColumn("oauth2_client", "deleted") {
		t.Fatal("oauth2_client 不得有 tenant_id/deleted（D4）")
	}
	var c1, a1 int64
	db.Table("oauth2_client").Count(&c1)
	db.Table("oauth2_authorize_code").Count(&a1)
	if err := runner.Run(); err != nil {
		t.Fatalf("run #2: %v", err)
	}
	var c2, a2 int64
	db.Table("oauth2_client").Count(&c2)
	db.Table("oauth2_authorize_code").Count(&a2)
	if c1 != c2 || a1 != a2 {
		t.Errorf("双跑行数变化: client %d→%d, code %d→%d", c1, c2, a1, a2)
	}
	if c1 != 2 {
		t.Errorf("种子 client = %d, want 2（alexgo-admin + alexgo-app）", c1)
	}
}

// 种子 SQL 直跑双次幂等（绕开 golang-migrate 只跑 pending 的真空坑）。
const seedClientsPath = "modules/system/oauth2/migrations/20261011000002_oauth2_seed_clients.up.sql"

func openMultiStmtDB(t *testing.T) *sql.DB {
	t.Helper()
	chdirRepoRoot(t)
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		t.Skip("DB_DSN not set; skip integration test")
	}
	if strings.Contains(dsn, "?") {
		dsn += "&multiStatements=true"
	} else {
		dsn += "?multiStatements=true"
	}
	raw, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := raw.Ping(); err != nil {
		t.Skipf("mysql unreachable: %v", err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	return raw
}

func TestOAuth2SeedClients_SQLFileExecutedTwiceStable(t *testing.T) {
	raw := openMultiStmtDB(t)
	_, runner := runOAuth2(t)
	if err := runner.Run(); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	src, err := os.ReadFile(seedClientsPath)
	if err != nil {
		t.Fatal(err)
	}
	count := func() int64 {
		var n int64
		if err := raw.QueryRow(`SELECT COUNT(*) FROM oauth2_client WHERE client_id IN ('alexgo-admin','alexgo-app')`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	before := count()
	for i := 1; i <= 2; i++ {
		if _, err := raw.Exec(string(src)); err != nil {
			t.Fatalf("直跑 SQL #%d: %v", i, err)
		}
	}
	if after := count(); after != before || after != 2 {
		t.Errorf("种子行数 before=%d after=%d, want 恒 2", before, after)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `DB_DSN=$(grep 'dsn:' alexgo-server/configs/config.yaml | sed 's/.*dsn: "//;s/".*//') go test -tags=integration ./modules/system/oauth2/... -v`
Expected: FAIL —— 包不存在 / `oauth2.NewMigrationSource` undefined（模块尚未创建）。

- [ ] **Step 3: 最小实现**

`modules/system/oauth2/migration_source.go`（照抄 infra 模板）：

```go
package oauth2

// NewMigrationSource 注册 oauth2 迁移源（runner 按 GetModuleName 建 schema_migrations_oauth2 记录表）。
func NewMigrationSource() *MigrationSource { return &MigrationSource{} }

type MigrationSource struct{}

func (s *MigrationSource) GetMigrationPath() string { return "modules/system/oauth2/migrations" }
func (s *MigrationSource) GetModuleName() string     { return "oauth2" }
```

`modules/system/oauth2/model/client.go`：

```go
package model

import "time"

// Oauth2Client 接入方注册（D4：无 tenant_id/deleted，硬删除）。
type Oauth2Client struct {
	ID            uint64    `gorm:"primaryKey" json:"id"`
	ClientID      string    `gorm:"size:64;uniqueIndex" json:"client_id"`
	ClientSecret  string    `gorm:"size:128" json:"-"`
	ClientName    string    `gorm:"size:128" json:"client_name"`
	Website       string    `gorm:"size:255" json:"website"`
	GrantTypes    string    `gorm:"size:255" json:"grant_types"`    // 逗号分隔
	RedirectURIs  string    `gorm:"type:text" json:"redirect_uris"` // 换行分隔，精确匹配
	AccountTypes  string    `gorm:"size:64" json:"account_types"`   // 逗号分隔 system,member,client
	Scopes        string    `gorm:"size:64" json:"scopes"`
	Activity      int       `json:"activity"` // 1 启用 0 停用
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (Oauth2Client) TableName() string { return "oauth2_client" }

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// AllowsGrant grant 白名单精确匹配。
func (c *Oauth2Client) AllowsGrant(g string) bool {
	for _, x := range splitCSV(c.GrantTypes) {
		if x == g {
			return true
		}
	}
	return false
}

// AllowsAccountType 账号类型白名单（D10 防跨类型冒用）。
func (c *Oauth2Client) AllowsAccountType(t string) bool {
	for _, x := range splitCSV(c.AccountTypes) {
		if x == t {
			return true
		}
	}
	return false
}

// AllowsRedirect redirect_uri 精确匹配（多条换行分隔）。
func (c *Oauth2Client) AllowsRedirect(u string) bool {
	for _, line := range strings.Split(c.RedirectURIs, "\n") {
		if strings.TrimSpace(line) == u && u != "" {
			return true
		}
	}
	return false
}
```

（需补 `import "strings"`。）

`modules/system/oauth2/model/authorize_code.go`：

```go
package model

import "time"

// Oauth2AuthorizeCode 授权码（D4：租户列 Go 名 OwnerTenantID 规避租户插件注入；
// username 列供 jwt 模式授权码流回填 claims）。
type Oauth2AuthorizeCode struct {
	ID                 uint64    `gorm:"primaryKey" json:"id"`
	Code               string    `gorm:"size:64;uniqueIndex" json:"code"`
	ClientID           string    `gorm:"size:64;index" json:"client_id"`
	UserID             uint64    `json:"user_id"`
	Username           string    `gorm:"size:64" json:"username"`
	AccountType        string    `gorm:"size:16" json:"account_type"`
	OwnerTenantID      uint64    `gorm:"column:tenant_id" json:"owner_tenant_id"`
	RedirectURI        string    `gorm:"type:text" json:"redirect_uri"`
	Scopes             string    `gorm:"size:64" json:"scopes"`
	CodeChallenge      string    `gorm:"size:128" json:"code_challenge"`
	CodeChallengeMethod string   `gorm:"size:16" json:"code_challenge_method"`
	ExpiresAt          time.Time `json:"expires_at"`
	Revoked            bool      `json:"revoked"`
	CreatedAt          time.Time `json:"created_at"`
}

func (Oauth2AuthorizeCode) TableName() string { return "oauth2_authorize_code" }
```

`modules/system/oauth2/migrations/20261011000001_oauth2_tables.up.sql`：

```sql
-- oauth2 两表（spec §9 D4）：client 注册 + 授权码。
-- oauth2_client 无 tenant_id/deleted（硬删除，租户插件因模型缺字段跳过）。
CREATE TABLE IF NOT EXISTS `oauth2_client` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `client_id` VARCHAR(64) NOT NULL,
  `client_secret` VARCHAR(128) NOT NULL DEFAULT '',
  `client_name` VARCHAR(128) NOT NULL DEFAULT '',
  `website` VARCHAR(255) NOT NULL DEFAULT '',
  `grant_types` VARCHAR(255) NOT NULL DEFAULT '',
  `redirect_uris` TEXT NOT NULL,
  `account_types` VARCHAR(64) NOT NULL DEFAULT '',
  `scopes` VARCHAR(64) NOT NULL DEFAULT 'all',
  `activity` INT NOT NULL DEFAULT 1,
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_oauth2_client_id` (`client_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 授权码表：tenant 列由 Go 字段 OwnerTenantID 映射（列名 tenant_id）。
CREATE TABLE IF NOT EXISTS `oauth2_authorize_code` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `code` VARCHAR(64) NOT NULL,
  `client_id` VARCHAR(64) NOT NULL,
  `user_id` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `username` VARCHAR(64) NOT NULL DEFAULT '',
  `account_type` VARCHAR(16) NOT NULL DEFAULT '',
  `tenant_id` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `redirect_uri` TEXT NOT NULL,
  `scopes` VARCHAR(64) NOT NULL DEFAULT 'all',
  `code_challenge` VARCHAR(128) NOT NULL DEFAULT '',
  `code_challenge_method` VARCHAR(16) NOT NULL DEFAULT '',
  `expires_at` DATETIME(3) NOT NULL,
  `revoked` TINYINT(1) NOT NULL DEFAULT 0,
  `created_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_oauth2_code` (`code`),
  KEY `idx_oauth2_client_id` (`client_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

`modules/system/oauth2/migrations/20261011000002_oauth2_seed_clients.up.sql`（D10；bcrypt 哈希为字面量，`alexgo-admin` secret 空=公开）：

```sql
-- 种子 client（D10）：alexgo-admin 公开（secret 空，授权码流靠 PKCE）、alexgo-app 机密。
-- 幂等：NOT EXISTS 守卫，可重复跑。
INSERT INTO `oauth2_client`
  (`client_id`,`client_secret`,`client_name`,`website`,`grant_types`,`redirect_uris`,`account_types`,`scopes`,`activity`,`created_at`,`updated_at`)
SELECT 'alexgo-admin', '', 'alexGo 管理后台', '',
       'authorization_code,password,refresh_token',
       'http://localhost:5173/callback\nhttp://localhost:8080/callback',
       'system', 'all', 1, NOW(), NOW()
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `oauth2_client` WHERE `client_id`='alexgo-admin');

INSERT INTO `oauth2_client`
  (`client_id`,`client_secret`,`client_name`,`website`,`grant_types`,`redirect_uris`,`account_types`,`scopes`,`activity`,`created_at`,`updated_at`)
SELECT 'alexgo-app', '$2a$10$zIbcMCyAYwodY.mYbdDSNuzeZpbXjTsLxQ9ocIarutvqnwVJCwgd2', 'alexGo app 端', '',
       'password,refresh_token', '', 'member', 'all', 1, NOW(), NOW()
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `oauth2_client` WHERE `client_id`='alexgo-app');
```

`modules/system/oauth2/module.go`（最小骨架：只挂迁移源，无路由；Task 3 起逐步加 Provide。迁移源**从一开始**就按 infra 模式注解进 `group:"migration_sources"`——不注解则生产启动的 `migrate.NewRunner` 收不到本模块，oauth2 迁移静默不跑）：

```go
package oauth2

import (
	"go.uber.org/fx"

	"alexGo-cloud/pkg/migrate"
)

// FxModule oauth2 模块注册（D7：硬编码进两个 main.go）。
// 照抄 modules/system/infra/module.go：迁移源注解进 group:"migration_sources"（ResultTags）。
var FxModule = fx.Module("oauth2",
	fx.Provide(
		fx.Annotate(
			NewMigrationSource,
			fx.As(new(migrate.Source)),
			fx.ResultTags(`group:"migration_sources"`),
		),
	),
)
```

（fx import 路径以 `alexgo-server/go.mod` 中既有写法为准——全仓库搜 `go.uber.org/fx` 既有 import 行照抄。）

`alexgo-server/cmd/main.go` opts 列表（原 `infra.FxModule, system.FxModule, order.FxModule`）改为在 `infra.FxModule` 前插入：

```go
	oauth2.FxModule,
	infra.FxModule,
```

（补 import `"alexGo-cloud/modules/system/oauth2"`。）`modules/member/cmd/main.go` 在 `member.FxModule,` 行之前插 `oauth2.FxModule,` 同 import。

- [ ] **Step 4: 运行确认通过**

Run: `CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go test ./alexgo-server/cmd/ ./modules/member/cmd/`
Expected: PASS（两 main 装配可编译可测）。

Run: `DB_DSN=$(grep 'dsn:' alexgo-server/configs/config.yaml | sed 's/.*dsn: "//;s/".*//') go test -tags=integration ./modules/system/oauth2/... -v`
Expected: PASS（两集成测试绿）。若 `runner.Run` 首跑未建表，检查 runner 吞错路径与 chdirRepoRoot。

- [ ] **Step 5: Commit**

```bash
git add modules/system/oauth2/ alexgo-server/cmd/main.go modules/member/cmd/main.go
git commit -m "feat(oauth2): 模块骨架、两表迁移与种子 client、双 main 装配"
```

---

### Task 3: CaptchaService + GET /api/oauth2/captcha + ProtocolController 初版

**Files:**
- Create: `modules/system/oauth2/service/captcha.go`（含 `ErrCaptchaInvalid`）
- Create: `modules/system/oauth2/service/captcha_test.go`
- Create: `modules/system/oauth2/controller/protocol.go`（初版 ctor；Task 9 换成 `ProtocolDeps`(fx.In) 同名 ctor，此后 Task 11/13 只往结构体加字段、ctor 签名不再变）
- Create: `modules/system/oauth2/controller/protocol_test.go`
- Modify: `modules/system/oauth2/module.go`（Provide + 路由注册）

**Interfaces:**
- Consumes: `cfg.Auth.Captcha.Enabled`（Task 1）、`fx.In` + `optional:"true"` 范式、`pkg/redis` 的 `NewClient` 返回 `(*goredis.Client, error)`，未启用时 `(nil, nil)`。
- Produces:
```go
type CaptchaChallenge struct {
	UUID    string `json:"uuid"`
	Image   string `json:"image"` // 含 "data:image/png;base64," 前缀
	Enabled bool   `json:"enabled"`
}
type CaptchaDeps struct {
	fx.In
	Cfg   *config.Config
	Redis *goredis.Client `optional:"true"`
}
func NewCaptchaService(deps CaptchaDeps) *CaptchaService
func (s *CaptchaService) Enabled() bool
func (s *CaptchaService) NewChallenge(ctx context.Context) (*CaptchaChallenge, error)
func (s *CaptchaService) Verify(ctx context.Context, uuid, code string) error
// ErrCaptchaInvalid 在此文件
```
（Task 8/10/11 消费 `Verify`、`ErrCaptchaInvalid`；Task 11 消费 `NewChallenge`。）

- [ ] **Step 1: 写失败测试**

`modules/system/oauth2/service/captcha_test.go`：

```go
package service

import (
	"context"
	"errors"
	"testing"

	"alexGo-cloud/pkg/config"
)

func newTestCaptcha(enabled bool) *CaptchaService {
	cfg := &config.Config{}
	cfg.Auth.Captcha.Enabled = enabled
	return NewCaptchaService(CaptchaDeps{Cfg: cfg})
}

// 开关关：Verify 恒过（无论 uuid/code 是否存在）。
func TestCaptcha_DisabledSkip(t *testing.T) {
	s := newTestCaptcha(false)
	if s.Enabled() {
		t.Fatal("Enabled() 应为 false")
	}
	if err := s.Verify(context.Background(), "no-such", "1234"); err != nil {
		t.Fatalf("disabled 时 Verify 应 nil, got %v", err)
	}
}

// 开关开：先取挑战，再用正确答案通过、错误答案拒。
func TestCaptcha_VerifyPaths(t *testing.T) {
	s := newTestCaptcha(true)
	ch, err := s.NewChallenge(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ch.UUID == "" || ch.Image == "" {
		t.Fatalf("挑战缺字段: %+v", ch)
	}
	if ch.Enabled != true {
		t.Error("Enabled 应 true")
	}
	// 错码 → ErrCaptchaInvalid
	if err := s.Verify(context.Background(), ch.UUID, "0000-wrong"); !errors.Is(err, ErrCaptchaInvalid) {
		t.Fatalf("错码应 ErrCaptchaInvalid, got %v", err)
	}
	// 正确答案：需要拿到 answer——测试通过内部 store 校验不可行，
	// 改为断言「同 uuid 错码二次仍拒」+「未知 uuid 拒」。
	if err := s.Verify(context.Background(), "unknown-uuid", "1234"); !errors.Is(err, ErrCaptchaInvalid) {
		t.Fatalf("未知 uuid 应 ErrCaptchaInvalid, got %v", err)
	}
}

// 一次性：正确验证（clear=true）后同 uuid 再用即拒。
func TestCaptcha_SingleUse(t *testing.T) {
	s := newTestCaptcha(true)
	ch, err := s.NewChallenge(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// base64Captcha 的 Verify(id, answer, clear=true) 消费条目；
	// 答案未知时无法直接走通正确路径——用 Store 层面断言：
	// 连续两次 Verify 错码（clear=false 保留）后，第三次仍能查到；
	// 而 Verify(..., clear=true) 命中即删除。这里以「未知/已消费 uuid 拒」为准。
	_ = ch
	if err := s.Verify(context.Background(), "", ""); !errors.Is(err, ErrCaptchaInvalid) {
		t.Fatalf("空 uuid 应 ErrCaptchaInvalid, got %v", err)
	}
}
```

> 注：base64Captcha 不回吐答案给调用方，「正确答案通过」路径由实现内部保证（`Verify` 直通库）。为让正确路径可断言，实现里给 `CaptchaService` 保留测试钩子——**更好的做法**：测试直接用 `s.driver`/`s.store` 不可行（未导出）。裁决：实现暴露 `verifyAnswer` 无；改用 **注入 answer 的白盒**——`NewChallenge` 返回结构不变，测试文件内用包私有字段访问（同包测试可读未导出字段）：实现中把最后一次生成的 answer 存进 `s.lastAnswers[uuid]` **仅测试构建**不行。最终裁决（写进实现）：`CaptchaService` 不藏答案；测试通过 `NewChallenge` 后调用包内 `s.store.Get(uuid, false)` 拿到缓存值直接比对——`base64Captcha.Store.Get(id, clear)` 返回存的字符串即答案。同包测试可调用未导出字段 `s.store`。故 `TestCaptcha_VerifyPaths` 中正确答案段改为：

```go
	// 同包白盒：store 里存的即答案（Get(id, clear=false) 不消费）。
	answer := s.store.Get(ch.UUID, false)
	if answer == "" {
		t.Fatal("store 未存答案")
	}
	if err := s.Verify(context.Background(), ch.UUID, answer); err != nil {
		t.Fatalf("正确答案应通过, got %v", err)
	}
	// 一次性：正确验证已 clear，同 uuid 再验拒。
	if err := s.Verify(context.Background(), ch.UUID, answer); !errors.Is(err, ErrCaptchaInvalid) {
		t.Fatalf("已消费 uuid 应 ErrCaptchaInvalid, got %v", err)
	}
```

`modules/system/oauth2/controller/protocol_test.go`（HTTP 层 RED——GET /oauth2/captcha 端点本身的第一条测试；Task 9 Step 5 整文件重写时把它迁到 `newProtoRouter` 新夹具）：

```go
package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
	"alexGo-cloud/pkg/config"
)

// TestCaptchaImage_OK 无论开关 GET /oauth2/captcha 都 200 + uuid/image/enabled（spec §3.4）。
func TestCaptchaImage_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	cfg.Auth.Captcha.Enabled = true
	p := NewProtocolController(oauth2svc.NewCaptchaService(oauth2svc.CaptchaDeps{Cfg: cfg}))
	r := gin.New()
	p.Register(r.Group("/api"))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/oauth2/captcha", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var ch struct {
		UUID    string `json:"uuid"`
		Image   string `json:"image"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &ch); err != nil {
		t.Fatalf("响应非 JSON: %v (%s)", err, w.Body.String())
	}
	if ch.UUID == "" || ch.Image == "" {
		t.Fatalf("响应缺 uuid/image 字段: %s", w.Body.String())
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `CGO_ENABLED=0 go test ./modules/system/oauth2/service/ ./modules/system/oauth2/controller/ -v`
Expected: FAIL（编译错误）—— service 侧 `undefined: CaptchaService / NewCaptchaService / ErrCaptchaInvalid`；controller 侧 `undefined: NewProtocolController`（及 `oauth2svc.NewCaptchaService`；controller 包此期只有测试文件，`go test` 正常报未定义符号）。

- [ ] **Step 3: 最小实现**

`modules/system/oauth2/service/captcha.go`：

```go
package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/mojocn/base64Captcha"
	goredis "github.com/redis/go-redis/v9"

	"alexGo-cloud/pkg/config"
)

// ErrCaptchaInvalid 验证码缺失/错误/过期/已消费（Task 8/10 映射 400 captcha_invalid）。
var ErrCaptchaInvalid = errors.New("captcha invalid")

// CaptchaChallenge 一次图形验证码（Image 含 data URI 前缀，前端直接作 img src）。
type CaptchaChallenge struct {
	UUID    string `json:"uuid"`
	Image   string `json:"image"`
	Enabled bool   `json:"enabled"`
}

// CaptchaDeps Redis optional：member 进程无 redis 也能装配（D6）。
type CaptchaDeps struct {
	fx.In
	Cfg   *config.Config
	Redis *goredis.Client `optional:"true"`
}
```

> `fx.In` 需要 import `go.uber.org/fx`。继续：

```go
type CaptchaService struct {
	enabled bool
	driver  base64Captcha.Driver
	store   base64Captcha.Store
}

// NewCaptchaService redis 可用则挑战值存 redis（key captcha:{uuid} TTL 2 分钟），
// 否则内存 store（D6）。
func NewCaptchaService(deps CaptchaDeps) *CaptchaService {
	enabled := deps.Cfg != nil && deps.Cfg.Auth.Captcha.Enabled
	driver := base64Captcha.NewDriverDigit(80, 240, 5, 0.7, 80)
	var store base64Captcha.Store
	if deps.Redis != nil {
		store = newRedisStore(deps.Redis, 2*time.Minute)
	} else {
		store = base64Captcha.NewMemoryStore(1024, 2*time.Minute)
	}
	return &CaptchaService{enabled: enabled, driver: driver, store: store}
}

func (s *CaptchaService) Enabled() bool { return s.enabled }

// NewChallenge 无论开关都生成图像（开关只影响登录时是否校验，spec §3.4）。
func (s *CaptchaService) NewChallenge(ctx context.Context) (*CaptchaChallenge, error) {
	c := base64Captcha.NewCaptcha(s.driver, s.store)
	id, b64s, _, err := c.Generate()
	if err != nil {
		return nil, fmt.Errorf("captcha generate: %w", err)
	}
	return &CaptchaChallenge{
		UUID:    id,
		Image:   "data:image/png;base64," + b64s,
		Enabled: s.enabled,
	}, nil
}

// Verify disabled 恒过；错/未知/已消费 → ErrCaptchaInvalid（clear=true 一次性）。
func (s *CaptchaService) Verify(ctx context.Context, uuid, code string) error {
	if !s.enabled {
		return nil
	}
	if uuid == "" || code == "" {
		return ErrCaptchaInvalid
	}
	c := base64Captcha.NewCaptcha(s.driver, s.store)
	if !c.Verify(uuid, code, true) {
		return ErrCaptchaInvalid
	}
	return nil
}

// redisStore 适配 base64Captcha.Store，值即答案字符串。
type redisStore struct {
	rdb *goredis.Client
	ttl time.Duration
	mu  sync.Mutex
}

func newRedisStore(rdb *goredis.Client, ttl time.Duration) base64Captcha.Store {
	return &redisStore{rdb: rdb, ttl: ttl}
}

func (s *redisStore) Set(id, value string) error {
	ctx := context.Background()
	return s.rdb.Set(ctx, "captcha:"+id, value, s.ttl).Err()
}

func (s *redisStore) Get(id, clear bool) string {
	ctx := context.Background()
	key := "captcha:" + id
	v, err := s.rdb.Get(ctx, key).Result()
	if err != nil {
		return ""
	}
	if clear {
		s.rdb.Del(ctx, key)
	}
	return v
}

func (s *redisStore) Verify(id, answer string, clear bool) bool {
	v := s.Get(id, clear)
	return v != "" && v == answer
}
```

`modules/system/oauth2/controller/protocol.go` 初版（只做 captcha 端点；Task 11 换签名扩展同名 ctor）：

```go
package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
)

// ProtocolController 协议端点（RFC 6749 信封）。
type ProtocolController struct {
	captcha *oauth2svc.CaptchaService
}

// NewProtocolController 初版只接 captcha；Task 11 扩展为 ProtocolDeps。
func NewProtocolController(captcha *oauth2svc.CaptchaService) *ProtocolController {
	return &ProtocolController{captcha: captcha}
}

// Register 挂 /oauth2/*（module 传入的分组已带 /api 前缀）。
func (p *ProtocolController) Register(r gin.IRouter) {
	r.GET("/oauth2/captcha", p.captchaImage)
}

func (p *ProtocolController) captchaImage(c *gin.Context) {
	ch, err := p.captcha.NewChallenge(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "captcha generate failed", "error_description": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ch)
}
```

`modules/system/oauth2/module.go` 扩展（整文件如下：保留 Task 2 的迁移源注解，新增控制器聚合与路由挂载）：

```go
package oauth2

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"alexGo-cloud/alexgo-server/server"
	"alexGo-cloud/modules/system/oauth2/controller"
	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
	"alexGo-cloud/pkg/migrate"
)

// FxModule oauth2 模块装配（D7：硬编码进两个 main.go）。照抄 modules/system/infra/module.go：
// 迁移源注解进 group:"migration_sources"，NewModule 注解进 group:"modules"（均 ResultTags）。
var FxModule = fx.Module("oauth2",
	fx.Provide(
		fx.Annotate(
			NewMigrationSource,
			fx.As(new(migrate.Source)),
			fx.ResultTags(`group:"migration_sources"`),
		),
		oauth2svc.NewCaptchaService,
		controller.NewProtocolController,
		fx.Annotate(
			NewModule,
			fx.As(new(server.Module)),
			fx.ResultTags(`group:"modules"`),
		),
	),
)

type oauth2Module struct {
	proto *controller.ProtocolController
}

// NewModule 聚合控制器，产出 server.Module（挂进 group:"modules"）。
func NewModule(proto *controller.ProtocolController) server.Module {
	return &oauth2Module{proto: proto}
}

// RegisterRoutes 入参是 /api group（server/http.go 统一挂载，infra 同款），
// 故路径写相对前缀 /oauth2/*；内部不再 r.Group("/api")。
func (m *oauth2Module) RegisterRoutes(r *gin.RouterGroup) {
	m.proto.Register(r)
}
```

> 权威模板是 `modules/system/infra/module.go`，**逐行对齐、不新发明**：注解用 `fx.Annotate(NewModule, fx.As(new(server.Module)), fx.ResultTags(`group:"modules"`))`——是 **ResultTags 不是 ParamTags**（注解的是构造函数的返回值）；`server.Module` 接口为 `RegisterRoutes(r *gin.RouterGroup)`（`alexgo-server/server/http.go:32`），**不是** `*gin.Engine`，且入参已是 /api 组、内部不再 `r.Group("/api")`。**不需要 `fx.Invoke(registerRoutes)`**——infra 模式是 Module 实现 `server.Module` 进 `group:"modules"`，由 server 的 `Modules []Module` 收集后统一挂载。

- [ ] **Step 4: 运行确认通过**

Run: `CGO_ENABLED=0 go test ./modules/system/oauth2/... -v && CGO_ENABLED=0 go test ./alexgo-server/cmd/ ./modules/member/cmd/`
Expected: PASS。

（若 `base64Captcha` 未引入：`go get github.com/mojocn/base64Captcha@v1.3.8 && go mod tidy`。）

- [ ] **Step 5: Commit**

```bash
git add modules/system/oauth2/ go.mod go.sum
git commit -m "feat(oauth2): 验证码服务（redis/内存双后端、开关）与 GET /oauth2/captcha"
```

---

### Task 4: 统一登录接口定义 + system 侧 UserAuthenticator / JWTGenerator / 审计映射

**Files:**
- Create: `modules/system/oauth2/service/login.go`（哨兵错误、ProtocolError、全部接口）
- Create: `modules/system/oauth2/service/login_test.go`（接口形状/哨兵测试）
- Create: `modules/system/service/user_authenticator.go`
- Create: `modules/system/service/user_authenticator_test.go`
- Modify: `modules/system/module.go`（Provide 增三行，约 58 行附近）

**Interfaces:**
- Consumes: `modules/system/repository.UserRepository.GetByUsername(ctx, tenantID, username)`、`service.PermissionService.{UserRoles, EnsureUserRolePolicy(ctx, username, roles)}`、`pkg/auth.GenerateToken(userID, username, tenantID, cfg)`、`service.AuditService.RecordLogin`。
- Produces（Task 5/8/10/11/12 消费，逐字）：

```go
package oauth2svc // import 路径 alexGo-cloud/modules/system/oauth2/service

const (
	AccountTypeSystem = "system"
	AccountTypeMember = "member"
	AccountTypeClient = "client"
)

type LoginParams struct {
	AccountType  string
	Username     string
	Password     string
	ClientID     string
	CaptchaUUID  string
	CaptchaCode  string
	IP           string
	UserAgent    string
}

type LoginResult struct {
	AccessToken  string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type AuthIdentity struct {
	UserID      uint64
	TenantID    uint64
	Username    string
	AccountType string
}

type UnifiedLogin interface {
	// Login 完整登录：类型校验→captcha→路由→IssueFor→恰一次审计，返回 token。
	Login(ctx context.Context, p LoginParams) (*LoginResult, *AuthIdentity, error)
	// AuthorizeLogin 授权码流登录：类型校验→captcha→路由→恰一次审计，
	// **不签发 token**（防止 authorize 侧产生无人认领的孤儿 token）；
	// p.ClientID 在此方法中忽略（client 校验归 controller）。Task 9/11 消费。
	AuthorizeLogin(ctx context.Context, p LoginParams) (*AuthIdentity, error)
	IssueFor(ctx context.Context, ident *AuthIdentity, clientID string) (*LoginResult, error)
}

type UserAuthenticator interface {
	Authenticate(ctx context.Context, username, password string) (*AuthIdentity, error)
}
type MemberAuthenticator interface {
	Authenticate(ctx context.Context, mobile, password, ip string) (*AuthIdentity, error)
}
type JWTGenerator interface {
	Issue(ctx context.Context, userID uint64, username string, tenantID uint64) (string, error)
}
type LoginAuditor interface {
	RecordLogin(ctx context.Context, username string, userID uint64, ip, ua string, success bool, msg string) error
}

// ProtocolError RFC 6749 错误（Error() = "%d %s: %s"）。
type ProtocolError struct {
	Status      int
	Code        string
	Description string
}

func (e *ProtocolError) Error() string {
	return fmt.Sprintf("%d %s: %s", e.Status, e.Code, e.Description)
}

var (
	ErrInvalidCredentials     = errors.New("invalid credentials")
	ErrAccountTypeUnsupported = errors.New("account type unsupported")
	ErrAuthenticatorMissing   = errors.New("authenticator missing")
	ErrUserDisabled           = errors.New("user disabled")
	ErrIssuance               = errors.New("token issuance failed")
)

type UnifiedDeps struct {
	fx.In
	Cfg       *config.Config
	Issuer    token.Issuer
	Captcha   *CaptchaService
	UserAuth  UserAuthenticator  `optional:"true"`
	MemberAuth MemberAuthenticator `optional:"true"`
	Auditor   LoginAuditor       `optional:"true"`
	JWTGen    JWTGenerator       `optional:"true"`
}

func NewUnifiedLogin(deps UnifiedDeps) UnifiedLogin
```

system 侧（Task 4 产出，Task 8 消费）：

```go
// modules/system/service/user_authenticator.go
func NewUserAuthenticator(userRepo repository.UserRepository, permSvc PermissionService) oauth2svc.UserAuthenticator
func NewJWTGenerator(cfg *config.Config) oauth2svc.JWTGenerator
// modules/system/module.go Provide 增：
//   service.NewUserAuthenticator,
//   service.NewJWTGenerator,
//   func(svc service.AuditService) oauth2svc.LoginAuditor { return svc },
```

- [ ] **Step 1: 写失败测试**

`modules/system/oauth2/service/login_test.go`（先钉接口与哨兵；UnifiedLogin 实现属 Task 8，这里只测类型形状可编译 + ProtocolError 文案）：

```go
package oauth2svc

import "testing"

func TestProtocolError_Format(t *testing.T) {
	e := &ProtocolError{Status: 400, Code: "invalid_grant", Description: "账号或密码错误"}
	if e.Error() != "400 invalid_grant: 账号或密码错误" {
		t.Errorf("Error() = %q", e.Error())
	}
}

func TestSentinels_Distinct(t *testing.T) {
	// 五哨兵两两不等（错误映射矩阵依赖 errors.Is 区分）。
	all := []error{ErrInvalidCredentials, ErrAccountTypeUnsupported, ErrAuthenticatorMissing, ErrUserDisabled, ErrIssuance}
	for i := range all {
		for j := i + 1; j < len(all); j++ {
			if all[i] == all[j] {
				t.Errorf("哨兵 %d 与 %d 相同", i, j)
			}
		}
	}
}
```

`modules/system/service/user_authenticator_test.go`（先写——引用尚不存在的构造器，即 RED。**同包复用 `auth_test.go` 既有夹具**：`fakeUserRepo`（内含 `GetByUsername(_, _ uint64, _)` 三参桩）、`fakePerm`（`EnsureUserRolePolicy(ctx, string, []*model.Role)` 零值实现）、`enabledUser()`（alice/pw，Status 1）、`hashOf`——不新建任何桩）：

```go
package service

import (
	"context"
	"errors"
	"testing"

	"alexGo-cloud/modules/system/model"
	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
)

func TestNewUserAuthenticator_BadPassword(t *testing.T) {
	svc := NewUserAuthenticator(&fakeUserRepo{user: enabledUser()}, fakePerm{})
	_, err := svc.Authenticate(context.Background(), "alice", "wrong")
	if !errors.Is(err, oauth2svc.ErrInvalidCredentials) {
		t.Fatalf("错密码应 ErrInvalidCredentials, got %v", err)
	}
}

func TestNewUserAuthenticator_OK(t *testing.T) {
	repo := &fakeUserRepo{user: enabledUser()}
	svc := NewUserAuthenticator(repo, fakePerm{})
	ident, err := svc.Authenticate(context.Background(), "alice", "pw")
	if err != nil {
		t.Fatal(err)
	}
	if ident.UserID != 3 || ident.Username != "alice" || ident.AccountType != oauth2svc.AccountTypeSystem {
		t.Errorf("identity = %+v", ident)
	}
}

func TestNewUserAuthenticator_Disabled(t *testing.T) {
	repo := &fakeUserRepo{user: &model.User{ID: 6, Username: "x", Status: 0, PasswordHash: hashOf("pw")}}
	svc := NewUserAuthenticator(repo, fakePerm{})
	_, err := svc.Authenticate(context.Background(), "x", "pw")
	if !errors.Is(err, oauth2svc.ErrUserDisabled) {
		t.Fatalf("停用应 ErrUserDisabled, got %v", err)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `CGO_ENABLED=0 go test ./modules/system/oauth2/service/ ./modules/system/service/ -run 'TestProtocolError|TestSentinels|TestNewUserAuthenticator' -v`
Expected: FAIL —— system 侧 `undefined: NewUserAuthenticator`；oauth2 侧若 login.go 未建则 `undefined: ProtocolError`。

- [ ] **Step 3: 最小实现**

1. 按 Produces 块原样创建 `modules/system/oauth2/service/login.go`（补 `fmt`/`errors`/`context`/`go.uber.org/fx`/`alexGo-cloud/pkg/config`/`alexGo-cloud/pkg/token` import；`NewUnifiedLogin` 此时只返回结构体占位——**但 Task 8 才实现 Login/IssueFor**，为使本任务编译，先给出接收 deps 返回 `UnifiedLogin` 的最小 stub，Task 8 覆写为真逻辑。裁决：`NewUnifiedLogin` 与 `unifiedLogin` 结构在 Task 4 就建好，`Login`/`IssueFor` 返回 `nil, ErrAuthenticatorMissing`（Task 8 补真身，测试 Task 8 写）。——**记录 Ruling 见 Task 8 开头**）。
2. `modules/system/service/user_authenticator.go`：

```go
package service

import (
	"context"
	"errors"

	"golang.org/x/crypto/bcrypt"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/repository"
	"alexGo-cloud/pkg/auth"
	"alexGo-cloud/pkg/config"
	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
)

// NewUserAuthenticator system 账号凭据校验（users 表零改动，spec §2 边界1）。
func NewUserAuthenticator(userRepo repository.UserRepository, permSvc PermissionService) oauth2svc.UserAuthenticator {
	return &userAuthenticator{repo: userRepo, perm: permSvc}
}

type userAuthenticator struct {
	repo repository.UserRepository
	perm PermissionService
}

func (a *userAuthenticator) Authenticate(ctx context.Context, username, password string) (*oauth2svc.AuthIdentity, error) {
	tid := tenant.TenantIDFromContext(ctx)
	u, err := a.repo.GetByUsername(ctx, tid, username)
	if err != nil || u == nil {
		return nil, oauth2svc.ErrInvalidCredentials
	}
	if u.Status != 1 {
		return nil, oauth2svc.ErrUserDisabled
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return nil, oauth2svc.ErrInvalidCredentials
	}
	// 登录成功补挂默认角色策略（对齐旧 AuthService.Login 行为：
	// 先取角色再 Ensure，失败仅告警不阻断——与 auth.go 64-72 行同构）。
	if a.perm != nil {
		if roles, rerr := a.perm.UserRoles(ctx, u.ID); rerr == nil {
			_ = a.perm.EnsureUserRolePolicy(ctx, u.Username, roles)
		}
	}
	return &oauth2svc.AuthIdentity{
		UserID:      u.ID,
		TenantID:    u.TenantID,
		Username:    u.Username,
		AccountType: oauth2svc.AccountTypeSystem,
	}, nil
}

// NewJWTGenerator jwt 回滚模式（mode=jwt）用（D2）。
func NewJWTGenerator(cfg *config.Config) oauth2svc.JWTGenerator {
	return &jwtGenerator{cfg: cfg}
}

type jwtGenerator struct{ cfg *config.Config }

func (g *jwtGenerator) Issue(ctx context.Context, userID uint64, username string, tenantID uint64) (string, error) {
	return auth.GenerateToken(userID, username, tenantID, g.cfg)
}
```

> `auth.GenerateToken` 真实签名（`pkg/auth/jwt.go:26`）：`GenerateToken(userID uint64, username string, tenantID uint64, cfg *config.Config) (string, error)`——HS256、exp 24h、`cfg.System.JWTSecret` 空则报错。`user_authenticator.go` 需补 import `alexGo-cloud/pkg/tenant`（`tenant.TenantIDFromContext`）。

3. `modules/system/module.go` Provide 块（`service.NewAuthService` 行附近）追加：

```go
		service.NewUserAuthenticator,
		service.NewJWTGenerator,
		func(svc service.AuditService) oauth2svc.LoginAuditor { return svc },
```

补 import `oauth2svc "alexGo-cloud/modules/system/oauth2/service"`。

- [ ] **Step 4: 运行确认通过**

Run: `CGO_ENABLED=0 go test ./modules/system/oauth2/... ./modules/system/... -v`
Expected: PASS（新测试 + 既有 system 测试全绿）。

- [ ] **Step 5: Commit**

```bash
git add modules/system/oauth2/service/login.go modules/system/oauth2/service/login_test.go \
        modules/system/service/user_authenticator.go modules/system/service/user_authenticator_test.go \
        modules/system/module.go
git commit -m "feat(oauth2): 统一登录接口定义与 system 侧凭据/JWT/审计接线"
```

---

### Task 5: member Authenticate + MemberAuthenticator 映射

**Files:**
- Modify: `modules/member/service/member.go`（加 `Authenticate`；`Login` 改为委托）
- Modify: `modules/member/service/member_test.go`（凭据测试改用 Authenticate）
- Modify: `modules/member/module.go`（Provide 增映射一行）
- Test: `modules/member/service/member_authenticate_test.go`（新也可并入 member_test.go）

**Interfaces:**
- Consumes: Task 4 的 `oauth2svc.MemberAuthenticator`、`AuthIdentity`。
- Produces: `MemberService.Authenticate(ctx, mobile, password, ip string) (*oauth2svc.AuthIdentity, error)`（Task 8 消费）；module 映射 `func(svc service.MemberService) oauth2svc.MemberAuthenticator { return svc }`。

- [ ] **Step 1: 写失败测试**

在 `modules/member/service/member_test.go` 追加（该文件夹具是 `memRepo`/`fakeIssuer`（返回 "ma"/"mr"）——**没有** `enabledRepo`/`okIssuer`，不要发明；用下面的助手就地构造）。该文件需补 import `golang.org/x/crypto/bcrypt` 与 `oauth2svc "alexGo-cloud/modules/system/oauth2/service"`：

```go
// newAuthenticateRepo 造一个持有效凭据的 member 行（Mobile 13800000000 / pw123456），
// status 可翻转停用。memRepo.Create 分配 nextID=1 → 行 ID 恒 1；ctx 无租户 → tid 0，
// 与 GetByMobile(ctx, 0, mobile) 对齐。
func newAuthenticateRepo(t *testing.T, status int) *memRepo {
	t.Helper()
	repo := newMemRepo()
	h, err := bcrypt.GenerateFromPassword([]byte("pw123456"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(context.Background(), &model.MemberUser{
		TenantID: 0, Mobile: "13800000000", Password: string(h), Status: status, Nickname: "tester",
	}); err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestAuthenticate_OK(t *testing.T) {
	svc := NewMemberService(newAuthenticateRepo(t, 1), &fakeIssuer{}, nil)
	ident, err := svc.Authenticate(context.Background(), "13800000000", "pw123456", "1.2.3.4")
	if err != nil {
		t.Fatal(err)
	}
	if ident.UserID != 1 || ident.Username != "13800000000" ||
		ident.AccountType != oauth2svc.AccountTypeMember || ident.TenantID != 0 {
		t.Errorf("identity = %+v", ident)
	}
}

func TestAuthenticate_WrongPassword(t *testing.T) {
	svc := NewMemberService(newAuthenticateRepo(t, 1), &fakeIssuer{}, nil)
	_, err := svc.Authenticate(context.Background(), "13800000000", "bad", "")
	if !errors.Is(err, oauth2svc.ErrInvalidCredentials) {
		t.Fatalf("应 ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthenticate_Disabled(t *testing.T) {
	svc := NewMemberService(newAuthenticateRepo(t, 0), &fakeIssuer{}, nil)
	_, err := svc.Authenticate(context.Background(), "13800000000", "pw123456", "")
	if !errors.Is(err, oauth2svc.ErrUserDisabled) {
		t.Fatalf("停用应 ErrUserDisabled, got %v", err)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `CGO_ENABLED=0 go test ./modules/member/service/ -run TestAuthenticate -v`
Expected: FAIL —— `svc.Authenticate undefined`（MemberService 接口无此方法）。

- [ ] **Step 3: 最小实现**

1. `modules/member/service/member.go` 接口（`MemberService`）加一行：

```go
	Authenticate(ctx context.Context, mobile, password, ip string) (*oauth2svc.AuthIdentity, error)
```

（补 import `oauth2svc "alexGo-cloud/modules/system/oauth2/service"`。）实现（全部签名已对过仓库现实：`GetByMobile(ctx, tid, mobile)` 三参、字段 `u.Password`、无 `UpdateLoginInfo`）：

```go
// Authenticate member 账号凭据校验（members 表零改动；校验通过即写 LoginIP/LoginDate，
// 与旧 Login 成功路径同语义——统一登录/兼容层共用）。
func (s *memberService) Authenticate(ctx context.Context, mobile, password, ip string) (*oauth2svc.AuthIdentity, error) {
	tid := tenant.TenantIDFromContext(ctx)
	u, err := s.repo.GetByMobile(ctx, tid, mobile)
	if err != nil || u == nil {
		return nil, oauth2svc.ErrInvalidCredentials
	}
	if u.Status != 1 {
		return nil, oauth2svc.ErrUserDisabled
	}
	if bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)) != nil {
		return nil, oauth2svc.ErrInvalidCredentials
	}
	if ip != "" {
		now := time.Now()
		u.LoginIP, u.LoginDate = ip, &now
		_ = s.repo.Update(ctx, u)
	}
	return &oauth2svc.AuthIdentity{
		UserID:      u.ID,
		TenantID:    tid,
		Username:    u.Mobile,
		AccountType: oauth2svc.AccountTypeMember,
	}, nil
}
```

> 拆解：旧 `Login`（member.go:110 起）里「查用户→比密码→写登录 IP/日期→签 token」的**前三步**抽成本方法；`Login` 剩余改为 `ident, err := s.Authenticate(ctx, mobile, password, ip); if err != nil { return nil, err }` 后照旧 `issuer.Issue(...)`（ClientID 仍 `"alexgo-app"`、UserType 仍 member）——**旧 Login 签名与返回类型 `*LoginResult` 不变**，既有 `TestRegister_ThenLogin` 断言 `res.AccessToken=="ma"` 必须保持绿。

module 映射（`modules/member/module.go` Provide 块）追加：

```go
		func(svc service.MemberService) oauth2svc.MemberAuthenticator { return svc },
```

补 import `oauth2svc "alexGo-cloud/modules/system/oauth2/service"`。

- [ ] **Step 4: 运行确认通过**

Run: `CGO_ENABLED=0 go test ./modules/member/... -v && CGO_ENABLED=0 go test ./modules/member/cmd/`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add modules/member/
git commit -m "feat(member): Authenticate 凭据校验与 MemberAuthenticator 装配"
```

---

### Task 6: pkg/token 客户端凭证扩展（UserTypeClient=3）

**Files:**
- Modify: `pkg/token/service.go`（Issue 参数校验、Validate switch、Refresh）
- Test: `pkg/token/service_test.go`（追加 client 三例）

**Interfaces:**
- Consumes: 既有 `UserTypeAdmin=1`/`UserTypeMember=2`、`IssueParams{UserID, UserType, TenantID, ClientID}`、`Claims`、`Validate` 的 switch、`newTestService(t)` 助手。
- Produces: `token.UserTypeClient = 3`；Issue/Validate/Refresh 对 type 3 的语义（Task 7/8/10/15 消费）——**D9**：Issue 允许 `UserID==0` 当且仅当 `UserType==3 && ClientID!=""`；Validate type 3 → `claims.Username = row.ClientID`（不查库）；Refresh 对 client 行走通（机机实际不用 refresh，但 Issue 放行后 Refresh 自然可用——**裁决：Refresh 保持可用，D9「机机无 refresh」由 TokenService 层不给 client_credentials 发 refresh 实现**）。

- [ ] **Step 1: 写失败测试**

`pkg/token/service_test.go` 追加（用既有 `newTestService(t)`）：

```go
func TestIssue_ClientCredentials_AllowZeroUser(t *testing.T) {
	svc := newTestService(t)
	// D9：type 3 + ClientID 非空 → UserID 0 放行
	issued, err := svc.Issue(context.Background(), IssueParams{
		UserID: 0, UserType: UserTypeClient, TenantID: 0, ClientID: "svc-bot",
	})
	if err != nil {
		t.Fatalf("type3 UserID=0 应放行, got %v", err)
	}
	if issued.RefreshToken != "" {
		t.Error("机机 token 不应带 refresh")
	}
	// Validate：username 取 ClientID
	claims, err := svc.Validate(context.Background(), issued.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Username != "svc-bot" || claims.UserType != UserTypeClient {
		t.Errorf("claims = %+v", claims)
	}
}

func TestIssue_ClientCredentials_MissingClientID(t *testing.T) {
	svc := newTestService(t)
	if _, err := svc.Issue(context.Background(), IssueParams{UserID: 0, UserType: UserTypeClient}); err == nil {
		t.Fatal("type3 且 ClientID 空应拒")
	}
}

func TestIssue_UnknownTypeRejected(t *testing.T) {
	svc := newTestService(t)
	if _, err := svc.Issue(context.Background(), IssueParams{UserID: 1, UserType: 99}); err == nil {
		t.Fatal("未知 UserType 应拒")
	}
}
```

> 若 `Issue` 原实现不校验 UserType 枚举（只校验 UserID/UserType 非零），「未知类型拒」是新行为——D9 要求加上。机机 Issue 时 refresh 行为：原 Issue 总发 refresh；type 3 需不发——测试第一例已断言。

- [ ] **Step 2: 运行确认失败**

Run: `CGO_ENABLED=0 go test ./pkg/token/ -run 'TestIssue_Client|TestIssue_Unknown' -v`
Expected: FAIL —— `undefined: UserTypeClient`（编译错误）。

- [ ] **Step 3: 最小实现**

`pkg/token/service.go`：

```go
// UserTypeClient 机机凭证（D9）。
const UserTypeClient = 3
```

Issue 校验段改：

```go
	// 原：if p.UserID == 0 || p.UserType == 0 { return err }
	// D9：type 3 允许 UserID==0，当且仅当 ClientID 非空；UserType 枚举白名单。
	switch p.UserType {
	case UserTypeAdmin, UserTypeMember:
		if p.UserID == 0 {
			return errors.New("token: user id required")
		}
	case UserTypeClient:
		if p.ClientID == "" {
			return errors.New("token: client id required for client credentials")
		}
	default:
		return errors.New("token: unknown user type")
	}
```

type 3 不写 refresh 行（`if p.UserType != UserTypeClient { ...写 refresh... }`——**注意**：若 refresh 行与 access 行同函数生成，需跳过 refresh 随机与插入；access 照写）。

Validate 的 switch 加：

```go
	case UserTypeClient:
		claims.Username = row.ClientID
```

（无库查询。）

- [ ] **Step 4: 运行确认通过**

Run: `CGO_ENABLED=0 go test ./pkg/token/ -v`
Expected: PASS（新增 3 例 + 既有全绿）。

- [ ] **Step 5: Commit**

```bash
git add pkg/token/
git commit -m "feat(token): UserTypeClient=3 机机凭证（零用户放行、ClientID 作 username）"
```

---

### Task 7: ClientRepository + ClientService（client 注册存储与认证）

**Files:**
- Create: `modules/system/oauth2/repository/client.go`
- Create: `modules/system/oauth2/service/client.go`
- Test: `modules/system/oauth2/service/client_test.go`
- Modify: `modules/system/oauth2/module.go`（Provide 块追加两行）

**Interfaces:**
- Consumes: Task 2 的 `model.Oauth2Client`（字段 `ClientID/ClientSecret/ClientName/GrantTypes/RedirectURIs/AccountTypes/Scopes/Activity` + `AllowsGrant/AllowsAccountType/AllowsRedirect`）、`gorm.DB`（fx 基座已 Provide）、D10 公开客户端规则（库中 secret 空 → 跳过 bcrypt）。
- Produces（Task 9/10/13 消费，逐字）：

```go
package repository // import 路径 alexGo-cloud/modules/system/oauth2/repository

func NewClientRepository(db *gorm.DB) *ClientRepository
func (r *ClientRepository) GetByClientID(ctx context.Context, clientID string) (*model.Oauth2Client, error)
func (r *ClientRepository) GetByID(ctx context.Context, id uint64) (*model.Oauth2Client, error)
func (r *ClientRepository) Create(ctx context.Context, c *model.Oauth2Client) error
func (r *ClientRepository) Update(ctx context.Context, c *model.Oauth2Client) error
func (r *ClientRepository) Delete(ctx context.Context, id uint64) error
func (r *ClientRepository) List(ctx context.Context, page, size int) ([]*model.Oauth2Client, int64, error)

package oauth2svc // import 路径 alexGo-cloud/modules/system/oauth2/service

var ErrInvalidClient = errors.New("invalid client") // 未知/停用/凭据不符 → 401 invalid_client
func NewClientService(repo *repository.ClientRepository) *ClientService
func (s *ClientService) Get(ctx context.Context, clientID string) (*model.Oauth2Client, error)
func (s *ClientService) Authenticate(ctx context.Context, clientID, clientSecret string) (*model.Oauth2Client, error)
func (s *ClientService) List(ctx context.Context, page, size int) ([]*model.Oauth2Client, int64, error)
func (s *ClientService) Create(ctx context.Context, c *model.Oauth2Client) error
func (s *ClientService) Update(ctx context.Context, c *model.Oauth2Client) error
func (s *ClientService) Delete(ctx context.Context, id uint64) error
```

（Task 9 消费 `Get`、Task 10 消费 `Authenticate`、Task 13 消费 CRUD 四方法；`List` 返回的 `total` 是 admin 信封 `{data,total}` 的 total 来源。）

- [ ] **Step 1: 写失败测试**

`modules/system/oauth2/service/client_test.go`：

```go
package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"alexGo-cloud/modules/system/oauth2/model"
	"alexGo-cloud/modules/system/oauth2/repository"
)

func newTestClientSvc(t *testing.T) (*ClientService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Oauth2Client{}); err != nil {
		t.Fatal(err)
	}
	return NewClientService(repository.NewClientRepository(db)), db
}

func mustCreateClient(t *testing.T, svc *ClientService, c *model.Oauth2Client) {
	t.Helper()
	if err := svc.Create(context.Background(), c); err != nil {
		t.Fatal(err)
	}
}

// D10：公开客户端（库中 secret 空）跳过 bcrypt 比对，任意提交值放行。
func TestClientService_Authenticate_PublicSkipBcrypt(t *testing.T) {
	svc, _ := newTestClientSvc(t)
	mustCreateClient(t, svc, &model.Oauth2Client{
		ClientID: "pub", ClientName: "公开",
		GrantTypes: "authorization_code", AccountTypes: "system",
	})
	c, err := svc.Authenticate(context.Background(), "pub", "任意secret")
	if err != nil {
		t.Fatalf("公开客户端任意 secret 应过, got %v", err)
	}
	if c.ClientID != "pub" {
		t.Errorf("client = %+v", c)
	}
}

func TestClientService_Authenticate_WrongSecret(t *testing.T) {
	svc, _ := newTestClientSvc(t)
	mustCreateClient(t, svc, &model.Oauth2Client{
		ClientID: "conf", ClientSecret: "s3cret",
		GrantTypes: "password", AccountTypes: "member",
	})
	if _, err := svc.Authenticate(context.Background(), "conf", "nope"); !errors.Is(err, ErrInvalidClient) {
		t.Fatalf("错 secret 应 ErrInvalidClient, got %v", err)
	}
	if _, err := svc.Authenticate(context.Background(), "conf", "s3cret"); err != nil {
		t.Fatalf("正确 secret 应过, got %v", err)
	}
}

func TestClientService_Authenticate_Inactive(t *testing.T) {
	svc, db := newTestClientSvc(t)
	mustCreateClient(t, svc, &model.Oauth2Client{ClientID: "off", ClientSecret: "pw", GrantTypes: "password"})
	if err := db.Model(&model.Oauth2Client{}).Where("client_id = ?", "off").Update("activity", 0).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Authenticate(context.Background(), "off", "pw"); !errors.Is(err, ErrInvalidClient) {
		t.Fatalf("停用应 ErrInvalidClient, got %v", err)
	}
	if _, err := svc.Get(context.Background(), "off"); !errors.Is(err, ErrInvalidClient) {
		t.Fatalf("Get 停用应 ErrInvalidClient, got %v", err)
	}
}

func TestClientService_Get_Unknown(t *testing.T) {
	svc, _ := newTestClientSvc(t)
	if _, err := svc.Get(context.Background(), "nope"); !errors.Is(err, ErrInvalidClient) {
		t.Fatalf("未知 client 应 ErrInvalidClient, got %v", err)
	}
}

func TestClientService_Create_HashesSecret(t *testing.T) {
	svc, db := newTestClientSvc(t)
	mustCreateClient(t, svc, &model.Oauth2Client{
		ClientID: "c1", ClientSecret: "plain",
		Activity: 0, Scopes: "",
	})
	var row model.Oauth2Client
	if err := db.Where("client_id = ?", "c1").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.ClientSecret == "plain" || bcrypt.CompareHashAndPassword([]byte(row.ClientSecret), []byte("plain")) != nil {
		t.Errorf("secret 未 bcrypt: %q", row.ClientSecret)
	}
	if row.Activity != 1 {
		t.Errorf("Activity 应默认 1, got %d", row.Activity)
	}
	if row.Scopes != "all" {
		t.Errorf("Scopes 应默认 all, got %q", row.Scopes)
	}
	// client_id 唯一：重复创建应报错
	dup := &model.Oauth2Client{ClientID: "c1"}
	if err := svc.Create(context.Background(), dup); err == nil {
		t.Error("重复 client_id 应报错")
	}
}

func TestClientService_List_Pagination(t *testing.T) {
	svc, _ := newTestClientSvc(t)
	for i := 0; i < 3; i++ {
		mustCreateClient(t, svc, &model.Oauth2Client{
			ClientID: fmt.Sprintf("c%d", i), GrantTypes: "password", AccountTypes: "member",
		})
	}
	rows, total, err := svc.List(context.Background(), 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || total != 3 {
		t.Errorf("rows=%d total=%d, want 2/3", len(rows), total)
	}
}

// Update secret 空 → 保留旧哈希（防编辑表单误清机密）；非空 → 重哈希。
func TestClientService_Update_SecretRules(t *testing.T) {
	svc, db := newTestClientSvc(t)
	mustCreateClient(t, svc, &model.Oauth2Client{ClientID: "u1", ClientSecret: "old", GrantTypes: "password"})

	// 空 secret：保留旧值
	c := &model.Oauth2Client{ID: 0, ClientID: "u1", ClientSecret: "", GrantTypes: "password,refresh_token"}
	var row model.Oauth2Client
	if err := db.Where("client_id = ?", "u1").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	c.ID = row.ID
	if err := svc.Update(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&row, row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if bcrypt.CompareHashAndPassword([]byte(row.ClientSecret), []byte("old")) != nil {
		t.Errorf("空 secret 应保留旧哈希, got %q", row.ClientSecret)
	}

	// 非空 secret：重哈希
	c.ClientSecret = "new"
	if err := svc.Update(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&row, row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if bcrypt.CompareHashAndPassword([]byte(row.ClientSecret), []byte("new")) != nil {
		t.Errorf("非空 secret 应重哈希, got %q", row.ClientSecret)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `CGO_ENABLED=0 go test ./modules/system/oauth2/service/ -run TestClientService -v`
Expected: FAIL —— `undefined: NewClientService / repository.NewClientRepository`（编译错误）。

- [ ] **Step 3: 最小实现**

`modules/system/oauth2/repository/client.go`：

```go
package repository

import (
	"context"

	"gorm.io/gorm"

	"alexGo-cloud/modules/system/oauth2/model"
)

// ClientRepository oauth2_client 访问层（D4：模型无 tenant_id/deleted → 租户插件跳过、硬删除）。
type ClientRepository struct {
	db *gorm.DB
}

func NewClientRepository(db *gorm.DB) *ClientRepository {
	return &ClientRepository{db: db}
}

func (r *ClientRepository) GetByClientID(ctx context.Context, clientID string) (*model.Oauth2Client, error) {
	var c model.Oauth2Client
	if err := r.db.WithContext(ctx).Where("client_id = ?", clientID).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *ClientRepository) GetByID(ctx context.Context, id uint64) (*model.Oauth2Client, error) {
	var c model.Oauth2Client
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *ClientRepository) Create(ctx context.Context, c *model.Oauth2Client) error {
	return r.db.WithContext(ctx).Create(c).Error
}

func (r *ClientRepository) Update(ctx context.Context, c *model.Oauth2Client) error {
	return r.db.WithContext(ctx).Save(c).Error
}

// Delete 硬删（D4）；不存在返回 nil——删除幂等。
func (r *ClientRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Oauth2Client{}).Error
}

func (r *ClientRepository) List(ctx context.Context, page, size int) ([]*model.Oauth2Client, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&model.Oauth2Client{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []*model.Oauth2Client
	err := r.db.WithContext(ctx).
		Order("id ASC").Offset((page - 1) * size).Limit(size).
		Find(&rows).Error
	return rows, total, err
}
```

`modules/system/oauth2/service/client.go`：

```go
package oauth2svc

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"alexGo-cloud/modules/system/oauth2/model"
	"alexGo-cloud/modules/system/oauth2/repository"
)

// ErrInvalidClient 未知/停用/凭据不符（Task 9 映射 400 invalid_client、Task 10 映射 401 invalid_client）。
var ErrInvalidClient = errors.New("invalid client")

type ClientService struct {
	repo *repository.ClientRepository
}

func NewClientService(repo *repository.ClientRepository) *ClientService {
	return &ClientService{repo: repo}
}

// Get 取启用中的 client（authorize 端点入口校验，Task 9）。
func (s *ClientService) Get(ctx context.Context, clientID string) (*model.Oauth2Client, error) {
	c, err := s.repo.GetByClientID(ctx, clientID)
	if err != nil || c == nil || c.Activity != 1 {
		return nil, ErrInvalidClient
	}
	return c, nil
}

// Authenticate client 凭据校验（Task 10 token 端点第一步）：
// 库中 secret 空 = 公开客户端跳过比对（D10，授权码流靠 PKCE）；
// 机密客户端 bcrypt 比对；未知/停用/不符一律 ErrInvalidClient（不区分原因，防探测）。
func (s *ClientService) Authenticate(ctx context.Context, clientID, clientSecret string) (*model.Oauth2Client, error) {
	c, err := s.repo.GetByClientID(ctx, clientID)
	if err != nil || c == nil || c.Activity != 1 {
		return nil, ErrInvalidClient
	}
	if c.ClientSecret == "" {
		return c, nil
	}
	if bcrypt.CompareHashAndPassword([]byte(c.ClientSecret), []byte(clientSecret)) != nil {
		return nil, ErrInvalidClient
	}
	return c, nil
}

func (s *ClientService) List(ctx context.Context, page, size int) ([]*model.Oauth2Client, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	return s.repo.List(ctx, page, size)
}

// Create 校验并落库：client_id 非空；secret 非空 → bcrypt 存哈希（明文只在请求里出现一次）；
// Activity 零值补 1（新建即启用，停用走 Update）；Scopes 空补 "all"。
func (s *ClientService) Create(ctx context.Context, c *model.Oauth2Client) error {
	if c.ClientID == "" {
		return fmt.Errorf("client_id required")
	}
	if c.ClientSecret != "" {
		h, err := bcrypt.GenerateFromPassword([]byte(c.ClientSecret), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		c.ClientSecret = string(h)
	}
	if c.Activity == 0 {
		c.Activity = 1
	}
	if c.Scopes == "" {
		c.Scopes = "all"
	}
	return s.repo.Create(ctx, c)
}

// Update secret 空 → 保留库中旧值（编辑表单留空即不改机密）；非空 → 重哈希。
func (s *ClientService) Update(ctx context.Context, c *model.Oauth2Client) error {
	if c.ID == 0 {
		return fmt.Errorf("invalid id")
	}
	if c.ClientID == "" {
		return fmt.Errorf("client_id required")
	}
	old, err := s.repo.GetByID(ctx, c.ID)
	if err != nil {
		return fmt.Errorf("client not found")
	}
	if c.ClientSecret == "" {
		c.ClientSecret = old.ClientSecret
	} else {
		h, herr := bcrypt.GenerateFromPassword([]byte(c.ClientSecret), bcrypt.DefaultCost)
		if herr != nil {
			return herr
		}
		c.ClientSecret = string(h)
	}
	if c.Scopes == "" {
		c.Scopes = "all"
	}
	return s.repo.Update(ctx, c)
}

func (s *ClientService) Delete(ctx context.Context, id uint64) error {
	if id == 0 {
		return fmt.Errorf("invalid id")
	}
	return s.repo.Delete(ctx, id)
}
```

`modules/system/oauth2/module.go` 的 `fx.Provide` 块追加（import 补 `"alexGo-cloud/modules/system/oauth2/repository"`）：

```go
		repository.NewClientRepository,
		oauth2svc.NewClientService,
```

- [ ] **Step 4: 运行确认通过**

Run: `CGO_ENABLED=0 go test ./modules/system/oauth2/... -v && CGO_ENABLED=0 go test ./alexgo-server/cmd/ ./modules/member/cmd/`
Expected: PASS（新测试 + fx 干跑图仍可满足——ClientService 依赖的 `*gorm.DB` 基座已 Provide）。

- [ ] **Step 5: Commit**

```bash
git add modules/system/oauth2/
git commit -m "feat(oauth2): ClientRepository/ClientService（公开跳 bcrypt、CRUD、分页）"
```

---

### Task 8: UnifiedLogin 真身（冻结语义：类型校验→captcha→路由→签发→恰一次审计）

> **Ruling（覆写 Task 4）**：Task 4 落地的 `NewUnifiedLogin` 是占位 stub（`Login/IssueFor` 返回 `ErrAuthenticatorMissing`），仅为让当时的编译与图装配通过。**本任务删除 stub 方法体，换成下面的真逻辑**；Task 4 的接口、`LoginParams/LoginResult/AuthIdentity`、五哨兵、`ProtocolError`、`UnifiedDeps` 全部不动。

**Files:**
- Modify: `modules/system/oauth2/service/login.go`（删 stub，写真身）
- Test: `modules/system/oauth2/service/unified_login_test.go`（新建）
- Modify: `modules/system/oauth2/module.go`（Provide 追加 `oauth2svc.NewUnifiedLogin`）

**Interfaces:**
- Consumes: Task 4 的 `UnifiedLogin` 接口三方法、`UnifiedDeps`、五哨兵；Task 3 的 `CaptchaService.Verify/Enabled`、`ErrCaptchaInvalid`；`token.Issuer.Issue(IssueParams) (*Issued, error)`、`token.UserTypeAdmin/UserTypeMember`（+ Task 6 的 `UserTypeClient`）；`cfg.Auth.Mode`（`"jwt"` 回滚模式）。
- Produces（Task 10/11/12 消费，行为契约）：
  - `Login`：类型校验（`""`→system；仅 `system|member`，否则 `ErrAccountTypeUnsupported` **先于 captcha/审计、无审计**）→ captcha（`Captcha` 字段 nil 安全跳过；`Verify` 内部按开关放行）→ 按类型路由 `UserAuth/MemberAuth`（nil→`ErrAuthenticatorMissing`）→ `IssueFor` → 审计 **每恰一次**（成功 `[true]` msg `"oauth2: ok"`；失败 `[false]` msg `"oauth2: "+err`；unsupported 无审计）。
  - `AuthorizeLogin`：同上前半（校验→captcha→路由→恰一次审计），**不签发 token**；`p.ClientID` 忽略。
  - `IssueFor(ident, clientID)`：`ident==nil`→wrap `ErrIssuance`；`AccountType==client`→恒走 `Issuer` 签 `UserTypeClient/UserID=0/TenantID=0`，**refresh 强制 `""`（jwt 模式也如此，D9）**；`system && cfg.Auth.Mode=="jwt"`→`JWTGen`（nil→wrap `ErrIssuance`），返回 `{AccessToken}` refresh 空 expires 0；`system||member` 其余→`Issuer` 按类型签不透明；**任何 Issue/jwtGen 错误 wrap `ErrIssuance`**（`errors.Is` 可判）。

- [ ] **Step 1: 写失败测试**

`modules/system/oauth2/service/unified_login_test.go`（夹具与 `captcha_test.go` 同包共享 `newTestCaptcha`）：

```go
package oauth2svc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/token"
)

type stubUserAuth struct {
	ident *AuthIdentity
	err   error
	calls int
}

func (s *stubUserAuth) Authenticate(_ context.Context, _, _ string) (*AuthIdentity, error) {
	s.calls++
	return s.ident, s.err
}

type stubMemberAuth struct {
	ident  *AuthIdentity
	err    error
	calls  int
	lastIP string
}

func (s *stubMemberAuth) Authenticate(_ context.Context, _, _, ip string) (*AuthIdentity, error) {
	s.calls++
	s.lastIP = ip
	return s.ident, s.err
}

type stubAuditor struct {
	success []bool
	msgs    []string
}

func (s *stubAuditor) RecordLogin(_ context.Context, _ string, _ uint64, _, _ string, success bool, msg string) error {
	s.success = append(s.success, success)
	s.msgs = append(s.msgs, msg)
	return nil
}

type stubJWTGen struct {
	tk    string
	err   error
	calls int
}

func (s *stubJWTGen) Issue(_ context.Context, _ uint64, _ string, _ uint64) (string, error) {
	s.calls++
	return s.tk, s.err
}

type stubIssuer struct {
	issued *token.Issued
	err    error
	calls  []token.IssueParams
}

func (s *stubIssuer) Issue(_ context.Context, p token.IssueParams) (*token.Issued, error) {
	s.calls = append(s.calls, p)
	return s.issued, s.err
}
func (s *stubIssuer) Refresh(context.Context, string) (*token.Issued, error)  { return nil, nil }
func (s *stubIssuer) Revoke(context.Context, string) error                    { return nil }
func (s *stubIssuer) RevokeAll(context.Context, token.UserType, uint64) error { return nil }

func newTestCfg(mode string) *config.Config {
	cfg := &config.Config{}
	cfg.Auth.Mode = mode
	return cfg
}

func okIssuer() *stubIssuer {
	return &stubIssuer{issued: &token.Issued{AccessToken: "a", RefreshToken: "r", ExpiresIn: 60}}
}

// Review Focus #2：登录链路确实经过 captcha Verify（开关双路径与一次性由 Task 3 钉）。
func TestLogin_CaptchaTogglePaths(t *testing.T) {
	ident := &AuthIdentity{UserID: 3, Username: "alice", AccountType: AccountTypeSystem}

	// 开关关：无 uuid/code 也放行，全链路走通。
	audOff := &stubAuditor{}
	off := NewUnifiedLogin(UnifiedDeps{
		Cfg: newTestCfg("token"), Issuer: okIssuer(),
		Captcha: newTestCaptcha(false), UserAuth: &stubUserAuth{ident: ident}, Auditor: audOff,
	})
	res, _, err := off.Login(context.Background(), LoginParams{Username: "alice", Password: "pw"})
	if err != nil || res.AccessToken != "a" {
		t.Fatalf("disabled 开关应放行, res=%+v err=%v", res, err)
	}
	if len(audOff.success) != 1 || audOff.success[0] != true {
		t.Errorf("审计 = %v, want [true]", audOff.success)
	}

	// 开关开：错码 → ErrCaptchaInvalid；不路由、不签发、审计 false 恰一次。
	ua := &stubUserAuth{ident: ident}
	iss := &stubIssuer{}
	aud := &stubAuditor{}
	on := NewUnifiedLogin(UnifiedDeps{
		Cfg: newTestCfg("token"), Issuer: iss,
		Captcha: newTestCaptcha(true), UserAuth: ua, Auditor: aud,
	})
	_, _, err = on.Login(context.Background(), LoginParams{
		Username: "alice", Password: "pw", CaptchaUUID: "x", CaptchaCode: "0000",
	})
	if !errors.Is(err, ErrCaptchaInvalid) {
		t.Fatalf("应 ErrCaptchaInvalid, got %v", err)
	}
	if ua.calls != 0 || len(iss.calls) != 0 {
		t.Errorf("captcha 失败不得路由/签发: userAuth=%d issuer=%d", ua.calls, len(iss.calls))
	}
	if len(aud.success) != 1 || aud.success[0] != false {
		t.Errorf("审计 = %v, want [false] 恰一次", aud.success)
	}
	if !strings.HasPrefix(aud.msgs[0], "oauth2: ") {
		t.Errorf("审计 msg 应带 oauth2: 前缀, got %q", aud.msgs[0])
	}
}

func TestLogin_WrongPassword_AuditFalseOnce(t *testing.T) {
	ua := &stubUserAuth{err: ErrInvalidCredentials}
	iss := &stubIssuer{}
	aud := &stubAuditor{}
	ul := NewUnifiedLogin(UnifiedDeps{
		Cfg: newTestCfg("token"), Issuer: iss, UserAuth: ua, Auditor: aud,
	})
	_, _, err := ul.Login(context.Background(), LoginParams{Username: "a", Password: "bad"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("应 ErrInvalidCredentials, got %v", err)
	}
	if len(aud.success) != 1 || aud.success[0] != false {
		t.Errorf("审计 = %v, want [false] 恰一次", aud.success)
	}
	if len(iss.calls) != 0 {
		t.Errorf("凭据失败不得签发, issuer calls=%d", len(iss.calls))
	}
}

func TestLogin_AccountTypeUnsupported_NoAudit(t *testing.T) {
	aud := &stubAuditor{}
	ul := NewUnifiedLogin(UnifiedDeps{
		Cfg: newTestCfg("token"), Issuer: okIssuer(),
		UserAuth: &stubUserAuth{ident: &AuthIdentity{UserID: 1}}, Auditor: aud,
	})
	_, _, err := ul.Login(context.Background(), LoginParams{
		AccountType: "admin", Username: "x", Password: "y",
	})
	if !errors.Is(err, ErrAccountTypeUnsupported) {
		t.Fatalf("应 ErrAccountTypeUnsupported, got %v", err)
	}
	if len(aud.success) != 0 {
		t.Errorf("类型不支持先于审计, got %v", aud.success)
	}
}

func TestLogin_EmptyAccountTypeDefaultsSystem(t *testing.T) {
	ua := &stubUserAuth{ident: &AuthIdentity{UserID: 3, Username: "alice", AccountType: AccountTypeSystem}}
	ma := &stubMemberAuth{}
	ul := NewUnifiedLogin(UnifiedDeps{
		Cfg: newTestCfg("token"), Issuer: okIssuer(),
		UserAuth: ua, MemberAuth: ma, Auditor: &stubAuditor{},
	})
	if _, _, err := ul.Login(context.Background(), LoginParams{Username: "alice", Password: "pw"}); err != nil {
		t.Fatal(err)
	}
	if ua.calls != 1 || ma.calls != 0 {
		t.Errorf("缺省应路由 system: user=%d member=%d", ua.calls, ma.calls)
	}
}

func TestLogin_IssuerNil_WrapsErrIssuance(t *testing.T) {
	aud := &stubAuditor{}
	ul := NewUnifiedLogin(UnifiedDeps{
		Cfg: newTestCfg("token"), Issuer: nil,
		UserAuth: &stubUserAuth{ident: &AuthIdentity{UserID: 3, AccountType: AccountTypeSystem}},
		Auditor:  aud,
	})
	_, _, err := ul.Login(context.Background(), LoginParams{Username: "a", Password: "b"})
	if !errors.Is(err, ErrIssuance) {
		t.Fatalf("Issuer nil 应 wrap ErrIssuance, got %v", err)
	}
	if len(aud.success) != 1 || aud.success[0] != false {
		t.Errorf("签发失败审计 = %v, want [false]", aud.success)
	}
}

func TestIssueFor_JwtMode_SystemOnly(t *testing.T) {
	jw := &stubJWTGen{tk: "jwt-token"}
	iss := okIssuer()
	ul := NewUnifiedLogin(UnifiedDeps{Cfg: newTestCfg("jwt"), Issuer: iss, JWTGen: jw})

	res, err := ul.IssueFor(context.Background(),
		&AuthIdentity{UserID: 3, Username: "alice", AccountType: AccountTypeSystem}, "alexgo-admin")
	if err != nil {
		t.Fatal(err)
	}
	if res.AccessToken != "jwt-token" || res.RefreshToken != "" || res.ExpiresIn != 0 {
		t.Errorf("jwt 模式 system 结果 = %+v", res)
	}
	if jw.calls != 1 || len(iss.calls) != 0 {
		t.Errorf("应只走 JWTGen: jwt=%d issuer=%d", jw.calls, len(iss.calls))
	}

	// D2：jwt 回滚只针对 system；member 仍签不透明令牌。
	res2, err := ul.IssueFor(context.Background(),
		&AuthIdentity{UserID: 9, Username: "m", AccountType: AccountTypeMember}, "alexgo-app")
	if err != nil {
		t.Fatal(err)
	}
	if res2.AccessToken != "a" || res2.RefreshToken != "r" {
		t.Errorf("member jwt 模式应不透明, got %+v", res2)
	}
	if len(iss.calls) != 1 || iss.calls[0].UserType != token.UserTypeMember {
		t.Errorf("issuer calls = %+v", iss.calls)
	}
}

// D9：机机恒不透明——jwt 模式也不走 JWTGen，refresh 强制空。
func TestIssueFor_ClientOpaque(t *testing.T) {
	jw := &stubJWTGen{}
	iss := okIssuer()
	ul := NewUnifiedLogin(UnifiedDeps{Cfg: newTestCfg("jwt"), Issuer: iss, JWTGen: jw})

	res, err := ul.IssueFor(context.Background(),
		&AuthIdentity{AccountType: AccountTypeClient, Username: "e2e-m2m"}, "e2e-m2m")
	if err != nil {
		t.Fatal(err)
	}
	if res.RefreshToken != "" {
		t.Errorf("机机不得带 refresh, got %q", res.RefreshToken)
	}
	if jw.calls != 0 {
		t.Errorf("client 不得走 JWTGen, calls=%d", jw.calls)
	}
	if len(iss.calls) != 1 {
		t.Fatalf("issuer calls = %d", len(iss.calls))
	}
	p := iss.calls[0]
	if p.UserType != token.UserTypeClient || p.UserID != 0 || p.TenantID != 0 || p.ClientID != "e2e-m2m" {
		t.Errorf("IssueParams = %+v", p)
	}
}

func TestIssueFor_IssueError_Wraps(t *testing.T) {
	iss := &stubIssuer{err: errors.New("db down")}
	ul := NewUnifiedLogin(UnifiedDeps{Cfg: newTestCfg("token"), Issuer: iss})
	_, err := ul.IssueFor(context.Background(),
		&AuthIdentity{UserID: 3, AccountType: AccountTypeSystem}, "alexgo-admin")
	if !errors.Is(err, ErrIssuance) {
		t.Fatalf("底层错误应 wrap ErrIssuance, got %v", err)
	}
}

func TestAuthorizeLogin_NoIssuance_AuditOnce(t *testing.T) {
	iss := okIssuer()
	aud := &stubAuditor{}
	ul := NewUnifiedLogin(UnifiedDeps{
		Cfg: newTestCfg("token"), Issuer: iss,
		UserAuth: &stubUserAuth{ident: &AuthIdentity{UserID: 3, Username: "alice", AccountType: AccountTypeSystem}},
		Auditor:  aud,
	})
	ident, err := ul.AuthorizeLogin(context.Background(), LoginParams{
		Username: "alice", Password: "pw", ClientID: "alexgo-admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ident.UserID != 3 {
		t.Errorf("ident = %+v", ident)
	}
	if len(iss.calls) != 0 {
		t.Errorf("AuthorizeLogin 不得签发, issuer calls=%d", len(iss.calls))
	}
	if len(aud.success) != 1 || aud.success[0] != true {
		t.Errorf("审计 = %v, want [true] 恰一次", aud.success)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `CGO_ENABLED=0 go test ./modules/system/oauth2/service/ -run 'TestLogin_|TestIssueFor_|TestAuthorizeLogin_' -v`
Expected: FAIL —— stub 的 `Login` 返回 `ErrAuthenticatorMissing`/`IssueFor` 返回 `ErrAuthenticatorMissing`，多数用例断言不过（如 `TestLogin_EmptyAccountTypeDefaultsSystem` 收到 `authenticator missing`）。

- [ ] **Step 3: 实现真身**

`modules/system/oauth2/service/login.go`：**删除 Task 4 留下的 stub**（`NewUnifiedLogin` 的占位构造与 `Login/AuthorizeLogin/IssueFor` 的占位方法），替换为：

```go
// === Task 8 真身（覆写 Task 4 stub；接口/哨兵/ProtocolError/UnifiedDeps 不动）===

type unifiedLogin struct {
	cfg        *config.Config
	issuer     token.Issuer
	captcha    *CaptchaService
	userAuth   UserAuthenticator
	memberAuth MemberAuthenticator
	auditor    LoginAuditor
	jwtGen     JWTGenerator
}

func NewUnifiedLogin(deps UnifiedDeps) UnifiedLogin {
	return &unifiedLogin{
		cfg:        deps.Cfg,
		issuer:     deps.Issuer,
		captcha:    deps.Captcha,
		userAuth:   deps.UserAuth,
		memberAuth: deps.MemberAuth,
		auditor:    deps.Auditor,
		jwtGen:     deps.JWTGen,
	}
}

// checkAndRoute 类型校验（先于 captcha 与审计）→ captcha → 按类型路由凭据校验。
// 不签发、不记审计——两者归 Login/AuthorizeLogin（每恰一次）。
func (u *unifiedLogin) checkAndRoute(ctx context.Context, p LoginParams) (*AuthIdentity, error) {
	at := p.AccountType
	if at == "" {
		at = AccountTypeSystem // 缺省 system（authorize/login 请求体 account_type 可选）
	}
	if at != AccountTypeSystem && at != AccountTypeMember {
		return nil, ErrAccountTypeUnsupported
	}
	if u.captcha != nil {
		if err := u.captcha.Verify(ctx, p.CaptchaUUID, p.CaptchaCode); err != nil {
			return nil, err
		}
	}
	switch at {
	case AccountTypeSystem:
		if u.userAuth == nil {
			return nil, ErrAuthenticatorMissing
		}
		return u.userAuth.Authenticate(ctx, p.Username, p.Password)
	default: // member
		if u.memberAuth == nil {
			return nil, ErrAuthenticatorMissing
		}
		return u.memberAuth.Authenticate(ctx, p.Username, p.Password, p.IP)
	}
}

// audit 恰一次记账（D13：msg 前缀 "oauth2: "；auditor nil 安全——图未接线时静默跳过）。
func (u *unifiedLogin) audit(ctx context.Context, p LoginParams, ident *AuthIdentity, success bool, cause error) {
	if u.auditor == nil {
		return
	}
	username, userID := p.Username, uint64(0)
	if ident != nil {
		username, userID = ident.Username, ident.UserID
	}
	msg := "oauth2: ok"
	if !success {
		msg = "oauth2: " + cause.Error()
	}
	_ = u.auditor.RecordLogin(ctx, username, userID, p.IP, p.UserAgent, success, msg)
}

func (u *unifiedLogin) Login(ctx context.Context, p LoginParams) (*LoginResult, *AuthIdentity, error) {
	ident, err := u.checkAndRoute(ctx, p)
	if err != nil {
		// ErrAccountTypeUnsupported 是请求形状错误，先于审计（无审计）；
		// 其余失败一律 RecordLogin(success=false)（spec §6）。
		if !errors.Is(err, ErrAccountTypeUnsupported) {
			u.audit(ctx, p, nil, false, err)
		}
		return nil, nil, err
	}
	res, ierr := u.IssueFor(ctx, ident, p.ClientID)
	if ierr != nil {
		u.audit(ctx, p, ident, false, ierr)
		return nil, nil, ierr
	}
	u.audit(ctx, p, ident, true, nil)
	return res, ident, nil
}

func (u *unifiedLogin) AuthorizeLogin(ctx context.Context, p LoginParams) (*AuthIdentity, error) {
	ident, err := u.checkAndRoute(ctx, p)
	if err != nil {
		if !errors.Is(err, ErrAccountTypeUnsupported) {
			u.audit(ctx, p, nil, false, err)
		}
		return nil, err
	}
	u.audit(ctx, p, ident, true, nil)
	return ident, nil
}

// IssueFor 按账号类型与模式签发（D2/D9 冻结语义）：
//   - client：恒不透明（jwt 模式也如此），refresh 强制空；
//   - system + mode=jwt：JWTGenerator，无 refresh、expires 0；
//   - system/member 其余：token.Issuer 按类型签不透明令牌。
// 任何签发失败 wrap ErrIssuance（协议端点 500 server_error 的映射依据）。
func (u *unifiedLogin) IssueFor(ctx context.Context, ident *AuthIdentity, clientID string) (*LoginResult, error) {
	if ident == nil {
		return nil, fmt.Errorf("%w: nil identity", ErrIssuance)
	}
	if u.cfg != nil && u.cfg.Auth.Mode == "jwt" && ident.AccountType == AccountTypeSystem {
		if u.jwtGen == nil {
			return nil, fmt.Errorf("%w: jwt generator not wired", ErrIssuance)
		}
		tk, err := u.jwtGen.Issue(ctx, ident.UserID, ident.Username, ident.TenantID)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrIssuance, err)
		}
		return &LoginResult{AccessToken: tk}, nil
	}
	if u.issuer == nil {
		return nil, fmt.Errorf("%w: issuer not wired", ErrIssuance)
	}
	ip := token.IssueParams{
		UserID:   ident.UserID,
		TenantID: ident.TenantID,
		ClientID: clientID,
	}
	switch ident.AccountType {
	case AccountTypeSystem:
		ip.UserType = token.UserTypeAdmin
	case AccountTypeMember:
		ip.UserType = token.UserTypeMember
	case AccountTypeClient:
		ip.UserType = token.UserTypeClient
		ip.UserID, ip.TenantID = 0, 0
	default:
		return nil, fmt.Errorf("%w: unknown account type %q", ErrIssuance, ident.AccountType)
	}
	issued, err := u.issuer.Issue(ctx, ip)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrIssuance, err)
	}
	res := &LoginResult{
		AccessToken:  issued.AccessToken,
		RefreshToken: issued.RefreshToken,
		ExpiresIn:    issued.ExpiresIn,
	}
	if ident.AccountType == AccountTypeClient {
		res.RefreshToken = "" // 机机无 refresh（D9），即便底层返回也强制清空
	}
	return res, nil
}
```

（`fmt`、`errors`、`context`、`go.uber.org/fx`、`alexGo-cloud/pkg/config`、`alexGo-cloud/pkg/token` 的 import Task 4 已备好。）

`modules/system/oauth2/module.go` 的 `fx.Provide` 块追加：

```go
		oauth2svc.NewUnifiedLogin,
```

- [ ] **Step 4: 运行确认通过**

Run: `CGO_ENABLED=0 go test ./modules/system/oauth2/... -v && CGO_ENABLED=0 go test ./alexgo-server/cmd/ ./modules/member/cmd/`
Expected: PASS（新增 10 例全绿；两个 fx 干跑图仍可满足——`UnifiedDeps` 的必填项 `Cfg/Issuer/Captcha` 均已 Provide，其余 optional）。

- [ ] **Step 5: Commit**

```bash
git add modules/system/oauth2/
git commit -m "feat(oauth2): UnifiedLogin 真身（类型校验→captcha→路由→签发→恰一次审计）"
```

---

### Task 9: AuthorizeCodeService + GET /api/oauth2/authorize + module.go 具体化

**Files:**
- Create: `modules/system/oauth2/repository/authorize_code.go`
- Create: `modules/system/oauth2/service/authorize_code.go`
- Test: `modules/system/oauth2/service/authorize_code_test.go`
- Modify: `modules/system/oauth2/controller/protocol.go`（ctor 换 `ProtocolDeps`、加 GET /oauth2/authorize、`protocolErr` helper）
- Modify: `modules/system/oauth2/controller/protocol_test.go`（整文件重写：新 ctor 夹具 + captcha 回归 + authorize 四例）
- Modify: `modules/system/oauth2/module.go`（**整文件重写**为 infra 模式的具体实现）

**Interfaces:**
- Consumes: Task 2 的 `model.Oauth2AuthorizeCode`（含 `OwnerTenantID`/`CodeChallenge`/`CodeChallengeMethod`/`Revoked`）与 `model.Oauth2Client.AllowsGrant/AllowsRedirect`；Task 7 的 `ClientService.Get`、`ErrInvalidClient`；Task 3 的 `NewProtocolController` 同名 ctor（本任务换参数类型为 `ProtocolDeps`，此后 Task 11/13 只加字段不再改签名）；`cfg.Auth.LoginPage`（Task 1，默认 `/auth/login`）。
- Produces（Task 10/11 消费，逐字）：

```go
package repository

func NewAuthorizeCodeRepository(db *gorm.DB) *AuthorizeCodeRepository
func (r *AuthorizeCodeRepository) Create(ctx context.Context, row *model.Oauth2AuthorizeCode) error
func (r *AuthorizeCodeRepository) GetByCode(ctx context.Context, code string) (*model.Oauth2AuthorizeCode, error)
// MarkRevoked CAS 单次消费：UPDATE revoked=1 WHERE code=? AND revoked=0；
// 返回 RowsAffected>0（false = 已被首次尝试消费，含并发重放）。
func (r *AuthorizeCodeRepository) MarkRevoked(ctx context.Context, code string) (bool, error)

package oauth2svc

const CodeTTL = 2 * time.Minute // 授权码 TTL（D15 补充语义）
var ErrCodeInvalid = errors.New("authorize code invalid")

type IssueCodeParams struct {
	ClientID            string
	UserID              uint64
	Username            string
	AccountType         string
	TenantID            uint64
	RedirectURI         string
	Scopes              string
	CodeChallenge       string
	CodeChallengeMethod string
}
type ExchangeParams struct {
	Code         string
	ClientID     string
	RedirectURI  string
	CodeVerifier string
}
func NewAuthorizeCodeService(repo *repository.AuthorizeCodeRepository) *AuthorizeCodeService
func (s *AuthorizeCodeService) Issue(ctx context.Context, p IssueCodeParams) (string, error)
// Exchange：载入→**MarkRevoked 先行**（false→ErrCodeInvalid，首错即消费）→
// 过期/client/redirect/PKCE 校验，任何失败 → ErrCodeInvalid；成功返回 row。
func (s *AuthorizeCodeService) Exchange(ctx context.Context, p ExchangeParams) (*model.Oauth2AuthorizeCode, error)

package controller

type ProtocolDeps struct {
	fx.In
	Captcha *oauth2svc.CaptchaService
	Codes   *oauth2svc.AuthorizeCodeService
	Clients *oauth2svc.ClientService
	Cfg     *config.Config
	// Task 11 追加：Login oauth2svc.UnifiedLogin; Tokens *oauth2svc.TokenService
	// Task 13 追加：Validator token.Validator `optional:"true"`
}
func NewProtocolController(deps ProtocolDeps) *ProtocolController
```

- [ ] **Step 1: 写失败测试（service 层）**

`modules/system/oauth2/service/authorize_code_test.go`：

```go
package oauth2svc

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/system/oauth2/model"
	"alexGo-cloud/modules/system/oauth2/repository"
)

func newTestCodes(t *testing.T) (*AuthorizeCodeService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Oauth2AuthorizeCode{}); err != nil {
		t.Fatal(err)
	}
	return NewAuthorizeCodeService(repository.NewAuthorizeCodeRepository(db)), db
}

func baseCodeParams() IssueCodeParams {
	return IssueCodeParams{
		ClientID: "spa", UserID: 3, Username: "alice",
		AccountType: AccountTypeSystem, TenantID: 0,
		RedirectURI: "http://localhost:5173/callback", Scopes: "all",
	}
}

func TestExchangeCode_OK(t *testing.T) {
	svc, _ := newTestCodes(t)
	code, err := svc.Issue(context.Background(), baseCodeParams())
	if err != nil {
		t.Fatal(err)
	}
	if code == "" {
		t.Fatal("Issue 返回空 code")
	}
	row, err := svc.Exchange(context.Background(), ExchangeParams{
		Code: code, ClientID: "spa", RedirectURI: "http://localhost:5173/callback",
	})
	if err != nil {
		t.Fatal(err)
	}
	if row.UserID != 3 || row.Username != "alice" || row.AccountType != AccountTypeSystem {
		t.Errorf("row = %+v", row)
	}
	// 成功消费后重放 → 拒
	if _, err := svc.Exchange(context.Background(), ExchangeParams{
		Code: code, ClientID: "spa", RedirectURI: "http://localhost:5173/callback",
	}); !errors.Is(err, ErrCodeInvalid) {
		t.Fatalf("重放应 ErrCodeInvalid, got %v", err)
	}
}

func TestExchangeCode_Expired(t *testing.T) {
	svc, db := newTestCodes(t)
	code, err := svc.Issue(context.Background(), baseCodeParams())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Oauth2AuthorizeCode{}).
		Where("code = ?", code).
		Update("expires_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Exchange(context.Background(), ExchangeParams{
		Code: code, ClientID: "spa", RedirectURI: "http://localhost:5173/callback",
	}); !errors.Is(err, ErrCodeInvalid) {
		t.Fatalf("过期应 ErrCodeInvalid, got %v", err)
	}
}

// Review Focus #4：首次尝试即消费——即便首次参数错，第二次全对也拒（CAS）。
func TestExchangeCode_RevokedAfterFirstAttempt(t *testing.T) {
	svc, _ := newTestCodes(t)
	code, err := svc.Issue(context.Background(), baseCodeParams())
	if err != nil {
		t.Fatal(err)
	}
	// 第一次：client 不匹配 → 拒，且已消费
	if _, err := svc.Exchange(context.Background(), ExchangeParams{
		Code: code, ClientID: "other", RedirectURI: "http://localhost:5173/callback",
	}); !errors.Is(err, ErrCodeInvalid) {
		t.Fatalf("client 不匹配应 ErrCodeInvalid, got %v", err)
	}
	// 第二次：参数全对也拒
	if _, err := svc.Exchange(context.Background(), ExchangeParams{
		Code: code, ClientID: "spa", RedirectURI: "http://localhost:5173/callback",
	}); !errors.Is(err, ErrCodeInvalid) {
		t.Fatalf("首错后应已消费, got %v", err)
	}
}

func TestExchangeCode_RedirectMismatch(t *testing.T) {
	svc, _ := newTestCodes(t)
	code, err := svc.Issue(context.Background(), baseCodeParams())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Exchange(context.Background(), ExchangeParams{
		Code: code, ClientID: "spa", RedirectURI: "http://evil.example/steal",
	}); !errors.Is(err, ErrCodeInvalid) {
		t.Fatalf("redirect 不匹配应 ErrCodeInvalid, got %v", err)
	}
}

func TestExchangeCode_PKCE(t *testing.T) {
	svc, _ := newTestCodes(t)
	verifier := "correct-verifier-0123456789"
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	p := baseCodeParams()
	p.CodeChallenge, p.CodeChallengeMethod = challenge, "S256"
	code, err := svc.Issue(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	// 错 verifier（首错即消费，后续全对也拒——本例只验错拒）
	if _, err := svc.Exchange(context.Background(), ExchangeParams{
		Code: code, ClientID: "spa", RedirectURI: "http://localhost:5173/callback",
		CodeVerifier: "wrong-verifier",
	}); !errors.Is(err, ErrCodeInvalid) {
		t.Fatalf("错 verifier 应 ErrCodeInvalid, got %v", err)
	}
	// 空 verifier
	code2, _ := svc.Issue(context.Background(), p)
	if _, err := svc.Exchange(context.Background(), ExchangeParams{
		Code: code2, ClientID: "spa", RedirectURI: "http://localhost:5173/callback",
	}); !errors.Is(err, ErrCodeInvalid) {
		t.Fatalf("空 verifier 应 ErrCodeInvalid, got %v", err)
	}
	// 正确 verifier
	code3, _ := svc.Issue(context.Background(), p)
	if _, err := svc.Exchange(context.Background(), ExchangeParams{
		Code: code3, ClientID: "spa", RedirectURI: "http://localhost:5173/callback",
		CodeVerifier: verifier,
	}); err != nil {
		t.Fatalf("正确 verifier 应过, got %v", err)
	}
	// method 非 S256：Exchange 也拒（authorize 侧已 400，双保险）
	p2 := baseCodeParams()
	p2.CodeChallenge, p2.CodeChallengeMethod = challenge, "plain"
	code4, _ := svc.Issue(context.Background(), p2)
	if _, err := svc.Exchange(context.Background(), ExchangeParams{
		Code: code4, ClientID: "spa", RedirectURI: "http://localhost:5173/callback",
		CodeVerifier: verifier,
	}); !errors.Is(err, ErrCodeInvalid) {
		t.Fatalf("method plain 应 ErrCodeInvalid, got %v", err)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `CGO_ENABLED=0 go test ./modules/system/oauth2/service/ -run TestExchangeCode -v`
Expected: FAIL —— `undefined: NewAuthorizeCodeService`（编译错误）。

- [ ] **Step 3: 实现 repo + service**

`modules/system/oauth2/repository/authorize_code.go`：

```go
package repository

import (
	"context"

	"gorm.io/gorm"

	"alexGo-cloud/modules/system/oauth2/model"
)

// AuthorizeCodeRepository 授权码访问层（D4：OwnerTenantID → tenant_id 列）。
type AuthorizeCodeRepository struct {
	db *gorm.DB
}

func NewAuthorizeCodeRepository(db *gorm.DB) *AuthorizeCodeRepository {
	return &AuthorizeCodeRepository{db: db}
}

func (r *AuthorizeCodeRepository) Create(ctx context.Context, row *model.Oauth2AuthorizeCode) error {
	return r.db.WithContext(ctx).Create(row).Error
}

func (r *AuthorizeCodeRepository) GetByCode(ctx context.Context, code string) (*model.Oauth2AuthorizeCode, error) {
	var row model.Oauth2AuthorizeCode
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// MarkRevoked CAS 单次消费：只有未消费的行能翻转成功。
// 返回 false（RowsAffected=0）= 已被首次尝试消费——授权码「首错即消费」的落点。
func (r *AuthorizeCodeRepository) MarkRevoked(ctx context.Context, code string) (bool, error) {
	res := r.db.WithContext(ctx).
		Model(&model.Oauth2AuthorizeCode{}).
		Where("code = ? AND revoked = 0", code).
		Update("revoked", true)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}
```

`modules/system/oauth2/service/authorize_code.go`：

```go
package oauth2svc

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"alexGo-cloud/modules/system/oauth2/model"
	"alexGo-cloud/modules/system/oauth2/repository"
)

// CodeTTL 授权码有效期 2 分钟（D15 补充语义）。
const CodeTTL = 2 * time.Minute

// ErrCodeInvalid 授权码过期/已消费/client 或 redirect 不匹配/PKCE 失败——
// 统一对外 400 invalid_grant（不区分原因，防探测）。
var ErrCodeInvalid = errors.New("authorize code invalid")

type IssueCodeParams struct {
	ClientID            string
	UserID              uint64
	Username            string
	AccountType         string
	TenantID            uint64
	RedirectURI         string
	Scopes              string
	CodeChallenge       string
	CodeChallengeMethod string
}

type ExchangeParams struct {
	Code         string
	ClientID     string
	RedirectURI  string
	CodeVerifier string
}

type AuthorizeCodeService struct {
	repo *repository.AuthorizeCodeRepository
}

func NewAuthorizeCodeService(repo *repository.AuthorizeCodeRepository) *AuthorizeCodeService {
	return &AuthorizeCodeService{repo: repo}
}

// Issue 生成授权码：32B crypto/rand → base64url（无填充，D15 补充语义）。
func (s *AuthorizeCodeService) Issue(ctx context.Context, p IssueCodeParams) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("authorize code rand: %w", err)
	}
	code := base64.RawURLEncoding.EncodeToString(buf)
	row := &model.Oauth2AuthorizeCode{
		Code:                code,
		ClientID:            p.ClientID,
		UserID:              p.UserID,
		Username:            p.Username,
		AccountType:         p.AccountType,
		OwnerTenantID:       p.TenantID,
		RedirectURI:         p.RedirectURI,
		Scopes:              p.Scopes,
		CodeChallenge:       p.CodeChallenge,
		CodeChallengeMethod: p.CodeChallengeMethod,
		ExpiresAt:           time.Now().Add(CodeTTL),
		Revoked:             false,
	}
	if err := s.repo.Create(ctx, row); err != nil {
		return "", fmt.Errorf("authorize code store: %w", err)
	}
	return code, nil
}

// Exchange 换发校验。顺序固定：
// 载入 → **MarkRevoked 先行**（false → ErrCodeInvalid；首次尝试即消费，无论后续校验对错，
// CAS 单次消费，D15 补充语义）→ 过期 → client 归属 → redirect 精确匹配 → PKCE。
// 任何失败统一 ErrCodeInvalid。
func (s *AuthorizeCodeService) Exchange(ctx context.Context, p ExchangeParams) (*model.Oauth2AuthorizeCode, error) {
	row, err := s.repo.GetByCode(ctx, p.Code)
	if err != nil || row == nil {
		return nil, ErrCodeInvalid
	}
	ok, err := s.repo.MarkRevoked(ctx, p.Code)
	if err != nil {
		return nil, fmt.Errorf("authorize code revoke: %w", err)
	}
	if !ok {
		return nil, ErrCodeInvalid // 已被首次尝试消费（含并发重放）
	}
	if time.Now().After(row.ExpiresAt) {
		return nil, ErrCodeInvalid
	}
	if row.ClientID != p.ClientID {
		return nil, ErrCodeInvalid
	}
	if row.RedirectURI != p.RedirectURI || p.RedirectURI == "" {
		return nil, ErrCodeInvalid // 精确匹配，失效/篡改一律拒（D15 补充语义）
	}
	if row.CodeChallenge != "" {
		if row.CodeChallengeMethod != "S256" {
			return nil, ErrCodeInvalid
		}
		if p.CodeVerifier == "" {
			return nil, ErrCodeInvalid
		}
		sum := sha256.Sum256([]byte(p.CodeVerifier))
		if base64.RawURLEncoding.EncodeToString(sum[:]) != row.CodeChallenge {
			return nil, ErrCodeInvalid
		}
	}
	return row, nil
}
```

- [ ] **Step 4: 运行确认通过**

Run: `CGO_ENABLED=0 go test ./modules/system/oauth2/service/ -run TestExchangeCode -v`
Expected: PASS（5 例全绿）。

- [ ] **Step 5: 写失败测试（HTTP 层）**

`modules/system/oauth2/controller/protocol_test.go` **整文件重写**（新 ctor 夹具 + Task 3 的 captcha 回归用例迁到新夹具 + authorize 四例）：

```go
package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/system/oauth2/model"
	"alexGo-cloud/modules/system/oauth2/repository"
	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
	"alexGo-cloud/pkg/config"
)

// newProtoRouter 搭 /api 组下的协议端点测试路由（Task 9/11/13 共用夹具）。
// cfg nil → 默认 LoginPage=/auth/login。返回 sqlite 上的真实 Clients/Codes 供种子。
func newProtoRouter(t *testing.T, cfg *config.Config) (*gin.Engine, *oauth2svc.ClientService, *oauth2svc.AuthorizeCodeService) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Oauth2Client{}, &model.Oauth2AuthorizeCode{}); err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		cfg = &config.Config{}
		cfg.Auth.LoginPage = "/auth/login"
	}
	clients := oauth2svc.NewClientService(repository.NewClientRepository(db))
	codes := oauth2svc.NewAuthorizeCodeService(repository.NewAuthorizeCodeRepository(db))
	proto := NewProtocolController(ProtocolDeps{
		Captcha: oauth2svc.NewCaptchaService(oauth2svc.CaptchaDeps{Cfg: cfg}),
		Codes:   codes,
		Clients: clients,
		Cfg:     cfg,
	})
	r := gin.New()
	g := r.Group("/api")
	proto.Register(g)
	return r, clients, codes
}

func seedSpaClient(t *testing.T, clients *oauth2svc.ClientService) {
	t.Helper()
	if err := clients.Create(context.Background(), &model.Oauth2Client{
		ClientID: "spa", ClientName: "SPA",
		GrantTypes:    "authorization_code,password,refresh_token",
		RedirectURIs:  "http://localhost:5173/callback",
		AccountTypes:  "system",
	}); err != nil {
		t.Fatal(err)
	}
}

// Task 3 captcha 端点回归（迁到新夹具）。
func TestCaptchaImage_OK(t *testing.T) {
	r, _, _ := newProtoRouter(t, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/oauth2/captcha", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "uuid") || !strings.Contains(body, "image") {
		t.Errorf("captcha 响应缺字段: %s", body)
	}
}

func TestAuthorize_OK_302LoginPage(t *testing.T) {
	r, clients, _ := newProtoRouter(t, nil)
	seedSpaClient(t, clients)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/api/oauth2/authorize?client_id=spa&response_type=code&state=xyz&scope=all&redirect_uri="+
			url.QueryEscape("http://localhost:5173/callback"), nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	loc := w.Header().Get("Location")
	if !strings.HasPrefix(loc, "/auth/login?") {
		t.Errorf("Location = %q, 应指登录页", loc)
	}
	for _, want := range []string{"client_id=spa", "state=xyz", "response_type=code"} {
		if !strings.Contains(loc, want) {
			t.Errorf("Location 缺回带参数 %q: %q", want, loc)
		}
	}
}

// Review Focus #1：redirect_uri 篡改 → 400 且绝不带 Location。
func TestAuthorize_Validate_RedirectMismatch_No302(t *testing.T) {
	r, clients, _ := newProtoRouter(t, nil)
	seedSpaClient(t, clients)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/api/oauth2/authorize?client_id=spa&response_type=code&redirect_uri="+
			url.QueryEscape("http://evil.example/steal"), nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "" {
		t.Errorf("篡改 redirect 不得带 Location, got %q", loc)
	}
	if !strings.Contains(w.Body.String(), `"error":"invalid_request"`) {
		t.Errorf("body = %s", w.Body.String())
	}
}

func TestAuthorize_UnknownClient_400(t *testing.T) {
	r, _, _ := newProtoRouter(t, nil)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/api/oauth2/authorize?client_id=nope&response_type=code&redirect_uri="+
			url.QueryEscape("http://localhost:5173/callback"), nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "invalid_client") {
		t.Errorf("code=%d body=%s", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc != "" {
		t.Errorf("不得带 Location, got %q", loc)
	}
}

func TestGrant_NotAllowed_400(t *testing.T) {
	r, clients, _ := newProtoRouter(t, nil)
	if err := clients.Create(context.Background(), &model.Oauth2Client{
		ClientID: "pw-only", GrantTypes: "password",
		RedirectURIs: "http://localhost:5173/callback", AccountTypes: "system",
	}); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/api/oauth2/authorize?client_id=pw-only&response_type=code&redirect_uri="+
			url.QueryEscape("http://localhost:5173/callback"), nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "unauthorized_client") {
		t.Errorf("code=%d body=%s", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc != "" {
		t.Errorf("不得带 Location, got %q", loc)
	}
}
```

- [ ] **Step 6: 运行确认失败**

Run: `CGO_ENABLED=0 go test ./modules/system/oauth2/controller/ -v`
Expected: FAIL —— `undefined: ProtocolDeps`（ctor 尚未换签名）、`NewProtocolController` 参数不匹配（编译错误）。

- [ ] **Step 7: 实现 controller + module 具体化**

`modules/system/oauth2/controller/protocol.go` **整文件重写**：

```go
package controller

import (
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"alexGo-cloud/modules/system/oauth2/model"
	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
	"alexGo-cloud/pkg/config"
)

// ProtocolDeps 协议控制器依赖（Task 9 起 fx.In 结构体 ctor——
// Task 11 加 Login/Tokens、Task 13 加 Validator，ctor 签名不再变）。
type ProtocolDeps struct {
	fx.In
	Captcha *oauth2svc.CaptchaService
	Codes   *oauth2svc.AuthorizeCodeService
	Clients *oauth2svc.ClientService
	Cfg     *config.Config
}

// ProtocolController 协议端点（RFC 6749 信封 {"error","error_description"}，
// 与 admin 的 {error}/{data,total} 信封区分——spec §6）。
type ProtocolController struct {
	deps ProtocolDeps
}

func NewProtocolController(deps ProtocolDeps) *ProtocolController {
	return &ProtocolController{deps: deps}
}

// Register 挂 /oauth2/*（module 传入的分组已带 /api 前缀）。
func (p *ProtocolController) Register(r gin.IRouter) {
	r.GET("/oauth2/captcha", p.captchaImage)
	r.GET("/oauth2/authorize", p.authorize)
}

// protocolErr 写 RFC 6749 错误信封；不设 Location（400 绝不 302 的结构保证）。
func (p *ProtocolController) protocolErr(c *gin.Context, status int, code, desc string) {
	c.JSON(status, gin.H{"error": code, "error_description": desc})
}

func (p *ProtocolController) captchaImage(c *gin.Context) {
	ch, err := p.deps.Captcha.NewChallenge(c.Request.Context())
	if err != nil {
		p.protocolErr(c, http.StatusInternalServerError, "server_error", "验证码生成失败")
		return
	}
	c.JSON(http.StatusOK, ch)
}

// authorize 授权码入口（spec §4.2 ①）：
// 缺参 → 400 invalid_request；Get 失败 → 400 invalid_client；
// grant 白名单 → 400 unauthorized_client；redirect 精确匹配失败 → 400 invalid_request；
// response_type != code → 400 unsupported_response_type；
// challenge 非空且 method != S256 → 400 invalid_request。
// **以上全部无 Location（失效/篡改一律 400 绝不 302，D15 补充语义）。**
// 全部通过 → 302 cfg.Auth.LoginPage?client_id&redirect_uri&state&code_challenge&...
func (p *ProtocolController) authorize(c *gin.Context) {
	q := c.Request.URL.Query()
	clientID := q.Get("client_id")
	redirectURI := q.Get("redirect_uri")
	responseType := q.Get("response_type")
	if clientID == "" || redirectURI == "" || responseType == "" {
		p.protocolErr(c, http.StatusBadRequest, "invalid_request", "缺少 client_id/redirect_uri/response_type")
		return
	}
	ctx := c.Request.Context()
	client, err := p.deps.Clients.Get(ctx, clientID)
	if err != nil {
		p.protocolErr(c, http.StatusBadRequest, "invalid_client", "未知或停用的 client")
		return
	}
	if !client.AllowsGrant("authorization_code") {
		p.protocolErr(c, http.StatusBadRequest, "unauthorized_client", "client 未获授权码许可")
		return
	}
	if !client.AllowsRedirect(redirectURI) {
		p.protocolErr(c, http.StatusBadRequest, "invalid_request", "redirect_uri 不在白名单")
		return
	}
	if responseType != "code" {
		p.protocolErr(c, http.StatusBadRequest, "unsupported_response_type", "仅支持 response_type=code")
		return
	}
	challenge := q.Get("code_challenge")
	method := q.Get("code_challenge_method")
	if challenge != "" && method != "S256" {
		p.protocolErr(c, http.StatusBadRequest, "invalid_request", "PKCE 仅支持 S256")
		return
	}
	loginPage := p.deps.Cfg.Auth.LoginPage
	if loginPage == "" {
		loginPage = "/auth/login"
	}
	vals := url.Values{}
	vals.Set("client_id", clientID)
	vals.Set("redirect_uri", redirectURI)
	vals.Set("response_type", responseType)
	for _, k := range []string{"state", "scope", "code_challenge", "code_challenge_method"} {
		if v := q.Get(k); v != "" {
			vals.Set(k, v)
		}
	}
	c.Redirect(http.StatusFound, loginPage+"?"+vals.Encode())
}
```

（`model` import 实际未用则删——`client` 由 `Get` 返回，只调方法不命名类型；实现时按编译器提示收敛 import。）

`modules/system/oauth2/module.go` **整文件重写**（照抄 `modules/system/infra/module.go` 的 fx 范式——迁移源必须带 `ResultTags` 进 `group:"migration_sources"`，否则生产启动不跑 oauth2 迁移）：

```go
package oauth2

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"alexGo-cloud/alexgo-server/server"
	"alexGo-cloud/modules/system/oauth2/controller"
	"alexGo-cloud/modules/system/oauth2/repository"
	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
	"alexGo-cloud/pkg/migrate"
)

// FxModule oauth2 模块装配（D7：硬编码进两个 main.go）。
// 照抄 modules/system/infra/module.go：迁移源注解进 group:"migration_sources"，
// NewModule 注解进 group:"modules"（ResultTags 不是 ParamTags）。
var FxModule = fx.Module("oauth2",
	fx.Provide(
		fx.Annotate(
			NewMigrationSource,
			fx.As(new(migrate.Source)),
			fx.ResultTags(`group:"migration_sources"`),
		),
		oauth2svc.NewCaptchaService,
		repository.NewClientRepository,
		repository.NewAuthorizeCodeRepository,
		oauth2svc.NewClientService,
		oauth2svc.NewAuthorizeCodeService,
		oauth2svc.NewUnifiedLogin,
		controller.NewProtocolController,
		fx.Annotate(
			NewModule,
			fx.As(new(server.Module)),
			fx.ResultTags(`group:"modules"`),
		),
	),
)

type oauth2Module struct {
	proto *controller.ProtocolController
}

// NewModule 聚合控制器，产出 server.Module（挂进 group:"modules"）。
func NewModule(proto *controller.ProtocolController) server.Module {
	return &oauth2Module{proto: proto}
}

// RegisterRoutes 入参是 /api group（见 server/http.go，infra 同款），
// 故路径写相对前缀 /oauth2/*。
func (m *oauth2Module) RegisterRoutes(r *gin.RouterGroup) {
	m.proto.Register(r)
}
```

> 若 `alexgo-server/server` 的 `Module` 接口方法名与 `RegisterRoutes` 签名有出入，**打开 `modules/system/infra/module.go` 与 `alexgo-server/server/http.go` 逐行对齐**——infra 是唯一权威模板，不新发明。Task 2/3 期的过渡版 FxModule（注解形态已同 infra，仅 Provide 面更小）在本步被整体替换。

- [ ] **Step 8: 运行确认通过**

Run: `CGO_ENABLED=0 go test ./modules/system/oauth2/... -v && CGO_ENABLED=0 go test ./alexgo-server/cmd/ ./modules/member/cmd/`
Expected: PASS（service 5 例 + controller 6 例 + 两 main fx 干跑图可满足——新增 Provide 的依赖 `*gorm.DB`/`*config.Config` 基座已有，`migrate.Source` 进组后 `migrate.NewRunner` 的 `group:"migration_sources"` 能收到）。

- [ ] **Step 9: Commit**

```bash
git add modules/system/oauth2/
git commit -m "feat(oauth2): 授权码签发/消费（首错即消费 CAS）+ GET /authorize + module 具体化"
```

---

### Task 10: TokenService（四 grant 状态机 + LoginProtoErr）

**Files:**
- Create: `modules/system/oauth2/service/token.go`
- Test: `modules/system/oauth2/service/token_test.go`
- （module.go 本任务不动——`NewTokenService` 的 Provide 在 Task 11 随控制器依赖一起接）

**Interfaces:**
- Consumes: Task 7 `ClientService.Authenticate/Get` + `model.Oauth2Client.AllowsGrant/AllowsAccountType` + `ErrInvalidClient`；Task 9 `AuthorizeCodeService.Exchange` + `ErrCodeInvalid` + `model.Oauth2AuthorizeCode`（`OwnerTenantID/Username/AccountType`）；Task 8 `UnifiedLogin.Login/IssueFor` + 五哨兵 + `ProtocolError` + `LoginResult/AuthIdentity`；`token.Issuer.Refresh`；`cfg.Auth.Mode`。
- Produces（Task 11 消费，逐字）：

```go
package oauth2svc

const (
	GrantAuthorizationCode = "authorization_code"
	GrantPassword          = "password"
	GrantClientCredentials = "client_credentials"
	GrantRefreshToken      = "refresh_token"
)

// LoginProtoErr 统一登录错误 → RFC 6749 协议错误（控制器复用）。
// ErrInvalidCredentials→400 invalid_grant「账号或密码错误」；
// ErrUserDisabled→400 invalid_grant「账号已停用」；ErrCaptchaInvalid→400 captcha_invalid；
// ErrIssuance|ErrAuthenticatorMissing→500 server_error「签发服务不可用」；
// ErrAccountTypeUnsupported→400 invalid_request；其余→500 server_error。
func LoginProtoErr(err error) *ProtocolError

type TokenRequest struct {
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Code         string `json:"code"`
	RedirectURI  string `json:"redirect_uri"`
	CodeVerifier string `json:"code_verifier"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	AccountType  string `json:"account_type"`
	RefreshToken string `json:"refresh_token"`
	CaptchaUUID  string `json:"captcha_uuid"`
	CaptchaCode  string `json:"captcha_code"`
	IP           string `json:"-"` // controller 填充
	UserAgent    string `json:"-"`
}
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"` // 恒 "Bearer"
}
type TokenServiceDeps struct {
	fx.In
	Cfg     *config.Config
	Clients *ClientService
	Codes   *AuthorizeCodeService
	Login   UnifiedLogin
	Issuer  token.Issuer
}
func NewTokenService(deps TokenServiceDeps) *TokenService
func (s *TokenService) Exchange(ctx context.Context, req *TokenRequest) (*TokenResponse, *ProtocolError)
```

- [ ] **Step 1: 写失败测试**

`modules/system/oauth2/service/token_test.go`（复用同包 `okIssuer`/`stubUnified` 需本文件自带——`stubUnified` 为本任务新夹具）：

```go
package oauth2svc

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/system/oauth2/model"
	"alexGo-cloud/modules/system/oauth2/repository"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/token"
)

// stubUnified TokenService 视角的 UnifiedLogin 桩：Login 与 IssueFor 共用 res/err，
// 记账供断言（Review Focus #3：账号类型白名单落点）。
type stubUnified struct {
	res    *LoginResult
	err    error
	logins []LoginParams
	issued []issueCall
}

type issueCall struct {
	ident    *AuthIdentity
	clientID string
}

func (s *stubUnified) Login(_ context.Context, p LoginParams) (*LoginResult, *AuthIdentity, error) {
	s.logins = append(s.logins, p)
	return s.res, s.ident(), s.err
}
func (s *stubUnified) ident() *AuthIdentity { return &AuthIdentity{UserID: 3, Username: p(s), AccountType: AccountTypeSystem} }
func (s *stubUnified) AuthorizeLogin(_ context.Context, p LoginParams) (*AuthIdentity, error) {
	s.logins = append(s.logins, p)
	if s.err != nil {
		return nil, s.err
	}
	return &AuthIdentity{UserID: 3, Username: p.Username, AccountType: p.AccountType}, nil
}
func (s *stubUnified) IssueFor(_ context.Context, ident *AuthIdentity, clientID string) (*LoginResult, error) {
	s.issued = append(s.issued, issueCall{ident: ident, clientID: clientID})
	return s.res, s.err
}

// refreshIssuer 在 stubIssuer（unified_login_test.go 同包）基础上补 Refresh。
type refreshIssuer struct {
	stubIssuer
	refreshed  *token.Issued
	refreshErr error
	refreshN   int
}

func (s *refreshIssuer) Refresh(_ context.Context, _ string) (*token.Issued, error) {
	s.refreshN++
	if s.refreshErr != nil {
		return nil, s.refreshErr
	}
	return s.refreshed, nil
}

func newTestTokenSvc(t *testing.T, mode string) (*TokenService, *ClientService, *AuthorizeCodeService, *stubUnified, *refreshIssuer) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Oauth2Client{}, &model.Oauth2AuthorizeCode{}); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Auth.Mode = mode
	clients := NewClientService(repository.NewClientRepository(db))
	codes := NewAuthorizeCodeService(repository.NewAuthorizeCodeRepository(db))
	ul := &stubUnified{res: &LoginResult{AccessToken: "a", RefreshToken: "r", ExpiresIn: 60}}
	iss := &refreshIssuer{refreshed: &token.Issued{AccessToken: "a2", RefreshToken: "r2", ExpiresIn: 60}}
	svc := NewTokenService(TokenServiceDeps{Cfg: cfg, Clients: clients, Codes: codes, Login: ul, Issuer: iss})
	return svc, clients, codes, ul, iss
}

func seedAdminClient(t *testing.T, clients *ClientService) {
	t.Helper()
	if err := clients.Create(context.Background(), &model.Oauth2Client{
		ClientID: "alexgo-admin",
		GrantTypes: "authorization_code,password,refresh_token",
		AccountTypes: "system",
		RedirectURIs: "http://localhost:5173/callback",
	}); err != nil {
		t.Fatal(err)
	}
}

func TestExchange_Password_OK(t *testing.T) {
	svc, clients, _, ul, _ := newTestTokenSvc(t, "token")
	seedAdminClient(t, clients)
	res, perr := svc.Exchange(context.Background(), &TokenRequest{
		GrantType: GrantPassword, ClientID: "alexgo-admin",
		Username: "alice", Password: "pw",
	})
	if perr != nil {
		t.Fatalf("perr = %v", perr)
	}
	if res.AccessToken != "a" || res.TokenType != "Bearer" {
		t.Errorf("res = %+v", res)
	}
	if len(ul.logins) != 1 {
		t.Fatalf("logins = %d", len(ul.logins))
	}
	lp := ul.logins[0]
	if lp.AccountType != AccountTypeSystem || lp.ClientID != "alexgo-admin" {
		t.Errorf("login params = %+v", lp)
	}
}

// Review Focus #3：client 账号类型白名单——admin client 换不了 member token。
func TestExchange_Password_AccountTypeNotAllowed(t *testing.T) {
	svc, clients, _, ul, _ := newTestTokenSvc(t, "token")
	seedAdminClient(t, clients)
	res, perr := svc.Exchange(context.Background(), &TokenRequest{
		GrantType: GrantPassword, ClientID: "alexgo-admin",
		Username: "13800000000", Password: "pw", AccountType: AccountTypeMember,
	})
	if res != nil {
		t.Errorf("应拒, res = %+v", res)
	}
	if perr == nil || perr.Status != 400 || perr.Code != "unauthorized_client" {
		t.Fatalf("perr = %+v", perr)
	}
	if len(ul.logins) != 0 {
		t.Errorf("白名单拒后不得进登录, logins=%d", len(ul.logins))
	}
}

func TestExchange_Password_UnsupportedGrant(t *testing.T) {
	svc, clients, _, _, _ := newTestTokenSvc(t, "token")
	if err := clients.Create(context.Background(), &model.Oauth2Client{
		ClientID: "code-only", GrantTypes: "authorization_code", AccountTypes: "system",
		RedirectURIs: "http://localhost:5173/callback",
	}); err != nil {
		t.Fatal(err)
	}
	_, perr := svc.Exchange(context.Background(), &TokenRequest{
		GrantType: GrantPassword, ClientID: "code-only", Username: "a", Password: "b",
	})
	if perr == nil || perr.Status != 400 || perr.Code != "unsupported_grant_type" {
		t.Fatalf("perr = %+v", perr)
	}
}

func TestExchange_ClientAuth_Unknown_401(t *testing.T) {
	svc, _, _, _, _ := newTestTokenSvc(t, "token")
	res, perr := svc.Exchange(context.Background(), &TokenRequest{
		GrantType: GrantPassword, ClientID: "nope", Username: "a", Password: "b",
	})
	if res != nil {
		t.Errorf("res = %+v", res)
	}
	if perr == nil || perr.Status != 401 || perr.Code != "invalid_client" {
		t.Fatalf("perr = %+v", perr)
	}
}

// Review Focus #3：公开客户端禁机机凭证。
func TestExchange_ClientCredentials_Public_401(t *testing.T) {
	svc, clients, _, _, _ := newTestTokenSvc(t, "token")
	if err := clients.Create(context.Background(), &model.Oauth2Client{
		ClientID: "pub-m2m", GrantTypes: "client_credentials", AccountTypes: "system",
	}); err != nil {
		t.Fatal(err)
	}
	_, perr := svc.Exchange(context.Background(), &TokenRequest{
		GrantType: GrantClientCredentials, ClientID: "pub-m2m",
	})
	if perr == nil || perr.Status != 401 || perr.Code != "invalid_client" {
		t.Fatalf("perr = %+v", perr)
	}
}

func TestExchange_ClientCredentials_OK(t *testing.T) {
	svc, clients, _, ul, _ := newTestTokenSvc(t, "token")
	if err := clients.Create(context.Background(), &model.Oauth2Client{
		ClientID: "m2m", ClientSecret: "s3cret",
		GrantTypes: "client_credentials", AccountTypes: "system",
	}); err != nil {
		t.Fatal(err)
	}
	res, perr := svc.Exchange(context.Background(), &TokenRequest{
		GrantType: GrantClientCredentials, ClientID: "m2m", ClientSecret: "s3cret",
	})
	if perr != nil {
		t.Fatalf("perr = %v", perr)
	}
	if res.AccessToken != "a" || res.TokenType != "Bearer" {
		t.Errorf("res = %+v", res)
	}
	if len(ul.issued) != 1 {
		t.Fatalf("issued = %d", len(ul.issued))
	}
	if ul.issued[0].ident.AccountType != AccountTypeClient || ul.issued[0].ident.Username != "m2m" {
		t.Errorf("ident = %+v", ul.issued[0].ident)
	}
}

func TestExchange_Refresh_OK(t *testing.T) {
	svc, clients, _, _, iss := newTestTokenSvc(t, "token")
	seedAdminClient(t, clients)
	res, perr := svc.Exchange(context.Background(), &TokenRequest{
		GrantType: GrantRefreshToken, ClientID: "alexgo-admin", RefreshToken: "old-r",
	})
	if perr != nil {
		t.Fatalf("perr = %v", perr)
	}
	if res.AccessToken != "a2" || res.RefreshToken != "r2" || iss.refreshN != 1 {
		t.Errorf("res=%+v refreshN=%d", res, iss.refreshN)
	}
}

func TestExchange_Refresh_JwtMode_400(t *testing.T) {
	svc, clients, _, _, _ := newTestTokenSvc(t, "jwt")
	seedAdminClient(t, clients)
	_, perr := svc.Exchange(context.Background(), &TokenRequest{
		GrantType: GrantRefreshToken, ClientID: "alexgo-admin", RefreshToken: "old-r",
	})
	if perr == nil || perr.Status != 400 || perr.Code != "invalid_grant" {
		t.Fatalf("perr = %+v", perr)
	}
}

func TestExchange_AuthCode_OK(t *testing.T) {
	svc, clients, codes, ul, _ := newTestTokenSvc(t, "token")
	seedAdminClient(t, clients)
	code, err := codes.Issue(context.Background(), IssueCodeParams{
		ClientID: "alexgo-admin", UserID: 3, Username: "alice",
		AccountType: AccountTypeSystem, TenantID: 0,
		RedirectURI: "http://localhost:5173/callback", Scopes: "all",
	})
	if err != nil {
		t.Fatal(err)
	}
	res, perr := svc.Exchange(context.Background(), &TokenRequest{
		GrantType: GrantAuthorizationCode, ClientID: "alexgo-admin",
		Code: code, RedirectURI: "http://localhost:5173/callback",
	})
	if perr != nil {
		t.Fatalf("perr = %v", perr)
	}
	if res.AccessToken != "a" || res.TokenType != "Bearer" {
		t.Errorf("res = %+v", res)
	}
	if len(ul.issued) != 1 {
		t.Fatalf("issued = %d", len(ul.issued))
	}
	id := ul.issued[0].ident
	if id.UserID != 3 || id.Username != "alice" || id.AccountType != AccountTypeSystem {
		t.Errorf("ident = %+v", id)
	}
	if ul.issued[0].clientID != "alexgo-admin" {
		t.Errorf("clientID = %q", ul.issued[0].clientID)
	}
}

func TestExchange_AuthCode_BadCode_400(t *testing.T) {
	svc, clients, _, _, _ := newTestTokenSvc(t, "token")
	seedAdminClient(t, clients)
	_, perr := svc.Exchange(context.Background(), &TokenRequest{
		GrantType: GrantAuthorizationCode, ClientID: "alexgo-admin",
		Code: "bogus", RedirectURI: "http://localhost:5173/callback",
	})
	if perr == nil || perr.Status != 400 || perr.Code != "invalid_grant" {
		t.Fatalf("perr = %+v", perr)
	}
}

func TestLoginProtoErr_Mapping(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{ErrInvalidCredentials, 400, "invalid_grant"},
		{ErrUserDisabled, 400, "invalid_grant"},
		{ErrCaptchaInvalid, 400, "captcha_invalid"},
		{ErrIssuance, 500, "server_error"},
		{ErrAuthenticatorMissing, 500, "server_error"},
		{ErrAccountTypeUnsupported, 400, "invalid_request"},
		{errors.New("boom"), 500, "server_error"},
	}
	for _, c := range cases {
		perr := LoginProtoErr(c.err)
		if perr.Status != c.status || perr.Code != c.code {
			t.Errorf("LoginProtoErr(%v) = %+v, want %d/%s", c.err, perr, c.status, c.code)
		}
	}
	// wrap 链也要命中（IssueFor 的 fmt.Errorf("%w: ...")）
	wrapped := fmt.Errorf("%w: issuer down", ErrIssuance)
	if p := LoginProtoErr(wrapped); p.Status != 500 || p.Code != "server_error" {
		t.Errorf("wrap 链 = %+v", p)
	}
}
```

> 夹具勘误：上面 `stubUnified.ident()` 辅助写法有误——`Login` 直接返回 `s.ident` 字段即可。**实现测试文件时修正为**：

```go
type stubUnified struct {
	res    *LoginResult
	ident  *AuthIdentity
	err    error
	logins []LoginParams
	issued []issueCall
}

func (s *stubUnified) Login(_ context.Context, p LoginParams) (*LoginResult, *AuthIdentity, error) {
	s.logins = append(s.logins, p)
	id := s.ident
	if id == nil {
		id = &AuthIdentity{UserID: 3, Username: p.Username, AccountType: p.AccountType}
	}
	return s.res, id, s.err
}
```

（`Login` 测试只断言 `ul.logins` 与 `res`，ident 不参与断言。）测试文件需 import `fmt`。

- [ ] **Step 2: 运行确认失败**

Run: `CGO_ENABLED=0 go test ./modules/system/oauth2/service/ -run 'TestExchange_|TestLoginProtoErr' -v`
Expected: FAIL —— `undefined: NewTokenService / LoginProtoErr / GrantPassword`（编译错误）。

- [ ] **Step 3: 最小实现**

`modules/system/oauth2/service/token.go`：

```go
package oauth2svc

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/fx"

	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/token"
)

// RFC 6749 grant 类型（表 1）。
const (
	GrantAuthorizationCode = "authorization_code"
	GrantPassword          = "password"
	GrantClientCredentials = "client_credentials"
	GrantRefreshToken      = "refresh_token"
)

// LoginProtoErr 统一登录错误 → RFC 6749 协议错误（Task 11 控制器复用）。
// 映射矩阵是冻结契约（Review Focus：错误码不能随实现漂移）。
func LoginProtoErr(err error) *ProtocolError {
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		return &ProtocolError{Status: 400, Code: "invalid_grant", Description: "账号或密码错误"}
	case errors.Is(err, ErrUserDisabled):
		return &ProtocolError{Status: 400, Code: "invalid_grant", Description: "账号已停用"}
	case errors.Is(err, ErrCaptchaInvalid):
		return &ProtocolError{Status: 400, Code: "captcha_invalid", Description: "验证码缺失或错误"}
	case errors.Is(err, ErrIssuance), errors.Is(err, ErrAuthenticatorMissing):
		return &ProtocolError{Status: 500, Code: "server_error", Description: "签发服务不可用"}
	case errors.Is(err, ErrAccountTypeUnsupported):
		return &ProtocolError{Status: 400, Code: "invalid_request", Description: "不支持的账号类型"}
	default:
		return &ProtocolError{Status: 500, Code: "server_error", Description: "内部错误"}
	}
}

// TokenRequest POST /oauth2/token 请求体（RFC 6749 §5.1 字段 + D12 captcha 两字段）。
// IP/UserAgent 由 controller 从请求上下文填充，不进 JSON。
type TokenRequest struct {
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Code         string `json:"code"`
	RedirectURI  string `json:"redirect_uri"`
	CodeVerifier string `json:"code_verifier"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	AccountType  string `json:"account_type"`
	RefreshToken string `json:"refresh_token"`
	CaptchaUUID  string `json:"captcha_uuid"`
	CaptchaCode  string `json:"captcha_code"`
	IP           string `json:"-"`
	UserAgent    string `json:"-"`
}

// TokenResponse RFC 6749 §5.1 成功信封。
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type TokenServiceDeps struct {
	fx.In
	Cfg     *config.Config
	Clients *ClientService
	Codes   *AuthorizeCodeService
	Login   UnifiedLogin
	Issuer  token.Issuer
}

type TokenService struct {
	deps TokenServiceDeps
}

func NewTokenService(deps TokenServiceDeps) *TokenService {
	return &TokenService{deps: deps}
}

func toTokenResponse(r *LoginResult) *TokenResponse {
	return &TokenResponse{
		AccessToken:  r.AccessToken,
		RefreshToken: r.RefreshToken,
		ExpiresIn:    r.ExpiresIn,
		TokenType:    "Bearer",
	}
}

// Exchange 四 grant 状态机。第一步恒为 client 认证（所有 grant 通用）：
// 失败 → 401 invalid_client（含公开客户端走机机、未知 client、停用、错 secret）。
func (s *TokenService) Exchange(ctx context.Context, req *TokenRequest) (*TokenResponse, *ProtocolError) {
	client, err := s.deps.Clients.Authenticate(ctx, req.ClientID, req.ClientSecret)
	if err != nil {
		return nil, &ProtocolError{Status: 401, Code: "invalid_client", Description: "client 认证失败"}
	}
	switch req.GrantType {
	case GrantPassword:
		return s.passwordGrant(ctx, client, req)
	case GrantClientCredentials:
		return s.clientCredentialsGrant(ctx, client, req)
	case GrantRefreshToken:
		return s.refreshGrant(ctx, client, req)
	case GrantAuthorizationCode:
		return s.authCodeGrant(ctx, client, req)
	default:
		return nil, &ProtocolError{Status: 400, Code: "unsupported_grant_type", Description: "不支持的 grant_type"}
	}
}

func (s *TokenService) passwordGrant(ctx context.Context, client *model.Oauth2Client, req *TokenRequest) (*TokenResponse, *ProtocolError) {
	if !client.AllowsGrant(GrantPassword) {
		return nil, &ProtocolError{Status: 400, Code: "unsupported_grant_type", Description: "client 未获 password 许可"}
	}
	at := req.AccountType
	if at == "" {
		at = AccountTypeSystem
	}
	if !client.AllowsAccountType(at) {
		// Review Focus #3：账号类型白名单（防 admin client 换 member token）。
		return nil, &ProtocolError{Status: 400, Code: "unauthorized_client", Description: "client 不允许该账号类型"}
	}
	res, _, lerr := s.deps.Login.Login(ctx, LoginParams{
		AccountType: at, Username: req.Username, Password: req.Password,
		ClientID: req.ClientID, CaptchaUUID: req.CaptchaUUID, CaptchaCode: req.CaptchaCode,
		IP: req.IP, UserAgent: req.UserAgent,
	})
	if lerr != nil {
		return nil, LoginProtoErr(lerr)
	}
	return toTokenResponse(res), nil
}

func (s *TokenService) clientCredentialsGrant(ctx context.Context, client *model.Oauth2Client, _ *TokenRequest) (*TokenResponse, *ProtocolError) {
	if !client.AllowsGrant(GrantClientCredentials) {
		return nil, &ProtocolError{Status: 400, Code: "unsupported_grant_type", Description: "client 未获 client_credentials 许可"}
	}
	if client.ClientSecret == "" {
		// Review Focus #3：公开客户端禁机机凭证。
		return nil, &ProtocolError{Status: 401, Code: "invalid_client", Description: "公开客户端不支持机机凭证"}
	}
	ident := &AuthIdentity{Username: client.ClientID, AccountType: AccountTypeClient}
	res, ierr := s.deps.Login.IssueFor(ctx, ident, client.ClientID)
	if ierr != nil {
		return nil, LoginProtoErr(ierr)
	}
	return toTokenResponse(res), nil
}

func (s *TokenService) refreshGrant(ctx context.Context, client *model.Oauth2Client, req *TokenRequest) (*TokenResponse, *ProtocolError) {
	if !client.AllowsGrant(GrantRefreshToken) {
		return nil, &ProtocolError{Status: 400, Code: "unsupported_grant_type", Description: "client 未获 refresh_token 许可"}
	}
	if s.deps.Cfg != nil && s.deps.Cfg.Auth.Mode == "jwt" {
		return nil, &ProtocolError{Status: 400, Code: "invalid_grant", Description: "jwt 模式不支持 refresh"}
	}
	issued, rerr := s.deps.Issuer.Refresh(ctx, req.RefreshToken)
	if rerr != nil {
		return nil, &ProtocolError{Status: 400, Code: "invalid_grant", Description: "刷新令牌无效"}
	}
	// D14：refresh 不做 client 绑定（refresh token 即凭证），只过了上面的 client 活跃+grant 白名单。
	return &TokenResponse{
		AccessToken:  issued.AccessToken,
		RefreshToken: issued.RefreshToken,
		ExpiresIn:    issued.ExpiresIn,
		TokenType:    "Bearer",
	}, nil
}

func (s *TokenService) authCodeGrant(ctx context.Context, client *model.Oauth2Client, req *TokenRequest) (*TokenResponse, *ProtocolError) {
	if !client.AllowsGrant(GrantAuthorizationCode) {
		return nil, &ProtocolError{Status: 400, Code: "unsupported_grant_type", Description: "client 未获 authorization_code 许可"}
	}
	row, cerr := s.deps.Codes.Exchange(ctx, ExchangeParams{
		Code: req.Code, ClientID: req.ClientID,
		RedirectURI: req.RedirectURI, CodeVerifier: req.CodeVerifier,
	})
	if cerr != nil {
		return nil, &ProtocolError{Status: 400, Code: "invalid_grant", Description: "授权码无效"}
	}
	if !client.AllowsAccountType(row.AccountType) {
		return nil, &ProtocolError{Status: 400, Code: "unauthorized_client", Description: "client 不允许该账号类型"}
	}
	ident := &AuthIdentity{
		UserID: row.UserID, TenantID: row.OwnerTenantID,
		Username: row.Username, AccountType: row.AccountType,
	}
	res, ierr := s.deps.Login.IssueFor(ctx, ident, row.ClientID)
	if ierr != nil {
		return nil, LoginProtoErr(ierr)
	}
	return toTokenResponse(res), nil
}
```

（`model` import：`alexGo-cloud/modules/system/oauth2/model`。`fmt` 若未用则删。）

- [ ] **Step 4: 运行确认通过**

Run: `CGO_ENABLED=0 go test ./modules/system/oauth2/... -v`
Expected: PASS（新 12 例 + 既有全绿）。

- [ ] **Step 5: Commit**

```bash
git add modules/system/oauth2/service/
git commit -m "feat(oauth2): TokenService 四 grant 状态机 + LoginProtoErr 映射矩阵"
```

---

### Task 11: POST /authorize/login + POST /token 协议端点

**Files:**
- Modify: `modules/system/oauth2/controller/protocol.go`
- Test: `modules/system/oauth2/controller/protocol_test.go`（追加夹具与 7 例）
- Modify: `modules/system/oauth2/module.go`（Provide 块加一行 `oauth2svc.NewTokenService`）

**Interfaces:**
- Consumes: Task 9 `ProtocolDeps`/`protocolErr`/`seedSpaClient` 与 authorize 校验顺序；Task 8 `UnifiedLogin.AuthorizeLogin` + `LoginParams`；Task 10 `NewTokenService/Exchange/TokenRequest/TokenResponse/LoginProtoErr/Grant*`；Task 7 `ClientService.Get/AllowsGrant/AllowsRedirect`；Task 9 `AuthorizeCodeService.Issue(IssueCodeParams) (string, error)`。
- Produces（Task 13 消费，逐字——Task 13 只再往 deps 加 `Validator` 字段，ctor 签名不再变）：

```go
package controller

type ProtocolDeps struct {
	fx.In
	Captcha *oauth2svc.CaptchaService
	Codes   *oauth2svc.AuthorizeCodeService
	Clients *oauth2svc.ClientService
	Cfg     *config.Config
	Login   oauth2svc.UnifiedLogin
	Tokens  *oauth2svc.TokenService
}
func NewProtocolController(deps ProtocolDeps) *ProtocolController
// Register（入参分组已带 /api 前缀）：
//   GET  /oauth2/captcha         → captchaImage
//   GET  /oauth2/authorize       → authorize
//   POST /oauth2/authorize/login → authorizeLogin
//   POST /oauth2/token           → token
```

**裁决**：`/authorize/login` 与 `/token` 请求体一律 **JSON**（仓库控制器惯例，不采用 RFC 6749 的 `application/x-www-form-urlencoded`——三端前端与 e2e 全部 `fetch` JSON）。错误一律 RFC 6749 信封 `{"error","error_description"}`；除成功路径的 302 外，**任何失败都不设 Location 头**。

- [ ] **Step 1: 写失败测试**

`modules/system/oauth2/controller/protocol_test.go` 追加（在 Task 9 文件末尾；import 增 `encoding/json`、`"alexGo-cloud/pkg/token"`）：

```go
// —— Task 11 夹具：真 UnifiedLogin + 真 TokenService 的协议全栈 ——

// stubUA UserAuthenticator 桩：记账调用——验证码/白名单拒绝时不得触达凭据校验（ua.calls==0 断言）。
type stubUA struct {
	ident *oauth2svc.AuthIdentity
	err   error
	calls []string
}

func (s *stubUA) Authenticate(_ context.Context, username, _ string) (*oauth2svc.AuthIdentity, error) {
	s.calls = append(s.calls, username)
	if s.err != nil {
		return nil, s.err
	}
	if s.ident != nil {
		return s.ident, nil
	}
	return &oauth2svc.AuthIdentity{UserID: 3, Username: username, AccountType: oauth2svc.AccountTypeSystem}, nil
}

// fakeIssuer token.Issuer 桩：Issue 恒出 at-1/rt-1，Refresh 出 at-2/rt-2。
type fakeIssuer struct{}

func (fakeIssuer) Issue(context.Context, token.IssueParams) (*token.Issued, error) {
	return &token.Issued{AccessToken: "at-1", RefreshToken: "rt-1", ExpiresIn: 7200}, nil
}
func (fakeIssuer) Refresh(context.Context, string) (*token.Issued, error) {
	return &token.Issued{AccessToken: "at-2", RefreshToken: "rt-2", ExpiresIn: 7200}, nil
}
func (fakeIssuer) Revoke(context.Context, string) error                   { return nil }
func (fakeIssuer) RevokeAll(context.Context, token.UserType, uint64) error { return nil }

type fullEnv struct {
	clients *oauth2svc.ClientService
	codes   *oauth2svc.AuthorizeCodeService
	ua      *stubUA
}

// newFullRouter 搭真 UnifiedLogin + 真 TokenService 的协议全栈（cfg nil → 默认 LoginPage）。
// 与 newProtoRouter（Task 9，无 Login/Tokens）并存：老测试不动，新测试用本夹具。
func newFullRouter(t *testing.T, cfg *config.Config) (*gin.Engine, *fullEnv) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Oauth2Client{}, &model.Oauth2AuthorizeCode{}); err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		cfg = &config.Config{}
		cfg.Auth.LoginPage = "/auth/login"
	}
	clients := oauth2svc.NewClientService(repository.NewClientRepository(db))
	codes := oauth2svc.NewAuthorizeCodeService(repository.NewAuthorizeCodeRepository(db))
	ua := &stubUA{}
	iss := fakeIssuer{}
	captcha := oauth2svc.NewCaptchaService(oauth2svc.CaptchaDeps{Cfg: cfg})
	ul := oauth2svc.NewUnifiedLogin(oauth2svc.UnifiedDeps{
		Cfg: cfg, Issuer: iss, Captcha: captcha, UserAuth: ua,
	})
	ts := oauth2svc.NewTokenService(oauth2svc.TokenServiceDeps{
		Cfg: cfg, Clients: clients, Codes: codes, Login: ul, Issuer: iss,
	})
	proto := NewProtocolController(ProtocolDeps{
		Captcha: captcha, Codes: codes, Clients: clients, Cfg: cfg,
		Login: ul, Tokens: ts,
	})
	r := gin.New()
	proto.Register(r.Group("/api"))
	return r, &fullEnv{clients: clients, codes: codes, ua: ua}
}

func postJSON(r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// Review Focus #1：authorize/login 的 redirect 篡改 → 400 且无 Location，
// 且拒绝必须发生在凭据校验之前（ua.calls==0）。
func TestAuthorizeLogin_RedirectTamper_400NoLocation(t *testing.T) {
	r, env := newFullRouter(t, nil)
	seedSpaClient(t, env.clients)
	w := postJSON(r, "/api/oauth2/authorize/login",
		`{"client_id":"spa","redirect_uri":"http://evil.example/steal","username":"alice","password":"pw"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc != "" {
		t.Errorf("篡改 redirect 不得带 Location, got %q", loc)
	}
	if !strings.Contains(w.Body.String(), `"error":"invalid_request"`) {
		t.Errorf("body = %s", w.Body.String())
	}
	if len(env.ua.calls) != 0 {
		t.Errorf("Authenticate calls = %v, want 0（redirect 校验先于凭据）", env.ua.calls)
	}
}

// 成功：302 回跳带 code/state，回跳的 code 可被 Exchange 消费，凭据恰校验一次。
func TestAuthorizeLogin_OK_302WithCode(t *testing.T) {
	r, env := newFullRouter(t, nil)
	seedSpaClient(t, env.clients)
	w := postJSON(r, "/api/oauth2/authorize/login",
		`{"client_id":"spa","redirect_uri":"http://localhost:5173/callback","state":"xyz","scope":"all","username":"alice","password":"pw"}`)
	if w.Code != http.StatusFound {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	loc := w.Header().Get("Location")
	if !strings.HasPrefix(loc, "http://localhost:5173/callback?") {
		t.Fatalf("Location = %q", loc)
	}
	for _, want := range []string{"code=", "state=xyz"} {
		if !strings.Contains(loc, want) {
			t.Errorf("Location 缺 %q: %q", want, loc)
		}
	}
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatal(err)
	}
	code := u.Query().Get("code")
	if code == "" {
		t.Fatal("回跳缺 code")
	}
	if _, err := env.codes.Exchange(context.Background(), oauth2svc.ExchangeParams{
		Code: code, ClientID: "spa", RedirectURI: "http://localhost:5173/callback",
	}); err != nil {
		t.Errorf("回跳 code 应可消费: %v", err)
	}
	if len(env.ua.calls) != 1 {
		t.Errorf("Authenticate calls = %v, want 恰一次", env.ua.calls)
	}
}

// Review Focus #2：开关开 → 缺 captcha 字段 400 captcha_invalid，凭据不触达（authorize/login 路径）。
func TestAuthorizeLogin_Captcha_400(t *testing.T) {
	cfg := &config.Config{}
	cfg.Auth.LoginPage = "/auth/login"
	cfg.Auth.Captcha.Enabled = true
	r, env := newFullRouter(t, cfg)
	seedSpaClient(t, env.clients)
	w := postJSON(r, "/api/oauth2/authorize/login",
		`{"client_id":"spa","redirect_uri":"http://localhost:5173/callback","username":"alice","password":"pw"}`)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "captcha_invalid") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc != "" {
		t.Errorf("失败不得带 Location, got %q", loc)
	}
	if len(env.ua.calls) != 0 {
		t.Errorf("Authenticate calls = %v, want 0（captcha 先于凭据）", env.ua.calls)
	}
}

func TestToken_Password_OK_200(t *testing.T) {
	r, env := newFullRouter(t, nil)
	seedSpaClient(t, env.clients)
	w := postJSON(r, "/api/oauth2/token",
		`{"grant_type":"password","client_id":"spa","username":"alice","password":"pw"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.AccessToken != "at-1" || resp.RefreshToken != "rt-1" || resp.ExpiresIn != 7200 || resp.TokenType != "Bearer" {
		t.Errorf("resp = %+v", resp)
	}
	if len(env.ua.calls) != 1 {
		t.Errorf("Authenticate calls = %v, want 恰一次", env.ua.calls)
	}
}

// Review Focus #2：开关开 + 错误验证码 → 400 captcha_invalid（/token 路径），凭据不触达。
func TestToken_CaptchaInvalid_400(t *testing.T) {
	cfg := &config.Config{}
	cfg.Auth.Captcha.Enabled = true
	r, env := newFullRouter(t, cfg)
	seedSpaClient(t, env.clients)
	w := postJSON(r, "/api/oauth2/token",
		`{"grant_type":"password","client_id":"spa","username":"alice","password":"pw","captcha_uuid":"u1","captcha_code":"0000-wrong"}`)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "captcha_invalid") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if len(env.ua.calls) != 0 {
		t.Errorf("Authenticate calls = %v, want 0（captcha 先于凭据）", env.ua.calls)
	}
}

func TestToken_UnknownGrant_400(t *testing.T) {
	r, env := newFullRouter(t, nil)
	seedSpaClient(t, env.clients)
	w := postJSON(r, "/api/oauth2/token",
		`{"grant_type":"urn:ietf:params:oauth:grant-type:device_code","client_id":"spa"}`)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "unsupported_grant_type") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestToken_ClientAuthFail_401(t *testing.T) {
	r, _ := newFullRouter(t, nil)
	w := postJSON(r, "/api/oauth2/token",
		`{"grant_type":"password","client_id":"nope"}`)
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "invalid_client") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `CGO_ENABLED=0 go test ./modules/system/oauth2/controller/ -v`
Expected: FAIL —— 编译错：`unknown field "Login" in struct literal of type controller.ProtocolDeps`、`undefined: oauth2svc.NewTokenService`。

- [ ] **Step 3: 实现两个协议端点 + module Provide**

`modules/system/oauth2/controller/protocol.go` 修改：

1. `ProtocolDeps` 追加两字段（放 `Cfg` 之后，其余不动）：

```go
	Login   oauth2svc.UnifiedLogin
	Tokens  *oauth2svc.TokenService
```

2. `Register` 追加两条路由：

```go
	r.POST("/oauth2/authorize/login", p.authorizeLogin)
	r.POST("/oauth2/token", p.token)
```

3. 文件末尾追加两个 handler：

```go
// authorizeLogin 统一登录表单提交（spec §4.2 ②③；JSON body——裁决见任务头）。
// 校验顺序与 GET /authorize 一致：形状/白名单校验全部先于凭据，
// **前 6 步失败一律 400 且无 Location**（失效/篡改绝不 302，D15 补充语义）；
// 凭据经 Login.AuthorizeLogin（不签发 token）；成功才 Codes.Issue 并 302 回跳。
func (p *ProtocolController) authorizeLogin(c *gin.Context) {
	var req struct {
		ClientID            string `json:"client_id"`
		RedirectURI         string `json:"redirect_uri"`
		State               string `json:"state"`
		Scope               string `json:"scope"`
		ResponseType        string `json:"response_type"`
		Username            string `json:"username"`
		Password            string `json:"password"`
		AccountType         string `json:"account_type"`
		CaptchaUUID         string `json:"captcha_uuid"`
		CaptchaCode         string `json:"captcha_code"`
		CodeChallenge       string `json:"code_challenge"`
		CodeChallengeMethod string `json:"code_challenge_method"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		p.protocolErr(c, http.StatusBadRequest, "invalid_request", "请求体必须是 JSON 对象")
		return
	}
	if req.ClientID == "" || req.RedirectURI == "" {
		p.protocolErr(c, http.StatusBadRequest, "invalid_request", "缺少 client_id/redirect_uri")
		return
	}
	ctx := c.Request.Context()
	client, err := p.deps.Clients.Get(ctx, req.ClientID)
	if err != nil {
		p.protocolErr(c, http.StatusBadRequest, "invalid_client", "未知或停用的 client")
		return
	}
	if !client.AllowsGrant(oauth2svc.GrantAuthorizationCode) {
		p.protocolErr(c, http.StatusBadRequest, "unauthorized_client", "client 未获授权码许可")
		return
	}
	if !client.AllowsRedirect(req.RedirectURI) {
		p.protocolErr(c, http.StatusBadRequest, "invalid_request", "redirect_uri 不在白名单")
		return
	}
	if req.ResponseType != "" && req.ResponseType != "code" {
		p.protocolErr(c, http.StatusBadRequest, "unsupported_response_type", "仅支持 response_type=code")
		return
	}
	if req.CodeChallenge != "" && req.CodeChallengeMethod != "S256" {
		p.protocolErr(c, http.StatusBadRequest, "invalid_request", "PKCE 仅支持 S256")
		return
	}
	ident, lerr := p.deps.Login.AuthorizeLogin(ctx, oauth2svc.LoginParams{
		AccountType: req.AccountType, // 空值由统一登录缺省 system（Task 8 checkAndRoute）
		Username:    req.Username,
		Password:    req.Password,
		ClientID:    req.ClientID,
		CaptchaUUID: req.CaptchaUUID,
		CaptchaCode: req.CaptchaCode,
		IP:          c.ClientIP(),
		UserAgent:   c.GetHeader("User-Agent"),
	})
	if lerr != nil {
		pe := oauth2svc.LoginProtoErr(lerr)
		p.protocolErr(c, pe.Status, pe.Code, pe.Description)
		return
	}
	code, ierr := p.deps.Codes.Issue(ctx, oauth2svc.IssueCodeParams{
		ClientID: req.ClientID, UserID: ident.UserID, Username: ident.Username,
		AccountType: ident.AccountType, TenantID: ident.TenantID,
		RedirectURI: req.RedirectURI, Scopes: req.Scope,
		CodeChallenge: req.CodeChallenge, CodeChallengeMethod: req.CodeChallengeMethod,
	})
	if ierr != nil {
		p.protocolErr(c, http.StatusInternalServerError, "server_error", "签发授权码失败")
		return
	}
	vals := url.Values{}
	vals.Set("code", code)
	if req.State != "" {
		vals.Set("state", req.State)
	}
	if req.Scope != "" {
		vals.Set("scope", req.Scope)
	}
	c.Redirect(http.StatusFound, req.RedirectURI+"?"+vals.Encode())
}

// token 四 grant 统一换 token（spec §5；JSON body——裁决见任务头）。
// 成功 200 信封 TokenResponse；失败把 TokenService 的 *ProtocolError 原样写出。
func (p *ProtocolController) token(c *gin.Context) {
	var req oauth2svc.TokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		p.protocolErr(c, http.StatusBadRequest, "invalid_request", "请求体必须是 JSON 对象")
		return
	}
	req.IP = c.ClientIP()
	req.UserAgent = c.GetHeader("User-Agent")
	resp, perr := p.deps.Tokens.Exchange(c.Request.Context(), &req)
	if perr != nil {
		p.protocolErr(c, perr.Status, perr.Code, perr.Description)
		return
	}
	c.JSON(http.StatusOK, resp)
}
```

4. `modules/system/oauth2/module.go`：Provide 块 `oauth2svc.NewUnifiedLogin,` 之后加一行：

```go
			oauth2svc.NewTokenService,
```

- [ ] **Step 4: 运行确认通过**

Run: `CGO_ENABLED=0 go test ./modules/system/oauth2/... -v && CGO_ENABLED=0 go test ./alexgo-server/cmd/ ./modules/member/cmd/`
Expected: PASS（controller 新 7 例 + 既有 6 例全绿；两 main fx 干跑图满足 `NewTokenService` 的 `Cfg/Clients/Codes/Login/Issuer` 五个依赖）。

- [ ] **Step 5: Commit**

```bash
git add modules/system/oauth2/controller/ modules/system/oauth2/module.go
git commit -m "feat(oauth2): authorize/login 302 回跳与 POST /token 协议端点"
```

---

### Task 12: 三端兼容层切换统一登录 + AuthService.Login 下线

**Files:**
- Modify: `modules/system/controller/vben/controller.go`、`modules/system/controller/vben/fakes_test.go`、`modules/system/controller/vben/controller_test.go`
- Modify: `modules/system/controller/app/controller.go`；Create: `modules/system/controller/app/controller_test.go`
- Modify: `modules/member/controller/app/member_auth.go`、`modules/member/controller/app/member_auth_test.go`
- Modify: `modules/system/service/auth.go`、`modules/system/service/auth_test.go`
- （三端 module.go **无 Provide 行变化**——ctor 名不变，fx 按新签名自动重解析；`UnifiedLogin` 由 oauth2 模块提供，两个 main 都装了 `oauth2.FxModule`（D7））

**Interfaces:**
- Consumes: Task 8 `UnifiedLogin.Login/LoginParams/LoginResult`；Task 3 `ErrCaptchaInvalid`；Task 4 `NewUserAuthenticator/NewJWTGenerator`；Task 8 `NewUnifiedLogin/UnifiedDeps` 五哨兵。
- Produces（逐字——Task 13/14 消费）：

```go
// modules/system/controller/vben
func NewController(login oauth2svc.UnifiedLogin, authSvc service.AuthService,
	permSvc service.PermissionService, userSvc service.UserService) *Controller
// modules/system/controller/app
func NewAppController(unified oauth2svc.UnifiedLogin) *AppController
// modules/member/controller/app
func NewAuthController(svc service.MemberService, unified oauth2svc.UnifiedLogin) *AuthController
// modules/system/service —— AuthService 缩为两方法（Login + clientID() 删除，
// vben+app 是仅有的 Login 调用方，已 grep 确认；member 走 MemberService 不受影响）
type AuthService interface {
	Refresh(ctx context.Context, refreshToken string) (*LoginResult, error)
	Logout(ctx context.Context, accessToken string) error
}
```

**裁决（执行时记 ledger）：**
1. 三端请求体新增**可选** `captcha_uuid`/`captcha_code` 字段（`auth.captcha.enabled` 默认 false，既有请求零改动全绿）。
2. ClientID / AccountType 固定值：vben=`alexgo-admin`/`system`、system app=`alexgo-admin`/`system`、member=`alexgo-app`/`member`（对齐旧 `clientID()` 与旧 member `Issue` 参数）。
3. 错误映射：vben——`ErrCaptchaInvalid`→400 `fail(c,400,"captcha invalid")`，其余→401 固定 `invalid username or password`（契约不变，防枚举）；app——`ErrCaptchaInvalid`→400 `{"error":...}`，其余→500 `{"error": err.Error()}`（**怪癖保留**）；member——`ErrIssuance|ErrAuthenticatorMissing`→503 精确 `{"error":"auth service unavailable"}`、`ErrCaptchaInvalid`→400、其余→401 `{"error": err.Error()}`（哨兵文案 "invalid credentials"/"user disabled" 与旧字符串逐字相同，body 不变）。
4. 三端 **删除内联审计**（D13：审计归统一登录恰一次，msg 前缀 "oauth2: "，防双记）。
5. **预记 ledger 两处行为差**：app 空用户名错误文案从 "username or password is empty" 变 "invalid credentials"（统一登录无空前置检查）；member 旧 `repo.Update` 错误透传被 Task 5 `Authenticate` 的 `_ =` 吞掉。
6. **Refresh/Logout 兼容端点保持现状、不改调统一服务（对 spec §4.4「member 登录/刷新/登出改调统一服务」的裁决）**：`UnifiedLogin` 接口（Task 4 冻结）只有 `Login/AuthorizeLogin/IssueFor`，本无 Refresh/Logout；旧 `AuthService.Refresh→issuer.Refresh`（含 jwt 模式拒绝）与协议端点 `refreshGrant→Issuer.Refresh`（T10 同样 jwt 拒绝）底层同源 `pkg/token`、语义逐条等价（含 D14 无 client 绑定差异仅在协议侧多两道白名单检查），member `Logout→issuer.Revoke` 亦不动——改调需扩接口、行为零收益，YAGNI。故 vben/app/member 的 refresh/logout 端点与其测试在本任务全部保持原样（各步骤「Refresh/Logout 不动」即此裁决落地）。

- [ ] **Step 1: 改写四组测试（RED）**

**(a) `vben/fakes_test.go`**：删 `fakeAuth.Login/loginRes/loginErr`（保留 `Refresh/Logout/logoutCount`）、**删 `fakeAudit` 整块**、新增：

```go
// fakeUnified 实现 oauth2svc.UnifiedLogin：Login 按注入值返回并记账（断言 AccountType/ClientID）。
type fakeUnified struct {
	res    *oauth2svc.LoginResult
	err    error
	logins []oauth2svc.LoginParams
}

func (f *fakeUnified) Login(_ context.Context, p oauth2svc.LoginParams) (*oauth2svc.LoginResult, *oauth2svc.AuthIdentity, error) {
	f.logins = append(f.logins, p)
	if f.err != nil {
		return nil, nil, f.err
	}
	return f.res, &oauth2svc.AuthIdentity{UserID: 7, Username: p.Username, AccountType: p.AccountType}, nil
}
func (f *fakeUnified) AuthorizeLogin(_ context.Context, p oauth2svc.LoginParams) (*oauth2svc.AuthIdentity, error) {
	f.logins = append(f.logins, p)
	if f.err != nil {
		return nil, f.err
	}
	return &oauth2svc.AuthIdentity{UserID: 7, Username: p.Username, AccountType: p.AccountType}, nil
}
func (f *fakeUnified) IssueFor(_ context.Context, _ *oauth2svc.AuthIdentity, _ string) (*oauth2svc.LoginResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.res, nil
}
```

补 import `oauth2svc "alexGo-cloud/modules/system/oauth2/service"`。

**(b) `vben/controller_test.go`**：

- `newTestRouter` 改签名（去 audit、加 unified）：

```go
func newTestRouter(auth *fakeAuth, unified *fakeUnified, perm *fakePerm, user *fakeUser) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	NewController(unified, auth, perm, user).Register(r.Group("/api"))
	return r
}
```

- `TestLogin_Success_Envelope`：夹具 `unified := &fakeUnified{res: &oauth2svc.LoginResult{AccessToken: "at-1", RefreshToken: "rt-1", ExpiresIn: 7200}}`，envelope 断言**逐字保留**，`audit.successes` 断言**删除**，改为：

```go
	if len(unified.logins) != 1 ||
		unified.logins[0].AccountType != oauth2svc.AccountTypeSystem ||
		unified.logins[0].ClientID != "alexgo-admin" {
		t.Errorf("logins = %+v, want 恰一次且 account_type=system client_id=alexgo-admin", unified.logins)
	}
```

- `TestLogin_WrongPassword_401Envelope`：夹具 `unified := &fakeUnified{err: oauth2svc.ErrInvalidCredentials}`，401 信封断言保留，audit 断言删除。
- **新增**：

```go
func TestLogin_CaptchaInvalid_400(t *testing.T) {
	unified := &fakeUnified{err: oauth2svc.ErrCaptchaInvalid}
	w := postJSON(newTestRouter(&fakeAuth{}, unified, &fakePerm{}, &fakeUser{}),
		"/api/auth/login", `{"username":"admin","password":"x","captcha_uuid":"u","captcha_code":"y"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"code":400`) {
		t.Errorf("body = %s", w.Body.String())
	}
}
```

- `TestLogin_BadBody_400` / `TestLogout_*` / `TestRegister_MountsAllFiveRoutes`：`newTestRouter` 调用点的 `&fakeAudit{}` 换 `&fakeUnified{res: &oauth2svc.LoginResult{AccessToken: "at", RefreshToken: "rt", ExpiresIn: 1}}`（TestLogin_BadBody 用 `&fakeUnified{}` 即可）。
- 直接 `NewController(...)` 的用例（Codes/UserInfo/MenuAll/NilCodes/NoClaims）：第一个参数换 `&fakeUnified{}`，第二参数 `&fakeAuth{}`（audit 参数位消失）。

**(c) 新建 `app/controller_test.go`**：

```go
package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
)

// stubUnified 本包独立夹具（system/controller/app 与 vben 是不同包，不能共用）。
type stubUnified struct {
	res    *oauth2svc.LoginResult
	err    error
	logins []oauth2svc.LoginParams
}

func (s *stubUnified) Login(_ context.Context, p oauth2svc.LoginParams) (*oauth2svc.LoginResult, *oauth2svc.AuthIdentity, error) {
	s.logins = append(s.logins, p)
	if s.err != nil {
		return nil, nil, s.err
	}
	return s.res, &oauth2svc.AuthIdentity{UserID: 1, Username: p.Username, AccountType: p.AccountType}, nil
}
func (s *stubUnified) AuthorizeLogin(_ context.Context, p oauth2svc.LoginParams) (*oauth2svc.AuthIdentity, error) {
	s.logins = append(s.logins, p)
	if s.err != nil {
		return nil, s.err
	}
	return &oauth2svc.AuthIdentity{UserID: 1, Username: p.Username, AccountType: p.AccountType}, nil
}
func (s *stubUnified) IssueFor(_ context.Context, _ *oauth2svc.AuthIdentity, _ string) (*oauth2svc.LoginResult, error) {
	return s.res, s.err
}

func appLogin(t *testing.T, u *stubUnified, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/app/system/auth/login", NewAppController(u).Login)
	req := httptest.NewRequest(http.MethodPost, "/api/app/system/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestAppLogin_Quirks 冻结 app 端怪癖契约（Review Focus #5 三端兼容之一）：
// 成功 200 三元组；凭据错 500（不是 401，怪癖）；captcha 拒绝 400；body 非 JSON 400。
func TestAppLogin_Quirks(t *testing.T) {
	t.Run("success 200 triple", func(t *testing.T) {
		u := &stubUnified{res: &oauth2svc.LoginResult{AccessToken: "at", RefreshToken: "rt", ExpiresIn: 60}}
		w := appLogin(t, u, `{"username":"a","password":"b"}`)
		if w.Code != http.StatusOK {
			t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
		}
		for _, want := range []string{`"token":"at"`, `"refresh_token":"rt"`, `"expires_in":60`} {
			if !strings.Contains(w.Body.String(), want) {
				t.Errorf("body 缺 %s: %s", want, w.Body.String())
			}
		}
		if len(u.logins) != 1 || u.logins[0].ClientID != "alexgo-admin" ||
			u.logins[0].AccountType != oauth2svc.AccountTypeSystem {
			t.Errorf("logins = %+v", u.logins)
		}
	})
	t.Run("bad credentials 500 quirk", func(t *testing.T) {
		u := &stubUnified{err: oauth2svc.ErrInvalidCredentials}
		w := appLogin(t, u, `{"username":"a","password":"b"}`)
		if w.Code != http.StatusInternalServerError ||
			!strings.Contains(w.Body.String(), "invalid credentials") {
			t.Fatalf("code=%d body=%s（怪癖：凭据错也 500）", w.Code, w.Body.String())
		}
	})
	t.Run("captcha rejected 400", func(t *testing.T) {
		u := &stubUnified{err: oauth2svc.ErrCaptchaInvalid}
		w := appLogin(t, u, `{"username":"a","password":"b","captcha_uuid":"u","captcha_code":"y"}`)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
		}
	})
	t.Run("bad body 400", func(t *testing.T) {
		w := appLogin(t, &stubUnified{}, `not-json`)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
		}
	})
}
```

**(d) `member_auth_test.go` 整文件重写**（旧 `stubRepo/badIssuer/okIssuer/enabledRepo` 夹具与 `service/bcrypt/model/repository/token` import 随登录路径一起下线——本文件只剩登录测试）：

```go
package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
)

// stubUnified 本包独立夹具（member/controller/app 独立命名空间）。
type stubUnified struct {
	res    *oauth2svc.LoginResult
	err    error
	logins []oauth2svc.LoginParams
}

func (s *stubUnified) Login(_ context.Context, p oauth2svc.LoginParams) (*oauth2svc.LoginResult, *oauth2svc.AuthIdentity, error) {
	s.logins = append(s.logins, p)
	if s.err != nil {
		return nil, nil, s.err
	}
	return s.res, &oauth2svc.AuthIdentity{UserID: 1, Username: p.Username, AccountType: p.AccountType}, nil
}
func (s *stubUnified) AuthorizeLogin(_ context.Context, p oauth2svc.LoginParams) (*oauth2svc.AuthIdentity, error) {
	s.logins = append(s.logins, p)
	if s.err != nil {
		return nil, s.err
	}
	return &oauth2svc.AuthIdentity{UserID: 1, Username: p.Username, AccountType: p.AccountType}, nil
}
func (s *stubUnified) IssueFor(_ context.Context, _ *oauth2svc.AuthIdentity, _ string) (*oauth2svc.LoginResult, error) {
	return s.res, s.err
}

// doLogin 走 httptest 完整路由（复刻 module 注册的 POST /api/app/member/auth/login）。
// svc 归 Register/Refresh/Logout，Login 不再经 MemberService → 传 nil。
func doLogin(t *testing.T, unified *stubUnified, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	c := NewAuthController(nil, unified)
	r.POST("/api/app/member/auth/login", c.Login)
	req := httptest.NewRequest(http.MethodPost, "/api/app/member/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// I2：统一登录返回 wrap ErrIssuance → 503（签发故障绝不冒充 401）。
func TestLogin_NilIssuer_503(t *testing.T) {
	u := &stubUnified{err: fmt.Errorf("%w: issuer not wired", oauth2svc.ErrIssuance)}
	w := doLogin(t, u, `{"mobile":"13800000000","password":"pw123456"}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body=%s", w.Code, w.Body.String())
	}
	if w.Body.String() != `{"error":"auth service unavailable"}` {
		t.Errorf("body = %s", w.Body.String())
	}
}

// I2：issuer 故障（wrap ErrIssuance）→ 503。
func TestLogin_BadIssuer_503(t *testing.T) {
	u := &stubUnified{err: fmt.Errorf("%w: deadline exceeded", oauth2svc.ErrIssuance)}
	w := doLogin(t, u, `{"mobile":"13800000000","password":"pw123456"}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body=%s", w.Code, w.Body.String())
	}
}

// 凭据错误 → 401（与 503 判别：签发故障绝不冒充密码错误，反之亦然）。
func TestLogin_BadPassword_401(t *testing.T) {
	u := &stubUnified{err: oauth2svc.ErrInvalidCredentials}
	w := doLogin(t, u, `{"mobile":"13800000000","password":"wrong-pass"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body=%s", w.Code, w.Body.String())
	}
	// 哨兵文案与旧字符串逐字相同 → body 不变（裁决 3）。
	if w.Body.String() != `{"error":"invalid credentials"}` {
		t.Errorf("body = %s", w.Body.String())
	}
}

// 正常签发 → 200 且返回 token 三元组；登录参数按裁决 2 固定。
func TestLogin_OK_200(t *testing.T) {
	u := &stubUnified{res: &oauth2svc.LoginResult{AccessToken: "a", RefreshToken: "r", ExpiresIn: 7200}}
	w := doLogin(t, u, `{"mobile":"13800000000","password":"pw123456"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"token":"a"`) {
		t.Errorf("body = %s, want token payload", w.Body.String())
	}
	if len(u.logins) != 1 || u.logins[0].AccountType != oauth2svc.AccountTypeMember ||
		u.logins[0].ClientID != "alexgo-app" || u.logins[0].Username != "13800000000" {
		t.Errorf("logins = %+v", u.logins)
	}
}

// Review Focus #5：三端 captcha 映射——member 端 400。
func TestLogin_CaptchaInvalid_400(t *testing.T) {
	u := &stubUnified{err: oauth2svc.ErrCaptchaInvalid}
	w := doLogin(t, u, `{"mobile":"13800000000","password":"pw123456","captcha_uuid":"u","captcha_code":"y"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
}
```

**(e) `system/service/auth_test.go`**：删 `TestLogin_IssuesToken`、`TestLogin_BadPassword_NoIssue` 两例；`TestLogin_JwtRollbackMode` **迁移改写**（删例产生的 RED 由实现步的接口收缩兑现，本步先落测试）：

```go
// TestLogin_JwtRollbackMode：回滚开关端到端——mode=jwt 时统一登录必须签发旧静态
// JWT（auth.ParseToken 可解析），且绝不触碰 token.Issuer；Refresh 快速拒绝、Logout 幂等。
func TestLogin_JwtRollbackMode(t *testing.T) {
	cfg := &config.Config{}
	cfg.Auth.Mode = "jwt"
	cfg.System.JWTSecret = "rollback-secret"
	iss := &fakeIssuer{issued: &token.Issued{AccessToken: "a1", RefreshToken: "r1", ExpiresIn: 7200}}
	ul := oauth2svc.NewUnifiedLogin(oauth2svc.UnifiedDeps{
		Cfg:       cfg,
		Issuer:    iss,
		Captcha:   oauth2svc.NewCaptchaService(oauth2svc.CaptchaDeps{Cfg: cfg}),
		UserAuth:  NewUserAuthenticator(&fakeUserRepo{user: enabledUser()}, fakePerm{}),
		JWTGen:    NewJWTGenerator(cfg),
	})

	res, _, err := ul.Login(context.Background(), oauth2svc.LoginParams{
		AccountType: oauth2svc.AccountTypeSystem,
		Username:    "alice",
		Password:    "pw",
		ClientID:    "alexgo-admin",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	// 判别性①：jwt 回滚绝不走不透明令牌签发器。
	if iss.issueCalls != 0 {
		t.Errorf("Issue calls = %d, want 0（jwt rollback must not use token.Issuer）", iss.issueCalls)
	}
	// 判别性②：返回的 token 必须能被中间件 jwt 分支解析且 claims 正确。
	claims, perr := auth.ParseToken(res.AccessToken, cfg)
	if perr != nil {
		t.Fatalf("issued token must parse as legacy static JWT: %v", perr)
	}
	if claims.UserID != 3 || claims.Username != "alice" {
		t.Errorf("claims = %+v", claims)
	}
	// 判别性③：Refresh 快速拒绝 / Logout 幂等 —— 归属未缩的 AuthService。
	svc := NewAuthService(cfg, &fakeUserRepo{}, fakePerm{}, iss)
	if _, rerr := svc.Refresh(context.Background(), "r1"); rerr == nil {
		t.Error("Refresh must be rejected in jwt mode")
	}
	if lerr := svc.Logout(context.Background(), res.AccessToken); lerr != nil {
		t.Errorf("Logout() in jwt mode = %v, want nil", lerr)
	}
}
```

补 import `oauth2svc "alexGo-cloud/modules/system/oauth2/service"`（`auth`/`token`/`config`/`context` 既有）。

Run: `CGO_ENABLED=0 go test ./modules/system/... ./modules/member/... 2>&1 | tail -30`
Expected: FAIL —— 编译错：vben `NewController` 参数不匹配、app `NewAppController` 参数不匹配、member `NewAuthController` 参数不匹配（auth_test 迁移例此时已可编译，随包级失败一起呈现）。

- [ ] **Step 2: 实现四组改造（GREEN）**

**(a) `vben/controller.go`**：结构体与 ctor 改为 `unified/authSvc/permSvc/userSvc`（删 `auditSvc`），`Login` 整体重写（`errors`、`oauth2svc` 补 import）：

```go
// Login POST /api/auth/login —— vben 登录页契约（内部走统一登录；D13 审计在服务层恰一次）。
// 映射：captcha 拒绝 → 400；其余错误 → 401 固定文案（契约不变，防枚举）。
func (ctrl *Controller) Login(c *gin.Context) {
	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		CaptchaUUID string `json:"captcha_uuid"`
		CaptchaCode string `json:"captcha_code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid request body")
		return
	}
	res, _, err := ctrl.unified.Login(c.Request.Context(), oauth2svc.LoginParams{
		AccountType: oauth2svc.AccountTypeSystem,
		Username:    req.Username,
		Password:    req.Password,
		ClientID:    "alexgo-admin",
		CaptchaUUID: req.CaptchaUUID,
		CaptchaCode: req.CaptchaCode,
		IP:          c.ClientIP(),
		UserAgent:   c.GetHeader("User-Agent"),
	})
	if err != nil {
		if errors.Is(err, oauth2svc.ErrCaptchaInvalid) {
			fail(c, http.StatusBadRequest, "captcha invalid")
			return
		}
		fail(c, http.StatusUnauthorized, errInvalidCredentials.Error())
		return
	}
	ok(c, gin.H{
		"accessToken":  res.AccessToken,
		"refreshToken": res.RefreshToken,
		"expiresIn":    res.ExpiresIn,
	})
}
```

其余方法（Logout/Codes/UserInfo/MenuAll）不动。

**(b) `app/controller.go` 整文件重写**（`service` import 移除）：

```go
package app

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
)

type AppController struct {
	unified oauth2svc.UnifiedLogin
}

func NewAppController(unified oauth2svc.UnifiedLogin) *AppController {
	return &AppController{unified: unified}
}

type loginRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	CaptchaUUID string `json:"captcha_uuid"`
	CaptchaCode string `json:"captcha_code"`
}

// Login app 端登录（怪癖保留：除 captcha 与 bind 外一律 500——既有前端契约）。
// 审计归统一登录（D13），本层不再记账。
func (ctrl *AppController) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, _, err := ctrl.unified.Login(c.Request.Context(), oauth2svc.LoginParams{
		AccountType: oauth2svc.AccountTypeSystem,
		Username:    req.Username,
		Password:    req.Password,
		ClientID:    "alexgo-admin",
		CaptchaUUID: req.CaptchaUUID,
		CaptchaCode: req.CaptchaCode,
		IP:          c.ClientIP(),
		UserAgent:   c.GetHeader("User-Agent"),
	})
	if err != nil {
		if errors.Is(err, oauth2svc.ErrCaptchaInvalid) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 响应字段与旧版逐字一致：前端读 res.token 不变。
	c.JSON(http.StatusOK, gin.H{
		"token": res.AccessToken, "refresh_token": res.RefreshToken, "expires_in": res.ExpiresIn,
	})
}
```

**(c) `member_auth.go`**：`AuthController` 加 `unified oauth2svc.UnifiedLogin` 字段，ctor 改 `NewAuthController(svc, unified)`；`loginRequest` 加两个 captcha 字段；`Login` 重写（`oauth2svc` 补 import）：

```go
// Login 会员登录（公开接口）——内部走统一登录（account_type=member，client=alexgo-app）。
// 映射：签发/装配故障 → 503（不冒充 401）；captcha 拒绝 → 400；其余 → 401。
func (c *AuthController) Login(ctx *gin.Context) {
	var req loginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		errJSON(ctx, http.StatusBadRequest, err)
		return
	}
	res, _, err := c.unified.Login(ctx.Request.Context(), oauth2svc.LoginParams{
		AccountType: oauth2svc.AccountTypeMember,
		Username:    req.Mobile,
		Password:    req.Password,
		ClientID:    "alexgo-app",
		CaptchaUUID: req.CaptchaUUID,
		CaptchaCode: req.CaptchaCode,
		IP:          ctx.ClientIP(),
		UserAgent:   ctx.GetHeader("User-Agent"),
	})
	if err != nil {
		switch {
		case errors.Is(err, oauth2svc.ErrIssuance), errors.Is(err, oauth2svc.ErrAuthenticatorMissing):
			errJSON(ctx, http.StatusServiceUnavailable, fmt.Errorf("auth service unavailable"))
		case errors.Is(err, oauth2svc.ErrCaptchaInvalid):
			errJSON(ctx, http.StatusBadRequest, err)
		default:
			errJSON(ctx, http.StatusUnauthorized, err)
		}
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"token": res.AccessToken, "refresh_token": res.RefreshToken, "expires_in": res.ExpiresIn,
	})
}
```

Register/Refresh/Logout 与 `svc` 用法不动。

**(d) `system/service/auth.go`**：接口删 `Login` 行；`authService` 删 `clientID()` 与 `Login` 方法（`Refresh/Logout/toResult/NewAuthService` 签名与实现全部不动；`permSvc` 字段保留——`NewAuthService` 调用方与签名不变）。

Run: `CGO_ENABLED=0 go test ./modules/system/... ./modules/member/... -v 2>&1 | tail -60`
Expected: PASS（vben 信封/captcha 例、app `TestAppLogin_Quirks` 4 子例、member 5 例、auth_test 迁移版 + Refresh/Logout 例全绿；system 既有用例全绿）。

- [ ] **Step 3: 全量回归 + 接线红线**

Run: `CGO_ENABLED=0 go test ./... && CGO_ENABLED=0 go test ./alexgo-server/cmd/ ./modules/member/cmd/`
Expected: PASS（全仓绿；两 main fx 干跑——vben/app/member 三个新 ctor 的 `oauth2svc.UnifiedLogin` 由 oauth2 模块提供，两进程都装了 `oauth2.FxModule`）。

- [ ] **Step 4: Commit**

```bash
git add modules/system/controller/ modules/system/service/auth.go modules/system/service/auth_test.go \
        modules/member/controller/app/
git commit -m "feat(auth): 三端兼容层切换统一登录（审计归服务层）+ AuthService.Login 下线"
```

---

### Task 13: userinfo 自校验端点 + client 管理 CRUD + oauth2:client 路由注册

**Files:**
- Modify: `modules/system/oauth2/controller/protocol.go`（`ProtocolDeps` 加 `Validator`、Register 加路由、`userInfo` handler）
- Test: `modules/system/oauth2/controller/protocol_test.go`（追加夹具与 4 例）
- Create: `modules/system/oauth2/controller/admin/client.go`
- Test: `modules/system/oauth2/controller/admin/client_test.go`（新包，仅测试文件起步）
- Modify: `modules/system/oauth2/module.go`（Provide 加 `admin.NewClientController`、`NewModule` 聚合、`RegisterRoutes` 挂 admin 组）
- Modify: `modules/system/service/permission_routes.go`（加 `"oauth2:client"` 条目）
- Test: `modules/system/service/permission_routes_test.go`（追加注册守护 1 例）

**Interfaces:**
- Consumes: Task 9 `ProtocolDeps`/`protocolErr`/`Register`（本任务只往 deps **加字段**，ctor 签名不变）；Task 7 `ClientService.List/Create/Update/Delete` 与 `model.Oauth2Client`（`ClientSecret json:"-"`、`Activity`、`Scopes`）；`pkg/auth.ParseToken(tokenStr, cfg) (*auth.Claims, error)`、`pkg/token.Validator{Validate(ctx, string) (*token.Claims, error)}`、`token.UserTypeAdmin/UserTypeMember`；中间件 `parseClaims` 的 jwt 恒管理员语义（`pkg/middleware/auth.go`，userinfo 对齐）。
- Produces（Task 15/16 消费，逐字）：

```go
package controller

type ProtocolDeps struct {
	fx.In
	Captcha   *oauth2svc.CaptchaService
	Codes     *oauth2svc.AuthorizeCodeService
	Clients   *oauth2svc.ClientService
	Cfg       *config.Config
	Login     oauth2svc.UnifiedLogin
	Tokens    *oauth2svc.TokenService
	Validator token.Validator `optional:"true"` // Task 13 追加
}
// Register 在 POST /oauth2/token 之后追加：
//   GET /oauth2/userinfo → userInfo
// 响应 200 {"user_id","username","user_type","tenant_id","scope":"all"}；
// 失败/缺 token 401 {"error":"invalid_token","error_description":...}（RFC 信封）。

package admin // alexGo-cloud/modules/system/oauth2/controller/admin

func NewClientController(svc *oauth2svc.ClientService) *ClientController
// module RegisterRoutes 内联挂载（r 为 /api 组）：
//   g := r.Group("/admin/oauth2")
//   GET    /clients     → List    200 {data, total}
//   POST   /clients     → Create  200 {data}（明文 secret 一次性回显）
//   PUT    /clients/:id → Update  200 {data}
//   DELETE /clients/:id → Delete  200 {"status":"ok"}
//   校验错 400 {"error"}；读库/内部错 500 {"error"}（admin 信封，非 RFC）

// modules/system/service/permission_routes.go 追加：
"oauth2:client": {"/api/admin/oauth2/clients", "/api/admin/oauth2/clients/*"},
```

**裁决：**
1. userinfo **自校验**（`/api/oauth2/*` 是公开路径，中间件不产 claims）：jwt 模式走 `auth.ParseToken` 并**恒回 `user_type=1`**（对齐中间件 `parseClaims`：旧静态 JWT 仅管理端签发）；token 模式走 `Validator.Validate`，`user_type` 原样透传（member=2）。token 模式但 `Validator==nil` → **fail-closed 401**（与中间件同款：装配错绝不放行）。
2. Create 明文 secret **一次性回显**（spec §3.1「创建时明文只回显一次」）：`svc.Create` 原地哈希前先存明文，响应包一层 `createClientResponse`（嵌入 model + `ClientSecret string \`json:"client_secret"\`` 覆盖 `json:"-"`）；此后 List/Update 恒不回显。
3. `client_id` 空 → controller 先行 400（不让表单错误变 500）；其余 service 错（DB/不存在）→ 500；bind/id 错 → 400（对齐 `menu.go`/`user.go` 惯例）。
4. 分页：controller `size>200` 截断 200；`page<1`/`size<1` 由 `ClientService.List` 归正（1/20）——controller 只补上限。

- [ ] **Step 1: 写失败测试**

**(a) `modules/system/oauth2/controller/protocol_test.go` 追加**（import 增 `"alexGo-cloud/pkg/auth"`）：

```go
// —— Task 13：userinfo 自校验 ——

// fakeValidator token.Validator 桩（token 模式 userinfo 用）。
type fakeValidator struct {
	claims *token.Claims
	err    error
}

func (f *fakeValidator) Validate(context.Context, string) (*token.Claims, error) {
	return f.claims, f.err
}

// newValidatorRouter 带 Validator 的协议路由（其余依赖与 newProtoRouter 同款填充）。
func newValidatorRouter(t *testing.T, cfg *config.Config, v token.Validator) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Oauth2Client{}, &model.Oauth2AuthorizeCode{}); err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		cfg = &config.Config{}
		cfg.Auth.LoginPage = "/auth/login"
	}
	proto := NewProtocolController(ProtocolDeps{
		Captcha:   oauth2svc.NewCaptchaService(oauth2svc.CaptchaDeps{Cfg: cfg}),
		Codes:     oauth2svc.NewAuthorizeCodeService(repository.NewAuthorizeCodeRepository(db)),
		Clients:   oauth2svc.NewClientService(repository.NewClientRepository(db)),
		Cfg:       cfg,
		Validator: v,
	})
	r := gin.New()
	proto.Register(r.Group("/api"))
	return r
}

func userInfoGet(r *gin.Engine, bearer string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/oauth2/userinfo", nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// jwt 模式：auth.ParseToken 自校验，恒按管理员回（对齐中间件 parseClaims）。
func TestUserInfo_JwtMode_OK(t *testing.T) {
	cfg := &config.Config{}
	cfg.Auth.Mode = "jwt"
	cfg.System.JWTSecret = "userinfo-secret"
	tok, err := auth.GenerateToken(3, "alice", 0, cfg)
	if err != nil {
		t.Fatal(err)
	}
	w := userInfoGet(newValidatorRouter(t, cfg, nil), tok)
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["user_id"] != float64(3) || body["username"] != "alice" ||
		body["user_type"] != float64(1) || body["scope"] != "all" {
		t.Errorf("body = %v", body)
	}
}

// token 模式：Validator 自校验，user_type/tenant_id 原样透传（member=2）。
func TestUserInfo_TokenMode_OK(t *testing.T) {
	v := &fakeValidator{claims: &token.Claims{
		UserID: 5, Username: "bob", UserType: token.UserTypeMember, TenantID: 7,
	}}
	w := userInfoGet(newValidatorRouter(t, nil, v), "opaque-1")
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["user_id"] != float64(5) || body["username"] != "bob" ||
		body["user_type"] != float64(2) || body["tenant_id"] != float64(7) {
		t.Errorf("body = %v", body)
	}
}

// 缺 token / 坏 token → 401 invalid_token（RFC 信封），绝不 200。
func TestUserInfo_MissingOrBad_401(t *testing.T) {
	w := userInfoGet(newValidatorRouter(t, nil, nil), "")
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "invalid_token") {
		t.Errorf("missing: code=%d body=%s", w.Code, w.Body.String())
	}
	cfg := &config.Config{}
	cfg.Auth.Mode = "jwt"
	cfg.System.JWTSecret = "s"
	w = userInfoGet(newValidatorRouter(t, cfg, nil), "garbage")
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "invalid_token") {
		t.Errorf("bad: code=%d body=%s", w.Code, w.Body.String())
	}
}

// token 模式 Validator 未装配 → fail-closed 401（裁决 1：装配错绝不放行）。
func TestUserInfo_ValidatorMissing_401(t *testing.T) {
	w := userInfoGet(newValidatorRouter(t, nil, nil), "any-token")
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "invalid_token") {
		t.Errorf("code=%d body=%s", w.Code, w.Body.String())
	}
}
```

**(b) 新建 `modules/system/oauth2/controller/admin/client_test.go`**：

```go
package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/system/oauth2/model"
	"alexGo-cloud/modules/system/oauth2/repository"
	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
)

func newClientRouter(t *testing.T) (*gin.Engine, *oauth2svc.ClientService) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Oauth2Client{}); err != nil {
		t.Fatal(err)
	}
	svc := oauth2svc.NewClientService(repository.NewClientRepository(db))
	c := NewClientController(svc)
	r := gin.New()
	g := r.Group("/api/admin/oauth2")
	g.GET("/clients", c.List)
	g.POST("/clients", c.Create)
	g.PUT("/clients/:id", c.Update)
	g.DELETE("/clients/:id", c.Delete)
	return r, svc
}

func doJSON(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// List 信封 {data,total}；secret 恒不回显（json:"-"）。
func TestClients_ListEnvelope(t *testing.T) {
	r, svc := newClientRouter(t)
	for _, id := range []string{"c1", "c2"} {
		if err := svc.Create(context.Background(), &model.Oauth2Client{
			ClientID: id, GrantTypes: "password,refresh_token",
		}); err != nil {
			t.Fatal(err)
		}
	}
	w := doJSON(r, http.MethodGet, "/api/admin/oauth2/clients?page=1&size=20", "")
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Data  []map[string]any `json:"data"`
		Total int64            `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data) != 2 || body.Total != 2 {
		t.Errorf("data=%d total=%d, want 2/2", len(body.Data), body.Total)
	}
	if strings.Contains(w.Body.String(), "client_secret") {
		t.Errorf("列表不得出现 client_secret 字段: %s", w.Body.String())
	}
}

// Create 坏 body → 400 {"error"}（admin 信封，非 RFC）。
func TestClients_Create_400BadBody(t *testing.T) {
	r, _ := newClientRouter(t)
	w := doJSON(r, http.MethodPost, "/api/admin/oauth2/clients", "not-json")
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), `"error"`) {
		t.Errorf("code=%d body=%s", w.Code, w.Body.String())
	}
}

// 裁决 2：明文 secret 创建时一次性回显，此后列表绝不出现。
func TestClients_Create_OK_SecretEchoOnce(t *testing.T) {
	r, _ := newClientRouter(t)
	w := doJSON(r, http.MethodPost, "/api/admin/oauth2/clients",
		`{"client_id":"c1","client_secret":"plain-1","grant_types":"password,refresh_token"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"client_secret":"plain-1"`) {
		t.Errorf("创建响应须一次性回显明文 secret: %s", w.Body.String())
	}
	w = doJSON(r, http.MethodGet, "/api/admin/oauth2/clients", "")
	if strings.Contains(w.Body.String(), "plain-1") {
		t.Errorf("列表绝不回显明文 secret: %s", w.Body.String())
	}
}

// Delete → {"status":"ok"}，随后列表归零。
func TestClients_Delete_OK(t *testing.T) {
	r, svc := newClientRouter(t)
	if err := svc.Create(context.Background(), &model.Oauth2Client{
		ClientID: "gone", GrantTypes: "password",
	}); err != nil {
		t.Fatal(err)
	}
	w := doJSON(r, http.MethodGet, "/api/admin/oauth2/clients", "")
	var list struct {
		Data []struct {
			ID uint64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Data) != 1 {
		t.Fatalf("seed list: %s", w.Body.String())
	}
	w = doJSON(r, http.MethodDelete,
		"/api/admin/oauth2/clients/"+strconv.FormatUint(list.Data[0].ID, 10), "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"status":"ok"`) {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	w = doJSON(r, http.MethodGet, "/api/admin/oauth2/clients", "")
	if !strings.Contains(w.Body.String(), `"total":0`) {
		t.Errorf("delete 后 total: %s", w.Body.String())
	}
}
```

**(c) `modules/system/service/permission_routes_test.go` 追加**：

```go
// Task 13：oauth2 client 管理路由前缀必须注册——否则 Task 16 菜单种子产生的
// oauth2:client 权限在 Casbin 无 policy，admin 打 client 管理页 403。
func TestPermissionRoutes_Oauth2ClientRegistered(t *testing.T) {
	patterns, ok := permissionRoutes["oauth2:client"]
	if !ok {
		t.Fatal("permissionRoutes 缺 oauth2:client（client 管理页会 403）")
	}
	want := []string{"/api/admin/oauth2/clients", "/api/admin/oauth2/clients/*"}
	for _, w := range want {
		found := false
		for _, p := range patterns {
			if p == w {
				found = true
			}
		}
		if !found {
			t.Errorf("oauth2:client 缺路由模式 %q, got %v", w, patterns)
		}
	}
}
```

Run: `CGO_ENABLED=0 go test ./modules/system/oauth2/... ./modules/system/service/ 2>&1 | tail -30`
Expected: FAIL —— 编译错 `undefined: NewClientController`（admin 包仅测试文件）、`unknown field "Validator" in struct literal of type controller.ProtocolDeps`；外加 `--- FAIL: TestPermissionRoutes_Oauth2ClientRegistered`（缺键断言）。

- [ ] **Step 2: 实现（GREEN）**

**(a) `modules/system/oauth2/controller/protocol.go`**：

1. import 补 `"strings"`（若未有）、`"alexGo-cloud/pkg/auth"`、`"alexGo-cloud/pkg/token"`。
2. `ProtocolDeps` 追加字段（`Tokens` 之后）：

```go
	Validator token.Validator `optional:"true"`
```

3. `Register` 在 `r.POST("/oauth2/token", p.token)` 之后加：

```go
	r.GET("/oauth2/userinfo", p.userInfo)
```

4. 文件末尾追加 handler：

```go
// userInfo GET /api/oauth2/userinfo —— 按 token 取当前账号信息（spec §5，第三方接入常用）。
// 公开路径无中间件 claims，故自校验（裁决 1）：
// jwt 模式 auth.ParseToken 并恒回 user_type=1（对齐中间件 parseClaims——旧静态 JWT
// 仅管理端签发）；token 模式 Validator.Validate 原样透传；Validator 未装配 fail-closed 401。
// 失败/缺 token 一律 RFC 6749 信封 401 invalid_token。
func (p *ProtocolController) userInfo(c *gin.Context) {
	raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(c.GetHeader("Authorization")), "Bearer "))
	if raw == "" {
		p.protocolErr(c, http.StatusUnauthorized, "invalid_token", "缺少 access token")
		return
	}
	var (
		uid    uint64
		name   string
		utype  int
		tenant uint64
	)
	if p.deps.Cfg != nil && p.deps.Cfg.Auth.Mode == "jwt" {
		cl, err := auth.ParseToken(raw, p.deps.Cfg)
		if err != nil {
			p.protocolErr(c, http.StatusUnauthorized, "invalid_token", "token 无效或已过期")
			return
		}
		uid, name, tenant = cl.UserID, cl.Username, cl.TenantID
		utype = int(token.UserTypeAdmin) // 旧静态 JWT 恒管理员（与中间件一致）
	} else {
		if p.deps.Validator == nil {
			p.protocolErr(c, http.StatusUnauthorized, "invalid_token", "token 校验器未装配")
			return
		}
		cl, err := p.deps.Validator.Validate(c.Request.Context(), raw)
		if err != nil {
			p.protocolErr(c, http.StatusUnauthorized, "invalid_token", "token 无效或已过期")
			return
		}
		uid, name, utype, tenant = cl.UserID, cl.Username, int(cl.UserType), cl.TenantID
	}
	c.JSON(http.StatusOK, gin.H{
		"user_id":  uid,
		"username": name,
		"user_type": utype,
		"tenant_id": tenant,
		"scope":     "all", // spec：不做 scope 树，默认 all 单值
	})
}
```

**(b) 新建 `modules/system/oauth2/controller/admin/client.go`**：

```go
package admin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/oauth2/model"
	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
)

// ClientController OAuth2 client 管理（admin 信封 {data}/{data,total}/{status}/{error}，
// 与 system admin 控制器同款；非 RFC 6749 信封——协议信封只属于 /api/oauth2/*）。
type ClientController struct {
	svc *oauth2svc.ClientService
}

func NewClientController(svc *oauth2svc.ClientService) *ClientController {
	return &ClientController{svc: svc}
}

// createClientResponse Create 响应：嵌入 model 展平字段，外层 ClientSecret
// （深度 0）覆盖嵌入的 json:"-"（深度 1）——明文仅此一次回显（裁决 2，spec §3.1）。
type createClientResponse struct {
	*model.Oauth2Client
	ClientSecret string `json:"client_secret"`
}

// List GET /admin/oauth2/clients?page&size → {data,total}。
// page/size 归正（1/20）在 ClientService.List；本层只补 size 上限 200（裁决 4）。
func (c *ClientController) List(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(ctx.DefaultQuery("size", "20"))
	if size > 200 {
		size = 200
	}
	items, total, err := c.svc.List(ctx.Request.Context(), page, size)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": items, "total": total})
}

// Create POST /admin/oauth2/clients → {data}；client_id 空先行 400（裁决 3）。
func (c *ClientController) Create(ctx *gin.Context) {
	var req model.Oauth2Client
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.ClientID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "client_id required"})
		return
	}
	req.ID = 0 // 新建忽略请求 id
	plain := req.ClientSecret
	if err := c.svc.Create(ctx.Request.Context(), &req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": createClientResponse{
		Oauth2Client: &req, ClientSecret: plain,
	}})
}

// Update PUT /admin/oauth2/clients/:id → {data}；id 非法/bind 错 400，service 错 500。
// body 中 secret 留空 → 服务层保留库中旧值（Task 7 Update 语义），且响应经 json:"-" 不回显。
func (c *ClientController) Update(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req model.Oauth2Client
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.ClientID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "client_id required"})
		return
	}
	req.ID = id
	if err := c.svc.Update(ctx.Request.Context(), &req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": req})
}

// Delete DELETE /admin/oauth2/clients/:id → {"status":"ok"}。
func (c *ClientController) Delete(ctx *gin.Context) {
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

**(c) `modules/system/oauth2/module.go`**：import 补 `"alexGo-cloud/modules/system/oauth2/controller/admin"`；Provide 块 `controller.NewProtocolController,` 之后加一行：

```go
		admin.NewClientController,
```

`oauth2Module`/`NewModule`/`RegisterRoutes` 改为：

```go
type oauth2Module struct {
	proto   *controller.ProtocolController
	clients *admin.ClientController
}

// NewModule 聚合控制器，产出 server.Module（挂进 group:"modules"）。
func NewModule(proto *controller.ProtocolController, clients *admin.ClientController) server.Module {
	return &oauth2Module{proto: proto, clients: clients}
}

// RegisterRoutes 入参是 /api group（见 server/http.go，infra 同款）。
func (m *oauth2Module) RegisterRoutes(r *gin.RouterGroup) {
	m.proto.Register(r)
	g := r.Group("/admin/oauth2")
	g.GET("/clients", m.clients.List)
	g.POST("/clients", m.clients.Create)
	g.PUT("/clients/:id", m.clients.Update)
	g.DELETE("/clients/:id", m.clients.Delete)
}
```

**(d) `modules/system/service/permission_routes.go`**：`permissionRoutes` 中 `"infra:codegen"` 条目之后加一行：

```go
	"oauth2:client": {"/api/admin/oauth2/clients", "/api/admin/oauth2/clients/*"},
```

Run: `CGO_ENABLED=0 go test ./modules/system/oauth2/... ./modules/system/service/ -v 2>&1 | tail -60`
Expected: PASS（userinfo 4 例 + admin client 4 例 + `TestPermissionRoutes_Oauth2ClientRegistered` + 既有全绿）。

- [ ] **Step 3: 全量回归 + 接线红线**

Run: `CGO_ENABLED=0 go test ./... && CGO_ENABLED=0 go test ./alexgo-server/cmd/ ./modules/member/cmd/`
Expected: PASS（`NewModule` 多收一个 `*admin.ClientController`——fx 自动解析，其依赖 `*oauth2svc.ClientService` 已 Provide，两 main 干跑图仍满足）。

- [ ] **Step 4: Commit**

```bash
git add modules/system/oauth2/ modules/system/service/permission_routes.go modules/system/service/permission_routes_test.go
git commit -m "feat(oauth2): userinfo 自校验 + client 管理 CRUD + oauth2:client 路由注册"
```

---

### Task 14: 前端 SSO 改造（PKCE + /callback，web-antd）

**Files:**
- Create: `apps/web-antd/src/utils/pkce.ts`
- Create: `apps/web-antd/src/api/core/oauth2.ts`
- Create: `apps/web-antd/src/views/_core/authentication/callback.vue`
- Modify: `apps/web-antd/src/api/core/index.ts`（追加 `export * from './oauth2'`）
- Modify: `apps/web-antd/src/router/routes/core.ts`（coreRoutes 顶层加 `/callback`）
- Modify: `apps/web-antd/src/store/auth.ts`（`authLogin` 改 SSO，`logout`/`fetchUserInfo` 不动）

**Interfaces:**
- Consumes: Task 11 `POST /api/oauth2/authorize/login`（JSON body：`client_id/redirect_uri/state/response_type/username/password/account_type/captcha_uuid/captcha_code/code_challenge/code_challenge_method`；成功 **302** → `redirect_uri?code&state`，失败 400 RFC 信封 `{"error","error_description"}`、无 Location）与 `POST /api/oauth2/token`（JSON `{"grant_type":"authorization_code","code","redirect_uri","client_id","code_verifier"}` → 200 `{"access_token","refresh_token","expires_in","token_type"}`）；D10 种子 client `alexgo-admin`（公开、redirect 白名单含 `http://localhost:5173/callback` 与 `http://localhost:8080/callback`）；`useAccessStore`（`setAccessToken/setAccessCodes/loginExpired/setLoginExpired`）、`useUserStore.setUserInfo`、`getUserInfoApi`/`getAccessCodesApi`（`#/api`）、`LOGIN_PATH`（`@vben/constants`）、`preferences.app.defaultHomePath`。
- Produces: `randomUrlSafe(byteLength?: number): string`、`deriveChallenge(verifier: string): Promise<string>`（`#/utils/pkce`）；`OAUTH_SESSION_KEY: 'alexgo_oauth_session'`、`interface OAuthSession {verifier; state; redirect}`、`authorizeLoginApi(params: AuthorizeLoginParams): Promise<string>`、`exchangeTokenApi(params): Promise<OAuthTokenResponse>`、`class OAuthHttpError extends Error {status; body}`（`#/api/core/oauth2`，并由 `#/api` 再导出）；路由名 `OAuthCallback`（进 coreRoutes → `coreRouteNames` 自动免守卫）；`authLogin` 签名不变、返回恒 `{userInfo: null}`（跳转归 /callback）。后续任务不消费本任务符号；`loginApi` 保留导出但不再被调用（同 `refreshTokenApi` 先例，下线评估归 Task 17）。

**裁决：**
1. **typecheck 替代 RED→GREEN**：前端仓库无测试框架，门禁=`pnpm -C apps/web-antd typecheck`（Global Constraints 既定）。RED 手法=先改引用方（auth.ts/core.ts/index.ts）指向尚不存在的模块，typecheck 必报 4×`TS2307 Cannot find module`；再补三个新文件 → 全绿。
2. `authLogin(params, _onSuccess?)` 签名保留（参数改名 `_onSuccess` 过 `noUnusedParameters`），但 SSO 下**不再调用**——唯一调用方 login.vue `@submit="authStore.authLogin"` 只传表单值，导航职责整体移到 /callback。
3. 失败提示用**字面中文** `notification.error({message:'登录失败', description})`：`authentication.json` 无 `loginFailed` key（已核），不动 i18n 资源；成功提示沿用既有 `loginSuccess/loginSuccessDesc` key（渲染移到 /callback）。
4. 协议调用用**裸 fetch**（`oauth2.ts`），不走 requestClient：① RFC 6749 信封过不了 vben `{code,data}` 拦截器；② 需要 `redirect:'follow'` 后读 `resp.url` 提取 code。`apiURL` 取 `useAppConfig(import.meta.env, import.meta.env.PROD)`（同 `request.ts`，dev=`/api`）。
5. 后端图形验证码 UI 本任务**不做**（`auth.captcha.enabled` 默认 false，滑块验证码仅前端门禁）；登录请求不发 `captcha_uuid/captcha_code`。记入已知限制，配置开关打开时需另行补 UI。
6. 生产域名的 redirect 白名单登记（D10 只含 localhost 两条）属部署方配置，不在本任务代码内。

- [ ] **Step 1: 引用方先行（RED）**

**(a) `src/api/core/index.ts`** 追加一行：

```ts
export * from './oauth2';
```

**(b) `src/router/routes/core.ts`** —— `coreRoutes` 数组中 Authentication 条目（`path: '/auth'` 的那个）**之后、数组闭合前**追加顶层路由：

```ts
  {
    component: () => import('#/views/_core/authentication/callback.vue'),
    meta: {
      hideInTab: true,
      ignoreAccess: true,
      title: 'OAuth Callback',
    },
    name: 'OAuthCallback',
    path: '/callback',
  },
```

（`routes/index.ts` 的 `coreRouteNames = traverseTreeValues(coreRoutes, route => route.name)` 自动含 `OAuthCallback` → 守卫 coreRouteNames 分支（guard.ts:54）直接放行，**guard.ts 零改动**；fallbackNotFoundRoute 在 routes/index.ts 中恒最后注册，`/callback` 优先命中。）

**(c) `src/store/auth.ts` 整文件重写**（`loginApi`/`getAccessCodesApi`/`preferences`/`$t` 四个 import 删除——`noUnusedLocals` 会拦；`logout`/`fetchUserInfo`/`$reset` 与原样一致）：

```ts
import type { Recordable, UserInfo } from '@vben/types';

import { ref } from 'vue';
import { useRouter } from 'vue-router';

import { LOGIN_PATH } from '@vben/constants';
import { resetAllStores, useAccessStore, useUserStore } from '@vben/stores';

import { notification } from 'ant-design-vue';
import { defineStore } from 'pinia';

import { getUserInfoApi, logoutApi } from '#/api';
import {
  authorizeLoginApi,
  OAuthHttpError,
  OAUTH_SESSION_KEY,
  type OAuthSession,
} from '#/api/core/oauth2';
import { deriveChallenge, randomUrlSafe } from '#/utils/pkce';

export const useAuthStore = defineStore('auth', () => {
  const accessStore = useAccessStore();
  const userStore = useUserStore();
  const router = useRouter();

  const loginLoading = ref(false);

  /**
   * 异步处理登录操作（SSO：OAuth2 授权码 + PKCE，spec §9 D15）
   * 生成 verifier/state/S256 → 存 sessionStorage → POST /oauth2/authorize/login
   * （redirect:'follow'，从 resp.url 提取 code 并比对 state）→ 转 /callback 完成
   * code→token、用户信息与权限码加载、登录后跳转。
   * _onSuccess 保留签名但 SSO 下不再调用（导航归 /callback；唯一调用方
   * login.vue 只传表单值）。
   * @param params 登录表单数据（只消费 username/password）
   */
  async function authLogin(
    params: Recordable<any>,
    _onSuccess?: () => Promise<void> | void,
  ) {
    let userInfo: null | UserInfo = null;
    try {
      loginLoading.value = true;

      const verifier = randomUrlSafe(32);
      const state = randomUrlSafe(16);
      const codeChallenge = await deriveChallenge(verifier);
      const session: OAuthSession = {
        verifier,
        state,
        redirect: String(router.currentRoute.value.query.redirect ?? ''),
      };
      sessionStorage.setItem(OAUTH_SESSION_KEY, JSON.stringify(session));

      const finalUrl = await authorizeLoginApi({
        account_type: 'system',
        client_id: 'alexgo-admin',
        code_challenge: codeChallenge,
        code_challenge_method: 'S256',
        password: params.password,
        redirect_uri: `${window.location.origin}/callback`,
        response_type: 'code',
        state,
        username: params.username,
      });

      // 302 已被 fetch 跟随到 /callback?code&state——提取后回放给路由
      const callbackUrl = new URL(finalUrl);
      const code = callbackUrl.searchParams.get('code');
      const returnedState = callbackUrl.searchParams.get('state');
      if (!code || returnedState !== state) {
        throw new Error('授权回调校验失败（缺 code 或 state 不匹配）');
      }

      await router.push({
        path: '/callback',
        query: { code, state: returnedState },
      });
    } catch (error: any) {
      sessionStorage.removeItem(OAUTH_SESSION_KEY);
      notification.error({
        description:
          error instanceof OAuthHttpError
            ? error.message
            : error?.message || '请检查账号密码后重试',
        duration: 3,
        message: '登录失败',
      });
    } finally {
      loginLoading.value = false;
    }

    return { userInfo };
  }

  async function logout(redirect: boolean = true) {
    try {
      await logoutApi();
    } catch {
      // 不做任何处理
    }
    resetAllStores();
    accessStore.setLoginExpired(false);

    // 回登录页带上当前路由地址
    await router.replace({
      path: LOGIN_PATH,
      query: redirect
        ? {
            redirect: encodeURIComponent(router.currentRoute.value.fullPath),
          }
        : {},
    });
  }

  async function fetchUserInfo() {
    const userInfo = await getUserInfoApi();
    userStore.setUserInfo(userInfo);
    return userInfo;
  }

  function $reset() {
    loginLoading.value = false;
  }

  return {
    $reset,
    authLogin,
    fetchUserInfo,
    loginLoading,
    logout,
  };
});
```

Run: `pnpm -C apps/web-antd typecheck`
Expected: **FAIL**，至少 4 处 `TS2307 Cannot find module`——`./oauth2`（index.ts）、`#/api/core/oauth2` 与 `#/utils/pkce`（store/auth.ts）、`#/views/_core/authentication/callback.vue`（core.ts 动态 import）。

- [ ] **Step 2: 创建三个新文件（GREEN）**

**(a) `src/utils/pkce.ts`**（全新）：

```ts
/**
 * PKCE（RFC 7636）工具——SSO 授权码流前端侧。
 * verifier/challenge 一律 base64url（无填充）；S256 用 Web Crypto。
 */

/** 字节序列 → base64url（去填充） */
function base64UrlEncode(bytes: ArrayBuffer | Uint8Array): string {
  const view = bytes instanceof Uint8Array ? bytes : new Uint8Array(bytes);
  let binary = '';
  for (const byte of view) {
    binary += String.fromCharCode(byte);
  }
  return btoa(binary)
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '');
}

/** N 字节随机 → base64url；默认 32B → 43 字符（RFC 7636 verifier 43–128） */
export function randomUrlSafe(byteLength: number = 32): string {
  const buf = new Uint8Array(byteLength);
  crypto.getRandomValues(buf);
  return base64UrlEncode(buf);
}

/** S256 挑战：base64url(SHA-256(verifier)) */
export async function deriveChallenge(verifier: string): Promise<string> {
  const digest = await crypto.subtle.digest(
    'SHA-256',
    new TextEncoder().encode(verifier),
  );
  return base64UrlEncode(digest);
}
```

**(b) `src/api/core/oauth2.ts`**（全新）：

```ts
import { useAppConfig } from '@vben/hooks';

/**
 * OAuth2 协议端点调用——必须用裸 fetch（不走 requestClient）：
 * ① RFC 6749 错误信封 {error,error_description} 不经过 vben 的 {code,data} 拦截器；
 * ② authorize/login 成功是 302，需要 redirect:'follow' 后从 resp.url 提取 code。
 */
const { apiURL } = useAppConfig(import.meta.env, import.meta.env.PROD);

/** PKCE 会话的 sessionStorage 键（登录页写入，/callback 消费后删除） */
export const OAUTH_SESSION_KEY = 'alexgo_oauth_session';

/** 登录页与 /callback 之间传递的 PKCE 上下文 */
export interface OAuthSession {
  verifier: string;
  state: string;
  /** 登录前目标地址（守卫 encodeURIComponent 后的 query.redirect，可为空串） */
  redirect: string;
}

export interface AuthorizeLoginParams {
  account_type?: string;
  client_id: string;
  code_challenge: string;
  code_challenge_method: 'S256';
  password: string;
  redirect_uri: string;
  response_type: 'code';
  state: string;
  username: string;
}

export interface OAuthTokenResponse {
  access_token: string;
  expires_in: number;
  refresh_token?: string;
  token_type: string;
}

/** 协议端点失败：HTTP 状态 + RFC 6749 信封 body */
export class OAuthHttpError extends Error {
  readonly body: null | Record<string, any>;

  constructor(status: number, body: null | Record<string, any>) {
    super(body?.error_description || body?.error || `HTTP ${status}`);
    this.name = 'OAuthHttpError';
    this.body = body;
  }
}

async function readBody(resp: Response): Promise<null | Record<string, any>> {
  try {
    return (await resp.json()) as null | Record<string, any>;
  } catch {
    return null;
  }
}

/**
 * POST /api/oauth2/authorize/login——成功 302 → redirect_uri?code&state，
 * fetch 跟随后返回最终 URL（/callback?code&state），由调用方提取并比对 state。
 */
export async function authorizeLoginApi(
  params: AuthorizeLoginParams,
): Promise<string> {
  const resp = await fetch(`${apiURL}/oauth2/authorize/login`, {
    body: JSON.stringify(params),
    headers: { 'Content-Type': 'application/json' },
    method: 'POST',
    redirect: 'follow',
  });
  if (!resp.ok) {
    throw new OAuthHttpError(resp.status, await readBody(resp));
  }
  return resp.url;
}

/** POST /api/oauth2/token——grant_type=authorization_code 换 token。 */
export async function exchangeTokenApi(params: {
  code: string;
  code_verifier: string;
  client_id: string;
  redirect_uri: string;
}): Promise<OAuthTokenResponse> {
  const resp = await fetch(`${apiURL}/oauth2/token`, {
    body: JSON.stringify({ grant_type: 'authorization_code', ...params }),
    headers: { 'Content-Type': 'application/json' },
    method: 'POST',
  });
  const body = await readBody(resp);
  if (!resp.ok) {
    throw new OAuthHttpError(resp.status, body);
  }
  return body as unknown as OAuthTokenResponse;
}
```

**(c) `src/views/_core/authentication/callback.vue`**（全新）：

```vue
<script setup lang="ts">
import { onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { LOGIN_PATH } from '@vben/constants';
import { preferences } from '@vben/preferences';
import { useAccessStore, useUserStore } from '@vben/stores';

import { notification } from 'ant-design-vue';

import { getAccessCodesApi, getUserInfoApi } from '#/api';
import type { OAuthSession } from '#/api/core/oauth2';
import {
  exchangeTokenApi,
  OAuthHttpError,
  OAUTH_SESSION_KEY,
} from '#/api/core/oauth2';
import { $t } from '#/locales';

const route = useRoute();
const router = useRouter();
const accessStore = useAccessStore();
const userStore = useUserStore();

/** 统一失败出口：清 PKCE 会话 → 中文提示 → 回登录页（登录页守卫会回带 redirect） */
function failLogin(reason: string) {
  sessionStorage.removeItem(OAUTH_SESSION_KEY);
  notification.error({
    description: reason,
    duration: 3,
    message: '登录失败',
  });
  router.replace(LOGIN_PATH);
}

onMounted(async () => {
  const code = typeof route.query.code === 'string' ? route.query.code : '';
  const state =
    typeof route.query.state === 'string' ? route.query.state : '';
  if (!code || !state) {
    failLogin('回调参数缺失');
    return;
  }

  let session: null | OAuthSession = null;
  try {
    const raw = sessionStorage.getItem(OAUTH_SESSION_KEY);
    session = raw ? (JSON.parse(raw) as OAuthSession) : null;
  } catch {
    session = null;
  }
  if (!session) {
    failLogin('登录会话已失效，请重新登录');
    return;
  }
  if (session.state !== state) {
    failLogin('state 校验失败');
    return;
  }

  try {
    const token = await exchangeTokenApi({
      code,
      client_id: 'alexgo-admin',
      code_verifier: session.verifier,
      redirect_uri: `${window.location.origin}/callback`,
    });
    accessStore.setAccessToken(token.access_token);
    sessionStorage.removeItem(OAUTH_SESSION_KEY);

    const [userInfo, accessCodes] = await Promise.all([
      getUserInfoApi(),
      getAccessCodesApi(),
    ]);
    userStore.setUserInfo(userInfo);
    accessStore.setAccessCodes(accessCodes);

    if (accessStore.loginExpired) {
      accessStore.setLoginExpired(false);
      return;
    }

    const target =
      (session.redirect && decodeURIComponent(session.redirect)) ||
      userInfo.homePath ||
      preferences.app.defaultHomePath;
    await router.push(target);

    if (userInfo.realName) {
      notification.success({
        description: `${$t('authentication.loginSuccessDesc')}:${userInfo.realName}`,
        duration: 3,
        message: $t('authentication.loginSuccess'),
      });
    }
  } catch (error: any) {
    failLogin(
      error instanceof OAuthHttpError
        ? error.message
        : error?.message || '请稍后重试',
    );
  }
});
</script>

<template>
  <div class="flex h-screen items-center justify-center">正在完成登录…</div>
</template>
```

时序说明（供执行者核对）：`authLogin` 只负责到 `router.push('/callback?code&state')` 为止——sessionStorage 里存着 verifier；fetch 跟随的那次 GET `/callback` 只取 URL，不触发 vue-router；callback.vue 挂载后读 session 校验 state → 换 token → 存 token → 拉用户信息+权限码 → 跳目标页（守卫此时首次 `generateAccess`）。任一步失败走 `failLogin` 回登录页；**授权码已被首次 exchange 消费（CAS，Task 9 语义），刷新 /callback 不会二次换发成功**，只会回到登录页。

- [ ] **Step 3: typecheck 确认通过**

Run: `pnpm -C apps/web-antd typecheck`
Expected: PASS（0 错误；`vue-tsc --noEmit --skipLibCheck` 退出码 0）。

- [ ] **Step 4: Commit**

```bash
git add apps/web-antd/src
git commit -m "feat(web-antd): 登录页切 OAuth2 授权码+PKCE SSO，新增 /callback 回调路由"
```

---

### Task 15: 端到端集成测试（password / 授权码全流 / 403 / 401 / 篡改 / 重放）

**Files:**
- Create: `modules/system/oauth2/e2e_test.go`（`//go:build integration` + `package oauth2`——与生产代码同包，直接消费根包 `NewMigrationSource`；手工 DI 装配，不经 fx）

**Interfaces:**
- Consumes:
  - Task 2：根包 `NewMigrationSource() *MigrationSource`（实现 `migrate.Source`）；种子 client `alexgo-admin`（secret 空=公开、grants `authorization_code,password,refresh_token`、account_types `system`、redirect 白名单含 `http://localhost:5173/callback`）；`model.Oauth2Client` 全字段（`ClientSecret string` 带 `gorm:"size:128"`、`Activity int`、无 tenant/deleted 列）。
  - Task 3：`NewCaptchaService(CaptchaDeps{Cfg: *config.Config; Redis optional}) *CaptchaService`。
  - Task 4：`NewUserAuthenticator(userRepo repository.UserRepository, permSvc PermissionService) oauth2svc.UserAuthenticator`——其 `Authenticate` 成功路径调 `EnsureUserRolePolicy`（起手 `loadPolicy` 重载、收尾 `savePolicy`，见 `modules/system/service/rbac.go:529`）。
  - Task 5：`MemberService.Authenticate(ctx, mobile, password, ip) (*oauth2svc.AuthIdentity, error)`（方法集结构满足 `MemberAuthenticator`）。
  - Task 7：`ClientService.Authenticate(ctx, clientID, secret)`——库中 secret 空直接放行（D10 公开规则）；`ClientService.Get(ctx, clientID)`。
  - Task 8：`NewUnifiedLogin(UnifiedDeps) UnifiedLogin`——`Auditor/JWTGen` 均 `optional:"true"` 且实现 nil 安全（缺省省略即可）。
  - Task 9/11：GET `/api/oauth2/authorize` 全过 → 302 `cfg.Auth.LoginPage?...`（缺省 `/auth/login`；`http.Redirect` 对相对 Location 不补 host，断言用前缀）；POST `/api/oauth2/authorize/login` JSON body（字段 `client_id/redirect_uri/state/response_type/username/password/account_type/code_challenge/code_challenge_method`）、**白名单校验先于凭据**、成功 302 `redirect_uri?code&state`；POST `/api/oauth2/token` 绑定 `TokenRequest`（`grant_type/client_id/client_secret/code/redirect_uri/code_verifier/username/password/account_type`）。
  - Task 10：`Exchange` 检查顺序（Authenticate 失败 401 `invalid_client` → `AllowsGrant` 400 → 机机公开 401「公开客户端不支持机机凭证」）；授权码**首次 exchange 尝试即 MarkRevoked**（重放 400 `invalid_grant`）；D9 机机 token 恒无 refresh。
  - Task 13：`ProtocolDeps{Captcha, Codes, Clients, Cfg, Login, Tokens, Validator}`、`Register(r gin.IRouter)` 挂 `/oauth2/{captcha,authorize,authorize/login,token,userinfo}`——挂在 `/api` 组下即 `/api/oauth2/*`。
  - Task 1：`cfg.Auth.Captcha.Enabled` / `cfg.Auth.LoginPage` / `cfg.Auth.Mode` / `cfg.Auth.AccessExpireHour` / `cfg.Auth.RefreshExpireDay`。
  - pkg（仓库现状，已核）：`auth.NewCasbinEnforcer(db) (*casbin.Enforcer, error)`、`auth.UserSub(tenantID uint64, userType int, name string, userID uint64) string`；`middleware.NewAuthMiddleware(middleware.AuthDeps{Cfg, Enforcer, Validator, ScopeLoader})`（公开规则放行 `/api/oauth2/*`；`obj = c.FullPath()`，Enforce(sub, obj, Method)）；`token.NewService(db, cfg) *Service`（同时实现 `token.Issuer` 与 `token.Validator`；Validate 对 admin 回查 `system_users.username`、member 回 `member_user.nickname`、type3 由 Task 6 扩展回 `ClientID`）；`migrate.NewRunner(db, cfg, []migrate.Source)`（`cfg.Migrate.Auto=true` 才跑；自带 `multiStatements` 迁移池）；`sysrepo.NewRepository/NewRoleRepository/NewMenuRepository/NewUserRoleRepository/NewRoleMenuRepository(db)`；`sysvc.NewPermissionService(roleRepo, menuRepo, userRoleRepo, roleMenuRepo, enforcer) PermissionService`；`membersvc.NewMemberService(repo, issuer, nil)`。
- Produces（终测，无下游任务消费——自审按 Review Focus 核对覆盖面）：`TestE2E_PasswordGrant_AdminAPI_200`、`TestE2E_MemberToken_AdminAPI_403`、`TestE2E_ClientCredentials_Public_401`、`TestE2E_ClientCredentials_M2M_200NoRefresh`、`TestE2E_AuthCode_FullFlow`、`TestE2E_RedirectTamper_400`、`TestE2E_CodeReplay_400`；辅助 `chdirRepoRoot(t *testing.T)`、`newE2E(t) (*gin.Engine, *casbin.Enforcer)`、`mustBcrypt(t, pw string) string`、`allowPing(t *testing.T, e *casbin.Enforcer)`、`postJSON(t, r *gin.Engine, path string, body any) *httptest.ResponseRecorder`、`doReq(t, r *gin.Engine, method, path, bearer string) *httptest.ResponseRecorder`、`pkcePair(t) (verifier, challenge string)`、`locParam(t, w *httptest.ResponseRecorder, key string) string`。

**裁决：**
1. **三迁移源**：`{NewMigrationSource(), system.NewMigrationSource(), member.NewMigrationSource()}`——e2e 横跨 `oauth2_*`（含种子 client）、`system_users/roles/menus`、`member_user` 三组表，缺一源即首查询报表不存在。
2. **e2e 数据自造自销**：`e2eadmin` / `13800000001` / `e2e-m2m·e2e-member·e2e-pub-m2m` 三个 client 由测试 seed + `t.Cleanup(purge)`，**绝不改种子行**——种子 client 被测试污染会让 dev 库行为漂移。
3. **受保护路由用桩** `/api/admin/system/e2e/ping`：真实 system 全量路由归 fx 接线红线（步骤 5），e2e 只验证 200/403 通断（中间件 + Casbin 判定真实走通）。
4. **`allowPing` 必须在拿到 token 之后**：登录内部 `EnsureUserRolePolicy` 起手 `loadPolicy` 从库重载，事前 `AddPolicy` 的内存策略会被整批洗掉——先换 token、后加策略。
5. **`UnifiedDeps.Auditor/JWTGen` 省略**：两者 `optional:"true"` 且实现 nil 审计安全（Task 8 裁决），e2e 不验审计与 jwt 模式（jwt 回滚语义由 Task 6/8 单测覆盖）。
6. **bcrypt 用 `bcrypt.MinCost`**：7 个用例各自造哈希，MinCost 把启动耗时压到毫秒级；校验侧 `CompareHashAndPassword` 与 cost 无关，不削弱覆盖。
7. **公开机机 401 需要 grants 含 `client_credentials`**：`Exchange` 先 `AllowsGrant`（400 `unsupported_grant_type`）后公开判定（401 `invalid_client`），白名单不含机机则到不了 401 分支——`e2e-pub-m2m` 的 grants 故意含机机。

- [ ] **Step 1: 写失败测试**

创建 `modules/system/oauth2/e2e_test.go`。**先只写测试**：import 块只含用例本体用到的 5 个包（辅助函数尚不存在，gin/casbin 等类型在用例里全部靠推断，不出现于 import），7 个用例引用尚不存在的辅助函数 → 编译失败即 RED。

```go
//go:build integration

package oauth2

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// tokenResp /oauth2/token 200 响应形状（service.TokenResponse 的 JSON 视图）。
type tokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
}

// TestE2E_PasswordGrant_AdminAPI_200：system 账号 password grant 换 token 后
// 可访问受 Casbin 保护的 admin 路由（Review Focus 3 正例）。
// allowPing 必须在拿 token 之后——登录内部 EnsureUserRolePolicy 会 loadPolicy
// 重载，事前加的内存策略会被洗掉（裁决 4）。
func TestE2E_PasswordGrant_AdminAPI_200(t *testing.T) {
	r, e := newE2E(t)
	w := postJSON(t, r, "/api/oauth2/token", map[string]string{
		"grant_type":   "password",
		"client_id":    "alexgo-admin",
		"username":     "e2eadmin",
		"password":     "pw123456",
		"account_type": "system",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("password grant 200, got %d body=%s", w.Code, w.Body.String())
	}
	var tok tokenResp
	if err := json.Unmarshal(w.Body.Bytes(), &tok); err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken == "" || tok.TokenType != "Bearer" {
		t.Fatalf("access_token/token_type 异常: %+v", tok)
	}
	allowPing(t, e)
	ping := doReq(t, r, http.MethodGet, "/api/admin/system/e2e/ping", tok.AccessToken)
	if ping.Code != http.StatusOK || !strings.Contains(ping.Body.String(), "pong") {
		t.Fatalf("admin ping 200, got %d body=%s", ping.Code, ping.Body.String())
	}
}

// TestE2E_MemberToken_AdminAPI_403：member 账号拿到合法 token，
// 但 userType=2 无 admin 策略 → 访问 admin 路由 403（Review Focus 3 跨账号类型冒用）。
func TestE2E_MemberToken_AdminAPI_403(t *testing.T) {
	r, _ := newE2E(t)
	w := postJSON(t, r, "/api/oauth2/token", map[string]string{
		"grant_type":    "password",
		"client_id":     "e2e-member",
		"client_secret": "e2e-member-secret",
		"username":      "13800000001",
		"password":      "mempw123",
		"account_type":  "member",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("member password grant 200, got %d body=%s", w.Code, w.Body.String())
	}
	var tok tokenResp
	if err := json.Unmarshal(w.Body.Bytes(), &tok); err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken == "" {
		t.Fatal("member access_token 空")
	}
	ping := doReq(t, r, http.MethodGet, "/api/admin/system/e2e/ping", tok.AccessToken)
	if ping.Code != http.StatusForbidden {
		t.Fatalf("member 访问 admin 路由 403, got %d body=%s", ping.Code, ping.Body.String())
	}
}

// TestE2E_ClientCredentials_Public_401：公开客户端（库中 secret 空）走机机
// 凭证 → 401 invalid_client（RFC 信封），绝不降级放行。
func TestE2E_ClientCredentials_Public_401(t *testing.T) {
	r, _ := newE2E(t)
	w := postJSON(t, r, "/api/oauth2/token", map[string]string{
		"grant_type": "client_credentials",
		"client_id":  "e2e-pub-m2m",
	})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("公开机机 401, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "invalid_client") {
		t.Fatalf("body 含 invalid_client, got %s", w.Body.String())
	}
}

// TestE2E_ClientCredentials_M2M_200NoRefresh：机密 client 机机凭证 200，
// 恒无 refresh_token（D9），且机机 token 不得进 admin API（403，
// Review Focus 3 的 client-token 用例）。
func TestE2E_ClientCredentials_M2M_200NoRefresh(t *testing.T) {
	r, _ := newE2E(t)
	w := postJSON(t, r, "/api/oauth2/token", map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     "e2e-m2m",
		"client_secret": "e2e-secret",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("机机 200, got %d body=%s", w.Code, w.Body.String())
	}
	var tok tokenResp
	if err := json.Unmarshal(w.Body.Bytes(), &tok); err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken == "" {
		t.Fatal("机机 access_token 空")
	}
	if tok.RefreshToken != "" {
		t.Fatalf("机机不该带 refresh_token, got %q", tok.RefreshToken)
	}
	ping := doReq(t, r, http.MethodGet, "/api/admin/system/e2e/ping", tok.AccessToken)
	if ping.Code != http.StatusForbidden {
		t.Fatalf("机机 token 访问 admin 路由 403, got %d body=%s", ping.Code, ping.Body.String())
	}
}

// TestE2E_AuthCode_FullFlow：授权码全流——GET authorize 302 登录页 →
// POST authorize/login 302 回跳带 code/state → POST token 换 JWT 全通。
func TestE2E_AuthCode_FullFlow(t *testing.T) {
	r, _ := newE2E(t)
	verifier, challenge := pkcePair(t)
	redirect := "http://localhost:5173/callback"

	q := url.Values{}
	q.Set("client_id", "alexgo-admin")
	q.Set("redirect_uri", redirect)
	q.Set("response_type", "code")
	q.Set("state", "st-full")
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	w := doReq(t, r, http.MethodGet, "/api/oauth2/authorize?"+q.Encode(), "")
	if w.Code != http.StatusFound {
		t.Fatalf("authorize 302, got %d body=%s", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); !strings.HasPrefix(loc, "/auth/login?") {
		t.Fatalf("Location = %q, want 前缀 /auth/login?", loc)
	}

	w2 := postJSON(t, r, "/api/oauth2/authorize/login", map[string]string{
		"client_id":             "alexgo-admin",
		"redirect_uri":          redirect,
		"state":                 "st-full",
		"response_type":         "code",
		"username":              "e2eadmin",
		"password":              "pw123456",
		"account_type":          "system",
		"code_challenge":        challenge,
		"code_challenge_method": "S256",
	})
	if w2.Code != http.StatusFound {
		t.Fatalf("authorize/login 302, got %d body=%s", w2.Code, w2.Body.String())
	}
	loc2 := w2.Header().Get("Location")
	if !strings.HasPrefix(loc2, redirect+"?") {
		t.Fatalf("Location = %q, want 前缀 %s?", loc2, redirect)
	}
	code := locParam(t, w2, "code")
	if st := locParam(t, w2, "state"); st != "st-full" {
		t.Fatalf("state = %q, want st-full", st)
	}

	w3 := postJSON(t, r, "/api/oauth2/token", map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     "alexgo-admin",
		"code":          code,
		"redirect_uri":  redirect,
		"code_verifier": verifier,
	})
	if w3.Code != http.StatusOK {
		t.Fatalf("exchange 200, got %d body=%s", w3.Code, w3.Body.String())
	}
	var tok tokenResp
	if err := json.Unmarshal(w3.Body.Bytes(), &tok); err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken == "" || tok.TokenType != "Bearer" {
		t.Fatalf("exchange 响应异常: %+v", tok)
	}
}

// TestE2E_RedirectTamper_400：redirect_uri 篡改——authorize 与 authorize/login
// 两侧都 400 且绝不带 Location（开放重定向防线，Review Focus 1）；
// login 侧凭据合法也必须 400，钉死「白名单先于凭据」的顺序。
func TestE2E_RedirectTamper_400(t *testing.T) {
	r, _ := newE2E(t)
	evil := "http://evil.example/cb"
	_, challenge := pkcePair(t)

	q := url.Values{}
	q.Set("client_id", "alexgo-admin")
	q.Set("redirect_uri", evil)
	q.Set("response_type", "code")
	q.Set("state", "st-evil")
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	w := doReq(t, r, http.MethodGet, "/api/oauth2/authorize?"+q.Encode(), "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("authorize 篡改 400, got %d body=%s", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc != "" {
		t.Fatalf("400 不得带 Location, got %q", loc)
	}
	if !strings.Contains(w.Body.String(), "invalid_request") {
		t.Fatalf("body 含 invalid_request, got %s", w.Body.String())
	}

	w2 := postJSON(t, r, "/api/oauth2/authorize/login", map[string]string{
		"client_id":             "alexgo-admin",
		"redirect_uri":          evil,
		"state":                 "st-evil",
		"response_type":         "code",
		"username":              "e2eadmin",
		"password":              "pw123456",
		"account_type":          "system",
		"code_challenge":        challenge,
		"code_challenge_method": "S256",
	})
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("authorize/login 篡改 400, got %d body=%s", w2.Code, w2.Body.String())
	}
	if loc := w2.Header().Get("Location"); loc != "" {
		t.Fatalf("400 不得带 Location, got %q", loc)
	}
	if !strings.Contains(w2.Body.String(), "invalid_request") {
		t.Fatalf("body 含 invalid_request, got %s", w2.Body.String())
	}
}

// TestE2E_CodeReplay_400：授权码一次性——首次 exchange 200 后重放
// 必须 400 invalid_grant（Review Focus 4；CAS 单次消费）。
func TestE2E_CodeReplay_400(t *testing.T) {
	r, _ := newE2E(t)
	verifier, challenge := pkcePair(t)
	redirect := "http://localhost:5173/callback"

	wLogin := postJSON(t, r, "/api/oauth2/authorize/login", map[string]string{
		"client_id":             "alexgo-admin",
		"redirect_uri":          redirect,
		"state":                 "st-replay",
		"response_type":         "code",
		"username":              "e2eadmin",
		"password":              "pw123456",
		"account_type":          "system",
		"code_challenge":        challenge,
		"code_challenge_method": "S256",
	})
	if wLogin.Code != http.StatusFound {
		t.Fatalf("authorize/login 302, got %d body=%s", wLogin.Code, wLogin.Body.String())
	}
	code := locParam(t, wLogin, "code")

	exchange := map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     "alexgo-admin",
		"code":          code,
		"redirect_uri":  redirect,
		"code_verifier": verifier,
	}
	w1 := postJSON(t, r, "/api/oauth2/token", exchange)
	if w1.Code != http.StatusOK {
		t.Fatalf("首次 exchange 200, got %d body=%s", w1.Code, w1.Body.String())
	}
	w2 := postJSON(t, r, "/api/oauth2/token", exchange)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("重放 400, got %d body=%s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "invalid_grant") {
		t.Fatalf("body 含 invalid_grant, got %s", w2.Body.String())
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `DB_DSN=$(grep 'dsn:' alexgo-server/configs/config.yaml | sed 's/.*dsn: "//;s/".*//') CGO_ENABLED=0 go test -tags=integration -run TestE2E ./modules/system/oauth2/ -v`

Expected: **FAIL（编译错误）**——`undefined: newE2E`、`undefined: postJSON`、`undefined: doReq`、`undefined: allowPing`、`undefined: pkcePair`、`undefined: locParam` 六个未定义符号（编译期失败，与 DSN 无关）。

- [ ] **Step 3: 补齐辅助函数（手工装配 e2e 栈）**

两件事：**(a)** 把 Step 1 文件的 import 块**整体替换**为下面的完整版本（覆盖用例 + 辅助函数的全部依赖）；**(b)** 在 7 个用例之后（文件末尾）追加辅助函数。

```go
import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"alexGo-cloud/modules/member"
	membermodel "alexGo-cloud/modules/member/model"
	memberrepo "alexGo-cloud/modules/member/repository"
	membersvc "alexGo-cloud/modules/member/service"
	"alexGo-cloud/modules/system/oauth2/controller"
	oauth2model "alexGo-cloud/modules/system/oauth2/model"
	oauth2repo "alexGo-cloud/modules/system/oauth2/repository"
	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
	"alexGo-cloud/modules/system"
	sysmodel "alexGo-cloud/modules/system/model"
	sysrepo "alexGo-cloud/modules/system/repository"
	sysvc "alexGo-cloud/modules/system/service"
	"alexGo-cloud/pkg/auth"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/middleware"
	"alexGo-cloud/pkg/migrate"
	"alexGo-cloud/pkg/token"
)
```

```go
// chdirRepoRoot 切到仓库根：Runner 迁移源路径 file://modules/... 相对进程 cwd
// 解析，本包位于 modules/system/oauth2（距根两级）。幂等：已在根则不上跳。
func chdirRepoRoot(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
		return
	}
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
}

// mustBcrypt 造测试口令哈希（MinCost：7 个用例各自造哈希，压启动耗时，裁决 6）。
func mustBcrypt(t *testing.T, pw string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return string(h)
}

// newE2E 搭最小可跑栈：切根 → 三迁移源建表（裁决 1）→ 清残留并 seed e2e 数据
// （裁决 2，绝不碰种子行）→ 手工 DI（不经 fx）→ 鉴权中间件 + 受保护桩 + 协议路由。
func newE2E(t *testing.T) (*gin.Engine, *casbin.Enforcer) {
	t.Helper()
	chdirRepoRoot(t)
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		t.Skip("DB_DSN not set; skip integration test")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("mysql unreachable: %v", err)
	}

	cfg := &config.Config{}
	cfg.Auth.Mode = "token"
	cfg.Auth.Captcha.Enabled = false
	cfg.Auth.LoginPage = "/auth/login"
	cfg.Auth.AccessExpireHour = 2
	cfg.Auth.RefreshExpireDay = 7
	cfg.Migrate.Auto = true
	cfg.Database.DSN = dsn

	runner := migrate.NewRunner(db, cfg, []migrate.Source{
		NewMigrationSource(),       // oauth2_client / oauth2_authorize_code + 种子 client
		system.NewMigrationSource(), // system_users / roles / menus / token 表
		member.NewMigrationSource(), // member_user
	})
	if err := runner.Run(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// 自造自销：跑前清上次中断残留，收尾再清——只动 e2e 专属行（裁决 2）。
	purge := func() {
		for _, sql := range []string{
			"DELETE FROM system_users WHERE username='e2eadmin'",
			"DELETE FROM member_user WHERE mobile='13800000001'",
			"DELETE FROM oauth2_client WHERE client_id IN ('e2e-m2m','e2e-member','e2e-pub-m2m')",
		} {
			_ = db.Exec(sql).Error
		}
	}
	purge()
	t.Cleanup(purge)

	if err := db.Create(&sysmodel.User{
		Username:     "e2eadmin",
		Nickname:     "e2e 管理员",
		PasswordHash: mustBcrypt(t, "pw123456"),
		Status:       1,
		TenantID:     0,
	}).Error; err != nil {
		t.Fatalf("seed e2eadmin: %v", err)
	}
	if err := db.Create(&membermodel.MemberUser{
		Mobile:   "13800000001",
		Nickname: "e2e 会员",
		Password: mustBcrypt(t, "mempw123"),
		Status:   1,
		TenantID: 0,
	}).Error; err != nil {
		t.Fatalf("seed e2e member: %v", err)
	}
	for _, c := range []oauth2model.Oauth2Client{
		{ClientID: "e2e-m2m", ClientSecret: mustBcrypt(t, "e2e-secret"),
			GrantTypes: "client_credentials", AccountTypes: "system", Scopes: "all", Activity: 1},
		{ClientID: "e2e-member", ClientSecret: mustBcrypt(t, "e2e-member-secret"),
			GrantTypes: "password,refresh_token", AccountTypes: "member", Scopes: "all", Activity: 1},
		// 公开机机：secret 空 + grants 含 client_credentials——先过 AllowsGrant
		// 才会到 401「公开客户端不支持机机凭证」分支（裁决 7）。
		{ClientID: "e2e-pub-m2m", ClientSecret: "",
			GrantTypes: "client_credentials", AccountTypes: "system", Scopes: "all", Activity: 1},
	} {
		if err := db.Create(&c).Error; err != nil {
			t.Fatalf("seed client %s: %v", c.ClientID, err)
		}
	}

	enforcer, err := auth.NewCasbinEnforcer(db)
	if err != nil {
		t.Fatalf("casbin: %v", err)
	}
	issuer := token.NewService(db, cfg) // 同时充当 Issuer 与 Validator
	captcha := oauth2svc.NewCaptchaService(oauth2svc.CaptchaDeps{Cfg: cfg})
	clients := oauth2svc.NewClientService(oauth2repo.NewClientRepository(db))
	codes := oauth2svc.NewAuthorizeCodeService(oauth2repo.NewAuthorizeCodeRepository(db))
	userAuth := sysvc.NewUserAuthenticator(
		sysrepo.NewRepository(db),
		sysvc.NewPermissionService(
			sysrepo.NewRoleRepository(db),
			sysrepo.NewMenuRepository(db),
			sysrepo.NewUserRoleRepository(db),
			sysrepo.NewRoleMenuRepository(db),
			enforcer,
		),
	)
	memberAuth := membersvc.NewMemberService(memberrepo.NewMemberRepository(db), issuer, nil)
	unified := oauth2svc.NewUnifiedLogin(oauth2svc.UnifiedDeps{
		Cfg:        cfg,
		Issuer:     issuer,
		Captcha:    captcha,
		UserAuth:   userAuth,
		MemberAuth: memberAuth,
	}) // Auditor/JWTGen 省略——optional 且 nil 安全（裁决 5）
	tokens := oauth2svc.NewTokenService(oauth2svc.TokenServiceDeps{
		Cfg: cfg, Clients: clients, Codes: codes, Login: unified, Issuer: issuer,
	})
	proto := controller.NewProtocolController(controller.ProtocolDeps{
		Captcha: captcha, Codes: codes, Clients: clients, Cfg: cfg,
		Login: unified, Tokens: tokens, Validator: issuer,
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api", middleware.NewAuthMiddleware(middleware.AuthDeps{
		Cfg:       cfg,
		Enforcer:  enforcer,
		Validator: issuer,
	})) // ScopeLoader 省略（nil 跳过数据权限装载）
	// 受保护桩：真实 system 全量路由归 fx 接线红线（裁决 3），
	// e2e 只验中间件 + Casbin 的 200/403 通断。
	api.GET("/admin/system/e2e/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": "pong"})
	})
	proto.Register(api)
	return router, enforcer
}

// allowPing 给 e2eadmin 授受保护桩的读策略。**必须在拿到 token 之后调用**：
// 登录内部 EnsureUserRolePolicy 起手 loadPolicy 重载，事前加的内存策略会被
// 整批洗掉（裁决 4）。重复 AddPolicy 幂等（已存在返回 false，不报错）。
func allowPing(t *testing.T, e *casbin.Enforcer) {
	t.Helper()
	sub := auth.UserSub(0, 1, "e2eadmin", 0) // tenant 0 / UserTypeAdmin=1 / e2eadmin
	if _, err := e.AddPolicy(sub, "/api/admin/system/e2e/ping", "GET"); err != nil {
		t.Fatalf("AddPolicy: %v", err)
	}
}

// postJSON POST JSON 体（Content-Type: application/json），返回响应记录。
func postJSON(t *testing.T, r *gin.Engine, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// doReq 发任意方法请求；bearer 非空时带 Authorization: Bearer。
func doReq(t *testing.T, r *gin.Engine, method, path, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	r.ServeHTTP(w, req)
	return w
}

// pkcePair 生成 RFC 7636 对：32B crypto/rand 的 base64url verifier +
// 其 SHA-256 的 base64url challenge（S256）。
func pkcePair(t *testing.T) (verifier, challenge string) {
	t.Helper()
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	verifier = base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge
}

// locParam 从响应 Location 取 query 参数（无 Location / 缺参即 Fatal——
// 这两个断言本身就是用例意图的一部分）。
func locParam(t *testing.T, w *httptest.ResponseRecorder, key string) string {
	t.Helper()
	loc := w.Header().Get("Location")
	if loc == "" {
		t.Fatal("响应无 Location")
	}
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("解析 Location %q: %v", loc, err)
	}
	v := u.Query().Get(key)
	if v == "" {
		t.Fatalf("Location 缺参数 %q: %q", key, loc)
	}
	return v
}
```

- [ ] **Step 4: 运行确认通过**

Run: `DB_DSN=$(grep 'dsn:' alexgo-server/configs/config.yaml | sed 's/.*dsn: "//;s/".*//') CGO_ENABLED=0 go test -tags=integration -run TestE2E ./modules/system/oauth2/ -v`

Expected: **PASS——7 例全绿**（`--- PASS: TestE2E_...` ×7）。DSN 未设置时为 7×`SKIP`，不计入门禁；门禁以带 DSN 的真跑为准。

- [ ] **Step 5: 全量回归 + 接线红线**

Run: `CGO_ENABLED=0 go test ./... && CGO_ENABLED=0 go test ./alexgo-server/cmd/ ./modules/member/cmd/`

Expected: 全绿（e2e 带 `integration` tag 不参与本命令；两 main 的 fx 干跑不受本任务影响——本任务零生产代码改动）。

- [ ] **Step 6: Commit**

```bash
git add modules/system/oauth2/e2e_test.go
git commit -m "feat(oauth2): 端到端集成测试（password/授权码全流/403/401/篡改/重放）"
```

---

### Task 16: OAuth2 客户端管理菜单种子 + 权限码扫描测试 + 前端管理页

**Files:**
- Test: `modules/system/service/permission_routes_test.go`（追加扫描守护 1 例）
- Create: `modules/system/migrations/20261011000003_oauth2_client_menu.up.sql`
- Create: `modules/system/migrations/20261011000003_oauth2_client_menu.down.sql`
- Test: `modules/system/infra/migrations/migrate_test.go`（追加直跑双次幂等 1 例）
- Create: `apps/web-antd/src/views/system/Oauth2ClientsPage.vue`
- Modify: `apps/web-antd/src/api/admin.ts`（文件末尾追加 OAuth2 客户端两接口 + 四函数）

**Interfaces:**
- Consumes: Task 13 已注册的 `"oauth2:client": {"/api/admin/oauth2/clients", "/api/admin/oauth2/clients/*"}`（`permission_routes.go`——本任务**不碰**该文件，只依赖它先落地）与守护例 `TestPermissionRoutes_Oauth2ClientRegistered`；Task 13 admin 端点语义（`GET /admin/oauth2/clients?page&size` → 200 `{data,total}`，page/size 归正 1/20、size 上限 200；`POST` → `{data}` 且 `createClientResponse` 明文 `client_secret` 仅创建回显一次；`PUT /clients/:id` → `{data}` 且 secret 留空=保留旧值、`json:"-"` 恒不回显；`DELETE /clients/:id` → `{"status":"ok"}`）；Task 7 `model.Oauth2Client` JSON 字段名（`id/client_id/client_name/website/grant_types/redirect_uris/account_types/scopes/activity/created_at/updated_at`，`ClientSecret json:"-"`）；codegen 种子模板 `modules/system/migrations/20261011000002_codegen_menus.up.sql`/`.down.sql`（COALESCE 父目录、按业务键 NOT EXISTS、按钮 JOIN 子表、role_menus JOIN roles、down 两段 DELETE）；`modules/system/infra/migrations/migrate_test.go` 的 `chdirRepoRoot/openMultiStmtDB/runSystem` 夹具与 `TestInfraSeedMenus_SQLFileExecutedTwiceStable` 直跑双次模式；`permission_routes_test.go` 的 `TestInfraSeed_PermsRegistered` 扫描模式（`os.ReadDir` + 文件名过滤 + `permRe` + `checked==0` 自失效断言）；`localPage<T>(rows, currentPage, pageSize): T[]`（`#/utils/local-page`）与 `SystemPostsPage` 的 size=200+本地切页网格模式（`requestClient` 拦截器解包 `{data,total}` 后 **total 丢失**，`request.ts:84-90` 的 `dataField` 只取 `response.data`）；`SystemMenusPage` 编辑模式（`editingId`/`onCreate`/`onEdit` 的 setState→open→setValues 入队顺序注释、`onOpenChange` 分支、`handleSubmit` 分支）；`v-access:code` **精确匹配**语义（`packages/effects/access/src/directive.ts` → `hasAccessByCodes` 是 `Set.has` 逐字比对，无通配展开；`UserPermCodes` 返回 `UserMenus` 的原始 `permission` 且**不过滤 type** → 按钮行权限码会到达前端；admin 角色由迁移 role_menus JOIN + seeder `SetRoleMenus` 全量绑定兜底）；`access.ts` 的 `pageMap = import.meta.glob('../views/**/*.vue')` + `normalizeViewPath`（去 `../` 前缀、补 `.vue` 后缀）→ component 值 `views/system/Oauth2ClientsPage` 自动解析，**零路由文件改动**；表单组件 `'VbenInputPassword'`/`'Textarea'`（`BaseFormComponentType`）；门禁 `pnpm -C apps/web-antd typecheck`。
- Produces（终测消费——本任务为计划最后一个功能任务）：

```go
// modules/system/service/permission_routes_test.go 追加：
func TestOauth2Seed_PermsRegistered(t *testing.T)
// 扫 modules/system/migrations/*oauth2_client_menu* 内 '(oauth2:[a-z0-9_:]+)' 字面量，
// 每个权限码前缀必须在 permissionRoutes；checked==0 → t.Error 自失效。

// modules/system/infra/migrations/migrate_test.go 追加：
const oauth2SeedSQLPath = "modules/system/migrations/20261011000003_oauth2_client_menu.up.sql"
func oauth2SeedMenuCount(t *testing.T, raw *sql.DB) int64
// 口径：(path IN ('/oauth2','/oauth2/clients') AND type IN ('dir','menu') AND deleted=0)
//        OR (permission LIKE 'oauth2:%' AND deleted=0) → 恒 5（1目录+1页面+3按钮）
func TestOauth2MenuSeed_SQLFileExecutedTwiceStable(t *testing.T)
```

```ts
// apps/web-antd/src/api/admin.ts 追加（末尾）：
export interface Oauth2Client { id; client_id; client_name; website; grant_types;
  redirect_uris; account_types; scopes; activity; created_at: string; updated_at: string }
export interface Oauth2ClientPayload { client_id; client_secret?: string; client_name;
  website; grant_types; redirect_uris; account_types; scopes; activity: number }
export function listClients(): Promise<Oauth2Client[]>          // GET /admin/oauth2/clients?page=1&size=200
export function createClient(payload: Oauth2ClientPayload): Promise<Oauth2Client & { client_secret?: string }>
export function updateClient(id: number, payload: Oauth2ClientPayload): Promise<Oauth2Client>
export function deleteClient(id: number): Promise<{ status: string }>
// apps/web-antd/src/views/system/Oauth2ClientsPage.vue：
// defineOptions name 'Oauth2ClientsPage'；本地切页网格 + 新建/编辑/删除三按钮
// v-access 精确码 ['oauth2:client:create'|'oauth2:client:update'|'oauth2:client:delete']；
// 新建响应含 client_secret → AntdModal.info 一次性回显。
```

**裁决：**
1. **`permission_routes.go` 不在本任务改**——`oauth2:client` 条目与守护例由 Task 13 落地（plan 顺序保证 T13 先执行）；本任务的扫描测试因此在 SQL 写完后即可绿。菜单种子文件头注释复述该依赖（对齐 codegen 种子的死权限警告注释）。
2. **按钮独立权限码 + 前端精确匹配**（对齐 codegen 种子结构，但前端用**逐字码**而非页面码复用）：页面 `oauth2:client`，按钮 `oauth2:client:create/update/delete`。机制已核实——`UserMenus`（`rbac.go:324`）返回角色全部菜单**含按钮行** → `UserPermCodes`（`rbac.go:354`）原样下发 → `Set.has` 逐字命中；admin 绑定靠迁移 role_menus JOIN（fresh 库 0 行）+ seeder `SetRoleMenus` 全量兜底。若执行期发现按钮码未下发（缺绑定），回退方案：三按钮改挂页面码 `oauth2:client`（现仓库 SystemMenusPage/SystemPostsPage 即此形态），种子按钮行保留（服务扫描测试）。
3. **unwrap 丢 total → `size=200` + `localPage` 本地切页**：`request.ts` 的 `dataField` 把 `{data,total}` 解成裸数组，total 不可达；controller `size>200` 截断 200，客户端注册数远小于 200，一次拉满可接受。`listClients` 走 URL 内联 query（对齐 `listOperateLogs` 先例）。
4. **编辑表单 `client_id` 可改**：后端 `Update` 允许（service 只校验非空），撞唯一索引 → 500 由拦截器 toast 兜底——与「可删除种子 client」同级的管理员操作，不引入仓库无先例的 `formApi.updateSchema` 禁用态。`client_secret` 编辑留空=不改（Task 7 Update 语义），placeholder 写明。
5. **零路由改动**：component 值 `views/system/Oauth2ClientsPage` 经 `normalizeViewPath`（去 `../`、补 `.vue`）与 glob key 精确对上——不改任何路由文件（对齐 `access.ts:18` glob 机制）。
6. **CSV/换行文本输入不做多选组件**：`grant_types`/`account_types` 用 Input（逗号分隔占位符）、`redirect_uris` 用 Textarea（每行一个占位符）——与库中字符串列存储一致，YAGNI。
7. **新建成功一次性回显明文 secret**：`createClient` 返回体含 `client_secret`（Task 13 `createClientResponse`），页面用 `AntdModal.info` 展示并提示仅此一次；List/Update 恒不回显，错过即不可再取（spec §3.1）。

- [ ] **Step 1: 写失败测试（后端两处）**

**(a) `modules/system/service/permission_routes_test.go` 追加**（import 块无需变——`os/path/filepath/regexp/strings/testing` 已齐）：

```go
// 扫 oauth2 菜单种子 SQL：出现的 oauth2:client* 权限码前缀必须都已注册路由——
// 否则 Task 16 种子产生的页面/按钮在 Casbin 无 policy，admin 403。
// 正则要求冒号：目录权限字面量 'oauth2' 与 LIKE 'oauth2:%' 天然排除
//（前者无冒号不匹配；后者冒号后是 %，不在 [a-z0-9_:] 字符类内）。
func TestOauth2Seed_PermsRegistered(t *testing.T) {
	migPath := filepath.Join("..", "migrations")
	entries, err := os.ReadDir(migPath)
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	permRe := regexp.MustCompile(`'(oauth2:[a-z0-9_:]+)'`)
	checked := 0
	for _, e := range entries {
		if !strings.Contains(e.Name(), "oauth2_client_menu") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(migPath, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range permRe.FindAllStringSubmatch(string(b), -1) {
			checked++
			if _, ok := permissionRoutes[permPrefix(m[1])]; !ok {
				t.Errorf("%s 中权限码 %q 前缀 %q 未注册路由", e.Name(), m[1], permPrefix(m[1]))
			}
		}
	}
	if checked == 0 {
		t.Error("oauth2 菜单迁移未扫到任何权限码——文件名或正则失配，测试失效")
	}
}
```

**(b) `modules/system/infra/migrations/migrate_test.go` 文件末尾追加**（import 块无需变——`os/sql/testing` 已齐）：

```go
const oauth2SeedSQLPath = "modules/system/migrations/20261011000003_oauth2_client_menu.up.sql"

// oauth2SeedMenuCount 与 runner 双跑测试同口径的种子行计数：
// 目录/页面按 path+type，按钮按 permission（页面两条件 OR 命中去重，恒计 1 行）。
func oauth2SeedMenuCount(t *testing.T, raw *sql.DB) int64 {
	t.Helper()
	var n int64
	if err := raw.QueryRow(`SELECT COUNT(*) FROM menus
		WHERE (path IN ('/oauth2','/oauth2/clients') AND type IN ('dir','menu') AND deleted = 0)
		   OR (permission LIKE 'oauth2:%' AND deleted = 0)`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// 直跑 up.sql 原文两遍：对 NOT EXISTS 守卫的真断言——守卫被删时本测试必须失败
//（runner 双跑测不出，评审 Important#3 的既定模式，抄 TestInfraSeedMenus_SQLFileExecutedTwiceStable）。
func TestOauth2MenuSeed_SQLFileExecutedTwiceStable(t *testing.T) {
	raw := openMultiStmtDB(t)
	_, runner := runSystem(t)
	if err := runner.Run(); err != nil {
		t.Fatalf("baseline run: %v", err)
	}
	src, err := os.ReadFile(oauth2SeedSQLPath)
	if err != nil {
		t.Fatal(err)
	}
	before := oauth2SeedMenuCount(t, raw)
	for i := 1; i <= 2; i++ {
		if _, err := raw.Exec(string(src)); err != nil {
			t.Fatalf("直跑 SQL #%d: %v", i, err)
		}
	}
	after := oauth2SeedMenuCount(t, raw)
	if after != before {
		t.Errorf("直跑两遍行数变化: %d → %d", before, after)
	}
	if after != 5 {
		t.Errorf("菜单数 = %d, want 5（目录+页面+3按钮）", after)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `CGO_ENABLED=0 go test ./modules/system/service/ -run TestOauth2Seed -v`

Expected: FAIL —— `--- FAIL: TestOauth2Seed_PermsRegistered`，错误 `oauth2 菜单迁移未扫到任何权限码——文件名或正则失配，测试失效`（种子文件尚不存在，`checked==0`）。

Run（DSN 可达时）: `DB_DSN=$(grep 'dsn:' alexgo-server/configs/config.yaml | sed 's/.*dsn: "//;s/".*//') go test -tags=integration ./modules/system/infra/migrations/ -run TestOauth2MenuSeed -v`

Expected: FAIL —— `open modules/system/migrations/20261011000003_oauth2_client_menu.up.sql: no such file or directory`；若 `DB_DSN` 未配置则 SKIP（集成 RED 以本条为准，门禁在 Step 4 统一跑）。

- [ ] **Step 3: 写迁移 SQL**

`modules/system/migrations/20261011000003_oauth2_client_menu.up.sql`：

```sql
-- OAuth2 客户端管理菜单 + 按钮（spec §4.5 管理端配套：复用 codegen 种子模式，挂系统管理下）。
-- 幂等：全部 NOT EXISTS 守卫（目录按 path+type，页面按 path，按钮按 permission），可重复跑。
-- 权限码前缀 oauth2:client 必须已在 permission_routes.go 注册（Task 13），
-- 否则本迁移产生的页面/按钮是死权限（403）。
-- 表结构对齐 20261009000001（menus：type 'dir'/'menu'/'button'、sort、deleted、tenant_id=0）。
-- 父目录定位：系统管理 由 StartSeeder 在迁移之后创建（fresh 库），故 parent 用
-- COALESCE 兜底 0——已有库挂系统管理下，fresh 库落根（seed 尾部 SetRoleMenus 全量绑定，
-- 403 闭环不依赖嵌套层级）。模式抄 20261011000002 codegen_menus。

-- 1) 目录「OAuth2」（挂在系统管理下；fresh 库父不存在则 parent=0）
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, COALESCE((SELECT p.`id` FROM `menus` p WHERE p.`tenant_id`=0 AND p.`name`='系统管理' AND p.`type`='dir' AND p.`deleted`=0 LIMIT 1), 0),
       'dir', 'OAuth2', '/oauth2', '', 'lucide:shield-check', 'oauth2', 91, 1, 0, NOW(), NOW()
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND `path`='/oauth2' AND `type`='dir' AND `deleted`=0);

-- 2) 页面菜单「客户端管理」（挂在目录下；component 由 access.ts glob 自动解析，零路由改动）
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', '客户端管理', '/oauth2/clients', 'views/system/Oauth2ClientsPage', 'lucide:key-round', 'oauth2:client', 1, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/oauth2' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND `path`='/oauth2/clients' AND `deleted`=0);

-- 3) 按钮：新建 / 编辑 / 删除（挂在页面菜单下；前端 v-access 逐字匹配这些码）
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, m.`id`, 'button', t.`name`, '', '', '', t.`perm`, t.`sort`, 1, 0, NOW(), NOW()
FROM `menus` m
JOIN (
  SELECT '新建' AS `name`, 'oauth2:client:create' AS perm, 1 AS `sort`
  UNION ALL SELECT '编辑', 'oauth2:client:update', 2
  UNION ALL SELECT '删除', 'oauth2:client:delete', 3
) t
WHERE m.`tenant_id`=0 AND m.`permission`='oauth2:client' AND m.`type`='menu' AND m.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` b WHERE b.`tenant_id`=0 AND b.`permission`=t.`perm` AND b.`deleted`=0);

-- 4) admin 角色挂菜单（抄 codegen ③：JOIN roles 而非标量子查询——
--    fresh 库尚无 admin 角色时产出 0 行，由 seed 尾部 SetRoleMenus 全量兜底）
INSERT INTO `role_menus` (`role_id`,`menu_id`,`tenant_id`)
SELECT r.`id`, m.`id`, 0
FROM `menus` m
JOIN `roles` r ON r.`tenant_id`=0 AND r.`code`='admin' AND r.`deleted`=0
WHERE m.`tenant_id`=0 AND m.`deleted`=0
  AND (m.`path` IN ('/oauth2','/oauth2/clients') OR m.`permission` LIKE 'oauth2:%')
  AND NOT EXISTS (SELECT 1 FROM `role_menus` rm WHERE rm.`tenant_id`=0 AND rm.`role_id`=r.`id` AND rm.`menu_id`=m.`id`);
```

`modules/system/migrations/20261011000003_oauth2_client_menu.down.sql`：

```sql
-- 逆序删除：先角色关联，再按钮/页面/目录（模式抄 20261011000002 codegen down）。
DELETE rm FROM `role_menus` rm
JOIN `menus` m ON m.`id` = rm.`menu_id`
WHERE m.`permission` LIKE 'oauth2:%' OR m.`path` IN ('/oauth2','/oauth2/clients');

DELETE FROM `menus`
WHERE `permission` LIKE 'oauth2:%'
   OR `path` IN ('/oauth2','/oauth2/clients');
```

- [ ] **Step 4: 运行确认通过**

Run: `CGO_ENABLED=0 go test ./modules/system/service/ -run "TestOauth2Seed|TestPermissionRoutes" -v`

Expected: PASS —— `TestOauth2Seed_PermsRegistered` 扫到 5 处字面量（up.sql：页面码 1 + 按钮码 3 + 按钮 WHERE 引用 1；down.sql 仅 LIKE 与 path 字面量不入正则），全部 `permPrefix → oauth2:client` 命中 Task 13 注册的键。

Run: `DB_DSN=$(grep 'dsn:' alexgo-server/configs/config.yaml | sed 's/.*dsn: "//;s/".*//') go test -tags=integration ./modules/system/infra/migrations/ -run "TestOauth2MenuSeed|TestInfraSeedMenus" -v`

Expected: PASS —— `TestOauth2MenuSeed_SQLFileExecutedTwiceStable`：baseline 后 `before=5`，直跑两遍 `after=5`；同包既有 codegen 断言（恒 5）不受影响（谓词仅 `infra:codegen%`）。

Run: `CGO_ENABLED=0 go test ./... && CGO_ENABLED=0 go test ./alexgo-server/cmd/ ./modules/member/cmd/`

Expected: 全绿（本任务未动生产代码，仅测试与 SQL；e2e 带 `integration` tag 不参与本命令）。

- [ ] **Step 5: Commit（后端）**

```bash
git add modules/system/service/permission_routes_test.go modules/system/migrations/20261011000003_oauth2_client_menu.up.sql modules/system/migrations/20261011000003_oauth2_client_menu.down.sql modules/system/infra/migrations/migrate_test.go
git commit -m "feat(system): OAuth2 客户端管理菜单种子 + 权限码注册扫描"
```

- [ ] **Step 6: 前端 RED（写页面，typecheck 必失败）**

创建 `apps/web-antd/src/views/system/Oauth2ClientsPage.vue`（完整内容）：

```vue
<script lang="ts" setup>
import type { Oauth2Client, Oauth2ClientPayload } from '#/api/admin';
import type { VxeGridProps } from '#/adapter/vxe-table';

import { Page, useVbenModal } from '@vben/common-ui';

import { Button, message, Modal as AntdModal } from 'ant-design-vue';

import { ref } from 'vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import {
  createClient,
  deleteClient,
  listClients,
  updateClient,
} from '#/api/admin';
import { localPage } from '#/utils/local-page';

defineOptions({ name: 'Oauth2ClientsPage' });

const gridOptions: VxeGridProps<Oauth2Client> = {
  columns: [
    { field: 'id', title: 'ID', width: 70 },
    { field: 'client_id', title: 'Client ID', minWidth: 160 },
    { field: 'client_name', title: '名称', minWidth: 140 },
    { field: 'grant_types', title: '授权模式', minWidth: 200 },
    { field: 'account_types', title: '账号类型', width: 130 },
    {
      field: 'activity',
      formatter: ({ cellValue }) =>
        Number(cellValue) === 1 ? '启用' : '停用',
      title: '状态',
      width: 80,
    },
    {
      field: 'operation',
      fixed: 'right',
      slots: { default: 'actions' },
      title: '操作',
      width: 150,
    },
  ],
  height: 'auto',
  keepSource: true,
  pagerConfig: {},
  proxyConfig: {
    ajax: {
      query: async ({ page }) => {
        // {data,total} 被 requestClient 拦截器解包后 total 丢失：
        // 一次拉全量（controller size 上限 200）+ 本地切页，对齐 SystemPostsPage。
        const rows = await listClients();
        return {
          items: localPage(rows, page.currentPage, page.pageSize),
          total: rows.length,
        };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: true, zoom: true },
};

const [Grid, gridApi] = useVbenVxeGrid({ gridOptions });

/** null=新建；非空=正在编辑该 id 的客户端 */
const editingId = ref<number | null>(null);

const [Modal, modalApi] = useVbenModal({
  fullscreenButton: false,
  title: '新建客户端',
  onOpenChange(isOpen: boolean) {
    if (isOpen) formApi.resetForm();
    else editingId.value = null;
  },
  onConfirm: async () => {
    try {
      await formApi.validateAndSubmitForm();
    } catch {
      // 拦截器已 toast，弹窗保持打开
    }
  },
});

const [Form, formApi] = useVbenForm({
  handleSubmit: async (values) => {
    const payload = values as Oauth2ClientPayload;
    if (editingId.value) {
      await updateClient(editingId.value, payload);
      message.success('保存成功');
    } else {
      const created = await createClient(payload);
      message.success('创建成功');
      if (created.client_secret) {
        // 明文 secret 仅创建响应回显一次（spec §3.1），错过即无法再取
        AntdModal.info({
          title: '客户端密钥（仅显示一次）',
          content: created.client_secret,
        });
      }
    }
    modalApi.close();
    gridApi.reload();
  },
  schema: [
    {
      component: 'Input',
      componentProps: { placeholder: '如 third-party-crm' },
      fieldName: 'client_id',
      label: 'Client ID',
      rules: 'required',
    },
    {
      component: 'VbenInputPassword',
      componentProps: { placeholder: '新建留空=公开客户端；编辑留空=不改' },
      fieldName: 'client_secret',
      label: '客户端密钥',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '如 第三方 CRM' },
      fieldName: 'client_name',
      label: '名称',
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { placeholder: 'https://example.com' },
      fieldName: 'website',
      label: '主页',
      defaultValue: '',
    },
    {
      component: 'Textarea',
      componentProps: {
        placeholder:
          '逗号分隔：authorization_code,client_credentials,password,refresh_token',
      },
      fieldName: 'grant_types',
      label: '授权模式',
      rules: 'required',
    },
    {
      component: 'Textarea',
      componentProps: {
        placeholder: '每行一个，精确匹配：http://localhost:5173/callback',
      },
      fieldName: 'redirect_uris',
      label: '回跳白名单',
      defaultValue: '',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '逗号分隔：system,member' },
      fieldName: 'account_types',
      label: '账号类型',
      rules: 'required',
    },
    {
      component: 'Input',
      defaultValue: 'all',
      fieldName: 'scopes',
      label: 'Scope',
    },
    {
      component: 'RadioGroup',
      componentProps: {
        options: [
          { label: '启用', value: 1 },
          { label: '停用', value: 0 },
        ],
      },
      defaultValue: 1,
      fieldName: 'activity',
      label: '状态',
      rules: 'required',
    },
  ],
  showDefaultActions: false,
});

function onDelete(row: Oauth2Client) {
  AntdModal.confirm({
    title: `确认删除客户端「${row.client_id}」？`,
    onOk: async () => {
      try {
        await deleteClient(row.id);
      } catch {
        return; // 拦截器已 toast
      }
      message.success('删除成功');
      gridApi.reload();
    },
  });
}

function onCreate() {
  editingId.value = null;
  modalApi.setState({ title: '新建客户端' });
  modalApi.open();
}

function onEdit(row: Oauth2Client) {
  editingId.value = row.id;
  modalApi.setState({ title: '编辑客户端' });
  modalApi.open();
  // open() 同步触发 onOpenChange 的 resetForm（其 await getForm 先入队），
  // 随后的 setValues 在其后入队 → 先重置再回填，顺序有保证
  void formApi.setValues({
    account_types: row.account_types,
    activity: row.activity,
    client_id: row.client_id,
    client_secret: '', // 留空=不改（后端 Update 语义）
    client_name: row.client_name,
    grant_types: row.grant_types,
    redirect_uris: row.redirect_uris,
    scopes: row.scopes,
    website: row.website,
  });
}
</script>

<template>
  <Page auto-content-height>
    <Grid>
      <template #toolbar-tools>
        <Button
          class="mr-2"
          type="primary"
          v-access:code="['oauth2:client:create']"
          @click="onCreate"
        >
          新建客户端
        </Button>
      </template>
      <template #actions="{ row }">
        <Button
          size="small"
          type="link"
          v-access:code="['oauth2:client:update']"
          @click="onEdit(row)"
        >
          编辑
        </Button>
        <Button
          danger
          size="small"
          type="link"
          v-access:code="['oauth2:client:delete']"
          @click="onDelete(row)"
        >
          删除
        </Button>
      </template>
    </Grid>
    <Modal>
      <Form />
    </Modal>
  </Page>
</template>
```

Run: `pnpm -C apps/web-antd typecheck`

Expected: FAIL —— 至少 6 个 `TS2305: Module '"#/api/admin"' has no exported member '…'`（`Oauth2Client`/`Oauth2ClientPayload`/`listClients`/`createClient`/`updateClient`/`deleteClient`），可另有级联错误（`VxeGridProps<Oauth2Client>` 类型未知）——全部随 Step 7 消失。

- [ ] **Step 7: 补 API（GREEN）**

`apps/web-antd/src/api/admin.ts` **文件末尾追加**：

```ts
// ---- OAuth2 客户端 ----
export interface Oauth2Client {
  id: number;
  client_id: string;
  client_name: string;
  website: string;
  grant_types: string; // 逗号分隔
  redirect_uris: string; // 换行分隔，精确匹配
  account_types: string; // 逗号分隔 system,member
  scopes: string;
  activity: number; // 1 启用 0 停用
  created_at: string;
  updated_at: string;
}

export interface Oauth2ClientPayload {
  client_id: string;
  client_secret?: string; // 明文；编辑留空=不改（后端 Update 语义）
  client_name: string;
  website: string;
  grant_types: string;
  redirect_uris: string;
  account_types: string;
  scopes: string;
  activity: number;
}

// page/size 走 URL 内联 query（对齐 listOperateLogs 风格）；
// 响应 {data,total} 被拦截器解出 data 后 total 丢失 → 页面本地切页，这里一次拉满上限 200。
export function listClients() {
  return requestClient.get<Oauth2Client[]>(
    '/admin/oauth2/clients?page=1&size=200',
  );
}

// 明文 secret 仅创建响应回显一次（createClientResponse），List/Update 恒不回。
export function createClient(payload: Oauth2ClientPayload) {
  return requestClient.post<Oauth2Client & { client_secret?: string }>(
    '/admin/oauth2/clients',
    payload,
  );
}

export function updateClient(id: number, payload: Oauth2ClientPayload) {
  return requestClient.put<Oauth2Client>(`/admin/oauth2/clients/${id}`, payload);
}

export function deleteClient(id: number) {
  return requestClient.delete<{ status: string }>(
    `/admin/oauth2/clients/${id}`,
  );
}
```

Run: `pnpm -C apps/web-antd typecheck`

Expected: PASS —— 0 errors（Step 6 的 TS2305 与级联错误全部消失；仓库当前 typecheck 基线绿）。

- [ ] **Step 8: Commit（前端）**

```bash
git add apps/web-antd/src/views/system/Oauth2ClientsPage.vue apps/web-antd/src/api/admin.ts
git commit -m "feat(web-antd): OAuth2 客户端管理页（CRUD + 权限指令）"
```

---

### Task 17: 兼容层下线评估文档 + 删除死代码 loginApi

**Files:**
- Create: `docs/oauth2-compat-retirement.md`
- Modify: `apps/web-antd/src/api/core/auth.ts`（删 `loginApi` 函数与 `AuthApi.LoginParams`/`AuthApi.LoginResult` 两接口）

**Interfaces:**
- Consumes: Task 12「三端兼容层切换统一登录」的最终形态（vben `NewController`、system app `NewAppController`、member `NewAuthController` 均内部转发 `UnifiedLogin`，响应契约逐字不变——文档「现状」表的事实来源）；Task 14 产物（`store/auth.ts` 的 `authLogin` 已改 PKCE SSO，不再 import `loginApi`——本任务 grep 证据成立的前提，**故本任务必须在 T14 之后执行**，plan 顺序保证）；保留项的消费方——`refreshTokenApi` 被 `apps/web-antd/src/api/request.ts:53` 消费、`logoutApi`/`getAccessCodesApi` 被 `store/auth.ts` 消费（三者本任务**不动**）。
- Produces（终测消费）: `docs/oauth2-compat-retirement.md`（端点覆盖表 + 下线条件清单，供发布评审）；`apps/web-antd/src/api/core/auth.ts` 不再导出 `loginApi`、`AuthApi.LoginParams`、`AuthApi.LoginResult`（执行期全仓 grep 无输出即证）。

**裁决：**
1. **服务端兼容层保留，本任务只出评估不动端点**：仓库外消费者（原生 app、第三方 H5/小程序）打 `POST /api/app/*/auth/login`，代码库无法证明已迁移——删端点 = 外部登录静默全挂且无编译/测试信号。文档给条件清单（410 过渡、QPS 归零窗口），不给删除动作（spec §4.4「前端全迁后下线」的前端部分已由 T14+本任务达成）。
2. **前端死代码立即删**：`loginApi` 在 T14 后零调用（grep 可证），保留只会误导后来者以为老登录仍可用。与 `refreshTokenApi` 先例不同——后者仍被 `request.ts` 消费，必须留（文档写明边界）。
3. **两个独立 commit**（docs 与 refactor 分离）：回滚粒度独立、前缀各归其类；文档先提交（引用 grep 证据），代码后提交（其后 grep 归零）。

- [ ] **Step 1: 收集证据（核对文档引文）**

Run: `grep -rn "loginApi" apps/web-antd/src`

Expected: **仅 1 行** —— `apps/web-antd/src/api/core/auth.ts:24:export async function loginApi(...)`（Task 14 已切断 `store/auth.ts` 的 import 与调用；若仍见 store 行，说明 T14 未执行——停止并核对 plan 顺序）。

Run: `grep -rn '"/auth/login"' modules/ --include="*.go"`

Expected: **3 行** —— `modules/member/module.go:56`（appGroup `/app/member`）、`modules/system/module.go:218`（appGroup `/app/system`）、`modules/system/controller/vben/controller.go:42`（挂 /api 根组）。与文档表 1 逐字一致。

- [ ] **Step 2: 写评估文档**

创建 `docs/oauth2-compat-retirement.md`（完整内容）：

```markdown
# OAuth2 统一登录——兼容层下线评估

- 日期：2026-10-11
- 关联：spec `docs/superpowers/specs/2026-10-11-unified-login-design.md` §4.4（老端点降级为兼容层）、§9 D11；实施计划 Task 12（切换统一登录）、Task 14（前端 SSO）、Task 17（本文档 + 死代码清理）
- 结论先行：**服务端兼容层保留**（仓库外消费者不可证迁移）；**前端死代码 `loginApi` 本任务删除**。

## 1. 现状：兼容端点覆盖表

Task 12 已把三端老登录端点内部切到统一登录（`UnifiedLogin.Login`），响应契约逐字不变；
「下线」指整体删除以下端点注册。

| 老端点 | 注册点 | 现状（Task 12 之后） |
|---|---|---|
| `POST /api/auth/login` | system `controller/vben/controller.go:42`（挂 /api 根组） | 内部走统一登录（account_type=system、client=alexgo-admin），vben 契约不变；同组兄弟 `/auth/logout`、`/auth/codes` |
| `POST /api/app/system/auth/login` | system `module.go:218`（appGroup `/app/system`） | 同上（D11 勘误后的实际路径）；兄弟 `/auth/refresh`、`/auth/logout` |
| `POST /api/app/member/auth/login` | member `module.go:56`（appGroup `/app/member`） | 走统一登录（account_type=member、client=alexgo-app）；兄弟 `register/refresh/logout`（55-58 行） |

## 2. 消费方盘点（仓库内证据）

```bash
$ grep -rn "loginApi" apps/web-antd/src        # Task 14 之后、本任务删除之前
apps/web-antd/src/api/core/auth.ts:24:export async function loginApi(...)
$ grep -rn '"/auth/login"' modules/ --include='*.go'
modules/member/module.go:56:	appGroup.POST("/auth/login", m.authCtrl.Login)
modules/system/module.go:218:	appGroup.POST("/auth/login", m.appCtrl.Login)
modules/system/controller/vben/controller.go:42:	r.POST("/auth/login", ctrl.Login)
```

- 仓库内唯一前端登录入口 web-antd：Task 14 后走 `POST /api/oauth2/authorize/login` + `/callback`（PKCE），`loginApi` 零调用 → 死代码（本任务删除）。
- **仓库外消费者不可见**：原生 app 端、第三方 H5/小程序仍打 `POST /api/app/*/auth/login` 等端点——代码库内无法证明其已迁移到 `POST /api/oauth2/token`。

## 3. 下线条件（全部满足才动服务端）

- [ ] 仓库内零调用（本任务达成：`grep -rn "loginApi" apps/web-antd/src` 无输出）
- [ ] app 端发版确认改用 `POST /api/oauth2/token`（grant_type=password + client alexgo-app）
- [ ] 老端点 QPS 观测归零 ≥ 2 周（含 member refresh/logout）
- [ ] member 外部接入方（H5/小程序）盘点完毕并书面确认迁移
- [ ] 下线方式：先返回 `410 Gone`（`{"error":"gone"}`）观察一周再删路由——绝不直接 404/静默消失
- [ ] 下线前对老端点加 4xx/5xx 告警，异常即撤销下线

## 4. 风险

- 贸然删除：外部 app/member 登录与刷新瞬间全挂；仓库内**无任何编译错误或测试失败**会提示（消费方在库外）。
- 保留成本≈0：兼容层已是薄壳——校验/审计/签发单点在统一登录（D13 恰一次审计），端点无独立业务逻辑，无双轨漂移风险。

## 5. 结论

服务端兼容层**保留**，按 §3 清单推进下线；本任务只落地仓库内可证的部分：
删除 web-antd 的 `loginApi` 与 `AuthApi.LoginParams`/`AuthApi.LoginResult`
（`refreshTokenApi` 仍被 `apps/web-antd/src/api/request.ts` 消费，保留）。
```

- [ ] **Step 3: 删除死代码**

`apps/web-antd/src/api/core/auth.ts` 改为以下完整内容（删首部 `LoginParams`/`LoginResult` 两接口与 `loginApi` 函数，其余逐字保留）：

```ts
import { baseRequestClient, requestClient } from '#/api/request';

export namespace AuthApi {
  export interface RefreshTokenResult {
    data: string;
    status: number;
  }
}

/**
 * 刷新accessToken
 */
export async function refreshTokenApi() {
  return baseRequestClient.post<AuthApi.RefreshTokenResult>('/auth/refresh', {
    withCredentials: true,
  });
}

/**
 * 退出登录
 */
export async function logoutApi() {
  // alexGo：走 requestClient（带 Authorization）——后端按 token 撤销；
  // 后端恒 200，撤销失败也不阻断前端登出流程。refreshTokenApi 保留原样（一期关闭 refresh 链）。
  return requestClient.post('/auth/logout');
}

/**
 * 获取用户权限码
 */
export async function getAccessCodesApi() {
  return requestClient.get<string[]>('/auth/codes');
}
```

- [ ] **Step 4: 验证**

Run: `grep -rn "loginApi" apps/web-antd/src && grep -rn "AuthApi.Login" apps/web-antd/src || true`

Expected: **无输出**（两 grep 均零命中）。

Run: `pnpm -C apps/web-antd typecheck`

Expected: PASS —— 0 errors（`AuthApi.LoginParams/LoginResult` 执行期全仓无引用——已核实仅 `auth.ts` 自用；`store/auth.ts` 在 Task 14 后已不 import `loginApi`）。

- [ ] **Step 5: Commit（文档）**

```bash
git add docs/oauth2-compat-retirement.md
git commit -m "docs: OAuth2 兼容层下线评估（端点覆盖 + 下线条件清单）"
```

- [ ] **Step 6: Commit（死代码）**

```bash
git add apps/web-antd/src/api/core/auth.ts
git commit -m "refactor(web-antd): 删除 SSO 后零调用的 loginApi 死代码"
```

---
