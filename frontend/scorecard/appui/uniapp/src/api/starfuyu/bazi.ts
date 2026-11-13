/**
 * starfuyu 八字接口（modules/starfuyu/module.go appGroup 路由，逐字对齐）。
 */
import type { Bazi, BaziReq } from './types'
import { scoreGet, scoreSend } from '@/api/scorecard/http'

const BASE = '/api/app/starfuyu'

/** 计算八字（DeepSeek 排盘并覆盖保存；限流命中 → ApiError 30503）。POST /bazi/calculate → Bazi。 */
export function calculateBazi(data: BaziReq): Promise<Bazi> {
  return scoreSend<Bazi>('POST', `${BASE}/bazi/calculate`, data)
}

/** 我的八字（无 → ApiError 30504）。GET /bazi/my → Bazi。 */
export function getMyBazi(): Promise<Bazi> {
  return scoreGet<Bazi>(`${BASE}/bazi/my`)
}

/** 更新我的八字（同 calculate 同样限流 30503）。PUT /bazi/my → Bazi。 */
export function updateMyBazi(data: BaziReq): Promise<Bazi> {
  return scoreSend<Bazi>('PUT', `${BASE}/bazi/my`, data)
}
