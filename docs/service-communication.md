# alexGo-cloud 服务间调用说明

> 本文依据当前代码整理，是理解模块/服务之间如何互相调用的单一参考。
> 与实现不一致时，以代码为准。架构全景见 `docs/architecture.md`。

## 1. 总览：双形态，由配置决定

服务间通信有**两种形态**，运行时由 `deployment.mode` 与 `microservice.enabled`
（`pkg/config/config.go`）共同决定，不存在第三种旁路：

| 形态 | 触发条件 | 调用方式 | 有无网络调用 |
| --- | --- | --- | --- |
| **mono 单体**（默认） | `deployment.mode != "micro"` 且 `microservice.enabled=false` | fx 依赖注入的**进程内函数直调** | 无 |
| **micro 双服务** | `DEPLOYMENT_MODE=micro`（或旧开关 `microservice.enabled`） | member → system 走 **gRPC**（:50051） | 有（Token 签发） |

启动入口对应关系：

| 命令 | 形态 | 进程内容 |
| --- | --- | --- |
| `make run` | mono | 单进程 system+order+member，:8080 |
| `make run-system` | micro（system 侧） | `DEPLOYMENT_MODE=micro go run ./alexgo-server/cmd/main.go`，:8080 + gRPC :50051 |
| `make run-member` | micro（member 侧） | `HTTP_ADDR=:8081 SYSTEM_GRPC_ADDR=127.0.0.1:50051 go run ./modules/member/cmd/main.go`，:8081 |

当前 `alexgo-server/configs/config.yaml` 为 `microservice.enabled: false`、
`deployment.mode` 未设（默认 `mono`）——**即当前生效的是 mono 形态，服务间没有任何网络调用**。

## 2. mono 形态：进程内 fx 直调

入口 `alexgo-server/cmd/main.go` 单进程装配全部模块，模块之间通过 Go 接口直接调用，
零 RPC 开销。跨模块依赖全部由本地实现满足：

| 跨模块依赖 | 满足方式 | 代码位置 |
| --- | --- | --- |
| `token.Issuer` / `token.Validator` | 本地 `*token.Service` 显式映射（system/member 两模块共用） | `cmd/main.go` `ifaceOptions()` |
| `systemservice.UserService` | `fx.Decorate` 在 `microservice.enabled=false` 时返回本地实现 | `cmd/main.go` decorate 块 |
| member 的签发/额度检查 | `member.FxModule` 同进程注册，Issuer/AccountLimitChecker 本地满足 | `cmd/main.go` 条件追加 `member.FxModule` |
| 模块 HTTP 路由 | 各模块经 `group:"modules"` 聚合，统一挂 `/api` | `alexgo-server/server/http.go` |

mono 形态下 gRPC 相关组件的状态：

- `client.NewGRPCConn`：两开关皆关 → 返回 `nil` conn，不拨号
  （`pkg/client/grpc_conn.go`）；
- `grpcserver.StartGRPCServer`：内部按 `deployment.mode` 门控，mono 直接 return，
  **不监听 :50051**（`modules/system/grpcserver/server.go`）。

## 3. micro 形态：双进程 + gRPC

```
member-server (:8081)                      system-server (:8080 + :50051)
├─ 会员登录/刷新/登出 ──── gRPC ──────────▶ TokenService（唯一签发方）
│   token.Issuer = tokenGRPCClient           IssueToken / RefreshToken / RevokeToken
├─ token 校验 ────────────── 本地 ──┐
│   token.Service 查共享 token 表   │   同一个 MySQL 库（spec §9.10 同库约束）
└─ 账号额度检查 ──────────── 本地 ──┘   tenant.NewAccountLimitChecker 直查 DB
```

两个进程的差异（member-server 入口 `modules/member/cmd/main.go` 头注释）：

| 关注点 | system-server | member-server |
| --- | --- | --- |
| HTTP 端口 | :8080 | :8081（`HTTP_ADDR` 覆盖） |
| gRPC TokenService | :50051 监听（唯一签发方） | 不监听，只做客户端 |
| `token.Issuer` | 本地 `*token.Service` | **gRPC 客户端**（委托 system） |
| `token.Validator` | 本地 `*token.Service` | 本地 `*token.Service`（读共享 token 表） |
| Casbin/审计 | 有 | 无（member 无 RBAC） |
| 账号额度 | system 模块实现 | `pkg/tenant` 的 DB 窄实现（system 不在本进程） |

### 3.1 调用语义

- **签发必须走 system**：`client.NewTokenIssuer`（`pkg/client/token_client.go`）包装
  gRPC `TokenService`，实现 `token.Issuer` 接口，调用方（member 登录/刷新）无感知；
- **失败即 503、不降级**：gRPC 故障直接向上抛，controller 映射 503；
- **conn 为 nil → fail-fast**：`tokenIssuerProvider` 先用具体指针判 `conn == nil`，
  返回 `nilIssuer{}`（所有方法返回 "system grpc not connected"），
  绝不允许退化成本地签发（双进程下 secret/表约定可能不一致）；
- **校验不走网络**：member 本地 `token.Service` 查共享 token 表（两服务共用同一数据库
  是硬约束），所以登录后的接口鉴权不依赖 gRPC 连通性；
- **`RevokeAll` 明确不支持跨 gRPC**：踢人接口仅 system-server 本地使用
  （member"禁用+踢人"一期不做联动，见 `token_client.go` 注释）。

### 3.2 地址解析与拨号门控

- **拨号条件**（`pkg/client/grpc_conn.go`）：`deployment.mode=micro`
  **或** `microservice.enabled` 任一开启即拨号；两者皆关返回 nil conn。
- **地址解析**（`pkg/client/registry.go` `GetServiceAddress`），优先级：
  1. 显式 `system_grpc_addr` 配置 / `SYSTEM_GRPC_ADDR` 环境变量
     （默认值 `127.0.0.1:50051`，见 `pkg/config/loader.go`）；
  2. `microservice.enabled=true` 但没配地址 → 占位 `localhost:50051`；
  3. 都没有 → 报错。
- 拨号用 `grpc.NewClient`（懒连接、不做 I/O）+ insecure 凭证，连接生命周期由 fx 管理。

### 3.3 system-server 侧 gRPC 装配

- `TokenServiceImpl` 经 `group:"grpc_registrars"` 注册（`modules/system/module.go`），
  `StartGRPCServer` 聚合 group 内所有 registrar 挂到 `grpc.Server` 上；
- 另有独立纯 gRPC 入口 `modules/system/cmd/grpc_main.go`（只装配
  `system.FxModule` + `StartGRPCServer`，不起 HTTP）；
- proto 定义：`modules/system/api/rpc/token.proto`，三个 RPC —
  `IssueToken` / `RefreshToken` / `RevokeToken`。

## 4. 旧演进路径：`microservice.enabled` + fx.Decorate

`microservice.enabled` 是一期预留的"单体 → 微服务"演进开关，与 `deployment.mode`
叠加生效（任一开启即拨 gRPC 连接），当前 `config.yaml` 中为 `false`。

`cmd/main.go` 的 `fx.Decorate` 在依赖图构建完成后替换 `systemservice.UserService`：

- `microservice.enabled=false`：返回本地实现，跨模块调用零开销；
- `microservice.enabled=true`：返回 `client.NewUserGRPCClient(conn)`——
  同一 Go 接口的 gRPC 客户端实现，所有注入该接口的**调用方零改动**；
  若此时 conn 为 nil 直接返回错误（防止"配置说微服务、运行时却是单体"的不一致）。

## 5. 异步事件通路（Outbox → NATS）

同步调用之外，模块间还可用事件解耦（`docs/architecture.md` §6）：

- 业务事务内写 `outbox_events` → `pkg/outbox` Relay 轮询
  `FOR UPDATE SKIP LOCKED` 抢占 → 发布到 `mq.Broker`（NATS JetStream）；
- **当前状态**：`outbox.enabled=true` 但 `mq.nats.enabled=false` → broker 为 nil，
  Relay 轮询后实际 no-op，事件只落表不外发。启用 NATS 后即成为跨服务异步通道。

## 6. 服务发现现状（consul 未接入）

`config.yaml` 中的 `microservice.registry: "consul://127.0.0.1:8500"` **目前是占位**：

```go
// pkg/client/registry.go
_ = serviceName // consul 服务发现未接入：serviceName 暂为占位参数
```

服务寻址**全部依赖静态地址配置**（`system_grpc_addr` / `SYSTEM_GRPC_ADDR`）。
接入 consul 属于后续演进项，接入前改地址只能改配置/env，不能靠注册中心发现。

## 7. 快速对照

| 问题 | 答案 |
| --- | --- |
| 现在（mono）服务之间怎么调用？ | 不走网络——同进程 fx 注入的普通函数调用 |
| micro 下 member → system 调什么？ | 仅 Token 签发/刷新/注销走 gRPC :50051 |
| micro 下校验 token 走网络吗？ | 不走，本地查共享 token 表（同库硬约束） |
| gRPC 挂了会怎样？ | 登录/刷新 503，不降级、不本地签发 |
| 服务发现用什么？ | 静态地址；consul 配置是占位，未接入 |
| 模块间异步怎么通信？ | Outbox → NATS（NATS 当前关闭，事件只落表） |
| 拆新服务要改调用方代码吗？ | 不用——`fx.Decorate` 换同接口实现，调用方无感知 |
