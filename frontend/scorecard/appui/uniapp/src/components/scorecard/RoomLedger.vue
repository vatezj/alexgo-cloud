<script setup lang="ts">
/**
 * 流水账板（设计 02 .ledger）
 *
 * - type=1 记分行：from −score → to +score（附茶 +tea 走 jade）；score=0 纯茶行
 *   「A 茶饭 −T 入池」；其余为虚线系统行
 * - type 文案映射：1 记分 / 2 茶饭达标 / 3 进房 / 5 结算 / 6 上限提醒 / 8 超时关房 / 9 转让房主
 * - showMore=true（默认）「查看更多」→ emit more（页面弹分页弹层）；弹层内传 false
 * - scrollable=true（页面账板）：rows 用 scroll-view 内部滚动、「查看更多」固定组件底部，
 *   外层页面把账板压进 flex 剩余空间、筹码轨贴底；false（弹层内）自然流式，外层 .lp-scroll 滚
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
  /** 内部滚动模式：true = rows 为 scroll-view + 查看更多固定底部；false = 自然流式 */
  scrollable?: boolean
}

withDefaults(defineProps<Props>(), {
  showMore: true,
  scrollable: false,
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
  <view
    class="ledger"
    :class="{ fill: scrollable }"
  >
    <view class="ledger-h">
      <text class="lt">
        本局流水
      </text>
      <text class="lh-cap">
        {{ logs.length ? `${logs.length} 条` : '暂无记录' }}
      </text>
    </view>
    <!-- rows 容器：始终 scroll-view（uniapp 不支持 <component :is> 切换内置标签）；
         scrollable=true 时 scroll-y + flex 剩余高度；false 时 scroll-y=false 自然流式（弹层）。
         小程序 scroll-view 不能自带 display:flex 排子项（会破坏滚动布局），flex/gap 放内层 view；
         高度用 height:0 + flex:1 强制参与 flex 分配（仅 min-height:0 在 MP 上拿不到高度） -->
    <scroll-view
      class="ledger-rows"
      :class="{ sc: scrollable }"
      :scroll-y="scrollable"
      :show-scrollbar="false"
    >
      <view class="ledger-rows-inner">
        <view
          v-for="log in logs"
          :key="log.id"
          class="lrow"
          :class="{ sys: log.type !== 1 }"
        >
          <!-- 纯茶行（只记茶）：A 茶饭 −T 入池，无分数转移结构 -->
          <template v-if="log.type === 1 && log.score === 0 && log.tea > 0">
            <text class="who">
              {{ nicknameMap[log.from_user_id] || '牌友' }}
            </text>
            <text class="tea-tag">
              茶饭
            </text>
            <ScoreText
              class="amt-j"
              :value="-log.tea"
              tone="jade"
            />
            <text class="arr">
              入池
            </text>
          </template>
          <!-- 记分行：from −score → to +score（茶附收） -->
          <template v-else-if="log.type === 1">
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

        <!-- 非滚动模式（弹层）：查看更多留在列表尾 -->
        <view
          v-if="!scrollable && showMore && logs.length"
          class="lrow more"
          @click="emit('more')"
        >
          <view class="more-text">
            查看更多
            <wd-icon
              css-icon
              class="more-icon"
              name="i-carbon-chevron-right"
            />
          </view>
        </view>
      </view>
    </scroll-view>

    <!-- 滚动模式（页面账板）：查看更多固定组件底部，不随滚动消失 -->
    <view
      v-if="scrollable && showMore && logs.length"
      class="lrow more as-footer"
      @click="emit('more')"
    >
      <view class="more-text">
        查看更多
        <wd-icon
          css-icon
          class="more-icon"
          name="i-carbon-chevron-right"
        />
      </view>
    </view>
  </view>
</template>

<style lang="scss" scoped>
.ledger {
  position: relative;
  display: flex;
  flex-direction: column;
  padding: 11px 13px 12px;
  margin: 10px 16px 0;
  overflow: hidden;
  background: var(--ink-700);
  border: 1px solid var(--brass-a50);
  border-radius: 18px;
}

/* 滚动模式：页面把账板压进 flex 剩余空间（牌桌与筹码轨之间），筹码轨贴底 */
.ledger.fill {
  flex: 1;
  min-height: 0;
}

.ledger::before {
  position: absolute;
  top: 0;
  right: 13px;
  left: 13px;
  height: 1.5px;
  content: '';
  background: linear-gradient(90deg, transparent, var(--brass), transparent);
  opacity: 0.6;
}

.ledger-h {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin-bottom: 8px;
}

.ledger-h .lt {
  font-family: 'Noto Serif SC', serif;
  font-size: 14px;
  font-weight: 700;
  color: var(--on-ink);
  letter-spacing: 0.06em;
}

.ledger-h .lh-cap {
  font-size: 10.5px;
  color: var(--dim);
}

.ledger-rows {
  box-sizing: border-box;
}

/* 行排布在内层：小程序 scroll-view 自带 display:flex 会破坏滚动布局 */
.ledger-rows-inner {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

/* 滚动模式：rows 吃满账板剩余高度，唯一滚动区。
   height:0 + flex:1 强制参与 flex 分配（仅 min-height:0 在 MP 拿不到确定高度） */
.ledger-rows.sc {
  flex: 1;
  height: 0;
  min-height: 0;
}

.lrow {
  display: flex;
  gap: 7px;
  align-items: center;
  padding: 8px 10px;
  font-size: 12px;
  color: var(--on-ink);
  background: var(--ink-600);
  border-radius: 9px;
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
  font-size: 11px;
  color: var(--dim);
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
  font-size: 10px;
  color: var(--dim);
}

.lrow .tm {
  flex: none;
  margin-left: auto;
  font-size: 10px;
  color: var(--dim);
}

/* 系统行：虚线弱化 */
.lrow.sys {
  font-size: 11.5px;
  color: var(--dim);
  background: transparent;
  border: 1px dashed var(--on-ink-a25);
}

.lrow.sys .systext {
  flex: 1;
  min-width: 0;
}

/* 查看更多入口（滚动模式固定组件底部时留出上间距） */
.lrow.more {
  flex: none;
  justify-content: center;
  font-size: 12px;
  color: var(--brass);
  letter-spacing: 0.05em;
  cursor: pointer;
  background: transparent;
  border: 1px dashed var(--brass-a50);
}

/* as-footer：不可用 fixed——UnoCSS 会把 class 里的 "fixed" 当成 position:fixed 工具类 */
.lrow.more.as-footer {
  margin-top: 6px;
}

.lrow.more .more-text {
  display: flex;
  align-items: center;
  gap: 4px;
}

.lrow.more .more-icon {
  font-size: 14px;
  line-height: 1;
}
</style>
