<script lang="ts" setup>
/**
 * 星座全屏选择器（迁自 luckyU components/zodiac/ZodiacSelector.vue）
 *
 * - 类型换 store/starfuyu/zodiac 的 Zodiac；图标改用本地 zodiac.icon
 *   （原远程 `${base}/constellation/` 与 /static/tinified/ 兜底在 appui 无对应资产/环境变量）
 * - 容器 min-height:100vh → flex:1：appui 页面 sf-nav 常驻，100vh 会顶出双倍滚动
 */
import type { Zodiac } from '@/store/starfuyu/zodiac'

interface Props {
  zodiacList: Zodiac[]
}

defineProps<Props>()

const emit = defineEmits<Emits>()

interface Emits {
  (e: 'select', zodiacId: number): void
}

function handleSelect(zodiacId: number) {
  emit('select', zodiacId)
}
</script>

<template>
  <view class="zodiac-selector-container">
    <view class="selector-header">
      <text class="selector-title">
        选择你的星座
      </text>
      <text class="selector-subtitle">
        查看今日运势
      </text>
    </view>
    <view class="zodiac-grid">
      <view
        v-for="zodiac in zodiacList"
        :key="zodiac.id"
        class="zodiac-card"
        @click="handleSelect(zodiac.id)"
      >
        <image :src="zodiac.icon" class="zodiac-icon" mode="aspectFit" />
        <text class="zodiac-name">
          {{ zodiac.name }}
        </text>
        <text class="zodiac-date">
          {{ zodiac.dateRange }}
        </text>
      </view>
    </view>
  </view>
</template>

<style lang="scss" scoped>
.zodiac-selector-container {
  flex: 1;
  min-height: 0;
  padding: 60rpx 30rpx 40rpx;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  overflow-y: auto;
}

.selector-header {
  text-align: center;
  margin-bottom: 50rpx;
}

.selector-title {
  display: block;
  font-size: 48rpx;
  font-weight: bold;
  color: #fff;
  margin-bottom: 20rpx;
}

.selector-subtitle {
  display: block;
  font-size: 28rpx;
  color: rgba(255, 255, 255, 0.9);
}

.zodiac-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 24rpx;
}

.zodiac-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 30rpx 20rpx;
  border-radius: 20rpx;
  background-color: rgba(255, 255, 255, 0.95);
  box-shadow: 0 8rpx 24rpx rgba(0, 0, 0, 0.15);
  transition: all 0.3s ease;
  cursor: pointer;

  &:active {
    transform: scale(0.95);
    box-shadow: 0 4rpx 12rpx rgba(0, 0, 0, 0.2);
  }
}

.zodiac-icon {
  width: 100rpx;
  height: 100rpx;
  margin-bottom: 16rpx;
}

.zodiac-name {
  font-size: 30rpx;
  font-weight: 600;
  color: #333;
  margin-bottom: 8rpx;
}

.zodiac-date {
  font-size: 24rpx;
  color: #999;
}
</style>
