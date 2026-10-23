<script setup lang="ts">
import type { RoomMember } from '@/api/scorecard/types'
/**
 * 顶部横滚座位列表（设计 02 .roster）
 *
 * - 座位：头像（无头像取昵称首字）/ 昵称 pill / cur_score（ScoreText win/lose）
 * - me 描红：user_id === myUserId 时头像锁定红描边（--vermillion，四主题锁定）
 * - 房主徽标：user_id === creatorId 时头像右上角「房」
 * - 座位描边色按序轮换 --indigo-hi / --jade-hi / --brass（设计 --seatc）
 * - 点击座位 emit member-click（页面跳支出页预填 toUserId）
 * - 布局：roster 横滚区限宽 max-width:70%（flex:1+min-width:0 可收缩），邀请座位固定跟在其后不滚动
 * - 邀请「+」座位（虚线圆 + 加号）：点击 emit invite（页面开二维码邀请弹层）
 * - TODO(mock)：MOCK_MEMBERS 为横滚布局调试假人，验证后整段删除并把 displayMembers 改回 props.members
 */
import { computed } from 'vue'
import ScoreText from './ScoreText.vue'

interface Props {
  members: RoomMember[]
  /** 我的 user_id（me 描红；0 = 未识别） */
  myUserId: number
  /** 房主 user_id（徽标） */
  creatorId: number
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'member-click', userId: number): void
  (e: 'invite'): void
}>()

/** TODO(mock)：布局调试用假人，验证横滚后整段删除。 */
const MOCK_MEMBERS: RoomMember[] = Array.from({ length: 9 }, (_, i) => ({
  user_id: 90001 + i,
  nickname: `牌友${i + 1}`,
  avatar: '',
  cur_score: (i % 3 === 0 ? -1 : 1) * (i + 1) * 10,
  cum_lose_score: 0,
  is_creator: false,
}))

/** TODO(mock)：真实成员后追加假人撑宽 roster；删 mock 时改回 props.members。 */
const displayMembers = computed(() => [...props.members])

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
  <view class="roster-row">
    <!-- 座位横滚区：max-width 限宽，超限横向滚动 -->
    <scroll-view
      class="roster"
      scroll-x
      :show-scrollbar="false"
    >
      <view class="roster-inner">
        <view
          v-for="(m, i) in displayMembers"
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

    <!-- 邀请「+」座位：固定跟在 roster 后面，不随滚动 -->
    <view
      class="seat invite"
      @click="emit('invite')"
    >
      <view class="av inv">
        <text class="inv-plus">
          ＋
        </text>
      </view>
      <text class="nm">
        邀请
      </text>
      <!-- 与成员座位 ScoreText 行等高占位，三行基线对齐 -->
      <text class="sc-spacer" />
    </view>
  </view>
</template>

<style lang="scss" scoped>
/* 行容器：roster（可收缩滚动）+ 邀请座位（固定不滚动） */
.roster-row {
  box-sizing: border-box;
  display: flex;
  gap: 8px;
  align-items: flex-start;
  width: 100%;
  padding: 0 12px 0 0;
}

.roster {
  position: relative;
  z-index: 2;

  /* flex-basis:0 + min-width:0 双保险，scroll-view 必须可收缩才滚动而不是溢出 */
  flex: 1 1 0;
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
}

.roster-inner {
  display: inline-flex;
  gap: 6px;
  max-width: 500rpx;
  padding: 0 2px;
  padding-left: 10px;
}

.seat {
  display: inline-flex;
  flex: none;
  flex-direction: column;
  gap: 4px;
  align-items: center;
  width: 62px;
}

.av {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  font-size: 14px;
  font-weight: 500;
  color: var(--on-ink);
  background: var(--ink-900);
  border: 2.5px solid var(--seatc, var(--on-ink-a28));
  border-radius: 50%;
  box-shadow: var(--shadow-chip), inset 0 -4px 0 var(--chip-dark-a);
}

/* me 描红：锁定红（四主题同值） */
.av.me {
  color: var(--vermillion);
  border-color: var(--vermillion);
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
  align-items: center;
  justify-content: center;
  min-width: 18px;
  height: 18px;
  padding: 0 4px;
  font-size: 10px;
  font-weight: 700;
  color: var(--tile);
  background: var(--vermillion);
  border-radius: 9px;
}

.nm {
  max-width: 62px;
  padding: 1px 5px;
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 11px;
  color: var(--dim);
  white-space: nowrap;
  background: var(--felt-deep);
  border-radius: 999px;
}

.sc {
  font-size: 17px;
}

/* ── 邀请「+」座位：虚线圆 + 加号，与成员座位同宽对齐 ── */
.av.inv {
  color: var(--brass);
  background: transparent;
  border: 2px dashed var(--on-ink-a28);
  box-shadow: none;
}

.inv-plus {
  font-size: 20px;
  font-weight: 300;
  line-height: 1;
}

/* 与 ScoreText（17px / line-height 1）等高占位，三行基线对齐 */
.sc-spacer {
  display: block;
  height: 17px;
}
</style>
