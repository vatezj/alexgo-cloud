<script lang="ts" setup>
/**
 * 运势内容列（迁自 luckyU components/fortune/FortuneContent.vue）；类型换 Task 1 Fortune。
 */
import type { Fortune } from '@/api/starfuyu'
import FortuneCard from './FortuneCard.vue'

interface Props {
  fortuneData: Fortune
}

defineProps<Props>()
</script>

<template>
  <view class="fortune-content">
    <!-- 整体运势 -->
    <FortuneCard type="overall" :fortune-data="fortuneData" />

    <!-- 爱情运势 -->
    <FortuneCard type="love" :fortune-data="fortuneData" />

    <!-- 事业运势 -->
    <FortuneCard type="career" :fortune-data="fortuneData" />

    <!-- 财富运势 -->
    <FortuneCard type="wealth" :fortune-data="fortuneData" />

    <!-- 健康运势 -->
    <FortuneCard type="health" :fortune-data="fortuneData" />

    <!-- 幸运元素 -->
    <FortuneCard type="lucky" :fortune-data="fortuneData" />

    <!-- 今日提醒 -->
    <FortuneCard type="reminder" :fortune-data="fortuneData" />
  </view>
</template>

<style lang="scss" scoped>
.fortune-content {
  flex: 1;
  padding: 30rpx;
  padding-bottom: 100rpx;
}
</style>
