<script setup lang="ts">
/**
 * 房间页 · 俯视牌桌（视觉源：desgin/preview/miniapp-breakout.html 02 房间）
 *
 * - 状态门：快照 status!=0（已结算/已关闭）→ replaceAll 去结算页（Task 9 落地前为占位页）
 * - 俯视牌桌：RoomRoster 横滚座位 + 中央最近一手（最新 type=1）+ 茶饭池铭牌
 *   （tea_pool_total + tea_percent 摘要；点击 = Task 11 弹层占位 toast）
 * - 流水：recent_logs 账板 + 「查看更多」→ wd-popup 内 logs 分页上拉
 * - 筹码轨：客服占位 | 支出 → expense（预选空）| 我的占位（Task 13 换 AvatarPopup）；
 *   点成员头像 → expense 页预填 toUserId query
 * - 轮询：onShow refresh（错误本页 toast）+ start；onHide/onUnload stop；下拉刷新 refresh
 * - 业务错误本页 toast（ApiError 非 401/403，handlers 已全局提示的不重复）；
 *   轮询 tick 内错误由 useRoomPolling 静默
 * - 色值零硬编码：全走主题 token 与全局工具类（.comply/.num/.win-n/.lose-n/.jade-n）
 */
import type { RoomLogItem } from '@/api/scorecard/types'
import { ApiError } from '@/api/core/handlers'
import { logs as fetchLogs } from '@/api/scorecard/room'
import ComplianceBanner from '@/components/scorecard/ComplianceBanner.vue'
import RoomLedger from '@/components/scorecard/RoomLedger.vue'
import RoomRoster from '@/components/scorecard/RoomRoster.vue'
import ScoreText from '@/components/scorecard/ScoreText.vue'
import { useRoomPolling } from '@/composables/scorecard/useRoomPolling'
import { useScorecardRoomStore } from '@/store/scorecard/room'

definePage({
  style: {
    navigationStyle: 'custom',
    enablePullDownRefresh: true,
  },
})

const router = useRouter()
const toast = useToast()
const authStore = useScorecardAuthStore()
const roomStore = useScorecardRoomStore()

/**
 * query.roomId → number（非法/缺失返回0）。
 * 读 router.currentRoute 而非 useRoute() 副本：后者的 reactive 拷贝依赖 watcher flush，
 * 应用内跳转的 onShow 时机仍是上一路由的 query；currentRoute 由全局 mixin 在 onLoad
 * 同步写入（先于组件 onShow），任何读取时机都是本页 query（R1 Important）。
 */
const routeRoomId = computed(() => {
  const raw = router.currentRoute.value.query.roomId
  const str = Array.isArray(raw) ? (raw[0] ?? '') : (raw ?? '')
  const n = Number(str)
  return Number.isInteger(n) && n > 0 ? n : 0
})

/**
 * 本次房间 id：query 优先，兜底 store / lastRoomId（回访直达）。
 * 必须惰性求值（computed）：@wot-ui/router 的 currentRoute 在 setup 之后（onLoad/onShow）
 * 才同步本页 query，setup 时同步读到的是上一路由的值（R1 Important）。
 */
const currentRoomId = computed(() => routeRoomId.value || roomStore.roomId || authStore.lastRoomId)

const polling = useRoomPolling(currentRoomId)

const snapshot = computed(() => roomStore.snapshot)
const myUserId = computed(() => authStore.user?.user_id ?? 0)

const members = computed(() => snapshot.value?.members ?? [])
const creatorId = computed(() => snapshot.value?.creator_id ?? 0)
const recentLogs = computed(() => snapshot.value?.recent_logs ?? [])

/** user_id → 昵称（账板/最近一手共用）。 */
const nicknameMap = computed<Record<number, string>>(() => {
  const map: Record<number, string> = {}
  for (const m of members.value)
    map[m.user_id] = m.nickname
  return map
})

/** 最近一手：recent_logs（created_at 倒序）里最新一条记分流水。 */
const lastHand = computed(() => recentLogs.value.find(item => item.type === 1) ?? null)

const lastHandTimeText = computed(() => {
  if (!lastHand.value)
    return '点头像可直接支出'
  const d = new Date(lastHand.value.created_at)
  if (Number.isNaN(d.getTime()))
    return '最近一手'
  const hm = `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
  return `${hm} · 最近一手 · 点头像可直接支出`
})

const roomCodeText = computed(() => {
  const code = snapshot.value?.room_code
  return code ? `${code.toUpperCase()} 房间` : '房间'
})

const teaPoolTotal = computed(() => snapshot.value?.tea_pool_total ?? 0)
const teaPercentText = computed(() => {
  const p = snapshot.value?.tea_percent ?? 0
  return `${p}%`
})

/** 业务信封错误本页提示；401/403（handlers 已清态全局提示）不重复 toast。 */
function toastApiError(err: unknown) {
  if (err instanceof ApiError && err.code !== 401 && err.code !== 403)
    toast.error({ msg: err.message || '请求失败' })
}

/* ── 导航 ── */

function goBack() {
  router.replaceAll({ path: '/pages/index/index' })
}

/** 支出页（Task 8 落地前为占位页）：toUserId 空 = 预选空。 */
function goExpense(toUserId?: number) {
  const query: Record<string, string> = { roomId: String(currentRoomId.value) }
  if (toUserId)
    query.toUserId = String(toUserId)
  router.push({ path: '/pages/expense/index', query })
}

function onMemberClick(userId: number) {
  goExpense(userId)
}

/* ── 占位交互 ── */

/** 茶饭池铭牌：Task 11 换真弹层，先 toast 占位。 */
function onTeaPoolClick() {
  toast.info({ msg: '茶饭明细 · Task 11 接入' })
}

function onServiceClick() {
  toast.info({ msg: '客服入口 · 敬请期待' })
}

/** 我的：Task 13 换 AvatarPopup，先 toast 占位。 */
function onMineClick() {
  toast.info({ msg: '个人面板 · Task 13 接入' })
}

/* ── 全部流水弹层（分页上拉） ── */

const LOG_PAGE_SIZE = 20

const showLedgerPopup = ref(false)
const popupLogs = ref<RoomLogItem[]>([])
const popupTotal = ref(0)
const popupPage = ref(1)
const popupLoading = ref(false)

async function loadPopupLogs(append: boolean) {
  if (popupLoading.value)
    return
  if (append && popupLogs.value.length >= popupTotal.value)
    return
  popupLoading.value = true
  try {
    const page = append ? popupPage.value + 1 : 1
    const res = await fetchLogs(currentRoomId.value, { page, size: LOG_PAGE_SIZE })
    popupPage.value = page
    popupTotal.value = res.total
    popupLogs.value = append ? [...popupLogs.value, ...res.list] : res.list
  }
  catch (err) {
    toastApiError(err)
  }
  finally {
    popupLoading.value = false
  }
}

function openLedgerPopup() {
  showLedgerPopup.value = true
  void loadPopupLogs(false)
}

/** scroll-view 触底：还有下一页则续拉。 */
function onPopupScrollEnd() {
  if (popupLogs.value.length < popupTotal.value)
    void loadPopupLogs(true)
}

/* ── 生命周期 ── */

/** 状态门：status!=0（已结算/已关闭）→ 结算页（Task 9 落地前为占位页）。 */
watch(
  () => snapshot.value?.status,
  (status) => {
    if (status != null && status !== 0)
      router.replaceAll({ path: '/pages/settle/index', query: { roomId: String(currentRoomId.value) } })
  },
)

onShow(() => {
  if (!currentRoomId.value) {
    toast.error('房间信息缺失')
    router.replaceAll({ path: '/pages/index/index' })
    return
  }
  // lastRoomId / store.roomId 同步（继续上局 & 回访兜底）
  authStore.lastRoomId = currentRoomId.value
  roomStore.roomId = currentRoomId.value
  if (!authStore.user)
    void authStore.fetchMe().catch(() => {})
  void polling.refresh().catch(toastApiError)
  polling.start()
})

onHide(() => {
  polling.stop()
})

onUnload(() => {
  polling.stop()
})

onPullDownRefresh(() => {
  polling.refresh()
    .catch(toastApiError)
    .finally(() => {
      uni.stopPullDownRefresh()
    })
})
</script>

<template>
  <view class="room-page">
    <!-- 自绘 nav -->
    <view class="nav">
      <view
        class="side"
        @click="goBack"
      >
        <text class="navicon">
          ⌂
        </text>
      </view>
      <view class="ntitle">
        {{ roomCodeText }}
        <text class="nsub">
          无操作 2 小时自动关闭 · 只能自己退房
        </text>
      </view>
      <view class="side right">
        <view class="capsule">
          <text class="cap-dots">
            •••
          </text>
          <view class="sep" />
          <view class="cap-c2">
            <view class="ring" />
          </view>
        </view>
      </view>
    </view>

    <!-- 合规横幅（置顶；可关，本会话记住） -->
    <ComplianceBanner />

    <!-- 牌桌 -->
    <view class="table">
      <view class="mark" />

      <RoomRoster
        :members="members"
        :my-user-id="myUserId"
        :creator-id="creatorId"
        @member-click="onMemberClick"
      />

      <view class="center-stack">
        <!-- 中央最近一手 -->
        <view class="center-flow">
          <view
            v-if="lastHand"
            class="cf"
          >
            <text>{{ nicknameMap[lastHand.from_user_id] || '牌友' }}</text>
            <ScoreText
              class="amt-p"
              :value="-lastHand.score"
              tone="lose"
            />
            <text class="arr">
              →
            </text>
            <text>{{ nicknameMap[lastHand.to_user_id] || '牌友' }}</text>
            <ScoreText
              class="amt-w"
              :value="lastHand.score"
              tone="win"
            />
          </view>
          <view
            v-else
            class="cf empty"
          >
            <text class="cap">
              还没有记分 · 点头像开记
            </text>
          </view>
          <text class="t">
            {{ lastHandTimeText }}
          </text>
        </view>

        <!-- 茶饭池铭牌（点击 = Task 11 弹层占位） -->
        <view
          class="base-pot"
          @click="onTeaPoolClick"
        >
          <text class="bp">
            ☕ 茶饭
          </text>
          <ScoreText
            class="pv j"
            :value="teaPoolTotal"
            tone="jade"
            :signed="false"
          />
          <view class="sep" />
          <text class="bp">
            达标
          </text>
          <text class="pv b">
            {{ teaPercentText }}
          </text>
        </view>
      </view>
    </view>

    <!-- 本局流水账板 -->
    <RoomLedger
      :logs="recentLogs"
      :nickname-map="nicknameMap"
      @more="openLedgerPopup"
    />

    <view class="grow" />

    <!-- 筹码轨 -->
    <view class="rail">
      <view
        class="side"
        @click="onServiceClick"
      >
        <text class="ico">
          🎧
        </text>
        <text>客服</text>
      </view>
      <view
        class="main"
        @click="goExpense()"
      >
        <view class="bigchip">
          <text>支出</text>
        </view>
        <text class="cap2">
          记分入口 · 头像快捷
        </text>
      </view>
      <view
        class="side"
        @click="onMineClick"
      >
        <text class="ico">
          🀄
        </text>
        <text>我的</text>
      </view>
    </view>

    <!-- 全部流水弹层（分页上拉） -->
    <wd-popup
      v-model="showLedgerPopup"
      position="bottom"
      closable
      :safe-area-inset-bottom="true"
      custom-style="border-radius: 20px 20px 0 0; background: var(--ink-700);"
    >
      <view class="ledger-popup">
        <text class="lp-title">
          全部流水
        </text>
        <text class="lp-cap">
          共 {{ popupTotal }} 条 · 上拉加载
        </text>
        <scroll-view
          class="lp-scroll"
          scroll-y
          :scroll-top="0"
          @scrolltolower="onPopupScrollEnd"
        >
          <RoomLedger
            :logs="popupLogs"
            :nickname-map="nicknameMap"
            :show-more="false"
          />
          <text
            v-if="popupLoading"
            class="lp-foot"
          >
            加载中…
          </text>
          <text
            v-else-if="popupTotal > 0 && popupLogs.length >= popupTotal"
            class="lp-foot"
          >
            没有更多了
          </text>
        </scroll-view>
      </view>
    </wd-popup>
  </view>
</template>

<style lang="scss" scoped>
.room-page {
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
  min-height: 100vh;
  padding: calc(env(safe-area-inset-top, 0px) + 6px) 0 0;
  background: var(--ink-900);
  background-image: var(--spot);
  color: var(--on-ink);
}

/* ── 自绘 nav ── */
.nav {
  display: flex;
  flex: none;
  align-items: center;
  justify-content: space-between;
  padding: 6px 16px 8px;
}

.nav .side {
  display: flex;
  min-width: 60px;
  align-items: center;
}

.nav .side.right {
  justify-content: flex-end;
}

.navicon {
  margin-right: 8px;
  color: var(--brass);
  font-family: 'Noto Serif SC', serif;
  font-size: 20px;
  line-height: 1;
}

.ntitle {
  color: var(--on-ink);
  font-family: 'Noto Serif SC', serif;
  font-size: 17px;
  font-weight: 500;
  letter-spacing: 0.06em;
  text-align: center;
}

.nsub {
  display: block;
  margin-top: 2px;
  color: var(--dim);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 10.5px;
  font-weight: 400;
  letter-spacing: 0.02em;
}

/* 胶囊占位（装饰，对齐源稿右上 capsule） */
.capsule {
  display: flex;
  width: 88px;
  height: 32px;
  align-items: center;
  border: 1px solid var(--on-ink-a28);
  border-radius: 999px;
  background: var(--on-ink-a04);
}

.cap-dots {
  flex: 1;
  color: var(--on-ink);
  font-size: 14px;
  letter-spacing: 1.5px;
  line-height: 1;
  text-align: center;
}

.capsule .sep {
  width: 1px;
  height: 17px;
  background: var(--on-ink-a28);
}

.cap-c2 {
  display: flex;
  flex: 1;
  align-items: center;
  justify-content: center;
}

.capsule .ring {
  position: relative;
  width: 13px;
  height: 13px;
  border: 2px solid var(--on-ink);
  border-radius: 50%;
}

.capsule .ring::after {
  position: absolute;
  inset: 3px;
  border-radius: 50%;
  background: var(--on-ink);
  content: '';
}

/* ── 牌桌 ── */
.table {
  position: relative;
  height: 252px;
  margin: 12px 14px 0;
  padding: 14px 0 0;
  border: 1px solid var(--brass-a50);
  border-radius: 44px;
  background: radial-gradient(85% 65% at 50% 38%, var(--felt-hi) 0%, var(--felt) 55%, var(--felt-deep) 100%);
  box-shadow: var(--shadow-tile), inset 0 0 0 2px var(--brass-a50);
}

.table .mark {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 170px;
  height: 170px;
  border: 1.5px dashed var(--brass-a50);
  border-radius: 50%;
  transform: translate(-50%, -50%);
}

.table .mark::after {
  position: absolute;
  top: 50%;
  left: -85px;
  width: 340px;
  height: 1.5px;
  background: linear-gradient(90deg, transparent, var(--brass-a50), transparent);
  content: '';
}

/* 中央堆叠：最近一手 + 茶饭池铭牌 */
.center-stack {
  position: absolute;
  bottom: 18px;
  left: 50%;
  display: flex;
  width: 300px;
  flex-direction: column;
  align-items: center;
  gap: 9px;
  transform: translateX(-50%);
}

.center-flow {
  width: 290px;
  padding: 12px 14px 10px;
  border: 1.5px solid var(--brass-a50);
  border-radius: 14px;
  background: var(--felt-deep);
  box-shadow: 0 10px 26px var(--felt-deep);
  text-align: center;
}

.center-flow .cf {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-wrap: wrap;
  gap: 2px;
  color: var(--on-ink);
  font-size: 13.5px;
}

.center-flow .cf .amt-p,
.center-flow .cf .amt-w {
  font-size: 15px;
}

.center-flow .arr {
  color: var(--dim);
  font-size: 11px;
}

.center-flow .cap {
  color: var(--dim);
  font-size: 12.5px;
}

.center-flow .t {
  display: block;
  margin-top: 5px;
  color: var(--dim);
  font-size: 10px;
}

/* 茶饭池铭牌 */
.base-pot {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 7px 18px;
  border: 1px solid var(--brass-a50);
  border-radius: 999px;
  background: var(--felt-deep);
  cursor: pointer;
}

.base-pot .bp {
  color: var(--dim);
  font-size: 11px;
  letter-spacing: 0.1em;
}

.base-pot .pv {
  font-size: 16px;
}

.base-pot .sep {
  width: 1px;
  height: 16px;
  background: var(--on-ink-a25);
}

/* ── 筹码轨 ── */
.rail {
  display: flex;
  flex: none;
  align-items: flex-start;
  justify-content: center;
  gap: 30px;
  padding: 14px 0 calc(env(safe-area-inset-bottom, 0px) + 16px);
  background: linear-gradient(180deg, transparent, var(--ink-900) 34%);
  z-index: 4;
}

.rail .side {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 5px;
  padding-top: 6px;
  color: var(--dim);
  cursor: pointer;
  font-size: 11px;
}

.rail .side .ico {
  font-size: 18px;
  line-height: 1;
}

.rail .main {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  cursor: pointer;
}

.rail .main .bigchip {
  position: relative;
  display: flex;
  width: 84px;
  height: 84px;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: repeating-conic-gradient(from 8deg, var(--brass-lo) 0 18deg, var(--brass) 18deg 36deg);
  box-shadow: 0 5px 0 var(--brass-lo), 0 14px 26px var(--ink-900);
}

.rail .main .bigchip::before {
  position: absolute;
  inset: 8px;
  border-radius: 50%;
  background: radial-gradient(circle at 50% 36%, var(--brass-hi), var(--brass) 78%);
  content: '';
}

.rail .main .bigchip text {
  position: relative;
  z-index: 1;
  color: var(--ink-900);
  font-size: 19px;
  font-weight: 700;
  letter-spacing: 0.05em;
}

.rail .main .cap2 {
  color: var(--dim);
  font-size: 11px;
}

/* ── 全部流水弹层 ── */
.ledger-popup {
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
  padding: 18px 0 calc(env(safe-area-inset-bottom, 0px) + 8px);
}

.lp-title {
  padding: 0 20px;
  color: var(--on-ink);
  font-family: 'Noto Serif SC', serif;
  font-size: 17px;
  font-weight: 700;
  letter-spacing: 0.05em;
}

.lp-cap {
  padding: 4px 20px 10px;
  color: var(--dim);
  font-size: 11px;
}

.lp-scroll {
  height: 55vh;
}

.lp-scroll .ledger {
  margin-top: 0;
}

.lp-foot {
  display: block;
  padding: 10px 0 6px;
  color: var(--dim);
  font-size: 11px;
  text-align: center;
}
</style>
