<script setup lang="ts">
// 结算页占位（Task 9 换竖排开牌完整页）：房间页状态门 replaceAll 落点，先能进能显。
// query 约定（房间页已按此传）：roomId 必带。
definePage({
  style: {
    navigationStyle: 'custom',
  },
})

const route = useRoute()
const roomId = computed(() => String(route.query.roomId ?? ''))
</script>

<template>
  <view class="placeholder-page">
    <text class="title">
      结算
    </text>
    <text class="hint">
      界面施工中（Task 9 接入）{{ roomId ? ` · 房间 ${roomId}` : '' }}
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
