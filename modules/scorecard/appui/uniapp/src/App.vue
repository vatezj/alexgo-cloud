<script setup lang="ts">
onLaunch(() => {
  // 启动恢复主题：读 persist 的 themeStore，重放根节点互斥 class
  // H5 挂 documentElement；小程序端根视图 class 由 App.ku.vue 按 store 响应式绑定
  const { themeId, setTheme } = useScorecardTheme()
  setTheme(themeId.value)
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
