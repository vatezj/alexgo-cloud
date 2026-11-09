<script setup lang="ts">
import type { Rates, SalaryParams } from '@/utils/funSalary/calc'
import type { HolidayTable } from '@/utils/funSalary/holidays'
/**
 * 工资到账计算器 pages/fun/salary/index（设计稿 desgin/preview/salary-arrival-calculator.html v4）
 *
 * - 混合录入（修订轮 3.2）：月薪=数字输入框（唯一数据源 salaryStr，默认 15000）+「选」按钮
 *   开 wd-picker 轮选（3000–100000 步500，确认回填输入框）；四时间=轮选带默认（HH:MM 步5）
 * - 确认折叠两态机（openspec fun-salary-confirm-flow）：phase input=表单展开（编辑态
 *   结果区隐藏）/ result=摘要行+结果区；确认值 confirmed 与草稿分离，编辑不碰结果
 * - 口径锁定（openspec fun-salary）：21.75 法定月计薪天数 / 日薪 / 时薪 / 每秒（utils/funSalary/calc.ts）
 * - 工作日认节假日调休（openspec fun-salary-holiday-workdays）：假日表本地打包优先、
 *   缺年份回退远程 JSON，失败退化周一至周五；21.75 计薪口径不变
 * - hero「入账中」：今日入账金额每秒 +perSec 恒 2 位小数 + 今日工作时间 H:MM:SS 同频跳动
 *   （anchor+elapsed：参数变更重锚，无旧值突跳；卸载清定时器）
 * - 明细：本月入账通栏（整工作日×日薪 + 今日部分）+ 2×2；内容区滚动、底注钉底（复刻 v4）
 * - 纯本机计算：工资数据零网络零 storage（假日表只读 GET 公开 JSON，不外发用户数据）；
 *   文案红线禁 稳赚/躺赚/暴富/理财收益/投资回报
 */
import ModNav from '@/components/funCompass/ModNav.vue'
import { useSystemBar } from '@/composables/useSystemBar'
import { useScorecardShareConfigStore } from '@/store/scorecard/shareConfig'
import {
  calcRates,
  formatHMS,
  formatMoney,
  fullWorkdaysBeforeToday,
  groupInt,
  isWorkday,
  monthWorkdays,
  todayEarnings,
  todayWorkedSeconds,
  WORK_DAYS,
} from '@/utils/funSalary/calc'
import { loadHolidays } from '@/utils/funSalary/holidays'

definePage({
  style: {
    navigationStyle: 'custom',
  },
})

const router = useRouter()
const toast = useToast()
const { pageStyle } = useSystemBar()

/** 当年假日表（null=未加载/加载失败 → 判定退化周一至周五） */
const holidays = ref<HolidayTable | null>(null)

/* ── 选项生成（步长锁定 spec） ── */

interface Opt { label: string, value: string }

function pad2(n: number): string {
  return String(n).padStart(2, '0')
}

/** 月薪 3000–100000 步 500 */
const salaryOptions: Opt[] = Array.from({ length: (100000 - 3000) / 500 + 1 }, (_, i) => {
  const v = 3000 + i * 500
  return { label: `¥${v}`, value: String(v) }
})

/** 时间 HH:MM 步 5 分（00:00–23:55） */
const timeOptions: Opt[] = Array.from({ length: 1440 / 5 }, (_, i) => {
  const m = i * 5
  const label = `${pad2(Math.floor(m / 60))}:${pad2(m % 60)}`
  return { label, value: label }
})

/* ── 表单状态（修订轮 3.2：月薪输入框+轮选双通道带默认；时间轮选带默认） ── */

/** 月薪唯一数据源：输入框字符串（键入/轮选回填都写它） */
const salaryStr = ref('15000')
/** 轮选独立 v-model（确认后回填 salaryStr） */
const salaryPickV = ref<string[]>(['15000'])
const amStartV = ref(['09:00'])
const amEndV = ref(['12:00'])
const pmStartV = ref(['13:30'])
const pmEndV = ref(['18:00'])

const pick = reactive({
  salary: false,
  amStart: false,
  amEnd: false,
  pmStart: false,
  pmEnd: false,
})

/** 打开月薪轮选：先用当前输入值吸附到最近步长项，避免回显跳变 */
function openSalaryPicker() {
  const n = Number(salaryStr.value)
  if (Number.isFinite(n) && n >= 3000 && n <= 100000) {
    const snapped = Math.round(n / 500) * 500
    salaryPickV.value = [String(snapped)]
  }
  pick.salary = true
}

/** 轮选确认回填输入框 */
watch(salaryPickV, ([v]) => {
  if (v)
    salaryStr.value = v
})

/** 草稿参数（月薪读输入框、时间读 picker）；月薪空/非正数或时间缺 → null */
const draft = computed<SalaryParams | null>(() => {
  const salary = Number(salaryStr.value)
  const amStart = amStartV.value[0]
  const amEnd = amEndV.value[0]
  const pmStart = pmStartV.value[0]
  const pmEnd = pmEndV.value[0]
  if (!salaryStr.value.trim() || !Number.isFinite(salary) || salary <= 0)
    return null
  if (!amStart || !amEnd || !pmStart || !pmEnd)
    return null
  return { salary, amStart, amEnd, pmStart, pmEnd }
})

/* ── 两态机（design D1）：input=表单展开（结果区隐藏）/ result=摘要行+结果区 ── */

const phase = ref<'input' | 'result'>('input')

/** 确认值分离：结果区只反映 confirmed，编辑期间不碰结果 */
const confirmed = ref<SalaryParams | null>(null)
const rates = ref<Rates | null>(null)

/** 确认入账：月薪空/非法、时段倒挂 → toast 且保持展开；通过 → 写确认值、重锚、折叠 */
function onConfirm() {
  const p = draft.value
  if (!p) {
    // 时间恒有默认值，draft 为 null 几乎只可能是月薪空/非正数
    toast.warning({ msg: '请输入工资数额' })
    return
  }
  const fresh = calcRates(p)
  if (!fresh) {
    toast.warning({ msg: '下班时间要晚于上班时间' })
    return
  }
  confirmed.value = p
  rates.value = fresh
  rebase()
  phase.value = 'result'
}

/** 摘要行点击 → 重新展开编辑（编辑期间结果区隐藏） */
function expand() {
  phase.value = 'input'
}

/* ── hero 双跳动（design D4：anchor + elapsed，参数变更重锚） ── */

const anchorMs = ref(Date.now())
const anchorWorked = ref(0)
const anchorToday = ref(0)
const elapsedSec = ref(0)
let timer: ReturnType<typeof setInterval> | null = null

let anchorDay = ''

function rebase() {
  const p = confirmed.value
  if (!p || !rates.value)
    return
  const now = new Date()
  anchorDay = now.toDateString()
  anchorMs.value = now.getTime()
  anchorWorked.value = todayWorkedSeconds(now, p, holidays.value)
  anchorToday.value = todayEarnings(rates.value, anchorWorked.value)
  elapsedSec.value = 0
}

const displayedToday = computed(() =>
  anchorToday.value + elapsedSec.value * (rates.value?.perSec ?? 0))

const displayedWorked = computed(() => anchorWorked.value + elapsedSec.value)

const todayMoneyText = computed(() => formatMoney(displayedToday.value))
const workedText = computed(() => formatHMS(displayedWorked.value))
const perSecCaption = computed(() =>
  rates.value ? `每秒 +¥${rates.value.perSec.toFixed(4)}` : '—')

/** 时间胶囊占位：未选显 --:--（默认值下恒有值，保留兜底） */
function timeDisplay(v: string[]): string {
  return v[0] ?? '--:--'
}

/** 空态占位标记（时间默认值下恒 false，保留兜底） */
function timeEmpty(v: string[]): boolean {
  return !v[0]
}

/** 摘要行：¥15,000 · 09:00–12:00 · 13:30–18:00（result 态折叠头） */
const summaryText = computed(() => {
  const p = confirmed.value
  if (!p)
    return ''
  return `¥${groupInt(String(p.salary))} · ${p.amStart}–${p.amEnd} · ${p.pmStart}–${p.pmEnd}`
})

onMounted(() => {
  // 假日表异步就绪（本地打包通常同步命中）；就绪后若有确认值则重锚，
  // 让今日入账/本月口径从「周一至周五」切到「认节假日调休」
  loadHolidays(new Date().getFullYear()).then((table) => {
    holidays.value = table
    if (confirmed.value && rates.value)
      rebase()
  })

  timer = setInterval(() => {
    // 未确认（空态/编辑态）不跳动；结果区隐藏期间锚点不动
    if (!confirmed.value || !rates.value)
      return
    const now = new Date()
    // 跨日（或跨月口径）：重锚——今日工作时长/入账按新的一天重算，避免旧日基准漂移
    if (now.toDateString() !== anchorDay) {
      rebase()
      return
    }
    elapsedSec.value = Math.floor((now.getTime() - anchorMs.value) / 1000)
  }, 1000)
})

onUnmounted(() => {
  if (timer)
    clearInterval(timer)
  timer = null
})

/* ── 明细 ── */

const dayRateText = computed(() => (rates.value ? formatMoney(rates.value.dayRate) : '—'))
const hourRateText = computed(() => (rates.value ? formatMoney(rates.value.hourRate) : '—'))
const dailyHoursText = computed(() =>
  rates.value ? `${Number(rates.value.dailyHours.toFixed(2))}` : '—')

/** 本月 = 已过整工作日 × 日薪 + 今日（实时随 hero 走；认节假日调休） */
const monthMoneyText = computed(() => {
  if (!rates.value)
    return '—'
  const full = fullWorkdaysBeforeToday(new Date(), holidays.value)
  return formatMoney(full * rates.value.dayRate + displayedToday.value)
})

/**
 * 口径小字（openspec fun-salary-holiday-workdays）：
 * 假日表就绪 → 「本月 X 个工作日 · 第 N 个」（X=当月实际工作日总数，随月变化；
 * N=今天是第几个工作日，假日/补班修正后口径，周末或放假日为已过数）；
 * 未就绪 → 回退旧文案「本月第 N 个工作日计」。
 */
const monthDayCaption = computed(() => {
  const now = new Date()
  const full = fullWorkdaysBeforeToday(now, holidays.value)
  const n = isWorkday(now, holidays.value) ? full + 1 : full
  if (!holidays.value)
    return `本月第 ${n} 个工作日计`
  const total = monthWorkdays(now.getFullYear(), now.getMonth(), holidays.value)
  return `本月 ${total} 个工作日 · 第 ${n} 个`
})

const workDaysText = computed(() => String(WORK_DAYS))

/* ── 转发：title/图读后台配置（openspec share-card-config）；path 落本页、不带 inv ── */

const shareCfgStore = useScorecardShareConfigStore()

onShareAppMessage(() => {
  const card = shareCfgStore.get('salary')
  return {
    title: card.title,
    // 空串不传：微信回退当前页截图
    ...(card.imageUrl ? { imageUrl: card.imageUrl } : {}),
    path: '/pages/fun/salary/index',
  }
})

onShareTimeline(() => ({
  title: shareCfgStore.get('salary').title,
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
    router.replaceAll({ path: '/pages/mine/index' })
}
</script>

<template>
  <view
    class="sal-screen"
    :style="pageStyle"
  >
    <ModNav
      title="工资到账计算器"
      sub="趣味小工具 · 仅供娱乐"
      @back="goBack"
    />

    <!-- 可滚动内容区（复刻 v4：底注钉底、内容滚动） -->
    <view class="sal-content">
      <!-- input 态（空态/编辑态同构）：仅输入卡，结果区不渲染 -->
      <template v-if="phase === 'input'">
        <view class="sal-card">
          <text class="sal-card-t">
            填工资，看入账
          </text>

          <view class="sal-field">
            <text class="sal-k">
              月薪
            </text>
            <!-- 双通道：输入框打字 +「选」开轮选（回填输入框） -->
            <view class="sal-salary-row">
              <input
                v-model="salaryStr"
                class="sal-input"
                type="number"
                :maxlength="7"
                placeholder="请输入"
                placeholder-class="sal-input-ph"
              >
              <view
                class="sal-pick-btn"
                hover-class="sal-pill--hover"
                @click="openSalaryPicker"
              >
                <text class="sal-pick-btn-t">
                  选
                </text>
              </view>
            </view>
          </view>

          <view class="sal-field">
            <text class="sal-k">
              上午
            </text>
            <view
              class="sal-pill"
              hover-class="sal-pill--hover"
              @click="pick.amStart = true"
            >
              <text
                class="sal-pill-t"
                :class="{ 'sal-pill-t--ph': timeEmpty(amStartV) }"
              >
                {{ timeDisplay(amStartV) }}
              </text>
            </view>
            <text class="sal-dash">
              —
            </text>
            <view
              class="sal-pill"
              hover-class="sal-pill--hover"
              @click="pick.amEnd = true"
            >
              <text
                class="sal-pill-t"
                :class="{ 'sal-pill-t--ph': timeEmpty(amEndV) }"
              >
                {{ timeDisplay(amEndV) }}
              </text>
            </view>
          </view>

          <view class="sal-field">
            <text class="sal-k">
              下午
            </text>
            <view
              class="sal-pill"
              hover-class="sal-pill--hover"
              @click="pick.pmStart = true"
            >
              <text
                class="sal-pill-t"
                :class="{ 'sal-pill-t--ph': timeEmpty(pmStartV) }"
              >
                {{ timeDisplay(pmStartV) }}
              </text>
            </view>
            <text class="sal-dash">
              —
            </text>
            <view
              class="sal-pill"
              hover-class="sal-pill--hover"
              @click="pick.pmEnd = true"
            >
              <text
                class="sal-pill-t"
                :class="{ 'sal-pill-t--ph': timeEmpty(pmEndV) }"
              >
                {{ timeDisplay(pmEndV) }}
              </text>
            </view>
          </view>

          <view class="sal-priv">
            <view class="sal-priv-dot" />
            <text class="sal-priv-t">
              本机即时计算 · 数据不上传
            </text>
          </view>

          <!-- 确认入账（design：始终可点，toast 引导，不用 disabled） -->
          <view
            class="sal-confirm"
            hover-class="sal-pill--hover"
            @click="onConfirm"
          >
            <text class="sal-confirm-t">
              确认入账
            </text>
          </view>
        </view>
      </template>

      <!-- result 态：摘要行（点击再展开编辑） + 结果区 -->
      <template v-else>
        <view
          class="sal-summary"
          hover-class="sal-pill--hover"
          @click="expand"
        >
          <text class="sal-summary-t">
            {{ summaryText }}
          </text>
          <wd-icon
            css-icon
            class="sal-summary-ic"
            name="i-carbon-chevron-down"
          />
        </view>
      </template>

      <!-- hero：入账中（今日入账 + 今日工作时间 双跳；编辑期隐藏） -->
      <view v-if="phase === 'result'" class="sal-hero">
        <view class="sal-pulse-row">
          <view class="sal-pulse-dot" />
          <text class="sal-pulse-t">
            入账中…
          </text>
        </view>

        <text class="sal-amount">
          ¥{{ todayMoneyText }}
        </text>
        <view class="sal-hairline" />
        <text class="sal-hero-cap">
          今日入账 · {{ perSecCaption }}
        </text>

        <text class="sal-work-k">
          今日工作时间
        </text>
        <text class="sal-work">
          {{ workedText }}
        </text>
      </view>

      <!-- 明细：本月通栏 + 2×2（编辑期隐藏） -->
      <view v-if="phase === 'result'" class="sal-grid">
        <view class="sal-cell wide">
          <view class="sal-cell-main">
            <text class="sal-cell-k">
              本月入账
            </text>
            <text class="sal-cell-sub">
              {{ monthDayCaption }}
            </text>
          </view>
          <text class="sal-cell-v brass">
            ¥{{ monthMoneyText }}
          </text>
        </view>

        <view class="sal-cell">
          <text class="sal-cell-k">
            每月工作日 <text class="sal-info">
              ⓘ
            </text>
          </text>
          <text class="sal-cell-sub">
            法定月计薪天数
          </text>
          <text class="sal-cell-v">
            {{ workDaysText }}<text class="sal-unit">
              天
            </text>
          </text>
        </view>

        <view class="sal-cell">
          <text class="sal-cell-k">
            日薪
          </text>
          <text class="sal-cell-v">
            ¥{{ dayRateText }}
          </text>
        </view>

        <view class="sal-cell">
          <text class="sal-cell-k">
            每日工时
          </text>
          <text class="sal-cell-v">
            {{ dailyHoursText }}<text class="sal-unit">
              h
            </text>
          </text>
        </view>

        <view class="sal-cell">
          <text class="sal-cell-k">
            时薪
          </text>
          <text class="sal-cell-v">
            ¥{{ hourRateText }}
          </text>
        </view>
      </view>
    </view>

    <!-- 底注钉底 -->
    <view class="sal-footer">
      <text class="sal-footer-t">
        结果仅供娱乐参考 · 算薪请以实际合同为准
      </text>
    </view>

    <!-- 选择器：月薪 -->
    <wd-picker
      v-model="salaryPickV"
      v-model:visible="pick.salary"
      :columns="salaryOptions"
      title="选择月薪"
    />
    <!-- 选择器：四时间（步 5 分） -->
    <wd-picker
      v-model="amStartV"
      v-model:visible="pick.amStart"
      :columns="timeOptions"
      title="上午上班时间"
    />
    <wd-picker
      v-model="amEndV"
      v-model:visible="pick.amEnd"
      :columns="timeOptions"
      title="上午下班时间"
    />
    <wd-picker
      v-model="pmStartV"
      v-model:visible="pick.pmStart"
      :columns="timeOptions"
      title="下午上班时间"
    />
    <wd-picker
      v-model="pmEndV"
      v-model:visible="pick.pmEnd"
      :columns="timeOptions"
      title="下午下班时间"
    />
  </view>
</template>

<style lang="scss" scoped>
.sal-screen {
  display: flex;
  box-sizing: border-box;
  flex-direction: column;
  height: 100vh;
  padding: calc(var(--status-bar-h, env(safe-area-inset-top, 0px)) + 6px) 0 calc(env(safe-area-inset-bottom, 0px));
  background: var(--ink-900);
  background-image: var(--spot);
  color: var(--on-ink);
  overflow: hidden;
}

/* 内容区滚动（nav/底注钉住） */
.sal-content {
  flex: 1;
  min-height: 0;
  padding: 6px 16px 12px;
  overflow-y: auto;
}

/* 宽屏（桌面 H5）：收进 480 手机栏居中，底/侧仍是全幅墨底。
   box-sizing 必须显式 border-box：content/footer 带水平 padding，
   content-box 下 width:100% 不含 padding → 窄屏（小程序/390px）总宽溢出右边。 */
.fcn-nav,
.sal-content,
.sal-footer {
  box-sizing: border-box;
  width: 100%;
  max-width: 480px;
  margin-inline: auto;
}

/* ── 输入卡（骨白） ── */
.sal-card {
  margin-bottom: 14px;
  padding: 18px 14px 14px;
  border-radius: var(--r-tile);
  background: var(--tile);
  box-shadow: var(--shadow-tile);
  color: var(--jet);
}

.sal-card-t {
  display: block;
  margin-bottom: 14px;
  color: var(--jet);
  font-family: 'Noto Serif SC', serif;
  font-size: 21px;
  font-weight: 700;
  letter-spacing: 0.03em;
}

.sal-field {
  display: flex;
  gap: 10px;
  align-items: center;
  margin-bottom: 10px;
}

.sal-k {
  flex: none;
  width: 34px;
  color: var(--jet);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 14px;
  font-weight: 500;
}

.sal-pill {
  display: flex;
  flex: 1;
  height: 44px;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  background: var(--ink-700);
}

/* 按压反馈（hover-class，MP/H5 通用） */
.sal-pill--hover {
  opacity: 0.82;
  transform: scale(0.98);
}

.sal-pill-t {
  color: var(--on-ink);
  font-family: 'Oswald', sans-serif;
  font-size: 19px;
  font-weight: 600;
  letter-spacing: 0.04em;
}

/* ── 月薪双通道：输入框 +「选」按钮（修订轮 3.2） ── */
.sal-salary-row {
  display: flex;
  flex: 1;
  gap: 8px;
  align-items: center;
  min-width: 0;
}

.sal-input {
  box-sizing: border-box;
  flex: 1;
  min-width: 0;
  height: 44px;
  padding: 0 16px;
  border-radius: 999px;
  background: var(--ink-700);
  color: var(--on-ink);
  font-family: 'Oswald', sans-serif;
  font-size: 19px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-align: center;
}

.sal-input-ph {
  color: var(--dim);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 15px;
  font-weight: 400;
}

.sal-pick-btn {
  display: flex;
  flex: none;
  width: 44px;
  height: 44px;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  background: var(--ink-700);
  border: 1px solid var(--on-ink-a28);
}

.sal-pick-btn-t {
  color: var(--brass);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 15px;
  font-weight: 500;
}

/* 空态占位（dim 色） */
.sal-pill-t--ph {
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 15px;
  font-weight: 400;
  opacity: 0.45;
}

.sal-dash {
  flex: none;
  color: var(--jet);
  font-size: 16px;
  opacity: 0.5;
}

.sal-priv {
  display: flex;
  gap: 6px;
  align-items: center;
  margin-top: 12px;
}

.sal-priv-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--jade);
}

.sal-priv-t {
  color: var(--jet);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 11.5px;
  opacity: 0.65;
}

/* ── 确认入账（卡内深底 → 骨白主按钮层级） ── */
.sal-confirm {
  display: flex;
  height: 46px;
  align-items: center;
  justify-content: center;
  margin-top: 14px;
  border-radius: 999px;
  background: var(--ink-700);
}

.sal-confirm-t {
  color: var(--on-ink);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 15.5px;
  font-weight: 500;
  letter-spacing: 0.05em;
}

/* ── 摘要行（result 态折叠头，点击再展开） ── */
.sal-summary {
  display: flex;
  gap: 8px;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
  padding: 14px;
  border-radius: var(--r-tile);
  background: var(--tile);
  box-shadow: var(--shadow-tile);
  color: var(--jet);
}

.sal-summary-t {
  flex: 1;
  min-width: 0;
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 12.5px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.sal-summary-ic {
  flex: none;
  color: var(--jet);
  opacity: 0.55;
}

/* ── hero 入账卡 ── */
.sal-hero {
  position: relative;
  margin-bottom: 14px;
  padding: 16px 14px 18px;
  border-radius: var(--r-tile);
  background: var(--ink-700);
  box-shadow: var(--shadow-tile);
  text-align: center;
  overflow: hidden;
}

.sal-hero::before {
  position: absolute;
  top: 0;
  right: 0;
  left: 0;
  height: 14px;
  background: repeating-linear-gradient(
    45deg,
    rgba(201, 162, 75, 0.55) 0 4px,
    transparent 4px 8px
  );
  content: '';
  opacity: 0.65;
}

.sal-pulse-row {
  display: flex;
  gap: 7px;
  align-items: center;
  justify-content: flex-start;
  margin-bottom: 8px;
}

.sal-pulse-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--brass);
  animation: sal-pulse 1.2s ease-in-out infinite;
}

@keyframes sal-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.35; }
}

.sal-pulse-t {
  color: var(--dim);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 12.5px;
}

.sal-amount {
  display: block;
  color: var(--brass);
  font-family: 'Oswald', sans-serif;
  font-size: 46px;
  font-weight: 700;
  letter-spacing: 0.02em;
  line-height: 1.15;
}

.sal-hairline {
  width: 62%;
  height: 1px;
  margin: 10px auto 8px;
  background: linear-gradient(
    90deg,
    transparent 0%,
    rgba(201, 162, 75, 0.15) 45%,
    rgba(201, 162, 75, 0.9) 50%,
    rgba(201, 162, 75, 0.15) 55%,
    transparent 100%
  );
  background-size: 200% 100%;
  animation: sal-shimmer 2.6s linear infinite;
}

@keyframes sal-shimmer {
  from { background-position: 200% 0; }
  to { background-position: -200% 0; }
}

.sal-hero-cap {
  display: block;
  color: var(--dim);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 12px;
}

.sal-work-k {
  display: block;
  margin-top: 12px;
  color: var(--dim);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 12px;
}

.sal-work {
  display: block;
  margin-top: 2px;
  color: var(--on-ink);
  font-family: 'Oswald', sans-serif;
  font-size: 26px;
  font-weight: 600;
  letter-spacing: 0.06em;
}

/* ── 明细网格 ── */
.sal-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}

.sal-cell {
  display: flex;
  flex-direction: column;
  gap: 3px;
  padding: 13px 12px;
  border-radius: 14px;
  background: var(--tile);
  box-shadow: var(--shadow-tile);
  color: var(--jet);
}

.sal-cell.wide {
  display: flex;
  grid-column: 1 / -1;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  padding: 14px;
}

.sal-cell-main {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.sal-cell-k {
  color: var(--jet);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 12.5px;
  opacity: 0.75;
}

.sal-info {
  font-size: 11px;
  opacity: 0.55;
}

.sal-cell-sub {
  color: var(--jet);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 10.5px;
  opacity: 0.5;
}

.sal-cell-v {
  color: var(--jet);
  font-family: 'Oswald', sans-serif;
  font-size: 27px;
  font-weight: 600;
}

.sal-cell-v.brass {
  color: var(--brass);
  font-size: 30px;
}

.sal-unit {
  margin-left: 3px;
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 13px;
  font-weight: 400;
  opacity: 0.6;
}

/* ── 底注（钉底） ── */
.sal-footer {
  flex: none;
  padding: 10px 16px 8px;
  text-align: center;
}

.sal-footer-t {
  color: var(--dim);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 11.5px;
}
</style>
