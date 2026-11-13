/*
 * @Description: Mock 适配器 —— starfuyu 验收期启用（luckyu-into-scorecard Task 6）。
 *               仅注册 modules/starfuyu（pathname 全部以 /api/app/starfuyu 开头）；
 *               未命中的请求 fallback 真请求（uniappRequestAdapter），记账局叙直连本地后端
 *               （VITE_API_BASE_URL）不受影响。pet/user/store/common 旧模块保持不注册。
 *               验收结束关闭：enable 改回 false（或摘除 starfuyu 注册）。
 * @FilePath: /wot-starter/src/api/mock/mockAdapter.ts
 */
import { uniappMockResponse, uniappRequestAdapter } from '@alova/adapter-uniapp'
import { createAlovaMockAdapter } from '@alova/mock'
import starfuyuMocks from './modules/starfuyu'

// 只注册 starfuyu 验收用例（pet/user/store/common 不 import——真接口必须走真请求）
const allMocks: Parameters<typeof createAlovaMockAdapter>[0] = [...starfuyuMocks]

const mockAdapter = createAlovaMockAdapter(allMocks, {
  // 使用 uniapp 请求适配器处理未命中 mock 的请求
  httpAdapter: uniappRequestAdapter,

  // 使用 uniapp mock 响应适配器
  onMockResponse: uniappMockResponse,

  // starfuyu 验收期启用；未命中 fallback 真请求
  enable: true,

  // 路径匹配模式 - 使用完整路径匹配
  matchMode: 'pathname',
})

export default mockAdapter
