/**
 * 扫码/转发 scene 暂存（模块级单例）+ 静默绑好友
 *
 * - App onShow 解析 scene（decodeURIComponent 后形如 `qr=7AVW` / `qr=7AVW,inv=100` / `inv=100`；
 *   历史分享卡片可能是 `&` 分隔，解析双兼容）
 *   → parseScene 提取房号+邀请人 → setPendingScene 写内存 + storage（进程被杀冷启可续）
 * - 消费方（首页 consumePendingScene / 登录 afterLogin）调 consumeScene 只消费一次（读后即清）
 * - 有房号 join 进房；inviterId 经 bindInviterSilently 静默 addFriend（错误全吞不阻塞）
 * - 旧协议 `qr=<房号>`（无 inv）、纯 `inv=<uid>`（首页转发只绑好友）、历史 `&` 分隔场景均兼容
 */
import type { Ref } from 'vue'
import { ref } from 'vue'
import { addFriend } from '@/api/scorecard/friend'

/** 待消费的 scene 提取结果：roomCode 与 inviterId 至少一项非 null。 */
export interface PendingScene {
  roomCode: string | null
  inviterId: number | null
}

/** storage 键（落盘防进程被杀丢暂存）。 */
const STORAGE_KEY = 'scorecard:pendingScene'

/** 模块级共享：App 与各页面拿到同一实例。 */
const pendingScene: Ref<PendingScene | null> = ref(null)

/**
 * 纯函数：按 `,` 或 `&` 分段解析 scene 字符串。
 * 兼容协议：`qr=<房号>` / `qr=<房号>,inv=<uid>` / `inv=<uid>`；inv 非正整数置 null；两段皆空返回 null。
 * 小程序码 scene 由后端拼（微信拒含 `&` 的 scene，40129，故码一律 `,` 分隔）；历史分享卡片仍是 `&`，双兼容。
 */
export function parseScene(raw: string): PendingScene | null {
  let roomCode: string | null = null
  let inviterId: number | null = null
  for (const part of raw.split(/[&,]/)) {
    const eq = part.indexOf('=')
    if (eq <= 0)
      continue
    const key = part.slice(0, eq)
    const val = part.slice(eq + 1).trim()
    if (!val)
      continue
    if (key === 'qr') {
      roomCode = val
    }
    else if (key === 'inv') {
      const n = Number(val)
      if (Number.isInteger(n) && n > 0)
        inviterId = n
    }
  }
  if (!roomCode && inviterId == null)
    return null
  return { roomCode, inviterId }
}

/** 写内存 ref + 落盘 storage。空/非法值由调用方先经 parseScene 过滤（非 null 才写入）。 */
export function setPendingScene(p: PendingScene) {
  pendingScene.value = p
  try {
    uni.setStorageSync(STORAGE_KEY, p)
  }
  catch {
    // storage 失败仅内存暂存（本次进程内仍可消费）
  }
}

/** 从 storage 恢复到内存 ref（模块加载 / App onLaunch 调用；已有内存值不覆盖）。 */
export function restorePendingScene() {
  if (pendingScene.value)
    return
  try {
    const raw = uni.getStorageSync(STORAGE_KEY) as Partial<PendingScene> | string | null
    if (raw && typeof raw === 'object') {
      const roomCode = typeof raw.roomCode === 'string' && raw.roomCode ? raw.roomCode : null
      const inviterId = typeof raw.inviterId === 'number' && raw.inviterId > 0 ? raw.inviterId : null
      if (roomCode || inviterId != null)
        pendingScene.value = { roomCode, inviterId }
    }
  }
  catch {
    // storage 读失败：无暂存
  }
}

/** 取出并清空暂存（只消费一次：二次调用返回 null）；同步清 storage。 */
export function consumeScene(): PendingScene | null {
  const p = pendingScene.value
  pendingScene.value = null
  try {
    uni.removeStorageSync(STORAGE_KEY)
  }
  catch {
    // 清理失败不影响消费语义
  }
  return p
}

/**
 * 静默绑定邀请人为好友：相等/空值直接 return；否则 addFriend。
 * 错误全 catch 静默（30109 已好友 / 30108 拉黑 / 10001 自加 / 网络）——不 toast、不上抛，不阻塞进房/登录。
 */
export async function bindInviterSilently(inviterId: number | null, myId: number | null): Promise<void> {
  if (!inviterId || !myId || inviterId === myId)
    return
  try {
    await addFriend(inviterId)
  }
  catch {
    // 静默吞：绑定结果不影响后续进房/登录链路
  }
}

export function useSceneRoom() {
  restorePendingScene()
  return { pendingScene, parseScene, setPendingScene, consumeScene, restorePendingScene, bindInviterSilently }
}

// 模块加载时恢复上次未消费暂存（冷启 App onLaunch 之前就可读到）
restorePendingScene()
