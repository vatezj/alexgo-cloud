<script setup lang="ts">
import type { SeatDirection } from '@/utils/funCompass/zhiwei'
/**
 * 实时罗盘 pages/fun/compass/compass（设计 preview/compass.html；spec 罗盘跨端行为）
 *
 * - MP-WEIXIN：wx.onCompassChange 驱动表盘（onShow 注册 / onHide 注销），
 *   绝不调用任何定位 API；2.5s 无读数 → 权限引导条（去开启 = 重试 + 系统设置提示，可关）
 * - H5：无传感器 → 表盘可拖动（touch + mouse）+「模拟」角标
 * - 结构：读数 chip → **目标对向卡（D11 选座联动）** → 旋转盘面（刻度/八方位/指针/方位水印）
 *   → 民俗签 → 映射条 → 权限条 → 校准/说明 CTA → 只存本机 caption → 底部免责
 * - D11：读共享推荐 fun_compass_rec（模拟器排盘写入）——目标座/差角/|夹角|≤15° 已对准态；
 *   目标方位字标黄铜高亮 + 盘面黄铜三角随盘转（转到顶部索引下 = 对准）；无推荐 → 去排座引导
 */
import ModDisclaimer from '@/components/funCompass/ModDisclaimer.vue'
import ModNav from '@/components/funCompass/ModNav.vue'
import { useSystemBar } from '@/composables/useSystemBar'
import { useFunCompassRecommendationStore } from '@/store/funCompass/recommendation'

definePage({
  style: {
    navigationStyle: 'custom',
  },
})

const DIRS = ['北', '东北', '东', '东南', '南', '西南', '西', '西北'] as const
/** 八方位在盘面上的角度（北=0，顺时针） */
const DIR_ANGLES = [0, 45, 90, 135, 180, 225, 270, 315]

const router = useRouter()
const toast = useToast()
// 裸 useDialog() inject 不到该 key 会自建孤儿 ref，confirm 永不弹且 promise 挂起。
const dialog = useDialog('globalDialog')
const { pageStyle } = useSystemBar()

const heading = ref(0)
const hasReading = ref(false)
const showPermGuide = ref(false)

const dirIndex = computed(() => Math.round(heading.value / 45) % 8)
const dirChar = computed(() => DIRS[dirIndex.value])
const headingText = computed(() => `${Math.round(heading.value)}° · ${DIRS[dirIndex.value]}`)

/* ── D11 选座对向（共享推荐 → 目标卡/对准判定） ── */

const recStore = useFunCompassRecommendationStore()

/** 四座目标度数（正北基准） */
const TARGET_ANGLE: Record<SeatDirection, number> = { 北: 0, 东: 90, 南: 180, 西: 270 }

const target = computed<SeatDirection | null>(() => recStore.direction)
const targetAngle = computed<number | null>(() =>
  target.value === null ? null : TARGET_ANGLE[target.value])

/** 有符号夹角（−180…180）：目标 − 当前 */
const deltaDeg = computed<number | null>(() => {
  if (targetAngle.value === null)
    return null
  return ((targetAngle.value - heading.value + 540) % 360) - 180
})

/** 对准判定：|夹角| ≤ 15° */
const aligned = computed(() =>
  deltaDeg.value !== null && Math.abs(deltaDeg.value) <= 15)

function normalize(deg: number): number {
  return ((deg % 360) + 360) % 360
}

/* ── MP：磁力计 ── */

type CompassCb = WechatMiniprogram.OnCompassChangeCallback

let compassHandler: CompassCb | null = null
let noReadingTimer: ReturnType<typeof setTimeout> | null = null

function clearNoReadingTimer() {
  if (noReadingTimer) {
    clearTimeout(noReadingTimer)
    noReadingTimer = null
  }
}

function startWatch() {
  // #ifdef MP-WEIXIN
  if (typeof wx === 'undefined' || typeof wx.onCompassChange !== 'function') {
    showPermGuide.value = true
    return
  }
  stopWatch()
  compassHandler = (res) => {
    // 兼容字段：官方文档 heading、 typings direction（面向方向度数）
    const anyRes = res as { heading?: number, direction?: number }
    const h = anyRes.heading ?? anyRes.direction ?? 0
    hasReading.value = true
    showPermGuide.value = false
    heading.value = normalize(h)
  }
  wx.onCompassChange(compassHandler)
  clearNoReadingTimer()
  noReadingTimer = setTimeout(() => {
    if (!hasReading.value)
      showPermGuide.value = true
  }, 2500)
  // #endif
  // #ifndef MP-WEIXIN
  showPermGuide.value = false
  // #endif
}

function stopWatch() {
  // #ifdef MP-WEIXIN
  clearNoReadingTimer()
  if (compassHandler && typeof wx !== 'undefined' && typeof wx.offCompassChange === 'function')
    wx.offCompassChange(compassHandler as unknown as WechatMiniprogram.OffCompassChangeCallback)
  compassHandler = null
  // #endif
}

/** 去开启：重试监听；仍无读数时提示去系统设置放开传感器。 */
function onEnablePerm() {
  hasReading.value = false
  startWatch()
  setTimeout(() => {
    if (!hasReading.value)
      toast.info({ msg: '若仍无反应，请在系统设置中允许本小程序使用传感器' })
  }, 1500)
}

function onCloseGuide() {
  showPermGuide.value = false
}

function onCalibrate() {
  toast.info({ msg: '将手机悬空画「8」字，即可校准磁力计' })
  startWatch()
}

function onExplain() {
  dialog
    .confirm({
      title: '方位说明',
      msg: '东=木 · 南=火 · 西=金 · 北=水。方位读数来自设备磁力计（不获取定位），民俗趣味参考，和记账理财无关。',
      confirmButtonText: '知道了',
      cancelButtonText: '',
    })
    .catch(() => {})
}

/* ── H5：拖动模拟（touch + mouse；MP 上 touch 生效、mouse 无害） ── */

let dragging = false
let lastX = 0

function onDragStart(x: number) {
  dragging = true
  lastX = x
}

function onDragMove(x: number) {
  if (!dragging)
    return
  const dx = x - lastX
  lastX = x
  heading.value = normalize(heading.value + dx * 0.6)
}

function onDragEnd() {
  dragging = false
}

function onTouchStart(e: TouchEvent) {
  onDragStart(e.touches[0]?.clientX ?? 0)
}

function onTouchMove(e: TouchEvent) {
  onDragMove(e.touches[0]?.clientX ?? 0)
}

function onMouseDown(e: MouseEvent) {
  onDragStart(e.clientX)
}

function onMouseMove(e: MouseEvent) {
  onDragMove(e.clientX)
}

/* ── 生命周期 ── */

onShow(() => {
  startWatch()
})

onHide(() => {
  stopWatch()
})

function goBack() {
  if (getCurrentPages().length > 1)
    uni.navigateBack()
  else
    router.replaceAll({ path: '/pages/fun/compass/index' })
}

/** 刻度：36 根（每 10°），i%3===0 为 30° 大刻度带度数 */
const ticks = Array.from({ length: 36 }, (_, i) => ({
  angle: i * 10,
  major: i % 3 === 0,
  label: i * 10,
}))
</script>

<template>
  <view
    class="fc-page"
    :style="pageStyle"
  >
    <ModNav
      title="实时罗盘"
      @back="goBack"
    />

    <!-- 读数 chip -->
    <view class="fc-readout">
      <text
        class="fc-readout-t"
        :class="{ on: aligned }"
      >
        {{ headingText }}
      </text>
    </view>

    <!-- 目标对向卡（D11）：有推荐 → 目标+差角/对准；无推荐 → 去排座 -->
    <view
      v-if="target"
      class="fc-target"
      :class="{ on: aligned }"
    >
      <view class="fc-target-main">
        <text class="fc-target-k">
          目标 · {{ target }}座（{{ targetAngle }}°）
        </text>
        <text
          class="fc-target-d"
          :class="{ ok: aligned }"
        >
          {{ aligned ? '已对准 · 可入座' : `差 ${Math.abs(Math.round(deltaDeg ?? 0))}°` }}
        </text>
      </view>
      <text
        v-if="recStore.birthLabel"
        class="fc-target-birth"
      >
        {{ recStore.birthLabel }}
      </text>
    </view>
    <view
      v-else
      class="fc-target empty"
      @click="router.push({ path: '/pages/fun/compass/simulator' })"
    >
      <text class="fc-target-k">
        还没排个座？
      </text>
      <text class="fc-target-d">
        去排座 →
      </text>
    </view>

    <!-- 表盘 -->
    <view
      class="fc-dial"
      @touchstart="onTouchStart"
      @touchmove="onTouchMove"
      @touchend="onDragEnd"
      @mousedown="onMouseDown"
      @mousemove="onMouseMove"
      @mouseup="onDragEnd"
      @mouseleave="onDragEnd"
    >
      <!-- 旋转盘面（跟随方位；H5 拖动改 heading） -->
      <view
        class="fc-rot"
        :style="{ transform: `rotate(${-heading}deg)` }"
      >
        <view
          v-for="t in ticks"
          :key="t.angle"
          class="fc-tick"
          :class="{ major: t.major }"
          :style="{ transform: `translate(-50%, -50%) rotate(${t.angle}deg) translateY(-132px)` }"
        />
        <text
          v-for="(t, i) in ticks.filter(t => t.major)"
          :key="`l${t.angle}`"
          class="fc-tick-label"
          :style="{ transform: `translate(-50%, -50%) rotate(${t.angle}deg) translateY(-112px) rotate(${-t.angle}deg)` }"
        >
          {{ i * 30 }}
        </text>

        <!-- 八方位字 -->
        <text
          v-for="(d, i) in DIRS"
          :key="d"
          class="fc-dir"
          :class="{ cardinal: i % 2 === 0, tgt: DIRS[i] === target }"
          :style="{ transform: `translate(-50%, -50%) rotate(${DIR_ANGLES[i]}deg) translateY(-84px) rotate(${-DIR_ANGLES[i]}deg)` }"
        >
          {{ d }}
        </text>

        <!-- 指针（指向盘面北）+ 中心轴 -->
        <view class="fc-needle" />
        <view class="fc-hub" />

        <!-- 目标方位标记（D11：黄铜菱标，随盘转到顶部索引下 = 对准） -->
        <view
          v-if="targetAngle !== null"
          class="fc-tgt-dot"
          :style="{ transform: `translate(-50%, -50%) rotate(${targetAngle}deg) translateY(-106px)` }"
        />

        <!-- 方位水印 -->
        <text class="fc-watermark">
          {{ dirChar }}
        </text>
      </view>

      <!-- 固定：顶部方位索引 -->
      <view class="fc-index" />

      <!-- H5 模拟角标 -->
      <!-- #ifdef H5 -->
      <text class="fc-sim-badge">
        模拟
      </text>
      <!-- #endif -->
    </view>

    <!-- 民俗签 -->
    <view class="fc-sign">
      <text class="fc-sign-t">
        此向为{{ dirChar }} · 民俗参考
      </text>
    </view>

    <!-- 四方位映射条（骨白） -->
    <view class="fc-map-bar">
      <view
        v-for="m in ['东=木', '南=火', '西=金', '北=水']"
        :key="m"
        class="fc-map-cell"
      >
        <text class="fc-map-cell-t">
          {{ m }}
        </text>
      </view>
    </view>

    <!-- 磁力计权限引导（仅小程序；未出读数时出现） -->
    <!-- #ifdef MP-WEIXIN -->
    <view
      v-if="showPermGuide"
      class="fc-perm"
    >
      <text class="fc-perm-t">
        开启磁力计权限，罗盘才能转起来
      </text>
      <view
        class="fc-perm-btn"
        @click="onEnablePerm"
      >
        去开启
      </view>
      <text
        class="fc-perm-x"
        @click="onCloseGuide"
      >
        ✕
      </text>
    </view>

    <view class="fc-note">
      <text class="fc-note-t">
        使用磁力计测方向 · 不获取定位
      </text>
    </view>
    <!-- #endif -->

    <view class="fc-spacer" />

    <!-- CTA -->
    <view class="fc-cta-row">
      <view
        class="fc-btn primary"
        @click="onCalibrate"
      >
        校准罗盘
      </view>
      <view
        class="fc-btn ghost"
        @click="onExplain"
      >
        方位说明
      </view>
    </view>

    <text class="fc-cap">
      数据只在本机 · 不上传
    </text>

    <ModDisclaimer />
  </view>
</template>

<style lang="scss" scoped>
.fc-page {
  display: flex;
  box-sizing: border-box;
  flex-direction: column;
  min-height: 100vh;
  padding: calc(var(--status-bar-h, env(safe-area-inset-top, 0px)) + 6px) 16px calc(env(safe-area-inset-bottom, 0px) + 16px);
  background: var(--ink-900);
  background-image: var(--spot);
  color: var(--on-ink);
}

/* 读数 chip */
.fc-readout {
  display: flex;
  justify-content: center;
  margin: 4px 0 10px;
}

.fc-readout-t.on {
  border-width: 2px;
  box-shadow: 0 0 14px rgba(201, 162, 75, 0.35);
}

.fc-readout-t {
  padding: 6px 18px;
  border: 1px solid var(--brass);
  border-radius: 999px;
  background: var(--ink-700);
  color: var(--brass);
  font-family: 'Oswald', 'Noto Sans SC', sans-serif;
  font-size: 17px;
  font-weight: 600;
  letter-spacing: 0.04em;
}

/* 表盘 */
.fc-dial {
  position: relative;
  width: 300px;
  height: 300px;
  margin: 0 auto 10px;
  border: 1px solid var(--on-ink-a28);
  border-radius: 50%;
  background: radial-gradient(circle at 50% 42%, var(--ink-700), var(--ink-900) 78%);
  box-shadow: inset 0 0 34px rgba(0, 0, 0, 0.55);
  touch-action: none;
  user-select: none;
}

.fc-rot {
  position: absolute;
  inset: 0;
  border-radius: 50%;
  /* 盘面随方位逆转（looking east → 东转到顶） */
  transition: transform 0.12s linear;
}

.fc-tick {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 1px;
  height: 8px;
  background: var(--on-ink-a28);
}

.fc-tick.major {
  width: 2px;
  height: 14px;
  background: var(--on-ink-a50, rgba(237, 230, 216, 0.5));
}

.fc-tick-label {
  position: absolute;
  top: 50%;
  left: 50%;
  color: var(--dim);
  font-family: 'Oswald', sans-serif;
  font-size: 11px;
}

.fc-dir {
  position: absolute;
  top: 50%;
  left: 50%;
  color: var(--dim);
  font-family: 'Noto Serif SC', serif;
  font-size: 13px;
  font-weight: 500;
}

.fc-dir.cardinal {
  color: var(--on-ink);
  font-size: 22px;
  font-weight: 700;
}

/* 指针：自中心指向盘面北（盘顶），红尖 = 北端 */
.fc-needle {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 5px;
  height: 112px;
  border-radius: 3px;
  background: linear-gradient(to bottom, var(--vermillion) 0 46%, var(--on-ink) 46% 100%);
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.5);
  transform: translate(-50%, -100%);
}

.fc-hub {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 14px;
  height: 14px;
  border: 3px solid var(--brass);
  border-radius: 50%;
  background: var(--ink-900);
  transform: translate(-50%, -50%);
}

.fc-watermark {
  position: absolute;
  top: 50%;
  left: 50%;
  color: var(--brass);
  font-family: 'Noto Serif SC', serif;
  font-size: 42px;
  font-weight: 700;
  opacity: 0.12;
  transform: translate(-50%, -50%);
}

/* 顶部固定索引（黄铜小三角） */
.fc-index {
  position: absolute;
  top: 6px;
  left: 50%;
  width: 0;
  height: 0;
  border-right: 7px solid transparent;
  border-left: 7px solid transparent;
  border-top: 10px solid var(--brass);
  transform: translateX(-50%);
  z-index: 3;
}

.fc-sim-badge {
  position: absolute;
  top: 18px;
  right: 26px;
  padding: 1px 8px;
  border: 1px solid var(--on-ink-a28);
  border-radius: 999px;
  background: var(--ink-700);
  color: var(--dim);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 10.5px;
  z-index: 3;
}

/* ── 目标对向卡（D11） ── */
.fc-target {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin: 0 auto 10px;
  padding: 8px 16px;
  border: 1px solid var(--on-ink-a28);
  border-radius: 14px;
  background: var(--ink-700);
  text-align: center;
}

.fc-target.empty {
  flex-direction: row;
  gap: 8px;
  align-items: center;
  justify-content: center;
}

.fc-target.on {
  border-color: var(--brass);
  background: rgba(201, 162, 75, 0.12);
}

.fc-target-main {
  display: flex;
  gap: 10px;
  align-items: baseline;
  justify-content: center;
}

.fc-target-k {
  color: var(--on-ink);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 13px;
  font-weight: 500;
}

.fc-target-d {
  color: var(--dim);
  font-family: 'Oswald', 'Noto Sans SC', sans-serif;
  font-size: 14px;
  font-weight: 600;
}

.fc-target.on .fc-target-d,
.fc-target-d.ok {
  color: var(--brass);
}

.fc-target-birth {
  color: var(--dim);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 10.5px;
}

/* 目标方位字标 + 盘面菱标 */
.fc-dir.tgt {
  color: var(--brass);
}

.fc-tgt-dot {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 9px;
  height: 9px;
  border: 2px solid var(--brass);
  border-radius: 2px;
  background: var(--ink-900);
  z-index: 2;
}

/* 民俗签 */
.fc-sign {
  display: flex;
  justify-content: center;
  margin-bottom: 12px;
}

.fc-sign-t {
  padding: 3px 14px;
  border: 1px dashed var(--on-ink-a28);
  border-radius: 999px;
  color: var(--dim);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 11.5px;
}

/* 映射条 */
.fc-map-bar {
  display: flex;
  overflow: hidden;
  margin-bottom: 12px;
  border-radius: 14px;
  background: var(--tile);
  box-shadow: var(--shadow-tile);
}

.fc-map-cell {
  display: flex;
  flex: 1;
  min-height: 46px;
  align-items: center;
  justify-content: center;
  border-right: 1px solid var(--tile-shade);
}

.fc-map-cell:last-child {
  border-right: none;
}

.fc-map-cell-t {
  color: var(--jet);
  font-family: 'Noto Serif SC', serif;
  font-size: 15px;
  font-weight: 700;
}

/* 权限引导条（MP） */
.fc-perm {
  display: flex;
  gap: 8px;
  align-items: center;
  margin-bottom: 8px;
  padding: 10px 12px;
  border: 1px solid var(--brass);
  border-radius: 14px;
  background: var(--ink-700);
}

.fc-perm-t {
  flex: 1;
  color: var(--on-ink);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 12.5px;
}

.fc-perm-btn {
  flex: none;
  padding: 5px 14px;
  border-radius: 999px;
  background: var(--brass);
  color: var(--ink-900);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 12.5px;
  font-weight: 600;
}

.fc-perm-x {
  flex: none;
  padding: 0 4px;
  color: var(--dim);
  font-size: 13px;
}

.fc-note {
  display: flex;
  justify-content: center;
  margin-bottom: 6px;
}

.fc-note-t {
  color: var(--dim);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 11px;
}

.fc-spacer {
  flex: 1;
  min-height: 16px;
}

/* CTA */
.fc-cta-row {
  display: flex;
  gap: 10px;
  margin-bottom: 8px;
}

.fc-btn {
  display: flex;
  height: 48px;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 15.5px;
  font-weight: 500;
}

.fc-btn.primary {
  flex: 1;
  background: var(--tile);
  box-shadow: var(--shadow-tile);
  color: var(--jet);
}

.fc-btn.ghost {
  flex: none;
  padding: 0 22px;
  border: 1px solid var(--on-ink-a28);
  color: var(--on-ink);
}

.fc-cap {
  display: block;
  margin-bottom: 6px;
  color: var(--dim);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 11.5px;
  text-align: center;
}
</style>
