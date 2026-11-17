# Job Manager 设计（pkg/job 统一模块任务设施）

日期：2026-11-16
状态：待评审
方案：方案 2（fx 单例 + 注册制 Manager，用户已选定）

## 1. 背景与动机

项目里后台任务已有三种写法，前两种是复制粘贴的 fx 生命周期样板：

| 现状 | 位置 | 问题 |
|---|---|---|
| `fx.Invoke` + `time.NewTicker` 手写 goroutine | `modules/system/module.go:102`（token 清扫，每小时）、`modules/scorecard/module.go:93`（超时关房，每分钟） | runCtx/OnStart ctx 坑、错误只 warn、`logger.Log` nil 门控——每处重写一遍 |
| `fx OnStart/OnStop` + robfig/cron | `modules/starfuyu/cron.go`（抓运势 + 生图两任务） | parser/CRON_TZ/safeJob 模块开关守卫全是模块私有，新模块要整份照抄 |
| 手动补跑各写各的 | `modules/starfuyu/controller/admin` 的 `/tasks/*` 三端点 + `service/tasks.go` TaskRunner | 每加一个模块就要加一套任务端点与权限前缀 |

目标：**每个模块写 job 只剩一行注册**——起停、时区、panic 恢复、模块开关、配置改期、手动补跑全部收敛到 `pkg/job`。

## 2. 目标 / 非目标

**目标**
- 模块内一行 `m.Register(name, 默认表达式, fn)` 即得到定时任务。
- cron 表达式进 `config.yaml` 的统一 `jobs:` 段，运维改时刻不改代码。
- 统一 admin API：列表（含下次/上次执行状态）+ 手动触发。
- 收编三处样板：starfuyu cron、system ticker、scorecard ticker 全部迁到 Manager。
- 统一安全外壳：panic recover 成 warn 不崩进程、错误只 warn 不致命、表达式非法启动拒启。

**非目标（明确不做）**
- 不做 DB 持久化执行历史、重试/死信、错过补跑策略（方案 3 范围，无需求）。
- 不做前端管理页面（本期只有 API；菜单/页面后续另起）。
- 不做分布式锁/多实例去重——现状所有 cron 都是每实例各跑一遍，本设计**维持现状**（单副本假设），多副本去重留给 outbox 式 SKIP LOCKED 的未来需求。
- 不改 starfuyu 带参手动端点（`/tasks/*` 的 date/force 参数无法被通用 trigger 表达，保留原样）。

## 3. 现状约束（设计依据）

- fx 装配：`alexgo-server/cmd/main.go:204` 汇总各模块 `FxModule`；`OnStart` 的 ctx 启动完成即 cancel，后台 goroutine 必须用独立 runCtx（system/scorecard 注释已钉）。
- cron 惯例（starfuyu）：5 段标准 parser（**不 WithSeconds**）、`CRON_TZ=Asia/Shanghai` 前缀、`time/tzdata` 进程级嵌入（`modules/starfuyu/cron.go:7`）、`safeJob` panic recover、模块开关 `cfg.Modules[name]=false` 时**在解析表达式之前** return（`cron_test.go:64` 用非法表达式钉住顺序）。
- admin 鉴权：casbin 按 `permissionRoutes` 前缀（`modules/system/service/permission_routes.go`）+ `keyMatch2`；handler 不取 claims；新路由必须同步登记权限键。
- 路由挂载：模块实现 `server.RegisterRoutes(r *gin.RouterGroup)`（入参 `/api` group），admin 段前缀 `/admin/<module>`。
- 配置：`pkg/config/config.go` 全局 `Config` struct + `cfg.Modules map[string]bool` 模块开关。

## 4. 设计

### 4.1 包结构

```
pkg/job/
  manager.go       Manager：注册表 + fx 生命周期 + robfig/cron 起停
  spec.go          表达式解析（5 段 parser + CRON_TZ 前缀 + tzdata）
  status.go        执行状态跟踪（lastStart/lastEnd/lastErr/running/触发计数）
  manager_test.go  生命周期/开关/配置覆盖/触发 测试
  spec_test.go
```

依赖方向：`pkg/job` 只依赖 `pkg/config`、`pkg/logger`、robfig/cron、fx。**不 import `alexgo-server/server`**（pkg 层不反向依赖装配层）。

### 4.2 Manager API

```go
package job

// Fn 任务体。error 只记 warn，不影响其他任务。
type Fn func(ctx context.Context) error

type Manager struct { ... }

// Register 注册任务。name 约定 "模块.任务名"（如 "starfuyu.fetch_fortune"），
// 模块段用于 cfg.Modules 开关判定。必须在 fx.Invoke 阶段调用（OnStart 之前）；
// 重复 name panic（开发期错误尽早炸）。
func (m *Manager) Register(name, defaultSpec string, fn Fn)

// 列表/触发供 admin API 用。
func (m *Manager) List() []Status
func (m *Manager) Trigger(ctx context.Context, name string) error  // 未注册 → errs 305xx 系
```

fx 装配（`job.FxModule`，挂进 main.go options）：

```go
var FxModule = fx.Module("job",
    fx.Provide(NewManager),
    fx.Invoke(bindLifecycle),   // OnStart: 解析并 AddFunc 全部已注册任务；OnStop: c.Stop() 等收尾
)
```

生命周期语义（照抄 starfuyu 已验证的顺序）：
1. `OnStart`：模块开关关 → 跳过该模块全部任务（**先于表达式解析**）；`cfg.Jobs[name]` 有值用配置值，缺省用 `Register` 的 defaultSpec；解析失败 → 返回 error 拒启。
2. `OnStart`：`c.Start()`；任务体统一包 `safeJob`（panic recover → warn 日志 + 状态记 lastErr，不崩进程）。
3. `OnStop`：`select { <-c.Stop().Done() / <-ctx.Done() }` 等在跑任务收尾，ctx 截断防拖死关停。
4. 扫描/任务 ctx：`context.Background()`（cron 无 ctx；手动触发传 request ctx）。

### 4.3 配置

`pkg/config/config.go` 新增：

```go
// Jobs：job 表达式覆盖表，key = "模块.任务名"。
// 缺键 → 用代码 Register 时的 defaultSpec；显式空串 → 停用该任务（运维开关）；
// 未知 key（未注册的名字）→ 启动时 warn，不报错。
Jobs map[string]string `mapstructure:"jobs"`
```

`config.yaml`（带注释的示范段）：

```yaml
jobs:
  system.sweep_tokens:  "@every 1h"   # 缺省即此值，写出仅为示范
  scorecard.sweep_rooms: "@every 1m"
  starfuyu.fetch_fortune: "10 0 * * *"
  starfuyu.generate_images: "20 0 * * *"
```

- 时区：非 `@every` 表达式统一加 `CRON_TZ=Asia/Shanghai` 前缀（沿用 `service.CronSpec` 语义，迁入 `pkg/job` 成为唯一实现）。
- `@every`/`@hourly` 等 descriptor 由 5 段 parser 的 `Descriptor` 位支持（starfuyu 现有 parser 已开）。

### 4.4 Admin API（挂 system 模块）

system 是平台模块，由它注入 `*job.Manager` 并注册两条路由（`modules/system/module.go` 的 RegisterRoutes）：

| 方法 | 路径 | 行为 |
|---|---|---|
| GET | `/api/admin/jobs` | 返回全部已注册任务：name、生效表达式、module、running、lastStart/lastEnd/lastErr、下次执行时间（cron.Entry.Next） |
| POST | `/api/admin/jobs/:name/trigger` | 同步执行一次任务体（request ctx），返回成功/失败；任务名未注册 → 参数错误 |

- 权限键：`"system:job": {"/api/admin/jobs", "/api/admin/jobs/*"}` 登记进 `permissionRoutes`。
- **权限落地（评审 Q1，已核实）**：casbin 策略由菜单 `permission` 经 `rebuildRolePolicies` 重建，无 superadmin 通配绕过——只登记 `permissionRoutes` 会全角色 403。解法照 `fileStorageButtons` 既有模式（`modules/system/service/seed.go:437`）：seed 追加两条**按钮型菜单** `system:job:read` / `system:job:trigger`（无前端页面，type=button），进 `allMenus` → `SetRoleMenus` 绑 admin 种子角色 → `RebuildPolicies` 自动获得策略。新增任务 3.5。
- 响应走统一信封 `resp.OK/resp.Fail`；handler 不取 claims（casbin 全局中间件已鉴权，与 starfuyu admin 同构）。
- **停用任务语义（评审 Q2，已定）**：`List` 返回**全部注册任务**，含 `enabled` 布尔与 `disabledReason`（`module_off` / `spec_empty`）；`Trigger` 对停用任务**拒绝**（参数错误）——防止 `modules.starfuyu: false` 时仍能手动打 DeepSeek。
- **trigger 同步语义（评审 Q4，已核实）**：starfuyu 现有 `/tasks/*` 即同步返回，`http.Server` 无 WriteTimeout（`alexgo-server/server/http.go:171`），同步 trigger 与现状同构，不改异步。
- 触发并发语义：**允许并发**（不加锁），状态表只记最近一次的起止；`running` 布尔反映"至少一个在跑"。理由：任务体自持幂等语义（与现状 cron 重叠跑一致），加锁反而让长任务阻塞管理端。
- starfuyu 原 `/tasks/*` 三端点**保留**（date/force 带参语义不可通用化），但其 cron 调度面迁到 Manager。

### 4.5 模块内注册写法（新模块模板）

```go
// modules/<mod>/module.go 或 module.go 的 fx.Invoke
fx.Invoke(func(m *job.Manager, r TaskRunner) {
    m.Register("<mod>.do_thing", "*/10 * * * *", func(ctx context.Context) error {
        return r.DoToday(ctx)
    })
})
```

ticker 型任务直接写 `"@every 1m"`，不再手写 goroutine。

### 4.6 迁移（三个现有任务面全部收编）

| 任务 | 现状 | 迁后 |
|---|---|---|
| system token 清扫 | module.go 手写 ticker goroutine | `m.Register("system.sweep_tokens", "@every 1h", ...)`，删手写 goroutine |
| scorecard 超时关房 | module.go 手写 ticker goroutine | `m.Register("scorecard.sweep_rooms", "@every 1m", ...)`，删手写 goroutine |
| starfuyu 抓取+生图 | cron.go `cronLifecycle` | `m.Register("starfuyu.fetch_fortune", "10 0 * * *", ...)` ×2，**删 cron.go**；`service.CronSpec` 迁 `pkg/job`；cron_test 三件（parser 钩子/开关守卫/非法表达式）随迁改写 |

- **行为变化（评审 Q3，已定）**：system/scorecard 两个旧 ticker **不查** `cfg.Modules` 开关，迁后统一受模块开关守卫——`modules.scorecard: false` 会连带停掉超时关房清扫。裁定：**接受此变化**（模块关 = 路由不注册 = 无新房间，清扫停摆语义一致；当前 `config.yaml` 全 true，无实际影响）。plan 中标注。
- starfuyu `skipFetch`（空 APIKey 跳过）留在任务体闭包内，语义不变。
- `cfg.Starfuyu.Cron.{FetchFortune,GenerateImages}` 字段删除，值迁 `cfg.Jobs`；config.yaml 同步改写（旧键忽略，无兼容包袱——项目未发布）。
- system/scorecard 的 `SweepExpired` service 方法不动，只换调度外壳。

### 4.7 错误与边界

| 场景 | 行为 |
|---|---|
| 表达式非法 | OnStart 返回 error，进程拒启（运维可见） |
| 模块开关 false | 该模块任务跳过注册（不解析表达式），其余模块照常 |
| `cfg.Jobs` 空串 | 该任务停用（相当于运维关闭，不报错） |
| `cfg.Jobs` 未知 key | 启动 warn（配置写错名可见），不影响启动 |
| 任务 panic | safeJob recover → warn + 状态记错，进程与其余任务不受影响 |
| 任务返回 error | warn 日志 + 状态记错，下轮照常跑 |
| trigger 未注册 name | `errs.CodeParamInvalid` |
| trigger 时任务已在跑 | 允许并发，照常执行 |

## 5. 测试计划

- `pkg/job` 单测（fxtest lifecycle，风格照 `starfuyu/cron_test.go`）：
  - 开关关 + 非法表达式 → Start 不报错（守卫先于解析，钉顺序）；
  - 开关开 + 非法表达式 → Start 报错；
  - `cfg.Jobs` 覆盖 defaultSpec、空串停用、未知 key 仅 warn；
  - `Trigger` 正常执行/未注册报错/panic 被 recover 不外溢；
  - `List` 状态字段（running/lastErr）正确。
- 迁移回归：system/scorecard 原有测试全绿（若无 ticker 测试则补 `Register+"@every"` 冒烟）；starfuyu `cron_test` 改写后绿；`module_test` 路由测试补 `/api/admin/jobs` 两条。
- admin 路由：system module_test 补 GET/POST 用例 + 未带 token 401（casbin 中间件路径）。

## 6. 实施任务切分（供 plan 阶段展开）

1. `pkg/job` 骨架：spec 解析 + Manager 注册表 + 生命周期 + safeJob + 状态跟踪（含测试）。
2. `pkg/config`：`Jobs` 字段 + config.yaml 示范段。
3. `job.FxModule` 挂 main.go + system 模块注册 admin 两条路由 + `permissionRoutes` 加 `system:job` + seed 按钮菜单两条 `system:job:{read,trigger}`（含测试；新测试文件独立建，system 无既有 RegisterRoutes 测试受波及——评审 Q5 已核实）。
4. 迁移 starfuyu：删 cron.go、注册两任务、CronSpec 迁走、cron_test 改写。
5. 迁移 system + scorecard ticker → `@every`，删两处手写 goroutine（行为变化见 §4.6）。
6. 全量回归：`go test ./...` + lint。

## 7. 留给实施者的判断

- `errs` 错误码选段（305xx starfuyu 已占用邻域，选不冲突的段）。
- `Status` JSON 字段命名与 swagger 注解细节。
- system RegisterRoutes 注入 `*job.Manager` 的方式（`optional:"true"` 还是必填——建议必填，FxModule 恒挂）。
- 各测试文件的 fixture 组织（照各模块现有风格）。
