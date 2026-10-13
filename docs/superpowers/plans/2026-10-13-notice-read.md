# 通知已读 / 我的通知（P0-1）实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在现有公告 CRUD（`system:notice`）之上补全已读回执：每用户可看到自己对每条公告的已读状态，可单条/一键标记已读，并新增「我的通知」菜单页。

**Architecture:** 新表 `system_notice_reads`（UNIQUE(tenant_id,user_id,notice_id) 保证幂等）+ `NoticeService` 扩展三个方法（ListMine 两次查询 Go 内合并读状态，方言安全）+ BasicController 三个新端点（挂在既有 `/api/admin/system/notices` 前缀下，casbin 前缀匹配天然放行）+ seed.go 按 path 回补「我的通知」菜单（抄租户管理回补模式）+ 前端 `SystemMyNoticesPage.vue`（抄 SystemNoticesPage 骨架）。**不改** `permission_routes.go`、**不改** fx DI（扩展现有接口）。

**Tech Stack:** Go 1.25 + gin v1.12 + GORM + golang-migrate（file 源）+ glebarez/sqlite 内存测试；vben5（Vue3 + AntDV + vxe-table）。

**Spec:** 无正式 spec——权威输入是 `docs/roadmap.md` §P0-1（「已读回执表 + 我的未读/已读接口 + 我的通知页面（菜单种子）」）。设计裁定集中在本计划 Architecture 与各任务注释，冲突时以 roadmap 条目为准。

**路由预验（已做，不是任务步骤）:** gin v1.12.0 同层混挂静态段与参数段（`GET /notices/mine`、`POST /notices/read-all`、`POST /notices/:id/read` 与既有 `GET /notices`、`DELETE /notices/:id`）已用临时探针测试实证不 panic、全部正确匹配，探针已删。Task 2 的测试路由按 module.go 完整七路由同形装配，作常驻回归。

## Global Constraints

- macOS 一律 `CGO_ENABLED=0` 跑 Go 测试/构建（默认 CGO 链接失败是本机环境问题，CI 不受影响）；全量口径 `CGO_ENABLED=0 go test ./... -count=1` 全绿。
- 无 testify/gomock：service 层测试用 `gorm.Open(sqlite.Open(":memory:"))` + 真 repo + 手写断言；controller 层用 `gin.SetMode(gin.TestMode)` + `httptest` + fake + 手写断言。
- 前端 typecheck 口径：`pnpm --filter @vben/web-antd run typecheck`（根目录无裸 `typecheck` 脚本）。
- **直接落 main、不建功能分支、不 push**（push 挂起等用户决定）；提交信息中文、conventional 前缀（`feat:`/`docs:`/`test:`）。
- 响应信封：读 `{"data":...}`、写 `{"status":"ok"}`、错误 `{"error":...}`（400 客户端错 / 500 服务端错 / 缺 claims 401）。
- 权限三层链路：菜单 `menus.permission` 唯一数据源 → 前端 `v-access:code` 逐字匹配 → 后端 casbin 取权限码**前两段**（permPrefix）查 `permissionRoutes` + keyMatch2。新权限码前两段必须落在已注册组上，否则死权限 403。**本计划不改 `permission_routes.go`**（`system:notice` → `/api/admin/system/notices/*` 已覆盖新端点）。
- 租户：service 层一律 `tenant.TenantIDFromContext(ctx)`；不改 GORM 租户插件配置；不改 casbin sub 结构。
- 迁移：golang-migrate file 源，`modules/system/migrations/` 加 `20261013000001_notice_reads.{up,down}.sql`（须大于既有最大版本 `20261011000003`），**只加文件、不改 Go 注册**；版本账本 `schema_migrations_system` 按系统模块独立、时间序递增；幂等可重复跑。
- 菜单种子：seed.go 每次启动跑，必须 path 幂等（`hasMenuPath` 判据），且新菜单 append 进 `allMenus`（尾部 `SetRoleMenus` 才会绑给 admin 角色）。
- 中文注释；代码风格抄所在文件既有写法（一方法一行风格、`gin.H` 信封、`ctx *gin.Context` 形参名）。

## Review Focus

以下五条是本功能最可能咬人的失败模式，各自钉在拥有其代码的任务里：

1. **重复标记不幂等**（用户双击/重试 → 唯一键冲突 500，或落重复行）：同一用户对同一公告 MarkRead 三次须无错且库中恰一行 → Task 1 `TestNoticeService_MarkRead_Idempotent`（sqlite 真唯一索引 + `clause.OnConflict` 真跑，直接 `Count` 断言行数）。
2. **跨租户串读**（租户 1 的 ctx 能标记/看见租户 2 的公告，或 MarkAllRead 扫到别租户）：租户 2 的公告对租户 1 ctx 必须 `ErrNoticeNotFound`，MarkAllRead 后回执行数只等于本租户公告数 → Task 1 `TestNoticeService_MarkRead_NotFound` + `TestNoticeService_MarkAllRead`。
3. **claims 缺失 panic/500**（中间件被绕过、测试直打）：无 claims 打 mine 端点必须 401 信封，不 500 不 panic → Task 2 `TestListMyNotices_NoClaims_401`。
4. **gin 静态/参数同层混挂 panic**（构建路由树即 panic → 整个进程起不来）：测试路由按 module.go 完整七路由同形装配（含 `GET /notices`+`GET /notices/mine`、`POST /notices`+`read-all`+`:id/read`、`DELETE /notices/:id`），每个 Task 2 用例跑过即回归 → Task 2 `newNoticeRouter`（全部 6 个用例共用）。
5. **菜单死权限**（页面可见但接口 403：`system:notice:mine` 前两段没落进 permissionRoutes）：新菜单权限码的 permPrefix 必须命中 `permissionRoutes` 注册键 → Task 3 `TestSeedMyNoticesPagePerm_MatchRouteKey`（抄租户回补同款防线）。

---

### Task 1: 已读回执数据层（表迁移 + model + repo + service + 测试）

**Files:**
- Create: `modules/system/migrations/20261013000001_notice_reads.up.sql`
- Create: `modules/system/migrations/20261013000001_notice_reads.down.sql`
- Modify: `modules/system/model/basic.go`（`SystemNotice` TableName 之后，约 :82）
- Modify: `modules/system/repository/basic.go`（`NoticeRepository` 接口 :42-47 与实现 :115-124；import 加 `errors`、`gorm.io/gorm/clause`）
- Modify: `modules/system/service/basic.go`（`NoticeService` 接口 :44-49 与实现 :194-219；import 加 `errors`）
- Test: `modules/system/service/notice_read_test.go`（新建）

**Interfaces:**
- Consumes: 既有 `repository.NoticeRepository.List/Create/Update/Delete`、`tenant.TenantIDFromContext(ctx)`、`model.SystemNotice`。
- Produces（Task 2 依赖，签名必须逐字一致）:
  - `service.NoticeService` 新增方法：`ListMine(ctx context.Context, userID uint64) ([]*NoticeItem, error)`、`MarkRead(ctx context.Context, userID, noticeID uint64) error`、`MarkAllRead(ctx context.Context, userID uint64) error`
  - `service.NoticeItem{ *model.SystemNotice; ReadStatus int }`（`ReadStatus` json tag `read_status`，0 未读 / 1 已读）
  - `service.ErrNoticeNotFound`（`errors.New("notice not found")`，Task 2 用 `errors.Is` 转 400）
  - `model.SystemNoticeRead`（TableName `system_notice_reads`，三列组合唯一索引）
  - 迁移 `20261013000001_notice_reads`

- [ ] **Step 1: 写失败测试**

新建 `modules/system/service/notice_read_test.go`：

```go
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/repository"
	"alexGo-cloud/pkg/tenant"
)

// setupNoticeService：sqlite :memory: + 公告/已读回执表，真 repo（抄 setupMenuService 模式）。
// 返回 db 供行数断言（幂等、跨租户回执隔离都要数行）。
func setupNoticeService(t *testing.T) (NoticeService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.SystemNotice{}, &model.SystemNoticeRead{}); err != nil {
		t.Fatal(err)
	}
	return NewNoticeService(repository.NewNoticeRepository(db)), db
}

// seedNotice 走 service.Create 造一行公告（自带租户/时间戳），返回 id。
func seedNotice(t *testing.T, svc NoticeService, ctx context.Context, title string) uint64 {
	t.Helper()
	n, err := svc.Create(ctx, &model.SystemNotice{Title: title, Content: "内容-" + title})
	if err != nil {
		t.Fatalf("seed notice %q: %v", title, err)
	}
	return n.ID
}

// 读状态合并：ListMine 按 id desc 返回，read_status 逐条正确（0 未读 / 1 已读）。
func TestNoticeService_ListMine_ReadStatus(t *testing.T) {
	svc, _ := setupNoticeService(t)
	ctx := tenant.WithTenantID(context.Background(), 1)
	idA := seedNotice(t, svc, ctx, "A")
	seedNotice(t, svc, ctx, "B")

	if err := svc.MarkRead(ctx, 7, idA); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	items, err := svc.ListMine(ctx, 7)
	if err != nil {
		t.Fatalf("ListMine: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	// 列表 id desc：B 后建（id 大）在前、未读；A 在后、已读
	if items[0].Title != "B" || items[0].ReadStatus != 0 {
		t.Errorf("items[0] = %s/read=%d, want B/0", items[0].Title, items[0].ReadStatus)
	}
	if items[1].Title != "A" || items[1].ReadStatus != 1 {
		t.Errorf("items[1] = %s/read=%d, want A/1", items[1].Title, items[1].ReadStatus)
	}
}

// 幂等（Review Focus 1）：重复标记不报错，库中恰一行。
func TestNoticeService_MarkRead_Idempotent(t *testing.T) {
	svc, db := setupNoticeService(t)
	ctx := tenant.WithTenantID(context.Background(), 1)
	id := seedNotice(t, svc, ctx, "A")

	for i := 0; i < 3; i++ {
		if err := svc.MarkRead(ctx, 7, id); err != nil {
			t.Fatalf("MarkRead #%d: %v", i+1, err)
		}
	}
	var rows int64
	if err := db.Model(&model.SystemNoticeRead{}).Count(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Errorf("receipt rows = %d, want 1", rows)
	}
}

// 不存在与跨租户都返回 ErrNoticeNotFound（Review Focus 2 前半）。
func TestNoticeService_MarkRead_NotFound(t *testing.T) {
	svc, _ := setupNoticeService(t)
	ctx1 := tenant.WithTenantID(context.Background(), 1)
	ctx2 := tenant.WithTenantID(context.Background(), 2)

	if err := svc.MarkRead(ctx1, 7, 999); !errors.Is(err, ErrNoticeNotFound) {
		t.Fatalf("missing id err = %v, want ErrNoticeNotFound", err)
	}
	crossID := seedNotice(t, svc, ctx2, "T2")
	if err := svc.MarkRead(ctx1, 7, crossID); !errors.Is(err, ErrNoticeNotFound) {
		t.Fatalf("cross-tenant err = %v, want ErrNoticeNotFound", err)
	}
}

// 全部已读：本租户全变已读；别租户不受影响、回执行数只等于本租户公告数（Review Focus 2 后半）。
func TestNoticeService_MarkAllRead(t *testing.T) {
	svc, db := setupNoticeService(t)
	ctx1 := tenant.WithTenantID(context.Background(), 1)
	ctx2 := tenant.WithTenantID(context.Background(), 2)
	seedNotice(t, svc, ctx1, "A")
	seedNotice(t, svc, ctx1, "B")
	seedNotice(t, svc, ctx2, "T2")

	if err := svc.MarkAllRead(ctx1, 7); err != nil {
		t.Fatalf("MarkAllRead: %v", err)
	}
	items, err := svc.ListMine(ctx1, 7)
	if err != nil {
		t.Fatalf("ListMine: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	for _, it := range items {
		if it.ReadStatus != 1 {
			t.Errorf("notice %q read=%d, want 1 after MarkAllRead", it.Title, it.ReadStatus)
		}
	}
	other, err := svc.ListMine(ctx2, 7)
	if err != nil {
		t.Fatalf("ListMine tenant2: %v", err)
	}
	if len(other) != 1 || other[0].ReadStatus != 0 {
		t.Errorf("tenant2 items = %+v, want 1 条且未读", other)
	}
	var rows int64
	if err := db.Model(&model.SystemNoticeRead{}).Count(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if rows != 2 {
		t.Errorf("receipt rows = %d, want 2（仅租户1 两条）", rows)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `CGO_ENABLED=0 go test ./modules/system/service/ -count=1 -run TestNoticeService`
Expected: **编译失败**（red in Go 即编译错）——`undefined: svc.MarkRead`（`NoticeService` 尚无新方法）、`undefined: ErrNoticeNotFound`、`undefined: model.SystemNoticeRead`。

- [ ] **Step 3: 实现数据层（迁移 + model + repo + service）**

**3a.** Create `modules/system/migrations/20261013000001_notice_reads.up.sql`：

```sql
-- 公告已读回执（P0-1 通知已读）：每租户每用户对每公告一行。
-- 幂等：CREATE TABLE IF NOT EXISTS 可重复跑。
-- 唯一键 uk(tenant_id,user_id,notice_id) 是幂等写的基础：service 层用
-- gorm clause.OnConflict DoNothing（MySQL 方言译作 INSERT IGNORE），
-- 重复标记不报错、不落重复行。
-- 表结构对齐 20260408000006_basic_admin（id 自增、created_at/updated_at、tenant_id、InnoDB utf8mb4）。
CREATE TABLE IF NOT EXISTS `system_notice_reads` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0,
  `user_id` bigint unsigned NOT NULL,
  `notice_id` bigint unsigned NOT NULL,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_notice_reads_tenant_user_notice` (`tenant_id`,`user_id`,`notice_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='公告已读回执';
```

**3b.** Create `modules/system/migrations/20261013000001_notice_reads.down.sql`：

```sql
DROP TABLE IF EXISTS `system_notice_reads`;
```

**3c.** Modify `modules/system/model/basic.go`——在 `func (SystemNotice) TableName() string { return "system_notices" }` 之后插入：

```go
// SystemNoticeRead 公告已读回执：每租户每用户对每公告一行。
// 三列组合 uniqueIndex 同时服务两处：生产建表由上面迁移的 uk 落地、
// 测试 AutoMigrate 由本标签落地——缺任一处，clause.OnConflict 无冲突目标可依，
// 幂等写会退化成重复插行（NoticeRead_Idempotent 测试钉住）。
type SystemNoticeRead struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	TenantID  uint64    `gorm:"uniqueIndex:uk_notice_reads_tenant_user_notice" json:"tenant_id"`
	UserID    uint64    `gorm:"uniqueIndex:uk_notice_reads_tenant_user_notice" json:"user_id"`
	NoticeID  uint64    `gorm:"uniqueIndex:uk_notice_reads_tenant_user_notice" json:"notice_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (SystemNoticeRead) TableName() string { return "system_notice_reads" }
```

**3d.** Modify `modules/system/repository/basic.go`：

import 块改为：

```go
import (
	"context"
	"errors"

	"alexGo-cloud/modules/system/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)
```

`NoticeRepository` 接口（:42-47）整体替换为：

```go
type NoticeRepository interface {
	List(ctx context.Context, tenantID uint64) ([]*model.SystemNotice, error)
	Create(ctx context.Context, n *model.SystemNotice) error
	Update(ctx context.Context, n *model.SystemNotice) error
	Delete(ctx context.Context, tenantID uint64, id uint64) error
	// 以下为已读回执（P0-1 通知已读）
	FindByID(ctx context.Context, tenantID, id uint64) (*model.SystemNotice, error) // 不存在返回 (nil, nil)
	ListReadIDs(ctx context.Context, tenantID, userID uint64) (map[uint64]bool, error)
	InsertRead(ctx context.Context, r *model.SystemNoticeRead) error                // 幂等：uk 冲突 DoNothing
	ListNoticeIDs(ctx context.Context, tenantID uint64) ([]uint64, error)
}
```

在 `noticeRepo` 既有方法（:115-124）之后追加实现：

```go
func (r *noticeRepo) FindByID(ctx context.Context, tenantID, id uint64) (*model.SystemNotice, error) {
	var n model.SystemNotice
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&n).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func (r *noticeRepo) ListReadIDs(ctx context.Context, tenantID, userID uint64) (map[uint64]bool, error) {
	var ids []uint64
	err := r.db.WithContext(ctx).Model(&model.SystemNoticeRead{}).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Pluck("notice_id", &ids).Error
	if err != nil {
		return nil, err
	}
	set := make(map[uint64]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set, nil
}

func (r *noticeRepo) InsertRead(ctx context.Context, rd *model.SystemNoticeRead) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "user_id"}, {Name: "notice_id"}},
		DoNothing: true,
	}).Create(rd).Error
}

func (r *noticeRepo) ListNoticeIDs(ctx context.Context, tenantID uint64) ([]uint64, error) {
	var ids []uint64
	err := r.db.WithContext(ctx).Model(&model.SystemNotice{}).
		Where("tenant_id = ?", tenantID).Pluck("id", &ids).Error
	return ids, err
}
```

**3e.** Modify `modules/system/service/basic.go`：

import 块加 `"errors"`（保持既有分组顺序，`errors` 放 `context` 前）：

```go
import (
	"context"
	"errors"
	"time"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/repository"
	"alexGo-cloud/pkg/tenant"
)
```

`NoticeService` 接口（:44-49）整体替换为：

```go
type NoticeService interface {
	List(ctx context.Context) ([]*model.SystemNotice, error)
	Create(ctx context.Context, n *model.SystemNotice) (*model.SystemNotice, error)
	Update(ctx context.Context, n *model.SystemNotice) error
	Delete(ctx context.Context, id uint64) error
	// 以下为已读回执（P0-1 通知已读）
	ListMine(ctx context.Context, userID uint64) ([]*NoticeItem, error)
	MarkRead(ctx context.Context, userID, noticeID uint64) error
	MarkAllRead(ctx context.Context, userID uint64) error
}

// NoticeItem 我的通知响应项：公告字段经内嵌指针平铺 + 当前用户已读状态。
type NoticeItem struct {
	*model.SystemNotice
	ReadStatus int `json:"read_status"` // 0 未读 1 已读
}

// ErrNoticeNotFound 标记已读的目标公告不存在（不存在或跨租户），controller 转 400。
var ErrNoticeNotFound = errors.New("notice not found")
```

在 `noticeService` 既有方法（:194-219）之后追加实现：

```go
func (s *noticeService) ListMine(ctx context.Context, userID uint64) ([]*NoticeItem, error) {
	tid := tenant.TenantIDFromContext(ctx)
	notices, err := s.repo.List(ctx, tid)
	if err != nil {
		return nil, err
	}
	readIDs, err := s.repo.ListReadIDs(ctx, tid, userID)
	if err != nil {
		return nil, err
	}
	items := make([]*NoticeItem, 0, len(notices))
	for _, n := range notices {
		item := &NoticeItem{SystemNotice: n}
		if readIDs[n.ID] {
			item.ReadStatus = 1
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *noticeService) MarkRead(ctx context.Context, userID, noticeID uint64) error {
	tid := tenant.TenantIDFromContext(ctx)
	n, err := s.repo.FindByID(ctx, tid, noticeID)
	if err != nil {
		return err
	}
	if n == nil {
		return ErrNoticeNotFound
	}
	now := time.Now()
	return s.repo.InsertRead(ctx, &model.SystemNoticeRead{
		TenantID: tid, UserID: userID, NoticeID: noticeID,
		CreatedAt: now, UpdatedAt: now,
	})
}

func (s *noticeService) MarkAllRead(ctx context.Context, userID uint64) error {
	tid := tenant.TenantIDFromContext(ctx)
	ids, err := s.repo.ListNoticeIDs(ctx, tid)
	if err != nil {
		return err
	}
	now := time.Now()
	for _, id := range ids {
		if err := s.repo.InsertRead(ctx, &model.SystemNoticeRead{
			TenantID: tid, UserID: userID, NoticeID: id,
			CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `CGO_ENABLED=0 go test ./modules/system/service/ -count=1 -run TestNoticeService`
Expected: `ok`，5 个用例全过（LIST 下 5 个 `TestNoticeService_*`）。

Run: `CGO_ENABLED=0 go build ./...`
Expected: 无输出（全仓编译过）。

- [ ] **Step 5: 提交**

```bash
git add modules/system/migrations/20261013000001_notice_reads.* modules/system/model/basic.go modules/system/repository/basic.go modules/system/service/basic.go modules/system/service/notice_read_test.go
git commit -m "feat(system): 公告已读回执数据层（表迁移+repo+service+测试）"
```

---

### Task 2: 我的通知 HTTP 端点（controller + 路由 + 测试）

**Files:**
- Modify: `modules/system/controller/admin/basic.go`（import 加 `errors` 与 `alexGo-cloud/pkg/auth`；新增 `claimsOf` helper；文件末尾追加 3 个 handler）
- Modify: `modules/system/module.go:211-214` 之后（追加 3 行路由）
- Test: `modules/system/controller/admin/notice_read_test.go`（新建，本包首个测试文件）

**Interfaces:**
- Consumes（Task 1，签名逐字）: `service.NoticeService.ListMine/MarkRead/MarkAllRead`、`service.NoticeItem`、`service.ErrNoticeNotFound`；中间件注入 `c.Set("claims", *auth.Claims)`（`pkg/middleware/auth.go` 既有行为，handler 侧纵深防御再取一次）。
- Produces（Task 3 前端依赖的端点契约）:
  - `GET  /api/admin/system/notices/mine` → 200 `{"data":[{...公告字段平铺, "read_status":0|1}]}`；无 claims → 401 `{"error":"unauthorized"}`
  - `POST /api/admin/system/notices/:id/read` → 200 `{"status":"ok"}`；`id=0` 或非数字 → 400 `{"error":"invalid id"}`；公告不存在/跨租户 → 400 `{"error":"notice not found"}`
  - `POST /api/admin/system/notices/read-all` → 200 `{"status":"ok"}`
  - 全部挂在既有 `system:notice` 权限前缀下（**不改 permission_routes.go**）

- [ ] **Step 1: 写失败测试**

新建 `modules/system/controller/admin/notice_read_test.go`：

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

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/service"
	"alexGo-cloud/pkg/auth"
)

// fakeNoticeSvc：可编程 NoticeService 桩（本包无 testify，手写）。
type fakeNoticeSvc struct {
	items    []*service.NoticeItem
	readErr  error  // 非 nil 时 MarkRead/MarkAllRead 返回
	markID   uint64 // 最近一次 MarkRead 的 noticeID
	markUser uint64 // 最近一次 MarkRead/MarkAllRead 的 userID
	markAll  bool
}

func (f *fakeNoticeSvc) List(context.Context) ([]*model.SystemNotice, error) {
	return nil, nil
}
func (f *fakeNoticeSvc) Create(_ context.Context, n *model.SystemNotice) (*model.SystemNotice, error) {
	return n, nil
}
func (f *fakeNoticeSvc) Update(context.Context, *model.SystemNotice) error { return nil }
func (f *fakeNoticeSvc) Delete(context.Context, uint64) error              { return nil }
func (f *fakeNoticeSvc) ListMine(context.Context, uint64) ([]*service.NoticeItem, error) {
	return f.items, nil
}
func (f *fakeNoticeSvc) MarkRead(_ context.Context, userID, noticeID uint64) error {
	if f.readErr != nil {
		return f.readErr
	}
	f.markID, f.markUser = noticeID, userID
	return nil
}
func (f *fakeNoticeSvc) MarkAllRead(_ context.Context, userID uint64) error {
	if f.readErr != nil {
		return f.readErr
	}
	f.markUser, f.markAll = userID, true
	return nil
}

// newNoticeRouter 按 module.go:211-214 + 新增三行的同形顺序装配**全部七个**公告路由
// （静态段 mine/read-all 与参数段 :id 混挂——Review Focus 4：树构建 panic 会让本用例全灭）。
// claims 为 nil 模拟中间件缺失。同包测试直接构造结构体字面量，绕开 NewBasicController 的 5 依赖。
func newNoticeRouter(svc service.NoticeService, claims *auth.Claims) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	ctrl := &BasicController{noticeSvc: svc}
	g := r.Group("/api/admin/system")
	withClaims := func(h gin.HandlerFunc) gin.HandlerFunc {
		return func(c *gin.Context) {
			if claims != nil {
				c.Set("claims", claims)
			}
			h(c)
		}
	}
	g.GET("/notices", withClaims(ctrl.ListNotices))
	g.POST("/notices", withClaims(ctrl.CreateNotice))
	g.PUT("/notices", withClaims(ctrl.UpdateNotice))
	g.DELETE("/notices/:id", withClaims(ctrl.DeleteNotice))
	g.GET("/notices/mine", withClaims(ctrl.ListMyNotices))
	g.POST("/notices/read-all", withClaims(ctrl.ReadAllNotices))
	g.POST("/notices/:id/read", withClaims(ctrl.MarkNoticeRead))
	return r
}

func doReq(r *gin.Engine, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(method, path, nil))
	return w
}

// mine 信封：data 平铺公告字段 + read_status（NoticeItem 内嵌指针展开）。
func TestListMyNotices_OK(t *testing.T) {
	svc := &fakeNoticeSvc{items: []*service.NoticeItem{
		{SystemNotice: &model.SystemNotice{ID: 1, Title: "A", Content: "c"}, ReadStatus: 1},
		{SystemNotice: &model.SystemNotice{ID: 2, Title: "B", Content: "c"}, ReadStatus: 0},
	}}
	w := doReq(newNoticeRouter(svc, &auth.Claims{UserID: 42, TenantID: 1}),
		http.MethodGet, "/api/admin/system/notices/mine")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data []struct {
			ID         uint64 `json:"id"`
			Title      string `json:"title"`
			ReadStatus int    `json:"read_status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("data len = %d, want 2", len(resp.Data))
	}
	if resp.Data[0].Title != "A" || resp.Data[0].ReadStatus != 1 ||
		resp.Data[1].Title != "B" || resp.Data[1].ReadStatus != 0 {
		t.Errorf("data = %+v, want A/1 B/0", resp.Data)
	}
}

// Review Focus 3：缺 claims 必须 401，不 500 不 panic。
func TestListMyNotices_NoClaims_401(t *testing.T) {
	w := doReq(newNoticeRouter(&fakeNoticeSvc{}, nil),
		http.MethodGet, "/api/admin/system/notices/mine")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "unauthorized") {
		t.Errorf("body = %s, want unauthorized", w.Body.String())
	}
}

func TestMarkNoticeRead_OK(t *testing.T) {
	svc := &fakeNoticeSvc{}
	w := doReq(newNoticeRouter(svc, &auth.Claims{UserID: 42}),
		http.MethodPost, "/api/admin/system/notices/5/read")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if svc.markID != 5 || svc.markUser != 42 {
		t.Errorf("MarkRead(notice=%d, user=%d), want (5, 42)", svc.markID, svc.markUser)
	}
	if !strings.Contains(w.Body.String(), `"status":"ok"`) {
		t.Errorf("body = %s, want {\"status\":\"ok\"}", w.Body.String())
	}
}

func TestMarkNoticeRead_NotFound_400(t *testing.T) {
	svc := &fakeNoticeSvc{readErr: service.ErrNoticeNotFound}
	w := doReq(newNoticeRouter(svc, &auth.Claims{UserID: 42}),
		http.MethodPost, "/api/admin/system/notices/5/read")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "notice not found") {
		t.Errorf("body = %s, want notice not found", w.Body.String())
	}
}

func TestMarkNoticeRead_ZeroID_400(t *testing.T) {
	w := doReq(newNoticeRouter(&fakeNoticeSvc{}, &auth.Claims{UserID: 42}),
		http.MethodPost, "/api/admin/system/notices/0/read")
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "invalid id") {
		t.Errorf("status = %d body = %s, want 400 invalid id", w.Code, w.Body.String())
	}
}

func TestReadAllNotices_OK(t *testing.T) {
	svc := &fakeNoticeSvc{}
	w := doReq(newNoticeRouter(svc, &auth.Claims{UserID: 42}),
		http.MethodPost, "/api/admin/system/notices/read-all")
	if w.Code != http.StatusOK || !svc.markAll || svc.markUser != 42 {
		t.Errorf("status = %d markAll = %v user = %d, want 200/true/42", w.Code, svc.markAll, svc.markUser)
	}
	if !strings.Contains(w.Body.String(), `"status":"ok"`) {
		t.Errorf("body = %s, want {\"status\":\"ok\"}", w.Body.String())
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `CGO_ENABLED=0 go test ./modules/system/controller/admin/ -count=1`
Expected: **编译失败**——`ctrl.ListMyNotices undefined (type *BasicController has no field or method ListMyNotices)`（`MarkNoticeRead`、`ReadAllNotices` 同理）。路由混挂若真会 panic，也在本步构建/跑用例时暴露。

- [ ] **Step 3: 实现 controller 与路由**

**3a.** Modify `modules/system/controller/admin/basic.go`——import 块改为：

```go
import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/service"
	"alexGo-cloud/pkg/auth"
)
```

**3b.** 在 `NewBasicController` 函数之后（`ListDepts` 之前）插入 helper：

```go
// claimsOf 取中间件注入的 claims（生产路径由 auth 中间件保证；handler 内再判一次作纵深防御，
// 与 controller/vben 的同名 helper 同构）。
func claimsOf(ctx *gin.Context) (*auth.Claims, bool) {
	v, exists := ctx.Get("claims")
	if !exists {
		return nil, false
	}
	cl, ok := v.(*auth.Claims)
	if !ok || cl == nil {
		return nil, false
	}
	return cl, true
}
```

**3c.** 文件末尾（`DeleteNotice` 之后）追加三个 handler：

```go
// ListMyNotices 我的通知：本租户公告 + 当前用户已读标记（P0-1）。
func (c *BasicController) ListMyNotices(ctx *gin.Context) {
	claims, ok := claimsOf(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	list, err := c.noticeSvc.ListMine(ctx.Request.Context(), claims.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": list})
}

// MarkNoticeRead 标记单条已读（幂等；公告不存在或跨租户 → 400）。
func (c *BasicController) MarkNoticeRead(ctx *gin.Context) {
	claims, ok := claimsOf(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := c.noticeSvc.MarkRead(ctx.Request.Context(), claims.UserID, id); err != nil {
		if errors.Is(err, service.ErrNoticeNotFound) {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ReadAllNotices 一键全部标记已读（幂等）。
func (c *BasicController) ReadAllNotices(ctx *gin.Context) {
	claims, ok := claimsOf(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if err := c.noticeSvc.MarkAllRead(ctx.Request.Context(), claims.UserID); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}
```

**3d.** Modify `modules/system/module.go`——`adminGroup.DELETE("/notices/:id", ...)`（:214）之后追加：

```go
	// P0-1 我的通知：静态段（mine/read-all）与参数段（:id/read）同层混挂，
	// gin v1.12 实测不 panic（静态优先匹配，探针已验）——注册顺序即下三行与上四行的同形顺序，
	// 测试侧 notice_read_test.go 的 newNoticeRouter 保持同形作常驻回归。
	adminGroup.GET("/notices/mine", m.basicCtrl.ListMyNotices)
	adminGroup.POST("/notices/read-all", m.basicCtrl.ReadAllNotices)
	adminGroup.POST("/notices/:id/read", m.basicCtrl.MarkNoticeRead)
```

- [ ] **Step 4: 跑测试确认通过**

Run: `CGO_ENABLED=0 go test ./modules/system/... -count=1`
Expected: 全部包 `ok`（含新增 `.../controller/admin` 包 6 个用例）。

- [ ] **Step 5: 提交**

```bash
git add modules/system/controller/admin/basic.go modules/system/controller/admin/notice_read_test.go modules/system/module.go
git commit -m "feat(system): 我的通知/已读三端点（mine、单条 read、read-all）"
```

---

### Task 3: 菜单种子回补 + 我的通知前端页 + 文档收口

**Files:**
- Modify: `modules/system/service/seed.go`（租户管理回补块 :299-317 之后插入我的通知回补）
- Test: `modules/system/service/seed_test.go`（末尾加权限码守卫测试）
- Modify: `apps/web-antd/src/api/admin.ts`（`Notice` 接口 :77-83 之后加 `NoticeItem`；通知公告函数段 :278-295 之后加三函数）
- Create: `apps/web-antd/src/views/system/SystemMyNoticesPage.vue`
- Modify: `docs/modules.md`（8 处精确 diff，见 Step 6）
- Modify: `docs/roadmap.md`（Step 9 状态收口）

**Interfaces:**
- Consumes（Task 2 端点契约）: `GET /admin/system/notices/mine`、`POST /admin/system/notices/:id/read`、`POST /admin/system/notices/read-all`（requestClient 自动带 `/api` 前缀与 token）。
- Produces: 菜单 `system:notice:mine`（path `/system/notices/mine`，component `views/system/SystemMyNoticesPage`，seed path 幂等回补 + append allMenus → SetRoleMenus 绑 admin）；页面 `SystemMyNoticesPage`；文档/roadmap 状态。

**Ruling（相对早前设想的变更，执行时以此为准）:** 菜单**不走独立迁移**，走 seed.go path 回补——抄租户管理既有模式：fresh 库与老库统一路径、parent 锚定 `/system` 目录（无 COALESCE parent=0 落根问题）、append `allMenus` 后尾部 `SetRoleMenus` 免费完成角色绑定与 casbin 重建。表迁移只有 Task 1 的 `20261013000001_notice_reads`。

- [ ] **Step 1: 写权限码守卫测试（先钉后实现）**

Modify `modules/system/service/seed_test.go`——`TestSeedTenantPagePerm_MatchRouteKey` 之后追加：

```go
// 我的通知（P0-1）回补页面的权限码同样必须落在 permissionRoutes 已注册组上——
// system:notice:mine 前两段 system:notice 已映射 /api/admin/system/notices/*，
// 否则页面可见、接口 403（死权限）。
func TestSeedMyNoticesPagePerm_MatchRouteKey(t *testing.T) {
	const perm = "system:notice:mine"
	if _, ok := permissionRoutes[permPrefix(perm)]; !ok {
		t.Errorf("my notices page %q → prefix %q has no permissionRoutes entry", perm, permPrefix(perm))
	}
}
```

Run: `CGO_ENABLED=0 go test ./modules/system/service/ -count=1 -run TestSeedMyNotices`
Expected: **PASS**（守卫测试钉既有注册状态——`system:notice` 已在 permissionRoutes；这是回归钉，不是红绿驱动，红了说明有人动了 permissionRoutes）。

- [ ] **Step 2: 实现 seed.go 菜单回补**

Modify `modules/system/service/seed.go`——租户管理回补块（`if rootID != 0 && !hasMenuPath(allMenus, "/system/tenants") { ... }`，:299-317）之后、按钮补齐 `for _, b := range []struct{` 之前插入：

```go
	// 我的通知页面回补（P0-1 通知已读）：与租户管理同模式——按 path 幂等回补，
	// fresh 库与老库统一走本路径；权限码 system:notice:mine 前两段命中 system:notice
	// 既有路由组（seed_test 有守卫）。append 进 allMenus → 尾部 SetRoleMenus 绑 admin。
	// sort=12 排在租户管理（11）之后，不做存量重排。
	if rootID != 0 && !hasMenuPath(allMenus, "/system/notices/mine") {
		myNotices := &model.Menu{
			ParentID:   rootID,
			Type:       "menu",
			Name:       "我的通知",
			Path:       "/system/notices/mine",
			Component:  "views/system/SystemMyNoticesPage",
			Icon:       "lucide:mail-open",
			Permission: "system:notice:mine",
			Sort:       12,
			Status:     1,
			TenantID:   tid,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if err := p.Menus.Create(ctx, myNotices); err == nil {
			allMenus = append(allMenus, myNotices)
		}
	}
```

注：图标 `lucide:mail-open`（已在 `@iconify/json` lucide 集合核实存在；`mail-opened` **不存在**，勿改回）。

- [ ] **Step 3: 跑后端全量**

Run: `CGO_ENABLED=0 go test ./modules/system/... -count=1`
Expected: 全部 `ok`（seed 守卫 + Task 1/2 用例不回归）。

- [ ] **Step 4: 前端 API 封装**

Modify `apps/web-antd/src/api/admin.ts`——`Notice` 接口（:77-83）之后插入：

```ts
export interface NoticeItem extends Notice {
  read_status: number;
}
```

`deleteNotice` 函数（:291-295）之后、`// ---- 日志 ----` 之前插入：

```ts
// ---- 我的通知（已读回执） ----
export function listMyNotices() {
  return requestClient.get<NoticeItem[]>('/admin/system/notices/mine');
}

export function markNoticeRead(id: number) {
  return requestClient.post<{ status: string }>(
    `/admin/system/notices/${id}/read`,
  );
}

export function markAllNoticesRead() {
  return requestClient.post<{ status: string }>(
    '/admin/system/notices/read-all',
  );
}
```

- [ ] **Step 5: 新建「我的通知」页面**

Create `apps/web-antd/src/views/system/SystemMyNoticesPage.vue`（完整文件；骨架抄 `SystemNoticesPage.vue`——localPage 前端切页、VxeGrid、useVbenModal 看详情；差异点：读 `listMyNotices`、标题点击查看详情、未读打开即标记、工具条「全部已读」、**无 v-access**——可见性由菜单权限控制）：

```vue
<script lang="ts" setup>
import type { VxeGridProps } from '#/adapter/vxe-table';
import type { NoticeItem } from '#/api/admin';

import { ref } from 'vue';
import { Page, useVbenModal } from '@vben/common-ui';

import { Button, message } from 'ant-design-vue';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import {
  listMyNotices,
  markAllNoticesRead,
  markNoticeRead,
} from '#/api/admin';
import { formatDateTime } from '#/utils/format';
import { localPage } from '#/utils/local-page';

defineOptions({ name: 'SystemMyNoticesPage' });

const gridOptions: VxeGridProps<NoticeItem> = {
  columns: [
    { field: 'id', title: 'ID', width: 80 },
    { field: 'title', slots: { default: 'title' }, title: '标题', minWidth: 220 },
    {
      field: 'read_status',
      formatter: ({ cellValue }) => (Number(cellValue) === 1 ? '已读' : '未读'),
      title: '状态',
      width: 80,
    },
    {
      field: 'created_at',
      formatter: ({ cellValue }) => formatDateTime(cellValue),
      title: '创建时间',
      width: 180,
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
        const rows = await listMyNotices();
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

const [Modal, modalApi] = useVbenModal({
  fullscreenButton: false,
  footer: false, // 纯查看弹窗，隐藏默认底部（与 codegen 预览同款）
  title: '公告详情',
});

const current = ref<NoticeItem | null>(null);

// 打开详情；未读则顺手标记已读（失败只 toast，弹窗照开——看公告不该被标记失败挡住）
async function openView(row: NoticeItem) {
  current.value = row;
  modalApi.open();
  if (row.read_status !== 1) {
    try {
      await markNoticeRead(row.id);
    } catch {
      return; // 拦截器已 toast
    }
    gridApi.reload();
  }
}

async function onMarkRead(row: NoticeItem) {
  try {
    await markNoticeRead(row.id);
  } catch {
    return; // 拦截器已 toast
  }
  message.success('已标记为已读');
  gridApi.reload();
}

async function onReadAll() {
  try {
    await markAllNoticesRead();
  } catch {
    return; // 拦截器已 toast
  }
  message.success('已全部标记为已读');
  gridApi.reload();
}
</script>

<template>
  <Page auto-content-height>
    <Grid>
      <template #toolbar-tools>
        <Button class="mr-2" type="primary" @click="onReadAll">
          全部已读
        </Button>
      </template>
      <template #title="{ row }">
        <Button size="small" type="link" @click="openView(row)">
          {{ row.title }}
        </Button>
      </template>
      <template #actions="{ row }">
        <Button size="small" type="link" @click="openView(row)">
          查看
        </Button>
        <Button
          v-if="row.read_status !== 1"
          size="small"
          type="link"
          @click="onMarkRead(row)"
        >
          标记已读
        </Button>
      </template>
    </Grid>
    <Modal>
      <div class="mb-2 text-base font-medium">
        {{ current?.title }}
      </div>
      <div class="mb-3 text-xs text-gray-400">
        {{ current ? formatDateTime(current.created_at) : '' }}
      </div>
      <div class="whitespace-pre-wrap">{{ current?.content }}</div>
    </Modal>
  </Page>
</template>
```

说明：component 字符串 `views/system/SystemMyNoticesPage`（不带 `.vue`、相对 `src`）由 `access.ts` 的 glob 自动解析——文件名必须逐字一致，**零路由改动**。

- [ ] **Step 6: 更新 docs/modules.md（8 处精确 diff）**

逐处 Edit（old_string 均已核对现存原文）：

1. §1 模块总览表 system 行端点数：old `| 54 | 全表` → new `| 57 | 全表`（该行完整上下文：`| **system** | ... | `/api/admin/system`、`/api/app/system`、`/api` 根（vben 5 端点） | 54 | 全表`）。
2. §3.2 菜单种子表，`租户管理` 行之后加一行：

   ```
   | menu（seed 回补） | `system:notice:mine` | 我的通知 `/system/notices/mine` |
   ```

3. `### 3.3 后端端点（54 条）` → `### 3.3 后端端点（57 条）`。
4. §3.3 基础数据段，`- **通知公告** `/notices`` 之后追加：

   ```markdown
   **我的通知**（P0-1 已读回执，同一 BasicController）：

   | 方法 | 路径 | handler | 说明 |
   | --- | --- | --- | --- |
   | GET | `/notices/mine` | ListMyNotices | 本租户公告 + 当前用户 `read_status`（0 未读/1 已读） |
   | POST | `/notices/:id/read` | MarkNoticeRead | 标记单条已读（幂等；不存在/跨租户 400） |
   | POST | `/notices/read-all` | ReadAllNotices | 全部标记已读（幂等） |
   ```

5. §2 租户级表清单（`system_notices`、`login_logs` …）：`system_notices`、`login_logs` → `system_notices`、`system_notice_reads`、`login_logs`。
6. §3.5 前端页面表，`/system/notices` 行之后加：

   ```
   | /system/notices/mine | SystemMyNoticesPage.vue | 我的公告列表、详情查看（未读打开自动标记）、单条/全部已读 | 无 v-access（可见性由菜单 `system:notice:mine` 控制） |
   ```

7. §8 菜单树，`租户管理` 行之后加（保持树形前缀）：

   ```
   │  ├─ 我的通知       /system/notices/mine  SystemMyNoticesPage              perm="system:notice:mine"
   ```

8. §9.1 权限前缀表 `system:notice` 行的映射已含 `.../notices/*`（天然覆盖 mine/read/read-all），**不改**；§9.2 v-access 清单新页面无 v-access，**不改**。

- [ ] **Step 7: 前端 typecheck**

Run: `pnpm --filter @vben/web-antd run typecheck`
Expected: exit 0，无类型错误。

- [ ] **Step 8: 全量回归 + 提交**

Run: `CGO_ENABLED=0 go test ./... -count=1`
Expected: 全部包 `ok`（约 38 包；CGO 默认跑会有既有 cgo 链接失败，属本机环境问题）。

```bash
git add modules/system/service/seed.go modules/system/service/seed_test.go apps/web-antd/src/api/admin.ts apps/web-antd/src/views/system/SystemMyNoticesPage.vue docs/modules.md
git commit -m "feat(system): 我的通知页面（种子回补菜单+前端页+手册收口）"
```

- [ ] **Step 9: 收口 roadmap 状态并提交**

Modify `docs/roadmap.md`：

1. 状态总览 P0 行：`| P0 | 基座开箱短板（收尾欠账） | 1/4 ✅（codegen M3 已完成） |` → `| P0 | 基座开箱短板（收尾欠账） | 2/4 ✅（codegen M3、通知已读） |`
2. P0-1 行状态列：`⬜ 当前任务` → `✅ <Step 8 的提交号>`（用 `git log -1 --format=%h` 取 Step 8 提交的实际短号回填；P0-2 行状态由 `⬜` 改 `⬜ 当前任务`）。

```bash
git add docs/roadmap.md
git commit -m "docs(roadmap): P0-1 通知已读完成，P0 进度 2/4"
```

不 push（等用户决定）。

---

## 计划自审记录（写完即查）

- **Spec 覆盖**：roadmap P0-1 三要素——已读回执表（Task 1 迁移）、我的未读/已读接口（Task 1 service + Task 2 三端点）、我的通知页面+菜单种子（Task 3 seed 回补 + 前端页）——逐项有任务落点；无遗漏。
- **占位符扫描**：无 TBD/TODO/「同上」「类似 Task N」；所有代码步骤给全文；文档 diff 给确切 old→new。
- **类型一致性**：`ListMine/MarkRead/MarkAllRead`、`NoticeItem{*SystemNotice, ReadStatus}`、`ErrNoticeNotFound` 在 Task 1 Produces / Task 2 消费与 fake 实现 / Task 3 前端信封三处签名一致；端点路径与 `admin.ts` 函数、菜单 component 与文件名逐字对齐。
- **Review Focus**：五条各钉到具体用例（见各任务 Step 1 与文件头清单）；gin 混挂另有已删探针的预验记录 + Task 2 同形七路由常驻回归。
