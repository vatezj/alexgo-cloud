/**
 * 趣味方位参考 · 共享推荐状态（选座 ↔ 罗盘联动，design.md D11）
 *
 * - $id = 'fun_compass_rec'：persist 插件按 $id 落盘，与档案键 fun_compass_archives 分离
 * - 写入方：座位模拟器每次排盘成功（含空推荐 → direction 置 null）
 * - 消费方：罗盘页目标卡 + 对准判定（|夹角|≤15°）
 */
import type { SeatDirection } from '@/utils/funCompass/zhiwei'

export interface FunCompassRecommendation {
  /** 聚合推荐座；空推荐 → null */
  direction: SeatDirection | null
  /** 归因理由（服务原文） */
  reasons: string[]
  /** 生辰摘要（如 1996年8月15日 14:30），目标卡溯源用 */
  birthLabel: string
  updatedAt: number
}

export const useFunCompassRecommendationStore = defineStore('fun_compass_rec', {
  state: (): FunCompassRecommendation => ({
    direction: null,
    reasons: [],
    birthLabel: '',
    updatedAt: 0,
  }),

  actions: {
    setRecommendation(input: { direction: SeatDirection | null, reasons: string[], birthLabel: string }): void {
      this.direction = input.direction
      this.reasons = input.reasons
      this.birthLabel = input.birthLabel
      this.updatedAt = Date.now()
    },

    clear(): void {
      this.direction = null
      this.reasons = []
      this.birthLabel = ''
      this.updatedAt = 0
    },
  },
})
