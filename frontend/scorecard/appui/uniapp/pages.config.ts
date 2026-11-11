/*
 * @Author: weisheng
 * @Date: 2025-06-23 22:23:05
 * @LastEditTime: 2025-06-27 13:04:54
 * @LastEditors: weisheng
 * @Description:
 * @FilePath: /wot-starter/pages.config.ts
 * 记得注释
 */
import { defineUniPages } from '@uni-helper/vite-plugin-uni-pages'

export default defineUniPages({
  pages: [],
  globalStyle: {
    // 全局自绘导航（打牌记账：牌桌即界面，导航栏由各页自绘）
    navigationStyle: 'custom',
    navigationBarTitleText: '打牌记账',

    // 页面背景配置（原生底色 = T1 墨金 BASELINE ink-900；页面内容背景走 var(--ink-900)）
    backgroundColor: '#14120F',

    // 下拉刷新配置
    enablePullDownRefresh: false,
    onReachBottomDistance: 50,

    // 动画配置
    animationType: 'pop-in',
    animationDuration: 300,
  },
})
