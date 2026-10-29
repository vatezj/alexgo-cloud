<script setup lang="ts">
/**
 * 结算页 · 竖排开牌（视觉源：desgin/preview/miniapp-breakout.html 05 结算；Task 9 整体重写占位页）
 *
 * - 排行大牌：members 按 cur_score 降序，第一名骨白大牌 + 黄铜描边（win-card）；
 *   竖排「战斗胜利」标题；全零/空态「本局无胜负」
 * - IOU：calcIou 贪心简化（utils/scorecard/iou.ts 纯函数）+ 「策略参考，可不按」
 * - 操作托盘：直接退房（二次确认 leave）/ 房间结算单 / 总分排行榜（同成员列表弹层）/
 *   房间流水（logs 分页弹层）/ 历史房间 → records / 使用手册 → manual
 * - 房主（creator_id==我 且 status==0）：「结束牌局」→ dialog 二次确认 → settle → 刷新快照
 * - status!=0：隐藏退房/结束牌局等可写操作，只读展示
 * - 轮询：onShow refresh（错误本页 toast）+ start；onHide/onUnload stop
 * - 业务错误本页 toast（ApiError 非 401/403）；401 由 handlers 全局清态跳登录
 * - 色值零硬编码：全走主题 token 与全局工具类（.comply/.num/.win-n/.lose-n/.btn.brass/.pcard）
 * - 文案红线：全页文案只允许记分/结算词汇（锁定合规语除外）
 */
import type { RoomLogItem } from '@/api/scorecard/types'
import { ApiError } from '@/api/core/handlers'
import { logs as fetchLogs, leave, settle } from '@/api/scorecard/room'
import ComplianceBanner from '@/components/scorecard/ComplianceBanner.vue'
import PlayCard from '@/components/scorecard/PlayCard.vue'
import RoomLedger from '@/components/scorecard/RoomLedger.vue'
import ScoreText from '@/components/scorecard/ScoreText.vue'
import { useRoomPolling } from '@/composables/scorecard/useRoomPolling'
import { useSystemBar } from '@/composables/useSystemBar'
import { useScorecardAuthStore } from '@/store/scorecard/auth'
import { useScorecardRoomStore } from '@/store/scorecard/room'
import { calcIou } from '@/utils/scorecard/iou'

definePage({
  style: {
    navigationStyle: 'custom',
  },
})

const router = useRouter()
const toast = useToast()
// 必须带 'globalDialog' selector，共享 ref 由 App.ku.vue（global-ku-root）provide：
// 裸 useDialog() inject 不到该 key 会自建孤儿 ref，confirm 永不弹且 promise 挂起。
const dialog = useDialog('globalDialog')
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
const members = computed(() => snapshot.value?.members ?? [])
const status = computed(() => snapshot.value?.status ?? 0)
const roomCodeText = computed(() => {
  const code = snapshot.value?.room_code
  return code ? `${code.toUpperCase()} 房间` : '房间'
})
const roundText = computed(() => `${roomCodeText.value} · ${snapshot.value?.round_no ?? 0} 局`)

/** 房主且本局仍进行中：快照在场 + status===0 + creator 是我（快照未载入 fail-closed 不渲染）。 */
const canSettle = computed(() => {
  const snap = snapshot.value
  return !!snap && snap.status === 0 && myUserId.value > 0 && snap.creator_id === myUserId.value
})
/** 进行中可退房（任何身份；房主退房后端自动结算）：快照在场且 status===0（fail-closed）。 */
const canLeave = computed(() => {
  const snap = snapshot.value
  return !!snap && snap.status === 0
})

const statusText = computed(() => {
  if (status.value === 0)
    return '未结算'
  if (status.value === 1)
    return '已结算'
  if (status.value === 2)
    return '已关闭'
  return '对局结束'
})

/** 排行：cur_score 降序（第一名上大牌）。 */
const sortedMembers = computed(() => [...members.value].sort((a, b) => b.cur_score - a.cur_score))
/** 头名（须为正分；全零/空态无头名 → 「本局无胜负」）。 */
const topMember = computed(() => sortedMembers.value.find(m => m.cur_score > 0) ?? null)

/** user_id → 昵称（IOU 行 / 流水弹层共用）。 */
const nicknameMap = computed<Record<number, string>>(() => {
  const map: Record<number, string> = {}
  for (const m of members.value)
    map[m.user_id] = m.nickname
  return map
})

/** IOU 行（贪心简化 + 昵称/首字预取，供模板直用）。 */
const iouRows = computed(() =>
  calcIou(members.value).map(edge => ({
    ...edge,
    fromName: nicknameMap.value[edge.from_user_id] || '牌友',
    toName: nicknameMap.value[edge.to_user_id] || '牌友',
    fromInitial: (nicknameMap.value[edge.from_user_id] || '牌').slice(0, 1).toUpperCase(),
  })),
)

/** 业务信封错误本页提示；401/403（handlers 已清态全局提示）不重复 toast。 */
function toastApiError(err: unknown) {
  if (err instanceof ApiError && err.code !== 401 && err.code !== 403)
    toast.error({ msg: err.message || '请求失败' })
}

/* ── 导航 ── */

function goBack() {
  if (getCurrentPages().length > 1)
    uni.navigateBack()
  else
    router.replaceAll({ path: '/pages/index/index' })
}

/** 历史房间 → 战绩流水页（Task 10 落地前为占位页）。 */
function goRecords() {
  router.push({ path: '/pages/records/index' })
}

/** 使用手册页。 */
function goManual() {
  router.push({ path: '/pages/manual/index' })
}

/* ── 可写操作（status==0 才渲染） ── */

const acting = ref(false)

/** 直接退房：二次确认 → leave → 清快照回首页（房主退房后端自动结算）。 */
async function onLeave() {
  if (acting.value)
    return
  // 锁在 confirm 之前：弹窗停留期间再点不重入（R1 Minor 2）
  acting.value = true
  try {
    try {
      await dialog.confirm({
        title: '退房确认',
        msg: '确认退出本房间？房主退房将自动结束本局。',
        confirmButtonText: '确认退房',
        cancelButtonText: '再想想',
      })
    }
    catch {
      return
    }
    await leave(currentRoomId.value)
    roomStore.clearSnapshot()
    toast.success('已退房')
    router.replaceAll({ path: '/pages/index/index' })
  }
  catch (err) {
    toastApiError(err)
  }
  finally {
    acting.value = false
  }
}

/** 结束牌局（仅房主）：二次确认 → settle → refresh 拉新快照（status 变 1，页面转只读）。 */
async function onSettle() {
  if (acting.value)
    return
  // 锁在 confirm 之前：弹窗停留期间再点不重入（R1 Minor 2）
  acting.value = true
  try {
    try {
      await dialog.confirm({
        title: '结束牌局',
        msg: '结束后本局转为已结算、只读展示。确认结束？',
        confirmButtonText: '结束牌局',
        cancelButtonText: '再想想',
      })
    }
    catch {
      return
    }
    await settle(currentRoomId.value)
    await polling.refresh()
    toast.success('已结算')
    // Task 13 接线点：结算胜利 entertain 播报（可选，后续接 useVoice）——
    // speak(`${topMember.value?.nickname || ''}战斗胜利`, 'entertain')
  }
  catch (err) {
    toastApiError(err)
  }
  finally {
    acting.value = false
  }
}

/* ── 成员分数列表弹层（结算单 / 总分排行榜共用） ── */

const showScorePopup = ref(false)
const scorePopupTitle = ref('房间结算单')

function openSettleSheet() {
  scorePopupTitle.value = '房间结算单'
  showScorePopup.value = true
}

function openRanking() {
  scorePopupTitle.value = '总分排行榜'
  showScorePopup.value = true
}

/* ── 房间流水弹层（分页上拉，同房间页） ── */

const LOG_PAGE_SIZE = 20

const showLogsPopup = ref(false)
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

function openLogsPopup() {
  showLogsPopup.value = true
  void loadPopupLogs(false)
}

/** scroll-view 触底：还有下一页则续拉。 */
function onPopupScrollEnd() {
  if (popupLogs.value.length < popupTotal.value)
    void loadPopupLogs(true)
}

/* ── 生命周期 ── */

onShow(() => {
  if (!currentRoomId.value) {
    toast.error('房间信息缺失')
    router.replaceAll({ path: '/pages/index/index' })
    return
  }
  roomStore.roomId = currentRoomId.value
  if (!authStore.user)
    void authStore.fetchMe().catch(() => {})
  // 快照兜底：无论 store 是否已有本房快照，onShow 都立即拉一次（H5 直达/刷新场景）
  void polling.refresh().catch(toastApiError)
  polling.start()
})

onHide(() => {
  polling.stop()
})

onUnload(() => {
  polling.stop()
})
</script>

<template>
  <view
    class="settle-page"
    :style="pageStyle"
  >
    <!-- 自绘 nav -->
    <view class="nav">
      <view
        class="side"
        @click="goBack"
      >
        <wd-icon
          css-icon
          class="navicon"
          name="i-carbon-chevron-left"
        />
      </view>
      <view class="ntitle">
        结算
        <text class="nsub">
          {{ roundText }}
        </text>
      </view>
      <view class="side right">
        <!-- #ifndef MP-WEIXIN -->
        <view class="capsule">
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

    <!-- 排行大牌：竖排「战斗胜利」+ 头名骨白大牌；全零/空态「本局无胜负」 -->
    <view class="hero">
      <template v-if="topMember">
        <text class="vtitle">
          战斗胜利
        </text>
        <view class="hero-stack">
          <view class="hero-ghost g1" />
          <view class="hero-ghost g2" />
          <PlayCard
            class="hero-card"
            suit="♥"
            rank="A"
            tone="win"
          >
            <view class="wc-in">
              <text class="wc-k">
                总 计
              </text>
              <ScoreText
                class="wc-v"
                :value="topMember.cur_score"
                tone="win"
              />
              <text class="wc-t">
                {{ topMember.nickname || '牌友' }}
              </text>
            </view>
          </PlayCard>
        </view>
      </template>
      <view
        v-else
        class="hero-empty"
      >
        <text class="he-t">
          本局无胜负
        </text>
        <text class="he-s">
          牌友们握手言和
        </text>
      </view>
    </view>

    <!-- IOU：贪心简化行 + 「策略参考，可不按」 -->
    <view class="iou-sec">
      <text class="st">
        {{ statusText }}
      </text>
      <view
        v-for="(row, idx) in iouRows"
        :key="`${row.from_user_id}-${row.to_user_id}-${idx}`"
        class="iou"
      >
        <view class="uav">
          <text class="uav-t">
            {{ row.fromInitial }}
          </text>
        </view>
        <view class="unm-wrap">
          <text class="unm">
            {{ row.fromName }} → {{ row.toName }}
          </text>
          <text class="us">
            记给 {{ row.toName }} · 线下自行处理
          </text>
        </view>
        <ScoreText
          class="uv"
          :value="row.amount"
          tone="lose"
          :signed="false"
        />
      </view>
      <text
        v-if="!iouRows.length"
        class="iou-empty"
      >
        本局无未结记分
      </text>
      <text class="iou-cap">
        策略参考，可不按
      </text>
    </view>

    <view class="grow" />

    <!-- 操作区 -->
    <view class="actions">
      <button
        v-if="canSettle"
        class="btn brass settle-btn"
        :disabled="acting"
        @click="onSettle"
      >
        结束牌局
      </button>
      <view class="tray">
        <text class="tray-k">
          操作托盘
        </text>
        <button
          v-if="canLeave"
          class="btn ghost"
          :disabled="acting"
          @click="onLeave"
        >
          直接退房
        </button>
        <button
          class="btn ghost"
          @click="openSettleSheet"
        >
          房间结算单
        </button>
        <button
          class="btn ghost"
          @click="openRanking"
        >
          总分排行榜
        </button>
        <button
          class="btn ghost"
          @click="openLogsPopup"
        >
          房间流水
        </button>
        <button
          class="btn ghost"
          @click="goRecords"
        >
          历史房间
        </button>
        <button
          class="btn ghost"
          @click="goManual"
        >
          使用手册
        </button>
      </view>
      <text class="cap">
        只记分数，不涉及金钱
      </text>
    </view>

    <!-- 成员分数列表弹层（房间结算单 / 总分排行榜共用） -->
    <wd-popup
      v-model="showScorePopup"
      position="bottom"
      closable
      :safe-area-inset-bottom="true"
      custom-style="border-radius: 20px 20px 0 0; background: var(--ink-700);"
    >
      <view class="list-popup">
        <text class="lp-title">
          {{ scorePopupTitle }}
        </text>
        <text class="lp-cap">
          共 {{ sortedMembers.length }} 人 · 按本局分数排序
        </text>
        <scroll-view
          class="lp-scroll"
          scroll-y
        >
          <view
            v-for="(m, idx) in sortedMembers"
            :key="m.user_id"
            class="rank-row"
          >
            <text class="rk num">
              {{ idx + 1 }}
            </text>
            <view class="rav">
              <image
                v-if="m.avatar"
                class="rav-img"
                :src="m.avatar"
                mode="aspectFill"
              />
              <text
                v-else
                class="rav-t"
              >
                {{ (m.nickname || '牌').slice(0, 1).toUpperCase() }}
              </text>
            </view>
            <view class="rnm-wrap">
              <text class="rnm">
                {{ m.nickname || '牌友' }}
                <text
                  v-if="m.user_id === myUserId"
                  class="is-me"
                >
                  我
                </text>
                <text
                  v-if="m.is_creator"
                  class="badge"
                >
                  房主
                </text>
              </text>
            </view>
            <ScoreText
              :value="m.cur_score"
              :tone="m.cur_score > 0 ? 'win' : (m.cur_score < 0 ? 'lose' : 'none')"
            />
          </view>
          <text
            v-if="!sortedMembers.length"
            class="lp-foot"
          >
            暂无成员
          </text>
        </scroll-view>
      </view>
    </wd-popup>

    <!-- 房间流水弹层（分页上拉） -->
    <wd-popup
      v-model="showLogsPopup"
      position="bottom"
      closable
      :safe-area-inset-bottom="true"
      custom-style="border-radius: 20px 20px 0 0; background: var(--ink-700);"
    >
      <view class="ledger-popup">
        <text class="lp-title">
          房间流水
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
.settle-page {
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
  min-height: 100vh;
  padding: calc(var(--status-bar-h, env(safe-area-inset-top, 0px)) + 6px) 0 0;
  background: var(--ink-900);
  background-image: var(--spot);
  color: var(--on-ink);
}

/* ── 自绘 nav（同房间页） ── */
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
  font-size: 26px;
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

/* ── 排行大牌（设计 05 settle-hero） ── */
.hero {
  display: flex;
  flex: none;
  align-items: center;
  justify-content: center;
  gap: 18px;
  padding-top: 12px;
}

.vtitle {
  color: var(--brass);
  font-family: 'Noto Serif SC', serif;
  font-size: 42px;
  font-weight: 700;
  letter-spacing: 0.34em;
  text-shadow: 0 4px 24px var(--brass-a50);
  writing-mode: vertical-rl;
}

.hero-stack {
  position: relative;
  width: 220px;
  height: 276px;
}

.hero-ghost {
  position: absolute;
  top: 0;
  left: 0;
  width: 206px;
  height: 268px;
  border-radius: 14px;
  box-shadow: 0 10px 24px var(--ink-900);
}

.hero-ghost.g1 {
  top: 4px;
  left: -4px;
  background: var(--tile-emboss);
  transform: rotate(-8deg);
}

.hero-ghost.g2 {
  top: 2px;
  left: 14px;
  background: var(--tile-shade);
  transform: rotate(6deg);
}

/* 头名 win-card：骨白大牌 + 黄铜描边（class 直落 PlayCard 根节点） */
.hero-card {
  position: absolute;
  top: 0;
  left: 7px;
  width: 206px;
  height: 268px;
  border: 1.5px solid var(--brass);
  transform: rotate(-1.5deg);
}

.wc-in {
  padding: 44px 18px 20px;
  text-align: center;
}

.wc-k {
  display: block;
  color: var(--dim);
  font-size: 12px;
  letter-spacing: 0.2em;
}

.wc-v {
  display: block;
  margin-top: 4px;
  font-size: 56px;
  font-weight: 700;
  line-height: 1.05;
}

.wc-t {
  display: block;
  margin-top: 16px;
  color: var(--jet);
  font-family: 'Noto Serif SC', serif;
  font-size: 15px;
  font-weight: 700;
  letter-spacing: 0.06em;
}

/* 全零 / 空态 */
.hero-empty {
  display: flex;
  width: 220px;
  height: 276px;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  border: 1.5px dashed var(--on-ink-a28);
  border-radius: 20px;
  background: var(--on-ink-a04);
}

.he-t {
  color: var(--on-ink);
  font-family: 'Noto Serif SC', serif;
  font-size: 21px;
  font-weight: 700;
  letter-spacing: 0.14em;
}

.he-s {
  color: var(--dim);
  font-size: 12px;
}

/* ── IOU 区（设计 05 unsec） ── */
.iou-sec {
  flex: none;
  padding: 12px 20px 0;
}

.iou-sec .st {
  display: block;
  margin-bottom: 11px;
  color: var(--on-ink);
  font-family: 'Noto Serif SC', serif;
  font-size: 16px;
  font-weight: 700;
  letter-spacing: 0.14em;
  text-align: center;
}

.iou {
  position: relative;
  display: flex;
  align-items: center;
  gap: 11px;
  margin-bottom: 8px;
  padding: 12px 14px;
  border-radius: 12px;
  background: var(--tile);
  box-shadow: var(--shadow-tile);
  color: var(--jet);
}

.iou::before {
  position: absolute;
  top: 8px;
  bottom: 8px;
  left: 44px;
  border-left: 1.5px dashed var(--tile-emboss);
  content: '';
}

.iou .uav {
  display: flex;
  width: 34px;
  height: 34px;
  flex: none;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: var(--ink-600);
  overflow: hidden;
}

.iou .uav-t {
  color: var(--tile-shade);
  font-size: 13px;
}

.iou .unm-wrap {
  min-width: 0;
  flex: 1;
  padding-left: 8px;
}

.iou .unm {
  display: block;
  overflow: hidden;
  font-size: 14px;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.iou .us {
  display: block;
  margin-top: 2px;
  color: var(--dim);
  font-size: 11px;
  font-weight: 400;
}

.iou .uv {
  flex: none;
  margin-left: auto;
  font-size: 21px;
}

.iou-empty {
  display: block;
  padding: 10px 0 2px;
  color: var(--dim);
  font-size: 12px;
  text-align: center;
}

.iou-cap {
  display: block;
  margin-top: 9px;
  color: var(--dim);
  font-size: 12px;
  text-align: center;
}

.grow {
  flex: 1;
}

/* ── 操作区 ── */
.actions {
  flex: none;
  padding: 10px 20px calc(env(safe-area-inset-bottom, 0px) + 16px);
}

.settle-btn {
  margin-bottom: 10px;
}

.settle-btn[disabled] {
  background: var(--on-ink-a04);
  border: 1px solid var(--on-ink-a28);
  box-shadow: none;
  color: var(--dim);
  opacity: 1;
}

.tray {
  position: relative;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  padding: 16px 13px 13px;
  border: 1.5px dashed var(--brass-a50);
  border-radius: 20px;
}

.tray-k {
  position: absolute;
  top: -9px;
  left: 50%;
  padding: 0 10px;
  background: var(--ink-900);
  color: var(--brass);
  font-size: 10px;
  letter-spacing: 0.2em;
  transform: translateX(-50%);
}

.tray .btn.ghost {
  height: 44px;
  font-size: 13.5px;
}

.tray .btn.ghost[disabled] {
  opacity: 0.5;
}

.actions .cap {
  display: block;
  margin-top: 10px;
  color: var(--dim);
  font-size: 11px;
  text-align: center;
}

/* ── 弹层通用（结算单 / 排行榜 / 流水） ── */
.list-popup,
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

/* 成员分数行 */
.rank-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 11px 20px;
  border-bottom: 1px solid var(--on-ink-a04);
}

.rank-row .rk {
  width: 22px;
  flex: none;
  color: var(--dim);
  font-size: 15px;
  text-align: center;
}

.rank-row:first-of-type .rk {
  color: var(--win-dark);
  font-weight: 700;
}

.rav {
  display: flex;
  width: 34px;
  height: 34px;
  flex: none;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: var(--ink-600);
  overflow: hidden;
}

.rav-img {
  width: 100%;
  height: 100%;
  border-radius: 50%;
}

.rav-t {
  color: var(--tile-shade);
  font-size: 13px;
}

.rnm-wrap {
  min-width: 0;
  flex: 1;
}

.rnm {
  display: block;
  overflow: hidden;
  color: var(--on-ink);
  font-size: 14px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 「我」标：类名用 is-me 避开全局 UnoCSS 工具类 .me（margin-inline-end:32rpx 会把房主标推开 16px） */
.rnm .is-me {
  margin-left: 6px;
  color: var(--brass);
  font-size: 11px;
}

.rnm .badge {
  margin-left: 6px;
  color: var(--jade-hi);
  font-size: 10px;
  letter-spacing: 0.06em;
}
</style>
