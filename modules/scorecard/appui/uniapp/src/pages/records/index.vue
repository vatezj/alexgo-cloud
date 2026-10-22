<script setup lang="ts">
// 战绩流水页占位（Task 10 整体重写）：先保证首页 pill push 不 throw
definePage({
  style: {
    navigationStyle: 'custom',
  },
})
</script>

<template>
  <view class="placeholder-page">
    <text class="title">
      战绩流水
    </text>
    <text class="hint">
      界面施工中（Task 10 接入）
    </text>
  </view>
</template>

<style lang="scss" scoped>
.placeholder-page {
  box-sizing: border-box;
  min-height: 100vh;
  padding: calc(env(safe-area-inset-top, 0px) + 32px) 32px 32px;
  background: var(--ink-900);
  color: var(--on-ink);
}

.title {
  color: var(--brass);
  font-family: 'Noto Serif SC', serif;
  font-size: 22px;
  font-weight: 700;
}

.hint {
  display: block;
  margin-top: 12px;
  color: var(--dim);
  font-size: 13px;
}
</style>
