/**
 * 小程序静默登录（免点击）：uni.login 取 code → wxLogin(code) → setLogin + fetchMe
 *
 * - 模块级 inFlight 去重：并发触发（App onLaunch / 首页 onShow / 登录页）复用同一 promise，不双发 wxLogin
 * - failed：本小程序生命周期内自动触发失败过一次即不再自动重试（ensureSilentLogin 短路）；
 *   登录页手动点击不受此限制（走 silentWxLogin 仍可重试）
 * - explicitLogout（store persist 标记）：主动退出后 ensureSilentLogin 不自动登回，手动登录成功（setLogin）清除
 * - 仅 MP-WEIXIN 真正执行；H5 ensureSilentLogin 恒 false、silentWxLogin 抛错（调用点也应条件编译）
 */
import type { LoginResult } from '@/api/scorecard/types'
import { wxLogin } from '@/api/scorecard/auth'
import { useScorecardAuthStore } from '@/store/scorecard/auth'

/** 在途静默登录 promise（并发去重）；完成后置 null 允许下次（手动重试）发起。 */
let inFlight: Promise<LoginResult> | null = null
/** 自动触发失败封顶标记（本次小程序生命周期）；手动点击失败也会置位（下次 onShow 不再自动打接口）。 */
let failed = false

async function doSilentWxLogin(): Promise<LoginResult> {
  // 条件编译分支内只赋值、函数单一出口：TS/ESLint 把 ifdef 当注释，避免 return 后 throw 被判 unreachable
  let res: LoginResult | null = null
  // #ifdef MP-WEIXIN
  const loginRes = await uni.login()
  const code = loginRes.code || ''
  if (!code)
    throw new Error('微信登录失败：code 为空')
  res = await wxLogin(code)
  // #endif
  if (!res)
    throw new Error('静默登录仅支持微信小程序')
  const authStore = useScorecardAuthStore()
  authStore.setLogin(res)
  try {
    await authStore.fetchMe()
  }
  catch {
    // 资料缓存失败不阻断登录（token 已落 store 并持久化），同 afterLogin 口径
  }
  return res
}

/**
 * 单次静默微信登录；并发调用复用同一在途 promise（spec「并发触发去重」）。
 * 失败：置 failed 封顶后向调用方抛出（登录页可 catch 后展示手动按钮）。
 */
export function silentWxLogin(): Promise<LoginResult> {
  if (inFlight)
    return inFlight
  const p = doSilentWxLogin().finally(() => {
    inFlight = null
  })
  inFlight = p
  p.catch(() => {
    failed = true
  })
  return p
}

/**
 * 确保本地有可用登录态：有 token → true；explicitLogout / failed / H5 → false；
 * 否则发起（或复用在途）静默登录，成功 true 失败 false（不抛）。
 */
export async function ensureSilentLogin(): Promise<boolean> {
  const authStore = useScorecardAuthStore()
  if (authStore.token)
    return true
  if (authStore.explicitLogout || failed)
    return false
  let ok = false
  // #ifdef MP-WEIXIN
  try {
    await silentWxLogin()
    ok = true
  }
  catch {
    ok = false
  }
  // #endif
  return ok
}
