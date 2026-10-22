<script setup lang="ts">
/**
 * 好友排名行（设计 08 .rrow + .sw-wrap）：wd-swipe-action 包装 + 右侧删除/拉黑
 *
 * - 行内容 = 奖牌名次 + 头像（图/首字，描边色按 user_id 取 token）+ 昵称
 *   （自己标「· 我」黄铜；已拉黑标「已拉黑」朱红）+ 胜率 + 总分（ScoreText brass）
 * - 右滑按钮：删除（朱红）/ 拉黑（输色蓝）；已拉黑行换成「解除」（墨面）
 * - T10 遗留 minor 教训：in-slot 操作按钮不加 .stop——加了会绕过
 *   wd-swipe-action 自动收起，统一靠页面根 closeOutside 收
 * - 色值零硬编码：全走主题 token（--vermillion/--indigo-hi/--tile-shade/
 *   --brass/--brass-hi/--ink-600/--dim）
 */
import type { FriendItem } from '@/api/scorecard/types'
import ScoreText from './ScoreText.vue'

interface Props {
  item: FriendItem
  /** 榜内名次（1 起，含自己合并排序后的诚实榜位） */
  rank: number
  /** 是否自己行（标「· 我」+ 黄铜描边高亮） */
  self?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  self: false,
})

const emit = defineEmits<{
  (e: 'delete'): void
  (e: 'black'): void
  (e: 'unblack'): void
}>()

/** 头像描边色：user_id 轮转四套主题内的强调 token（设计稿 --rc 语义）。 */
const RING_TOKENS = ['var(--vermillion)', 'var(--brass)', 'var(--indigo-hi)', 'var(--jade-hi)']

const ringColor = computed(() => RING_TOKENS[props.item.user_id % RING_TOKENS.length] ?? 'var(--brass)')

/** 无头像图时取昵称首字（空昵称兜底「友」）。 */
const avatarText = computed(() => (props.item.nickname || '友').slice(0, 1))

/** 胜率百分：win_rate 为 0~1 浮点（分母 0 后端给 0，同战绩页）。 */
const winRateText = computed(() => `${Math.round(props.item.win_rate * 100)}%`)

/** 奖牌样式：前三特殊色，其余 mn。 */
const medalClass = computed(() => {
  if (props.rank === 1)
    return 'm1'
  if (props.rank === 2)
    return 'm2'
  if (props.rank === 3)
    return 'm3'
  return 'mn'
})
</script>

<template>
  <wd-swipe-action>
    <view
      class="rrow"
      :class="{ me: self }"
    >
      <text
        class="medal num"
        :class="medalClass"
      >
        {{ rank }}
      </text>
      <view
        class="rav"
        :style="{ '--rc': ringColor }"
      >
        <image
          v-if="item.avatar"
          class="av-img"
          :src="item.avatar"
          mode="aspectFill"
        />
        <text v-else>
          {{ avatarText }}
        </text>
      </view>
      <view class="rmain">
        <view class="rn-line">
          <text class="rn">
            {{ item.nickname || '牌友' }}
          </text>
          <text
            v-if="self"
            class="me-tag"
          >
            · 我
          </text>
          <text
            v-if="item.is_blocked"
            class="blk-tag"
          >
            已拉黑
          </text>
        </view>
        <text class="rsub">
          胜率 {{ winRateText }}
        </text>
      </view>
      <!-- 总分：黄铜色经继承注入（同战绩页 stat-chip 模式，.brass-n 无全局定义） -->
      <view class="rs brass">
        <ScoreText
          :value="item.total_score"
          :signed="false"
        />
      </view>
    </view>
    <template #right>
      <view class="sw-actions">
        <view
          class="sa del"
          @click="emit('delete')"
        >
          <text>删除</text>
        </view>
        <view
          v-if="!item.is_blocked"
          class="sa ban"
          @click="emit('black')"
        >
          <text>拉黑</text>
        </view>
        <view
          v-else
          class="sa unban"
          @click="emit('unblack')"
        >
          <text>解除</text>
        </view>
      </view>
    </template>
  </wd-swipe-action>
</template>

<style lang="scss" scoped>
.rrow {
  display: flex;
  box-sizing: border-box;
  align-items: center;
  gap: 11px;
  padding: 10px 13px;
  border: 1px solid var(--on-ink-a04);
  border-radius: 12px;
  background: var(--ink-700);
  color: var(--on-ink);
}

/* 自己行：黄铜描边 + 淡黄铜底（设计 .rrow.me） */
.rrow.me {
  border-color: var(--brass-a50);
  background: color-mix(in srgb, var(--brass) 7%, transparent);
}

/* 奖牌（设计 .medal；前三各一色，token 化） */
.medal {
  display: flex;
  width: 26px;
  height: 26px;
  flex: none;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  font-size: 14px;
  font-weight: 700;
}

.medal.m1 {
  background: linear-gradient(180deg, var(--brass-hi), var(--brass));
  color: var(--ink-900);
}

.medal.m2 {
  background: var(--tile-shade);
  color: var(--jet);
}

.medal.m3 {
  background: var(--dim);
  color: var(--ink-900);
}

.medal.mn {
  background: var(--ink-600);
  color: var(--dim);
}

/* 头像：图 or 首字，描边 --rc（user_id 轮转 token） */
.rav {
  display: flex;
  width: 40px;
  height: 40px;
  flex: none;
  align-items: center;
  justify-content: center;
  border: 2px solid var(--rc, var(--on-ink-a28));
  border-radius: 50%;
  background: var(--ink-600);
  color: var(--on-ink);
  font-size: 15px;
  overflow: hidden;
}

.rav .av-img {
  width: 100%;
  height: 100%;
}

.rmain {
  min-width: 0;
  flex: 1;
}

.rn-line {
  display: flex;
  align-items: center;
  gap: 6px;
}

.rn {
  overflow: hidden;
  font-size: 14px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 「· 我」黄铜小标（设计 .rn 内嵌 span） */
.me-tag {
  flex: none;
  color: var(--brass);
  font-size: 11px;
}

/* 「已拉黑」标记 */
.blk-tag {
  display: inline-flex;
  flex: none;
  align-items: center;
  padding: 1px 6px;
  border: 1px solid var(--vermillion);
  border-radius: 6px;
  color: var(--vermillion);
  font-size: 10px;
}

.rsub {
  display: block;
  margin-top: 1px;
  color: var(--dim);
  font-size: 10.5px;
}

.rs {
  flex: none;
  font-size: 19px;
  font-weight: 600;
}

/* 黄铜总分：token 色经继承注入 ScoreText（同战绩页 stat-chip .brass） */
.rs.brass {
  color: var(--brass);
}

/* 右侧操作（设计 .sw-actions .sa：66px 竖排字；按钮不加 .stop） */
.sw-actions {
  display: flex;
  height: 100%;
}

.sw-actions .sa {
  display: flex;
  width: 66px;
  height: 100%;
  flex: none;
  align-items: center;
  justify-content: center;
  color: var(--tile);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 12.5px;
}

.sw-actions .sa.del {
  background: var(--vermillion);
}

.sw-actions .sa.ban {
  background: var(--lose-dark);
}

.sw-actions .sa.unban {
  background: var(--ink-600);
  color: var(--on-ink);
}
</style>
