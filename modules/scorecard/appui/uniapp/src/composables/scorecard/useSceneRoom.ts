/**
 * 扫码进房 scene 暂存（模块级单例）
 *
 * - App onShow 解析小程序码 scene（decodeURIComponent 后形如 `qr=7AVW`）→ 提取房号写入 pendingScene
 * - 登录成功后的 afterLogin 调 consumeScene() 取房号并 joinRoom——**只消费一次**（读后即清，防 onShow 重复触发）
 * - 完整进房接线在 Task 6（首页已登录）/ Task 12（闭环验收），本文件只做暂存/消费骨架
 */
import type { Ref } from 'vue'
import { ref } from 'vue'

/** 待消费的房号（scene 提取结果；null = 无暂存）。模块级共享：App 与各页面拿到同一实例。 */
const pendingScene: Ref<string | null> = ref(null)

export function useSceneRoom() {
  /** 取出并清空暂存房号（只消费一次：二次调用返回 null）。 */
  function consumeScene(): string | null {
    const roomCode = pendingScene.value
    pendingScene.value = null
    return roomCode
  }

  return { pendingScene, consumeScene }
}
