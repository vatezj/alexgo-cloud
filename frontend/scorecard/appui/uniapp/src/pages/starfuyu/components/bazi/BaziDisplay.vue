<script setup lang="ts">
/**
 * 四柱展示（迁自 luckyU bazhi/components/BaziDisplay.vue）
 *
 * - 纯展示无 API；入参结构 {label,heaven,earth} 与源稿一致（页面侧由 Task 1
 *   BaziInfo 填充），本地 BaziItem 接口原样保留
 */
interface BaziItem {
  label: string
  heaven: string
  earth: string
}

defineProps<{
  baziData: BaziItem[]
}>()
</script>

<template>
  <view class="bazi-grid">
    <view v-for="(col, idx) in baziData" :key="idx" class="bazi-col">
      <text class="bazi-label">
        {{ col.label }}
      </text>
      <view class="bazi-char bazi-heaven">
        <text class="bazi-char-text">
          {{ col.heaven }}
        </text>
      </view>
      <view class="bazi-char bazi-earth">
        <text class="bazi-char-text bazi-earth-text">
          {{ col.earth }}
        </text>
      </view>
    </view>
  </view>
</template>

<style scoped lang="scss">
/* ========== 八字展示 ========== */
.bazi-grid {
  display: flex;
  flex-direction: row;
  justify-content: space-between;
  gap: 16rpx;
}
.bazi-col {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10rpx;
}
.bazi-label {
  font-size: 24rpx;
  color: #666666;
  margin-bottom: 10rpx;
  font-family: 'NotoSerifSC', serif;
}
.bazi-char {
  width: 100%;
  height: 120rpx;
  border-radius: 8rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: inset 0 0 20rpx rgba(0, 0, 0, 0.2);
}
.bazi-heaven {
  background-color: #8b0000;
}
.bazi-earth {
  background-color: #1a1a1a;
}
.bazi-char-text {
  font-family: 'MaShanZheng', cursive;
  font-size: 48rpx;
  color: #ffffff;
}
.bazi-earth-text {
  color: #d4af37;
}
</style>
