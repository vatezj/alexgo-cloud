<script setup lang="ts">
/**
 * 星座运势 · 八字+求签（starfuyu 分包占位壳，luckyu-into-scorecard Task 3）
 *
 * - 分包 pages/starfuyu/bazi（vite subPackages 扫描，主包剔除；迁自 luckyU bazhi/）
 * - 占位壳：自绘 nav 标题 + 返回；业务（排盘/求签）后续任务接入，不接 API
 * - 色值全 token；类名前缀 sf-（避 UnoCSS 撞名清单）
 */
import { useSystemBar } from '@/composables/useSystemBar'

definePage({
  style: {
    navigationStyle: 'custom',
  },
})

const router = useRouter()
const { pageStyle } = useSystemBar()

function goBack() {
  if (getCurrentPages().length > 1)
    uni.navigateBack()
  else
    router.replaceAll({ path: '/pages/starfuyu/index/index' })
}
</script>

<template>
  <view
    class="sf-page"
    :style="pageStyle"
  >
    <view class="sf-nav">
      <view
        class="sf-side"
        @click="goBack"
      >
        <wd-icon
          css-icon
          class="sf-navicon"
          name="i-carbon-chevron-left"
        />
      </view>
      <view class="sf-title">
        八字求签
        <text class="sf-sub">
          趣味小工具 · 仅供娱乐
        </text>
      </view>
      <view class="sf-side right" />
    </view>

    <view class="sf-body">
      <text class="sf-ph">
        敬请期待
      </text>
    </view>
  </view>
</template>

<style lang="scss" scoped>
.sf-page {
  box-sizing: border-box;
  min-height: 100vh;
  padding: calc(var(--status-bar-h, env(safe-area-inset-top, 0px)) + 6px) 16px calc(env(safe-area-inset-bottom, 0px) + 16px);
  color: var(--on-ink);
  background: var(--ink-900);
  background-image: var(--spot);
}

/* 自绘 nav（同 mine/manual 页骨架） */
.sf-nav {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 0 8px;

  /* 小程序：右侧让位微信真胶囊（根已水平 16px，变量扣掉避免双倍缩进） */
  padding-right: calc(var(--nav-right-pad, 16px) - 16px);
}

.sf-side {
  display: flex;
  align-items: center;
  min-width: 60px;
}

.sf-side.right {
  justify-content: flex-end;
}

.sf-navicon {
  margin-right: 8px;
  color: var(--brass);
  font-family: 'Noto Serif SC', serif;
  font-size: 26px;
  line-height: 1;
}

.sf-title {
  color: var(--on-ink);
  font-family: 'Noto Serif SC', serif;
  font-size: 17px;
  font-weight: 500;
  letter-spacing: 0.06em;
  text-align: center;
}

.sf-sub {
  display: block;
  margin-top: 2px;
  color: var(--dim);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 10.5px;
  font-weight: 400;
  letter-spacing: 0.02em;
}

.sf-body {
  display: flex;
  align-items: center;
  justify-content: center;
  padding-top: 120px;
}

.sf-ph {
  color: var(--dim);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 14px;
}
</style>
