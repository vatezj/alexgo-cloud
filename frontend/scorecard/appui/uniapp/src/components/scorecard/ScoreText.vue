<script setup lang="ts">
/**
 * Oswald tabular 分数字：win/lose/jade 色走全局工具类（.num + .win-n/.lose-n/.jade-n）
 *
 * - signed=true（默认）按 +N / −N（U+2212 对齐源稿）/ 0 渲染差值
 * - 池量类非差值场景（茶饭池总数等）传 signed=false，直接显示原值
 * - 色值零硬编码：色类映射到主题 token（--win-dark / --lose-dark / --jade-hi）
 */
interface Props {
  /** 分数值 */
  value: number
  /** 色调：win=赢色 lose=输色 jade=茶池绿 brass=黄铜 none=正文色 */
  tone?: 'win' | 'lose' | 'jade' | 'brass' | 'none'
  /** 是否按 +N / −N 签名渲染（池量等传 false） */
  signed?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  tone: 'none',
  signed: true,
})

/** 签名文案：正 +N；负用减号 U+2212；零不带号 */
const text = computed(() => {
  if (props.signed) {
    if (props.value > 0)
      return `+${props.value}`
    if (props.value < 0)
      return `−${Math.abs(props.value)}`
    return '0'
  }
  return String(props.value)
})

const toneClass = computed(() => (props.tone === 'none' ? '' : `${props.tone}-n`))
</script>

<template>
  <text class="num score-text" :class="toneClass">
    {{ text }}
  </text>
</template>

<style lang="scss" scoped>
.score-text {
  line-height: 1;
}
</style>
