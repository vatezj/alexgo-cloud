<script setup lang="ts">
import type { CalcRecommendation, SeatDirection } from '@/utils/funCompass/zhiwei'
/**
 * 座位模拟器 pages/fun/compass/simulator（2026-11-06 修订轮）
 *
 * - 双相状态机：form（年月日+时间+性别，修订轮 9 全选择器、默认 2006-08-08）
 *   → result（zhiwei 排盘求座）；「修改」回表单、「重置」回初始
 * - 排盘：后端 POST /api/calc → top3 列表 + 8→4 聚合出推荐座（zhiwei.ts aggregateSeats）
 * - 推荐座：黄铜描边 +「推荐」角标；手动点其他座 = 选中切换 +「已手动改选」，
 *   推荐结论与角标保留降态（.rec.stale）；空推荐 → 不高亮 + 随缘文案
 * - 详情卡展示归因理由（服务原文）+ 环境自检四项；「从档案选择」读 fun_compass_archives
 * - 失败可重试（表单不丢）；文案锁定 copy.ts；底部免责 ModDisclaimer
 */
import ModDisclaimer from '@/components/funCompass/ModDisclaimer.vue'
import ModNav from '@/components/funCompass/ModNav.vue'
import { useSystemBar } from '@/composables/useSystemBar'
import { useFunCompassArchivesStore } from '@/store/funCompass/archives'
import { useFunCompassRecommendationStore } from '@/store/funCompass/recommendation'
import { useScorecardShareConfigStore } from '@/store/scorecard/shareConfig'
import { LOCAL_ONLY } from '@/utils/funCompass/copy'
import { aggregateSeats, calcRecommend } from '@/utils/funCompass/zhiwei'

definePage({
  style: {
    navigationStyle: 'custom',
  },
})

type SeatKey = 'n' | 'e' | 's' | 'w'
type Phase = 'form' | 'result'

const SEATS: { key: SeatKey, dir: SeatDirection, char: string }[] = [
  { key: 'n', dir: '北', char: '北' },
  { key: 'e', dir: '东', char: '东' },
  { key: 's', dir: '南', char: '南' },
  { key: 'w', dir: '西', char: '西' },
]

const SELF_CHECK = ['座椅有靠背', '桌面光线均匀', '背后无走道', '起身无遮挡']

const router = useRouter()
const toast = useToast()
const { pageStyle } = useSystemBar()
const archiveStore = useFunCompassArchivesStore()
/** 共享推荐（design D11）：排盘成功写入，罗盘页消费 */
const recStore = useFunCompassRecommendationStore()

/* ── 状态（修订轮 9：生辰单个 datetime-picker，默认 2006-08-08 14:30） ── */

const phase = ref<Phase>('form')
/** 生辰唯一数据源（时间戳）；wd-datetime-picker 仅 confirm 时落值 */
const birthTs = ref(new Date(2006, 7, 8, 14, 30).getTime())
const showBirth = ref(false)
const minBirth = new Date(1900, 0, 1).getTime()
const maxBirth = new Date(2100, 11, 31, 23, 59, 59).getTime()
const gender = ref<'male' | 'female'>('male')
const recommendations = ref<CalcRecommendation[]>([])
const computing = ref(false)
/** 当前选中座（result 相才有意义） */
const selected = ref<SeatDirection | null>(null)

function pad2(n: number): string {
  return String(n).padStart(2, '0')
}

/** 触发区回显：2006年8月8日 14:30 */
const birthLabel = computed(() => {
  const d = new Date(birthTs.value)
  return `${d.getFullYear()}年${d.getMonth() + 1}月${d.getDate()}日 ${pad2(d.getHours())}:${pad2(d.getMinutes())}`
})

/** 滚筒列文案：2026年 / 8月 / 8日 / 14时 / 30分 */
function birthFormatter(type: 'year' | 'month' | 'date' | 'hour' | 'minute' | 'second', value: number | string): string {
  const suffix: Record<string, string> = { year: '年', month: '月', date: '日', hour: '时', minute: '分', second: '秒' }
  return `${value}${suffix[type] ?? ''}`
}

/* ── 校验与排盘 ── */

function parseForm(): { year: number, month: number, day: number, hour: number, minute: number } | null {
  const d = new Date(birthTs.value)
  const year = d.getFullYear()
  const month = d.getMonth() + 1
  const day = d.getDate()
  const hour = d.getHours()
  const minute = d.getMinutes()
  // datetime-picker min/max 已限 1900–2100，此处仅兜底非法时间戳
  if (!Number.isFinite(birthTs.value) || year < 1900 || year > 2100) {
    toast.error({ msg: '出生时间选择有误，请重新选择' })
    return null
  }
  return { year, month, day, hour, minute }
}

/** 生辰摘要（排盘写共享推荐 / 结果摘要行共用） */
const summaryLine = computed(() => birthLabel.value)

async function onCompute() {
  const form = parseForm()
  if (!form || computing.value)
    return
  computing.value = true
  try {
    const res = await calcRecommend({ ...form, gender: gender.value })
    recommendations.value = res.recommendations
    const agg = aggregateSeats(res.recommendations)
    selected.value = agg.seat
    phase.value = 'result'
    recStore.setRecommendation({
      direction: agg.seat,
      reasons: agg.reasons,
      birthLabel: summaryLine.value,
    })
    if (res.recommendations.length === 0)
      toast.info({ msg: '本次无显著方位，随缘落座' })
  }
  catch (err) {
    toast.error({ msg: err instanceof Error ? err.message : '测算失败，请重试' })
  }
  finally {
    computing.value = false
  }
}

/** 换个推荐：按当前生辰重排（服务随起局时间变化，可得新结果） */
function onRefresh() {
  void onCompute()
}

function onModify() {
  phase.value = 'form'
}

/** 重置：清结果与选中，回未排盘初始态（生辰输入保留） */
function onReset() {
  recommendations.value = []
  selected.value = null
  recStore.clear()
  phase.value = 'form'
}

function pick(dir: SeatDirection) {
  if (phase.value !== 'result')
    return
  selected.value = dir
}

/* ── 派生 ── */

const agg = computed(() => aggregateSeats(recommendations.value))

const recommended = computed<SeatDirection | null>(() =>
  phase.value === 'result' ? agg.value.seat : null)

const isStale = computed(() =>
  !!recommended.value && !!selected.value && selected.value !== recommended.value)

function seatClass(dir: SeatDirection): Record<string, boolean> {
  return {
    sel: selected.value === dir,
    rec: recommended.value === dir,
    stale: recommended.value === dir && isStale.value,
  }
}

const markChar = computed(() => {
  if (phase.value !== 'result' || !selected.value)
    return ''
  return selected.value
})

/** 复核行：已选 X座 · [推荐采纳|已手动改选] · 自检 4/4 */
const caption = computed(() => {
  if (phase.value !== 'result' || !selected.value)
    return ''
  const parts = [`已选 ${selected.value}座`]
  if (recommended.value)
    parts.push(isStale.value ? '已手动改选' : '推荐采纳')
  parts.push('自检 4/4')
  return parts.join(' · ')
})

/** 详情卡推荐理由（归因于推荐座的服务原文；手选非推荐座时展示推荐座理由作对照） */
const detailReasons = computed(() => agg.value.reasons)

/* ── 从档案选择 ── */

function onPickFromArchives() {
  const list = archiveStore.sorted
  if (list.length === 0) {
    toast.error({ msg: '还没有档案，先去测算页存一份' })
    return
  }
  uni.showActionSheet({
    itemList: list.map(a => `${a.name || '未命名'} · ${a.year}.${a.month}.${a.day}`),
    success: ({ tapIndex }) => {
      const a = list[tapIndex ?? -1]
      if (!a)
        return
      birthTs.value = new Date(a.year, a.month - 1, a.day, a.hour, a.minute).getTime()
      gender.value = a.gender
      if (phase.value === 'result')
        void onCompute()
    },
  })
}

/** 携推荐去罗盘对向（推荐已在排盘时写入 recStore） */
function goCompassAlign() {
  router.push({ path: '/pages/fun/compass/compass' })
}

/* ── 转发：title/图读后台配置（openspec share-card-config）；path 落本页、不带 inv ── */

const shareCfgStore = useScorecardShareConfigStore()

onShareAppMessage(() => {
  const card = shareCfgStore.get('simulator')
  return {
    title: card.title,
    // 空串不传：微信回退当前页截图
    ...(card.imageUrl ? { imageUrl: card.imageUrl } : {}),
    path: '/pages/fun/compass/simulator',
  }
})

onShareTimeline(() => ({
  title: shareCfgStore.get('simulator').title,
  query: '',
}))

onShow(() => {
  // 分享卡配置后台刷新（不 await，转发钩子同步读缓存）
  void shareCfgStore.ensureFresh()
})

function goBack() {
  if (getCurrentPages().length > 1)
    uni.navigateBack()
  else
    router.replaceAll({ path: '/pages/fun/compass/index' })
}
</script>

<template>
  <view
    class="fc-page"
    :style="pageStyle"
  >
    <ModNav
      title="座位模拟器"
      @back="goBack"
    />

    <!-- ══ 相 1：填写生辰 ══ -->
    <template v-if="phase === 'form'">
      <view class="fc-form-card">
        <text class="fc-form-t">
          填出生时间，测个座
        </text>
        <view class="fc-priv">
          <view class="fc-priv-dot" />
          <text class="fc-priv-t">
            {{ LOCAL_ONLY }}
          </text>
        </view>

        <view class="fc-field">
          <text class="fc-field-k">
            出生时间
          </text>
          <view
            class="fc-input fc-picker"
            @click="showBirth = true"
          >
            {{ birthLabel }}
          </view>
          <wd-datetime-picker
            v-model="birthTs"
            v-model:visible="showBirth"
            type="datetime"
            title="选择出生时间"
            :min-date="minBirth"
            :max-date="maxBirth"
            :formatter="birthFormatter"
          />
        </view>

        <view class="fc-field">
          <text class="fc-field-k">
            性别
          </text>
          <view class="fc-gender">
            <view
              class="fc-gchip"
              :class="{ on: gender === 'male' }"
              @click="gender = 'male'"
            >
              男
            </view>
            <view
              class="fc-gchip"
              :class="{ on: gender === 'female' }"
              @click="gender = 'female'"
            >
              女
            </view>
          </view>
        </view>

        <view
          class="fc-from-arch"
          @click="onPickFromArchives"
        >
          <text class="fc-from-arch-t">
            从出生档案选择
          </text>
          <wd-icon
            css-icon
            class="fc-from-arch-c"
            name="i-carbon-chevron-right"
          />
        </view>
      </view>

      <view class="fc-cta-row">
        <view
          class="fc-btn primary"
          :class="{ disabled: computing }"
          @click="onCompute"
        >
          {{ computing ? '计算中…' : '试试方位 · 求个座' }}
        </view>
      </view>
    </template>

    <!-- ══ 相 2：出结果 ══ -->
    <template v-else>
      <!-- 生辰摘要行 -->
      <view class="fc-summary">
        <text class="fc-summary-t">
          {{ summaryLine }}
        </text>
        <view
          class="fc-summary-act"
          @click="onModify"
        >
          修改
        </view>
      </view>

      <!-- zhiwei top3 推荐列表 -->
      <view class="fc-form-card">
        <text class="fc-res-t">
          推荐方位
        </text>

        <view
          v-if="recommendations.length === 0"
          class="fc-res-empty"
        >
          本次无显著方位信号，随缘落座即可
        </view>

        <view
          v-for="r in recommendations"
          :key="r.rank"
          class="fc-rec-row"
        >
          <view class="fc-rec-rank">
            {{ r.rank }}
          </view>
          <view class="fc-rec-main">
            <view class="fc-rec-head">
              <text class="fc-rec-dir">
                {{ r.direction }}
              </text>
              <text class="fc-rec-score">
                {{ r.score }}
              </text>
            </view>
            <text class="fc-rec-reason">
              {{ r.reason }}
            </text>
          </view>
        </view>
      </view>

      <!-- 俯视麻将桌 -->
      <view class="fc-table">
        <view class="fc-table-felt" />
        <view class="fc-table-dash" />
        <view class="fc-corner tl" />
        <view class="fc-corner tr" />
        <view class="fc-corner bl" />
        <view class="fc-corner br" />
        <text
          v-if="markChar"
          class="fc-table-mark"
        >
          {{ markChar }}
        </text>

        <view
          v-for="s in SEATS"
          :key="s.key"
          class="fc-seat"
          :class="[s.key, seatClass(s.dir)]"
          @click="pick(s.dir)"
        >
          <text class="fc-seat-t">
            {{ s.char }}
          </text>
          <text
            v-if="recommended === s.dir"
            class="fc-rec-badge"
            :class="{ dim: isStale }"
          >
            推荐
          </text>
        </view>
      </view>

      <!-- 座位详情卡 -->
      <view
        v-if="selected"
        class="fc-detail"
      >
        <view class="fc-detail-head">
          <text class="fc-detail-t">
            {{ selected }}座
          </text>
          <text
            v-if="isStale"
            class="fc-stale-tag"
          >
            已手动改选
          </text>
        </view>

        <view
          v-if="detailReasons.length"
          class="fc-reasons"
        >
          <text class="fc-reasons-k">
            推荐理由
          </text>
          <text
            v-for="(reason, i) in detailReasons"
            :key="i"
            class="fc-reason-t"
          >
            {{ reason }}
          </text>
        </view>

        <view
          v-if="recommended"
          class="fc-to-compass"
          @click="goCompassAlign"
        >
          <text class="fc-to-compass-t">
            用罗盘对准{{ recommended }}座
          </text>
          <wd-icon
            css-icon
            class="fc-to-compass-c"
            name="i-carbon-chevron-right"
          />
        </view>

        <text class="fc-check-k">
          环境自检
        </text>
        <view class="fc-check-grid">
          <view
            v-for="c in SELF_CHECK"
            :key="c"
            class="fc-check-item"
          >
            <text class="fc-check-ok">
              ✓
            </text>
            <text class="fc-check-t">
              {{ c }}
            </text>
          </view>
        </view>
      </view>

      <view class="fc-cta-row">
        <view
          class="fc-btn primary"
          :class="{ disabled: computing }"
          @click="onRefresh"
        >
          {{ computing ? '计算中…' : '换个推荐' }}
        </view>
        <view
          class="fc-btn ghost"
          @click="onReset"
        >
          重置
        </view>
      </view>

      <text
        v-if="caption"
        class="fc-caption"
      >
        {{ caption }}
      </text>
    </template>

    <view class="fc-spacer" />

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

/* ── 骨白卡（表单/结果共用） ── */
.fc-form-card {
  margin-bottom: 14px;
  padding: 16px 14px;
  border-radius: var(--r-tile);
  background: var(--tile);
  box-shadow: var(--shadow-tile);
  color: var(--jet);
}

.fc-form-t {
  display: block;
  color: var(--jet);
  font-family: 'Noto Serif SC', serif;
  font-size: 21px;
  font-weight: 700;
  letter-spacing: 0.03em;
}

.fc-priv {
  display: flex;
  gap: 6px;
  align-items: center;
  margin: 6px 0 12px;
}

.fc-priv-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--jade);
}

.fc-priv-t {
  color: var(--jet);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 11px;
  opacity: 0.65;
}

.fc-field {
  display: flex;
  gap: 10px;
  align-items: center;
  margin-bottom: 10px;
}

.fc-field-k {
  flex: none;
  width: 34px;
  color: var(--jet);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 14px;
  font-weight: 500;
  line-height: 40px;
}

.fc-input {
  box-sizing: border-box;
  flex: 1;
  height: 40px;
  padding: 0 16px;
  border-radius: 999px;
  background: var(--ink-700);
  color: var(--on-ink);
  font-family: 'Oswald', sans-serif;
  font-size: 17px;
  font-weight: 500;
  text-align: center;
}

.fc-picker {
  display: flex;
  align-items: center;
  justify-content: center;
  line-height: 40px;
}

/* 性别 chips */
.fc-gender {
  display: flex;
  flex: 1;
  gap: 10px;
}

.fc-gchip {
  display: flex;
  flex: 1;
  height: 40px;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--tile-shade);
  border-radius: 999px;
  background: var(--tile);
  color: var(--jet);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 15px;
}

.fc-gchip.on {
  border-color: var(--ink-700);
  background: var(--ink-700);
  color: var(--on-ink);
  font-weight: 600;
}

.fc-from-arch {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  margin-top: 6px;
  padding: 8px 0 2px;
}

.fc-from-arch-t {
  color: var(--jet);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 12.5px;
  text-decoration: underline;
  opacity: 0.7;
}

.fc-from-arch-c {
  color: var(--jet);
  font-size: 14px;
  opacity: 0.5;
}

/* ── 摘要行（result 相） ── */
.fc-summary {
  display: flex;
  gap: 10px;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
  padding: 12px 14px;
  border-radius: var(--r-tile);
  background: var(--tile);
  box-shadow: var(--shadow-tile);
}

.fc-summary-t {
  color: var(--jet);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 14.5px;
  font-weight: 500;
}

.fc-summary-act {
  flex: none;
  padding: 5px 14px;
  border: 1px solid var(--tile-shade);
  border-radius: 999px;
  color: var(--jet);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 12.5px;
}

/* ── top3 推荐列表 ── */
.fc-res-t {
  display: block;
  margin-bottom: 8px;
  color: var(--jet);
  font-family: 'Noto Serif SC', serif;
  font-size: 16.5px;
  font-weight: 700;
}

.fc-res-empty {
  padding: 10px 4px;
  color: var(--jet);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 13px;
  opacity: 0.65;
  text-align: center;
}

.fc-rec-row {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  padding: 8px 0;
  border-top: 1px solid var(--tile-shade);
}

.fc-rec-row:first-of-type {
  border-top: none;
}

.fc-rec-rank {
  display: flex;
  flex: none;
  width: 22px;
  height: 22px;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: var(--ink-700);
  color: var(--on-ink);
  font-family: 'Oswald', sans-serif;
  font-size: 12px;
  font-weight: 600;
}

.fc-rec-main {
  display: flex;
  flex: 1;
  min-width: 0;
  flex-direction: column;
  gap: 3px;
}

.fc-rec-head {
  display: flex;
  gap: 8px;
  align-items: baseline;
}

.fc-rec-dir {
  color: var(--jet);
  font-family: 'Noto Serif SC', serif;
  font-size: 18px;
  font-weight: 700;
}

.fc-rec-score {
  color: var(--brass);
  font-family: 'Oswald', sans-serif;
  font-size: 16px;
  font-weight: 600;
}

.fc-rec-reason {
  color: var(--jet);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 11.5px;
  line-height: 1.55;
  opacity: 0.7;
}

/* ── 俯视麻将桌 ── */
.fc-table {
  position: relative;
  height: 250px;
  margin-bottom: 14px;
}

.fc-table-felt {
  position: absolute;
  inset: 10px 4px;
  border-radius: 22px;
  background: var(--ink-700);
  box-shadow: inset 0 0 40px rgba(0, 0, 0, 0.45);
}

.fc-table-dash {
  position: absolute;
  inset: 34px 30px;
  border: 1px dashed var(--on-ink-a28);
  border-radius: 12px;
}

.fc-corner {
  position: absolute;
  width: 18px;
  height: 18px;
  border-color: var(--brass);
  border-style: solid;
  opacity: 0.75;
}

.fc-corner.tl {
  top: 34px;
  left: 30px;
  border-width: 2px 0 0 2px;
}

.fc-corner.tr {
  top: 34px;
  right: 30px;
  border-width: 2px 2px 0 0;
}

.fc-corner.bl {
  bottom: 34px;
  left: 30px;
  border-width: 0 0 2px 2px;
}

.fc-corner.br {
  right: 30px;
  bottom: 34px;
  border-width: 0 2px 2px 0;
}

.fc-table-mark {
  position: absolute;
  top: 50%;
  left: 50%;
  color: var(--brass);
  font-family: 'Noto Serif SC', serif;
  font-size: 64px;
  font-weight: 700;
  opacity: 0.12;
  transform: translate(-50%, -50%);
}

/* 座位牌片 */
.fc-seat {
  position: absolute;
  display: flex;
  width: 64px;
  height: 46px;
  align-items: center;
  justify-content: center;
  border: 2px solid transparent;
  border-radius: 12px;
  background: var(--tile);
  box-shadow: var(--shadow-tile);
  z-index: 2;
}

.fc-seat-t {
  color: var(--jet);
  font-family: 'Noto Serif SC', serif;
  font-size: 24px;
  font-weight: 700;
}

.fc-seat.n {
  top: 0;
  left: 50%;
  transform: translateX(-50%);
}

.fc-seat.s {
  bottom: 0;
  left: 50%;
  transform: translateX(-50%);
}

.fc-seat.w {
  top: 50%;
  left: 0;
  transform: translateY(-50%);
}

.fc-seat.e {
  top: 50%;
  right: 0;
  transform: translateY(-50%);
}

/* 选中 / 推荐 / 降态 */
.fc-seat.sel {
  border-color: var(--jet);
}

.fc-seat.rec {
  border-color: var(--brass);
  box-shadow: var(--shadow-tile), 0 0 14px rgba(201, 162, 75, 0.35);
}

.fc-seat.rec .fc-seat-t {
  color: color-mix(in srgb, var(--brass) 45%, var(--jet));
}

.fc-seat.rec.stale {
  border-style: dashed;
  box-shadow: var(--shadow-tile);
}

.fc-rec-badge {
  position: absolute;
  top: -9px;
  right: -14px;
  padding: 1px 7px;
  border-radius: 999px;
  background: var(--vermillion);
  color: var(--tile);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 10px;
  font-weight: 500;
}

.fc-rec-badge.dim {
  opacity: 0.55;
}

/* ── 座位详情卡 ── */
.fc-detail {
  margin-bottom: 14px;
  padding: 14px;
  border-radius: var(--r-tile);
  background: var(--tile);
  box-shadow: var(--shadow-tile);
  color: var(--jet);
}

.fc-detail-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.fc-detail-t {
  color: var(--jet);
  font-family: 'Noto Serif SC', serif;
  font-size: 17px;
  font-weight: 700;
}

.fc-stale-tag {
  padding: 2px 8px;
  border: 1px dashed var(--jet);
  border-radius: 999px;
  color: var(--jet);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 10.5px;
  opacity: 0.7;
}

.fc-reasons {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-top: 10px;
  padding: 10px;
  border-radius: 12px;
  background: var(--ink-700);
}

.fc-reasons-k {
  color: var(--on-ink);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 11.5px;
  font-weight: 500;
  opacity: 0.75;
}

.fc-reason-t {
  color: var(--on-ink);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 12px;
  line-height: 1.6;
}

/* 用罗盘对向入口（D11） */
.fc-to-compass {
  display: flex;
  gap: 6px;
  align-items: center;
  justify-content: center;
  margin-top: 12px;
  padding: 9px 12px;
  border: 1px solid var(--brass);
  border-radius: 999px;
  background: transparent;
}

.fc-to-compass-t {
  color: var(--jet);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 13px;
  font-weight: 500;
}

.fc-to-compass-c {
  color: var(--jet);
  font-size: 15px;
  opacity: 0.55;
}

.fc-check-k {
  display: block;
  margin: 12px 0 8px;
  color: var(--jet);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 12px;
  font-weight: 500;
  opacity: 0.65;
}

.fc-check-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px 12px;
}

.fc-check-item {
  display: flex;
  gap: 6px;
  align-items: center;
}

.fc-check-ok {
  color: var(--jade);
  font-size: 13px;
  font-weight: 700;
}

.fc-check-t {
  color: var(--jet);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 12.5px;
}

/* ── CTA / caption ── */
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

.fc-btn.disabled {
  opacity: 0.6;
}

.fc-btn.ghost {
  flex: none;
  padding: 0 26px;
  border: 1px solid var(--on-ink-a28);
  color: var(--on-ink);
}

.fc-caption {
  display: block;
  margin-bottom: 6px;
  color: var(--dim);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 12px;
  text-align: center;
}

.fc-spacer {
  flex: 1;
  min-height: 16px;
}
</style>
