/*
 * @Description: starfuyu 验收 mock 用例（luckyu-into-scorecard Task 6）。
 * match 规则（matchMode 'pathname'）：key 的 pathname 须以 /api/app/starfuyu 开头，
 *   形如 [GET]/api/app/starfuyu/profile，与请求 pathname 逐段比对（'/' split，段数+字面量全等）。
 *   未命中 /api/app/starfuyu 前缀的 pathname（scorecard 记账局叙等）不进本表 → fallback 真请求。
 * 覆盖（brief 钉死 7 例 + UI 闭环补 2 例）：
 *   GET  /api/app/starfuyu/profile            → { code:0, data:{ constellation } }（初始白羊座，PUT 覆盖）
 *   GET  /api/app/starfuyu/fortune/my/today   → { code:0, data: <Fortune> }（按当前 profile 星座出样例）
 *   GET  /api/app/starfuyu/fortune/today      → { code:0, data: [12 条 Fortune] }
 *   POST /api/app/starfuyu/bazi/calculate     → { code:0, data: <Bazi 样例> }
 *   POST /api/app/starfuyu/divination/draw    → { code:0, data: <Divination 样例> }（回显 question/category）
 *   GET  /api/app/starfuyu/divination/history → { code:0, data:{ list:[1 条], total:1, page:1, size:10 } }
 *   GET  /api/app/starfuyu/bazi/my            → { code:0, data: <Bazi 样例> }——二选一取成功样例（进页即有数据；
 *       30504 空态分支留真联调。POST /bazi/calculate 用例照注册，页面 hasSavedBazi=false 时才命中）
 *   PUT  /api/app/starfuyu/profile            → 回显 constellation（zodiac store.select 闭环，brief 外补）
 *   PUT  /api/app/starfuyu/bazi/my            → <Bazi 样例>（已存八字再排盘，brief 外补）
 * 信封对齐 pkg/resp.OK：{code:0, msg:'ok', data}。无测试运行器 → 静态断言见 task-6-report.md。
 * @FilePath: /wot-starter/src/api/mock/modules/starfuyu.ts
 */
import type { Bazi, Divination, DivinationReq, Fortune, PageData, Profile } from '@/api/starfuyu/types'
import { defineMock } from '@alova/mock'

/**
 * 验收期 profile 星座：GET /profile 读、PUT /profile 写、fortune/my/today 随之出样例——
 * 模块级状态仅存于本次运行（刷新回白羊座）。不引 store/starfuyu/zodiac（其值 import setProfile
 * → api → instance → 本文件，会成环），12 星座名在此硬编码与其对齐。
 */
const ZODIAC_NAMES = [
  '白羊座',
  '金牛座',
  '双子座',
  '巨蟹座',
  '狮子座',
  '处女座',
  '天秤座',
  '天蝎座',
  '射手座',
  '摩羯座',
  '水瓶座',
  '双鱼座',
] as const

let profileConstellation = '白羊座'

/** 成功信封（对齐 pkg/resp.OK）。 */
function ok<T>(data: T): { code: number, msg: string, data: T } {
  return { code: 0, msg: 'ok', data }
}

/** 本地时区 YYYY-MM-DD（与 pages/starfuyu/utils/mockFortune 同 toISOString 口径）。 */
function todayStr(): string {
  return new Date().toISOString().split('T')[0]
}

/** Fortune 样例工厂：字段对齐 Task 1 types.Fortune；分数按 id 确定性错开，12 条互异。 */
function makeFortune(constellation: string, id: number): Fortune {
  const base = 68 + (id % 6) * 4 // 68~88，同 id 稳定
  return {
    id,
    constellation,
    date: todayStr(),
    daily_overview: '今日运势平稳，宜规划未来。',
    overall_score: base,
    love_score: Math.min(99, base + 5),
    career_score: Math.max(50, base - 3),
    wealth_score: Math.min(99, base + 2),
    health_score: Math.max(50, base - 6),
    lucky_color: { chinese: '红色', english: 'Red', hex: '#FF4500' },
    lucky_number: (id % 9) + 1,
    summary: '把握节奏，稳步前行的一天。',
    love_tip: '与亲友多沟通，感情升温。',
    career_tip: '处理细节时多一分耐心。',
    wealth_tip: '理性消费，避免冲动购物。',
    health_tip: '注意作息，适当运动。',
  }
}

/** 当前 profile 星座的 my/today 样例（名字不在表内兜底白羊座 id=1）。 */
function makeMyFortune(): Fortune {
  const idx = (ZODIAC_NAMES as readonly string[]).indexOf(profileConstellation)
  return makeFortune(profileConstellation, idx >= 0 ? idx + 1 : 1)
}

/** Bazi 样例：1990-05-15 12:30 广州 → 庚午年/辛巳月/甲子日/庚午时（五鼠遁甲日午时=庚午），生肖马、金牛座自洽。 */
const BAZI_SAMPLE: Bazi = {
  id: 1,
  birth_date: '1990-05-15',
  birth_time: '12:30:00',
  birth_place: '广东省广州市',
  bazi_info: {
    year_gan: '庚',
    year_zhi: '午',
    month_gan: '辛',
    month_zhi: '巳',
    day_gan: '甲',
    day_zhi: '子',
    hour_gan: '庚',
    hour_zhi: '午',
    wuxing: {
      year_wuxing: '金',
      month_wuxing: '金',
      day_wuxing: '木',
      hour_wuxing: '金',
      // 庚辛庚=金3，午巳午=火3，甲=木1，子=水1，土0（四柱八字计数）
      total_wuxing: { 金: 3, 木: 1, 水: 1, 火: 3, 土: 0 },
    },
    solar_term: '立夏',
    lunar_date: '四月廿一',
    zodiac: '马',
    constellation: '金牛座',
  },
}

/** Divination 样例：签文多行结构供 QianTong.parseSignContent 拆 解曰/诗句。 */
function makeDivination(req?: Partial<DivinationReq>): Divination {
  return {
    id: 1,
    question: req?.question || '近期运势如何',
    sign_number: 18,
    sign_content: '第18签：上上签\n天时地利人和，万事皆可成。\n解曰：\n宜守正待时，所求顺遂。',
    interpretation: '此签大吉。所问之事天时地利人和，顺势而为可成；守正待时，勿急勿躁。',
    category: req?.category || '综合',
    lucky_direction: '东方',
    lucky_color: '红色',
    advice: '谋定而后动，稳步前行；好事多磨，终有所成。',
    created_at: new Date().toISOString(),
  }
}

// 导出用例数组（brief Step 2「导出用例数组」）：mockAdapter 以 [...starfuyuMocks] 展开注册
export default [defineMock({
  // 星座偏好：读模块状态（初始白羊座；与 store select 的「服务端权威」口径一致）
  '[GET]/api/app/starfuyu/profile': () => ok<Profile>({ constellation: profileConstellation }),

  // 设置星座：写状态并回显（枚举校验留真联调，mock 不拦）
  '[PUT]/api/app/starfuyu/profile': ({ data }) => {
    const constellation = (data as Partial<Profile> | undefined)?.constellation
    if (constellation !== undefined)
      profileConstellation = constellation
    return ok<Profile>({ constellation: profileConstellation })
  },

  // 我的今日运势：随 profile 星座联动（选座后重进页/重选的闭环数据）
  '[GET]/api/app/starfuyu/fortune/my/today': () => ok<Fortune>(makeMyFortune()),

  // 今日全部 12 星座运势
  '[GET]/api/app/starfuyu/fortune/today': () =>
    ok<Fortune[]>(ZODIAC_NAMES.map((name, index) => makeFortune(name, index + 1))),

  // 计算八字：DeepSeek 排盘留真联调，mock 固定样例（页面只消费 bazi_info 四柱）
  '[POST]/api/app/starfuyu/bazi/calculate': () => ok<Bazi>(BAZI_SAMPLE),

  // 我的八字：二选一取成功样例（30504 空态信封不 mock，理由见文件头）
  '[GET]/api/app/starfuyu/bazi/my': () => ok<Bazi>(BAZI_SAMPLE),

  // 更新我的八字：同 calculate 固定样例（已存后再排盘走此径）
  '[PUT]/api/app/starfuyu/bazi/my': () => ok<Bazi>(BAZI_SAMPLE),

  // 求签落库：回显入参 question/category
  '[POST]/api/app/starfuyu/divination/draw': ({ data }) =>
    ok<Divination>(makeDivination(data as Partial<DivinationReq> | undefined)),

  // 求签历史：brief 钉死单页信封（页面未消费，仅注册覆盖）
  '[GET]/api/app/starfuyu/divination/history': () =>
    ok<PageData<Divination>>({ list: [makeDivination()], total: 1, page: 1, size: 10 }),
})]
