# HTTP 基座规范化（子项目 A）设计

> 日期：2026-10-15
> 状态：已与用户逐节确认
> 范围：统一响应信封、业务错误码、统一分页、时间参数约定、访问/异常日志
> 不在范围：文件存储、读写分离、分布式锁、接口签名、报文加解密、验证码改造（分属子项目 B–E）

## 背景与目标

现状盘点（2026-10-15）：

- 返回信封三套并存：`{error}`×121、`{data}`×38、`{status:"ok"}`×30（`modules/**/controller`）
- 分页 3 处手写 `Offset((page-1)*size).Limit(size)`（member/codegen/oauth2 client repo），无 size 上限、无统一封装
- 无时间参数解析约定；无 form/query 时间字段；JSON 时间走 RFC3339 默认
- 访问日志仅 method+path（`pkg/middleware/middleware.go:Logger()`），无状态码/耗时/trace_id
- panic 走 `gin.Recovery()` 默认输出（非结构化、无统一错误信封）

目标：一次收敛上述五项，形成「以后写接口只会写对一种样子」的基座。前端
`apps/web-antd/src/api/request.ts` 已支持双信封（`code === undefined || code === 0` 均视为成功），
迁移窗口内不打断页面；全迁完成后收紧为 `code === 0`。

## 决策记录（用户逐项确认）

| 决策点 | 选择 |
|---|---|
| 迁移范围 | 存量全迁（~190 处 `ctx.JSON`） |
| 错误码 | 五位分段（1xxxx 参数 / 2xxxx 认证 / 3xxxx 业务 / 5xxxx 系统） |
| 分页 | 统一信封 + `Paginate[T]` 泛型 helper + size≤100 |
| 时间参数 | `types.DateTime` 宽松解析，不新增时间范围筛选（YAGNI） |
| 日志 | 结构化访问日志补全 + 自定义 Recovery，不入库 |
| 错误传递架构 | 方案一：显式 `resp` helper（排除全局错误中间件方案） |

## 1. 统一响应信封

三种形态，HTTP status 与 body.code 并存（status 继续驱动前端拦截器 rejected 链，401 刷新 token 逻辑不变）：

```jsonc
// 单对象 / 动作成功  HTTP 200
{ "code": 0, "msg": "ok", "data": { ... } }

// 无返回值动作（原 {"status":"ok"}）  HTTP 200
{ "code": 0, "msg": "ok", "data": null }

// 分页列表  HTTP 200
{ "code": 0, "msg": "ok", "data": { "list": [...], "total": 123, "page": 1, "size": 10 } }

// 失败  HTTP status 按码段映射
{ "code": 30001, "msg": "用户名已存在", "data": null }
```

HTTP status 映射（`resp.Fail` 内部推导）：

| 码段 | 含义 | HTTP status |
|---|---|---|
| `1xxxx` | 参数/校验 | 400 |
| `2xxxx` | 认证授权 | 401（`20003` 无权限 → 403） |
| `3xxxx` | 业务规则 | 400 |
| `5xxxx` | 系统内部 | 500 |

helper（`pkg/resp`）：

- `resp.OK(c, data)` — 无 data 动作传 `nil`
- `resp.Page(c, list, total, pageReq)` — 分页信封；pageReq 须是 Defaults() 之后的值
- `resp.Fail(c, err)` — `errors.As` 提取 `*BizError` 得 code/msg；非 `*BizError` 一律 50000 + `zap.Error` 落日志（原始 err 只进日志，不外泄）

controller 迁移样例：

```go
if err := ctx.ShouldBindJSON(&req); err != nil {
    resp.Fail(ctx, errors.New(errs.CodeParamInvalid, err))
    return
}
req.PageReq.Defaults()
items, total, err := svc.List(ctx, req.PageReq)
if err != nil { resp.Fail(ctx, err); return }
resp.Page(ctx, items, total, req.PageReq)
```

### 豁免清单（不套业务信封）

- `modules/system/oauth2/controller/protocol.go` 全部端点 — OAuth2 RFC 6749 响应体
- `/health`、`/health/ready`、`/metrics`、`/debug/pprof/**`

## 2. 错误码注册表（pkg/errors 扩展）

现有 `pkg/errors/errors.go` 保留，新增 code 体系：

```go
type Code int

const (
    // 1xxxx 参数
    CodeParamInvalid Code = 10001 // 参数校验失败
    CodeParamMissing Code = 10002 // 必填参数缺失
    // 2xxxx 认证授权
    CodeUnauthorized Code = 20001
    CodeTokenExpired Code = 20002
    CodeForbidden    Code = 20003
    // 3xxxx 业务 — 首批只注册现有 controller 出现过的语义，
    // 迁移中遇到新语义逐个补（如 CodeUsernameTaken=30001、CodeUserNotFound=30002、
    // CodeRoleTaken=30003、CodeTenantDisabled=30004、CodeCaptchaInvalid=30005 …）
    // 5xxxx 系统
    CodeInternal Code = 50000 // 兜底
)

type BizError struct {
    Code  Code
    cause error // 可为 nil
}

func New(code Code, cause error) *BizError // msg 从注册表查；查不到用占位文案
func (e *BizError) Msg() string            // 注册表中文文案，写死代码里，不做 i18n
func Is(err error, code Code) bool
```

- 注册表为 `map[Code]string`；单测钉住「码无重复、每个导出 Code 有文案」
- service 返回 `errors.New(errs.CodeXxx, nil)`；底层系统错误 `fmt.Errorf("...: %w", err)` 透传，
  `resp.Fail` 兜底 50000 并落日志
- gin 校验错误细节（字段名等）保留在 cause 进日志，前端 msg 只有「参数校验失败」

## 3. 包布局与依赖方向

```
pkg/types      DateTime（无依赖）
pkg/errors     +Code/BizError/注册表（标准库 only）
pkg/resp       OK/Page/Fail + PageReq ← pkg/errors（gin）
pkg/repo       Paginate[T]（gorm）
pkg/middleware AccessLog、Recovery（zap/gin，依赖 resp/errors）
```

只向下依赖，无环。

### types.DateTime

- `UnmarshalJSON` / `UnmarshalParam` 宽松解析，依次尝试
  `2006-01-02 15:04:05` → `2006-01-02` → RFC3339
- `MarshalJSON` 输出 `2006-01-02 15:04:05`（本地时区，与 MySQL `parseTime=True&loc=Local` 一致）
- 本期只定约定 + 提供类型；现有接口没有时间过滤参数，不新增（YAGNI）

## 4. 统一分页

```go
// pkg/resp
type PageReq struct {
    Page int `form:"page" binding:"omitempty,min=1"`
    Size int `form:"size" binding:"omitempty,min=1,max=100"`
}
func (p *PageReq) Defaults() // 缺省 page=1, size=10

// pkg/repo
func Paginate[T any](ctx context.Context, q *gorm.DB, page, size int) ([]T, int64, error)
// Count + Offset + Limit + Find 一次完成；Where/Order 由调用方预先链好，
// Paginate 不注入默认 Order（避免掩盖调用方排序意图）；page/size 需已过 Defaults()
```

- 存量 3 处手写分页全部改用 `Paginate[T]`
- size 上限 100 由 binding 拦截，防大页拖库
- 调用约定：controller 绑定后先 `req.PageReq.Defaults()`，再把结果传给 service → repo 与 `resp.Page`

## 5. 日志

### AccessLog（替换 `middleware.Logger()`）

挂载链尾（OTEL 之后），保证 trace_id 已注入、auth 已执行：

```
Recovery → Tenant → Prometheus → RateLimit → Auth → OperateLog → OTEL → AccessLog
```

- 字段：`method / path / query(截断 2KB) / status / latency_ms / client_ip / user_id / tenant_id / trace_id`
- 级别：5xx→`zap.Error`，4xx→`zap.Warn`，其余 `zap.Info`
- 排除：`/health`、`/health/ready`、`/metrics`、`/debug/pprof/**`（与 OperateLog 同口径）
- 不入库（运营审计已有 `operate_logs`；access 入库暂无消费方，YAGNI）

### 自定义 Recovery（替换 `gin.Recovery()`）

- `recover()` → `zap.Error` 带 `debug.Stack()` → 回 `resp.Fail` 50000 信封

## 6. 前端适配（同一子项目）

- `apps/web-antd/src/api/request.ts`：全迁完成后 `successCode` 收紧为 `code === 0`
  （迁移窗口内保持 `code === undefined || code === 0` 双兼容）
- 列表页改吃 `data.list` / `data.total`；废掉「拉满 200 本地切页」workaround
  （`api/admin.ts:374`、`api/infra.ts:6` 注释所指问题）
- 所有读 `{status:"ok"}` 的调用点改判 void
- oauth2 登录链路（captcha/登录）走独立 client，预计零改动，验证即可

## 7. 迁移策略（6 步，每步单独提交、测试全绿，直接落 main）

| 步 | 内容 |
|---|---|
| 1 | pkg 基建：`types.DateTime`、`errors` 码表、`resp`、`repo.Paginate`、AccessLog、Recovery + 各自单测（不动存量） |
| 2 | system 模块 controller 全迁（~120 处）+ repo 分页改造 |
| 3 | member + order 模块迁移（~40 处） |
| 4 | 前端：successCode 收紧、列表页 `.list` 适配、废 200 workaround |
| 5 | oauth2 前端登录链路验证（预期零改动） |
| 6 | 回归：`CGO_ENABLED=0 go test ./...` + `pnpm typecheck` + 冒烟（登录/用户列表/创建用户/翻页） |

每步同步更新 `docs/roadmap.md`（新增 P1-5「HTTP 基座规范化」并随进度打勾）。

## 8. 测试

- `resp`：code→status 映射表驱动测试；非 BizError 兜底 50000；Page 信封形状
- `errors`：注册表无重复、导出码全有文案
- `DateTime`：三种格式解析 + 非法输入拒绝 + 序列化格式
- `Paginate`：glebarez/sqlite 内存库跑真实 SQL（项目已有先例）
- AccessLog/Recovery：httptest + 断言字段与信封
- 存量断言响应体的单测随模块迁移同步改信封断言
- 前端 vitest（如有涉及拦截器的用例）+ `pnpm typecheck`

## 非目标

- 时间范围筛选参数（等有真实需求再加）
- access_logs 入库
- i18n 错误文案
- 游标分页
- 全局错误中间件（error-as-return 改造）——明确选了显式 helper
