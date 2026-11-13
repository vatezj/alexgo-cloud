<script lang="ts" setup>
/**
 * 悬浮操作钮（迁自 luckyU components/common/FloatingActions.vue）
 * 「算一卦」目标换 appui 分包八字页路径（/pages/starfuyu/bazi/index）。
 */
defineProps<{
  onShare: () => void
  onReselect: () => void
}>()

function onCalculateBazi() {
  uni.navigateTo({
    url: '/pages/starfuyu/bazi/index',
  })
}
</script>

<template>
  <view class="floating-actions">
    <button class="floating-btn reselect-btn" @click="onCalculateBazi">
      <text class="btn-text">
        算一卦
      </text>
    </button>
    <!-- #ifdef MP-WEIXIN -->
    <button class="floating-btn share-btn" open-type="share">
      <text class="btn-text">
        分享
      </text>
    </button>
    <!-- #endif -->
    <!-- #ifndef MP-WEIXIN -->
    <button class="floating-btn share-btn" @click="onShare">
      <text class="btn-text">
        分享
      </text>
    </button>
    <!-- #endif -->
    <button class="floating-btn reselect-btn" @click="onReselect">
      <text class="btn-text">
        重选
      </text>
    </button>
  </view>
</template>

<style lang="scss" scoped>
.floating-actions {
  position: fixed;
  right: 30rpx;
  bottom: 30rpx;
  display: flex;
  flex-direction: column;
  gap: 20rpx;
  z-index: 999;
  // #ifdef MP-WEIXIN || MP-ALIPAY || MP-BAIDU || MP-TOUTIAO || MP-QQ
  bottom: calc(30rpx + env(safe-area-inset-bottom));
  // #endif
}

.floating-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8rpx;
  width: 80rpx;
  height: 80rpx;
  border-radius: 60rpx;
  border: none;
  box-shadow: 0 8rpx 24rpx rgba(0, 0, 0, 0.15);
  transition: all 0.3s ease;
  position: relative;
  overflow: hidden;

  &::before {
    content: '';
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: linear-gradient(135deg, rgba(255, 255, 255, 0.3) 0%, rgba(255, 255, 255, 0.1) 100%);
    opacity: 0;
    transition: opacity 0.3s ease;
  }

  &:active {
    transform: scale(0.95);
    box-shadow: 0 4rpx 12rpx rgba(0, 0, 0, 0.2);

    &::before {
      opacity: 1;
    }
  }
}

.share-btn {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: #fff;
}

.reselect-btn {
  background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%);
  color: #fff;
}

.btn-text {
  font-size: 24rpx;
  font-weight: 500;
  line-height: 1;
}
</style>
