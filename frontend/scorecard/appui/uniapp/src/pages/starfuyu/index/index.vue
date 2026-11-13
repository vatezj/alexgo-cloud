<script setup lang="ts">
/**
 * 星座运势 · 运势首页（luckyu-into-scorecard Task 4；覆写 Task 3 占位）
 *
 * - 迁自 luckyU pages/index/index.vue：FortuneHeader + FortuneContent / LoadingSpinner /
 *   ZodiacSelector / FloatingActions 四态结构保留
 * - 数据流（spec 决定 14）：useProfileZodiac 进页回填 → checkZodiac → getMyTodayFortune；
 *   30505 清本地选座回选择器（useFortune 内，不 mock 不红 toast）；其他失败/空 → mock 降级（决定 16）
 * - 选座：zodiacStore.select(name)（PUT /profile 成功才落本地）→ 成功后 loadFortuneData()
 * - 自绘 nav 保留 Task 3 骨架（分包内返回）；luckyU 页内 wd-navbar 不迁（appui 全局 custom nav）
 * - 类名前缀 sf-（避 UnoCSS 撞名）；luckyU 组件内部类名原样保留（scoped，不冲突）
 */
import { ApiError } from '@/api/core/handlers'
import { useSystemBar } from '@/composables/useSystemBar'
import { useProfileZodiac } from '@/store/starfuyu/useProfileZodiac'
import { useZodiacStore, zodiacList } from '@/store/starfuyu/zodiac'
import FloatingActions from '../components/FloatingActions.vue'
import FortuneContent from '../components/FortuneContent.vue'
import FortuneHeader from '../components/FortuneHeader.vue'
import LoadingSpinner from '../components/LoadingSpinner.vue'
import ZodiacSelector from '../components/ZodiacSelector.vue'
import { useFortune } from '../composables/useFortune'

definePage({
  style: {
    navigationStyle: 'custom',
  },
})

const router = useRouter()
const toast = useToast()
const { pageStyle } = useSystemBar()
const zodiacStore = useZodiacStore()

// 进页回填星座偏好（onMounted 自动 GET /profile → hydrate）
useProfileZodiac()

const {
  fortuneData,
  isLoading,
  currentZodiac,
  hasSelectedZodiac,
  checkZodiac,
  loadFortuneData,
  handleReselect,
} = useFortune()

// 本地缓存已选 → 挂载即拉（数据以服务端 profile 为准）；
// 回填后「未选→已选」（本地无缓存、服务端有）→ 补拉。已加载中则由 loadFortuneData 幂等跳过。
onMounted(checkZodiac)
watch(hasSelectedZodiac, (has, was) => {
  if (has && !was)
    checkZodiac()
})

/** 选择星座：先 PUT /profile（服务端赢，失败不落本地），成功后拉运势。 */
async function handleZodiacSelect(zodiacId: number) {
  const target = zodiacList.find(z => z.id === zodiacId)
  if (!target)
    return
  try {
    await zodiacStore.select(target.name)
  }
  catch (error) {
    // 401/403 已由全局 handlers 清态提示，不重复 toast
    if (error instanceof ApiError && error.code !== 401 && error.code !== 403)
      toast.error({ msg: error.message || '选择失败，请重试' })
    return
  }
  await loadFortuneData()
}

function goBack() {
  if (getCurrentPages().length > 1)
    uni.navigateBack()
  else
    router.replaceAll({ path: '/pages/mine/index' })
}

/** 分享文案（非微信端复制到剪贴板；微信端走 button open-type="share" → onShareAppMessage）。 */
function shareFortune() {
  const fortune = fortuneData.value
  if (!fortune) {
    toast.show({ msg: '暂无运势数据' })
    return
  }
  const shareText = `${fortune.constellation}今日运势\n\n综合运势：${fortune.summary}\n\n爱情运势：${fortune.love_tip}\n\n事业运势：${fortune.career_tip}\n\n财富运势：${fortune.wealth_tip}\n\n健康运势：${fortune.health_tip}`

  // #ifndef MP-WEIXIN
  uni.setClipboardData({
    data: shareText,
    success: () => toast.show({ msg: '运势内容已复制' }),
    fail: () => toast.error({ msg: '复制失败，请重试' }),
  })
  // #endif
}

onShareAppMessage(() => {
  const fortune = fortuneData.value
  if (!fortune)
    return { title: '星座运势', path: '/pages/starfuyu/index/index' }
  return {
    title: `${fortune.constellation}今日运势`,
    desc: fortune.summary,
    path: '/pages/starfuyu/index/index',
  }
})

onShareTimeline(() => {
  const fortune = fortuneData.value
  if (!fortune)
    return { title: '星座运势', path: '/pages/starfuyu/index/index' }
  return {
    title: `${fortune.constellation}今日运势`,
    path: '/pages/starfuyu/index/index',
  }
})
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
        星座运势
        <text class="sf-sub">
          趣味小工具 · 仅供娱乐
        </text>
      </view>
      <view class="sf-side right" />
    </view>

    <view class="sf-body">
      <!-- 运势内容展示（已选择星座） -->
      <view
        v-if="fortuneData && currentZodiac"
        class="fortune-container"
      >
        <FortuneHeader
          :zodiac="currentZodiac"
          :fortune-data="fortuneData"
          :date="fortuneData.date"
          :lucky-color="fortuneData.lucky_color"
          @share="shareFortune"
          @reselect="handleReselect"
        />
        <FortuneContent :fortune-data="fortuneData" />
      </view>

      <!-- 加载状态 -->
      <LoadingSpinner
        v-else-if="isLoading"
        text="正在加载运势..."
      />

      <!-- 星座选择界面（未选择星座时显示） -->
      <ZodiacSelector
        v-else
        :zodiac-list="zodiacList"
        @select="handleZodiacSelect"
      />

      <!-- 悬浮操作按钮（仅在显示运势时显示） -->
      <FloatingActions
        v-if="fortuneData && currentZodiac"
        :on-share="shareFortune"
        :on-reselect="handleReselect"
      />
    </view>
  </view>
</template>

<style lang="scss" scoped>
.sf-page {
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  min-height: 100vh;
  padding: calc(var(--status-bar-h, env(safe-area-inset-top, 0px)) + 6px) 16px calc(env(safe-area-inset-bottom, 0px) + 16px);
  color: var(--on-ink);
  background: var(--ink-900);
  background-image: var(--spot);
}

/* 自绘 nav（同 mine/manual 页骨架；Task 3 占位原样保留） */
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

/* 四态容器：纵向弹性布局，选择器 flex:1 吃掉剩余高度（内部 overflow 滚动） */
.sf-body {
  position: relative;
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
}

/* 运势长内容走页面滚动（luckyU 原样；6 卡高度恒超视口，min-height 不生效） */
.fortune-container {
  width: 100%;
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}
</style>
