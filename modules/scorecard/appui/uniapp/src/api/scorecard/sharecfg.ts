/**
 * 三页分享卡配置（modules/scorecard/service/sharecfg.go → GET /api/public/scorecard/share-config）。
 * 免鉴权公开端点：无 Bearer 也可读（scoreGet 走 alova，无 token 时不带 Authorization）。
 * 值由后台「系统参数」页 scorecard.share.* 维护；key 缺失/坏 JSON 服务端回退内置默认。
 */
import { scoreGet } from './http'

/** 单页分享卡：title=转发标题，imageUrl=卡片图（空串=微信回退页截图）。 */
export interface ShareCard {
  title: string
  imageUrl: string
}

/** 三页分享卡，键=页面后缀（与路由 key 后缀一致）。 */
export interface ShareCards {
  index: ShareCard
  simulator: ShareCard
  salary: ShareCard
}

/** 拉取三页分享卡配置（信封 code!=0 时 throw ApiError，调用方静默降级）。 */
export function getShareConfig(): Promise<ShareCards> {
  return scoreGet<ShareCards>('/api/public/scorecard/share-config')
}
