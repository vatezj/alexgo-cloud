/**
 * 打牌记账语音 5 档 store（设计 10 / 手册 §3.8）
 *
 * - state 三 bool：{entertain, ledger, roomDyn}（娱乐 / 流水 / 房间动态），persist 按 $id
 *   = 'scorecard-voice'（脚手架 pinia persist 插件自动落盘）
 * - 5 档顺序（nextMode 环）：all-off → all-on → entertain-off → ledger-off → ops-only → all-off
 * - mode 与三 bool 的映射是单射（仅 5 个可达组合）；modeFromFlags 对持久化坏数据
 *   回退 'ops-only'（默认档：操作播放）
 * - 默认档 ops-only（手册：默认操作播放——人员进出/结算）；纯函数 nextMode/modeFromFlags
 *   可在控制台直接对稿顺序
 */

export type VoiceMode = 'all-off' | 'all-on' | 'entertain-off' | 'ledger-off' | 'ops-only'

export interface VoiceFlags {
  entertain: boolean
  ledger: boolean
  roomDyn: boolean
}

/** 5 档循环顺序（对稿手册 §3.8：全部关闭 → 全部播放 → 娱乐关闭 → 流水关闭 → 操作播放）。 */
export const VOICE_MODE_ORDER: readonly VoiceMode[] = ['all-off', 'all-on', 'entertain-off', 'ledger-off', 'ops-only']

/** mode → 三 bool 映射（speak 档位过滤源）。 */
export const VOICE_MODE_FLAGS: Record<VoiceMode, VoiceFlags> = {
  'all-off': { entertain: false, ledger: false, roomDyn: false },
  'all-on': { entertain: true, ledger: true, roomDyn: true },
  'entertain-off': { entertain: false, ledger: true, roomDyn: true },
  'ledger-off': { entertain: true, ledger: false, roomDyn: true },
  'ops-only': { entertain: false, ledger: false, roomDyn: true },
}

/** 5 档文案（设计 10 vopt 逐字）。 */
export const VOICE_MODE_LABELS: Record<VoiceMode, string> = {
  'all-off': '全部关闭',
  'all-on': '全部播放',
  'entertain-off': '娱乐关闭',
  'ledger-off': '流水关闭',
  'ops-only': '操作播放',
}

/** 5 档图标（设计 10 vopt 逐字）。 */
export const VOICE_MODE_ICONS: Record<VoiceMode, string> = {
  'all-off': '🔇',
  'all-on': '🔊',
  'entertain-off': '🎭',
  'ledger-off': '💰',
  'ops-only': '✅',
}

/** 5 档副文案（设计 10 vdesc；无副文案的档给空串）。 */
export const VOICE_MODE_DESCS: Record<VoiceMode, string> = {
  'all-off': '',
  'all-on': '',
  'entertain-off': '人声欢迎语等',
  'ledger-off': '记分播报',
  'ops-only': '默认 · 进出/结算',
}

/** 下一档（环）。纯函数：console 可直接对稿循环顺序。 */
export function nextMode(mode: VoiceMode): VoiceMode {
  const idx = VOICE_MODE_ORDER.indexOf(mode)
  return VOICE_MODE_ORDER[(idx + 1) % VOICE_MODE_ORDER.length] ?? 'all-off'
}

/** 三 bool → mode（仅 5 可达组合命中；持久化坏数据回退 'ops-only'）。 */
export function modeFromFlags(flags: VoiceFlags): VoiceMode {
  for (const mode of VOICE_MODE_ORDER) {
    const t = VOICE_MODE_FLAGS[mode]
    if (t.entertain === flags.entertain && t.ledger === flags.ledger && t.roomDyn === flags.roomDyn)
      return mode
  }
  return 'ops-only'
}

export const useScorecardVoiceStore = defineStore('scorecard-voice', {
  state: () => ({
    // 默认档：操作播放（手册 §3.8——人员进出/结算）
    entertain: false,
    ledger: false,
    roomDyn: true,
  }),

  actions: {
    /** 写入指定档（三 bool 组合写入，保证恒为 5 可达组合之一）。 */
    setMode(mode: VoiceMode) {
      const flags = VOICE_MODE_FLAGS[mode]
      this.entertain = flags.entertain
      this.ledger = flags.ledger
      this.roomDyn = flags.roomDyn
    },

    /** 5 档循环一格。 */
    cycle() {
      this.setMode(nextMode(modeFromFlags(this)))
    },
  },
})
