<script setup lang="ts">
/**
 * 趣味方位模块 · 底部免责条（六页共用，spec 合规文案）
 *
 * - 两句均来自 copy.ts 锁定常量，逐字渲染，禁止改写
 * - caption 级 dim 色；页面根底部留白由各页 padding 处理
 */
import { DISCLAIMER_1, DISCLAIMER_2 } from '@/utils/funCompass/copy'
</script>

<template>
  <view class="fcdis-bar">
    <text class="fcdis-line">
      {{ DISCLAIMER_1 }}
    </text>
    <text class="fcdis-line">
      {{ DISCLAIMER_2 }}
    </text>
  </view>
</template>

<style lang="scss" scoped>
.fcdis-bar {
  margin-top: auto;
  padding-top: 14px;
}

.fcdis-line {
  display: block;
  color: var(--dim);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 10.5px;
  line-height: 1.55;
}
</style>
