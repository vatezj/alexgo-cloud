# alexGo-cloud 用户体系（SaaS 多租户 + 全登录方式）设计文档

> **状态**：已评审通过；一期（块①②③⑦ + 双服务骨架 + 双运行模式）已实施（见 git log）
> 二/三期（块④⑤⑥：短信/三方/小程序）待排期
> **参考**：yudao-cloud《用户体系》文档 + 用户提供的三表结构图（system_users / system_oauth2_access_token / member_user）
> **基线代码**：`main` @ `ffadd5a`（2026-10-08）
>
> **实施偏差（一期）**：与本 spec 不一致、经评审裁决按下列口径落地——
> 1. **Token 无进程内缓存**：§4.2 曾提议"LRU 缓存 60s"，裁决移除：cache-aside 有 TOCTOU
>    （Validate 读库后、写缓存前可与 Revoke 交错把已吊销 token 写回），且多进程下缓存让
>    "踢人立即失效"最长延迟 60s，违背验收项；改为恒查库（单表唯一索引点查，正确性优先）。
> 2. **状态位统一 `1=启用 0=停用`**：本 spec DDL 注释里"0开启1停用"一律以此为准改写
>    （与既有 `users.Status==1` 才可登录一致）。
> 3. **`deleted` 列用 `TINYINT(1)`（不是 `BIT(1)`）**：BIT(1) 经 go-sql-driver 返回原始字节，
>    与 GORM bool 字段扫描不兼容（源码级证实）；且一期仅落列、不启用 `gorm.DeletedAt` 软删。

## 1. 背景与现状

### 1.1 现有能力

| 能力 | 现状 |
| --- | --- |
| 用户表 | 单表 `users`（id/username/nickname/password_hash/status/tenant_id/时间戳），仅管理员 |
| 登录 | 账号密码 → 静态密钥 JWT（HS256，24h），单接口 `POST /api/app/system/auth/login` |
| 认证中间件 | `AuthMiddleware` 解析 Bearer JWT，`/api/admin/**` 强制，Casbin RBAC |
| 路由风格 | `/api/admin/**`（管理端）、`/api/app/**`（应用端），等价于 yudao 的 `/admin-api`、`/app-api` |
| 多租户 | **仅雏形**：各表有 `tenant_id` 列 + `X-Tenant-ID` 头解析中间件；**无租户表、无查询隔离、登录不解析租户** |
| 缺失 | 会员用户、OAuth2 Token 表、短信、三方登录、小程序登录、租户管理 |

### 1.2 目标

以 yudao-cloud 用户体系为蓝本，在本项目实现：

1. **双用户架构**（硬性需求，用户明确要求）：AdminUser 与 AppUser（会员）**必须分表**——管理员存 `system_users`、会员存 `member_user`，两表结构各自独立、绝不合并，`user_type` 统一区分
2. **OAuth2 Token 体系**：统一令牌表签发/刷新/注销，双端 Bearer 认证
3. **SaaS 多租户**：租户表 + 登录租户解析 + 查询级数据隔离 + 租户管理
4. **短信登录**：验证码发送（可插拔 Provider）、短信登录 + 未注册自动开通会员
5. **三方登录**：授权跳转 / code 快登录 / 绑定登录（可插拔 Provider）
6. **微信小程序登录**：phoneCode + loginCode 一键登录
7. **权限管理对齐**（块⑦，见 4.7）：role_menus → Casbin 策略同步、`data_scope` 数据权限、Casbin 租户隔离、role 表补列

### 1.3 非目标（本期不做）

- 数据库级多租户（每租户独立库，yudao 的"数据库隔离"方案）——本期只做**字段隔离**
- 统一会员/员工档案（yudao 也未做，双方表保持独立）
- 支付、商城等 member 侧业务功能
- OAuth2 **授权码模式**对外开放（`/oauth2/authorize` 给第三方应用授权）——本期 Token 体系只覆盖**本系统自家客户端**的登录签发
- Redis 缓存 Token（先直查 DB，预留缓存位）

## 2. 总体架构（双微服务，方案 A 已确认）

> **架构形态决策（2026-10-08 评审确认）**：拆为 **两个微服务进程**——`system-server` 与 `member-server`；
> **OAuth2 认证中心内聚在 system-server**（Token 表 + 签发/刷新/注销的权威），member-server 登录成功后经
> **gRPC 委托签发**。一期**同库不同进程**（共享一个 MySQL），真分库留二期。**不引入 API 网关**，用
> 路径前缀分流（vite proxy / Ingress path 规则）。

```
                          admin-web / 用户App / 小程序
                                   │ HTTP
                    ┌──────────────┴───────────────┐
                    │  路径前缀分流（无网关）         │
   /api/app/member/**│                    其余全部  │
                    ▼                              ▼
        ┌───────────────────────┐      ┌────────────────────────┐
        │  member-server :8081  │      │  system-server :8080   │
        │  （独立进程/二进制）     │      │  （现有 alexgo-server） │
        │  modules/member 路由   │      │  modules/system + order│
        │  · 会员注册/登录        │ gRPC │  · 管理员用户/租户/RBAC │
        │  · 短信/三方/小程序登录  │─────▶│  · OAuth2 认证中心      │
        │  · 会员管理(admin API) │      │    IssueToken/Refresh/ │
        │  · Token 校验(共享表)   │      │    Revoke + Token 表    │
        └───────────┬───────────┘      └───────────┬────────────┘
                    │                              │
                    └──────────────┬───────────────┘
                                   ▼
              同一 MySQL：system_users / member_user /
              system_oauth2_access_token / tenants / roles / menus …
              （user_id + user_type + tenant_id 三维度贯穿）
```

### 2.1 服务职责与边界

| | system-server（:8080） | member-server（:8081） |
| --- | --- | --- |
| 进程入口 | `alexgo-server/cmd/main.go`（现有） | `modules/member/cmd/main.go`（新建，独立二进制） |
| 业务模块 | `modules/system` + `modules/order` | `modules/member` |
| HTTP 路由 | `/api/admin/system/**`、`/api/admin/tenants/**`、`/api/app/system/**` | `/api/app/member/**`（会员登录族）、`/api/admin/member/**`（会员管理——模块自持双端路由，见 4.1） |
| gRPC | **服务端**：`TokenService`（监听 `grpc_addr`） | **客户端**：调 TokenService |
| 认证 | 本地签发 + 本地校验（共享 token 表） | 登录验证后委托签发；请求校验走共享 token 表 |
| 中间件 | 全套（Casbin/审计/限流/熔断） | Tenant + Auth(Token) + 日志/指标（**无 Casbin**，会员无 RBAC） |

### 2.2 gRPC 契约（`modules/system/api/rpc/token.proto`）

```protobuf
service TokenService {
  rpc IssueToken(IssueTokenRequest) returns (IssueTokenResponse);
  rpc RefreshToken(RefreshTokenRequest) returns (IssueTokenResponse);
  rpc RevokeToken(RevokeTokenRequest) returns (RevokeTokenResponse);
}
message IssueTokenRequest {
  int64 user_id = 1;      // 已在 member-server 侧验证通过的用户
  int32 user_type = 2;    // 1=Admin(仅 system 内部用) 2=Member
  int64 tenant_id = 3;
  string client_id = 4;   // alexgo-app / alexgo-admin
}
message IssueTokenResponse {
  string access_token = 1;
  string refresh_token = 2;
  int64  expires_in = 3;  // 秒
}
```

- 生成：`make proto`（现有 `gen_proto.sh` 链路）
- 连接：`pkg/client`（现有 grpc_conn/registry），配置 `system_grpc_addr`；**gRPC 不可用时登录接口 fail-fast 503 + 有限重试**，不降级签发
- 本地调用优化：system-server 进程内**直接注入本地 `token.Service`**（不走自环 gRPC）——沿用现有 `fx.Decorate` 模式

### 2.3 HTTP 分流（不引入网关）

- **本地开发**：`admin-web/vite.config.ts` proxy 按前缀分流——`/api/app/member` → `localhost:8081`，其余 → `localhost:8080`
- **K8s/Helm**：Ingress path 规则 `/api/app/member` → member-service，其余 → system-service；compose 前端按同样规则配两条 upstream
- 网关（统一入口/鉴权/限流下沉）列为**二期以后**的演进项，本期不做

### 2.4 代码落位（共享能力放 pkg，两服务复用）

- `modules/system`（扩展）：管理员用户增强、租户管理、**TokenService gRPC 服务端**
- `modules/member`（新建）：会员用户、短信/三方/小程序登录、会员管理、`cmd/main.go` 独立进程
- `pkg/token`（新建）：签发/校验/刷新/注销核心库——**两服务共用**（system 是权威写入方，member 只读校验 + gRPC 委托写入）
- `pkg/tenant`（扩展）：租户解析 + gorm 查询隔离插件（两服务都挂）
- `pkg/sms`、`pkg/social`（新建）：可插拔 Provider + mock（主要在 member 侧使用）

**全局枚举 `UserType`**（放 `pkg/token`，避免循环依赖）：

```go
type UserType int8
const (
    UserTypeAdmin  UserType = 1 // 管理员 → system_users
    UserTypeMember UserType = 2 // 会员   → member_user
)
```

### 2.5 双运行模式（单体一键启动 保留，已确认）

> 本项目立身之本是"模块化单体、统一启动"——开发/CI/demo **不得**被迫起两个进程。
> 切换机制沿用现有 `fx.Decorate` 模式（与 `UserService` 本地↔gRPC 切换同构），**同一套模块代码，只换注入**。

```
单体模式（默认，deployment.mode: mono）         双服务模式（部署形态，deployment.mode: micro）
┌────────────────────────────────────┐   ┌──────────────────────────────────────────┐
│ alexgo-server 单进程 :8080          │   │ system-server :8080 (+gRPC :50051)        │
│ system + order + member 全部装配    │   │ member-server :8081（独立二进制）          │
│ Token：本地签发，不起 gRPC           │   │ Token：member 经 gRPC 委托（Decorate 注入） │
│ 路由：/api/** 全在 8080             │   │ 分流：vite/Ingress 按前缀                 │
│ vite proxy：全部指向 8080，不分流    │   │                                            │
└────────────────────────────────────┘   └──────────────────────────────────────────┘
```

| | 单体模式（mono，**默认**） | 双服务模式（micro） |
| --- | --- | --- |
| 启动 | `make run` 一条命令 | `make run-system` + `make run-member` |
| gRPC TokenService | 不启动；`fx.Decorate` 把 `TokenIssuer` 换成本地 `pkg/token.Service` | member-server 连 `system_grpc_addr` |
| 路由 | 全部 `:8080`，`/api/app/member/**` 等无需分流 | 前缀分流（2.3） |
| 适用 | 本地开发、CI、demo、小规模生产 | 目标部署形态（k8s/helm 默认 micro） |

**接口切分**：`modules/member` 只依赖 `TokenIssuer` 接口——mono 进程注入本地实现，member-server 进程注入 gRPC client（`pkg/client`）。两实现共享 `pkg/token` 的请求/响应结构。

**约定**：
- `deployment.mode` 默认 `mono`；helm/k8s/compose 显式 `micro`
- mono 模式下 `modules/member/cmd/main.go` 不参与，但代码始终编译在同一模块内（`go build ./...` 覆盖）
- 单体模式也跑同一套 Token 表与租户隔离（行为一致，只有进程拓扑不同）

## 3. 数据模型

> 命名说明：采用参考图中的 yudao 表名（`system_` 前缀、`member_user` 单数）。
> 现有 `users` 表**重命名为 `system_users`** 并补齐参考图字段；项目原无前缀风格就此切换到 yudao 风格（新表全部带前缀），存量表（roles/menus/depts…）本期不重命名，避免大范围改动。
>
> 审计列说明：按参考图补齐 `creator/create_time/updater/update_time/deleted`——`deleted` 用 gorm 软删（`gorm.DeletedAt`），`creator/updater` 由中间件从登录上下文注入（现有 OperateLog 中间件已有相同数据源）。

### 3.1 `system_users`（迁移自 `users` + 补列）

```sql
-- 迁移：2026xxxx_000001_user_system.up.sql
RENAME TABLE `users` TO `system_users`;
ALTER TABLE `system_users`
  ADD COLUMN `remark`    VARCHAR(500) NOT NULL DEFAULT '' COMMENT '备注' AFTER `nickname`,
  ADD COLUMN `dept_id`   BIGINT       NOT NULL DEFAULT 0   COMMENT '部门ID' AFTER `remark`,
  ADD COLUMN `post_ids`  VARCHAR(255) NOT NULL DEFAULT ''  COMMENT '岗位ID数组' AFTER `dept_id`,
  ADD COLUMN `email`     VARCHAR(50)  NOT NULL DEFAULT ''  COMMENT '邮箱' AFTER `post_ids`,
  ADD COLUMN `mobile`    VARCHAR(11)  NOT NULL DEFAULT ''  COMMENT '手机号' AFTER `email`,
  ADD COLUMN `sex`       TINYINT      NOT NULL DEFAULT 0    COMMENT '性别 0未知 1男 2女' AFTER `mobile`,
  ADD COLUMN `avatar`    VARCHAR(100) NOT NULL DEFAULT ''   COMMENT '头像' AFTER `sex`,
  ADD COLUMN `login_ip`  VARCHAR(50)  NOT NULL DEFAULT ''   COMMENT '最近登录IP' AFTER `status`,
  ADD COLUMN `login_date` DATETIME    NULL COMMENT '最近登录时间' AFTER `login_ip`,
  ADD COLUMN `deleted`   BIT(1)       NOT NULL DEFAULT 0    COMMENT '是否删除' AFTER `login_date`,
  ADD COLUMN `creator`   VARCHAR(64)  NOT NULL DEFAULT ''   COMMENT '创建者' AFTER `deleted`,
  ADD COLUMN `updater`   VARCHAR(64)  NOT NULL DEFAULT ''   COMMENT '更新者' AFTER `creator`;
-- password_hash 列名保留（不改成 yudao 的 password，减少代码改动）
```

### 3.2 `member_user`（新建）

```sql
CREATE TABLE IF NOT EXISTS `member_user` (
  `id`          BIGINT       NOT NULL AUTO_INCREMENT,
  `nickname`    VARCHAR(30)  NOT NULL DEFAULT '' COMMENT '用户昵称',
  `avatar`      VARCHAR(255) NOT NULL DEFAULT '' COMMENT '用户头像',
  `status`      TINYINT      NOT NULL DEFAULT 0  COMMENT '用户状态 0开启 1停用',
  `mobile`      VARCHAR(11)  NOT NULL DEFAULT '' COMMENT '用户手机号（登录账号）',
  `password`    VARCHAR(100) NOT NULL DEFAULT '' COMMENT '密码（bcrypt，可为空=未设密码）',
  `register_ip` VARCHAR(32)  NOT NULL DEFAULT '' COMMENT '注册IP',
  `login_ip`    VARCHAR(50)  NOT NULL DEFAULT '' COMMENT '最近登录IP',
  `login_date`  DATETIME     NULL COMMENT '最近登录时间',
  `deleted`     BIT(1)       NOT NULL DEFAULT 0  COMMENT '是否删除',
  `creator`     VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '创建者',
  `create_time` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updater`     VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '更新者',
  `update_time` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `tenant_id`   BIGINT       NOT NULL DEFAULT 0  COMMENT '租户编号',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_mobile_tenant` (`mobile`, `tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='会员用户表';
```

> `(mobile, tenant_id)` 唯一：同一手机号在不同租户可注册不同会员（SaaS 隔离）。

### 3.3 `system_oauth2_access_token`（新建，参考图原样）

```sql
CREATE TABLE IF NOT EXISTS `system_oauth2_access_token` (
  `id`            BIGINT       NOT NULL AUTO_INCREMENT,
  `user_id`       BIGINT       NOT NULL COMMENT '用户编号',
  `user_type`     TINYINT      NOT NULL COMMENT '用户类型 1管理员 2会员',
  `access_token`  VARCHAR(255) NOT NULL COMMENT '访问令牌',
  `refresh_token` VARCHAR(32)  NOT NULL DEFAULT '' COMMENT '刷新令牌',
  `client_id`     VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '客户端编号',
  `scopes`        VARCHAR(255) NOT NULL DEFAULT '' COMMENT '授权范围',
  `expires_time`  DATETIME     NOT NULL COMMENT '过期时间',
  `deleted`       BIT(1)       NOT NULL DEFAULT 0,
  `creator`       VARCHAR(64)  NOT NULL DEFAULT '',
  `create_time`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updater`       VARCHAR(64)  NOT NULL DEFAULT '',
  `update_time`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `tenant_id`     BIGINT       NOT NULL DEFAULT 0 COMMENT '租户编号',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_access_token` (`access_token`),
  UNIQUE KEY `uk_refresh_token` (`refresh_token`),
  KEY `idx_user` (`user_id`, `user_type`),
  KEY `idx_expires` (`expires_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='OAuth2 访问令牌表';
```

### 3.4 `tenants`（新建，SaaS 租户）

```sql
CREATE TABLE IF NOT EXISTS `tenants` (
  `id`            BIGINT      NOT NULL AUTO_INCREMENT,
  `name`          VARCHAR(64) NOT NULL COMMENT '租户名称',
  `package_id`    BIGINT      NOT NULL DEFAULT 0  COMMENT '租户套餐编号（预留）',
  `status`        TINYINT     NOT NULL DEFAULT 0  COMMENT '状态 0开启 1停用',
  `expire_time`   DATETIME    NULL COMMENT '过期时间（NULL=永久）',
  `account_limit` INT         NOT NULL DEFAULT -1 COMMENT '账号额度 -1不限',
  `domain`        VARCHAR(64) NOT NULL DEFAULT '' COMMENT '绑定域名（登录解析用，可空）',
  `deleted`       BIT(1)      NOT NULL DEFAULT 0,
  `creator`       VARCHAR(64) NOT NULL DEFAULT '',
  `create_time`   DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updater`       VARCHAR(64) NOT NULL DEFAULT '',
  `update_time`   DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_domain` (`domain`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='租户表';
```

> 说明：`tenants` 自身**不带 tenant_id**（全局表，超级管理员/平台侧管理）。停用租户登录时拒绝。

### 3.5 `social_user`（三期·三方绑定，新建）

```sql
CREATE TABLE IF NOT EXISTS `social_user` (
  `id`          BIGINT      NOT NULL AUTO_INCREMENT,
  `user_id`     BIGINT      NOT NULL COMMENT '本系统用户编号',
  `user_type`   TINYINT     NOT NULL COMMENT '用户类型 1管理员 2会员',
  `social_type` TINYINT     NOT NULL COMMENT '社交类型 1微信 2QQ 3钉钉…',
  `openid`      VARCHAR(64) NOT NULL COMMENT '三方平台 openid/uid',
  `nickname`    VARCHAR(64) NOT NULL DEFAULT '',
  `deleted`     BIT(1)      NOT NULL DEFAULT 0,
  `creator`     VARCHAR(64) NOT NULL DEFAULT '',
  `create_time` DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updater`     VARCHAR(64) NOT NULL DEFAULT '',
  `update_time` DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `tenant_id`   BIGINT      NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_type_openid` (`social_type`, `openid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='社交用户绑定表';
```

### 3.6 短信验证码存储

验证码**存 Redis**（`sms:code:{tenant}:{mobile}`，TTL 5 分钟，含发送频控计数），不建表——与项目现有 Redis 能力对齐；Redis 未启用时降级为进程内 map（开发模式，配置开关 `sms.mode`）。

### 3.7 `roles` 增强（块⑦ 权限管理对齐）

> 现有 `roles` 仅 code/name/status/tenant。对照 yudao `system_role` 补齐（表名是否加 `system_` 前缀随 §9.1 一并决定，本节以 `roles` 现名做 ALTER，重命名只做一次）：

```sql
ALTER TABLE `roles`
  ADD COLUMN `sort`                 INT          NOT NULL DEFAULT 0   COMMENT '显示顺序' AFTER `name`,
  ADD COLUMN `data_scope`           TINYINT      NOT NULL DEFAULT 1   COMMENT '数据范围 1全部 2自定义 3本部门 4本部门及以下 5仅本人' AFTER `sort`,
  ADD COLUMN `data_scope_dept_ids`  VARCHAR(500) NOT NULL DEFAULT ''  COMMENT '自定义数据范围部门ID数组' AFTER `data_scope`,
  ADD COLUMN `type`                 TINYINT      NOT NULL DEFAULT 2   COMMENT '角色类型 1系统内置 2自定义' AFTER `data_scope_dept_ids`,
  ADD COLUMN `remark`               VARCHAR(500) NOT NULL DEFAULT ''  COMMENT '备注' AFTER `type`,
  ADD COLUMN `deleted`              BIT(1)       NOT NULL DEFAULT 0   COMMENT '是否删除' AFTER `remark`,
  ADD COLUMN `creator`              VARCHAR(64)  NOT NULL DEFAULT ''  COMMENT '创建者' AFTER `deleted`,
  ADD COLUMN `updater`              VARCHAR(64)  NOT NULL DEFAULT ''  COMMENT '更新者' AFTER `creator`;
-- created_at/updated_at 保留现有列名（不改 yudao 的 create_time，减少存量代码改动）
```

`menus` 已具备 `type`(dir/menu/button) + `permission`，与 yudao `menu_type` 菜单/操作二分语义等价，**不改结构**；仅补审计列：

```sql
ALTER TABLE `menus`
  ADD COLUMN `deleted` BIT(1)      NOT NULL DEFAULT 0  COMMENT '是否删除' AFTER `status`,
  ADD COLUMN `creator` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '创建者' AFTER `deleted`,
  ADD COLUMN `updater` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '更新者' AFTER `creator`;
```

`casbin_rule` 结构不变（租户隔离通过策略主体改造实现，见 4.7，无需加列）。

## 4. 分块设计

### 4.1 块① 双用户架构

**路由**（延续现有 `/api/admin`、`/api/app` 分离）：

| 端 | 接口 | 说明 |
| --- | --- | --- |
| admin | `POST /api/admin/system/users` 等现有用户 CRUD | 不变，落 `system_users` |
| admin | `GET/POST/PUT/DELETE /api/admin/system/tenants` | 租户管理（见 4.3） |
| admin | `GET /api/admin/member/users` | 会员列表/禁用/重置密码（新增，包在 member 模块的 admin controller） |
| app | `POST /api/app/system/auth/login` | 管理员账号密码登录（改造为 Token 签发） |
| app | `POST /api/app/member/auth/*` | 会员侧全部登录入口（新增） |

**代码落位**：
- `modules/member/`（**member-server 服务的业务模块**）：model/member_user.go、repository、service（Register/Login/SmsLogin/SocialLogin…）、controller/admin（会员管理）、controller/app（登录入口）、module.go（Fx 装配 + 路由注册 + migration source）、`cmd/main.go`（独立进程：HTTP :8081 + 中间件 + token gRPC 客户端注入）
- `modules/system`：User 模型补字段、Admin 用户 CRUD 扩展（dept/post/email/sex 等）
- **会员管理接口归属**：`/api/admin/member/**` 由 member-server 承载（模块自持 admin+app 两端路由，与现有 module 模式一致）；admin-web 按 2.3 前缀分流（`/api/admin/member` 也走 :8081）
- 现有 seed（admin/admin123）逻辑迁移至 `system_users`

**双端登录返回统一结构**：`{ token, refresh_token, expires_in }`。

### 4.2 块② OAuth2 Token 体系

**决策：从"静态密钥 JWT"切换为"不透明 Token + DB 校验"（yudao 同款）**，理由：
- 支持**即时注销/踢人**（DB 删行即失效），JWT 做不到
- `user_id + user_type + tenant_id` 集中在一张表，审计/查询方便
- 代价：每次鉴权查 DB → ~~中间件加**进程内 LRU 缓存（60s TTL）**缓解，预留 Redis 升级位~~（已裁决移除，见偏差注）

**流程（双服务形态，方案 A）**：
1. **签发（权威在 system-server）** `pkg/token.Service`：生成随机 `access_token`（crypto/rand 32 字节 base64url，DB 存明文——与参考图一致，依赖 DB 访问控制）、`refresh_token`（32 位随机）、写 `system_oauth2_access_token`，默认 access 2h、refresh 7d（配置化）；`client_id` 固定——admin 侧 `alexgo-admin`、app 侧 `alexgo-app`，`scopes` 暂置 `all`
   - **system-server 本地登录**（管理员账号密码等）：进程内直接调 `token.Service`（`fx.Decorate` 注入，不走自环 gRPC）
   - **member-server 登录**（账号/短信/三方/小程序验证通过后）：gRPC 调 `TokenService.IssueToken(user_type=2, …)` → 返回 token 对；gRPC 失败 → 登录接口 503（不降级）
2. **校验（两服务各自本地做）**：`AuthMiddleware` 查共享 `system_oauth2_access_token` 表（~~经 LRU 缓存，60s~~ 已裁决移除缓存恒查库，见偏差注）→ 得 `(user_id, user_type, tenant_id)` → 按 user_type 加载对应用户表 → 注入 context（`claims` 扩展 `user_type`）→ Casbin 仅 system-server 启用
   - 依赖约束：一期两服务连**同一个 MySQL**（分进程不分库）；未来真分库时把校验替换为 gRPC introspection（`ValidateToken` 预留，不在一期）
3. **刷新**：`POST /api/app/system/auth/refresh`（system 本地）与 `POST /api/app/member/auth/refresh`（member → gRPC `RefreshToken`）
4. **注销/踢人**：统一落 system-server 权威——admin 侧踢人删行；member 的 logout 走 gRPC `RevokeToken`
5. **过期清扫**：system-server 内复用 Outbox Relay 式 ticker，每小时删 `expires_time < now - 7d` 的行
6. **兼容期**：`pkg/auth/jwt.go` 保留（测试覆盖）；配置 `auth.mode: token | jwt`（默认 token，回滚开关）

### 4.3 块③ 多租户 SaaS

**三个子能力**：

1. **租户管理**（admin 侧 CRUD + 停用/启用）：停用 → 该租户所有 Token 校验失败、登录拒绝；`account_limit` 控制 `system_users + member_user` 总数（创建用户时校验，简化版套餐）
2. **登录租户解析**（`pkg/tenant` 升级，优先级从高到低）：
   - 显式 `X-Tenant-ID` 头（现有，App/开发用）
   - 登录请求按 `Host` 域名匹配 `tenants.domain`（SaaS Web 端用）
   - 都没有 → **平台租户**（id=1，单租户部署模式，等价现状）
   - 签发的 Token 携带 `tenant_id`，后续请求以 Token 内租户为准（头与 Token 冲突时以 Token 为准）
3. **查询级隔离（gorm 插件）**——`pkg/tenant/gormplugin`：
   - 挂 `gorm.Callbacks`：INSERT 自动填 `tenant_id`（从 ctx 取）；SELECT/UPDATE/DELETE 自动追加 `WHERE tenant_id = ?`
   - **豁免白名单**：`tenants` 表、`casbin_rule`、迁移/seed（按表名匹配）；无 `tenant_id` 列的存量表自动跳过（回调里检测列元数据）
   - ctx 取值：`TenantMiddleware` 解析后写入 `context.Context`，仓储层 `WithContext(ctx)` 贯穿（现有 repository 模式已传 ctx，改动集中在插件）
   - 现有"手动不带 tenant_id 条件"的查询行为不变（插件补的是隔离下限）
   - **单测策略**：sqlite 内存库驱动插件，断言注入的 SQL/结果隔离

**非目标确认**：跨租户超级管理（平台侧查全部）通过豁免白名单 + 显式 `IgnoreTenant()` API 提供，不做自动切换。

### 4.4 块④ 短信登录（二期）

- `pkg/sms`：`Provider interface { Send(ctx, mobile, code, scene) error }`，实现：`MockProvider`（日志打印，dev 默认）+ `HTTPProvider`（通用短信网关，配置 URL/签名/模板，接阿里云/腾讯云需各自适配层，本期只做通用 HTTP + Mock）
- 验证码：6 位数字，Redis TTL 5min，同号 60s 频控 + 每日上限（复用现有 `pkg/limiter` 思路独立计数）
- `POST /api/app/member/auth/send-sms-code`（场景 login/register）
- `POST /api/app/member/auth/sms-login`：验证码通过 → 按 `(mobile, tenant_id)` 查 `member_user` → **不存在则自动注册**（默认 nickname=`用户{尾4位}`、status=0、记 register_ip）→ 签发 Token
- 管理端可选：`POST /api/admin/system/auth/sms-login`（配置开关，默认关）

### 4.5 块⑤ 三方登录（三期）

- `pkg/social`：`Provider interface { GetAuthorizeURL(type, redirect) (string, error); ExchangeCode(ctx, type, code) (openid, nickname, err) }`，实现：`MockProvider`（dev）+ `WeChatProvider`/`QQProvider`（HTTP 直连，配置 appid/secret）
- 三步流程（yudao 同款）：
  1. `GET .../auth/social-auth-redirect?type=&redirectUri=` → 返回授权 URL
  2. `POST .../auth/social-login {socialType, socialCode, socialState}` → ExchangeCode 得 openid → 查 `social_user` 命中 → 直接签发 Token；未命中 → 返回 `need_bind`（前端转绑定）
  3. **绑定登录**：账号密码/短信登录接口附带 `socialType+socialCode` 参数时，登录成功后追加写 `social_user` 绑定
- openid 全局唯一键保证一人一号；`user_type` 区分绑定的是管理员还是会员

### 4.6 块⑥ 微信小程序登录（三期，依赖④⑤）

- `POST /api/app/member/auth/mini-app-login {code, phoneCode}`
- `code` → `jscode2session`（微信 HTTP API，`pkg/social/wechat` 复用）得 openid/session_key；`phoneCode` → `getPhoneNumber` 得 mobile
- mobile 已注册 → 登录；未注册 → 自动注册（同 4.4）→ 签发 Token
- 配置：`wechat.miniapp.appid/secret`；无配置时 Mock（返回固定测试号）

### 4.7 块⑦ 权限管理对齐（一期）

> 现状核查：菜单/按钮树、`permission` 码、前端路由下发均可用；但 **`AssignMenus` 只写 `role_menus` 不同步 Casbin**，种子只给默认 admin 角色一条通配策略——**新建角色后端全 403**；且 `casbin_rule` 无租户维度、角色 code 仅租户内唯一 → 跨租户策略串用。

**三件事**：

1. **role_menus → Casbin 策略同步**（修复断层）
   - 同步单元：以**菜单的路由模板 + method** 生成 `p`：`v0={roleCode}`、`v1=路由模板`（如 `/api/admin/system/users`，取菜单关联的 API 路由或 `permission` 码映射表）、`v2=GET|POST|...`（`.*` 起步，按路由注册表精确化）
   - **实现方案（简化且与现有模型匹配）**：Casbin 继续只管**后端 API 强制**，策略由"角色 ↔ 菜单权限码"驱动——`AssignMenus`/角色删除/菜单删除时触发 `rebuildRolePolicies(roleID)`：删旧 `p` → 按当前 `role_menus` 全量重建
   - **路由映射**：维护 `permission 码 → (route template, method)` 映射（与 `module.go` 路由注册对照生成；一期用"菜单 permission 前缀 → 路由前缀"的 keyMatch2 规则，如 `system:user` → `/api/admin/system/users`），种子/迁移时初始化
   - 触发点：`AssignMenus`、`Create/Delete Role`、`Create/Delete Menu`（事务提交后）
2. **`data_scope` 数据权限**（查询层注入，依赖块③ gorm 插件）
   - 校验顺序：登录后把 `(userID, deptID, roleDataScopes)` 写入 ctx；仓储查询回调按"最宽松角色"拼条件——1 全部不加条件；2 自定义 → `dept_id IN (data_scope_dept_ids)`；3 `dept_id = 我的部门`；4 `dept_id IN (本部门及以下，按 depts.parent 链)`；5 `id = 我`
   - 白名单：`dept_id` 不存在的表、豁免表（同块③豁免机制追加 `data_exempt` 标记）
   - 粒度：**只对带 `dept_id` 的业务表生效**（一期=system_users；order 等业务表后续按需挂）
3. **Casbin 租户隔离**（消除串策略）
   - 策略主体改造：`sub` 从裸 `username/roleCode` 改为 **`{tenantId}:{userType}:{username}` / `{tenantId}:{roleCode}`**
     （user_type 维度为 C1 修正：member 昵称与管理员用户名可同名，缺维度时 `g(x,x)` 恒等即提权）
   - 影响点：`EnsureUserRolePolicy`（登录时 g 绑定）、种子 `AddPolicy`、`AuthMiddleware` 组装 sub（从 Token 的 tenant_id + user_type + username 拼接）、存量 `casbin_rule` 数据迁移（启动时重建：清空 + 按 role_menus 重灌）
   - Casbin model 的 matcher 不变（`g`/`keyMatch2`/`regexMatch` 照旧）

**角色表补列**（§3.7 DDL）随本块的迁移一并执行；`type=1` 系统内置角色禁止删除（service 层校验）。

## 5. 配置扩展（config.yaml）

```yaml
deployment:
  mode: mono             # mono=单体一键启动（默认）；micro=双服务（member 独立进程+gRPC）

server:
  http_addr: ":8080"     # member-server 用 ":8081"（modules/member/cmd 独立配置）
  grpc_addr: ":50051"    # system-server 的 TokenService 监听地址（仅 micro 模式启动）

# member-server → system-server 的 gRPC 直连地址（服务发现一期用静态地址，
# registry consul:// 为演进项）
system_grpc_addr: "127.0.0.1:50051"

auth:
  mode: token            # token | jwt（回滚开关）
  access_expire_hour: 2
  refresh_expire_day: 7

tenant:
  resolve: header_domain # header 仅X-Tenant-ID；domain 启用域名解析；header_domain 两者都开
  default_id: 1          # 无解析结果时的平台租户

sms:
  provider: mock         # mock | http
  mode: redis            # redis | memory（Redis 未启用时自动降级 memory，供开发环境）
  expire_minute: 5
  daily_limit: 10

social:
  provider: mock         # mock | real
  wechat:
    appid: ""
    secret: ""
  miniapp:
    appid: ""
    secret: ""
```

## 6. 认证与授权影响面

| 现有件 | 变化 |
| --- | --- |
| `AuthMiddleware` | 校验源 JWT → Token 表（~~L1 LRU 缓存~~ 已裁决移除缓存恒查库，见偏差注）；claims 增加 `user_type`；`auth.mode=jwt` 可回滚；**sub 改为 `{tenantId}:{userType}:{username}`（块⑦ + C1 修正）**；`/api/admin/**` 仅管理员（C1 门槛）；Token 内租户覆盖 `X-Tenant-ID` 头（spec §4.3） |
| Casbin | **模型 matcher 不变**，但策略主体带租户前缀；`EnsureUserRolePolicy`、种子、存量 `casbin_rule` 迁移时全量重建；`AssignMenus` 等触发策略同步（块⑦） |
| `OperateLog` / `Audit` | `creator/updater` 自动注入（补列后可真实落库） |
| 现有 JWT 测试（pkg/auth 7包之一） | 保留不删；`pkg/token` 新增独立测试包 |
| admin-web 登录 | 响应结构变化：`{token, refresh_token, expires_in}` → 前端 `stores/auth.ts` 适配 |
| admin-web 代理 | vite proxy 按前缀分流：`/api/app/member`、`/api/admin/member` → :8081，其余 → :8080（K8s Ingress 同规则） |
| 部署清单 | compose/k8s/helm 从 1 Deployment 变 2（system/member 各自 Service；helm values 增 `member.*` 段）；member-server 无 Casbin/审计中间件 |

## 7. 测试策略（沿用项目铁律：进程内、零外部依赖）

- `pkg/token`：签发/校验/过期/刷新/注销/多租户隔离 + 并发签发唯一性
- `pkg/tenant/gormplugin`：sqlite 内存库断言 INSERT 填充、SELECT/UPDATE/DELETE 的 tenant_id 注入、白名单豁免、无列跳过
- `modules/member`：注册/登录/sms-login 自动注册（Mock SMS，Redis 用 miniredis 或进程内降级模式）
- `pkg/sms`：Mock Provider + 频控；`pkg/social`：Mock Provider 全流程
- `AuthMiddleware`：Token 模式 401/200/踢人失效/租户不符 403
- 块⑦：`AssignMenus` 后 Casbin 策略重建断言（角色获得/失去权限立即生效）、跨租户同 code 角色策略不串（sub 前缀）、`data_scope` 五种范围各一条 sqlite 查询断言、系统内置角色删除被拒
- 双服务链路：TokenService gRPC 服务端单测 + member-server 侧 client 用 bufconn/内存 gRPC mock（IssueToken 成功/失败→503）；两服务对同一 token 的校验结果一致
- 全量 `go test ./...`（CI 加 `-race` 已就位）

## 8. 实施分期（计划按期拆分）

| 期 | 内容 | 交付判定 |
| --- | --- | --- |
| **一期** | 块①②③⑦ + **双服务骨架 + 双运行模式**：三表迁移 + member 模块 + member-server 独立进程（cmd/main、:8081、vite/Ingress 分流）+ gRPC TokenService（Issue/Refresh/Revoke）+ `TokenIssuer` 接口（mono/micro 双注入）+ Token 体系 + 租户表/解析/隔离插件 + 双端登录改造 + 租户/会员管理接口 + **权限管理对齐（Casbin 策略同步 / data_scope / 租户隔离 / role 补列）** | **单体模式 `make run` 一条命令全起（默认）**；双服务模式两进程各自可启动且 member 经 gRPC 委托签发；双端登录走 Token 表；两租户数据互不可见且策略不串；踢人立即失效；**新角色分配菜单后端 API 立即可用**；全测试绿 |
| **二期** | 块④：SMS Provider + 验证码 + 短信登录/自动注册 | Mock 下全流程通过；频控生效 |
| **三期** | 块⑤⑥：social Provider + 绑定/快登录 + 小程序登录 | Mock 下三方与小程序全流程通过 |

> 每期独立走 writing-plans → 子代理实施 → 审查 → 合并 `main`（本项目不建分支）。

## 9. 风险与开放决策（评审重点）

1. **`users` → `system_users` 重命名**：波及现有 seed/仓储/测试的表名——用迁移一次性完成，代码 `TableName()` 改一处；**是否接受切到 yudao 表名风格**（存量表不改名）
2. **Token 明文入库**：参考图/ yudao 即如此，依赖 DB 访问控制；是否要加盐哈希（会偏离参考图，查询需先哈希索引）
3. **JWT → 不透明 Token** 是行为级变更：admin-web 前端要适配新响应结构；`auth.mode=jwt` 保留回滚
4. **gorm 隔离插件误伤面**：白名单/无列跳过的边界要靠测试钉死（一期最高风险点）
5. **SMS/微信真实凭证**：本期只交付 Provider 接口 + Mock + 通用 HTTP 实现，真实厂商适配留配置接入
6. **`account_limit` 额度**：只做"创建用户时计数校验"，不做套餐/续费（预留 package_id）
7. **Casbin sub 改造（块⑦）**：影响登录 g 绑定、种子策略、存量 `casbin_rule`——迁移采用"启动时清空 + 按 role_menus 重灌"策略，需确认可接受（现网 casbin_rule 仅种子数据，风险低）
8. **策略生成粒度（块⑦）**：一期 `permission 前缀 → 路由前缀 keyMatch2`（粗粒度、实现快），二期可按路由注册表精确到 method——需要确认接受一期粒度
9. **无网关的路径分流（仅 micro 模式）**：`/api/app/member`、`/api/admin/member` 两个前缀必须在 vite proxy、Ingress、compose 三处保持一致——漂移会导致 404；**mono 模式不分流故开发期无此风险**；网关列入二期以后演进
10. **同库约束（仅 micro 模式）**：两服务共享一个 MySQL（分进程不分库），`system_oauth2_access_token` 的读写一致性依赖单库；真分库时需把 member 侧校验切到 gRPC introspection（`ValidateToken` 预留）
11. **gRPC 故障语义（仅 micro 模式）**：system-server 不可达时 member-server 登录/刷新 fail-fast 503（有限重试后放弃），**不降级本地签发**——需要确认接受该可用性取舍；mono 模式无此问题（本地直调）
12. **双模式行为一致性**：mono/micro 只许有"进程拓扑"差异，不许有业务行为差异——用同一组测试跑两种注入方式（表驱动/接口 fake）钉住，防止只在一种模式下工作

## 10. 里程碑

- M1（一期）：双微服务骨架（system-server / member-server）+ 双用户 + Token（gRPC 委托签发）+ 多租户 + 权限管理对齐（块⑦）落地，admin-web 分流与登录适配完成
- M2（二期）：短信登录全链路
- M3（三期）：三方 + 小程序登录
- 每个 M 完成即合并 `main` 并更新 README/架构文档对应章节
