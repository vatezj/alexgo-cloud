<!--
  八字页头部（迁自 luckyU bazhi/components/BaziHeader.vue）
  - 纯 CSS 太极 + 标题，无 wot-ui 控件、无 API 依赖 → 模板/样式原样保留
  - 字体族 MaShanZheng/NotoSerifSC 沿用源稿（字体文件未迁，回退 cursive/serif，
    与 luckyU 实际运行态一致——其 static/font 目录为空、@font-face 全注释）
-->
<template>
  <view class="header">
    <view class="yin-yang-wrap">
      <!-- CSS 太极图 -->
      <view class="yin-yang">
        <view class="yin-yang-light">
          <view class="yin-yang-light-dot" />
        </view>
        <view class="yin-yang-dark">
          <view class="yin-yang-dark-dot" />
        </view>
      </view>
    </view>
    <text class="header-title">
      紫微斗数
    </text>
    <view class="header-subtitle">
      <view class="subtitle-line" />
      <text class="subtitle-text">
        批流年 · 测运势 · 解疑惑
      </text>
      <view class="subtitle-line" />
    </view>
  </view>
</template>

<style scoped lang="scss">
/* ========== 太极图 (纯 CSS) ========== */
.yin-yang-wrap {
  display: flex;
  justify-content: center;
  margin-bottom: 20rpx;
}
.yin-yang {
  width: 80rpx;
  height: 80rpx;
  border-radius: 50%;
  background: linear-gradient(to right, #8b0000 50%, #d4af37 50%);
  position: relative;
  overflow: hidden;
  animation: spin 10s linear infinite;
  border: 2rpx solid #8b0000;
}
.yin-yang-light {
  position: absolute;
  top: 0;
  left: 50%;
  transform: translateX(-50%);
  width: 40rpx;
  height: 40rpx;
  border-radius: 50%;
  background-color: #d4af37;
}
.yin-yang-light-dot {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 12rpx;
  height: 12rpx;
  border-radius: 50%;
  background-color: #8b0000;
}
.yin-yang-dark {
  position: absolute;
  bottom: 0;
  left: 50%;
  transform: translateX(-50%);
  width: 40rpx;
  height: 40rpx;
  border-radius: 50%;
  background-color: #8b0000;
}
.yin-yang-dark-dot {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 12rpx;
  height: 12rpx;
  border-radius: 50%;
  background-color: #d4af37;
}
@keyframes spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}

/* ========== 标题区域 ========== */
.header {
  padding: 80rpx 40rpx 40rpx;
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
}
.header-title {
  font-family: 'MaShanZheng', cursive;
  font-size: 84rpx;
  color: #8b0000;
  letter-spacing: 8rpx;
  margin-bottom: 10rpx;
  text-shadow: 4rpx 4rpx 0rpx rgba(212, 175, 55, 0.3);
}
.header-subtitle {
  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: center;
  gap: 20rpx;
}
.subtitle-line {
  width: 60rpx;
  height: 2rpx;
  background-color: #5c4033;
}
.subtitle-text {
  font-family: 'NotoSerifSC', serif;
  font-size: 28rpx;
  color: #5c4033;
  letter-spacing: 4rpx;
}
</style>
