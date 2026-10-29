<script setup lang="ts">
import { parseScene, restorePendingScene, setPendingScene } from '@/composables/scorecard/useSceneRoom'

onLaunch(() => {
  // 先判断API是否可用（低版本微信兼容）
  if (!uni.canIUse('getUpdateManager')) {
    return
  }
  const updateManager = uni.getUpdateManager()

  // 检查是否有新版本
  updateManager.onCheckForUpdate((res) => {
    console.log('是否有更新：', res.hasUpdate)
  })

  // 新版本下载完成
  updateManager.onUpdateReady(() => {
    uni.showModal({
      title: '更新提示',
      content: '新版本已就绪，是否重启更新？',
      success: (res) => {
        if (res.confirm) {
          // 应用新版本 + 自动重启
          updateManager.applyUpdate()
        }
      },
    })
  })

  // 更新失败
  updateManager.onUpdateFailed(() => {
    uni.showToast({ title: '更新失败，请稍后重试', icon: 'none' })
  })

  // 启动恢复主题：读 persist 的 themeStore，重放根节点互斥 class
  // H5 挂 documentElement；小程序端根视图 class 由 App.ku.vue 按 store 响应式绑定
  const { themeId, setTheme } = useScorecardTheme()
  setTheme(themeId.value)
  // 恢复上次未消费的 scene 暂存（storage → 内存；进程被杀冷启可续）
  restorePendingScene()
  // #ifdef MP-WEIXIN
  // 冷启动静默登录（fire-and-forget）：不阻塞启动；首页 onShow 会 await ensureSilentLogin
  // 复用同一在途 promise（inFlight 去重）。失败静默降级（failed 封顶，不循环）。
  if (!useScorecardAuthStore().token)
    void ensureSilentLogin()
  // #endif
})

// scene 进房/绑好友：decode 后形如 qr=7AVW / qr=7AVW,inv=100 / inv=100（& 为历史卡片格式，parseScene 双兼容）
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
  box-sizing: border-box;
  min-height: calc(100vh - var(--window-top));
  background: var(--ink-900);
}
</style>
