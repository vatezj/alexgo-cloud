/**
 * 工资到账计算器 · 纯计算（design.md D3；口径锁定于 openspec fun-salary）
 *
 * - 每月计薪天数固定 21.75（法定月计薪天数 (365−104)/12），页面标注，恒定不随月变
 * - 工作日 = 认法定节假日与调休（table 有表时；缺省/无表退化为周一至周五）
 * - 时段 HH:MM（选择器步 5 分产出），分钟差非法（<=0）→ calcRates 返回 null，页面保持上次有效值
 * - 纯函数零 IO（假日表由调用方注入）：可 node --experimental-strip-types 冒烟
 */

import type { HolidayTable } from './holidays'

/** 法定月计薪天数 */
export const WORK_DAYS = 21.75

export interface SalaryParams {
  /** 月薪（元） */
  salary: number
  /** 如 '09:00' */
  amStart: string
  amEnd: string
  pmStart: string
  pmEnd: string
}

export interface Rates {
  /** 每日工时（小时，浮点） */
  dailyHours: number
  /** 日薪 = 月薪 / 21.75 */
  dayRate: number
  /** 时薪 = 日薪 / 工时 */
  hourRate: number
  /** 每秒 = 时薪 / 3600 */
  perSec: number
}

/** 'HH:MM' → 当日分钟数；格式非法返回 NaN */
export function minutesFromHHMM(hhmm: string): number {
  const m = /^(\d{1,2}):(\d{2})$/.exec(hhmm.trim())
  if (!m)
    return Number.NaN
  const h = Number(m[1])
  const min = Number(m[2])
  if (h < 0 || h > 23 || min < 0 || min > 59)
    return Number.NaN
  return h * 60 + min
}

/**
 * 计算费率。任一时段分钟差非法（NaN 或 <=0）→ null（页面层用上次有效值兜底）。
 */
export function calcRates(p: SalaryParams): Rates | null {
  const am = minutesFromHHMM(p.amEnd) - minutesFromHHMM(p.amStart)
  const pm = minutesFromHHMM(p.pmEnd) - minutesFromHHMM(p.pmStart)
  if (!Number.isFinite(am) || !Number.isFinite(pm) || am <= 0 || pm <= 0)
    return null
  const dailyHours = (am + pm) / 60
  if (!(p.salary > 0))
    return null
  const dayRate = p.salary / WORK_DAYS
  const hourRate = dayRate / dailyHours
  return {
    dailyHours,
    dayRate,
    hourRate,
    perSec: hourRate / 3600,
  }
}

/** 日期 → YYYY-MM-DD（本地时区，与假日表键一致） */
function dateKey(d: Date): string {
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${d.getFullYear()}-${m}-${day}`
}

/**
 * 工作日判定（openspec fun-salary-holiday-workdays）。
 * - table 有表：调休补班日（周末上班）= 工作日；法定假日（含周中放假）= 非工作日；否则看星期
 * - table 缺省/null：退化为周一至周五（与旧版逐字一致）
 */
export function isWorkday(d: Date, table?: HolidayTable | null): boolean {
  if (table) {
    const key = dateKey(d)
    if (table.makeupDays.has(key))
      return true
    if (table.offDays.has(key))
      return false
  }
  const w = d.getDay()
  return w !== 0 && w !== 6
}

/** 今日已过班次秒数（仅工作日；时段 clamp 到 now，未到/已过返回 0） */
export function todayWorkedSeconds(now: Date, p: SalaryParams, table?: HolidayTable | null): number {
  if (!isWorkday(now, table))
    return 0
  const nowMin = now.getHours() * 60 + now.getMinutes() + now.getSeconds() / 60
  const windows: [number, number][] = [
    [minutesFromHHMM(p.amStart), minutesFromHHMM(p.amEnd)],
    [minutesFromHHMM(p.pmStart), minutesFromHHMM(p.pmEnd)],
  ]
  let total = 0
  for (const [s, e] of windows) {
    if (!Number.isFinite(s) || !Number.isFinite(e) || e <= s)
      continue
    const elapsed = Math.min(nowMin, e) - s
    if (elapsed > 0)
      total += elapsed
  }
  return Math.round(total * 60)
}

/** 本月 1 日 .. 昨日 的工作日数（不含今日） */
export function fullWorkdaysBeforeToday(now: Date, table?: HolidayTable | null): number {
  let count = 0
  const y = now.getFullYear()
  const m = now.getMonth()
  const today = now.getDate()
  for (let d = 1; d < today; d++) {
    const day = new Date(y, m, d)
    if (isWorkday(day, table))
      count++
  }
  return count
}

/**
 * 当月实际工作日总数（含节假日/调休修正；openspec fun-salary-holiday-workdays）。
 * @param year 四位年份
 * @param month0 0 基月份（Date.getMonth 口径）
 * @param table 假日表；缺省/null → 数周一至周五（退化口径）
 */
export function monthWorkdays(year: number, month0: number, table?: HolidayTable | null): number {
  const daysInMonth = new Date(year, month0 + 1, 0).getDate()
  let count = 0
  for (let d = 1; d <= daysInMonth; d++) {
    if (isWorkday(new Date(year, month0, d), table))
      count++
  }
  return count
}

/** 今日入账（元）= 今日已工作秒 × 每秒 */
export function todayEarnings(rates: Rates, workedSec: number): number {
  return Math.max(0, workedSec) * rates.perSec
}

/** 本月入账（元）= 已过完整工作日 × 日薪 + 今日已入账 */
export function monthEarnings(rates: Rates, p: SalaryParams, now: Date, table?: HolidayTable | null): number {
  const full = fullWorkdaysBeforeToday(now, table)
  return full * rates.dayRate + todayEarnings(rates, todayWorkedSeconds(now, p, table))
}

/** 秒 → H:MM:SS（小时不补零）；负值归 0 */
export function formatHMS(sec: number): string {
  const s = Math.max(0, Math.floor(sec))
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const ss = s % 60
  return `${h}:${String(m).padStart(2, '0')}:${String(ss).padStart(2, '0')}`
}

/** 金额 → 恒 2 位小数字符串 */
/** 整数段千分位分组（月薪胶囊等整数金额用） */
export function groupInt(n: number | string): string {
  return String(n).replace(/\B(?=(\d{3})+(?!\d))/g, ',')
}

/** 金额 → 千分位 + 恒 2 位小数（如 7310.38 → 7,310.38） */
export function formatMoney(n: number): string {
  if (!Number.isFinite(n))
    return '0.00'
  const [int, frac] = n.toFixed(2).split('.')
  return `${groupInt(int)}.${frac}`
}
