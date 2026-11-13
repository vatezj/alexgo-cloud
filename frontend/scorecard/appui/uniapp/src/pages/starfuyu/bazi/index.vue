<script setup lang="ts">
/**
 * 星座运势 · 八字+求签页（luckyu-into-scorecard Task 5；覆写 Task 3 占位壳）
 *
 * - 迁自 luckyU bazhi/index.vue（340 行）；组件在 ../components/bazi/（8 个，brief 列 5 个
 *   之外多迁 BirthForm/BaziCard/PageDecorations——页面结构必需，同 Task 4 Deviations-2 口径）
 * - API 换 Task 1（@/api/starfuyu）：
 *     旧 calculateBazi        → 同名（无已存八字时；ApiError 30503 限流）
 *     旧 —（无）              → getMyBazi 进页预取（30504 = 尚未保存，文案表 toast）
 *     旧 —（无）              → updateMyBazi（已存八字后的再次排盘，覆盖保存）
 *     旧 drawDivination       → 同名（30503 限流）
 *     divinationHistory/getSign → 源页无历史列表、签文取自 draw 结果 → 未消费（见报告）
 * - 错误码 toast 文案表：30503→今日次数已达上限（锁定提交至次日本地日期翻转，new Date() 不引
 *   dayjs）、30504→尚未保存生辰八字、30505→请先选择星座、10001→参数校验失败；
 *   401/403 全局 handlers 已提示不重复
 * - 未迁：handleLogin（luckyU wxLogin 流，appui 走 scorecard 登录态 + 全局 401 handlers）、
 *   loadFonts（源码整段注释、static/font 空）、setupWeChatShare 的旧路径换分包路径
 * - 底色用 luckyU 纸面 #f3e5d8（组件均为纸面设计：0.6 白卡/暗红标题/四角装饰，墨色壳上对比度
 *   不足）；sf-nav 结构保留 Task 3 骨架、配色随纸面改暗红（见报告 Deviations）
 * - 类名前缀 sf-（避 UnoCSS 撞名）；组件内部类名原样（scoped）
 */
import type { Bazi, BaziInfo, Divination } from '@/api/starfuyu'
import { ApiError } from '@/api/core/handlers'
import { calculateBazi, drawDivination, getMyBazi, updateMyBazi } from '@/api/starfuyu'
import { useSystemBar } from '@/composables/useSystemBar'
import BaziCard from '../components/bazi/BaziCard.vue'
import BaziDisplay from '../components/bazi/BaziDisplay.vue'
import BaziHeader from '../components/bazi/BaziHeader.vue'
import BaziLoading from '../components/bazi/BaziLoading.vue'
import BirthForm from '../components/bazi/BirthForm.vue'
import DivinationForm from '../components/bazi/DivinationForm.vue'
import PageDecorations from '../components/bazi/PageDecorations.vue'
import QianTong from '../components/bazi/QianTong.vue'

definePage({
  style: {
    navigationStyle: 'custom',
  },
})

const router = useRouter()
const toast = useToast()
const { pageStyle } = useSystemBar()

/* ---- 八字数据（四柱展示；label 结构沿用源稿） ---- */
const baziData = reactive([
  { label: '年柱', heaven: '甲', earth: '辰' },
  { label: '月柱', heaven: '丙', earth: '寅' },
  { label: '日柱', heaven: '庚', earth: '午' },
  { label: '时柱', heaven: '壬', earth: '申' },
])

const showBazi = ref(false)
const showResult = ref(false)
const isLoading = ref(false)
const loadingType = ref<'bazi' | 'divination'>('bazi')
const loadingMessage = ref('')
const birthFormRef = ref<InstanceType<typeof BirthForm> | null>(null)

// 签文结果（Task 1 Divination；draw 返回即含 sign_number/sign_content，无需 getSign）
const qianResult = ref<Divination | undefined>(undefined)

// 服务端已有八字（进页预取或首次排盘成功后置位；决定 update vs calculate）
const hasSavedBazi = ref(false)

/* ---- 30503 限流锁定：本地日期翻转解锁（new Date()，不引 dayjs） ---- */
/** 命中 30503 的本地日期 'YYYY-MM-DD'；'' = 未锁定。 */
const rateLimitedOn = ref('')
const isRateLimited = computed(() => rateLimitedOn.value !== '' && rateLimitedOn.value === todayStr())

function todayStr(): string {
  const d = new Date()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${d.getFullYear()}-${m}-${day}`
}

/** 次日本地日期已翻转则清锁（onShow + 60s interval + 提交前调用，computed 依赖翻转）。 */
function refreshRateLimit(): void {
  if (rateLimitedOn.value && rateLimitedOn.value !== todayStr())
    rateLimitedOn.value = ''
}

/* ---- 错误码 toast 文案表（Task 5 绑定） ---- */
const ERR_MSG: Record<number, string> = {
  30503: '今日次数已达上限',
  30504: '尚未保存生辰八字',
  30505: '请先选择星座',
  10001: '参数校验失败',
}

/** 业务信封错误本页提示；30503 顺带锁定提交；401/403（handlers 已全局提示）不重复 toast。 */
function toastBaziError(err: unknown, fallback: string): void {
  if (err instanceof ApiError) {
    if (err.code === 401 || err.code === 403)
      return
    if (err.code === 30503)
      rateLimitedOn.value = todayStr()
    toast.error({ msg: ERR_MSG[err.code] || err.message || fallback })
    return
  }
  toast.error({ msg: err instanceof Error && err.message ? err.message : fallback })
}

/** BaziInfo → 四柱展示数据。 */
function applyBaziInfo(info: BaziInfo): void {
  baziData[0].heaven = info.year_gan
  baziData[0].earth = info.year_zhi
  baziData[1].heaven = info.month_gan
  baziData[1].earth = info.month_zhi
  baziData[2].heaven = info.day_gan
  baziData[2].earth = info.day_zhi
  baziData[3].heaven = info.hour_gan
  baziData[3].earth = info.hour_zhi
  showBazi.value = true
}

/** 进页预取我的八字：有则展示；30504 = 尚未保存（预期空态，文案表提示「首次排盘即可」）；其余静默。 */
async function loadMyBazi(): Promise<void> {
  try {
    const saved = await getMyBazi()
    applyBaziInfo(saved.bazi_info)
    hasSavedBazi.value = true
  }
  catch (error) {
    if (error instanceof ApiError && error.code === 30504)
      toastBaziError(error, '')
    // 401/403 全局 handlers；网络/500 等预取失败静默——不打断表单主流程
  }
}

/** 60s 超时竞速（源稿原样）。 */
function withTimeout<T>(promise: Promise<T>): Promise<T> {
  const timeoutPromise = new Promise<never>((_, reject) => {
    setTimeout(() => {
      reject(new Error('请求超时，请检查网络后重试'))
    }, 60000)
  })
  return Promise.race([promise, timeoutPromise])
}

/** 排盘测算：无已存八字 calculate，已存 update（同为覆盖保存，语义对齐 Consumes）。 */
async function handleSubmit(): Promise<void> {
  refreshRateLimit()
  if (isRateLimited.value) {
    toast.error({ msg: ERR_MSG[30503] })
    return
  }
  if (!birthFormRef.value)
    return

  isLoading.value = true
  loadingType.value = 'bazi'
  loadingMessage.value = ''

  try {
    const birthData = birthFormRef.value.getBirthData()
    const result: Bazi = await withTimeout(
      hasSavedBazi.value ? updateMyBazi(birthData) : calculateBazi(birthData),
    )
    applyBaziInfo(result.bazi_info)
    hasSavedBazi.value = true
    toast.success('排盘成功')
  }
  catch (error) {
    toastBaziError(error, '排盘失败，请重试')
  }
  finally {
    isLoading.value = false
  }
}

/** 求签：成功 → 展示签文/解读（draw 结果自带 sign_number，无需 getSign）。 */
async function handleDivinationSubmit(data: { question: string, category: string }): Promise<void> {
  refreshRateLimit()
  if (isRateLimited.value) {
    toast.error({ msg: ERR_MSG[30503] })
    return
  }

  isLoading.value = true
  loadingType.value = 'divination'
  loadingMessage.value = ''

  try {
    const result: Divination = await withTimeout(
      drawDivination({ question: data.question, category: data.category }),
    )
    qianResult.value = result
    showResult.value = true
    toast.success('求签成功')
  }
  catch (error) {
    toastBaziError(error, '求签失败，请重试')
  }
  finally {
    isLoading.value = false
  }
}

/** 重新求签：隐藏结果回表单。 */
function handleShake(): void {
  showResult.value = false
  qianResult.value = undefined
}

function goBack(): void {
  if (getCurrentPages().length > 1)
    uni.navigateBack()
  else
    router.replaceAll({ path: '/pages/starfuyu/index/index' })
}

/* ---- 生命周期 ---- */
let rateLimitTimer: ReturnType<typeof setInterval> | undefined

onMounted(() => {
  refreshRateLimit()
  void loadMyBazi()
  // 页面常开跨零点：60s 巡检解锁（按钮 disabled 时点击不触发，无法靠提交解锁）
  rateLimitTimer = setInterval(refreshRateLimit, 60_000)
})

onShow(() => {
  refreshRateLimit()
})

onUnmounted(() => {
  clearInterval(rateLimitTimer)
})

onShareAppMessage(() => {
  return {
    title: '算一卦，测一命',
    path: '/pages/starfuyu/bazi/index',
    imageUrl: '/static/starfuyu/share_bazi.jpg',
  }
})

onShareTimeline(() => {
  return {
    title: '算一卦，测一命',
    path: '/pages/starfuyu/bazi/index',
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
        八字求签
        <text class="sf-sub">
          趣味小工具 · 仅供娱乐
        </text>
      </view>
      <view class="sf-side right" />
    </view>

    <PageDecorations />

    <BaziHeader />

    <!-- 主内容区 -->
    <view class="sf-content">
      <!-- 表单卡片 -->
      <BaziCard>
        <BirthForm
          ref="birthFormRef"
          :on-submit="handleSubmit"
          :disabled="isRateLimited"
        />
      </BaziCard>

      <!-- 八字展示卡片 -->
      <BaziCard v-if="showBazi">
        <BaziDisplay :bazi-data="baziData" />
      </BaziCard>

      <!-- 求签卡片 -->
      <BaziCard>
        <DivinationForm
          v-if="!showResult"
          :disabled="isRateLimited"
          @submit="handleDivinationSubmit"
        />
        <QianTong
          v-else
          :show-result="showResult"
          :result="qianResult"
          @shake="handleShake"
        />
      </BaziCard>
    </view>

    <!-- 加载遮罩 -->
    <BaziLoading
      v-if="isLoading"
      :type="loadingType"
      :message="loadingMessage"
    />
  </view>
</template>

<style lang="scss" scoped>
/* 纸面底：组件均为 luckyU 纸面设计（0.6 白卡/暗红标题/四角装饰），墨色壳上对比度不足 */
.sf-page {
  box-sizing: border-box;
  min-height: 100vh;
  padding: calc(var(--status-bar-h, env(safe-area-inset-top, 0px)) + 6px) 16px calc(env(safe-area-inset-bottom, 0px) + 16px);
  color: #1a1a1a;
  background-color: #f3e5d8;
  overflow-x: hidden;
}

/* 自绘 nav（Task 3 骨架原样；配色随纸面改暗红系） */
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
  color: #8b0000;
  font-family: 'Noto Serif SC', serif;
  font-size: 26px;
  line-height: 1;
}

.sf-title {
  color: #8b0000;
  font-family: 'Noto Serif SC', serif;
  font-size: 17px;
  font-weight: 500;
  letter-spacing: 0.06em;
  text-align: center;
}

.sf-sub {
  display: block;
  margin-top: 2px;
  color: #5c4033;
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 10.5px;
  font-weight: 400;
  letter-spacing: 0.02em;
}

/* 内容区（源稿 .content padding 0 48rpx + page-container 底距原样） */
.sf-content {
  padding: 0 48rpx 60rpx;
}
</style>
