/**
 * 趣味方位参考 · 模块开关（fun_compass_enable）
 *
 * - 当前后端无配置接口（proposal 非目标）：本地默认 true，入口常显；
 * - 后端接口落地时**只改本文件**：请求 → `enabled.value = res.fun_compass_enable !== false`，
 *   请求失败保持 true（可用性优先；风控隐藏需后端显式下发 false），页面零改动；
 * - 消费方：【我的】页趣味分组 `v-if="enabled"`（spec 入口门控）。
 */
import { ref } from 'vue'

const enabled = ref(true)

export function useFunCompassConfig() {
  return { enabled }
}
