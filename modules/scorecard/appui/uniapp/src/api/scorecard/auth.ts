/**
 * member 鉴权 + 个人资料接口（modules/member/controller/app）。
 * 路径逐字对齐 modules/member/module.go（挂在 /api group 下）。
 */
import type { LoginByMobileReq, LoginResult, MeResult, UpdateMeReq } from './types'
import { ApiError } from '../core/handlers'
import { apiBaseURL } from '../core/instance'
import { scoreGet, scoreSend, scoreUpload } from './http'

/** 手机号密码登录（公开）。POST /api/app/member/auth/login → LoginResult。 */
export function loginByMobile(data: LoginByMobileReq): Promise<LoginResult> {
  return scoreSend<LoginResult>('POST', '/api/app/member/auth/login', data)
}

/** 微信小程序 code 登录（公开，与 member auth 同构）。POST /api/app/social/wechat/mp/login。 */
export function wxLogin(code: string): Promise<LoginResult> {
  return scoreSend<LoginResult>('POST', '/api/app/social/wechat/mp/login', { code })
}

/** 当前登录会员资料。GET /api/app/member/me → MeResult。 */
export function getMe(): Promise<MeResult> {
  return scoreGet<MeResult>('/api/app/member/me')
}

/**
 * 部分更新资料。PUT /api/app/member/me → MeResult。
 * 指针语义：只传要改的键（不传 = 不改）；nickname 空串会被后端 10001 拒，avatar 空串 = 清空。
 */
export function updateMe(data: UpdateMeReq): Promise<MeResult> {
  return scoreSend<MeResult>('PUT', '/api/app/member/me', data)
}

/**
 * 上传头像并返回可直接入 <image> 的绝对地址。
 * POST /api/app/infra/file/upload（multipart 字段 file）；后端仅收图片扩展名、
 * 强制 Public=true（public_url 走 /api/public/infra/file/<key>，相对路径需拼 baseURL）。
 */
export function uploadAvatar(filePath: string): Promise<string> {
  return scoreUpload<{ public_url: string }>('/api/app/infra/file/upload', filePath).then((res) => {
    const url = res?.public_url
    if (!url)
      throw new ApiError('上传失败', -1)
    if (url.startsWith('http'))
      return url
    return `${apiBaseURL.replace(/\/$/, '')}${url}`
  })
}
