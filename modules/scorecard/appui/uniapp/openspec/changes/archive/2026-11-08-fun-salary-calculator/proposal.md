# Proposal: fun-salary-calculator

## Why

用户要的【更多趣味小工具】第二个工具：**工资到账计算器**——填工资与上下班时间，算每月工作日、日薪、时薪、每秒入账，以及今日/本月已入账的实时跳动数字（设计稿四轮定稿 `desgin/preview/salary-arrival-calculator.html`）。纯前端本机计算，延续趣味工具矩阵。

## What Changes

- 新页 `pages/fun/salary/index`（挂现有 `pages/fun` 分包，路由前缀 `pages/fun/salary/*`）：
  - **选择式输入**（用户点名）：月薪与四个时间点全部用 wot 选择器（wot 实名 **`wd-picker`**，库无 `wd-select`）弹轮选择，不自由输入；月薪档 3000–100000 步 500，时间 `HH:MM` 步 5 分。
  - 计算口径（设计稿锁定）：每月工作日 **21.75**（法定月计薪天数）；每日工时 = 上午班 + 下午班；日薪=月薪/21.75；时薪=日薪/工时；每秒=时薪/3600。
  - **hero「入账中」卡**：今日入账金额按秒 `+每秒值` 自增恒 2 位小数（caption 保留「每秒 +¥x.xxxx」）+「今日工作时间」`H:MM:SS` 同频跳动；金额与时间同基准（今日已工作时长）。
  - 明细：**本月入账通栏**（已过整工作日×日薪 + 今日已工作部分，工作日=周一至周五）+ 2×2（每月工作日/日薪/每日工时/时薪）。
  - 全程 `uni` 本机计算，无任何网络请求、不上传。
- 【我的】页趣味分组新增行「工资到账计算器」→ 新页（`日期计算器` 占位保留）。
- 文案红线：禁 稳赚、躺赚、暴富、理财收益、投资回报 类词；底注「结果仅供娱乐参考 · 算薪请以实际合同为准」。

**非目标**：个税/社保/加班费精算；日历自选月份；后端接口；广告位。

**验收标准**：`pnpm lint` + `pnpm type-check` + `build:h5` + `build:mp-weixin`（分包含 salary 页）全绿；H5 端到端：选择器选值改数、hero 双数值同频跳动（金额 2 位小数、时间 H:MM:SS）、六项指标数学自洽（快照口径：15000/7.5h→689.66/91.95/0.0255）；黑名单 0 命中；`openspec validate` 通过。

## Capabilities

### New Capabilities

- `fun-salary`: 工资到账计算器——选择式录入、算薪口径、今日/本月入账实时跳动、我的页入口行。

### Modified Capabilities

（无）

## Impact

- **代码**：`src/pages/fun/salary/index.vue`（新）、`src/utils/funSalary/calc.ts`（新，纯函数）、`src/pages/mine/index.vue`（加一行）；路由由分包扫描自动生成。
- **依赖**：无新增（wd-picker 为现有 wot-ui 组件）。
- **数据/网络**：零 storage、零请求。
