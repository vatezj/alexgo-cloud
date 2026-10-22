<script setup lang="ts">
/**
 * 首页 · 扇面牌桌（视觉源：desgin/preview/miniapp-breakout.html 01 首屏）
 *
 * - 身份牌：authStore.user（昵称/头像/注册天数）+ stat 总分/胜率 + listFriends 好友数
 * - 扇形 CTA：我要开房 createRoom（默认配置全 0）→ 进房间页；扫码进房 mp=uni.scanCode 解析 qr= 房号 / H5=wd-popup 输房号
 * - scene 进房：onShow 时已登录且 pendingScene 有值 → consumeScene 只消费一次 → joinRoom 进房（T5 afterLogin 消费点的已登录侧）
 * - 未登录持有 scene：先去登录（scene 暂存保留，登录后 afterLogin 消费进房）；未登录点开房 → 跳登录（已知简化：登录后回首页，不回跳原动作）
 * - 业务错误 catch → 本页 toast（ApiError 非 401/403）；数据拉取失败降级空态，不阻塞渲染
 * - 色值零硬编码：全走主题 token 与全局工具类（.pcard/.btn.brass/.num/.win-n/.lose-n）
 */
import type { RecordItem, StatResult } from '@/api/scorecard/types'
import { ApiError } from '@/api/core/handlers'
import { listFriends } from '@/api/scorecard/friend'
import { myRecords, stat } from '@/api/scorecard/record'
import { createRoom, joinRoom } from '@/api/scorecard/room'
import PlayCard from '@/components/scorecard/PlayCard.vue'
import { useSceneRoom } from '@/composables/scorecard/useSceneRoom'

definePage({
  style: {
    navigationStyle: 'custom',
  },
})

const authStore = useScorecardAuthStore()
const router = useRouter()
const toast = useToast()
const { pendingScene, consumeScene } = useSceneRoom()

const statData = ref<StatResult | null>(null)
const friendCount = ref(0)
const recentRecords = ref<RecordItem[]>([])
const loadingData = ref(false)
const creating = ref(false)
const joining = ref(false)
const showJoinPopup = ref(false)
const joinCode = ref('')

const isLoggedIn = computed(() => Boolean(authStore.token))

const nickname = computed(() => {
  if (authStore.user?.nickname)
    return authStore.user.nickname
  return isLoggedIn.value ? '牌友' : '未登录'
})

/** 头像兜底：取昵称首字（无昵称用「牌」） */
const initial = computed(() => (nickname.value.slice(0, 1) || '牌').toUpperCase())

const avatarUrl = computed(() => authStore.user?.avatar || '')

/** 注册天数（牌龄）：me.created_at → 自然日下取整，至少 1 天；资料缺失返回 null */
const memberDays = computed(() => {
  const created = authStore.user?.created_at
  if (!created)
    return null
  const ts = new Date(created).getTime()
  if (Number.isNaN(ts))
    return null
  return Math.max(1, Math.floor((Date.now() - ts) / 86400000))
})

const subLine = computed(() => {
  if (!isLoggedIn.value)
    return '登录后开房记账 · 好友互通'
  const parts: string[] = []
  if (memberDays.value != null)
    parts.push(`牌龄 ${memberDays.value} 天`)
  parts.push(`常驻桌友 ${friendCount.value} 人`)
  return parts.join(' · ')
})

/** win_rate 是 0–1 比值（后端 (win)/(win+lose)）→ 百分比整数文案 */
const winRateText = computed(() =>
  statData.value ? `${Math.round(statData.value.win_rate * 100)}%` : '—',
)

const totalScoreText = computed(() =>
  statData.value ? formatDelta(statData.value.total_score) : '—',
)

const winCountText = computed(() => (statData.value ? `${statData.value.win}次` : '—'))
const loseCountText = computed(() => (statData.value ? `${statData.value.lose}次` : '—'))

const emptyRecordsText = computed(() => {
  if (loadingData.value)
    return '加载中…'
  if (!isLoggedIn.value)
    return '登录后查看最近战绩'
  return '暂无战绩 · 开一桌试试'
})

/** 结算类型映射文案（RecordItem.settle_type 契约：1 正常 2 房主退房自动结算 3 退房 4 零数据空战绩） */
const SETTLE_TEXT: Record<number, string> = {
  1: '正常结算',
  2: '房主退房自动结算',
  3: '退房',
  4: '零数据空战绩',
}

function settleText(type: number): string {
  return SETTLE_TEXT[type] ?? '结算'
}

/** RFC3339 → M月D日（本地时区） */
function recordDate(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime()))
    return ''
  return `${d.getMonth() + 1}月${d.getDate()}日`
}

/** 分数差文案：正 +N；负用减号 U+2212 对齐源稿 */
function formatDelta(score: number): string {
  if (score > 0)
    return `+${score}`
  if (score < 0)
    return `−${Math.abs(score)}`
  return '0'
}

/** 业务信封错误本页提示；401/403（handlers 已清态全局提示）不重复 toast。 */
function toastApiError(err: unknown) {
  if (err instanceof ApiError && err.code !== 401 && err.code !== 403)
    toast.error({ msg: err.message || '请求失败' })
}

/** uni 扫码 fail 载荷兜底取文案（可能是 {errMsg} 对象而非 Error） */
function errText(err: unknown): string {
  if (typeof err === 'string')
    return err
  if (err && typeof err === 'object') {
    const e = err as { errMsg?: string, message?: string }
    return e.errMsg || e.message || ''
  }
  return ''
}

/**
 * 从扫码结果提取 qr= 房号：
 * 小程序码 path 形如 `pages/index/index?scene=qr%3D7AVW`（scene 前缀 qr= 由后端拼），
 * 部分端直接给 scene 字段/原文。识别不出返回 null。
 */
function pickQrFrom(input?: string): string | null {
  if (!input)
    return null
  let s = input
  try {
    s = decodeURIComponent(input)
  }
  catch {
    // 非法百分号编码：按明文继续尝试解析
  }
  const idx = s.indexOf('qr=')
  if (idx === -1)
    return null
  const code = s.slice(idx + 3).split(/[&#/?\s]/)[0]?.trim() ?? ''
  return code || null
}

/** 首页数据：stat（总分/胜率/胜负次数）+ listFriends（好友数）+ myRecords（最近战绩）。失败降级空态。 */
async function loadHomeData() {
  if (!isLoggedIn.value) {
    statData.value = null
    friendCount.value = 0
    recentRecords.value = []
    return
  }
  loadingData.value = true
  try {
    await authStore.fetchMe()
  }
  catch {
    // 资料刷新失败不阻断（persist 缓存兜底；401 由 handlers 清态跳登录）
  }
  const [s, f, r] = await Promise.allSettled([
    stat(),
    listFriends(),
    myRecords({ page: 1, size: 5 }),
  ])
  if (s.status === 'fulfilled')
    statData.value = s.value
  if (f.status === 'fulfilled')
    friendCount.value = f.value.length
  if (r.status === 'fulfilled')
    recentRecords.value = r.value.list
  const failed = [s, f, r].find(item => item.status === 'rejected')
  if (failed && failed.status === 'rejected')
    toastApiError(failed.reason)
  loadingData.value = false
}

/** scene 进房（已登录侧）：pendingScene 有值才消费（只消费一次，与 T5 afterLogin 互斥不双进房）。 */
async function consumePendingScene() {
  if (!isLoggedIn.value)
    return
  const code = consumeScene()
  if (!code)
    return
  try {
    const snap = await joinRoom({ room_code: code })
    authStore.lastRoomId = snap.id
    router.push({ path: '/pages/room/index', query: { roomId: String(snap.id) } })
  }
  catch (err) {
    toastApiError(err)
  }
}

onShow(() => {
  void consumePendingScene()
  void loadHomeData()
})

/** 未登录统一去登录（已知简化：登录后回首页，不做原动作回跳）。 */
function ensureLogin(): boolean {
  if (isLoggedIn.value)
    return true
  router.push({ path: '/pages/login/index' })
  return false
}

function goLogin() {
  router.push({ path: '/pages/login/index' })
}

/** 我要开房：默认配置全 0（brief 逐字）→ 进房间页。 */
async function onCreateRoom() {
  if (creating.value || joining.value)
    return
  if (!ensureLogin())
    return
  creating.value = true
  try {
    const snap = await createRoom({
      tea_percent: 0,
      tea_target_amount: 0,
      tea_auto_enabled: false,
      max_lose_score: 0,
    })
    authStore.lastRoomId = snap.id
    router.push({ path: '/pages/room/index', query: { roomId: String(snap.id) } })
  }
  catch (err) {
    toastApiError(err)
  }
  finally {
    creating.value = false
  }
}

/** 按房号进房：已登录直接 joinRoom；未登录暂存 scene 去登录（afterLogin 消费进房）。 */
async function enterRoomByCode(code: string) {
  if (!isLoggedIn.value) {
    pendingScene.value = code
    showJoinPopup.value = false
    router.push({ path: '/pages/login/index' })
    return
  }
  joining.value = true
  try {
    const snap = await joinRoom({ room_code: code })
    authStore.lastRoomId = snap.id
    showJoinPopup.value = false
    router.push({ path: '/pages/room/index', query: { roomId: String(snap.id) } })
  }
  catch (err) {
    toastApiError(err)
  }
  finally {
    joining.value = false
  }
}

/** 扫码进房（mp）：uni.scanCode → path/scene/result 解析 qr= 房号；识别不出 toast「无效二维码」。 */
async function scanAndJoin() {
  if (creating.value || joining.value)
    return
  try {
    // #ifdef MP-WEIXIN
    const res = await uni.scanCode({ onlyFromCamera: true }) as UniNamespace.ScanCodeSuccessRes & { scene?: string }
    const code = pickQrFrom(res.path) ?? pickQrFrom(res.scene) ?? pickQrFrom(res.result)
    if (!code) {
      toast.error('无效二维码')
      return
    }
    await enterRoomByCode(code)
    // #endif
    // #ifndef MP-WEIXIN
    toast.error('请在微信小程序中扫码')
    // #endif
  }
  catch (err) {
    // enterRoomByCode 内部自 catch 不外抛：这里只会是 scanCode 本身的失败/取消
    const msg = errText(err)
    if (msg.includes('cancel'))
      return
    toast.error('扫码失败，请重试')
  }
}

/** 扫码进房入口：mp 走扫码；H5 弹房号输入（wd-popup）。 */
function onScanJoin() {
  if (creating.value || joining.value)
    return
  // #ifdef MP-WEIXIN
  void scanAndJoin()
  // #endif
  // #ifndef MP-WEIXIN
  joinCode.value = ''
  showJoinPopup.value = true
  // #endif
}

/** H5 弹层确认：输房号进房。 */
async function confirmJoinCode() {
  const code = joinCode.value.trim()
  if (!code) {
    toast.warning('请输入房号')
    return
  }
  await enterRoomByCode(code)
}

function goFriends() {
  router.push({ path: '/pages/friends/index' })
}

function goRecords() {
  router.push({ path: '/pages/records/index' })
}

function goManual() {
  router.push({ path: '/pages/manual/index' })
}
</script>

<template>
  <view class="home-page">
    <!-- 自绘 nav -->
    <view class="nav">
      <view class="side">
        <text class="navicon">
          ♠
        </text>
        <text class="ntitle">
          打牌记账
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

    <!-- 身份牌 -->
    <PlayCard
      class="idcard"
      suit="♠"
      rank="A"
      style="--suit: var(--vermillion);"
    >
      <view class="id-inner">
        <view class="pav">
          <image
            v-if="avatarUrl"
            class="pav-img"
            :src="avatarUrl"
            mode="aspectFill"
          />
          <text
            v-else
            class="pav-txt"
          >
            {{ initial }}
          </text>
        </view>
        <view class="id-main">
          <text class="nm">
            {{ nickname }}
          </text>
          <text class="sub">
            {{ subLine }}
          </text>
          <view
            v-if="isLoggedIn"
            class="stats"
          >
            <view class="st">
              <text class="v num">
                {{ totalScoreText }}
              </text>
              <text class="k">
                总积分
              </text>
            </view>
            <view class="st">
              <text class="v num g">
                {{ winRateText }}
              </text>
              <text class="k">
                胜率
              </text>
            </view>
          </view>
          <text
            v-else
            class="login-guide"
            @click="goLogin"
          >
            登录后同步战绩与好友 ›
          </text>
        </view>
      </view>
    </PlayCard>

    <!-- 赢 / 输 stripline -->
    <view
      v-if="isLoggedIn"
      class="stripline"
    >
      <text>
        赢<text class="win-n">
          {{ winCountText }}
        </text>
      </text>
      <text>/</text>
      <text>
        输<text class="lose-n">
          {{ loseCountText }}
        </text>
      </text>
      <text>/</text>
      <text
        class="go"
        @click="goRecords"
      >
        战绩 ›
      </text>
    </view>

    <!-- 扇形 CTA -->
    <view class="fan-cta">
      <button
        class="fc back"
        :disabled="creating || joining"
        @click="onScanJoin"
      >
        扫码进房
      </button>
      <button
        class="fc front"
        :disabled="creating || joining"
        @click="onCreateRoom"
      >
        {{ creating ? '开房中…' : '我要开房' }}
      </button>
    </view>

    <!-- 快捷 pill -->
    <view class="fan-quick">
      <text
        class="pill p1"
        @click="goFriends"
      >
        好友排名
      </text>
      <text
        class="pill"
        @click="goRecords"
      >
        战绩流水
      </text>
      <text
        class="pill p3"
        @click="goManual"
      >
        使用手册
      </text>
    </view>

    <!-- 最近战绩 -->
    <view class="section records">
      <text class="h2">
        最近战绩
      </text>
      <view class="flows">
        <view
          v-for="r in recentRecords"
          :key="r.id"
          class="flow"
        >
          <text class="who">
            {{ recordDate(r.created_at) }} · {{ settleText(r.settle_type) }}
          </text>
          <text
            class="num amt"
            :class="r.score >= 0 ? 'win-n' : 'lose-n'"
          >
            {{ formatDelta(r.score) }}
          </text>
        </view>
        <view
          v-if="!recentRecords.length"
          class="flow empty"
        >
          <text class="cap">
            {{ emptyRecordsText }}
          </text>
        </view>
      </view>
      <text class="cap rec-cap">
        左滑房间记录可看流水 / 排行 / 删除
      </text>
    </view>

    <!-- 广告位虚线占位 -->
    <view class="adslot">
      <text class="tag">
        广告
      </text>
      <text class="txt">
        感谢使用与反馈！广告位已避开功能按钮，减少误入
      </text>
    </view>

    <view class="grow" />

    <view class="footpills">
      <text class="fpill">
        客服
      </text>
      <text class="fpill">
        获取头像昵称
      </text>
      <text
        class="fpill"
        @click="goManual"
      >
        使用手册
      </text>
    </view>

    <!-- H5 扫码进房 = 输房号弹层（mp 走 uni.scanCode，不打开本弹层） -->
    <wd-popup
      v-model="showJoinPopup"
      position="center"
      closable
      custom-style="width: 300px; border-radius: 18px; background: var(--ink-700);"
      @close="joinCode = ''"
    >
      <view class="join-box">
        <text class="join-title">
          扫码进房
        </text>
        <text class="join-cap">
          输入房号加入牌桌
        </text>
        <wd-input
          v-model="joinCode"
          class="join-field"
          :maxlength="8"
          placeholder="房号"
        />
        <button
          class="btn brass join-btn"
          :disabled="joining"
          @click="confirmJoinCode"
        >
          {{ joining ? '进房中…' : '进入房间' }}
        </button>
      </view>
    </wd-popup>
  </view>
</template>

<style lang="scss" scoped>
.home-page {
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
  white-space: nowrap;
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

/* ── 身份牌 ── */
.idcard {
  width: 85%;
  max-width: 330px;
  height: 172px;
  margin: 14px auto 0;
}

.id-inner {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 26px 22px 20px 44px;
}

.pav {
  display: flex;
  flex: none;
  width: 74px;
  height: 74px;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border: 3px solid var(--brass);
  border-radius: 50%;
  background: var(--ink-700);
}

.pav-img {
  width: 100%;
  height: 100%;
  border-radius: 50%;
}

.pav-txt {
  color: var(--brass);
  font-size: 26px;
  font-weight: 500;
}

.id-main {
  min-width: 0;
}

.nm {
  display: block;
  overflow: hidden;
  color: var(--jet);
  font-family: 'Noto Serif SC', serif;
  font-size: 21px;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sub {
  display: block;
  margin-top: 2px;
  color: var(--dim);
  font-size: 11px;
}

.stats {
  display: flex;
  gap: 16px;
  margin-top: 10px;
}

.st .v {
  display: block;
  color: var(--jet);
  font-size: 24px;
  line-height: 1;
}

.st .v.g {
  color: var(--vermillion-600);
}

.st .k {
  display: block;
  margin-top: 3px;
  color: var(--dim);
  font-size: 10px;
}

.login-guide {
  display: block;
  margin-top: 12px;
  color: var(--vermillion-600);
  font-size: 13px;
  font-weight: 500;
}

/* ── stripline ── */
.stripline {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 14px 20px 0;
  color: var(--dim);
  font-size: 13px;
}

.stripline .go {
  color: var(--brass);
  font-weight: 500;
}

/* ── 扇形 CTA ── */
.fan-cta {
  position: relative;
  height: 106px;
  margin: 20px 20px 0;
}

.fan-cta .fc {
  position: absolute;
  right: 0;
  left: 0;
  display: flex;
  height: 56px;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: none;
  border-radius: 999px;
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 17px;
  font-weight: 500;
  line-height: 1;
}

.fan-cta .fc::after {
  border: none;
}

.fan-cta .fc[disabled] {
  opacity: 0.75;
}

.fan-cta .back {
  top: 42px;
  z-index: 1;
  border: 1.5px solid var(--on-ink-a28);
  background: var(--ink-700);
  color: var(--on-ink);
  transform: rotate(2.2deg);
}

.fan-cta .front {
  top: 0;
  z-index: 2;
  background: linear-gradient(180deg, var(--brass-hi), var(--brass));
  box-shadow: 0 4px 0 var(--brass-lo), var(--shadow-press);
  color: var(--ink-900);
  transform: rotate(-1.6deg);
}

/* ── 快捷 pill ── */
.fan-quick {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 26px 20px 0;
}

.pill {
  display: inline-flex;
  height: 40px;
  align-items: center;
  justify-content: center;
  padding: 0 16px;
  border: 1px solid var(--on-ink-a25);
  border-radius: 999px;
  background: var(--on-ink-a04);
  color: var(--on-ink);
  font-size: 13px;
}

.fan-quick .p1 {
  transform: rotate(-4deg);
}

.fan-quick .p3 {
  transform: rotate(4deg);
}

/* ── 最近战绩 ── */
.section.records {
  padding: 22px 20px 0;
}

.h2 {
  display: block;
  margin-bottom: 10px;
  color: var(--on-ink);
  font-family: 'Noto Serif SC', serif;
  font-size: 17px;
  font-weight: 700;
  letter-spacing: 0.05em;
}

.flows {
  display: flex;
  flex-direction: column;
  gap: 9px;
}

.flow {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 11px 13px;
  border: 1px solid var(--on-ink-a04);
  border-radius: 10px;
  background: var(--ink-700);
}

.flow .who {
  color: var(--on-ink);
  font-size: 13px;
}

.flow .amt {
  font-size: 17px;
}

.flow.empty {
  justify-content: center;
}

.cap {
  color: var(--dim);
  font-size: 11px;
  line-height: 1.55;
}

.rec-cap {
  display: block;
  margin-top: 8px;
}

/* ── 广告位虚线占位 ── */
.adslot {
  display: flex;
  align-items: center;
  gap: 9px;
  margin: 14px 20px 0;
  padding: 9px 12px;
  border: 1px dashed var(--on-ink-a25);
  border-radius: 10px;
  background: var(--on-ink-a04);
}

.adslot .tag {
  flex: none;
  padding: 1px 5px;
  border: 1px solid var(--on-ink-a25);
  border-radius: 4px;
  color: var(--dim);
  font-size: 10px;
}

.adslot .txt {
  color: var(--dim);
  font-size: 11.5px;
  line-height: 1.5;
}

.grow {
  flex: 1;
  min-height: 16px;
}

/* ── 底部 pill ── */
.footpills {
  display: flex;
  flex: none;
  justify-content: center;
  gap: 10px;
  padding: 0 20px calc(env(safe-area-inset-bottom, 0px) + 16px);
}

.fpill {
  display: inline-flex;
  height: 34px;
  align-items: center;
  justify-content: center;
  padding: 0 14px;
  border: 1px solid var(--on-ink-a04);
  border-radius: 999px;
  color: var(--dim);
  font-size: 12px;
}

/* ── H5 输房号弹层 ── */
.join-box {
  display: flex;
  flex-direction: column;
  padding: 24px 20px 22px;
}

.join-title {
  color: var(--on-ink);
  font-family: 'Noto Serif SC', serif;
  font-size: 17px;
  font-weight: 700;
  text-align: center;
}

.join-cap {
  margin-top: 6px;
  color: var(--dim);
  font-size: 12px;
  text-align: center;
}

.join-field {
  --wot-input-bg: var(--ink-600);
  --wot-input-inner-color: var(--on-ink);
  --wot-input-inner-placeholder-color: var(--dim);

  margin: 16px 0 14px;
  border-radius: 10px;
}

.join-btn {
  flex: none;
}
</style>
