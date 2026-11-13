/**
 * starfuyu 运势接口（modules/starfuyu/module.go appGroup 路由，逐字对齐）。
 */
import type { Fortune } from './types'
import { scoreGet } from '@/api/scorecard/http'

const BASE = '/api/app/starfuyu'

/** 今日全部 12 星座运势（无数据空列表不报错）。GET /fortune/today → Fortune[]。 */
export function getTodayFortune(): Promise<Fortune[]> {
  return scoreGet<Fortune[]>(`${BASE}/fortune/today`)
}

/** 我的今日运势（需 profile 星座，未设 → ApiError 30505）。GET /fortune/my/today → Fortune。 */
export function getMyTodayFortune(): Promise<Fortune> {
  return scoreGet<Fortune>(`${BASE}/fortune/my/today`)
}

/** 指定日期运势（YYYY-MM-DD；无数据空列表）。GET /fortune/:date → Fortune[]。 */
export function getFortuneByDate(date: string): Promise<Fortune[]> {
  return scoreGet<Fortune[]>(`${BASE}/fortune/${date}`)
}

/** 我的指定日期运势（未设星座 → ApiError 30505）。GET /fortune/my/:date → Fortune。 */
export function getMyFortuneByDate(date: string): Promise<Fortune> {
  return scoreGet<Fortune>(`${BASE}/fortune/my/${date}`)
}
