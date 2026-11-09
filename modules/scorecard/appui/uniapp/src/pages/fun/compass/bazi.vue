<script setup lang="ts">
import type { CalcRecommendation } from '@/utils/funCompass/zhiwei'
/**
 * 生辰录入 & 排盘页 pages/fun/compass/bazi（2026-11-06 修订轮）
 *
 * - 表单：年/月/日 + 时间 picker(时:分) + 性别 chip(默认男) + 称呼；数据用途行 = copy LOCAL_ONLY
 * - CTA「排八字 · 看推荐」→ 后端 zhiwei POST /api/calc（zhiwei.ts）→ top3 推荐列表
 * - 「存入档案」写 fun_compass_archives（schema v2：hour/minute/gender）；query.id 首次 onShow 预填
 * - 无任何本地排盘计算；文案锁定 copy.ts；底部免责 ModDisclaimer
 */
import ModDisclaimer from '@/components/funCompass/ModDisclaimer.vue'
import ModNav from '@/components/funCompass/ModNav.vue'
import { useSystemBar } from '@/composables/useSystemBar'
import {
  useFunCompassArchivesStore,
} from '@/store/funCompass/archives'
import {
  LOCAL_ONLY,
} from '@/utils/funCompass/copy'
import { calcRecommend } from '@/utils/funCompass/zhiwei'

definePage({
  style: {
    navigationStyle: 'custom',
  },
})

const router = useRouter()
const toast = useToast()
const { pageStyle } = useSystemBar()
const store = useFunCompassArchivesStore()

/* ── 表单状态 ── */

const yearStr = ref('1996')
const monthStr = ref('8')
const dayStr = ref('15')
const timeStr = ref('14:30')
const gender = ref<'male' | 'female'>('male')
const nameStr = ref('')

const recommendations = ref<CalcRecommendation[] | null>(null)
const computing = ref(false)

/** 编辑模式：档案列表「编辑」带 query.id 进入，首次 onShow 预填（once，防回显覆盖改动） */
const editId = ref('')
let prefilled = false

function readQueryId(): string {
  const raw = router.currentRoute.value.query.id
  const str = Array.isArray(raw) ? (raw[0] ?? '') : (raw ?? '')
  return str
}

function pad2(n: number): string {
  return String(n).padStart(2, '0')
}

onShow(() => {
  if (prefilled)
    return
  prefilled = true
  const id = readQueryId()
  if (!id)
    return
  const archive = store.byId(id)
  if (!archive)
    return
  editId.value = id
  yearStr.value = String(archive.year)
  monthStr.value = String(archive.month)
  dayStr.value = String(archive.day)
  timeStr.value = `${pad2(archive.hour)}:${pad2(archive.minute)}`
  gender.value = archive.gender
  nameStr.value = archive.name
})

function onTimeChange(e: { detail: { value: string } }) {
  const v = e.detail?.value
  if (typeof v === 'string' && /^\d{1,2}:\d{2}$/.test(v))
    timeStr.value = v
}

/* ── 校验与排盘 ── */

function parseForm(): { year: number, month: number, day: number, hour: number, minute: number } | null {
  const year = Number(yearStr.value)
  const month = Number(monthStr.value)
  const day = Number(dayStr.value)
  const [hStr, mStr] = timeStr.value.split(':')
  const hour = Number(hStr)
  const minute = Number(mStr)
  if (!Number.isInteger(year) || year < 1900 || year > 2100) {
    toast.error({ msg: '年份填 1900–2100 之间' })
    return null
  }
  if (!Number.isInteger(month) || month < 1 || month > 12) {
    toast.error({ msg: '月份填 1–12' })
    return null
  }
  if (!Number.isInteger(day) || day < 1 || day > 31) {
    toast.error({ msg: '日期填 1–31' })
    return null
  }
  if (!Number.isInteger(hour) || hour < 0 || hour > 23 || !Number.isInteger(minute) || minute < 0 || minute > 59) {
    toast.error({ msg: '时间格式不对' })
    return null
  }
  return { year, month, day, hour, minute }
}

async function onCompute() {
  const form = parseForm()
  if (!form || computing.value)
    return
  computing.value = true
  try {
    const res = await calcRecommend({ ...form, gender: gender.value })
    recommendations.value = res.recommendations
    if (res.recommendations.length === 0)
      toast.info({ msg: '本次无显著方位，随缘落座' })
  }
  catch (err) {
    toast.error({ msg: err instanceof Error ? err.message : '排盘失败，请重试' })
  }
  finally {
    computing.value = false
  }
}

function onSaveArchive() {
  const form = parseForm()
  if (!form)
    return
  const draft = {
    name: nameStr.value.trim() || '未命名',
    ...form,
    gender: gender.value,
  }
  if (editId.value)
    store.update(editId.value, draft)
  else
    store.add(draft)
  toast.success({ msg: editId.value ? '档案已更新' : '已存入档案' })
  goBack()
}

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
      title="生辰排盘"
      @back="goBack"
    />

    <!-- 生辰表单卡 -->
    <view class="fc-form-card">
      <text class="fc-form-t">
        填生辰，排个盘
      </text>
      <view class="fc-priv">
        <view class="fc-priv-dot" />
        <text class="fc-priv-t">
          {{ LOCAL_ONLY }}
        </text>
      </view>

      <view class="fc-field">
        <text class="fc-field-k">
          年
        </text>
        <input
          v-model="yearStr"
          class="fc-input"
          type="number"
          :maxlength="4"
        >
      </view>
      <view class="fc-field">
        <text class="fc-field-k">
          月
        </text>
        <input
          v-model="monthStr"
          class="fc-input"
          type="number"
          :maxlength="2"
        >
      </view>
      <view class="fc-field">
        <text class="fc-field-k">
          日
        </text>
        <input
          v-model="dayStr"
          class="fc-input"
          type="number"
          :maxlength="2"
        >
      </view>

      <view class="fc-field">
        <text class="fc-field-k">
          时间
        </text>
        <picker
          type="time"
          :value="timeStr"
          @change="onTimeChange"
        >
          <view class="fc-input fc-picker">
            {{ timeStr }}
          </view>
        </picker>
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

      <view class="fc-field">
        <text class="fc-field-k">
          称呼
        </text>
        <input
          v-model="nameStr"
          class="fc-input"
          :maxlength="12"
          placeholder="如：老王"
          placeholder-class="fc-input-ph"
        >
      </view>
    </view>

    <!-- 排盘结果（点 CTA 后出现）：zhiwei top3 -->
    <view
      v-if="recommendations"
      class="fc-form-card"
    >
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
        <text
          v-if="r.rank === 1"
          class="fc-seal"
        >
          参考
        </text>
      </view>
    </view>

    <view class="fc-spacer" />

    <!-- CTA 拇指区 -->
    <view class="fc-cta-row">
      <view
        class="fc-btn primary"
        :class="{ disabled: computing }"
        @click="onCompute"
      >
        {{ computing ? '排盘中…' : '排八字 · 看推荐' }}
      </view>
      <view
        class="fc-btn ghost"
        @click="onSaveArchive"
      >
        存入档案
      </view>
    </view>

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

/* 骨白表单/结果卡 */
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

/* 字段行：标签 + 深色胶囊输入 */
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

.fc-input-ph {
  color: var(--dim);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 13px;
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

/* ── 结果：top3 推荐 ── */
.fc-res-t {
  display: block;
  margin-bottom: 10px;
  color: var(--jet);
  font-family: 'Noto Serif SC', serif;
  font-size: 16.5px;
  font-weight: 700;
}

.fc-res-empty {
  padding: 12px 4px;
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
  padding: 10px 0;
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

.fc-seal {
  flex: none;
  padding: 2px 8px;
  border-radius: 6px;
  background: var(--vermillion);
  color: var(--tile);
  font-family: 'Noto Serif SC', serif;
  font-size: 11px;
  font-weight: 700;
}

.fc-spacer {
  flex: 1;
  min-height: 16px;
}

/* CTA */
.fc-cta-row {
  display: flex;
  gap: 10px;
  margin-bottom: 6px;
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
  padding: 0 22px;
  border: 1px solid var(--on-ink-a28);
  color: var(--on-ink);
}
</style>
