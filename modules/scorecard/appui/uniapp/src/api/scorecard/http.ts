/**
 * scorecard API 手写 method 工厂：统一走 alovaInstance，负责信封解包。
 *
 * 信封约定（pkg/resp）：`{code,msg,data}`，code=0 成功返回 data；
 * code!=0 throw ApiError{code,msg}（data 挂 err.data）。
 * HTTP 层错误（网络/超时/4xx/4xx 通用）与 401 清态跳登录由 core/handlers 负责——
 * 注意 alova 语义：onSuccess 内抛出的错误直接透传给调用方、不会再进 onError，
 * 所以业务信封错误不会触发 handlers 的全局 toast，由页面自行提示。
 */
import { ApiError } from '../core/handlers'
import { alovaInstance } from '../core/instance'

/** 后端统一响应信封。 */
export interface ApiEnvelope<T = unknown> {
  code: number
  msg?: string
  data?: T
}

/** 信封解包：code=0 返回 data；非 0 throw ApiError（五位业务码原样透出）。 */
function unwrap<T>(json: ApiEnvelope<T>): T {
  if (!json || typeof json.code !== 'number' || json.code !== 0) {
    throw new ApiError(json?.msg || '请求失败', json?.code ?? -1, json?.data)
  }
  return json.data as T
}

/** GET + 信封解包。params 为 query 对象（不传则不带 query）。 */
export function scoreGet<T>(url: string, params?: Record<string, any>): Promise<T> {
  return alovaInstance.Get<ApiEnvelope<T>>(url, { params }).then(unwrap)
}

export type ScoreSendType = 'POST' | 'PUT' | 'DELETE'

/**
 * POST/PUT/DELETE + 信封解包。
 * data 经 JSON 序列化发送：对象里值为 undefined 的键会被丢弃——
 * 指针语义接口（ConfigReq/updateMe）靠「不传键 = 不改」实现，勿显式传 null。
 */
export function scoreSend<T>(type: ScoreSendType, url: string, data?: any): Promise<T> {
  let method
  if (type === 'POST') {
    method = alovaInstance.Post<ApiEnvelope<T>>(url, data)
  }
  else if (type === 'PUT') {
    method = alovaInstance.Put<ApiEnvelope<T>>(url, data)
  }
  else {
    method = alovaInstance.Delete<ApiEnvelope<T>>(url, data)
  }
  return method.then(unwrap)
}
