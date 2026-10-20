# scorecard 打牌记账模块 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 落地打牌记账后端业务模块 `modules/scorecard`（房间/计分/茶饭/结算/战绩/好友 app 端全量 + admin 只读监管），复用 member 用户体系。

**Architecture:** 脚手架 `make new-module` 原子落定装配触点（main.go/config/权限码/菜单种子），随后手工重写实体层为 6 表模型 + 迁移；核心是单事务支出记账（FOR UPDATE 行锁 + version 乐观锁 + 幂等键 + pending 配置「从下把生效」）；app 端 Bearer 鉴权走 `token.Validator`，admin 端走 casbin 三前缀；无 WS，客户端以快照 + version 对齐。

**Tech Stack:** Go 1.x + gin + fx + GORM(+gormplugin 租户插件) + MySQL(迁移)/sqlite(单测)；前端 vben5 Vue3 + Ant Design Vue + VxeGrid。

**Spec:** `docs/superpowers/specs/2026-10-20-scorecard-module-design.md`（本计划逐条论据来自它；执行者必须先读 spec）

## Global Constraints

- 合规红线：代码/字段/接口命名一律 `score`/`tea`，**禁止** `bet`/`cash`/`rake`/钱/赌注/抽成 等资金类命名与文案（spec 决策红线）。
- 统一信封 `pkg/resp`（`OK/Fail/Page`）；错误码五位分段（1xxxx 参数/2xxxx 鉴权/3xxxx 业务/5xxxx 系统），scorecard 业务码占 **30101–30113**。
- 全部 6 表带 `tenant_id`，模型必须含 `TenantID` 字段让 gormplugin 接管隔离。
- app 端 `/api/app/scorecard/**` 不走 casbin，handler 内 `token.Validator` 鉴权且 `UserType==UserTypeMember`；admin 端 `/api/admin/scorecard/**` 走 casbin。
- 命名与结构跟仓库既有模式：模块布局照 `modules/member`，事务照 `modules/order/service/order.go`，ticker 照 `modules/system/module.go` 的 SweepExpired，sqlite 测试 harness 照 `modules/order/repository/order_repo_test.go`（真注册 gormplugin）。
- 直接在 main 分支工作（仓库约定，不建功能分支）；每个 Task 以 commit 收尾。
- 房间 TTL 常量 `2h`、「无操作」刷 `expire_time`（心跳/刷新不刷）；茶饭总分守恒模型：自动模式输家 −S、赢家 +(S−T)、池 +T；手动 DIY 来源玩家 −T、赢家 +S、池 +T；`T` 不计入 `cum_lose_score`。

## Review Focus

最可能咬人的五类输入/条件（各归任务的测试钉住，见各 Task 的对应步骤）：

1. **同 `client_request_id` 重复提交**（客户端双击/重试）——期望：第二次返回上次结果、分数不重复扣。→ Task 7 幂等测试
2. **并发支出 / 过期 version**（两设备同时提交）——期望：FOR UPDATE 串行 + version 不匹配拒绝，总分守恒。→ Task 5 `TestRoomRepo_SaveVersioned_VersionConflict` + Task 7 `TestExpense_VersionIncrementsSerially`
3. **「从下把生效」边界**（改配置后当前把、下一笔各用什么配置）——期望：当前把旧配置、下一笔新配置。→ Task 7 `TestExpense_PendingAppliesNextRound`
4. **转让房主权限矩阵**（非房主/已结算/目标不在房/拉黑）——期望：全部拒绝且原因可辨；合法转让即时换权。→ Task 8 `TestTransferOwner_Matrix`
5. **跨租户房间访问**（A 租户 token 访问 B 租户房间）——期望：查无（404 语义 → 30101），插件隔离下不可见。→ Task 5 `TestRoom_GetByID_TenantIsolation` + Task 6 `TestSnapshot_CrossTenantDenied`

---

### Task 1: 脚手架打底

**Files:**
- Create: `modules/scorecard/` 全套（脚手架生成：module.go、migration_source.go、migrations/、model/、repository/、service/、controller/{admin,app}/）
- Modify: `alexgo-server/cmd/main.go`、`alexgo-server/configs/config.yaml`、`modules/system/service/permission_routes.go`、system migrations 新菜单 SQL 对
- Create: `apps/web-antd/src/api/scorecard/<6实体>/{api,index,types,__tests__}.ts` 与 `apps/web-antd/src/views/scorecard/<6实体>/{list,form,drawer}.vue`（脚手架 codegen 产物，形态已核 golden_paths.txt；Task 14 整目录删除重写为 `api/scorecard.ts` + 4 页）

**Interfaces:**
- Consumes: `tools/newmodule` CLI（`make new-module`）
- Produces: 目录与装配触点存在、`go build ./...` 绿。后续 Task 在此骨架上手工覆盖。

- [ ] **Step 1: 干跑确认计划**

```bash
make new-module NAME=scorecard ENTITIES=room,room_user,room_log,record,friend,friend_black ARGS="-dry-run"
```

Expected: 打印 ChangeSet（新建 `modules/scorecard/**`、改 main.go/config.yaml/permission_routes.go、新菜单迁移、前端 api+views），无报错。

- [ ] **Step 2: 实跑落盘**

```bash
make new-module NAME=scorecard ENTITIES=room,room_user,room_log,record,friend,friend_black
```

Expected: `✔ 新建模块 scorecard`，验证段显示 `go build ./... 通过`。

- [ ] **Step 3: 全量验证**

```bash
go build ./... && go test ./tools/newmodule/...
```

Expected: 全部通过。

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "chore(scorecard): newmodule 脚手架打底（装配/权限/菜单/前端骨架）"
```

---

### Task 2: scorecard 业务错误码

**Files:**
- Modify: `pkg/errors/code.go`

**Interfaces:**
- Produces: `errs.CodeScorecard*` 常量（后续所有 service/controller 用）。

- [ ] **Step 1: 在 `pkg/errors/code.go` 的 3xxxx 区块末尾（`CodeExportTooManyRows` 之后）加码表与文案**

```go
	// CodeScorecardRoomNotFound scorecard：房间不存在或非本租户。
	CodeScorecardRoomNotFound Code = 30101
	// CodeScorecardNotMember scorecard：非房间成员。
	CodeScorecardNotMember Code = 30102
	// CodeScorecardRoomClosed scorecard：房间已关闭。
	CodeScorecardRoomClosed Code = 30103
	// CodeScorecardRoomSettled scorecard：房间已结算，禁止记分。
	CodeScorecardRoomSettled Code = 30104
	// CodeScorecardNotOwner scorecard：仅房主可操作。
	CodeScorecardNotOwner Code = 30105
	// CodeScorecardRoomTaken scorecard：房号生成冲突重试耗尽。
	CodeScorecardRoomTaken Code = 30106
	// CodeScorecardTransferInvalid scorecard：转让目标无效（不在房/拉黑/同人）。
	CodeScorecardTransferInvalid Code = 30107
	// CodeScorecardBlacklisted scorecard：存在拉黑关系，禁止进房/加好友。
	CodeScorecardBlacklisted Code = 30108
	// CodeScorecardFriendTaken scorecard：好友关系已存在。
	CodeScorecardFriendTaken Code = 30109
	// CodeScorecardRecordNotFound scorecard：战绩不存在。
	CodeScorecardRecordNotFound Code = 30110
	// CodeScorecardNotYourRecord scorecard：仅能操作本人战绩。
	CodeScorecardNotYourRecord Code = 30111
	// CodeScorecardVersionConflict scorecard：并发冲突，请刷新快照重试。
	CodeScorecardVersionConflict Code = 30112
	// CodeScorecardTargetNotMember scorecard：目标不是房间成员。
	CodeScorecardTargetNotMember Code = 30113
```

registry（`Msg()` 文案表）同区追加：

```go
	CodeScorecardRoomNotFound:   "房间不存在",
	CodeScorecardNotMember:      "你不是该房间成员",
	CodeScorecardRoomClosed:     "房间已关闭",
	CodeScorecardRoomSettled:    "房间已结算",
	CodeScorecardNotOwner:       "仅房主可操作",
	CodeScorecardRoomTaken:      "房号生成失败，请重试",
	CodeScorecardTransferInvalid: "转让目标无效",
	CodeScorecardBlacklisted:    "存在拉黑关系，无法操作",
	CodeScorecardFriendTaken:    "好友关系已存在",
	CodeScorecardRecordNotFound: "战绩不存在",
	CodeScorecardNotYourRecord:  "仅能操作本人战绩",
	CodeScorecardVersionConflict: "操作冲突，请下拉刷新",
	CodeScorecardTargetNotMember: "目标不是房间成员",
```

- [ ] **Step 2: 验证**

```bash
go build ./... && go test ./pkg/errors/...
```

Expected: PASS（既有 code_test 若断言 registry 完整性会自动覆盖新码）。

- [ ] **Step 3: Commit**

```bash
git add pkg/errors/code.go && git commit -m "feat(scorecard): 业务错误码 30101-30113"
```

---

### Task 3: member BriefProvider 窄接口

**Files:**
- Modify: `modules/member/repository/member_repo.go`（接口加 `GetBriefs` + 实现）
- Create: `modules/member/service/brief.go`
- Modify: `modules/member/service/member.go`（`memberService` 实现 GetBriefs）
- Modify: `modules/member/module.go`（fx 接口映射）
- Test: `modules/member/service/brief_test.go`

**Interfaces:**
- Produces: `member/service.BriefProvider` — `GetBriefs(ctx context.Context, ids []uint64) ([]UserBrief, error)`；`UserBrief{ID uint64, Nickname string, Avatar string, Status int}`。Task 6+ 注入消费。

- [ ] **Step 1: 写失败测试**

```go
// modules/member/service/brief_test.go
package service

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/member/model"
	"alexGo-cloud/pkg/tenant/gormplugin"
)

func TestGetBriefs(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.MemberUser{}); err != nil {
		t.Fatal(err)
	}
	if err := gormplugin.Register(db, gormplugin.Options{ExemptTables: []string{"tenants", "casbin_rule"}}); err != nil {
		t.Fatal(err)
	}
	db.Create(&model.MemberUser{Nickname: "甲", Avatar: "a1", Status: 1})
	db.Create(&model.MemberUser{Nickname: "乙", Avatar: "b1", Status: 1})

	svc := &memberService{repo: &briefRepoStub{db: db}}
	got, err := svc.GetBriefs(context.Background(), []uint64{1, 2, 99})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 briefs (不存在的 99 跳过), got %d", len(got))
	}
	if got[0].Nickname != "甲" || got[1].Nickname != "乙" {
		t.Fatalf("unexpected: %+v", got)
	}
}
```

注：`briefRepoStub` 包内直接包一层 `repository.MemberRepository` 现实现即可（测试文件内写 3 行 stub，其余方法 panic）。

- [ ] **Step 2: 跑测确认失败**

```bash
go test ./modules/member/service/ -run TestGetBriefs -v
```

Expected: FAIL（`GetBriefs` 未定义）。

- [ ] **Step 3: 实现**

`modules/member/repository/member_repo.go` 的 `MemberRepository` 接口加：

```go
	GetBriefs(ctx context.Context, ids []uint64) ([]*model.MemberUser, error)
```

实现（`memberRepo` 结构上追加；`WHERE id IN ?`，保持租户插件自动过滤）：

```go
func (r *memberRepo) GetBriefs(ctx context.Context, ids []uint64) ([]*model.MemberUser, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var list []*model.MemberUser
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&list).Error
	return list, err
}
```

`modules/member/service/brief.go` 新建：

```go
package service

import "context"

// UserBrief 会员公开摘要：跨模块展示昵称/头像的最小面（不含手机号等敏感列）。
type UserBrief struct {
	ID       uint64 `json:"id"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Status   int    `json:"status"`
}

// BriefProvider member 对外的窄只读接口（scorecard 等业务模块消费；fx 映射见 module.go）。
type BriefProvider interface {
	GetBriefs(ctx context.Context, ids []uint64) ([]UserBrief, error)
}
```

`memberService` 实现（`member.go` 加方法）：

```go
func (s *memberService) GetBriefs(ctx context.Context, ids []uint64) ([]UserBrief, error) {
	rows, err := s.repo.GetBriefs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]UserBrief, 0, len(rows))
	for _, u := range rows {
		out = append(out, UserBrief{ID: u.ID, Nickname: u.Nickname, Avatar: u.Avatar, Status: u.Status})
	}
	return out, nil
}
```

（`model.MemberUser` 已核：`Nickname`/`Avatar`/`Status int` 字段名与上表一致，`member_repo.go` 的 `memberRepo.GetBriefs` 用 `SELECT id, nickname, avatar, status ... WHERE id IN (?) AND deleted = false`。）

`modules/member/module.go` FxModule 加映射：

```go
	fx.Provide(
		func(svc service.MemberService) service.BriefProvider { return svc },
	),
```

- [ ] **Step 4: 跑测通过 + 全量**

```bash
go test ./modules/member/... && go build ./...
```

Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add modules/member && git commit -m "feat(member): BriefProvider 窄只读接口（GetBriefs）"
```

---

### Task 4: 迁移 DDL + 6 表模型

**Files:**
- Modify: `modules/scorecard/migrations/2026xxxx_scorecard_*.up.sql`（脚手架生成的一对示例 SQL 重写为 6 表；文件名保留脚手架名或重命名为 `20261020000001_scorecard_tables.up.sql`，down 同步）
- Create/Modify: `modules/scorecard/model/{room,room_user,room_log,record,friend,friend_black}.go`（覆盖脚手架生成物）
- Test: `modules/scorecard/model/model_test.go`

**Interfaces:**
- Produces: `model.Room / RoomUser / RoomLog / Record / Friend / FriendBlack`（gorm tag 与 DDL 同构）。Task 5+ 的 repo/service 全部依赖这些结构。

- [ ] **Step 1: 写失败测试（模型与迁移列同构由 AutoMigrate + 手动断言钉住）**

```go
// modules/scorecard/model/model_test.go
package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestAutoMigrateAll(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Room{}, &RoomUser{}, &RoomLog{}, &Record{}, &Friend{}, &FriendBlack{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	var cols []string
	if err := db.Raw(`SELECT name FROM pragma_table_info('room')`).Scan(&cols).Error; err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"room_code": true, "tea_pool_total": true, "tea_auto_enabled": true,
		"pending_apply_round_no": true, "version": true, "tenant_id": true}
	for _, c := range cols {
		delete(want, c)
	}
	if len(want) != 0 {
		t.Fatalf("room 缺列: %v（实际列 %v）", want, cols)
	}
}
```

- [ ] **Step 2: 跑测确认失败**

```bash
go test ./modules/scorecard/model/ -v
```

Expected: FAIL（当前模型还是 6 列示例骨架）。

- [ ] **Step 3: 重写模型**

`model/room.go`：

```go
package model

import "time"

// Room 房间。Status: 0 进行中 1 已结算 2 已关闭。pending_* 为「从下把生效」暂存字段。
type Room struct {
	ID                    uint64    `gorm:"primaryKey" json:"id"`
	TenantID              uint64    `json:"tenant_id"`
	RoomCode              string    `gorm:"size:8;index" json:"room_code"`
	CreatorID             uint64    `json:"creator_id"`
	Status                int       `json:"status"`
	ExpireTime            time.Time `json:"expire_time"`
	TeaTargetAmount       int       `json:"tea_target_amount"`
	TeaPercent            float64   `json:"tea_percent"`
	TeaPoolTotal          int       `json:"tea_pool_total"`
	TeaAutoEnabled        int       `json:"tea_auto_enabled"`
	MaxLoseScore          int       `json:"max_lose_score"`
	RoundNo               int       `json:"round_no"`
	PendingTeaTargetAmount int      `json:"pending_tea_target_amount"`
	PendingTeaPercent     float64   `json:"pending_tea_percent"`
	PendingMaxLoseScore   int       `json:"pending_max_lose_score"`
	PendingApplyRoundNo   int       `json:"pending_apply_round_no"`
	Version               int       `json:"version"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}
```

（Room 的 TableName 返回 `room`，若 codegen 命名层默认复数则显式加 `func (Room) TableName() string { return "room" }`——6 个模型同理：`room_user`/`room_log`/`record`/`friend`/`friend_black`。）

`model/room_user.go`：

```go
package model

import "time"

// RoomUser 房间成员。cum_lose_score 本会话累计输分（退房重进经 session_no 重置）。
type RoomUser struct {
	ID           uint64    `gorm:"primaryKey" json:"id"`
	TenantID     uint64    `json:"tenant_id"`
	RoomID       uint64    `gorm:"index:idx_room_user_unique,unique" json:"room_id"`
	UserID       uint64    `gorm:"index:idx_room_user_unique,unique" json:"user_id"`
	CurScore     int       `json:"cur_score"`
	CumLoseScore int       `json:"cum_lose_score"`
	IsCreator    int       `json:"is_creator"`
	SessionNo    int       `json:"session_no"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
```

`model/room_log.go`：

```go
package model

import "time"

// RoomLog 流水。Type: 1 记分 2 茶饭 3 进房 4 退房 5 结算 6 上限提醒 7 配置变更 8 状态变更 9 转让房主。
// ClientRequestID 为支出幂等键（type=1 时必填且房间内唯一）；Extra 存触发累计值等扩展 JSON 文本。
type RoomLog struct {
	ID              uint64    `gorm:"primaryKey" json:"id"`
	TenantID        uint64    `json:"tenant_id"`
	RoomID          uint64    `gorm:"index:idx_room_log_room_created,priority:1" json:"room_id"`
	FromUserID      uint64    `json:"from_user_id"`
	ToUserID        uint64    `json:"to_user_id"`
	Score           int       `json:"score"`
	Tea             int       `json:"tea"`
	Type            int       `json:"type"`
	ClientRequestID string    `gorm:"size:64" json:"client_request_id"`
	Extra           string    `gorm:"type:text" json:"extra"`
	CreatedAt       time.Time `gorm:"index:idx_room_log_room_created,priority:2" json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
```

`model/record.go`：

```go
package model

import "time"

// Record 战绩：每成员一行。SettleType: 1 正常结算 2 房主退房自动结算 3 退房 4 零数据空战绩。
// 房间终局配置冗余快照，统计免 JOIN。
type Record struct {
	ID             uint64    `gorm:"primaryKey" json:"id"`
	TenantID       uint64    `json:"tenant_id"`
	RoomID         uint64    `gorm:"index" json:"room_id"`
	UserID         uint64    `gorm:"index:idx_record_user_created,priority:1" json:"user_id"`
	Score          int       `json:"score"`
	CumLoseScore   int       `json:"cum_lose_score"`
	TeaPoolTotal   int       `json:"tea_pool_total"`
	TeaPercent     float64   `json:"tea_percent"`
	TeaTargetAmount int      `json:"tea_target_amount"`
	MaxLoseScore   int       `json:"max_lose_score"`
	SettleType     int       `json:"settle_type"`
	CreatedAt      time.Time `gorm:"index:idx_record_user_created,priority:2" json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
```

`model/friend.go`：

```go
package model

import "time"

// Friend 好友关系（单向关系行，双向各存一行由 service 保证）；RankEnabled 0/1 参与排名。
type Friend struct {
	ID          uint64    `gorm:"primaryKey" json:"id"`
	TenantID    uint64    `json:"tenant_id"`
	UserID      uint64    `gorm:"index:idx_friend_unique,unique" json:"user_id"`
	FriendUserID uint64   `gorm:"index:idx_friend_unique,unique" json:"friend_user_id"`
	RankEnabled int       `json:"rank_enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
```

`model/friend_black.go`：

```go
package model

import "time"

// FriendBlack 拉黑：UserID 拉黑 BlockedUserID（单向行；判定时查双向）。
type FriendBlack struct {
	ID            uint64    `gorm:"primaryKey" json:"id"`
	TenantID      uint64    `json:"tenant_id"`
	UserID        uint64    `gorm:"index:idx_friend_black_unique,unique" json:"user_id"`
	BlockedUserID uint64    `gorm:"index:idx_friend_black_unique,unique" json:"blocked_user_id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
```

- [ ] **Step 4: 重写迁移 SQL**

`modules/scorecard/migrations/<保持脚手架文件名>.up.sql` 全量替换为（幂等，列与模型严格同构；`room_code` 唯一索引、`room_user` 复合唯一、`room_log(room_id,created_at)`、`record(user_id,created_at)`、`record(room_id)`、friend/friend_black 复合唯一）：

```sql
-- scorecard：打牌记账 6 表（列与 modules/scorecard/model 严格同构）
CREATE TABLE IF NOT EXISTS `room` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '租户ID',
  `room_code` varchar(8) NOT NULL DEFAULT '' COMMENT '房号4位字母数字',
  `creator_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '当前房主用户ID',
  `status` tinyint NOT NULL DEFAULT 0 COMMENT '状态 0进行中 1已结算 2已关闭',
  `expire_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '超时关闭时间',
  `tea_target_amount` int NOT NULL DEFAULT 0 COMMENT '茶饭目标金额 0=不限',
  `tea_percent` decimal(5,2) NOT NULL DEFAULT 0 COMMENT '茶饭百分比生效值',
  `tea_pool_total` int NOT NULL DEFAULT 0 COMMENT '茶饭公共池累计',
  `tea_auto_enabled` tinyint NOT NULL DEFAULT 0 COMMENT '自动抽取开关 达标自动置0',
  `max_lose_score` int NOT NULL DEFAULT 0 COMMENT '输分上限 0=不限',
  `round_no` int NOT NULL DEFAULT 0 COMMENT '当前把序号',
  `pending_tea_target_amount` int NOT NULL DEFAULT 0 COMMENT '待生效目标金额',
  `pending_tea_percent` decimal(5,2) NOT NULL DEFAULT 0 COMMENT '待生效百分比',
  `pending_max_lose_score` int NOT NULL DEFAULT 0 COMMENT '待生效上限',
  `pending_apply_round_no` int NOT NULL DEFAULT 0 COMMENT '待生效应用把序号 0=无',
  `version` int NOT NULL DEFAULT 0 COMMENT '乐观锁版本',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_room_code` (`room_code`),
  KEY `idx_room_tenant` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='打牌记账房间';

CREATE TABLE IF NOT EXISTS `room_user` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '租户ID',
  `room_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '房间ID',
  `user_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '成员用户ID',
  `cur_score` int NOT NULL DEFAULT 0 COMMENT '当前分数',
  `cum_lose_score` int NOT NULL DEFAULT 0 COMMENT '本会话累计输分',
  `is_creator` tinyint NOT NULL DEFAULT 0 COMMENT '是否房主 1=是',
  `session_no` int NOT NULL DEFAULT 1 COMMENT '进房会话序号 重进+1',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_room_user` (`room_id`,`user_id`),
  KEY `idx_room_user_tenant` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='房间成员';

CREATE TABLE IF NOT EXISTS `room_log` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '租户ID',
  `room_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '房间ID',
  `from_user_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '来源用户ID',
  `to_user_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '目标用户ID',
  `score` int NOT NULL DEFAULT 0 COMMENT '分数',
  `tea` int NOT NULL DEFAULT 0 COMMENT '茶饭',
  `type` tinyint NOT NULL DEFAULT 1 COMMENT '类型 1记分2茶房3进房4退房5结算6上限7配置8状态9转让',
  `client_request_id` varchar(64) NOT NULL DEFAULT '' COMMENT '幂等键',
  `extra` text COMMENT '扩展JSON',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_room_log_room_created` (`room_id`,`created_at`),
  KEY `idx_room_log_tenant` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='房间流水';

CREATE TABLE IF NOT EXISTS `record` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '租户ID',
  `room_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '房间ID',
  `user_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '归属用户ID',
  `score` int NOT NULL DEFAULT 0 COMMENT '终局分数',
  `cum_lose_score` int NOT NULL DEFAULT 0 COMMENT '终局累计输分',
  `tea_pool_total` int NOT NULL DEFAULT 0 COMMENT '终局茶饭池快照',
  `tea_percent` decimal(5,2) NOT NULL DEFAULT 0 COMMENT '终局百分比快照',
  `tea_target_amount` int NOT NULL DEFAULT 0 COMMENT '终局目标金额快照',
  `max_lose_score` int NOT NULL DEFAULT 0 COMMENT '终局上限快照',
  `settle_type` tinyint NOT NULL DEFAULT 1 COMMENT '1正常结算2房主退房自动3退房4零数据',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '结算时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_record_user_created` (`user_id`,`created_at`),
  KEY `idx_record_room` (`room_id`),
  KEY `idx_record_tenant` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='战绩每成员一行';

CREATE TABLE IF NOT EXISTS `friend` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '租户ID',
  `user_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '用户ID',
  `friend_user_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '好友用户ID',
  `rank_enabled` tinyint NOT NULL DEFAULT 1 COMMENT '参与排名 1=是',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_friend` (`user_id`,`friend_user_id`),
  KEY `idx_friend_tenant` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='好友关系';

CREATE TABLE IF NOT EXISTS `friend_black` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '租户ID',
  `user_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '拉黑人用户ID',
  `blocked_user_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '被拉黑用户ID',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_friend_black` (`user_id`,`blocked_user_id`),
  KEY `idx_friend_black_tenant` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='拉黑关系';
```

down.sql 全量替换为对应 6 条 `DROP TABLE IF EXISTS ...`。

- [ ] **Step 5: 跑测通过**

```bash
go test ./modules/scorecard/... && go build ./...
```

Expected: PASS。

- [ ] **Step 6: Commit**

```bash
git add modules/scorecard && git commit -m "feat(scorecard): 6 表模型与迁移 DDL（tenant_id 全带/唯一键/索引）"
```

---

### Task 5: repository 层

**Files:**
- Modify: `modules/scorecard/repository/`（覆盖脚手架生成的每实体 repo：`room.go`、`room_user.go`、`room_log.go`、`record.go`、`friend.go`、`friend_black.go`）
- Test: `modules/scorecard/repository/room_repo_test.go`

**Interfaces:**
- Produces（Task 6–11 消费的精确签名）:

```go
type RoomRepository interface {
	Create(ctx context.Context, r *model.Room) error
	GetByID(ctx context.Context, id uint64) (*model.Room, error)               // 软查（快照/校验用）
	GetForUpdate(ctx context.Context, tx *gorm.DB, id uint64) (*model.Room, error) // SELECT FOR UPDATE
	GetByCode(ctx context.Context, code string) (*model.Room, error)
	SaveVersioned(ctx context.Context, tx *gorm.DB, r *model.Room) error // UPDATE 全字段 WHERE id AND version；RowsAffected=0 → errs.CodeScorecardVersionConflict
	ListAdmin(ctx context.Context, status int, page, size int) ([]*model.Room, int64, error) // status<0 = 不过滤
	SweepExpired(ctx context.Context, now time.Time) ([]*model.Room, error)
}
type RoomUserRepository interface {
	Create(ctx context.Context, u *model.RoomUser) error
	Get(ctx context.Context, roomID, userID uint64) (*model.RoomUser, error)
	ListByRoom(ctx context.Context, roomID uint64) ([]*model.RoomUser, error)
	CountByRoom(ctx context.Context, roomID uint64) (int64, error)
	Delete(ctx context.Context, roomID, userID uint64) error
	AddScore(ctx context.Context, tx *gorm.DB, roomID, userID uint64, delta int) error
	AddCumLose(ctx context.Context, tx *gorm.DB, roomID, userID uint64, delta int) error // 同 AddScore，列换 cum_lose_score
	SetCreator(ctx context.Context, tx *gorm.DB, roomID, userID uint64, isCreator int) error
}
type RoomLogRepository interface {
	Create(ctx context.Context, l *model.RoomLog) error
	GetByRequestID(ctx context.Context, roomID uint64, reqID string) (*model.RoomLog, error)
	ListByRoom(ctx context.Context, roomID uint64, page, size int) ([]*model.RoomLog, int64, error)
	ListTypeByUser(ctx context.Context, roomID, userID, typ, limit int) ([]*model.RoomLog, error) // 上限频控：取该用户近期该类型日志，service 解码 extra.session_no 判本会话首触发
}
type RecordRepository interface {
	Create(ctx context.Context, recs []*model.Record) error // 批量（结算全员）
	ListByUser(ctx context.Context, userID uint64, page, size int) ([]*model.Record, int64, error)
	GetByID(ctx context.Context, id uint64) (*model.Record, error)
	Delete(ctx context.Context, id uint64) error
	ListByRoom(ctx context.Context, roomID uint64) ([]*model.Record, error)
	StatsByUser(ctx context.Context, userID uint64) (total int64, win int64, lose int64, draw int64, err error)
	ListAdmin(ctx context.Context, userID, roomID uint64, page, size int) ([]*model.Record, int64, error)
}
type FriendRepository interface {
	Create(ctx context.Context, f *model.Friend) error
	Get(ctx context.Context, userID, friendID uint64) (*model.Friend, error)
	ListByUser(ctx context.Context, userID uint64) ([]*model.Friend, error)
	SetRank(ctx context.Context, userID, friendID uint64, enabled int) error
	Delete(ctx context.Context, userID, friendID uint64) error
	ListAdmin(ctx context.Context, page, size int) ([]*model.Friend, int64, error)
}
type FriendBlackRepository interface {
	Create(ctx context.Context, b *model.FriendBlack) error
	Exists(ctx context.Context, a, b uint64) (bool, error) // 双向判定 (a黑b) 或 (b黑a)
	Delete(ctx context.Context, userID, blockedID uint64) error
	Get(ctx context.Context, userID, blockedID uint64) (*model.FriendBlack, error)
	ListAdmin(ctx context.Context, page, size int) ([]*model.FriendBlack, int64, error)
}
```

实现要点（全部 `r.db.WithContext(ctx)`；`GetForUpdate` 用 `clause.Locking{Strength: "UPDATE"}`，sqlite 单测中该子句被驱动忽略不影响）：

```go
func (r *roomRepo) GetForUpdate(ctx context.Context, tx *gorm.DB, id uint64) (*model.Room, error) {
	var out model.Room
	err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", id).First(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &out, err
}
```

`AddScore` 用表达式列更新：

```go
func (r *roomUserRepo) AddScore(ctx context.Context, tx *gorm.DB, roomID, userID uint64, delta int) error {
	return tx.WithContext(ctx).Model(&model.RoomUser{}).
		Where("room_id = ? AND user_id = ?", roomID, userID).
		UpdateColumn("cur_score", gorm.Expr("cur_score + ?", delta)).Error
}
```

`StatsByUser`：

```go
func (r *recordRepo) StatsByUser(ctx context.Context, userID uint64) (total, win, lose, draw int64, err error) {
	err = r.db.WithContext(ctx).Model(&model.Record{}).Where("user_id = ?", userID).
		Select("COALESCE(SUM(score),0) AS total,"+
			"COALESCE(SUM(CASE WHEN score>0 THEN 1 ELSE 0 END),0) AS win,"+
			"COALESCE(SUM(CASE WHEN score<0 THEN 1 ELSE 0 END),0) AS lose,"+
			"COALESCE(SUM(CASE WHEN score=0 THEN 1 ELSE 0 END),0) AS draw").
		Row().Scan(&total, &win, &lose, &draw)
	return
}
```

（其余方法照 `modules/order/repository/order_repo.go` 的直白 CRUD 写法；分页用 `Limit/Offset` + `Count`。）

- [ ] **Step 1: 写失败测试（租户隔离 + 关键查询，照 order_repo_test.go 真注册插件）**

```go
// modules/scorecard/repository/room_repo_test.go
package repository

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/scorecard/model"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/tenant/gormplugin"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Room{}, &model.RoomUser{}, &model.RoomLog{}, &model.Record{}, &model.Friend{}, &model.FriendBlack{}); err != nil {
		t.Fatal(err)
	}
	if err := gormplugin.Register(db, gormplugin.Options{ExemptTables: []string{"tenants", "casbin_rule"}}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestRoom_GetByID_TenantIsolation(t *testing.T) {
	db := newTestDB(t)
	repo := NewRoomRepository(db)
	ctx1 := tenant.WithTenantID(context.Background(), 1)
	ctx2 := tenant.WithTenantID(context.Background(), 2)
	r := &model.Room{RoomCode: "AB12", TenantID: 1, Status: 0}
	if err := db.WithContext(ctx1).Create(r).Error; err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByID(ctx2, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("跨租户应不可见，got %+v", got)
	}
}

func TestRoomRepo_SaveVersioned_VersionConflict(t *testing.T) {
	db := newTestDB(t)
	repo := NewRoomRepository(db)
	ctx := tenant.WithTenantID(context.Background(), 1)
	r := &model.Room{RoomCode: "XY99", TenantID: 1, Status: 0}
	if err := db.WithContext(ctx).Create(r).Error; err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByID(ctx, r.ID) // 持有过期快照（version=0）
	if err != nil || got == nil {
		t.Fatalf("get: %v", err)
	}
	// 模拟并发者已提交：库内 version 推进到 3
	if err := db.WithContext(ctx).Model(&model.Room{}).Where("id = ?", r.ID).
		UpdateColumn("version", 3).Error; err != nil {
		t.Fatal(err)
	}
	got.RoomCode = "XY98"
	err = repo.SaveVersioned(ctx, db, got) // 携带 version=0 的 WHERE 命中 0 行
	if !errs.Is(err, errs.CodeScorecardVersionConflict) {
		t.Fatalf("过期 version 应 30112, got %v", err)
	}
}
```

- [ ] **Step 2: 跑测确认失败**

```bash
go test ./modules/scorecard/repository/ -run TestRoom_GetByID_TenantIsolation -v
```

Expected: FAIL（repo 还是脚手架 CRUD）。

- [ ] **Step 3: 按接口块实现 6 个 repo（覆盖生成物）**

- [ ] **Step 4: 跑测通过 + 全量**

```bash
go test ./modules/scorecard/... && go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add modules/scorecard && git commit -m "feat(scorecard): repository 层（FOR UPDATE/聚合统计/租户隔离）"
```

---

### Task 6: RoomService 基础（create / join / snapshot）

**Files:**
- Modify: `modules/scorecard/service/room.go`（覆盖脚手架生成物）
- Create: `modules/scorecard/service/snapshot.go`（DTO）
- Test: `modules/scorecard/service/room_test.go`

**Interfaces:**
- Consumes: Task 3 `BriefProvider`、Task 5 全部 repo、Task 4 model。
- Produces（Task 7–13 依赖）:

```go
type RoomMember struct {
	UserID      uint64 `json:"user_id"`
	Nickname    string `json:"nickname"`
	Avatar      string `json:"avatar"`
	CurScore    int    `json:"cur_score"`
	CumLoseScore int   `json:"cum_lose_score"`
	IsCreator   bool   `json:"is_creator"`
}
type RoomLogItem struct {
	ID              uint64    `json:"id"`
	FromUserID      uint64    `json:"from_user_id"`
	ToUserID        uint64    `json:"to_user_id"`
	Score           int       `json:"score"`
	Tea             int       `json:"tea"`
	Type            int       `json:"type"`
	ClientRequestID string    `json:"client_request_id"`
	CreatedAt       time.Time `json:"created_at"`
}
type RoomSnapshot struct {
	ID               uint64        `json:"id"`
	RoomCode         string        `json:"room_code"`
	CreatorID        uint64        `json:"creator_id"`
	Status           int           `json:"status"`
	Version          int           `json:"version"`
	ExpireTime       time.Time     `json:"expire_time"`
	TeaTargetAmount  int           `json:"tea_target_amount"`
	TeaPercent       float64       `json:"tea_percent"`
	TeaPoolTotal     int           `json:"tea_pool_total"`
	TeaAutoEnabled   bool          `json:"tea_auto_enabled"`
	MaxLoseScore     int           `json:"max_lose_score"`
	RoundNo          int           `json:"round_no"`
	PendingApplyRoundNo int        `json:"pending_apply_round_no"`
	Members          []RoomMember  `json:"members"`
	RecentLogs       []RoomLogItem `json:"recent_logs"` // 最近 20 条流水（spec §4.1 快照含最近日志）
}
type CreateRoomReq struct {
	RoomCode         string  `json:"room_code"` // 可空=服务端生成
	TeaTargetAmount  int     `json:"tea_target_amount"`
	TeaPercent       float64 `json:"tea_percent"`
	TeaAutoEnabled   bool    `json:"tea_auto_enabled"`
	MaxLoseScore     int     `json:"max_lose_score"`
}
type RoomService interface {
	Create(ctx context.Context, userID uint64, req CreateRoomReq) (*RoomSnapshot, error)
	Join(ctx context.Context, userID uint64, roomCode string) (*RoomSnapshot, error)
	Snapshot(ctx context.Context, userID, roomID uint64) (*RoomSnapshot, error)
	Logs(ctx context.Context, userID, roomID uint64, page, size int) ([]*model.RoomLog, int64, error)
}
type roomService struct {
	db      *gorm.DB
	room    repository.RoomRepository
	member  repository.RoomUserRepository
	log     repository.RoomLogRepository
	briefs  membersvc.BriefProvider // import "alexGo-cloud/modules/member/service" 别名 membersvc
	black   repository.FriendBlackRepository
}
func NewRoomService(db *gorm.DB, room repository.RoomRepository, member repository.RoomUserRepository,
	log repository.RoomLogRepository, briefs service.BriefProvider, black repository.FriendBlackRepository) RoomService
```

（`service.BriefProvider` 的 import 路径 `alexGo-cloud/modules/member/service`，包名冲突时本地别名 `membersvc`。）

- [ ] **Step 1: 写失败测试**

```go
// modules/scorecard/service/room_test.go
package service

import (
	"context"
	"testing"

	"alexGo-cloud/modules/scorecard/model"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/tenant"
)

func TestCreate_GenerateCodeAndSnapshot(t *testing.T) {
	svc, _ := newRoomServiceForTest(t)
	ctx := tenant.WithTenantID(context.Background(), 1)
	snap, err := svc.Create(ctx, 100, CreateRoomReq{TeaPercent: 10, TeaAutoEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.RoomCode) != 4 {
		t.Fatalf("房号应 4 位, got %q", snap.RoomCode)
	}
	if snap.CreatorID != 100 || snap.Version != 1 || len(snap.Members) != 1 || !snap.Members[0].IsCreator {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}
}

func TestCreate_CustomCodeConflictRetry(t *testing.T) {
	svc, _ := newRoomServiceForTest(t)
	ctx := tenant.WithTenantID(context.Background(), 1)
	if _, err := svc.Create(ctx, 100, CreateRoomReq{RoomCode: "AB12"}); err != nil {
		t.Fatal(err)
	}
	// 同租户重复自定义房号：生成冲突耗尽 → 30106
	_, err := svc.Create(ctx, 101, CreateRoomReq{RoomCode: "AB12"})
	if !errs.Is(err, errs.CodeScorecardRoomTaken) {
		t.Fatalf("want 30106, got %v", err)
	}
}

func TestJoin_IdempotentAndSessionReset(t *testing.T) {
	svc, db := newRoomServiceForTest(t)
	ctx := tenant.WithTenantID(context.Background(), 1)
	snap, _ := svc.Create(ctx, 100, CreateRoomReq{})
	if _, err := svc.Join(ctx, 200, snap.RoomCode); err != nil {
		t.Fatal(err)
	}
	// 幂等：已在房，返回现状不重复加人
	snap2, err := svc.Join(ctx, 200, snap.RoomCode)
	if err != nil || len(snap2.Members) != 2 {
		t.Fatalf("join idempotent fail: %v %+v", err, snap2)
	}
	// 造数模拟退房后重进（Leave 在 Task 8 实现，此处直接删成员行）：
	// 重进应重新入房且累计清零
	if err := db.WithContext(ctx).Where("room_id = ? AND user_id = ?", snap.ID, uint64(200)).
		Delete(&model.RoomUser{}).Error; err != nil {
		t.Fatal(err)
	}
	snap3, err := svc.Join(ctx, 200, snap.RoomCode)
	if err != nil || len(snap3.Members) != 2 {
		t.Fatalf("重进失败: %v %+v", err, snap3)
	}
	for _, m := range snap3.Members {
		if m.UserID == 200 && (m.CurScore != 0 || m.CumLoseScore != 0) {
			t.Fatalf("重进应清零: %+v", m)
		}
	}
}

func TestSnapshot_CrossTenantDenied(t *testing.T) {
	svc, _ := newRoomServiceForTest(t)
	ctx1 := tenant.WithTenantID(context.Background(), 1)
	ctx2 := tenant.WithTenantID(context.Background(), 2)
	snap, _ := svc.Create(ctx1, 100, CreateRoomReq{})
	_, err := svc.Snapshot(ctx2, 100, snap.ID)
	if !errs.Is(err, errs.CodeScorecardRoomNotFound) {
		t.Fatalf("跨租户应 30101, got %v", err)
	}
}
```

`testhelper_test.go`（同包，Task 6 建立后 Task 7–11 复用；sqlite 内存库 + AutoMigrate 6 模型 + 真注册 gormplugin + 构造 6 repo + fake `BriefProvider`）：

```go
// testhelper_test.go
package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/member/service"
	"alexGo-cloud/modules/scorecard/model"
	"alexGo-cloud/modules/scorecard/repository"
	"alexGo-cloud/pkg/tenant/gormplugin"
)

type fakeBriefs struct{}

func (fakeBriefs) GetBriefs(ctx context.Context, ids []uint64) ([]service.UserBrief, error) {
	out := make([]service.UserBrief, 0, len(ids))
	for _, id := range ids {
		out = append(out, service.UserBrief{ID: id, Nickname: fmt.Sprintf("u%d", id), Status: 1})
	}
	return out, nil
}

func newRoomServiceForTest(t *testing.T) (RoomService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Room{}, &model.RoomUser{}, &model.RoomLog{},
		&model.Record{}, &model.Friend{}, &model.FriendBlack{}); err != nil {
		t.Fatal(err)
	}
	if err := gormplugin.Register(db, gormplugin.Options{ExemptTables: []string{"tenants", "casbin_rule"}}); err != nil {
		t.Fatal(err)
	}
	roomRepo := repository.NewRoomRepository(db)
	svc := NewRoomService(db, roomRepo, repository.NewRoomUserRepository(db),
		repository.NewRoomLogRepository(db), fakeBriefs{}, repository.NewFriendBlackRepository(db))
	return svc, db
}

// memberScoreMap 快照成员分 → user_id → cur_score。
func memberScoreMap(t *testing.T, svc RoomService, ctx context.Context, roomID uint64) map[uint64]int {
	t.Helper()
	snap, err := svc.Snapshot(ctx, snapCreatorID(t, svc, ctx, roomID), roomID)
	if err != nil {
		t.Fatal(err)
	}
	m := make(map[uint64]int, len(snap.Members))
	for _, u := range snap.Members {
		m[u.UserID] = u.CurScore
	}
	return m
}

func snapCreatorID(t *testing.T, svc RoomService, ctx context.Context, roomID uint64) uint64 {
	t.Helper()
	room, err := svc.(*roomService).room.GetByID(ctx, roomID)
	if err != nil || room == nil {
		t.Fatalf("room: %v", err)
	}
	return room.CreatorID
}

// listRecordsOfRoom 读该房全部 record 行（测试白盒，直接查库）。
func listRecordsOfRoom(t *testing.T, svc RoomService, ctx context.Context, roomID uint64) []*model.Record {
	t.Helper()
	var rows []*model.Record
	if err := svc.(*roomService).db.WithContext(ctx).Where("room_id = ?", roomID).Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	return rows
}

// mustCode 取房间当前房号。
func mustCode(t *testing.T, svc RoomService, ctx context.Context, roomID uint64) string {
	t.Helper()
	room, err := svc.(*roomService).room.GetByID(ctx, roomID)
	if err != nil || room == nil {
		t.Fatalf("room: %v", err)
	}
	return room.RoomCode
}

func ptr[T any](v T) *T { return &v }
```

（`Snapshot` 需要 viewer 在房内——`memberScoreMap` 用 `room.creator_id` 作为 viewer 保证非成员测试场景也成立；setupRoom 里 creator=100。）

- [ ] **Step 2: 跑测确认失败**

```bash
go test ./modules/scorecard/service/ -v
```

- [ ] **Step 3: 实现**

要点：

- 房号生成：`crypto/rand` 从 `ABCDEFGHJKLMNPQRSTUVWXYZ23456789` 取 4 位；冲突 `Create` 报唯一键错则重试 5 次 → `errs.New(errs.CodeScorecardRoomTaken, nil)`。自定义房号冲突不重试直接 30106。
- `Create`：`status=0`、`tea_auto_enabled` 按 req、`expire_time=now+2h`、`version=1`；随后写 `room_user(is_creator=1, session_no=1)`、`room_log(type=3 进房, from=userID)`；返回 Snapshot。
- `Join`：`GetByCode` → 不存在 30101；`status!=0` → 30103/30104；拉黑双向判定（`black.Exists` 对每个现有成员或对 creator 批量——实现取对 creator + 单查 `WHERE (user_id=joiner AND blocked_user_id IN members) OR (user_id IN members AND blocked_user_id=joiner)` 的 Exists 等价查询，repo 已有 `Exists(a,b)` 时对全成员循环 ≤20 次可接受）→ 30108；已在房 → 直接返回现状快照；不在房 → 建行（`session_no=1`）+ log(type=3) + 刷 expire_time。
- `Snapshot`：`GetByID` 空 → 30101；成员校验（非成员 30102）；组 DTO：成员行经 `briefs.GetBriefs` 批量补昵称头像（失败降级空昵称不炸接口），`RecentLogs` 取 `log.ListByRoom(ctx, roomID, 1, 20)` 最近 20 条（created_at 倒序）。
- `Logs`：`GetByID` 空 → 30101；成员校验（非成员 30102）；`log.ListByRoom(ctx, roomID, page, size)` 透传（Task 12 app 控制器消费）。
- `buildSnapshot(ctx, tx, roomID, userID)` 为事务内共用的快照组装（Task 7 expense 返回也走它），签名 `func (s *roomService) buildSnapshot(ctx context.Context, tx *gorm.DB, roomID, viewerID uint64) (*RoomSnapshot, error)`。

- [ ] **Step 4: 跑测通过**

```bash
go test ./modules/scorecard/service/ -v && go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add modules/scorecard && git commit -m "feat(scorecard): 房间创建/加入/快照（房号生成/幂等进房/拉黑拦截/租户隔离）"
```

---

### Task 7: 支出事务 + 配置 pending（核心）

**Files:**
- Modify: `modules/scorecard/service/expense.go`（新建，挂同一 `roomService`）
- Modify: `modules/scorecard/service/room.go`（加 `UpdateConfig`）
- Test: `modules/scorecard/service/expense_test.go`

**Interfaces:**
- Consumes: Task 6 `roomService` 结构与 repo。
- Produces:

```go
type ExpenseReq struct {
	Score          int    `json:"score"`
	Tea            int    `json:"tea"`
	ToUserID       uint64 `json:"to_user_id"`
	TeaFromUserID  uint64 `json:"tea_from_user_id"` // 手动 DIY 来源，缺省=支出方
	ClientRequestID string `json:"client_request_id"`
}
type ExpenseResult struct {
	Snapshot      *RoomSnapshot `json:"snapshot"`
	LogID         uint64        `json:"log_id"`
	TeaCharged    int           `json:"tea_charged"`
	AlarmedUserIDs []uint64     `json:"alarmed_user_ids"` // 首次达上限（全房提醒）的成员
}
type ConfigReq struct {
	TeaTargetAmount *int     `json:"tea_target_amount"`
	TeaPercent      *float64 `json:"tea_percent"`
	TeaAutoEnabled  *bool    `json:"tea_auto_enabled"`
	MaxLoseScore    *int     `json:"max_lose_score"`
}
// RoomService 接口追加：
//	Expense(ctx context.Context, userID, roomID uint64, req ExpenseReq) (*ExpenseResult, error)
//	UpdateConfig(ctx context.Context, userID, roomID uint64, req ConfigReq) (*RoomSnapshot, error)
```

- [ ] **Step 1: 写失败测试（Review Focus 1/2/3 + 守恒/达标/频控全落这里）**

```go
// modules/scorecard/service/expense_test.go
package service

import (
	"context"
	"testing"

	"alexGo-cloud/modules/scorecard/model"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/tenant"
)

func setupRoom(t *testing.T, teaPercent float64, auto bool, maxLose, target int) (svc RoomService, ctx context.Context, roomID uint64, loser, winner uint64) {
	t.Helper()
	svc, _ = newRoomServiceForTest(t)
	ctx = tenant.WithTenantID(context.Background(), 1)
	snap, err := svc.Create(ctx, 100, CreateRoomReq{TeaPercent: teaPercent, TeaAutoEnabled: auto, MaxLoseScore: maxLose, TeaTargetAmount: target})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Join(ctx, 200, snap.RoomCode); err != nil {
		t.Fatal(err)
	}
	return svc, ctx, snap.ID, 100, 200
}

func TestExpense_ConservationWithTea(t *testing.T) {
	svc, ctx, roomID, loser, winner := setupRoom(t, 10, true, 0, 0) // 10% 茶饭
	res, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 100, ToUserID: winner, ClientRequestID: "r1"})
	if err != nil {
		t.Fatal(err)
	}
	if res.TeaCharged != 10 {
		t.Fatalf("tea = round(100*10%%) = 10, got %d", res.TeaCharged)
	}
	m := memberScoreMap(t, svc, ctx, roomID)
	if m[loser] != -100 || m[winner] != 90 {
		t.Fatalf("守恒破坏: %v", m)
	}
	if res.Snapshot.TeaPoolTotal != 10 {
		t.Fatalf("pool: %+v", res.Snapshot)
	}
}

func TestExpense_IdempotentReplay(t *testing.T) {
	svc, ctx, roomID, loser, winner := setupRoom(t, 0, false, 0, 0)
	r1, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 30, ToUserID: winner, ClientRequestID: "dup"})
	if err != nil {
		t.Fatal(err)
	}
	r2, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 30, ToUserID: winner, ClientRequestID: "dup"})
	if err != nil {
		t.Fatal(err)
	}
	if r2.LogID != r1.LogID {
		t.Fatalf("幂等重放应同 log: %d vs %d", r1.LogID, r2.LogID)
	}
	m := memberScoreMap(t, svc, ctx, roomID)
	if m[loser] != -30 || m[winner] != 30 {
		t.Fatalf("重放重复扣分: %v", m)
	}
}

func TestExpense_VersionIncrementsSerially(t *testing.T) {
	svc, ctx, roomID, loser, winner := setupRoom(t, 0, false, 0, 0)
	r1, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 10, ToUserID: winner, ClientRequestID: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	r2, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 10, ToUserID: winner, ClientRequestID: "v2"})
	if err != nil {
		t.Fatal(err)
	}
	if r2.Snapshot.Version != r1.Snapshot.Version+1 {
		t.Fatalf("version 应单调 +1: %d → %d", r1.Snapshot.Version, r2.Snapshot.Version)
	}
	// 走到 SaveVersioned 被拒的 30112 路径在 repo 单测构造
	//（TestRoomRepo_SaveVersioned_VersionConflict，Task 5）：sqlite 无法模拟
	// 真并发，FOR UPDATE 串行化在 MySQL 集成环境生效，version 是第二道防线。
}

func TestExpense_PendingAppliesNextRound(t *testing.T) {
	svc, ctx, roomID, loser, winner := setupRoom(t, 10, true, 0, 0)
	if _, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 100, ToUserID: winner, ClientRequestID: "old"}); err != nil {
		t.Fatal(err)
	}
	// 改配置 20%：当前把之后的下一笔才生效
	if _, err := svc.UpdateConfig(ctx, 100, roomID, ConfigReq{TeaPercent: ptr(20.0)}); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 100, ToUserID: winner, ClientRequestID: "new"})
	if err != nil {
		t.Fatal(err)
	}
	if res.TeaCharged != 20 {
		t.Fatalf("从下把生效失败: tea=%d", res.TeaCharged)
	}
}

func TestExpense_TargetReachedAutoOff(t *testing.T) {
	svc, ctx, roomID, loser, winner := setupRoom(t, 50, true, 0, 30) // 目标 30
	res, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 100, ToUserID: winner, ClientRequestID: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if res.TeaCharged != 50 || res.Snapshot.TeaPoolTotal != 50 {
		t.Fatalf("%+v", res)
	}
	if res.Snapshot.TeaAutoEnabled {
		t.Fatal("达标应自动关闭自动抽取")
	}
}

func TestExpense_MaxLoseFrequencyControl(t *testing.T) {
	svc, ctx, roomID, loser, winner := setupRoom(t, 0, false, 50, 0)
	res1, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 50, ToUserID: winner, ClientRequestID: "m1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res1.AlarmedUserIDs) != 1 || res1.AlarmedUserIDs[0] != loser {
		t.Fatalf("首次达上限应全房提醒: %+v", res1)
	}
	res2, err := svc.Expense(ctx, winner, roomID, ExpenseReq{Score: 30, ToUserID: loser, ClientRequestID: "m2"})
	if err != nil {
		t.Fatal(err)
	}
	// loser 此时是赢家不涨累计输分；再让 loser 输一笔超限 → 仅日志不进 AlarmedUserIDs
	res3, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 30, ToUserID: winner, ClientRequestID: "m3"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res3.AlarmedUserIDs) != 0 {
		t.Fatalf("二次超限不应重复全房提醒: %+v", res3)
	}
}

func TestExpense_DiyTeaFromSource(t *testing.T) {
	svc, ctx, roomID, loser, winner := setupRoom(t, 0, false, 0, 0) // 手动模式
	res, err := svc.Expense(ctx, loser, roomID, ExpenseReq{
		Score: 100, Tea: 20, TeaFromUserID: winner, ToUserID: winner, ClientRequestID: "d1",
	})
	if err != nil {
		t.Fatal(err)
	}
	m := memberScoreMap(t, svc, ctx, roomID)
	// DIY：输家 -100，赢家 +100-20（来源=赢家），守恒
	if m[loser] != -100 || m[winner] != 80 || res.TeaCharged != 20 {
		t.Fatalf("DIY 口径错: %v tea=%d", m, res.TeaCharged)
	}
}
```

（`memberScoreMap`/`ptr` helper 见 Task 6 Step 1 的 `testhelper_test.go`。`TestExpense_VersionIncrementsSerially` 断言串行 version 递增；过期 version → 30112 的路径由 Task 5 的 `TestRoomRepo_SaveVersioned_VersionConflict` 钉住（sqlite 不模拟真并发，FOR UPDATE 串行化在 MySQL 集成环境生效）。）

- [ ] **Step 2: 跑测确认失败**

```bash
go test ./modules/scorecard/service/ -run TestExpense -v
```

- [ ] **Step 3: 实现（expense.go 核心事务，总分守恒）**

```go
func (s *roomService) Expense(ctx context.Context, userID, roomID uint64, req ExpenseReq) (*ExpenseResult, error) {
	if req.Score <= 0 || req.ToUserID == 0 || req.ToUserID == userID || req.ClientRequestID == "" {
		return nil, errs.New(errs.CodeParamInvalid, nil)
	}
	var out *ExpenseResult
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// ① 幂等：命中即回放
		if prev, err := s.log.GetByRequestID(ctx, roomID, req.ClientRequestID); err != nil {
			return err
		} else if prev != nil {
			snap, err := s.buildSnapshot(ctx, tx, roomID, userID)
			if err != nil {
				return err
			}
			out = &ExpenseResult{Snapshot: snap, LogID: prev.ID, TeaCharged: prev.Tea}
			return nil
		}
		// ② 锁房行
		room, err := s.room.GetForUpdate(ctx, tx, roomID)
		if err != nil {
			return err
		}
		if room == nil {
			return errs.New(errs.CodeScorecardRoomNotFound, nil)
		}
		if room.Status == 1 {
			return errs.New(errs.CodeScorecardRoomSettled, nil)
		}
		if room.Status != 0 {
			return errs.New(errs.CodeScorecardRoomClosed, nil)
		}
		if u, err := s.member.Get(ctx, roomID, userID); err != nil {
			return err
		} else if u == nil {
			return errs.New(errs.CodeScorecardNotMember, nil)
		}
		if to, err := s.member.Get(ctx, roomID, req.ToUserID); err != nil {
			return err
		} else if to == nil {
			return errs.New(errs.CodeScorecardTargetNotMember, nil)
		}
		// ③ 把序号 +1，应用到期 pending
		room.RoundNo++
		applyPending(room)
		// ④ 茶饭计算
		tea, srcUserID := 0, userID
		auto := room.TeaAutoEnabled == 1 && room.TeaPercent > 0 &&
			(room.TeaTargetAmount <= 0 || room.TeaPoolTotal < room.TeaTargetAmount)
		if auto {
			tea = int(math.Round(float64(req.Score) * room.TeaPercent / 100))
			srcUserID = req.ToUserID // 总分守恒：从赢家所得扣
		} else if req.Tea > 0 {
			tea = req.Tea // 手动 DIY
			if req.TeaFromUserID != 0 {
				srcUserID = req.TeaFromUserID
			}
		}
		// ⑤ 记账（表达式列更新，事务内）
		if err := s.member.AddScore(ctx, tx, roomID, userID, -req.Score); err != nil {
			return err
		}
		if err := s.member.AddCumLose(ctx, tx, roomID, userID, req.Score); err != nil {
			return err
		}
		if tea > 0 && auto {
			if err := s.member.AddScore(ctx, tx, roomID, srcUserID, -tea); err != nil {
				return err
			}
			if err := s.member.AddScore(ctx, tx, roomID, req.ToUserID, req.Score-tea); err != nil {
				return err
			}
			room.TeaPoolTotal += tea
		} else if tea > 0 { // DIY：赢家全额，来源扣 T
			if err := s.member.AddScore(ctx, tx, roomID, srcUserID, -tea); err != nil {
				return err
			}
			if err := s.member.AddScore(ctx, tx, roomID, req.ToUserID, req.Score); err != nil {
				return err
			}
			room.TeaPoolTotal += tea
		} else {
			if err := s.member.AddScore(ctx, tx, roomID, req.ToUserID, req.Score); err != nil {
				return err
			}
		}
		alarmed := []uint64{}
		// ⑥ 达标自动关
		if room.TeaTargetAmount > 0 && room.TeaPoolTotal >= room.TeaTargetAmount && room.TeaAutoEnabled == 1 {
			room.TeaAutoEnabled = 0
			if err := s.log.Create(ctx, &model.RoomLog{TenantID: room.TenantID, RoomID: roomID,
				FromUserID: userID, Type: 2, Tea: tea,
				Extra: `{"event":"tea_target_reached"}`}); err != nil {
				return err
			}
		}
		// ⑦ 上限频控：本会话首次达上限全房提醒（记入 AlarmedUserIDs），之后仅写日志。
		// 会话判定：日志 extra 存 session_no，重进房后老日志 session_no 不匹配 → 新会话重新首提醒。
		if room.MaxLoseScore > 0 {
			users, err := s.member.ListByRoom(ctx, roomID)
			if err != nil {
				return err
			}
			for _, u := range users {
				if u.CumLoseScore < room.MaxLoseScore {
					continue
				}
				first := true
				recent, err := s.log.ListTypeByUser(ctx, roomID, u.UserID, 6, 20)
				if err != nil {
					return err
				}
				for _, l := range recent {
					var e struct {
						SessionNo int `json:"session_no"`
					}
					if json.Unmarshal([]byte(l.Extra), &e) == nil && e.SessionNo == u.SessionNo {
						first = false
						break
					}
				}
				if err := s.log.Create(ctx, &model.RoomLog{TenantID: room.TenantID, RoomID: roomID,
					FromUserID: u.UserID, Type: 6, Score: u.CumLoseScore,
					Extra: fmt.Sprintf(`{"cum_lose":%d,"limit":%d,"session_no":%d}`, u.CumLoseScore, room.MaxLoseScore, u.SessionNo)}); err != nil {
					return err
				}
				if first {
					alarmed = append(alarmed, u.UserID)
				}
			}
		}
		// ⑧ 记分日志 + 版本落库（version 乐观锁第二道防线）
		logRow := &model.RoomLog{TenantID: room.TenantID, RoomID: roomID, FromUserID: userID,
			ToUserID: req.ToUserID, Score: req.Score, Tea: tea, Type: 1, ClientRequestID: req.ClientRequestID}
		if err := s.log.Create(ctx, logRow); err != nil {
			return err
		}
		room.ExpireTime = time.Now().Add(roomTTL)
		if err := s.room.SaveVersioned(ctx, tx, room); err != nil { // WHERE id=? AND version=?，RowsAffected=0 → 30112
			return err
		}
		snap, err := s.buildSnapshot(ctx, tx, roomID, userID)
		if err != nil {
			return err
		}
		out = &ExpenseResult{Snapshot: snap, LogID: logRow.ID, TeaCharged: tea, AlarmedUserIDs: alarmed}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// applyPending 「从下把生效」：当前把序号已推进到 pending 目标时刷入并清空。
func applyPending(r *model.Room) {
	if r.PendingApplyRoundNo > 0 && r.RoundNo >= r.PendingApplyRoundNo {
		r.TeaTargetAmount = r.PendingTeaTargetAmount
		r.TeaPercent = r.PendingTeaPercent
		r.MaxLoseScore = r.PendingMaxLoseScore
		r.PendingApplyRoundNo, r.PendingTeaTargetAmount, r.PendingTeaPercent, r.PendingMaxLoseScore = 0, 0, 0, 0
	}
}
```

（实现注记：`applyPending` 在 `round_no++` **之后**调用——pending 在「下一笔」把序号达到 `pending_apply_round_no` 时刷入，当前把用旧配置（spec §3.2）。`UpdateConfig`：软查房 → 校验房主（非房主 30105）→ 校验参数（percent ∈ [0,100]、target/maxLose ≥0，否则 10001）→ 写 `pending_*` + `pending_apply_round_no = room.RoundNo + 1` → 刷 `expire_time` → `SaveVersioned(ctx, s.db, room)` → 返回快照（`config` 变更本身不产生生效字段变化）。`expense.go` 顶部 import 需含 `encoding/json`、`fmt`、`math`、`time`。）

- [ ] **Step 4: 跑测通过（含 Task 6 回归）**

```bash
go test ./modules/scorecard/... -v && go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add modules/scorecard && git commit -m "feat(scorecard): 支出事务（幂等/version/守恒/从下把生效/达标关/上限频控）"
```

---

### Task 8: 结算 / 退房 / 转让房主 + record 写入

**Files:**
- Modify: `modules/scorecard/service/room.go`（加 Settle/Leave/TransferOwner）
- Create: `modules/scorecard/service/settle.go`
- Test: `modules/scorecard/service/settle_test.go`

**Interfaces:**
- Consumes: Task 7 的 roomService；RecordRepository.Create。
- Produces: `Settle(ctx, userID, roomID uint64) error`、`Leave(ctx, userID, roomID uint64) error`、`TransferOwner(ctx, userID, roomID, targetUserID uint64) error`（app 控制器 Task 12 消费）。

- [ ] **Step 1: 写失败测试**

```go
// modules/scorecard/service/settle_test.go
package service

import (
	"context"
	"testing"

	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/tenant"
)

func TestSettle_WritesRecordsForAll(t *testing.T) {
	svc, ctx, roomID, loser, winner := setupRoom(t, 0, false, 0, 0)
	if _, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 40, ToUserID: winner, ClientRequestID: "s1"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settle(ctx, 100, roomID); err != nil {
		t.Fatal(err)
	}
	// 结算后：记分被拒（Task 7 的 TestExpense_RejectClosedRoom 覆盖）
	// record 两行：loser -40 / winner +40
	recs := listRecordsOfRoom(t, svc, ctx, roomID)
	if len(recs) != 2 {
		t.Fatalf("全员 record 行, got %d", len(recs))
	}
}

func TestSettle_NonOwnerRejected(t *testing.T) {
	svc, ctx, roomID, _, _ := setupRoom(t, 0, false, 0, 0)
	if err := svc.Settle(ctx, 200, roomID); !errs.Is(err, errs.CodeScorecardNotOwner) {
		t.Fatalf("非房主应 30105, got %v", err)
	}
}

func TestExpense_RejectedAfterSettle(t *testing.T) {
	svc, ctx, roomID, loser, winner := setupRoom(t, 0, false, 0, 0)
	if err := svc.Settle(ctx, 100, roomID); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 1, ToUserID: winner, ClientRequestID: "z"})
	if !errs.Is(err, errs.CodeScorecardRoomSettled) {
		t.Fatalf("已结算应 30104, got %v", err)
	}
}

func TestLeave_OwnerAutoSettles(t *testing.T) {
	svc, ctx, roomID, _, _ := setupRoom(t, 0, false, 0, 0)
	if err := svc.Leave(ctx, 100, roomID); err != nil {
		t.Fatal(err)
	}
	// 房主退房 → 先结算（全员 record settle_type=2）→ 房主行移除
	recs := listRecordsOfRoom(t, svc, ctx, roomID)
	if len(recs) != 2 { // 100 与 200 各一行
		t.Fatalf("房主退房自动结算应写全员 record, got %d", len(recs))
	}
	snap, err := svc.Snapshot(ctx, 200, roomID)
	if err != nil || snap.Status != 1 {
		t.Fatalf("房间应已结算只读: %v %+v", err, snap)
	}
}

func TestLeave_ZeroDataWritesEmptyRecord(t *testing.T) {
	svc, ctx, roomID, _, _ := setupRoom(t, 0, false, 0, 0)
	if err := svc.Leave(ctx, 200, roomID); err != nil {
		t.Fatal(err)
	}
	recs := listRecordsOfRoom(t, svc, ctx, roomID)
	if len(recs) != 1 || recs[0].Score != 0 || recs[0].SettleType != 4 {
		t.Fatalf("零数据空战绩: %+v", recs)
	}
}

func TestRejoin_ResetsCumLose(t *testing.T) {
	svc, ctx, roomID, loser, winner := setupRoom(t, 0, false, 30, 0)
	if _, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 30, ToUserID: winner, ClientRequestID: "j1"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Leave(ctx, loser, roomID); err != nil {
		t.Fatal(err)
	}
	snap, err := svc.Join(ctx, loser, mustCode(t, svc, ctx, roomID))
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range snap.Members {
		if m.UserID == loser && (m.CumLoseScore != 0 || m.CurScore != 0) {
			t.Fatalf("重进应清零: %+v", m)
		}
	}
}

func TestTransferOwner_Matrix(t *testing.T) {
	svc, ctx, roomID, _, _ := setupRoom(t, 0, false, 0, 0)
	// 非房主发起 → 30105
	if err := svc.TransferOwner(ctx, 200, roomID, 100); !errs.Is(err, errs.CodeScorecardNotOwner) {
		t.Fatalf("非房主: %v", err)
	}
	// 目标不在房 → 30113
	if err := svc.TransferOwner(ctx, 100, roomID, 999); !errs.Is(err, errs.CodeScorecardTargetNotMember) {
		t.Fatalf("目标不在房: %v", err)
	}
	// 合法转让
	if err := svc.TransferOwner(ctx, 100, roomID, 200); err != nil {
		t.Fatal(err)
	}
	snap, _ := svc.Snapshot(ctx, 200, roomID)
	if snap.CreatorID != 200 {
		t.Fatalf("房主未切换: %+v", snap)
	}
	// 原房主变普通玩家后再转让 → 30105
	if err := svc.TransferOwner(ctx, 100, roomID, 200); err == nil {
		t.Fatal("原房主应失去权限")
	}
}
```

- [ ] **Step 2: 跑测确认失败**

```bash
go test ./modules/scorecard/service/ -run 'TestSettle|TestLeave|TestRejoin|TestTransfer' -v
```

- [ ] **Step 3: 实现**

`settle.go` 私有核心 `settleTx(ctx, tx, room, settleType int, excludeUserID uint64)`：全员 `room_user` 各写一行 `record`（终局快照字段从 room 拷贝，`score=u.CurScore`，`cum_lose_score=u.CumLoseScore`）→ `room.status=1` + `SaveVersioned` → `room_log(type=5, extra 含 settle_type)`。

- `Settle`：软查房 → 非房主 30105 → 非进行中 30103/30104 → 事务 `settleTx(settleType=1)`。
- `Leave`：锁房 → 非成员 30102 → 若 `is_creator=1`：事务内先 `settleTx(settleType=2)` 再删自己的 `room_user` + `room_log(type=4)`（其 record 行已由 settleTx 写入，settleType=2）；普通成员：删行 + 若 `CurScore==0 && CumLoseScore==0` 写空战绩（settleType=4）否则写 settleType=3 行 + `room_log(type=4)`；删完 `CountByRoom==0` → `status=2` + `log(type=8)`。**重进约定（已定）**：`room_user` 硬删，`Join` 对不存在的行从 `session_no=1`、`cur_score=0`、`cum_lose_score=0` 起（spec 要求累计清零即满足；`session_no` 语义 = 本行的进房序次，从 1 起）。测试 `TestRejoin_ResetsCumLose` 按此断言。
- `TransferOwner`：软查房 → 状态/房主/目标成员校验 → `black.Exists(userID,target) || black.Exists(target,userID)` → 30108；事务：两行 `SetCreator` 互换、`room.creator_id=target` + SaveVersioned、`room_log(type=9, from=旧, to=新)`。

- [ ] **Step 4: 跑测通过**

```bash
go test ./modules/scorecard/... -v
```

- [ ] **Step 5: Commit**

```bash
git add modules/scorecard && git commit -m "feat(scorecard): 结算/退房/转让房主（自动结算/空战绩/权限矩阵）"
```

---

### Task 9: 超时关房 ticker + expire 刷新

**Files:**
- Modify: `modules/scorecard/service/room.go`（加 `SweepExpired`）
- Modify: `modules/scorecard/module.go`（fx.Invoke ticker；覆盖脚手架装配）
- Test: `modules/scorecard/service/room_test.go`（追加）

**Interfaces:**
- Produces: `RoomService.SweepExpired(ctx context.Context) (int64, error)`；module.go 的每分钟 ticker（照 system SweepExpired 模式）。

- [ ] **Step 1: 写失败测试**

```go
func TestSweepExpired_ClosesAndLogs(t *testing.T) {
	svc, _ := newRoomServiceForTest(t)
	ctx := tenant.WithTenantID(context.Background(), 1)
	snap, _ := svc.Create(ctx, 100, CreateRoomReq{})
	roomSvc := svc.(*roomService)
	// 把 expire_time 拨到过去
	if err := roomSvc.db.WithContext(ctx).Model(&model.Room{}).Where("id = ?", snap.ID).
		UpdateColumn("expire_time", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	n, err := svc.SweepExpired(ctx)
	if err != nil || n != 1 {
		t.Fatalf("sweep: n=%d err=%v", n, err)
	}
	got, _ := roomSvc.room.GetByID(ctx, snap.ID)
	if got == nil || got.Status != 2 {
		t.Fatalf("应关闭: %+v", got)
	}
	logs, _, _ := roomSvc.log.ListByRoom(ctx, snap.ID, 1, 100)
	hasStatusLog := false
	for _, l := range logs {
		if l.Type == 8 {
			hasStatusLog = true
		}
	}
	if !hasStatusLog {
		t.Fatal("缺 type=8 状态变更日志")
	}
}
```

- [ ] **Step 2: 跑测确认失败**

```bash
go test ./modules/scorecard/service/ -run TestSweepExpired -v
```

- [ ] **Step 3: 实现**

`SweepExpired`：`SweepExpired(ctx, time.Now())` 拉逾期房（`status=0 AND expire_time < now`，**逐租户循环**以免跨租户 ctx 缺失——repo 实现内直接不带租户过滤查全部（系统级任务），写日志带各房 TenantID）→ 逐房事务：`status=2`、`room_log(type=8, extra={"event":"timeout_close"})` → 返回关闭数。

`module.go` 追加（照 `modules/system/module.go:90-117` 的 SweepExpired ticker 逐字模式；import 需补 `"context"`、`"time"`、`"alexGo-cloud/pkg/logger"`、`"go.uber.org/zap"`）：

```go
	fx.Invoke(func(lc fx.Lifecycle, svc RoomService) {
		runCtx, cancel := context.WithCancel(context.Background())
		lc.Append(fx.Hook{
			OnStart: func(context.Context) error {
				go func() {
					ticker := time.NewTicker(time.Minute)
					defer ticker.Stop()
					for {
						select {
						case <-runCtx.Done():
							return
						case <-ticker.C:
							if _, err := svc.SweepExpired(context.Background()); err != nil && logger.Log != nil {
								logger.Log.Warn("scorecard sweep failed", zap.Error(err))
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

（Task 14 之前 module.go 仍是脚手架版、控制器注入名不同——本步只加 ticker 段并把 FxModule 里实体控制器的 Provide 换成 roomService（后续 Task 12/13 接控制器时再补全 Provide）。保证每步 `go build` 绿：若脚手架控制器仍在，保留其 Provide 直到 Task 12 删除。）

- [ ] **Step 4: 跑测通过**

```bash
go test ./modules/scorecard/... && go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add modules/scorecard && git commit -m "feat(scorecard): 超时关房 SweepExpired ticker（2h TTL/心跳不续期）"
```

---

### Task 10: RecordService（战绩列表/明细/删除/统计）

**Files:**
- Modify: `modules/scorecard/service/record.go`（覆盖脚手架）
- Test: `modules/scorecard/service/record_test.go`

**Interfaces:**
- Produces:

```go
type RecordService interface {
	MyRecords(ctx context.Context, userID uint64, page, size int) ([]*model.Record, int64, error)
	Detail(ctx context.Context, userID, recordID uint64) ([]*model.Record, error) // 按 room_id 反查全房
	Delete(ctx context.Context, userID, recordID uint64) error
	Stat(ctx context.Context, userID uint64) (*StatResult, error)
	AdminList(ctx context.Context, userID, roomID uint64, page, size int) ([]*model.Record, int64, error) // Task 13 消费
}
type StatResult struct {
	TotalScore int64   `json:"total_score"`
	Win        int64   `json:"win"`
	Lose       int64   `json:"lose"`
	Draw       int64   `json:"draw"`
	WinRate    float64 `json:"win_rate"` // (win)/(win+lose)，分母 0 → 0
}
func NewRecordService(db *gorm.DB, repo repository.RecordRepository) RecordService
type recordService struct {
	db   *gorm.DB
	repo repository.RecordRepository // 测试白盒直取：rs.repo.Create 构造非本人行
}
```

- [ ] **Step 1: 写失败测试**

```go
// modules/scorecard/service/record_test.go
package service

import (
	"context"
	"testing"

	"alexGo-cloud/modules/scorecard/model"
	"alexGo-cloud/modules/scorecard/repository"
	"alexGo-cloud/pkg/tenant"
)

func TestRecords_Flow(t *testing.T) {
	svc, ctx, roomID, loser, winner := setupRoom(t, 0, false, 0, 0)
	if _, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 20, ToUserID: winner, ClientRequestID: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settle(ctx, 100, roomID); err != nil {
		t.Fatal(err)
	}
	// 与 setupRoom 同一个 sqlite 库（跨库会读不到上面刚结算的 record）
	roomInternal := svc.(*roomService)
	rs := NewRecordService(roomInternal.db, repository.NewRecordRepository(roomInternal.db))
	list, total, err := rs.MyRecords(ctx, loser, 1, 10)
	if err != nil || total != 1 || list[0].Score != -20 {
		t.Fatalf("我的战绩: total=%d list=%v err=%v", total, list, err)
	}
	detail, err := rs.Detail(ctx, loser, list[0].ID)
	if err != nil || len(detail) != 2 {
		t.Fatalf("明细应全房 2 行: %v %v", detail, err)
	}
	st, err := rs.Stat(ctx, winner)
	if err != nil || st.TotalScore != 20 || st.Win != 1 || st.WinRate != 1 {
		t.Fatalf("统计: %+v err=%v", st, err)
	}
	// 非本人删除被拒
	other := model.Record{TenantID: 1, RoomID: roomID, UserID: loser, Score: -20, SettleType: 1}
	if err := rs.repo.Create(ctx, []*model.Record{&other}); err != nil {
		t.Fatal(err)
	}
	if err := rs.Delete(ctx, winner, other.ID); err == nil {
		t.Fatal("非本人删除应拒")
	}
	if err := rs.Delete(ctx, loser, other.ID); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: 跑测确认失败 → Step 3 实现**

`Detail`：`GetByID` → 空 30110；`UserID != userID` 30111；`ListByRoom(record.RoomID)`。`Delete`：查行 → 归属校验 → 删。

- [ ] **Step 4: 跑测通过 + Commit**

```bash
go test ./modules/scorecard/... && go build ./...
git add modules/scorecard && git commit -m "feat(scorecard): 战绩查询/明细/删除/统计聚合"
```

---

### Task 11: FriendService（好友/拉黑/排名）

**Files:**
- Modify: `modules/scorecard/service/friend.go`（覆盖脚手架）
- Test: `modules/scorecard/service/friend_test.go`

**Interfaces:**
- Consumes: Task 3 `BriefProvider`、Friend/FriendBlack repo。
- Produces:

```go
type FriendItem struct {
	UserID      uint64  `json:"user_id"`
	Nickname    string  `json:"nickname"`
	Avatar      string  `json:"avatar"`
	RankEnabled bool    `json:"rank_enabled"`
	TotalScore  int64   `json:"total_score"`
	WinRate     float64 `json:"win_rate"`
	IsBlocked   bool    `json:"is_blocked"`
}
type FriendService interface {
	List(ctx context.Context, userID uint64) ([]FriendItem, error)   // 排名序：total_score 降序
	Add(ctx context.Context, userID, friendID uint64) error
	Remove(ctx context.Context, userID, friendID uint64) error
	SetBlack(ctx context.Context, userID, blockedID uint64, blocked bool) error
	SetRank(ctx context.Context, userID, friendID uint64, enabled bool) error
	AdminList(ctx context.Context, page, size int) ([]*model.Friend, int64, error) // Task 13 消费
}
type friendService struct {
	db     *gorm.DB
	friend repository.FriendRepository
	black  repository.FriendBlackRepository
	record repository.RecordRepository // List 排名聚合用
	briefs membersvc.BriefProvider
}
func NewFriendService(db *gorm.DB, friend repository.FriendRepository,
	black repository.FriendBlackRepository, record repository.RecordRepository,
	briefs membersvc.BriefProvider) FriendService
```

- [ ] **Step 1: 写失败测试**

```go
// modules/scorecard/service/friend_test.go
package service

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/scorecard/model"
	"alexGo-cloud/modules/scorecard/repository"
	"alexGo-cloud/pkg/tenant/gormplugin"
)

// newFriendServiceForTest 独立内存库（好友链路不依赖房间数据）。
func newFriendServiceForTest(t *testing.T) (FriendService, fakeBriefs) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Friend{}, &model.FriendBlack{}, &model.Record{}); err != nil {
		t.Fatal(err)
	}
	gormplugin.Register(db, gormplugin.Options{ExemptTables: []string{"tenants", "casbin_rule"}})
	return NewFriendService(db, repository.NewFriendRepository(db),
		repository.NewFriendBlackRepository(db), repository.NewRecordRepository(db), fakeBriefs{}), fakeBriefs{}
}

func TestFriend_AddBlackRank(t *testing.T) {
	fs, briefs := newFriendServiceForTest(t)
	ctx := tenant.WithTenantID(context.Background(), 1)
	_ = briefs // fake BriefProvider
	if err := fs.Add(ctx, 1, 2); err != nil {
		t.Fatal(err)
	}
	if err := fs.Add(ctx, 1, 2); err == nil { // 重复 → 30109
		t.Fatal("重复加好友应拒")
	}
	if err := fs.SetBlack(ctx, 2, 1, true); err != nil {
		t.Fatal(err)
	}
	// 拉黑后：1 再加 3 被 3 拉黑不了——此处断言 Exists 双向判定
	if err := fs.Add(ctx, 1, 2); err == nil { // 关系已存在仍 30109，先 Remove 再 Add 才测拉黑拦截
		t.Fatal("重复")
	}
	if err := fs.Remove(ctx, 1, 2); err != nil {
		t.Fatal(err)
	}
	if err := fs.Add(ctx, 1, 2); err == nil {
		t.Fatal("拉黑期间加好友应 30108")
	}
	if err := fs.SetBlack(ctx, 2, 1, false); err != nil {
		t.Fatal(err)
	}
	if err := fs.Add(ctx, 1, 2); err != nil {
		t.Fatal(err)
	}
	if err := fs.SetRank(ctx, 1, 2, false); err != nil {
		t.Fatal(err)
	}
	items, err := fs.List(ctx, 1)
	if err != nil || len(items) != 1 || items[0].RankEnabled {
		t.Fatalf("排名开关: %+v err=%v", items, err)
	}
}
```

- [ ] **Step 2: 跑测确认失败 → Step 3 实现**

要点：`Add` 校验 `friendID != userID`、对方 BriefProvider 存在（Status 任值都可）、未互相拉黑（30108）、未已存在（30109）；**双向各写一行**（我加你 + 你加我，`rank_enabled=1`）。`List`：取我方 friend 行 → briefs 批量 → `StatsByUser` 批量（循环 ≤20）→ 按 total_score 降序。`SetBlack(true)`：写我方行（幂等：已存在直接成功）；`blocked=false`：删行。

- [ ] **Step 4: 跑测通过 + Commit**

```bash
go test ./modules/scorecard/... && go build ./...
git add modules/scorecard && git commit -m "feat(scorecard): 好友/拉黑/排名开关（双向拦截）"
```

---

### Task 12: app 控制器 + 路由挂载

**Files:**
- Modify: `modules/scorecard/controller/app/app.go`（覆盖脚手架 6 个实体控制器，合一为 AppController）
- Modify: `modules/scorecard/module.go`（Provide 换 AppController；RegisterRoutes 挂 18 条 app 路由；删除脚手架实体 app 控制器文件）
- Test: `modules/scorecard/controller/app/app_test.go`

**Interfaces:**
- Consumes: Task 6–11 全部 service 接口、`token.Validator`（main.go 入口已映射）。
- Produces: `/api/app/scorecard/**` 18 端点（spec §4.1 全表）。

- [ ] **Step 1: 写失败测试（鉴权助手 + 一条业务链路的 httptest）**

```go
// modules/scorecard/controller/app/app_test.go
package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestClaims_MissingToken401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/x", nil)
	ctrl := &AppController{}
	if _, ok := ctrl.claims(c); ok {
		t.Fatal("无 token 应拒绝")
	}
}
```

（完整 service mock 链路若成本高，此 Task 以「单测鉴权助手 + go build + 手工 curl 验收清单」收口；业务正确性已由 service 层测试覆盖。）

- [ ] **Step 2: 跑测确认失败 → Step 3 实现**

`app.go` 全文件（覆盖生成物后）：

```go
package app

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/scorecard/model"
	"alexGo-cloud/modules/scorecard/service"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/resp"
	"alexGo-cloud/pkg/token"
)

// AppController app 端打牌记账接口（Bearer 自鉴权，不走 casbin）。
type AppController struct {
	rooms    service.RoomService
	records  service.RecordService
	friends  service.FriendService
	validator token.Validator
}

func NewAppController(rooms service.RoomService, records service.RecordService,
	friends service.FriendService, validator token.Validator) *AppController {
	return &AppController{rooms: rooms, records: records, friends: friends, validator: validator}
}

// claims 解析 Bearer 并校验 member 身份；失败已写响应，ok=false 直接 return。
func (c *AppController) claims(ctx *gin.Context) (*token.Claims, bool) {
	raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(ctx.GetHeader("Authorization")), "Bearer "))
	if raw == "" {
		resp.Fail(ctx, errs.New(errs.CodeUnauthorized, nil))
		return nil, false
	}
	cl, err := c.validator.Validate(ctx.Request.Context(), raw)
	if err != nil {
		resp.Fail(ctx, errs.New(errs.CodeUnauthorized, nil))
		return nil, false
	}
	if cl.UserType != token.UserTypeMember {
		resp.Fail(ctx, errs.New(errs.CodeForbidden, nil))
		return nil, false
	}
	return cl, true
}

func pathID(ctx *gin.Context, name string) (uint64, bool) {
	id, err := strconv.ParseUint(ctx.Param(name), 10, 64)
	if err != nil || id == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return 0, false
	}
	return id, true
}

func (c *AppController) CreateRoom(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	var req service.CreateRoomReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	snap, err := c.rooms.Create(ctx.Request.Context(), cl.UserID, req)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, snap)
}

func (c *AppController) JoinRoom(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	var req struct {
		RoomCode string `json:"room_code"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil || req.RoomCode == "" {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	snap, err := c.rooms.Join(ctx.Request.Context(), cl.UserID, req.RoomCode)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, snap)
}

func (c *AppController) Snapshot(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	id, ok := pathID(ctx, "id")
	if !ok {
		return
	}
	snap, err := c.rooms.Snapshot(ctx.Request.Context(), cl.UserID, id)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, snap)
}

func (c *AppController) UpdateConfig(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	id, ok := pathID(ctx, "id")
	if !ok {
		return
	}
	var req service.ConfigReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	snap, err := c.rooms.UpdateConfig(ctx.Request.Context(), cl.UserID, id, req)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, snap)
}

func (c *AppController) Expense(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	id, ok := pathID(ctx, "id")
	if !ok {
		return
	}
	var req service.ExpenseReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	res, err := c.rooms.Expense(ctx.Request.Context(), cl.UserID, id, req)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, res)
}

func (c *AppController) Logs(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	id, ok := pathID(ctx, "id")
	if !ok {
		return
	}
	var req resp.PageReq
	if err := ctx.ShouldBindQuery(&req); err != nil { // page/size；PageReq binding 的 max=100 挡超限
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	req.Defaults() // page=1 size=10（无返回值方法，绑定后再调）
	list, total, err := c.rooms.Logs(ctx.Request.Context(), cl.UserID, id, req.Page, req.Size)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.Page(ctx, list, total, req)
}

func (c *AppController) Settle(ctx *gin.Context)   { c.roomAction(ctx, c.rooms.Settle) }
func (c *AppController) Leave(ctx *gin.Context)    { c.roomAction(ctx, c.rooms.Leave) }

// roomAction userID+roomID 双参动作的统一壳（Settle/Leave）。
func (c *AppController) roomAction(ctx *gin.Context, fn func(ctx context.Context, userID, roomID uint64) error) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	id, ok := pathID(ctx, "id")
	if !ok {
		return
	}
	if err := fn(ctx.Request.Context(), cl.UserID, id); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

func (c *AppController) TransferOwner(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	id, ok := pathID(ctx, "id")
	if !ok {
		return
	}
	var req struct {
		TargetUserID uint64 `json:"target_user_id"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil || req.TargetUserID == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if err := c.rooms.TransferOwner(ctx.Request.Context(), cl.UserID, id, req.TargetUserID); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

func (c *AppController) MyRecords(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	var req resp.PageReq
	if err := ctx.ShouldBindQuery(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	req.Defaults()
	list, total, err := c.records.MyRecords(ctx.Request.Context(), cl.UserID, req.Page, req.Size)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.Page(ctx, list, total, req)
}

func (c *AppController) RecordDetail(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	id, ok := pathID(ctx, "recordId")
	if !ok {
		return
	}
	rows, err := c.records.Detail(ctx.Request.Context(), cl.UserID, id)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, rows)
}

func (c *AppController) DeleteRecord(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	id, ok := pathID(ctx, "recordId")
	if !ok {
		return
	}
	if err := c.records.Delete(ctx.Request.Context(), cl.UserID, id); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

func (c *AppController) Stat(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	st, err := c.records.Stat(ctx.Request.Context(), cl.UserID)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, st)
}

func (c *AppController) ListFriends(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	items, err := c.friends.List(ctx.Request.Context(), cl.UserID)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, items)
}

func (c *AppController) AddFriend(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	var req struct {
		FriendUserID uint64 `json:"friend_user_id"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil || req.FriendUserID == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if err := c.friends.Add(ctx.Request.Context(), cl.UserID, req.FriendUserID); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

func (c *AppController) RemoveFriend(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	id, ok := pathID(ctx, "userId")
	if !ok {
		return
	}
	if err := c.friends.Remove(ctx.Request.Context(), cl.UserID, id); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

func (c *AppController) SetFriendBlack(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	id, ok := pathID(ctx, "userId")
	if !ok {
		return
	}
	var req struct {
		Blocked int `json:"blocked"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil || (req.Blocked != 0 && req.Blocked != 1) {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if err := c.friends.SetBlack(ctx.Request.Context(), cl.UserID, id, req.Blocked == 1); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

func (c *AppController) SetFriendRank(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	id, ok := pathID(ctx, "userId")
	if !ok {
		return
	}
	var req struct {
		Enabled int `json:"enabled"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil || (req.Enabled != 0 && req.Enabled != 1) {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if err := c.friends.SetRank(ctx.Request.Context(), cl.UserID, id, req.Enabled == 1); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}
```

（import 清单：`context`、`net/http`、`strconv`、`strings`、`github.com/gin-gonic/gin`、scorecard `model`/`service`、`errs`、`resp`、`token`。`resp.PageReq` 形态已核对：`form:"page"/"size"`，`binding` 自带 `max=100`，`Defaults()` 无返回值须在 bind 后调用；`resp.Page[T](c, list, total, req)` 吃的是值。`roomAction` 的 `fn` 签名 = `func(context.Context, uint64, uint64) error`。）

`module.go` RegisterRoutes 的 app 段：

```go
	app := r.Group("/app/scorecard")
	app.POST("/rooms", c.appCtrl.CreateRoom)
	app.POST("/rooms/join", c.appCtrl.JoinRoom)
	app.GET("/rooms/:id/snapshot", c.appCtrl.Snapshot)
	app.PUT("/rooms/:id/config", c.appCtrl.UpdateConfig)
	app.POST("/rooms/:id/expense", c.appCtrl.Expense)
	app.GET("/rooms/:id/logs", c.appCtrl.Logs)
	app.POST("/rooms/:id/settle", c.appCtrl.Settle)
	app.POST("/rooms/:id/leave", c.appCtrl.Leave)
	app.POST("/rooms/:id/transfer-owner", c.appCtrl.TransferOwner)
	app.GET("/records", c.appCtrl.MyRecords)
	app.GET("/records/:recordId", c.appCtrl.RecordDetail)
	app.DELETE("/records/:recordId", c.appCtrl.DeleteRecord)
	app.GET("/stat", c.appCtrl.Stat)
	app.GET("/friends", c.appCtrl.ListFriends)
	app.POST("/friends", c.appCtrl.AddFriend)
	app.DELETE("/friends/:userId", c.appCtrl.RemoveFriend)
	app.POST("/friends/:userId/black", c.appCtrl.SetFriendBlack)
	app.PUT("/friends/:userId/rank", c.appCtrl.SetFriendRank)
```

FxModule：Provide `service.NewRoomService/NewRecordService/NewFriendService` + `NewAppController` + `NewAdminController`（Task 13 前先 Provide AppController，Admin 路由 Task 13 接）。**同时**删除脚手架实体残留（否则 build 断）：
- `controller/app/{room,room_user,room_log,record,friend,friend_black}.go`（6 实体 app 控制器）
- `controller/admin/{同 6 个}.go`（6 实体 admin 控制器——admin 业务 Task 13 才写，但文件依赖的实体 service 已删，必须此时一并删）
- `module_register_{room,room_user,room_log,record,friend,friend_black}.go`（6 个，内含 `Register<Entity>DBRoutes` 声明）
- `service/{room_user,room_log,friend_black}.go` 等仅被脚手架引用的实体 service/repository/model 文件（room/record/friend 的文件保留——Task 4-8 已重写为业务实现）
- `module.go` RegisterRoutes 里 6 行 `Register<Entity>DBRoutes(r, m.<Entity>AdminCtrl)` 调用整段删除（admin 路由 Task 13 重挂）

- [ ] **Step 4: 跑测通过**

```bash
go test ./modules/scorecard/... && go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add modules/scorecard && git commit -m "feat(scorecard): app 端 18 接口（Bearer 自鉴权）"
```

---

### Task 13: admin 控制器（只读 + 强制关房）

**Files:**
- Create: `modules/scorecard/controller/admin/admin.go`（Task 12 已删 6 实体 admin 控制器，此处新建合一版）
- Modify: `modules/scorecard/module.go`（挂 admin 路由）
- Modify: `modules/system/service/permission_routes.go`（删 3 条多余权限条目）
- Modify: `modules/system/migrations/<ver>_scorecard_menus.up.sql` + `.down.sql`（脚手架菜单种子重写：6 页 → 1 目录 + 3 页）
- Test: `modules/scorecard/service/admin_test.go`（关房逻辑在 service）

**Interfaces:**
- Produces（RoomService 接口追加；`AdminRoomDetail` 的 DTO 在 service 包内定义）:
  - `AdminListRooms(ctx context.Context, status int, page, size int) ([]*model.Room, int64, error)`（status<0 不过滤；repo `ListAdmin` 已在 Task 5 产出）
  - `AdminRoomDetail(ctx context.Context, roomID uint64) (*AdminRoomDetail, error)`；`type AdminRoomDetail struct { Room *model.Room; Members []*model.RoomUser; Logs []*model.RoomLog }`
  - `AdminClose(ctx context.Context, roomID uint64) error`
  - 路由 `/api/admin/scorecard/{rooms,rooms/:id,rooms/:id/close,records,friends}`

- [ ] **Step 1: 写失败测试（AdminClose）**

```go
// modules/scorecard/service/admin_test.go
package service

import (
	"testing"

	errs "alexGo-cloud/pkg/errors"
)

// setupRoom 见 room_test.go。
func TestAdminClose(t *testing.T) {
	svc, ctx, roomID, _, _ := setupRoom(t, 0, false, 0, 0)
	if err := svc.AdminClose(ctx, roomID); err != nil {
		t.Fatal(err)
	}
	room := svc.(*roomService)
	got, _ := room.room.GetByID(ctx, roomID)
	if got == nil || got.Status != 2 {
		t.Fatalf("强制关房: %+v", got)
	}
	// 已关闭再关 → 30103
	if err := svc.AdminClose(ctx, roomID); !errs.Is(err, errs.CodeScorecardRoomClosed) {
		t.Fatalf("重复关闭应 30103, got %v", err)
	}
}
```

- [ ] **Step 2: 跑测确认失败 → Step 3 实现 service 侧**

service 侧：`AdminClose` 无房主校验、进行中/已结算均可关（→status=2）、已关再关 → 30103、log(type=8, extra={"event":"admin_close"})；`AdminListRooms(status,page,size)`（status<0 不过滤）；`AdminRoomDetail(roomID)` 返回 `{Room, Members, Logs}`；`RecordService.AdminList(userID,roomID,page,size)`、`FriendService.AdminList(page,size)`（分页直查，userID/roomID 0=不过滤）。

- [ ] **Step 4: admin 控制器 + module.go 路由**

admin 控制器薄壳（claims 不需要——casbin 中间件已保证；Task 12 已删 6 实体 admin 控制器，此为新建合一版）：

```go
package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/scorecard/service"
	"alexGo-cloud/pkg/resp"
)

// AdminController admin 端打牌记账监管（只读 + 强制关房；casbin 已鉴权）。
type AdminController struct {
	rooms   service.RoomService
	records service.RecordService
	friends service.FriendService
}

func NewAdminController(rooms service.RoomService, records service.RecordService,
	friends service.FriendService) *AdminController {
	return &AdminController{rooms: rooms, records: records, friends: friends}
}

func (c *AdminController) ListRooms(ctx *gin.Context) {
	var req struct {
		resp.PageReq
		Status *int `form:"status"`
	}
	if err := ctx.ShouldBindQuery(&req); err != nil {
		resp.Fail(ctx, err)
		return
	}
	req.Defaults()
	status := -1
	if req.Status != nil {
		status = *req.Status
	}
	list, total, err := c.rooms.AdminListRooms(ctx.Request.Context(), status, req.Page, req.Size)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.Page(ctx, list, total, req.PageReq)
}

func (c *AdminController) RoomDetail(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil || id == 0 {
		resp.Fail(ctx, err)
		return
	}
	d, err := c.rooms.AdminRoomDetail(ctx.Request.Context(), id)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, d)
}

func (c *AdminController) CloseRoom(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil || id == 0 {
		resp.Fail(ctx, err)
		return
	}
	if err := c.rooms.AdminClose(ctx.Request.Context(), id); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

func (c *AdminController) ListRecords(ctx *gin.Context) {
	var req struct {
		resp.PageReq
		UserID uint64 `form:"user_id"`
		RoomID uint64 `form:"room_id"`
	}
	if err := ctx.ShouldBindQuery(&req); err != nil {
		resp.Fail(ctx, err)
		return
	}
	req.Defaults()
	list, total, err := c.records.AdminList(ctx.Request.Context(), req.UserID, req.RoomID, req.Page, req.Size)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.Page(ctx, list, total, req.PageReq)
}

func (c *AdminController) ListFriends(ctx *gin.Context) {
	var req resp.PageReq
	if err := ctx.ShouldBindQuery(&req); err != nil {
		resp.Fail(ctx, err)
		return
	}
	req.Defaults()
	list, total, err := c.friends.AdminList(ctx.Request.Context(), req.Page, req.Size)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.Page(ctx, list, total, req.PageReq)
}
```

（`resp.Fail`/`resp.Page` 的错误入参以 `pkg/resp` 现签名为准——若吃 `error` 而非 `*errs.Error`，controller 里 service 返回值直接透传即可，参数错误处用 `errs.New(errs.CodeParamInvalid, err)` 包装。）

module.go：FxModule 补 Provide `admin.NewAdminController`，ModuleParams/结构体补 `adminCtrl`，RegisterRoutes 挂 admin 段（脚手架的 6 行 `Register<Entity>DBRoutes` 已在 Task 12 删除，此处整体新建）：

```go
	admin := r.Group("/admin/scorecard")
	admin.GET("/rooms", m.adminCtrl.ListRooms)
	admin.GET("/rooms/:id", m.adminCtrl.RoomDetail)
	admin.POST("/rooms/:id/close", m.adminCtrl.CloseRoom)
	admin.GET("/records", m.adminCtrl.ListRecords)
	admin.GET("/friends", m.adminCtrl.ListFriends)
```

- [ ] **Step 5: 清理权限条目与菜单种子**

① **权限**：脚手架按 6 实体生成了 6 条（key 形态 `scorecard:<snake>`，已核 `tools/newmodule/edit_perm.go:76`）。只需 room/record/friend 三条，删除多余三条 `scorecard:room_user`、`scorecard:room_log`、`scorecard:friend_black`（改 `modules/system/service/permission_routes.go` map 字面量，整行删；跑 `gofmt` 确认对齐不碎）。保留的三条应为：

```go
	"scorecard:friend": {"/api/admin/scorecard/friends", "/api/admin/scorecard/friends/*"},
	"scorecard:record": {"/api/admin/scorecard/records", "/api/admin/scorecard/records/*"},
	"scorecard:room":   {"/api/admin/scorecard/rooms", "/api/admin/scorecard/rooms/*"},
```

（脚手架对 room 生成的 path 是 `/api/admin/scorecard/rooms`（naming.Plural），与上表一致；若个别条目路径不同按上表校正。）

② **菜单种子**：脚手架生成了 1 目录 + 6 页面。重写 `modules/system/migrations/<ver>_scorecard_menus.up.sql` 为 1 目录 + 3 页面（房间/战绩/好友；rooms/detail 是 query 子页不进菜单），component 指向 Task 14 的组件路径：

```sql
-- scorecard 模块菜单种子（脚手架生成后手工收敛：6 页 → 3 页）
-- 幂等：全部 INSERT 走 NOT EXISTS；tenant_id=0 全局菜单。

-- ① 模块根目录
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, 0, 'dir', 'Scorecard', '/scorecard', '', 'lucide:package', 'scorecard', 80, 1, 0, NOW(), NOW() FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND path='/scorecard' AND type='dir' AND deleted=0);

-- ② 房间监管页
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', 'Rooms', '/scorecard/rooms', 'views/scorecard/ScorecardRoomsPage', 'lucide:building', 'scorecard:room', 1, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/scorecard' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND path='/scorecard/rooms' AND deleted=0);

-- ③ 战绩监管页
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', 'Records', '/scorecard/records', 'views/scorecard/ScorecardRecordsPage', 'lucide:trophy', 'scorecard:record', 2, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/scorecard' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND path='/scorecard/records' AND deleted=0);

-- ④ 好友监管页
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', 'Friends', '/scorecard/friends', 'views/scorecard/ScorecardFriendsPage', 'lucide:users', 'scorecard:friend', 3, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/scorecard' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND path='/scorecard/friends' AND deleted=0);
```

down.sql 对称删这 4 行（按 path IN (...) AND type IN (...)）。**注意**：若脚手架迁移已跑过本地库、6 页已落库，补一段幂等 DELETE 清理 3 条废弃页面（path `/scorecard/room_users|room_logs|friend_blacks`、`/scorecard/records`/`friends` 若路径撞名则保留不动只清组件旧值——以 `SELECT path,component FROM menus WHERE path LIKE '/scorecard/%'` 现场核对）。

- [ ] **Step 6: 跑测通过**

```bash
go test ./modules/scorecard/... && go test ./modules/system/service/ -run PermissionRoutes && go build ./...
```

- [ ] **Step 7: Commit**

```bash
git add modules/scorecard modules/system && git commit -m "feat(scorecard): admin 监管接口 + 权限/菜单收敛（6页→3页）"
```

---

### Task 14: 前端监管页

**Files:**
- Create: `apps/web-antd/src/api/scorecard.ts`（新文件，单文件收敛版）
- Create: `apps/web-antd/src/views/scorecard/{ScorecardRoomsPage,RoomDetailPage,ScorecardRecordsPage,ScorecardFriendsPage}.vue`
- Delete: 脚手架 6 实体 api/views 目录——`apps/web-antd/src/api/scorecard/{room,room_user,room_log,record,friend,friend_black}/`（每目录含 `api.ts`/`index.ts`/`types.ts`/`__tests__/`）与 `apps/web-antd/src/views/scorecard/{room,room_user,room_log,record,friend,friend_black}/`（每目录含 `list.vue`/`form.vue`/`drawer.vue`）——已核 `tools/newmodule/testdata/golden_paths.txt` 的每实体目录形态

**Interfaces:**
- Consumes: Task 13 admin API；仓库前端惯例（`requestClient`、VxeGrid、Page、useVbenModal，照 `views/order/OrderOrdersPage.vue`）。

- [ ] **Step 1: 写 api 层**

`src/api/scorecard.ts`：

```ts
import { requestClient } from '#/api/request';

export interface ScorecardRoom {
  id: number;
  room_code: string;
  creator_id: number;
  status: number;
  tea_pool_total: number;
  tea_percent: number;
  tea_target_amount: number;
  max_lose_score: number;
  round_no: number;
  expire_time: string;
  created_at: string;
}
export interface ScorecardRecord {
  id: number;
  room_id: number;
  user_id: number;
  score: number;
  cum_lose_score: number;
  tea_pool_total: number;
  settle_type: number;
  created_at: string;
}
export interface ScorecardFriend {
  id: number;
  user_id: number;
  friend_user_id: number;
  rank_enabled: number;
  created_at: string;
}

export function getRoomsApi(params: Record<string, any>) {
  return requestClient.get<{ list: ScorecardRoom[]; total: number }>(
    '/admin/scorecard/rooms',
    { params },
  );
}
export function getRoomDetailApi(id: number) {
  return requestClient.get(`/admin/scorecard/rooms/${id}`);
}
export function closeRoomApi(id: number) {
  return requestClient.post(`/admin/scorecard/rooms/${id}/close`);
}
export function getRecordsApi(params: Record<string, any>) {
  return requestClient.get<{ list: ScorecardRecord[]; total: number }>(
    '/admin/scorecard/records',
    { params },
  );
}
export function getFriendsApi(params: Record<string, any>) {
  return requestClient.get<{ list: ScorecardFriend[]; total: number }>(
    '/admin/scorecard/friends',
    { params },
  );
}
```

（requestClient 前缀以 `src/api/request.ts` 实际配置为准——若无 `/api` 前缀拼接，路径保持 `/admin/scorecard/...`；信封解包 `code===0 → data` 已内置。）

- [ ] **Step 2: 写 4 个页面**

`ScorecardRoomsPage.vue`（完整）：

```vue
<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import { Button, message, Modal, Tag } from 'ant-design-vue';
import { Page } from '@vben/common-ui';

import { closeRoomApi, getRoomsApi } from '#/api/scorecard';

const router = useRouter();
const loading = ref(false);
const list = ref<any[]>([]);
const total = ref(0);
const pagination = ref({ current: 1, pageSize: 20 });

const statusMap: Record<number, { text: string; color: string }> = {
  0: { text: '进行中', color: 'green' },
  1: { text: '已结算', color: 'blue' },
  2: { text: '已关闭', color: 'default' },
};

async function load() {
  loading.value = true;
  try {
    const res = await getRoomsApi({
      page: pagination.value.current,
      size: pagination.value.pageSize,
    });
    list.value = (res as any).list ?? res;
    total.value = (res as any).total ?? 0;
  } finally {
    loading.value = false;
  }
}

function openDetail(row: any) {
  router.push({ path: '/scorecard/rooms/detail', query: { id: row.id } });
}

function confirmClose(row: any) {
  Modal.confirm({
    title: '强制关房',
    content: `确认关闭房间 ${row.room_code}？关闭后成员不可再记分。`,
    onOk: async () => {
      await closeRoomApi(row.id);
      message.success('已关闭');
      await load();
    },
  });
}

onMounted(load);
</script>

<template>
  <Page auto-content-height>
    <div class="p-4">
      <a-table
        :data-source="list"
        :loading="loading"
        :pagination="{ ...pagination, total, onChange: (p: number, ps: number) => { pagination.current = p; pagination.pageSize = ps; load(); } }"
        row-key="id"
      >
        <a-table-column title="房号" data-index="room_code" />
        <a-table-column title="状态" data-index="status">
          <template #default="{ record }">
            <Tag :color="statusMap[record.status]?.color">{{ statusMap[record.status]?.text }}</Tag>
          </template>
        </a-table-column>
        <a-table-column title="房主ID" data-index="creator_id" />
        <a-table-column title="茶饭池" data-index="tea_pool_total" />
        <a-table-column title="把数" data-index="round_no" />
        <a-table-column title="创建时间" data-index="created_at" />
        <a-table-column title="操作">
          <template #default="{ record }">
            <Button size="small" type="link" @click="openDetail(record)">详情</Button>
            <Button
              v-if="record.status === 0"
              danger
              size="small"
              type="link"
              @click="confirmClose(record)"
            >
              强制关房
            </Button>
          </template>
        </a-table-column>
      </a-table>
    </div>
  </Page>
</template>
```

`RoomDetailPage.vue`（完整）：

```vue
<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useRoute } from 'vue-router';
import { Page } from '@vben/common-ui';

import { getRoomDetailApi } from '#/api/scorecard';

const route = useRoute();
const loading = ref(false);
const detail = ref<any>(null);
const logs = ref<any[]>([]);
const logPagination = ref({ current: 1, pageSize: 20 });
const roomID = Number(route.query.id);

async function load() {
  if (!roomID) return;
  loading.value = true;
  try {
    detail.value = await getRoomDetailApi(roomID);
    logs.value = (detail.value as any)?.logs ?? [];
  } finally {
    loading.value = false;
  }
}

onMounted(load);
</script>

<template>
  <Page auto-content-height>
    <div class="p-4 space-y-4">
      <a-spin :spinning="loading">
        <a-descriptions v-if="detail" bordered :column="3" size="small">
          <a-descriptions-item label="房号">{{ detail.room?.room_code }}</a-descriptions-item>
          <a-descriptions-item label="状态">{{ ['进行中', '已结算', '已关闭'][detail.room?.status] }}</a-descriptions-item>
          <a-descriptions-item label="房主ID">{{ detail.room?.creator_id }}</a-descriptions-item>
          <a-descriptions-item label="茶饭池">{{ detail.room?.tea_pool_total }}</a-descriptions-item>
          <a-descriptions-item label="把数">{{ detail.room?.round_no }}</a-descriptions-item>
          <a-descriptions-item label="过期时间">{{ detail.room?.expire_time }}</a-descriptions-item>
        </a-descriptions>

        <h4 class="mt-4 mb-2 font-semibold">成员</h4>
        <a-table :data-source="detail?.members ?? []" :pagination="false" row-key="id" size="small">
          <a-table-column title="用户ID" data-index="user_id" />
          <a-table-column title="当前分" data-index="cur_score" />
          <a-table-column title="累计输分" data-index="cum_lose_score" />
          <a-table-column title="房主">
            <template #default="{ record }">{{ record.is_creator ? '是' : '' }}</template>
          </a-table-column>
        </a-table>

        <h4 class="mt-4 mb-2 font-semibold">流水</h4>
        <a-table
          :data-source="logs"
          :loading="loading"
          :pagination="{ ...logPagination, total: logs.length }"
          row-key="id"
          size="small"
        >
          <a-table-column title="类型" data-index="type" />
          <a-table-column title="from" data-index="from_user_id" />
          <a-table-column title="to" data-index="to_user_id" />
          <a-table-column title="分数" data-index="score" />
          <a-table-column title="茶饭" data-index="tea" />
          <a-table-column title="时间" data-index="created_at" />
        </a-table>
      </a-spin>
    </div>
  </Page>
</template>
```

`ScorecardRecordsPage.vue`（完整；明细 = 列表按 room_id 筛，admin API 无独立明细端点）：

```vue
<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { Button, message, Modal } from 'ant-design-vue';
import { Page } from '@vben/common-ui';

import { getRecordsApi } from '#/api/scorecard';

const loading = ref(false);
const list = ref<any[]>([]);
const filters = ref<{ user_id?: number; room_id?: number }>({});
const pagination = ref({ current: 1, pageSize: 20 });
const total = ref(0);

const settleTypeMap: Record<number, string> = {
  1: '正常结算',
  2: '房主退房自动',
  3: '退房',
  4: '空战绩',
};

async function load() {
  loading.value = true;
  try {
    const res = await getRecordsApi({
      page: pagination.value.current,
      size: pagination.value.pageSize,
      ...(filters.value.user_id ? { user_id: filters.value.user_id } : {}),
      ...(filters.value.room_id ? { room_id: filters.value.room_id } : {}),
    });
    list.value = (res as any).list ?? [];
    total.value = (res as any).total ?? 0;
  } finally {
    loading.value = false;
  }
}

function filterByRoom(row: any) {
  filters.value = { room_id: row.room_id };
  pagination.value.current = 1;
  load();
}

onMounted(load);
</script>

<template>
  <Page auto-content-height>
    <div class="p-4">
      <div class="mb-3 flex gap-2">
        <a-input-number v-model:value="filters.user_id" placeholder="用户ID" :min="1" class="w-40" />
        <a-input-number v-model:value="filters.room_id" placeholder="房间ID" :min="1" class="w-40" />
        <Button type="primary" @click="pagination.current = 1; load()">查询</Button>
        <Button @click="filters = {}; pagination.current = 1; load()">重置</Button>
      </div>
      <a-table
        :data-source="list"
        :loading="loading"
        :pagination="{ ...pagination, total, onChange: (p: number, ps: number) => { pagination.current = p; pagination.pageSize = ps; load(); } }"
        row-key="id"
        size="small"
      >
        <a-table-column title="战绩ID" data-index="id" />
        <a-table-column title="房间ID" data-index="room_id" />
        <a-table-column title="用户ID" data-index="user_id" />
        <a-table-column title="终局分" data-index="score" />
        <a-table-column title="累计输分" data-index="cum_lose_score" />
        <a-table-column title="茶饭池" data-index="tea_pool_total" />
        <a-table-column title="结算类型">
          <template #default="{ record }">{{ settleTypeMap[record.settle_type] ?? record.settle_type }}</template>
        </a-table-column>
        <a-table-column title="时间" data-index="created_at" />
        <a-table-column title="操作">
          <template #default="{ record }">
            <Button size="small" type="link" @click="filterByRoom(record)">看全房</Button>
          </template>
        </a-table-column>
      </a-table>
    </div>
  </Page>
</template>
```

`ScorecardFriendsPage.vue`（完整只读）：

```vue
<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { Page } from '@vben/common-ui';

import { getFriendsApi } from '#/api/scorecard';

const loading = ref(false);
const list = ref<any[]>([]);
const pagination = ref({ current: 1, pageSize: 20 });
const total = ref(0);

async function load() {
  loading.value = true;
  try {
    const res = await getFriendsApi({
      page: pagination.value.current,
      size: pagination.value.pageSize,
    });
    list.value = (res as any).list ?? [];
    total.value = (res as any).total ?? 0;
  } finally {
    loading.value = false;
  }
}

onMounted(load);
</script>

<template>
  <Page auto-content-height>
    <div class="p-4">
      <a-table
        :data-source="list"
        :loading="loading"
        :pagination="{ ...pagination, total, onChange: (p: number, ps: number) => { pagination.current = p; pagination.pageSize = ps; load(); } }"
        row-key="id"
        size="small"
      >
        <a-table-column title="ID" data-index="id" />
        <a-table-column title="用户ID" data-index="user_id" />
        <a-table-column title="好友ID" data-index="friend_user_id" />
        <a-table-column title="参与排名">
          <template #default="{ record }">{{ record.rank_enabled ? '是' : '否' }}</template>
        </a-table-column>
        <a-table-column title="创建时间" data-index="created_at" />
      </a-table>
    </div>
  </Page>
</template>
```

- [ ] **Step 3: 删除脚手架 6 实体 api/views 目录**

```bash
cd apps/web-antd
rm -r src/api/scorecard/room src/api/scorecard/room_user src/api/scorecard/room_log \
      src/api/scorecard/record src/api/scorecard/friend src/api/scorecard/friend_black
rm -r src/views/scorecard/room src/views/scorecard/room_user src/views/scorecard/room_log \
      src/views/scorecard/record src/views/scorecard/friend src/views/scorecard/friend_black
# 确认残留（应只剩 4 个 .vue，无子目录）
ls src/api/scorecard* src/views/scorecard/
```

- [ ] **Step 4: 验证**

```bash
cd apps/web-antd && pnpm typecheck 2>/dev/null || npx vue-tsc --noEmit -p tsconfig.json
```

Expected: 无 scorecard 相关错误。（若仓库无 typecheck 脚本，以 `pnpm dev` 启动无编译报错为准。）

- [ ] **Step 5: Commit**

```bash
git add apps/web-antd && git commit -m "feat(scorecard): admin 监管前端（房间/详情/战绩/好友 4 页）"
```

---

### Task 15: docs/modules.md 回填 + 全量验收

**Files:**
- Modify: `docs/modules.md`（新增 §11 scorecard 章节：模块地图行、端点表、数据模型、权限前缀三行进 §9.1、前端页面表；脚手架报告的手工项）
- Modify: `docs/usage.md`（如需，打牌记账运维要点一段）

- [ ] **Step 1: 回填 modules.md**

在 §1.1 模块地图表加行：`| scorecard | modules/scorecard | 打牌记账（房间/计分/茶饭/战绩/好友） | /api/app/scorecard、/api/admin/scorecard | 23 | 全表 tenant_id |`；
§9.1 前缀表加 `scorecard:room/record/friend` 三行（内容照 Task 13 Step 3 的路径）；
文末新增 `## 11. scorecard — 打牌记账模块`：功能一段、app 端点 18 条表、admin 5 条表、数据模型 6 表要点、跨模块（member BriefProvider）、超时关房 ticker、无 WS 快照一致策略。

- [ ] **Step 2: 全量验收**

```bash
go build ./... && go test ./...
make migrate   # 本地 MySQL：迁移幂等再跑一次
```

Expected: 全绿；迁移重复执行无报错。

- [ ] **Step 3: 冒烟（可选但推荐）**

```bash
make run   # 另开终端
# 用 member 登录拿 token：
curl -s -X POST localhost:8080/api/app/member/auth/login -H 'Content-Type: application/json' \
  -d '{"mobile":"13800000000","password":"..."}'
# 开房→进房→支出→快照→结算 五连（token 换 Bearer）：
curl -s -X POST localhost:8080/api/app/scorecard/rooms -H "Authorization: Bearer $TOKEN" -d '{}'
```

Expected: 开房返回 room_code 4 位；支出后 snapshot version 递增、分数守恒。

- [ ] **Step 4: Commit**

```bash
git add docs && git commit -m "docs(scorecard): modules.md 回填 scorecard 章节"
```

---

## 实现顺序依赖

```
Task 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9 → 10 → 11 → 12 → 13 → 14 → 15
         (码表)  (Brief) (模型) (repo) (房间) (支出) (结算) (TTL) (战绩) (好友) (app) (admin) (前端) (文档)
```

Task 10/11 可与 8/9 并行（互不依赖，仅依赖 5–7）；Task 2/3 可与 4/5 并行。
