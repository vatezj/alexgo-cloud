/**
 * 资料弹框编排（index / login 两页共用，配 ProfileGate.vue）
 *
 * - requireProfileForAction（gate 硬门，开房用）：资料完整返回 false（调用方自行继续）；
 *   不完整弹 gate 模式，保存成功后自动续走 action；取消/X 只清挂起、不执行
 * - ensureProfileThen（reminder 软提醒，四条进局路径用）：资料完整立即执行 action 返回 true；
 *   不完整弹 reminder（仅小跳过可关），保存/跳过后续走 action
 * - proceed / cancel：ProfileGate 的 saved/skipped 与 cancelled 事件统一收口
 *
 * pendingAction 只存内存（页面实例级），页面卸载即弃、无泄漏。
 */
import { useScorecardAuthStore } from '@/store/scorecard/auth'
import { isProfileComplete } from '@/utils/scorecard/profile'

export type ProfileGateMode = 'gate' | 'reminder' | 'edit'

export interface ProfileGateController {
  /** 弹框可见（reactive 解包后为 boolean，模板可直接 v-model:show） */
  show: boolean
  /** 当前模式（gate / reminder / edit） */
  mode: ProfileGateMode
  /** 开房硬门：不完整弹框挂起动作返回 true；完整返回 false（调用方继续）。 */
  requireProfileForAction: (action: () => void | Promise<void>) => boolean
  /** 进局软提醒：完整立即执行返回 true；不完整弹框挂起返回 false。 */
  ensureProfileThen: (action: () => void | Promise<void>) => boolean
  /** ProfileGate saved/skipped：关弹框、续走挂起动作。 */
  proceed: () => void
  /** ProfileGate cancelled（gate 取消/X）：只清挂起动作，不执行。 */
  cancel: () => void
}

export function useProfileGate(): ProfileGateController {
  const authStore = useScorecardAuthStore()
  const show = ref(false)
  const mode = ref<ProfileGateMode>('gate')
  const pendingAction = ref<(() => void | Promise<void>) | null>(null)

  function pend(next: ProfileGateMode, action: () => void | Promise<void>): boolean {
    mode.value = next
    pendingAction.value = action
    show.value = true
    return true
  }

  function requireProfileForAction(action: () => void | Promise<void>): boolean {
    if (isProfileComplete(authStore.user))
      return false
    return pend('gate', action)
  }

  function ensureProfileThen(action: () => void | Promise<void>): boolean {
    if (isProfileComplete(authStore.user)) {
      void action()
      return true
    }
    return pend('reminder', action)
  }

  function proceed() {
    show.value = false
    const run = pendingAction.value
    pendingAction.value = null
    if (run)
      void run()
  }

  function cancel() {
    pendingAction.value = null
  }

  // reactive 包装：内部仍是 ref（.value 读写），返回后属性解包为原始值，模板 `gate.show` 直接可用
  return reactive({ show, mode, requireProfileForAction, ensureProfileThen, proceed, cancel })
}
