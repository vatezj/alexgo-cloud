# starfuyu 星座运势模块（goLucky 融入）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 `modules/starfuyu/goLucky` 独立后端转化为 alexGo-cloud 标准业务模块 `modules/starfuyu`：member 体系 C 端 + casbin 管理端 + 定时任务 + 公众号文章，数据从零建表。

**Architecture:** fx 装配的 `server.Module`，三段路由（`/api/app/starfuyu` Bearer 自鉴权 / `/api/admin/starfuyu` casbin / `/api/public/starfuyu` 免鉴权）。业务层照 goLucky 源码语义重写（一次 DeepSeek 拉 12 星座、cron 只补缺），横切（信封/租户/错误码/权限前缀）全部对齐仓库惯例。luckyU 前端与 admin 页面不在本期。

**Tech Stack:** Go 1.25、Gin、Uber FX、GORM + glebarez/sqlite（测试）、`pkg/resp` 统一信封、`robfig/cron/v3`、`fogleman/gg`、DeepSeek/微信公众号 HTTP 客户端。

**Spec:** `docs/superpowers/specs/2026-11-12-starfuyu-module-design.md`（已定稿；冲突以 spec 为准）

## Global Constraints

- 项目根 `/Users/alex/Desktop/goWork/alexGo-cloud`，**直接在 main 分支提交**，每任务一次 commit
- 本地验证一律 **`CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go test ./modules/starfuyu/... ./pkg/errors/... ./modules/system/service/...`**（按任务触点增减包路径）；**测试零外部依赖**（sqlite 内存、httptest 假 DeepSeek/微信，不依赖 MySQL/网络）
- **所有 error 返回值必须处理**（CI errcheck 开），禁止裸 `_ =` 除非注释理由
- 注释中文、解释「为什么」；新文件保持仓库现有风格
- 模块名/表前缀/配置键/权限前缀一律 `starfuyu`；开关 `cfg.Modules["starfuyu"]`（缺省视为开，`false` 才跳过——fishgame 同款）
- C 端信封 = `pkg/resp`（`code=0` 成功）；**不是** goLucky 老 `{"code":200,...}`；admin 同信封 + casbin
- 错误码仅 **30501–30507**（spec §7），`pkg/errors/code.go` 常量 + `registry` 两处同加
- casbin 前缀仅 **`starfuyu:fortune` / `starfuyu:task`**（spec §7）；**本期不种菜单 SQL**（脚手架若生成菜单迁移必须删）
- 内容表（`starfuyu_fortune`、`starfuyu_sign_cache`）`tenant_id` 恒 0；用户表走租户插件（claims 写回 ctx）
- 模块内一切「今日」日期串与 cron 时刻钉 **`Asia/Shanghai`**（决定 18：`CRON_TZ=` 前缀 + `NowSH()` helper）；不改共享 Dockerfile
- 新依赖：`github.com/robfig/cron/v3`、`github.com/fogleman/gg`（`go get` 后 go.mod/go.sum 随任务提交）
- 存量 goLucky 数据**不迁移**；`goLucky/` 目录在 Task 12 整目录删除（`luckyU/` 保留）
- 不写 Swagger 注解生成流水（脚手架若带 `swagger_doc.go` 可留可删，不强制 swag init）

## Review Focus

1. **UTC 容器下的日期翻转**：进程 `TZ=UTC` 时限流 `date`、fortune `date`、cron「今日」仍必须按北京时间走 → Task 4 测试（钉 `NowSH` 在 UTC 环境下输出 `Asia/Shanghai` 日期）
2. **抓取补缺 vs 覆盖**：已有当日 12 条时 cron 默认跳过、`force:true` 才覆盖；DeepSeek 返回 ≠12 条整批失败不落库 → Task 5 测试
3. **公开签文端点不再打 LLM**：`GET sign/:number` 二次请求命中 `starfuyu_sign_cache`，同号稳定；draw 与公开 GET 共用缓存 → Task 6 测试
4. **C 端鉴权门**：无/坏 token 401 `20001`、非 member 403 `20003`、member 成功且 ctx 租户被写回 → Task 10 测试（scorecard `claims` 同构）
5. **模块开关与死权限**：`modules.starfuyu: false` 时三段路由全不注册；两个权限前缀已登记否则 admin 恒 403 → Task 1（登记）+ Task 11（开关测试）覆盖

---

### Task 1: 脚手架 + 横切接线（错误码 / 权限前缀 / 配置 / 开关）

**Files:**
- Create: `modules/starfuyu/` 全套骨架（newmodule 产物，后续任务覆写）
- Modify: `pkg/errors/code.go`（30501–30507 常量 + registry）
- Modify: `modules/system/service/permission_routes.go`（两前缀）
- Modify: `pkg/config/config.go`（`Starfuyu` 结构体）
- Modify: `alexgo-server/configs/config.yaml`（`modules.starfuyu: true` + `starfuyu:` 段）
- Delete: 脚手架生成的**菜单种子 SQL**（本期不种菜单）

**Interfaces:**
- Consumes: `tools/newmodule` CLI、`errors.New(code, cause)`、`permissionRoutes` map 字面量
- Produces: `cfg.Starfuyu`（后续全部任务的配置来源）；错误码 `errs.CodeStarfuyu*`；前缀 `starfuyu:fortune`/`starfuyu:task`；`modules/starfuyu` 骨架可编译

- [ ] **Step 1: 干跑脚手架确认计划**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud
go run ./tools/newmodule -name=starfuyu -entities=profile,bazi,divination,fortune,limit -skip-frontend -dry-run
```
Expected: 打印 ChangeSet（目录 + main.go + config.yaml + permission_routes + 菜单 SQL），无 `✘`

- [ ] **Step 2: 实跑脚手架**

```bash
go run ./tools/newmodule -name=starfuyu -entities=profile,bazi,divination,fortune,limit -skip-frontend
```
Expected: `✔ 新建模块 starfuyu`，`go build ./... 通过`

- [ ] **Step 3: 删菜单 SQL、按 spec 改写脚手架插入的权限条目**

- 删除 `modules/system/migrations/` 下脚手架新建的 starfuyu 菜单 SQL（`*starfuyu*menu*.sql`）
- `permission_routes.go`：把脚手架按实体生成的条目替换为 spec §7 两条（字母序插入）：

```go
"starfuyu:fortune": {"/api/admin/starfuyu/fortunes", "/api/admin/starfuyu/fortunes/*"},
"starfuyu:task":    {"/api/admin/starfuyu/tasks", "/api/admin/starfuyu/tasks/*", "/api/admin/starfuyu/limits"},
```

- [ ] **Step 4: 加错误码（TDD——先测后码）**

先在 `pkg/errors/code_test.go` 追加断言（沿用文件内既有风格，检查 30501–30507 均在 `registry` 且文案非空）：

```go
for _, c := range []Code{30501, 30502, 30503, 30504, 30505, 30506, 30507} {
    if registry[c] == "" {
        t.Errorf("code %d missing registry message", c)
    }
}
```

Run: `CGO_ENABLED=0 go test ./pkg/errors/ -run Registry -v`（若无该 test 名则 `go test ./pkg/errors/`）
Expected: FAIL（常量/条目不存在）

在 `pkg/errors/code.go` 30404 之后、`5xxxx` 之前追加常量与 registry（文案照 spec §7 表）：

```go
// starfuyu 305xx
CodeStarfuyuFortuneNotFound  Code = 30501
CodeStarfuyuImageFailed      Code = 30502
CodeStarfuyuLimitExceeded    Code = 30503
CodeStarfuyuBaziNotFound     Code = 30504
CodeStarfuyuConstellationMissing Code = 30505
CodeStarfuyuFetchFailed      Code = 30506
CodeStarfuyuMpUploadFailed   Code = 30507
```

registry 七条中文文案照 spec §7 逐字写入。

Run: `CGO_ENABLED=0 go test ./pkg/errors/`
Expected: PASS

- [ ] **Step 5: 配置结构体 + config.yaml（TDD）**

先写 `pkg/config` 测试（新建 `pkg/config/starfuyu_test.go` 或并入既有 config 测试文件）：加载一段含 `starfuyu:` 的 YAML，断言 `cfg.Starfuyu.Deepseek.Model`、`Limits.BaziCalculateDaily`、`Cron.FetchFortune`、`AssetsDir` 字段被解析；缺省 cron 为 `"10 0 * * *"` / `"20 0 * * *"`（缺省补写在 loader 或结构体零值处理——实现时与 Fishgame 段缺省风格一致，放 loader 层若已有先例，否则 service 层兜底，测试钉住最终读到的值）。

Run 预期 FAIL 后，在 `pkg/config/config.go` 的 `Fishgame` 段旁加：

```go
// Starfuyu：starfuyu 星座运势模块配置（spec §8）。
Starfuyu struct {
    Deepseek struct {
        APIKey     string `mapstructure:"api_key"`
        BaseURL    string `mapstructure:"base_url"`
        Model      string `mapstructure:"model"`
        Timeout    int    `mapstructure:"timeout"`     // 秒，<=0 取 120
        MaxRetries int    `mapstructure:"max_retries"`  // <=0 取 3
        RetryDelay int    `mapstructure:"retry_delay"`  // 秒，<=0 取 2
    } `mapstructure:"deepseek"`
    WechatMp struct {
        AppID     string `mapstructure:"app_id"`
        AppSecret string `mapstructure:"app_secret"`
    } `mapstructure:"wechat_mp"`
    Limits struct {
        BaziCalculateDaily  int `mapstructure:"bazi_calculate_daily"`  // <=0 取 5
        DivinationDrawDaily int `mapstructure:"divination_draw_daily"` // <=0 取 10
    } `mapstructure:"limits"`
    Cron struct {
        FetchFortune   string `mapstructure:"fetch_fortune"`   // 缺省 "10 0 * * *"
        GenerateImages string `mapstructure:"generate_images"` // 缺省 "20 0 * * *"
    } `mapstructure:"cron"`
    AssetsDir string `mapstructure:"assets_dir"` // 缺省 "modules/starfuyu/assets"
} `mapstructure:"starfuyu"`
```

`alexgo-server/configs/config.yaml`：`modules:` 下加 `starfuyu: true`；文末按 spec §8 加完整 `starfuyu:` 段（**密钥全空串，不抄 goLucky 明文**）。

Run: `CGO_ENABLED=0 go test ./pkg/config/ && CGO_ENABLED=0 go build ./...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat(starfuyu): 脚手架 + 305xx 错误码 + 权限前缀 + starfuyu 配置段"
```

---

### Task 2: 六张表模型 + 迁移

**Files:**
- Create: `modules/starfuyu/model/{profile,fortune,bazi,divination,request_limit,sign_cache}.go`（或单包多文件按实体拆）
- Create: `modules/starfuyu/migrations/20261112000001_starfuyu_tables.up.sql` / `.down.sql`
- Modify: 脚手架生成的 `model/*` 骨架（覆写为 spec §5）
- Test: `modules/starfuyu/model/model_test.go`

**Interfaces:**
- Consumes: Task 1 的模块目录
- Produces: `model.Profile` / `model.Fortune` / `model.Bazi` / `model.Divination` / `model.RequestLimit` / `model.SignCache`——后续仓储/服务层全部依赖这些类型与 `TableName()`

- [ ] **Step 1: 写失败测试（JSON valuer 往返）**

`modules/starfuyu/model/model_test.go`：

```go
func TestLuckyColorRoundtrip(t *testing.T) {
    in := LuckyColor{Chinese: "红色", English: "Red", Hex: "#ff0000"}
    v, err := in.Value() // driver.Valuer
    // err nil；用 LuckyColor{}.Scan(v) 还原后 DeepEqual in
}
func TestBaziInfoRoundtrip(t *testing.T) { /* 同构：四柱 + TotalWuxing map */ }
```

Run: `CGO_ENABLED=0 go test ./modules/starfuyu/model/`
Expected: FAIL（类型不存在）

- [ ] **Step 2: 实现六个模型**

字段照 goLucky 源码全集 + spec §5（`model/fortune.go` 含 overall/love/career/wealth/health 五分值、`lucky_number`、`daily_overview`、`lucky_color` JSON、tips；`bazi_info` JSON 含四柱/五行/节气/农历/生肖/星座）。每模型：

- `TenantID uint64` + `CreatedAt/UpdatedAt time.Time`，GORM 标签风格照 `modules/fishgame/model`
- `TableName()` = spec 表名（`starfuyu_*` 单数前缀全量）
- `LuckyColor`、`BaziInfo` 实现 `Scan`/`Value`（照 goLucky `models/fortune.go`、`models/bazi.go` 迁）
- 用户表带 `MemberUserID uint64`（列 `member_user_id`，唯一索引 `uk` 含 `tenant_id`）；fortune/sign_cache 的 tenant 恒 0 由仓储写死，模型仍带字段

- [ ] **Step 3: 写迁移 SQL**

`up.sql` 六表 `CREATE TABLE IF NOT EXISTS`，列与模型严格同构（照 `modules/fishgame/migrations/20261023000001_fishgame_tables.up.sql` 风格：bigint unsigned 主键、utf8mb4、无外键、注释中文）。要点：

| 表 | 唯一键 / 关键索引 |
|---|---|
| `starfuyu_profile` | `uk_profile_member` (`tenant_id`,`member_user_id`) |
| `starfuyu_fortune` | `uk_fortune_date_const` (`tenant_id`,`date`,`constellation`)——tenant 恒 0 仍入 uk |
| `starfuyu_bazi` | `uk_bazi_member` (`tenant_id`,`member_user_id`) |
| `starfuyu_divination` | `idx_div_member_created` (`tenant_id`,`member_user_id`,`created_at`) |
| `starfuyu_request_limit` | `uk_limit_user_date_type` (`tenant_id`,`member_user_id`,`date`,`type`) |
| `starfuyu_sign_cache` | `uk_sign_number` (`tenant_id`,`sign_number`) |

`down.sql` 对应 `DROP TABLE IF EXISTS` 六行（逆序）。

- [ ] **Step 4: 验证 SQL 与模型同构**

Run: `CGO_ENABLED=0 go test ./modules/starfuyu/model/ && CGO_ENABLED=0 go build ./...`
Expected: PASS（SQL 结构人工对照模型；sqlite 测试用 AutoMigrate 从模型迁，MySQL SQL 留给集成/启动 migrate）

可选加固：在 model 测试里用 sqlite `AutoMigrate` 六模型 + 抽查列名存在，防标签笔误。

- [ ] **Step 5: Commit**

```bash
git add modules/starfuyu/model modules/starfuyu/migrations
git commit -m "feat(starfuyu): 六表模型与迁移（内容表 tid=0，用户表挂 member_user_id）"
```

---

### Task 3: 仓储层

**Files:**
- Create/Modify: `modules/starfuyu/repository/{profile,fortune,bazi,divination,request_limit,sign_cache}.go`
- Test: `modules/starfuyu/repository/repository_test.go`

**Interfaces:**
- Consumes: Task 2 模型；`pkg/repo`（若既有工具）；gorm
- Produces（后续服务层唯一数据入口，签名钉死）：

```go
type ProfileRepository interface {
    GetByMember(ctx context.Context, memberUserID uint64) (*model.Profile, error) // 软查 miss → (nil, nil)
    Upsert(ctx context.Context, memberUserID uint64, constellation string) (*model.Profile, error)
}
type FortuneRepository interface {
    GetByDate(ctx context.Context, date string) ([]*model.Fortune, error) // 内容：调用方保证 ctx tid=0
    CountByDate(ctx context.Context, date string) (int64, error)
    GetOne(ctx context.Context, date, constellation string) (*model.Fortune, error) // miss → (nil, nil)
    UpsertBatch(ctx context.Context, rows []*model.Fortune) error // (date,constellation) 冲突更新
    ListAdmin(ctx context.Context, date, constellation string, page, size int) ([]*model.Fortune, int64, error)
}
type BaziRepository interface {
    GetByMember(ctx context.Context, memberUserID uint64) (*model.Bazi, error)
    Upsert(ctx context.Context, memberUserID uint64, in *model.Bazi) (*model.Bazi, error)
}
type DivinationRepository interface {
    Create(ctx context.Context, in *model.Divination) (*model.Divination, error)
    ListByMember(ctx context.Context, memberUserID uint64, page, size int) ([]*model.Divination, int64, error)
}
type RequestLimitRepository interface {
    // CheckAndIncrement 查或建当日行并 count+1；返回新 count。超限由 service 判定。
    CheckAndIncrement(ctx context.Context, memberUserID uint64, date, typ string) (int, error)
    AdminTodaySummary(ctx context.Context, date string) ([]TodayLimitRow, error) // type+count 聚合
}
type SignCacheRepository interface {
    Get(ctx context.Context, number int) (*model.SignCache, error) // miss → (nil, nil)
    Create(ctx context.Context, number int, content string) (*model.SignCache, error) // 冲突重读
}
```

构造函数统一 `NewXxxRepository(db *gorm.DB) XxxRepository`（fx Provide 用）。

- [ ] **Step 1: 写失败测试骨架**

`repository_test.go`：`newTestDB(t)` 用 `sqlite.Open(":memory:")` + `AutoMigrate` 六模型 + `gormplugin.Register`（照 `modules/fishgame/module_test.go` 的 `newModuleTestRouter` 前半）。先写 `TestProfileUpsertIdempotent`、`TestFortuneUpsertBatchOverwrites`、`TestSignCacheCreateRaceReRead`（同号 Create 两次第二次返回已存在行内容）。

Run: `CGO_ENABLED=0 go test ./modules/starfuyu/repository/`
Expected: FAIL（接口/实现不存在）

- [ ] **Step 2: 实现六仓储**

- 用户表查询/写入**不显式加 tenant 过滤**（依赖 gormplugin + claims 写回的 ctx；照 scorecard 惯例）
- fortune/sign_cache 写入显式 `TenantID: 0`；读路径由 service 层 `tenant.WithTenantID(ctx, 0)` 包裹（Task 4/5），仓储本身不猜
- `UpsertBatch` 用 `clause.OnConflict`（列 `tenant_id,date,constellation` → DoUpdates 全业务列）
- `CheckAndIncrement` 单条 SQL `ON DUPLICATE KEY UPDATE count=count+1` 或读改写事务（sqlite/mysql 双兼容优先读改写 + 唯一键冲突重试）

- [ ] **Step 3: 跑通测试**

Run: `CGO_ENABLED=0 go test ./modules/starfuyu/repository/ -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add modules/starfuyu/repository
git commit -m "feat(starfuyu): 六仓储（内容表 tid=0、用户表走插件）"
```

---

### Task 4: 时区 helper + Fortune 抓取/查询服务

**Files:**
- Create: `modules/starfuyu/service/clock.go`（`NowSH`/`TodaySH`）
- Create: `modules/starfuyu/service/deepseek.go`（HTTP 客户端）
- Create: `modules/starfuyu/service/fortune.go`
- Test: `modules/starfuyu/service/clock_test.go`、`fortune_test.go`

**Interfaces:**
- Consumes: Task 3 `FortuneRepository`；`cfg.Starfuyu.Deepseek`
- Produces:

```go
// clock.go
func NowSH() time.Time          // time.Now().In(loc)，loc = time.LoadLocation("Asia/Shanghai") 进程内缓存
func TodaySH() string           // NowSH().Format("2006-01-02")

// deepseek.go —— 抽接口便于测试替换
type FortuneFetcher interface {
    FetchFortunes(ctx context.Context, date string) ([]*model.Fortune, error) // 一次调用，必须恰好 12 条否则 error
}
func NewDeepSeekFetcher(cfg ...) FortuneFetcher // 真实现：POST cfg.BaseURL，prompt 照 goLucky service/deepseek.go

// fortune.go
type FortuneService interface {
    // Fetch 拉取并落库：已有 count==12 且 !force → (skipped=true, nil)；
    // force 或不足 12 → FetchFortunes + UpsertBatch（整批，失败不落）。
    Fetch(ctx context.Context, date string, force bool) (skipped bool, err error)
    GetByDate(ctx context.Context, date string) ([]*model.Fortune, error) // 包 tid=0 读；无数据空列表
    GetOne(ctx context.Context, date, constellation string) (*model.Fortune, error) // miss → 30501
    ListAdmin(ctx context.Context, date, constellation string, page, size int) ([]*model.Fortune, int64, error)
}
func NewFortuneService(fortunes repository.FortuneRepository, fetcher FortuneFetcher) FortuneService
```

- [ ] **Step 1: 写失败测试——时区**

`clock_test.go`：

```go
func TestTodaySH_PinnedToShanghai(t *testing.T) {
    t.Setenv("TZ", "UTC") // 进程 UTC；NowSH 不得跟随
    // 固定断言：TodaySH() 与 time.Now().In(sh).Format 一致；
    // 且格式匹配 ^\d{4}-\d{2}-\d{2}$
}
```

Run: `CGO_ENABLED=0 go test ./modules/starfuyu/service/ -run TodaySH -v`
Expected: FAIL

- [ ] **Step 2: 实现 clock + 写失败测试 Fetch 语义**

实现 `NowSH`/`TodaySH`。随后 `fortune_test.go` 用 stub：

```go
type stubFetcher struct {
    calls int
    rows  []*model.Fortune
    err   error
}
```

用例：
1. 库空 → `Fetch(date,false)` 调 stub 1 次、落库 12 条、`skipped=false`
2. 库已有 12 条 → `Fetch(date,false)` **不调 stub**、`skipped=true`
3. 库已有 12 条 + `force=true` → 调 stub、Upsert 覆盖
4. stub 返回 11 条 → error（30506 语义由 service 包 `errs.New(CodeStarfuyuFetchFailed, err)`）、库仍 0 条
5. `GetByDate` 空库 → 空切片非 error

Run: `CGO_ENABLED=0 go test ./modules/starfuyu/service/ -run Fortune -v`
Expected: FAIL（Fetch 未实现）

- [ ] **Step 3: 实现 DeepSeek fetcher + FortuneService**

- `NewDeepSeekFetcher`：超时/重试从 cfg 读（<=0 缺省 120s/3 次/2s）；prompt 与 12 条校验照 goLucky `service/deepseek.go`（可对照源码抄 prompt 块）；解析 JSON 数组后逐行填 `Date`/`TenantID=0`
- `Fetch`：`CountByDate==12 && !force` 短路；否则 `FetchFortunes` → 校验 12 → `UpsertBatch`；错误包 30506
- `GetByDate`：`ctx = tenant.WithTenantID(ctx, 0)` 后读
- `GetOne` miss → `errs.New(CodeStarfuyuFortuneNotFound, nil)`

- [ ] **Step 4: 跑通**

Run: `CGO_ENABLED=0 go test ./modules/starfuyu/service/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add modules/starfuyu/service
git commit -m "feat(starfuyu): 上海时区 helper + 运势抓取（一次12星座、只补缺、force覆盖）"
```

---

### Task 5: 限流 + 八字 + 星座偏好（Profile）服务

**Files:**
- Create: `modules/starfuyu/service/{limit,bazi,profile}.go`
- Test: `modules/starfuyu/service/{limit,bazi,profile}_test.go`

**Interfaces:**
- Consumes: Task 3 `RequestLimitRepository`/`BaziRepository`/`ProfileRepository`；Task 4 `FortuneFetcher` 同款的 `BaziCalculator` 接口
- Produces:

```go
type ProfileService interface {
    // GetMy 未设星座 → (nil, nil)（controller 映射空串，不报错——spec §4.1 /profile）
    GetMy(ctx context.Context, memberUserID uint64) (*model.Profile, error)
    // SetMy 校验 12 星座枚举（IsValidConstellation），非法 → 10001；upsert
    SetMy(ctx context.Context, memberUserID uint64, constellation string) (*model.Profile, error)
}
func NewProfileService(profiles repository.ProfileRepository) ProfileService

type LimitService interface {
    // Allow 校验并计数：新 count > dailyLimit → 30503（不回滚计数，照 goLucky CheckAndIncrement 语义）
    Allow(ctx context.Context, memberUserID uint64, typ string) error
}
func NewLimitService(limits repository.RequestLimitRepository, cfg *config.Config) LimitService
// typ 常量：TypeBaziCalculate = "bazi_calculate"、TypeDivinationDraw = "divination_draw"
// dailyLimit 从 cfg.Starfuyu.Limits 取，<=0 缺省 5/10

type BaziCalculator interface {
    Calculate(ctx context.Context, birthDate, birthTime, birthPlace string) (*model.BaziInfo, error)
}
type BaziService interface {
    // Calculate 限流 → 调 calculator → Upsert 我的八字
    Calculate(ctx context.Context, memberUserID uint64, req BaziReq) (*model.Bazi, error)
    GetMy(ctx context.Context, memberUserID uint64) (*model.Bazi, error) // miss → 30504
    UpdateMy(ctx context.Context, memberUserID uint64, req BaziReq) (*model.Bazi, error) // 同 Calculate
}
type BaziReq struct { BirthDate, BirthTime, BirthPlace string } // birth_date 必填 2006-01-02；birth_time 可空
func NewBaziService(bazis repository.BaziRepository, limits LimitService, calc BaziCalculator) BaziService
// NewDeepSeekBaziCalculator(cfg) 实现 BaziCalculator（prompt 照 goLucky service/bazi.go）
```

- [ ] **Step 1: 失败测试**

- `TestLimit_AllowBlocksOverDaily`：cfg limit=2，Allow 第 1/2 次 nil，第 3 次 `errs.Code` == 30503
- `TestLimit_UsesShanghaiDate`：`t.Setenv("TZ","UTC")` 下 key 日期 == `TodaySH()`
- `TestBazi_CalculateUpsertsAndGetMyMiss30504`：stub calculator 返回固定 `BaziInfo`；GetMy 空库断言错误码 30504；Calculate 后 GetMy 命中
- `TestProfile_SetMyRejectsUnknownConstellation`：`"十三座"` → 10001；`"白羊座"` upsert 后 GetMy 命中

Run: `CGO_ENABLED=0 go test ./modules/starfuyu/service/ -run 'Limit|Bazi|Profile' -v`
Expected: FAIL

- [ ] **Step 2: 实现并跑通**

- `Allow`：`CheckAndIncrement(ctx, uid, TodaySH(), typ)` → 超限 30503
- `Calculate`：先 `Allow`，再 calc，再 `Upsert`（`BaziInfo` JSON 已由模型 valuer 落列）
- `ProfileService`：`SetMy` 用 Task 7 才落地的 `IsValidConstellation`——**枚举函数提前到本任务**放进 `service/constellation.go`（从 goLucky `utils/constellation.go` 原样迁，Task 7 只复用），避免任务倒序依赖
- DeepSeek calculator：prompt/解析照 goLucky `service/bazi.go`；API 失败上抛用 `CodeInternal`，不占 305xx

Run: `CGO_ENABLED=0 go test ./modules/starfuyu/service/`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add modules/starfuyu/service
git commit -m "feat(starfuyu): 每日限流（沪时区）+ 八字计算 + 星座偏好"
```

---

### Task 6: 签文缓存 + 求签服务

**Files:**
- Create: `modules/starfuyu/service/divination.go`
- Test: `modules/starfuyu/service/divination_test.go`

**Interfaces:**
- Consumes: Task 3 `SignCacheRepository`/`DivinationRepository`/`BaziRepository`；Task 5 `LimitService`；`SignInterpreter` 接口
- Produces:

```go
type SignInterpreter interface {
    // SignContent 按签号生成签文（仅 cache miss 时调用）
    SignContent(ctx context.Context, number int) (string, error)
    // Interpret 结合八字/问题生成解读 JSON（draw 专用）
    Interpret(ctx context.Context, info *model.BaziInfo, signContent, question, category string) (*DivinationResult, error)
}
type DivinationResult struct {
    Interpretation, LuckyDirection, LuckyColor, Advice string
    Category string
}
type DivinationService interface {
    // GetSign 公开端点：cache hit 直读；miss → interpreter.SignContent → Create 缓存（305xx 不用，interpreter 失败回落模板）
    GetSign(ctx context.Context, number int) (string, error) // number∉[1,100] → 10001
    // Draw 限流 → 取/建签文缓存 →（可选）解读 → 落 Divination 行
    Draw(ctx context.Context, memberUserID uint64, question, category string) (*model.Divination, error)
    History(ctx context.Context, memberUserID uint64, page, size int) ([]*model.Divination, int64, error)
}
func NewDivinationService(cache repository.SignCacheRepository, divs repository.DivinationRepository,
    bazis repository.BaziRepository, limits LimitService, interp SignInterpreter) DivinationService
// NewDeepSeekSignInterpreter(cfg) + 内置模板兜底（照 goLucky getDefaultSignContent，扩为按 number 变体）
```

- [ ] **Step 1: 失败测试**

```go
func TestGetSign_SecondCallHitsCache(t *testing.T) {
    // stub interp.calls==1；GetSign(42) 两次 → 第二次 interp 不再调、两次内容相同
}
func TestGetSign_InvalidNumber(t *testing.T)      // 0/101 → 10001
func TestDraw_UsesCacheAndLimit(t *testing.T) {
    // 先 GetSign(7) 灌缓存；Draw 抽签可注入 rand 或循环至命中 7（实现用可注入 rng，测试钉 Draw 不二次调 SignContent）
    // limit=0 cfg → 第一次 Draw 30503
}
func TestDraw_PersistsRecord(t *testing.T)         // History 能查到 1 条，sign_number ∈ [1,100]
```

Run: `CGO_ENABLED=0 go test ./modules/starfuyu/service/ -run Divination -v`
Expected: FAIL

- [ ] **Step 2: 实现并跑通**

- `GetSign`：1–100 校验 → cache.Get → miss 则 interp + cache.Create；interp 失败用模板兜底（**兜底也写缓存**，保证二次稳定）
- `Draw`：`Allow(TypeDivinationDraw)` → 随机 sign_number（`rand` 可注入 `rng func(int)int`，缺省 `rand.Intn`）→ 经 `GetSign` 同路径取签文 → `bazis.GetByMember`（有则 interp，无则降级空解读字段非空模板）→ `divs.Create`
- 随机源与签号范围常量 `signMin=1, signMax=100`

Run: `CGO_ENABLED=0 go test ./modules/starfuyu/service/`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add modules/starfuyu/service
git commit -m "feat(starfuyu): 签文按号缓存 + 求签/历史（公开端点零 LLM 出口）"
```

---

### Task 7: 运势图片服务（gg + 内置字体 + storage）

**Files:**
- Create: `modules/starfuyu/service/image.go`
- Create: `modules/starfuyu/assets/fonts/`（放入 OFL 字体文件）与 `modules/starfuyu/assets/bgimage/bg.png`（从 goLucky 拷贝）
- （`service/constellation.go` 与 `IsValidConstellation` 已在 Task 5 落地，本任务直接 import）
- Test: `modules/starfuyu/service/image_test.go`

**Interfaces:**
- Consumes: `cfg.Starfuyu.AssetsDir`、`pkg/storage.Driver`、Task 4 `FortuneService.GetOne`
- Produces:

```go
type ImageService interface {
    // Ensure 生成（或已存在则跳过）当日星座图，返回 storage rel key：fortune/{date}/{constellation}.jpg
    Ensure(ctx context.Context, date, constellation string) (key string, err error)
    // GenerateAll 遍历 12 星座 Ensure；单张失败记日志继续，返回成功数
    GenerateAll(ctx context.Context, date string) (ok int, err error)
}
func NewImageService(fortunes FortuneService, driver storage.Driver, cfg *config.Config) ImageService
```

- [ ] **Step 1: 资产就位**

```bash
mkdir -p modules/starfuyu/assets/fonts modules/starfuyu/assets/bgimage
cp modules/starfuyu/goLucky/bgimage/bg.png modules/starfuyu/assets/bgimage/bg.png
# 字体：OFL 的 Noto Sans SC（或子集）放入 modules/starfuyu/assets/fonts/NotoSansSC-Regular.otf
# 获取方式：从 Google Fonts / notofonts 上游发行包拷贝（网络下载一次，二进制入库）
```

Expected: 两文件存在；README 不强制

- [ ] **Step 2: 失败测试**

`image_test.go`：内存 sqlite + stub fortune 12 条 + `storage.NewLocalDriver(t.TempDir())` + 真实 assets 路径：

```go
func TestEnsure_WritesJPEGAndSecondCallSkips(t *testing.T) {
    // Ensure → key 正确、盘上文件 >0 字节；
    // 再 Ensure → 不重绘（可用 mtime 或调用计数桩）
}
func TestEnsure_MissingFortune30501(t *testing.T)
func TestGenerateAll_PartialFailureContinues(t *testing.T) // 12 条中 stub 缺 1 → ok=11, err 非 nil 或按实现：spec 允许单张失败 continue，断言 ok>=11
```

Run: `CGO_ENABLED=0 go test ./modules/starfuyu/service/ -run Image -v`
Expected: FAIL

- [ ] **Step 3: 实现 ImageService**

- 字体加载顺序：`cfg` 未单列字体路径则扫 `AssetsDir/fonts/*.{ttf,otf,ttc}` 取第一个 → 加载失败 **30502**（不再静默 ASCII——决定 19）
- 绘制布局照 goLucky `service/image.go`（背景 + 标题/分数/tips），依赖 `github.com/fogleman/gg`
- 输出 JPEG 写 `driver.Save(ctx, key, bytes.Reader)`；`Ensure` 先 `driver.Open` 探测存在
- `GenerateAll`：遍历 `ConstellationList`（Task 5 已迁入 `service/constellation.go`）

- [ ] **Step 4: 跑通**

Run: `CGO_ENABLED=0 go test ./modules/starfuyu/service/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add modules/starfuyu/service modules/starfuyu/assets go.mod go.sum
git commit -m "feat(starfuyu): 运势生图（gg + 内置中文字体 + storage 落盘）"
```

---

### Task 8: 公众号文章 + 微信 MP 客户端

**Files:**
- Create: `modules/starfuyu/service/{article,wechat_mp}.go`
- Test: `modules/starfuyu/service/{article,wechat_mp}_test.go`

**Interfaces:**
- Consumes: Task 4 `FortuneService`、Task 7 `ImageService`、`cfg.Starfuyu.WechatMp`
- Produces:

```go
type ArticleService interface {
    // BuildHTML 生成整版（12 星座）或单星座 HTML（照 goLucky _generateFortuneArticleHTML/_generateSingle…）
    BuildHTML(ctx context.Context, date, constellation string) (string, error) // constellation=="" 整版
    // UploadToMP 生成文章 + 图素材 → 草稿箱；返回 [{title, media_id}]
    UploadToMP(ctx context.Context, date string) ([]UploadedArticle, error)
}
type UploadedArticle struct { Title string `json:"title"`; MediaID string `json:"media_id"` }
func NewArticleService(fortunes FortuneService, images ImageService, mp MPClient, cfg *config.Config) ArticleService

type MPClient interface {
    UploadImageMaterial(ctx context.Context, filePath string) (mediaID string, err error) // 或 bytes
    AddDraft(ctx context.Context, articles []MPArticle) (mediaID string, err error)
}
type MPArticle struct { Title, Content, Digest, Author string; ShowCoverPic int; ThumbMediaID string }
func NewWeChatMPClient(appID, appSecret string) MPClient // access_token 进程内缓存照 goLucky service/wechat_mp.go
```

- [ ] **Step 1: 失败测试**

- `TestBuildHTML_ContainsAllConstellations`：stub 12 条 → HTML 含 12 星座名与分数 stars 字样
- `TestUploadToMP_FailureMaps30507`：httptest 假微信 token/草稿接口返回 errcode≠0 → `errs.Code` == 30507
- `TestUploadToMP_SuccessReturnsMediaIDs`：假接口 200 → len==13（1 整版 + 12 单篇，照 goLucky handler 行为）或实现收敛为 1 篇整版——**钉住实现选择：整版 1 篇 + 可选单篇，测试与 BuildHTML 范围一致；默认整版 1 篇**（简化，spec §4.2 只要求返回 title/media_id 列表）

Run: `CGO_ENABLED=0 go test ./modules/starfuyu/service/ -run 'Article|MP' -v`
Expected: FAIL

- [ ] **Step 2: 实现**

- HTML 生成照 goLucky `handler/fortune.go` 私有函数迁为 service（去 gin 依赖）
- `WeChatMPClient`：token 缓存 + 提前 5 分钟刷新；HTTP 错误/errcode → 包 30507
- **密钥空配**：`UploadToMP` 直接 30507（不打网络）——验收 §11.4

- [ ] **Step 3: 跑通 + Commit**

Run: `CGO_ENABLED=0 go test ./modules/starfuyu/service/`
Expected: PASS

```bash
git add modules/starfuyu/service
git commit -m "feat(starfuyu): 公众号文章 HTML 生成与草稿箱上传"
```

---

### Task 9: 定时任务（robfig/cron + fx 生命周期）

**Files:**
- Create: `modules/starfuyu/service/tasks.go`（任务编排，可单测）
- Create/Modify: `modules/starfuyu/module.go` 内 cron 装配（或 `cron.go`）
- Test: `modules/starfuyu/service/tasks_test.go`

**Interfaces:**
- Consumes: Task 4 `FortuneService.Fetch`、Task 7 `ImageService.GenerateAll`、`cfg.Starfuyu.Cron`
- Produces:

```go
type TaskRunner interface {
    FetchToday(ctx context.Context, force bool) error   // date=TodaySH()
    GenerateImagesToday(ctx context.Context) error
    ExecuteAll(ctx context.Context, force bool) error   // Fetch + Generate
}
func NewTaskRunner(fortunes FortuneService, images ImageService) TaskRunner
// module 侧：
//   cronSpec(expr) = "CRON_TZ=Asia/Shanghai " + expr
//   fx.OnStart: robfig/cron.New(cron.WithSeconds() 若 expr 6 段，否则默认 5 段——
//   **钉 5 段标准 spec，不 WithSeconds**；WithParser(cron.NewParser(cron.Minute|Hour|Dom|Month|Dow|Descriptor))
//   AddFunc(cronSpec(cfg.Cron.FetchFortune), ...)
//   fx.OnStop: c.Stop() 等 ctx.Done()
```

- [ ] **Step 1: 失败测试**

```go
func TestExecuteAll_CallsFetchThenImages(t *testing.T) // 记录调用序
func TestCronSpec_PrefixedWithShanghai(t *testing.T)    // cronSpec("10 0 * * *") == "CRON_TZ=Asia/Shanghai 10 0 * * *"
func TestFetchToday_UsesTodaySH(t *testing.T)           // TZ=UTC 下 date 断言 == TodaySH()
```

Run: `CGO_ENABLED=0 go test ./modules/starfuyu/service/ -run 'Task|Cron' -v`
Expected: FAIL

- [ ] **Step 2: 实现 + 接进 module.go**

- `TaskRunner` 三方法；FetchToday 委托 `fortunes.Fetch(ctx, TodaySH(), force)`
- `module.go` 增加 `fx.Invoke`（或 Provide 一个 lifecycle hook）：OnStart 起 cron、OnStop 停；DeepSeek/资产空配时任务函数内 recover + log warn 不 panic
- `go get github.com/robfig/cron/v3`

- [ ] **Step 3: 跑通 + Commit**

Run: `CGO_ENABLED=0 go test ./modules/starfuyu/... && CGO_ENABLED=0 go build ./...`
Expected: PASS

```bash
git add modules/starfuyu go.mod go.sum
git commit -m "feat(starfuyu): 定时任务（CRON_TZ=Asia/Shanghai，fx 起停）"
```

---

### Task 10: C 端 Controller + `/api/app/starfuyu` 路由

**Files:**
- Create: `modules/starfuyu/controller/app/app.go`
- Modify: `modules/starfuyu/module.go`（RegisterRoutes app 段 + fx Provide 控制器）
- Test: `modules/starfuyu/controller/app/app_test.go`

**Interfaces:**
- Consumes: Task 4–6 服务接口 + Task 5 `ProfileService`；`token.Validator`（main.go 已 Provide）；`resp.OK/Fail`；`errs`
- Produces: 全部 app 端点（spec §4.1 表）；`claims(ctx) (*token.Claims, bool)` 与 scorecard `controller/app/app.go:51` 同构（含 tenant 写回）

**组合逻辑（实现者勿自行发挥）：**
- `fortune/my/*`：`profiles.GetMy` → nil/`constellation==""` → **30505**；有则 `fortune.GetOne(date, constellation)`
- `fortune/image/:constellation/:date`：`images.Ensure` → 响应 `resp.OK(ctx, gin.H{"url": "/api/public/starfuyu/image/"+key})`
- `GET /profile` 未设星座：`resp.OK(ctx, gin.H{"constellation": ""})`（不报错）

路由（`RegisterRoutes` 入参 `/api` group）：

```go
app := r.Group("/app/starfuyu")
app.GET("/fortune/today", ...)
app.GET("/fortune/:date", ...)
app.GET("/fortune/my/today", ...)
app.GET("/fortune/my/:date", ...)
app.GET("/fortune/image/:constellation/:date", ...)
app.POST("/bazi/calculate", ...)
app.GET("/bazi/my", ...)
app.PUT("/bazi/my", ...)
app.GET("/divination/sign/:number", ...)   // 公开：不挂 claims
app.POST("/divination/draw", ...)
app.GET("/divination/history", ...)
app.GET("/profile", ...)
app.PUT("/profile", ...)
```

注意 gin 路由冲突：`/fortune/today` 与 `/fortune/:date` 共存照 goLucky 原路由（静态段优先）。

- [ ] **Step 1: 失败测试（鉴权门 + 关键路径）**

`app_test.go` 照 scorecard `app_test.go` 的 `stubValidator`：

```go
// 用例钉 Review Focus #4：
// - 无 Authorization → 401 + code 20001
// - Validate 失败 → 401 20001
// - UserType=admin → 403 20003
// - member 成功 → 200，且后续 handler 读到的 ctx tenant == claims.TenantID
// - GET /fortune/today 无数据 → 200 空列表
// - GET /divination/sign/42 免 token → 200（公开）
// - POST /bazi/calculate 超限 → 400 code 30503
```

Run: `CGO_ENABLED=0 go test ./modules/starfuyu/controller/app/ -v`
Expected: FAIL

- [ ] **Step 2: 实现 controller + module 路由**

- `NewAppController(...services, validator token.Validator)`；每个需鉴权 handler 第一行 `claims`
- 统一信封只经 `resp.OK/Fail`；业务错误直接 `resp.Fail(ctx, err)`
- `module.go`：fx.Provide `app.NewAppController`；`RegisterRoutes` 挂 app 段（**在模块开关判断之后**）

- [ ] **Step 3: 跑通 + Commit**

Run: `CGO_ENABLED=0 go test ./modules/starfuyu/...`
Expected: PASS

```bash
git add modules/starfuyu
git commit -m "feat(starfuyu): C 端 13 端点（member Bearer 自鉴权）"
```

---

### Task 11: Admin Controller + `/api/admin/starfuyu` + 公开图片路由

**Files:**
- Create: `modules/starfuyu/controller/admin/admin.go`
- Modify: `modules/starfuyu/module.go`（admin 段 + `/public/starfuyu/image/:key`）
- Test: `modules/starfuyu/controller/admin/admin_test.go`；`modules/starfuyu/module_test.go`（整模块路由）

**Interfaces:**
- Consumes: 全部服务；`storage.Driver`；Task 1 权限前缀（casbin 由全局中间件管，controller 不校验）
- Produces: admin 端点（spec §4.2）+ 公开端点（spec §4.3）

```go
// admin（不取 claims）
GET  /fortunes              // query: date, constellation, page, size
GET  /fortunes/:date
GET  /fortunes/:date/article
POST /fortunes/:date/upload-mp
POST /tasks/fetch-fortune   // body: {date?, force?}
POST /tasks/generate-images // body: {date?}
POST /tasks/execute-all     // body: {date?, force?}
GET  /limits
// public
GET  /public/starfuyu/image/:key  // driver.Open → inline；miss/err → 30010（文件不存在码，沿用 infra file 公开读口径）
```

- [ ] **Step 1: 失败测试**

- admin：`resp.Page` 形状、`upload-mp` 密钥空 → 30507、`fetch-fortune` force 语义透传
- `module_test.go`（照 fishgame）：
  - `cfg.Modules["starfuyu"]=false` → 三段全 404
  - `=true` → app 段 401 门、public image 200、admin 段无 token 401（全局 auth 中间件测试里可手工挂 `mw.Auth` 简化为「路由已注册」断言 401/403 而非 404）

Run: `CGO_ENABLED=0 go test ./modules/starfuyu/... -v`
Expected: FAIL

- [ ] **Step 2: 实现 + 装配**

- `NewAdminController(fortunes, tasks, articles, limits)`；`RegisterRoutes` 挂 admin 与 public
- public `:key` 用单段或 `*key` 防 `..` 穿越（filepath.Clean + 前缀校验）
- `module.go` 开关判断置于所有 group 创建之前

- [ ] **Step 3: 跑通 + Commit**

Run: `CGO_ENABLED=0 go test ./modules/starfuyu/... && CGO_ENABLED=0 go build ./...`
Expected: PASS

```bash
git add modules/starfuyu
git commit -m "feat(starfuyu): admin 8 端点 + 公开图片读 + 模块开关测试"
```

---

### Task 12: 收口（装配核验 / 删 goLucky / modules.md 回填）

**Files:**
- Verify: `alexgo-server/cmd/main.go` 含 `starfuyu.FxModule`（脚手架已插，核对即可）
- Delete: `modules/starfuyu/goLucky/`（整目录）
- Modify: `docs/modules.md`（§1.1 地图行、§9.1 前缀两行、新增 §13）
- Modify: 本文档与 spec 状态行（如需）

**Interfaces:**
- Consumes: Task 1–11 全部产物
- Produces: 可交付的完整模块；goLucky 源码仅存于 git 历史

- [ ] **Step 1: 全量验证**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud
CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go vet ./...
CGO_ENABLED=0 go test ./...
```
Expected: 全 PASS（若既有包有无关失败，记录并确认非本模块引入）

- [ ] **Step 2: 删除 goLucky**

```bash
git rm -r modules/starfuyu/goLucky
# 确认 luckyU 仍在：ls modules/starfuyu/luckyU
```

- [ ] **Step 3: 回填 docs/modules.md**

- §1.1 表加行：`starfuyu | modules/starfuyu | 星座运势/八字/求签 | /api/app/starfuyu、/api/admin/starfuyu、/api/public/starfuyu | 13+8+1 | 内容表 tid=0，用户表 tenant_id`
- §9.1 加两前缀行
- 文末新增 `## 13. starfuyu — 星座运势模块`：功能、app/admin/public 端点表、6 表、定时任务、错误码 305xx、与 goLucky 差异摘要（照 spec §10 浓缩）、明确「admin 页面 / luckyU 接入为后续票」

- [ ] **Step 4: Commit + 状态同步**

```bash
git add -A
git commit -m "feat(starfuyu): 模块收口——删 goLucky 旧码、modules.md 回填"
```

把 spec 与本计划头部状态改为「已实现」（可选，若仓库惯例是实现后改）。

---

## 任务依赖与提交节奏

```
Task 1 ─┬─ Task 2 ─ Task 3 ─┬─ Task 4 ─┬─ Task 5 ─ Task 6
        │                   │          ├─ Task 7 ─ Task 8
        │                   │          └─ Task 9（依赖 4+7）
        │                   └──────────┴─ Task 10（依赖 4/5/6）
        │                              Task 11（依赖 7/8/9/10）
        └────────────────────────────── Task 12（依赖全部）
```

每任务一步一验一提交；接口块是跨任务契约，实现者只看本任务 + 上游 Interfaces。
