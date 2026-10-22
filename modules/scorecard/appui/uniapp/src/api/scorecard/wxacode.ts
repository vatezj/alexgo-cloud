/**
 * 房间小程序码（GET /api/app/scorecard/rooms/:id/wxacode）——
 * 非 JSON 信封：200 image/png 字节直出，responseType: 'arraybuffer'
 * （alova uniapp adapter 将 method.config 透传给 uni.request）。
 */
import { alovaInstance } from '../core/instance'

/** 取房间小程序码 PNG 字节（需登录态；非成员 30102 走 handlers 错误链路）。 */
export function getRoomWxacode(roomId: number): Promise<ArrayBuffer> {
  // handleAlovaResponse 对非 JSON 响应原样返回 data（ArrayBuffer 直通，不做信封解包）；
  // Method 继承 Promise<ArrayBuffer>，直接返回即可。
  return alovaInstance.Get<ArrayBuffer>(`/api/app/scorecard/rooms/${roomId}/wxacode`, {
    responseType: 'arraybuffer',
  })
}
