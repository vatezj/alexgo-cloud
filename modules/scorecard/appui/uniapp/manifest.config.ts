/*
 * @Author: weisheng
 * @Date: 2025-08-28 20:59:43
 * @LastEditTime: 2025-11-17 14:28:09
 * @LastEditors: weisheng
 * @Description:
 * @FilePath: /wot-starter/manifest.config.ts
 * 记得注释
 */
import { defineManifestConfig } from '@uni-helper/vite-plugin-uni-manifest'

export default defineManifestConfig({
  'name': 'wot-starter',
  'appid': '__UNI__1208592',
  'description': '',
  'versionName': '1.0.0',
  'versionCode': '100',
  'transformPx': false,
  /* 5+App特有相关 */
  'app-plus': {
    usingComponents: true,
    nvueStyleCompiler: 'uni-app',
    compilerVersion: 3,
    splashscreen: {
      alwaysShowBeforeRender: true,
      waiting: true,
      autoclose: true,
      delay: 0,
    },
    /* 模块配置 */
    modules: {},
    /* 应用发布信息 */
    distribute: {
      /* android打包配置 */
      android: {
        permissions: [
          '<uses-permission android:name="android.permission.CHANGE_NETWORK_STATE"/>',
          '<uses-permission android:name="android.permission.MOUNT_UNMOUNT_FILESYSTEMS"/>',
          '<uses-permission android:name="android.permission.VIBRATE"/>',
          '<uses-permission android:name="android.permission.READ_LOGS"/>',
          '<uses-permission android:name="android.permission.ACCESS_WIFI_STATE"/>',
          '<uses-feature android:name="android.hardware.camera.autofocus"/>',
          '<uses-permission android:name="android.permission.ACCESS_NETWORK_STATE"/>',
          '<uses-permission android:name="android.permission.CAMERA"/>',
          '<uses-permission android:name="android.permission.GET_ACCOUNTS"/>',
          '<uses-permission android:name="android.permission.READ_PHONE_STATE"/>',
          '<uses-permission android:name="android.permission.CHANGE_WIFI_STATE"/>',
          '<uses-permission android:name="android.permission.WAKE_LOCK"/>',
          '<uses-permission android:name="android.permission.FLASHLIGHT"/>',
          '<uses-feature android:name="android.hardware.camera"/>',
          '<uses-permission android:name="android.permission.WRITE_SETTINGS"/>',
        ],
      },
      /* ios打包配置 */
      ios: {},
      /* SDK配置 */
      sdkConfigs: {},
    },
  },
  /* 快应用特有相关 */
  'quickapp': {},
  /* 小程序特有相关 */
  'mp-weixin': {
    optimization: {
      subPackages: true,
    },
    // 小程序 appid：运营侧在小程序后台/发行配置填写，仓库不落凭据
    appid: '',
    // 微信同声传译插件（WechatSI，TTS 语音播报用）：代码先声明插件结构；
    // provider/version 为微信官方插件固定标识（非本应用凭据），运营侧需在小程序后台
    // 「设置-第三方设置-插件管理」添加同声传译插件后真机方可生效（未添加时 useVoice 降级静默）
    plugins: {
      WechatSI: {
        version: '0.3.6',
        provider: 'wx069ba97219f66d99',
      },
    },
    setting: {
      urlCheck: false,
    },
    usingComponents: true,
    darkmode: true,
    themeLocation: 'theme.json',
  },
  'app-harmony': {},
  'mp-harmony': {},
  'mp-alipay': {
    usingComponents: true,
    compileOptions: {
      globalObjectMode: 'enable',
      treeShaking: true,
    },
  },
  'mp-baidu': {
    usingComponents: true,
  },
  'mp-toutiao': {
    usingComponents: true,
  },
  'h5': {
    darkmode: true,
    themeLocation: 'theme.json',
  },
  'uniStatistics': {
    enable: false,
  },
  'vueVersion': '3',
})
