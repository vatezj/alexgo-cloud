/**
 * member 鉴权 + 个人资料接口（modules/member/controller/app）。
 * 路径逐字对齐 modules/member/module.go（挂在 /api group 下）。
 */
import type { LoginByMobileReq, LoginResult, MeResult, UpdateMeReq } from './types'
import { scoreGet, scoreSend } from './http'

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
