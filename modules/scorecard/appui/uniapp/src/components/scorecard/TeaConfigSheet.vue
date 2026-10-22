<script setup lang="ts">
/**
 * 茶饭设置底部弹层（设计 09：茶饭设置 · 底部弹层）
 *
 * - 字段：开关（tea_auto_enabled）+ 目标（tea_target_amount）+ 抽取百分比（tea_percent，
 *   预设 5/10/15/20 + ±1 微调，钳 0–100）+ 单人累计上限（max_lose_score）
 * - 只读门：snapshot.creator_id !== myUserId（非房主）或 status !== 0（已结算/已关闭）
 *   → 全字段 disabled、不渲染保存按钮
 * - 保存：以打开时刻的 snapshot 为基线 diff 出 ConfigReq（指针语义：只传改动项；
 *   无改动 → 不调接口直接关层）→ updateConfig → applySnapshot（version 单调门）
 * - toast：响应 pending_apply_round_no > round_no →「已暂存，第 N 把生效」（优先）；
 *   数值项有改动 →「下一把生效」；仅开关 →「已生效」
 * - 本地业务错误 toast（ApiError 非 401/403）；色值零硬编码（主题 token + 全局工具类）；
 *   数字 .num（Oswald）；步进用 ChipButton ± 筹码（同支出页风格）
 */
import type { ConfigReq, RoomSnapshot } from '@/api/scorecard/types'
import { ApiError } from '@/api/core/handlers'
import { updateConfig } from '@/api/scorecard/room'
import ChipButton from '@/components/scorecard/ChipButton.vue'
import { useScorecardRoomStore } from '@/store/scorecard/room'

interface Props {
  snapshot: RoomSnapshot | null
  myUserId: number
  show: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
}>()

const toast = useToast()
const roomStore = useScorecardRoomStore()

/** 百分比步进（预设外的 ±1 微调）。 */
const PERCENT_STEP = 1
/** 目标 / 单人累计上限步进（筹码 ± 档）。 */
const AMOUNT_STEP = 10

/** 预设百分比（设计 09 .presets）。 */
const percentPresets = [5, 10, 15, 20]

/* ── 表单态（打开时从 snapshot 回填；基线存副本做 diff） ── */

const teaAuto = ref(false)
const teaTarget = ref(0)
const teaPercent = ref(0)
const maxLose = ref(0)

/** 打开时刻基线：diff-ConfigReq 的对照快照。 */
const baseline = ref({
  tea_auto_enabled: false,
  tea_target_amount: 0,
  tea_percent: 0,
  max_lose_score: 0,
})

const saving = ref(false)

/** 只读门：非房主 或 非进行中房间（status 0=进行中）。 */
const readonly = computed(() =>
  !props.snapshot
  || props.snapshot.creator_id !== props.myUserId
  || props.snapshot.status !== 0,
)

function clampPercent(n: number): number {
  return Math.min(100, Math.max(0, Math.round(n)))
}

/** 打开时回填表单 + 记录基线。 */
function resetFromSnapshot() {
  const snap = props.snapshot
  teaAuto.value = snap?.tea_auto_enabled ?? false
  teaTarget.value = snap?.tea_target_amount ?? 0
  teaPercent.value = clampPercent(snap?.tea_percent ?? 0)
  maxLose.value = snap?.max_lose_score ?? 0
  baseline.value = {
    tea_auto_enabled: teaAuto.value,
    tea_target_amount: teaTarget.value,
    tea_percent: teaPercent.value,
    max_lose_score: maxLose.value,
  }
}

watch(
  () => props.show,
  (show) => {
    if (show)
      resetFromSnapshot()
  },
  { immediate: true },
)

/* ── 步进 ── */

function stepTarget(step: number) {
  teaTarget.value = Math.max(0, teaTarget.value + step * AMOUNT_STEP)
}

function stepMaxLose(step: number) {
  maxLose.value = Math.max(0, maxLose.value + step * AMOUNT_STEP)
}

function stepPercent(step: number) {
  teaPercent.value = clampPercent(teaPercent.value + step * PERCENT_STEP)
}

function pickPercent(p: number) {
  teaPercent.value = clampPercent(p)
}

/* ── 保存 ── */

/** 业务信封错误本层提示；401/403（handlers 已清态全局提示）不重复 toast。 */
function toastApiError(err: unknown) {
  if (err instanceof ApiError && err.code !== 401 && err.code !== 403)
    toast.error({ msg: err.message || '请求失败' })
}

function close() {
  emit('update:show', false)
}

async function onSave() {
  if (saving.value || readonly.value || !props.snapshot)
    return

  const base = baseline.value
  const req: ConfigReq = {}
  const changedNumeric
    = teaTarget.value !== base.tea_target_amount
      || teaPercent.value !== base.tea_percent
      || maxLose.value !== base.max_lose_score
  const changedSwitch = teaAuto.value !== base.tea_auto_enabled

  if (teaTarget.value !== base.tea_target_amount)
    req.tea_target_amount = teaTarget.value
  if (teaPercent.value !== base.tea_percent)
    req.tea_percent = teaPercent.value
  if (teaAuto.value !== base.tea_auto_enabled)
    req.tea_auto_enabled = teaAuto.value
  if (maxLose.value !== base.max_lose_score)
    req.max_lose_score = maxLose.value

  // 无改动：跳过接口直接关层
  if (!changedNumeric && !changedSwitch) {
    close()
    return
  }

  saving.value = true
  try {
    const snap = await updateConfig(props.snapshot.id, req)
    // version 单调门：旧轮询响应不会覆盖本次结果
    roomStore.applySnapshot(snap)

    if (snap.pending_apply_round_no > snap.round_no)
      toast.info({ msg: `已暂存，第 ${snap.pending_apply_round_no} 把生效` })
    else if (changedNumeric)
      toast.success('下一把生效')
    else
      toast.success('已生效')
    close()
  }
  catch (err) {
    toastApiError(err)
  }
  finally {
    saving.value = false
  }
}
</script>

<template>
  <wd-popup
    :model-value="show"
    position="bottom"
    closable
    :safe-area-inset-bottom="true"
    custom-style="border-radius: 20px 20px 0 0; background: var(--ink-700);"
    @update:model-value="emit('update:show', $event)"
  >
    <view class="tea-sheet">
      <view class="ts-head">
        <text class="ts-title">
          茶饭设置
        </text>
        <text class="ts-sub">
          目标 &gt; 0 则开关开启
        </text>
      </view>

      <!-- 开启茶饭锅 -->
      <view class="srow">
        <view class="srow-text">
          <text class="sk">
            开启茶饭锅
          </text>
          <text class="sd">
            每次支出按比例抽取，记入茶/饭
          </text>
        </view>
        <wd-switch
          v-model="teaAuto"
          :disabled="readonly"
        />
      </view>

      <!-- 目标 -->
      <view class="srow">
        <view class="srow-text">
          <text class="sk">
            目标
          </text>
          <text class="sd">
            达到后停止抽取
          </text>
        </view>
        <view
          v-if="!readonly"
          class="mini-step"
        >
          <ChipButton
            size="sm"
            tone="dark"
            :step="1"
            @press="stepTarget(-$event)"
          >
            −
          </ChipButton>
          <text class="sv num">
            {{ teaTarget }}
          </text>
          <ChipButton
            size="sm"
            tone="brass"
            :step="1"
            @press="stepTarget($event)"
          >
            ＋
          </ChipButton>
        </view>
        <text
          v-else
          class="sv num"
        >
          {{ teaTarget }}
        </text>
      </view>

      <!-- 抽取百分比 -->
      <view class="pct-block">
        <text class="sk pct-label">
          抽取百分比
        </text>
        <view class="presets">
          <text
            v-for="p in percentPresets"
            :key="p"
            class="pc"
            :class="{ on: teaPercent === p, disabled: readonly }"
            @click="!readonly && pickPercent(p)"
          >
            {{ p }}%
          </text>
          <template v-if="!readonly">
            <ChipButton
              size="sm"
              tone="dark"
              :step="PERCENT_STEP"
              @press="stepPercent(-$event)"
            >
              −
            </ChipButton>
            <text class="sv num">
              {{ teaPercent }}%
            </text>
            <ChipButton
              size="sm"
              tone="brass"
              :step="PERCENT_STEP"
              @press="stepPercent($event)"
            >
              ＋
            </ChipButton>
          </template>
          <text
            v-else
            class="sv num"
          >
            {{ teaPercent }}%
          </text>
        </view>
      </view>

      <!-- 单人累计上限 -->
      <view class="srow">
        <view class="srow-text">
          <text class="sk">
            单人累计上限
          </text>
          <text class="sd">
            达到后全房提醒，不硬拦记分
          </text>
        </view>
        <view
          v-if="!readonly"
          class="mini-step"
        >
          <ChipButton
            size="sm"
            tone="dark"
            :step="1"
            @press="stepMaxLose(-$event)"
          >
            −
          </ChipButton>
          <text class="sv num">
            {{ maxLose }}
          </text>
          <ChipButton
            size="sm"
            tone="brass"
            :step="1"
            @press="stepMaxLose($event)"
          >
            ＋
          </ChipButton>
        </view>
        <text
          v-else
          class="sv num"
        >
          {{ maxLose }}
        </text>
      </view>

      <!-- 规则说明（设计 09 方式一 / 方式二） -->
      <view class="rule">
        <view class="rl">
          <text class="rl-k brass">
            方式一
          </text>
          <text class="rl-t">
            ：抽取百分比 &gt; 0% 时，每次支出记账，赢方按比例另行记为茶饭
          </text>
        </view>
        <view class="rl">
          <text class="rl-k jade">
            方式二
          </text>
          <text class="rl-t">
            ：房间左下角支出按钮，直接 DIY 数值记为茶饭
          </text>
        </view>
      </view>

      <button
        v-if="!readonly"
        class="btn brass sheet-cta"
        :disabled="saving"
        hover-class="none"
        @click="onSave"
      >
        {{ saving ? '保存中…' : '保存设置' }}
      </button>
      <text
        v-else
        class="ro-hint"
      >
        仅房主可编辑
      </text>
      <text class="foot-cap">
        只记分数，不涉及金钱
      </text>
    </view>
  </wd-popup>
</template>

<style lang="scss" scoped>
.tea-sheet {
  display: flex;
  flex-direction: column;
  padding: 20px 20px calc(env(safe-area-inset-bottom, 0px) + 16px);
  color: var(--on-ink);
}

.ts-head {
  display: flex;
  gap: 8px;
  align-items: baseline;
  justify-content: space-between;
  margin-bottom: 4px;
}

.ts-title {
  font-family: 'Noto Serif SC', serif;
  font-size: 19px;
  font-weight: 600;
  color: var(--on-ink);
}

.ts-sub {
  font-size: 12px;
  color: var(--on-ink);
  opacity: 0.6;
}

/* ── 行（设计 09 .srow） ── */
.srow {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  padding: 14px 0 2px;
}

.srow-text {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 3px;
}

.sk {
  font-size: 15.5px;
  font-weight: 500;
  color: var(--on-ink);
}

.sd {
  font-size: 12px;
  line-height: 1.5;
  color: var(--on-ink);
  opacity: 0.55;
}

/* ── ± 步进（同支出页筹码风格） ── */
.mini-step {
  display: flex;
  gap: 10px;
  align-items: center;
}

.sv {
  min-width: 48px;
  font-size: 18px;
  text-align: center;
  color: var(--on-ink);
}

/* ── 百分比预设（设计 09 .presets） ── */
.pct-block {
  padding-top: 15px;
}

.pct-label {
  display: block;
  margin-bottom: 10px;
  font-size: 14.5px;
}

.presets {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}

.pc {
  padding: 7px 14px;
  border: 1.5px solid var(--on-ink-a28);
  border-radius: 999px;
  font-family: 'Oswald', sans-serif;
  font-variant-numeric: tabular-nums;
  font-size: 14px;
  font-weight: 600;
  color: var(--on-ink);
  opacity: 0.75;

  &.on {
    border-color: var(--brass);
    background: var(--on-ink-a04);
    color: var(--brass);
    opacity: 1;
  }

  &.disabled {
    opacity: 0.4;
  }
}

/* ── 方式一 / 方式二说明 ── */
.rule {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-top: 14px;
}

.rl {
  display: flex;
  font-size: 12px;
  line-height: 1.75;
  color: var(--on-ink);
  opacity: 0.65;
}

.rl-t {
  flex: 1;
}

.rl-k {
  font-weight: 600;
  opacity: 1;

  &.brass {
    color: var(--brass);
  }

  &.jade {
    color: var(--jade-hi);
  }
}

/* ── CTA / 只读提示 / 底注 ── */
.sheet-cta {
  margin-top: 16px;
  height: 48px;
  font-size: 16px;

  &[disabled] {
    opacity: 0.6;
  }
}

.ro-hint {
  margin-top: 16px;
  font-size: 13px;
  text-align: center;
  color: var(--on-ink);
  opacity: 0.5;
}

.foot-cap {
  margin-top: 10px;
  font-size: 12px;
  text-align: center;
  color: var(--on-ink);
  opacity: 0.5;
}
</style>
