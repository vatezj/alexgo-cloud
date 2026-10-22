<script setup lang="ts">
/**
 * 扑克牌底衬组件：pcard 底衬 + 顶部花色条纹 + 对角角标（视觉对齐 breakout `.pcard`）
 *
 * - 色值全走主题 token（--tile / --jet / --brass / --vermillion / --win-dark），零硬编码色
 * - 角标红桃/方块走锁定红；条纹色随花色（红桃/方块=锁定红）或 tone（brass=黄铜、win=赢色）
 * - 牌面正文走默认插槽（身份牌等自定义内容），label 为居中文案快捷方式
 */
interface Props {
  /** 花色符号（♠♥♦♣） */
  suit?: string
  /** 角标点数 */
  rank?: string
  /** 牌面中心文案（有插槽内容时可省） */
  label?: string
  /** 牌面基调：tile=骨白牌（默认） brass=黄铜牌 win=赢牌（骨白底+赢色条纹） */
  tone?: 'tile' | 'brass' | 'win'
}

const props = withDefaults(defineProps<Props>(), {
  suit: '♠',
  rank: 'A',
  label: '',
  tone: 'tile',
})

/** 红桃/方块：角标走锁定红，条纹同色（源稿 pcard 规则） */
const isRed = computed(() => props.suit === '♥' || props.suit === '♦')

/** 顶部条纹花色色（--suit 变量喂给全局 .pcard .strip 的 border-bottom） */
const stripSuit = computed(() => {
  if (props.tone === 'win')
    return 'var(--win-dark)'
  if (props.tone === 'brass')
    return 'var(--brass)'
  return isRed.value ? 'var(--vermillion)' : 'var(--jet)'
})
</script>

<template>
  <view
    class="pcard play-card"
    :class="`tone-${tone}`"
    :style="{ '--suit': stripSuit }"
  >
    <view class="strip" />
    <text
      class="idx tl"
      :class="{ red: isRed }"
    >
      {{ rank }}<text class="s">
        {{ suit }}
      </text>
    </text>
    <text
      class="idx br"
      :class="{ red: isRed }"
    >
      {{ rank }}<text class="s">
        {{ suit }}
      </text>
    </text>
    <text
      v-if="label"
      class="center-label"
    >
      {{ label }}
    </text>
    <slot />
  </view>
</template>

<style lang="scss" scoped>
.play-card.tone-brass {
  background: linear-gradient(180deg, var(--brass-hi), var(--brass));
  color: var(--ink-900);
}

.play-card.tone-brass .center-label {
  color: var(--ink-900);
}

.center-label {
  display: block;
  font-family: 'Noto Serif SC', serif;
  font-size: 18px;
  font-weight: 700;
  color: var(--jet);
}
</style>
