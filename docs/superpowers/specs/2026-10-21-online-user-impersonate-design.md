# 在线用户 + 代登录（P1-3）设计

> 日期：2026-10-21
> 状态：已与用户逐节确认（方案 A + 五决策 + 两节设计）
> 范围：管理员在线会话列表、踢人（Revoke）、代登录到会员（签发 + 审计）、前端在线用户页与菜单种子
> 不在范围：会员在线列表、IP/UA 展示、浏览器侧身份切换 UI、`/api/oauth2/introspect`/`revoke` 协议端点、跨 gRPC 踢人

## 背景与目标

现状盘点（2026-10-21）：踢人原语已齐——`pkg/token` 有 `Issue/Refresh/Revoke/RevokeAll`，`Validate` 恒查库无进程内缓存（`pkg/token/service.go` 注释明言为「踢人立即失效」）；但 HTTP 层零端点（全仓 grep `impersonat|代登录` 零命中），无在线列表页。`system_oauth2_access_token` 表有 `user_id/user_type/client_id/access_token/expires_time` 等列，**无 username/IP/UA 列**（IP/UA 只在 `login_logs` 审计表）。

目标（roadmap P1-3，对照 yudao 在线用户页）：管理员可查看自己租户内在线管理员会话、踢出异常会话、并以会员身份代登录（排查会员端问题）。

## 决策记录（用户逐项确认）

| 决策点 | 选择 |
|---|---|
| 在线列表范围 | **仅管理员**（`user_type=1` 未过期 token）；会员在线留后续按需 |
| 代登录目标 | **会员**（产出 member token）；不做管理员互登 |
| token 标记 | **专用 `client_id=impersonate-admin`**，token 行可辨识、可审计 |
| IP/UA 展示 | **不显示**（token 表不存；那本是 login_logs 审计的职责，不 join 近似展示、不扩表回写） |
| 审计 | OperateLog 中间件（自动）+ 代登录时向 `login_logs` 追加一条（`message` 备注操作者与目标） |
| 代码落位 | **方案 A：全部落 system 模块**（token 签发/吊销本就在 system 本地；`RevokeAll` 明确不可跨 gRPC） |
| 代登录消费 | **签发 + 审计即止**：前端不落/不回显 token、不做浏览器身份切换 UI（需要时另开小票） |

## 1. 架构与组件

```
前端 SystemOnlineUsersPage（vben antd）
  │ GET  /api/admin/system/online                  分页列表
  │ DELETE /api/admin/system/online/:id             踢人（按行 id）
  │ POST /api/admin/system/online/impersonate       代登录 {user_id}
  ▼
modules/system/controller/admin/online_controller.go（新）
  ▼
modules/system/service/online_service.go（新）
  · List：repo 分页（user_type=1 AND expires_time>now），join system_users 取 username
  · Kick：按 id 查行 → 校验 user_type=1 且非本人 → token.Issuer.Revoke
  · Impersonate：校验目标 member 存在/未删/租户有效 → token.Issuer.Issue
    （ClientID="impersonate-admin"）→ 写 login_logs → 返回 token 对
  ▼
modules/system/repository/online_repo.go（新）＋ pkg/token（既有 Issuer）
```

文件面：

| 层 | 文件 |
|---|---|
| repo | `modules/system/repository/online_repo.go`（+测试夹具内联） |
| service | `modules/system/service/online_service.go` + `online_service_test.go` |
| controller | `modules/system/controller/admin/online_controller.go` + `online_controller_test.go` |
| 路由 | `modules/system/module.go` admin 组注册三端点 |
| 权限 | `modules/system/service/permission_routes.go` 增 `"system:online"` 键；`permission_routes_test.go` 自动兜底 |
| 错误码 | `pkg/errors/code.go` 增 30201/30202/30203 |
| 菜单 | `modules/system/migrations/20261021000001_online_user_menus.up.sql`（新） |
| 前端 | `apps/web-antd/src/views/system/SystemOnlineUsersPage.vue`（新）+ `api/admin.ts` 新段 |

**端点与权限码**（权限三链路：菜单 permission + permissionRoutes 登记 + 前端 `v-access`）：

| 方法 | 路径 | 权限码 |
|---|---|---|
| GET | `/api/admin/system/online` | `system:online:list` |
| DELETE | `/api/admin/system/online/:id` | `system:online:kick` |
| POST | `/api/admin/system/online/impersonate` | `system:online:impersonate` |

`permission_routes.go` 增：`"system:online": {"/api/admin/system/online", "/api/admin/system/online/*"}`。

**明确不做**：会员在线列表、IP/UA 列与展示、token 表扩列（last_seen 回写会放大 Validate 热点路径）、浏览器侧身份切换 UI、oauth2 协议级 introspect/revoke 端点。

## 2. 数据流

### List
1. 前端 → `GET /api/admin/system/online?page=&size=`（`resp.PageReq`，size≤100）
2. repo：`user_type=1 AND expires_time > now AND deleted=0`，join `system_users`（`deleted=0`）取 username，`repo.Paginate` 服务端真分页；tenant 插件照常隔离
3. 出参行：`{id, user_id, username, client_id, is_impersonated, issued_at, expires_at}`——**access_token 明文不回传**（只标识行 id；防 XSS/日志泄露）
4. `is_impersonated`：user_type=1 的行 client_id 不会是 `impersonate-admin`（代登录产出 member token），此字段恒 false 也不剥——保留列供将来范围扩展；本期前端可不渲染

### Kick
1. 前端 Popconfirm → `DELETE /api/admin/system/online/:id`
2. repo 按 id 查行（含过期行也在列，踢过期行等价清理）→ 不存在 → 30203
3. `user_type != 1` → 30203（不可踢会员——范围外）
4. 行 `user_id == 当前操作者 user_id` → 30202（禁止踢自己）
5. `token.Issuer.Revoke(ctx, 行.access_token)`：物理删行；下一次 `Validate` 即失败（无缓存保证立即失效）
6. `resp.OK(ctx, nil)`，前端 toast + 刷新列表

### Impersonate
1. 前端 modal 选会员 → `POST /api/admin/system/online/impersonate {user_id}`
2. 查 `member_user`（`deleted=0` 且 `status=1` 启用）→ 不存在/已停用 → 30201（文案「目标会员不存在或不可用」；给停用会员签 token 等于绕过登录状态门）
3. 目标租户停用检查与正常登录同款（`Issue` 内部已做，失败按其错误透传）
4. `token.Issuer.Issue(ctx, IssueParams{UserID: 目标, UserType: UserTypeMember, TenantID: 目标租户, ClientID: "impersonate-admin"})`——`IssueParams.ClientID` 字段既有，零 pkg 改动
5. 写 `login_logs`：`username=会员名, user_id=会员id, ip/user_agent=当前管理员请求, success=1, message="代登录: 管理员{admin_id}/{admin_name} → 会员{member_id}/{member_name}"`
6. 出参 `{access_token, refresh_token, expires_in}`（与 member login 响应同形）；**前端不落盘不回显**，仅 toast「代登录成功，会话已生效（可在登录日志审计）」并刷新列表

## 3. 错误处理

| 场景 | 码 | 处理 |
|---|---|---|
| 目标会员不存在/已删除 | 30201 | `errs.New` → `resp.Fail` → 400 |
| 不能踢自己 | 30202 | 同上 |
| 会话不存在/非管理员 | 30203 | 同上（覆盖踢人竞态：行刚被 refresh 轮换或 sweep 清理删除；过期行仍可踢，等价清理，不算错） |
| 参数错（缺 user_id 等） | `errs.CodeParamInvalid` | bind 校验既有路径 |
| 租户停用 / Issue 内部错误 | 透传 | 与正常登录一致 |

## 4. 前端

- `SystemOnlineUsersPage.vue`：`useVbenVxeGrid` 服务端真分页（照 `api/scorecard.ts` 的 `{list,total}` 范式），列：用户名 / user_id / client_id（`impersonate-admin` 渲染「代登录」Tag；管理员行正常 client_id 如 `vben-web` 等原样展示）/ 签发时间 / 过期时间 / 操作列
- 操作列：踢人（Popconfirm + `v-access:code="['system:online:kick']"`）、代登录（Modal + `v-access:code="['system:online:impersonate']"`）
- 代登录 Modal：会员选择器——复用 `GET /api/admin/member/users` 列表接口（`member:user` 权限既有；该接口仅 page/size 无筛选），前端按昵称/手机本地过滤后单选；确认 → 调 impersonate → toast（不展示 token）
- `api/admin.ts` 新段：`listOnlineApi` / `kickOnlineApi` / `impersonateApi`（interface + requestClient，照 notice 段风格）
- 菜单种子 migration SQL：菜单「在线用户」挂系统管理下（`Path: online-users`，`Component: views/system/SystemOnlineUsersPage`，`Permission: system:online:list`）+ 踢人/代登录两按钮；照 `20261020000001_scorecard_menus.up.sql` 写法绑超管角色

## 5. 测试

无 testcontainers，三类照既有惯例：

1. **service**（`online_service_test.go`，glebarez/sqlite 内存库 + AutoMigrate + 裸 DSL 夹具，照 `pkg/token/service_test.go`）：
   - List 只含 user_type=1 且未过期行；join 出 username；分页 total 正确
   - Kick 删行后 `Validate` 失败（立即失效）；踢自己 → 30202；id 不存在/非管理员行 → 30203
   - Impersonate：签出 token `Validate` 返回 member Claims（user_type=2、目标 user_id/tenant）；token 行 client_id=`impersonate-admin` 可辨；`login_logs` 落一条 message 含双方身份；目标不存在 → 30201
2. **controller**（`online_controller_test.go`，gin 手装路由 + httptest，照 `notice_read_test.go`；路由形状与 module.go 同形作回归）：信封/分页形状/错误码映射
3. **回归**：`permission_routes_test.go` 新键自动兜底；`go test ./...` 全绿；`pnpm typecheck` 绿

## 6. 验收清单

- [ ] 三个端点挂 `/api/admin/system/online/**`，权限三链路齐（403 不出现）
- [ ] 在线列表只含本租户未过期管理员会话，token 明文不出接口
- [ ] 踢人后该 token 立即 401；踢自己被拒；菜单/按钮权限生效
- [ ] 代登录签出的 token 以 member 身份通过 Validate，client_id=`impersonate-admin`，login_logs 有审计行
- [ ] 前端页面可分页浏览、踢人、代登录（token 不回显）
- [ ] `go test ./...` 与 `pnpm typecheck` 绿
