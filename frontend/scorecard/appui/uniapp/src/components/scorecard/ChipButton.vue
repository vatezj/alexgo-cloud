<script setup lang="ts">
/**
 * ±筹码钮（设计 04 .chip）：点按一次 emit press(step)；按住 500ms 后每 200ms 连发
 *
 * - tone：dark=减档（筹码暗面）/ brass=加档（黄铜面），映射全局 .chip 工具类（零硬编码色）
 * - size：sm / md（全局 .chip.sm 收尺寸；字面字号本组件 scoped 控制）
 * - 方向不在组件内：加/减由父组件对 press 载荷的用法决定（− 实例收到正 step 后自减）
 * - 触摸流：touchstart.prevent 即触发并阻断合成 mouse 事件；桌面 H5 无 touch 走 mousedown 兜底
 *   （mp 端绑定 mouse 事件无害不触发）；beginPress 重入守卫防 touch+mouse 双触发连发
 */
interface Props {
  /** 每次 press 携带的步长 */
  step?: number
  /** 尺寸档 */
  size?: 'sm' | 'md'
  /** 色调：dark（减）/ brass（加） */
  tone?: 'dark' | 'brass'
}

const props = withDefaults(defineProps<Props>(), {
  step: 10,
  size: 'md',
  tone: 'brass',
})

const emit = defineEmits<{
  (e: 'press', step: number): void
}>()

/** 长按阈值：按住 500ms 后进入连发。 */
const HOLD_DELAY_MS = 500
/** 连发间隔：进入连发后每 200ms 一发。 */
const HOLD_INTERVAL_MS = 200

let delayTimer: ReturnType<typeof setTimeout> | null = null
let repeatTimer: ReturnType<typeof setInterval> | null = null

function clearTimers() {
  if (delayTimer) {
    clearTimeout(delayTimer)
    delayTimer = null
  }
  if (repeatTimer) {
    clearInterval(repeatTimer)
    repeatTimer = null
  }
}

/** 按下：立即一发 + 挂长按连发（重入守卫防 touch/mouse 双流叠加）。 */
function beginPress() {
  if (delayTimer || repeatTimer)
    return
  emit('press', props.step)
  delayTimer = setTimeout(() => {
    delayTimer = null
    repeatTimer = setInterval(() => emit('press', props.step), HOLD_INTERVAL_MS)
  }, HOLD_DELAY_MS)
}

/** 抬起/移出/取消：停连发。 */
function endPress() {
  clearTimers()
}

onBeforeUnmount(clearTimers)
</script>

<template>
  <button
    class="chip-btn chip"
    :class="[tone, size]"
    hover-class="none"
    @touchstart.prevent="beginPress"
    @touchend="endPress"
    @touchcancel="endPress"
    @mousedown="beginPress"
    @mouseup="endPress"
    @mouseleave="endPress"
  >
    <text class="glyph">
      <slot />
    </text>
  </button>
</template>

<style lang="scss" scoped>
.chip-btn {
  padding: 0;
}

/* 小程序 button 默认 ::after 描边去除（同全局 .btn 处理） */
.chip-btn::after {
  border: none;
}

.glyph {
  position: relative;
  z-index: 1;
  font-family: 'Noto Sans SC', sans-serif;
  font-weight: 700;
  line-height: 1;
}

.chip-btn.sm .glyph {
  font-size: 20px;
}

.chip-btn.md .glyph {
  font-size: 24px;
}
</style>
