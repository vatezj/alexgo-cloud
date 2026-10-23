/*
 * @Author: weisheng
 * @Date: 2025-04-17 15:58:11
 * @LastEditTime: 2025-06-15 21:47:22
 * @LastEditors: weisheng
 * @Description: Alova response and error handlers
 * @FilePath: /wot-starter/src/api/core/handlers.ts
 */
import type { Method } from 'alova'
import router from '@/router'
import { useScorecardAuthStore } from '@/store/scorecard/auth'

// Custom error class for API errors
export class ApiError extends Error {
  code: number
  data?: any

  constructor(message: string, code: number, data?: any) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.data = data
  }
}

// Define a type for the expected API response structure
interface ApiResponse {
  code: number
  msg?: string
  data?: any
  success?: boolean
  total?: number
  more?: boolean
}

// 401/403 统一善后：清登录态 + 提示 + 延时跳登录页。
// store 经静态 import 引入无环：authStore 只静态依赖 types，api 依赖是动态 import。
function handleUnauthorized(globalToast: ReturnType<typeof useGlobalToast>) {
  useScorecardAuthStore().logout()
  globalToast.error({ msg: '登录已过期，请重新登录！', duration: 500 })
  const timer = setTimeout(() => {
    clearTimeout(timer)
    // 路由表来自 virtual:uni-pages（页面无 name 字段），按 path 命中登录页
    router.replaceAll({ path: '/pages/login/index' })
  }, 500)
}

// Uint8Array → UTF-8 字符串（百分号编码 + decodeURIComponent；不依赖 TextDecoder，MP 也可用）
function utf8Decode(bytes: Uint8Array): string {
  let pct = ''
  for (let i = 0; i < bytes.length; i++)
    pct += `%${bytes[i].toString(16).padStart(2, '0')}`
  return decodeURIComponent(pct)
}

// 错误体信封还原：responseType:'arraybuffer' 请求（如 scorecard wxacode）的错误体同样是
// ArrayBuffer（uni 不做 JSON 解析），先按 UTF-8 解码再 JSON.parse，五位业务码才不丢
// （30102/50001 否则退化成通用 400 文案）。普通请求 data 已是解析好的对象则原样返回；
// 解码/解析失败返回 undefined，维持原通用 status 分支。
function toErrEnvelope(data: unknown): ApiResponse | undefined {
  try {
    let text: string
    if (data instanceof ArrayBuffer) {
      text = utf8Decode(new Uint8Array(data))
    }
    else if (typeof data === 'string') {
      text = data
    }
    else if (typeof data === 'object' && data !== null) {
      return data as ApiResponse
    }
    else {
      return undefined
    }
    const parsed = JSON.parse(text) as unknown
    return typeof parsed === 'object' && parsed !== null ? parsed as ApiResponse : undefined
  }
  catch {
    return undefined
  }
}

// Handle successful responses
export async function handleAlovaResponse(
  response: UniApp.RequestSuccessCallbackResult | UniApp.UploadFileSuccessCallbackResult | UniApp.DownloadSuccessData,
) {
  const globalToast = useGlobalToast()
  // Extract status code and data from UniApp response
  const { statusCode, data } = response as UniNamespace.RequestSuccessCallbackResult

  // 处理401/403错误（如果不是在handleAlovaResponse中处理的）
  if ((statusCode === 401 || statusCode === 403)) {
    // 如果是未授权错误，清除登录态并跳转到登录页
    handleUnauthorized(globalToast)

    throw new ApiError('登录已过期，请重新登录！', statusCode, data)
  }

  // Handle HTTP error status codes
  if (statusCode >= 400) {
    // 后端 pkg/resp.statusFor 把业务错误码映射成 HTTP 状态（3xxxx→400 等），
    // body 仍是 {code,msg,data} 信封——优先透出五位业务码，便于页面按码分支（如 30112 版本冲突）。
    // arraybuffer 请求错误体不被 uni 解析，toErrEnvelope 先还原成信封再取码。
    const errBody = toErrEnvelope(data)
    if (errBody && typeof errBody.code === 'number' && errBody.code !== 0) {
      throw new ApiError(errBody.msg || `Request failed with status: ${statusCode}`, errBody.code, errBody.data)
    }
    globalToast.error(`Request failed with status: ${statusCode}`)
    throw new ApiError(`Request failed with status: ${statusCode}`, statusCode, data)
  }

  // The data is already parsed by UniApp adapter
  const json = data as ApiResponse
  // Log response in development
  if (import.meta.env.MODE === 'development') {
    console.log('[Alova Response]', json)
  }

  // Return data for successful responses
  return json
}

// Handle request errors
export function handleAlovaError(error: any, method: Method) {
  const globalToast = useGlobalToast()
  // Log error in development
  if (import.meta.env.MODE === 'development') {
    console.error('[Alova Error]', error, method)
  }

  // 处理401/403错误（如果不是在handleAlovaResponse中处理的）
  if (error instanceof ApiError && (error.code === 401 || error.code === 403)) {
    // 如果是未授权错误，清除登录态并跳转到登录页
    handleUnauthorized(globalToast)
    throw new ApiError('登录已过期，请重新登录！', error.code, error.data)
  }

  // Handle different types of errors
  if (error.name === 'NetworkError') {
    globalToast.error('网络错误，请检查您的网络连接')
  }
  else if (error.name === 'TimeoutError') {
    globalToast.error('请求超时，请重试')
  }
  else if (error instanceof ApiError) {
    globalToast.error(error.message || '请求失败')
  }
  else {
    globalToast.error('发生意外错误')
  }

  throw error
}
