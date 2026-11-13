/**
 * starfuyu 求签接口（modules/starfuyu/module.go appGroup 路由，逐字对齐）。
 */
import type { Divination, DivinationReq, PageData, SignResult } from './types'
import { scoreGet, scoreSend } from '@/api/scorecard/http'

const BASE = '/api/app/starfuyu'

/** 求签落库（限流命中 → ApiError 30503）。POST /divination/draw → Divination。 */
export function drawDivination(data: DivinationReq): Promise<Divination> {
  return scoreSend<Divination>('POST', `${BASE}/divination/draw`, data)
}

/** 我的求签历史（created_at 倒序，page 默认 1、size 默认 10 上限 100）。GET /divination/history → PageData。 */
export function divinationHistory(params?: { page?: number, size?: number }): Promise<PageData<Divination>> {
  return scoreGet<PageData<Divination>>(`${BASE}/divination/history`, params)
}

/**
 * 指定签号签文（1-100 越界 → ApiError 10001；公开端点免 claims）。
 * GET /divination/sign/:number —— 后端 data 是 {sign_number,sign_content} 对象
 * （controller/app/app.go DivinationSign，与 brief「data 为签文字符串」不符），
 * 此处取 sign_content 归一化，保持签名 Promise<string> 不变。
 */
export function getSign(number: number): Promise<string> {
  return scoreGet<SignResult>(`${BASE}/divination/sign/${number}`).then(r => r.sign_content)
}
