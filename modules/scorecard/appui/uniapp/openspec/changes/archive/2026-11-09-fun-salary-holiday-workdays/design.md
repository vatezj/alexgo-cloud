# Design: fun-salary-holiday-workdays

## Context

`calc.ts` 的 `isWorkday` 只判 `getDay() ∈ {1..5}`。中国实际工作日受国务院年度放假通知影响：法定假日落在周中要扣除、调休补班的周末要补上，二者每月不同。用户已确定数据方案：本地打包优先，缺年份回退到其托管的 `https://alexcloud.vate.ren/static/json/holidays/{year}.json`（holiday-cn 同格式）。

## Goals / Non-Goals

**Goals:** 工作日判定认节假日与调休；展示当月实际工作日数；数据缺失优雅退化。
**Non-Goals:** 改 21.75 计薪口径；假日数据主动推送/更新；跨年拼表。

## Decisions

- **数据格式（holiday-cn 兼容）**：`{ year, days: [{ name, date: "YYYY-MM-DD", isOffDay }] }`。`isOffDay:true` = 放假；`false` = 调休补班（周末上班）。打包为 `src/utils/funSalary/holidays/2026.json`（从 NateScarlet/holiday-cn 下载，papers 字段保留溯源国务院通知）。
- **加载策略 `holidays.ts`**：
  - `loadHolidays(year): Promise<HolidayTable | null>`——内存 `Map<year, HolidayTable|null>` 缓存（不落 storage，守「零 storage」）；命中缓存直接返回。
  - 本地打包表（静态 import 的 `Record<number, HolidayTable>`）优先；缺年份 → `uni.request` GET 远程（超时 5s）；失败 → `null`。
  - `HolidayTable = { offDays: Set<"YYYY-MM-DD">, makeupDays: Set<...> }`（由 days 预编译，判定 O(1)）。
- **判定函数签名**：`isWorkday(d, table?: HolidayTable | null)`——`table` 缺省/为 null 时行为与现状完全一致（周一至周五）；有表时：`makeupDays.has(key) → true`（补班），`offDays.has(key) → false`（假日），否则看星期。`todayWorkedSeconds` / `fullWorkdaysBeforeToday` / `monthEarnings` 同步透传 `table?`。调用点全在 salary 页，改动面收口。
- **当月工作日数**：`monthWorkdays(year, month0, table?)` 纯函数——遍历当月逐日 `isWorkday`；`table` null 时退化为数周一至周五。
- **页面接线**：`const holidays = ref<HolidayTable | null>(null)`，onMounted `loadHolidays(now.getFullYear())` 后赋值并 `if (confirmed) rebase()`；`monthDayCaption` 改为 `table` 就绪时「本月 X 个工作日 · 第 N 个」（N 沿用现有进度算法，改为 `isWorkday(now, table)` 口径），否则回退现文案。hero/明细 computed 自动响应 `holidays`。
- **请求封装**：用 `uni.request`（MP/H5 通用，不引 fetch 兼容层）；URL 常量放 `holidays.ts` 顶部注释标明改地址入口；不进 `tools.ts`（那里是 zhiwei 专用端点表，且本请求是静态 JSON 非 API）。
- **打包格式注意**：uni-app/Vite JSON import 开箱即用（`resolveJsonModule: true`）；JSON 放 `holidays/` 子目录避免与 ts 同名混淆。

## Risks / Trade-offs

- [远程 JSON 被改坏格式] → `parse` try/catch，非法整体当 null 退化；单测覆盖。
- [MP request 合法域名] → `alexcloud.vate.ren` 已是 zhiwei 生产域（`VITE_ZHIWEI_BASE_URL`），同域复用，无需新配。
- [onMounted 异步一帧内文案闪变] → 回退文案与就绪文案都以「本月…个」开头，视觉差异小，可接受。
- [12 月跨年看 1 月] → 页面只算「今天/本月」，当前年表即够；1 月 1 日属新年表，loadHolidays 取 `getFullYear()` 自然正确。

## Migration Plan

纯增量：新模块 + 函数可选参数（默认值保持旧行为）；revert 即回滚。

## 验证方案

node `--experimental-strip-types` 单测（isWorkday 四象限 + monthWorkdays + parse 退化）；lint/type-check/build 双端；H5 e2e（假日/补班/退化三态）；`openspec validate`。
