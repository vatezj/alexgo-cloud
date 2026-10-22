/**
 * scorecard 房间接口（modules/scorecard/controller/app + module.go 路由，逐字对齐）。
 */
import type { ConfigReq, CreateRoomReq, ExpenseReq, ExpenseResult, JoinRoomReq, PageData, PageQuery, RoomLogItem, RoomSnapshot } from './types'
import { scoreGet, scoreSend } from './http'

const BASE = '/api/app/scorecard'

/** 开房（房号可空=服务端生成 4 位）。POST /rooms → RoomSnapshot。 */
export function createRoom(data: CreateRoomReq): Promise<RoomSnapshot> {
  return scoreSend<RoomSnapshot>('POST', `${BASE}/rooms`, data)
}

/** 进房（幂等：已在房返回现状）。POST /rooms/join → RoomSnapshot。 */
export function joinRoom(data: JoinRoomReq): Promise<RoomSnapshot> {
  return scoreSend<RoomSnapshot>('POST', `${BASE}/rooms/join`, data)
}

/** 房间全量快照（非成员 30102）。GET /rooms/:id/snapshot → RoomSnapshot。 */
export function snapshot(roomId: number): Promise<RoomSnapshot> {
  return scoreGet<RoomSnapshot>(`${BASE}/rooms/${roomId}/snapshot`)
}

/** 流水分页（created_at 倒序，page 默认 1、size 默认 10 上限 100）。GET /rooms/:id/logs。 */
export function logs(roomId: number, params?: PageQuery): Promise<PageData<RoomLogItem>> {
  return scoreGet<PageData<RoomLogItem>>(`${BASE}/rooms/${roomId}/logs`, params)
}

/**
 * 记分支出（幂等键 client_request_id 由调用方生成传入，API 层不生成）。
 * POST /rooms/:id/expense → ExpenseResult。
 */
export function expense(roomId: number, data: ExpenseReq): Promise<ExpenseResult> {
  return scoreSend<ExpenseResult>('POST', `${BASE}/rooms/${roomId}/expense`, data)
}

/**
 * 改配置（仅房主；数值项 pending「从下把生效」）。
 * ConfigReq 全指针字段：只传要改的键。PUT /rooms/:id/config → RoomSnapshot。
 */
export function updateConfig(roomId: number, data: ConfigReq): Promise<RoomSnapshot> {
  return scoreSend<RoomSnapshot>('PUT', `${BASE}/rooms/${roomId}/config`, data)
}

/** 结算（仅房主）。POST /rooms/:id/settle。 */
export function settle(roomId: number): Promise<void> {
  return scoreSend<void>('POST', `${BASE}/rooms/${roomId}/settle`)
}

/** 退房（房主先自动结算）。POST /rooms/:id/leave。 */
export function leave(roomId: number): Promise<void> {
  return scoreSend<void>('POST', `${BASE}/rooms/${roomId}/leave`)
}
