<script lang="ts" setup>
/**
 * 运势分卡（迁自 luckyU components/fortune/FortuneCard.vue）
 * 类型换 Task 1 Fortune（api/starfuyu）；字段名与 luckyU FortuneData 同名，模板零改动。
 */
import type { Fortune } from '@/api/starfuyu'

interface Props {
  type: 'overall' | 'love' | 'career' | 'wealth' | 'health' | 'lucky' | 'reminder'
  fortuneData: Fortune
}

const props = defineProps<Props>()

// 将分数（1-100）转换为星级（1-5星）
function scoreToStars(score: number): number {
  // 0-20: 1星, 21-40: 2星, 41-60: 3星, 61-80: 4星, 81-100: 5星
  if (score <= 20)
    return 1
  if (score <= 40)
    return 2
  if (score <= 60)
    return 3
  if (score <= 80)
    return 4
  return 5
}

// 获取当前运势的分数
function getCurrentScore(): number {
  switch (props.type) {
    case 'overall':
      return props.fortuneData.overall_score
    case 'love':
      return props.fortuneData.love_score
    case 'career':
      return props.fortuneData.career_score
    case 'wealth':
      return props.fortuneData.wealth_score
    case 'health':
      return props.fortuneData.health_score
    default:
      return 0
  }
}

const stars = computed(() => {
  if (props.type === 'lucky' || props.type === 'reminder')
    return 0
  return scoreToStars(getCurrentScore())
})
</script>

<template>
  <!-- 整体运势 -->
  <view v-if="type === 'overall'" class="fortune-card overall-card">
    <view class="card-header">
      <text class="card-title">
        整体运势
      </text>
      <view class="stars-display">
        <text
          v-for="i in 5"
          :key="i"
          class="star"
          :class="{ active: i <= stars }"
        >
          ★
        </text>
      </view>
    </view>
    <text class="card-content">
      {{ fortuneData.summary }}
    </text>
  </view>

  <!-- 爱情运势 -->
  <view v-else-if="type === 'love'" class="fortune-card love-card">
    <view class="card-header">
      <text class="card-title">
        💕 爱情运势
      </text>
      <view class="stars-display">
        <text
          v-for="i in 5"
          :key="i"
          class="star"
          :class="{ active: i <= stars }"
        >
          ★
        </text>
      </view>
    </view>
    <text class="card-content">
      {{ fortuneData.love_tip }}
    </text>
  </view>

  <!-- 事业运势 -->
  <view v-else-if="type === 'career'" class="fortune-card career-card">
    <view class="card-header">
      <text class="card-title">
        💼 事业运势
      </text>
      <view class="stars-display">
        <text
          v-for="i in 5"
          :key="i"
          class="star"
          :class="{ active: i <= stars }"
        >
          ★
        </text>
      </view>
    </view>
    <text class="card-content">
      {{ fortuneData.career_tip }}
    </text>
  </view>

  <!-- 财富运势 -->
  <view v-else-if="type === 'wealth'" class="fortune-card wealth-card">
    <view class="card-header">
      <text class="card-title">
        💰 财富运势
      </text>
      <view class="stars-display">
        <text
          v-for="i in 5"
          :key="i"
          class="star"
          :class="{ active: i <= stars }"
        >
          ★
        </text>
      </view>
    </view>
    <text class="card-content">
      {{ fortuneData.wealth_tip }}
    </text>
  </view>

  <!-- 健康运势 -->
  <view v-else-if="type === 'health'" class="fortune-card health-card">
    <view class="card-header">
      <text class="card-title">
        🏥 健康运势
      </text>
      <view class="stars-display">
        <text
          v-for="i in 5"
          :key="i"
          class="star"
          :class="{ active: i <= stars }"
        >
          ★
        </text>
      </view>
    </view>
    <text class="card-content">
      {{ fortuneData.health_tip }}
    </text>
  </view>

  <!-- 幸运元素 -->
  <view v-else-if="type === 'lucky'" class="fortune-card lucky-card">
    <view class="card-header">
      <text class="card-title">
        🍀 幸运元素
      </text>
    </view>
    <view class="lucky-items">
      <view class="lucky-item">
        <text class="lucky-label">
          幸运颜色
        </text>
        <view class="lucky-color-box" :style="{ backgroundColor: fortuneData.lucky_color.hex }" />
        <text class="lucky-value">
          {{ fortuneData.lucky_color.chinese }}
        </text>
      </view>
      <view class="lucky-item">
        <text class="lucky-label">
          幸运数字
        </text>
        <text class="lucky-number">
          {{ fortuneData.lucky_number }}
        </text>
      </view>
    </view>
  </view>

  <!-- 今日提醒 -->
  <!-- <view v-else-if="type === 'reminder'" class="fortune-card reminder-card">
    <view class="card-header">
      <text class="card-title">💡 今日提醒</text>
    </view>
    <text class="reminder-text">{{ fortuneData.summary }}</text>
  </view> -->
</template>

<style lang="scss" scoped>
.fortune-card {
  background-color: #fff;
  border-radius: 24rpx;
  padding: 30rpx;
  margin-bottom: 24rpx;
  box-shadow: 0 2rpx 12rpx rgba(0, 0, 0, 0.08);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20rpx;
  padding-bottom: 20rpx;
  border-bottom: 1rpx solid #f0f0f0;
}

.card-title {
  font-size: 32rpx;
  font-weight: bold;
  color: #333;
}

.stars-display {
  display: flex;
  align-items: center;
  gap: 6rpx;
}

.star {
  font-size: 32rpx;
  color: #e0e0e0;
  transition: all 0.3s ease;
  text-shadow: 0 1rpx 2rpx rgba(0, 0, 0, 0.1);

  &.active {
    color: #ffd700;
    text-shadow: 0 2rpx 8rpx rgba(255, 215, 0, 0.4);
    transform: scale(1.1);
  }
}

.card-content {
  font-size: 28rpx;
  line-height: 1.8;
  color: #666;
}

.lucky-items {
  display: flex;
  justify-content: space-around;
  align-items: center;
  gap: 20rpx;
}

.lucky-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12rpx;
  flex: 1;
}

.lucky-label {
  font-size: 24rpx;
  color: #999;
}

.lucky-color-box {
  width: 60rpx;
  height: 60rpx;
  border-radius: 50%;
  border: 2rpx solid #eee;
}

.lucky-value {
  font-size: 26rpx;
  font-weight: 500;
  color: #333;
}

.lucky-number {
  font-size: 48rpx;
  font-weight: bold;
  color: #667eea;
}

.reminder-text {
  font-size: 30rpx;
  line-height: 1.8;
  color: #ff6b6b;
  font-weight: 500;
}
</style>
