/**
 * 运势数据管理 Composable（改写自 luckyU composables/useFortune.ts；接口换 getMyTodayFortune）
 *
 * 分支约定（Review Focus #2 / 决定 16）：
 * - ApiError 30505（服务端未设星座）→ `zodiacStore.hydrate('')` 清本地选座 + 清 fortuneData
 *   → 页面回选择器；**不 mock、不 toast**（避免红错循环）
 * - 其他失败 / 空响应 → generateMockFortune 降级 + toast「获取运势失败，已使用缓存数据」
 *
 * 调用时机：须在页面 setup 内同步调用（useToast 走 provide/inject，依赖页面作用域）。
 */
import type { ComputedRef, Ref } from 'vue'
import type { Fortune } from '@/api/starfuyu'
import type { Zodiac } from '@/store/starfuyu/zodiac'
import { ApiError } from '@/api/core/handlers'
import { getMyTodayFortune } from '@/api/starfuyu'
import { useZodiacStore } from '@/store/starfuyu/zodiac'
import { generateMockFortune } from '../utils/mockFortune'

export interface FortuneApi {
  fortuneData: Ref<Fortune | null>
  isLoading: Ref<boolean>
  currentZodiac: ComputedRef<Zodiac | null>
  hasSelectedZodiac: ComputedRef<boolean>
  /** 有选座则 load；无选座保持选择器展示。 */
  checkZodiac: () => void
  /** getMyTodayFortune；30505 清选座回选择器，其他失败/空 → mock + toast。 */
  loadFortuneData: () => Promise<void>
  /** 重选星座：清本地选座与运势（服务端 profile 不动，再选时 select PUT 覆盖）。 */
  handleReselect: () => void
}

export function useFortune(): FortuneApi {
  const zodiacStore = useZodiacStore()
  const toast = useToast()
  const { current, hasSelected } = storeToRefs(zodiacStore)

  const fortuneData = ref<Fortune | null>(null)
  const isLoading = ref(false)

  /** mock 降级：伪造当前星座数据 + 缓存提示（决定 16）。 */
  function applyMockFallback(): void {
    const zodiac = current.value
    if (!zodiac)
      return
    fortuneData.value = generateMockFortune(zodiac.id, zodiac.name)
    toast.show({ msg: '获取运势失败，已使用缓存数据' })
  }

  function checkZodiac(): void {
    if (hasSelected.value && current.value)
      loadFortuneData()
  }

  async function loadFortuneData(): Promise<void> {
    if (!current.value)
      return
    // 幂等：选座成功后「watch 补拉」与页面显式 load 可能双触发，进行中则跳过
    if (isLoading.value)
      return

    isLoading.value = true
    try {
      const data = await getMyTodayFortune()
      if (data && data.constellation)
        fortuneData.value = data
      else
        applyMockFallback() // 空响应 → 降级
    }
    catch (error) {
      if (error instanceof ApiError && error.code === 30505) {
        // 未设星座：清本地选座回选择器——不伪造数据、不 toast（防红错循环）
        fortuneData.value = null
        zodiacStore.hydrate('')
      }
      else {
        applyMockFallback() // 其他失败 → 降级
      }
    }
    finally {
      isLoading.value = false
    }
  }

  function handleReselect(): void {
    zodiacStore.hydrate('')
    fortuneData.value = null
  }

  return {
    fortuneData,
    isLoading,
    currentZodiac: current,
    hasSelectedZodiac: hasSelected,
    checkZodiac,
    loadFortuneData,
    handleReselect,
  }
}
