<script setup lang="ts">
// 房间页占位（Task 7 换完整俯视牌桌 + 快照轮询）：登录扫码进房的落点，先能进能显。
definePage({
  style: {
    navigationStyle: 'custom',
  },
})

const route = useRoute()
const roomId = computed(() => String(route.query.roomId ?? ''))
</script>

<template>
  <view class="room-skeleton">
    <text class="title">
      房间 {{ roomId || '…' }}
    </text>
    <text class="hint">
      牌桌界面施工中（Task 7 接入）
    </text>
  </view>
</template>

<style lang="scss" scoped>
.room-skeleton {
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
  color: var(--brass);
}

.hint {
  display: block;
  margin-top: 12px;
  font-size: 13px;
  color: var(--dim);
}
</style>
