/**
 * 房间快照轮询（模块级单例风格，生命周期由页面驱动）
 *
 * - roomId 支持 number / ref / getter（惰性求值）：@wot-ui/router 的 currentRoute
 *   在页面 setup 之后（onLoad/onShow）才同步本页 query，setup 时读到的是上一路由的值，
 *   故页面统一传 computed，每次 refresh 现取现用（R1 Important）
 * - start()：挂 3s setInterval 拉 snapshot（幂等，重复 start 不叠加定时器）
 * - stop()：清定时器（幂等）；页面 onHide / onUnload 调用
 * - refresh()：立即拉一次（onShow / 下拉刷新调用）；错误抛给调用方自行 toast
 * - 写入走 roomStore.applySnapshot（version 单调门：等值去重、旧值丢弃，防旧覆盖新）
 * - 轮询 tick 内错误静默（网络抖动不刷屏；401 由 handlers 全局清态跳登录）
 * - pending 防重入：上一次未返回时不并发拉，避免慢响应乱序
 */
import type { MaybeRefOrGetter } from 'vue'
import { snapshot as fetchSnapshot } from '@/api/scorecard/room'
import { useScorecardRoomStore } from '@/store/scorecard/room'

/** 轮询间隔（毫秒）。 */
const POLL_INTERVAL_MS = 3000

export function useRoomPolling(roomId: MaybeRefOrGetter<number>) {
  const roomStore = useScorecardRoomStore()
  const timer = ref<ReturnType<typeof setInterval> | null>(null)
  const pending = ref(false)

  /** 立即拉一次快照并按 version 单调门写入 store。 */
  async function refresh(): Promise<void> {
    const id = toValue(roomId)
    if (!id || pending.value)
      return
    pending.value = true
    try {
      const snap = await fetchSnapshot(id)
      roomStore.applySnapshot(snap)
    }
    finally {
      pending.value = false
    }
  }

  /** 开始轮询（首次拉取由调用方 refresh 驱动，错误可在调用方处理）。 */
  function start(): void {
    if (timer.value)
      return
    timer.value = setInterval(() => {
      refresh().catch(() => {})
    }, POLL_INTERVAL_MS)
  }

  /** 停止轮询。 */
  function stop(): void {
    if (timer.value) {
      clearInterval(timer.value)
      timer.value = null
    }
  }

  return { start, stop, refresh }
}
