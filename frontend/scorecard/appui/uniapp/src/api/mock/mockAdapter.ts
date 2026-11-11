/*
 * @Description: Mock 适配器 —— 已关闭（恒空）。打牌记账直连本地后端（VITE_API_BASE_URL），
 *               不再注册任何 mock 用例；instance.ts 不再传入 mockRequest。
 *               如需临时 mock：在 modules/ 下补用例，并在 instance.ts 重新传入本适配器。
 * @FilePath: /wot-starter/src/api/mock/mockAdapter.ts
 */
import { uniappMockResponse, uniappRequestAdapter } from '@alova/adapter-uniapp'
import { createAlovaMockAdapter } from '@alova/mock'

// 恒空：不注册任何 mock 模块
const allMocks: Parameters<typeof createAlovaMockAdapter>[0] = []

const mockAdapter = createAlovaMockAdapter(allMocks, {
  // 使用 uniapp 请求适配器处理未命中 mock 的请求
  httpAdapter: uniappRequestAdapter,

  // 使用 uniapp mock 响应适配器
  onMockResponse: uniappMockResponse,

  // 关闭 mock：全部走真实请求
  enable: false,

  // 路径匹配模式 - 使用完整路径匹配
  matchMode: 'pathname',
})

export default mockAdapter
