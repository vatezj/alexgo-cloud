# 第三方登录（微信，一期）设计

> 日期：2026-10-21
> 状态：已与用户逐节确认（方案 A + 四决策 + 六节设计）
> 范围：租户级渠道配置（`social_app`）、微信小程序 code2session 登录、微信网页授权登录、第三方身份绑定（`member_identity`）、绑定/解绑/手机号合并、admin 渠道配置页与菜单种子、adapter 可扩展接口（抖音/支付宝留位）
> 不在范围：支付、模板/订阅消息、短信验证码、抖音/支付宝具体实现、unionid 之外的跨平台同人、用户侧"已绑定渠道"管理页、解绑审计日志

## 背景与目标

现状盘点（2026-10-21）：登录语义已统一——`modules/system/oauth2` 的 `UnifiedLogin` 按 `account_type`（system/member/client）路由凭据校验，经 `token.Issuer` 签发；全仓无任何第三方（微信/抖音/支付宝）登录代码与表。member 注册口径是「手机号 + 密码（≥8）、无短信验证码」。

目标：C 端用户可用微信（小程序 / 网页授权）登录本平台；每个**租户**配置自己的 appid（多租户 SaaS 形态）；身份联邦接口按可扩展设计，后续平台以新增 adapter 方式接入。

## 决策记录（用户逐项确认）

| 决策点 | 选择 |
|---|---|
| 一期需求边界 | **仅第三方登录**；支付、模板消息等不在本期（小程序/公众号按"登录渠道"建模，不预支渠道平台全家桶） |
| appid 配置归属 | **租户级**：每租户每平台一份配置，表带 `tenant_id` + admin 管理页 |
| 首次登录账号来源 | **混合**：未绑定自动注册空壳 member + 后续引导绑定手机号 |
| 一期平台清单 | **微信全家**（小程序 code2session + 网页授权），adapter 接口可扩展，抖音/支付宝留位 |
| 同人合并 | **unionid 优先 + 手机号兜底合并**（合并规则见 §4） |
| 代码落位 | **方案 A**：`modules/system/oauth2` 下加 `social` 子包（贴 `UnifiedLogin` 生长；micro 形态 member-server 已装 oauth2 + member，C 端登录进程直接可用） |

## 1. 架构与组件

```
C 端（小程序 / H5，本仓库外）
  │ POST /api/app/social/wechat/mp/login     {code}   公开
  │ POST /api/app/social/wechat/web/login    {code}   公开
  │ POST /api/app/member/auth/bind-social    {platform, code}   Bearer
  │ POST /api/app/member/auth/bind-mobile    {mobile}           Bearer
  │ DELETE /api/app/member/auth/social/:platform                Bearer
  ▼
modules/system/oauth2/social/（新子包）
  controller/app.go          app 端五端点
  adapter.go                 PlatformAdapter 接口 + registry
  wechat_mp.go               小程序 code2session 实现
  wechat_web.go              网页授权 code → access_token 实现
  service.go                 SocialLoginService：登录/绑定/解绑/合并
  ▼
modules/system/oauth2/social/repository.go（social_app / member_identity 读写）
modules/member（既有）：member_user 自动注册 / bind-mobile 写入
pkg/token（既有）：token.Issuer.Issue 签发 member token
```

admin 端（渠道配置 CRUD）：

```
GET/POST/PUT/DELETE /api/admin/social/apps     casbin 前缀 social:app
  ▼
modules/system/oauth2/social/controller/admin.go + service（同一 SocialService 或拆 AppConfigService）
```

文件面：

| 层 | 文件 |
|---|---|
| model | `modules/system/oauth2/social/model.go`（`SocialApp`、`MemberIdentity`） |
| 迁移 | `modules/system/oauth2/migrations/` 新增 up（两表）；down 对称 |
| repo | `modules/system/oauth2/social/repository.go` |
| adapter | `adapter.go` / `wechat_mp.go` / `wechat_web.go`（+ 假 adapter 测试夹具） |
| service | `service.go`（登录三分支、绑定、合并、解绑守门）+ `service_test.go` |
| controller | `controller/app.go`、`controller/admin.go`（+ 测试） |
| 装配 | `modules/system/oauth2/module.go` 注册 repo/service/controller/路由/adapter registry |
| 权限 | `modules/system/service/permission_routes.go` 增 `"social:app"` 键；`permission_routes_test.go` 自动兜底 |
| 错误码 | `pkg/errors/code.go` 增 303xx 段 |
| 菜单 | `modules/system/migrations/` 新增 `*_social_app_menu.up.sql`（目录 + 页面 + 按钮，仿 oauth2_client_menu 先例） |
| 前端 | `apps/web-antd/src/views/system/SocialAppsPage.vue`（新）+ `src/api/admin.ts` 新段 |
| 文档 | 本 spec；完成后回填 `docs/modules.md` oauth2 章节 |

**签发口径**：登录成功（含合并/补绑后）一律 `token.Issuer.Issue` 产出 member token，`client_id` 复用 `alexgo-app`（与 member 登录同款）；审计照 `UnifiedLogin` 惯例恰一次写 `login_logs`（message 注明 platform 与 openid 来源，如 `微信小程序登录`）。

**登录语义不进 `UnifiedLogin.Login`**：该方法入参是账号密码凭据校验器路由；social 流程自己完成身份解析后直调 `token.Issuer.Issue`（与代登录同构）。`UnifiedLogin` 零改动——这是方案 A 落位的直接红利。

## 2. 数据模型

### `social_app`（租户级渠道配置，走租户插件隔离）

| 字段 | 说明 |
|---|---|
| id / tenant_id / status / create_time / update_time | 常规；status 1 启用 0 停用 |
| platform | `wechat_mp` / `wechat_web`；字符串枚举，可扩展（douyin / alipay 后续加值） |
| appid | 平台应用 ID |
| secret | **明文存储**——调用平台 API 必须用明文（与 oauth2_client 的 bcrypt 不同源）；admin 端**不回显**，更新留空保留旧值 |

唯一键 `(tenant_id, platform)`：每租户每平台一份。

### `member_identity`（第三方身份绑定，走租户插件隔离）

| 字段 | 说明 |
|---|---|
| id / tenant_id / create_time | 常规 |
| member_user_id | 归属 `member_user.id` |
| platform | 同上枚举 |
| openid | 平台内唯一身份 |
| unionid | 可空；微信开放平台绑定后才有 |

唯一键 `(tenant_id, platform, openid)`；索引 `(tenant_id, unionid)` 供跨端同人查找（unionid 为空串的行不参与同人匹配——查询显式 `unionid != ''`）。

`member_user` 表**零改动**：Mobile/Password 空串即"未绑定"，空壳判定直接读既有列。

## 3. 登录流（小程序 / 网页授权共用骨架）

```
code → adapter.Exchange(code) → {openid, unionid, nickname, avatar}
  1) 查 member_identity (tid, platform, openid) 命中
       → member 存在且未删、status=1 → Issue 签发 → login_logs 审计 → 返回 token 对
       → member 已删/停用 → 拒绝（400，不静默注册新号）
  2) 未命中且 unionid != ""
       → 按 (tid, unionid) 查任一 member_identity
       → 命中 → 补绑本平台 identity 行（member_user_id 沿用）→ 签发 → 审计
  3) 都没有 → 自动注册 member_user 空壳（Nickname 平台给什么用什么，缺省空串；
       status=1 启用；不向平台额外索取用户信息——微信已匿名化，一期不接 getUserProfile）
       → 写 identity → 签发 → 审计
```

租户解析：登录请求经现有 Tenant 中间件（头 / 域名），拿 tid 查 `social_app`（platform 精确、status=1）——**未配置 / 停用直接 30301 报错**。租户未配置时 adapter 不执行、不消耗平台配额；天然不会串租户 appid。

`Exchange` 失败（code 过期 / 平台接口错误）统一映射 30302，**不区分**平台细分错误（一期不透传平台 errcode，防信息泄露；平台原始错误只记日志）。

签发链路故障（Issuer 报错）→ 503（与 member 登录同口径）。

### Adapter 接口（扩展点）

```go
type PlatformAdapter interface {
    Platform() string
    // Exchange 用 code 换平台身份；app/secret 由 service 查 social_app 后传入。
    Exchange(ctx context.Context, app *SocialApp, code string) (*Identity, error)
}

type Identity struct {
    OpenID  string
    UnionID string // 可空
    Nickname string // 可空（微信已匿名化，基本拿不到）
    Avatar  string // 可空
}
```

registry：`Register(adapter)` 按 platform 字符串索引；service 只认接口。抖音/支付宝 = 各加一个文件注册，service/端点零改动（路由层 `:platform` 参数化或按一期固定两条路由 + 二期扩展，见 §6）。

一期两条路由**不参数化**（`/social/wechat/mp/login`、`/social/wechat/web/login` 硬编码）：小程序与网页授权的 body 契约将来可能分化（如网页授权可能要 redirect 配合），过早参数化是伪扩展。真正要扩展的是**新平台**，新平台加新路由即可。

## 4. 绑定 / 解绑 / 手机号合并

### 绑定第三方（`POST /api/app/member/auth/bind-social`）

Bearer 登录态（member token）→ body `{platform, code}` → adapter.Exchange → identity 已绑**其他** member → 30303 报错（不迁移）；未绑任何账号 → 绑到当前登录 member；已绑自己 → 幂等成功。

### 绑定手机号（`POST /api/app/member/auth/bind-mobile`）

Bearer 登录态 → body `{mobile}`。**与现有注册同口径，不引入短信验证码**（SMS 基建不存在，本期不铺）。

- 当前账号已有手机号 → 30304（已绑定，不允许改绑；改绑留二期）。
- mobile 格式校验沿用 member 正则 `^1[3-9]\d{9}$`。
- mobile 未被占用 → 写入当前账号 `member_user.mobile`。
- mobile 已被另一 member 占用 → **合并**：
  - 仅当当前账号是**空壳**（`mobile == ""` 且 `password == ""`，即 social 自动注册产物）才自动合并；
  - 合并动作：本账号全部 `member_identity` 行 `member_user_id` 改指目标 member → 当前空壳 `member_user` 软删（`deleted=1`）→ 完成。**业务数据不迁移**——空壳刚注册无业务数据，此前提使合并不触碰 scorecard 等模块；将来若有逃逸，收紧空壳判定即可。
  - 当前账号非空壳（有手机号或密码，即用户自己注册过的）→ 30305 拒绝，提示人工处理。

### 解绑（`DELETE /api/app/member/auth/social/:platform`）

Bearer 登录态 → 删 `(tid, member_user_id, platform)` identity 行。守门：**解绑后必须仍剩至少一种登录方式**——当前账号有密码，或还绑着其他平台身份——否则 30306 拒绝（防把自己锁死）。

## 5. 端点与权限

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/api/app/social/wechat/mp/login` | 公开 | `{code}` → `{token, refresh_token, expires_in}` |
| POST | `/api/app/social/wechat/web/login` | 公开 | 同上 |
| POST | `/api/app/member/auth/bind-social` | Bearer | `{platform, code}` |
| POST | `/api/app/member/auth/bind-mobile` | Bearer | `{mobile}` |
| DELETE | `/api/app/member/auth/social/:platform` | Bearer | 解绑 |
| GET | `/api/admin/social/apps` | casbin `social:app` | 分页列表（secret 不回显） |
| POST | `/api/admin/social/apps` | casbin `social:app` | 新建 |
| PUT | `/api/admin/social/apps/:id` | casbin `social:app` | 更新；secret 留空保留旧值 |
| DELETE | `/api/admin/social/apps/:id` | casbin `social:app` | 删除 |

app 端五条挂 `/api/app` 公开组（bind-* 三条在 handler 内 Bearer 自鉴权，与 scorecard/member app 端同构）；admin 四条走 `/api/admin` 组 + casbin。

`permission_routes.go` 增：`"social:app": {"/api/admin/social/apps", "/api/admin/social/apps/*"}`。菜单种子：目录「渠道配置」`/social`（perm `social`）→ 页面 `/social/apps`（perm `social:app`，component `views/system/SocialAppsPage`）→ 按钮 `social:app:create/:update/:delete` → 绑 admin（迁移幂等、`tenant_id=0`，仿 `20261011000003_oauth2_client_menu`）。

### 错误码（`pkg/errors/code.go`，3xxxx 段 → HTTP 400）

| 码 | 语义 |
|---|---|
| 30301 | 该租户未配置此登录渠道（或已停用） |
| 30302 | 第三方授权换取失败（code 无效/过期/平台接口错误） |
| 30303 | 该第三方身份已绑定其他账号 |
| 30304 | 当前账号已绑定手机号 |
| 30305 | 手机号已被占用，当前账号不满足自动合并条件 |
| 30306 | 解绑后将失去唯一登录方式 |

## 6. 前端（admin 渠道配置页）

| 路由 | 组件 | 功能 |
|---|---|---|
| `/social/apps` | `views/system/SocialAppsPage.vue` | 渠道配置列表（平台下拉筛选）+ 新建/编辑/删除；表单 platform 下拉（wechat_mp / wechat_web）+ appid + secret（编辑时留空=保留）+ status |

API 封装：`src/api/admin.ts` 新增 socialApps 段（list/create/update/delete）。页内按钮 `v-access:code` 三枚（`social:app:create/:update/:delete`）；可见性靠菜单。样式与既有 System*Page 一致（VxeGrid + useVbenModal/useVbenForm 适配器）。

## 7. 测试

- **adapter**：wechat_mp / wechat_web 各自单测——用 `httptest.Server` 模拟平台端点（code2session / oauth2 access_token），断言请求参数与 Identity 解析；不打真平台。
- **service**：登录三分支（identity 命中 / unionid 并入补绑 / 自动注册空壳）；member 已删停用拒绝；unionid 空串不参与同人匹配；合并（空壳成功、非空壳拒绝、identity 改指、软删）；解绑守门（唯一登录方式拒绝、有密码放行）；bind-social 冲突。
- **controller**：app 五端点 + admin 四端点（admin 含 secret 不回显 / 留空保留断言）。
- **权限**：`permission_routes_test.go` 惯例自动兜底新前缀；菜单种子权限码前缀注册校验。
- **回归**：既有 oauth2/member 测试零改动通过（UnifiedLogin 不动）。

## 8. 明确不做（一期）

短信验证码（绑定手机号与注册同口径弱验证）；抖音/支付宝实现（接口与表字段已留位）；unionid 之外的跨平台同人；用户侧"我的绑定渠道"查询页；解绑审计日志（OperateLog 中间件只覆盖 admin 端，app 端 bind/unbind 动作审计留二期）；secret 加密存储（明文落库是平台 API 调用的硬约束，库访问控制兜底）；`/api/oauth2` 协议端点接入第三方（social 登录是 app 私有端点，不是 RFC6749 联邦）。

## 9. 风险与备注

- **空壳判定逃逸**：合并安全性完全依赖"空壳 = 无 mobile 无 password"。若将来空壳账号通过其他途径获得业务数据，合并会留下孤儿数据。防线：合并仅在 bind-mobile 触发且要求当前账号空壳；收紧点已隔离在一处判定函数。
- **微信 unionid 前提**：小程序与公众号/开放平台主体未绑同一微信开放平台时拿不到 unionid——跨端同人自动退化为"各平台 openid 独立身份 + 手机号兜底合并"，属预期行为而非缺陷。
- **租户切换登录**：第三方登录的租户由请求侧（域名/头）决定，同一个人在不同租户下是不同 member（identity 表按 tenant 隔离）——与手机号注册的多租户口径一致。
- **micro 形态**：social 子包随 oauth2 装配进 member-server，C 端可用；admin 端点只在 system-server。表由 oauth2 迁移源统一管理（与既有 oauth2 表同轨）。
