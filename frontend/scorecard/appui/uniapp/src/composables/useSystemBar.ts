import { computed, ref } from 'vue'

/**
 * 系统栏适配组合式 API：状态栏高度 + 微信真胶囊避让
 *
 * - 状态栏：小程序读 statusBarHeight 注入 --status-bar-h；未注入时页面 CSS 回退
 *   env(safe-area-inset-top)（H5 iOS 刘海兜底；安卓 H5 env 恒 0，与现状一致）
 * - 胶囊：MP-WEIXIN 读 getMenuButtonBoundingClientRect 计算 --nav-right-pad
 *   = windowWidth - capsule.left + 8，让自绘 nav 右侧内容（标题/按钮）止于真胶囊左侧
 * - 装饰假胶囊由模板条件编译 #ifndef MP-WEIXIN 控制：H5 保留源稿装饰，小程序
 *   隐藏避免与系统真胶囊重叠（房间页标题点击仍可进邀请，功能不丢）
 * - 同步 API 模块级只读一次，失败静默回退 CSS env() 兜底
 *
 * @example
 * ```vue
 * <script setup lang="ts">
 * import { useSystemBar } from '@/composables/useSystemBar'
 *
 * const { pageStyle } = useSystemBar()
 * </script>
 *
 * <template>
 *   <view class="xxx-page" :style="pageStyle">
 *     <view class="nav">…</view>
 *   </view>
 * </template>
 * ```
 */

const statusBarHeight = ref(0)
const navRightPad = ref(0)
let inited = false

function init() {
  if (inited)
    return
  inited = true
  try {
    const sys = uni.getSystemInfoSync()
    statusBarHeight.value = sys.statusBarHeight || 0
    // #ifdef MP-WEIXIN
    const rect = uni.getMenuButtonBoundingClientRect()
    if (rect && rect.width > 0 && sys.windowWidth)
      navRightPad.value = sys.windowWidth - rect.left + 8
    // #endif
  }
  catch {
    // 读取失败：不注入变量，页面回退 env(safe-area-inset-top) 与默认 padding
  }
}

export function useSystemBar() {
  init()

  /** 页面根节点内联样式：仅在取到有效值时注入对应 CSS 变量 */
  const pageStyle = computed(() => {
    const style: Record<string, string> = {}
    if (statusBarHeight.value > 0)
      style['--status-bar-h'] = `${statusBarHeight.value}px`
    if (navRightPad.value > 0)
      style['--nav-right-pad'] = `${navRightPad.value}px`
    return style
  })

  return { statusBarHeight, navRightPad, pageStyle }
}
