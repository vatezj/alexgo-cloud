<script setup lang="ts">
import type { ConfigProviderProps } from '@wot-ui/ui/components/wd-config-provider/types'

const { themeVars, theme } = useManualTheme()
// 打牌记账主题 class：小程序端无 document，主题根节点 class 挂在本根视图（H5 同时另挂 documentElement）
const { themeId } = useScorecardTheme()

// 在此 setup 调用 useDialog('globalDialog') 以向整棵子树 provide 共享 ref：
// GlobalDialog 与页面内容(<slot/>)是兄弟，GlobalDialog 自身 setup 里的 provide 只覆盖
// 它的子树（<wd-dialog selector="globalDialog"/>），到不了 AvatarPopup 等页面内组件——
// 后者 useDialog('globalDialog') inject 不到就自建孤儿 ref，confirm 永不弹且 promise 挂起。
// 包裹层（global-ku-root）是两者的共同祖先，在此 provide 一次两边都能 inject 到。
useDialog('globalDialog')

const buttonConfig: ConfigProviderProps['button'] = {
  size: 'large',
}

const rootClass = computed(() => `page-wraper ${theme.value} theme-t${themeId.value}`)
</script>

<template>
  <wd-config-provider :theme-vars="themeVars" :theme="theme" :button="buttonConfig" :custom-class="rootClass">
    <ku-root-view />
    <wd-notify />
    <wd-dialog />
    <wd-toast />
    <global-loading />
    <global-toast />
    <global-dialog />
    <!-- #ifdef MP-WEIXIN -->
    <privacy-popup />
    <!-- #endif -->
  </wd-config-provider>
</template>
