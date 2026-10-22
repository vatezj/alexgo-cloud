/**
 * 打牌记账登录态 store
 *
 * - persist：走脚手架 pinia persist 插件（store/persist.ts 按 $id 持久化），$id = 'scorecard-auth'
 * - token/refreshToken 令牌三元组（LoginResult）；user 为 /member/me 资料缓存（可空）
 * - lastRoomId：最近进入的房间 id（0 = 无），供首页「继续上局」用
 * - 循环依赖边界：本 store 只静态 import types；getMe 走动态 import
 *   （handlers/instance 会 import 本 store，静态引 api 会成环）
 */
import type { LoginResult, MeResult } from '@/api/scorecard/types'

export const useScorecardAuthStore = defineStore('scorecard-auth', {
  state: () => ({
    token: '',
    refreshToken: '',
    user: null as MeResult | null,
    lastRoomId: 0,
  }),

  actions: {
    /** 登录/刷新成功后写入令牌三元组（expires_in 前端不消费，仅持久化 token 对）。 */
    setLogin(res: LoginResult) {
      this.token = res.token
      this.refreshToken = res.refresh_token
    },

    /** 拉取当前会员资料写入 user（401 由 handlers 清态跳登录）。 */
    async fetchMe() {
      const { getMe } = await import('@/api/scorecard/auth')
      this.user = await getMe()
    },

    /** 本地清态（主动退出 / 401 失效共用）；lastRoomId 保留便于回访。 */
    logout() {
      this.token = ''
      this.refreshToken = ''
      this.user = null
    },
  },
})
