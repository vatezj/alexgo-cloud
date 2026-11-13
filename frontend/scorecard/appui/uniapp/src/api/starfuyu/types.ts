/**
 * starfuyu API DTO —— 字段名与后端 json 标签逐字对齐（named risk：DTO 漂移）。
 *
 * 核对来源：
 * - Fortune/LuckyColor：modules/starfuyu/model/fortune.go（tenant_id/created_at/updated_at 不入 DTO）
 * - Profile：modules/starfuyu/controller/app/app.go ProfileGet/Put（gin.H{constellation}）
 * - BaziInfo/WuxingInfo/Bazi：modules/starfuyu/model/bazi.go json tag（year_gan 等）
 * - BaziReq：modules/starfuyu/controller/app/app.go baziBirthReq（birth_date 必填格式校验在 service）
 * - Divination：modules/starfuyu/model/divination.go（可空字段标可选；member_user_id/bazi_id 等不入 DTO）
 * - DivinationReq：modules/starfuyu/controller/app/app.go DivinationDraw 绑定体（question/category 均可空）
 * - PageData：pkg/resp/page.go resp.Page → {list,total,page,size}（含 page/size，异于 scorecard PageData）
 * - 路径：modules/starfuyu/module.go RegisterRoutes appGroup（挂在 /api group 下）
 */

/** 每日星座运势内容（12 星座 × date；content 表读恒 tid=0）。 */
export interface Fortune {
  id: number
  constellation: string
  /** YYYY-MM-DD。 */
  date: string
  daily_overview: string
  /** 0-100。 */
  overall_score: number
  love_score: number
  career_score: number
  wealth_score: number
  health_score: number
  lucky_color: { chinese: string, english: string, hex: string }
  lucky_number: number
  summary: string
  love_tip: string
  career_tip: string
  wealth_tip: string
  health_tip: string
}

/** 星座偏好（GET/PUT /profile data；未设为 "" 不报错）。 */
export interface Profile {
  constellation: string
}

/** 五行信息（BaziInfo.wuxing 内嵌）。 */
export interface WuxingInfo {
  year_wuxing: string
  month_wuxing: string
  day_wuxing: string
  hour_wuxing: string
  /** 五行统计 {"金":2,"木":1,...}。 */
  total_wuxing: Record<string, number>
}

/** 八字信息（四柱 + 五行 + 节气/农历/生肖/星座；model/bazi.go json tag 逐字）。 */
export interface BaziInfo {
  year_gan: string
  year_zhi: string
  month_gan: string
  month_zhi: string
  day_gan: string
  day_zhi: string
  hour_gan: string
  hour_zhi: string
  wuxing: WuxingInfo
  solar_term: string
  lunar_date: string
  zodiac: string
  constellation: string
}

/** 我的八字行（model/bazi.go；member_user_id/created_at/updated_at 不入 DTO）。 */
export interface Bazi {
  id: number
  /** YYYY-MM-DD。 */
  birth_date: string
  /** HH:MM:SS。 */
  birth_time: string
  birth_place: string
  bazi_info: BaziInfo
}

/** 计算/更新八字入参（birth_date 必填；birth_time/birth_place 可选=不传）。 */
export interface BaziReq {
  birth_date: string
  birth_time?: string
  birth_place?: string
}

/** 求签记录（model/divination.go；可空列在 JSON 里可能为 ""，标可选便于调用方判空）。 */
export interface Divination {
  id: number
  question?: string
  /** 1-100。 */
  sign_number: number
  sign_content: string
  interpretation: string
  category?: string
  lucky_direction?: string
  /** 文本色名（非运势表 lucky_color JSON）。 */
  lucky_color?: string
  advice?: string
  /** RFC3339 时间串（Go time.Time JSON 直出）。 */
  created_at: string
}

/** 求签入参（question/category 均可空）。 */
export interface DivinationReq {
  question?: string
  category?: string
}

/** 分页信封 data（pkg/resp.Page → {list,total,page,size}）。 */
export interface PageData<T> {
  list: T[]
  total: number
  page: number
  size: number
}

/** GET /divination/sign/:number 的 data（后端 gin.H{sign_number,sign_content}）。 */
export interface SignResult {
  sign_number: number
  sign_content: string
}
