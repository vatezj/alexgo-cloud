# Design: fun-salary-calculator

## Context

- 设计稿四轮定稿：`desgin/preview/salary-arrival-calculator.html`（+ v1–v4 截图），hero 双跳动、本月通栏、2×2 口径格均已视觉验证。
- wot-ui v2 **无 `wd-select`**（MCP 实查），选择器实名为 **`wd-picker`**：`v-model`（数组）+ `v-model:visible` + `columns[{label,value}]` + 触发元素自绘；用户「尽量用选择的」意图 = 全部输入走轮选。
- 分包机制已就位：`vite.config.ts` `subPackages: ['src/pages/fun']` 递归扫描 → 新增 `salary/` 目录自动入包、路由 `pages/fun/salary/index`。

## Goals / Non-Goals

**Goals:**
- 单页复刻 v4 设计稿（含实时跳动），选择式录入，纯本机计算，零网络零 storage。

**Non-Goals:**
- 个税/社保/加班、日历选月、后端、广告位、方位模块的免责两句（非本工具）。

## Decisions

### D1 文件结构

```
src/utils/funSalary/calc.ts             纯函数：口径计算 + 格式化（可 node 冒烟）
src/pages/fun/salary/index.vue          页面（sal- 前缀 scoped 类，避 fc- 与 UnoCSS 撞名）
src/pages/mine/index.vue                趣味分组加一行（Modify）
```
nav 复用 `components/funCompass/ModNav.vue`（title/sub props 已具备，sub='趣味小工具 · 仅供娱乐'）。

### D2 选择器实现（5 个独立 wd-picker 实例）

| 字段 | 列 | 选项生成 | 默认 |
|---|---|---|---|
| 月薪 | 单列 | `3000…100000` 步 500，label `¥15000` | 15000 |
| 上午上班/下班、下午上班/下班 | 单列 | `00:00…23:55` 步 5 分，label=value `HH:MM` | 09:00 / 12:00 / 13:30 / 18:00 |

- 触发区 = 设计稿深色胶囊（自绘 view），`@click` 置对应 `visible=true`；`v-model` 绑定单元素数组，回显取 `[0]`。
- 五个 picker 各自独立（visible/value 分离），避免动态换 columns 的值同步坑。
- 月薪步 500 覆盖常见档且默认值在网格内；时间步 5 分覆盖设计稿全部示例（:00/:30）。

### D3 计算纯函数 `calc.ts`

```ts
export const WORK_DAYS = 21.75
export interface SalaryParams { salary: number; amStart: string; amEnd: string; pmStart: string; pmEnd: string } // HH:MM
export interface Rates { dailyHours: number; dayRate: number; hourRate: number; perSec: number }
export function calcRates(p: SalaryParams): Rates | null   // 工时<=0 → null（页面保持上次有效）
export function todayWorkedSeconds(now: Date, p: SalaryParams): number  // 周一至五 + 两个时段 clamp
export function fullWorkdaysBeforeToday(now: Date): number // 本月 1..昨日 的周一至五计数
export function todayEarnings(p, rates, now): number       // workedSec × perSec
export function monthEarnings(p, rates, now): number       // workdays×dayRate + todayEarnings
export function formatHMS(sec: number): string             // 4:30:22（时无前导零）
export function formatMoney(n: number): string             // toFixed(2)
export function minutesFromHHMM(hhmm: string): number
```
时段有效性：`pmEnd<=pmStart` 或 `amEnd<=amStart` → 该时段工时按 0 计并在页面 toast 提示（spec：保持上次有效值的实现取「rates 为 null 时不更新展示」——页面层用 `lastValidRates` 兜底）。

### D4 hero 双跳动

- 页面挂载取 `now0 = new Date()`，算 `baseToday = todayEarnings(...)`、`baseWorked = todayWorkedSeconds(...)`；`setInterval 1s`：`displayToday = baseToday + tick × perSec`（`toFixed(2)`）、`displayWorked = baseWorked + tick`（`formatHMS`）。
- 参数变更（选择器 confirm）→ 用当前时钟重算 `baseToday/baseWorked`（spec：不出现旧值突跳），`tick` 归零语义由「基准含 now + tick 计数」统一：**实现取 `elapsed = (Date.now()-anchor)/1000`**，展示 = 基准(锚点时刻) + elapsed×perSec —— 参数变更时重设 anchor 与基准，天然平滑。
- 卸载 `onUnmount` 清定时器；MP 后台 onHide 不强制（页面销毁即清）。

### D5 明细与布局

复刻 v4：本月入账通栏（`grid-column: 1/-1`）+ 2×2；内容区可滚动、底注钉底（`.sal-screen{height:100vh}` + `.sal-content{overflow:auto}` + footer 固定），与设计稿滚动行为一致。数字全 Oswald，卡片 token 同牌面。

### D6 mine 入口行

趣味方位参考行后插「工资到账计算器」：骨白小 tile 内 `¥`（brass）图标 + 副题「算日薪 · 看入账」→ `router.push('/pages/fun/salary/index')`；仍包在 `v-if="enabled"` 分组内。

## Risks / Trade-offs

- [wd-picker 弹层在 MP 的层级] 走组件标准 `root-portal` 默认即可；若与其他 fixed 冲突再开 `root-portal`。
- [195/288 项滚轮性能] wot picker 虚拟滚动常规负载，实测卡了再裁选项范围。
- [工资=金钱内容与 App 审核口径] 需求方已拍板（同方位模块先例）；文案不碰稳赚/暴富类。
- [真实时钟与设计稿快照差异] 实现按真实 now 计算（设计稿 413.78 为15:00快照示例）——e2e 断言用关系式（Δ金额≈3×每秒、Δ时间=3s）而非绝对值。

## Migration Plan

纯增量（新页+一行+一个 util）；回滚 revert 即可，不影响既有分包与页面。

## 验证方案

1. `calc.ts` node 冒烟（strip-types）：锁定快照 15000/7.5→689.66/91.95/0.0255±；formatHMS/money；非法时段→null；月/日入账样例。
2. `pnpm lint` + `type-check` + `build:h5` + `build:mp-weixin`（app.json 分包含 `salary/index`）。
3. H5 e2e：入口行跳转、三个选择器开轮选值回显、hero 3 秒双跳断言、非法时段降级、本月入账随参数变、黑名单 grep、底注存在。
4. `openspec validate`。

## Open Questions

（无——档位步长/滚动结构/口径均已定。）
