<script setup lang="ts">
/**
 * 顶部横滚座位列表（设计 02 .roster）
 *
 * - 座位：头像（无头像取昵称首字）/ 昵称 pill / cur_score（ScoreText win/lose）
 * - me 描红：user_id === myUserId 时头像锁定红描边（--vermillion，四主题锁定）
 * - 房主徽标：user_id === creatorId 时头像右上角「房」
 * - 座位描边色按序轮换 --indigo-hi / --jade-hi / --brass（设计 --seatc）
 * - 点击座位 emit member-click（页面跳支出页预填 toUserId）
 */
import type { RoomMember } from '@/api/scorecard/types'
import ScoreText from './ScoreText.vue'

interface Props {
  members: RoomMember[]
  /** 我的 user_id（me 描红；0 = 未识别） */
  myUserId: number
  /** 房主 user_id（徽标） */
  creatorId: number
}

defineProps<Props>()

const emit = defineEmits<{
  (e: 'member-click', userId: number): void
}>()

/** 座位描边色循环（设计 --seatc：indigo / jade / brass 轮换）。 */
const SEAT_COLORS = ['var(--indigo-hi)', 'var(--jade-hi)', 'var(--brass)']

function seatColor(index: number): string {
  return SEAT_COLORS[index % SEAT_COLORS.length]
}

/** 头像兜底字：昵称首字（空昵称用「牌」）。 */
function initial(name: string): string {
  return (name.slice(0, 1) || '牌').toUpperCase()
}
</script>

<template>
  <scroll-view
    class="roster"
    scroll-x
    :show-scrollbar="false"
  >
    <view class="roster-inner">
      <view
        v-for="(m, i) in members"
        :key="m.user_id"
        class="seat"
        @click="emit('member-click', m.user_id)"
      >
        <view
          class="av"
          :class="{ me: m.user_id === myUserId }"
          :style="{ '--seatc': seatColor(i) }"
        >
          <image
            v-if="m.avatar"
            class="av-img"
            :src="m.avatar"
            mode="aspectFill"
          />
          <text
            v-else
            class="av-txt"
          >
            {{ initial(m.nickname) }}
          </text>
          <text
            v-if="m.user_id === creatorId"
            class="badge"
          >
            房
          </text>
        </view>
        <text class="nm">
          {{ m.nickname || '牌友' }}
        </text>
        <ScoreText
          class="sc"
          :value="m.cur_score"
          :tone="m.cur_score >= 0 ? 'win' : 'lose'"
        />
      </view>
    </view>
  </scroll-view>
</template>

<style lang="scss" scoped>
.roster {
  position: relative;
  z-index: 2;
  width: 100%;
  white-space: nowrap;
}

.roster-inner {
  display: inline-flex;
  gap: 6px;
  padding: 0 2px;
}

.seat {
  display: inline-flex;
  flex: none;
  width: 62px;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}

.av {
  position: relative;
  display: flex;
  width: 44px;
  height: 44px;
  align-items: center;
  justify-content: center;
  border: 2.5px solid var(--seatc, var(--on-ink-a28));
  border-radius: 50%;
  background: var(--ink-900);
  box-shadow: var(--shadow-chip), inset 0 -4px 0 var(--chip-dark-a);
  color: var(--on-ink);
  font-size: 14px;
  font-weight: 500;
}

/* me 描红：锁定红（四主题同值） */
.av.me {
  border-color: var(--vermillion);
  color: var(--vermillion);
}

.av-img {
  width: 100%;
  height: 100%;
  border-radius: 50%;
}

.badge {
  position: absolute;
  top: -5px;
  right: -5px;
  display: flex;
  min-width: 18px;
  height: 18px;
  align-items: center;
  justify-content: center;
  padding: 0 4px;
  border-radius: 9px;
  background: var(--vermillion);
  color: var(--tile);
  font-size: 10px;
  font-weight: 700;
}

.nm {
  max-width: 62px;
  overflow: hidden;
  padding: 1px 5px;
  border-radius: 999px;
  background: var(--felt-deep);
  color: var(--dim);
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sc {
  font-size: 17px;
}
</style>
