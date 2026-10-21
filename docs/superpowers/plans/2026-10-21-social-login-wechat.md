# 第三方登录（微信一期） Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** C 端用户可用微信（小程序 code2session / 网页授权）登录本平台，租户级配置渠道 appid，支持第三方身份绑定、绑定手机号（含空壳合并）与解绑守门，admin 端渠道配置 CRUD 与菜单。

**Architecture:** 方案 A——`modules/system/oauth2` 下新增 `social` 子包，贴 `UnifiedLogin` 生长；登录语义**不进** `UnifiedLogin.Login`，social 自己解析身份后直调 `token.Issuer.Issue`（与代登录同构），`UnifiedLogin` 零改动。adapter 接口 + registry 扩展点（抖音/支付宝留位），一期两条登录路由硬编码。micro 形态下 social 随 `oauth2.FxModule` 装进 member-server，C 端可用；admin 端点在 system-server。

**Tech Stack:** Go 1.x / gin / uber fx / gorm / golang-migrate / casbin / sqlite(glebarez) 测试；Vue3 + VxeGrid + vben(useVbenModal/useVbenForm)；MySQL 生产库。

**Spec:** `docs/superpowers/specs/2026-10-21-social-login-wechat-design.md`（plan 以 spec 为论据，执行者两份都要读）

## Global Constraints

- 表名 `system_social_app` / `system_member_identity`；Go 类型 `SocialApp` / `MemberIdentity`。
- secret **明文存储**（平台 API 硬约束）；任何 admin 响应**不回显**（含 create）；更新留空保留旧值。model 用 `json:"-"`。
- 登录**不进** `UnifiedLogin.Login`；直调 `token.Issuer.Issue`，`ClientID: "alexgo-app"`，`UserType: token.UserTypeMember`。
- 一期两条登录路由硬编码：`POST /api/app/social/wechat/mp/login`、`POST /api/app/social/wechat/web/login`（不参数化 platform）。
- 错误码 30301–30306（3xxxx → HTTP 400，pkg/resp statusFor 既有规则）：30301 该租户未配置此登录渠道（或已停用）；30302 第三方授权换取失败；30303 该第三方身份已绑定其他账号；30304 当前账号已绑定手机号；30305 手机号已被占用不满足自动合并；30306 解绑后将失去唯一登录方式。
- 带租户语义的 repo 查询一律**显式 `tenant_id = ?` 过滤**（gormplugin 对 tid=0 沉默不注入，pkg/tenant/gormplugin/plugin.go）+ 插件层测试。
- 不引入短信验证码（绑定手机号与注册同口径）。
- 空壳判定：`mobile == "" && password == ""`。`member_user` 表**零改动**。
- member 手机号正则 `^1[3-9]\d{9}$`（social 包内复制，member 的 mobileRe 未导出）。
- 迁移不手动跑（启动自动应用）；oauth2 迁移走 `schema_migrations_oauth2` 独立 MigrationSource。
- 直接提交 main（不建功能分支）。
- 验证基线：`go test ./...` 全包 0 FAIL + `pnpm check:type`（根无 `typecheck` 脚本）；既有 oauth2/member 测试零改动通过。

## Review Focus

每个失败模式归属到拥有它的任务，任务内有对应用例。

1. **tid=0 插件不注入 → 跨租户读写渠道/identity**（生产主路径恰是 tid=0）→ Task 3 repo 测试：seed tid 0/1/2 行，tid=0 ctx 只回本租户行。
2. **微信 errcode≠0 或空 openid 被当合法身份落库** → Task 4/5 adapter httptest 断言返回 error（不返回 Identity）。
3. **secret 出现在 admin create/list/update 响应** → Task 9 controller 测试断言 body 不含 secret 值。
4. **空壳合并误伤**（非空壳拒绝 30305 / 跨租户 identity 不改指 / 软删后 GetByMobile 不占用）→ Task 7 测试。
5. **昵称含 `:` 提权**（member sub 命名空间撞车 casbin g(x,x)）+ **空昵称被 token.Validate 拒**（`SELECT nickname ... == ""` → "token: user not found"）→ Task 6 测试：sanitize 后无 `:`、空昵称落占位非空。
6. **unionid 空串参与同人匹配串号** → Task 6 测试：unionid 为空的 identity 不被 (tid, unionid) 命中。
7. **解绑守门失效把自己锁死**（无密码且唯一平台身份仍可解绑）→ Task 7 测试 30306。

---

### Task 1: 错误码 30301–30306

**Files:**
- Modify: `pkg/errors/code.go`
- Test: `pkg/errors/code_test.go`

**Interfaces:**
- Consumes: 既有 `Code` 类型、`registry` map、`errs.New`。
- Produces: `errs.CodeSocialAppNotConfigured`(30301)、`CodeSocialExchangeFailed`(30302)、`CodeSocialIdentityBound`(30303)、`CodeSocialMobileBound`(30304)、`CodeSocialMergeRefused`(30305)、`CodeSocialUnbindLast`(30306) —— 后续任务全部引用这些常量。

- [ ] **Step 1: 写失败测试**

在 `pkg/errors/code_test.go` 的 `TestRegistryNoDuplicateAndAllHaveMsg` 的硬编码 `[]Code{...}` 切片中追加六个码（该测试断言每个码在 registry 有文案）：

```go
CodeSocialAppNotConfigured, // 30301
CodeSocialExchangeFailed,   // 30302
CodeSocialIdentityBound,    // 30303
CodeSocialMobileBound,      // 30304
CodeSocialMergeRefused,     // 30305
CodeSocialUnbindLast,       // 30306
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./pkg/errors/ -run TestRegistryNoDuplicateAndAllHaveMsg -v`
Expected: FAIL（undefined: CodeSocialAppNotConfigured 等）

- [ ] **Step 3: 加常量与 registry 文案**

`pkg/errors/code.go`（放在 302xx 段之后）：

```go
// ---- 303xx 第三方登录（social） ----
const (
	CodeSocialAppNotConfigured Code = 30301 // 该租户未配置此登录渠道（或已停用）
	CodeSocialExchangeFailed   Code = 30302 // 第三方授权换取失败
	CodeSocialIdentityBound    Code = 30303 // 该第三方身份已绑定其他账号
	CodeSocialMobileBound      Code = 30304 // 当前账号已绑定手机号
	CodeSocialMergeRefused     Code = 30305 // 手机号已被占用，当前账号不满足自动合并条件
	CodeSocialUnbindLast       Code = 30306 // 解绑后将失去唯一登录方式
)
```

`registry` map 增：

```go
CodeSocialAppNotConfigured: "该租户未配置此登录渠道（或已停用）",
CodeSocialExchangeFailed:   "第三方授权换取失败",
CodeSocialIdentityBound:    "该第三方身份已绑定其他账号",
CodeSocialMobileBound:      "当前账号已绑定手机号",
CodeSocialMergeRefused:     "手机号已被占用，当前账号不满足自动合并条件",
CodeSocialUnbindLast:       "解绑后将失去唯一登录方式",
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./pkg/errors/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/errors/code.go pkg/errors/code_test.go
git commit -m "feat(social): 错误码 30301-30306（第三方登录）"
```

---

### Task 2: model + 迁移（两表）

**Files:**
- Create: `modules/system/oauth2/social/model.go`
- Create: `modules/system/oauth2/migrations/20261021000001_social_tables.up.sql`
- Create: `modules/system/oauth2/migrations/20261021000001_social_tables.down.sql`
- Test: `modules/system/oauth2/social/model_test.go`

**Interfaces:**
- Consumes: 无（纯数据层）。
- Produces: `const PlatformWechatMP = "wechat_mp"`、`PlatformWechatWeb = "wechat_web"`；`type SocialApp struct{...}`（TableName `system_social_app`）；`type MemberIdentity struct{...}`（TableName `system_member_identity`）。后续任务引用字段：SocialApp.ID/TenantID/Platform/AppID/Secret/Status；MemberIdentity.ID/TenantID/MemberUserID/Platform/OpenID/UnionID。

- [ ] **Step 1: 写失败测试**

`modules/system/oauth2/social/model_test.go`：

```go
package social

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTableNames(t *testing.T) {
	if got := (&SocialApp{}).TableName(); got != "system_social_app" {
		t.Errorf("SocialApp.TableName = %q", got)
	}
	if got := (&MemberIdentity{}).TableName(); got != "system_member_identity" {
		t.Errorf("MemberIdentity.TableName = %q", got)
	}
}

// 迁移 DDL 钉死唯一键/索引——漂移即红（Review Focus #1/#6 的库侧前提）。
func TestSocialTablesMigrationDDL(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "migrations", "20261021000001_social_tables.up.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(b)
	for _, frag := range []string{
		"CREATE TABLE IF NOT EXISTS `system_social_app`",
		"CREATE TABLE IF NOT EXISTS `system_member_identity`",
		"UNIQUE KEY `uk_social_app_tenant_platform` (`tenant_id`, `platform`)",
		"UNIQUE KEY `uk_identity_tenant_platform_openid` (`tenant_id`, `platform`, `openid`)",
		"KEY `idx_identity_tenant_unionid` (`tenant_id`, `unionid`)",
	} {
		if !strings.Contains(sql, frag) {
			t.Errorf("up.sql 缺片段: %s", frag)
		}
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./modules/system/oauth2/social/ -v`
Expected: FAIL（SocialApp undefined / 文件不存在）

- [ ] **Step 3: 写 model 与迁移**

`modules/system/oauth2/social/model.go`：

```go
package social

import "time"

// 平台枚举（字符串，可扩展：douyin / alipay 后续加值）。
const (
	PlatformWechatMP = "wechat_mp" // 微信小程序 code2session
	PlatformWechatWeb = "wechat_web" // 微信网页授权
)

// SocialApp 租户级第三方登录渠道配置（每租户每平台一份）。
// Secret 明文存储（调用平台 API 必须明文）；json:"-" 保证任何 admin 响应不回显。
type SocialApp struct {
	ID         uint64    `gorm:"primaryKey" json:"id"`
	TenantID   uint64    `gorm:"column:tenant_id;uniqueIndex:uk_social_app_tp" json:"tenant_id"`
	Platform   string    `gorm:"size:32;uniqueIndex:uk_social_app_tp" json:"platform"`
	AppID      string    `gorm:"column:appid;size:64" json:"appid"`
	Secret     string    `gorm:"size:128" json:"-"`
	Status     int       `json:"status"` // 1 启用 0 停用
	CreateTime time.Time `gorm:"column:create_time" json:"create_time"`
	UpdateTime time.Time `gorm:"column:update_time" json:"update_time"`
}

func (SocialApp) TableName() string { return "system_social_app" }

// MemberIdentity 第三方身份绑定行（(tenant,platform,openid) 唯一）。
type MemberIdentity struct {
	ID           uint64    `gorm:"primaryKey" json:"id"`
	TenantID     uint64    `gorm:"column:tenant_id;uniqueIndex:uk_identity_tpo;index:idx_identity_tu" json:"tenant_id"`
	MemberUserID uint64    `gorm:"column:member_user_id" json:"member_user_id"`
	Platform     string    `gorm:"size:32;uniqueIndex:uk_identity_tpo" json:"platform"`
	OpenID       string    `gorm:"column:openid;size:128;uniqueIndex:uk_identity_tpo" json:"openid"`
	UnionID      string    `gorm:"column:unionid;size:128;index:idx_identity_tu" json:"unionid"` // 可空；空串不参与同人匹配
	CreateTime   time.Time `gorm:"column:create_time" json:"create_time"`
}

func (MemberIdentity) TableName() string { return "system_member_identity" }
```

`modules/system/oauth2/migrations/20261021000001_social_tables.up.sql`：

```sql
CREATE TABLE IF NOT EXISTS `system_social_app` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `platform` VARCHAR(32) NOT NULL DEFAULT '',
  `appid` VARCHAR(64) NOT NULL DEFAULT '',
  `secret` VARCHAR(128) NOT NULL DEFAULT '',
  `status` TINYINT NOT NULL DEFAULT 1,
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_social_app_tenant_platform` (`tenant_id`, `platform`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `system_member_identity` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `member_user_id` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `platform` VARCHAR(32) NOT NULL DEFAULT '',
  `openid` VARCHAR(128) NOT NULL DEFAULT '',
  `unionid` VARCHAR(128) NOT NULL DEFAULT '',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_identity_tenant_platform_openid` (`tenant_id`, `platform`, `openid`),
  KEY `idx_identity_tenant_unionid` (`tenant_id`, `unionid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

`modules/system/oauth2/migrations/20261021000001_social_tables.down.sql`：

```sql
DROP TABLE IF EXISTS `system_member_identity`;
DROP TABLE IF EXISTS `system_social_app`;
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./modules/system/oauth2/social/ -v`
Expected: PASS（迁移不手动跑，仅断言 SQL 文件内容）

- [ ] **Step 5: Commit**

```bash
git add modules/system/oauth2/social/model.go modules/system/oauth2/social/model_test.go modules/system/oauth2/migrations/20261021000001_social_tables.up.sql modules/system/oauth2/migrations/20261021000001_social_tables.down.sql
git commit -m "feat(social): system_social_app/system_member_identity 模型与迁移"
```

---

### Task 3: repository（显式租户过滤）+ member repo 补 GetByID

**Files:**
- Create: `modules/system/oauth2/social/repository.go`
- Test: `modules/system/oauth2/social/repository_test.go`
- Modify: `modules/member/repository/member_repo.go`（加 GetByID）
- Test: `modules/member/repository/member_repo_test.go`（若无则创建，加 GetByID 用例）

**Interfaces:**
- Consumes: Task 2 的 `SocialApp` / `MemberIdentity`；既有 `gormplugin`、`tenant.WithTenantID`。
- Produces:

```go
type SocialRepository interface {
	GetApp(ctx context.Context, tenantID uint64, platform string) (*SocialApp, error)
	GetAppByID(ctx context.Context, tenantID, id uint64) (*SocialApp, error)
	ListApps(ctx context.Context, tenantID uint64, page, size int, platform string) ([]*SocialApp, int64, error) // platform=="" 不筛
	CreateApp(ctx context.Context, app *SocialApp) error
	UpdateApp(ctx context.Context, app *SocialApp) error // WHERE tenant_id=? AND id=? 走 Updates(map)
	DeleteApp(ctx context.Context, tenantID, id uint64) error
	GetIdentity(ctx context.Context, tenantID uint64, platform, openid string) (*MemberIdentity, error)
	GetIdentityByUnion(ctx context.Context, tenantID uint64, unionid string) (*MemberIdentity, error) // SQL 层再防 unionid=''
	ListIdentities(ctx context.Context, tenantID, memberUserID uint64) ([]*MemberIdentity, error)
	CreateIdentity(ctx context.Context, idn *MemberIdentity) error
	ReassignIdentities(ctx context.Context, tenantID, from, to uint64) (int64, error)
	DeleteIdentity(ctx context.Context, tenantID, memberUserID uint64, platform string) (int64, error)
}
func NewSocialRepository(db *gorm.DB) SocialRepository
```

未找到一律返回 `gorm.ErrRecordNotFound`。member repo 新增：

```go
// GetByID 按 tenant+id 直取（含已软删行——调用方按 Deleted 字段判定）。
func (r *memberRepo) GetByID(ctx context.Context, tenantID, id uint64) (*model.MemberUser, error)
```

- [ ] **Step 1: 写失败测试（social repo）**

`modules/system/oauth2/social/repository_test.go`——sqlite 内存 + 插件 + AutoMigrate，seed tid 0/1/2 行，逐口断言。骨架（完整写全，每个用例独立 subtest）：

```go
package social

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/tenant/gormplugin"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := gormplugin.Register(db, gormplugin.Options{}); err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&SocialApp{}, &MemberIdentity{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func ctxTid(tid uint64) context.Context { return tenant.WithTenantID(context.Background(), tid) }

func TestRepo_TenantIsolation(t *testing.T) {
	db := newTestDB(t)
	repo := NewSocialRepository(db)
	// seed：tid 0/1/2 各一份 wechat_mp 配置
	for _, tid := range []uint64{0, 1, 2} {
		if err := repo.CreateApp(ctxTid(1), &SocialApp{TenantID: tid, Platform: PlatformWechatMP, AppID: "app-" + string(rune('0'+tid)), Secret: "s", Status: 1}); err != nil {
			t.Fatal(err)
		}
	}
	// tid=0 只回 tid=0 行（Review Focus #1：插件对 tid=0 沉默不注入）
	app, err := repo.GetApp(ctxTid(0), PlatformWechatMP)
	if err != nil {
		t.Fatal(err)
	}
	if app.TenantID != 0 {
		t.Errorf("tid=0 读到跨租户行: tenant=%d", app.TenantID)
	}
	// tid=1 只回 tid=1 行
	app, err = repo.GetApp(ctxTid(1), PlatformWechatMP)
	if err != nil {
		t.Fatal(err)
	}
	if app.TenantID != 1 {
		t.Errorf("tid=1 读到跨租户行: tenant=%d", app.TenantID)
	}
	// GetAppByID 跨租户不可见 → ErrRecordNotFound
	if _, err := repo.GetAppByID(ctxTid(1), 3); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("跨租户 GetAppByID err = %v, want ErrRecordNotFound", err)
	}
}

func TestRepo_IdentityLookupAndReassign(t *testing.T) {
	db := newTestDB(t)
	repo := NewSocialRepository(db)
	// tid=1: member 100 有 wechat_mp openid=A；member 200 有 wechat_web openid=B
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(repo.CreateIdentity(ctxTid(1), &MemberIdentity{TenantID: 1, MemberUserID: 100, Platform: PlatformWechatMP, OpenID: "A", UnionID: "U1"}))
	must(repo.CreateIdentity(ctxTid(1), &MemberIdentity{TenantID: 1, MemberUserID: 200, Platform: PlatformWechatWeb, OpenID: "B", UnionID: ""}))
	must(repo.CreateIdentity(ctxTid(2), &MemberIdentity{TenantID: 2, MemberUserID: 100, Platform: PlatformWechatMP, OpenID: "A", UnionID: "U1"}))

	idn, err := repo.GetIdentity(ctxTid(1), PlatformWechatMP, "A")
	if err != nil || idn.MemberUserID != 100 {
		t.Fatalf("GetIdentity = %+v, %v", idn, err)
	}
	// unionid 空串不参与匹配（Review Focus #6）
	if _, err := repo.GetIdentityByUnion(ctxTid(1), ""); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("unionid='' 查询 err = %v, want ErrRecordNotFound", err)
	}
	// tid=1 有 U1（member 100），应命中且 tenant=1
	u1, err := repo.GetIdentityByUnion(ctxTid(1), "U1")
	if err != nil || u1.TenantID != 1 || u1.MemberUserID != 100 {
		t.Fatalf("GetIdentityByUnion = %+v, %v", u1, err)
	}
	// tid=3 无该 unionid → 不串租户
	if _, err := repo.GetIdentityByUnion(ctxTid(3), "U1"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("tid=3 GetIdentityByUnion err = %v, want ErrRecordNotFound", err)
	}
	// ReassignIdentities 显式 tenant 过滤：tid=1 改指 100→300，不影响 tid=2 同 member 行
	n, err := repo.ReassignIdentities(ctxTid(1), 100, 300)
	if err != nil || n != 1 {
		t.Fatalf("Reassign = %d, %v", n, err)
	}
	idn2, _ := repo.GetIdentity(ctxTid(2), PlatformWechatMP, "A")
	if idn2.MemberUserID != 100 {
		t.Errorf("跨租户 identity 被改指: %d", idn2.MemberUserID)
	}
	// DeleteIdentity 只删 (tid, member, platform)
	if n, err := repo.DeleteIdentity(ctxTid(1), 200, PlatformWechatWeb); err != nil || n != 1 {
		t.Fatalf("DeleteIdentity = %d, %v", n, err)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./modules/system/oauth2/social/ -v`
Expected: FAIL（NewSocialRepository undefined）

- [ ] **Step 3: 实现 repository**

`modules/system/oauth2/social/repository.go`（完整实现，全部查询显式 `tenant_id = ?`，带注释「显式 tenant_id 过滤——gormplugin 对 tid=0 沉默不注入」）：`socialRepo{db *gorm.DB}` 实现上述 12 方法；`GetApp` = `Where("tenant_id = ? AND platform = ?", tid, platform).First`；`ListApps` 按 page/size 分页 + 可选 platform 等值 + `Count`；`UpdateApp` = `Model(&SocialApp{}).Where("tenant_id = ? AND id = ?", app.TenantID, app.ID).Updates(map{platform,appid,secret,status,update_time})`；`GetIdentityByUnion` = `Where("tenant_id = ? AND unionid = ? AND unionid <> ''", tid, unionid).First`；`ReassignIdentities` = `Model(&MemberIdentity{}).Where("tenant_id = ? AND member_user_id = ?", tid, from).Update("member_user_id", to)` 返回 RowsAffected。

同时在 `modules/member/repository/member_repo.go` 加 `GetByID`（见 Interfaces 块），并在 member repo 测试中加一例：seed 两租户同 id 不可能（主键唯一），但断言跨租户 id 查不到 + 软删行仍可取回。

- [ ] **Step 4: 跑测试确认通过（含 member repo 既有测试）**

Run: `go test ./modules/system/oauth2/social/ ./modules/member/repository/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add modules/system/oauth2/social/repository.go modules/system/oauth2/social/repository_test.go modules/member/repository/member_repo.go modules/member/repository/member_repo_test.go
git commit -m "feat(social): 渠道/identity repository（显式租户过滤）+ member repo GetByID"
```

---

### Task 4: adapter 基座 + 微信小程序实现

**Files:**
- Create: `modules/system/oauth2/social/adapter.go`
- Create: `modules/system/oauth2/social/wechat_mp.go`
- Test: `modules/system/oauth2/social/adapter_test.go`
- Test: `modules/system/oauth2/social/wechat_mp_test.go`

**Interfaces:**
- Consumes: Task 2 `SocialApp`。
- Produces:

```go
type Identity struct {
	OpenID   string
	UnionID  string // 可空
	Nickname string // 可空（微信已匿名化）
	Avatar   string // 可空
}
type PlatformAdapter interface {
	Platform() string
	Exchange(ctx context.Context, app *SocialApp, code string) (*Identity, error)
}
type AdapterRegistry struct { /* mu sync.RWMutex; byPlatform map[string]PlatformAdapter */ }
func NewAdapterRegistry() *AdapterRegistry // 内部 Register(NewWechatMP(), NewWechatWeb())
func (r *AdapterRegistry) Register(a PlatformAdapter)   // dup panic（装配期编程错误）
func (r *AdapterRegistry) Get(platform string) (PlatformAdapter, bool)
func (r *AdapterRegistry) Exists(platform string) bool
```

- [ ] **Step 1: 写失败测试**

`adapter_test.go`：

```go
package social

import "testing"

func TestAdapterRegistry(t *testing.T) {
	r := NewAdapterRegistry() // 注意：Task 5 才有 wechat_web——本步先只 Register(NewWechatMP())
	if !r.Exists(PlatformWechatMP) {
		t.Error("registry 缺 wechat_mp")
	}
	if _, ok := r.Get("douyin"); ok {
		t.Error("未知平台应 miss")
	}
}
```

`wechat_mp_test.go`（httptest 注入 baseURL，不打真平台）：

```go
package social

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newMP(t *testing.T, handler http.HandlerFunc) (*WechatMP, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &WechatMP{baseURL: srv.URL, client: srv.Client()}, srv
}

func TestWechatMP_ExchangeOK(t *testing.T) {
	var gotQuery string
	mp, _ := newMP(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sns/jscode2session" {
			t.Errorf("path = %s", r.URL.Path)
		}
		gotQuery = r.URL.RawQuery
		w.Write([]byte(`{"openid":"OID","unionid":"UID","session_key":"k"}`))
	})
	idn, err := mp.Exchange(context.Background(), &SocialApp{AppID: "my-app", Secret: "my-secret"}, "code-1")
	if err != nil {
		t.Fatal(err)
	}
	if idn.OpenID != "OID" || idn.UnionID != "UID" {
		t.Errorf("Identity = %+v", idn)
	}
	for _, want := range []string{"appid=my-app", "secret=my-secret", "js_code=code-1", "grant_type=authorization_code"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query 缺 %q: %s", want, gotQuery)
		}
	}
}

func TestWechatMP_ExchangeErrcode(t *testing.T) {
	mp, _ := newMP(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errcode":40029,"errmsg":"invalid code"}`))
	})
	if _, err := mp.Exchange(context.Background(), &SocialApp{AppID: "a", Secret: "s"}, "bad"); err == nil {
		t.Error("errcode≠0 应返回 error（Review Focus #2）")
	}
}

func TestWechatMP_ExchangeEmptyOpenID(t *testing.T) {
	mp, _ := newMP(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"unionid":"UID"}`))
	})
	if _, err := mp.Exchange(context.Background(), &SocialApp{AppID: "a", Secret: "s"}, "c"); err == nil {
		t.Error("空 openid 应返回 error")
	}
}
```

（记得 import `"strings"`。）

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./modules/system/oauth2/social/ -run 'Adapter|WechatMP' -v`
Expected: FAIL（WechatMP undefined 等）

- [ ] **Step 3: 实现 adapter.go + wechat_mp.go**

`adapter.go`：上述接口 + registry（RWMutex，Register 重复 panic `"social: adapter already registered: "+platform`；NewAdapterRegistry 内部注册微信两实现——若 Task 5 未完成，先只注册 MP，Task 5 补齐）。

`wechat_mp.go`：

```go
package social

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// WechatMP 微信小程序 code2session。
type WechatMP struct {
	baseURL string
	client  *http.Client
}

func NewWechatMP() *WechatMP {
	return &WechatMP{baseURL: "https://api.weixin.qq.com", client: &http.Client{Timeout: 10 * time.Second}}
}

func (m *WechatMP) Platform() string { return PlatformWechatMP }

func (m *WechatMP) Exchange(ctx context.Context, app *SocialApp, code string) (*Identity, error) {
	q := url.Values{
		"appid":      {app.AppID},
		"secret":     {app.Secret},
		"js_code":    {code},
		"grant_type": {"authorization_code"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.baseURL+"/sns/jscode2session?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wechat mp code2session: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var out struct {
		OpenID  string `json:"openid"`
		UnionID string `json:"unionid"`
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("wechat mp code2session: bad json: %w", err)
	}
	// 平台原始错误只返回给调用方映射 30302，errcode 不上抛给客户端（spec §3）
	if out.ErrCode != 0 || out.OpenID == "" {
		return nil, fmt.Errorf("wechat mp code2session: errcode=%d errmsg=%s", out.ErrCode, out.ErrMsg)
	}
	return &Identity{OpenID: out.OpenID, UnionID: out.UnionID}, nil
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./modules/system/oauth2/social/ -run 'Adapter|WechatMP' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add modules/system/oauth2/social/adapter.go modules/system/oauth2/social/wechat_mp.go modules/system/oauth2/social/adapter_test.go modules/system/oauth2/social/wechat_mp_test.go
git commit -m "feat(social): PlatformAdapter registry + 微信小程序 code2session"
```

---

### Task 5: 微信网页授权 adapter

**Files:**
- Create: `modules/system/oauth2/social/wechat_web.go`
- Test: `modules/system/oauth2/social/wechat_web_test.go`
- Modify: `modules/system/oauth2/social/adapter.go`（NewAdapterRegistry 补注册 WechatWeb）

**Interfaces:**
- Consumes: Task 4 的 `PlatformAdapter` / `Identity` / registry。
- Produces: `NewWechatWeb()`、`WechatWeb.Platform() == PlatformWechatWeb`、`Exchange` 签名同上。

- [ ] **Step 1: 写失败测试**

`wechat_web_test.go`（结构同 Task 4 的 MP 测试）：

```go
package social

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWechatWeb_ExchangeOK(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sns/oauth2/access_token" {
			t.Errorf("path = %s", r.URL.Path)
		}
		gotQuery = r.URL.RawQuery
		w.Write([]byte(`{"access_token":"AT","openid":"OID","unionid":"UID"}`))
	}))
	defer srv.Close()
	web := &WechatWeb{baseURL: srv.URL, client: srv.Client()}
	idn, err := web.Exchange(context.Background(), &SocialApp{AppID: "a", Secret: "s"}, "code-1")
	if err != nil {
		t.Fatal(err)
	}
	if idn.OpenID != "OID" || idn.UnionID != "UID" {
		t.Errorf("Identity = %+v", idn)
	}
	for _, want := range []string{"appid=a", "secret=s", "code=code-1", "grant_type=authorization_code"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query 缺 %q: %s", want, gotQuery)
		}
	}
}

func TestWechatWeb_ExchangeErrcode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errcode":40029,"errmsg":"invalid code"}`))
	}))
	defer srv.Close()
	web := &WechatWeb{baseURL: srv.URL, client: srv.Client()}
	if _, err := web.Exchange(context.Background(), &SocialApp{AppID: "a", Secret: "s"}, "bad"); err == nil {
		t.Error("errcode≠0 应返回 error（Review Focus #2）")
	}
}

func TestAdapterRegistry_HasBothWechat(t *testing.T) {
	r := NewAdapterRegistry()
	if !r.Exists(PlatformWechatMP) || !r.Exists(PlatformWechatWeb) {
		t.Error("registry 应含微信两平台")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./modules/system/oauth2/social/ -run 'WechatWeb|Registry' -v`
Expected: FAIL（WechatWeb undefined / registry 缺 wechat_web）

- [ ] **Step 3: 实现 wechat_web.go + 注册**

`wechat_web.go` 与 MP 同构：GET `m.baseURL+"/sns/oauth2/access_token?"`，参数 `appid/secret/code/grant_type=authorization_code`，解析 `{access_token, openid, unionid, errcode, errmsg}`，`errcode≠0 || openid==""` → error。`NewWechatWeb()` baseURL 同 `https://api.weixin.qq.com`，timeout 10s。`adapter.go` 的 `NewAdapterRegistry` 改为 `Register(NewWechatMP(), NewWechatWeb())`。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./modules/system/oauth2/social/ -v`
Expected: PASS（Task 4 用例仍绿）

- [ ] **Step 5: Commit**

```bash
git add modules/system/oauth2/social/wechat_web.go modules/system/oauth2/social/wechat_web_test.go modules/system/oauth2/social/adapter.go
git commit -m "feat(social): 微信网页授权 adapter + registry 双平台注册"
```

---

### Task 6: SocialService 登录（三分支 + 签发 + 审计）+ fx 装配

**Files:**
- Create: `modules/system/oauth2/social/service.go`
- Test: `modules/system/oauth2/social/service_test.go`
- Modify: `modules/system/oauth2/module.go`（fx.Provide repo/registry/service）
- Modify: `modules/member/module.go`（Provide MemberStore 映射）

**Interfaces:**
- Consumes: Task 2/3/4/5 全部；`token.Issuer` / `token.IssueParams`（pkg/token）；`oauth2svc.LoginResult` / `LoginAuditor` / `ErrIssuance` / `ErrAuthenticatorMissing`（modules/system/oauth2/service/login.go）；`tenant.AccountLimitChecker`（pkg/tenant/account.go，optional）；`membermodel.MemberUser`。
- Produces:

```go
type MemberStore interface {
	GetByID(ctx context.Context, tenantID, id uint64) (*membermodel.MemberUser, error)
	GetByMobile(ctx context.Context, tenantID uint64, mobile string) (*membermodel.MemberUser, error)
	Create(ctx context.Context, u *membermodel.MemberUser) error
	Update(ctx context.Context, u *membermodel.MemberUser) error
}
type SocialDeps struct {
	fx.In
	Repo     SocialRepository
	Members  MemberStore               `optional:"true"` // system 进程无 member repo
	Adapters *AdapterRegistry
	Issuer   token.Issuer
	Auditor  oauth2svc.LoginAuditor     `optional:"true"` // member 进程无 system audit → nil 安全跳过
	Limit    tenant.AccountLimitChecker `optional:"true"`
}
func NewSocialService(deps SocialDeps) *SocialService
func (s *SocialService) Login(ctx context.Context, platform, code, ip, ua string) (*oauth2svc.LoginResult, error)
```

- [ ] **Step 1: 写失败测试**

`service_test.go`——环境：sqlite + gormplugin + AutoMigrate(SocialApp, MemberIdentity, membermodel.MemberUser) + ctx `tenant.WithTenantID`。夹具：

```go
type fakeIssuer struct{ last token.IssueParams; err error }
func (f *fakeIssuer) Issue(ctx context.Context, p token.IssueParams) (*token.Issued, error) {
	f.last = p
	if f.err != nil {
		return nil, f.err
	}
	return &token.Issued{AccessToken: "at", RefreshToken: "rt", ExpiresIn: 3600}, nil
}
func (f *fakeIssuer) Refresh(context.Context, string) (*token.Issued, error) { return nil, nil }
func (f *fakeIssuer) Revoke(context.Context, string) error                   { return nil }
func (f *fakeIssuer) RevokeAll(context.Context, token.UserType, uint64) error { return nil }

type fakeAuditor struct{ msgs []string }
func (f *fakeAuditor) RecordLogin(ctx context.Context, username string, userID uint64, ip, ua string, success bool, msg string) error {
	f.msgs = append(f.msgs, msg)
	return nil
}

type stubAdapter struct{ platform string; idn *Identity; err error; calls int }
func (a *stubAdapter) Platform() string { return a.platform }
func (a *stubAdapter) Exchange(ctx context.Context, app *SocialApp, code string) (*Identity, error) {
	a.calls++
	return a.idn, a.err
}
```

用例（每个 subtest 自建 DB）：

```go
func TestLogin_IdentityHit_IssuesAndAuditsOnce(t *testing.T)
// seed SocialApp{tid,wechat_mp,Status:1} + MemberIdentity{tid,openid=OID,member=100} + MemberUser{ID:100, Nickname:"n", Status:1}
// stubAdapter idn{OpenID:"OID"} → Login 成功；fakeIssuer.last == {UserID:100, TenantID:tid, ClientID:"alexgo-app", UserType:member}
// fakeAuditor 恰 1 条且含 "微信小程序登录" 与 openid

func TestLogin_MemberDeletedOrDisabled_Rejected(t *testing.T)
// MemberUser{Deleted:true} 或 Status=0 → err errors.Is CodeUserNotFound (30002, HTTP 400)

func TestLogin_UnionIDBackfill(t *testing.T)
// identity 只有 (tid,wechat_web,openid=B,unionid=U,member=100)；登录 wechat_mp openid=A unionid=U
// → 补建 identity 行 (tid,wechat_mp,A,member=100) + Issue

func TestLogin_EmptyUnionID_NotMatched(t *testing.T) // Review Focus #6
// 存量 identity unionid=""；新登录 unionid="" openid 不同 → 不补绑，走空壳注册

func TestLogin_NotConfigured_30301_NoAdapterCall(t *testing.T)
// 无 SocialApp 行 → 30301 且 stubAdapter.calls==0（不耗配额）

func TestLogin_Disabled_30301(t *testing.T)  // Status=0 → 30301
func TestLogin_ExchangeErr_30302(t *testing.T) // stubAdapter.err → 30302

func TestLogin_AutoRegisterShell_SanitizesNickname(t *testing.T) // Review Focus #5
// idn.Nickname = "微:信:用" → 落库 nickname 无 ':'
// idn.Nickname = "" → 落库 nickname != ""（占位 "微信用户"+openid尾4）

func TestLogin_QuotaError_Propagates(t *testing.T)   // stub Limit 返回 err → Login 返回同 err
func TestLogin_IssuerErr_WrapsErrIssuance(t *testing.T) // fakeIssuer.err → errors.Is ErrIssuance
func TestLogin_MembersNil_ErrAuthenticatorMissing(t *testing.T) // Members 不 Provide → errors.Is ErrAuthenticatorMissing
```

（断言错误码用 `errs.Is(err, errs.CodeSocialAppNotConfigured)` 等——按仓库既有 `errs` 断言口径写；若仓库用 `errors.As(*errs.BizError)` 取 Code 则用该口径。）

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./modules/system/oauth2/social/ -run TestLogin -v`
Expected: FAIL（NewSocialService undefined）

- [ ] **Step 3: 实现 service.go 登录部分**

核心流程（完整实现）：

```go
func (s *SocialService) Login(ctx context.Context, platform, code, ip, ua string) (*oauth2svc.LoginResult, error) {
	tid := tenant.TenantIDFromContext(ctx)
	ad, ok := s.adapters.Get(platform)
	if !ok {
		return nil, errs.New(errs.CodeSocialAppNotConfigured, nil)
	}
	app, err := s.repo.GetApp(ctx, tid, platform)
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && app.Status != 1) {
		return nil, errs.New(errs.CodeSocialAppNotConfigured, err)
	}
	if err != nil {
		return nil, err
	}
	ident, err := ad.Exchange(ctx, app, code)
	if err != nil {
		// 平台原始错误只记日志，不上抛 errcode（spec §3）
		log.Printf("social: exchange failed platform=%s: %v", platform, err)
		return nil, errs.New(errs.CodeSocialExchangeFailed, err)
	}
	u, res, err := s.resolveAndIssue(ctx, tid, platform, ident, ip, ua)
	if err != nil {
		return nil, err
	}
	if s.auditor != nil {
		_ = s.auditor.RecordLogin(ctx, u.Nickname, u.ID, ip, ua, true,
			"social: "+platformLabels[platform]+" openid="+ident.OpenID)
	}
	return res, nil
}
```

`resolveAndIssue(ctx, tid, platform, ident, ip, ua) (*membermodel.MemberUser, *oauth2svc.LoginResult, error)` 三分支（spec §3）：

1. `GetIdentity(tid, platform, openid)` 命中 → `members.GetByID(tid, idn.MemberUserID)`；`Members==nil` → `oauth2svc.ErrAuthenticatorMissing`；`u.Deleted || u.Status != 1` → `errs.New(errs.CodeUserNotFound, nil)`；→ issue。
2. 未命中且 `ident.UnionID != ""` → `GetIdentityByUnion(tid, unionid)` 命中 → `CreateIdentity{同 member}`（唯一键竞态 → 重新 Get，member 相同则幂等继续）→ issue。
3. 都没有 → `Limit`（非 nil 时）`CheckAccountLimit(ctx, tid)` → 创建空壳 member：

```go
shell := &membermodel.MemberUser{
	TenantID:   tid,
	Nickname:   sanitizeNickname(ident.Nickname, ident.OpenID), // 见下
	Avatar:     ident.Avatar,
	Status:     1,
	Mobile:     "",
	Password:   "",
	RegisterIP: ip,
	LoginDate:  &now,
}
```

`sanitizeNickname`：`strings.ReplaceAll(nick, ":", "")`（member sub 命名空间 `{tid}:role:{code}` vs user `{tid}:{ut}:{nickname}`，':' 撞车 = casbin 提权），trim 后为空 → `"微信用户"+lastN(openID,4)`（token.Validate 拒空昵称）。然后 `CreateIdentity` + issue。

`issue` 封装：

```go
issued, err := s.issuer.Issue(ctx, token.IssueParams{
	UserID: u.ID, TenantID: u.TenantID, ClientID: "alexgo-app", UserType: token.UserTypeMember,
})
if err != nil {
	return nil, fmt.Errorf("%w: %v", oauth2svc.ErrIssuance, err)
}
return &oauth2svc.LoginResult{AccessToken: issued.AccessToken, RefreshToken: issued.RefreshToken, ExpiresIn: issued.ExpiresIn}
```

`platformLabels = map[string]string{PlatformWechatMP: "微信小程序登录", PlatformWechatWeb: "微信网页授权登录"}`。

**fx 装配**：`modules/system/oauth2/module.go` 的 `fx.Provide(...)` 增 `social.NewSocialRepository, social.NewAdapterRegistry, social.NewSocialService`；`modules/member/module.go` 增：

```go
fx.Provide(func(r repository.MemberRepository) social.MemberStore { return r }),
```

（无 import 循环：social 只 import member/model；member/module import oauth2/social。micro 形态 system 进程无此 Provide → Members optional 为 nil。）

- [ ] **Step 4: 跑测试确认通过（含既有 oauth2/member 测试零改动）**

Run: `go test ./modules/system/oauth2/... ./modules/member/... -v`
Expected: PASS（UnifiedLogin 未动）

- [ ] **Step 5: Commit**

```bash
git add modules/system/oauth2/social/service.go modules/system/oauth2/social/service_test.go modules/system/oauth2/module.go modules/member/module.go
git commit -m "feat(social): SocialService 登录三分支 + 签发审计 + fx 装配"
```

---

### Task 7: 绑定 / 手机号合并 / 解绑守门

**Files:**
- Modify: `modules/system/oauth2/social/service.go`
- Test: `modules/system/oauth2/social/service_test.go`（追加用例）

**Interfaces:**
- Consumes: Task 6 的 deps/夹具；`mobileRe`（包内复制 `regexp.MustCompile(^1[3-9]\d{9}$)`）。
- Produces:

```go
func (s *SocialService) BindSocial(ctx context.Context, memberID uint64, platform, code string) error
func (s *SocialService) BindMobile(ctx context.Context, memberID uint64, mobile string) error
func (s *SocialService) UnbindSocial(ctx context.Context, memberID uint64, platform string) error
```

- [ ] **Step 1: 写失败测试（Review Focus #4/#7 全覆盖）**

```go
func TestBindSocial_PlatformUnknown_10001(t *testing.T)      // adapters.Exists false → CodeParamInvalid
func TestBindSocial_NotConfigured_30301(t *testing.T)
func TestBindSocial_ExchangeErr_30302(t *testing.T)
func TestBindSocial_AlreadyBoundSelf_Idempotent(t *testing.T) // 返回 nil 且不重复插行
func TestBindSocial_BoundOther_30303(t *testing.T)
func TestBindSocial_Unbound_Binds(t *testing.T)

func TestBindMobile_FormatInvalid_10001(t *testing.T)        // "123" → CodeParamInvalid
func TestBindMobile_AlreadyHasMobile_30304(t *testing.T)
func TestBindMobile_Free_SetsMobile(t *testing.T)
func TestBindMobile_Taken_NonShell_30305(t *testing.T)        // Review Focus #4：对方 mobile/password 非空
func TestBindMobile_Taken_Shell_Merges(t *testing.T)
//   空壳 me(无 mobile 无 password, 有 identity wechat_mp) + 目标 member 有 mobile
//   → Reassign: me 的 identity 全指目标；me.Deleted=true
//   → 跨租户同 member_user_id 的 identity 不被改指（seed tid=2 对照行）
//   → 软删后 GetByMobile(tid, mobile) 只回目标行
func TestBindMobile_MembersNil_ErrAuthenticatorMissing(t *testing.T)

func TestUnbind_NoIdentity_Idempotent(t *testing.T)           // 返回 nil
func TestUnbind_HasPassword_Allows(t *testing.T)              // me.Password != "" → 删除成功
func TestUnbind_OtherPlatformRemains_Allows(t *testing.T)
func TestUnbind_LastLoginMethod_30306(t *testing.T)           // Review Focus #7：Password=="" 且 identities 只剩本平台
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./modules/system/oauth2/social/ -run 'Bind|Unbind' -v`
Expected: FAIL（方法 undefined）

- [ ] **Step 3: 实现三方法**

```go
func (s *SocialService) BindSocial(ctx context.Context, memberID uint64, platform, code string) error {
	if !s.adapters.Exists(platform) {
		return errs.New(errs.CodeParamInvalid, nil) // 未知平台是用户输入
	}
	ad, _ := s.adapters.Get(platform)
	tid := tenant.TenantIDFromContext(ctx)
	app, err := s.repo.GetApp(ctx, tid, platform)
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && app.Status != 1) {
		return errs.New(errs.CodeSocialAppNotConfigured, err) // 未配置/停用
	}
	if err != nil {
		return err
	}
	if code == "" {
		return errs.New(errs.CodeParamMissing, nil)
	}
	ident, err := ad.Exchange(ctx, app, code)
	if err != nil {
		log.Printf("social: bind exchange failed platform=%s: %v", platform, err)
		return errs.New(errs.CodeSocialExchangeFailed, err)
	}
	idn, err := s.repo.GetIdentity(ctx, tid, platform, ident.OpenID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if idn != nil {
		if idn.MemberUserID == memberID {
			return nil // 已绑自己 → 幂等成功
		}
		return errs.New(errs.CodeSocialIdentityBound, nil) // 已绑他人，不迁移
	}
	return s.repo.CreateIdentity(ctx, &MemberIdentity{
		TenantID: tid, MemberUserID: memberID,
		Platform: platform, OpenID: ident.OpenID, UnionID: ident.UnionID,
	})
}

func (s *SocialService) BindMobile(ctx context.Context, memberID uint64, mobile string) error {
	if !mobileRe.MatchString(mobile) {
		return errs.New(errs.CodeParamInvalid, nil)
	}
	if s.members == nil {
		return oauth2svc.ErrAuthenticatorMissing
	}
	tid := tenant.TenantIDFromContext(ctx)
	me, err := s.members.GetByID(ctx, tid, memberID)
	if err != nil || me.Deleted {
		return errs.New(errs.CodeUserNotFound, err)
	}
	if me.Mobile != "" {
		return errs.New(errs.CodeSocialMobileBound, nil) // 已绑定，不允许改绑
	}
	taken, err := s.members.GetByMobile(ctx, tid, mobile)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		me.Mobile = mobile
		return s.members.Update(ctx, me)
	}
	if err != nil {
		return err
	}
	// mobile 已被占用 → 仅空壳自动合并（Review Focus #4）
	isShell := me.Mobile == "" && me.Password == ""
	if !isShell {
		return errs.New(errs.CodeSocialMergeRefused, nil)
	}
	if _, err := s.repo.ReassignIdentities(ctx, tid, me.ID, taken.ID); err != nil {
		return err // 显式 tenant 过滤：跨租户 identity 不改指
	}
	me.Deleted = true // 空壳软删；业务数据不迁移（空壳刚注册无业务数据）
	return s.members.Update(ctx, me)
}

func (s *SocialService) UnbindSocial(ctx context.Context, memberID uint64, platform string) error {
	if !s.adapters.Exists(platform) {
		return errs.New(errs.CodeParamInvalid, nil)
	}
	tid := tenant.TenantIDFromContext(ctx)
	list, err := s.repo.ListIdentities(ctx, tid, memberID)
	if err != nil {
		return err
	}
	var target bool
	others := 0
	for _, idn := range list {
		if idn.Platform == platform {
			target = true
		} else {
			others++
		}
	}
	if !target {
		return nil // 幂等：没有该平台绑定
	}
	if s.members == nil {
		return oauth2svc.ErrAuthenticatorMissing
	}
	me, err := s.members.GetByID(ctx, tid, memberID)
	if err != nil {
		return errs.New(errs.CodeUserNotFound, err)
	}
	if me.Password == "" && others == 0 {
		return errs.New(errs.CodeSocialUnbindLast, nil) // Review Focus #7：防锁死
	}
	_, err = s.repo.DeleteIdentity(ctx, tid, memberID, platform)
	return err
}
```

（`mobileRe = regexp.MustCompile(`^1[3-9]\d{9}$`)` 包级变量，Task 7 首次引入。）

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./modules/system/oauth2/social/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add modules/system/oauth2/social/service.go modules/system/oauth2/social/service_test.go
git commit -m "feat(social): 绑定第三方/绑定手机号合并/解绑守门"
```

---

### Task 8: AppConfigService（admin CRUD 业务）

**Files:**
- Create: `modules/system/oauth2/social/appconfig.go`
- Test: `modules/system/oauth2/social/appconfig_test.go`

**Interfaces:**
- Consumes: Task 3 `SocialRepository`、Task 4 `AdapterRegistry`。
- Produces:

```go
type AppConfigDeps struct {
	fx.In
	Repo     SocialRepository
	Adapters *AdapterRegistry
}
func NewAppConfigService(deps AppConfigDeps) *AppConfigService
func (s *AppConfigService) List(ctx context.Context, page, size int, platform string) ([]*SocialApp, int64, error)
func (s *AppConfigService) Create(ctx context.Context, app *SocialApp) error
func (s *AppConfigService) Update(ctx context.Context, app *SocialApp) error
func (s *AppConfigService) Delete(ctx context.Context, id uint64) error
```

- [ ] **Step 1: 写失败测试**

```go
func TestAppConfig_Create_UnknownPlatform_10001(t *testing.T)   // Adapters.Exists false → CodeParamInvalid
func TestAppConfig_Create_MissingAppIDOrSecret_10002(t *testing.T) // errs.CodeParamMissing (10002 缺少必填参数)
func TestAppConfig_Create_DupPlatform_10001(t *testing.T)       // 同 (tid,platform) 已存在
func TestAppConfig_Create_OK_SecretStoredPlain(t *testing.T)    // 落库后 GetApp 能取回同 secret
func TestAppConfig_Update_SecretEmpty_KeepsOld(t *testing.T)    // Secret=="" → 旧值保留
func TestAppConfig_Update_UnknownPlatform_10001(t *testing.T)
func TestAppConfig_Update_CrossTenant_10001(t *testing.T)       // GetAppByID(tid 不符) → ErrRecordNotFound → 10001
func TestAppConfig_Update_PlatformTakenByOtherRow_10001(t *testing.T)
func TestAppConfig_List_PlatformFilter(t *testing.T)            // platform=="" 返回全部；指定只回该平台
func TestAppConfig_Delete_OK(t *testing.T)
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./modules/system/oauth2/social/ -run AppConfig -v`
Expected: FAIL

- [ ] **Step 3: 实现 appconfig.go**

规则（写全）：
- `Create`：`Adapters.Exists` 否 → `errs.CodeParamInvalid`；`AppID=="" || Secret==""` → `errs.CodeParamMissing`；`Status ∉ {0,1}` → `errs.CodeParamInvalid`；`GetApp(tid, platform)` 非 ErrRecordNotFound → `errs.CodeParamInvalid`（同平台重复）；`app.ID=0; app.TenantID=tid`（从 ctx 取）；`CreateApp`。
- `Update`：`ID==0` → `errs.CodeParamInvalid`；`Exists`/`Status` 同校验；`AppID==""` → `errs.CodeParamMissing`；`GetAppByID(tid, app.ID)` 未找到 → `errs.CodeParamInvalid`（跨租户不可见天然覆盖）；`Secret==""` → `app.Secret = old.Secret`；platform 变更且 `GetApp(tid, newPlatform)` 命中**不同 ID** → `errs.CodeParamInvalid`；`UpdateApp`。
- `List`：透传 repo（platform 可选）。
- `Delete`：`DeleteApp(tid, id)`（RowsAffected==0 → param invalid）。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./modules/system/oauth2/social/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add modules/system/oauth2/social/appconfig.go modules/system/oauth2/social/appconfig_test.go
git commit -m "feat(social): AppConfigService 渠道配置 CRUD 规则"
```

---

### Task 9: controllers（app 五端点 + admin 四端点）+ 路由 + permissionRoutes

**Files:**
- Create: `modules/system/oauth2/social/controller/app.go`
- Create: `modules/system/oauth2/social/controller/admin.go`
- Test: `modules/system/oauth2/social/controller/app_test.go`
- Test: `modules/system/oauth2/social/controller/admin_test.go`
- Modify: `modules/system/oauth2/module.go`（struct 字段 + RegisterRoutes 九条 + fx Provides controllers）
- Modify: `modules/system/service/permission_routes.go`
- Test: `modules/system/service/permission_routes_test.go`（加 TestPermissionRoutes_SocialAppRegistered）

**Interfaces:**
- Consumes: Task 6/7 `LoginBinder` 三方法 + Login、Task 8 `AppConfigService`；`token.Validator`；`resp.OK/Fail/Page/PageReq`；`errs`；scorecard `claims()` 模式。
- Produces: 九条路由（见下）；`permissionRoutes["social:app"]`。

- [ ] **Step 1: 写失败测试**

`app_test.go`——stub：

```go
type stubLoginBinder struct {
	login func(ctx context.Context, platform, code, ip, ua string) (*oauth2svc.LoginResult, error)
	bindSocial func(ctx context.Context, memberID uint64, platform, code string) error
	bindMobile func(ctx context.Context, memberID uint64, mobile string) error
	unbind     func(ctx context.Context, memberID uint64, platform string) error
	// 记录收到的 platform/memberID
}
type stubValidator struct{ claims *token.Claims; err error }
func (v *stubValidator) Validate(ctx context.Context, tok string) (*token.Claims, error) { return v.claims, v.err }
```

用例：
```go
func TestAppLogin_MP_RoutesWechatMP(t *testing.T)      // POST /app/social/wechat/mp/login {"code":"c"} → stub 收到 platform=wechat_mp；返回 200 {code:0,data:{token,...}}
func TestAppLogin_Web_RoutesWechatWeb(t *testing.T)    // 同上 wechat_web
func TestAppLogin_EmptyCode_10001(t *testing.T)        // {} → 400 + CodeParamInvalid
func TestAppLogin_IssuanceErr_503(t *testing.T)        // login 返回 ErrIssuance → HTTP 503 + unavailableBody{code:50000,msg:"auth service unavailable"}
func TestAppBindSocial_NoAuth_401(t *testing.T)        // 无 Authorization → 401
func TestAppBindSocial_WrongUserType_403(t *testing.T) // UserType=system → 403
func TestAppBindSocial_OK_PassesUserID(t *testing.T)   // Bearer + {"platform":"wechat_mp","code":"c"} → stub 收 memberID=claims.UserID
func TestAppBindMobile_OK(t *testing.T)
func TestAppUnbind_PlatformParam(t *testing.T)         // DELETE /app/member/auth/social/wechat_mp
```

`admin_test.go`——stub `AppConfiger`（List/Create/Update/Delete 四方法），gin TestMode + httptest：

```go
func TestAdminList_ReturnsJSON_NoSecretInBody(t *testing.T) // stub 返回带 Secret 的 SocialApp → body 字符串不含 secret 值（Review Focus #3）
func TestAdminCreate_BodyNeverEchoesSecret(t *testing.T)    // 同上，create 响应断言
func TestAdminCreate_PassesTenantFromCtx(t *testing.T)
func TestAdminUpdate_IDFromPath(t *testing.T)              // PUT /apps/7 → app.ID==7
func TestAdminDelete_IDFromPath(t *testing.T)
```

`permission_routes_test.go` 追加（先写，跑红）：

```go
func TestPermissionRoutes_SocialAppRegistered(t *testing.T) {
	patterns, ok := permissionRoutes["social:app"]
	if !ok {
		t.Fatal(`permissionRoutes 缺 "social:app"（渠道配置页会 403）`)
	}
	want := []string{"/api/admin/social/apps", "/api/admin/social/apps/*"}
	for _, w := range want {
		found := false
		for _, p := range patterns {
			if p == w {
				found = true
			}
		}
		if !found {
			t.Errorf("social:app 缺路由模式 %q, got %v", w, patterns)
		}
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./modules/system/... -run 'Social|AppLogin|AdminList' -v`
Expected: FAIL

- [ ] **Step 3: 实现 controllers + 路由 + 权限键**

`controller/app.go`：

```go
package controller

// LoginBinder SocialService 的 app 端门面（controller 只依赖接口，测试可 stub）。
type LoginBinder interface {
	Login(ctx context.Context, platform, code, ip, ua string) (*oauth2svc.LoginResult, error)
	BindSocial(ctx context.Context, memberID uint64, platform, code string) error
	BindMobile(ctx context.Context, memberID uint64, mobile string) error
	UnbindSocial(ctx context.Context, memberID uint64, platform string) error
}

type AppController struct {
	svc       LoginBinder
	validator token.Validator
}

func NewAppController(svc LoginBinder, validator token.Validator) *AppController {
	return &AppController{svc: svc, validator: validator}
}
```

`claims()` 逐字复制 scorecard 模式（Bearer 解析 → Validate → UserTypeMember 校验 → `tenant.WithTenantID(ctx.Request.Context(), cl.TenantID)` 写回，失败已写响应返回 ok=false）。`unavailableBody` 结构复制 member_auth.go：

```go
type unavailableBody struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}
```

共享 `login(ctx, platform)`：bind JSON `{code}`，code 空 → 10001；`svc.Login(ctx.Request.Context(), platform, code, ctx.ClientIP(), ctx.GetHeader("User-Agent"))`；`errors.Is(err, oauth2svc.ErrIssuance) || errors.Is(err, oauth2svc.ErrAuthenticatorMissing)` → `ctx.JSON(503, unavailableBody{Code: int(errs.CodeInternal), Msg: "auth service unavailable"})`；其他 → `resp.Fail`。`LoginMP` = login(social.PlatformWechatMP)，`LoginWeb` = login(social.PlatformWechatWeb)（import `oauth2/social`）。

Bind 三端点：先 `claims()`，再 bind body/param，空参 → 10001，调 svc 后 `resp.Fail/OK`。

`controller/admin.go`：`AppConfiger` 接口四方法同 Task 8 签名；List 用 `resp.PageReq` + `Defaults()` + `ctx.Query("platform")` + `resp.Page`；Create/Update/Delete 仿 oauth2 admin/client.go 但直接绑定 `model.SocialApp`（`json:"-"` 使 secret 恒不回显，无需 createClientResponse 拆分）；Update `id := pathID`（0 → 10001）→ `app.ID = id`。

`module.go`：`oauth2Module` struct 增 `socialApp *socialctl.AppController`、`socialAdmin *socialctl.AdminController`；`NewModule(...)` 签名加两参；fx.Provide 加 `socialctl.NewAppController, socialctl.NewAdminController` + `social.NewAppConfigService`；`RegisterRoutes` 增：

```go
app := r.Group("/app")
app.POST("/social/wechat/mp/login", m.socialApp.LoginMP)
app.POST("/social/wechat/web/login", m.socialApp.LoginWeb)
app.POST("/member/auth/bind-social", m.socialApp.BindSocial)
app.POST("/member/auth/bind-mobile", m.socialApp.BindMobile)
app.DELETE("/member/auth/social/:platform", m.socialApp.UnbindSocial)

soc := r.Group("/admin/social")
soc.GET("/apps", m.socialAdmin.List)
soc.POST("/apps", m.socialAdmin.Create)
soc.PUT("/apps/:id", m.socialAdmin.Update)
soc.DELETE("/apps/:id", m.socialAdmin.Delete)
```

`permission_routes.go`（紧邻 `system:online` 条目）：

```go
"social:app": {"/api/admin/social/apps", "/api/admin/social/apps/*"},
```

- [ ] **Step 4: 跑测试确认通过（全 system 包）**

Run: `go test ./modules/system/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add modules/system/oauth2/social/controller/ modules/system/oauth2/module.go modules/system/service/permission_routes.go modules/system/service/permission_routes_test.go
git commit -m "feat(social): app/admin 控制器 + 九条路由 + social:app 权限注册"
```

---

### Task 10: 菜单种子迁移 + 种子权限扫描测试

**Files:**
- Create: `modules/system/migrations/20261021000002_social_app_menu.up.sql`
- Create: `modules/system/migrations/20261021000002_social_app_menu.down.sql`
- Test: `modules/system/service/permission_routes_test.go`（加 TestSocialSeed_PermsRegistered）

**Interfaces:**
- Consumes: Task 9 的 `permissionRoutes["social:app"]`；模板 `20261011000003_oauth2_client_menu.up.sql`。
- Produces: 菜单 目录「渠道配置」/social → 页面 /social/apps → 按钮 social:app:create/:update/:delete → 绑 admin。

- [ ] **Step 1: 写失败测试**

```go
func TestSocialSeed_PermsRegistered(t *testing.T) {
	migPath := filepath.Join("..", "migrations")
	entries, err := os.ReadDir(migPath)
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	permRe := regexp.MustCompile(`'(social:[a-z0-9_:]+)'`)
	checked := 0
	for _, e := range entries {
		if !strings.Contains(e.Name(), "social_app_menu") {
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
		t.Error("social 菜单迁移未扫到任何权限码——文件名或正则失配，测试失效")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./modules/system/service/ -run TestSocialSeed -v`
Expected: FAIL（checked==0）

- [ ] **Step 3: 写种子迁移（逐字段仿 oauth2_client_menu）**

`20261021000002_social_app_menu.up.sql` 要点（完整 SQL 按 oauth2 模板逐段改写，全部 `NOT EXISTS` 幂等、`tenant_id=0`）：
- 目录：name「渠道配置」，path `/social`，permission `social`，type `dir`，parent_id = COALESCE(系统管理 id, 0)，sort 92，icon `lucide:link`。
- 页面：name「渠道配置」，path `/social/apps`，component `views/system/SocialAppsPage`，permission `social:app`，type `menu`，parent = 目录 id，sort 1。
- 按钮三枚：permission `social:app:create` / `social:app:update` / `social:app:delete`，type `button`，parent = 页面 id。
- role_menus：JOIN roles `code='admin'`，guard `permission LIKE 'social:%' OR path IN ('/social','/social/apps')`。

`.down.sql`：`DELETE FROM ... WHERE permission LIKE 'social:%' OR path IN ('/social','/social/apps')`（按种子涉及的表逐表删，仿既有 down 惯例）。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./modules/system/service/ -v`
Expected: PASS（TestSocialSeed_PermsRegistered checked>0 且全注册）

- [ ] **Step 5: Commit**

```bash
git add modules/system/migrations/20261021000002_social_app_menu.up.sql modules/system/migrations/20261021000002_social_app_menu.down.sql modules/system/service/permission_routes_test.go
git commit -m "feat(social): 渠道配置菜单种子 + 权限扫描测试"
```

---

### Task 11: 前端渠道配置页

**Files:**
- Modify: `apps/web-antd/src/api/admin.ts`
- Create: `apps/web-antd/src/views/system/SocialAppsPage.vue`
- Modify: `cspell.json`（words 加 `"wechat"`）

**Interfaces:**
- Consumes: Task 9 admin 四端点；模板 `Oauth2ClientsPage.vue`；菜单种子 component 路径 `views/system/SocialAppsPage`（access.ts glob 自动接，零路由改动）。
- Produces: `SocialApp` 接口 + `listSocialApps/createSocialApp/updateSocialApp/deleteSocialApp`。

- [ ] **Step 1: api/admin.ts 新增段（无 secret 字段）**

```ts
// ---- 社交登录渠道配置（微信一期） ----
export interface SocialApp {
  id: number;
  tenant_id: number;
  platform: string; // wechat_mp | wechat_web
  appid: string;
  status: number; // 1 启用 0 停用
  create_time: string;
  update_time: string;
  // secret 恒不回显（后端 json:"-"）——故接口不声明该字段
}

export interface SocialAppPayload {
  platform: string;
  appid: string;
  secret?: string; // 明文；编辑留空=保留旧值（后端 Update 语义）
  status: number;
}

export function listSocialAppsPage(params: {
  page: number;
  platform?: string;
  size: number;
}) {
  return requestClient.get<{ list: SocialApp[]; total: number }>(
    '/admin/social/apps',
    { params },
  );
}

export function createSocialApp(payload: SocialAppPayload) {
  return requestClient.post<SocialApp>('/admin/social/apps', payload);
}

export function updateSocialApp(id: number, payload: SocialAppPayload) {
  return requestClient.put<SocialApp>(`/admin/social/apps/${id}`, payload);
}

export function deleteSocialApp(id: number): Promise<void> {
  return requestClient.delete(`/admin/social/apps/${id}`);
}
```

- [ ] **Step 2: 写 SocialAppsPage.vue（克隆 Oauth2ClientsPage 结构）**

完整页（逐段对照 Oauth2ClientsPage.vue：VxeGrid + `#toolbar-tools` 新建按钮 + `#actions` 编辑/删除 + useVbenModal + useVbenForm），差异点：
- `defineOptions({ name: 'SocialAppsPage' })`。
- columns：`id`、`platform`（formatter 下拉值显示 微信小程序/微信网页）、`appid`、`status`（启用/停用 tag）、`操作`（编辑/删除）。
- toolbar 额外平台筛选 Select（options: 全部(空串)/微信小程序 wechat_mp/微信网页 wechat_web）→ change 时 `gridApi.commitProxy('query')`，query 参数带 `platform`。
- proxyConfig ajax.query 调 `listSocialAppsPage` 返回 `{items: list, total}`。
- form schema：platform Select（必填，options 两项，新建可选/编辑禁改可留可选）+ appid Input 必填 + secret `VbenInputPassword`（label 注明「编辑留空=保留原值」，编辑时 `formApi.setValues({secret: ''})`）+ status RadioGroup（1 启用/0 停用）。
- 新建校验 secret 必填（编辑不校验）；onConfirm 走 `formApi.validateAndSubmitForm` → create/update。
- 所有按钮 `v-access:code="'social:app:create'"` 等三枚。

- [ ] **Step 3: cspell.json words 加 `"wechat"`**

- [ ] **Step 4: 跑门禁**

Run: `pnpm check:type` 和 `pnpm check:cspell`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add apps/web-antd/src/api/admin.ts apps/web-antd/src/views/system/SocialAppsPage.vue cspell.json
git commit -m "feat(social): admin 渠道配置页（列表/筛选/新建/编辑/删除）"
```

---

### Task 12: docs 回填 + 全量回归

**Files:**
- Modify: `docs/modules.md`（oauth2 章节）

**Interfaces:**
- Consumes: Task 1–11 全部产物。
- Produces: 文档与回归绿。

- [ ] **Step 1: 回填 docs/modules.md oauth2 章节**

补：social 子包分层表、九端点、两表（system_social_app / system_member_identity）、30301–30306 错误码段、adapter 扩展点（新增平台 = 新 adapter 文件 + 新路由 + registry 注册，service 零改动）、micro 形态说明（social 随 oauth2 进 member-server，MemberStore optional）。

- [ ] **Step 2: 全量回归**

Run: `go test ./...`
Expected: 全包 0 FAIL（含既有 oauth2/member 测试零改动通过）

Run: `pnpm check:type`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add docs/modules.md
git commit -m "docs(social): 回填 modules.md oauth2 章节（第三方登录一期）"
```
