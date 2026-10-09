# alexGo-cloud

[![CI](https://github.com/vatezj/alexgo-cloud/actions/workflows/ci.yml/badge.svg)](https://github.com/vatezj/alexgo-cloud/actions/workflows/ci.yml)

**模块化单体（Modular Monolith）架构的 Go 后台基座**，对标 [yudao-cloud](https://github.com/YunaiV/yudao-cloud) 思路：模块独立、统一启动、按需演进微服务。

技术栈：**Go 1.25 · Gin · Uber FX（依赖注入）· GORM（MySQL）· Casbin · JWT · Redis · NATS JetStream · gRPC · golang-migrate · Zap · OpenTelemetry · Prometheus**

## 📊 内置功能

### 业务模块

| 模块 | 功能 |
| --- | --- |
| 系统管理（`modules/system`） | 用户、角色、菜单、部门、岗位、字典、配置、通知、租户 |
| 登录与用户体系 | 账号密码 / Token 签发刷新注销（OAuth2 不透明令牌）/ 会员 mobile 注册登录 |
| 权限与安全 | Casbin RBAC（菜单粒度授权，主体带租户前缀）、data_scope 数据权限、登录日志、操作审计 |
| 会员（`modules/member`） | mobile 注册/登录/刷新/登出、会员列表与启停（micro 形态经 gRPC 委托 system 签发） |
| 订单演示（`modules/order`） | 单表 CRUD 示例（`modules.order: false` 默认关闭，用于演示模块接入） |
| 后台前端（`admin-web/`） | Vue 3 + Naive UI + Vite 管理后台，覆盖上述系统管理页面 |

### 基础设施（`pkg/`）

| 能力 | 说明 |
| --- | --- |
| 统一装配 | Uber FX 依赖注入，模块经 `group:"modules"` 聚合注册路由 |
| 单体 → 微服务 | `fx.Decorate` 一键把本地实现替换为 gRPC 客户端，调用方零改动 |
| 入口防护 | Redis Token Bucket 限流 + 熔断器（均可配置开关，故障 fail-open） |
| 可靠事件 | Transactional Outbox（`FOR UPDATE SKIP LOCKED`）→ NATS JetStream |
| 多租户 | `X-Tenant-ID` 头 / 域名解析注入租户上下文，GORM 插件自动过滤 `tenant_id`，租户管理 |
| 可观测性 | `/health` 存活、`/health/ready` DB 探活、`/metrics` Prometheus、OTel 链路追踪、pprof（默认关闭） |
| 代码生成 | 模板 CRUD 生成器 + 按数据表反向生成整套 CRUD |
| 部署 | docker-compose / Kustomize / Helm（dev·gray·prod）+ ArgoCD 多环境 |

## 🚀 快速开始

### 环境要求

- Go 1.25+、MySQL 8.x、Node.js 18+（前端）
- 可选：Redis（限流）、NATS（事件）——不启动也能跑，默认配置已关闭

### 后端启动

```bash
# 1. 创建数据库（DSN 见 alexgo-server/configs/config.yaml，默认 root:root）
mysql -uroot -p -e "CREATE DATABASE IF NOT EXISTS alexgo DEFAULT CHARSET utf8mb4;"

# 2. 执行数据库迁移（make run 时也会自动执行，migrate.auto 默认开启）
make migrate

# 3. 启动（监听 :8080）
make run
```

验证：

```bash
curl http://localhost:8080/health        # {"status":"ok",...}
curl http://localhost:8080/health/ready  # 就绪探活（ping DB）
```

### 后台前端

```bash
cd admin-web
npm i
npm run dev
```

- 访问 <http://localhost:5174>，登录接口为 `/api/app/system/auth/login`
- 默认账号（后端启动后自动初始化）：**admin / admin123**

> 🍎 macOS 用户提示：本机如遇 `go build` / `go test` 链接报错（SDK 兼容问题），
> 加 `CGO_ENABLED=0` 执行即可，如 `CGO_ENABLED=0 make test`。

## 运行模式

| 模式 | 命令 | 说明 |
| --- | --- | --- |
| mono（默认） | `make run` | 单进程全部模块（system+member，order 按配置开关），Token 本地签发，端口 :8080 |
| micro（部署形态） | `make run-system` + `make run-member` | system-server :8080(+gRPC :50051) 与 member-server :8081，member 经 gRPC 委托签发 |

配置：`deployment.mode`（mono|micro）；member 上游地址 `SYSTEM_GRPC_ADDR`。
会话行为：access token 2h、refresh token 7d（`auth.access_expire_hour` / `auth.refresh_expire_day` 可调）；
admin-web 的自动刷新为后续项——access 过期后需重新登录（app 端可用 `/api/app/member/auth/refresh` 换新）。
micro 本地联调前端分流：`MEMBER_PROXY=http://localhost:8081 npm run dev`。

## 📁 项目结构

```
alexGo-cloud/
├── alexgo-server/     # 启动入口 + HTTP Server 装配（cmd/main.go、server/）
├── modules/           # 业务模块（system、order、member），各含 controller/service/repository/model
├── pkg/               # 20+ 基础能力包（auth、limiter、outbox、middleware、migrate…）
├── admin-web/         # 管理后台前端（Vue 3 + Naive UI + Vite）
├── deployments/       # docker-compose / kubernetes / helm / argocd
├── scripts/           # proto 生成、CRUD 生成器、k6 压测脚本
├── tools/dbgen/       # 按数据表反向生成 CRUD
└── docs/              # 文档
```

## 📚 文档

- [架构说明](docs/architecture.md)：模块装配、中间件链、单体→微服务切换、Outbox、配置体系、测试策略、部署拓扑
- 使用指南树（yudao-cloud 开发指南式：快速启动 / 技术选型 / 专题指南）**建设中**

## 🐳 部署与运维

### Docker Compose

```bash
make docker-up     # 构建并启动
make docker-down   # 停止
```

### Kubernetes（Kustomize）

```bash
kubectl apply -k deployments/kubernetes
```

> ⚠️ 部署前必须覆盖 `deployments/kubernetes/secret.yaml` 的占位值
> （或改用 Sealed Secrets / External Secrets），否则 readiness 探活会失败。

### Helm（多环境）

```bash
make helm-upgrade
```

多环境 values：

- `deployments/helm/alexgo-cloud/values.yaml`（基础，pprof 关、密钥留空）
- `deployments/helm/alexgo-cloud/values-dev.yaml`
- `deployments/helm/alexgo-cloud/values-gray.yaml`
- `deployments/helm/alexgo-cloud/values-prod.yaml`

密钥通过 `--set secrets.dbDsn=... --set secrets.jwtSecret=...` 注入（留空则回退 configmap 兜底，生产不建议）。

### 监控栈

```bash
docker-compose -f deployments/docker-compose/docker-compose.monitor.yml up
```

### 健康检查

| 端点 | 用途 |
| --- | --- |
| `GET /health` | 存活探针（不依赖 DB，livenessProbe 用） |
| `GET /health/ready` | 就绪探针（ping DB，失败 503，readinessProbe 用） |
| `GET /metrics` | Prometheus 指标 |

### 压测

```bash
make load-test    # k6，脚本 scripts/load_test.js
```

### pprof

默认关闭（`server.pprof_enabled: false`），本地 `alexgo-server/configs/config.yaml` 已开启；生产保持关闭，调试时临时开启并限制网络访问。

```bash
make pprof
```

## ⌨️ 命令参考

| 命令 | 说明 |
| --- | --- |
| `make run` | mono 模式启动（单进程全部模块，含自动迁移） |
| `make run-system` | micro 模式 system-server（`DEPLOYMENT_MODE=micro`，含 gRPC TokenService :50051） |
| `make run-member` | micro 模式 member-server（:8081，`SYSTEM_GRPC_ADDR=127.0.0.1:50051`） |
| `make migrate` | 仅执行数据库迁移（`--migrate-only`，适合发布前 / init Job） |
| `make test` | 单元测试（进程内，不依赖 MySQL/Redis/NATS） |
| `make build` | 编译到 `bin/alexgo-server` |
| `make proto` | 生成 protobuf 代码（需 protoc） |
| `make generate-crud` | 模板 CRUD 生成（不覆盖已有文件） |
| `DB_DSN=... make generate-db-crud MODULE=system TABLES=users,roles,menus` | 按数据表反向生成 CRUD |
| `make lint` | golangci-lint（CI 同款） |
| `make load-test` | k6 压测 |
| `make helm-upgrade` | Helm 安装/升级 |
