<script setup lang="ts">
import { useSceneRoom } from '@/composables/scorecard/useSceneRoom'

onLaunch(() => {
  // 启动恢复主题：读 persist 的 themeStore，重放根节点互斥 class
  // H5 挂 documentElement；小程序端根视图 class 由 App.ku.vue 按 store 响应式绑定
  const { themeId, setTheme } = useScorecardTheme()
  setTheme(themeId.value)
})

// 小程序码 scene 进房：decodeURIComponent 后形如 qr=7AVW → 提取房号暂存
// （登录后 afterLogin consumeScene 消费一次；已登录路径 Task 6 接，闭环验收 Task 12）
const { pendingScene } = useSceneRoom()
onShow((options) => {
  // 微信 App 级 options.scene 是数字场景值，自定义码值落 query.scene（部分端在 options.scene 直给字符串，兜底兼容）
  const queryScene = options?.query?.scene
  const raw = typeof queryScene === 'string' ? queryScene : typeof options?.scene === 'string' ? options.scene : ''
  if (!raw)
    return
  let decoded = raw
  try {
    decoded = decodeURIComponent(raw)
  }
  catch {
    // 非法百分号编码：按明文继续尝试解析
  }
  if (!decoded.startsWith('qr='))
    return
  const roomCode = decoded.slice(3).trim()
  if (roomCode)
    pendingScene.value = roomCode
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
