<script lang="ts" setup>
/**
 * 运势头部渐变卡（迁自 luckyU components/fortune/FortuneHeader.vue）
 *
 * - 类型换 Task 1 Fortune（api/starfuyu）；图标用本地 zodiac.icon（原远程 /constellation/ 换掉）
 * - 去掉 luckyU wd-navbar 与状态栏 padding：appui 全局 custom nav，状态栏/返回由页面 sf-nav 承担
 * - 样式类名/渐变逻辑保留 luckyU 原样（非目标：不重写视觉）
 */
import type { Fortune } from '@/api/starfuyu'
import type { Zodiac } from '@/store/starfuyu/zodiac'

interface Props {
  fortuneData: Fortune
  zodiac: Zodiac
  date: string
  luckyColor: Fortune['lucky_color']
}

const props = defineProps<Props>()

defineEmits<Emits>()

interface Emits {
  (e: 'share'): void
  (e: 'reselect'): void
}

function formatDate(dateStr: string) {
  const date = new Date(dateStr)
  const month = date.getMonth() + 1
  const day = date.getDate()
  const weekdays = ['日', '一', '二', '三', '四', '五', '六']
  const weekday = weekdays[date.getDay()]
  return `${month}月${day}日 星期${weekday}`
}

// 将十六进制颜色转换为 RGB
function hexToRgb(hex: string): { r: number, g: number, b: number } | null {
  const result = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(hex)
  return result
    ? {
        r: Number.parseInt(result[1], 16),
        g: Number.parseInt(result[2], 16),
        b: Number.parseInt(result[3], 16),
      }
    : null
}

// 生成渐变颜色（基于幸运色生成更亮的渐变色）
function generateGradientColors(hex: string): string[] {
  const rgb = hexToRgb(hex)
  if (!rgb)
    return [hex, hex]

  // 生成更亮的颜色（增加亮度）
  const lighten = (r: number, g: number, b: number, factor: number) => {
    return {
      r: Math.min(255, Math.round(r + (255 - r) * factor)),
      g: Math.min(255, Math.round(g + (255 - g) * factor)),
      b: Math.min(255, Math.round(b + (255 - b) * factor)),
    }
  }

  // 生成更暗的颜色（减少亮度）
  const darken = (r: number, g: number, b: number, factor: number) => {
    return {
      r: Math.round(r * (1 - factor)),
      g: Math.round(g * (1 - factor)),
      b: Math.round(b * (1 - factor)),
    }
  }

  const light = lighten(rgb.r, rgb.g, rgb.b, 0.2)
  const dark = darken(rgb.r, rgb.g, rgb.b, 0.1)

  const toHex = (r: number, g: number, b: number) => {
    return `#${[r, g, b].map((x) => {
      const hex = x.toString(16)
      return hex.length === 1 ? `0${hex}` : hex
    }).join('')}`
  }

  return [toHex(light.r, light.g, light.b), hex, toHex(dark.r, dark.g, dark.b)]
}

const gradientColors = computed(() => {
  return generateGradientColors(props.luckyColor.hex)
})
</script>

<template>
  <view
    class="fortune-header"
    :style="{
      background: `linear-gradient(to bottom right, ${gradientColors[0]} 0%, ${gradientColors[1]} 50%, ${gradientColors[2]} 100%)`,
    }"
  >
    <view class="header-content">
      <view class="zodiac-info">
        <image :src="zodiac.icon" class="zodiac-header-icon" mode="aspectFit" />
        <view class="zodiac-name-date">
          <text class="zodiac-name-large">
            {{ zodiac.name }}
          </text>
          <text class="fortune-date">
            {{ formatDate(date) }}
          </text>
        </view>
      </view>

      <view class="fortune-summary">
        <text class="fortune-summary-text">
          {{ fortuneData.summary }}
        </text>
      </view>
    </view>
  </view>
</template>

<style lang="scss" scoped>
.fortune-header {
  padding: 20rpx 30rpx 30rpx;
  color: #fff;
  box-shadow: 0 4rpx 20rpx rgba(0, 0, 0, 0.1);
  position: relative;
}

.header-content {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 30rpx;
  padding-top: 20rpx;
  min-height: 0;
}

.zodiac-info {
  display: flex;
  align-items: center;
  gap: 20rpx;
  flex-shrink: 0;
  min-width: 0;
}

.zodiac-header-icon {
  width: 100rpx;
  height: 100rpx;
  border-radius: 50%;
  background-color: rgba(255, 255, 255, 0.2);
  padding: 10rpx;
}

.zodiac-name-date {
  display: flex;
  flex-direction: column;
  gap: 8rpx;
  min-width: 0;
}

.zodiac-name-large {
  font-size: 44rpx;
  font-weight: bold;
  color: #fff;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.fortune-date {
  font-size: 26rpx;
  color: rgba(255, 255, 255, 0.9);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.fortune-summary {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: flex-start;
  padding-top: 10rpx;
}

.fortune-summary-text {
  font-size: 28rpx;
  line-height: 1.6;
  color: rgba(255, 255, 255, 0.95);
  word-wrap: break-word;
  word-break: break-all;
}
</style>
