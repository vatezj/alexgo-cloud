<script setup lang="ts">
/**
 * 趣味方位参考 · 模块首页（设计 preview/index-about.html 屏 01；spec 模块首页结构）
 *
 * - 唯一入口来自【我的】折叠组；返回 → 回【我的】（栈深 1 兜底 replaceAll）
 * - 三张骨白入口卡：生辰档案→bazi / 实时罗盘→compass / 座位模拟器→simulator
 * - 前置提示与脚注为锁定文案（copy.ts 逐字）；底部免责 ModDisclaimer
 * - 无 TabBar；语义类 fc- 前缀
 */
import ModDisclaimer from '@/components/funCompass/ModDisclaimer.vue'
import ModNav from '@/components/funCompass/ModNav.vue'
import { useSystemBar } from '@/composables/useSystemBar'
import { HOME_FOOT, HOME_LEAD_TIP, HOME_LOCAL_DOT } from '@/utils/funCompass/copy'

definePage({
  style: {
    navigationStyle: 'custom',
  },
})

const router = useRouter()
const { pageStyle } = useSystemBar()

/** PRD：模块首页返回 → 回【我的】；直达（栈深 1）兜底 replaceAll 到我的。 */
function goBack() {
  if (getCurrentPages().length > 1)
    uni.navigateBack()
  else
    router.replaceAll({ path: '/pages/mine/index' })
}

function goBazi() {
  router.push({ path: '/pages/fun/compass/bazi' })
}

function goCompassPage() {
  router.push({ path: '/pages/fun/compass/compass' })
}

function goSimulator() {
  router.push({ path: '/pages/fun/compass/simulator' })
}

function goAbout() {
  router.push({ path: '/pages/fun/compass/about' })
}
</script>

<template>
  <view
    class="fc-page"
    :style="pageStyle"
  >
    <ModNav
      title="趣味方位参考"
      sub="休闲小彩蛋 · 和记账无关"
      @back="goBack"
    />

    <!-- 前置提示（锁定逐字） -->
    <view class="fc-tip">
      <text class="fc-tip-mark">
        📌
      </text>
      <text class="fc-tip-t">
        {{ HOME_LEAD_TIP }}
      </text>
    </view>

    <!-- 三张入口卡 -->
    <view
      class="fc-card"
      @click="goBazi"
    >
      <view class="fc-ico">
        <wd-icon
          css-icon
          class="fc-ico-i"
          name="i-carbon-document"
        />
      </view>
      <view class="fc-card-main">
        <text class="fc-card-t">
          生辰档案
        </text>
        <text class="fc-card-s">
          本地生辰，算五行
        </text>
      </view>
      <wd-icon
        css-icon
        class="fc-chev"
        name="i-carbon-chevron-right"
      />
    </view>

    <view
      class="fc-card"
      @click="goCompassPage"
    >
      <view class="fc-ico">
        <wd-icon
          css-icon
          class="fc-ico-i"
          name="i-carbon-compass"
        />
      </view>
      <view class="fc-card-main">
        <text class="fc-card-t">
          实时罗盘
        </text>
        <text class="fc-card-s">
          磁力计指方向
        </text>
      </view>
      <wd-icon
        css-icon
        class="fc-chev"
        name="i-carbon-chevron-right"
      />
    </view>

    <view
      class="fc-card"
      @click="goSimulator"
    >
      <view class="fc-ico tile">
        <text class="fc-ico-tile-t">
          東
        </text>
      </view>
      <view class="fc-card-main">
        <text class="fc-card-t">
          座位模拟器
        </text>
        <text class="fc-card-s">
          排八字，求个座
        </text>
      </view>
      <wd-icon
        css-icon
        class="fc-chev"
        name="i-carbon-chevron-right"
      />
    </view>

    <!-- 脚注 -->
    <text class="fc-foot">
      {{ HOME_FOOT }}
    </text>
    <view class="fc-dot-line">
      <view class="fc-dot" />
      <text class="fc-dot-t">
        {{ HOME_LOCAL_DOT }}
      </text>
    </view>

    <view class="fc-spacer" />

    <!-- 关于与免责次级入口 -->
    <view
      class="fc-row"
      @click="goAbout"
    >
      <text class="fc-row-t">
        关于与免责
      </text>
      <wd-icon
        css-icon
        class="fc-chev"
        name="i-carbon-chevron-right"
      />
    </view>

    <ModDisclaimer />
  </view>
</template>

<style lang="scss" scoped>
.fc-page {
  display: flex;
  box-sizing: border-box;
  flex-direction: column;
  min-height: 100vh;
  padding: calc(var(--status-bar-h, env(safe-area-inset-top, 0px)) + 6px) 16px calc(env(safe-area-inset-bottom, 0px) + 16px);
  background: var(--ink-900);
  background-image: var(--spot);
  color: var(--on-ink);
}

/* 前置提示（黄铜细描边） */
.fc-tip {
  display: flex;
  gap: 8px;
  align-items: flex-start;
  margin: 6px 0 14px;
  padding: 10px 12px;
  border: 1px solid var(--brass);
  border-radius: 12px;
  background: var(--ink-700);
}

.fc-tip-mark {
  flex: none;
  font-size: 13px;
  line-height: 1.5;
}

.fc-tip-t {
  color: var(--on-ink);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 12.5px;
  line-height: 1.55;
}

/* 骨白入口卡 */
.fc-card {
  display: flex;
  gap: 12px;
  align-items: center;
  margin-bottom: 12px;
  padding: 14px;
  border-radius: var(--r-tile);
  background: var(--tile);
  box-shadow: var(--shadow-tile);
  color: var(--jet);
}

.fc-ico {
  display: flex;
  flex: none;
  width: 44px;
  height: 44px;
  align-items: center;
  justify-content: center;
  border-radius: 12px;
  background: var(--ink-700);
}

.fc-ico-i {
  color: var(--on-ink);
  font-size: 22px;
}

.fc-ico.tile {
  background: var(--tile);
  box-shadow: inset 0 0 0 1px var(--tile-shade);
}

.fc-ico-tile-t {
  color: var(--vermillion);
  font-family: 'Noto Serif SC', serif;
  font-size: 24px;
  font-weight: 700;
}

.fc-card-main {
  display: flex;
  flex: 1;
  min-width: 0;
  flex-direction: column;
  gap: 2px;
}

.fc-card-t {
  color: var(--jet);
  font-family: 'Noto Serif SC', serif;
  font-size: 16.5px;
  font-weight: 700;
  letter-spacing: 0.02em;
}

.fc-card-s {
  color: var(--jet);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 11.5px;
  opacity: 0.6;
}

.fc-chev {
  flex: none;
  color: var(--jet);
  font-size: 18px;
  opacity: 0.4;
}

/* 脚注 */
.fc-foot {
  display: block;
  margin-top: 4px;
  color: var(--dim);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 11px;
}

.fc-dot-line {
  display: flex;
  gap: 6px;
  align-items: center;
  margin-top: 6px;
}

.fc-dot {
  flex: none;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--jade-hi);
}

.fc-dot-t {
  color: var(--dim);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 10.5px;
}

.fc-spacer {
  flex: 1;
  min-height: 20px;
}

/* 关于与免责行（深色） */
.fc-row {
  display: flex;
  min-height: 50px;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 4px;
  padding: 10px 14px;
  border-radius: 14px;
  background: var(--ink-700);
}

.fc-row-t {
  color: var(--on-ink);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 14.5px;
}
</style>
