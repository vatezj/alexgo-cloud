# Proposal: fun-salary-holiday-workdays

## Why

用户 2026-11-09 指出：法定工作日（当月实际工作日数）每个月都不一样，当前 `isWorkday` 只判周一至周五——国庆整周会被误算为上班（今日入账虚高）、调休补班的周六又漏算（入账偏低），且页面无法展示「本月共几个工作日」。

## What Changes

- **节假日数据源**：新增 `src/utils/funSalary/holidays.ts`——本地打包 holiday-cn 格式 JSON（当前 2026 年，含放假 `isOffDay:true` 与调休补班 `isOffDay:false`）优先；本地缺该年份时回退 fetch `https://alexcloud.vate.ren/static/json/holidays/{year}.json`（用户托管，同格式）；两者都失败 → 退化为纯周一至周五判定（现状），不阻塞计算。
- **工作日判定**：`isWorkday(d)` 认节假日与调休（补班周六=工作日、放假周一至周五=非工作日）；`todayWorkedSeconds` / `fullWorkdaysBeforeToday` / `monthEarnings` 随之传入假日表。
- **当月工作日数**：新增 `monthWorkdays(year, month, holidays)` 纯函数；页面「本月入账」副标题改为「本月 X 个工作日 · 第 N 个」（X=当月实际工作日总数，随月份变化；数据未加载时回退现文案）。
- **加载时机**：页面 onMounted 拉取当年假日表（内存缓存，不落 storage，不破「零 storage」）；未返回前按周一至周五口径渲染，返回后 `rebase()` 重算。
- **口径不动**：`日薪 = 月薪 ÷ 21.75`（法定月计薪天数恒定）；「每月工作日 21.75 天」格子保留。

**非目标**：改 21.75 计薪口径；跨年月份的假日表拼接（页面只展示当月/今日，单年表足够）；节假日数据更新推送。

**验收标准**：lint/type-check/build:h5/build:mp-weixin 全绿；单测覆盖 isWorkday 四象限（工作日/周末/放假工作日/补班周末）与 monthWorkdays；H5 e2e（国庆日今日入账 ¥0.00、补班周六正常跳动、副标题 X 随月变、断网退化不炸）；`openspec validate` 通过。

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `fun-salary`: 工作日判定从「周一至周五」改为「认法定节假日与调休」；新增当月实际工作日数展示。

## Impact

- **代码**：`src/utils/funSalary/calc.ts`（判定与计数函数）、新增 `src/utils/funSalary/holidays.ts` + 打包 JSON、`src/pages/fun/salary/index.vue`（加载与副标题）；单测文件。
- **数据/网络**：只读 GET 当年假日 JSON（本地打包命中则零网络）；无用户数据外发。
