/**
 * 趣味小工具 · 外部服务地址配置（⚠️ 以后要改地址，优先看这里）
 *
 * ── 修改指引 ─────────────────────────────────────────────
 * 1. 本文件只负责「读取 + 去尾斜杠 + 本地兜底」，按环境取值来自 .env.*：
 *    - .env.development  → 本地开发（默认 http://127.0.0.1:3100，zhiwei 本地起法：
 *                          PORT=3100 node modules/scorecard/plugin/zhiwei/src/index.js）
 *    - .env.staging      → 测试环境网关域名
 *    - .env.production   → 生产网关域名
 * 2. 改环境地址 = 改对应 .env.* 里的 `VITE_ZHIWEI_BASE_URL`（只填域名根，不带路径），
 *    客户端会自动拼 `/api/calc`；改完需重新 build 才生效。
 * 3. 若未来新增其他工具服务，在下方 TOOL_ENDPOINTS 增加一项并同样走 .env 变量。
 *
 * ── 当前部署（2026-11-09 实测可用） ──────────────────────
 * 生产：https://alexcloud.vate.ren/api/calc
 *   → 网关把 `/api/calc` 路由到 zhiwei 服务（紫微+奇门+评分），
 *     与 Go 后端的 `/api/app/*` 同域不同路径，故 BASE 填域名根。
 */

function normalizeBase(raw: string | undefined, fallback: string): string {
  const trimmed = (raw ?? '').trim().replace(/\/+$/, '')
  return trimmed || fallback
}

/** zhiwei 排盘服务地址（域名根，末尾不带斜杠） */
export const ZHIWEI_BASE_URL = normalizeBase(
  import.meta.env.VITE_ZHIWEI_BASE_URL as string | undefined,
  'http://127.0.0.1:3100',
)

/** 各工具服务端点汇总（集中一处，方便以后增改） */
export const TOOL_ENDPOINTS = {
  /** 紫微+奇门排盘：POST {base}/api/calc */
  zhiweiCalc: `${ZHIWEI_BASE_URL}/api/calc`,
} as const
