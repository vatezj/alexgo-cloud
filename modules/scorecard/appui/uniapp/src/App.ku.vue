<script setup lang="ts">
import type { ConfigProviderProps } from '@wot-ui/ui/components/wd-config-provider/types'

const { themeVars, theme } = useManualTheme()
// 打牌记账主题 class：小程序端无 document，主题根节点 class 挂在本根视图（H5 同时另挂 documentElement）
const { themeId } = useScorecardTheme()

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
