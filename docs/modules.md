# 模块功能与权限手册

> 覆盖后端业务子系统（顶层 4 模块：**system 含 oauth2、infra 两个子模块**，另有 member、order、scorecard；功能口径仍按 system / infra / member / oauth2 / order / scorecard 划分）的**功能、权限、后端端点、前端页面**，以及横切的认证、RBAC、租户体系。
> 配套阅读：[架构说明](architecture.md)（中间件链/配置/部署）、[OAuth2 统一登录](oauth2.md)（登录与签发细节）、[使用文档](usage.md)（菜单管理与代码生成操作）、[服务通信](service-communication.md)（mono/micro 双形态）。
> 与实现不一致时以代码为准；本文关键事实与代码交叉核对于 2026-10-13，文件存储 / Excel 导出 / 统一信封等补录条目核对于 2026-10-20，scorecard 章节核对于 2026-10-21，第三方登录（微信一期，§6.7）补录条目核对于 2026-10-21。

---

## 目录

1. [项目总览](#1-项目总览)
2. [认证与权限体系（横切）](#2-认证与权限体系横切)
3. [system — 系统管理模块](#3-system--系统管理模块)
4. [infra — 基础设施模块（代码生成）](#4-infra--基础设施模块代码生成)
5. [member — 会员模块](#5-member--会员模块)
6. [oauth2 — OAuth2 授权模块](#6-oauth2--oauth2-授权模块)
7. [order — 订单模块](#7-order--订单模块)
8. [前端总览（apps/web-antd）](#8-前端总览appsweb-antd)
9. [权限码速查总表](#9-权限码速查总表)
10. [文档地图](#10-文档地图)
11. [scorecard — 打牌记账模块](#11-scorecard--打牌记账模块)

---

## 1. 项目总览

alexGo-cloud 是一个多租户的管理后台 + C 端会员服务，后端 Go（fx 装配 + gin），前端 vben5（Vue3 + Ant Design Vue）。

### 1.1 模块地图

| 模块 | 目录 | 定位 | 端点前缀 | 端点数 | 租户隔离 |
| --- | --- | --- | --- | --- | --- |
| **system** | `modules/system` | 用户/角色/菜单/部门/岗位/字典/参数/公告/日志/租户；RBAC 与 Casbin 策略的唯一实现方 | `/api/admin/system`、`/api/app/system`、`/api` 根（vben 5 端点） | 58 | 全表 `tenant_id`（`tenants`/`casbin_rule` 豁免） |
| **infra** | `modules/system/infra` | 代码生成（库表导入、字段配置、同步、预览、ZIP 下载）+ 统一文件上传（`infra/file`） | `/api/admin/infra/codegen`、`/api/admin/infra/file`、`/api/public/infra/file` | 15 | 无（系统级） |
| **member** | `modules/member` | C 端会员注册/登录 + 管理端会员启停 | `/api/app/member`、`/api/admin/member` | 7 | `member_user.tenant_id` |
| **oauth2** | `modules/system/oauth2` | OAuth2 协议端点（授权码+PKCE/四种 grant）、统一登录门面、客户端管理、第三方登录（`social/` 子包，微信一期，见 §6.7） | `/api/oauth2`、`/api/admin/oauth2`、`/api/app/social`、`/api/app/member/auth`（social 三端点）、`/api/admin/social` | 17 | client 全局；授权码手管 `tenant_id`；social 两表租户级 |
| **order** | `modules/order` | 订单 CRUD + Transactional Outbox 发事件（可由 `cfg.Modules["order"]` 开关整体跳过） | `/api/admin/order` | 3 | 无（gap，见 §7.4） |
| scorecard | modules/scorecard | 打牌记账（房间/计分/茶饭/战绩/好友） | /api/app/scorecard、/api/admin/scorecard | 23 | 全表 tenant_id |

所有模块实现 `server.Module.RegisterRoutes`，由 `alexgo-server/server/http.go` 统一挂到 `/api` 组。

### 1.2 部署形态

| 形态 | 进程 | 说明 |
| --- | --- | --- |
| **mono**（默认） | 单进程 `alexgo-server`：oauth2 + infra + system + order + member + scorecard | 模块间 fx 直调 |
| **micro** | `system-server`（`make run-system`，含 gRPC TokenService）+ `member-server`（`make run-member`，`:8081`，只装 oauth2 + member，**无 Casbin**） | member 经 gRPC `TokenService` 委托签发/刷新/吊销，校验查共享 token 表 |

详见 [service-communication.md](service-communication.md)。

### 1.3 默认账号与入口

- 初始管理员：`admin / admin123`（seed 生成，`tenant_id=0` 平台租户）
- 后端默认 `http://localhost:8080`，前端 dev `http://localhost:5666`
- 健康检查：`GET /health`、`GET /health/ready`、`GET /metrics`

---

## 2. 认证与权限体系（横切）

### 2.1 中间件链（顺序即优先级）

`alexgo-server/server/http.go` 全局链：

```
Recovery → Tenant → Prometheus → RateLimitAndBreaker → Auth → OperateLog → OTEL → AccessLog → Trace
```

- **Tenant**：`X-Tenant-ID` 头 → Host 域名匹配 `tenants.domain` → 默认 `0`；登录后由 **token 内 tenant_id 覆盖**（防头伪造跨租户：`/api/admin/**` 与 vben 自读在 `pkg/middleware/auth.go` 中间件统一回写；`/api/app/**` 由各 handler 的 `claims()` 按需回写——member/system 的 7 条 auth 端点不回写，但均不读写租户数据，无越权面）。
- **RateLimitAndBreaker**：Redis Token Bucket（key `{tid}:{ip}:{FullPath}`）+ 熔断；两开关默认 false，`/health`、`/metrics` 豁免。
- **Auth**（`pkg/middleware/auth.go`）：作用范围 = `/api/admin/**` + vben 自读 3 端点（`/api/auth/codes`、`/api/user/info`、`/api/menu/all`）。按序：解析 token（token 模式查库 fail-closed / jwt 模式静态解析）→ tenant 覆盖 → **`user_type != 1` 恒 403（置于 Casbin 之前）** → data_scope 注入（失败按 Mode 5 最严兜底）→ `Casbin Enforce(sub, FullPath, method)`。
- **OperateLog**：仅 `/api/admin/**`，写 `operate_logs` 表。
- **`/api/app/**` 与 `/api/oauth2/**` 全部不走此清单**——其鉴权在 handler 内自行完成（见各模块节）。

### 2.2 登录与令牌

| 入口 | 端点 | 说明 |
| --- | --- | --- |
| 前端后台主链路 | `POST /api/oauth2/authorize/login` → 302 → `/callback` 换 `POST /api/oauth2/token` | 授权码 + PKCE，见 [oauth2.md](oauth2.md) |
| vben 兼容端点 | `POST /api/auth/login` | 兼容层，保留条件见 [oauth2-compat-retirement.md](oauth2-compat-retirement.md) |
| app 端 system 账号 | `POST /api/app/system/auth/login` | 统一登录（client=`alexgo-admin`） |
| app 端会员 | `POST /api/app/member/auth/login` | 统一登录（client=`alexgo-app`、account_type=member） |
| app 端第三方登录（微信） | `POST /api/app/social/wechat/mp/login`、`POST /api/app/social/wechat/web/login` | 公开 code 换令牌，**不经 UnifiedLogin**（直调 `SocialService` → `token.Issuer`，见 §6.7） |

签发双模式（`auth.mode`，默认 `token`）：

| | access_token | refresh_token |
| --- | --- | --- |
| 形态 | 不透明随机串（32B/24B base64url），落表 `system_oauth2_access_token` | 同表 |
| 有效期 | 默认 2h（`auth.access_expire_hour`） | 窗口 7d（`auth.refresh_expire_day`），**单次使用轮换**：删旧行再重签 |
| 校验 | 每次查库无缓存 + 复查 `tenants.status` | 吊销即删行；过期行清扫保留 7 天 |
| `mode=jwt` | HS256、24h 固定过期（`pkg/auth/jwt.go`）；**回滚开关**：无 refresh、logout 不可吊销 | — |

统一登录门面 `UnifiedLogin`（`modules/system/oauth2/service/login.go`）：captcha（默认关）→ 按 `account_type` 路由到 system/member 凭据校验器 → `IssueFor` 签发 → 恰一次登录审计写 `login_logs`。

### 2.3 Casbin 模型与策略来源

- 模型（`pkg/auth/casbin.go`）：`m = g(r.sub, p.sub) && keyMatch2(r.obj, p.obj) && regexMatch(r.act, p.act)`，策略存表 `casbin_rule`（gorm-adapter，租户插件白名单豁免）。
- **sub**：用户 `{tenantID}:{userType}:{username}`（空用户名退化为 userID）；角色独立命名空间 `{tid}:role:{code}`——防跨租户同名串策略、防成员昵称撞角色 sub 提权。
- **obj** = gin `FullPath()`（如 `/api/admin/system/users/:id`）；**act** 恒为 `.*`（一期前缀粗粒度，method 级放二期）。
- **策略无 SQL 种子**，全部由 `role_menus` 派生：菜单增删改 / 角色授权 / 启动 seed 触发 `RebuildAllPolicies`（全量）或 `RebuildRolePolicies`（单角色）→ `AddPolicy(roleSub, route, ".*")`。用户→角色 g 绑定由登录时 `EnsureUserRolePolicy` 补挂。
- **超管没有 `is_super` 字段**：靠内置角色 `code=admin`（`type=1` 禁删）+ seed 把**当时全部菜单**绑给它 → 拿到 `permissionRoutes` 里所有路由的策略 = 全站接口。

### 2.4 权限码三层链路

以 `oauth2:client:create` 为例：

1. **数据源**（唯一）：`menus` 表一行 `type='button', permission='oauth2:client:create'`，经 `role_menus` 绑到某角色。
2. **前端**（逐字精确）：`GET /api/auth/codes` 返回用户全部菜单 permission 去重数组（含 `*` 字面量）→ `v-access:code="['oauth2:client:create']"` 用 Set 精确相等匹配，不命中 `el.remove()` 移除按钮。**改菜单权限码会导致前端按钮整体消失（无前缀/通配展开）。**
3. **后端**（前两段前缀）：`permissionRoutes`（`modules/system/service/permission_routes.go`）取 `permPrefix` = 前两段（`oauth2:client:create` → `oauth2:client`）→ 映射路由 `["/api/admin/oauth2/clients", "/api/admin/oauth2/clients/*"]` → 重建出 p 策略行 → 中间件 `Enforce` 放行/403。

**两处关键取舍**：

- 前端可按 `:create/:update/:delete` 分按钮，后端只到**前两段前缀**——只拿 `create` 按钮的角色也拿到整个 `clients*` 的读写（spec 决策 8，method 级精确放二期）。
- **死权限陷阱**：新权限码的前缀必须在 `permissionRoutes` 登记，否则该前缀无路由映射、接口一律 403（有测试 `permission_routes_test.go` 兜底）。

### 2.5 租户体系与数据权限

- **解析**：头 → 域名 → 默认 0；登录请求由 token 内 tenant_id 覆盖。
- **GORM 插件**（`pkg/tenant/gormplugin`）：Create 零值回填 `tenant_id`，Query/Update/Delete 自动追加 `tenant_id =`；Query 另叠加 data_scope。跳过条件：`IgnoreTenant` 显式标记、`ExemptTables` 白名单（`tenants`、`casbin_rule`）、模型无 `TenantID` 字段、`tid=0` 平台租户。
- **数据权限五档**（`roles.data_scope`）：1 全部 / 2 自定义 / 3 本部门 / 4 本部门及以下 / 5 仅本人。多角色取**最宽松**档；mode 4 沿部门树算后代（环判重）；加载失败兜底 Mode 5 最严。
- **表分级**：
  - 全局豁免：`tenants`、`casbin_rule`
  - 系统级（列上无 tenant_id）：`oauth2_client`、`codegen_table`/`codegen_column`
  - 特例：`oauth2_authorize_code` 字段名 `OwnerTenantID` + `column:tenant_id`（规避插件注入，手动管理）
  - 租户级（插件隔离）：`system_users`、`roles`、`menus`、`user_roles`、`role_menus`、`depts`、`posts`、`dict_*`、`system_configs`、`system_notices`、`system_notice_reads`、`login_logs`、`operate_logs`、`outbox_events`、`member_user`、`system_oauth2_access_token`、`system_social_app`、`system_member_identity`（social 包读写**恒显式 `tenant_id` 过滤**、写入 `IgnoreTenant` + 显式赋值，见 §6.7）
  - **未隔离（gap）**：`orders` 无 `TenantID` 字段——service 注释宣称多租户但插件不过滤该表
- **租户能力**：租户 CRUD（§3）、账号额度闸门（`account_limit<0` 不限）、停用租户在签发与校验两道闸查 `tenants.status`。

### 2.6 初始种子数据

由 `modules/system/service/seed.go`（OnStart 异步）+ 迁移 SQL 双源写入（全部幂等、`tenant_id=0`）：

- 管理员 `admin`（bcrypt 密码，读 `modules/system/configs/config.yaml`）+ 角色 `admin`（type=1 内置、data_scope=1）+ `user_roles` 绑定
- 菜单树：见 §8.2 完整菜单树（seed 写系统管理 1 目录 + 12 页面 + 3 按钮；迁移补工作台/订单/codegen/oauth2/social 菜单）
- 末尾：全部菜单绑 admin 角色 → `RebuildPolicies` → `AddRoleForUser`
- 种子 client：`alexgo-admin`（公开、授权码+password+refresh、account_types=system）、`alexgo-app`（机密、password+refresh、account_types=member）
- **没有任何 SQL 版 roles/tenants 种子**——租户需在后台手动创建

---

## 3. system — 系统管理模块

### 3.1 功能

RBAC 与全站权限的中枢：用户管理（含列表导出 xlsx）、角色管理、菜单管理（权限码的唯一数据源）、部门/岗位/字典/系统参数/通知公告（含附件，走 infra 文件子域见 §4.7）、登录与操作日志、租户管理、统一登录的 system 账号凭据校验、refresh/logout、在线会话监管（列表/踢人/代登录，见 §3.7）、vben 前端契约 5 端点。

装配要点（`modules/system/module.go`）：

- **Repository**：User/Dept/Post/Dict/Config/Notice/Audit/Role/Menu/UserRole/RoleMenu/Tenant/Online 十三类
- **Service**：`PermissionService`（RBAC 核心：UserRoles/UserMenus/UserPermCodes/UserRoutes + casbin 策略重建）、`RoleService`（type=1 内置角色禁删）、`MenuService`（删除后全量重建策略）、`TenantService`（账号额度闸门 `AccountLimitChecker`，供 member）、`DataScopeLoader`（供中间件）、`AuditService`（同时映射为操作日志 recorder 与 oauth2 登录 auditor）、`UserAuthenticator`/`JWTGenerator`（供 oauth2 统一登录）、`AuthService`（refresh/logout）、`OnlineService`（在线会话列表/踢人/代登录，见 §3.7）
- **后台任务**：启动 seeder；token 过期清扫 ticker（每小时 `SweepExpired`）
- **gRPC**：`TokenServiceImpl`（`IssueToken/RefreshToken/RevokeToken`）进 `group:"grpc_registrars"`，仅 micro 模式监听；独立入口 `modules/system/cmd/grpc_main.go`

### 3.2 权限

- 全站 casbin 前缀表 `permissionRoutes` 定义在本模块 `service/permission_routes.go`，但**保护的是所有模块**的 admin 路由（含 `infra:codegen`、`infra:file`、`oauth2:client`、`member:user`、`order:order`、`scorecard:room`/`scorecard:record`/`scorecard:friend`）。
- 本模块权限前缀 12 组：`system:user` / `role` / `menu` / `dept` / `post` / `dict` / `config` / `notice` / `log` / `online` / `auth` / `tenant`，各自映射 `/api/admin/system/<资源>` 及 `/*`。
- 菜单/按钮种子（seed.go）：

| 类型 | 权限码 | 菜单 |
| --- | --- | --- |
| dir | `system` | 系统管理 `/system` |
| menu | `system:user:list` | 用户管理 `/system/users` |
| menu | `system:role:*` | 角色管理 `/system/roles` |
| menu | `system:menu:*` | 菜单管理 `/system/menus` |
| menu | `system:dept:*` | 部门管理 `/system/depts` |
| menu | `system:post:*` | 岗位管理 `/system/posts` |
| menu | `system:dict:*` | 字典管理 `/system/dict` |
| menu | `system:config:*` | 系统参数 `/system/configs` |
| menu | `system:notice:*` | 通知公告 `/system/notices` |
| menu | `system:log:login` | 登录日志 `/system/logins` |
| menu | `system:log:operate` | 操作日志 `/system/operates` |
| menu（迁移回补） | `system:tenant:*` | 租户管理 `/system/tenants` |
| menu（seed 回补） | `system:notice:mine` | 我的通知 `/system/notices/mine` |
| menu（迁移回补） | `system:online:list` | 在线用户 `/system/online-users` |
| button | `system:auth:profile` | 登录鉴权按钮 |
| button | `system:tenant:manage` | 租户管理按钮（前端未用） |
| button | `member:user:manage` | 会员管理按钮（顺带为 member 补，前端无页面） |
| button（迁移回补） | `system:online:kick` | 在线用户页「踢出」按钮 |
| button（迁移回补） | `system:online:impersonate` | 在线用户页「代登录」按钮 |

### 3.3 后端端点（61 条）

#### 用户 / 鉴权 — `/api/admin/system`

| 方法 | 路径 | handler | 说明 |
| --- | --- | --- | --- |
| GET | `/users` | AdminController.ListUsers | 当前租户全量用户列表 |
| GET | `/users/export` | UserController.ExportUsers | 导出当前租户全量用户 xlsx（xlsx 下载流，5 万行保险丝，见 [usage.md §4](usage.md)） |
| POST | `/users` | UserController.Create | 新建账号（校验租户额度、重名、**禁用户名含 `:`**） |
| PUT | `/users/:id` | UserController.Update | 更新昵称、状态 |
| POST | `/users/:id/reset-password` | UserController.ResetPassword | 重置密码 |
| POST | `/users/:id/roles` | UserController.SetRoles | 设置角色（同步重建 casbin g 绑定） |
| GET | `/auth/profile` | AuthController.Profile | 当前用户 claims + 角色 + 权限码 + 菜单 + vben 路由树 |
| POST | `/auth/refresh` | AuthController.Refresh | refresh_token 换新令牌对 |
| POST | `/auth/logout` | AuthController.Logout | 注销当前 access token |

#### 角色 / 菜单 — `/api/admin/system`

| 方法 | 路径 | handler | 说明 |
| --- | --- | --- | --- |
| GET | `/roles` | RoleController.List | 角色列表 |
| POST | `/roles` | RoleController.Create | 创建（code/name/sort/数据范围） |
| DELETE | `/roles/:id` | RoleController.Delete | 删除（内置角色拒绝） |
| POST | `/roles/:id/menus` | RoleController.AssignMenus | 菜单授权并**单角色重建** casbin 策略 |
| GET | `/menus` | MenuController.List | 菜单平铺列表（前端自行建树） |
| POST | `/menus` | MenuController.Create | 新建（dir/menu/button） |
| PUT | `/menus/:id` | MenuController.Update | 更新（含权限码） |
| DELETE | `/menus/:id` | MenuController.Delete | 删除并**全量重建**策略 |

#### 基础数据 — `/api/admin/system`（BasicController）

每组均为标准 4 CRUD（GET 列表 / POST 新建 / PUT 更新 / DELETE `/:id` 删除）：

- **部门** `/depts`（树形 parent_id/sort）
- **岗位** `/posts`
- **字典类型** `/dict/types`、**字典数据** `/dict/datas`（GET 需 query `type_id`；删类型级联处理数据）
- **系统参数** `/configs`
- **通知公告** `/notices`

**我的通知**（P0-1 已读回执，同一 BasicController）：

| 方法 | 路径 | handler | 说明 |
| --- | --- | --- | --- |
| GET | `/notices/mine` | ListMyNotices | 本租户公告 + 当前用户 `read_status`（0 未读/1 已读） |
| POST | `/notices/:id/read` | MarkNoticeRead | 标记单条已读（幂等；不存在/跨租户 400） |
| POST | `/notices/read-all` | ReadAllNotices | 全部标记已读（幂等） |

#### 日志 / 租户 — `/api/admin/system`

| 方法 | 路径 | handler | 说明 |
| --- | --- | --- | --- |
| GET | `/logs/login` | AuditController.ListLogin | 登录日志（query `limit`） |
| GET | `/logs/operate` | AuditController.ListOperate | 操作日志（query `limit`） |
| GET | `/tenants` | TenantController.List | 租户列表 |
| POST | `/tenants` | TenantController.Create | 新建（account_limit 0→-1 表示不限） |
| PUT | `/tenants/:id` | TenantController.Update | 更新名称/域名/状态/额度 |
| DELETE | `/tenants/:id` | TenantController.Delete | 软删租户 |

#### 在线用户 — `/api/admin/system`

| 方法 | 路径 | handler | 说明 |
| --- | --- | --- | --- |
| GET | `/online` | OnlineController.List | 本租户**未过期管理员**会话分页（查 `system_oauth2_access_token`；出行 id/用户名/user_id/client_id/签发/过期，**明文 token 不出接口**） |
| DELETE | `/online/:id` | OnlineController.Kick | 按行 id 吊销（`Revoke` 即删行）；禁踢自己 30202、查无/跨租户/非管理员 30203（语义见 §3.7） |
| POST | `/online/impersonate` | OnlineController.Impersonate | 代登录到会员：入参 `{user_id}` → 出参 `{access_token, refresh_token, expires_in}`（`client_id=impersonate-admin`，见 §3.7） |

#### app 端 — `/api/app/system`（公开，不走 casbin）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/auth/login` | app 端账号密码 + 验证码登录（统一登录，client=`alexgo-admin`） |
| POST | `/auth/refresh` | 刷新令牌 |
| POST | `/auth/logout` | 注销令牌 |

#### vben 前端契约 — `/api` 根组

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/auth/login` | vben 登录页（信封 `{code,data,error,message}`） |
| POST | `/api/auth/logout` | 退出（恒 200，尽力注销） |
| GET | `/api/auth/codes` | 权限码数组（v-access 逐字匹配的数据源） |
| GET | `/api/user/info` | vben 用户信息（userId 字符串/roles/realName） |
| GET | `/api/menu/all` | vben 后端动态路由树（LAYOUT component 置空） |

> `/api/auth/codes`、`/api/user/info`、`/api/menu/all` 属"vben 自读"清单：需 token 但**跳过 data_scope 与 casbin**（`pkg/middleware/auth.go`）。

### 3.4 数据模型

| 实体 | 表 | 关键字段语义 |
| --- | --- | --- |
| User | `system_users` | username/nickname/password_hash、dept_id、post_ids（逗号数组）、status、login_ip/login_date、tenant_id |
| Role | `roles` | code/name/sort、**data_scope 五档**、data_scope_dept_ids、type（1 内置禁删 / 2 自定义）、tenant_id |
| Menu | `menus` | parent_id、type（dir/menu/button）、path/component、**permission（权限码载体）**、sort、tenant_id |
| UserRole / RoleMenu | `user_roles` / `role_menus` | 关联表，唯一键含 tenant_id |
| Dept / Post | `depts` / `posts` | 树形部门（parent_id/sort）；岗位 code/name/sort |
| DictType / DictData | `dict_types` / `dict_datas` | 字典类型与项（label/value/sort） |
| SystemConfig / SystemNotice | `system_configs` / `system_notices` | key/value 参数；标题/内容/状态公告 |
| LoginLog / OperateLog | `login_logs` / `operate_logs` | 登录（ip/ua/success/message）；操作（method/path/status/latency_ms/error） |
| Tenant | `tenants`（全局表） | name/domain（唯一）、status、expire_time、account_limit（-1 不限） |
| OutboxEvent | `outbox_events` | 事务性发件箱（生产方是 order 模块） |
| — | `casbin_rule`（全局表） | casbin 策略存储 |
| — | `system_oauth2_access_token` | 访问/刷新令牌（本模块建表，供 pkg/token 读写） |

迁移 16 个 up（`modules/system/migrations/`），含初始化、outbox、RBAC、审计、基础数据、`users`→`system_users` 重命名、tenants、vben 菜单回填、codegen/oauth2 菜单种子。

### 3.5 对外 RPC / 事件

- gRPC `TokenService`（Issue/Refresh/Revoke），micro 模式才监听；mono 直调本地 `token.Service`。
- `outbox_events` 表的**建表与模型在本模块**，但本模块不生产事件；`pkg/outbox/relay` 后台轮询发往 NATS（生产方是 order）。
- 预留未实现 proto：`api/admin/v1/{role,user}.proto`（无 Go 引用）。

### 3.6 前端页面

| 路由 | 组件 | 功能 | 页内权限码（v-access） |
| --- | --- | --- | --- |
| `/system/users` | `views/system/SystemUsersPage.vue` | 用户列表 + 新建 + 重置密码 | `system:user:list`（**读写复用，粒度不足**） |
| `/system/roles` | `SystemRolesPage.vue` | 角色新建/删除；**无"分配菜单" UI**（后端 `POST /roles/:id/menus` 前端未封装） | `system:role:*` |
| `/system/menus` | `SystemMenusPage.vue` | 菜单树形表格 + 新建/编辑/删除 | `system:menu:*` |
| `/system/depts` | `SystemDeptsPage.vue` | 部门新建/删除（无编辑） | `system:dept:*` |
| `/system/posts` | `SystemPostsPage.vue` | 岗位新建/删除（无编辑） | `system:post:*` |
| `/system/dict` | `SystemDictPage.vue` | 左类型右数据双表联动，各自新建/删除 | `system:dict:*`（5 处） |
| `/system/configs` | `SystemConfigsPage.vue` | 参数新建/删除（无编辑） | `system:config:*` |
| `/system/notices` | `SystemNoticesPage.vue` | 公告新建/删除（无编辑） | `system:notice:*` |
| /system/notices/mine | SystemMyNoticesPage.vue | 我的公告列表、详情查看（未读打开自动标记）、单条/全部已读 | 无 v-access（可见性由菜单 `system:notice:mine` 控制） |
| `/system/logins` | `SystemLoginLogsPage.vue` | 登录日志只读（limit=100） | 无（靠菜单可见性） |
| `/system/operates` | `SystemOperateLogsPage.vue` | 操作日志只读 | 无 |
| `/system/tenants` | `SystemTenantsPage.vue` | 租户新建/编辑/删除 | `system:tenant:*` |
| `/system/online-users` | `SystemOnlineUsersPage.vue` | 在线会话列表（踢人 Popconfirm、代登录 modal 选会员） | `system:online:kick`、`system:online:impersonate`（两按钮；页面可见性靠菜单 `system:online:list`） |

> 前端"编辑"按钮缺失的根因：`src/api/admin.ts` 未封装 `PUT /depts|/posts|/dict/*|/configs|/notices`（后端有端点）。

### 3.7 在线用户与代登录（P1-3 补录 2026-10-21）

在线会话监管子域（设计：
[`superpowers/specs/2026-10-21-online-user-impersonate-design.md`](superpowers/specs/2026-10-21-online-user-impersonate-design.md)；
端点见 §3.3，操作见 [usage.md §6.5](usage.md)）。菜单「系统管理 → 在线用户」`/system/online-users`
（迁移 `20261021000001_online_user_menus.up.sql` 种子，页面 + 踢人/代登录两按钮）。

- **权限**：casbin 前缀 `system:online` → `["/api/admin/system/online", ".../online/*"]`（见 §9.1）；
  权限码三枚——`system:online:list`（菜单可见性）、`system:online:kick` / `system:online:impersonate`
  （页内两按钮，`v-access` 逐字匹配）。
- **列表范围**：只列**本租户、未过期、`user_type=1`（管理员）**的 access token 行
  （`system_oauth2_access_token`，分页 `id DESC`；表无 IP/UA 列，`username` 按页补查 `system_users`）。
  `is_impersonated` 由 `client_id=impersonate-admin` 推得——代登录签的是 **member** token，
  不在本列表范围，故管理员行恒 false（保留字段供范围扩展）。
- **踢人**：`DELETE /online/:id` 按行 id 调 `issuer.Revoke`（删 access_token 行，明文不出接口；
  踢过期行等价清理）。**禁踢自己**（防把自己锁在外面）；查无 / 跨租户 / 非管理员行 → 30203。
- **代登录**：`POST /online/impersonate` `{user_id}` → 点查 `member_user`（须存在、未删、`status=1` 启用）→
  以 `client_id=impersonate-admin` 签 member token（`IssueParams.ClientID` 既有字段，零 pkg 改动）→
  出参 `{access_token, refresh_token, expires_in}`；**同时写 `login_logs` 审计**，
  message 形如 `代登录: 管理员{id}/{name} → 会员{id}/{name}`（IP/UA 取请求侧）。
- **错误码**（`pkg/errors/code.go`，3xxxx 段 → HTTP 400）：

| 码 | 出站文案（registry） | 语义 |
| --- | --- | --- |
| `30201` | `目标会员不存在或不可用` | 目标会员不存在/已删除/已停用（给停用账号签 token = 绕过登录状态门） |
| `30202` | `不能踢出自己的会话` | 禁踢自己 |
| `30203` | `会话不存在` | 查无 / 跨租户 / 已非管理员会话（覆盖 refresh 轮换、sweep 删除的竞态兜底） |

- **前端**：`src/api/admin.ts` 的 `listOnlineApi` / `kickOnlineApi` / `impersonateApi` +
  `views/system/SystemOnlineUsersPage.vue`（antd Table 真分页 20/页；代登录 modal 选会员 → 确认 → toast，
  **token 不回显、不落盘**——接口返回令牌对，页面不展示、不写存储、不切换当前浏览器身份）。
- **明确不做**：会员在线列表、IP/UA 列与展示（仅代登录写进 login_logs）、浏览器侧身份切换 UI、
  token 表扩列（last_seen 回写会放大 Validate 热点路径）、oauth2 协议级 introspect/revoke 端点、跨 gRPC 踢人。

---

## 4. infra — 基础设施模块（代码生成 + 文件存储）

### 4.1 功能

从 MySQL `information_schema` 导入库表 → 生成/编辑字段配置 → 与库结构同步差异 → 内存渲染代码预览 → ZIP 下载。生成引擎在 `pkg/codegen/{builder,metadata,model}`，构建选项 `Module=infra`、`TemplateType=Single`、`FrontType=Vben5Antd`。

装配要点（`modules/system/infra/module.go`）：

- `CodegenRepository`：配置表/字段 CRUD + 同步差异事务落库（`ApplySync`）
- `MetadataReader`：按配置 DSN 惰性连接 information_schema，OnStop 关闭
- `CodegenService`：导入/字段配置/同步/预览/下载；单测骨架开关 `cfg.Codegen.UnitTestEnable`
- 系统级定位：**无 tenant_id、无 deleted、硬删**

### 4.2 权限

- **后端**：casbin 前缀 `infra:codegen` → `["/api/admin/infra/codegen", "/api/admin/infra/codegen/*"]`（映射表在 system 的 `permission_routes.go`）。
- **菜单种子**（注意放在 system 的迁移里）`modules/system/migrations/20261011000002_codegen_menus.up.sql`：
  - 目录「代码生成」`/codegen` → `infra`
  - 页面「代码生成配置」`/codegen/config` → `infra:codegen`（component `views/infra/codegen/index`）
  - 按钮：`infra:codegen:import`（导入）、`infra:codegen:sync`（同步）、`infra:codegen:delete`（删除）
- **前端**：`infra:codegen`（生成代码/字段配置/编辑/预览/下载）、`infra:codegen:import`、`infra:codegen:sync`、`infra:codegen:delete`。

### 4.3 后端端点（11 条，前缀 `/api/admin/infra/codegen`）

| 方法 | 路径 | handler | 说明 |
| --- | --- | --- | --- |
| GET | `/tables` | listTables | 分页列出已导入表配置（统一信封 `data:{list,total,page,size}`） |
| GET | `/db-tables` | listDBTables | 库内可导入表名下拉（information_schema） |
| POST | `/import` | importTable | 按 table_name 导入表并生成字段配置 |
| GET | `/tables/:id` | getTable | 表配置详情 + 字段列表（信封 `data:{data,columns}`） |
| PUT | `/tables/:id` | updateTable | 更新表级配置（路径 id 压过 body） |
| DELETE | `/tables/:id` | deleteTable | 删除表配置及其字段 |
| GET | `/tables/:id/columns` | listColumns | 字段配置列表 |
| PUT | `/tables/:id/columns` | saveColumns | 全量保存字段配置（body 数组） |
| POST | `/tables/:id/sync` | sync | 与库元数据同步（增/刷/废弃计数） |
| POST | `/preview` | preview | 纯内存渲染（body `{"table_ids":[...]}` → `{data:[{path,content}]}`） |
| POST | `/download` | download | 渲染结果打 ZIP 流下载（`codegen.zip`） |

状态码约定（统一信封 `pkg/resp`）：读库失败 50000→500，校验 10001/不存在 30007/查重 30008 → 400（无 404）。

### 4.4 数据模型

- `codegen_table`：table_name（唯一）、table_comment、module、business_name、class_name、template_type、front_type、parent_table_id（主子表）、remark。
- `codegen_column`：table_id+name 唯一；**快照列**（type/comment/go_type/json_name/is_pk/auto_increment/nullable，同步刷新）+ **配置列**（html_type/list_enable/form_enable/query_enable/query_operation/list_required/form_required/dict_type/sort_order，同步**不覆盖**）+ `deprecated`（源库已消失列不物理删）。
- 迁移：`20261011000001_codegen_config` 建两表。

### 4.5 对外依赖

无 gRPC、无事件、无定时任务；依赖 MySQL information_schema（只读）与 `pkg/codegen` 生成引擎。CLI 工具：`tools/codegen`（配置模式默认、`-import` 必配 `-module`、无匹配 exit 2），Makefile `generate-crud` / `generate-db-crud`。

### 4.6 前端页面

| 路由 | 组件 | 功能 |
| --- | --- | --- |
| `/codegen/config` | `views/infra/codegen/index.vue` | 代码生成主页面：表列表（**服务端真分页**，`resp.Page` 信封 size≤100，M4 跟进①已修）、导入表弹窗、字段配置编辑、编辑表配置、同步、预览弹窗（分组渲染）、ZIP 下载（`saveBlob` 委托 `downloadFileFromBlob`）、删除 |

API 封装：`src/api/infra.ts`（11 个函数 + 类型），全部走 `requestClient`。

### 4.7 文件存储（infra file，P1-1 补录 2026-10-20）

`modules/system/infra` 除 codegen 外还挂**统一文件上传**子域（设计：
[`superpowers/specs/2026-10-16-file-storage-design.md`](superpowers/specs/2026-10-16-file-storage-design.md)，
操作见 [usage.md §3](usage.md)）。

- **权限**：casbin 前缀 `infra:file` → `["/api/admin/infra/file", ".../file/*"]`（见 §9.1）。
- **端点**（`file_controller.go`）：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/admin/infra/file/upload` | multipart 上传（字段 `file`，可选 `public=true`、`biz_type`） |
| GET | `/api/admin/infra/file/:id` | 受控下载（attachment） |
| DELETE | `/api/admin/infra/file/:id` | 删除（盘 + 库） |
| GET | `/api/public/infra/file/:key` | 公开读（免鉴权、`public_key` 随机键、inline；private/不存在统一 30010） |

- **分层**：驱动 `pkg/storage`（`Driver{Save,Open,Delete}`，本地盘实现 `local.go`，纯 IO 零策略）；
  策略（扩展名白名单 / 大小双保险 / 双 MIME / 随机名 / 落库 `system_files`）在
  `modules/system/infra/service/file_service.go`。配置段 `storage:`（`root`、`max_size_mb`、
  可选 `allowed_exts`）。
- **业务闭环**：公告附件（`system_notices.attachments` 存 JSON 数组文本，前端
  `utils/notice-attachments.ts` 恒 `public:true + bizType:notice`）；后续头像、codegen 导入产物复用同一子域。
- **前端**：`src/api/infra.ts` 的 `uploadFile` / `requestClient.download('/admin/infra/file/' + id)`；
  文件管理页为后续票（当前只有公告编辑器内嵌上传入口）。

---

## 5. member — 会员模块

### 5.1 功能

C 端会员的注册/登录/刷新/登出 + 管理端会员列表与启停。**自己不实现登录语义**——登录委托 oauth2 的 `UnifiedLogin.Login`（account_type=member、client=`alexgo-app`），令牌经 `token.Issuer`（mono 本地 / micro gRPC）。

装配要点（`modules/member/module.go`）：

- `MemberRepository`（`member_user` 按租户隔离）
- `MemberService`：Register（手机号正则 `^1[3-9]\d{9}$`、密码 ≥8、**昵称禁含 `:`**、租户内查重、账号额度闸门、bcrypt）、Authenticate（status=1 + bcrypt + 记 login_ip/login_date，失败不阻断）、Login/Refresh/Logout、List/Disable
- `MemberAuthenticator` 接口桥 → 供 oauth2 统一登录消费
- micro 形态独立进程 `modules/member/cmd`（`:8081`，只装 oauth2 + member，**不装配 Enforcer/ScopeLoader/OperateRecorder**）

### 5.2 权限

| 端 | 端点 | 鉴权 |
| --- | --- | --- |
| app | `/api/app/member/**` | **公开**；refresh/logout 是 possession-based（凭 body `refresh_token` / `Authorization` 头自证） |
| admin | `/api/admin/member/**` | 管理员 token + casbin，前缀 **`member:user`** → `["/api/admin/member/users", "/api/admin/member/users/*"]` |

菜单种子：只有**按钮** `member:user:manage`（`seed.go`，挂在系统管理下），**没有页面菜单，前端也没有会员管理页**（仅 vite proxy 配了路径）。

> member 自己的 token（user_type=2）访问 `/api/admin/**` 恒 403——user_type 门槛在 casbin 之前。

### 5.3 后端端点（7 条）

#### app 端 `/api/app/member`（公开）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/auth/register` | 手机号注册 |
| POST | `/auth/login` | 统一登录，返回 `{token,refresh_token,expires_in}`；503=签发故障 / 400=captcha / 401=凭据 |
| POST | `/auth/refresh` | possession-based 轮换令牌 |
| POST | `/auth/logout` | possession-based 吊销 |

#### admin 端 `/api/admin/member`

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/users` | 分页列会员（`page/size`） |
| GET | `/users/export` | 导出当前租户全量会员 xlsx（下载流，见 [usage.md §4](usage.md)） |
| PUT | `/users/:id/status` | 启停（body `status` ∈{0,1}；**停用不吊销既有 token**，一期取舍） |

### 5.4 数据模型

- `member_user`（迁移 `20261008000001_init_member`）：nickname/avatar/status/**mobile**（`uk_mobile_tenant` 唯一键含 tenant_id）/password/register_ip/login_ip/login_date/deleted/tenant_id 等；无外键。
- 令牌存 `system_oauth2_access_token`（system 建表）。

### 5.5 跨模块调用

```
member 登录 → oauth2.UnifiedLogin.Login → MemberAuthenticator.Authenticate（本地接口）
           → token.Issuer（mono 本地 token.Service / micro gRPC TokenService → system-server）
谁调 member：① oauth2 统一登录按 account_type=member 路由
            ② pkg/token.Service.Validate 查 member_user 取 nickname
```

- gRPC client：`pkg/client/token_client.go`（Issue/Refresh/Revoke；`RevokeAll` 显式不支持）；失败 → 登录 503 不降级。
- 无 gRPC server、无事件。

### 5.6 前端页面

**无**。后台没有会员管理页（按钮权限 `member:user:manage` 已种但无消费者）；C 端由 app（非本仓库前端）接入 `/api/app/member/*`。

---

## 6. oauth2 — OAuth2 授权模块

### 6.1 功能

- **协议端点**（RFC 6749）：授权码 + PKCE、四种 grant 的 token 端点、userinfo、图形验证码。
- **统一登录门面** `UnifiedLogin`：captcha → 按 account_type 路由 system/member 凭据校验 → 签发 → 恰一次登录审计——是 member 登录、vben SSO 的公共入口。
- **客户端管理**：client CRUD（admin 端）。
- **第三方登录（`social/` 子包，微信一期）**：微信小程序/网页授权 code 换令牌的三分支登录、第三方/手机号绑定与解绑、租户级渠道配置管理——详见 §6.7。

装配要点（`modules/system/oauth2/module.go`）：CaptchaService（Redis，`auth.captcha.enabled` 默认 false）、ClientRepository、AuthorizeCodeRepository、ClientService（secret bcrypt 比对）、AuthorizeCodeService（授权码 TTL **2min**、CAS 单次消费 + PKCE S256）、TokenService（四 grant 状态机，**第一步恒 client 认证**）。

依赖的跨模块窄接口（fx optional）：`token.Issuer`/`Validator`、`UserAuthenticator`（system）、`MemberAuthenticator`（member）、`LoginAuditor`（system）、`JWTGenerator`（jwt 回滚）。

### 6.2 权限

**协议端点 `/api/oauth2/**`：无 casbin、无登录态**，防护手段：

1. **client 认证**：secret bcrypt；未知/停用/错 secret → 401 `invalid_client`；公开 client（secret 空）禁用 `client_credentials`。
2. **redirect_uri 精确白名单**（换行分隔逐条比对 `AllowsRedirect`）。
3. **grant_types 白名单** `AllowsGrant` + **account_types 白名单** `AllowsAccountType`（防 admin client 换 member token）。
4. **PKCE** 仅 S256；`Exchange` 时 `MarkRevoked` 先行单次消费，过期/不符统一 `invalid_grant`。
5. **captcha** 默认关；**userinfo** Bearer 自校验（token 模式 Validator 未装配 **fail-closed 401**）。

**管理端 `/api/admin/oauth2/**`**：管理员 token + casbin，前缀 **`oauth2:client`** → `["/api/admin/oauth2/clients", ".../clients/*"]`。

菜单种子（迁移 `20261011000003_oauth2_client_menu`）：目录 `/oauth2` → 页面 `/oauth2/clients`（`oauth:client`，component `views/system/Oauth2ClientsPage`）→ 按钮 `oauth2:client:create/:update/:delete` → 绑 admin。

### 6.3 后端端点（8 条）

#### 协议端点（RFC 6749 错误信封 `{"error","error_description"}`）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/oauth2/captcha` | 图形验证码挑战（Redis 存储） |
| GET | `/api/oauth2/authorize` | 授权端点入口：校验 client/grant 白名单/redirect_uri/response_type=code/PKCE，失败 → 400 **不 302**；通过 → 302 到登录页并回传 `client_id&redirect_uri&state&code_challenge…` |
| POST | `/api/oauth2/authorize/login` | 登录表单提交（JSON）：校验失败 400 无 Location；成功 → `Codes.Issue` → 302 回 `redirect_uri?code&state&scope`（**不签发 token**） |
| POST | `/api/oauth2/token` | 四 grant（authorization_code / password / client_credentials / refresh_token）；client 认证失败 → 401 `invalid_client`；成功 `TokenResponse{access_token,refresh_token,expires_in,token_type:"Bearer"}` |
| GET | `/api/oauth2/userinfo` | Bearer 自校验，返回 `{user_id,username,user_type,tenant_id,scope:"all"}` |

> **没有** `/oauth2/introspect`、`/oauth2/revoke`——撤销走 member logout 或 system Revoke。

#### 管理端（admin 信封）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/admin/oauth2/clients` | 分页列 client（`resp.Page` 信封，size 上限 100） |
| POST | `/api/admin/oauth2/clients` | 新建，**明文 secret 仅响应回显一次**（落库 bcrypt） |
| PUT | `/api/admin/oauth2/clients/:id` | 更新；body secret 留空保留旧值且不回显 |
| DELETE | `/api/admin/oauth2/clients/:id` | 硬删（表无 deleted 列） |

### 6.4 数据模型

- `oauth2_client`（全局，无 tenant_id/deleted）：client_id、secret_hash、grant_types、redirect_uris、account_types、status；`AllowsGrant/AllowsAccountType/AllowsRedirect` 判定方法。
- `oauth2_authorize_code`：code（`uk_oauth2_code`）、client、owner、scope、code_challenge(+method)、revoked；`OwnerTenantID` 映射 `column:tenant_id` 规避租户插件。
- 迁移 3 个 up + 种子 client `alexgo-admin` / `alexgo-app`（+ dev 回调 `localhost:5666/callback`）。
- 第三方登录两表 `system_social_app`（租户级渠道配置）/ `system_member_identity`（第三方身份绑定），子包 `social/`、迁移 `20261021000001_social_tables`——字段与唯一键详见 §6.7。

### 6.5 对外关系

- 无 gRPC、无事件；仅消费跨模块窄接口。
- 被谁用：member 登录直调 `UnifiedLogin.Login`；vben 前端 SSO 走 `src/api/core/oauth2.ts`（authorize → authorize/login → token）。

### 6.6 前端页面

| 路由 | 组件 | 功能 | 权限码 |
| --- | --- | --- | --- |
| `/oauth2/clients` | `views/system/Oauth2ClientsPage.vue` | 客户端列表 + 新建/编辑/删除（client_id/grant_types/账号类型/启停） | 菜单 `oauth2:client`；按钮 `:create/:update/:delete` |
| `/social/apps` | `views/system/SocialAppsPage.vue` | 第三方登录渠道配置（平台筛选 + 新建/编辑/删除；表单 platform 下拉 `wechat_mp`/`wechat_web` + appid + secret（编辑留空=保留）+ status） | 菜单 `social:app`；按钮 `:create/:update/:delete` |

登录相关静态页见 §8.3（`/auth/login` 走 PKCE、`/callback` 换 token）。

### 6.7 第三方登录 social 子包（微信一期补录 2026-10-21）

`modules/system/oauth2/social/`——微信小程序 / 网页授权第三方登录（设计：
[`superpowers/specs/2026-10-21-social-login-wechat-design.md`](superpowers/specs/2026-10-21-social-login-wechat-design.md)）。
**不经 `UnifiedLogin`**（无本地凭据可校验）：controller → `SocialService` → `token.Issuer`（`ClientID="alexgo-app"`、
`user_type=member`）；`UnifiedLogin` 与 `member_user` 表**零改动**，social 无独立模块开关（随 `oauth2.FxModule` 整体挂载）。

**子包分层**（adapter / service / controller / appconfig / model / repository 六层文件）：

| 层 | 文件 | 职责 |
| --- | --- | --- |
| adapter | `adapter.go`、`wechat_mp.go`、`wechat_web.go` | `PlatformAdapter{Platform(), Exchange(ctx, app, code)}` 接口 + `AdapterRegistry`（装配期构造、运行期只读，重复注册 panic）；`wechat_mp` 走 `sns/jscode2session`、`wechat_web` 走 `sns/oauth2/access_token`（HTTP 10s 超时），平台 errcode 只记服务端日志不上抛（统一映射 30302） |
| service | `service.go` | `SocialService`：登录三分支 + 签发 + 恰一次审计、绑定/手机号合并/解绑（见下） |
| appconfig | `appconfig.go` | `AppConfigService`：渠道配置 CRUD 规则（平台须已注册适配器、appid/secret 必填 10002、`(tid,platform)` 查重、编辑 secret 留空=保留旧值、租户与 ID 归位在 service） |
| controller | `controller/app.go`、`controller/admin.go` | app 五端点（`claims()` 解析 Bearer 自鉴权，与 scorecard app 端同构）+ admin 四端点（casbin `social:app`）；只依赖 `LoginBinder`/`AppConfiger` 接口，测试可 stub |
| model | `model.go` | `SocialApp`（`system_social_app`）/ `MemberIdentity`（`system_member_identity`）；`SocialApp.Secret` 带 `json:"-"`（admin 响应恒不回显） |
| repository | `repository.go` | 12 方法；**读写恒显式 `tenant_id` 过滤**（gormplugin 对 tid=0 沉默不注入，不依赖插件兜底），读 miss 一律 `gorm.ErrRecordNotFound`；两处写入走 `IgnoreTenant` + 显式 `TenantID`（漏传即落错租户） |

**登录三分支**（`SocialService.Login` → `resolveAndIssue`，spec §3）：

1. `(tenant, platform, openid)` 命中绑定行 → 验会员存活（Members 未接线 / 软删 / `status!=1` → 30002）→ 签发；
2. 未命中且 `unionid` 非空 → 同 `(tenant, unionid)` 命中 → **补建绑定行**（唯一键竞态：归属同一会员则幂等继续，绑他人则回传插入错误）→ 签发；**空串 unionid 不参与同人匹配**（SQL 层防）；
3. 都没有 → 账号额度闸门（`AccountLimitChecker`，缺席跳过）→ 创建**空壳会员**（mobile/password 空；昵称清洗去 `:` 防 casbin 子策略撞名提权，空昵称占位「微信用户+openid 尾 4」）+ 绑定行 → 签发。

签发恒 `token.Issuer.Issue(ClientID="alexgo-app", UserType=member)`；**成功恰一次**写 `login_logs`（msg `social: 微信小程序登录 openid=…` / `social: 微信网页授权登录 openid=…`），失败路径不记审计（第三方登录无凭据可审）、`Auditor` 未接线时静默跳过。

**九端点**（app 五 + admin 四，见 spec §5）：

| 方法 | 路径 | 鉴权 | 说明 |
| --- | --- | --- | --- |
| POST | `/api/app/social/wechat/mp/login` | 公开 | body `{code}` → `LoginResult{token, refresh_token, expires_in}`（小程序 code2session） |
| POST | `/api/app/social/wechat/web/login` | 公开 | 同上（网页授权 snsapi） |
| POST | `/api/app/member/auth/bind-social` | Bearer | `{platform, code}` 绑第三方；已绑自己幂等、已绑他人 30303（不迁移） |
| POST | `/api/app/member/auth/bind-mobile` | Bearer | `{mobile}` 绑手机号；已有号 30304；号码被占仅**空壳**自动合并（identity 全改指目标 + 当前空壳软删），非空壳 30305 |
| DELETE | `/api/app/member/auth/social/:platform` | Bearer | 解绑；无该绑定幂等；解绑后将无任何登录方式（无密码且无其他平台绑定）→ 30306 |
| GET | `/api/admin/social/apps` | casbin `social:app` | 分页列渠道（`?page&size&platform`，`resp.Page` 信封、size≤100，`platform==""` 全平台，**secret 不回显**） |
| POST | `/api/admin/social/apps` | casbin `social:app` | 新建（平台须已注册适配器 / 必填 10002 / `(tid,platform)` 查重） |
| PUT | `/api/admin/social/apps/:id` | casbin `social:app` | 更新；body secret 留空=保留旧值、带值=轮换；路径 id 优先于 body |
| DELETE | `/api/admin/social/apps/:id` | casbin `social:app` | 删除（按 `(tid, id)`） |

app 五条挂 `/api/app` 组：登录两条公开；bind 三条在 handler 内 `claims()` 自鉴权（Bearer 校验 member 身份后**把 token 内 tenant 写回请求 ctx**，伪造 `X-Tenant-ID` 无法跨租户，不走 casbin）。admin 四条走 `/api/admin` 组 + casbin 前缀 **`social:app`** → `["/api/admin/social/apps", ".../apps/*"]`（`permission_routes.go`，见 §9.1）。
失败映射：签发链路故障（`ErrIssuance` / `ErrAuthenticatorMissing`）→ **HTTP 503** + code `50000` `auth service unavailable`（与 member 登录同口径，不冒充 401）；3030x 业务码 → 统一信封（3xxxx 段 HTTP 400）。

**错误码 30301–30306**（`pkg/errors/code.go`）：

| 码 | 出站文案（registry） | 语义 |
| --- | --- | --- |
| `30301` | 该租户未配置此登录渠道（或已停用） | `system_social_app` 无 (tid,platform) 行或 `status!=1` |
| `30302` | 第三方授权换取失败 | code 无效/过期/平台接口错（errcode 只记日志） |
| `30303` | 该第三方身份已绑定其他账号 | bind-social 冲突，不迁移绑定 |
| `30304` | 当前账号已绑定手机号 | 不允许改绑 |
| `30305` | 手机号已被占用，当前账号不满足自动合并条件 | 非空壳（有密码或已有手机号）拒绝自动合并 |
| `30306` | 解绑后将失去唯一登录方式 | 解绑守门防锁死 |

**两表**（迁移 `modules/system/oauth2/migrations/20261021000001_social_tables`，up/down 对称，`IF NOT EXISTS` 幂等）：

- `system_social_app` 租户级渠道配置：`tenant_id`+`platform` 唯一（`uk_social_app_tenant_platform`）、`appid`、**secret 明文存储**（调平台 API 必须明文；`json:"-"` 保证任何 admin 响应不回显）、`status`（1 启用 0 停用）、create/update_time。
- `system_member_identity` 第三方身份绑定：`(tenant_id, platform, openid)` 唯一（`uk_identity_tenant_platform_openid`）+ `(tenant_id, unionid)` 普通索引、`member_user_id` 指向 `member_user`、**无 `deleted` 列**（解绑即硬删）。

菜单种子（system 迁移 `20261021000002_social_app_menu`，全部 `NOT EXISTS` 幂等守卫，仿 `20261011000003_oauth2_client_menu`）：目录「渠道配置」`/social`（perm `social`，sort 92，挂系统管理下、fresh 库父不存在落根）→ 页面 `/social/apps`（perm `social:app`，component `views/system/SocialAppsPage`）→ 按钮 `social:app:create/:update/:delete` → 绑 admin；权限码前缀必须已在 `permissionRoutes` 注册否则是死权限（403）。

**adapter 扩展点（新增平台 = 新 adapter 文件 + 新路由 + registry 注册，service 零改动）**：

1. 新增平台实现文件（如 `douyin.go`，实现 `PlatformAdapter`）；
2. `NewAdapterRegistry()` 内 `Register(...)` 加一行（重复平台注册 = 装配期 panic，启动即暴露）；
3. 新增登录路由一行——一期两条登录路由**不参数化**（`/social/wechat/{mp,web}/login` 硬编码，body 契约将来可能分化；绑定/解绑已按 `platform` 参数化，无需改动）。

`system_social_app.platform` 枚举留位（`wechat_mp`/`wechat_web`，douyin/alipay 后续加值）；`SocialService`、绑定/解绑、admin 渠道 CRUD 均零改动。

**micro 形态**：social 是 oauth2 子包、随 `oauth2.FxModule` 整体挂载——micro 下**随 oauth2 进 member-server**（`modules/member/cmd` 直接引 `oauth2.FxModule`，system 不在本进程）。`SocialDeps` 三个依赖均 `optional:"true"`：

- **`MemberStore`**（`GetByID/GetByMobile/Create/Update`，由 `member/module.go` 把 `MemberRepository` 映射供给，无 import 循环——social 只 import `member/model`）：member-server 有 member 模块即接线；**缺席装配**（仅 system 进程）三分支与绑定/解绑的会员读写处一律 `ErrAuthenticatorMissing` → controller 映射 503；
- **`Auditor`**（system `AuditService` → `oauth2svc.LoginAuditor`）：member-server 无 system 模块 → nil → 成功登录不写审计；
- **`Limit`**（`tenant.AccountLimitChecker`）：mono 由 system `TenantService` 映射，member-server 用 `tenant.NewAccountLimitChecker` DB 窄实现补齐（空壳注册的额度闸门）。

**前端**：`views/system/SocialAppsPage.vue` + `src/api/admin.ts` social 段（`listSocialAppsPage`/`createSocialApp`/`updateSocialApp`/`deleteSocialApp`，走 `requestClient` 统一信封），见 §6.6/§9.2；C 端 app（非本仓库前端）接入两条公开登录与三条绑定端点。

**明确不做（一期）**：短信验证码、抖音/支付宝实现、unionid 之外的跨平台同人、用户侧绑定列表查询页、app 端解绑/绑定审计（`OperateLog` 中间件只覆盖 admin 端，留二期）、secret 加密存储、`/api/oauth2` 协议端点联邦化（social 登录是 app 私有端点，不是 RFC6749 联邦）。

---

## 7. order — 订单模块

### 7.1 功能

管理端订单创建与列表；创建走 **Transactional Outbox**——同一事务内：跨模块校验用户存在 → `tx.Create(order)` → 写 `outbox_events`（`order.created`，status=pending）。`cfg.Modules["order"] == false` 时整体不挂载。

### 7.2 权限

- 全部在 `/api/admin/order/**` 下：管理员 token + casbin，前缀 **`order:order`** → `["/api/admin/order/orders", ".../orders/*"]`。
- 菜单种子（迁移 `20261009000001_menus_vben_backfill`）：根目录「订单」`/order`（perm `order`）→ 页面「订单列表」`/order/orders`（perm **`order:order:*`**，component `views/order/OrderOrdersPage`）→ 绑 admin。

### 7.3 后端端点（3 条）

| 方法 | 路径 | handler | 说明 |
| --- | --- | --- | --- |
| GET | `/api/admin/order/orders` | ListOrders | 订单列表（信封 `data:[...]`，**无分页**，注释标注生产需补分页/租户过滤/权限过滤） |
| GET | `/api/admin/order/orders/export` | ExportOrders | 导出当前租户全量订单 xlsx（下载流，见 [usage.md §4](usage.md)） |
| POST | `/api/admin/order/orders` | CreateOrder | 校验用户 → 落库 → 同事务写 outbox |

### 7.4 数据模型

- `orders`：ID/UserID/ProductID/Amount/Status/OrderNo/CreatedAt/UpdatedAt，`idx_orders_order_no`；**无 tenant_id、无软删**——service 注释宣称多租户，但插件因模型无 `TenantID` 字段不隔离该表（**已知 gap**）。
- 关联写 `systemmodel.OutboxEvent` → `outbox_events`（带 tenant_id）。

### 7.5 跨模块调用与事件

- **order → system**：`userSvc.GetUserByID`；mono 本地直调；micro 经 `fx.Decorate` 换 `pkg/client/system_user_grpc.go`——**该 client 6 个方法全部返回 "not implemented" 占位**，且 system 侧无对应用户服务 proto/server。
- **事件链**：`outbox_events(pending)` → `pkg/outbox/relay`（默认开，batch=100、interval=5s、`FOR UPDATE SKIP LOCKED`）→ NATS JetStream `Publish(subject=EventType)`（前缀 `alexgo.>`）→ 成功标 published / 失败 failed。**仓库内没有 `order.created` 的订阅方**。
- 自身 proto `order/api/admin/v1/order.proto` 仅为草案：无生成代码、无注册。

### 7.6 前端页面

| 路由 | 组件 | 功能 | 权限码 |
| --- | --- | --- | --- |
| `/order/orders` | `views/order/OrderOrdersPage.vue` | 分页表格 + 新建订单弹窗（order_no/user_id/product_id/amount/status） | 菜单 `order:order:*`；按钮 `v-access:code="['order:order:*']"` |

---

## 8. 前端总览（apps/web-antd）

### 8.1 路由与权限模式

- **`accessMode: 'backend'`**（`src/preferences.ts`）：菜单 + 动态路由全部后端下发，前端**零静态业务路由表**（`router/routes/modules/` 目录不存在；静态路由只有 `core.ts` 的登录/回调/404，例外 `external/scorecard.ts`——scorecard 房间详情子页，T14 引入、不进菜单，见 §11.9）。
- 守卫流程（`router/guard.ts` + `router/access.ts`）：无 token → `/login`；首次进站拉 `GET /api/user/info` → `GET /api/menu/all` → `generateRoutesByBackend` 把 `component` 字符串（如 `views/system/SystemUsersPage`）经 `import.meta.glob('../views/**/*.vue')` 解析成组件（找不到回落 not-found）→ 目录节点补 redirect 到首子路由 → 生成侧栏菜单 → `setAccessChecked`（**动态路由只生成一次**；`accessCodes` 持久化 localStorage，菜单/路由不持久化，刷新重拉）。
- 后端路由命名：`/system/users` → `system_users`；`meta` 仅 `title/icon/orderNo/order/permissions`（**无 keepAlive/hideInMenu/link** → 标签页缓存、iframe 外链暂不可用）。
- **`v-access:code` 指令**：Set 精确交集，不命中直接 `el.remove()`；多码任一命中即显示。`v-access:role` 在 backend 模式下退化为按 code（无人使用）。

### 8.2 完整菜单树（admin 角色，后端 `GET /api/menu/all` 下发）

```
├─ 工作台            /dashboard            perm=""
│  ├─ 分析看板       /dashboard/analytics  views/dashboard/analytics/index
│  └─ 工作台         /dashboard/workspace  views/dashboard/workspace/index
├─ 订单              /order                perm="order"
│  └─ 订单列表       /order/orders         views/order/OrderOrdersPage      perm="order:order:*"
├─ 系统管理          /system               perm="system"  sort=10
│  ├─ 用户管理       /system/users         views/system/SystemUsersPage     perm="system:user:list"
│  ├─ 角色管理       /system/roles         SystemRolesPage                  perm="system:role:*"
│  ├─ 菜单管理       /system/menus         SystemMenusPage                  perm="system:menu:*"
│  ├─ 部门管理       /system/depts         SystemDeptsPage                  perm="system:dept:*"
│  ├─ 岗位管理       /system/posts         SystemPostsPage                  perm="system:post:*"
│  ├─ 字典管理       /system/dict          SystemDictPage                   perm="system:dict:*"
│  ├─ 系统参数       /system/configs       SystemConfigsPage                perm="system:config:*"
│  ├─ 通知公告       /system/notices       SystemNoticesPage                perm="system:notice:*"
│  ├─ 登录日志       /system/logins        SystemLoginLogsPage              perm="system:log:login"
│  ├─ 操作日志       /system/operates      SystemOperateLogsPage            perm="system:log:operate"
│  ├─ 租户管理       /system/tenants       SystemTenantsPage                perm="system:tenant:*"
│  ├─ 我的通知       /system/notices/mine  SystemMyNoticesPage              perm="system:notice:mine"
│  ├─ 在线用户       /system/online-users  SystemOnlineUsersPage            perm="system:online:list"
│  ├─ [按钮] 踢人      perm="system:online:kick"
│  ├─ [按钮] 代登录    perm="system:online:impersonate"
│  ├─ [按钮] 登录鉴权   perm="system:auth:profile"
│  ├─ [按钮] 租户管理    perm="system:tenant:manage"
│  ├─ [按钮] 会员管理    perm="member:user:manage"
│  └─ 渠道配置        /social              perm="social"  sort=92
│     └─ 渠道配置      /social/apps         views/system/SocialAppsPage    perm="social:app"
│        ├─ [按钮] 新建   perm="social:app:create"
│        ├─ [按钮] 编辑   perm="social:app:update"
│        └─ [按钮] 删除   perm="social:app:delete"
├─ Scorecard         /scorecard            perm="scorecard"  sort=80
│  ├─ Rooms          /scorecard/rooms      views/scorecard/ScorecardRoomsPage     perm="scorecard:room"
│  ├─ Records        /scorecard/records    views/scorecard/ScorecardRecordsPage   perm="scorecard:record"
│  └─ Friends        /scorecard/friends    views/scorecard/ScorecardFriendsPage   perm="scorecard:friend"
├─ 代码生成          /codegen               perm="infra"  sort=90
│  └─ 代码生成配置   /codegen/config        views/infra/codegen/index        perm="infra:codegen"
│     ├─ [按钮] 导入   perm="infra:codegen:import"
│     ├─ [按钮] 同步   perm="infra:codegen:sync"
│     └─ [按钮] 删除   perm="infra:codegen:delete"
└─ OAuth2            /oauth2                perm="oauth2"  sort=91
   └─ 客户端管理     /oauth2/clients        views/system/Oauth2ClientsPage   perm="oauth2:client"
      ├─ [按钮] 新建   perm="oauth2:client:create"
      ├─ [按钮] 编辑   perm="oauth2:client:update"
      └─ [按钮] 删除   perm="oauth2:client:delete"
```

- 按钮（type=button）不进路由/菜单，只贡献权限码。
- `/codegen`、`/oauth2`、`/social` 目录 parent_id 用 `COALESCE(系统管理 id, 0)`——**迁移先于 seed 执行的全新库会落到根级**，以实际库为准。
- `/scorecard` 目录与 3 页面是迁移种子的**英文占位名**（`Scorecard`/`Rooms`/`Records`/`Friends`，system 迁移 `20261020000001_scorecard_menus.up.sql`，parent_id=0 直挂根级），后台菜单管理可改中文；房间详情子页 `/scorecard/rooms/detail` **不在菜单**，走前端 externalRoutes 静态注册（见 §11.8）。

### 8.3 静态核心路由（不走权限）

| 路由 | 功能 |
| --- | --- |
| `/` | BasicLayout 根容器，redirect → `/dashboard/analytics` |
| `/auth/login` | 账号密码登录（滑块验证码 + OAuth2 PKCE，走 `authorize/login` → 302 → `/callback`） |
| `/auth/{code-login,qrcode-login,forget-password,register}` | vben 模板遗留（未接后端） |
| `/callback` | 授权码回调：换 token → 并发拉 `user/info` + `auth/codes` → 跳目标页（`ignoreAccess`） |
| `/:path(.*)*` | 404；403 页面作 forbiddenComponent 备用 |

**孤儿/死链（模板遗留）**：`views/demos/antd`（无路由）、`views/_core/{about,profile,…settings}`（无路由，但顶栏用户下拉 `router.push({name:'Profile'})` 是**死链**）、workspace 页内 `/demos/features/*` 死链。设计文档已要求删除 `views/demos` 但未执行。

### 8.4 API 封装层（`src/api/`）

| 文件 | 对应后端 | 主要导出 |
| --- | --- | --- |
| `request.ts` | 公共客户端 | `requestClient`（统一信封解包 `code===0 → data`，非 0/非 2xx 走 rejected 弹后端 `msg`、401 弹登录过期）、`baseRequestClient`（refresh 用）、`download`（Blob 下载流） |
| `core/auth.ts` | vben 契约 | `logoutApi`、`getAccessCodesApi`、`refreshTokenApi`（一期已关） |
| `core/user.ts` | vben 契约 | `getUserInfoApi` |
| `core/menu.ts` | vben 契约 | `getAllMenusApi` |
| `core/oauth2.ts` | oauth2 协议端点 | `authorizeLoginApi`、`exchangeTokenApi`——**裸 fetch 不走拦截器**（因 RFC6749 信封/302 语义） |
| `admin.ts` | system + order | 用户/角色/菜单/部门/岗位/字典/参数/公告/日志/订单/OAuth2 client/租户/在线用户（list/kick/impersonate）/渠道配置 social（list/create/update/delete，见 §6.7） 全套函数 + TS 类型 |
| `infra.ts` | infra | codegen 11 函数 + 文件存储（`uploadFile` multipart、`getFile` 等，见 §4.7）+ `saveBlob`（委托 `downloadFileFromBlob`）+ 类型 |
| `scorecard.ts` | scorecard admin 监管 | `getRoomsApi` / `getRoomDetailApi` / `closeRoomApi` / `getRecordsApi` / `getFriendsApi` + 3 类型（app 端 18 接口由微信小程序侧接入，本仓库不封装，见 §11） |

**已知封装缺口**：`PUT /admin/system/users/:id`、`POST /users/:id/roles`（用户设角色）、`POST /roles/:id/menus`（角色授权菜单——UI 也缺）、各类 `PUT`（部门/岗位/字典/参数/公告——页面因此只有新建删除）。

### 8.5 布局与通用能力

- `layouts/basic.vue`（@vben/layouts BasicLayout）：顶栏用户下拉、通知中心（mock）、锁屏、登录过期弹窗、水印；侧栏/标签页/主题/暗色/紧凑/多语言由 @vben/layouts + preferences 提供。
- `preferences.ts`：`accessMode:'backend'`、`defaultHomePath:'/dashboard/analytics'`、`enableRefreshToken:false`、`tenantMode`、`defaultTableSize` 等。
- 适配器：`adapter/{vxe-table,form,component}`——页面统一 VxeGrid + `Page`/`useVbenModal`/`useVbenForm`。
- 工具：`utils/local-page.ts`（旧接口全量返回前端切页）、`utils/pkce.ts`、`utils/format.ts`。
- i18n 仅覆盖 vben 通用文案（zh-CN/en-US）；**菜单与页面标题来自后端中文 `name`，不走 i18n**。

---

## 9. 权限码速查总表

### 9.1 后端 casbin 前缀 → 路由（`permission_routes.go`，共 21 组）

| 前缀 | 保护路由 |
| --- | --- |
| `system:user` | `/api/admin/system/users`, `/api/admin/system/users/*` |
| `system:role` | `/api/admin/system/roles`, `.../roles/*` |
| `system:menu` | `/api/admin/system/menus`, `.../menus/*` |
| `system:dept` | `/api/admin/system/depts`, `.../depts/*` |
| `system:post` | `/api/admin/system/posts`, `.../posts/*` |
| `system:dict` | `/api/admin/system/dict/types(+/*)`, `/api/admin/system/dict/datas(+/*)` |
| `system:config` | `/api/admin/system/configs`, `.../configs/*` |
| `system:notice` | `/api/admin/system/notices`, `.../notices/*` |
| `system:log` | `/api/admin/system/logs/login`, `.../logs/operate` |
| `system:online` | `/api/admin/system/online`, `/api/admin/system/online/*` |
| `system:auth` | `/api/admin/system/auth/profile`, `.../auth/refresh`, `.../auth/logout` |
| `system:tenant` | `/api/admin/system/tenants`, `.../tenants/*` |
| `infra:codegen` | `/api/admin/infra/codegen`, `/api/admin/infra/codegen/*` |
| `infra:file` | `/api/admin/infra/file`, `/api/admin/infra/file/*` |
| `oauth2:client` | `/api/admin/oauth2/clients`, `/api/admin/oauth2/clients/*` |
| `social:app` | `/api/admin/social/apps`, `/api/admin/social/apps/*` |
| `member:user` | `/api/admin/member/users`, `/api/admin/member/users/*` |
| `order:order` | `/api/admin/order/orders`, `/api/admin/order/orders/*` |
| `scorecard:room` | `/api/admin/scorecard/rooms`, `/api/admin/scorecard/rooms/*` |
| `scorecard:record` | `/api/admin/scorecard/records`, `/api/admin/scorecard/records/*` |
| `scorecard:friend` | `/api/admin/scorecard/friends`, `/api/admin/scorecard/friends/*` |

### 9.2 前端 `v-access:code` 使用清单

| 权限码 | 页面 | 控制对象 |
| --- | --- | --- |
| `system:user:list` | SystemUsersPage | 新建用户、重置密码 |
| `system:role:*` | SystemRolesPage | 新建/删除角色 |
| `system:menu:*` | SystemMenusPage | 新建/编辑/删除菜单 |
| `system:dept:*` | SystemDeptsPage | 新建/删除部门 |
| `system:post:*` | SystemPostsPage | 新建/删除岗位 |
| `system:dict:*` | SystemDictPage | 字典类型/数据 新建删除（5 处） |
| `system:config:*` | SystemConfigsPage | 新建/删除参数 |
| `system:notice:*` | SystemNoticesPage | 新建/删除公告 |
| `system:tenant:*` | SystemTenantsPage | 新建/编辑/删除租户 |
| `system:online:kick` | SystemOnlineUsersPage | 踢出会话按钮 |
| `system:online:impersonate` | SystemOnlineUsersPage | 代登录按钮 |
| `order:order:*` | OrderOrdersPage | 新建订单 |
| `oauth2:client:create/:update/:delete` | Oauth2ClientsPage | 客户端增改删（分按钮） |
| `social:app:create/:update/:delete` | SocialAppsPage | 渠道配置增改删（分按钮） |
| `infra:codegen` | codegen/index.vue | 生成/字段配置/编辑/预览/下载 |
| `infra:codegen:import` | 同上 | 导入表 |
| `infra:codegen:sync` | 同上 | 同步 |
| `infra:codegen:delete` | 同上 | 删除 |

仅菜单/后端使用（前端无 v-access）：目录级 `system`/`order`/`infra`/`oauth2`/`scorecard`/`social`、`system:log:login`、`system:log:operate`、`system:auth:profile`、`system:tenant:manage`、`member:user:manage`、`system:online:list`、`scorecard:room`/`scorecard:record`/`scorecard:friend`（菜单可见性 + 后端 casbin 双层控制，页内不拆按钮，见 §11.8）。

### 9.3 注意事项

1. 前端**逐字精确**匹配（`*` 是字面量），改菜单权限码 = 前端按钮消失。
2. 后端只取**前两段前缀**（method 级放二期）；新前缀必须登记 `permissionRoutes` 否则 403。
3. `system:user:list` 被写操作按钮复用——读写粒度不足，待拆。
4. 有 `policy_sync_test.go` / `permission_routes_test.go` 保证：种子权限码前缀必须已注册路由、角色按权限码必须拿到对应路由。
5. `system:notice:mine`（我的通知）按一期前缀模型等价于公告模块全权（`/notices/*` act `.*`）——勿单独授给非管理角色；粒度拆分挂二期按 method 精确路由（含 mine/read/read-all 三端点）。

---

## 10. 文档地图

| 文档 | 覆盖 | 与本文分工 |
| --- | --- | --- |
| [README](../README.md) | 门面：技术栈、快速开始、命令 | 入口 |
| [architecture.md](architecture.md) | 装配、中间件链、Outbox、配置、测试、部署 | 横切"怎么跑、怎么装配"；权限细节看本文 §2 |
| [oauth2.md](oauth2.md) | 登录与签发：SSO 链路、四种 grant、错误排查 | 本文 §6 的"操作向"扩展 |
| [oauth2-compat-retirement.md](oauth2-compat-retirement.md) | 老登录端点下线条件清单 | 本文 §3.3/§5.3 兼容端点的去留依据 |
| [service-communication.md](service-communication.md) | mono/micro 双形态、gRPC 委托、失败语义 | 本文 §1.2 的展开 |
| [usage.md](usage.md) | 菜单管理 + 代码生成 + 文件上传（§3）+ Excel 导出（§4）+ 新建模块脚手架（§5）操作手册 | 本文 §3/§4 的"怎么用"视角 |
| [roadmap.md](roadmap.md) | P0/P1/P2 分层排期与收口状态 | 本文只记现状，进度与欠账看它 |
| [superpowers/specs/2026-10-19-module-template-design.md](superpowers/specs/2026-10-19-module-template-design.md) | newmodule 脚手架设计：触点清单、全有或全无、回滚语义 | 用法在 usage.md §5；新增模块后**手工回填本文对应章节**（脚手架不自动改 modules.md） |

---

## 11. scorecard — 打牌记账模块

### 11.1 功能

线下打牌的**纯分数记账**全链路：房间（开房/进房/配置/流水）、记分支出、茶饭池（自动百分比 + 手动 DIY）、结算/退房/转让房主、战绩与统计、好友/拉黑/排名开关。产品红线是**只记分数**——代码、字段、接口命名一律 `score`/`tea`，全程计量只有分数与茶饭两样。设计与需求见
[`superpowers/specs/2026-10-20-scorecard-module-design.md`](superpowers/specs/2026-10-20-scorecard-module-design.md)
与 [modules/dapaijizhang.md](modules/dapaijizhang.md)（微信小程序端不在本仓库，接本节 app 端 18 接口）。

装配要点（`modules/scorecard/module.go`；`alexgo-server/configs/config.yaml` 的 `modules.scorecard: false` 时 `RegisterRoutes` 直接返回）：

- **Repository**：Room / RoomUser / RoomLog / Record / Friend / FriendBlack 六类；写路径 `FOR UPDATE` 行锁 + `SaveVersioned`（`WHERE id AND version`）乐观锁
- **Service**：`RoomService`（开房/进房/快照/改配置/支出/流水/结算/退房/转让 + 监管三方法）、`RecordService`（我的战绩/明细/删除/统计 + 监管列表）、`FriendService`（好友/拉黑/排名开关 + 监管列表）
- **Controller**：`AppController`（18 接口，Bearer 自鉴权）与 `AdminController`（5 接口，casbin 放行后只做参数与领域校验，**不取 claims、监管动作不落操作者身份**）
- **后台任务**：超时关房 ticker（每分钟 `SweepExpired`，见 §11.6）
- 用户体系**复用 member**：不建用户表、不做登录，身份一律 `member_user.id`；`scorecard.FxModule` 装配于 `alexgo-server/cmd/main.go`

### 11.2 权限与菜单

| 端 | 范围 | 鉴权 |
| --- | --- | --- |
| app | `/api/app/scorecard/**`（18 条） | handler 内解析 `Authorization: Bearer` → `token.Validator.Validate` → `UserType==Member`，**不走 casbin**（与 member app 端同构）；缺/坏 token 401 `20001`、非 member 403 `20003`、路径 id 非法 400 `10001` |
| admin | `/api/admin/scorecard/**`（5 条） | 管理员 token + casbin（`user_type=1` 门槛在前），三前缀见 §9.1：`scorecard:room` / `scorecard:record` / `scorecard:friend` |

菜单种子在 **system 模块迁移**
`modules/system/migrations/20261020000001_scorecard_menus.up.sql`（全部 `NOT EXISTS` 幂等、`tenant_id=0`）：
目录「Scorecard」`/scorecard`（perm `scorecard`、`lucide:package`、sort=80、直挂根级）+ 3 页面
「Rooms」`/scorecard/rooms`、「Records」`/scorecard/records`、「Friends」`/scorecard/friends`
（perm 取前两段，component 指向 §11.9 三个列表页）。菜单名是**英文占位**，后台菜单管理可改中文；
启动 seed 全量绑 admin 角色并重建策略（§2.6）。房间详情页**不在菜单**（见 §11.9）；
按钮级权限不拆——后端只认前两段前缀（§9.3）。

### 11.3 app 端点（18 条，前缀 `/api/app/scorecard`）

| 方法 | 路径 | handler | 说明 |
| --- | --- | --- | --- |
| POST | `/rooms` | AppController.CreateRoom | 开房：`room_code` 可空 = 服务端生成 4 位去混淆房号（冲突重试 5 次、耗尽 30106），可带默认配置；创建者入房（`is_creator=1`、`session_no=1`），`version=1`、`expire=now+2h`，返回快照 |
| POST | `/rooms/join` | AppController.JoinRoom | 按 `room_code` 进房：已结算 30104 / 已关闭 30103 拒、与任一成员互拉黑 30108 拒；**幂等**——已在房直接返回现状（不重复加人、不记流水、不刷过期）；成员行是硬删，重进 = 新会话（`session_no=1`、计分清零） |
| GET | `/rooms/:id/snapshot` | AppController.Snapshot | 全量快照：`version` / 成员分 / 配置 / 茶饭池 / 最近 20 条流水；非成员 30102；昵称头像走 member BriefProvider（拉取失败降级空昵称） |
| PUT | `/rooms/:id/config` | AppController.UpdateConfig | 改配置，仅房主 30105；数值项写 `pending_*` 且 `pending_apply_round_no = round_no+1`（**从下把生效**，下一笔支出整组刷入），`tea_auto_enabled` 提供即改生效字段；刷心跳 |
| POST | `/rooms/:id/expense` | AppController.Expense | 记分支出核心事务：body `score` / `tea` / `to_user_id` / `client_request_id`（幂等键，必填）/ `tea_from_user_id`（手动 DIY 来源，缺省 = 支出方）；行锁 → 幂等回放 → `round_no+1` 应用到期 pending → 茶饭计算（自动 `T=round(S×percent)` 从赢家所得扣）→ 记账（总分守恒）→ 达标关池 → 上限频控（本会话首提醒全房）→ type=1 流水 → `version` 落库，任一步失败整体回滚 |
| GET | `/rooms/:id/logs` | AppController.Logs | 流水分页（`page`/`size`，size ≤ 100，默认 1/10） |
| POST | `/rooms/:id/settle` | AppController.Settle | 结算，仅房主：`status 0→1` + 批量写全员 `scorecard_record` + type=5 流水，此后禁记分（30104） |
| POST | `/rooms/:id/leave` | AppController.Leave | 退房：进行中房房主先自动结算（全员 `scorecard_record`、`settle_type=2`）再退；普通成员先为自己写 `scorecard_record`（全零 → `settle_type=4` 空战绩，否则 3）再删行 + type=4 流水；最后一人离开 → `status=2` |
| POST | `/rooms/:id/transfer-owner` | AppController.TransferOwner | 转让房主（`target_user_id`）：仅房主、`status=0`、目标在房 30113、非同人 30107、双方不互拉黑 30108 → `creator_id` 换、两行 `is_creator` 互换、type=9 流水；V1 不需对方确认 |
| GET | `/records` | AppController.MyRecords | 我的战绩分页（时间倒序） |
| GET | `/records/:recordId` | AppController.RecordDetail | 明细：不存在 30110 / 非本人 30111 → 按 `room_id` 反查全房成员行 |
| DELETE | `/records/:recordId` | AppController.DeleteRecord | 删本人战绩行（30110 / 30111） |
| GET | `/stat` | AppController.Stat | 我的统计（聚合 `scorecard_record`：`total_score`/`win`/`lose`/`draw`/`win_rate`，胜率 = 胜/(胜+负)） |
| GET | `/friends` | AppController.ListFriends | 好友列表（含 `rank_enabled` 排名开关） |
| POST | `/friends` | AppController.AddFriend | 加好友（`friend_user_id`）：对方存在性 → 已是好友 30109 → 双向拉黑 30108 → 双向各写一行 |
| DELETE | `/friends/:userId` | AppController.RemoveFriend | 删好友 |
| POST | `/friends/:userId/black` | AppController.SetFriendBlack | 拉黑/解除（`blocked` 0/1）；任一方向命中即双向拦截进房与加好友（30108） |
| PUT | `/friends/:userId/rank` | AppController.SetFriendRank | 排名参与开关（`enabled` 0/1） |

### 11.4 admin 端点（5 条，前缀 `/api/admin/scorecard`）

| 方法 | 路径 | handler | 权限码 | 说明 |
| --- | --- | --- | --- | --- |
| GET | `/rooms` | AdminController.ListRooms | `scorecard:room` | 房间分页（`status` 缺省 -1 不过滤；`room_code` 缺省空串不过滤、非空精确等值；`page`/`size` size ≤ 100） |
| GET | `/rooms/:id` | AdminController.RoomDetail | `scorecard:room` | 房行 + 全量成员 + 最近流水（**cap 200 条**，全量流水走 app 端分页接口）；房不存在/跨租户 30101 |
| POST | `/rooms/:id/close` | AdminController.CloseRoom | `scorecard:room` | **唯一运维写动作**：强制关房，`status` 0/1 → 2，无房主校验；行锁 + `version` 就地 +1 + type=8 流水（`extra.event=admin_close`）；已关再关 30103 |
| GET | `/records` | AdminController.ListRecords | `scorecard:record` | 战绩分页（`user_id`/`room_id` 传 0 = 不过滤） |
| GET | `/friends` | AdminController.ListFriends | `scorecard:friend` | 好友/拉黑关系分页（只读） |

### 11.5 数据模型（6 表）

迁移 `modules/scorecard/migrations/20261020000001_scorecard_tables.{up,down}.sql`
（`CREATE TABLE IF NOT EXISTS`、无外键），表名**单数**（照需求文档 §7 原名，
以 `modules/scorecard/model` 的 `TableName()` 与 gorm 标签为准）；
**6 表全带 `tenant_id`** + `created_at`/`updated_at`，走 GORM 租户插件隔离（不重蹈 §7.4 `orders` 的 gap）：

| 表 | 要点 |
| --- | --- |
| `scorecard_room` | `room_code`（唯一，4 位）、`creator_id`（当前房主）、`status` 0 进行中 / 1 已结算 / 2 已关闭、`expire_time`、茶饭配置 `tea_target_amount`/`tea_percent`/`tea_pool_total`/`tea_auto_enabled`、`max_lose_score`、`round_no` + `pending_tea_*`/`pending_max_lose_score`/`pending_apply_round_no`（从下把生效）、**`version` 乐观锁** |
| `scorecard_room_user` | `room_id`+`user_id` 唯一键、`cur_score`、`cum_lose_score`、`is_creator`、`session_no`（本行进房序次；退房硬删、重进从 1 起） |
| `scorecard_room_log` | 流水：`from_user_id`/`to_user_id`、`score`、`tea`、`type` 1 记分 / 2 茶饭 / 3 进房 / 4 退房 / 5 结算 / 6 上限提醒 / 7 配置变更 / 8 状态变更 / 9 转让房主、`client_request_id`（幂等键）、`extra`（json 扩展备注）；索引 `(room_id,created_at)` |
| `scorecard_record` | **每成员一行**：`score`/`cum_lose_score` + 终局配置冗余（`tea_pool_total`/`tea_percent`/`tea_target_amount`/`max_lose_score`，统计不 JOIN）、`settle_type` 1 正常结算 / 2 房主退房自动结算 / 3 退房 / 4 零数据空战绩；索引 `(user_id,created_at)`、`(room_id)` |
| `scorecard_friend` | `user_id`+`friend_user_id` 唯一键、`rank_enabled` 排名开关 |
| `scorecard_friend_black` | `user_id`+`blocked_user_id` 唯一键；任一方向命中即双向拦截 |

仓内「不存在」与「跨租户」统一软查为 `nil` 再由 service 映射 30101（房）/ 30110（战绩）——
租户插件过滤在前，两种原因不可区分。

### 11.6 跨模块与后台任务

- **member BriefProvider（fx optional）**：窄只读接口 `modules/member/service/brief.go` 的
  `GetBriefs(ctx, ids) → []UserBrief{ID, Nickname, Avatar, Status}`，映射在 `modules/member/module.go`；
  scorecard 侧 `RoomService`/`FriendService` 的该注入标 `optional:"true"`——**micro 形态（本进程不装
  member.FxModule）仍可装配**，缺席时快照昵称降级为空、加好友跳过对方存在校验。敏感列不在
  SELECT 面内，不存在/已删的 id 自然缺席。
- **超时关房 ticker**（`module.go` 的 `fx.Invoke`，照 system token `SweepExpired` 模式）：
  OnStart 起 goroutine **每分钟**跑 `RoomService.SweepExpired`——跨租户拉
  `status=0 AND expire_time < now` 的房 → 逐房事务 `FOR UPDATE` 复验（并发下可能已被结算/关闭/续期）
  → `status=2` + type=8 流水（`extra.timeout_close`）；错误只记 warn 不致命、下轮再扫；
  OnStop cancel 退出。
- **心跳口径**：`expire_time = now + 2h` 只在**开房/进房/退房/配置变更**与**每笔支出**时刷新；
  **快照读不构成心跳、不续期**。

### 11.7 实时一致（无 WS）

设计决策 6：不做 WebSocket。客户端轮询 `GET /rooms/:id/snapshot` 取全量状态
（`version` / 成员分 / 配置 / 茶饭池 / 最近流水）；支出等写操作的响应带回最新 `version`
（支出另带 `log_id`、实收茶饭、首达上限成员名单）。`version` 由 `SaveVersioned`
（`UPDATE ... WHERE id AND version`，`RowsAffected=0` → 30112）**单调递增**：客户端以快照对齐、
按 version 去重并丢弃过期响应，同房间写操作经行锁天然串行（分数守恒的前提）。

### 11.8 错误码（30101–30113）

`pkg/errors/code.go` 的 scorecard 段，落在 3xxxx 业务段（HTTP 400）：

| 码 | 含义 | 码 | 含义 |
| --- | --- | --- | --- |
| 30101 | 房间不存在或非本租户 | 30108 | 存在拉黑关系，禁止进房/加好友 |
| 30102 | 非房间成员 | 30109 | 好友关系已存在 |
| 30103 | 房间已关闭 | 30110 | 战绩不存在 |
| 30104 | 房间已结算，禁止记分 | 30111 | 仅能操作本人战绩 |
| 30105 | 仅房主可操作 | 30112 | 并发冲突，请刷新快照重试 |
| 30106 | 房号生成冲突重试耗尽 | 30113 | 目标不是房间成员 |
| 30107 | 转让目标无效（不在房/拉黑/同人） | | |

### 11.9 前端页面（apps/web-antd，4 页）

| 路由 | 组件 | 功能 | 权限与可见性 |
| --- | --- | --- | --- |
| `/scorecard/rooms` | `views/scorecard/ScorecardRoomsPage.vue` | 房间分页列表（状态下拉 + 房号输入筛选，随查询传 `status`/`room_code`、空值不传参；详情入口、强制关房二次确认，仅 `status===0` 给入口） | 菜单行 perm `scorecard:room` |
| `/scorecard/rooms/detail` | `RoomDetailPage.vue`（query `?id=`） | 房行信息 + 成员表 + 流水（本地分页，后端 cap 200） | **不在菜单**：走 `src/router/routes/external/scorecard.ts` 静态注册（`meta.hideInMenu`，保留登录守卫），数据权限由 admin API 的 casbin（`scorecard:room`）强制 |
| `/scorecard/records` | `ScorecardRecordsPage.vue` | 战绩分页（`user_id`/`room_id` 筛选；「看全房」按 `room_id` 收窄列表，实现是列表按房筛、非弹窗） | 菜单行 perm `scorecard:record` |
| `/scorecard/friends` | `ScorecardFriendsPage.vue` | 好友/拉黑关系只读列表 | 菜单行 perm `scorecard:friend` |

- **详情页为何不进菜单**：`accessMode: 'backend'` 下非 core 路由只由 `/api/menu/all` 的菜单树生成，
  补菜单行会在侧栏多出可见项（`menus` 表也无 hide 列）——故走 externalRoutes 静态注册
  （§8.1「前端零静态业务路由表」的唯一例外），精确路径赢过 catch-all 404、不吞三个列表路径。
- API 封装 `src/api/scorecard.ts`（5 函数 + 3 类型，`requestClient` → `/api/admin/scorecard/**`）；
  **页内不使用 `v-access`**——可见性靠菜单、操作靠后端 casbin（§9.2 已列入「仅菜单/后端使用」）。
- 脚手架按实体生成的 6 套 CRUD 视图与 api 目录已全部删除，scorecard 路径下只剩上述 4 页 + `api/scorecard.ts`。
- app 端 18 接口由微信小程序侧（本仓库外）接入，前端不封装。

---

## 12. fishgame — 摸鱼小游戏后端模块

### 12.1 功能

微信小游戏《摸鱼》后端：X-Session 自持会话、每日循环（签到/领筐/运气）、埋点批量上报、
图鉴双路径记账（报告幂等合并 + 事件回放校正）、库内好友榜、分享图上传与公开读、
确定性鱼王（周粒度 sha256）。设计见
[`superpowers/specs/2026-10-23-fishgame-module-design.md`](superpowers/specs/2026-10-23-fishgame-module-design.md)；
客户端为 Godot 小游戏（`scripts/online_service.gd`，本仓库外），接本节 v1 契约 10 端点。

装配要点（`modules/fishgame/module.go`；`modules.fishgame: false` 时 `RegisterRoutes` 直接返回）：

- **Repository**：Player / Session / Fish / Dex / TrackEvent / Sign / Friend / Config / Share 九类
- **Service**：`AuthService`（code2session + X-Session）、`DailyService`、`TrackService`（canonical dedup）、
  `DexService`（双路径合并）、`FriendService`（双口径排名）、`ShareService`、`AdminService`
- **Controller**：`controller/app`（10 契约端点，**裸 JSON 直出**，错误 `{"error":"<english>"}`）
  与 `controller/admin`（13 路由 5 组，统一信封 + casbin，不取 claims）
- 用户体系**独立**：不复用 member/oauth2 token，身份 = `openid`（code2session 换取），
  会话 = 自持 `X-Session` 随机串（`fishgame_session` 表）；game-end 类逻辑 ctx `tenant_id=0`

### 12.2 权限与菜单

| 端 | 范围 | 鉴权 |
| --- | --- | --- |
| app | `/api/v1/fishgame` 契约 10 条（实际挂 `/api/v1/**`） | `X-Session` 中间件（login 免挂）；缺/坏会话 401 |
| public | `/api/public/fishgame/share/:key` | 免鉴权（签名 URL 公开读） |
| admin | `/api/admin/fishgame/**`（13 路由 5 组） | 管理员 token + casbin，五前缀见 §9：`fishgame:fish` / `fishgame:config` / `fishgame:event` / `fishgame:player` / `fishgame:stats` |

菜单种子在 system 模块迁移
`modules/system/migrations/20261023000001_fishgame_menus.up.sql`（全部 `NOT EXISTS` 幂等、`tenant_id=0`）：
目录「Fishgame」`/fishgame`（perm `fishgame`、`lucide:package`、sort=85）+ 5 页面
Fishes/Configs/Events/Players/Stats（perm `fishgame:fish/config/event/player/stats`，
component 指向 §12.8 五个页面）。启动 seed 全量绑 admin 并重建策略（§2.6）。

### 12.3 契约端点（10 条，前缀 `/api/v1`，裸 JSON）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/auth/login` | code2session 换 openid → 建/续 `X-Session`；免中间件（自鉴权入口） |
| GET | `/config/daily` | 每日配置（时段/水层权重，luck 配置生效态） |
| POST | `/daily/sign` | 签到（幂等当日一次） |
| GET | `/basket` | 领筐状态 |
| POST | `/track` | 埋点批量上报（canonical dedup key 幂等） |
| GET | `/dex/table` | 图鉴表（fish 元数据 + 我的点亮数） |
| POST | `/dex/report` | 打捞报告（双路径记账：MergeReport 幂等 + 事件回放校正） |
| POST | `/friends` | 加好友（库内关系，一期） |
| GET | `/friends/rank` | 好友榜（点亮数 desc、nick asc 决胜；空好友集 → 全体榜） |
| POST | `/share/image` | 分享图上传 → 返回签名 URL key |

响应**无 `code` 顶层键**（`pkg/resp` 不经手）；错误统一裸形状 `{"error":"<english>"}`。
完整字段形状见 spec §4.1 与 `docs/backend-api.md`。

### 12.4 admin 端点（13 路由 5 组，前缀 `/api/admin/fishgame`）

| 组 | 路由 | 权限码 | 说明 |
| --- | --- | --- | --- |
| fish | GET/POST `/fishes`、PUT/DELETE `/fishes/:id` | `fishgame:fish` | 图鉴条目 CRUD：`rarity` 筛选、`status` 缺省 -1 不过滤；重复 fish_id 30401；枚举校验 10001；删除引用检查（dex/事件 props LIKE）→ 30402 |
| config | GET/POST `/configs`、PUT/DELETE `/configs/:id` | `fishgame:config` | 运营配置：type `luck`/`fish_king`；luck `depth_bias∈[0,0.3]`；fish_king fish_id+week 必填、鱼不存在/停用 30403；查无 30404 |
| event | GET `/events` | `fishgame:event` | 埋点分页（name/player_id/起止时间筛选，id 倒序） |
| player | GET `/players`、GET `/players/:id`、GET `/players/:id/rank` | `fishgame:player` | 玩家监管：openid/nick 模糊 + 行内点亮数；详情（点亮明细 count≥1 + 最近 20 事件）；榜单位次（客服排查）；查无 30002 |
| stats | GET `/stats` | `fishgame:stats` | 近 7 日看板：玩家总数/今日新增、事件计数、按日趋势、四结果漏斗（catch/empty/escape/broken）+ 占比、签到/领筐/广告计数 |

admin 全走 `resp.OK/Fail/Page` 统一信封；`resp.statusFor` 把 304xx 映射 HTTP 400。

### 12.5 数据模型（9 表）

迁移 `modules/fishgame/migrations/20261023000001_fishgame_tables.{up,down}.sql`
（`CREATE TABLE IF NOT EXISTS`、无外键），48 条 fish 种子在
`20261023000002_fishgame_fish_seed.up.sql`。**9 表全带 `tenant_id`** + `created_at`/`updated_at`：

| 表 | 要点 |
| --- | --- |
| `fishgame_player` | `openid` 唯一、`nick`、`last_login_at` |
| `fishgame_session` | `X-Session` 随机串、`player_id`、过期 |
| `fishgame_fish` | 48 条种子：`fish_id` 唯一 token、`rarity` normal/misc/rare/epic、`period`（any,day,dusk,night 逗号可组合）、`depth` any/shallow/mid/deep、`flavor`、`sort`、`status` 启停 |
| `fishgame_dex` | 点亮进度：`player_id`+`fish_id` 唯一、`count`（双路径记账的汇合点） |
| `fishgame_track_event` | 埋点：`name`、`t`（秒）、`props` JSON、`dedup_key` 唯一（canonical JSON） |
| `fishgame_sign` | 签到：玩家日粒度幂等 |
| `fishgame_friend` | 库内好友（一期；微信关系链留位 `FriendSource` 抽象） |
| `fishgame_config` | 运营配置：type `luck`（period+depth_bias）/ `fish_king`（fish_id+week） |
| `fishgame_share` | 分享图：storage.Driver 落盘 + `public_base_url` 签名 key |

### 12.6 关键机制

- **双路径图鉴记账**：`POST /dex/report` 的报告走 `MergeReport` 幂等合并；`/track` 的 `catch`
  事件异步回放校正——两路都汇入 `fishgame_dex.count`，对账以事件为准。
- **确定性鱼王**：`SeedFromSHA256(week)` 驱动，admin 下发 `fish_king` 配置当周生效；
  确定性保证同周全服同王、可复算。
- **canonical dedup**：`dedup_key` 由 props 规范 JSON 序列化生成，字段序无关，客户端重试安全。
- **好友榜双口径**：`rankEntries` 共享核心（点亮数 desc → nick asc）；空好友集降级全体榜。
- **Stats 聚合**：`ListNameTSince` pluck 后 Go 内分桶（7 日窗口有界），mysql/sqlite 双兼容。

### 12.7 错误码（30401–30404）

`pkg/errors/code.go` 的 fishgame 段（HTTP 400）：

| 码 | 含义 | 码 | 含义 |
| --- | --- | --- | --- |
| 30401 | 图鉴条目不存在（查无/跨租户；重复 fish_id 复用此码） | 30403 | 鱼王指定的条目不存在或已停用 |
| 30402 | 条目已被进度或事件引用，不可删除 | 30404 | 运营配置不存在 |

### 12.8 前端页面（apps/web-antd，5 页）

| 路由 | 组件 | 功能 |
| --- | --- | --- |
| `/fishgame/fishes` | `views/fishgame/FishgameFishesPage.vue` | 图鉴条目分页 + rarity/状态筛选 + 新建/编辑/删除 + Switch 启停 |
| `/fishgame/configs` | `FishgameConfigsPage.vue` | 配置列表 + type 切换 luck/fish_king（鱼王 fish_id 下拉取自启用 fishes） |
| `/fishgame/events` | `FishgameEventsPage.vue` | 埋点分页 + name/player_id/时间范围筛选 + props JSON 折叠 |
| `/fishgame/players` | `FishgamePlayersPage.vue` | 玩家模糊搜索 + 详情抽屉（点亮明细 + 最近 20 事件 + 榜单位次） |
| `/fishgame/stats` | `FishgameStatsPage.vue` | 总数卡 + 四结果漏斗 + 事件计数 + 按日趋势 |

API 封装 `src/api/fishgame.ts`（requestClient → `/api/admin/fishgame/**`）；
页内不使用 `v-access`——可见性靠菜单、操作靠后端 casbin。

### 12.9 联调要点

1. 后端起服务，微信小游戏后台配置 request 合法域名。
2. 客户端 `online_service.gd` 顶部 `base_url` 填后端地址。
3. 微信开发者工具 Network 面板观察 `/track` 批量上报与 dedup 幂等。
4. admin 下发指定 fish_id 的 `fish_king` 配置，验证游戏内当周鱼王切换。

---

## 附：已知缺口与遗留（写文档时核实）

- `orders` 表无 tenant_id，多租户隔离不生效（§7.4）。
- micro 形态 `system_user_grpc.go` 6 个方法全部 "not implemented" 占位（§7.5）。
- `architecture.md` §5 原两处漂移（app 端 logout/refresh 是否走鉴权、casbin sub 格式）已随本文提交更正。
- 前端：角色"分配菜单"UI 缺失；多数 CRUD 页无"编辑"按钮；顶栏 Profile 死链；`views/demos` 未删；通知中心 mock。
- member 有按钮权限无页面菜单（§5.2）；codegen/oauth2 列表已改服务端真分页（M4 跟进① 已修，size 上限 100）。
