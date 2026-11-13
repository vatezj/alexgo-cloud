<!--
  页面四角 + 云纹装饰（迁自 luckyU bazhi/components/PageDecorations.vue）
  - 去掉源稿顶部空 wd-navbar（appui 全局 custom nav，标题/返回/状态栏由页面 sf-nav 承担，
    与 Task 4 FortuneHeader 同口径）；其余装饰原样
-->
<template>
  <!-- 四角装饰 -->
  <view class="corner-decor corner-tl">
    <view class="corner-dot" />
  </view>
  <view class="corner-decor corner-tr">
    <view class="corner-dot" />
  </view>
  <view class="corner-decor corner-bl">
    <view class="corner-dot" />
  </view>
  <view class="corner-decor corner-br">
    <view class="corner-dot" />
  </view>

  <!-- 云纹装饰 -->
  <text class="cloud-pattern cloud-1">
    &#9729;
  </text>
  <text class="cloud-pattern cloud-2">
    &#9729;
  </text>
</template>

<style scoped lang="scss">
/* ========== 四角装饰 ========== */
.corner-decor {
  position: fixed;
  width: 80rpx;
  height: 80rpx;
  border-style: solid;
  border-color: #8b0000;
  z-index: 10;
}
.corner-tl {
  top: 30rpx;
  left: 30rpx;
  border-width: 4rpx 0 0 4rpx;
}
.corner-tr {
  top: 30rpx;
  right: 30rpx;
  border-width: 4rpx 4rpx 0 0;
}
.corner-bl {
  bottom: 30rpx;
  left: 30rpx;
  border-width: 0 0 4rpx 4rpx;
}
.corner-br {
  bottom: 30rpx;
  right: 30rpx;
  border-width: 0 4rpx 4rpx 0;
}
.corner-dot {
  position: absolute;
  width: 20rpx;
  height: 20rpx;
  background-color: #8b0000;
}
.corner-tl .corner-dot {
  top: 0;
  left: 0;
}
.corner-tr .corner-dot {
  top: 0;
  right: 0;
}
.corner-bl .corner-dot {
  bottom: 0;
  left: 0;
}
.corner-br .corner-dot {
  bottom: 0;
  right: 0;
}

/* ========== 云纹 ========== */
.cloud-pattern {
  position: absolute;
  opacity: 0.1;
  pointer-events: none;
  color: #8b0000;
}
.cloud-1 {
  top: 200rpx;
  left: -40rpx;
  font-size: 120rpx;
}
.cloud-2 {
  top: 800rpx;
  right: -40rpx;
  font-size: 160rpx;
}
</style>
