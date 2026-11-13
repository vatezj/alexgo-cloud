<script lang="ts" setup>
/**
 * 星座选择弹层（迁自 luckyU components/zodiac/ZodiacSelectorModal.vue；luckyU 内未被页面引用，随迁备用）
 * zodiacList 是 store/starfuyu/zodiac 的模块级导出（非 store state），直接 import，无需 storeToRefs。
 */
import { zodiacList } from '@/store/starfuyu/zodiac'

interface Props {
  show: boolean
}

interface Emits {
  (e: 'update:show', value: boolean): void
  (e: 'select', zodiacId: number): void
}

defineProps<Props>()
const emit = defineEmits<Emits>()

// 关闭弹窗
function closeModal() {
  emit('update:show', false)
}

// 选择星座
function selectZodiac(zodiacId: number) {
  emit('select', zodiacId)
  closeModal()
}

// 点击遮罩关闭
function handleMaskClick() {
  closeModal()
}
</script>

<template>
  <view v-if="show" class="zodiac-modal" @click="handleMaskClick">
    <view class="zodiac-modal-content" @click.stop>
      <view class="zodiac-modal-header">
        <text class="zodiac-modal-title">
          选择你的星座
        </text>
        <text class="zodiac-modal-close" @click="closeModal">
          ×
        </text>
      </view>
      <view class="zodiac-grid">
        <view
          v-for="zodiac in zodiacList"
          :key="zodiac.id"
          class="zodiac-card"
          @click="selectZodiac(zodiac.id)"
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
  </view>
</template>

<style lang="scss" scoped>
.zodiac-modal {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 999;
  animation: fadeIn 0.3s ease;
}

.zodiac-modal-content {
  width: 90%;
  max-width: 600rpx;
  background-color: #fff;
  border-radius: 24rpx;
  padding: 40rpx 30rpx;
  max-height: 80vh;
  overflow-y: auto;
  animation: slideUp 0.3s ease;
}

.zodiac-modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 30rpx;
}

.zodiac-modal-title {
  font-size: 36rpx;
  font-weight: bold;
  color: #333;
}

.zodiac-modal-close {
  font-size: 48rpx;
  color: #999;
  line-height: 1;
  width: 48rpx;
  height: 48rpx;
  display: flex;
  align-items: center;
  justify-content: center;
}

.zodiac-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 20rpx;
}

.zodiac-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 20rpx;
  border-radius: 16rpx;
  background-color: #f8f8f8;
  transition: all 0.3s ease;
  cursor: pointer;

  &:active {
    transform: scale(0.95);
    background-color: #eee;
  }
}

.zodiac-icon {
  width: 80rpx;
  height: 80rpx;
  margin-bottom: 10rpx;
}

.zodiac-name {
  font-size: 28rpx;
  font-weight: 500;
  color: #333;
  margin-bottom: 8rpx;
}

.zodiac-date {
  font-size: 22rpx;
  color: #999;
}

@keyframes fadeIn {
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
}

@keyframes slideUp {
  from {
    transform: translateY(100rpx);
    opacity: 0;
  }
  to {
    transform: translateY(0);
    opacity: 1;
  }
}
</style>
