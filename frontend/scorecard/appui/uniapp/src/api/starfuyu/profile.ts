/**
 * starfuyu 星座偏好接口（modules/starfuyu/module.go appGroup 路由，逐字对齐）。
 */
import type { Profile } from './types'
import { scoreGet, scoreSend } from '@/api/scorecard/http'

const BASE = '/api/app/starfuyu'

/** 我的星座偏好（未设 → constellation:"" 不报错）。GET /profile → Profile。 */
export function getProfile(): Promise<Profile> {
  return scoreGet<Profile>(`${BASE}/profile`)
}

/** 设置/更新星座（12 星座枚举校验失败 → ApiError 10001）。PUT /profile {constellation} → Profile。 */
export function setProfile(constellation: string): Promise<Profile> {
  return scoreSend<Profile>('PUT', `${BASE}/profile`, { constellation })
}
