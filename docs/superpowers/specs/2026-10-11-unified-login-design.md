# 统一登录（对齐 yudao-cloud）· Design Spec

- 日期：2026-10-11
- 状态：待评审
- 参考：yudao-cloud 认证架构（Sa-Token 多终端 + OAuth2 机机/人机 + SSO）、RFC 6749、`go-oauth2/oauth2` v4

## 1. 背景与目标

alexGo-cloud 现有 3 套各自为政的登录：

| 端点 | 用途 | 所在模块 |
|---|---|---|
| `POST /api/auth/login` | 管理后台（vben 契约） | system |
| `POST /api/app/auth/login` | app 端 | system |
| member `auth/login` + refresh/logout | 会员（手机号+密码） | member |

底层是 `pkg/auth` 的 JWT + Casbin；仓库无 OAuth2/SSO/统一账号概念。yudao-cloud 的统一登录核心是：**一个认证层管多终端、账号存储分 system/member 两套、OAuth2 四种 grant 对外授权、SSO 打通**。

**目标**：搭一套标准 OAuth2 授权服务，同时承担**对内三端统一认证 + SSO** 与**对外第三方应用授权**；向 yudao 学协议形态与产品设计（它是 Java/Sa-Token，Go 无主流 Sa-Token 移植，不照搬运行时）。

### 已确认的需求基线

| 决策点 | 结论 |
|---|---|
| 总体范围 | 完整 OAuth2 授权服务 + SSO：三端（管理后台/app/member）统一认证、OAuth2 对外授权第三方（授权码为主） |
| 账号模型 | **认证统一、存储分开**——users 与 members 表零改动，认证层一套（学 yudao） |
| 登录方式 | 账号密码 + 图形验证码；**验证码是否必校验做成配置开关**（`auth.captcha.enabled`） |
| 技术路线 | 方案 B：`go-oauth2/oauth2` v4 做协议层，JWT（现有 `pkg/auth`）作 access token，Casbin 鉴权不动 |
| 内部 SSO | 管理后台走授权码 + PKCE；app/member 走密码模式（移动端不做 redirect，与 yudao 一致） |

### 非目标（YAGNI，明确不做）

- 不引入 Sa-Token / 不自研会话框架（JWT 无状态校验保留）
- 不合并 users/members 账号表
- 不做短信/扫码/微信登录（表结构不预留登录方式枚举之外的东西）
- 不做 scope 树（默认 `all` 单值）、不做 refresh token 轮转、不做设备指纹
- 不做 access token 的服务端即时吊销（靠短 TTL 收敛，见 §4）

## 2. 总体架构与组件边界

```
                          ┌────────────────────────────┐
   第三方应用 ──授权码/机机──▶│  modules/oauth2（新模块）    │
   (外部系统)              │  · /oauth2/authorize      │
                          │  · /oauth2/token          │
   管理后台 SPA ──登录/SSO──▶│  · /oauth2/authorize/login│──▶ 签发 JWT（复用 pkg/auth）
   app 端 ───密码模式──────▶│  · client 管理 CRUD        │      │
   member 端 ──密码模式────▶│  · 验证码 + 开关           │      ▼
                          └───────┬────────────┬───────┘   Casbin 鉴权不变
                                  │            │
                     账号校验接口（Go 接口）      │
                    ┌─────────────┴──────┐    │
              modules/system         modules/member
              UserAuthenticator      MemberAuthenticator
              (users 表)              (members 表)
```

**四条边界规则：**

1. **账号存储不动**：oauth2 模块不建账号表；凭据校验走 `UserAuthenticator`（system 实现）与 `MemberAuthenticator`（member 实现）两个 Go 接口，按 `account_type` 路由。users/members 表结构零改动。
2. **token 签发仍走 `pkg/auth`**：access token 保持 JWT 格式，网关中间件、Casbin 判定零改动；oauth2 只是「谁有权拿到这个 JWT」的新守门人。
3. **oauth2 模块自治**：client 表、授权码、refresh token 都在 `modules/oauth2/migrations`，fx 装配同 infra 模块模式（实现 `server.Module`，路由挂 `/api` 组）。
4. **老登录端点降级为兼容层**：vben/app/member 登录端点内部改调统一登录服务，响应结构不变；前端全迁后下线（计划单列任务）。

**库选型**：`github.com/go-oauth2/oauth2/v4`（协议层：四种 grant、code/refresh 生命周期）+ 其 `jwt` 子包对接现有 `pkg/auth` 密钥；存储层由本模块实现其 Store 接口，落到自己的 MySQL 表。

## 3. 数据模型与配置

`modules/oauth2/migrations`，四张表（对齐 yudao oauth2 系列表）：

### 3.1 `oauth2_client` — 接入方注册（第三方 + 内部端）

| 列 | 说明 |
|---|---|
| `client_id` / `client_secret` | 唯一；secret **bcrypt 哈希存储**，创建时明文只回显一次 |
| `client_name` / `website` | 接入方名称/主页 |
| `grant_types` | 逗号分隔白名单：`authorization_code,client_credentials,password,refresh_token` |
| `redirect_uris` | 授权码回跳白名单，多条换行分隔，**精确匹配** |
| `account_types` | 允许的账号类型：`system,member`（防 admin client 换会员 token） |
| `scopes` | 授权范围，默认 `all` |
| `access_token_ttl` / `refresh_token_ttl` | 秒；NULL 取全局默认 |
| `activity` | 启停开关 |

### 3.2 `oauth2_authorize_code` — 授权码

`code` 唯一 · `client_id` · `user_id` · `account_type` · `redirect_uri` · `scopes` · PKCE（`code_challenge` / `code_challenge_method` 可空）· `expires_at`（**2 分钟**）· `revoked`

### 3.3 `oauth2_access_token` / `oauth2_refresh_token` — 签发台账

`token` 唯一 · `client_id` · `user_id` · `account_type` · `scopes` · `expires_at` · `revoked` · `created_at`

### 3.4 配置 `configs/config.yaml`

```yaml
auth:
  captcha:
    enabled: true            # 图形验证码开关：false 时登录接口跳过校验
  access_token_ttl: 1800     # 秒，默认 30 分钟
  refresh_token_ttl: 2592000 # 秒，默认 30 天
  login_page: /login         # authorize 未认证时 302 的统一登录页
```

验证码挑战值存 Redis（复用 `pkg/appredis`），key `captcha:{uuid}`、TTL 2 分钟；`GET /oauth2/captcha` 无论开关都返回图像，开关只影响登录时是否校验。

## 4. 运行语义（token 执行与 SSO 时序）

### 4.1 token 语义（明确的折中）

- **access token = JWT，中间件无状态校验**（零改动）；`oauth2_access_token` 台账行服务于吊销查询/内省。
- **登出吊销 refresh token 即时生效**；access token 靠 30 分钟 TTL 自然过期。
- refresh 换发：换发时旧 refresh 立即吊销（一次性使用），签发新 refresh；不做旋转指纹等高级特性。

### 4.2 管理后台 SPA（授权码 + PKCE）

```
web-antd 无 token
 ① GET /oauth2/authorize?client_id&redirect_uri&state&code_challenge
    AS 校验 client 活跃、grant 白名单、redirect_uri 精确匹配
 ② 302 → auth.login_page（默认 /login，query 回带原参数）
    SPA 登录页 POST /oauth2/authorize/login（凭据+验证码[开关]+redirect_uri/state/challenge）
 ③ AS 校验通过 → 302 redirect_uri?code&state（浏览器直接跳）
 ④ SPA /callback 收 code → POST /oauth2/token
    grant_type=authorization_code + code_verifier → JWT + refresh
```

redirect_uri 不在白名单：**authorize 与 authorize/login 都直接 400，绝不 302**（防开放重定向）。

### 4.3 app 端 / member 端（密码模式）

```
POST /oauth2/token
  grant_type=password + client_id/client_secret
  + username & password & account_type=system|member
  + captcha_token（auth.captcha.enabled=true 时必传）
→ { access_token, refresh_token, expires_in, token_type }

刷新：grant_type=refresh_token（旧 refresh 吊销）
```

内部 client 注册约定：`app`（account_types=system）、`member_portal`（account_types=member），secret 由部署方在 client 管理里创建后配进前端。

### 4.4 老端点兼容层

| 老端点 | 改造 |
|---|---|
| `POST /api/auth/login`（vben 契约） | 内部走统一登录（account_type=system），响应结构不变 |
| system app 登录 | 同上 |
| member 登录/刷新/登出 | 改调统一服务 |

### 4.5 管理端配套

- client 管理 CRUD：`/api/admin/oauth2/clients`，权限码 `oauth2:client` 注册进 `permission_routes.go` + 菜单种子迁移（复用 codegen 种子模式，挂系统管理下）。
- 统一登录复用 `RecordLogin` 审计，来源标 oauth2。

## 5. API 一览

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/oauth2/captcha` | 取验证码图（uuid + base64 图） |
| GET | `/api/oauth2/authorize` | 授权码入口（校验 → 302 登录页或直接 400） |
| POST | `/api/oauth2/authorize/login` | 统一登录表单提交 → 302 回跳带 code |
| POST | `/oauth2/token` | 四种 grant 统一换 token（协议端点，RFC 6749 错误格式） |
| GET | `/api/oauth2/userinfo` | 按 token 取当前账号信息（第三方接入常用） |
| GET/POST/PUT/DELETE | `/api/admin/oauth2/clients*` | client 管理（admin 信封 `{data,status,error}`） |

> `/oauth2/token` 不带 `/api` 前缀，遵循 OAuth2 惯例路径；其挂载在 server `/api` 组之外的根组，实现时确认中间件链（限流/JWT 中间件不得要求该端点自带 token）。

## 6. 错误处理

**协议端点用 RFC 6749 格式**（`{"error","error_description"}`），与 admin 的 `{error}` 信封区分；admin 端点维持现有信封与 400/500 惯例。

| 场景 | 响应 |
|---|---|
| client 认证失败/停用 | 401 `invalid_client` |
| 授权码过期/已用/redirect_uri 不匹配 | 400 `invalid_grant` |
| 凭据错误 | 400 `invalid_grant` + 描述「账号或密码错误」（不区分账号不存在，防枚举） |
| 验证码缺失/错误/过期（开关开时） | 400 `captcha_invalid` |
| grant 不在 client 白名单 | 400 `unsupported_grant_type` |
| redirect_uri 不在白名单 | authorize 侧 400，**绝不 302** |

- 登录失败一律 `RecordLogin(success=false)` 审计。
- `/api/oauth2/authorize/login`、`/oauth2/token`、`/api/oauth2/captcha` 套用现有 Redis 限流中间件（`pkg/limiter`）防撞库。
- 兼容层把统一登录错误映射回各老契约的错误结构。

## 7. 测试策略（TDD）

**单元层**（`modules/oauth2/service`，sqlite 内存库 + 假 authenticator）：
- 验证码开关 true/false 两条路径（核心配置需求，RED 先行）
- account_type 路由与 client 白名单拒绝
- 授权码：过期/重放/redirect 不匹配/verifier 错误拒绝
- refresh 换发 + 旧吊销；登出吊销
- 兼容层：vben/app/member 三契约响应结构不变

**集成层**（`DB_DSN` + `-tags=integration`）：
- 迁移幂等：**直跑 SQL 文件双次执行**断言行数不变（M2 修复轮确立的模式，绕开 golang-migrate Up 只跑 pending 的真空坑）
- 端到端：注册 client → password grant 换 JWT → 调受 Casbin 保护的 admin API 通 → 授权码全流程 → redirect_uri 篡改 400

**回归红线**：现有 JWT 中间件/Casbin 测试、vben 登录契约测试必须全绿。

## 8. 实施里程碑（供 writing-plans 拆分）

- **M1**：模块骨架 + 迁移（四表）+ captcha（含开关）+ `UserAuthenticator`/`MemberAuthenticator` 接口与实现接线
- **M2**：`/oauth2/token` 四 grant + 登录服务 + refresh/登出吊销
- **M3**：authorize 授权码流 + SPA 登录页/回调改造（web-antd）+ 老端点兼容层切换
- **M4**：client 管理 CRUD + 权限码/菜单种子 + 集成端到端 + 兼容层下线评估

## 9. 勘误与实现裁决（writing-plans 前代码调研追加）

对照仓库现实逐条修正 §1–§8；**本节覆盖与前文冲突处**，实施计划（`docs/superpowers/plans/2026-10-11-unified-login.md`）按本节裁决执行。

| # | 原文 | 裁决 |
|---|---|---|
| D1 | §2 库选型 `go-oauth2/oauth2` v4 | **作废**：四种 grant + PKCE 的状态机足够薄，协议层手写；唯一新依赖 `github.com/mojocn/base64Captcha@v1.3.8`（验证码）。 |
| D2 | §4.1 "access=JWT，中间件零改动" | **token 模式（默认）**走 `pkg/token` 不透明令牌，中间件/Casbin 零改动；jwt 回滚模式由 system 提供 `JWTGenerator`（wrap `auth.GenerateToken`），统一登录在 `mode=jwt` 且 `account_type=system` 时签 JWT（无 refresh）。 |
| D3 | §5 `/oauth2/token` 不带 `/api` | 全部端点挂 **`/api/oauth2/*`**（Module 只拿 /api 组，公开路径规则已放行，无须动中间件链）。 |
| D4 | §3 四张表 | 只新建 **两表**：`oauth2_client`（无 `tenant_id`/`deleted` 列→租户插件跳过，硬删除）、`oauth2_authorize_code`（租户列 Go 字段名 `OwnerTenantID` 规避插件注入；含 `username` 列供 jwt 模式授权码流）。§3.3 签发台账**作废**——复用既有 `system_oauth2_access_token`（`pkg/token` 管理）。 |
| D5 | §3.4 TTL 1800s/30d、per-client TTL | **作废**：沿用 `auth.access_expire_hour=2` / `auth.refresh_expire_day=7`。config 只新增 `auth.captcha.enabled`（默认 false）与 `auth.login_page`（默认 **`/auth/login`**——vben `LOGIN_PATH` 实际值，原文 `/login` 有误）。 |
| D6 | §3.4 验证码存 `pkg/appredis` | 用 **`pkg/redis`**：redis 可用则 key `captcha:{uuid}` TTL 2 分钟，否则回退内存 store（`*redis.Client` 在 deps 标 `optional`，member 进程无 redis 也能装配）。 |
| D7 | （前文未写）模块装配 | `oauth2.FxModule` **硬编码进** `alexgo-server/cmd/main.go` 与 `modules/member/cmd/main.go`（config modules map 不动）；member 进程也装——member 兼容层消费 `UnifiedLogin`，迁移同库幂等。 |
| D8 | §6 逐端点限流 | **作废**：复用既有全局 RateLimitAndBreaker（已覆盖全路径）。 |
| D9 | （前文未写）client_credentials | 新增 `token.UserTypeClient=3`；`Issue` 允许 `UserID==0` 当且仅当 `UserType==3` 且 `ClientID` 非空；`Validate` 对 type 3 以 `ClientID` 为 username；机机 token 不带 refresh，**jwt 模式也签不透明机机 token**。 |
| D10 | §4.3 内部 client 名 `app`/`member_portal` | 种子 client（`20261011000002_oauth2_seed_clients`，NOT EXISTS 幂等）：**`alexgo-admin`**（secret 空=公开客户端，grants `authorization_code,password,refresh_token`，account_types `system`，redirect 白名单 `http://localhost:5173/callback\nhttp://localhost:8080/callback`）与 **`alexgo-app`**（机密，grants `password,refresh_token`，account_types `member`）。公开规则：库中 secret 为空 → 跳过 bcrypt（授权码流靠 PKCE）。 |
| D11 | §1 表 app 端点 `/api/app/auth/login` | 实际路径为 **`POST /api/app/system/auth/login`**（既有行为不变，勘误笔误）。 |
| D12 | §4.3 验证码字段 `captcha_token` | 请求字段改为 **`captcha_uuid` + `captcha_code`** 两个。 |
| D13 | §4.5 "复用 RecordLogin 审计" | 审计职责**移入统一登录服务**（兼容层 controller 不再内联记账，防双记）；`LoginLog` 无 source 列 → msg 统一加前缀 `"oauth2: "`。 |
| D14 | （前文未写）refresh 绑定 | refresh **不做 client 绑定**（refresh token 即凭证）；只校验 client 存在+活跃+grant 白名单含 `refresh_token`。 |
| D15 | §4.2 SPA 时序 | 前端登录页生成 state/verifier（crypto.getRandomValues 32B base64url）+ S256 challenge → sessionStorage → `fetch POST /api/oauth2/authorize/login`（`redirect:'follow'`，从 `resp.url` 提取 code 并比对 state）→ `router.push('/callback?code&state')` → **/callback 路由**（进 coreRoutes，免权限）校验 PKCE session、`POST /api/oauth2/token`、setAccessToken+fetchUserInfo+权限码后跳转。前端固定使用 client `alexgo-admin`、redirect `origin+/callback`（自造 PKCE），不消费 authorize 回带参数；GET `/api/oauth2/authorize` 302→login_page 保留供外部/深链（e2e 覆盖服务端时序）。 |

补充语义（计划按此实现）：授权码 TTL **2 分钟**、`code`=32B crypto/rand base64url；**首次 exchange 尝试即 MarkRevoked（无论 verifier 对错，CAS 单次消费）**；redirect_uri **精确匹配、失效/篡改一律 400 绝不 302**；`authorize/login` 请求体含可选 `account_type`（缺省 `system`）。
