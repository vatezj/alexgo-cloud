/**
 * 节假日工作日判定单测（openspec fun-salary-holiday-workdays 3.1）
 * 运行：node --experimental-strip-types src/utils/funSalary/holidays.test.ts
 */
import assert from 'node:assert/strict'
import { isWorkday, monthWorkdays, todayWorkedSeconds } from './calc.ts'
import { loadHolidays, parseHolidays } from './holidays.ts'

// ── isWorkday 四象限（假日表来自本地打包 2026.json） ──

const table = await loadHolidays(2026)
assert.ok(table, '2026 本地打包表应加载成功')

// 普通工作日：2026-11-09 周一，无假日条目
assert.equal(isWorkday(new Date(2026, 10, 9), table), true, '普通周一应为工作日')
// 普通周末：2026-11-08 周日
assert.equal(isWorkday(new Date(2026, 10, 8), table), false, '普通周日应为休息日')
// 国庆周中放假：2026-10-05 周一，isOffDay=true
assert.equal(isWorkday(new Date(2026, 9, 5), table), false, '国庆放假周一应为休息日')
// 调休补班：2026-10-10 周六，isOffDay=false
assert.equal(isWorkday(new Date(2026, 9, 10), table), true, '补班周六应为工作日')
// 另一补班：2026-02-14 周六
assert.equal(isWorkday(new Date(2026, 1, 14), table), true, '春节补班周六应为工作日')

// ── table 缺省退化（与旧版逐字一致） ──

assert.equal(isWorkday(new Date(2026, 9, 5)), true, '无表时国庆周一退化为工作日')
assert.equal(isWorkday(new Date(2026, 9, 10)), false, '无表时补班周六退化为休息日')
assert.equal(isWorkday(new Date(2026, 10, 8)), false, '无表时周日仍为休息日')

// ── monthWorkdays：X 随月变化 + null 退化 ──

// 2026-02：纯周末口径 20，含调休（春节假 02-15..02-23 工作日放假 + 补班 02-14/02-28）→ 16
assert.equal(monthWorkdays(2026, 1, table), 16, '2026-02 含调休应为 16 个工作日')
// 2026-10：纯周末口径 22，含调休（国庆 10-01..10-07 中 5 个工作日放假 + 补班 10-10）→ 18
assert.equal(monthWorkdays(2026, 9, table), 18, '2026-10 含调休应为 18 个工作日')
assert.notEqual(
  monthWorkdays(2026, 1, table),
  monthWorkdays(2026, 9, table),
  'X 必须随月份不同',
)
// null 表退化为纯周末口径
assert.equal(monthWorkdays(2026, 1, null), 20, 'null 表 2 月退化为 20')
assert.equal(monthWorkdays(2026, 9, null), 22, 'null 表 10 月退化为 22')

// ── parseHolidays 非法输入 → null ──

assert.equal(parseHolidays(null), null, 'null 输入 → null')
assert.equal(parseHolidays('garbage'), null, '字符串输入 → null')
assert.equal(parseHolidays({}), null, '缺 days → null')
assert.equal(parseHolidays({ days: 'x' }), null, 'days 非数组 → null')
assert.equal(parseHolidays({ days: [{ date: 'bad', isOffDay: true }] }), null, '非法日期 → null')
assert.ok(parseHolidays({ days: [] }), '空 days 合法 → 空表')
assert.deepEqual(
  [...parseHolidays({ days: [{ date: '2026-10-01', isOffDay: true }, { date: '2026-10-10', isOffDay: false }] })!.offDays],
  ['2026-10-01'],
  'offDays 收录放假',
)

// ── 今日入账随假日表停跳/恢复（hero 口径） ──

const p = { salary: 15000, amStart: '09:00', amEnd: '12:00', pmStart: '13:30', pmEnd: '18:00' }
// 国庆假期内周一 10-05 10:00（原上班时段）→ 有表 0 秒；无表按工作日算出正值
const nationalDay = new Date(2026, 9, 5, 10, 0, 0)
assert.equal(todayWorkedSeconds(nationalDay, p, table), 0, '国庆放假周中入账应 0 秒')
assert.ok(
  todayWorkedSeconds(nationalDay, p, null) > 0,
  '无表时国庆周一按工作日算出正值（退化口径）',
)
// 补班周六 10-10 10:00 → 有表按工作日算出正值；无表 0 秒
const makeupSat = new Date(2026, 9, 10, 10, 0, 0)
assert.ok(todayWorkedSeconds(makeupSat, p, table) > 0, '补班周六应正常累计工时')
assert.equal(todayWorkedSeconds(makeupSat, p, null), 0, '无表时补班周六为 0 秒')

// ── loadHolidays 缓存命中（同引用） ──

const again = await loadHolidays(2026)
assert.equal(again, table, '二次加载应命中内存缓存（同一引用）')

console.log('holidays.test.ts: ALL PASS')
