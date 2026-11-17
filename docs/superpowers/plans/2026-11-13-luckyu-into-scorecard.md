# luckyU 融入 scorecard 分包 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 `modules/starfuyu/luckyU` 的星座运势/八字/求签业务以微信分包形式并入 `frontend/scorecard/appui/uniapp`，「我的」页单入口，复用 scorecard 登录态与 alova 请求层，验收走 mock，融后删 luckyU。

**Architecture:** 分包 root `src/pages/starfuyu/`（与既有 `src/pages/fun` 同构，经 `vite.config.ts` uni-pages `subPackages` 扫描自动派生，**不手写 pages.json**）。API 新建 `src/api/starfuyu/` 复用 `scoreGet/scoreSend`（统一信封 + Bearer 自动注入）；星座以服务端 profile 为权威；UI 全部 `@wot-ui/ui`，不引入 `wot-design-uni`/`z-paging`。

**Tech Stack:** Vue3 `<script setup lang="ts">`、uni-app、alova、pinia（persist 插件）、@wot-ui/ui、UnoCSS、@uni-helper/vite-plugin-uni-pages。

**Spec:** `docs/superpowers/specs/2026-11-13-luckyu-into-scorecard-design.md`（已定稿；冲突以 spec 为准）

## Global Constraints

- 工作目录：`frontend/scorecard/appui/uniapp`（下称 **appui**）；分支惯例直接 main，每任务一次 commit，**只 `git add` 本任务路径**
- 验证三连（appui 根执行）：`pnpm lint` && `pnpm type-check` && `pnpm build`（uni build；若需小程序产物加 `pnpm build:mp-weixin` 若脚本存在）——每任务收口必绿
- **分包目录钉死 `src/pages/starfuyu/`**（root 派生为 `pages/starfuyu`）；在 `vite.config.ts` 的 `UniHelperPages({ subPackages: [...] })` 数组追加 `'src/pages/starfuyu'`——**禁止手改 `src/pages.json`**（生成物）
- 页面用 `definePage({ style: {...} })` 声明（uni-pages 扫描），不手工登记路由
- API 全走 `scoreGet/scoreSend`（`src/api/scorecard/http.ts` 导出，信封 `{code,msg,data}` code=0）；base 路径前缀 **`/api/app/starfuyu`**；**禁止** luckyU 旧裸路径 `api/fortune/today`
- Bearer 由 `alovaInstance.beforeRequest` 自动注（`useScorecardAuthStore().token`）——starfuyu API 不自己拼 Authorization
- 登录复用：未登录 `router.push({ path: '/pages/login/index' })`（mine 页同语义）；**不迁** luckyU `pages-auth`
- UI 仅 `@wot-ui/ui`；产物 grep 不得出现 `wot-design-uni`、`z-paging`（easycom 规则若残留 `wd-` 指向 wot-design-uni 的映射，starfuyu 分包内组件一律用 wot-ui 真名）
- **星座数据流（spec 决定 14）**：`GET /profile` 回填 → 选座 `PUT /profile` → 主取数 `GET /fortune/my/today`；本地 `useZodiacStore` 仅缓存，冲突服务端赢；30505 → 引导选座
- **保留 mock 降级（决定 16）**：接口失败/空 → `generateMockFortune` 同款伪造 + toast「获取运势失败，已使用缓存数据」（可从 luckyU `utils/imageGenerator.ts` 抽纯函数迁入，**只迁该函数**，不迁死代码 generateHomePageImage）
- **验收 mock（决定 15）**：mockAdapter 注册 starfuyu 用例且 `enable: true`；未命中 mock 的请求仍走真 HTTP（`httpAdapter` fallback）——**scorecard 既有接口行为不得被 mock 吃掉**
- 不新增依赖（决定 19：无 dayjs）；不引入 `wot-design-uni`/`z-paging`/`vue-router`
- 源材料只读：`modules/starfuyu/luckyU/**`（Task 7 删除前可读）；字段名对齐后端 `modules/starfuyu/model` json tag 与 `docs/superpowers/specs/2026-11-12-starfuyu-module-design.md` §4.1
- 融完删除 `modules/starfuyu/luckyU/` 整目录（用户裁定）；`modules/starfuyu/` 只留后端

## Review Focus

1. **信封/路径错接**：新 API 若漏 `/api/app/starfuyu` 前缀或不走 unwrap，页面拿到裸信封渲染空白 → Task 1 测试钉 `scoreGet` 被以 `BASE` 调用且 mock 断言响应解包后的 `data`
2. **30505 未设星座分支**：`my/today` 未设星座返回 30505，页面必须引导选座而不是报错死循环 → Task 4 测试（mock 返回 code 30505 → 显示选座入口、不 toast 红错）
3. **mock 吃掉真接口**：mockAdapter `enable: true` 后若 match 过宽，scorecard 房间等真接口被截 → Task 6 测试：注册的 mock 仅匹配 `/api/app/starfuyu/` 前缀；未命中路径断言仍走 `uniappRequestAdapter`（或至少 mock 列表 pathname 全部带 starfuyu 前缀的静态断言）
4. **未登录点入口**：mine 卡片未登录必须去登录页，不能进分包后各接口 401 乱跳 → Task 3 测试（ isLoggedIn false → 路由断言到 login，不 navigateTo 分包）
5. **双 UI 库进包**：分包组件若 `import 'wot-design-uni/...'` 或 easycom 解析到它 → Task 7 收口 `grep -r "wot-design-uni\|z-paging" src/pages/starfuyu src/api/starfuyu` 必须零命中 + `pnpm build` 成功

---

### Task 1: starfuyu API 层（types + 四组函数）

**Files:**
- Create: `frontend/scorecard/appui/uniapp/src/api/starfuyu/types.ts`
- Create: `frontend/scorecard/appui/uniapp/src/api/starfuyu/fortune.ts`
- Create: `frontend/scorecard/appui/uniapp/src/api/starfuyu/bazi.ts`
- Create: `frontend/scorecard/appui/uniapp/src/api/starfuyu/divination.ts`
- Create: `frontend/scorecard/appui/uniapp/src/api/starfuyu/profile.ts`
- Create: `frontend/scorecard/appui/uniapp/src/api/starfuyu/index.ts`（桶文件 re-export）

**Interfaces:**
- Consumes: `scoreGet`, `scoreSend` from `@/api/scorecard/http`；`ApiError` from `@/api/core/handlers`
- Produces（后续任务唯一数据入口，签名钉死）:

```ts
// types.ts —— 字段对齐 modules/starfuyu/model json tag
export interface Fortune {
  id: number
  constellation: string
  date: string
  daily_overview: string
  overall_score: number
  love_score: number
  career_score: number
  wealth_score: number
  health_score: number
  lucky_color: { chinese: string, english: string, hex: string }
  lucky_number: number
  summary: string
  love_tip: string
  career_tip: string
  wealth_tip: string
  health_tip: string
}
export interface Profile { constellation: string }  // GET /profile data；未设为 ""
export interface BaziInfo { /* 四柱+wuxing+节气/农历/生肖/星座，照后端 BaziInfo json tag */ }
export interface Bazi { id: number, birth_date: string, birth_time: string, birth_place: string, bazi_info: BaziInfo }
export interface BaziReq { birth_date: string, birth_time?: string, birth_place?: string }
export interface Divination {
  id: number, question?: string, sign_number: number, sign_content: string,
  interpretation: string, category?: string, lucky_direction?: string,
  lucky_color?: string, advice?: string, created_at: string
}
export interface DivinationReq { question?: string, category?: string }
export interface PageData<T> { list: T[], total: number, page: number, size: number }

// fortune.ts
export function getTodayFortune(): Promise<Fortune[]>           // GET  /fortune/today → data 数组
export function getMyTodayFortune(): Promise<Fortune>           // GET  /fortune/my/today（30505 抛 ApiError）
export function getFortuneByDate(date: string): Promise<Fortune[]>
export function getMyFortuneByDate(date: string): Promise<Fortune>

// profile.ts
export function getProfile(): Promise<Profile>                  // GET  /profile
export function setProfile(constellation: string): Promise<Profile> // PUT  /profile {constellation}

// bazi.ts
export function calculateBazi(data: BaziReq): Promise<Bazi>     // POST /bazi/calculate（限流 30503 → ApiError.code）
export function getMyBazi(): Promise<Bazi>                      // GET  /bazi/my（30504 → ApiError）
export function updateMyBazi(data: BaziReq): Promise<Bazi>      // PUT  /bazi/my

// divination.ts
export function drawDivination(data: DivinationReq): Promise<Divination> // POST /divination/draw
export function divinationHistory(params?: { page?: number, size?: number }): Promise<PageData<Divination>> // GET /divination/history
export function getSign(number: number): Promise<string>        // GET  /divination/sign/:number → data 为签文字符串
```

- [ ] **Step 1: 写 types.ts（先于函数）**

按 Produces 块逐字段写 `types.ts`；`BaziInfo` 打开 `modules/starfuyu/model/bazi.go` 的 json tag 抄字段名（year_gan 等），注释标核对来源（照 `src/api/scorecard/types.ts` 文件头风格）。

- [ ] **Step 2: 实现五组 API 函数**

每个文件：`const BASE = '/api/app/starfuyu'`；函数体一行 `scoreGet`/`scoreSend` 调用（照 `src/api/scorecard/room.ts` 注释密度）。`getSign` 注意后端 data 是字符串不是对象。

- [ ] **Step 3: 验证**

Run: `cd frontend/scorecard/appui/uniapp && pnpm type-check && pnpm lint`
Expected: 0 error（本任务无单测框架，type-check 即门禁）

- [ ] **Step 4: Commit**

```bash
git add frontend/scorecard/appui/uniapp/src/api/starfuyu
git commit -m "feat(scorecard-appui): starfuyu API 层（信封复用 scoreGet/scoreSend）"
```

---

### Task 2: zodiac store + profile 权威数据流

**Files:**
- Create: `frontend/scorecard/appui/uniapp/src/store/starfuyu/zodiac.ts`
- Create: `frontend/scorecard/appui/uniapp/src/store/starfuyu/useProfileZodiac.ts`（或 composable 同文件，二选一实现者定，接口见下）
- Modify: 从 luckyU 迁 12 星座静态表（`modules/starfuyu/luckyU/src/store/zodiac.ts` 的 `zodiacList` + `Zodiac` 接口）

**Interfaces:**
- Consumes: Task 1 `getProfile`/`setProfile`；`store/persist.ts` 的 persist 插件（**$id 不在排除列表即自动持久化**，无需注册）
- Produces:

```ts
export interface Zodiac { id: number, name: string, enName: string, dateRange: string, icon: string, mascot: string, luckyColor: string, luckyColors: string[] }
export const zodiacList: Zodiac[]  // 12 条，icon/mascot 路径改为分包 static 相对路径（见 Task 4 资产）
export const useZodiacStore = defineStore('starfuyu-zodiac', {
  state: () => ({ currentName: '' as string }),  // 只存服务端认可的星座中文名；'' = 未设
  getters: { current: (s) => zodiacList.find(z => z.name === s.currentName) ?? null, hasSelected: ... },
  actions: {
    /** 服务端回填/本地缓存写入（不打接口） */
    hydrate(constellation: string): void
    /** 选座：先 PUT /profile，成功 hydrate；失败抛错由 UI toast（本地不落，服务端赢） */
    async select(constellation: string): Promise<void>
  },
})
// useProfileZodiac(): 进页调用——getProfile() → hydrate；401 交全局 handlers；30505/空串保持未选
```

- [ ] **Step 1: 写失败测试**

appui 若无测试运行器：以 `pnpm type-check` + 下方「可执行断言脚本」代替——在 `src/store/starfuyu/zodiac.test.md` 不建；改用 **类型即测试**：`useProfileZodiac` 的返回类型注释钉死。若仓库存在 vitest（查 `package.json` devDeps，有则写真测试）：

```ts
// select 成功：mock setProfile resolve → store.currentName === '白羊座'
// select 失败：mock setProfile reject → store.currentName 保持旧值且 error 上抛
// hydrate('') → hasSelected === false
```

Run: `pnpm type-check`（或 vitest 若有）
Expected: FAIL（文件不存在）

- [ ] **Step 2: 实现 store + composable**

- `zodiacList` 从 luckyU 原样拷 12 条（id/name/enName/dateRange/色值不动；icon/mascot 改 `/static/starfuyu/zodiac/...` 分包可访问路径——静态资源迁入放 Task 4，本任务先写目标路径）
- `select`：`await setProfile(name)` 成功才 `this.currentName = name`；失败上抛不改 state
- `useProfileZodiac`：`onMounted` 或显式 `init()` 调 `getProfile()` → `hydrate(data.constellation || '')`；请求失败静默（本地缓存兜底）

- [ ] **Step 3: 验证 + Commit**

Run: `pnpm type-check && pnpm lint`
Expected: PASS

```bash
git add frontend/scorecard/appui/uniapp/src/store/starfuyu
git commit -m "feat(scorecard-appui): starfuyu 星座 store（服务端 profile 权威）"
```

---

### Task 3: 分包骨架 + vite 扫描 + 「我的」入口

**Files:**
- Modify: `frontend/scorecard/appui/uniapp/vite.config.ts`（`subPackages` 数组 + `'src/pages/starfuyu'`）
- Create: `frontend/scorecard/appui/uniapp/src/pages/starfuyu/index/index.vue`（占位壳：标题「星座运势」+ 返回）
- Create: `frontend/scorecard/appui/uniapp/src/pages/starfuyu/bazi/index.vue`（占位壳）
- Modify: `frontend/scorecard/appui/uniapp/src/pages/mine/index.vue`（「更多趣味小工具」分组内加入口卡）

**Interfaces:**
- Consumes: `useScorecardAuthStore`（isLoggedIn）；router
- Produces: 可导航的两页分包；mine 入口行为——`isLoggedIn` → `uni.navigateTo({ url: '/pages/starfuyu/index/index' })`，否则 `router.push({ path: '/pages/login/index' })`

- [ ] **Step 1: vite subPackages 追加扫描根**

`vite.config.ts` L36 附近：`subPackages: ['src/pages/fun', 'src/pages/starfuyu']`（注释一行说明 starfuyu 分包，仿 fun 注释风格）。

- [ ] **Step 2: 两个占位页**

各页 `<script setup>` + `definePage({ style: { navigationStyle: 'custom' } })`（与 fun/salary 一致）+ 最小模板（标题 + 返回按钮 `uni.navigateBack`）。**不接 API。**

- [ ] **Step 3: mine 入口（含登录门）**

在「更多趣味小工具」折叠分组内（工资计算器同级）加一行/卡片「星座运势」：

```ts
function goStarfuyu() {
  if (!isLoggedIn.value) {
    router.push({ path: '/pages/login/index' })
    return
  }
  uni.navigateTo({ url: '/pages/starfuyu/index/index' })
}
```

样式 scoped、语义类前缀 `mine-*`、色值走 token（照 mine 页现有约定）。

- [ ] **Step 4: 验证生成物 + 构建**

Run: `pnpm type-check && pnpm lint && pnpm build`
Expected: PASS；并 **目检 `src/pages.json`**（构建/ dev 后生成）：出现 `"root": "pages/starfuyu"` 的 subPackages 段、两页 path 为 `index/index` 与 `bazi/index`、主包 pages 列表**不含** starfuyu 页。若生成物未更新，跑 `pnpm dev:mp-weixin` 一次触发或检查 uni-pages 缓存——**以生成结果为准，不手改 pages.json**。

- [ ] **Step 5: Commit**

```bash
git add frontend/scorecard/appui/uniapp/vite.config.ts frontend/scorecard/appui/uniapp/src/pages/starfuyu frontend/scorecard/appui/uniapp/src/pages/mine/index.vue frontend/scorecard/appui/uniapp/src/pages.json
git commit -m "feat(scorecard-appui): starfuyu 分包骨架 + 我的页入口（登录门）"
```

（pages.json 若在 .gitignore 则不 add——先 `git check-ignore src/pages.json` 确认。）

---

### Task 4: 运势首页（迁移 + wot-ui + mock 降级）

**Files:**
- Create: `frontend/scorecard/appui/uniapp/src/pages/starfuyu/index/index.vue`（覆写 Task 3 占位；迁自 `modules/starfuyu/luckyU/src/pages/index/index.vue`）
- Create: `frontend/scorecard/appui/uniapp/src/pages/starfuyu/components/`（`FortuneHeader.vue` `FortuneCard.vue` `FortuneContent.vue` `ZodiacSelector.vue` `ZodiacSelectorModal.vue`——按 luckyU `src/components/{fortune,zodiac}` 迁）
- Create: `frontend/scorecard/appui/uniapp/src/pages/starfuyu/composables/useFortune.ts`（改写自 luckyU 同名）
- Create: `frontend/scorecard/appui/uniapp/src/pages/starfuyu/utils/mockFortune.ts`（只迁 `generateMockFortune` 纯函数，**不迁** generateHomePageImage）
- Create: 静态资源 `frontend/scorecard/appui/uniapp/src/static/starfuyu/zodiac/**`（从 `luckyU/src/static/images/zodiac/**` 拷 icons+mascots）

**Interfaces:**
- Consumes: Task 1 `getMyTodayFortune`/`getFortuneByDate`（按需）；Task 2 `useZodiacStore`/`useProfileZodiac`/`zodiacList`；`ApiError`
- Produces:

```ts
// useFortune.ts
export function useFortune(): {
  fortuneData: Ref<Fortune | null>
  isLoading: Ref<boolean>
  currentZodiac: ComputedRef<Zodiac | null>
  hasSelectedZodiac: ComputedRef<boolean>
  checkZodiac(): void                 // 有选座则 load；无选座显示选择器
  loadFortuneData(): Promise<void>    // getMyTodayFortune；ApiError.code===30505 → 清 local选座、显示选择器；
                                      // 其他失败/空 → generateMockFortune + toast「获取运势失败，已使用缓存数据」（决定 16）
  handleReselect(): void
}
```

- [ ] **Step 1: 迁静态资源**

```bash
mkdir -p frontend/scorecard/appui/uniapp/src/static/starfuyu
cp -R modules/starfuyu/luckyU/src/static/images/zodiac frontend/scorecard/appui/uniapp/src/static/starfuyu/zodiac
# 若 luckyU 分包页还引用其他 static（bg 等），按引用清单同迁到 static/starfuyu/
```

- [ ] **Step 2: 迁组件并替换 UI 库**

逐文件拷贝 `luckyU/src/components/{fortune,zodiac}/*.vue` → `pages/starfuyu/components/`：
- `import ... from 'wot-design-uni/...'` 或模板 `wd-*`（easycom 指向 wot-design-uni 的）→ `@wot-ui/ui` 对应组件（读 appui 已有页面用法，如 mine/expense 的 `wd-*` 实际解析目标——**以 appui 现状为准**，必要时查 `.agents/skills/` wot-ui 说明）
- 路径别名 `@/` 在分包下仍指 `src/`——组件内 `@/static/images/...` 改 `@/static/starfuyu/zodiac/...`
- 样式类名/布局保留 luckyU 原样（非目标：不重写视觉）

- [ ] **Step 3: 写失败测试（useFortune 三分支）**

若无测试运行器，用「类型 + 手测清单」降级并在报告注明；若有 vitest：

```ts
// A. getMyTodayFortune resolve 正常 Fortune → fortuneData 等于返回值，无 toast
// B. reject ApiError(30505) → hasSelected 变 false（显示选择器），不伪造数据
// C. reject ApiError(50000) → fortuneData 为 mock（constellation 一致），触发降级
```

Run 对应测试 / `pnpm type-check`
Expected: FAIL（composable 不存在）

- [ ] **Step 4: 实现 useFortune + mockFortune + 首页装配**

- `mockFortune.ts`：签名 `generateMockFortune(id: number, name: string): Fortune`——从 luckyU `utils/imageGenerator.ts` **剪切该函数**（读源），返回类型改 Task 1 的 `Fortune`
- `useFortune.ts` 按 Produces；30505 分支显式 `zodiacStore.hydrate('')`
- `index/index.vue`：迁 luckyU 首页结构，接 `useFortune` + `ZodiacSelector`；选座回调 `zodiacStore.select(name)` → 成功后 `loadFortuneData()`

- [ ] **Step 5: 验证 + Commit**

Run: `pnpm type-check && pnpm lint && pnpm build`
Expected: PASS

```bash
git add frontend/scorecard/appui/uniapp/src/pages/starfuyu frontend/scorecard/appui/uniapp/src/static/starfuyu
git commit -m "feat(scorecard-appui): 运势首页迁入分包（wot-ui + 30505 分支 + mock 降级）"
```

---

### Task 5: 八字/求签页（bazi 分包页）

**Files:**
- Create: `frontend/scorecard/appui/uniapp/src/pages/starfuyu/bazi/index.vue`（覆写占位；迁自 `luckyU/src/bazhi/index.vue` 340 行）
- Create: `frontend/scorecard/appui/uniapp/src/pages/starfuyu/components/bazi/`（`BaziHeader.vue` `BaziDisplay.vue` `BaziLoading.vue` `DivinationForm.vue` `QianTong.vue`——迁自 `luckyU/src/bazhi/components/`）
- Modify: `pages/starfuyu/index/index.vue`（加「求签/八字」导航 → `uni.navigateTo('/pages/starfuyu/bazi/index')`，若首页已有入口则接通）

**Interfaces:**
- Consumes: Task 1 `calculateBazi`/`getMyBazi`/`updateMyBazi`/`drawDivination`/`divinationHistory`/`getSign`；Task 2 store；`ApiError.code`（30503 限流、30504 无八字、10001 参数）
- Produces: bazi 页完整交互；错误码 toast 文案表——`30503→今日次数已达上限`、`30504→尚未保存生辰八字`（首次 calculate 即可）、`30505→请先选择星座`（若求签依赖路径出现）

- [ ] **Step 1: 迁组件 + UI 替换**

同 Task 4 Step 2 规则（wot-ui、static 路径）。

- [ ] **Step 2: 迁页面并接新 API**

对照 `luckyU/src/bazhi/index.vue` 与 `luckyU/src/api/{bazi,divination}.ts`：
- 旧调用 → Task 1 函数名逐一替换（列对照表进报告）
- 限流 30503：toast 后禁用提交按钮至次日（本地 date 翻转用 `new Date()` 即可，**不**引 dayjs）
- 求签成功 → 展示签文/解读；历史列表若页面有 → `divinationHistory` 分页

- [ ] **Step 3: 验证 + Commit**

Run: `pnpm type-check && pnpm lint && pnpm build`
Expected: PASS

```bash
git add frontend/scorecard/appui/uniapp/src/pages/starfuyu
git commit -m "feat(scorecard-appui): 八字/求签页迁入分包（新契约 + 限流文案）"
```

---

### Task 6: mock 适配器接线（验收数据）

**Files:**
- Modify: `frontend/scorecard/appui/uniapp/src/api/mock/mockAdapter.ts`（enable: true + 注册 starfuyu 用例）
- Create: `frontend/scorecard/appui/uniapp/src/api/mock/modules/starfuyu.ts`（用例定义）
- Modify: `frontend/scorecard/appui/uniapp/src/api/core/instance.ts`（**仅当** 当前未挂 mockRequest——现状注释称 instance 不再传 mock；需重新传入且不破坏真请求 fallback）

**Interfaces:**
- Consumes: `createAlovaMockAdapter` 既有模式（`src/api/mock/modules/*.ts` 现有用例写法）
- Produces: `/api/app/starfuyu/**` 路径命中的 mock 响应；其余路径行为与 Task 6 改动前**完全一致**

- [ ] **Step 1: 写失败断言**

mock 模块文件头注释钉 match 规则；若无可测运行器，以静态结构断言替代（实现者在报告贴 `grep` 结果）：

```ts
// starfuyu.ts 用例 pathname 全部以 /api/app/starfuyu 开头
// 至少覆盖：
// GET  /api/app/starfuyu/profile            → { code:0, data:{ constellation:'白羊座' } }
// GET  /api/app/starfuyu/fortune/my/today   → { code:0, data: <Fortune 白羊座样例> }
// GET  /api/app/starfuyu/fortune/today      → { code:0, data: [12 条样例] }
// POST /api/app/starfuyu/bazi/calculate     → { code:0, data: <Bazi 样例> }
// POST /api/app/starfuyu/divination/draw    → { code:0, data: <Divination 样例> }
// GET  /api/app/starfuyu/divination/history → { code:0, data:{ list:[...], total:1, page:1, size:10 } }
// GET  /api/app/starfuyu/bazi/my            → { code:0, data: <Bazi 样例> }（或 30504 信封，二选一写死并注释）
```

- [ ] **Step 2: 实现 mock 模块 + 挂载**

- `modules/starfuyu.ts` 导出用例数组（字段对齐 Task 1 types）
- `mockAdapter.ts`：`allMocks = [...starfuyuMocks]`、`enable: true`；文件头注释更新（「starfuyu 验收期启用；未命中 fallback 真请求」）
- `instance.ts`：把 mockAdapter 接到 createAlova 的 requestAdapter（照 @alova/mock + uniapp adapter 标准接法；**scorecard 既有 URL 不在 mock 列表 → 仍走 uniappRequestAdapter**）

- [ ] **Step 3: 验证（含 Review Focus #3）**

Run: `pnpm type-check && pnpm lint && pnpm build`
Expected: PASS

静态断言（报告贴 grep）：`grep -E "pathname|url" src/api/mock/modules/starfuyu.ts` 全部含 `/api/app/starfuyu`；`src/api/mock/modules/{pet,user,store,common}.ts` 若仍存在且 enable 后会被注册——**只注册 starfuyu**，其余模块保持不 import（现状 allMocks 恒空即无他模块注册，确认不要顺手把 pet/user 加回来）。

- [ ] **Step 4: Commit**

```bash
git add frontend/scorecard/appui/uniapp/src/api/mock frontend/scorecard/appui/uniapp/src/api/core/instance.ts
git commit -m "feat(scorecard-appui): starfuyu 验收 mock（仅前缀命中，真请求 fallback）"
```

---

### Task 7: 收口——双 UI 库清零 + 删 luckyU + 文档

**Files:**
- Modify（若 grep 有命中）: `pages/starfuyu/**`、easycom 相关配置
- Delete: `modules/starfuyu/luckyU/`（整目录）
- Modify: `frontend/scorecard/appui/uniapp/CLAUDE.md` 或 `docs/`——**仅当**有现成模块说明位；否则跳过（modules.md 是后端文档，本任务可补一行 starfuyu 前端入口说明到 `docs/modules.md` §13.10 后续票行——可选）

**Interfaces:**
- Consumes: Task 1–6 全部产物
- Produces: 可交付状态；luckyU 消失

- [ ] **Step 1: 双 UI 库清零检查（Review Focus #5）**

```bash
cd frontend/scorecard/appui/uniapp
grep -rn "wot-design-uni\|z-paging" src/pages/starfuyu src/api/starfuyu src/store/starfuyu || echo CLEAN
pnpm build && grep -rn "wot-design-uni\|z-paging" dist/ || echo DIST_CLEAN
```
Expected: 两处 CLEAN（若 easycom 全局规则字符串命中 dist 属模板残留，确认无实际 import/组件解析后在报告说明；有实际引用则修）

- [ ] **Step 2: 全量验证**

Run: `pnpm lint && pnpm type-check && pnpm build`
Expected: PASS

- [ ] **Step 3: 删 luckyU**

```bash
# 确认无代码引用
grep -rn "luckyU\|modules/starfuyu/luckyU" frontend/scorecard/appui/uniapp/src || echo NO_REF
rm -rf modules/starfuyu/luckyU
ls modules/starfuyu/   # 应只剩后端（module.go、service、…）与可能的 assets
```

- [ ] **Step 4: Commit**

```bash
git add -A modules/starfuyu frontend/scorecard/appui/uniapp
git status  # 确认无意外文件（工作区既有脏文件除外——若 -A 卷入无关脏文件，改显式路径）
git commit -m "feat(scorecard-appui): starfuyu 分包收口——双UI库清零、删除 luckyU"
```

**注意**：若 `git add -A` 卷入本仓库其他未提交改动，撤销并改用显式路径（`git rm -r modules/starfuyu/luckyU` + 本任务修改文件）。

---

## 任务依赖

```
Task 1 ─┬─ Task 2 ─ Task 3 ─┬─ Task 4 ─ Task 5
        │                   └─ Task 6（可与 4/5 并行，依赖 1）
        └─────────────────── Task 7（依赖全部）
```

Task 3 可在 Task 1 后立刻做（骨架不依赖 API 实现细节）；推荐顺序 1→2→3→4→5→6→7。每任务一验一提交；Interfaces 块是跨任务契约。
