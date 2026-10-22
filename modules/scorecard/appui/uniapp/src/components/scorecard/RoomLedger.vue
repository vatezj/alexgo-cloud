<script setup lang="ts">
/**
 * 流水账板（设计 02 .ledger）
 *
 * - type=1 记分行：from −score → to +score（附茶 +tea 走 jade）；其余为虚线系统行
 * - type 文案映射：1 记分 / 2 茶饭达标 / 3 进房 / 5 结算 / 6 上限提醒 / 8 超时关房 / 9 转让房主
 * - showMore=true（默认）底部「查看更多」→ emit more（页面弹分页弹层）；弹层内传 false
 * - 色值零硬编码：分值走 ScoreText（--lose-dark/--win-dark/--jade-hi），边框描边走 token
 */
import type { RoomLogItem } from '@/api/scorecard/types'
import ScoreText from './ScoreText.vue'

interface Props {
  logs: RoomLogItem[]
  /** user_id → 昵称（页面从快照 members 构建；缺失兜底「牌友」） */
  nicknameMap: Record<number, string>
  /** 是否显示「查看更多」入口（页面账板 true；弹层内分页列表 false） */
  showMore?: boolean
}

withDefaults(defineProps<Props>(), {
  showMore: true,
})

const emit = defineEmits<{
  (e: 'more'): void
}>()

/** RFC3339 → HH:MM（本地时区；非法时间返回空串）。 */
function hhmm(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime()))
    return ''
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}
</script>

<template>
  <view class="ledger">
    <view class="ledger-h">
      <text class="lt">
        本局流水
      </text>
      <text class="lh-cap">
        {{ logs.length ? `${logs.length} 条` : '暂无记录' }}
      </text>
    </view>
    <view class="ledger-rows">
      <view
        v-for="log in logs"
        :key="log.id"
        class="lrow"
        :class="{ sys: log.type !== 1 }"
      >
        <!-- 记分行：from −score → to +score（茶附收） -->
        <template v-if="log.type === 1">
          <text class="who">
            {{ nicknameMap[log.from_user_id] || '牌友' }}
          </text>
          <ScoreText
            class="amt-p"
            :value="-log.score"
            tone="lose"
          />
          <text class="arr">
            →
          </text>
          <text class="who">
            {{ nicknameMap[log.to_user_id] || '牌友' }}
          </text>
          <ScoreText
            class="amt-w"
            :value="log.score"
            tone="win"
          />
          <template v-if="log.tea > 0">
            <text class="tea-tag">
              茶
            </text>
            <ScoreText
              class="amt-j"
              :value="log.tea"
              tone="jade"
            />
          </template>
        </template>
        <!-- 系统行：按 type 映射文案 -->
        <text
          v-else
          class="systext"
        >
          <template v-if="log.type === 3">
            {{ nicknameMap[log.from_user_id] || '牌友' }} 进入了房间
          </template>
          <template v-else-if="log.type === 2">
            {{ nicknameMap[log.to_user_id || log.from_user_id] || '牌友' }} 茶饭达标
          </template>
          <template v-else-if="log.type === 5">
            本局已结算
          </template>
          <template v-else-if="log.type === 6">
            {{ nicknameMap[log.to_user_id || log.from_user_id] || '牌友' }} 达上限提醒
          </template>
          <template v-else-if="log.type === 8">
            无操作超时，房间自动关闭
          </template>
          <template v-else-if="log.type === 9">
            房主已转让给 {{ nicknameMap[log.to_user_id] || '牌友' }}
          </template>
          <template v-else>
            房间动态
          </template>
        </text>
        <text class="tm">
          {{ hhmm(log.created_at) }}
        </text>
      </view>

      <view
        v-if="!logs.length"
        class="lrow sys"
      >
        <text class="systext">
          暂无流水 · 点头像记第一笔
        </text>
      </view>

      <view
        v-if="showMore && logs.length"
        class="lrow more"
        @click="emit('more')"
      >
        <text>查看更多 ›</text>
      </view>
    </view>
  </view>
</template>

<style lang="scss" scoped>
.ledger {
  position: relative;
  overflow: hidden;
  margin: 10px 16px 0;
  padding: 11px 13px 12px;
  border: 1px solid var(--brass-a50);
  border-radius: 18px;
  background: var(--ink-700);
}

.ledger::before {
  position: absolute;
  top: 0;
  right: 13px;
  left: 13px;
  height: 1.5px;
  background: linear-gradient(90deg, transparent, var(--brass), transparent);
  content: '';
  opacity: 0.6;
}

.ledger-h {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin-bottom: 8px;
}

.ledger-h .lt {
  color: var(--on-ink);
  font-family: 'Noto Serif SC', serif;
  font-size: 14px;
  font-weight: 700;
  letter-spacing: 0.06em;
}

.ledger-h .lh-cap {
  color: var(--dim);
  font-size: 10.5px;
}

.ledger-rows {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.lrow {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 8px 10px;
  border-radius: 9px;
  background: var(--ink-600);
  color: var(--on-ink);
  font-size: 12px;
}

.lrow .who {
  flex: none;
  max-width: 90px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.lrow .arr {
  flex: none;
  color: var(--dim);
  font-size: 11px;
}

.lrow .amt-p,
.lrow .amt-w,
.lrow .amt-j {
  flex: none;
  font-size: 14px;
}

.lrow .tea-tag {
  flex: none;
  margin-left: 2px;
  color: var(--dim);
  font-size: 10px;
}

.lrow .tm {
  flex: none;
  margin-left: auto;
  color: var(--dim);
  font-size: 10px;
}

/* 系统行：虚线弱化 */
.lrow.sys {
  border: 1px dashed var(--on-ink-a25);
  background: transparent;
  color: var(--dim);
  font-size: 11.5px;
}

.lrow.sys .systext {
  min-width: 0;
  flex: 1;
}

/* 查看更多入口 */
.lrow.more {
  justify-content: center;
  border: 1px dashed var(--brass-a50);
  background: transparent;
  color: var(--brass);
  cursor: pointer;
  font-size: 12px;
  letter-spacing: 0.05em;
}
</style>
