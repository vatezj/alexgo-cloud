# alexGo-cloud 用户体系（SaaS 多租户 + 全登录方式）设计文档

> **状态**：待评审（Review Draft）
> **参考**：yudao-cloud《用户体系》文档 + 用户提供的三表结构图（system_users / system_oauth2_access_token / member_user）
> **基线代码**：`main` @ `ffadd5a`（2026-10-08）

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

1. **双用户架构**：AdminUser / MemberUser 分表存储，`user_type` 统一区分
2. **OAuth2 Token 体系**：统一令牌表签发/刷新/注销，双端 Bearer 认证
3. **SaaS 多租户**：租户表 + 登录租户解析 + 查询级数据隔离 + 租户管理
4. **短信登录**：验证码发送（可插拔 Provider）、短信登录 + 未注册自动开通会员
5. **三方登录**：授权跳转 / code 快登录 / 绑定登录（可插拔 Provider）
6. **微信小程序登录**：phoneCode + loginCode 一键登录

### 1.3 非目标（本期不做）

- 数据库级多租户（每租户独立库，yudao 的"数据库隔离"方案）——本期只做**字段隔离**
- 统一会员/员工档案（yudao 也未做，双方表保持独立）
- 支付、商城等 member 侧业务功能
- OAuth2 **授权码模式**对外开放（`/oauth2/authorize` 给第三方应用授权）——本期 Token 体系只覆盖**本系统自家客户端**的登录签发
- Redis 缓存 Token（先直查 DB，预留缓存位）

## 2. 总体架构

```
                        ┌──────────────────────────────────────────┐
   管理后台 (admin-web) │  /api/admin/**                          │
                        │  AuthMiddleware(JWT/Token+Casbin)       │
   用户 App / 小程序     │  TenantMiddleware(租户解析)              │
   /api/app/**          └──────────────────────────────────────────┘
                                     │
             ┌───────────────────────┼───────────────────────┐
             ▼                       ▼                       ▼
      system 模块               member 模块             pkg 层公共能力
   AdminUser 管理            MemberUser 注册/登录      token(签发/校验/刷新)
   租户管理(CRUD)            短信登录(自动注册)          tenant(解析+gorm隔离插件)
   RBAC / 审计               三方绑定 / 小程序登录      sms / social 可插拔Provider
             │                       │                       │
             └───────────────────────┴───────────────────────┘
                                     ▼
                    system_users / member_user / system_oauth2_access_token / tenants
                    （user_id + user_type + tenant_id 三维度贯穿）
```

**模块落位**：
- `modules/system`（扩展）：管理员用户增强、租户管理、Token 签发的 admin 侧
- `modules/member`（新建）：会员用户、短信登录/注册、三方绑定、小程序登录
- `pkg/token`（新建）：Token 签发/校验/刷新/注销（两端共用）
- `pkg/tenant`（扩展）：租户解析 + gorm 查询隔离插件
- `pkg/sms`、`pkg/social`（新建）：可插拔 Provider 接口 + mock 实现

**全局枚举 `UserType`**（放 `pkg/token` 或 `pkg/utype`，避免循环依赖）：

```go
type UserType int8
const (
    UserTypeAdmin  UserType = 1 // 管理员 → system_users
    UserTypeMember UserType = 2 // 会员   → member_user
)
```

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
- `modules/member/`：model/member_user.go、repository、service（Register/Login/SmsLogin/SocialLogin…）、controller/admin（会员管理）、controller/app（登录入口）、module.go（Fx 装配 + 路由注册 + migration source）
- `modules/system`：User 模型补字段、Admin 用户 CRUD 扩展（dept/post/email/sex 等）
- 现有 seed（admin/admin123）逻辑迁移至 `system_users`

**双端登录返回统一结构**：`{ token, refresh_token, expires_in }`。

### 4.2 块② OAuth2 Token 体系

**决策：从"静态密钥 JWT"切换为"不透明 Token + DB 校验"（yudao 同款）**，理由：
- 支持**即时注销/踢人**（DB 删行即失效），JWT 做不到
- `user_id + user_type + tenant_id` 集中在一张表，审计/查询方便
- 代价：每次鉴权查 DB → 中间件加**进程内 LRU 缓存（60s TTL）**缓解，预留 Redis 升级位

**流程**：
1. **签发** `pkg/token.Service`：生成随机 `access_token`（crypto/rand 32 字节 base64url，DB 存明文哈希前缀校验——简化为 yudao 式直接存明文，依赖 DB 访问控制；spec 选**存明文**与参考图一致）、`refresh_token`（32 位随机）、写 `system_oauth2_access_token`，默认 access 2h、refresh 7d（配置化）；`client_id` 按端固定——admin 侧 `alexgo-admin`、app 侧 `alexgo-app`，`scopes` 暂置 `all`
2. **校验**：`AuthMiddleware` 改为查 Token 表（经 LRU）→ 得 `(user_id, user_type, tenant_id)` → 按 user_type 加载对应用户表 → 注入 context（`claims` 结构扩展 `user_type`）→ Casbin 照旧
3. **刷新**：`POST /api/app/**/auth/refresh`，refresh_token 换新 access（旧 access 作废）
4. **注销**：`POST .../auth/logout` 删行；admin 侧"踢人"= 按 user_id 删其全部 Token
5. **过期清扫**：复用 Outbox Relay 式后台 ticker，每小时删 `expires_time < now - 7d` 的行
6. **兼容期**：现有 JWT 中间件代码保留在 `pkg/auth/jwt.go`（测试已覆盖），`AuthMiddleware` 内切换为 Token 校验；配置 `auth.mode: token | jwt`（默认 token，回滚开关）

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

## 5. 配置扩展（config.yaml）

```yaml
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
| `AuthMiddleware` | 校验源 JWT → Token 表（L1 LRU 缓存）；claims 增加 `user_type`；`auth.mode=jwt` 可回滚 |
| Casbin | 不变（sub 仍是用户名/用户ID，admin 侧才有 RBAC；member 无权限模型，仅登录态） |
| `OperateLog` / `Audit` | `creator/updater` 自动注入（补列后可真实落库） |
| 现有 JWT 测试（pkg/auth 7包之一） | 保留不删；`pkg/token` 新增独立测试包 |
| admin-web 登录 | 响应结构变化：`{token, refresh_token, expires_in}` → 前端 `stores/auth.ts` 适配 |

## 7. 测试策略（沿用项目铁律：进程内、零外部依赖）

- `pkg/token`：签发/校验/过期/刷新/注销/多租户隔离 + 并发签发唯一性
- `pkg/tenant/gormplugin`：sqlite 内存库断言 INSERT 填充、SELECT/UPDATE/DELETE 的 tenant_id 注入、白名单豁免、无列跳过
- `modules/member`：注册/登录/sms-login 自动注册（Mock SMS，Redis 用 miniredis 或进程内降级模式）
- `pkg/sms`：Mock Provider + 频控；`pkg/social`：Mock Provider 全流程
- `AuthMiddleware`：Token 模式 401/200/踢人失效/租户不符 403
- 全量 `go test ./...`（CI 加 `-race` 已就位）

## 8. 实施分期（计划按期拆分）

| 期 | 内容 | 交付判定 |
| --- | --- | --- |
| **一期** | 块①②③：三表迁移 + member 模块骨架 + Token 体系 + 租户表/解析/隔离插件 + 双端登录改造 + 租户/会员管理接口 | 双端登录走 Token 表；两租户数据互不可见；踢人立即失效；全测试绿 |
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

## 10. 里程碑

- M1（一期）：双用户 + Token + 多租户落地，admin-web 登录适配完成
- M2（二期）：短信登录全链路
- M3（三期）：三方 + 小程序登录
- 每个 M 完成即合并 `main` 并更新 README/架构文档对应章节
