/**
 * 资料完整度判定（开房硬门 / 进局软提醒 / 牌局内编辑共用，纯函数）
 *
 * 规则（与后端无关，纯前端判定）：
 * - 占位昵称 = 登录默认值：微信登录「微信用户」+ openid 尾 4 hex；手机注册「用户」+ 手机尾 4
 * - 资料完整 = 有头像 且 昵称非空、非登录占位（不完整 → gate 弹「完善资料」/ 进局弹软提醒）
 */

/** 登录占位昵称（后端缺省）：微信登录「微信用户」+ openid 尾 4 hex；手机注册「用户」+ 手机尾 4。 */
export function isPlaceholderNickname(name: string): boolean {
  return /^微信用户[0-9a-f]{4}$/.test(name) || /^用户\d{4}$/.test(name)
}

/** 资料判定输入（MeResult 的可空子集，页面/组件均可传）。 */
export interface ProfileLike {
  nickname: string
  avatar: string
}

/** 资料完整 = 有头像 且 昵称非空/非登录占位（null 资料视为不完整）。 */
export function isProfileComplete(user: ProfileLike | null): boolean {
  if (!user)
    return false
  const nick = (user.nickname || '').trim()
  if (!nick || isPlaceholderNickname(nick))
    return false
  return Boolean((user.avatar || '').trim())
}
