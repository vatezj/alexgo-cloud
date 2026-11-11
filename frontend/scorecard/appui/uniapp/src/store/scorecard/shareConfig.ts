/**
 * 三页分享卡配置 store（openspec share-card-config）
 *
 * - 三级回退：内存 state →（persist 插件启动时已把 storage 灌进 state）→ DEFAULTS
 *   DEFAULTS 与服务端 modules/scorecard/service/sharecfg.go 的 shareCardDefaults 同值——
 *   接口全挂时行为 = 硬编码时代，不劣化
 * - persist：走脚手架 pinia persist 插件（$id = 'scorecard:shareCards' 即 storage 键）
 * - ensureFresh：拉取成功才写 state（失败静默、不清已有缓存）；在途 promise 去重
 * - 转发钩子是同步函数：只读 get()，永不 await 请求
 */
import type { ShareCard, ShareCards } from '@/api/scorecard/sharecfg'

/** 内置默认（与服务端 sharecfg.go shareCardDefaults 逐字一致）。 */
const DEFAULTS: ShareCards = {
  index: { title: '登录后开房记账 · 好友互通', imageUrl: 'http://qiniucdn.vate.ren/dapaijizhang/logo.png' },
  simulator: { title: '座位模拟器 · 趣味方位参考 · 仅供娱乐', imageUrl: '' },
  salary: { title: '工资到账计算器 · 填工资看入账', imageUrl: '' },
}

/** 拉取在途去重（跨 store 实例共享，onShow 连触发只发一次请求）。 */
let inFlight: Promise<void> | null = null

export const useScorecardShareConfigStore = defineStore('scorecard:shareCards', {
  state: () => ({
    /** 后台最新值（persist 落盘；空对象 = 尚未拉取过） */
    cards: {} as Partial<ShareCards>,
  }),

  actions: {
    /**
     * 读单页分享卡：内存 → 内置默认。
     * 字段级兜底（后台只写了 title 漏了 imageUrl 等半残数据也不产 undefined）。
     */
    get(page: keyof ShareCards): ShareCard {
      const cached = this.cards[page]
      const def = DEFAULTS[page]
      return {
        title: cached?.title || def.title,
        imageUrl: cached?.imageUrl ?? def.imageUrl,
      }
    },

    /**
     * 后台刷新：拉取写 state + persist 自动落盘。
     * 失败静默（不清缓存、不上 toast——分享回退默认即可）；在途去重。
     */
    ensureFresh(): Promise<void> {
      if (inFlight)
        return inFlight
      inFlight = (async () => {
        try {
          const { getShareConfig } = await import('@/api/scorecard/sharecfg')
          this.cards = await getShareConfig()
        }
        catch {
          // 网络失败/非 0 code：保留旧缓存与默认回退，静默降级
        }
        finally {
          inFlight = null
        }
      })()
      return inFlight
    },
  },
})
