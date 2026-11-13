/**
 * 进页回填星座偏好：getProfile() → hydrate（不打 select，只读回填）。
 *
 * - 401 由全局 handlers 清态跳登录；30505 / constellation 空串保持未选
 * - 其它请求失败静默：persist 本地缓存兜底（服务端为权威，下次进页再拉）
 * - 返回类型钉死：{ init: () => Promise<void> }（无测试运行器，类型即测试）
 */
import { getProfile } from '@/api/starfuyu'
import { useZodiacStore } from './zodiac'

export interface ProfileZodiacApi {
  /** 显式拉取 profile 并回填（组件挂载时已自动调一次；下拉刷新等场景可手动再调）。 */
  init: () => Promise<void>
}

export function useProfileZodiac(): ProfileZodiacApi {
  const store = useZodiacStore()

  async function init(): Promise<void> {
    try {
      const data = await getProfile()
      store.hydrate(data?.constellation || '')
    }
    catch {
      // 请求失败静默：本地持久化缓存兜底；401 已由全局 handlers 处理
    }
  }

  // 进页自动回填；非组件上下文调用时 onMounted 不生效，可手动 init()
  onMounted(init)

  return { init }
}
