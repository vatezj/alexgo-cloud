<script setup lang="ts">
/**
 * 战绩左滑行（设计 07 .sw-wrap / .sw-content）：wd-swipe-action 包装 + 右侧三操作
 *
 * - 内容行 = 日期 + ±分（ScoreText）+ 「我的净成绩 · 结算类型」
 * - 右滑按钮：流水（黄铜）/ 排行（墨面）/ 删除（朱红），点击 emit detail/rank/delete
 * - settle_type 对照 modules/scorecard/model/record.go：1 正常结算 2 房主退房自动结算
 *   3 退房 4 零数据空战绩（非 brief 草案的 0/1/2/3）
 * - 色值零硬编码：全走主题 token（--brass/--ink-600/--vermillion/--on-ink-a04/--dim）
 */
import type { RecordItem } from '@/api/scorecard/types'
import ScoreText from './ScoreText.vue'

interface Props {
  record: RecordItem
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'detail'): void
  (e: 'rank'): void
  (e: 'delete'): void
}>()

/** settle_type → 结算类型文案（对照 model.Record 注释；未知值兜底）。 */
function settleLabel(t: number): string {
  if (t === 1)
    return '正常结算'
  if (t === 2)
    return '房主退房自动结算'
  if (t === 3)
    return '退房'
  if (t === 4)
    return '零数据空战绩'
  return '结算'
}

/** RFC3339 → M月D日（本地时区；非法时间返回空串）。 */
function monthDay(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime()))
    return ''
  return `${d.getMonth() + 1}月${d.getDate()}日`
}

const scoreTone = computed(() =>
  props.record.score > 0 ? 'win' : (props.record.score < 0 ? 'lose' : 'none'),
)
</script>

<template>
  <wd-swipe-action>
    <view class="sw-content">
      <view class="r1">
        <text class="date">
          {{ monthDay(record.created_at) }}
        </text>
        <ScoreText
          class="amt"
          :value="record.score"
          :tone="scoreTone"
        />
      </view>
      <text class="r2">
        我的净成绩 · {{ settleLabel(record.settle_type) }}
      </text>
    </view>
    <template #right>
      <view class="sw-actions">
        <view
          class="sa flow"
          @click.stop="emit('detail')"
        >
          <text>流水</text>
        </view>
        <view
          class="sa rank"
          @click.stop="emit('rank')"
        >
          <text>排行</text>
        </view>
        <view
          class="sa del"
          @click.stop="emit('delete')"
        >
          <text>删除</text>
        </view>
      </view>
    </template>
  </wd-swipe-action>
</template>

<style lang="scss" scoped>
.sw-content {
  box-sizing: border-box;
  padding: 12px 13px;
  border: 1px solid var(--on-ink-a04);
  border-radius: 12px;
  background: var(--ink-700);
  color: var(--on-ink);
}

.r1 {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 13.5px;
}

.r1 .amt {
  font-size: 18px;
  font-weight: 600;
}

.r2 {
  display: block;
  margin-top: 3px;
  color: var(--dim);
  font-size: 11px;
}

/* 右侧三操作（设计 .sw-actions .sa：66px 竖排字） */
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
  color: var(--on-ink);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 12.5px;
}

.sw-actions .sa.flow {
  background: linear-gradient(180deg, var(--brass-hi), var(--brass));
  color: var(--ink-900);
}

.sw-actions .sa.rank {
  background: var(--ink-600);
}

.sw-actions .sa.del {
  background: var(--vermillion);
  color: var(--tile);
}
</style>
