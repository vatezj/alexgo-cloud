<script setup lang="ts">
// 首页最小骨架：自绘导航 + 主题 token 背景（完整扇面牌首页见 Task 6）
definePage({
  style: {
    navigationStyle: 'custom',
  },
})
</script>

<template>
  <view class="home-skeleton">
    <text class="title">
      打牌记账
    </text>
  </view>
</template>

<style lang="scss" scoped>
.home-skeleton {
  box-sizing: border-box;
  min-height: 100vh;
  padding: calc(env(safe-area-inset-top, 0px) + 32px) 32px 32px;
  background: var(--ink-900);
  color: var(--on-ink);
}

.title {
  font-family: 'Noto Serif SC', serif;
  font-size: 22px;
  font-weight: 700;
  letter-spacing: 0.08em;
  color: var(--brass);
}
</style>
