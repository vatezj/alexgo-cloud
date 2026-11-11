/**
 * 打牌记账房间 store
 *
 * - snapshot：当前房间全量快照（RoomSnapshot|null），房间页渲染源，支出/结算任务复用
 * - roomId：当前房间 id（0 = 无）；persist 插件按 $id = 'scorecard-room' 持久化整个 state，
 *   跨启动保留最近房间（lastRoomId 语义，首页「继续上局」/回访兜底可用）
 * - applySnapshot：version 单调门（Review Focus：防旧轮询响应覆盖新数据）——
 *   同房间仅写入 version 更大的快照（等值 = 去重跳过，更旧 = 丢弃）；换房（id 不同）无条件写
 */
import type { RoomSnapshot } from '@/api/scorecard/types'

export const useScorecardRoomStore = defineStore('scorecard-room', {
  state: () => ({
    roomId: 0,
    snapshot: null as RoomSnapshot | null,
  }),

  actions: {
    /** 无条件写入快照并同步 roomId（首次进房 / 换房入口；version 门由 applySnapshot 把守）。 */
    setSnapshot(snap: RoomSnapshot) {
      this.roomId = snap.id
      this.snapshot = snap
    },

    /** version 单调写入：同房间 version 更大才写；返回是否已写入（false = 等值去重或旧值丢弃）。 */
    applySnapshot(snap: RoomSnapshot): boolean {
      if (this.snapshot && snap.id === this.snapshot.id && snap.version <= this.snapshot.version)
        return false
      this.setSnapshot(snap)
      return true
    },

    /** 清快照（离开房间前重置；roomId 保留作 lastRoomId）。 */
    clearSnapshot() {
      this.snapshot = null
    },
  },
})
