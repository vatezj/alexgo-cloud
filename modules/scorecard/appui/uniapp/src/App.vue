<script setup lang="ts">
import { parseScene, restorePendingScene, setPendingScene } from '@/composables/scorecard/useSceneRoom'

onLaunch(() => {
  // 启动恢复主题：读 persist 的 themeStore，重放根节点互斥 class
  // H5 挂 documentElement；小程序端根视图 class 由 App.ku.vue 按 store 响应式绑定
  const { themeId, setTheme } = useScorecardTheme()
  setTheme(themeId.value)
  // 恢复上次未消费的 scene 暂存（storage → 内存；进程被杀冷启可续）
  restorePendingScene()
})

// scene 进房/绑好友：decode 后形如 qr=7AVW / qr=7AVW&inv=100 / inv=100
// → parseScene 提取房号+邀请人 → setPendingScene 写内存+storage（消费方 consumeScene 只读一次）
onShow((options) => {
  // 微信 App 级 options.scene 是数字场景值，自定义码值落 query.scene（部分端在 options.scene 直给字符串，兜底兼容）
  const queryScene = options?.query?.scene
  let raw = typeof queryScene === 'string' ? queryScene : typeof options?.scene === 'string' ? options.scene : ''
  // 朋友圈 onShareTimeline 的 query 直给 page query（inv=<uid>），不经 scene 键——兜底提取
  if (!raw && typeof options?.query?.inv === 'string')
    raw = options.query.inv
  if (!raw)
    return
  let decoded = raw
  try {
    decoded = decodeURIComponent(raw)
  }
  catch {
    // 非法百分号编码：按明文继续尝试解析
  }
  const p = parseScene(decoded)
  if (p)
    setPendingScene(p)
})
</script>

<style lang="scss">
@use '@wot-ui/ui/styles/theme/index.scss' as *;
@use './theme/scorecard.scss';

.page-wraper {
  min-height: calc(100vh - var(--window-top));
  box-sizing: border-box;
  background: var(--ink-900);
}
</style>
