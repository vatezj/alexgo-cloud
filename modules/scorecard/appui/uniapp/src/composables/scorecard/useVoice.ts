/**
 * 语音播报组合式（设计 10 / 手册 §3.8）
 *
 * - speak(text, category)：档位过滤（category 对应 entertain/ledger/roomDyn 三 bool）→
 *   mp 端 WechatSI 插件可用则 TTS（uni.createInnerAudioContext 播 filename）；
 *   不可用 toast 一次「语音插件未启用」后本会话静默；H5 走 speechSynthesis（不可用静默）；
 *   其他端静默。**任何失败不 throw**（外层 try/catch 兜底，调用方零风险接线）
 * - currentMode：Ref<VoiceMode>（由 voiceStore 三 bool 反查，坏数据回退 ops-only）
 * - 本文件是 expense 记分 / 结算 entertain 等播报的唯一入口
 */
import type { VoiceMode } from '@/store/scorecard/voice'
import { modeFromFlags, useScorecardVoiceStore, VOICE_MODE_FLAGS } from '@/store/scorecard/voice'

/** 播报分类（与三 bool 键名一致）。 */
export type SpeakCategory = 'entertain' | 'ledger' | 'roomDyn'

/** WechatSI.textToSpeech 形状（插件未装时 requirePlugin 抛错/返回空，调用侧已兜底）。 */
interface WechatSIPlugin {
  textToSpeech: (opts: {
    lang: string
    tts: boolean
    content: string
    success?: (res: { filename?: string }) => void
    fail?: () => void
  }) => void
}

/** 本会话插件缺失 toast 只弹一次（模块级单例，跨组件共享）。 */
let pluginWarned = false

/** 取 WechatSI 插件实例（未声明/未安装/形状不对 → null；永不 throw）。 */
function getWechatSI(): WechatSIPlugin | null {
  try {
    const req = (globalThis as { requirePlugin?: (name: string) => unknown }).requirePlugin
    if (typeof req !== 'function')
      return null
    const plugin = req('WechatSI') as WechatSIPlugin | undefined
    return plugin && typeof plugin.textToSpeech === 'function' ? plugin : null
  }
  catch {
    return null
  }
}

/** 插件缺失提示：toast 一次后本会话静默。 */
function warnPluginOnce(): void {
  if (pluginWarned)
    return
  pluginWarned = true
  try {
    // 走全局 toast store（Pinia），不能用 useToast()——后者靠 inject/provide，只在 setup 有效；
    // speak 跑在事件回调里（currentInstance 为 null），useToast 写的 ref 无人渲染（同 api/core/handlers 模式）
    const toast = useGlobalToast()
    toast.info({ msg: '语音插件未启用' })
  }
  catch {
    // toast 初始化失败也静默（speak 永不 throw）
  }
}

/** H5：speechSynthesis（不可用/失败静默）。 */
function speakH5(text: string): void {
  // #ifdef H5
  try {
    if (typeof window === 'undefined' || !('speechSynthesis' in window) || typeof SpeechSynthesisUtterance === 'undefined')
      return
    window.speechSynthesis.cancel()
    const utter = new SpeechSynthesisUtterance(text)
    utter.lang = 'zh-CN'
    window.speechSynthesis.speak(utter)
  }
  catch {
    // 静默
  }
  // #endif
}

/** mp：WechatSI TTS → innerAudioContext 播放；插件不可用 toast 一次后静默。 */
function speakMp(text: string): void {
  // #ifdef MP-WEIXIN
  try {
    const plugin = getWechatSI()
    if (!plugin) {
      warnPluginOnce()
      return
    }
    plugin.textToSpeech({
      lang: 'zh_CN',
      tts: true,
      content: text,
      success: (res) => {
        try {
          const src = res?.filename
          if (!src)
            return
          const audio = uni.createInnerAudioContext()
          audio.src = src
          audio.onEnded(() => {
            try {
              audio.destroy()
            }
            catch {
              // 静默
            }
          })
          audio.onError(() => {
            try {
              audio.destroy()
            }
            catch {
              // 静默
            }
          })
          audio.play()
        }
        catch {
          // 静默
        }
      },
      // TTS 失败静默（不 toast：属渲染失败而非插件缺失）
      fail: () => {},
    })
  }
  catch {
    warnPluginOnce()
  }
  // #endif
}

/**
 * 打牌记账语音播报
 *
 * @example
 * ```ts
 * const { speak, currentMode } = useVoice()
 * speak(`${nickname}支出${score}分`, 'ledger') // 档位过滤 + 平台降级，永不 throw
 * ```
 */
export function useVoice() {
  const voiceStore = useScorecardVoiceStore()

  /** 当前档（三 bool 反查；持久化坏数据回退 ops-only）。 */
  const currentMode = computed<VoiceMode>(() => modeFromFlags(voiceStore))

  /** 播报（档位过滤该 category；任何失败不 throw）。 */
  function speak(text: string, category: SpeakCategory): void {
    try {
      if (!text)
        return
      // 以规范映射过滤（不用 store 原始 bool，坏数据也能落到规范档行为）
      if (!VOICE_MODE_FLAGS[currentMode.value][category])
        return
      speakH5(text)
      speakMp(text)
    }
    catch {
      // 任何失败不 throw：播报永远不打断业务流
    }
  }

  return { speak, currentMode }
}
