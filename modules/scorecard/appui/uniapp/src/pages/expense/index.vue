<script setup lang="ts">
// 支出页占位（Task 8 换筹码档完整页）：房间页「支出 / 点头像」push 落点，先能进能显。
// query 约定（房间页已按此传）：roomId 必带；toUserId 可选 = 预选收款人（空 = 预选空）。
definePage({
  style: {
    navigationStyle: 'custom',
  },
})

const route = useRoute()
const roomId = computed(() => String(route.query.roomId ?? ''))
const toUserId = computed(() => String(route.query.toUserId ?? ''))
</script>

<template>
  <view class="placeholder-page">
    <text class="title">
      支出记分
    </text>
    <text class="hint">
      界面施工中（Task 8 接入）{{ roomId ? ` · 房间 ${roomId}` : '' }}{{ toUserId ? ` · 对方 ${toUserId}` : '' }}
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
