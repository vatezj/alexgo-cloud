# OAuth2 统一登录使用文档

> 本文依据当前代码整理。与实现不一致时，以代码为准。
> 设计规格：[unified-login-design](superpowers/specs/2026-10-11-unified-login-design.md)；兼容层评估：[oauth2-compat-retirement.md](oauth2-compat-retirement.md)；操作入口另见 [usage.md](usage.md)。

## 0. 概览

`modules/system/oauth2` 是全站唯一的签发点：三端登录（后台 / app / member）内部都走统一登录，token 为**不透明令牌**（`auth.mode=opaque`，DB 落库；`jwt` 模式下不支持 refresh）。

**信封约定**（与 admin 接口不同）：

| 场景 | 信封 | 状态码 |
| --- | --- | --- |
| 协议端点 `/api/oauth2/*`（authorize/token 等） | RFC 6749：`{"error","error_description"}` | 400 / 401 / 500；**失效/篡改绝不 302** |
| 管理端点 `/api/admin/oauth2/clients` | admin 惯例：`{data,total}` / `{status:"ok"}` / `{error}` | 400 / 500 |

**端点一览**：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/oauth2/captcha` | 验证码挑战（`captcha_uuid` + 图片） |
| GET | `/api/oauth2/authorize` | 授权码入口（浏览器重定向）→ 302 登录页 |
| POST | `/api/oauth2/authorize/login` | 登录表单提交（**JSON body**）→ 302 `redirect_uri?code&state` |
| POST | `/api/oauth2/token` | 换 token（**JSON body**，四种 grant） |
| GET | `/api/oauth2/userinfo` | 按 Bearer token 取当前账号信息 |
| GET/POST/PUT/DELETE | `/api/admin/oauth2/clients` | 客户端管理 CRUD（权限码 `oauth2:client`） |

## 1. SSO 登录链路（授权码 + PKCE S256）

后台前端（`apps/web-antd`）登录走完整授权码流，**不经过** `/api/auth/login`（那是兼容层，见 §7）：

```
登录表单 (store/auth.ts)
  │ 1. 生成 verifier/state → SHA256 得 code_challenge（S256）
  │ 2. POST /api/oauth2/authorize/login  {client_id, redirect_uri, username, password,
  │                                       code_challenge, code_challenge_method, state, ...}
  │    └─ 成功：302 Location = redirect_uri?code=xxx&state=xxx（fetch redirect:'follow' 跟随）
  │    └─ 失败：400 {error, error_description}（形状/白名单校验先于凭据，绝不 302）
  ▼
路由 /callback (views/_core/authentication/callback.vue)
  │ 3. 校验 state 与 sessionStorage 中一致
  │ 4. POST /api/oauth2/token  {grant_type:"authorization_code", client_id, code,
  │                             redirect_uri, code_verifier}
  ▼
{access_token, refresh_token, expires_in, token_type:"Bearer"} → 后续请求
     Authorization: Bearer <access_token>
```

curl 复现（本地种子 client，默认账号 `admin / admin123`）：

```bash
# ① 登录换授权码（JSON body！表单编码会 400 "请求体必须是 JSON 对象"）
curl -i -X POST http://localhost:8080/api/oauth2/authorize/login \
  -H 'Content-Type: application/json' \
  -d '{"client_id":"alexgo-admin","redirect_uri":"http://localhost:5666/callback",
       "response_type":"code","username":"admin","password":"admin123",
       "account_type":"system","state":"abc",
       "code_challenge":"<S256(verifier)>","code_challenge_method":"S256"}'
# → 302，从 Location 取 code

# ② 授权码换 token（同样是 JSON body）
curl -X POST http://localhost:8080/api/oauth2/token \
  -H 'Content-Type: application/json' \
  -d '{"grant_type":"authorization_code","client_id":"alexgo-admin",
       "code":"<上一步的 code>","redirect_uri":"http://localhost:5666/callback",
       "code_verifier":"<verifier>"}'
# → {"access_token":"...","refresh_token":"...","expires_in":...,"token_type":"Bearer"}
```

**PKCE 约定**：`code_challenge_method` 只接受 `S256`（`plain` 拒绝）；带 challenge 时换 token 必须带匹配的 `code_verifier`。

## 2. redirect_uri 白名单

- **精确匹配**，字段存换行分隔的多行文本（`Oauth2Client.AllowsRedirect`）。
- 校验顺序：**形状/白名单全部先于凭据**——不匹配返回 `400 invalid_request "redirect_uri 不在白名单"` 且**无 Location**（凭据错了也一样先拒白名单）。
- 种子 client `alexgo-admin` 的白名单（`20261011000002_oauth2_seed_clients` + `20261012000001_oauth2_admin_redirect_dev_origin`）：

  ```
  http://localhost:5173/callback
  http://localhost:8080/callback
  http://localhost:5666/callback   ← web-antd dev 端口
  ```

  前端固定用 `${window.location.origin}/callback`，因此**换端口跑前端时白名单必须同步加一行**，否则登录 400。
- 新增方式：管理后台 **OAuth2 → 客户端管理** 编辑 `redirect_uris`（多行文本），或直接 UPDATE 数据表后重启/等缓存刷新。

## 3. 种子客户端

迁移 `20261011000002_oauth2_seed_clients.up.sql`（幂等 NOT EXISTS）：

| client_id | 类型 | grant_types | account_types | redirect_uris | 用途 |
| --- | --- | --- | --- | --- | --- |
| `alexgo-admin` | 公开（secret 空，靠 PKCE） | authorization_code, password, refresh_token | system | 本地三个 callback | 后台 SSO / 管理员 password 登录 |
| `alexgo-app` | 机密（bcrypt secret） | password, refresh_token | member | （空，无浏览器回调） | app 端 / member 登录 |

`account_types` 是白名单：admin client 换不出 member token（`unauthorized_client "client 不允许该账号类型"`）。

## 4. token 端点（POST /api/oauth2/token）

**必须 JSON body**（form-encoded → 400 `invalid_request "请求体必须是 JSON 对象"`）。

| grant_type | 必填字段 | 说明 |
| --- | --- | --- |
| `authorization_code` | `client_id`, `code`, `redirect_uri`, `code_verifier` | 授权码 + PKCE 换 token |
| `password` | `client_id`, `username`, `password`（`account_type` 缺省 system） | 直接换 token；验证码字段 `captcha_uuid`/`captcha_code` 仅在 `auth.captcha.enabled=true` 时必填（**默认 false**） |
| `refresh_token` | `client_id`, `refresh_token` | 空串直接拒（`invalid_grant "刷新令牌无效"`）；**jwt 模式不支持**；不做 client 绑定（refresh token 即凭证，D14） |
| `client_credentials` | `client_id`, `client_secret` | 机机凭证；**公开客户端（secret 空）401 拒绝** |

成功响应（RFC 6749 §5.1）：

```json
{"access_token":"...","refresh_token":"...","expires_in":7200,"token_type":"Bearer"}
```

失败即 `TokenService` 的 `*ProtocolError` 原样写出（400/401 + `error`/`error_description`）。

## 5. 授权与用户信息

- **GET /api/oauth2/authorize**（浏览器入口）：缺参/未知 client/无 grant/白名单不过/`response_type != code`/PKCE 非 S256 → 400 且**无 Location**；全部通过 → 302 `cfg.Auth.LoginPage?client_id&redirect_uri&state&code_challenge&...`。
- **GET /api/oauth2/userinfo**：`Authorization: Bearer <access_token>` 取当前账号信息（第三方接入常用）。
- **GET /api/oauth2/captcha**：返回挑战（uuid + 图片），配合 password/authorize 登录提交。

## 6. 客户端管理（管理后台）

入口：**OAuth2 → 客户端管理**（`apps/web-antd/src/views/system/Oauth2ClientsPage.vue`，菜单由 `20261011000003_oauth2_client_menu` 种入）。

- 权限码 `oauth2:client` → 前缀 `/api/admin/oauth2/clients`（`permission_routes.go` 注册，通配覆盖子资源）。
- 字段要点：`grant_types` 逗号分隔白名单、`redirect_uris` 多行精确白名单、`account_types` 账号类型白名单、`activity` 停用即全端点 400 `invalid_client`。

## 7. 兼容层（老登录端点）

`POST /api/auth/login`、`/api/app/system/auth/login`、`/api/app/member/auth/login` 已内部切到统一登录，响应契约不变；仓库内前端零调用，**服务端保留**（下线条件清单见 [oauth2-compat-retirement.md](oauth2-compat-retirement.md)）。新接入一律走本文 §1/§4 的协议端点。

## 8. 常见错误排查

| 现象 | 原因 | 处理 |
| --- | --- | --- |
| 400 `redirect_uri 不在白名单` | 前端 origin 不在 client 的 redirect_uris | 管理页加一行（如 `http://localhost:5666/callback`），见 §2 |
| 400 `请求体必须是 JSON 对象` | authorize/login 或 token 用了 form 编码 | 改 `Content-Type: application/json` |
| 400 `unauthorized_client: client 未获 xxx 许可` | client 的 grant_types 白名单不含该 grant | 管理页改 grant_types |
| 400 `unauthorized_client: client 不允许该账号类型` | account_types 白名单不匹配（如 admin client 请求 member） | 用对应 client，或调整白名单 |
| 400 `invalid_grant: 授权码无效` | code 已用/过期，或 code_verifier 不匹配，或 redirect_uri 与签发时不一致 | 走全新登录流程 |
| 400 `invalid_grant: 刷新令牌无效` | refresh_token 空串/已撤销/过期；或 `auth.mode=jwt` | 空串前端需拦截；jwt 模式不支持 refresh |
| 401 `invalid_client` | client 未知、`activity=0` 停用、或公开 client 走 client_credentials | 检查 client 状态与 secret |
