/**
 * scorecard 好友接口（modules/scorecard/controller/app + module.go 路由，逐字对齐）。
 * setBlack/setRank 入参用 boolean（后端 body 为 0/1 整型，此处转换）。
 */
import type { FriendItem } from './types'
import { scoreGet, scoreSend } from './http'

const BASE = '/api/app/scorecard'

/** 好友排名列表（total_score 降序）。GET /friends → FriendItem[]。 */
export function listFriends(): Promise<FriendItem[]> {
  return scoreGet<FriendItem[]>(`${BASE}/friends`)
}

/** 加好友（双向写行；自加/已是好友/拉黑拦截走错误码）。POST /friends。 */
export function addFriend(friendUserId: number): Promise<void> {
  return scoreSend<void>('POST', `${BASE}/friends`, { friend_user_id: friendUserId })
}

/** 删好友（双向删行，空删幂等）。DELETE /friends/:userId。 */
export function removeFriend(userId: number): Promise<void> {
  return scoreSend<void>('DELETE', `${BASE}/friends/${userId}`)
}

/** 拉黑/解除（我方单向行，幂等）。POST /friends/:userId/black {blocked:0|1}。 */
export function setBlack(userId: number, blocked: boolean): Promise<void> {
  return scoreSend<void>('POST', `${BASE}/friends/${userId}/black`, { blocked: blocked ? 1 : 0 })
}

/** 排名参与开关（只动我方指向行）。PUT /friends/:userId/rank {enabled:0|1}。 */
export function setRank(userId: number, enabled: boolean): Promise<void> {
  return scoreSend<void>('PUT', `${BASE}/friends/${userId}/rank`, { enabled: enabled ? 1 : 0 })
}
