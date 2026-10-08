# alexGo-cloud

模块化单体架构 Go 项目，对标 yudao-cloud 思路：模块独立、统一启动、按需演进微服务。

## 启动

```bash
make run
```

## 测试

```bash
make test
```

测试为进程内单元测试，不依赖 MySQL/Redis/NATS。

## 迁移

```bash
make migrate
```

## Proto 生成

```bash
make proto
```

## CRUD 生成

```bash
make generate-crud
```

## 基于数据表生成 CRUD

```bash
DB_DSN="root:root@tcp(127.0.0.1:3306)/alexgo?charset=utf8mb4&parseTime=True&loc=Local" make generate-db-crud MODULE=system TABLES=users,roles,menus
```

## Docker Compose

```bash
make docker-up
make docker-down
```

## Helm

```bash
make helm-upgrade
```

Helm 多环境 values：
- deployments/helm/alexgo-cloud/values.yaml
- deployments/helm/alexgo-cloud/values-dev.yaml
- deployments/helm/alexgo-cloud/values-prod.yaml
- deployments/helm/alexgo-cloud/values-gray.yaml

## 监控栈

```bash
docker-compose -f deployments/docker-compose/docker-compose.monitor.yml up
```

## 健康检查

- `GET /health`：存活探针（不依赖 DB，livenessProbe 用）
- `GET /health/ready`：就绪探针（ping DB，失败 503，readinessProbe 用）

## 后台前端（Vben Admin + Naive UI 风格）

目录：admin-web/

```bash
cd admin-web
npm i
npm run dev
```

登录：
- 访问 http://localhost:5174
- 用后端的登录接口获取 token（页面内会调用 /api/app/system/auth/login）

默认账号（启动后自动初始化）：
- username: admin
- password: admin123

## 前端目录

- `admin-web/`：管理后台（Vue 3 + Naive UI + Vite），唯一维护的前端

## Kubernetes

```bash
kubectl apply -k deployments/kubernetes
```

## 压测

```bash
make load-test
```

## pprof

默认关闭（`server.pprof_enabled: false`）。本地 `alexgo-server/configs/config.yaml` 已开启；
生产环境请保持关闭，调试时临时开启并限制网络访问。

```bash
make pprof
```
