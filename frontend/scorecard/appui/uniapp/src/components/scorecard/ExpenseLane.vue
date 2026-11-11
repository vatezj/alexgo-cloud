<script setup lang="ts">
/**
 * 支出档（设计 04 .lane）：分数档 / 茶饭档共用
 *
 * - lane=score 黄铜顶线（--lane=--brass）；lane=pot 茶池绿顶线（--lane=--jade-hi）
 * - 结构：档名+徽标 | who（支出方头像/茶杯） + ChipButton −/＋ + 井字大数字（可键入） | 预设 chips
 * - 大数字为 number input：直接键入整数，blur/外部改值时归一；−/＋ 与预设 chips 为快捷方式
 * - 值只增减不落下限：键入/减档均 clamp ≥ 0；提交门（score>0）由页面把守
 * - 数字 Oswald tabular（全局 .num）；色值零硬编码（--lane 由 lane 档映射主题 token）
 */
import ChipButton from './ChipButton.vue'

interface Props {
  /** 档名（支出分数 / 支出茶·饭） */
  title: string
  /** 右上徽标（SCORE / 茶饭锅） */
  tag: string
  /** 当前值 */
  value: number
  /** 预设 chips */
  presets: number[]
  /** ± 步进 */
  step?: number
  /** 档类：score=分数档 brass / pot=茶饭档 jade */
  lane: 'score' | 'pot'
  /** 分数档：支出方头像（空 = 昵称首字兜底） */
  whoAvatar?: string
  /** 分数档：支出方昵称 */
  whoName?: string
}

const props = withDefaults(defineProps<Props>(), {
  step: 10,
  whoAvatar: '',
  whoName: '',
})

const emit = defineEmits<{
  (e: 'update:value', value: number): void
}>()

/** 头像兜底字：昵称首字（空昵称用「支」）。 */
const initial = computed(() => (props.whoName.slice(0, 1) || '支').toUpperCase())

/** 键入草稿：允许清空重打；与 value 同步由 watch + blur 归一。 */
const draft = ref(String(props.value))

watch(
  () => props.value,
  (v) => {
    if (Number(draft.value) !== v)
      draft.value = String(v)
  },
)

/** 键入：剥非数字 → floor → emit（空/非法记 0，不落下限）。 */
function onDraftInput(e: { detail: { value: string } }) {
  const raw = e.detail.value.replace(/\D/g, '')
  draft.value = raw
  emit('update:value', Math.floor(Number(raw) || 0))
}

function onDraftBlur() {
  draft.value = String(props.value)
}

function onMinus(step: number) {
  emit('update:value', Math.max(0, props.value - step))
}

function onPlus(step: number) {
  emit('update:value', props.value + step)
}

function onPreset(p: number) {
  emit('update:value', p)
}
</script>

<template>
  <view
    class="lane"
    :class="lane"
  >
    <view class="lh">
      <text class="lt">
        {{ title }}
      </text>
      <text class="ltag">
        {{ tag }}
      </text>
    </view>

    <view class="lbody">
      <view
        class="who"
        :class="{ cup: lane === 'pot' }"
      >
        <view class="w-av">
          <image
            v-if="lane === 'score' && whoAvatar"
            class="w-img"
            :src="whoAvatar"
            mode="aspectFill"
          />
          <text
            v-else-if="lane === 'score'"
            class="w-txt"
          >
            {{ initial }}
          </text>
          <wd-icon
            v-else
            css-icon
            class="w-cup"
            name="i-carbon-cafe"
          />
        </view>
        <text class="w-n">
          {{ lane === 'pot' ? '茶/饭' : (whoName || '我') }}
        </text>
      </view>

      <ChipButton
        size="sm"
        tone="dark"
        :step="step"
        @press="onMinus"
      >
        −
      </ChipButton>

      <view class="well">
        <input
          v-model="draft"
          class="num rv well-in"
          type="number"
          :maxlength="6"
          @input="onDraftInput"
          @blur="onDraftBlur"
        >
        <text class="ru">
          分
        </text>
      </view>

      <ChipButton
        size="sm"
        tone="brass"
        :step="step"
        @press="onPlus"
      >
        ＋
      </ChipButton>
    </view>

    <view class="presets">
      <view
        v-for="p in presets"
        :key="p"
        class="pc"
        :class="{ on: value === p }"
        @click="onPreset(p)"
      >
        <text class="num">
          {{ p }}
        </text>
      </view>
    </view>
  </view>
</template>

<style lang="scss" scoped>
.lane {
  --lane: var(--brass);

  position: relative;
  margin: 12px 14px 0;
  padding: 14px 14px 13px;
  border: 1px solid var(--on-ink-a25);
  border-radius: 20px;
  background: var(--ink-700);
}

.lane.pot {
  --lane: var(--jade-hi);
}

/* 顶缘档色渐变线（设计 .lane::before） */
.lane::before {
  position: absolute;
  top: 0;
  right: 18px;
  left: 18px;
  height: 2px;
  background: linear-gradient(90deg, transparent, var(--lane), transparent);
  content: '';
  opacity: 0.7;
}

.lh {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 11px;
}

.lt {
  color: var(--on-ink);
  font-size: 14px;
  font-weight: 500;
}

.ltag {
  padding: 1px 7px;
  border: 1px solid var(--lane);
  border-radius: 4px;
  color: var(--lane);
  font-size: 10px;
  letter-spacing: 0.08em;
  opacity: 0.85;
}

.lbody {
  display: flex;
  align-items: center;
  gap: 10px;
}

.who {
  display: flex;
  width: 52px;
  flex: none;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}

.w-av {
  display: flex;
  width: 44px;
  height: 44px;
  align-items: center;
  justify-content: center;
  border: 2px solid var(--lane);
  border-radius: 50%;
  background: var(--ink-600);
  color: var(--lane);
  font-size: 14px;
  font-weight: 500;
  overflow: hidden;
}

.who.cup .w-av {
  font-size: 19px;
}

.w-img {
  width: 100%;
  height: 100%;
  border-radius: 50%;
}

.w-n {
  max-width: 52px;
  overflow: hidden;
  color: var(--dim);
  font-size: 10px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.well {
  display: flex;
  height: 62px;
  flex: 1;
  align-items: center;
  justify-content: center;
  gap: 7px;
  border-radius: 14px;
  background: var(--ink-900);
  box-shadow: inset 0 3px 8px var(--chip-dark-a);
}

.rv {
  color: var(--lane);
  font-size: 38px;
  line-height: 1;
}

.well-in {
  width: auto;
  min-width: 48px;
  max-width: 110px;
  flex: 1;
  padding: 0;
  border: none;
  background: transparent;
  text-align: center;
  // 小程序 input 默认字体/内边距抹平，保持井字大数字观感
  font-family: 'Oswald', sans-serif;
  font-variant-numeric: tabular-nums;
  font-weight: 600;
  line-height: 1;
}

.ru {
  align-self: flex-end;
  padding-bottom: 9px;
  color: var(--dim);
  font-size: 12px;
}

.presets {
  display: flex;
  gap: 8px;
  margin-top: 11px;
}

.pc {
  display: flex;
  height: 34px;
  flex: 1;
  align-items: center;
  justify-content: center;
  border: 1px dashed var(--on-ink-a28);
  border-radius: 999px;
  background: var(--on-ink-a04);
  color: var(--dim);
  cursor: pointer;
  font-size: 15px;
}

.pc.on {
  border-color: var(--brass);
  border-style: solid;
  background: var(--on-ink-a04);
  color: var(--brass);
}
</style>
