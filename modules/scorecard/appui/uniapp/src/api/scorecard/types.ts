/**
 * scorecard API DTO —— 字段名与后端 json 标签逐字对齐（named risk：DTO 漂移）。
 *
 * 核对来源：
 * - RoomSnapshot/RoomMember/RoomLogItem/CreateRoomReq：modules/scorecard/service/snapshot.go
 * - ExpenseReq/ExpenseResult/ConfigReq：modules/scorecard/service/expense.go
 * - StatResult：modules/scorecard/service/record.go；RecordItem：modules/scorecard/model/record.go
 * - FriendItem：modules/scorecard/service/friend.go
 * - LoginResult：modules/member/service/member.go（oauth2 LoginResult 同形）；MeResult：modules/member/service/member.go ProfileResult
 * - PageData：pkg/resp/page.go（resp.Page → {list,total}）
 * - 路径：modules/scorecard/module.go RegisterRoutes（挂在 server/http.go 的 /api group 下）
 */

/** 登录/刷新令牌三元组（member auth 与 social wechat mp login 同构）。 */
export interface LoginResult {
  token: string
  refresh_token: string
  /** access token 有效期（秒，后端 int64 直出）。 */
  expires_in: number
}

/** 手机号密码登录请求（POST /api/app/member/auth/login；captcha 项按后端配置可选）。 */
export interface LoginByMobileReq {
  mobile: string
  password: string
  captcha_uuid?: string
  captcha_code?: string
}

/** 当前登录会员资料（GET/PUT /api/app/member/me 的 data）。 */
export interface MeResult {
  user_id: number
  nickname: string
  avatar: string
  /** RFC3339 时间串（Go time.Time JSON 直出）。 */
  created_at: string
}

/** 部分更新资料（PUT /api/app/member/me）：可选 = 不改（后端指针语义）。 */
export interface UpdateMeReq {
  nickname?: string
  avatar?: string
}

/** 房间成员（快照内嵌；昵称头像由后端 BriefProvider 批量补齐，失败降级空串）。 */
export interface RoomMember {
  user_id: number
  nickname: string
  avatar: string
  cur_score: number
  cum_lose_score: number
  is_creator: boolean
}

/** 房间流水条目（快照内嵌最近 20 条 / 分页接口同构）。 */
export interface RoomLogItem {
  id: number
  from_user_id: number
  /** 无收款人时为 0（状态变更类流水）。 */
  to_user_id: number
  score: number
  tea: number
  /** 1 记分 2 茶饭达标 3 进房 5 结算 6 上限提醒 8 超时关房 9 转让房主。 */
  type: number
  client_request_id: string
  created_at: string
}

/** 房间全量快照（开房/进房/快照/改配置/支出共用返回体）。 */
export interface RoomSnapshot {
  id: number
  room_code: string
  creator_id: number
  /** 0 进行中 1 已结算 2 已关闭。 */
  status: number
  /** 乐观锁版本（每次写房行 +1）。 */
  version: number
  expire_time: string
  tea_target_amount: number
  /** 茶饭百分比（0–100，可为小数）。 */
  tea_percent: number
  tea_pool_total: number
  tea_auto_enabled: boolean
  max_lose_score: number
  round_no: number
  /** >0 表示有「下把生效」的 pending 配置，值为生效的把序号。 */
  pending_apply_round_no: number
  members: RoomMember[]
  /** 最近 20 条流水（created_at 倒序）。 */
  recent_logs: RoomLogItem[]
}

/** 开房请求（POST /rooms）。room_code 空 = 服务端生成 4 位。 */
export interface CreateRoomReq {
  room_code?: string
  tea_target_amount: number
  tea_percent: number
  tea_auto_enabled: boolean
  max_lose_score: number
}

/** 进房请求（POST /rooms/join）。 */
export interface JoinRoomReq {
  room_code: string
}

/** 记分支出请求（POST /rooms/:id/expense）。client_request_id 由调用方生成（幂等键，API 层不生成）。 */
export interface ExpenseReq {
  /** 输家分 S（>0）。 */
  score: number
  /** 手动 DIY 茶饭；自动百分比模式下后端忽略此值。 */
  tea: number
  to_user_id: number
  /** 手动 DIY 扣茶来源，0 = 缺省=支出方；非 0 时须在房。 */
  tea_from_user_id: number
  client_request_id: string
}

/** 支出结果：最新快照 + 流水号 + 实收茶饭 + 首次达上限成员。 */
export interface ExpenseResult {
  snapshot: RoomSnapshot
  log_id: number
  tea_charged: number
  alarmed_user_ids: number[]
}

/**
 * 改配置请求（PUT /rooms/:id/config）：全可选字段 = 后端 Go 指针语义（不传 = 不改）。
 * JSON.stringify 会丢弃 undefined 键，序列化天然满足「不传=不改」——
 * 调用方只传要改的项即可，勿显式传 null。
 * 数值项（tea_target_amount/tea_percent/max_lose_score）写 pending「从下把生效」；
 * tea_auto_enabled 提供即改生效开关本身。
 */
export interface ConfigReq {
  tea_target_amount?: number
  tea_percent?: number
  tea_auto_enabled?: boolean
  max_lose_score?: number
}

/** 个人战绩统计聚合（GET /stat）。 */
export interface StatResult {
  total_score: number
  win: number
  lose: number
  draw: number
  /** (win)/(win+lose)，分母 0 → 0。 */
  win_rate: number
}

/** 战绩行（每成员终局一行；model.Record 的 json 标签）。 */
export interface RecordItem {
  id: number
  room_id: number
  user_id: number
  score: number
  cum_lose_score: number
  tea_pool_total: number
  tea_percent: number
  tea_target_amount: number
  max_lose_score: number
  /** 1 正常结算 2 房主退房自动结算 3 退房 4 零数据空战绩。 */
  settle_type: number
  created_at: string
  updated_at: string
}

/** 好友排名行（GET /friends）。 */
export interface FriendItem {
  user_id: number
  nickname: string
  avatar: string
  /** 好友是否参与排名（我方指向他的开关行）。 */
  rank_enabled: boolean
  total_score: number
  win_rate: number
  /** 「我拉黑了对方」的单向行存在与否。 */
  is_blocked: boolean
}

/** 分页信封 data（pkg/resp.Page → {list,total}）。 */
export interface PageData<T> {
  list: T[]
  total: number
}

/** 分页查询参数（后端 resp.PageReq：page 默认 1，size 默认 10，size 上限 100）。 */
export interface PageQuery {
  page?: number
  size?: number
}
