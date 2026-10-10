# alexGo-cloud 架构说明

> 本文依据当前代码整理，是理解项目结构、装配方式与运维入口的单一参考。
> 与实现不一致时，以代码为准。

## 1. 定位

alexGo-cloud 是一个**模块化单体（Modular Monolith）** Go 后台基座，对标 yudao-cloud 的思路：
模块独立开发、单进程统一启动、按需把单个模块拆成微服务。核心抽象是 `server.Module` 接口
（`alexgo-server/server/http.go`）——每个业务模块实现 `RegisterRoutes(*gin.RouterGroup)`，
把自己的路由挂到统一的 `/api` 组下；单体模式下模块之间通过 Go 接口直接调用，零 RPC 开销。

技术栈：**Go 1.25 + Gin + Uber FX（依赖注入）+ GORM(MySQL) + Casbin(RBAC) + JWT +
Redis(Token Bucket 限流) + NATS JetStream(消息) + OpenTelemetry + Prometheus**。

## 2. 目录结构

| 目录 | 职责 |
| --- | --- |
| `alexgo-server/` | 单体进程入口与 HTTP 层：`cmd/main.go` 统一启动装配，`configs/` 全局配置，`server/` Gin 路由、中间件链与 `server.Module` 抽象 |
| `modules/` | 业务模块，当前有 `system`（用户/角色/菜单/字典/部门/公告/审计/租户等）、`order`（示例业务）与 `member`（会员注册/登录，micro 形态独立成 `cmd/` 入口）；内部按 `api / controller / model / repository / service` 分层 |
| `pkg/` | 与业务无关的基础设施与横切能力：`config`、`database`、`migrate`、`middleware`、`auth`(JWT/Casbin)、`limiter`、`circuitbreaker`、`outbox`、`mq`、`redis`、`tenant`、`audit`、`monitor`、`trace`、`logger`、`client`(gRPC)、`errors` 等 |
| `apps/web-antd/` | 管理后台前端（vendored vue-vben-admin v5.7.0，Ant Design Vue），经 `/api/**` 调用后端 |
| `deployments/` | 交付物：`docker-compose/`（本地）、`kubernetes/`（kustomize 清单）、`helm/`（多环境 values）、`argocd/`（GitOps Application） |
| `scripts/` | 工程脚本：proto 生成（`gen_proto.sh`）、CRUD 生成器、k6 压测（`load_test.js`）、代码模板 |
| `tools/dbgen` | 按数据表生成 CRUD 代码（`make generate-db-crud`） |

模块内部补充：`modules/system` 额外持有 `migrations/`（golang-migrate SQL 迁移）、`grpcserver/`
与 `cmd/grpc_main.go`（独立 gRPC 进程入口）、`configs/config.yaml`（模块级配置，如 `jwt_secret`）；
`modules/order` 为最小示例模块（repository/service/controller 三层）。

## 3. 启动与依赖装配（Uber FX）

入口是 `alexgo-server/cmd/main.go`。`main` 解析 `--migrate-only` flag、初始化全局 Zap logger，
然后按“基础设施 → 横切副作用 → 模块”的顺序组装 fx 选项：

1. **`fx.Provide`（声明怎么构造）**：`config.LoadGlobalConfig` → `database.NewDB`（GORM +
   连接池 + Fx OnStop 优雅关闭）→ `client.NewGRPCConn` → `auth.NewCasbinEnforcer` →
   `appredis.NewClient` → `limiter.NewRateLimiter` → 熔断器（`breaker.enabled=false` 时返回 nil）→
   `mq.NewNATSBroker` → `outbox.NewRelay` → `migrate.NewRunner`（经 `fx.Annotate` +
   `group:"migration_sources"` 聚合各模块提交的迁移源）。
2. **`fx.Invoke`（声明启动时做什么）**：`outbox.StartRelay`（后台轮询 goroutine，由
   `outbox.enabled` 控制是否真正启动）→ `trace.InitTracer`（OTel TracerProvider）→
   `migrate.Runner.Run`（`migrate.auto=false` 时整体跳过）。
3. **`fx.Decorate`（替换最终实现）**：见第 4 节。
4. **模块聚合**：`system.FxModule`、`order.FxModule` 通过
   `fx.Annotate(..., fx.As(new(server.Module)), fx.ResultTags('group:"modules"'))` 把自己注册进
   `group:"modules"`；`server.StartHTTPServer` 的 `HTTPServerParams` 收到该 group 后，在
   `/api` 组上按序调用每个模块的 `RegisterRoutes`。
5. 仅当**非** `--migrate-only` 时才追加 `fx.Invoke(server.StartHTTPServer)`；最后
   `fx.New(opts...).Run()` 阻塞运行直至收到退出信号，由 FX Lifecycle 完成 HTTP/DB 的优雅关闭。

`--migrate-only`：跳过 HTTP Server 装配，只做迁移与后台初始化。仓库当前**没有** K8s Job
清单，它用于发布前手动执行（`make migrate` 或直接跑二进制），未来也可挂成 init Job。

## 4. 单体 → 微服务切换

> 服务间调用的完整说明（双形态对照、gRPC 通路、地址解析、异步事件、consul 现状）
> 见 `docs/service-communication.md`。本节只保留装配层要点。

`cmd/main.go` 用 `fx.Decorate` 在依赖图构建完成后替换 `systemservice.UserService` 的最终实现：

- `microservice.enabled=false`（默认）：直接返回本地实现，跨模块调用零开销；
- `microservice.enabled=true`：返回 `client.NewUserGRPCClient(conn)`——同一 Go 接口的 gRPC
  客户端实现，所有注入该接口的调用方**零改动**；若此时 gRPC 连接为 nil 则直接返回错误，
  避免“配置说微服务、运行时却是单体”的不一致。

配套入口：`modules/system/cmd/grpc_main.go` 提供 system 模块的独立进程化装配
（`system.FxModule` + `grpcserver.StartGRPCServer`）。拆分只换装配，不改调用方代码。

**双运行模式（`deployment.mode`）**：`mono`（默认）时 `cmd/main.go` 额外装配 `member.FxModule`，
system+order+member 单进程跑在 :8080；`micro` 时本进程不装 member 路由，会员侧由
`modules/member/cmd/main.go` 独立启动（:8081），其 Token 签发经 gRPC `TokenService`
（system 侧 :50051，`StartGRPCServer` 内部按 `deployment.mode` 门控，mono 直接 return）委托完成。
gRPC 连接拨号条件是 `deployment.mode=micro` **或** `microservice.enabled` 任一开启
（地址 `system_grpc_addr` 优先，见 `pkg/client.GetServiceAddress`）。

## 5. HTTP 中间件链（顺序即优先级）

`alexgo-server/server/http.go` 的 `newRouter` 用 `gin.New()` 按下列顺序挂全局中间件
（先注册先执行，外层 → 内层；顺序即优先级）：

1. `middleware.Recovery()` — 捕获 panic，对外返回 500，不打挂进程；
2. `middleware.NewTenantMiddleware(domainLookup)` — 解析租户并注入上下文：`X-Tenant-ID` 请求头
   优先，其次 Host 域名（`tenant.DomainLookup`，nil 时只认请求头）；是多租户隔离、审计与限流
   维度的基础；
3. `middleware.Logger()` — 结构化请求日志；
4. `monitor.PrometheusMiddleware()` — 记录 `http_requests_total`（method/path/status）等指标；
5. `middleware.RateLimitAndBreaker(cfg, limiter, breaker)` — Redis Token Bucket 限流 + 熔断的
   入口防护；limiter/breaker 为 nil（对应 `limiter.enabled` / `breaker.enabled` 关闭）时优雅放行；
6. `middleware.NewAuthMiddleware(AuthDeps{Cfg, Enforcer, Validator, ScopeLoader})` — **token/jwt
   双模式**鉴权（`pkg/middleware/auth.go`）：
   - **模式**：`auth.mode=token`（默认）用不透明令牌经 `token.Validator` 查库校验；Validator 未装配
     属装配错误，直接 401（fail-closed，绝不放行）。`auth.mode=jwt` 走旧静态 JWT 解析，作为回滚开关。
   - **作用范围**：`/api/admin/**` 全量 + app 端仅 `/api/app/member/auth/logout|refresh`
     （登录、注册等入口公开）；`/health`、`/health/ready`、`/metrics`、`/debug/pprof/` 直接放行。
   - **Casbin sub 前缀**：用户主体为 `{tenant}:{user}`（用户名为空退化为 `{tenant}:{userId}`）；
     角色策略主体另占独立命名空间 `{tenant}:role:{code}`（`modules/system/service/permission_routes.go`
     的 `roleSub`），防止跨租户同名串策略、也防止成员自注册昵称撞角色 sub 提权。
   - **data_scope 注入**：仅管理端用户经 `tenant.ScopeLoader` 计算数据权限档写入 ctx（加载失败按
     Mode 5「仅本人」最严兜底、只告警不放行更多数据）；member 用户与未装配（ScopeLoader=nil）时不注入，
     GORM 插件只剩租户隔离，保持安全缺省。
7. `middleware.OperateLogMiddleware(recorder)` — 操作审计，同样只作用于 `/api/admin/**`
   （recorder 为 nil 时不记录）；
8. `trace.OTELMiddleware()` — OpenTelemetry（otelgin）链路追踪，可导出 Jaeger/Tempo/OTLP Collector；
9. `middleware.Trace()` — trace 信息补全的占位。

模块路由统一挂在 `/api` 组（`r.Group("/api")` 后遍历 `Modules` 调用 `RegisterRoutes`）。

**运维端点**（不经模块注册，直接挂在 Engine 上）：

| 端点 | 用途 | 行为 |
| --- | --- | --- |
| `GET /health` | liveness 探针 | 不依赖 DB/Redis/NATS，进程活着即 200 |
| `GET /health/ready` | readiness 探针 | ping DB（`PingContext`，**2s 超时**）；失败返回 **503** 供 K8s 摘流；DB 未配置（nil）时返回 200 `db=unconfigured` |
| `GET /metrics` | Prometheus 抓取 | `promhttp.Handler()` |
| `/debug/pprof/**` | 性能分析 | **仅 `server.pprof_enabled=true` 时挂载**（`/`、`cmdline`、`profile`、`symbol`、`trace`）；代码默认 `false`（`pkg/config/loader.go` 的 `SetDefault`），本地 `alexgo-server/configs/config.yaml` 为便于调试开启，Helm `values.yaml` 为 `false`（`values-dev.yaml` 为 `true`） |

约定：livenessProbe 用 `/health`、readinessProbe 用 `/health/ready`，避免 DB 故障触发全员重启
（kustomize 与 Helm 清单均已按此配置）。pprof 默认关闭，防止生产经业务端口泄露运行时信息。

## 6. 可靠事件：Transactional Outbox

解决“业务已提交但事件丢失”的一致性问题，链路如下：

1. 业务事务内同时写业务表与 `outbox_events`（模型 `modules/system/model/outbox.go`，初始
   `status=pending`），两者原子提交；
2. `pkg/outbox` 的 Relay 在 Fx OnStart 起后台 goroutine（`outbox.enabled=true` 才启动；
   broker 为 nil 时 `processPending` 直接 no-op），按 `outbox.interval_second`（**默认 5s**）
   tick，每轮最多抓 `outbox.batch_size`（默认 100）条；
3. 单事务内
   `SELECT * FROM outbox_events WHERE status = 'pending' ORDER BY id LIMIT ? FOR UPDATE SKIP LOCKED`
   抢占事件——多实例 Relay 可并行运行而不重复处理同一行；
4. 逐条发布到 `mq.Broker`（当前实现为 NATS JetStream，由 `mq.nats.enabled` 控制），成功标记
   `published` 并写 `published_at`，失败标记 `failed` 且不阻塞后续事件。

抓取与发布在同一事务内完成，以事务 + `FOR UPDATE SKIP LOCKED` 实现互斥；代码注释中也写明了
演进方向（pending → processing + worker_id、失败重试、死信等）。

## 7. 配置体系

入口为 `pkg/config/loader.go` 的 `LoadGlobalConfig`，产出统一的 `*config.Config`
（结构定义见 `pkg/config/config.go`）。合并优先级（从高到低）：

1. **环境变量显式覆盖（最终生效）**：`v.Unmarshal(&cfg)` **之后**调用
   `applyEnvOverrides`，把 `DB_DSN` → `database.dsn`、`JWT_SECRET` → `system.jwt_secret`、
   `REDIS_PASSWORD` → `redis.password`、`DEPLOYMENT_MODE` → `deployment.mode`、
   `SYSTEM_GRPC_ADDR` → `system_grpc_addr`（全部仅非空时覆盖）。必须在这一步做显式覆盖的原因见函数
   注释：没有显式绑定且名字对不上（AutomaticEnv 按 key 转写查的是 `DATABASE_DSN` 而非
   `DB_DSN`），且模块配置合并用的 `v.Set()` 在 viper 中优先级高于 env——不经这一步，
   密钥 env 永远输给模块 yaml。
2. **模块 yaml（`v.Set` 命名空间合并）**：读入全局配置后检查 `modules.<name>` 开关，为 true 的
   模块加载 `modules/<name>/configs/config.yaml`，并以 `模块名.` 为前缀 `v.Set` 合并
   （如 `modules/system/configs/config.yaml` 的 `jwt_secret` → `system.jwt_secret`）。
3. **全局 `alexgo-server/configs/config.yaml`**；另有显式 `BindEnv` 的常用别名映射
   `HTTP_ADDR` / `GRPC_ADDR` / `NATS_URL` / `REDIS_ADDR`（→ `server.http_addr` /
   `mq.nats.url` / `redis.addr`），按 viper 原生优先级高于配置文件。注意区分：
   `EnvKeyReplacer`（`.`→`_`）服务于 `AutomaticEnv` 的 **`SERVER_HTTP_ADDR` 这类
   “viper key 转写”** 的隐式 env 查找；`HTTP_ADDR` 这类短名是 `BindEnv` 的显式绑定，
   `DB_DSN` 这类则走 Unmarshal 后的 `applyEnvOverrides` 显式覆盖，三者不要混为一谈。
4. **代码默认值**：`SetDefault(...)`，如 `server.pprof_enabled=false`、`server.grpc_addr=":50051"`、
   `migrate.auto=true`、`outbox.enabled=true`、`outbox.interval_second=5`、`limiter.enabled=false`、
   `mq.nats.enabled=false`、`redis.enabled=false`、`auth.mode="token"`、
   `auth.access_expire_hour=2`、`auth.refresh_expire_day=7`、`deployment.mode="mono"`、
   `system_grpc_addr="127.0.0.1:50051"` 等，保证无配置文件也能启动。

一期新增的三组键（`pkg/config/config.go`）：

| 键 | 取值 | 作用 |
| --- | --- | --- |
| `auth.mode` | `token`（默认）/ `jwt` | 签发与校验走 OAuth2 不透明令牌（查库校验），`jwt` 为回滚开关；`auth.access_expire_hour` / `auth.refresh_expire_day` 控制有效期 |
| `deployment.mode` | `mono`（默认）/ `micro` | 决定模块装配（是否装 member）与 gRPC TokenService 是否监听 |
| `system_grpc_addr` | 默认 `127.0.0.1:50051` | member-server → system-server TokenService 地址（micro 联调/部署用） |

其他约定：

- 配置文件缺失不报错（`ConfigFileNotFoundError` 直接忽略），便于容器里只靠 env 启动；
- 开关类配置（redis/limiter/breaker/nats/outbox/microservice）关闭时对应组件返回 nil，
  中间件与服务统一做 graceful no-op；
- `modules.<name>` 开关同时决定模块是否加载自身配置（order 模块还会据此跳过路由注册）。

## 8. 测试策略

全部是进程内单元测试，**CI 不依赖任何外部服务**，共 20 个测试包（`go test ./... | grep -c "^ok"`）：
`alexgo-server/cmd`、`alexgo-server/server`、`modules/member/cmd`、`modules/member/service`、
`modules/system`、`modules/system/cmd`、`modules/system/grpcserver`、`modules/system/model`、
`modules/system/repository`、`modules/system/service`、`pkg/auth`、`pkg/circuitbreaker`、
`pkg/client`、`pkg/config`、`pkg/limiter`、`pkg/middleware`、`pkg/outbox`、`pkg/tenant`、
`pkg/tenant/gormplugin`、`pkg/token`。
依赖替身：

- **miniredis**：模拟 Redis，实际跑限流 Lua 脚本（`pkg/limiter`）；
- **内存 Casbin**：`model.NewModelFromString` + `AddPolicy` 构造 enforcer，覆盖鉴权中间件的
  401/403/200、租户前缀 sub 与 data_scope 注入（`pkg/middleware`），以及 role_menus → 策略重建/清陈旧
  （`modules/system/service` 的 `policy_sync_test.go`）；
- **glebarez/sqlite 内存库**：三类用途——`/health/ready` 可探活/可断开的 DB（`alexgo-server/server`）、
  **租户字段隔离插件**的 INSERT 填充 / SELECT 过滤 / data_scope 叠加验证（`pkg/tenant/gormplugin`、
  `pkg/tenant`）、仓库层与 TokenService 的落库路径（`modules/system/repository`、
  `modules/system/grpcserver`、`pkg/token`）；
- **bufconn gRPC**：`google.golang.org/grpc/test/bufconn` 内存监听，验证 TokenClient ↔ TokenService
  的通路与参数映射（`pkg/client`），不起真实端口；
- **fake Issuer / Validator**：`token.Issuer`、`token.Validator` 的内存替身，覆盖签发-刷新-登出语义
  与中间件校验分支（`modules/system/service`、`modules/member/service`、`pkg/middleware`），
  另有 FX 依赖图反向干跑（`alexgo-server/cmd`、`modules/member/cmd`）；
- **fake Broker**：`mq.Broker` 的假实现，验证 Outbox 发布成功 → `published`、失败 → `failed`
  的状态语义（`pkg/outbox`）。

跑法：`make test`（即 `go test ./...`）。补充一句事实：本机若默认 `CGO_ENABLED=1`，测试二进制
可能在链接阶段因系统 C 工具链问题失败，`CGO_ENABLED=0 go test ./...` 可全量通过——这是本机
环境问题，CI（GitHub Actions `ubuntu-latest`）不受影响。

## 9. 部署拓扑

- **运行形态与双服务**：本地 `make run` 是 **mono**（单进程 system+order+member，:8080）；
  compose / kustomize / Helm 交付的默认形态是 **micro 双服务**：
  - `alexgo`（system-server）：业务 :8080 + **gRPC TokenService :50051**（`DEPLOYMENT_MODE=micro`，
    `StartGRPCServer` 按该开关门控），只装 system+order 路由；
  - `member`（member-server，容器入口 `/app/member-server`）：:8081，`SYSTEM_GRPC_ADDR` 指向前者
    的 50051，会员签发/刷新经 gRPC 委托；**两服务共用同一数据库**（同库约束）；
  - **前缀分流**：`/api/app/member`、`/api/admin/member` 两个前缀走 member 服务——
    `deployments/kubernetes/ingress.yaml` 与 Helm `templates/ingress.yaml` 把这两条 path 排在 `/`
    之前（nginx-ingress 最长前缀匹配）；本地前端用 `MEMBER_PROXY=http://localhost:8081 pnpm --filter @vben/web-antd dev`
    把 Vite 的 member 代理指到 :8081（`apps/web-antd/vite.config.ts`），mono 时默认全部指 :8080。
- **本地**：`make run` 直跑；或 `make docker-up`（`deployments/docker-compose/docker-compose.yml`，
  含 MySQL，`DB_DSN` 经 `environment:` 注入，内置 system/member 双容器）；前端 `apps/web-antd/` 独立
  `pnpm --filter @vben/web-antd dev`。micro 本地双进程用 `make run-system` + `make run-member`。
- **K8s**，二选一：
  - kustomize：`kubectl apply -k deployments/kubernetes`（namespace / configmap / deployment +
    deployment-member / service + service-member / hpa / ingress）；
  - Helm 多环境：`deployments/helm/alexgo-cloud`，`values.yaml` 加
    `values-dev.yaml` / `values-gray.yaml` / `values-prod.yaml` 覆盖差异（副本数、镜像 tag、
    `pprofEnabled`、域名、HPA），并含 `deployment.mode`（默认 micro）与 `member.replicaCount`；
    `deployments/argocd/` 提供 dev/gray/prod 三个 Application
    （`valueFiles: [values.yaml, values-<env>.yaml]`，自动 sync + prune + CreateNamespace）。
- **配置与密钥注入**：清单把 `config.yaml` 以 volume 挂载进容器作为兜底，密钥类走 env 通道
  且**由 K8s Secret 注入、压过 ConfigMap 里的 config.yaml**（`applyEnvOverrides` 在 Unmarshal
  **之后**覆盖，env 始终生效）：
  - kustomize：`deployments/kubernetes/secret.yaml` 提供 **占位值** 的 `alexgo-secrets`
    （`DB_DSN`/`JWT_SECRET`），deployment 经 `env` + `secretKeyRef` 注入；ConfigMap 顶层的
    明文 `DB_DSN` 已删除，config.yaml 只保留 `database.dsn` 兜底。compose 用 `environment:`。
  - Helm：`values.yaml` 的 `secrets.dbDsn` / `secrets.jwtSecret` 控制——**为空则不渲染 Secret**
    （`templates/secret.yaml` 有条件生成），deployment 的 `secretKeyRef` 标了 `optional: true`，
    Secret 缺失时回退到 ConfigMap 渲染的 config.yaml。
  - 生产务必覆盖占位值：推荐用 **Sealed Secrets / External Secrets / kubectl create secret**
    覆盖 `secret.yaml` 的占位值（切勿把真实 DSN/JWT secret 提交进 git），代码无需改动。
- **探针**：liveness → `/health`，readiness → `/health/ready`（kustomize 与 Helm 模板均已配置）。
- **CI/CD**：`.github/workflows/ci.yml`（push `main`/`develop` 与 PR）：依赖安装 → golangci-lint
  → proto 生成 → `go test ./... -v -race` → `make build` → `helm lint` → docker build；
  `cd.yml`（tag `v*`）：docker build-push → Helm 升级部署。
