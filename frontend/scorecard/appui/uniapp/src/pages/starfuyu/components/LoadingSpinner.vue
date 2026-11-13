<script lang="ts" setup>
interface Props {
  text?: string
}

withDefaults(defineProps<Props>(), {
  text: '正在加载...',
})
</script>

<template>
  <view class="loading-container">
    <view class="loading-content">
      <view class="loading-spinner" />
      <view class="loading-text">
        {{ text }}
      </view>
    </view>
  </view>
</template>

<style lang="scss" scoped>
.loading-container {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: rgba(255, 255, 255, 0.9);
  z-index: 10;
}

.loading-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 30rpx;
}

.loading-spinner {
  width: 60rpx;
  height: 60rpx;
  border: 4rpx solid rgba(102, 126, 234, 0.3);
  border-top-color: #667eea;
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.loading-text {
  font-size: 32rpx;
  color: #666;
}
</style>
