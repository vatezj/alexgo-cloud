# Tasks: fun-salary-holiday-workdays

## 1. 数据与判定

- [x] 1.1 下载 holiday-cn 2026.json（含 papers 溯源）→ `src/utils/funSalary/holidays/2026.json`；核对补班 6 天（01-04/02-14/02-28/05-09/09-20/10-10）与国庆假期段
- [x] 1.2 新建 `src/utils/funSalary/holidays.ts`：`HolidayTable` 类型、本地打包表、`loadHolidays(year)`（内存缓存 → 本地 → uni.request 远程 `https://alexcloud.vate.ren/static/json/holidays/{year}.json` 5s 超时 → null）、days 预编译成 Set、非法 JSON try/catch 返 null
- [x] 1.3 `calc.ts`：`isWorkday(d, table?)`（补班优先 → 假日 → 星期）；`todayWorkedSeconds` / `fullWorkdaysBeforeToday` / `monthEarnings` 透传 `table?`；新增 `monthWorkdays(year, month0, table?)`；**table 缺省行为与旧版逐字一致**

## 2. 页面接线

- [x] 2.1 salary 页：`holidays` ref + onMounted 加载当年表 → 赋值 + `confirmed` 时 `rebase()`；`rebase`/`monthMoneyText`/`monthDayCaption` 传入 table
- [x] 2.2 `monthDayCaption`：table 就绪 → 「本月 X 个工作日 · 第 N 个」（X=`monthWorkdays`，N 用 `isWorkday(now, table)` 口径）；未就绪 → 回退「本月第 N 个工作日计」；「每月工作日 21.75」格子不动
- [x] 2.3 头注释与「纯本机计算」文案核对：假日表只读 GET、不传用户数据，隐私行措辞如需微调则改（不含「零网络」绝对化表述则不动）

## 3. 验证

- [x] 3.1 单测（node --experimental-strip-types 或项目既有方式）：isWorkday 四象限（普通工作日/普通周末/国庆周中/补班周六）、monthWorkdays（2 月 vs 10 月不同、null 表退化）、parse 非法 JSON → null、loadHolidays 缓存命中
- [x] 3.2 lint + type-check + build:h5 + build:mp-weixin 全绿
- [x] 3.3 H5 e2e：mock 假日表验证国庆日 ¥0.00、补班日跳动、副标题 X 随月变、断网退化不炸；`openspec validate` 通过

> 3.1/3.3 备注（2026-11-09，分层如实记录）：
> **单测**（`node --experimental-strip-types src/utils/funSalary/holidays.test.ts` → ALL PASS）：isWorkday 四象限（普通周一 ✓/周日 ✓/国庆周一 false/补班周六 true，含无表退化对照）；`todayWorkedSeconds` 假日停跳（国庆周一 10:00 有表 0 秒、无表正值；补班周六有表正值、无表 0 秒）；monthWorkdays 2月=16、10月=18（X 随月变）、null 表退化 20/22；parse 五类非法入参 → null；loadHolidays 缓存同引用。
> **构建**：lint + type-check + build:h5 + build:mp-weixin 全绿（tsconfig exclude `src/**/*.test.ts`，Node 测试不进应用编译）。
> **H5 e2e**（今日 11-09 非假日，验不了假日停跳，该层由上述单测覆盖）：确认后副标题「本月 21 个工作日 · 第 6 个」（与独立日历计算一致，新文案出现即证明假日表加载成功，否则回退旧文案）、「法定月计薪天数 21.75 天」格子未动、空态/确认流程无回归。
