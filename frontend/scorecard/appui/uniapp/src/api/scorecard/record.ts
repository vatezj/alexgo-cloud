/**
 * scorecard 战绩接口（modules/scorecard/controller/app + module.go 路由，逐字对齐）。
 */
import type { PageData, PageQuery, RecordItem, StatResult } from './types'
import { scoreGet, scoreSend } from './http'

const BASE = '/api/app/scorecard'

/** 我的战绩分页（created_at 倒序）。GET /records → PageData<RecordItem>。 */
export function myRecords(params?: PageQuery): Promise<PageData<RecordItem>> {
  return scoreGet<PageData<RecordItem>>(`${BASE}/records`, params)
}

/** 战绩明细：按 recordId 反查全房终局快照（非本人 30111）。GET /records/:recordId。 */
export function recordDetail(recordId: number): Promise<RecordItem[]> {
  return scoreGet<RecordItem[]>(`${BASE}/records/${recordId}`)
}

/** 删本人战绩行（硬删）。DELETE /records/:recordId。 */
export function deleteRecord(recordId: number): Promise<void> {
  return scoreSend<void>('DELETE', `${BASE}/records/${recordId}`)
}

/** 个人统计（总分 + 胜/负/平 + 胜率）。GET /stat → StatResult。 */
export function stat(): Promise<StatResult> {
  return scoreGet<StatResult>(`${BASE}/stat`)
}
