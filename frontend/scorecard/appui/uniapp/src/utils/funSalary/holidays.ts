/**
 * 中国法定节假日 / 调休表（fun-salary 工作日判定数据源）
 *
 * ── 数据格式（holiday-cn 兼容）──────────────────────────────
 * `{ year, papers, days: [{ name, date: "YYYY-MM-DD", isOffDay }] }`
 *   - isOffDay: true  = 放假（国庆等，含落在周末的日子）
 *   - isOffDay: false = 调休补班（周末上班，如 2026-10-10 周六）
 *
 * ── 加载策略（内存缓存，不落 storage，守「零 storage」）─────
 *   1. 命中内存缓存 → 直接返回
 *   2. 本地打包表（holidays/{year}.json，随包发布）优先 → 零网络
 *   3. 缺该年份 → GET 远程（用户托管，同格式）
 *      ⚠️ 改地址唯一入口 = 下方 REMOTE_BASE（末尾不带斜杠）
 *      生产实测：https://alexcloud.vate.ren/static/json/holidays/2026.json
 *   4. 任一失败/格式非法 → null（调用方退化为周一至周五判定）
 */

import json2026 from './holidays/2026.json' with { type: 'json' }

/** 远程假日表托管地址（缺本地打包年份时回退；改这里即可换源） */
const REMOTE_BASE = 'https://alexcloud.vate.ren/static/json/holidays'

/** 远程请求超时（ms） */
const REMOTE_TIMEOUT = 5000

/** holiday-cn 单日条目 */
export interface HolidayDay {
  name: string
  /** YYYY-MM-DD */
  date: string
  isOffDay: boolean
}

/** holiday-cn 原始文件 */
export interface HolidayRaw {
  year: number
  papers?: string[]
  days: HolidayDay[]
}

/** 预编译判定表（O(1) 查找） */
export interface HolidayTable {
  /** 放假日集合 YYYY-MM-DD */
  offDays: Set<string>
  /** 调休补班日集合 YYYY-MM-DD（周末上班） */
  makeupDays: Set<string>
}

/**
 * 解析 holiday-cn JSON → HolidayTable。
 * 任一环节非法（非对象/days 非数组/条目缺 date）→ 整体返 null（宁可退化不可错判）。
 */
export function parseHolidays(raw: unknown): HolidayTable | null {
  try {
    if (!raw || typeof raw !== 'object')
      return null
    const days = (raw as HolidayRaw).days
    if (!Array.isArray(days))
      return null
    const offDays = new Set<string>()
    const makeupDays = new Set<string>()
    for (const d of days) {
      if (!d || typeof d.date !== 'string' || !/^\d{4}-\d{2}-\d{2}$/.test(d.date))
        return null
      if (d.isOffDay)
        offDays.add(d.date)
      else
        makeupDays.add(d.date)
    }
    return { offDays, makeupDays }
  }
  catch {
    return null
  }
}

/**
 * 本地打包表（随包发布；缺年份走远程回退）。
 * 更新方式：从 https://github.com/NateScarlet/holiday-cn 下载当年 JSON
 * 放入 holidays/ 并在下方登记（国务院 11 月发次年通知后仓库跟更）。
 */
const LOCAL_TABLES: Record<number, HolidayTable | null> = {
  2026: parseHolidays(json2026),
}

/** 内存缓存：year → table（null 表示已知加载失败，避免同会话反复请求） */
const cache = new Map<number, HolidayTable | null>()

/** 进行中的请求（并发去重） */
const inflight = new Map<number, Promise<HolidayTable | null>>()

/** 远程拉取（uni.request，MP/H5 通用；超时/非 200/解析失败 → null） */
function fetchRemote(year: number): Promise<HolidayTable | null> {
  return new Promise((resolve) => {
    try {
      uni.request({
        url: `${REMOTE_BASE}/${year}.json`,
        method: 'GET',
        timeout: REMOTE_TIMEOUT,
        success: (res) => {
          if (res.statusCode !== 200) {
            resolve(null)
            return
          }
          resolve(parseHolidays(res.data))
        },
        fail: () => resolve(null),
      })
    }
    catch {
      resolve(null)
    }
  })
}

/**
 * 加载某年假日表。优先级：缓存 → 本地打包 → 远程 → null。
 * 返回 null 表示不可用（调用方按周一至周五退化）。
 */
export async function loadHolidays(year: number): Promise<HolidayTable | null> {
  const hit = cache.get(year)
  if (hit !== undefined)
    return hit

  const pending = inflight.get(year)
  if (pending)
    return pending

  const task = (async (): Promise<HolidayTable | null> => {
    const local = LOCAL_TABLES[year] ?? null
    const table = local ?? await fetchRemote(year)
    cache.set(year, table)
    inflight.delete(year)
    return table
  })()
  inflight.set(year, task)
  return task
}

/** 测试钩子：清空缓存（生产代码勿调用） */
export function __clearHolidaysCache(): void {
  cache.clear()
  inflight.clear()
}
