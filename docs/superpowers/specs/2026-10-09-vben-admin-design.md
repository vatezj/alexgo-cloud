# vben-admin 接入设计 — alexGo-cloud 后台管理前端替换

> 日期：2026-10-09 · 状态：设计已逐节获批，待 spec 审阅
> 范围：以后端兼容路由 + 数据回填 + vendored vben monorepo 三部分，将 `admin-web/` 完整替换为 vben-admin（web-antd），登录、动态菜单、按钮权限与全部 11 个业务页面跑通。

## 1. 背景与目标

现有 `admin-web/`（Vue3 + naive-ui + 手写路由/权限）替换为 [vben-admin](https://doc.vben.pro/) V5（web-antd 变体）。vben 对后端有明确契约（5 个端点 + `{code,data}` 信封 + backend 动态菜单模式），本设计定义适配边界与迁移路径。

**成功标准**：vben 登录 → 侧栏按角色展示 工作台/订单/系统管理 三组菜单（带图标）→ 11 个业务页面 CRUD 可用 → 按钮权限码生效 → 登出后旧 token 立即失效；旧接口（`/api/admin/system/auth/profile`、`/api/app/system/auth/login` 等）与既有测试零破坏。

## 2. 已确认决策

| # | 决策点 | 结论 |
|---|---|---|
| 1 | vben 与 admin-web 关系 | **完全替换**（旧前端删除，git 历史保留） |
| 2 | UI 变体 | **web-antd**（Ant Design Vue） |
| 3 | 落库方式 | **monorepo 精简版**（保留 pnpm workspace + packages，仅留 web-antd） |
| 4 | 契约适配位置 | **后端加兼容路由**（薄壳对齐 vben 预期契约） |
| 5 | 迁移范围 | **骨架 + 全部 11 页面** |
| 6 | 整体方案 | **方案一**：后端兼容命名空间 + 数据幂等回填 + 前端最小改动 |

否掉的备选：全局信封统一封（方案二，动几十个 handler 与全部响应测试，与"仅 5 端点敏感信封"不成比例）；纯前端适配层（方案三，`/user/info` 等端点后端不存在，绕不开后端改动，且转换逻辑散落拦截器难排障）。

## 3. 架构总览

```
浏览器  vben web-antd（pnpm dev, :5666, accessMode=backend）
   │  vite proxy：/api → localhost:8080（member 两前缀 → MEMBER_PROXY）
   ▼
alexgo-server :8080
   ├─ [公开]        POST /api/auth/login ─────┐
   ├─ [token+admin]  GET  /api/auth/codes      │ 薄壳层（新包 controller/vben）
   ├─ [token+admin]  GET  /api/user/info       │ 统一回 {code:0,data,error,message}
   ├─ [token+admin]  GET  /api/menu/all        │
   ├─ [token 可选]   POST /api/auth/logout ─────┘
   ├─ [原样]         /api/admin/**（含旧 profile，不改）
   └─ [原样]         /api/app/**（含旧登录，不改）
        │ 壳层内部复用
        ▼
   authSvc.Login（含登录审计）/ Profile 服务 / UserRoutes() / token.Validator
        ▼
   MySQL（迁移回填 dashboard/order 菜单 + icon 更新）
```

**登录时序**（vben 标准流程，本设计只负责 5 个端点都答对）：

1. `POST /api/auth/login` → `{code:0, data:{accessToken}}`
2. 并发 `GET /api/user/info`（homePath/roles）+ `GET /api/auth/codes`（权限码数组）
3. 首次路由导航触发 `GET /api/menu/all` → 生成动态路由与侧边栏 → 跳 `homePath || defaultHomePath`（`/dashboard/analytics`）
4. 登出：`POST /api/auth/logout`（服务端尽力撤销；前端 catch 兜底）

**边界不变量**：

- 旧契约零改动：`/api/admin/system/auth/profile`、`/api/app/system/auth/login` 等继续可用，既有测试不破；
- 信封与字段映射只发生在壳层，业务 service 层返回结构不动；
- 菜单可见性 = role_menus（数据层授权），vben 自读端点不叠 Casbin。

## 4. 后端兼容路由

新包 `modules/system/controller/vben/`（与 `admin/`、`app/` 平行），在 `systemModule.RegisterRoutes` 中注册三组路由：`/auth`、`/user`、`/menu`（挂于 `/api` 组下）。

**信封助手**（包内私有）：

- 成功：HTTP 200 `{"code":0,"data":…,"error":null,"message":"ok"}`
- 壳层失败：HTTP 4xx/5xx `{"code":<status>,"data":null,"error":"<msg>","message":"<msg>"}`
- 中间件门控失败（401/403）：沿用现有 `{"error":"unauthorized"/"forbidden"}`——vben 错误拦截器读 `error ?? message` 弹 toast，触发重登靠**状态码**，格式零改动

**端点定义**：

| 端点 | 门控 | 请求 | data 载荷 | 复用来源 |
|---|---|---|---|---|
| `POST /api/auth/login` | 公开 | `{username, password}` | `{accessToken, refreshToken, expiresIn}`（vben 只读 accessToken；其余与旧登录响应字段对等） | 与 `/api/app/system/auth/login` 同一 service 方法（含登录审计）；失败 → 401 + service 错误文案 |
| `GET /api/auth/codes` | token+admin | — | `string[]`：按钮权限码并集 | Profile 服务已有的 `perms` |
| `GET /api/user/info` | token+admin | — | `{userId, username, realName, avatar, roles[], desc:"", homePath:"", token}` | claims + Profile：`realName`←nickname 兜底 username；`avatar`←users.avatar；`desc`/`homePath` 恒空串（homePath 由前端 `defaultHomePath` 兜底，后端不硬编码前端路由）；`token`←请求所携 access token |
| `GET /api/menu/all` | token+admin | — | `VbenRoute[]`（空菜单回 `[]`，不回 null） | `UserRoutes()`；**`"LAYOUT"` → `""`** 转换在壳层做；button 已跳过、status=1 已过滤；JSON 字段已与 vben 对齐（`path/name/component/meta{title,icon,orderNo,permissions}/children`） |
| `POST /api/auth/logout` | **公开**（不进鉴权清单） | — | 恒 200 | handler 自解析 `Authorization`：Bearer 有效 → 调 logout service 撤销；缺失/无效 → no-op。token 过期触发的 re-auth 登出路径因此不会 401 弹错 |

**为什么 `LAYOUT` 必须转换**：vben `convertRoutes` 先查 `layoutMap`（仅 `BasicLayout`/`IFrameView`），miss 后走 `normalizeViewPath` 拼 `pageMap`——`LAYOUT` 拼出 `/LAYOUT.vue` 必然 miss，落 not-found + console.error。改回空串后两分支都跳过，顶层目录组件由 vben `accessible.ts` 的 `delete route.component` 逻辑清理。

**component 映射验证**（零数据迁移的依据）：DB 现值 `views/system/SystemUsersPage` → `normalizeViewPath`（去 `./`/`../` 前缀 → 补 `/` 开头 → 去 `/views` 前缀 → 补 `.vue`）→ `/system/SystemUsersPage.vue`；pageMap 键 `../views/system/SystemUsersPage.vue` 规范化后同值。**新页面文件必须命名为 `apps/web-antd/src/views/system/SystemUsersPage.vue` 这一形态**。

**配套前端一行**：`src/api/core/auth.ts` 的 `logoutApi` 从 `baseRequestClient` 改 `requestClient`（补 Authorization 头）。否则正常登出不带 token，服务端撤销永远不会发生（token 活到 2h 过期）。

**文件级改动清单（后端）**：

| 文件 | 动作 |
|---|---|
| `modules/system/controller/vben/`（新包） | 5 端点 + 信封助手 |
| `modules/system/module.go` | RegisterRoutes 挂 `/auth`、`/user`、`/menu` 三组 |
| `pkg/middleware/auth.go` | 路径三档分类（§5） |
| `modules/system/service/permission_routes.go` | 加 `order:order` 条目（§6） |
| `modules/system/migrations/20261009000006_menus_vben_backfill.{up,down}.sql` | 新迁移（§6） |
| 测试 | 壳路由 httptest、中间件三档、order 策略与 rebuild 幂等（§8） |

## 5. 中间件与鉴权

`pkg/middleware/auth.go` 路径判定由二分改三档：

| 档位 | 路径 | 校验链 |
|---|---|---|
| **公开** | 其余一切（含 `/api/auth/login`、`/api/auth/logout`、`/api/app/**`、health/metrics/pprof） | 直接放行（现状） |
| **admin 完整门控**（不变） | `/api/admin/**` | token → member 403 → Casbin Enforce → data_scope 注入 → claims 覆盖租户 |
| **vben 自读门控**（新增） | **精确匹配**三个：`/api/auth/codes`、`/api/user/info`、`/api/menu/all` | token → member 403 → claims 覆盖租户 → **跳过 Casbin、跳过 data_scope** |

要点：

- **token 校验完全复用**：`auth.mode=token` 时 Validator 为 nil 仍 fail-closed 401；
- **member 403 前置**：`user_type=2` 打不进自读档（管理员控制台不对会员开放），仍置于 Enforcer 判定之前；
- **跳过 Casbin 的理由**：三端点返回"凭 token 认识你自己"的数据，授权已由 role_menus 在数据层决定；叠 Casbin 等于"先有登录鉴权按钮菜单才能读到菜单列表"——自定义角色漏勾即登录后白屏（锁死）。因此**不把这三条路径加进 `permissionRoutes`**；
- **跳过 data_scope 的理由**：无行级列表查询；租户隔离靠 claims 覆盖 + `UserMenus` 按 tenant 过滤（与现 profile 一致）；
- **精确匹配而非前缀**：防未来新增 `/api/auth/*` 路径被静默门控；
- 操作日志中间件仍只管 `/api/admin/**`：新端点不写操作日志（登录审计在 login service 内部已有，不丢）。

## 6. 菜单与数据

**现状**：`menus` 表 14 行（1 目录 + 10 页面 + 3 按钮），component 值与 vben 映射零冲突；icon 是裸名（`settings` 等）需换 iconify 格式。

**新增菜单**（幂等回填）：

| 菜单 | Path | Type | Component | Icon | Sort | Permission |
|---|---|---|---|---|---|---|
| 工作台 | `/dashboard` | dir | `""` | `lucide:layout-dashboard` | 1 | `""` |
| 数据分析 | `/dashboard/analytics` | page | `views/dashboard/analytics/index` | `lucide:area-chart` | 1 | `""` |
| 工作空间 | `/dashboard/workspace` | page | `views/dashboard/workspace/index` | `carbon:workspace` | 2 | `""` |
| 订单 | `/order` | dir | `""` | `lucide:shopping-cart` | 2 | `order` |
| 订单列表 | `/order/orders` | page | `views/order/OrderOrdersPage` | `lucide:list` | 1 | `order:order:*` |

- dashboard 两个子页用 **vben 自带组件**（不迁 admin-web 的 DashboardPage）；`/system` 目录现 Sort=10 不动，dashboard/order 插在其前；子页 Sort 在各自父级内排序；
- 前端 `preferences.ts` 的 `defaultHomePath` 设为 **`/dashboard/analytics`**（直达叶子——父目录无 redirect，push `/dashboard` 会空白；vben 菜单目录项点击只展开不导航，风险仅剩手输 URL）；
- order 子页 perm `order:order:*` 的前两段 `order:order` 正是 `permissionRoutes` 新条目的键（§下）。

**现有菜单 icon UPDATE**（一次性，11 处含目录）：

`settings→lucide:settings`、`user→lucide:user`、`team→lucide:users`、`menu→lucide:list-tree`、`apartment→lucide:building-2`、`idcard→lucide:id-card`、`book→lucide:book-open`、`setting→lucide:settings-2`、`bell→lucide:bell`、`history→lucide:history`、`profile→lucide:file-text`。

**order 接口 403 修复**（接入前它是坏的）：order 路由 `/api/admin/order/orders` 受 Casbin 门控，但 `permissionRoutes` 无 order 条目 → 永远无策略。修复分两半：

1. **代码**：`permissionRoutes` 加 `"order:order": {"/api/admin/order/orders", "/api/admin/order/orders/*"}`（`/*` 变体覆盖未来 item 路由）；
2. **策略生成时机——无需新增代码**（设计核实后的简化，原方案"seed 尾部无条件 RebuildPolicies"已存在）：`seed()` 的菜单创建块在 ~273 行闭合，其后的 `SetRoleMenus`（全量重绑）与 `p.Perm.RebuildPolicies`（无条件）**每次启动都执行**；且 `StartSeeder` 的 fx.Invoke 先于 `StartHTTPServer`。因此迁移插完菜单 → 同一次启动内自动完成绑定与 order 策略重建。

**载体**：golang-migrate SQL 迁移 `20261009000006_menus_vben_backfill`：

- `NOT EXISTS` 幂等 INSERT（`menus` 5 行 + `role_menus` 绑定，`tenant_id=0`、`roles.code='admin'`，唯一键 `uk_role_menus_tenant_role_menu` 防重）——迁移完立即可见，不等 seeder；seeder 的全量重绑是稳态兜底；
- icon UPDATE 同文件（`NOT EXISTS` 语义同样幂等：按旧值匹配才更新）；
- down：删除新增 5 行 + icon 还原旧值。

**验收**：迁移连跑两遍幂等；vben 登录 → 侧栏 工作台 → 订单 → 系统管理；order 列表 API 200。

## 7. 前端改造

**落位**：路径保持 `admin-web/`（README/Makefile/CI 目录引用不破），内容整体换入 vben 官方仓库裁剪版；vendor 时记录上游 commit SHA 到 `admin-web/README.md`，不追上游。

**精简范围**：

| 保留 | 删除 |
|---|---|
| `apps/web-antd`、`packages/**`（@core/effects/stores/utils/hooks/request/types/access/locales/…）、`internal/**`（vite-config、lint-configs，web-antd 构建依赖）、`pnpm-workspace.yaml`（catalog 版本表）、根 `package.json`（scripts 裁剪） | `apps/web-ele`、`web-naive`、`web-tdesign`、`web-antdv-next`、`backend-mock`、`playground`、`docs`（vitepress 文档站同为 workspace 包） |

顺序：先全量落地跑通 `pnpm install && pnpm --filter @vben/web-antd build` → 删冗余 → 清根配置（knip/eslint）对被删包的引用 → 复验构建。

**定制点清单**（vendored 文件内的精确改动）：

1. `src/preferences.ts`：`app.accessMode: 'backend'`、`app.defaultHomePath: '/dashboard/analytics'`；copyright 品牌改 alexGo-cloud；
2. `.env`：`VITE_APP_TITLE=alexGo-cloud 管理后台`、`VITE_APP_STORE_SECURE_KEY` 换随机值（官方默认是占位串）；`.env.development`：`VITE_NITRO_MOCK=false`；
3. `vite.config.ts`：proxy `/api` → `http://localhost:8080`，**去掉默认 rewrite**（后端路由自带 `/api` 前缀）；`/api/app/member`、`/api/admin/member` → `MEMBER_PROXY` 环境变量（默认 :8080，micro 联调指 :8081，沿用现惯例）；`VITE_PORT=5666` 保留；
4. `src/api/core/auth.ts`：`logoutApi` 改用 `requestClient`（§4 一行）；
5. `src/router/routes/modules/`：**删 `dashboard.ts`/`demos.ts`/`vben.ts`** 及 `views/demos`——backend 模式忽略前端静态路由，留着只误导；`core.ts`（登录/404 等）保留；
6. `src/api/`：新建业务 API 函数，调现有 `/api/admin/system/**`、`/api/admin/order/**`（业务 API 格式自定，vben 信封只约束 §4 五端点）。

**页面迁移**（11 个，功能基线 = 现 admin-web 每页）：

- `views/system/`：`SystemUsersPage` / `SystemRolesPage` / `SystemMenusPage` / `SystemDeptsPage` / `SystemPostsPage` / `SystemDictPage` / `SystemConfigsPage` / `SystemNoticesPage` / `SystemLoginLogsPage` / `SystemOperateLogsPage`
- `views/order/OrderOrdersPage.vue`
- 文件名严格等于菜单 component 值（§4 映射规则）；实现复用 vben 自带封装（`useVbenVxeGrid` 表格、`useVbenModal` 弹窗、`v-access` 接 `/auth/codes` 权限码）；
- **不迁**：LoginPage（vben 自带 `_core/authentication/login.vue`，`loginApi` 已对齐）、DashboardPage（用 vben 自带 dashboard 两页）。

**工程命令**：`cd admin-web && pnpm install && pnpm dev`（:5666）；README/Makefile 的 `npm run dev` 改 pnpm；**检查项**：ci.yml / docker / helm 若有前端构建引用则同步更新（实施时逐处核对，不预设存在）。

## 8. 实施顺序与测试

**阶段**（每步独立验收；C 依赖 A+B，D 依赖 C）：

| 阶段 | 内容 | 验收 |
|---|---|---|
| A · 后端 | 5 壳路由 + 中间件三档 + `permissionRoutes` order 条目 + 单测 | curl 五端点信封/字段/菜单树正确；既有测试全绿 |
| B · 数据 | SQL 迁移，`--migrate-only` 跑 | 连跑两遍幂等；`GET /menu/all` 出三组菜单；order API 200 |
| C · 前端骨架 | vendored 落地 → 精简 → 定制点 → `pnpm build` | 登录 → 侧栏 → dashboard 全链路通 |
| D · 页面 | 11 页迁移（可按页推进） | 每页冒烟 |
| E · 收尾 | README/Makefile/CI 引用检查、文档 | `make test` + `pnpm build` 全绿 |

**后端单测**（延续"CI 零外部依赖"）：

- 壳路由 httptest：`code:0` 信封、`accessToken` 字段、`LAYOUT→""` 转换、登录失败 401+message、logout 无头也 200、空菜单回 `[]`；
- 中间件：自读端点无 token → 401、member token → 403、**enforcer 无策略仍 200**（证明跳过 Casbin）、`/api/admin/**` 逐一回归不变；
- service：order 条目 rebuild 后策略存在、`RebuildPolicies` 连跑两次幂等且不吞 g 绑定。

**迁移测试**：golang-migrate + 真实 MySQL，单测覆盖不到 → `--migrate-only` 对本地与远程库各跑两遍验证幂等，down 可还原。

**前端**：一期不建测试基建（vben 页面测试在 playground，不在 web-antd），以手工验收清单替代：

1. 登录：错密码有提示；正确登录进 `/dashboard/analytics`；
2. 侧栏：三组菜单、图标渲染、排序（工作台 → 订单 → 系统管理）；
3. 11 页 CRUD 冒烟（列表加载 + 关键操作）；
4. `v-access` 按钮权限随角色生效；
5. 登出后旧 token 调接口 401（验证真撤销）；
6. member token 打自读端点 403（curl）；
7. micro 形态 `MEMBER_PROXY` 联调（沿用惯例，可选）。

**回滚**：后端全增量；SQL 有 down；前端整目录替换落在独立 commit，可单点 revert。

## 9. 风险与边界

| 风险 | 处置 |
|---|---|
| vben 上游升级 | 钉死 vendor commit，不追上游；升级另立任务 |
| 图标渲染为空 | 只用 iconify catalog 内名字（lucide/carbon，`@iconify/json` 在 catalog） |
| token 过期体验 | `enableRefreshToken` 保持 false（vben 默认），过期即重登页——与现 admin-web 行为一致；refresh 兼容端点一期不做 |
| 手输 `/dashboard` 空白 | 父目录无 redirect（vben 对绝对路径子路由不自动补）；`defaultHomePath` 直指叶子，目录项点击只展开不导航 |
| 前端构建引用遗漏 | 阶段 E 逐处核对 ci.yml/docker/helm，不预设清单 |
| 精简后 workspace 引用断裂 | 先全量跑通 build 再删，删后复验 |

## 10. 总验收清单

- [ ] `make test` 全绿（含新增三类单测）
- [ ] `--migrate-only` 迁移两遍幂等、down 可还原
- [ ] curl 五端点：信封/字段/门控（401/403/member 403）符合 §4/§5
- [ ] `admin-web`：`pnpm install && pnpm --filter @vben/web-antd build` 通过
- [ ] 手工验收清单 7 项全过
- [ ] 旧接口回归：`/api/admin/system/auth/profile`、`/api/app/system/auth/login` 行为不变
- [ ] README/Makefile/CI 引用已更新
