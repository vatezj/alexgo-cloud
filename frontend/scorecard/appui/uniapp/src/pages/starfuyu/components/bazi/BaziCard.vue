<!--
  卡片容器（迁自 luckyU bazhi/components/BaziCard.vue）——纯样式 + slot，原样保留
-->
<template>
  <view class="card">
    <view class="card-border-inner" />
    <slot />
  </view>
</template>

<style scoped lang="scss">
/* ========== 卡片 ========== */
.card {
  background-color: rgba(255, 255, 255, 0.6);
  border: 2rpx solid #d4af37;
  padding: 40rpx;
  margin-bottom: 40rpx;
  position: relative;
  box-shadow: 0 8rpx 30rpx rgba(139, 0, 0, 0.05);
}
.card-border-inner {
  position: absolute;
  top: 8rpx;
  bottom: 8rpx;
  left: 8rpx;
  right: 8rpx;
  border: 2rpx solid #8b0000;
  opacity: 0.3;
  pointer-events: none;
}
</style>
