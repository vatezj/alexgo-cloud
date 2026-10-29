<script setup lang="ts">
/**
 * 房间页 · 俯视牌桌（视觉源：desgin/preview/miniapp-breakout.html 02 房间）
 *
 * - 状态门：快照 status!=0（已结算/已关闭）→ replaceAll 去结算页（Task 9 落地前为占位页）
 * - 俯视牌桌：RoomRoster 横滚座位 + 中央最近一手（最新 type=1）+ 茶饭池铭牌
 *   （tea_pool_total + tea_percent 摘要；点击 → TeaConfigSheet 茶饭设置弹层）
 * - 邀请（设计 03）：房号标题/右上 ••• → QrInvitePanel 扇牌小程序码弹层；
 *   转发 onShareAppMessage 与小程序码同走 scene 进房协议（roomId 直达不再作为邀请 path）
 * - 流水：recent_logs 账板 + 「查看更多」→ wd-popup 内 logs 分页上拉
 * - 筹码轨：客服占位 | 支出 → expense（预选空）| 结束牌局 → settle（房主 status===0）| 我的 → AvatarPopup（设计 10 头像弹窗）；
 *   点成员头像 → expense 页预填 toUserId query
 * - 轮询：onShow refresh（错误本页 toast）+ start；onHide/onUnload stop；下拉刷新 refresh
 * - 业务错误本页 toast（ApiError 非 401/403，handlers 已全局提示的不重复）；
 *   轮询 tick 内错误由 useRoomPolling 静默
 * - 色值零硬编码：全走主题 token 与全局工具类（.comply/.num/.win-n/.lose-n/.jade-n）
 */
import type { RoomLogItem } from '@/api/scorecard/types'
import { ApiError } from '@/api/core/handlers'
import { logs as fetchLogs } from '@/api/scorecard/room'
import AvatarPopup from '@/components/scorecard/AvatarPopup.vue'
import ComplianceBanner from '@/components/scorecard/ComplianceBanner.vue'
import QrInvitePanel from '@/components/scorecard/QrInvitePanel.vue'
import RoomLedger from '@/components/scorecard/RoomLedger.vue'
import RoomRoster from '@/components/scorecard/RoomRoster.vue'
import ScoreText from '@/components/scorecard/ScoreText.vue'
import TeaConfigSheet from '@/components/scorecard/TeaConfigSheet.vue'
import { useRoomPolling } from '@/composables/scorecard/useRoomPolling'
import { useSystemBar } from '@/composables/useSystemBar'
import { useScorecardAuthStore } from '@/store/scorecard/auth'
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
const { pageStyle } = useSystemBar()

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

/** 本房快照（store 可能持有其它房快照，按 id 过滤）。 */
const snapshot = computed(() =>
  roomStore.snapshot && roomStore.snapshot.id === currentRoomId.value ? roomStore.snapshot : null,
)
const myUserId = computed(() => authStore.user?.user_id ?? 0)

/** 房主且本局仍进行中：与 settle 页同门（快照未载入 fail-closed 不渲染）。 */
const canSettle = computed(() => {
  const snap = snapshot.value
  return !!snap && snap.status === 0 && myUserId.value > 0 && snap.creator_id === myUserId.value
})

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

/**
 * 最近一手：recent_logs（created_at 倒序）里最新一条**有分数转移**的流水。
 * 只记茶（score=0）不是「一手」，跳过，避免中央展示成 A −0 → B +0。
 */
const lastHand = computed(() => recentLogs.value.find(item => item.type === 1 && item.score > 0) ?? null)

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

/** 结束牌局入口（筹码轨）：房主 status===0 → settle 页（页内二次确认后 settle）。 */
function goSettle() {
  router.push({ path: '/pages/settle/index', query: { roomId: String(currentRoomId.value) } })
}

/** 头像弹窗显隐（「我的」筹码轨与点自己头像共用入口）。 */
const showAvatarPopup = ref(false)

function onMemberClick(userId: number) {
  // 点自己头像 → 头像弹窗（设计 10）；点其他成员 → expense 预填记分对象
  if (userId === myUserId.value) {
    showAvatarPopup.value = true
    return
  }
  goExpense(userId)
}

/* ── 邀请弹层（设计 03） ── */

const showQrInvite = ref(false)

/** 房号标题 / 右上 ••• → 邀请弹层（快照就绪才有房号可复制/出码）。 */
function onInviteClick() {
  if (!snapshot.value)
    return
  showQrInvite.value = true
}

/** 转发卡片：邀请弹层 open-type="share" 与右上角胶囊菜单共用；scene 协议进房（幂等 join）+ inv 绑好友。 */
onShareAppMessage(() => {
  const code = snapshot.value?.room_code
  const inv = myUserId.value
  // 收敛到 scene 协议：index onShow consumePendingScene → joinRoom 幂等（已在房返回现状）；inv 静默绑好友。
  // 分隔符逗号与小程序码一致（parseScene 兼容历史 & 卡片）
  const scene = code
    ? (inv ? `qr=${code},inv=${inv}` : `qr=${code}`)
    : null
  return {
    title: `${roomCodeText.value} · 邀你打牌记账`,
    path: scene
      ? `/pages/index/index?scene=${encodeURIComponent(scene)}`
      : '/pages/index/index',
  }
})

/* ── 占位交互 ── */

/** 茶饭池铭牌 → 茶饭设置弹层（非房主/已结算在组件内只读）。 */
const showTeaConfig = ref(false)

function onTeaPoolClick() {
  if (!snapshot.value)
    return
  showTeaConfig.value = true
}

/** 我的 → 头像弹窗（设计 10：昵称 / 语音 5 档 / 主题入口 / 退出房间）。 */
function onMineClick() {
  showAvatarPopup.value = true
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

/**
 * 状态门：status!=0（已结算/已关闭）→ 结算页。
 * 不用 immediate：setup 期 currentRoute 尚未同步（R1 契约），兜底链会拉回旧房 id，
 * 若 persist 快照恰为旧房终局会被劫持到旧房 settle 页。回访终局在 onShow 补检（见下）。
 */
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
  // 回访补检：persist 快照已是终局且属于本房（onShow 时 currentRoute 已同步，id 过滤已排除异房）。
  // version 等值去重不会触发上面的 watch，故此处显式跳转。
  const snap = snapshot.value
  if (snap && snap.status !== 0 && snap.id === currentRoomId.value) {
    router.replaceAll({ path: '/pages/settle/index', query: { roomId: String(currentRoomId.value) } })
    return
  }
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
  <view
    class="room-page"
    :style="pageStyle"
  >
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
      <view
        class="ntitle"
        @click="onInviteClick"
      >
        {{ roomCodeText }}
        <text class="nsub">
          无操作 2 小时自动关闭 · 只能自己退房
        </text>
      </view>
      <view class="side right">
        <!-- #ifndef MP-WEIXIN -->
        <view
          class="capsule"
          @click="onInviteClick"
        >
          <text class="cap-dots">
            •••
          </text>
          <view class="sep" />
          <view class="cap-c2">
            <view class="ring" />
          </view>
        </view>
        <!-- #endif -->
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
        @invite="onInviteClick"
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

        <!-- 茶饭池铭牌（点击 → 茶饭设置弹层） -->
        <view
          class="base-pot"
          @click="onTeaPoolClick"
        >
          <view class="bp">
            <wd-icon
              css-icon
              class="bp-icon"
              name="i-carbon-cafe"
            />
            茶饭
          </view>
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

    <!-- 本局流水账板：flex 链 room-page(100vh) → zone → 组件 host(.ledger-host) →
         .ledger.fill → scroll-view，逐层 flex:1 + min-height:0 压出定高，内部滚动。
         注意：MP 自定义组件会渲染真实 host 节点（未开 virtualHost），它是 zone 的
         flex 子元素——不给 host 定 flex 规则的话，flex:1 只作用在组件内部根节点上，
         host 按内容自然高度把账板顶进筹码轨且 scroll-view 无法滚动（H5 无 host 节点，
         class 直接合并到组件根，同一条规则两边都成立） -->
    <view class="ledger-zone">
      <RoomLedger
        class="ledger-host"
        :logs="recentLogs"
        :nickname-map="nicknameMap"
        scrollable
        @more="openLedgerPopup"
      />
    </view>

    <!-- 筹码轨 -->
    <view class="rail">
      <wd-button
        variant="text"
        open-type="contact"
        custom-style="display: block; padding: 0; height: auto; line-height: 1; background: transparent;"
      >
        <view class="side">
          <wd-icon
            css-icon
            class="ico"
            name="i-carbon-headset"
          />
          <text>客服</text>
        </view>
      </wd-button>
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
        v-if="canSettle"
        class="side"
        @click="goSettle"
      >
        <wd-icon
          css-icon
          class="ico"
          name="i-carbon-flag"
        />
        <text>结束牌局</text>
      </view>
      <view
        class="side"
        @click="onMineClick"
      >
        <wd-icon
          css-icon
          class="ico"
          name="i-carbon-user"
        />
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

    <!-- 茶饭设置弹层（设计 09；非房主/已结算只读） -->
    <TeaConfigSheet
      v-model:show="showTeaConfig"
      :snapshot="snapshot"
      :my-user-id="myUserId"
    />

    <!-- 邀请弹层（设计 03；房号标题/••• 入口，转发 onShareAppMessage） -->
    <QrInvitePanel
      v-model:show="showQrInvite"
      :room-id="currentRoomId"
      :room-code="snapshot?.room_code ?? ''"
    />

    <!-- 头像弹窗（设计 10；「我的」入口） -->
    <AvatarPopup
      v-model:show="showAvatarPopup"
      :room-id="currentRoomId"
    />
  </view>
</template>

<style lang="scss" scoped>
.room-page {
  box-sizing: border-box;
  display: flex;
  flex-direction: column;

  /* 定高：H5 页面根内容驱动高度时 flex 兜底失效（MP 页面容器自带定高，H5 必须显式给） */
  height: 100vh;
  min-height: 100vh;
  padding: calc(var(--status-bar-h, env(safe-area-inset-top, 0px)) + 6px) 0 0;
  color: var(--on-ink);
  background: var(--ink-900);
  background-image: var(--spot);
}

/* ── 自绘 nav ── */
.nav {
  display: flex;
  flex: none;
  align-items: center;
  justify-content: space-between;
  padding: 6px 16px 8px;

  /* 小程序：右侧让位微信真胶囊 */
  padding-right: var(--nav-right-pad, 16px);
}

.nav .side {
  display: flex;
  align-items: center;
  min-width: 60px;
}

.nav .side.right {
  justify-content: flex-end;
}

.navicon {
  margin-right: 8px;
  font-family: 'Noto Serif SC', serif;
  font-size: 20px;
  line-height: 1;
  color: var(--brass);
}

.ntitle {
  font-family: 'Noto Serif SC', serif;
  font-size: 17px;
  font-weight: 500;
  color: var(--on-ink);
  text-align: center;
  letter-spacing: 0.06em;
}

.nsub {
  display: block;
  margin-top: 2px;
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 10.5px;
  font-weight: 400;
  color: var(--dim);
  letter-spacing: 0.02em;
}

/* 胶囊占位（装饰，对齐源稿右上 capsule） */
.capsule {
  display: flex;
  align-items: center;
  width: 88px;
  height: 32px;
  background: var(--on-ink-a04);
  border: 1px solid var(--on-ink-a28);
  border-radius: 999px;
}

.cap-dots {
  flex: 1;
  font-size: 14px;
  line-height: 1;
  color: var(--on-ink);
  text-align: center;
  letter-spacing: 1.5px;
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
  content: '';
  background: var(--on-ink);
  border-radius: 50%;
}

/* ── 牌桌 ── */
.table {
  position: relative;
  flex: none;
  height: 252px;
  padding: 14px 0 0;
  margin: 12px 14px 0;
  background: radial-gradient(85% 65% at 50% 38%, var(--felt-hi) 0%, var(--felt) 55%, var(--felt-deep) 100%);
  border: 1px solid var(--brass-a50);
  border-radius: 44px;
  box-shadow: var(--shadow-tile), inset 0 0 0 2px var(--brass-a50);
}

/* 账板占位区：吃满牌桌与筹码轨之间剩余空间（flex 链 min-height:0 保证可压缩） */
.ledger-zone {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
}

/* 组件 host 节点（<RoomLedger class="ledger-host"> 的 class 落在 MP host 上）：
   必须自身成为 flex 列容器并可收缩，内部 .ledger.fill / scroll-view 才拿得到定高 */
.ledger-host {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
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
  content: '';
  background: linear-gradient(90deg, transparent, var(--brass-a50), transparent);
}

/* 中央堆叠：最近一手 + 茶饭池铭牌 */
.center-stack {
  position: absolute;
  bottom: 18px;
  left: 50%;
  display: flex;
  flex-direction: column;
  gap: 9px;
  align-items: center;
  width: 300px;
  transform: translateX(-50%);
}

.center-flow {
  width: 290px;
  padding: 12px 14px 10px;
  text-align: center;
  background: var(--felt-deep);
  border: 1.5px solid var(--brass-a50);
  border-radius: 14px;
  box-shadow: 0 10px 26px var(--felt-deep);
}

.center-flow .cf {
  display: flex;
  flex-wrap: wrap;
  gap: 2px;
  align-items: center;
  justify-content: center;
  font-size: 13.5px;
  color: var(--on-ink);
}

.center-flow .cf .amt-p,
.center-flow .cf .amt-w {
  font-size: 15px;
}

.center-flow .arr {
  font-size: 11px;
  color: var(--dim);
}

.center-flow .cap {
  font-size: 12.5px;
  color: var(--dim);
}

.center-flow .t {
  display: block;
  margin-top: 5px;
  font-size: 10px;
  color: var(--dim);
}

/* 茶饭池铭牌 */
.base-pot {
  display: flex;
  gap: 10px;
  align-items: center;
  padding: 7px 18px;
  cursor: pointer;
  background: var(--felt-deep);
  border: 1px solid var(--brass-a50);
  border-radius: 999px;
}

.base-pot .bp {
  display: flex;
  gap: 3px;
  align-items: center;
  font-size: 11px;
  color: var(--dim);
  letter-spacing: 0.1em;
}

.base-pot .bp-icon {
  font-size: 12px;
  line-height: 1;
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
  z-index: 4;
  display: flex;
  flex: none;
  gap: 30px;
  align-items: flex-start;
  justify-content: center;
  padding: 14px 0 calc(env(safe-area-inset-bottom, 0px) + 16px);
  background: linear-gradient(180deg, transparent, var(--ink-900) 34%);
}

.rail .side {
  display: flex;
  flex-direction: column;
  gap: 5px;
  align-items: center;
  padding-top: 6px;
  font-size: 11px;
  color: var(--dim);
  cursor: pointer;
}

.rail .side .ico {
  font-size: 18px;
  line-height: 1;
}

.rail .main {
  display: flex;
  flex-direction: column;
  gap: 6px;
  align-items: center;
  cursor: pointer;
}

.rail .main .bigchip {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 84px;
  height: 84px;
  background: repeating-conic-gradient(from 8deg, var(--brass-lo) 0 18deg, var(--brass) 18deg 36deg);
  border-radius: 50%;
  box-shadow: 0 5px 0 var(--brass-lo), 0 14px 26px var(--ink-900);
}

.rail .main .bigchip::before {
  position: absolute;
  inset: 8px;
  content: '';
  background: radial-gradient(circle at 50% 36%, var(--brass-hi), var(--brass) 78%);
  border-radius: 50%;
}

.rail .main .bigchip text {
  position: relative;
  z-index: 1;
  font-size: 19px;
  font-weight: 700;
  color: var(--ink-900);
  letter-spacing: 0.05em;
}

.rail .main .cap2 {
  font-size: 11px;
  color: var(--dim);
}

/* ── 全部流水弹层 ── */
.ledger-popup {
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  padding: 18px 0 calc(env(safe-area-inset-bottom, 0px) + 8px);
}

.lp-title {
  padding: 0 20px;
  font-family: 'Noto Serif SC', serif;
  font-size: 17px;
  font-weight: 700;
  color: var(--on-ink);
  letter-spacing: 0.05em;
}

.lp-cap {
  padding: 4px 20px 10px;
  font-size: 11px;
  color: var(--dim);
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
  font-size: 11px;
  color: var(--dim);
  text-align: center;
}
</style>
