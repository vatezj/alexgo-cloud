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
| `modules/` | 业务模块，当前有 `system`（用户/角色/菜单/字典/部门/公告/审计等）与 `order`（示例业务）；内部按 `api / controller / model / repository / service` 分层 |
| `pkg/` | 与业务无关的基础设施与横切能力：`config`、`database`、`migrate`、`middleware`、`auth`(JWT/Casbin)、`limiter`、`circuitbreaker`、`outbox`、`mq`、`redis`、`tenant`、`audit`、`monitor`、`trace`、`logger`、`client`(gRPC)、`errors` 等 |
| `admin-web/` | 管理后台前端（Vue 3 + Vite），经 `/api/**` 调用后端 |
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

`--migrate-only`：跳过 HTTP Server 装配，只做迁移与后台初始化，用于 K8s initJob / 发布前迁移
（`make migrate`）。

## 4. 单体 → 微服务切换

`cmd/main.go` 用 `fx.Decorate` 在依赖图构建完成后替换 `systemservice.UserService` 的最终实现：

- `microservice.enabled=false`（默认）：直接返回本地实现，跨模块调用零开销；
- `microservice.enabled=true`：返回 `client.NewUserGRPCClient(conn)`——同一 Go 接口的 gRPC
  客户端实现，所有注入该接口的调用方**零改动**；若此时 gRPC 连接为 nil 则直接返回错误，
  避免“配置说微服务、运行时却是单体”的不一致。

配套入口：`modules/system/cmd/grpc_main.go` 提供 system 模块的独立进程化装配
（`system.FxModule` + `grpcserver.StartGRPCServer`）。拆分只换装配，不改调用方代码。

## 5. HTTP 中间件链（顺序即优先级）

`alexgo-server/server/http.go` 的 `newRouter` 用 `gin.New()` 按下列顺序挂全局中间件
（先注册先执行，外层 → 内层；顺序即优先级）：

1. `middleware.Recovery()` — 捕获 panic，对外返回 500，不打挂进程；
2. `middleware.TenantMiddleware()` — 从 `X-Tenant-ID` 请求头注入租户上下文，是多租户隔离、
   审计与限流维度的基础；
3. `middleware.Logger()` — 结构化请求日志；
4. `monitor.PrometheusMiddleware()` — 记录 `http_requests_total`（method/path/status）等指标；
5. `middleware.RateLimitAndBreaker(cfg, limiter, breaker)` — Redis Token Bucket 限流 + 熔断的
   入口防护；limiter/breaker 为 nil（对应 `limiter.enabled` / `breaker.enabled` 关闭）时优雅放行；
6. `middleware.AuthMiddleware(cfg, enforcer)` — **仅对 `/api/admin/**` 前缀**做 JWT 校验 +
   Casbin RBAC；其余路径直接放行，不影响健康检查、指标与 app 端接口；
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

1. **环境变量显式覆盖（密钥类，最终生效）**：`v.Unmarshal(&cfg)` **之后**调用
   `applyEnvOverrides`，把 `DB_DSN` → `database.dsn`、`JWT_SECRET` → `system.jwt_secret`、
   `REDIS_PASSWORD` → `redis.password`（仅非空时覆盖）。必须放在 Unmarshal 之后的原因见函数
   注释：`viper.AutomaticEnv` 的隐式映射不参与 Unmarshal（viper 已知行为），且模块配置合并用的
   `v.Set()` 在 viper 中优先级高于 env——不经这一步，密钥 env 永远输给模块 yaml。
2. **模块 yaml（`v.Set` 命名空间合并）**：读入全局配置后检查 `modules.<name>` 开关，为 true 的
   模块加载 `modules/<name>/configs/config.yaml`，并以 `模块名.` 为前缀 `v.Set` 合并
   （如 `modules/system/configs/config.yaml` 的 `jwt_secret` → `system.jwt_secret`）。
3. **全局 `alexgo-server/configs/config.yaml`**；另有 `BindEnv` 的常用映射
   `HTTP_ADDR` / `GRPC_ADDR` / `NATS_URL` / `REDIS_ADDR`（`server.http_addr` 等经
   `EnvKeyReplacer` 把 `.` 转 `_`），按 viper 原生优先级高于配置文件。
4. **代码默认值**：`SetDefault(...)`，如 `server.pprof_enabled=false`、`migrate.auto=true`、
   `outbox.enabled=true`、`outbox.interval_second=5`、`limiter.enabled=false`、
   `mq.nats.enabled=false`、`redis.enabled=false` 等，保证无配置文件也能启动。

其他约定：

- 配置文件缺失不报错（`ConfigFileNotFoundError` 直接忽略），便于容器里只靠 env 启动；
- 开关类配置（redis/limiter/breaker/nats/outbox/microservice）关闭时对应组件返回 nil，
  中间件与服务统一做 graceful no-op；
- `modules.<name>` 开关同时决定模块是否加载自身配置（order 模块还会据此跳过路由注册）。

## 8. 测试策略

全部是进程内单元测试，**CI 不依赖任何外部服务**，共 7 个测试包：`pkg/config`、`pkg/auth`、
`pkg/circuitbreaker`、`pkg/limiter`、`pkg/middleware`、`pkg/outbox`、`alexgo-server/server`。
依赖替身：

- **miniredis**：模拟 Redis，实际跑限流 Lua 脚本（`pkg/limiter`）；
- **内存 Casbin**：`model.NewModelFromString` + `AddPolicy` 构造 enforcer，覆盖鉴权中间件的
  401/403/200 与非 admin 路径放行（`pkg/middleware`）；
- **glebarez/sqlite 内存库**：给 `/health/ready` 提供可探活、可断开的 DB（`alexgo-server/server`）；
- **fake Broker**：`mq.Broker` 的假实现，验证 Outbox 发布成功 → `published`、失败 → `failed`
  的状态语义（`pkg/outbox`）。

跑法：`make test`（即 `go test ./...`）。补充一句事实：本机若默认 `CGO_ENABLED=1`，测试二进制
可能在链接阶段因系统 C 工具链问题失败，`CGO_ENABLED=0 go test ./...` 可全量通过——这是本机
环境问题，CI（GitHub Actions `ubuntu-latest`）不受影响。

## 9. 部署拓扑

- **本地**：`make run` 直跑；或 `make docker-up`（`deployments/docker-compose/docker-compose.yml`，
  含 MySQL，`DB_DSN` 经 `environment:` 注入）；前端 `admin-web/` 独立 `npm run dev`。
- **K8s**，二选一：
  - kustomize：`kubectl apply -k deployments/kubernetes`（namespace / configmap / deployment /
    service / hpa / ingress）；
  - Helm 多环境：`deployments/helm/alexgo-cloud`，`values.yaml` 加
    `values-dev.yaml` / `values-gray.yaml` / `values-prod.yaml` 覆盖差异（副本数、镜像 tag、
    `pprofEnabled`、域名、HPA）；`deployments/argocd/` 提供 dev/gray/prod 三个 Application
    （`valueFiles: [values.yaml, values-<env>.yaml]`，自动 sync + prune + CreateNamespace）。
- **配置与密钥注入**：清单把 `config.yaml` 以 volume 挂载进容器，同时提供 env 注入通道——
  kustomize 清单用 `envFrom` ConfigMap 注入 `DB_DSN`/`HTTP_ADDR`，compose 用 `environment:`；
  由于 `applyEnvOverrides` 在 Unmarshal **之后**覆盖，**env 始终压过配置文件**。仓库当前没有
  K8s Secret 清单（DSN/JWT secret 暂放在 ConfigMap 或 Helm values 渲染的 config.yaml 里），
  生产应把 `DB_DSN` / `JWT_SECRET` / `REDIS_PASSWORD` 放入 Secret 后以 env 注入，代码无需改动。
- **探针**：liveness → `/health`，readiness → `/health/ready`（kustomize 与 Helm 模板均已配置）。
- **CI/CD**：`.github/workflows/ci.yml`（push `main`/`develop` 与 PR）：依赖安装 → golangci-lint
  → proto/CRUD 生成 → `go test ./... -v` → `make build` → `helm lint` → docker build；
  `cd.yml`（tag `v*`）：docker build-push → Helm 升级部署。
