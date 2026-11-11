/**
 * zhiwei 排盘客户端（后端计算；契约对齐 modules/scorecard/plugin/zhiwei 的 POST /api/calc）
 *
 * - 服务 = 紫微 iztro + 奇门 qimen-rs + 评分引擎（插件自带 37 项测试）
 * - birthHourIndex：iztro 口径 0–12（0=早子 00:00–00:59，1=丑 01–02，…，11=亥 21–22，12=晚子 23–23:59）
 * - playTime 必须带**设备本地时区偏移**（服务不读机器时区，见 zhiwei.md 实操细节 1）
 * - 8→4 聚合：麻将桌只有东南西北四座——正向座计全额、间向半分归邻座（design.md D4）
 */

/** 四座（麻将桌方位） */
import { TOOL_ENDPOINTS } from '@/config/tools'

export type SeatDirection = '东' | '南' | '西' | '北'

export interface CalcRecommendation {
  rank: number
  /** 八方位：东/南/西/北/东北/东南/西南/西北 */
  direction: string
  score: number
  reason: string
}

export interface CalcResult {
  recommendations: CalcRecommendation[]
  meta?: {
    qimenConvention?: string
    dayBoundary?: string
    trueSolarTime?: boolean
  }
  disclaimer?: string
}

export interface CalcInput {
  /** 公历年 */
  year: number
  month: number
  day: number
  hour: number
  minute: number
  gender: 'male' | 'female'
}

/** 时辰序号换算（hour 0–23 → iztro timeIndex 0–12） */
export function timeToHourIndex(hour: number): number {
  const h = ((Math.trunc(hour) % 24) + 24) % 24
  if (h === 23)
    return 12 // 晚子
  if (h === 0)
    return 0 // 早子
  return Math.floor((h + 1) / 2) // 1-2→1 丑 … 21-22→11 亥
}

/** 设备本地时区偏移的 ISO8601（如 2026-11-06T14:30:00+08:00） */
export function localISO(d: Date = new Date()): string {
  const pad = (n: number) => String(Math.abs(n)).padStart(2, '0')
  const offMin = -d.getTimezoneOffset()
  const sign = offMin >= 0 ? '+' : '−'
  const offH = Math.floor(Math.abs(offMin) / 60)
  const offM = Math.abs(offMin) % 60
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
    + `T${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
    + `${sign}${pad(offH)}:${pad(offM)}`
}

function pad2(n: number): string {
  return String(n).padStart(2, '0')
}

/** POST /api/calc —— 排盘（后端计算，出生信息仅用于本次请求） */
export async function calcRecommend(input: CalcInput): Promise<CalcResult> {
  const body = {
    birthDate: `${input.year}-${pad2(input.month)}-${pad2(input.day)}`,
    birthHourIndex: timeToHourIndex(input.hour),
    gender: input.gender,
    playTime: localISO(),
  }
  const res = await new Promise<UniApp.RequestSuccessCallbackResult>((resolve, reject) => {
    uni.request({
      url: TOOL_ENDPOINTS.zhiweiCalc,
      method: 'POST',
      data: body,
      header: { 'Content-Type': 'application/json' },
      timeout: 10000,
      success: resolve,
      fail: err => reject(new Error(err.errMsg || '测算请求失败')),
    })
  })
  if (res.statusCode < 200 || res.statusCode >= 300) {
    const msg = res.data as { error?: string } | string | undefined
    const text = typeof msg === 'string' ? msg : msg?.error
    throw new Error(text || `测算服务异常(${res.statusCode})`)
  }
  const data = res.data as CalcResult
  if (!data || !Array.isArray(data.recommendations))
    throw new Error('测算服务返回格式异常')
  return data
}

const SEAT_ORDER: SeatDirection[] = ['东', '南', '西', '北']

/** 间向 → 相邻两座（半分归邻） */
const ADJACENT: Record<string, [SeatDirection, SeatDirection]> = {
  东北: ['北', '东'],
  东南: ['东', '南'],
  西南: ['南', '西'],
  西北: ['西', '北'],
}

export interface SeatAggregate {
  /** 聚合分最高的座；全空推荐 → null */
  seat: SeatDirection | null
  scores: Record<SeatDirection, number>
  /** 归因于推荐座的理由（按 rank 序） */
  reasons: string[]
}

/** 8→4 聚合（design.md D4）：正向全额、间向半分归邻座；并列按东→南→西→北 取首 */
export function aggregateSeats(recs: CalcRecommendation[]): SeatAggregate {
  const scores: Record<SeatDirection, number> = { 东: 0, 南: 0, 西: 0, 北: 0 }
  for (const r of recs) {
    if ((SEAT_ORDER as string[]).includes(r.direction)) {
      scores[r.direction as SeatDirection] += r.score
    }
    else {
      const pair = ADJACENT[r.direction]
      if (pair) {
        scores[pair[0]] += r.score / 2
        scores[pair[1]] += r.score / 2
      }
    }
  }
  let seat: SeatDirection | null = null
  let best = 0
  for (const s of SEAT_ORDER) {
    if (scores[s] > best) {
      best = scores[s]
      seat = s
    }
  }
  const reasons = seat
    ? recs
        .filter((r) => {
          if (r.direction === seat)
            return true
          const pair = ADJACENT[r.direction]
          return !!pair && (pair[0] === seat || pair[1] === seat)
        })
        .sort((a, b) => a.rank - b.rank)
        .map(r => r.reason)
    : []
  return { seat, scores, reasons }
}
