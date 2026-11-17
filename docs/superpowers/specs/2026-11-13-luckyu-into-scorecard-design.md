# luckyU 融入 scorecard appui 子包设计

> 日期：2026-11-13 ｜ 状态：已定稿（builder-check 一轮 + 3 项产品裁决已并入）
> 来源：
> - `modules/starfuyu/luckyU/` —— 被融合的 unibest 工程（星座运势/八字/求签小程序前端，融后**整目录删除**）
> - `frontend/scorecard/appui/uniapp/` —— 目标工程（wot-ui v2 + alova + uni-pages）
> - 后端契约：`docs/superpowers/specs/2026-11-12-starfuyu-module-design.md` §4.1（app 端 13 接口，统一信封）

---

## 1. 意图与背景

starfuyu 后端已融入 alexGo-cloud（member 登录、`/api/app/starfuyu/*` 新契约）。其旧前端 luckyU（unibest 模板工程）需并入仓库内唯一的 C 端小程序工程 `frontend/scorecard/appui/uniapp`，作为**一个分包**，在「我的」页提供**唯一入口**。融完删除 `modules/starfuyu/luckyU/`。

## 2. 目标 / 非目标 / 反目标

**目标**
- luckyU 全部业务页（运势首页、八字/求签、星座选择）以微信小程序**分包**形式进入 scorecard 工程，主包不膨胀。
- 「我的」页一个入口卡片进入运势首页。
- API 全部改接 starfuyu 新契约（统一信封 `code/data`、Bearer member token）。
- 复用 scorecard 登录态与 token 管理；**不带入** luckyU 的 login/register 页。
- 构建/ lint/type-check 与 scorecard 现有链路一致；融后 `pnpm lint`、`pnpm type-check`、`pnpm build:mp-weixin`（或现有等价脚本）绿。
- 融完 `modules/starfuyu/luckyU/` 整目录删除。

**非目标**
- 不重写业务交互视觉（布局/文案尽量保 luckyU 原样，仅换 UI 组件库适配层）。
- 不做 H5/其他小程序端专项适配（跟 scorecard 现有端范围一致）。
- 不改 starfuyu 后端。
- 不新增底部 tab（入口只在「我的」页）。
- 不迁移 luckyU 的 i18n demo 页、foo/alova demo、unibest 脚手架杂项。

**反目标（做出来算失败）**
- 引入第二套运行时 UI 库（`wot-design-uni` + `@wot-ui/ui` 并存）打包进产物。
- luckyU 自带登录页进入分包（与 member 登录态双轨）。
- 分包 root 放在主包 `src/pages/` 下造成主包超限或与 uni-pages 生成冲突。
- 新契约接口继续裸奔不走信封/token（旧 `api/fortune/today` 直连路径原样保留）。

## 3. 关键约束与既定决定

| # | 决定 | 依据 |
|---|---|---|
| 1 | 分包 root 定为 `src/pages-starfuyu/`，pages.json `subPackages[{root:"pages-starfuyu", pages:[...]}]`；与现有 `pages/fun` 分包并列 | scorecard 已有 subPackages 先例；`pages/*` 留给主包；uni-pages 插件对 root 不在 `pages/` 下的分包需在 `pages.config.ts` 显式配置（实现时以插件实际行为为准，必要时手写 pages.json 段） |
| 2 | 分包内页面：`index`（运势首页）、`bazi`（八字+求签一体，合并 luckyU `bazhi/index`）、`zodiac`（星座选择，若 luckyU 为 modal 则保留 modal 不单开页） | 用户裁定：一个入口；页面清单以 luckyU 实际业务页为准，demo 页不迁 |
| 3 | **UI 库：只用 `@wot-ui/ui`（scorecard 现有）**；luckyU 的 `wot-design-uni`/`z-paging` 组件逐个映射替换（wd-* → wd/@wot-ui 对应件；z-paging 列表 → scorecard 现有列表/Vxe 或简单分页） | 反目标禁双 UI 库；scorecard CLAUDE.md：最小改动、优先现有体系 |
| 4 | **HTTP：复用 scorecard `src/api` 体系（alova + 统一信封解包 + Bearer）**，为 starfuyu 新建 `src/api/starfuyu/`（fortune/bazi/divination/profile 四组函数 + 类型）；废弃 luckyU `src/http/*` 与 `src/api/*` 旧文件 | 后端契约已改统一信封；scorecard 请求层自带 401 处理 |
| 5 | **登录：复用 scorecard 登录态**；分包页面从 store 取 token；未登录点入口 → 跳 scorecard 现有登录页（或先登录再进，实现取现成路由） | 后端 member 体系；反目标禁双登录 |
| 6 | 状态：luckyU `store/zodiac.ts`（星座偏好）迁入 scorecard store（pinia），命名 `useZodiacStore`，持久化对齐 scorecard 现有 persist 方案 | 星座选择跨页共享 |
| 7 | 静态资源（星座图标、bg、字体）迁入 `src/pages-starfuyu/static/`（分包静态）或 scorecard 既有 `src/static` 按体积归置；>200KB 单文件优先分包内 | 微信分包静态资源规则 |
| 8 | 入口：`src/pages/mine/index.vue`（或「我的」实际文件名）加一张卡片/菜单项 → `uni.navigateTo` 到 `/pages-starfuyu/index/index` | 用户裁定：一个入口，位置默认「我的」页 |
| 9 | 依赖：**不新增** `dayjs`（业务零引用，决定 19）；`pinia-plugin-persistedstate` 若 scorecard 无则用其现有 persist 手段；**不**引入 `wot-design-uni`/`z-paging`/`vue-router` | 反目标 + 决定 19 |
| 10 | 工程规范：遵守 scorecard `CLAUDE.md`——`pnpm lint` / `pnpm type-check` / `pnpm build:h5`（及 mp 构建若有）为验收；页面生成走 uni-pages，不手工双写 pages.json 与 pages.config.ts（冲突时以能构建出正确 pages.json 为准） | 目标工程规范 |
| 11 | OpenSpec：**已豁免**（决定 17），只走 superpowers 主流程 | 用户裁决 |
| 12 | 融后删除 `modules/starfuyu/luckyU/`（含其 node_modules/dist）；`modules/starfuyu/` 只剩后端模块 | 用户裁定 |
| 13 | 新契约字段映射：luckyU 组件内旧字段名（若有 goLucky 信封差异）按后端 `pkg/resp` `{code,data,msg}` 与 starfuyu model JSON tag 对齐；`fortune/my/*` 需先设星座否则 30505 → 前端引导设星座 | starfuyu spec §4.1 |
| 14 | **星座数据流以服务端 profile 为权威**（设计定调）：进首页 `GET /profile` 回填；选择星座即 `PUT /profile`（失败 toast、本地乐观更新）；主取数 `GET /fortune/my/today`；本地 `useZodiacStore` 只作缓存/离线兜底，与服务端冲突时服务端赢 | builder-check Q2；跨设备一致 |
| 15 | **验收走 mock**（用户裁决）：用 scorecard 现成 `src/api/mock` 适配器造运势/八字/求签数据闭环；真 DeepSeek/真数据后补联调；验收项 3 的「闭环可点」按 mock 口径，「抓包无裸路径」仍真请求层断言 | 用户裁决：当前后端 key 空 |
| 16 | **保留 mock 降级**（用户裁决）：接口失败/空列表时沿用 luckyU `generateMockFortune` +「已使用缓存数据」提示，不新写空态 | 用户裁决；非目标「不重写交互」 |
| 17 | **OpenSpec 豁免**（用户裁决）：本变更走 superpowers specs/plans 主流程，不另建 OpenSpec 提案 | 用户裁决；scorecard CLAUDE.md 双流程冲突 |
| 18 | 分包实际仅 **2 页**：`index`、`bazi`（zodiac 是首页内联选择器非独立页；`pages-auth`、i18n demo 不迁） | builder-check 核对 luckyU 源码 |
| 19 | 不加 `dayjs` 依赖（luckyU 业务码零引用）；`z-paging` 无业务页使用不迁；`imageGenerator` 死代码不迁（§9 图片段作废） | builder-check 核对 |

## 4. 目标结构

```
frontend/scorecard/appui/uniapp/src/
├── pages/                    # 主包（不动，仅 mine 页加入口）
├── pages/fun/                # 既有分包（不动）
├── pages-starfuyu/           # 新分包 root
│   ├── index/index.vue       # 运势首页（迁自 luckyU pages/index）
│   ├── bazi/index.vue        # 八字+求签（迁自 luckyU bazhi/）
│   ├── components/           # fortune/*、zodiac/*、bazhi 组件（改用 @wot-ui）
│   ├── composables/          # useFortune、useShare 等（改接新 API）
│   └── static/               # 分包静态资源
├── api/starfuyu/             # 新：fortune.ts bazi.ts divination.ts profile.ts types.ts
└── store/zodiac.ts           # 新（或并入现有 store 目录约定）
```

`pages.config.ts` 增加 starfuyu 分包声明；easycom 规则确认覆盖 `pages-starfuyu/components`。

## 5. 入口与导航

- 「我的」页：卡片「星座运势」→ `uni.navigateTo({ url: '/pages-starfuyu/index/index' })`。
- 分包内返回：默认导航返回主包「我的」。
- 未登录：入口点击时检查 scorecard 登录态，未登录走现有登录流程（不新建登录页）。
- 运势首页内入口：星座选择 modal、查八字 → 分包内 navigateTo `bazi`。

## 6. API 对照（旧 → 新）

| luckyU 旧 | starfuyu 新（统一信封 + Bearer） |
|---|---|
| `GET api/fortune/today`（裸数组） | `GET /api/app/starfuyu/fortune/today`（`data: Fortune[]`） |
| （按星座取我的） | `GET /api/app/starfuyu/fortune/my/today`（需设星座，30505） |
| `POST api/bazi/*` | `POST /api/app/starfuyu/bazi/calculate` 等 |
| `POST api/divination/*` | `POST /api/app/starfuyu/divination/draw` 等 |
| 登录 `api/auth/login` | **不迁**——scorecard 登录态 |

具体字段以 `src/api/starfuyu/types` 与后端 model 对齐实现为准；分页 `history` 用 `resp.Page` 形状 `{list,total,page,size}`。

## 7. UI 映射要点

| luckyU | scorecard 替代 |
|---|---|
| `wd-*`（wot-design-uni） | `@wot-ui/ui` 同名/近义组件（wd-button、wd-toast 等以 wot-ui v2 文档为准） |
| `z-paging` 分页列表 | scorecard 既有列表模式（VxeGrid 或普通 scroll-view + 分页，跟邻近页面一致） |
| 自定义 fortune/zodiac 组件 | 保留结构与 class，内部控件换 wot-ui；UnoCSS 类名兼容保留 |
| unibest tabbar/config | 不迁（无 tab） |

## 8. 验收

1. `pnpm lint`、`pnpm type-check` 绿；`pnpm build`（h5 + mp-weixin 若有脚本）绿。
2. pages.json 含 `pages-starfuyu` 分包三页（或实际页数），主包 pages 无 starfuyu 页。
3. 「我的」→ 运势首页 → 八字/求签 闭环可点；未登录被登录页拦。
4. 请求层全部走 `api/starfuyu` + Bearer（mock 适配器挂同一请求层）；构建产物 grep 无旧 `api/fortune/today` 裸路径、无 `wot-design-uni`/`z-paging` 进包。
5. `modules/starfuyu/luckyU/` 删除后全仓 `go build` 不受影响（本就无 Go 引用）、`git status` 无残留。
6. 星座选择持久化，杀进程重进仍在。

## 9. 后续 / 切出

- 真环境联调（DeepSeek key 配置 + 当日运势数据）——mock 验收后的跟进票。
- ~~imageGenerator / 图片保存~~：死代码，不迁（决定 19）。
